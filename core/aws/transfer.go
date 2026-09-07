// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// Package aws — transfer.go: AWS Transfer Family server fetcher.
//
// docs/resources/transfer.md §0/§3: ListServers is informative for the
// Wave-1 State signal, but every §2 related-panel field (EndpointDetails,
// StructuredLogDestinations, Certificate, IdentityProviderDetails) and both
// §3.2 config signals (SecurityPolicyName, the logging gap) live only on
// DescribedServer, and pivots must resolve on the FIRST detail open — so
// this fetcher does ListServers + DescribeServer per id (in-fetcher N+1,
// the mwaa.go/eks.go pattern), all findings fetcher-written (Source:
// "wave1"). NO transfer_issue_enrichment.go, NO Wave2 catalog field.
//
// Degraded rows are RICH here (unlike mwaa's name-only degradation): a
// denied DescribeServer still has the full ListedServer, so the row is
// built from list fields (including the state finding) and then the shared
// details-denied code is appended with transfer's own §4 sentence — a
// listed server never vanishes and never loses the data the list already
// gave us.
package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/transfer"
	transfertypes "github.com/aws/aws-sdk-go-v2/service/transfer/types"

	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// transfer.* FindingCodes — docs/resources/transfer.md §3/§4.
const (
	transferCodeOffline      domain.FindingCode = "transfer.warn.offline"
	transferCodeStarting     domain.FindingCode = "transfer.warn.starting"
	transferCodeStopping     domain.FindingCode = "transfer.warn.stopping"
	transferCodeStartFailed  domain.FindingCode = "transfer.broken.start_failed"
	transferCodeStopFailed   domain.FindingCode = "transfer.warn.stop_failed"
	transferCodeLegacyPolicy domain.FindingCode = "transfer.warn.legacy_policy"
	transferCodeNoLogging    domain.FindingCode = "transfer.warn.no_logging"
)

// transferLegacySecurityPolicies is the denylist of known-weak security
// policy names (docs/resources/transfer.md §3.2/§4) — a denylist rather
// than latest-chasing so FIPS/PQ/restricted policy variants never
// false-positive.
var transferLegacySecurityPolicies = map[string]bool{ //nolint:gochecknoglobals // static denylist
	"TransferSecurityPolicy-2018-11": true,
	"TransferSecurityPolicy-2020-06": true,
}

// FetchTransferServersPage fetches a single page of Transfer Family
// servers. ListServers carries the Wave-1 State signal but none of the §2
// related-panel fields, so every row gets a per-id DescribeServer call
// (in-fetcher N+1, the mwaa/eks pattern — no separate
// transfer_issue_enrichment.go). Per-id DescribeServer failures are
// aggregated (E3/E5) rather than dropping the row: the row is kept,
// built from the ListedServer fields, with the details-denied finding
// appended. A ListServers failure returns an error, never an empty success
// (AccessDenied contract).
func FetchTransferServersPage(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
	input := &transfer.ListServersInput{
		MaxResults: aws.Int32(DefaultPageSize),
	}
	if continuationToken != "" {
		input.NextToken = aws.String(continuationToken)
	}

	listOutput, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*transfer.ListServersOutput, error) {
		return c.Transfer.ListServers(ctx, input)
	})
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("listing Transfer Family servers: %w", err)
	}

	total := len(listOutput.Servers)
	var resources []resource.Resource
	var failures []Failure
	for i := range listOutput.Servers {
		listed := listOutput.Servers[i]
		id := aws.ToString(listed.ServerId)

		describeOutput, describeErr := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*transfer.DescribeServerOutput, error) {
			return c.Transfer.DescribeServer(ctx, &transfer.DescribeServerInput{ServerId: aws.String(id)})
		})
		switch {
		case describeErr != nil:
			failures = append(failures, FailedCall(id, describeErr))
			resources = append(resources, buildTransferDegradedResource(listed, describeErr))
		case describeOutput.Server == nil:
			failures = append(failures, UnusableAnswer(id, "nil server in DescribeServer response"))
			resources = append(resources, buildTransferDegradedResource(listed, nil))
		default:
			resources = append(resources, buildTransferResource(describeOutput.Server))
		}
	}

	return resource.FetchResult{
		Resources: resources,
		Pagination: &resource.PaginationMeta{
			IsTruncated: listOutput.NextToken != nil,
			NextToken:   aws.ToString(listOutput.NextToken),
			PageSize:    len(resources),
			TotalHint:   -1,
		},
	}, AggregateFailures("transfer: DescribeServer", failures, total)
}

// buildTransferResource constructs a Resource from a DescribeServer
// response. RawStruct is the *DescribedServer pointer (mwaa's
// value/pointer convention).
func buildTransferResource(server *transfertypes.DescribedServer) resource.Resource {
	id := aws.ToString(server.ServerId)
	findings := computeTransferFindings(server)
	statusPhrase := domain.StatusPhrase(findings)
	if statusPhrase == "" && server.State != "" && server.State != transfertypes.StateOnline {
		// Defensive parity with mwaa.go's status-phrase fallback: an
		// undocumented future State value still surfaces as raw text
		// instead of silently rendering blank. ONLINE with zero findings
		// legitimately stays "" (Healthy).
		statusPhrase = string(server.State)
	}

	return resource.Resource{
		ID:   id,
		Name: id,
		Fields: map[string]string{
			"server_id":              id,
			"status":                 statusPhrase,
			"domain":                 string(server.Domain),
			"endpoint_type":          string(server.EndpointType),
			"identity_provider_type": string(server.IdentityProviderType),
			"user_count":             mwaaInt32ToStr(server.UserCount),
			"arn":                    aws.ToString(server.Arn),
		},
		RawStruct: server,
		Findings:  findings,
	}
}

// computeTransferFindings builds the ordered Finding slice for one server:
// the state-bucket finding (if any), then legacy-policy, then no-logging —
// docs/resources/transfer.md §4 precedence order.
func computeTransferFindings(server *transfertypes.DescribedServer) []domain.Finding {
	var findings []domain.Finding

	if f, ok := transferStateFinding(server.State); ok {
		findings = append(findings, f)
	}

	if policy := aws.ToString(server.SecurityPolicyName); transferLegacySecurityPolicies[policy] {
		findings = append(findings, domain.Finding{
			Code:     transferCodeLegacyPolicy,
			Phrase:   "legacy security policy",
			Detail:   catalog.Detail(transferCodeLegacyPolicy),
			Severity: domain.SevWarn,
			Source:   "wave1",
		})
	}

	if server.LoggingRole == nil && len(server.StructuredLogDestinations) == 0 {
		findings = append(findings, domain.Finding{
			Code:     transferCodeNoLogging,
			Phrase:   "no activity logging",
			Detail:   catalog.Detail(transferCodeNoLogging),
			Severity: domain.SevWarn,
			Source:   "wave1",
		})
	}

	return findings
}

// transferStateFindings maps ListedServer/DescribedServer.State to its
// docs/resources/transfer.md §4 state-bucket Finding. ONLINE has no entry
// (Healthy — no finding).
var transferStateFindings = map[transfertypes.State]domain.Finding{ //nolint:gochecknoglobals // static lookup table, the transferLegacySecurityPolicies precedent
	transfertypes.StateOffline: {
		Code: transferCodeOffline, Phrase: "offline: not accepting transfers",
		Severity: domain.SevWarn, Source: "wave1",
	},
	transfertypes.StateStarting: {
		Code: transferCodeStarting, Phrase: "starting",
		Severity: domain.SevWarn, Source: "wave1",
	},
	transfertypes.StateStopping: {
		Code: transferCodeStopping, Phrase: "stopping",
		Severity: domain.SevWarn, Source: "wave1",
	},
	transfertypes.StateStartFailed: {
		Code: transferCodeStartFailed, Phrase: "start failed",
		Severity: domain.SevBroken, Source: "wave1",
	},
	transfertypes.StateStopFailed: {
		Code: transferCodeStopFailed, Phrase: "stop failed",
		Severity: domain.SevWarn, Source: "wave1",
	},
}

// transferStateFinding maps ListedServer/DescribedServer.State to its
// docs/resources/transfer.md §4 state-bucket Finding. ok is false for
// ONLINE (Healthy — no finding). Shared by the healthy-row and degraded-row
// builders since State is present on both ListedServer and DescribedServer.
func transferStateFinding(state transfertypes.State) (domain.Finding, bool) {
	f, ok := transferStateFindings[state]
	f.Detail = catalog.Detail(f.Code)
	return f, ok
}

// buildTransferDegradedResource builds the RICH degraded row for a server
// whose DescribeServer call failed: the row is kept using the ListedServer
// fields (including the state finding), then the classified details finding
// is appended — transfer's own "details denied" sentence for an
// authorization failure, the neutral "details unavailable" one otherwise —
// never the generic degraded_resource.go denied text, since this row keeps
// far more than just the name. err is the DescribeServer call's error (nil
// for a nil Server body). RawStruct is the *ListedServer pointer (the
// fallback shape §0 specifies), so every related checker's Pattern F read
// against *DescribedServer type-asserts cleanly to "not found" here rather
// than panicking.
func buildTransferDegradedResource(listed transfertypes.ListedServer, err error) resource.Resource {
	id := aws.ToString(listed.ServerId)

	var findings []domain.Finding
	if f, ok := transferStateFinding(listed.State); ok {
		findings = append(findings, f)
	}
	findings = append(findings, degradedDetailsFinding("transfer", err))
	statusPhrase := domain.StatusPhrase(findings)

	raw := listed
	return resource.Resource{
		ID:   id,
		Name: id,
		Fields: map[string]string{
			"server_id":              id,
			"status":                 statusPhrase,
			"domain":                 string(listed.Domain),
			"endpoint_type":          string(listed.EndpointType),
			"identity_provider_type": string(listed.IdentityProviderType),
			"user_count":             mwaaInt32ToStr(listed.UserCount),
			"arn":                    aws.ToString(listed.Arn),
		},
		RawStruct: &raw,
		Findings:  findings,
	}
}
