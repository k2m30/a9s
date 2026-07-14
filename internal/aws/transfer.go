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

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
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

// transferDetailsDeniedDetail is transfer's own §4 S5 sentence for the
// details-denied finding — deliberately NOT the generic
// degraded_resource.go text, because transfer's degraded row keeps the full
// ListedServer, not just the name.
const transferDetailsDeniedDetail = "Access to server details was denied; only the listed fields are visible."

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
	var failures []string
	for i := range listOutput.Servers {
		listed := listOutput.Servers[i]
		id := aws.ToString(listed.ServerId)

		describeOutput, describeErr := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*transfer.DescribeServerOutput, error) {
			return c.Transfer.DescribeServer(ctx, &transfer.DescribeServerInput{ServerId: aws.String(id)})
		})
		switch {
		case describeErr != nil:
			failures = append(failures, fmt.Sprintf("%s: %s", id, describeErr.Error()))
			resources = append(resources, buildTransferDegradedResource(listed))
		case describeOutput.Server == nil:
			failures = append(failures, fmt.Sprintf("%s: nil server in DescribeServer response", id))
			resources = append(resources, buildTransferDegradedResource(listed))
		default:
			resources = append(resources, buildTransferResource(describeOutput.Server))
		}
	}

	isTruncated := listOutput.NextToken != nil
	var nextToken string
	if listOutput.NextToken != nil {
		nextToken = *listOutput.NextToken
	}

	return resource.FetchResult{
		Resources: resources,
		Pagination: &resource.PaginationMeta{
			IsTruncated: isTruncated,
			NextToken:   nextToken,
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
	statusPhrase := phraseFromFindings(findings)
	if statusPhrase == "" && server.State != "" && server.State != transfertypes.StateOnline {
		// Defensive parity with mwaa.go's phraseFromFindings fallback: an
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
			Detail:   fmt.Sprintf("Security policy %s allows weak ciphers / old TLS; move to a current policy.", policy),
			Severity: domain.SevWarn,
			Source:   "wave1",
		})
	}

	if server.LoggingRole == nil && len(server.StructuredLogDestinations) == 0 {
		findings = append(findings, domain.Finding{
			Code:     transferCodeNoLogging,
			Phrase:   "no activity logging",
			Detail:   "Neither a logging role nor structured log destinations are configured.",
			Severity: domain.SevWarn,
			Source:   "wave1",
		})
	}

	return findings
}

// transferStateFinding maps ListedServer/DescribedServer.State to its
// docs/resources/transfer.md §4 state-bucket Finding. ok is false for
// ONLINE (Healthy — no finding). Shared by the healthy-row and degraded-row
// builders since State is present on both ListedServer and DescribedServer.
func transferStateFinding(state transfertypes.State) (domain.Finding, bool) {
	switch state {
	case transfertypes.StateOffline:
		return domain.Finding{
			Code: transferCodeOffline, Phrase: "offline: not accepting transfers",
			Detail:   "Server is offline; partners cannot connect until it is started.",
			Severity: domain.SevWarn, Source: "wave1",
		}, true
	case transfertypes.StateStarting:
		return domain.Finding{
			Code: transferCodeStarting, Phrase: "starting",
			Detail:   "Server is starting; not yet fully able to respond.",
			Severity: domain.SevWarn, Source: "wave1",
		}, true
	case transfertypes.StateStopping:
		return domain.Finding{
			Code: transferCodeStopping, Phrase: "stopping",
			Detail:   "Server is stopping; transfers are draining.",
			Severity: domain.SevWarn, Source: "wave1",
		}, true
	case transfertypes.StateStartFailed:
		return domain.Finding{
			Code: transferCodeStartFailed, Phrase: "start failed",
			Detail:   "Server failed to come online; partner transfers are down.",
			Severity: domain.SevBroken, Source: "wave1",
		}, true
	case transfertypes.StateStopFailed:
		return domain.Finding{
			Code: transferCodeStopFailed, Phrase: "stop failed",
			Detail:   "Stop failed; the server may still be serving transfers.",
			Severity: domain.SevWarn, Source: "wave1",
		}, true
	}
	return domain.Finding{}, false
}

// buildTransferDegradedResource builds the RICH degraded row for a server
// whose DescribeServer call failed: the row is kept using the ListedServer
// fields (including the state finding), then the shared details-denied
// finding is appended with transfer's own §4 sentence — never the generic
// degraded_resource.go text, since this row keeps far more than just the
// name. RawStruct is the *ListedServer pointer (the fallback shape §0
// specifies), so every related checker's Pattern F read against
// *DescribedServer type-asserts cleanly to "not found" here rather than
// panicking.
func buildTransferDegradedResource(listed transfertypes.ListedServer) resource.Resource {
	id := aws.ToString(listed.ServerId)

	var findings []domain.Finding
	if f, ok := transferStateFinding(listed.State); ok {
		findings = append(findings, f)
	}
	findings = append(findings, domain.Finding{
		Code:     DetailsDeniedCode("transfer"),
		Phrase:   detailsDeniedPhrase,
		Detail:   transferDetailsDeniedDetail,
		Severity: domain.SevWarn,
		Source:   "wave1",
	})
	statusPhrase := phraseFromFindings(findings)

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
