package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	backuptypes "github.com/aws/aws-sdk-go-v2/service/backup/types"
	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

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
		"dbc": {"DB Cluster", true},
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

// ---------------------------------------------------------------------------
// Issue 3: checkDbcSnapDBC cache existence check missing
//
// Bug: checkDbcSnapDBC (dbc_snap_related.go:17-31) emits relatedResult("dbc", [id])
// directly from DBClusterIdentifier with NO cache existence check. Sister
// checkDBISnapDBI does the cache scan + ApproximateZero/UnknownRelated logic.
// Result: orphan dbc-snap rows whose source cluster is deleted will claim Count=1.
//
// These tests FAIL today because checkDbcSnapDBC always returns Count=1 for any
// non-empty DBClusterIdentifier, regardless of whether the cluster is in the cache.
// After fix, it must scan the dbc cache and return Count=0 (or ApproximateZero /
// UnknownRelated) when the cluster is absent.
// ---------------------------------------------------------------------------

// dbcSnapDBC_SnapshotWithDocDBRaw builds a dbc-snap source resource with a
// docdbtypes.DBClusterSnapshot RawStruct referencing the given cluster ID.
func dbcSnapDBC_SnapshotWithDocDBRaw(clusterID string) resource.Resource {
	return resource.Resource{
		ID:   "dbc-snap-" + clusterID + "-2026-01-01",
		Name: "dbc-snap-" + clusterID + "-2026-01-01",
		RawStruct: docdbtypes.DBClusterSnapshot{
			DBClusterSnapshotIdentifier: aws.String("dbc-snap-" + clusterID + "-2026-01-01"),
			DBClusterIdentifier:         aws.String(clusterID),
		},
	}
}

// dbcSnapDBC_SnapshotWithRDSRaw builds a dbc-snap source resource with an
// rdstypes.DBClusterSnapshot RawStruct referencing the given cluster ID.
func dbcSnapDBC_SnapshotWithRDSRaw(clusterID string) resource.Resource {
	return resource.Resource{
		ID:   "rds-dbc-snap-" + clusterID + "-2026-01-01",
		Name: "rds-dbc-snap-" + clusterID + "-2026-01-01",
		RawStruct: rdstypes.DBClusterSnapshot{
			DBClusterSnapshotIdentifier: aws.String("rds-dbc-snap-" + clusterID + "-2026-01-01"),
			DBClusterIdentifier:         aws.String(clusterID),
		},
	}
}

// dbcSnapDBC_CompleteCacheWithoutCluster builds a dbc cache that is NOT
// truncated and does NOT contain the given cluster ID.
func dbcSnapDBC_CompleteCacheWithoutCluster(missingClusterID string) resource.ResourceCache {
	return resource.ResourceCache{
		"dbc": resource.ResourceCacheEntry{
			IsTruncated: false, // complete — parent definitively absent
			Resources: []resource.Resource{
				{ID: "other-cluster", Name: "other-cluster"},
			},
		},
	}
}

// dbcSnapDBC_TruncatedCacheWithoutCluster builds a dbc cache that IS truncated
// and does NOT contain the given cluster ID in the visible window.
func dbcSnapDBC_TruncatedCacheWithoutCluster(missingClusterID string) resource.ResourceCache {
	return resource.ResourceCache{
		"dbc": resource.ResourceCacheEntry{
			IsTruncated: true, // truncated — parent may be in later page
			Resources: []resource.Resource{
				{ID: "other-cluster", Name: "other-cluster"},
			},
		},
	}
}

// dbcSnapDBC_CacheWithCluster builds a dbc cache containing the given cluster ID.
func dbcSnapDBC_CacheWithCluster(clusterID string) resource.ResourceCache {
	return resource.ResourceCache{
		"dbc": resource.ResourceCacheEntry{
			IsTruncated: false,
			Resources: []resource.Resource{
				{ID: clusterID, Name: clusterID},
			},
		},
	}
}

// TestRelated_DbcSnap_DBC_OrphanComplete_DocDB verifies that when the dbc cache
// is complete (IsTruncated=false) and does NOT contain the snapshot's parent
// cluster, the checker returns Count=0 (cluster is definitively deleted/absent).
//
// FAILS today: checkDbcSnapDBC returns Count=1 (relatedResult with the cluster ID)
// regardless of cache state — it has no cache scan.
func TestRelated_DbcSnap_DBC_OrphanComplete_DocDB(t *testing.T) {
	const ghostCluster = "ghost-cluster"
	res := dbcSnapDBC_SnapshotWithDocDBRaw(ghostCluster)
	cache := dbcSnapDBC_CompleteCacheWithoutCluster(ghostCluster)

	checker := dbcSnapCheckerByTarget(t, "dbc")
	result := checker(context.Background(), nil, res, cache)

	// FAILS today: checkDbcSnapDBC returns Count=1 unconditionally.
	// PROBE-TRUNCATION-LOST BUG cousin: orphan cluster appears to exist.
	if result.Count != 0 {
		t.Errorf(
			"checkDbcSnapDBC (docdb RawStruct): ghost cluster %q with complete cache: "+
				"Count = %d, want 0 — DBC-SNAP-NO-CACHE-CHECK BUG: orphan dbc-snap "+
				"reports cluster exists when it was deleted",
			ghostCluster, result.Count,
		)
	}
}

// TestRelated_DbcSnap_DBC_OrphanComplete_RDS verifies the same orphan scenario
// for rdstypes.DBClusterSnapshot RawStruct (the second branch in checkDbcSnapDBC).
//
// FAILS today: checkDbcSnapDBC returns Count=1 regardless of cache for both branches.
func TestRelated_DbcSnap_DBC_OrphanComplete_RDS(t *testing.T) {
	const ghostCluster = "ghost-rds-cluster"
	res := dbcSnapDBC_SnapshotWithRDSRaw(ghostCluster)
	cache := dbcSnapDBC_CompleteCacheWithoutCluster(ghostCluster)

	checker := dbcSnapCheckerByTarget(t, "dbc")
	result := checker(context.Background(), nil, res, cache)

	// FAILS today: checkDbcSnapDBC returns Count=1 unconditionally (rds branch).
	if result.Count != 0 {
		t.Errorf(
			"checkDbcSnapDBC (rds RawStruct): ghost cluster %q with complete cache: "+
				"Count = %d, want 0 — DBC-SNAP-NO-CACHE-CHECK BUG (RDS branch): orphan "+
				"dbc-snap reports cluster exists when it was deleted",
			ghostCluster, result.Count,
		)
	}
}

// TestRelated_DbcSnap_DBC_OrphanTruncated_DocDB verifies that when the dbc cache
// is truncated (IsTruncated=true) and the parent cluster is not in the visible
// window, the checker returns UnknownRelated (Count=-1) — the parent may be in
// a later page, so absence is non-definitive.
//
// FAILS today: checkDbcSnapDBC returns Count=1 regardless.
func TestRelated_DbcSnap_DBC_OrphanTruncated_DocDB(t *testing.T) {
	const ghostCluster = "ghost-cluster-trunc"
	res := dbcSnapDBC_SnapshotWithDocDBRaw(ghostCluster)
	cache := dbcSnapDBC_TruncatedCacheWithoutCluster(ghostCluster)

	checker := dbcSnapCheckerByTarget(t, "dbc")
	result := checker(context.Background(), nil, res, cache)

	// After fix: truncated cache + parent not found → UnknownRelated (Count=-1).
	// FAILS today: Count=1 (no cache scan).
	if result.Count != -1 {
		t.Errorf(
			"checkDbcSnapDBC (docdb RawStruct): ghost cluster %q with truncated cache: "+
				"Count = %d, want -1 (UnknownRelated) — DBC-SNAP-NO-CACHE-CHECK BUG: "+
				"parent may be in later page; answer must be unknown, not positive",
			ghostCluster, result.Count,
		)
	}
}

// TestRelated_DbcSnap_DBC_OrphanTruncated_RDS verifies the same scenario for
// rdstypes.DBClusterSnapshot RawStruct.
//
// FAILS today: Count=1 regardless.
func TestRelated_DbcSnap_DBC_OrphanTruncated_RDS(t *testing.T) {
	const ghostCluster = "ghost-rds-cluster-trunc"
	res := dbcSnapDBC_SnapshotWithRDSRaw(ghostCluster)
	cache := dbcSnapDBC_TruncatedCacheWithoutCluster(ghostCluster)

	checker := dbcSnapCheckerByTarget(t, "dbc")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != -1 {
		t.Errorf(
			"checkDbcSnapDBC (rds RawStruct): ghost cluster %q with truncated cache: "+
				"Count = %d, want -1 (UnknownRelated) — DBC-SNAP-NO-CACHE-CHECK BUG (RDS branch)",
			ghostCluster, result.Count,
		)
	}
}

// TestRelated_DbcSnap_DBC_PresentInCache_DocDB verifies that when the parent
// cluster IS in the dbc cache, the checker returns Count=1 with the matching ID.
//
// This should PASS today (the cluster ID is returned directly). It pins the
// correct behavior so the fix does not break the happy-path case.
func TestRelated_DbcSnap_DBC_PresentInCache_DocDB(t *testing.T) {
	const clusterID = "my-docdb-cluster"
	res := dbcSnapDBC_SnapshotWithDocDBRaw(clusterID)
	cache := dbcSnapDBC_CacheWithCluster(clusterID)

	checker := dbcSnapCheckerByTarget(t, "dbc")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("checkDbcSnapDBC (docdb RawStruct): cluster %q present in cache: Count = %d, want 1", clusterID, result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != clusterID {
		t.Errorf("checkDbcSnapDBC (docdb RawStruct): ResourceIDs = %v, want [%s]", result.ResourceIDs, clusterID)
	}
}

// TestRelated_DbcSnap_DBC_PresentInCache_RDS verifies the same happy-path for
// rdstypes.DBClusterSnapshot RawStruct.
//
// Should PASS today (cluster ID returned directly). Pins the happy-path.
func TestRelated_DbcSnap_DBC_PresentInCache_RDS(t *testing.T) {
	const clusterID = "my-aurora-cluster"
	res := dbcSnapDBC_SnapshotWithRDSRaw(clusterID)
	cache := dbcSnapDBC_CacheWithCluster(clusterID)

	checker := dbcSnapCheckerByTarget(t, "dbc")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("checkDbcSnapDBC (rds RawStruct): cluster %q present in cache: Count = %d, want 1", clusterID, result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != clusterID {
		t.Errorf("checkDbcSnapDBC (rds RawStruct): ResourceIDs = %v, want [%s]", result.ResourceIDs, clusterID)
	}
}
