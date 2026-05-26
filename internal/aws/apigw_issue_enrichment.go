// apigw_issue_enrichment.go — Wave 2 issue enrichment for the apigw resource type.
package aws

import (
	"context"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	apigatewayv2types "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// apigw canonical FindingCodes.
const (
	apigwCodeNoDeployedStages   domain.FindingCode = "apigw.no-deployed-stages"
	apigwCodeStageConfigIssues  domain.FindingCode = "apigw.stage-config-issues"
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
		Findings:     make(map[string]domain.Finding),
		FieldUpdates: make(map[string]map[string]string),
		TruncatedIDs: make(map[string]bool),
	}
	if clients.APIGatewayV2 == nil {
		return result, nil
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
				result.TruncatedIDs[r.ID] = true
				break
			}
			out, err := clients.APIGatewayV2.GetStages(ctx, &apigatewayv2.GetStagesInput{
				ApiId:     aws.String(apiID),
				NextToken: stagesNextToken,
			})
			stagePages++
			if err != nil {
				truncated = true
				result.TruncatedIDs[r.ID] = true
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
		result.FieldUpdates[apiID] = map[string]string{"stages_count": stagesCountStr}
		var summaries []string
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
					summaries = append(summaries, "no throttling configured (DoS risk)")
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
				summaries = append(summaries, "access logs disabled")
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

		stagesCount := len(stages)
		if stagesCount == 0 && !stagesTruncated && !result.TruncatedIDs[r.ID] {
			// No deployed stages — surface as an informational finding.
			// Only emitted when stage fetch succeeded (no error, no page cap).
			setWave2Finding(&result, apiID, apigwCodeNoDeployedStages, "no deployed stages", "~", "apigw", []domain.DetailRow{{
				Label: "Issue",
				Value: "no deployed stages",
				Tier:  "~",
			}})
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
		setWave2Finding(&result, apiID, apigwCodeStageConfigIssues, strings.Join(uniqueSummaries, "; "), "~", "apigw", rows)
	}
	// All API Gateway findings are severity "~" (informational).
	// IssueCount counts only "!" severity findings; "~" do not contribute.
	result.IssueCount = 0
	result.Truncated = truncated
	return result, nil
}
