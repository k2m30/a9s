package unit

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/backup"
	backuptypes "github.com/aws/aws-sdk-go-v2/service/backup/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
)

// backupJobsOnlyFake implements awsclient.BackupAPI.
// Only ListBackupJobs carries real logic — it filters by ByCreatedAfter exactly
// as the real AWS Backup API does. All other methods are no-op stubs required to
// satisfy the aggregate BackupAPI interface so the fake can be stored in
// ServiceClients.Backup.
type backupJobsOnlyFake struct {
	jobs    []backuptypes.BackupJob
	listErr error
}

func (f *backupJobsOnlyFake) ListBackupJobs(_ context.Context, input *backup.ListBackupJobsInput, _ ...func(*backup.Options)) (*backup.ListBackupJobsOutput, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	jobs := f.jobs
	if input != nil && input.ByCreatedAfter != nil {
		cutoff := *input.ByCreatedAfter
		filtered := make([]backuptypes.BackupJob, 0, len(jobs))
		for _, j := range jobs {
			if j.CreationDate != nil && !j.CreationDate.Before(cutoff) {
				filtered = append(filtered, j)
			}
		}
		jobs = filtered
	}
	return &backup.ListBackupJobsOutput{BackupJobs: jobs}, nil
}

// Stub methods — required by BackupAPI but unused by the enricher.
func (f *backupJobsOnlyFake) ListBackupPlans(_ context.Context, _ *backup.ListBackupPlansInput, _ ...func(*backup.Options)) (*backup.ListBackupPlansOutput, error) {
	return &backup.ListBackupPlansOutput{}, nil
}

func (f *backupJobsOnlyFake) GetBackupPlan(_ context.Context, _ *backup.GetBackupPlanInput, _ ...func(*backup.Options)) (*backup.GetBackupPlanOutput, error) {
	return &backup.GetBackupPlanOutput{}, nil
}

func (f *backupJobsOnlyFake) ListBackupSelections(_ context.Context, _ *backup.ListBackupSelectionsInput, _ ...func(*backup.Options)) (*backup.ListBackupSelectionsOutput, error) {
	return &backup.ListBackupSelectionsOutput{}, nil
}

func (f *backupJobsOnlyFake) DescribeBackupVault(_ context.Context, _ *backup.DescribeBackupVaultInput, _ ...func(*backup.Options)) (*backup.DescribeBackupVaultOutput, error) {
	return &backup.DescribeBackupVaultOutput{}, nil
}

func (f *backupJobsOnlyFake) GetBackupVaultNotifications(_ context.Context, _ *backup.GetBackupVaultNotificationsInput, _ ...func(*backup.Options)) (*backup.GetBackupVaultNotificationsOutput, error) {
	return &backup.GetBackupVaultNotificationsOutput{}, nil
}

func (f *backupJobsOnlyFake) ListRecoveryPointsByResource(_ context.Context, _ *backup.ListRecoveryPointsByResourceInput, _ ...func(*backup.Options)) (*backup.ListRecoveryPointsByResourceOutput, error) {
	return &backup.ListRecoveryPointsByResourceOutput{}, nil
}

// backupJobsFakeClients wraps backupJobsOnlyFake into ServiceClients.
// Other service clients are nil — the enricher only touches Backup.
func backupJobsFakeClients(fake *backupJobsOnlyFake) *awsclient.ServiceClients {
	return &awsclient.ServiceClients{Backup: fake}
}

// inWindowJob builds a BackupJob with CreationDate 2 hours ago (inside 24h window).
func inWindowJob(id string, state backuptypes.BackupJobState, planID string) backuptypes.BackupJob {
	return backuptypes.BackupJob{
		BackupJobId:  aws.String(id),
		State:        state,
		CreationDate: aws.Time(time.Now().Add(-2 * time.Hour)),
		CreatedBy: &backuptypes.RecoveryPointCreator{
			BackupPlanId: aws.String(planID),
		},
	}
}

// outOfWindowJob builds a BackupJob with CreationDate 48 hours ago (outside 24h window).
func outOfWindowJob(id string, state backuptypes.BackupJobState, planID string) backuptypes.BackupJob {
	return backuptypes.BackupJob{
		BackupJobId:  aws.String(id),
		State:        state,
		CreationDate: aws.Time(time.Now().Add(-48 * time.Hour)),
		CreatedBy: &backuptypes.RecoveryPointCreator{
			BackupPlanId: aws.String(planID),
		},
	}
}

// inWindowJobAt builds an in-window job with a specific CreationDate offset.
func inWindowJobAt(id string, state backuptypes.BackupJobState, planID string, offset time.Duration) backuptypes.BackupJob {
	return backuptypes.BackupJob{
		BackupJobId:  aws.String(id),
		State:        state,
		CreationDate: aws.Time(time.Now().Add(offset)),
		CreatedBy: &backuptypes.RecoveryPointCreator{
			BackupPlanId: aws.String(planID),
		},
	}
}

// assertNoFinding runs the enricher and asserts no finding for planID.
func assertNoFinding(t *testing.T, fake *backupJobsOnlyFake, planID string) {
	t.Helper()
	result, err := awsclient.EnrichBackupJobs(
		context.Background(),
		backupJobsFakeClients(fake),
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("EnrichBackupJobs returned error: %v", err)
	}
	if _, ok := result.Findings[planID]; ok {
		t.Fatalf("expected no finding for plan %s (found one: %+v)", planID, result.Findings[planID])
	}
}

// TestBackup_Enricher_OneFailed_ShowsBrokenPhrase asserts the exact Summary,
// Severity and FieldUpdates key, and that the Summary shares no Row content,
// for a plan with exactly one FAILED job in the 24h window.
func TestBackup_Enricher_OneFailed_ShowsBrokenPhrase(t *testing.T) {
	const planID = "plan-broken-1failed"
	fake := &backupJobsOnlyFake{
		jobs: []backuptypes.BackupJob{
			inWindowJob("job-1f-a", backuptypes.BackupJobStateFailed, planID),
		},
	}

	result, err := awsclient.EnrichBackupJobs(context.Background(), backupJobsFakeClients(fake), nil, nil)
	if err != nil {
		t.Fatalf("EnrichBackupJobs returned error: %v", err)
	}

	findings, ok := result.Findings[planID]
	if !ok {
		t.Fatalf("expected finding for plan %s; got keys %v", planID, findingKeys(result.Findings))
	}
	finding := findings[0]

	if finding.Severity != domain.SevBroken {
		t.Fatalf("Severity mismatch — FAILED must map to '!': got %v", finding.Severity)
	}

	if finding.Phrase != "1 job failed in last 24h" {
		t.Fatalf("Phrase mismatch — must match spec §4 S4 list text exactly: got %q", finding.Phrase)
	}

	updates, hasUpdates := result.FieldUpdates[planID]
	if !hasUpdates {
		t.Fatalf("FieldUpdates must contain an entry for plan %s", planID)
	}
	if updates["status"] != "1 job failed in last 24h" {
		t.Fatalf("FieldUpdates[status] must equal the S4 phrase: got %q", updates["status"])
	}
	if _, hasLastStatus := updates["last_status"]; hasLastStatus {
		t.Fatal("FieldUpdates must not contain the banned 'last_status' key")
	}

	// The Phrase must not contain any Row.Value. Pure-integer counts appear in
	// both by design, and the humanized job-state word "failed" is shared
	// vocabulary between the Row.Value and the "N job(s) failed..." Phrase.
	for _, row := range result.AttentionDetails[planID][finding.Code].Rows {
		if row.Value == "" || row.Value == "failed" {
			continue
		}
		if _, isNum := strconv.Atoi(row.Value); isNum == nil {
			continue
		}
		if strings.Contains(finding.Phrase, row.Value) {
			t.Fatalf("U11 violation: Phrase %q must not contain Row value %q", finding.Phrase, row.Value)
		}
	}

	if len(result.AttentionDetails[planID][finding.Code].Rows) == 0 {
		t.Fatal("Rows must not be empty — must carry job state detail")
	}
	stateFound := false
	for _, row := range result.AttentionDetails[planID][finding.Code].Rows {
		if row.Value == "failed" {
			stateFound = true
			break
		}
	}
	if !stateFound {
		t.Fatalf("Rows must contain a row with Value='failed' (humanized job state detail); Rows: %v", result.AttentionDetails[planID][finding.Code].Rows)
	}
}

// TestBackup_Enricher_TwoFailed_CountsCorrectly asserts that two failed jobs
// (one FAILED, one EXPIRED) produce Summary "2 jobs failed in last 24h" and
// both job states appear in Rows.
func TestBackup_Enricher_TwoFailed_CountsCorrectly(t *testing.T) {
	const planID = "plan-broken-2failed"
	fake := &backupJobsOnlyFake{
		jobs: []backuptypes.BackupJob{
			inWindowJobAt("job-2f-a", backuptypes.BackupJobStateFailed, planID, -5*time.Hour),
			inWindowJobAt("job-2f-b", backuptypes.BackupJobStateExpired, planID, -2*time.Hour),
		},
	}

	result, err := awsclient.EnrichBackupJobs(context.Background(), backupJobsFakeClients(fake), nil, nil)
	if err != nil {
		t.Fatalf("EnrichBackupJobs returned error: %v", err)
	}

	findings, ok := result.Findings[planID]
	if !ok {
		t.Fatalf("expected finding for plan %s; got keys %v", planID, findingKeys(result.Findings))
	}
	finding := findings[0]

	if finding.Severity != domain.SevBroken {
		t.Fatalf("Severity mismatch — 2 failed jobs must map to '!': got %v", finding.Severity)
	}
	if finding.Phrase != "2 jobs failed in last 24h" {
		t.Fatalf("Phrase must be '2 jobs failed in last 24h' per spec §4 S4: got %q", finding.Phrase)
	}

	updates, hasUpdates := result.FieldUpdates[planID]
	if !hasUpdates {
		t.Fatalf("FieldUpdates must contain an entry for plan %s", planID)
	}
	if updates["status"] != "2 jobs failed in last 24h" {
		t.Fatalf("FieldUpdates[status] must equal the S4 phrase: got %q", updates["status"])
	}

	// Pure-integer counts and the humanized "failed" word are shared with the Phrase.
	for _, row := range result.AttentionDetails[planID][finding.Code].Rows {
		if row.Value == "" || row.Value == "failed" {
			continue
		}
		if _, isNum := strconv.Atoi(row.Value); isNum == nil {
			continue
		}
		if strings.Contains(finding.Phrase, row.Value) {
			t.Fatalf("U11: Phrase %q must not contain Row value %q", finding.Phrase, row.Value)
		}
	}

	rowVals := make(map[string]bool)
	for _, row := range result.AttentionDetails[planID][finding.Code].Rows {
		rowVals[row.Value] = true
	}
	if !rowVals["failed"] {
		t.Fatalf("Rows must carry humanized failed state; rowValues: %v", rowVals)
	}
	if !rowVals["expired"] {
		t.Fatalf("Rows must carry humanized expired state; rowValues: %v", rowVals)
	}
}

// TestBackup_Enricher_OneAborted_IsAlsoBroken verifies that ABORTED maps to
// the same "failed" bucket as FAILED and EXPIRED.
func TestBackup_Enricher_OneAborted_IsAlsoBroken(t *testing.T) {
	const planID = "plan-broken-aborted"
	fake := &backupJobsOnlyFake{
		jobs: []backuptypes.BackupJob{
			inWindowJob("job-ab-a", backuptypes.BackupJobStateAborted, planID),
		},
	}

	result, err := awsclient.EnrichBackupJobs(context.Background(), backupJobsFakeClients(fake), nil, nil)
	if err != nil {
		t.Fatalf("EnrichBackupJobs returned error: %v", err)
	}

	findings, ok := result.Findings[planID]
	if !ok {
		t.Fatalf("expected finding for plan %s; ABORTED must map to '!' bucket", planID)
	}
	finding := findings[0]

	if finding.Severity != domain.SevBroken {
		t.Fatalf("ABORTED must map to Severity '!' per spec §3.2: got %v", finding.Severity)
	}
	if finding.Phrase != "1 job failed in last 24h" {
		t.Fatalf("ABORTED must use the same canonical phrase as FAILED per spec §4: got %q", finding.Phrase)
	}

	// Pure-integer counts are shared with the Phrase.
	for _, row := range result.AttentionDetails[planID][finding.Code].Rows {
		if row.Value == "" {
			continue
		}
		if _, isNum := strconv.Atoi(row.Value); isNum == nil {
			continue
		}
		if strings.Contains(finding.Phrase, row.Value) {
			t.Fatalf("U11: Phrase %q must not contain Row value %q", finding.Phrase, row.Value)
		}
	}
}

// TestBackup_Enricher_PartialOnly_IsWarning verifies that 1 PARTIAL job among
// 3 total jobs produces Severity "~", the exact partial phrase, and correct Rows.
func TestBackup_Enricher_PartialOnly_IsWarning(t *testing.T) {
	const planID = "plan-warning-partial"
	fake := &backupJobsOnlyFake{
		jobs: []backuptypes.BackupJob{
			inWindowJobAt("job-p-a", backuptypes.BackupJobStateCompleted, planID, -3*time.Hour),
			inWindowJobAt("job-p-b", backuptypes.BackupJobStateCompleted, planID, -2*time.Hour),
			inWindowJobAt("job-p-c", backuptypes.BackupJobStatePartial, planID, -1*time.Hour),
		},
	}

	result, err := awsclient.EnrichBackupJobs(context.Background(), backupJobsFakeClients(fake), nil, nil)
	if err != nil {
		t.Fatalf("EnrichBackupJobs returned error: %v", err)
	}

	findings, ok := result.Findings[planID]
	if !ok {
		t.Fatalf("expected finding for plan %s; got keys %v", planID, findingKeys(result.Findings))
	}
	finding := findings[0]

	if finding.Severity != domain.SevWarn {
		t.Fatalf("PARTIAL-only must produce Severity '~' (Warning, not Broken): got %v", finding.Severity)
	}
	if finding.Phrase != "partial: 1 of 3 resources skipped" {
		t.Fatalf("Phrase must match spec §4 S4 phrase exactly: got %q", finding.Phrase)
	}

	updates, hasUpdates := result.FieldUpdates[planID]
	if !hasUpdates {
		t.Fatalf("FieldUpdates must have entry for plan %s", planID)
	}
	if updates["status"] != "partial: 1 of 3 resources skipped" {
		t.Fatalf("FieldUpdates[status] must equal the S4 phrase: got %q", updates["status"])
	}

	// "~" findings do not bump IssueCount.
	bangCount := 0
	for _, fs := range result.Findings {
		for _, f := range fs {
			if f.Severity == domain.SevBroken {
				bangCount++
			}
		}
	}
	if bangCount != 0 {
		t.Fatalf("S1: ~ findings must not increment IssueCount; no '!' findings expected for PARTIAL-only: got %d", bangCount)
	}

	// The Phrase must not contain Row values; pure-integer counts are shared.
	for _, row := range result.AttentionDetails[planID][finding.Code].Rows {
		if row.Value == "" {
			continue
		}
		if _, isNum := strconv.Atoi(row.Value); isNum == nil {
			continue
		}
		if strings.Contains(finding.Phrase, row.Value) {
			t.Fatalf("U11: Phrase %q must not contain Row value %q", finding.Phrase, row.Value)
		}
	}

	rowVals := make(map[string]string, len(result.AttentionDetails[planID][finding.Code].Rows))
	for _, row := range result.AttentionDetails[planID][finding.Code].Rows {
		rowVals[row.Label] = row.Value
	}

	partialCountFound := false
	totalCountFound := false
	for _, row := range result.AttentionDetails[planID][finding.Code].Rows {
		if row.Value == "1" {
			partialCountFound = true
		}
		if row.Value == "3" {
			totalCountFound = true
		}
	}
	if !partialCountFound {
		t.Fatalf("Rows must carry the partial job count (value '1'); Rows: %v", result.AttentionDetails[planID][finding.Code].Rows)
	}
	if !totalCountFound {
		t.Fatalf("Rows must carry the total job count (value '3'); Rows: %v", result.AttentionDetails[planID][finding.Code].Rows)
	}
}

// TestBackup_Enricher_MixedFailedAndPartial_BrokenWins asserts that when both
// FAILED and PARTIAL jobs exist for a plan in the window, "!" (Broken) wins
// over "~" (Warning), and Rows still include both the failed job state and
// the partial count so no signal silently disappears.
func TestBackup_Enricher_MixedFailedAndPartial_BrokenWins(t *testing.T) {
	const planID = "plan-broken-mixed"
	fake := &backupJobsOnlyFake{
		jobs: []backuptypes.BackupJob{
			inWindowJobAt("job-m-a", backuptypes.BackupJobStateFailed, planID, -6*time.Hour),
			inWindowJobAt("job-m-b", backuptypes.BackupJobStatePartial, planID, -3*time.Hour),
			inWindowJobAt("job-m-c", backuptypes.BackupJobStateCompleted, planID, -1*time.Hour),
		},
	}

	result, err := awsclient.EnrichBackupJobs(context.Background(), backupJobsFakeClients(fake), nil, nil)
	if err != nil {
		t.Fatalf("EnrichBackupJobs returned error: %v", err)
	}

	findings, ok := result.Findings[planID]
	if !ok {
		t.Fatalf("expected finding for plan %s; got keys %v", planID, findingKeys(result.Findings))
	}
	finding := findings[0]

	if finding.Severity != domain.SevBroken {
		t.Fatalf("U7d: Broken must beat Warning when both FAILED and PARTIAL exist: got %v", finding.Severity)
	}

	if finding.Phrase != "1 job failed in last 24h" {
		t.Fatalf("U7d: Phrase uses the failed-bucket phrase when any '!' job exists: got %q", finding.Phrase)
	}

	updates, hasUpdates := result.FieldUpdates[planID]
	if !hasUpdates {
		t.Fatalf("FieldUpdates must have entry for plan %s", planID)
	}
	if updates["status"] != "1 job failed in last 24h" {
		t.Fatalf("FieldUpdates[status] must use the '!' phrase: got %q", updates["status"])
	}

	bangCount := 0
	for _, fs := range result.Findings {
		for _, f := range fs {
			if f.Severity == domain.SevBroken {
				bangCount++
			}
		}
	}
	if bangCount < 1 {
		t.Fatalf("S1: at least one '!' finding must bump IssueCount: got %d", bangCount)
	}

	rowVals := make(map[string]bool)
	for _, row := range result.AttentionDetails[planID][finding.Code].Rows {
		rowVals[row.Value] = true
	}
	if !rowVals["failed"] {
		t.Fatalf("Rows must contain humanized State=failed; rows: %v", result.AttentionDetails[planID][finding.Code].Rows)
	}

	var sawPartial bool
	for _, row := range result.AttentionDetails[planID][finding.Code].Rows {
		if row.Label == "Partial jobs" || row.Tier == "~" {
			sawPartial = true
			break
		}
	}
	if !sawPartial {
		t.Fatal("mixed FAILED+PARTIAL finding must surface partial evidence in Rows")
	}

	// Pure-integer counts and the humanized "failed" word are shared with the Phrase.
	for _, row := range result.AttentionDetails[planID][finding.Code].Rows {
		if row.Value == "" || row.Value == "failed" {
			continue
		}
		if _, isNum := strconv.Atoi(row.Value); isNum == nil {
			continue
		}
		if strings.Contains(finding.Phrase, row.Value) {
			t.Fatalf("U11: Phrase %q must not contain Row value %q", finding.Phrase, row.Value)
		}
	}
}

// TestBackup_Enricher_JobOutsideWindow_IsIgnored verifies that a FAILED job
// created 48h ago (outside the 24h window) produces no finding.
func TestBackup_Enricher_JobOutsideWindow_IsIgnored(t *testing.T) {
	const planID = "plan-old-failure"
	fake := &backupJobsOnlyFake{
		jobs: []backuptypes.BackupJob{
			outOfWindowJob("job-old-a", backuptypes.BackupJobStateFailed, planID),
		},
	}
	assertNoFinding(t, fake, planID)

	result, err := awsclient.EnrichBackupJobs(context.Background(), backupJobsFakeClients(fake), nil, nil)
	if err != nil {
		t.Fatalf("EnrichBackupJobs returned error: %v", err)
	}

	if updates, ok := result.FieldUpdates[planID]; ok {
		if _, hasStatus := updates["status"]; hasStatus {
			t.Fatal("FieldUpdates must not set 'status' for an out-of-window job")
		}
	}
}

// TestBackup_Enricher_NilBackupPlanID_NotBucketed verifies that a job with
// CreatedBy.BackupPlanId == nil does not produce a finding against any plan.
// The enricher must not use the BackupJobId as a fallback plan key.
func TestBackup_Enricher_NilBackupPlanID_NotBucketed(t *testing.T) {
	fake := &backupJobsOnlyFake{
		jobs: []backuptypes.BackupJob{
			{
				BackupJobId:  aws.String("on-demand-job-001"),
				State:        backuptypes.BackupJobStateFailed,
				CreationDate: aws.Time(time.Now().Add(-1 * time.Hour)),
				CreatedBy: &backuptypes.RecoveryPointCreator{
					BackupPlanId: nil, // on-demand job, no plan association
				},
			},
		},
	}

	result, err := awsclient.EnrichBackupJobs(context.Background(), backupJobsFakeClients(fake), nil, nil)
	if err != nil {
		t.Fatalf("EnrichBackupJobs returned error: %v", err)
	}

	if len(result.Findings) != 0 {
		t.Fatalf("a job with nil BackupPlanId must not produce a finding against any plan; got: %v",
			findingKeys(result.Findings))
	}
}

// TestBackup_Enricher_BannedWords_NeverAppear verifies that no wave-2 finding
// for backup contains banned internal implementation words in Summary or
// FieldUpdates["status"].
func TestBackup_Enricher_BannedWords_NeverAppear(t *testing.T) {
	const planID = "plan-banned-words-test"
	fake := &backupJobsOnlyFake{
		jobs: []backuptypes.BackupJob{
			inWindowJob("job-bw-a", backuptypes.BackupJobStateFailed, planID),
		},
	}

	result, err := awsclient.EnrichBackupJobs(context.Background(), backupJobsFakeClients(fake), nil, nil)
	if err != nil {
		t.Fatalf("EnrichBackupJobs returned error: %v", err)
	}

	findings, ok := result.Findings[planID]
	if !ok {
		t.Fatalf("expected finding for planID %q, got none", planID)
	}
	finding := findings[0]

	bannedWords := []string{
		"Wave 1", "Wave 2", "Wave 3",
		"finding", "enrichment", "probe",
		"truncated", "lower bound", "bucket", "severity",
	}

	for _, word := range bannedWords {
		if strings.Contains(finding.Phrase, word) {
			t.Fatalf("Phrase must not contain banned word %q; got Phrase=%q", word, finding.Phrase)
		}
	}

	if updates, ok := result.FieldUpdates[planID]; ok {
		if statusPhrase, ok := updates["status"]; ok {
			for _, word := range bannedWords {
				if strings.Contains(statusPhrase, word) {
					t.Fatalf("FieldUpdates[status] must not contain banned word %q; got %q", word, statusPhrase)
				}
			}
		}
	}

	bareKeywords := []string{"FAILED", "PARTIAL", "ABORTED", "EXPIRED"}
	if updates, ok := result.FieldUpdates[planID]; ok {
		if statusPhrase, ok := updates["status"]; ok {
			for _, kw := range bareKeywords {
				if statusPhrase == kw {
					t.Fatalf("FieldUpdates[status] must not be a bare state keyword; got %q", statusPhrase)
				}
			}
		}
	}
}

// TestBackup_Enricher_CadenceComparison_IsSilent: a plan whose most-recent
// successful job ran 4 days ago emits no finding; the enricher judges only
// jobs inside the 24h window, not a plan's cadence.
func TestBackup_Enricher_CadenceComparison_IsSilent(t *testing.T) {
	const planID = "plan-stale-cadence"

	fake := &backupJobsOnlyFake{
		jobs: []backuptypes.BackupJob{
			{
				BackupJobId:  aws.String("job-stale-a"),
				State:        backuptypes.BackupJobStateCompleted,
				CreationDate: aws.Time(time.Now().Add(-96 * time.Hour)),
				CreatedBy: &backuptypes.RecoveryPointCreator{
					BackupPlanId: aws.String(planID),
				},
			},
		},
	}

	result, err := awsclient.EnrichBackupJobs(context.Background(), backupJobsFakeClients(fake), nil, nil)
	if err != nil {
		t.Fatalf("EnrichBackupJobs returned error: %v", err)
	}

	if _, ok := result.Findings[planID]; ok {
		t.Fatal("Wave 3 cadence comparison is out-of-scope: plan with stale last-run must emit no finding")
	}

	if updates, ok := result.FieldUpdates[planID]; ok {
		if _, hasStatus := updates["status"]; hasStatus {
			t.Fatal("out-of-scope cadence check must not write 'status' field update")
		}
	}
}

// TestBackup_Enricher_JobWithNilCreationDate_IsSkipped verifies that a job
// with CreationDate == nil is skipped (not panicked on, not producing a finding).
func TestBackup_Enricher_JobWithNilCreationDate_IsSkipped(t *testing.T) {
	const planID = "plan-nil-date"
	fake := &backupJobsOnlyFake{
		jobs: []backuptypes.BackupJob{
			{
				BackupJobId:  aws.String("job-nil-date"),
				State:        backuptypes.BackupJobStateFailed,
				CreationDate: nil,
				CreatedBy: &backuptypes.RecoveryPointCreator{
					BackupPlanId: aws.String(planID),
				},
			},
		},
	}

	result, err := awsclient.EnrichBackupJobs(context.Background(), backupJobsFakeClients(fake), nil, nil)
	if err != nil {
		t.Fatalf("EnrichBackupJobs returned error: %v", err)
	}
	if _, ok := result.Findings[planID]; ok {
		t.Fatal("job with nil CreationDate must not produce a finding (enricher skips nil-date jobs)")
	}
}

// TestBackup_Enricher_JobWithNilCreatedBy_IsSkipped verifies that a job
// with CreatedBy == nil is skipped without panic or spurious findings.
func TestBackup_Enricher_JobWithNilCreatedBy_IsSkipped(t *testing.T) {
	fake := &backupJobsOnlyFake{
		jobs: []backuptypes.BackupJob{
			{
				BackupJobId:  aws.String("job-nil-by"),
				State:        backuptypes.BackupJobStateFailed,
				CreationDate: aws.Time(time.Now().Add(-1 * time.Hour)),
				CreatedBy:    nil,
			},
		},
	}

	result, err := awsclient.EnrichBackupJobs(context.Background(), backupJobsFakeClients(fake), nil, nil)
	if err != nil {
		t.Fatalf("EnrichBackupJobs returned error: %v", err)
	}
	if len(result.Findings) != 0 {
		t.Fatalf("job with nil CreatedBy must not produce any findings; got: %v",
			findingKeys(result.Findings))
	}
}

// TestBackup_Enricher_ListBackupJobsError_IsReturned verifies that when
// ListBackupJobs returns a sentinel error, the enricher returns that error
// and does not return partial findings.
func TestBackup_Enricher_ListBackupJobsError_IsReturned(t *testing.T) {
	sentinelErr := errors.New("simulated AWS Backup API error: ThrottlingException")
	fake := &backupJobsOnlyFake{
		listErr: sentinelErr,
	}

	result, err := awsclient.EnrichBackupJobs(context.Background(), backupJobsFakeClients(fake), nil, nil)
	if err == nil {
		t.Fatal("enricher must surface the ListBackupJobs error, not swallow it")
	}
	if !strings.Contains(err.Error(), "ThrottlingException") && !errors.Is(err, sentinelErr) {
		t.Fatalf("returned error must relate to the sentinel; got: %v", err)
	}

	if len(result.Findings) != 0 {
		t.Fatal("enricher must not return partial findings when ListBackupJobs errors")
	}
}

// TestBackup_Enricher_FailedBucket_AllStatesMapToBang is a table-driven test
// verifying that FAILED, EXPIRED, and ABORTED all produce Severity "!" with
// the singular "1 job failed in last 24h" phrase. Eliminates the risk that
// only FAILED is handled while EXPIRED or ABORTED are silently ignored.
func TestBackup_Enricher_FailedBucket_AllStatesMapToBang(t *testing.T) {
	cases := []struct {
		state   backuptypes.BackupJobState
		planID  string
		wantMsg string
	}{
		{
			state:   backuptypes.BackupJobStateFailed,
			planID:  "plan-table-failed",
			wantMsg: "1 job failed in last 24h",
		},
		{
			state:   backuptypes.BackupJobStateExpired,
			planID:  "plan-table-expired",
			wantMsg: "1 job failed in last 24h",
		},
		{
			state:   backuptypes.BackupJobStateAborted,
			planID:  "plan-table-aborted",
			wantMsg: "1 job failed in last 24h",
		},
	}

	for _, tc := range cases {
		t.Run(string(tc.state), func(t *testing.T) {
			fake := &backupJobsOnlyFake{
				jobs: []backuptypes.BackupJob{
					inWindowJob(fmt.Sprintf("job-%s-a", string(tc.state)), tc.state, tc.planID),
				},
			}

			result, err := awsclient.EnrichBackupJobs(context.Background(), backupJobsFakeClients(fake), nil, nil)
			if err != nil {
				t.Fatalf("EnrichBackupJobs returned error: %v", err)
			}

			findings, ok := result.Findings[tc.planID]
			if !ok {
				t.Fatalf("state %s must produce a finding; got keys %v", tc.state, findingKeys(result.Findings))
			}
			finding := findings[0]

			if finding.Severity != domain.SevBroken {
				t.Fatalf("state %s must map to Severity '!': got %v", tc.state, finding.Severity)
			}
			if finding.Phrase != tc.wantMsg {
				t.Fatalf("state %s Phrase must be %q: got %q", tc.state, tc.wantMsg, finding.Phrase)
			}

			// Pure-integer counts may appear inside the Phrase ("2 jobs failed in
			// last 24h"), and FAILED's Row.Value "failed" overlaps the Phrase's
			// "N job(s) failed..." text; the check targets a Phrase built from
			// descriptive Row values.
			for _, row := range result.AttentionDetails[tc.planID][finding.Code].Rows {
				if row.Value == "" || row.Value == "failed" {
					continue
				}
				if _, convErr := strconv.Atoi(row.Value); convErr == nil {
					continue
				}
				if strings.Contains(finding.Phrase, row.Value) {
					t.Fatalf("U11: [%s] Phrase %q must not contain Row value %q",
						tc.state, finding.Phrase, row.Value)
				}
			}
		})
	}
}

// TestBackup_Enricher_U11_SummaryNeverContainsRowValues: for every wave-2
// finding from the fixture suite, Summary contains no Row.Value.
func TestBackup_Enricher_U11_SummaryNeverContainsRowValues(t *testing.T) {
	plans := []struct {
		planID string
		jobs   []backuptypes.BackupJob
	}{
		{
			planID: "plan-u11-failed",
			jobs: []backuptypes.BackupJob{
				inWindowJob("job-u11-f", backuptypes.BackupJobStateFailed, "plan-u11-failed"),
			},
		},
		{
			planID: "plan-u11-expired",
			jobs: []backuptypes.BackupJob{
				inWindowJob("job-u11-e", backuptypes.BackupJobStateExpired, "plan-u11-expired"),
			},
		},
		{
			planID: "plan-u11-aborted",
			jobs: []backuptypes.BackupJob{
				inWindowJob("job-u11-ab", backuptypes.BackupJobStateAborted, "plan-u11-aborted"),
			},
		},
		{
			planID: "plan-u11-partial",
			jobs: []backuptypes.BackupJob{
				inWindowJobAt("job-u11-pa", backuptypes.BackupJobStateCompleted, "plan-u11-partial", -3*time.Hour),
				inWindowJobAt("job-u11-pb", backuptypes.BackupJobStatePartial, "plan-u11-partial", -1*time.Hour),
			},
		},
	}

	var allJobs []backuptypes.BackupJob
	for _, p := range plans {
		allJobs = append(allJobs, p.jobs...)
	}

	fake := &backupJobsOnlyFake{jobs: allJobs}
	result, err := awsclient.EnrichBackupJobs(context.Background(), backupJobsFakeClients(fake), nil, nil)
	if err != nil {
		t.Fatalf("EnrichBackupJobs returned error: %v", err)
	}

	for planID, findings := range result.Findings {
		for _, finding := range findings {
			for _, row := range result.AttentionDetails[planID][finding.Code].Rows {
				if row.Value == "" || row.Value == "failed" {
					continue
				}
				// Pure-integer counts appear in count phrases like "1 job failed in last 24h".
				if _, isNum := strconv.Atoi(row.Value); isNum == nil {
					continue
				}
				if strings.Contains(finding.Phrase, row.Value) {
					t.Fatalf("U11 violation for plan %s: Phrase %q contains Row value %q — Phrase and Rows must be disjoint",
						planID, finding.Phrase, row.Value)
				}
			}
		}
	}
}

// TestBackup_Enricher_PartialOfOne_ReadsAsOneResource: the partial phrase and
// the status cell come from the code's own declaration, and the declaration
// agrees its noun with the total it names — one job in the window is "of 1
// resource skipped", not "of 1 resources skipped".
func TestBackup_Enricher_PartialOfOne_ReadsAsOneResource(t *testing.T) {
	const planID = "plan-partial-of-one"
	fake := &backupJobsOnlyFake{
		jobs: []backuptypes.BackupJob{
			inWindowJobAt("job-solo", backuptypes.BackupJobStatePartial, planID, -1*time.Hour),
		},
	}

	result, err := awsclient.EnrichBackupJobs(context.Background(), backupJobsFakeClients(fake), nil, nil)
	if err != nil {
		t.Fatalf("EnrichBackupJobs returned error: %v", err)
	}
	findings, ok := result.Findings[planID]
	if !ok || len(findings) == 0 {
		t.Fatalf("expected a finding for plan %s", planID)
	}
	const want = "partial: 1 of 1 resource skipped"
	if findings[0].Phrase != want {
		t.Errorf("Phrase = %q, want %q", findings[0].Phrase, want)
	}
	if got := result.FieldUpdates[planID]["status"]; got != want {
		t.Errorf("FieldUpdates[status] = %q, want %q — the cell reads the same declaration the phrase does", got, want)
	}
}
