// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// sfn_issue_enrichment.go — Wave 2 issue enrichment for the sfn resource type.
package aws

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	sfntypes "github.com/aws/aws-sdk-go-v2/service/sfn/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// sfn canonical FindingCodes.
const (
	sfnCodeLatestExecutionFailed domain.FindingCode = "sfn.latest-execution-failed"
)

// EnrichStepFunctionsStatus calls ListExecutions(max:1) for each state machine (1 per SFN, cap ~50).
// Returns a Finding for each state machine whose latest execution is FAILED, TIMED_OUT, or ABORTED.
// Severity is "!" (broken/degraded). Summary: "latest execution <STATUS>".
func EnrichStepFunctionsStatus(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string][]domain.Finding),
		TruncatedIDs: make(map[string]bool),
		FieldUpdates: make(map[string]map[string]string),
	}
	if clients.SFN == nil {
		return result, nil
	}
	truncated := len(resources) > EnrichmentCap
	var failures []string
	total := 0
	n := min(len(resources), EnrichmentCap)
	var mu sync.Mutex
	_ = ForEachParallel(ctx, n, EnrichmentParallelism, func(i int) {
		r := resources[i]
		if r.ID == "" {
			return
		}
		// ListExecutions requires the state-machine ARN. The sfn fetcher
		// (sfn.go) sets ID = bare name and stores the ARN in Fields["arn"].
		// Passing r.ID errors with "Invalid ARN prefix" against real AWS.
		smARN := r.Fields["arn"]
		if smARN == "" {
			return
		}
		// EXPRESS state machines reject ListExecutions outright
		// (StateMachineTypeNotSupported) — skip the call entirely rather than
		// reacting to the guaranteed error.
		if r.Fields["type"] == "EXPRESS" {
			return
		}
		mu.Lock()
		total++
		mu.Unlock()
		out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*sfn.ListExecutionsOutput, error) {
			return clients.SFN.ListExecutions(ctx, &sfn.ListExecutionsInput{
				StateMachineArn: aws.String(smARN),
				MaxResults:      1,
			})
		})
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			// Defense in depth: if an EXPRESS machine still reaches here
			// (e.g. Fields["type"] was unset/stale), AWS rejects the call with
			// StateMachineTypeNotSupported — that is a benign, expected skip,
			// not a real failure.
			if code, _, _ := ClassifyAWSError(err); code == "StateMachineTypeNotSupported" {
				return
			}
			failures = append(failures, fmt.Sprintf("%s: %v", r.ID, err))
			truncated = true
			result.TruncatedIDs[r.ID] = true
			return
		}
		if len(out.Executions) > 0 {
			s := out.Executions[0].Status
			exec := out.Executions[0]
			lastRunVal := "OK"
			if s == sfntypes.ExecutionStatusFailed || s == sfntypes.ExecutionStatusTimedOut || s == sfntypes.ExecutionStatusAborted {
				statusVal := string(s)
				statusPhrase := domain.HumanizeStatusPhrase(statusVal)
				if exec.StopDate != nil {
					elapsed := time.Since(*exec.StopDate)
					hours := int(elapsed.Hours())
					lastRunVal = fmt.Sprintf("%s %dh ago", statusVal, hours)
				} else {
					lastRunVal = statusVal
				}
				rows := []domain.DetailRow{
					{Label: "Latest Status", Value: statusPhrase, Tier: "!"},
				}
				if exec.StopDate != nil {
					rows = append(rows, domain.DetailRow{Label: "Ended", Value: exec.StopDate.Format("2006-01-02")})
				}
				if exec.Name != nil && *exec.Name != "" {
					rows = append(rows, domain.DetailRow{Label: "Execution Name", Value: *exec.Name})
				}
				setWave2Finding(&result, r.ID, sfnCodeLatestExecutionFailed, fmt.Sprintf("latest execution %s", statusPhrase), "!", "sfn", rows, "")
			}
			result.FieldUpdates[r.ID] = map[string]string{
				"last_run": lastRunVal,
			}
		}
	})
	sort.Strings(failures)
	result.IssueCount = len(result.Findings)
	result.Truncated = truncated
	return result,
		AggregateFailures("sfn-enrich: ListExecutions", failures, total)
}
