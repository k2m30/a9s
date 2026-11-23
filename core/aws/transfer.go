// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// Package aws — transfer.go: AWS Transfer Family server fetcher.
//
// ListServers carries the Wave-1 State signal, but every related-panel field
// (EndpointDetails, StructuredLogDestinations, Certificate,
// IdentityProviderDetails) and both config signals (SecurityPolicyName, the
// logging gap) live only on DescribedServer, and pivots must resolve on the
// first detail open — so this fetcher does ListServers + DescribeServer per
// id (in-fetcher N+1, the mwaa.go/eks.go pattern), and every finding is
// fetcher-written (Source: "wave1").
//
// Degraded rows are RICH here (unlike mwaa's name-only degradation): a
// denied DescribeServer still has the full ListedServer, so the row is
// built from list fields (including the state finding) and then the shared
// details-denied code is appended with transfer's own sentence — a
// listed server never vanishes and never loses the data the list already
// gave us.
package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/transfer"
	transfertypes "github.com/aws/aws-sdk-go-v2/service/transfer/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// transfer.* FindingCodes — docs/resources/transfer.md.
const (
	transferCodeOffline      domain.FindingCode = "transfer.warn.offline"
	transferCodeStarting     domain.FindingCode = "transfer.warn.starting"
	transferCodeStopping     domain.FindingCode = "transfer.warn.stopping"
	transferCodeStartFailed  domain.FindingCode = "transfer.broken.start_failed"
	transferCodeStopFailed   domain.FindingCode = "transfer.warn.stop_failed"
	transferCodeLegacyPolicy domain.FindingCode = "transfer.warn.legacy_policy"
	transferCodeNoLogging    domain.FindingCode = "transfer.warn.no_logging"
	transferCodeUnrecognised domain.FindingCode = "transfer.warn.unrecognised_state"
)

// FetchTransferServersPage fetches a single page of Transfer Family
// servers. ListServers carries the Wave-1 State signal but none of the
// related-panel fields, so every row gets a per-id DescribeServer call
// (in-fetcher N+1, the mwaa/eks pattern). Per-id DescribeServer failures
// are aggregated rather than dropping the row: the row is kept, built from
// the ListedServer fields, with the details-denied finding appended. A
// ListServers failure returns an error, never an empty success.
func FetchTransferServersPage(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
	input := &transfer.ListServersInput{
		MaxResults: aws.Int32(DefaultPageSize),
	}
	if continuationToken != "" {
		input.NextToken = aws.String(continuationToken)
	}

	listOutput, err := c.Transfer.ListServers(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("listing Transfer Family servers: %w", err)
	}

	var resources []resource.Resource
	var tally describeTally
	for i := range listOutput.Servers {
		listed := listOutput.Servers[i]
		id := aws.ToString(listed.ServerId)

		rows := tally.one("DescribeServer", id, id, func() (*resource.Resource, error) {
			out, err := c.Transfer.DescribeServer(ctx, &transfer.DescribeServerInput{ServerId: aws.String(id)})
			if err != nil {
				return nil, err
			}
			if out.Server == nil {
				return nil, UnusableAnswerErr{Call: "DescribeServer", Field: "server"}
			}
			r := buildTransferResource(out.Server)
			return &r, nil
		}, func(err error) resource.Resource { return buildTransferDegradedResource(listed, err) })
		resources = append(resources, rows...)
	}

	return resource.FetchResult{
		Resources: resources,
		Pagination: &resource.PaginationMeta{
			IsTruncated: listOutput.NextToken != nil,
			NextToken:   aws.ToString(listOutput.NextToken),
			PageSize:    len(resources),
			TotalHint:   -1,
		},
	}, tally.err("transfer: DescribeServer")
}

// buildTransferResource constructs a Resource from a DescribeServer
// response. RawStruct is the *DescribedServer pointer (mwaa's
// value/pointer convention).
func buildTransferResource(server *transfertypes.DescribedServer) resource.Resource {
	id := aws.ToString(server.ServerId)
	findings := computeTransferFindings(server)

	return withWave1("transfer", resource.Resource{
		ID:   id,
		Name: id,
		Fields: map[string]string{
			"server_id":              id,
			"status":                 string(server.State),
			"domain":                 string(server.Domain),
			"endpoint_type":          string(server.EndpointType),
			"identity_provider_type": string(server.IdentityProviderType),
			"user_count":             mwaaInt32ToStr(server.UserCount),
			"arn":                    aws.ToString(server.Arn),
		},
		RawStruct: server,
	}, findings...)
}

// computeTransferFindings builds the ordered Finding slice for one server:
// the state-bucket finding (if any), then no-logging — the precedence order
// in docs/resources/transfer.md. The security policy is judged in Wave 2
// (EnrichTransferSecurityPolicy) by what DescribeSecurityPolicy says it
// allows.
func computeTransferFindings(server *transfertypes.DescribedServer) []domain.Finding {
	var posture []domain.Finding
	if server.LoggingRole == nil && len(server.StructuredLogDestinations) == 0 {
		posture = append(posture, wave1Finding(transferCodeNoLogging))
	}
	return withState("transfer", string(server.State), posture)
}

// buildTransferDegradedResource builds the RICH degraded row for a server
// whose DescribeServer call failed: the row is kept using the ListedServer
// fields (including the state finding), then the classified details finding
// is appended — transfer's own "details denied" sentence for an
// authorization failure, the neutral "details unavailable" one otherwise —
// never the generic degraded_resource.go denied text, since this row keeps
// far more than just the name. err is the DescribeServer call's error (nil
// for a nil Server body). RawStruct is the *ListedServer pointer, so every
// related checker's Pattern F read
// against *DescribedServer type-asserts cleanly to "not found" here rather
// than panicking.
func buildTransferDegradedResource(listed transfertypes.ListedServer, err error) resource.Resource {
	id := aws.ToString(listed.ServerId)

	findings := append(withState("transfer", string(listed.State), nil), degradedDetailsFinding("transfer", err))

	raw := listed
	return withWave1("transfer", resource.Resource{
		ID:   id,
		Name: id,
		Fields: map[string]string{
			"server_id":              id,
			"status":                 string(listed.State),
			"domain":                 string(listed.Domain),
			"endpoint_type":          string(listed.EndpointType),
			"identity_provider_type": string(listed.IdentityProviderType),
			"user_count":             mwaaInt32ToStr(listed.UserCount),
			"arn":                    aws.ToString(listed.Arn),
		},
		RawStruct: &raw,
	}, findings...)
}
