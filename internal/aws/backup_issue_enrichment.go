// backup_issue_enrichment.go — Wave 2 issue enrichment for the backup resource type.
package aws

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/backup"
	backuptypes "github.com/aws/aws-sdk-go-v2/service/backup/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	registerIssueEnricher("backup", EnrichBackupJobs, 100)
	resource.RegisterIssueEnricherFieldKeys("backup", []string{"last_status"})
}

// EnrichBackupJobs calls ListBackupJobs (account-wide, paginated) and returns a Finding
// for each BackupPlanId that has a failed/aborted/expired/partial job in the last 24h.
// Severity "!" for FAILED/ABORTED/EXPIRED, "~" for PARTIAL.
// IssueCount counts only "!" findings. First failure per plan wins.
// Pagination uses NextToken; walks up to EnrichmentCap pages.
func EnrichBackupJobs(ctx context.Context, clients *ServiceClients, _ []resource.Resource) (IssueEnricherResult, error) {
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
	for {
		if pages >= EnrichmentCap {
			truncated = true
			break
		}
		out, err := clients.Backup.ListBackupJobs(ctx, &backup.ListBackupJobsInput{
			NextToken: nextToken,
		})
		pages++
		if err != nil {
			return IssueEnricherResult{TruncatedIDs: truncatedIDs}, err
		}
		allJobs = append(allJobs, out.BackupJobs...)
		if out.NextToken == nil {
			break
		}
		nextToken = out.NextToken
	}
	cutoff := time.Now().Add(-24 * time.Hour)
	// Track newest job per plan regardless of age — last_status reflects the
	// most-recent execution even for weekly/monthly schedules. Findings (the
	// issue signal) still gate on the 24h cutoff.
	type jobRef struct {
		state    backuptypes.BackupJobState
		createAt time.Time
	}
	latestByPlan := make(map[string]jobRef)
	for _, job := range allJobs {
		if job.CreationDate == nil {
			continue
		}
		key := ""
		if job.CreatedBy != nil && job.CreatedBy.BackupPlanId != nil && *job.CreatedBy.BackupPlanId != "" {
			key = *job.CreatedBy.BackupPlanId
		} else if job.BackupJobId != nil {
			key = *job.BackupJobId
		}
		if key == "" {
			continue
		}
		if existing, ok := latestByPlan[key]; !ok || job.CreationDate.After(existing.createAt) {
			latestByPlan[key] = jobRef{state: job.State, createAt: *job.CreationDate}
		}
		if job.CreationDate.Before(cutoff) {
			continue
		}
		// First failure wins — skip if already recorded as a finding.
		if _, exists := findings[key]; exists {
			continue
		}
		switch job.State {
		case backuptypes.BackupJobStateFailed, backuptypes.BackupJobStateAborted, backuptypes.BackupJobStateExpired:
			stateStr := strings.ToLower(string(job.State))
			findings[key] = resource.EnrichmentFinding{
				Severity: "!",
				Summary:  fmt.Sprintf("backup %s in last 24h", stateStr),
				Rows: []resource.FindingRow{
					{Label: "State", Value: string(job.State), Tier: "!"},
				},
			}
		case backuptypes.BackupJobStatePartial:
			findings[key] = resource.EnrichmentFinding{
				Severity: "~",
				Summary:  "backup PARTIAL in last 24h",
				Rows: []resource.FindingRow{
					{Label: "State", Value: string(job.State), Tier: "~"},
				},
			}
		}
	}
	// Emit last_status once per plan based on the newest job seen, regardless
	// of whether that job triggered a finding.
	for key, ref := range latestByPlan {
		fieldUpdates[key] = map[string]string{"last_status": string(ref.state)}
	}
	issueCount := 0
	for _, f := range findings {
		if f.Severity == "!" {
			issueCount++
		}
	}
	return IssueEnricherResult{IssueCount: issueCount, Truncated: truncated, TruncatedIDs: truncatedIDs, Findings: findings, FieldUpdates: fieldUpdates}, nil
}
