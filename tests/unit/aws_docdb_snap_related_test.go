package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	backuptypes "github.com/aws/aws-sdk-go-v2/service/backup/types"
	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func docdbSnapCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("docdb-snap") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("docdb-snap related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("docdb-snap related checker for %s not found", target)
	return nil
}

func TestRelated_DocdbSnap_Registered(t *testing.T) {
	defs := resource.GetRelated("docdb-snap")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for docdb-snap")
	}

	type expectation struct {
		displayName string
		hasChecker  bool
	}
	expected := map[string]expectation{
		"dbc": {"DocumentDB Cluster", true},
		"kms": {"KMS Key", true},
	}
	for target, want := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == target {
				found = true
				if want.hasChecker && def.Checker == nil {
					t.Errorf("docdb-snap %q: Checker should not be nil", target)
				}
				if !want.hasChecker && def.Checker != nil {
					t.Errorf("docdb-snap %q: Checker should be nil (stub)", target)
				}
				if def.DisplayName != want.displayName {
					t.Errorf("docdb-snap %q: DisplayName = %q, want %q", target, def.DisplayName, want.displayName)
				}
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found", target)
		}
	}
}

// --- Backup checker tests (Pattern A — direct API call) ---

const docdbSnapTestARN = "arn:aws:rds:us-east-1:123456789012:cluster-snapshot:docdb-snap-abc123"
const docdbSnapRecoveryARN1 = "arn:aws:backup:us-east-1:123456789012:recovery-point:rp-docdb-aaa"
const docdbSnapRecoveryARN2 = "arn:aws:backup:us-east-1:123456789012:recovery-point:rp-docdb-bbb"

func docdbSnapSrcResource() resource.Resource {
	return resource.Resource{
		ID:   "docdb-snap-abc123",
		Name: "docdb-snap-abc123",
		Fields: map[string]string{},
		RawStruct: docdbtypes.DBClusterSnapshot{
			DBClusterSnapshotIdentifier: aws.String("docdb-snap-abc123"),
			DBClusterSnapshotArn:        aws.String(docdbSnapTestARN),
		},
	}
}

// TestRelated_DocdbSnap_Backup_Match verifies that two recovery points returned
// by the fake produce Count=2 with both ARNs in ResourceIDs.
func TestRelated_DocdbSnap_Backup_Match(t *testing.T) {
	fake := newFakeBackupWithRecoveryPoints([]backuptypes.RecoveryPointByResource{
		{RecoveryPointArn: aws.String(docdbSnapRecoveryARN1)},
		{RecoveryPointArn: aws.String(docdbSnapRecoveryARN2)},
	})
	clients := &awsclient.ServiceClients{Backup: fake}
	res := docdbSnapSrcResource()

	checker := docdbSnapCheckerByTarget(t, "backup")
	result := checker(context.Background(), clients, res, nil)

	if result.Count != 2 {
		t.Fatalf("Count = %d, want 2", result.Count)
	}
	if len(result.ResourceIDs) != 2 {
		t.Fatalf("ResourceIDs length = %d, want 2: %v", len(result.ResourceIDs), result.ResourceIDs)
	}
	seen := map[string]bool{}
	for _, id := range result.ResourceIDs {
		seen[id] = true
	}
	for _, want := range []string{docdbSnapRecoveryARN1, docdbSnapRecoveryARN2} {
		if !seen[want] {
			t.Errorf("ResourceIDs missing %q; got %v", want, result.ResourceIDs)
		}
	}
	if result.Err != nil {
		t.Errorf("unexpected Err: %v", result.Err)
	}
}

// TestRelated_DocdbSnap_Backup_Empty verifies that zero recovery points produce Count=0.
func TestRelated_DocdbSnap_Backup_Empty(t *testing.T) {
	fake := newFakeBackupWithRecoveryPoints([]backuptypes.RecoveryPointByResource{})
	clients := &awsclient.ServiceClients{Backup: fake}
	res := docdbSnapSrcResource()

	checker := docdbSnapCheckerByTarget(t, "backup")
	result := checker(context.Background(), clients, res, nil)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no recovery points)", result.Count)
	}
	if len(result.ResourceIDs) != 0 {
		t.Errorf("ResourceIDs = %v, want empty", result.ResourceIDs)
	}
}

// TestRelated_DocdbSnap_Backup_WrongRawStruct verifies that a wrong RawStruct
// type returns Count=-1 (defensive guard, assertStruct fails).
func TestRelated_DocdbSnap_Backup_WrongRawStruct(t *testing.T) {
	res := resource.Resource{
		ID:        "docdb-snap-abc123",
		RawStruct: "not-a-snapshot",
	}
	checker := docdbSnapCheckerByTarget(t, "backup")
	result := checker(context.Background(), nil, res, nil)

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (wrong RawStruct type)", result.Count)
	}
}
