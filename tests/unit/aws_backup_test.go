// aws_backup_test.go — fetcher behavior + default list column shape.
//
// Covers:
//   - TEST: healthy_plan_no_jobs_is_silent (U1, U7f)
//   - TEST: plan_with_zero_jobs_ever_is_still_healthy (U1, U7f)
//   - TEST: list_view_carries_exactly_one_status_column_backed_by_status_key (U10)
//   - TestBackup_Fetcher_MapsHealthyPlanFields — field-mapping contract.
//   - TestBackup_Fetcher_ResourceIssuesEmptyForAllFixtures — U7f invariant for all 8 fixtures.
//   - TestBackup_Fetcher_NilPlanID_Skipped — nil BackupPlanId does not panic.
package unit

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/backup"
	backuptypes "github.com/aws/aws-sdk-go-v2/service/backup/types"
	"github.com/stretchr/testify/require"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/demo/fakes"
	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
)

// ---------------------------------------------------------------------------
// minimal mock — BackupListBackupPlansAPI only (used in nil-plan-id test)
// ---------------------------------------------------------------------------

type backupPlanListMock struct {
	output *backup.ListBackupPlansOutput
	err    error
}

func (m *backupPlanListMock) ListBackupPlans(_ context.Context, _ *backup.ListBackupPlansInput, _ ...func(*backup.Options)) (*backup.ListBackupPlansOutput, error) {
	return m.output, m.err
}

// ---------------------------------------------------------------------------
// TEST: healthy_plan_no_jobs_is_silent (U1, U7f)
// ---------------------------------------------------------------------------

// TestBackup_Fetcher_HealthyPlan_StatusIsEmpty verifies that every plan
// returned by the fetcher has Status == "" and Issues == nil/empty.
// Spec §3.1: "No Wave 1 signals" — the list API is config-only.
func TestBackup_Fetcher_HealthyPlan_NoWave1Findings(t *testing.T) {
	fake := fakes.NewBackup()
	resources, err := awsclient.FetchBackupPlans(context.Background(), fake)
	require.NoError(t, err)
	require.NotEmpty(t, resources)

	for _, r := range resources {
		require.Empty(t, r.Findings,
			"Resource.Findings must be empty for plan %s — spec §3.1: no Wave-1 signals", r.ID)
	}
}

// ---------------------------------------------------------------------------
// TEST: plan_with_zero_jobs_ever_is_still_healthy
// ---------------------------------------------------------------------------

// TestBackup_Fetcher_NeverRanPlan_IsHealthy verifies that plan-never-ran
// (LastExecutionDate == nil, zero jobs ever) returns as a healthy resource
// with blank Status and empty last_execution field.
// Spec §4: "A plan that has *never* run is also Healthy by this rule."
func TestBackup_Fetcher_NeverRanPlan_IsHealthy(t *testing.T) {
	fake := fakes.NewBackup()
	resources, err := awsclient.FetchBackupPlans(context.Background(), fake)
	require.NoError(t, err)

	var found bool
	for _, r := range resources {
		if r.ID != fixtures.NeverRanPlanID {
			continue
		}
		found = true
		require.Empty(t, r.Findings,
			"plan-never-ran must have no Findings (Healthy — spec §4, no Wave-1 signals §3.1)")
		require.Empty(t, r.Fields["last_execution"],
			"plan-never-ran must have empty last_execution field (LastExecutionDate is nil)")
	}
	require.True(t, found, "plan-never-ran (%s) not found in fetcher output", fixtures.NeverRanPlanID)
}

// ---------------------------------------------------------------------------
// TestBackup_Fetcher_MapsHealthyPlanFields
// ---------------------------------------------------------------------------

// TestBackup_Fetcher_MapsHealthyPlanFields verifies field mapping for the
// healthy daily plan fixture. Asserts exact field values from the fixture file.
func TestBackup_Fetcher_MapsHealthyPlanFields(t *testing.T) {
	fake := fakes.NewBackup()
	resources, err := awsclient.FetchBackupPlans(context.Background(), fake)
	require.NoError(t, err)

	var found bool
	for _, r := range resources {
		if r.ID != fixtures.HealthyDailyPlanID {
			continue
		}
		found = true

		require.Equal(t, fixtures.HealthyDailyPlanID, r.ID, "Resource.ID mismatch")
		require.Equal(t, "acme-daily-backup", r.Name, "Resource.Name mismatch")
		require.Empty(t, r.Findings, "Resource.Findings must be empty (fetcher silence §3.1)")

		require.Equal(t, "acme-daily-backup", r.Fields["plan_name"],
			"Fields[plan_name] mismatch")
		require.Equal(t, fixtures.HealthyDailyPlanID, r.Fields["plan_id"],
			"Fields[plan_id] mismatch")

		// LastExecutionDate fixture = 2026-04-22T02:00:00Z → "2026-04-22 02:00"
		require.Equal(t, "2026-04-22 02:00", r.Fields["last_execution"],
			"Fields[last_execution] must be formatted '2006-01-02 15:04'")

		// resources CSV: healthy plan's selection covers HealthyBucketARN + EFS ARN.
		require.NotEmpty(t, r.Fields["resources"],
			"Fields[resources] must be non-empty — fetcher enumerates plan selections")
		require.Contains(t, r.Fields["resources"], fixtures.HealthyBucketARN,
			"Fields[resources] must contain HealthyBucketARN from the plan's selection")

		require.NotNil(t, r.RawStruct, "Resource.RawStruct must not be nil")
	}
	require.True(t, found, "HealthyDailyPlanID %s not found in fetcher output",
		fixtures.HealthyDailyPlanID)
}

// ---------------------------------------------------------------------------
// TestBackup_Fetcher_ResourceIssuesEmptyForAllFixtures (U7f)
// ---------------------------------------------------------------------------

// TestBackup_Fetcher_ResourceIssuesEmptyForAllFixtures asserts the U7f
// invariant: every plan returned by FetchBackupPlans has an empty Issues slice.
// Spec §3.1 explicitly states "No Wave 1 signals" — the fetcher must never
// populate Resource.Issues for any backup plan, regardless of job state.
func TestBackup_Fetcher_ResourceIssuesEmptyForAllFixtures(t *testing.T) {
	fake := fakes.NewBackup()
	resources, err := awsclient.FetchBackupPlans(context.Background(), fake)
	require.NoError(t, err)
	require.Len(t, resources, 8,
		"expected 8 fixture plans (impl-plan §2); update this count if fixtures change")

	for _, r := range resources {
		require.Empty(t, r.Findings,
			"Resource.Findings must be empty for plan %s (%s) — spec §3.1 declares no Wave-1 signals",
			r.ID, r.Name)
	}
}

// ---------------------------------------------------------------------------
// TestBackup_Fetcher_NilPlanID_Skipped
// ---------------------------------------------------------------------------

// TestBackup_Fetcher_NilPlanID_Skipped verifies that a BackupPlansListMember
// with BackupPlanId == nil does not cause a panic. The fetcher must handle nil
// IDs defensively. The valid plan must still appear in output.
func TestBackup_Fetcher_NilPlanID_Skipped(t *testing.T) {
	mock := &backupPlanListMock{
		output: &backup.ListBackupPlansOutput{
			BackupPlansList: []backuptypes.BackupPlansListMember{
				{
					// BackupPlanId intentionally nil.
					BackupPlanName: aws.String("orphan-plan-no-id"),
				},
				{
					BackupPlanId:   aws.String("valid-plan-001"),
					BackupPlanName: aws.String("valid-plan"),
				},
			},
		},
	}

	// Must not panic.
	resources, err := awsclient.FetchBackupPlans(context.Background(), mock)
	require.NoError(t, err)

	var validFound bool
	for _, r := range resources {
		if r.ID == "valid-plan-001" {
			validFound = true
			break
		}
	}
	require.True(t, validFound, "valid plan 'valid-plan-001' must appear in fetcher output")
}

// ---------------------------------------------------------------------------
// TEST: list_view_carries_exactly_one_status_column_backed_by_status_key (U10)
// ---------------------------------------------------------------------------

// TestBackup_DefaultListColumns_OneStatusColumn verifies the default backup
// list view column contract (spec §4 S4, U10):
//   - Exactly one column keyed "status".
//   - No column keyed "last_status" (banned old column).
//   - No jargon column titles from the banned set.
//   - Identity columns Plan Name, Plan ID, Created, Last Execution are present.
//
// This test will FAIL against the pre-rewrite defaults_backup.go which uses
// "Last Status"/"last_status" — that is intentional (TDD red phase).
func TestBackup_DefaultListColumns_OneStatusColumn(t *testing.T) {
	viewDef := config.DefaultViewDef("backup")
	cols := viewDef.List
	require.NotEmpty(t, cols, "default backup list columns must not be empty")

	// Exactly one column keyed "status".
	statusCount := 0
	for _, col := range cols {
		if col.Key == "status" {
			statusCount++
		}
	}
	require.Equal(t, 1, statusCount,
		"expected exactly one column keyed 'status'; got %d in columns %v",
		statusCount, cols)

	// No column keyed "last_status".
	for _, col := range cols {
		require.NotEqual(t, "last_status", col.Key,
			"column keyed 'last_status' is banned per spec §4 — replace with 'status'")
	}

	// No jargon titles.
	banned := []string{
		"Last Status", "CIS", "Flags", "Policy", "Issues",
		"NOBKP", "UNENC", "PUB", "NOPROT",
	}
	for _, col := range cols {
		for _, bad := range banned {
			require.NotEqual(t, bad, col.Title,
				"column title %q is in the banned jargon set and must not appear", bad)
		}
	}

	// Required identity columns must be present.
	required := []string{"Plan Name", "Plan ID", "Created", "Last Execution"}
	titleSet := make(map[string]bool, len(cols))
	for _, col := range cols {
		titleSet[col.Title] = true
	}
	for _, title := range required {
		require.True(t, titleSet[title],
			"expected identity column %q in default backup list view", title)
	}
}

// Compile-time guard that fakes.BackupFake satisfies awsclient.BackupAPI.
// Package-level var is the correct idiom (the former Test* wrapper was
// busywork — t.Helper is a no-op inside a Test function and the guard
// runs at compile time, not test time).
var _ awsclient.BackupAPI = fakes.NewBackup()

// ---------------------------------------------------------------------------
// PIN 4 — enumerateBackupPlanResources fail-closed regression pins
// ---------------------------------------------------------------------------
// These tests exercise the fail-closed contract of enumerateBackupPlanResources
// (backup.go). Pre-fix code returned partial data when GetBackupSelection
// failed, which could drop NotResources exclusions and yield false-positive
// backup coverage. Post-fix code returns ("","") on any error.
//
// The function is unexported; it is exercised indirectly via FetchBackupPlans
// by providing a mock that implements both BackupListBackupSelectionsAPI and
// BackupGetBackupSelectionAPI — the fetcher type-asserts at call time.

// backupFullMock implements BackupListBackupPlansAPI, BackupListBackupSelectionsAPI,
// and BackupGetBackupSelectionAPI so FetchBackupPlansPage can type-assert all
// three interfaces from a single mock value.
type backupFullMock struct {
	// ListBackupPlans response
	plansOutput *backup.ListBackupPlansOutput
	plansErr    error
	// ListBackupSelections response
	selectionsOutput *backup.ListBackupSelectionsOutput
	selectionsErr    error
	// GetBackupSelection response — same response returned for every selection ID
	getSelectionOutput *backup.GetBackupSelectionOutput
	getSelectionErr    error
}

func (m *backupFullMock) ListBackupPlans(_ context.Context, _ *backup.ListBackupPlansInput, _ ...func(*backup.Options)) (*backup.ListBackupPlansOutput, error) {
	return m.plansOutput, m.plansErr
}
func (m *backupFullMock) ListBackupSelections(_ context.Context, _ *backup.ListBackupSelectionsInput, _ ...func(*backup.Options)) (*backup.ListBackupSelectionsOutput, error) {
	return m.selectionsOutput, m.selectionsErr
}
func (m *backupFullMock) GetBackupSelection(_ context.Context, _ *backup.GetBackupSelectionInput, _ ...func(*backup.Options)) (*backup.GetBackupSelectionOutput, error) {
	return m.getSelectionOutput, m.getSelectionErr
}

// TestBackup_EnumerateSelection_FailClosedOnGetError verifies that when
// GetBackupSelection returns an error for any selection, Fields["resources"]
// and Fields["not_resources"] are both empty strings ("fail-closed").
//
// Pre-fix: partial data was returned for the selections that succeeded before
// the error, which could omit NotResources exclusions and cause false-positive
// backup coverage in related-panel checkers.
func TestBackup_EnumerateSelection_FailClosedOnGetError(t *testing.T) {
	selID := "sel-abc123"
	mock := &backupFullMock{
		plansOutput: &backup.ListBackupPlansOutput{
			BackupPlansList: []backuptypes.BackupPlansListMember{
				{
					BackupPlanId:   aws.String("plan-fail-closed-001"),
					BackupPlanName: aws.String("fail-closed-plan"),
				},
			},
		},
		selectionsOutput: &backup.ListBackupSelectionsOutput{
			BackupSelectionsList: []backuptypes.BackupSelectionsListMember{
				{SelectionId: aws.String(selID)},
			},
		},
		// GetBackupSelection fails — simulates a permissions error or transient failure.
		getSelectionOutput: nil,
		getSelectionErr:    fmt.Errorf("AccessDeniedException: insufficient permissions"),
	}

	resources, err := awsclient.FetchBackupPlans(context.Background(), mock)
	require.NoError(t, err, "FetchBackupPlans must not propagate GetBackupSelection errors")
	require.Len(t, resources, 1, "one plan must still be returned")

	r := resources[0]
	require.Empty(t, r.Fields["resources"],
		"Fields[resources] must be empty string when GetBackupSelection fails (fail-closed)")
	require.Empty(t, r.Fields["not_resources"],
		"Fields[not_resources] must be empty string when GetBackupSelection fails (fail-closed)")
}

// TestBackup_EnumerateSelection_SuccessReturnsBothCSVs verifies that when all
// GetBackupSelection calls succeed, Fields["resources"] contains the included
// ARNs and Fields["not_resources"] contains the excluded ARNs — both as
// comma-separated strings.
func TestBackup_EnumerateSelection_SuccessReturnsBothCSVs(t *testing.T) {
	includeARN := "arn:aws:s3:::acme-backups"
	excludeARN := "arn:aws:s3:::acme-temp"
	selID := "sel-xyz789"

	mock := &backupFullMock{
		plansOutput: &backup.ListBackupPlansOutput{
			BackupPlansList: []backuptypes.BackupPlansListMember{
				{
					BackupPlanId:   aws.String("plan-success-enum-001"),
					BackupPlanName: aws.String("success-enum-plan"),
				},
			},
		},
		selectionsOutput: &backup.ListBackupSelectionsOutput{
			BackupSelectionsList: []backuptypes.BackupSelectionsListMember{
				{SelectionId: aws.String(selID)},
			},
		},
		getSelectionOutput: &backup.GetBackupSelectionOutput{
			BackupSelection: &backuptypes.BackupSelection{
				SelectionName: aws.String("my-selection"),
				IamRoleArn:    aws.String("arn:aws:iam::123456789012:role/AWSBackupDefault"),
				Resources:     []string{includeARN},
				NotResources:  []string{excludeARN},
			},
		},
		getSelectionErr: nil,
	}

	resources, err := awsclient.FetchBackupPlans(context.Background(), mock)
	require.NoError(t, err)
	require.Len(t, resources, 1)

	r := resources[0]
	require.Equal(t, includeARN, r.Fields["resources"],
		"Fields[resources] must contain the ARN from BackupSelection.Resources")
	require.Equal(t, excludeARN, r.Fields["not_resources"],
		"Fields[not_resources] must contain the ARN from BackupSelection.NotResources")
}
