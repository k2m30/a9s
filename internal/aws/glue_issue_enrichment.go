// glue_issue_enrichment.go — Wave 2 issue enrichment for the glue resource type.
package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/glue"
	gluetypes "github.com/aws/aws-sdk-go-v2/service/glue/types"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// glue canonical FindingCodes.
const (
	glueCodeLatestRunFailed domain.FindingCode = "glue.latest-run-failed"
)

// EnrichGlueJobStatus calls GetJobRuns(max:1) for each job (1 per job, cap ~50).
// Returns a Finding for each job whose latest run is FAILED, ERROR, or TIMEOUT.
// Severity is "!" (broken/degraded). Summary: "latest run <STATUS>".
func EnrichGlueJobStatus(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string]domain.Finding),
		TruncatedIDs: make(map[string]bool),
		FieldUpdates: make(map[string]map[string]string),
	}
	if clients.Glue == nil {
		return result, nil
	}
	truncated := len(resources) > EnrichmentCap
	for i, r := range resources {
		if i >= EnrichmentCap {
			break
		}
		if r.Name == "" {
			continue
		}
		out, err := clients.Glue.GetJobRuns(ctx, &glue.GetJobRunsInput{
			JobName:    aws.String(r.Name),
			MaxResults: aws.Int32(1),
		})
		if err != nil {
			truncated = true
			result.TruncatedIDs[r.ID] = true
			continue
		}
		key := r.ID
		if key == "" {
			key = r.Name
		}
		if len(out.JobRuns) > 0 {
			run := out.JobRuns[0]
			s := run.JobRunState
			if s == gluetypes.JobRunStateFailed || s == gluetypes.JobRunStateError || s == gluetypes.JobRunStateTimeout {
				rows := []domain.DetailRow{
					{Label: "State", Value: string(s), Tier: "!"},
				}
				if run.CompletedOn != nil {
					rows = append(rows, domain.DetailRow{Label: "Ended", Value: run.CompletedOn.Format("2006-01-02")})
				}
				if run.ErrorMessage != nil && *run.ErrorMessage != "" {
					rows = append(rows, domain.DetailRow{Label: "Error", Value: *run.ErrorMessage, Tier: "!"})
				}
				setWave2Finding(&result, key, glueCodeLatestRunFailed, fmt.Sprintf("latest run %s", string(s)), "!", "glue", rows)
				result.FieldUpdates[key] = map[string]string{"last_run": string(s)}
			} else {
				result.FieldUpdates[key] = map[string]string{"last_run": "OK"}
			}
		}
	}
	result.IssueCount = len(result.Findings)
	result.Truncated = truncated
	return result, nil
}
