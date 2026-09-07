// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// apigw_issue_enrichment.go — Wave 2 issue enrichment for the apigw resource type.
package aws

import (
	"context"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigateway"
	apigwtypes "github.com/aws/aws-sdk-go-v2/service/apigateway/types"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	apigatewayv2types "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"

	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/iampolicy"
	"github.com/k2m30/a9s/v3/core/resource"
)

// apigw canonical FindingCodes.
const (
	apigwCodeNoDeployedStages  domain.FindingCode = "apigw.no-deployed-stages"
	apigwCodeStageConfigIssues domain.FindingCode = "apigw.stage-config-issues"

	// CodeAPIGWNoAuthorizerPublic — a REST API reachable from the internet
	// with no authorizer and no scoped resource policy.
	CodeAPIGWNoAuthorizerPublic domain.FindingCode = "apigw.no-authorizer-public"
	// CodeAPIGWNoAuthorizer — any other API with no authorizer: a private
	// REST API, or an HTTP API, which has no private endpoint type.
	CodeAPIGWNoAuthorizer domain.FindingCode = "apigw.no-authorizer"
	// CodeAPIGWNoAccessLogs — a stage with no access log settings. One code
	// covers the REST and the HTTP lane so the badge counts the gap once.
	CodeAPIGWNoAccessLogs domain.FindingCode = "apigw.no-access-logs"
	// CodeAPIGWTracingOff — a REST stage with tracing switched off.
	CodeAPIGWTracingOff domain.FindingCode = "apigw.tracing-off"
	// CodeAPIGWStageVariableSecret — a REST stage variable whose value scans
	// as a credential. The value never leaves this package.
	CodeAPIGWStageVariableSecret domain.FindingCode = "apigw.stage-variable-secret" //nolint:gosec // G101 false positive: a finding code, not a credential
)

// EnrichAPIGatewayStage calls GetStages per API (cap EnrichmentCap)
// and returns a Finding for any API with stage-level throttling or access-log issues.
//
// Findings (severity "~" — informational):
//   - Any stage with DefaultRouteSettings.ThrottlingBurstLimit == 0 OR ThrottlingRateLimit == 0
//     → "no throttling configured (DoS risk)"
//   - Any stage with AccessLogSettings == nil → "access logs disabled"
//
// Findings are aggregated per API (one finding per API, covering all stages).
// Skip if clients.APIGatewayV2 == nil. Per-API errors → truncated.
func EnrichAPIGatewayStage(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string][]domain.Finding),
		FieldUpdates: make(map[string]map[string]string),
		TruncatedIDs: make(map[string]bool),
	}
	if clients.APIGatewayV2 == nil {
		return result, nil
	}
	v1, hasV1 := clients.APIGatewayV1.(apigwV1API)
	ownAccount := accountIDFromClients(ctx, clients, clients.IdentityStore())
	n := min(len(resources), EnrichmentCap)
	var mu sync.Mutex
	_ = ForEachParallel(ctx, n, EnrichmentParallelism, func(i int) {
		r := resources[i]
		apiID := r.ID
		if apiID == "" {
			return
		}
		// The REST lane is a different API with different calls. An account
		// with no REST client still gets its HTTP APIs enriched.
		if r.Fields["protocol"] == "REST" {
			if !hasV1 {
				return
			}
			mu.Lock()
			defer mu.Unlock()
			if apigwRESTFindings(ctx, v1, &result, r, ownAccount) {
				result.TruncatedIDs[r.ID] = true
			}
			return
		}
		var stages []apigatewayv2types.Stage
		stagesTruncated := false
		var stagesNextToken *string
		stagePages := 0
		fetchErr := false
		for {
			if stagePages >= PerParentPageCap {
				stagesTruncated = true
				break
			}
			out, err := clients.APIGatewayV2.GetStages(ctx, &apigatewayv2.GetStagesInput{
				ApiId:     aws.String(apiID),
				NextToken: stagesNextToken,
			})
			stagePages++
			if err != nil {
				fetchErr = true
				break
			}
			stages = append(stages, out.Items...)
			if out.NextToken == nil {
				break
			}
			stagesNextToken = out.NextToken
		}

		stagesCountStr := resource.FormatExact(len(stages))
		if stagesTruncated {
			stagesCountStr = resource.FormatTruncated(len(stages))
		}
		var rows []domain.DetailRow

		for _, stage := range stages {
			stageName := stage.StageName
			if stageName == nil {
				stageName = aws.String("(unnamed)")
			}

			// Check throttling on DefaultRouteSettings.
			if drs := stage.DefaultRouteSettings; drs != nil {
				noThrottle := (drs.ThrottlingBurstLimit != nil && *drs.ThrottlingBurstLimit == 0) ||
					(drs.ThrottlingRateLimit != nil && *drs.ThrottlingRateLimit == 0)
				if noThrottle {
					rows = append(rows, domain.DetailRow{
						Label: "Stage",
						Value: *stageName,
						Tier:  "~",
					})
					rows = append(rows, domain.DetailRow{
						Label: "Issue",
						Value: "no throttling configured (DoS risk)",
						Tier:  "~",
					})
				}
			}

			// Check access log settings.
			if stage.AccessLogSettings == nil {
				rows = append(rows, domain.DetailRow{
					Label: "Stage",
					Value: *stageName,
					Tier:  "~",
				})
				rows = append(rows, domain.DetailRow{
					Label: "Issue",
					Value: "access logs disabled",
					Tier:  "~",
				})
			}
		}

		mu.Lock()
		defer mu.Unlock()

		if apigwHTTPNoAuthorizer(ctx, clients, &result, apiID) {
			result.TruncatedIDs[r.ID] = true
		}

		if stagesTruncated || fetchErr {
			result.TruncatedIDs[r.ID] = true
		}
		result.FieldUpdates[apiID] = map[string]string{"stages_count": stagesCountStr}

		stagesCount := len(stages)
		if stagesCount == 0 && !stagesTruncated && !fetchErr {
			// No deployed stages — surface as an informational finding.
			// Only emitted when stage fetch succeeded (no error, no page cap).
			// The phrase says there are none; the row says what kind of API
			// is sitting undeployed, which the phrase cannot.
			setWave2Finding(&result, apiID, apigwCodeNoDeployedStages, "no deployed stages", "~", "apigw", []domain.DetailRow{{
				Label: "Protocol",
				Value: strings.ToLower(r.Fields["protocol"]),
				Tier:  "~",
			}})

			return
		}
		if len(rows) == 0 {
			return
		}
		setWave2Finding(&result, apiID, apigwCodeStageConfigIssues,
			catalog.Phrase(apigwCodeStageConfigIssues), "~", "apigw", rows)
	})
	// apigw.no-authorizer-public and apigw.stage-variable-secret are "!", so
	// the cap now bounds the issue count and a capped pass must say so rather
	// than under-report the badge.
	result.Truncated = len(resources) > EnrichmentCap
	return result, nil
}

// apigwV1API is the pair of REST calls the enricher needs. It is reached by
// type assertion off clients.APIGatewayV1 rather than by widening
// APIGatewayV1API, so a client or fake that predates these calls still
// satisfies the aggregate. Same shape as CodeArtifactListPackagesAPI.
type apigwV1API interface {
	APIGatewayV1GetAuthorizersAPI
	APIGatewayV1GetStagesAPI
}

// apigwRESTFindings evaluates rows 18-21 for one REST API. It returns true
// when the stage listing failed, so the caller marks the row truncated rather
// than reporting an API whose posture it could not read as clean.
func apigwRESTFindings(ctx context.Context, api apigwV1API, result *IssueEnricherResult, r resource.Resource, ownAccount string) bool {
	apiID := r.ID
	emit := func(code domain.FindingCode, phrase, tier string, rows ...domain.DetailRow) {
		setWave2Finding(result, apiID, code, phrase, tier, "apigw", rows)
	}

	authorizers, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*apigateway.GetAuthorizersOutput, error) {
		return api.GetAuthorizers(ctx, &apigateway.GetAuthorizersInput{RestApiId: aws.String(apiID)})
	})
	if err != nil {
		return true
	}
	if len(authorizers.Items) == 0 {
		endpoint := strings.ToLower(r.Fields["endpoint"])
		// A resource policy that grants under a condition is a real control;
		// one open to everyone is not, and the API stays exposed.
		scoped := false
		if doc, perr := iampolicy.Parse(apigwRESTPolicy(r)); perr == nil {
			scoped = iampolicy.Evaluate(doc, ownAccount).Conditioned
		}
		switch {
		case scoped:
			// guarded by the policy — no finding
		case endpoint == "":
			// The endpoint type is unknown, and the common contract's nil rule
			// makes an unread field unknown rather than misconfigured.
		case endpoint == "private":
			emit(CodeAPIGWNoAuthorizer, "no authorizer", "~",
				domain.DetailRow{Label: "Authorizers", Value: "0", Tier: "~"},
				domain.DetailRow{Label: "Endpoint", Value: endpoint, Tier: "~"})
		default:
			emit(CodeAPIGWNoAuthorizerPublic, "internet-facing with no authorizer", "!",
				domain.DetailRow{Label: "Authorizers", Value: "0", Tier: "!"},
				domain.DetailRow{Label: "Endpoint", Value: endpoint, Tier: "!"})
		}
	}

	stages, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*apigateway.GetStagesOutput, error) {
		return api.GetStages(ctx, &apigateway.GetStagesInput{RestApiId: aws.String(apiID)})
	})
	if err != nil {
		return true
	}
	for _, st := range stages.Item {
		name := aws.ToString(st.StageName)
		if st.AccessLogSettings == nil {
			emit(CodeAPIGWNoAccessLogs, "no access logs", "~",
				domain.DetailRow{Label: "Stage", Value: name, Tier: "~"})
		}
		if !st.TracingEnabled {
			emit(CodeAPIGWTracingOff, "X-Ray tracing off", "~",
				domain.DetailRow{Label: "Stage", Value: name, Tier: "~"})
		}
		// Rows carry where and what kind, never the value itself.
		if rows := secretScanRows(st.Variables); len(rows) > 0 {
			emit(CodeAPIGWStageVariableSecret, "credential in stage variables", "!",
				append([]domain.DetailRow{{Label: "Stage", Value: name, Tier: "!"}}, rows...)...)
		}
	}
	return false
}

// apigwRESTPolicy returns the API's resource policy document, which the
// fetcher keeps on the retained RestApi.
func apigwRESTPolicy(r resource.Resource) string {
	api, ok := r.RawStruct.(apigwtypes.RestApi)
	if !ok {
		return ""
	}
	return aws.ToString(api.Policy)
}

// apigwHTTPNoAuthorizer evaluates row 18 for one HTTP (v2) API. There is no
// private endpoint type on v2, so an unauthorized one is always the warn code.
func apigwHTTPNoAuthorizer(ctx context.Context, clients *ServiceClients, result *IssueEnricherResult, apiID string) bool {
	// One authorizer on any page is enough to clear the row, so the walk stops
	// at the first page that has one.
	input := &apigatewayv2.GetAuthorizersInput{ApiId: aws.String(apiID)}
	for {
		out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*apigatewayv2.GetAuthorizersOutput, error) {
			return clients.APIGatewayV2.GetAuthorizers(ctx, input)
		})
		switch {
		case err != nil:
			return true
		case len(out.Items) > 0:
			return false
		case out.NextToken == nil:
			setWave2Finding(result, apiID, CodeAPIGWNoAuthorizer, "no authorizer", "~", "apigw",
				[]domain.DetailRow{{Label: "Authorizers", Value: "0", Tier: "~"}})
			return false
		}
		input.NextToken = out.NextToken
	}

}
