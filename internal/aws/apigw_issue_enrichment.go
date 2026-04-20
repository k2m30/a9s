// apigw_issue_enrichment.go — Wave 2 issue enrichment for the apigw resource type.
package aws

import (
	"context"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	apigatewayv2types "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	registerIssueEnricher("apigw", EnrichAPIGatewayStage, 100)
	resource.RegisterIssueEnricherFieldKeys("apigw", []string{"stages_count"})
}

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
func EnrichAPIGatewayStage(ctx context.Context, clients *ServiceClients, resources []resource.Resource) (IssueEnricherResult, error) {
	findings := make(map[string]resource.EnrichmentFinding)
	fieldUpdates := make(map[string]map[string]string)
	truncatedIDs := make(map[string]bool)
	if clients.APIGatewayV2 == nil {
		return IssueEnricherResult{Findings: findings, TruncatedIDs: truncatedIDs}, nil
	}
	truncated := len(resources) > EnrichmentCap
	for i, r := range resources {
		if i >= EnrichmentCap {
			break
		}
		apiID := r.ID
		if apiID == "" {
			continue
		}
		var stages []apigatewayv2types.Stage
		stagesTruncated := false
		var stagesNextToken *string
		stagePages := 0
		for {
			if stagePages >= PerParentPageCap {
				stagesTruncated = true
				truncatedIDs[r.ID] = true
				break
			}
			out, err := clients.APIGatewayV2.GetStages(ctx, &apigatewayv2.GetStagesInput{
				ApiId:     aws.String(apiID),
				NextToken: stagesNextToken,
			})
			stagePages++
			if err != nil {
				truncated = true
				truncatedIDs[r.ID] = true
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
			stagesCountStr = resource.FormatApproximate(len(stages))
		}
		fieldUpdates[apiID] = map[string]string{"stages_count": stagesCountStr}
		var summaries []string
		var rows []resource.FindingRow

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
					summaries = append(summaries, "no throttling configured (DoS risk)")
					rows = append(rows, resource.FindingRow{
						Label: "Stage",
						Value: *stageName,
						Tier:  "~",
					})
					rows = append(rows, resource.FindingRow{
						Label: "Issue",
						Value: "no throttling configured (DoS risk)",
						Tier:  "~",
					})
				}
			}

			// Check access log settings.
			if stage.AccessLogSettings == nil {
				summaries = append(summaries, "access logs disabled")
				rows = append(rows, resource.FindingRow{
					Label: "Stage",
					Value: *stageName,
					Tier:  "~",
				})
				rows = append(rows, resource.FindingRow{
					Label: "Issue",
					Value: "access logs disabled",
					Tier:  "~",
				})
			}
		}

		stagesCount := len(stages)
		if stagesCount == 0 && !stagesTruncated && !truncatedIDs[r.ID] {
			// No deployed stages — surface as an informational finding.
			// Only emitted when stage fetch succeeded (no error, no page cap).
			findings[apiID] = resource.EnrichmentFinding{
				Severity: "~",
				Summary:  "no deployed stages",
				Rows: []resource.FindingRow{{
					Label: "Issue",
					Value: "no deployed stages",
					Tier:  "~",
				}},
			}
			continue
		}
		if len(summaries) == 0 {
			continue
		}
		// Deduplicate repeated summary messages.
		seen := make(map[string]bool)
		var uniqueSummaries []string
		for _, s := range summaries {
			if !seen[s] {
				seen[s] = true
				uniqueSummaries = append(uniqueSummaries, s)
			}
		}
		findings[apiID] = resource.EnrichmentFinding{
			Severity: "~",
			Summary:  strings.Join(uniqueSummaries, "; "),
			Rows:     rows,
		}
	}
	// All API Gateway findings are severity "~" (informational).
	// IssueCount counts only "!" severity findings; "~" do not contribute.
	return IssueEnricherResult{IssueCount: 0, Truncated: truncated, TruncatedIDs: truncatedIDs, Findings: findings, FieldUpdates: fieldUpdates}, nil
}
