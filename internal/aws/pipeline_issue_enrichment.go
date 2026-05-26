// pipeline_issue_enrichment.go — Wave 2 issue enrichment for the pipeline resource type.
package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/codepipeline"
	cptypes "github.com/aws/aws-sdk-go-v2/service/codepipeline/types"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// pipeline canonical FindingCodes.
const (
	pipelineCodeStageFailed domain.FindingCode = "pipeline.stage-failed"
)

// EnrichCodePipelineStatus calls GetPipelineState for each pipeline (1 per pipeline, cap ~50).
// Returns a Finding for each pipeline with a failed stage.
// Severity is "!" (broken/degraded). Summary: "stage <Name> failed".
func EnrichCodePipelineStatus(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string]domain.Finding),
		TruncatedIDs: make(map[string]bool),
		FieldUpdates: make(map[string]map[string]string),
	}
	if clients.CodePipeline == nil {
		return result, nil
	}
	truncated := len(resources) > EnrichmentCap
	var failures []string
	total := 0
	for i, r := range resources {
		if i >= EnrichmentCap {
			break
		}
		if r.Name == "" {
			continue
		}
		total++
		out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*codepipeline.GetPipelineStateOutput, error) {
			return clients.CodePipeline.GetPipelineState(ctx, &codepipeline.GetPipelineStateInput{
				Name: aws.String(r.Name),
			})
		})
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", r.ID, err))
			truncated = true
			result.TruncatedIDs[r.ID] = true
			continue
		}
		key := r.ID
		if key == "" {
			key = r.Name
		}
		lastStatus := "OK"
		for _, stage := range out.StageStates {
			if stage.LatestExecution == nil || stage.LatestExecution.Status != cptypes.StageExecutionStatusFailed {
				continue
			}
			stageName := ""
			if stage.StageName != nil {
				stageName = *stage.StageName
			}
			lastStatus = stageName
			rows := []domain.DetailRow{
				{Label: "Failed Stage", Value: stageName, Tier: "!"},
				{Label: "Status", Value: string(stage.LatestExecution.Status)},
			}
			// Collect error details from any failed action in this stage.
			for _, action := range stage.ActionStates {
				if action.LatestExecution == nil {
					continue
				}
				if action.LatestExecution.Status != cptypes.ActionExecutionStatusFailed {
					continue
				}
				if action.LatestExecution.ErrorDetails != nil && action.LatestExecution.ErrorDetails.Message != nil {
					msg := *action.LatestExecution.ErrorDetails.Message
					if msg != "" {
						rows = append(rows, domain.DetailRow{Label: "Error", Value: msg, Tier: "!"})
					}
					break
				}
			}
			setWave2Finding(&result, key, pipelineCodeStageFailed, fmt.Sprintf("stage %s failed", stageName), "!", "pipeline", rows)
			break // first failed stage is sufficient
		}
		result.FieldUpdates[key] = map[string]string{"last_status": lastStatus}
	}
	result.IssueCount = len(result.Findings)
	result.Truncated = truncated
	return result,
		AggregateFailures("pipeline-enrich: GetPipelineState", failures, total)
}
