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
func TestBackup_Fetcher_HealthyPlan_StatusIsEmpty(t *testing.T) {
	fake := fakes.NewBackup()
	resources, err := awsclient.FetchBackupPlans(context.Background(), fake)
	require.NoError(t, err)
	require.NotEmpty(t, resources)

	for _, r := range resources {
		require.Empty(t, r.Status,
			"Resource.Status must be empty for plan %s — spec §3.1: no Wave-1 signals", r.ID)
		require.Empty(t, r.Issues,
			"Resource.Issues must be empty for plan %s — spec §3.1: no Wave-1 signals", r.ID)
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
		require.Empty(t, r.Status,
			"plan-never-ran must have empty Status (Healthy — spec §4)")
		require.Empty(t, r.Issues,
			"plan-never-ran must have empty Issues (spec §3.1 Wave-1 silent)")
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
		require.Empty(t, r.Status, "Resource.Status must be empty (fetcher silence §3.1)")
		require.Empty(t, r.Issues, "Resource.Issues must be empty (no Wave-1 signals §3.1)")

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
		require.Empty(t, r.Issues,
			"Resource.Issues must be empty for plan %s (%s) — spec §3.1 declares no Wave-1 signals",
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
