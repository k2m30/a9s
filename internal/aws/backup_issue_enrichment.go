// backup_issue_enrichment.go — Wave 2 issue enrichment for the backup resource type.
package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/backup"
	backuptypes "github.com/aws/aws-sdk-go-v2/service/backup/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	registerIssueEnricher("backup", EnrichBackupJobs, 100)
	resource.RegisterIssueEnricherFieldKeys("backup", []string{"status"})
}

// EnrichBackupJobs calls ListBackupJobs (account-wide, paginated) and returns a Finding
// for each BackupPlanId that has a failed/aborted/expired/partial job in the last 24h.
// Severity "!" for FAILED/ABORTED/EXPIRED, "~" for PARTIAL.
// IssueCount counts only "!" findings.
//
// Rule-7 suffix machinery (BumpFindingSuffix) is N/A for backup — spec §3.1 has zero
// Wave-1 signals so there are no coexisting Wave-1 warnings to apply the (+N) arithmetic.
func EnrichBackupJobs(ctx context.Context, clients *ServiceClients, _ []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	findings := make(map[string]resource.EnrichmentFinding)
	fieldUpdates := make(map[string]map[string]string)
	truncatedIDs := make(map[string]bool)
	if clients.Backup == nil {
		return IssueEnricherResult{Findings: findings, TruncatedIDs: truncatedIDs}, nil
	}

	var allJobs []backuptypes.BackupJob
	var nextToken *string
	truncated := false
	pages := 0
	var failures []string
	// Spec §3.2 — filter to the 24h window server-side so AWS returns only
	// the jobs we care about. Without this, accounts with months of job
	// history scan far more pages than needed and hit EnrichmentCap early,
	// setting truncated=true even when zero issues exist in the window.
	cutoff := time.Now().Add(-24 * time.Hour)
	for {
		if pages >= EnrichmentCap {
			truncated = true
			break
		}
		out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*backup.ListBackupJobsOutput, error) {
			return clients.Backup.ListBackupJobs(ctx, &backup.ListBackupJobsInput{
				ByCreatedAfter: &cutoff,
				NextToken:      nextToken,
			})
		})
		pages++
		if err != nil {
			truncated = true
			failures = append(failures, fmt.Sprintf("page %d: %v", pages, err))
			break
		}
		allJobs = append(allJobs, out.BackupJobs...)
		if out.NextToken == nil {
			break
		}
		nextToken = out.NextToken
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

	issueCount := 0
	for planID, b := range planBuckets {
		failedCount := len(b.failedJobs)
		partialCount := len(b.partialJobs)
		totalCount := b.totalCount

		if failedCount >= 1 {
			summary := fmt.Sprintf("%d job%s failed in last 24h", failedCount, plural(failedCount))

			// Cap displayed failed jobs at 5.
			cap := min(failedCount, 5)
			var rows []resource.FindingRow
			for _, job := range b.failedJobs[:cap] {
				rows = append(rows, resource.FindingRow{
					Label: "State",
					Value: string(job.State),
					Tier:  "!",
				})
			}
			// Most recent failed job creation date.
			var mostRecent *time.Time
			for _, j := range b.failedJobs {
				if mostRecent == nil || j.CreationDate.After(*mostRecent) {
					mostRecent = j.CreationDate
				}
			}
			if mostRecent != nil {
				rows = append(rows, resource.FindingRow{
					Label: "Most recent",
					Value: mostRecent.UTC().Format("2006-01-02 15:04 UTC"),
					Tier:  "!",
				})
			}
			// If there are also partial jobs, append a partial row so nothing silently disappears.
			if partialCount > 0 {
				rows = append(rows, resource.FindingRow{
					Label: "Partial jobs",
					Value: fmt.Sprintf("%d", partialCount),
					Tier:  "~",
				})
			}

			findings[planID] = resource.EnrichmentFinding{
				Severity: "!",
				Summary:  summary,
				Rows:     rows,
			}
			if _, ok := fieldUpdates[planID]; !ok {
				fieldUpdates[planID] = make(map[string]string)
			}
			fieldUpdates[planID]["status"] = summary
			issueCount++
		} else if partialCount >= 1 {
			summary := fmt.Sprintf("partial: %d of %d resources skipped", partialCount, totalCount)
			rows := []resource.FindingRow{
				{Label: "Partial jobs", Value: fmt.Sprintf("%d", partialCount), Tier: "~"},
				{Label: "Total jobs", Value: fmt.Sprintf("%d", totalCount), Tier: "~"},
			}
			findings[planID] = resource.EnrichmentFinding{
				Severity: "~",
				Summary:  summary,
				Rows:     rows,
			}
			if _, ok := fieldUpdates[planID]; !ok {
				fieldUpdates[planID] = make(map[string]string)
			}
			fieldUpdates[planID]["status"] = summary
			// "~" findings do not count toward issueCount.
		}
		// Else: only COMPLETED jobs — no finding, no FieldUpdate.
	}

	return IssueEnricherResult{
		IssueCount:   issueCount,
		Truncated:    truncated,
		TruncatedIDs: truncatedIDs,
		Findings:     findings,
		FieldUpdates: fieldUpdates,
	}, AggregateFailures("backup-enrich: ListBackupJobs", failures, pages)
}

// plural returns "s" when n != 1, "" otherwise.
func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
