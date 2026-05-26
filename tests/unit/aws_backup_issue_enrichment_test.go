// aws_backup_issue_enrichment_test.go — Wave 2 enricher tests for backup.
//
// All window-bounded tests use time.Now()-relative timestamps (not frozen Apr 2026
// fixture dates) so the window check is never accidentally satisfied by age of the
// test runner's clock.
//
// Covers (impl-plan §1 TEST: blocks):
//   - TEST: plan_with_one_failed_job_shows_broken_phrase
//   - TEST: plan_with_two_failed_jobs_counts_correctly
//   - TEST: plan_with_one_aborted_job_is_also_broken
//   - TEST: plan_with_partial_job_is_warning_with_tilde_glyph
//   - TEST: plan_mixed_failed_and_partial_picks_broken (U7d)
//   - TEST: plan_job_outside_window_is_ignored
//   - TEST: job_without_backupplanid_is_bucketed_nowhere
//   - TEST: banned_words_never_appear_in_status_or_detail
//   - TEST: out_of_scope_cadence_comparison_is_silent
//   - U11 invariant: Summary must not contain any Row.Value
//   - Adversarial: nil CreationDate, nil CreatedBy, ListBackupJobs API error
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
	"github.com/stretchr/testify/require"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/domain"
)

// ---------------------------------------------------------------------------
// inline fakes for enricher tests
// ---------------------------------------------------------------------------

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
	require.NoError(t, err)
	require.NotContains(t, result.Findings, planID,
		"expected no finding for plan %s (found one: %+v)", planID, result.Findings[planID])
}

// ---------------------------------------------------------------------------
// TEST: plan_with_one_failed_job_shows_broken_phrase
// ---------------------------------------------------------------------------

// TestBackup_Enricher_OneFailed_ShowsBrokenPhrase asserts the exact Summary,
// Severity, FieldUpdates key, and U11 (Summary ≠ Row content) for a plan with
// exactly one FAILED job in the 24h window.
func TestBackup_Enricher_OneFailed_ShowsBrokenPhrase(t *testing.T) {
	const planID = "plan-broken-1failed"
	fake := &backupJobsOnlyFake{
		jobs: []backuptypes.BackupJob{
			inWindowJob("job-1f-a", backuptypes.BackupJobStateFailed, planID),
		},
	}

	result, err := awsclient.EnrichBackupJobs(context.Background(), backupJobsFakeClients(fake), nil, nil)
	require.NoError(t, err)

	finding, ok := result.Findings[planID]
	require.True(t, ok, "expected finding for plan %s; got keys %v", planID, findingKeys(result.Findings))

	// Severity must be Broken.
	require.Equal(t, domain.SevBroken, finding.Severity, "Severity mismatch — FAILED must map to '!'")

	// Phrase is the exact spec §4 S4 phrase.
	require.Equal(t, "1 job failed in last 24h", finding.Phrase,
		"Phrase mismatch — must match spec §4 S4 list text exactly")

	// FieldUpdates must use key "status" (not "last_status").
	updates, hasUpdates := result.FieldUpdates[planID]
	require.True(t, hasUpdates, "FieldUpdates must contain an entry for plan %s", planID)
	require.Equal(t, "1 job failed in last 24h", updates["status"],
		"FieldUpdates[status] must equal the S4 phrase")
	require.NotContains(t, updates, "last_status",
		"FieldUpdates must not contain the banned 'last_status' key")

	// S1: IssueCount must be bumped (one "!" finding).
	require.GreaterOrEqual(t, result.IssueCount, 1,
		"IssueCount must be >= 1 when a '!' finding exists")

	// U11: Phrase must not contain any Row.Value (skip pure-integer counts — they appear in
	// both Phrase phrases and count rows by design).
	for _, row := range result.AttentionDetails[planID].Rows {
		if row.Value == "" {
			continue
		}
		if _, isNum := strconv.Atoi(row.Value); isNum == nil {
			continue // count values like "1", "3" appear in Phrase phrases — not a U11 violation
		}
		require.NotContains(t, finding.Phrase, row.Value,
			"U11 violation: Phrase %q must not contain Row value %q", finding.Phrase, row.Value)
	}

	// Rows must carry the state value for the failed job.
	require.NotEmpty(t, result.AttentionDetails[planID].Rows, "Rows must not be empty — must carry job state detail")
	stateFound := false
	for _, row := range result.AttentionDetails[planID].Rows {
		if row.Value == "FAILED" {
			stateFound = true
			break
		}
	}
	require.True(t, stateFound,
		"Rows must contain a row with Value='FAILED' (job state detail); Rows: %v", result.AttentionDetails[planID].Rows)
}

// ---------------------------------------------------------------------------
// TEST: plan_with_two_failed_jobs_counts_correctly
// ---------------------------------------------------------------------------

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
	require.NoError(t, err)

	finding, ok := result.Findings[planID]
	require.True(t, ok, "expected finding for plan %s; got keys %v", planID, findingKeys(result.Findings))

	require.Equal(t, domain.SevBroken, finding.Severity, "Severity mismatch — 2 failed jobs must map to '!'")
	require.Equal(t, "2 jobs failed in last 24h", finding.Phrase,
		"Phrase must be '2 jobs failed in last 24h' per spec §4 S4")

	updates, hasUpdates := result.FieldUpdates[planID]
	require.True(t, hasUpdates, "FieldUpdates must contain an entry for plan %s", planID)
	require.Equal(t, "2 jobs failed in last 24h", updates["status"],
		"FieldUpdates[status] must equal the S4 phrase")

	// U11: skip pure-integer count values — they naturally appear in count phrases.
	for _, row := range result.AttentionDetails[planID].Rows {
		if row.Value == "" {
			continue
		}
		if _, isNum := strconv.Atoi(row.Value); isNum == nil {
			continue
		}
		require.NotContains(t, finding.Phrase, row.Value,
			"U11: Phrase %q must not contain Row value %q", finding.Phrase, row.Value)
	}

	// Both FAILED and EXPIRED states must appear in Rows.
	rowVals := make(map[string]bool)
	for _, row := range result.AttentionDetails[planID].Rows {
		rowVals[row.Value] = true
	}
	require.True(t, rowVals["FAILED"], "Rows must carry FAILED state; rowValues: %v", rowVals)
	require.True(t, rowVals["EXPIRED"], "Rows must carry EXPIRED state; rowValues: %v", rowVals)
}

// ---------------------------------------------------------------------------
// TEST: plan_with_one_aborted_job_is_also_broken
// ---------------------------------------------------------------------------

// TestBackup_Enricher_OneAborted_IsAlsoBroken verifies that ABORTED maps to
// the same "failed" bucket as FAILED and EXPIRED per spec §3.2.
func TestBackup_Enricher_OneAborted_IsAlsoBroken(t *testing.T) {
	const planID = "plan-broken-aborted"
	fake := &backupJobsOnlyFake{
		jobs: []backuptypes.BackupJob{
			inWindowJob("job-ab-a", backuptypes.BackupJobStateAborted, planID),
		},
	}

	result, err := awsclient.EnrichBackupJobs(context.Background(), backupJobsFakeClients(fake), nil, nil)
	require.NoError(t, err)

	finding, ok := result.Findings[planID]
	require.True(t, ok, "expected finding for plan %s; ABORTED must map to '!' bucket", planID)

	require.Equal(t, domain.SevBroken, finding.Severity,
		"ABORTED must map to Severity '!' per spec §3.2")
	require.Equal(t, "1 job failed in last 24h", finding.Phrase,
		"ABORTED must use the same canonical phrase as FAILED per spec §4")

	// U11: skip pure-integer count values.
	for _, row := range result.AttentionDetails[planID].Rows {
		if row.Value == "" {
			continue
		}
		if _, isNum := strconv.Atoi(row.Value); isNum == nil {
			continue
		}
		require.NotContains(t, finding.Phrase, row.Value,
			"U11: Phrase %q must not contain Row value %q", finding.Phrase, row.Value)
	}
}

// ---------------------------------------------------------------------------
// TEST: plan_with_partial_job_is_warning_with_tilde_glyph
// ---------------------------------------------------------------------------

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
	require.NoError(t, err)

	finding, ok := result.Findings[planID]
	require.True(t, ok, "expected finding for plan %s; got keys %v", planID, findingKeys(result.Findings))

	require.Equal(t, domain.SevWarn, finding.Severity,
		"PARTIAL-only must produce Severity '~' (Warning, not Broken)")
	// Spec §4 S4: "partial: K of M resources skipped" where K=1 partial, M=3 total.
	require.Equal(t, "partial: 1 of 3 resources skipped", finding.Phrase,
		"Phrase must match spec §4 S4 phrase exactly")

	updates, hasUpdates := result.FieldUpdates[planID]
	require.True(t, hasUpdates, "FieldUpdates must have entry for plan %s", planID)
	require.Equal(t, "partial: 1 of 3 resources skipped", updates["status"],
		"FieldUpdates[status] must equal the S4 phrase")

	// S1: IssueCount must NOT bump (spec §4: "~ findings do not bump").
	// We check by counting only "!" findings in the result.
	bangCount := 0
	for _, f := range result.Findings {
		if f.Severity == domain.SevBroken {
			bangCount++
		}
	}
	require.Equal(t, 0, bangCount,
		"S1: ~ findings must not increment IssueCount; no '!' findings expected for PARTIAL-only")

	// U11: Phrase must not contain Row values (skip pure-integer counts).
	for _, row := range result.AttentionDetails[planID].Rows {
		if row.Value == "" {
			continue
		}
		if _, isNum := strconv.Atoi(row.Value); isNum == nil {
			continue
		}
		require.NotContains(t, finding.Phrase, row.Value,
			"U11: Phrase %q must not contain Row value %q", finding.Phrase, row.Value)
	}

	// Rows must carry "Partial jobs" and "Total jobs" (or functionally equivalent integer counts).
	rowVals := make(map[string]string, len(result.AttentionDetails[planID].Rows))
	for _, row := range result.AttentionDetails[planID].Rows {
		rowVals[row.Label] = row.Value
	}

	// Verify both counts appear somewhere in the rows — either as explicit
	// "Partial jobs"/"Total jobs" labels or as any row carrying the integer values.
	partialCountFound := false
	totalCountFound := false
	for _, row := range result.AttentionDetails[planID].Rows {
		if row.Value == "1" {
			partialCountFound = true
		}
		if row.Value == "3" {
			totalCountFound = true
		}
	}
	require.True(t, partialCountFound,
		"Rows must carry the partial job count (value '1'); Rows: %v", result.AttentionDetails[planID].Rows)
	require.True(t, totalCountFound,
		"Rows must carry the total job count (value '3'); Rows: %v", result.AttentionDetails[planID].Rows)
}

// ---------------------------------------------------------------------------
// TEST: plan_mixed_failed_and_partial_picks_broken (U7d)
// ---------------------------------------------------------------------------

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
	require.NoError(t, err)

	finding, ok := result.Findings[planID]
	require.True(t, ok, "expected finding for plan %s; got keys %v", planID, findingKeys(result.Findings))

	// U7d: Broken beats Warning.
	require.Equal(t, domain.SevBroken, finding.Severity,
		"U7d: Broken must beat Warning when both FAILED and PARTIAL exist")

	// One FAILED job drives the phrase.
	require.Equal(t, "1 job failed in last 24h", finding.Phrase,
		"U7d: Phrase uses the failed-bucket phrase when any '!' job exists")

	updates, hasUpdates := result.FieldUpdates[planID]
	require.True(t, hasUpdates, "FieldUpdates must have entry for plan %s", planID)
	require.Equal(t, "1 job failed in last 24h", updates["status"],
		"FieldUpdates[status] must use the '!' phrase")

	// S1: IssueCount bumps.
	bangCount := 0
	for _, f := range result.Findings {
		if f.Severity == domain.SevBroken {
			bangCount++
		}
	}
	require.GreaterOrEqual(t, bangCount, 1,
		"S1: at least one '!' finding must bump IssueCount")

	// Rows must include both the failed job State AND partial job count
	// so nothing silently disappears.
	rowVals := make(map[string]bool)
	for _, row := range result.AttentionDetails[planID].Rows {
		rowVals[row.Value] = true
	}
	require.True(t, rowVals["FAILED"],
		"Rows must contain State=FAILED; rows: %v", result.AttentionDetails[planID].Rows)

	// Partial evidence must be preserved alongside the FAILED evidence so the
	// enricher cannot silently drop partial context when a failed job exists.
	var sawPartial bool
	for _, row := range result.AttentionDetails[planID].Rows {
		if row.Label == "Partial jobs" || row.Tier == "~" {
			sawPartial = true
			break
		}
	}
	require.True(t, sawPartial, "mixed FAILED+PARTIAL finding must surface partial evidence in Rows")

	// U11: skip pure-integer count values.
	for _, row := range result.AttentionDetails[planID].Rows {
		if row.Value == "" {
			continue
		}
		if _, isNum := strconv.Atoi(row.Value); isNum == nil {
			continue
		}
		require.NotContains(t, finding.Phrase, row.Value,
			"U11: Phrase %q must not contain Row value %q", finding.Phrase, row.Value)
	}
}

// ---------------------------------------------------------------------------
// TEST: plan_job_outside_window_is_ignored
// ---------------------------------------------------------------------------

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
	require.NoError(t, err)

	// No FieldUpdates for a status phrase either.
	if updates, ok := result.FieldUpdates[planID]; ok {
		require.NotContains(t, updates, "status",
			"FieldUpdates must not set 'status' for an out-of-window job")
	}
}

// ---------------------------------------------------------------------------
// TEST: job_without_backupplanid_is_bucketed_nowhere
// ---------------------------------------------------------------------------

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
	require.NoError(t, err)

	require.Empty(t, result.Findings,
		"a job with nil BackupPlanId must not produce a finding against any plan; got: %v",
		findingKeys(result.Findings))
}

// ---------------------------------------------------------------------------
// TEST: banned_words_never_appear_in_status_or_detail
// ---------------------------------------------------------------------------

// TestBackup_Enricher_BannedWords_NeverAppear verifies that no wave-2 finding
// for backup contains banned internal implementation words in Summary or
// FieldUpdates["status"]. Spec §4 "Banned words" list.
func TestBackup_Enricher_BannedWords_NeverAppear(t *testing.T) {
	const planID = "plan-banned-words-test"
	fake := &backupJobsOnlyFake{
		jobs: []backuptypes.BackupJob{
			inWindowJob("job-bw-a", backuptypes.BackupJobStateFailed, planID),
		},
	}

	result, err := awsclient.EnrichBackupJobs(context.Background(), backupJobsFakeClients(fake), nil, nil)
	require.NoError(t, err)

	finding, ok := result.Findings[planID]
	require.Truef(t, ok, "expected finding for planID %q, got none", planID)

	bannedWords := []string{
		"Wave 1", "Wave 2", "Wave 3",
		"finding", "enrichment", "probe",
		"truncated", "lower bound", "bucket", "severity",
	}

	for _, word := range bannedWords {
		require.NotContains(t, finding.Phrase, word,
			"Phrase must not contain banned word %q; got Phrase=%q", word, finding.Phrase)
	}

	if updates, ok := result.FieldUpdates[planID]; ok {
		if statusPhrase, ok := updates["status"]; ok {
			for _, word := range bannedWords {
				require.NotContains(t, statusPhrase, word,
					"FieldUpdates[status] must not contain banned word %q; got %q", word, statusPhrase)
			}
		}
	}

	// Status field must not carry bare state keyword alone.
	bareKeywords := []string{"FAILED", "PARTIAL", "ABORTED", "EXPIRED"}
	if updates, ok := result.FieldUpdates[planID]; ok {
		if statusPhrase, ok := updates["status"]; ok {
			for _, kw := range bareKeywords {
				require.NotEqual(t, kw, statusPhrase,
					"FieldUpdates[status] must not be a bare state keyword; got %q", statusPhrase)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// TEST: out_of_scope_cadence_comparison_is_silent
// ---------------------------------------------------------------------------

// TestBackup_Enricher_CadenceComparison_IsSilent is an anti-test for spec §3.3.
// It verifies that a plan whose most-recent successful job ran 4 days ago
// (older than a hypothetical daily cadence × 2) emits NO finding.
// Wave 3 cadence comparison is explicitly out-of-scope.
func TestBackup_Enricher_CadenceComparison_IsSilent(t *testing.T) {
	const planID = "plan-stale-cadence"

	fake := &backupJobsOnlyFake{
		jobs: []backuptypes.BackupJob{
			// Only job: COMPLETED, but 4 days ago — outside the 24h window.
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
	require.NoError(t, err)

	require.NotContains(t, result.Findings, planID,
		"Wave 3 cadence comparison is out-of-scope: plan with stale last-run must emit no finding")

	// Verify row color would be green (no status update at all).
	if updates, ok := result.FieldUpdates[planID]; ok {
		require.NotContains(t, updates, "status",
			"out-of-scope cadence check must not write 'status' field update")
	}
}

// ---------------------------------------------------------------------------
// Adversarial: nil CreationDate — job must be skipped without panic
// ---------------------------------------------------------------------------

// TestBackup_Enricher_JobWithNilCreationDate_IsSkipped verifies that a job
// with CreationDate == nil is skipped (not panicked on, not producing a finding).
func TestBackup_Enricher_JobWithNilCreationDate_IsSkipped(t *testing.T) {
	const planID = "plan-nil-date"
	fake := &backupJobsOnlyFake{
		jobs: []backuptypes.BackupJob{
			{
				BackupJobId:  aws.String("job-nil-date"),
				State:        backuptypes.BackupJobStateFailed,
				CreationDate: nil, // nil date — must be skipped
				CreatedBy: &backuptypes.RecoveryPointCreator{
					BackupPlanId: aws.String(planID),
				},
			},
		},
	}

	// Must not panic.
	result, err := awsclient.EnrichBackupJobs(context.Background(), backupJobsFakeClients(fake), nil, nil)
	require.NoError(t, err)
	require.NotContains(t, result.Findings, planID,
		"job with nil CreationDate must not produce a finding (enricher skips nil-date jobs)")
}

// ---------------------------------------------------------------------------
// Adversarial: nil CreatedBy — job must be skipped without panic
// ---------------------------------------------------------------------------

// TestBackup_Enricher_JobWithNilCreatedBy_IsSkipped verifies that a job
// with CreatedBy == nil is skipped without panic or spurious findings.
func TestBackup_Enricher_JobWithNilCreatedBy_IsSkipped(t *testing.T) {
	fake := &backupJobsOnlyFake{
		jobs: []backuptypes.BackupJob{
			{
				BackupJobId:  aws.String("job-nil-by"),
				State:        backuptypes.BackupJobStateFailed,
				CreationDate: aws.Time(time.Now().Add(-1 * time.Hour)),
				CreatedBy:    nil, // nil CreatedBy — must be skipped
			},
		},
	}

	// Must not panic.
	result, err := awsclient.EnrichBackupJobs(context.Background(), backupJobsFakeClients(fake), nil, nil)
	require.NoError(t, err)
	require.Empty(t, result.Findings,
		"job with nil CreatedBy must not produce any findings; got: %v",
		findingKeys(result.Findings))
}

// ---------------------------------------------------------------------------
// Adversarial: ListBackupJobs API error — enricher surfaces the error
// ---------------------------------------------------------------------------

// TestBackup_Enricher_ListBackupJobsError_IsReturned verifies that when
// ListBackupJobs returns a sentinel error, the enricher returns that error
// and does not return partial findings.
func TestBackup_Enricher_ListBackupJobsError_IsReturned(t *testing.T) {
	sentinelErr := errors.New("simulated AWS Backup API error: ThrottlingException")
	fake := &backupJobsOnlyFake{
		listErr: sentinelErr,
	}

	result, err := awsclient.EnrichBackupJobs(context.Background(), backupJobsFakeClients(fake), nil, nil)
	require.Error(t, err,
		"enricher must surface the ListBackupJobs error, not swallow it")
	require.True(t, strings.Contains(err.Error(), "ThrottlingException") ||
		errors.Is(err, sentinelErr),
		"returned error must relate to the sentinel; got: %v", err)

	// Partial findings must not be present when the API failed completely.
	require.Empty(t, result.Findings,
		"enricher must not return partial findings when ListBackupJobs errors")
}

// ---------------------------------------------------------------------------
// Table-driven: FAILED / EXPIRED / ABORTED all map to "!" severity
// ---------------------------------------------------------------------------

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
		tc := tc
		t.Run(string(tc.state), func(t *testing.T) {
			fake := &backupJobsOnlyFake{
				jobs: []backuptypes.BackupJob{
					inWindowJob(fmt.Sprintf("job-%s-a", string(tc.state)), tc.state, tc.planID),
				},
			}

			result, err := awsclient.EnrichBackupJobs(context.Background(), backupJobsFakeClients(fake), nil, nil)
			require.NoError(t, err)

			finding, ok := result.Findings[tc.planID]
			require.True(t, ok,
				"state %s must produce a finding; got keys %v", tc.state, findingKeys(result.Findings))

			require.Equal(t, domain.SevBroken, finding.Severity,
				"state %s must map to Severity '!'", tc.state)
			require.Equal(t, tc.wantMsg, finding.Phrase,
				"state %s Phrase must be %q", tc.state, tc.wantMsg)

			// U11 for every case. Skip pure-integer row values — counts are
			// allowed to appear inside the Phrase (e.g. "2 jobs failed
			// in last 24h" legitimately contains "2"), and U11 is meant to
			// catch Phrase concatenated from descriptive Row values, not
			// numeric match-ups.
			for _, row := range result.AttentionDetails[tc.planID].Rows {
				if row.Value == "" {
					continue
				}
				if _, convErr := strconv.Atoi(row.Value); convErr == nil {
					continue
				}
				require.NotContains(t, finding.Phrase, row.Value,
					"U11: [%s] Phrase %q must not contain Row value %q",
					tc.state, finding.Phrase, row.Value)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// U11 invariant — comprehensive check across all triggering fixtures
// ---------------------------------------------------------------------------

// TestBackup_Enricher_U11_SummaryNeverContainsRowValues is a comprehensive
// U11 assertion: for all wave-2 findings from the fixture suite, Summary must
// not contain any Row.Value. Uses fresh time.Now()-relative jobs so the 24h
// window is always satisfied.
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
	require.NoError(t, err)

	for planID, finding := range result.Findings {
		for _, row := range result.AttentionDetails[planID].Rows {
			if row.Value == "" {
				continue
			}
			// Skip pure-integer count values — they naturally appear in count phrases
			// like "1 job failed in last 24h" and are not a U11 violation.
			if _, isNum := strconv.Atoi(row.Value); isNum == nil {
				continue
			}
			require.NotContains(t, finding.Phrase, row.Value,
				"U11 violation for plan %s: Phrase %q contains Row value %q — Phrase and Rows must be disjoint",
				planID, finding.Phrase, row.Value)
		}
	}
}
