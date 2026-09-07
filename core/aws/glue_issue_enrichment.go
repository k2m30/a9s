// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// glue_issue_enrichment.go — Wave 2 issue enrichment for the glue resource type.
package aws

import (
	"context"
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/glue"
	gluetypes "github.com/aws/aws-sdk-go-v2/service/glue/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
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
		Findings:     make(map[string][]domain.Finding),
		TruncatedIDs: make(map[string]bool),
		FieldUpdates: make(map[string]map[string]string),
	}
	if clients.Glue == nil {
		return result, nil
	}
	truncated := false
	resources = capAtEnrichmentCap(&result, resources, resourceIDsOf)
	n := len(resources)
	var failures []Failure
	var mu sync.Mutex
	_ = ForEachParallel(ctx, n, EnrichmentParallelism, func(i int) {
		r := resources[i]
		if r.Name == "" {
			return
		}
		out, err := clients.Glue.GetJobRuns(ctx, &glue.GetJobRunsInput{
			JobName:    aws.String(r.Name),
			MaxResults: aws.Int32(1),
		})
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			truncated = true
			MarkSkipped(&result, r.ID, &failures, err)
			return
		}
		key := r.ID
		if key == "" {
			key = r.Name
		}
		if len(out.JobRuns) > 0 {
			run := out.JobRuns[0]
			s := run.JobRunState
			if s == gluetypes.JobRunStateFailed || s == gluetypes.JobRunStateError || s == gluetypes.JobRunStateTimeout {
				stateVal := string(s)
				statePhrase := domain.HumanizeStatusPhrase(stateVal)
				var rows []domain.DetailRow
				if run.CompletedOn != nil {
					rows = append(rows, domain.DetailRow{Label: "Ended", Value: run.CompletedOn.Format("2006-01-02")})
				}
				if run.ErrorMessage != nil && *run.ErrorMessage != "" {
					rows = append(rows, domain.DetailRow{Label: "Error", Value: *run.ErrorMessage, Tier: "!"})
				}
				setWave2Finding(&result, key, glueCodeLatestRunFailed, fmt.Sprintf("latest run %s", statePhrase), "!", "glue", rows)
				result.FieldUpdates[key] = map[string]string{"last_run": stateVal}
			} else {
				result.FieldUpdates[key] = map[string]string{"last_run": "OK"}
			}
		}
	})
	SetTruncated(&result, truncated)
	return result, AggregateFailures("glue-enrich: GetJobRuns", failures, n)
}
