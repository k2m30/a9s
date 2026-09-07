// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// backup_issue_enrichment.go — Wave 2 issue enrichment for the backup resource type.
package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/backup"
	backuptypes "github.com/aws/aws-sdk-go-v2/service/backup/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// backup canonical FindingCodes.
const (
	backupCodeJobFailed  domain.FindingCode = "backup.job-failed"
	backupCodeJobPartial domain.FindingCode = "backup.job-partial"
)

// EnrichBackupJobs calls ListBackupJobs (account-wide, paginated) and returns a Finding
// for each BackupPlanId that has a failed/aborted/expired/partial job in the last 24h.
// Severity "!" for FAILED/ABORTED/EXPIRED, "~" for PARTIAL.
//
// Rule-7 (+N) stacking is N/A for backup — spec §3.1 has zero Wave-1 signals so
// there are no coexisting Wave-1 warnings to stack with the Wave-2 finding.
func EnrichBackupJobs(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string][]domain.Finding),
		TruncatedIDs: make(map[string]bool),
		FieldUpdates: make(map[string]map[string]string),
	}
	if clients.Backup == nil {
		return result, nil
	}

	var failures []Failure
	// Spec §3.2 — filter to the 24h window server-side so AWS returns only
	// the jobs we care about. Without this, accounts with months of job
	// history scan far more pages than needed and hit EnrichmentCap early,
	// reporting a cut walk even when zero issues exist in the window.
	cutoff := time.Now().Add(-24 * time.Hour)
	allJobs, pages, cut, walkErr := walkAccountPages(&result, resources,
		func(job backuptypes.BackupJob) string {
			if job.CreatedBy == nil {
				return ""
			}
			return aws.ToString(job.CreatedBy.BackupPlanId)
		},
		func(token *string) ([]backuptypes.BackupJob, *string, error) {
			out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*backup.ListBackupJobsOutput, error) {
				return clients.Backup.ListBackupJobs(ctx, &backup.ListBackupJobsInput{
					ByCreatedAfter: &cutoff,
					NextToken:      token,
				})
			})
			if err != nil {
				return nil, nil, err
			}
			return out.BackupJobs, out.NextToken, nil
		})
	if walkErr != nil {
		failures = append(failures, FailedCall(fmt.Sprintf("page %d", pages), walkErr))
	}

	// Bucket jobs by plan ID. Each plan tracks all in-window jobs.
	type planBucket struct {
		failedJobs  []backuptypes.BackupJob
		partialJobs []backuptypes.BackupJob
		totalCount  int
	}
	planBuckets := make(map[string]*planBucket)

	for _, job := range allJobs {
		if job.CreationDate == nil {
			continue
		}
		if job.CreatedBy == nil || job.CreatedBy.BackupPlanId == nil {
			continue
		}
		planID := *job.CreatedBy.BackupPlanId
		if planID == "" {
			continue
		}
		if job.CreationDate.Before(cutoff) {
			continue
		}
		if _, ok := planBuckets[planID]; !ok {
			planBuckets[planID] = &planBucket{}
		}
		b := planBuckets[planID]
		b.totalCount++
		switch job.State {
		case backuptypes.BackupJobStateFailed,
			backuptypes.BackupJobStateExpired,
			backuptypes.BackupJobStateAborted:
			b.failedJobs = append(b.failedJobs, job)
		case backuptypes.BackupJobStatePartial:
			b.partialJobs = append(b.partialJobs, job)
		}
	}

	for planID, b := range planBuckets {
		failedCount := len(b.failedJobs)
		partialCount := len(b.partialJobs)
		totalCount := b.totalCount

		if failedCount >= 1 {
			summary := fmt.Sprintf("%d job%s failed in last 24h", failedCount, plural(failedCount))

			// The two facts about the whole bucket lead, and the per-job rows
			// follow, because only the per-job list is unbounded: capRows
			// keeps the head of what it is given, so a fact placed after the
			// list would be the first thing a plan with many failures loses.
			var mostRecent *time.Time
			for _, j := range b.failedJobs {
				if mostRecent == nil || j.CreationDate.After(*mostRecent) {
					mostRecent = j.CreationDate
				}
			}
			var rows []domain.DetailRow
			if mostRecent != nil {
				rows = append(rows, domain.DetailRow{
					Label: "Most recent",
					Value: mostRecent.UTC().Format("2006-01-02 15:04 UTC"),
					Tier:  "!",
				})
			}
			if partialCount > 0 {
				rows = append(rows, domain.DetailRow{
					Label: "Partial jobs",
					Value: fmt.Sprintf("%d", partialCount),
					Tier:  "~",
				})
			}
			for _, job := range b.failedJobs {
				rows = append(rows, domain.DetailRow{
					Label: "State",
					Value: domain.HumanizeStatusPhrase(string(job.State)),
					Tier:  "!",
				})
			}

			setWave2Finding(&result, planID, backupCodeJobFailed, summary, "!", "backup", rows)
			if result.FieldUpdates[planID] == nil {
				result.FieldUpdates[planID] = make(map[string]string)
			}
			result.FieldUpdates[planID]["status"] = summary
		} else if partialCount >= 1 {
			summary := fmt.Sprintf("partial: %d of %d resources skipped", partialCount, totalCount)
			rows := []domain.DetailRow{
				{Label: "Partial jobs", Value: fmt.Sprintf("%d", partialCount), Tier: "~"},
				{Label: "Total jobs", Value: fmt.Sprintf("%d", totalCount), Tier: "~"},
			}
			setWave2Finding(&result, planID, backupCodeJobPartial, summary, "~", "backup", rows)
			if result.FieldUpdates[planID] == nil {
				result.FieldUpdates[planID] = make(map[string]string)
			}
			result.FieldUpdates[planID]["status"] = summary
		}
		// Else: only COMPLETED jobs — no finding, no FieldUpdate.
	}

	SetTruncated(&result, cut)
	return result, AggregateFailures("backup-enrich: ListBackupJobs", failures, pages)
}

// plural returns "s" when n != 1, "" otherwise.
func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
