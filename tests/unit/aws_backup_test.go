package unit

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/backup"
	backuptypes "github.com/aws/aws-sdk-go-v2/service/backup/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/config"
	"github.com/k2m30/a9s/v3/core/demo/fakes"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/resource"
)

type backupPlanListMock struct {
	output *backup.ListBackupPlansOutput
	err    error
}

func (m *backupPlanListMock) ListBackupPlans(_ context.Context, _ *backup.ListBackupPlansInput, _ ...func(*backup.Options)) (*backup.ListBackupPlansOutput, error) {
	return m.output, m.err
}

// TestBackup_Fetcher_HealthyPlan_NoWave1Findings: the list API is config-only,
// so no plan returned by the fetcher carries a wave-1 finding.
func TestBackup_Fetcher_HealthyPlan_NoWave1Findings(t *testing.T) {
	fake := fakes.NewBackup()
	resources, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchBackupPlansPage(context.Background(), fake, token)
	})
	if err != nil {
		t.Fatalf("FetchBackupPlans returned error: %v", err)
	}
	if len(resources) == 0 {
		t.Fatal("FetchBackupPlans returned no resources")
	}

	for _, r := range resources {
		if len(r.Findings) != 0 {
			t.Fatalf("Resource.Findings must be empty for plan %s — spec §3.1: no Wave-1 signals", r.ID)
		}
	}
}

// TestBackup_Fetcher_NeverRanPlan_IsHealthy verifies that plan-never-ran
// (LastExecutionDate == nil, zero jobs ever) returns as a healthy resource
// with blank Status and empty last_execution field.
func TestBackup_Fetcher_NeverRanPlan_IsHealthy(t *testing.T) {
	fake := fakes.NewBackup()
	resources, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchBackupPlansPage(context.Background(), fake, token)
	})
	if err != nil {
		t.Fatalf("FetchBackupPlans returned error: %v", err)
	}

	var found bool
	for _, r := range resources {
		if r.ID != fixtures.NeverRanPlanID {
			continue
		}
		found = true
		if len(r.Findings) != 0 {
			t.Fatal("plan-never-ran must have no Findings (Healthy — spec §4, no Wave-1 signals §3.1)")
		}
		if r.Fields["last_execution"] != "" {
			t.Fatal("plan-never-ran must have empty last_execution field (LastExecutionDate is nil)")
		}
	}
	if !found {
		t.Fatalf("plan-never-ran (%s) not found in fetcher output", fixtures.NeverRanPlanID)
	}
}

// TestBackup_Fetcher_MapsHealthyPlanFields verifies field mapping for the
// healthy daily plan fixture. Asserts exact field values from the fixture file.
func TestBackup_Fetcher_MapsHealthyPlanFields(t *testing.T) {
	fake := fakes.NewBackup()
	resources, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchBackupPlansPage(context.Background(), fake, token)
	})
	if err != nil {
		t.Fatalf("FetchBackupPlans returned error: %v", err)
	}

	var found bool
	for _, r := range resources {
		if r.ID != fixtures.HealthyDailyPlanID {
			continue
		}
		found = true

		if r.ID != fixtures.HealthyDailyPlanID {
			t.Fatalf("Resource.ID mismatch: got %q, want %q", r.ID, fixtures.HealthyDailyPlanID)
		}
		if r.Name != "acme-daily-backup" {
			t.Fatalf("Resource.Name mismatch: got %q, want %q", r.Name, "acme-daily-backup")
		}
		if len(r.Findings) != 0 {
			t.Fatal("Resource.Findings must be empty (fetcher silence §3.1)")
		}

		if r.Fields["plan_name"] != "acme-daily-backup" {
			t.Fatalf("Fields[plan_name] mismatch: got %q, want %q", r.Fields["plan_name"], "acme-daily-backup")
		}
		if r.Fields["plan_id"] != fixtures.HealthyDailyPlanID {
			t.Fatalf("Fields[plan_id] mismatch: got %q, want %q", r.Fields["plan_id"], fixtures.HealthyDailyPlanID)
		}

		if r.Fields["last_execution"] != "2026-04-22 02:00" {
			t.Fatalf("Fields[last_execution] must be formatted '2006-01-02 15:04': got %q", r.Fields["last_execution"])
		}

		sels, complete := awsclient.BackupPlanSelections(r)
		if !complete || len(sels) == 0 || !slices.Contains(sels[0].Resources, fixtures.HealthyBucketARN) {
			t.Fatalf("BackupPlanSelections = %+v (complete %v), want the plan's selection naming HealthyBucketARN", sels, complete)
		}

		if r.RawStruct == nil {
			t.Fatal("Resource.RawStruct must not be nil")
		}
	}
	if !found {
		t.Fatalf("HealthyDailyPlanID %s not found in fetcher output", fixtures.HealthyDailyPlanID)
	}
}

// TestBackup_Fetcher_ResourceIssuesEmptyForAllFixtures: the list API is
// config-only, so FetchBackupPlans never populates Resource.Issues for any
// backup plan, regardless of job state.
func TestBackup_Fetcher_ResourceIssuesEmptyForAllFixtures(t *testing.T) {
	fake := fakes.NewBackup()
	resources, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchBackupPlansPage(context.Background(), fake, token)
	})
	if err != nil {
		t.Fatalf("FetchBackupPlans returned error: %v", err)
	}
	// plan-fleet-wide is the blanket selection that covers every demo row except
	// the four that show "not covered by a backup plan".
	if len(resources) != 9 {
		t.Fatalf("expected 9 fixture plans (impl-plan §2 plus plan-fleet-wide); update this count if fixtures change: got %d", len(resources))
	}

	for _, r := range resources {
		if len(r.Findings) != 0 {
			t.Fatalf("Resource.Findings must be empty for plan %s (%s) — spec §3.1 declares no Wave-1 signals",
				r.ID, r.Name)
		}
	}
}

// TestBackup_Fetcher_NilPlanID_Skipped verifies that a BackupPlansListMember
// with BackupPlanId == nil does not cause a panic. The fetcher must handle nil
// IDs defensively. The valid plan must still appear in output.
func TestBackup_Fetcher_NilPlanID_Skipped(t *testing.T) {
	mock := &backupPlanListMock{
		output: &backup.ListBackupPlansOutput{
			BackupPlansList: []backuptypes.BackupPlansListMember{
				{
					BackupPlanName: aws.String("orphan-plan-no-id"),
				},
				{
					BackupPlanId:   aws.String("valid-plan-001"),
					BackupPlanName: aws.String("valid-plan"),
				},
			},
		},
	}

	resources, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchBackupPlansPage(context.Background(), mock, token)
	})
	if err != nil {
		t.Fatalf("FetchBackupPlans returned error: %v", err)
	}

	var validFound bool
	for _, r := range resources {
		if r.ID == "valid-plan-001" {
			validFound = true
			break
		}
	}
	if !validFound {
		t.Fatal("valid plan 'valid-plan-001' must appear in fetcher output")
	}
}

// TestBackup_DefaultListColumns_OneStatusColumn verifies the default backup list view column contract.
func TestBackup_DefaultListColumns_OneStatusColumn(t *testing.T) {
	viewDef := config.DefaultViewDef("backup")
	cols := viewDef.List
	if len(cols) == 0 {
		t.Fatal("default backup list columns must not be empty")
	}

	statusCount := 0
	for _, col := range cols {
		if col.Key == "status" {
			statusCount++
		}
	}
	if statusCount != 1 {
		t.Fatalf("expected exactly one column keyed 'status'; got %d in columns %v",
			statusCount, cols)
	}

	for _, col := range cols {
		if col.Key == "last_status" {
			t.Fatal("column keyed 'last_status' is banned per spec §4 — replace with 'status'")
		}
	}

	banned := []string{
		"Last Status", "CIS", "Flags", "Policy", "Issues",
		"NOBKP", "UNENC", "PUB", "NOPROT",
	}
	for _, col := range cols {
		for _, bad := range banned {
			if col.Title == bad {
				t.Fatalf("column title %q is in the banned jargon set and must not appear", bad)
			}
		}
	}

	required := []string{"Plan Name", "Plan ID", "Created", "Last Execution"}
	titleSet := make(map[string]bool, len(cols))
	for _, col := range cols {
		titleSet[col.Title] = true
	}
	for _, title := range required {
		if !titleSet[title] {
			t.Fatalf("expected identity column %q in default backup list view", title)
		}
	}
}

var _ awsclient.BackupAPI = fakes.NewBackup()

// enumerateBackupPlanResources is fail-closed: any GetBackupSelection error
// yields ("", ""), because partial data could drop NotResources exclusions and
// report false-positive backup coverage. The function is unexported and is
// exercised through FetchBackupPlans with a mock implementing both
// BackupListBackupSelectionsAPI and BackupGetBackupSelectionAPI, which the
// fetcher type-asserts at call time.

// backupFullMock implements BackupListBackupPlansAPI, BackupListBackupSelectionsAPI,
// and BackupGetBackupSelectionAPI so FetchBackupPlansPage can type-assert all
// three interfaces from a single mock value.
type backupFullMock struct {
	plansOutput      *backup.ListBackupPlansOutput
	plansErr         error
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
// GetBackupSelection returns an error for a selection, the plan row says its
// selections are incomplete, so no reader takes it for a plan that selects
// nothing.
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

	resources, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchBackupPlansPage(context.Background(), mock, token)
	})
	if err != nil {
		t.Fatalf("FetchBackupPlans must not propagate GetBackupSelection errors: %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("one plan must still be returned: got %d", len(resources))
	}

	if sels, complete := awsclient.BackupPlanSelections(resources[0]); complete || len(sels) != 0 {
		t.Errorf("BackupPlanSelections = %d selections, complete %v; want none read and incomplete", len(sels), complete)
	}
}

// TestBackup_EnumerateSelection_SuccessKeepsBothLists verifies that when all
// GetBackupSelection calls succeed, the plan row's selection carries both its
// included and its excluded ARNs.
func TestBackup_EnumerateSelection_SuccessKeepsBothLists(t *testing.T) {
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

	resources, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchBackupPlansPage(context.Background(), mock, token)
	})
	if err != nil {
		t.Fatalf("FetchBackupPlans returned error: %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	sels, complete := awsclient.BackupPlanSelections(resources[0])
	if !complete || len(sels) != 1 {
		t.Fatalf("BackupPlanSelections = %d selections, complete %v; want 1, complete", len(sels), complete)
	}
	if !slices.Equal(sels[0].Resources, []string{includeARN}) || !slices.Equal(sels[0].NotResources, []string{excludeARN}) {
		t.Errorf("selection Resources/NotResources = %v/%v, want [%s]/[%s]", sels[0].Resources, sels[0].NotResources, includeARN, excludeARN)
	}
}
