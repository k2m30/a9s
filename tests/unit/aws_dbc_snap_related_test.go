package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	backuptypes "github.com/aws/aws-sdk-go-v2/service/backup/types"
	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func dbcSnapCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("dbc-snap") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("dbc-snap related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("dbc-snap related checker for %s not found", target)
	return nil
}

func TestRelated_DbcSnap_Registered(t *testing.T) {
	defs := resource.GetRelated("dbc-snap")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for dbc-snap")
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
					t.Errorf("dbc-snap %q: Checker should not be nil", target)
				}
				if !want.hasChecker && def.Checker != nil {
					t.Errorf("dbc-snap %q: Checker should be nil (stub)", target)
				}
				if def.DisplayName != want.displayName {
					t.Errorf("dbc-snap %q: DisplayName = %q, want %q", target, def.DisplayName, want.displayName)
				}
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found", target)
		}
	}
}

// --- Backup checker tests (Pattern C — cache scan, zero API calls) ---
//
// The dbc-snap → backup pivot now mirrors the dbi-snap → backup pattern:
// resolve the snapshot's parent cluster ARN via the dbc cache, then scan the
// loaded backup PLAN cache for plans whose Fields["resources"] cover that
// cluster ARN. Returns plan IDs (not recovery-point ARNs) so drill-through
// lands on the backup-plan list.

const dbcSnapTestClusterID = "acme-docdb-prod"
const dbcSnapTestClusterARN = "arn:aws:rds:us-east-1:123456789012:cluster:acme-docdb-prod"

func dbcSnapBackupSrcResource() resource.Resource {
	return resource.Resource{
		ID:     "rds:acme-docdb-prod-2026-03-20",
		Name:   "rds:acme-docdb-prod-2026-03-20",
		Fields: map[string]string{},
		RawStruct: docdbtypes.DBClusterSnapshot{
			DBClusterSnapshotIdentifier: aws.String("rds:acme-docdb-prod-2026-03-20"),
			DBClusterIdentifier:         aws.String(dbcSnapTestClusterID),
		},
	}
}

func dbcSnapBackupCache(planResources string) resource.ResourceCache {
	dbcParent := docdbtypes.DBCluster{
		DBClusterIdentifier: aws.String(dbcSnapTestClusterID),
		DBClusterArn:        aws.String(dbcSnapTestClusterARN),
	}
	return resource.ResourceCache{
		"dbc": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{
					ID:        dbcSnapTestClusterID,
					Name:      dbcSnapTestClusterID,
					RawStruct: dbcParent,
				},
			},
		},
		"backup": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{
					ID:   "plan-aaa",
					Name: "plan-aaa",
					Fields: map[string]string{
						"resources": planResources,
					},
				},
				{
					ID:   "plan-bbb",
					Name: "plan-bbb",
					Fields: map[string]string{
						"resources": "arn:aws:rds:us-east-1:123456789012:cluster:other-cluster",
					},
				},
			},
		},
	}
}

// TestRelated_DbcSnap_Backup_Match verifies a single plan whose resources
// cover the parent cluster ARN resolves to Count=1 with that plan's ID.
func TestRelated_DbcSnap_Backup_Match(t *testing.T) {
	res := dbcSnapBackupSrcResource()
	cache := dbcSnapBackupCache(dbcSnapTestClusterARN)

	checker := dbcSnapCheckerByTarget(t, "backup")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Fatalf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "plan-aaa" {
		t.Errorf("ResourceIDs = %v, want [plan-aaa]", result.ResourceIDs)
	}
}

// TestRelated_DbcSnap_Backup_Empty verifies Count=0 when no plan covers the
// parent cluster ARN.
func TestRelated_DbcSnap_Backup_Empty(t *testing.T) {
	res := dbcSnapBackupSrcResource()
	cache := dbcSnapBackupCache("arn:aws:rds:us-east-1:123456789012:cluster:unrelated")

	checker := dbcSnapCheckerByTarget(t, "backup")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no plan matches)", result.Count)
	}
}

// TestRelated_DbcSnap_Backup_NoParentReference verifies Count=0 when the
// snapshot has no DBClusterIdentifier (manual/shared snapshot).
func TestRelated_DbcSnap_Backup_NoParentReference(t *testing.T) {
	res := resource.Resource{
		ID: "snap-1",
		RawStruct: docdbtypes.DBClusterSnapshot{
			DBClusterSnapshotIdentifier: aws.String("snap-1"),
			// DBClusterIdentifier intentionally nil
		},
	}
	checker := dbcSnapCheckerByTarget(t, "backup")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no parent reference)", result.Count)
	}
}

// Suppress unused-import warning when backuptypes is no longer needed.
var _ = backuptypes.RecoveryPointByResource{}
