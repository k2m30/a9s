package unit

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/backup"
	backuptypes "github.com/aws/aws-sdk-go-v2/service/backup/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// T-BK01 - Test Backup ListBackupPlans fetch
// ---------------------------------------------------------------------------

func TestFetchBackupPlans_ParsesMultiplePlans(t *testing.T) {
	creationDate := time.Date(2025, 3, 15, 10, 30, 0, 0, time.UTC)
	lastExecution := time.Date(2025, 12, 1, 8, 0, 0, 0, time.UTC)

	listMock := &mockBackupListBackupPlansClient{
		output: &backup.ListBackupPlansOutput{
			BackupPlansList: []backuptypes.BackupPlansListMember{
				{
					BackupPlanName:    aws.String("daily-backup"),
					BackupPlanId:      aws.String("plan-111-aaa"),
					BackupPlanArn:     aws.String("arn:aws:backup:us-east-1:123456789012:backup-plan:plan-111-aaa"),
					CreationDate:      &creationDate,
					LastExecutionDate: &lastExecution,
					VersionId:         aws.String("v1"),
				},
				{
					BackupPlanName: aws.String("weekly-backup"),
					BackupPlanId:   aws.String("plan-222-bbb"),
					BackupPlanArn:  aws.String("arn:aws:backup:us-east-1:123456789012:backup-plan:plan-222-bbb"),
					CreationDate:   &creationDate,
					VersionId:      aws.String("v2"),
				},
			},
		},
	}

	resources, err := awsclient.FetchBackupPlans(context.Background(), listMock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	// Verify required fields
	requiredFields := []string{"plan_name", "plan_id", "creation_date", "last_execution"}
	for i, r := range resources {
		for _, key := range requiredFields {
			if _, ok := r.Fields[key]; !ok {
				t.Errorf("resource[%d].Fields missing key %q", i, key)
			}
		}
	}

	// Verify first plan
	r0 := resources[0]
	if r0.ID != "plan-111-aaa" {
		t.Errorf("resource[0].ID: expected %q, got %q", "plan-111-aaa", r0.ID)
	}
	if r0.Name != "daily-backup" {
		t.Errorf("resource[0].Name: expected %q, got %q", "daily-backup", r0.Name)
	}
	if r0.Fields["plan_name"] != "daily-backup" {
		t.Errorf("resource[0].Fields[\"plan_name\"]: expected %q, got %q", "daily-backup", r0.Fields["plan_name"])
	}
	if r0.Fields["plan_id"] != "plan-111-aaa" {
		t.Errorf("resource[0].Fields[\"plan_id\"]: expected %q, got %q", "plan-111-aaa", r0.Fields["plan_id"])
	}

	// Verify last_execution is set for first but empty for second
	if r0.Fields["last_execution"] == "" {
		t.Error("resource[0].Fields[\"last_execution\"] should not be empty")
	}

	r1 := resources[1]
	if r1.Fields["last_execution"] != "" {
		t.Errorf("resource[1].Fields[\"last_execution\"] should be empty (no last execution), got %q", r1.Fields["last_execution"])
	}

	// Verify RawStruct is set
	if r0.RawStruct == nil {
		t.Error("resource[0].RawStruct should not be nil")
	}

}

func TestFetchBackupPlans_ListError(t *testing.T) {
	listMock := &mockBackupListBackupPlansClient{
		err: fmt.Errorf("AWS API error: access denied"),
	}

	resources, err := awsclient.FetchBackupPlans(context.Background(), listMock)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if resources != nil {
		t.Errorf("expected nil resources on error, got %d resources", len(resources))
	}
}

func TestFetchBackupPlans_EmptyResponse(t *testing.T) {
	listMock := &mockBackupListBackupPlansClient{
		output: &backup.ListBackupPlansOutput{
			BackupPlansList: []backuptypes.BackupPlansListMember{},
		},
	}

	resources, err := awsclient.FetchBackupPlans(context.Background(), listMock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}

// ---------------------------------------------------------------------------
// T-BK02 - Resource type definition
// ---------------------------------------------------------------------------

func TestBackup_ResourceTypeDef(t *testing.T) {
	rt := resource.FindResourceType("backup")
	if rt == nil {
		t.Fatal("resource type 'backup' not found")
	}

	if rt.Name != "Backup Plans" {
		t.Errorf("expected name %q, got %q", "Backup Plans", rt.Name)
	}

	expected := []struct {
		title string
		key   string
		width int
	}{
		{"Plan Name", "plan_name", 32},
		{"Plan ID", "plan_id", 38},
		{"Created", "creation_date", 22},
		{"Last Execution", "last_execution", 22},
	}

	if len(rt.Columns) != len(expected) {
		t.Fatalf("expected %d columns, got %d", len(expected), len(rt.Columns))
	}

	for i, want := range expected {
		col := rt.Columns[i]
		if col.Title != want.title {
			t.Errorf("column %d: expected title %q, got %q", i, want.title, col.Title)
		}
		if col.Key != want.key {
			t.Errorf("column %d (%s): expected key %q, got %q", i, want.title, want.key, col.Key)
		}
		if col.Width != want.width {
			t.Errorf("column %d (%s): expected width %d, got %d", i, want.title, want.width, col.Width)
		}
	}
}

func TestBackup_Aliases(t *testing.T) {
	aliases := []string{"backup", "backup-plans"}
	for _, alias := range aliases {
		rt := resource.FindResourceType(alias)
		if rt == nil {
			t.Errorf("expected resource type for alias %q, got nil", alias)
		}
	}
}
