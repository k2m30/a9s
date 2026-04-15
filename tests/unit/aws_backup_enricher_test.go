package unit

// aws_backup_enricher_test.go — Behavioral tests for EnrichBackupJobs.
//
// Contract assertions:
//   - ListBackupJobs is called once (account-wide).
//   - State=FAILED|ABORTED|EXPIRED, CreationDate within last 24h → Finding keyed by BackupPlanId, severity "!".
//   - State=PARTIAL, CreationDate within last 24h → Finding keyed by BackupPlanId, severity "~".
//   - State=COMPLETED → no finding regardless of age.
//   - Job older than 24h → no finding regardless of state.
//   - clients.Backup == nil → (EnricherResult{Findings: non-nil empty}, nil).
//   - API error → (EnricherResult{}, error propagated).

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/backup"
	backuptypes "github.com/aws/aws-sdk-go-v2/service/backup/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
)

// backupJobFake implements BackupAPI for enrichment testing.
// It embeds the interface and overrides only ListBackupJobs.
type backupJobFake struct {
	awsclient.BackupAPI
	jobs []backuptypes.BackupJob
	err  error
}

func (f *backupJobFake) ListBackupJobs(
	_ context.Context,
	_ *backup.ListBackupJobsInput,
	_ ...func(*backup.Options),
) (*backup.ListBackupJobsOutput, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &backup.ListBackupJobsOutput{BackupJobs: f.jobs}, nil
}

// now is a helper that returns a *time.Time set to the current moment.
func nowPtr() *time.Time {
	t := time.Now()
	return &t
}

// hoursAgoPtr returns a *time.Time N hours in the past.
func hoursAgoPtr(n int) *time.Time {
	t := time.Now().Add(-time.Duration(n) * time.Hour)
	return &t
}

// TestEnrichBackupJobs_FailedRecentJobProducesFindingSevBang verifies that a
// FAILED job created within the last 24h produces a finding with severity "!".
func TestEnrichBackupJobs_FailedRecentJobProducesFindingSevBang(t *testing.T) {
	fake := &backupJobFake{
		jobs: []backuptypes.BackupJob{
			{
				State:        backuptypes.BackupJobStateFailed,
				CreationDate: nowPtr(),
				CreatedBy:    &backuptypes.RecoveryPointCreator{BackupPlanId: aws.String("plan-1")},
			},
		},
	}
	clients := &awsclient.ServiceClients{Backup: fake}

	result, err := awsclient.EnrichBackupJobs(context.Background(), clients, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f, ok := result.Findings["plan-1"]
	if !ok {
		t.Fatalf("expected finding keyed by BackupPlanId %q", "plan-1")
	}
	if f.Severity != "!" {
		t.Errorf("severity = %q, want %q", f.Severity, "!")
	}
}

// TestEnrichBackupJobs_AbortedRecentJobProducesFindingSevBang verifies that an
// ABORTED job created recently produces a finding with severity "!".
func TestEnrichBackupJobs_AbortedRecentJobProducesFindingSevBang(t *testing.T) {
	fake := &backupJobFake{
		jobs: []backuptypes.BackupJob{
			{
				State:        backuptypes.BackupJobStateAborted,
				CreationDate: nowPtr(),
				CreatedBy:    &backuptypes.RecoveryPointCreator{BackupPlanId: aws.String("plan-aborted")},
			},
		},
	}
	clients := &awsclient.ServiceClients{Backup: fake}

	result, err := awsclient.EnrichBackupJobs(context.Background(), clients, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f, ok := result.Findings["plan-aborted"]
	if !ok {
		t.Fatalf("expected finding keyed by BackupPlanId %q", "plan-aborted")
	}
	if f.Severity != "!" {
		t.Errorf("severity = %q, want %q", f.Severity, "!")
	}
}

// TestEnrichBackupJobs_ExpiredRecentJobProducesFindingSevBang verifies that an
// EXPIRED job created recently produces a finding with severity "!".
func TestEnrichBackupJobs_ExpiredRecentJobProducesFindingSevBang(t *testing.T) {
	fake := &backupJobFake{
		jobs: []backuptypes.BackupJob{
			{
				State:        backuptypes.BackupJobStateExpired,
				CreationDate: nowPtr(),
				CreatedBy:    &backuptypes.RecoveryPointCreator{BackupPlanId: aws.String("plan-expired")},
			},
		},
	}
	clients := &awsclient.ServiceClients{Backup: fake}

	result, err := awsclient.EnrichBackupJobs(context.Background(), clients, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f, ok := result.Findings["plan-expired"]
	if !ok {
		t.Fatalf("expected finding keyed by BackupPlanId %q", "plan-expired")
	}
	if f.Severity != "!" {
		t.Errorf("severity = %q, want %q", f.Severity, "!")
	}
}

// TestEnrichBackupJobs_PartialRecentJobProducesFindingSevTilde verifies that a
// PARTIAL job created within the last 24h produces a finding with severity "~".
func TestEnrichBackupJobs_PartialRecentJobProducesFindingSevTilde(t *testing.T) {
	fake := &backupJobFake{
		jobs: []backuptypes.BackupJob{
			{
				State:        backuptypes.BackupJobStatePartial,
				CreationDate: nowPtr(),
				CreatedBy:    &backuptypes.RecoveryPointCreator{BackupPlanId: aws.String("plan-partial")},
			},
		},
	}
	clients := &awsclient.ServiceClients{Backup: fake}

	result, err := awsclient.EnrichBackupJobs(context.Background(), clients, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f, ok := result.Findings["plan-partial"]
	if !ok {
		t.Fatalf("expected finding keyed by BackupPlanId %q", "plan-partial")
	}
	if f.Severity != "~" {
		t.Errorf("severity = %q, want %q", f.Severity, "~")
	}
}

// TestEnrichBackupJobs_CompletedJobProducesNoFinding verifies that a COMPLETED
// job, regardless of recency, never produces a finding.
func TestEnrichBackupJobs_CompletedJobProducesNoFinding(t *testing.T) {
	fake := &backupJobFake{
		jobs: []backuptypes.BackupJob{
			{
				State:        backuptypes.BackupJobStateCompleted,
				CreationDate: nowPtr(),
				CreatedBy:    &backuptypes.RecoveryPointCreator{BackupPlanId: aws.String("plan-ok")},
			},
		},
	}
	clients := &awsclient.ServiceClients{Backup: fake}

	result, err := awsclient.EnrichBackupJobs(context.Background(), clients, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result.Findings["plan-ok"]; ok {
		t.Error("COMPLETED job must NOT appear in Findings")
	}
	if result.IssueCount != 0 {
		t.Errorf("IssueCount = %d, want 0", result.IssueCount)
	}
}

// TestEnrichBackupJobs_FailedOldJobProducesNoFinding verifies that a FAILED job
// created more than 24h ago does NOT produce a finding.
func TestEnrichBackupJobs_FailedOldJobProducesNoFinding(t *testing.T) {
	fake := &backupJobFake{
		jobs: []backuptypes.BackupJob{
			{
				State:        backuptypes.BackupJobStateFailed,
				CreationDate: hoursAgoPtr(49), // 2 days old
				CreatedBy:    &backuptypes.RecoveryPointCreator{BackupPlanId: aws.String("plan-old")},
			},
		},
	}
	clients := &awsclient.ServiceClients{Backup: fake}

	result, err := awsclient.EnrichBackupJobs(context.Background(), clients, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result.Findings["plan-old"]; ok {
		t.Error("FAILED job older than 24h must NOT appear in Findings")
	}
	if result.IssueCount != 0 {
		t.Errorf("IssueCount = %d, want 0", result.IssueCount)
	}
}

// TestEnrichBackupJobs_NilClientReturnsEmptyFindingsNoError verifies that when
// clients.Backup is nil, the enricher returns a non-nil empty Findings map and
// no error.
func TestEnrichBackupJobs_NilClientReturnsEmptyFindingsNoError(t *testing.T) {
	clients := &awsclient.ServiceClients{Backup: nil}

	result, err := awsclient.EnrichBackupJobs(context.Background(), clients, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Findings == nil {
		t.Error("Findings must not be nil when Backup client is nil")
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected empty Findings, got %d entries", len(result.Findings))
	}
}

// TestEnrichBackupJobs_APIErrorIsPropagated verifies that an API error from
// ListBackupJobs is propagated as the enricher's return error.
func TestEnrichBackupJobs_APIErrorIsPropagated(t *testing.T) {
	apiErr := errors.New("backup: list jobs failed")
	fake := &backupJobFake{err: apiErr}
	clients := &awsclient.ServiceClients{Backup: fake}

	_, err := awsclient.EnrichBackupJobs(context.Background(), clients, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, apiErr) {
		t.Errorf("error = %v, want to wrap %v", err, apiErr)
	}
}

// Compile-time check: backupJobFake satisfies BackupAPI.
var _ awsclient.BackupAPI = (*backupJobFake)(nil)

// TestEnrichBackupJobs_IssueCountEqualsFindings verifies that IssueCount equals
// len(Findings) when multiple jobs are processed.
func TestEnrichBackupJobs_IssueCountEqualsFindings(t *testing.T) {
	fake := &backupJobFake{
		jobs: []backuptypes.BackupJob{
			{
				State:        backuptypes.BackupJobStateFailed,
				CreationDate: nowPtr(),
				CreatedBy:    &backuptypes.RecoveryPointCreator{BackupPlanId: aws.String("plan-A")},
			},
			{
				State:        backuptypes.BackupJobStateAborted,
				CreationDate: nowPtr(),
				CreatedBy:    &backuptypes.RecoveryPointCreator{BackupPlanId: aws.String("plan-B")},
			},
			{
				State:        backuptypes.BackupJobStateCompleted,
				CreationDate: nowPtr(),
				CreatedBy:    &backuptypes.RecoveryPointCreator{BackupPlanId: aws.String("plan-C")},
			},
		},
	}
	clients := &awsclient.ServiceClients{Backup: fake}

	result, err := awsclient.EnrichBackupJobs(context.Background(), clients, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IssueCount != 2 {
		t.Errorf("IssueCount = %d, want 2", result.IssueCount)
	}
	if result.IssueCount != len(result.Findings) {
		t.Errorf("IssueCount (%d) != len(Findings) (%d)", result.IssueCount, len(result.Findings))
	}
}
