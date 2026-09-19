package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	backuptypes "github.com/aws/aws-sdk-go-v2/service/backup/types"
	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	_ "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
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

// The dbc-snap → backup pivot resolves the snapshot's parent cluster ARN via
// the dbc cache, then scans the backup plan cache for plans whose
// Fields["resources"] cover that ARN. It returns plan IDs, not recovery-point
// ARNs, so drill-through lands on the backup-plan list.

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

func TestRelated_DbcSnap_Backup_Match(t *testing.T) {
	res := dbcSnapBackupSrcResource()
	cache := dbcSnapBackupCache(dbcSnapTestClusterARN)

	checker := dbcSnapCheckerByTarget(t, "backup")
	result := checker(context.Background(), nil, res, cache)

	if result.Count() != 1 {
		t.Fatalf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "plan-aaa" {
		t.Errorf("ResourceIDs = %v, want [plan-aaa]", result.ResourceIDs())
	}
}

func TestRelated_DbcSnap_Backup_Empty(t *testing.T) {
	res := dbcSnapBackupSrcResource()
	cache := dbcSnapBackupCache("arn:aws:rds:us-east-1:123456789012:cluster:unrelated")

	checker := dbcSnapCheckerByTarget(t, "backup")
	result := checker(context.Background(), nil, res, cache)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no plan matches)", result.Count())
	}
}

func TestRelated_DbcSnap_Backup_NoParentReference(t *testing.T) {
	res := resource.Resource{
		ID: "snap-1",
		RawStruct: docdbtypes.DBClusterSnapshot{
			DBClusterSnapshotIdentifier: aws.String("snap-1"),
		},
	}
	checker := dbcSnapCheckerByTarget(t, "backup")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no parent reference)", result.Count())
	}
}

// Keeps the backuptypes import referenced.
var _ = backuptypes.RecoveryPointByResource{}

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

func dbcSnapDBC_CompleteCacheWithoutCluster(_ string) resource.ResourceCache {
	return resource.ResourceCache{
		"dbc": resource.ResourceCacheEntry{
			IsTruncated: false,
			Resources: []resource.Resource{
				{ID: "other-cluster", Name: "other-cluster"},
			},
		},
	}
}

func dbcSnapDBC_TruncatedCacheWithoutCluster(_ string) resource.ResourceCache {
	return resource.ResourceCache{
		"dbc": resource.ResourceCacheEntry{
			IsTruncated: true,
			Resources: []resource.Resource{
				{ID: "other-cluster", Name: "other-cluster"},
			},
		},
	}
}

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

func TestRelated_DbcSnap_DBC_OrphanComplete_DocDB(t *testing.T) {
	const ghostCluster = "ghost-cluster"
	res := dbcSnapDBC_SnapshotWithDocDBRaw(ghostCluster)
	cache := dbcSnapDBC_CompleteCacheWithoutCluster(ghostCluster)

	checker := dbcSnapCheckerByTarget(t, "dbc")
	result := checker(context.Background(), nil, res, cache)

	if result.Count() != 0 {
		t.Errorf(
			"checkDbcSnapDBC (docdb RawStruct): ghost cluster %q with complete cache: "+
				"Count = %d, want 0 — DBC-SNAP-NO-CACHE-CHECK BUG: orphan dbc-snap "+
				"reports cluster exists when it was deleted",
			ghostCluster, result.Count(),
		)
	}
}

func TestRelated_DbcSnap_DBC_OrphanComplete_RDS(t *testing.T) {
	const ghostCluster = "ghost-rds-cluster"
	res := dbcSnapDBC_SnapshotWithRDSRaw(ghostCluster)
	cache := dbcSnapDBC_CompleteCacheWithoutCluster(ghostCluster)

	checker := dbcSnapCheckerByTarget(t, "dbc")
	result := checker(context.Background(), nil, res, cache)

	if result.Count() != 0 {
		t.Errorf(
			"checkDbcSnapDBC (rds RawStruct): ghost cluster %q with complete cache: "+
				"Count = %d, want 0 — DBC-SNAP-NO-CACHE-CHECK BUG (RDS branch): orphan "+
				"dbc-snap reports cluster exists when it was deleted",
			ghostCluster, result.Count(),
		)
	}
}

// The dbc list is the target list, so a truncated page that did not carry the
// parent is a resolved zero carrying the truncation flag, rendered "(0+)".
// Unknown is reserved for a list that was never read.
func TestRelated_DbcSnap_DBC_OrphanTruncated_DocDB(t *testing.T) {
	const ghostCluster = "ghost-cluster-trunc"
	res := dbcSnapDBC_SnapshotWithDocDBRaw(ghostCluster)
	cache := dbcSnapDBC_TruncatedCacheWithoutCluster(ghostCluster)

	checker := dbcSnapCheckerByTarget(t, "dbc")
	result := checker(context.Background(), nil, res, cache)

	assertDbcSnapTruncatedZero(t, "docdb RawStruct", ghostCluster, result)
}

func TestRelated_DbcSnap_DBC_OrphanTruncated_RDS(t *testing.T) {
	const ghostCluster = "ghost-rds-cluster-trunc"
	res := dbcSnapDBC_SnapshotWithRDSRaw(ghostCluster)
	cache := dbcSnapDBC_TruncatedCacheWithoutCluster(ghostCluster)

	checker := dbcSnapCheckerByTarget(t, "dbc")
	result := checker(context.Background(), nil, res, cache)

	assertDbcSnapTruncatedZero(t, "rds RawStruct", ghostCluster, result)
}

func TestRelated_DbcSnap_DBC_PresentInCache_DocDB(t *testing.T) {
	const clusterID = "my-docdb-cluster"
	res := dbcSnapDBC_SnapshotWithDocDBRaw(clusterID)
	cache := dbcSnapDBC_CacheWithCluster(clusterID)

	checker := dbcSnapCheckerByTarget(t, "dbc")
	result := checker(context.Background(), nil, res, cache)

	if result.Count() != 1 {
		t.Errorf("checkDbcSnapDBC (docdb RawStruct): cluster %q present in cache: Count = %d, want 1", clusterID, result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != clusterID {
		t.Errorf("checkDbcSnapDBC (docdb RawStruct): ResourceIDs = %v, want [%s]", result.ResourceIDs(), clusterID)
	}
}

func TestRelated_DbcSnap_DBC_PresentInCache_RDS(t *testing.T) {
	const clusterID = "my-aurora-cluster"
	res := dbcSnapDBC_SnapshotWithRDSRaw(clusterID)
	cache := dbcSnapDBC_CacheWithCluster(clusterID)

	checker := dbcSnapCheckerByTarget(t, "dbc")
	result := checker(context.Background(), nil, res, cache)

	if result.Count() != 1 {
		t.Errorf("checkDbcSnapDBC (rds RawStruct): cluster %q present in cache: Count = %d, want 1", clusterID, result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != clusterID {
		t.Errorf("checkDbcSnapDBC (rds RawStruct): ResourceIDs = %v, want [%s]", result.ResourceIDs(), clusterID)
	}
}

// On a truncated page the list was read, so the zero is real so far, and the
// truncation flag says a later page may still carry the cluster. The
// snapshot's own cluster id never becomes the count.
func assertDbcSnapTruncatedZero(t *testing.T, shape, ghostCluster string, result domain.RelatedCheckResult) {
	t.Helper()
	if result.State() != domain.RelatedResolved {
		t.Errorf("checkDbcSnapDBC (%s): ghost cluster %q on a truncated page: state = %v, want Resolved",
			shape, ghostCluster, result.State())
	}
	if result.Count() != 0 {
		t.Errorf("checkDbcSnapDBC (%s): ghost cluster %q: Count = %d, want 0 — the id came from the snapshot, not the list",
			shape, ghostCluster, result.Count())
	}
	if !result.Truncated() {
		t.Errorf("checkDbcSnapDBC (%s): Truncated = false, want true — a later page may carry the cluster", shape)
	}
}

// When the dbc list was never read, the snapshot's own DBClusterIdentifier is
// a field of the source resource, not an answer from the target list, so the
// panel reports unknown.
func TestRelated_DbcSnap_DBC_ColdCacheIsUnknown(t *testing.T) {
	for _, tc := range []struct {
		shape string
		res   resource.Resource
	}{
		{"docdb", dbcSnapDBC_SnapshotWithDocDBRaw("my-docdb-cluster")},
		{"rds", dbcSnapDBC_SnapshotWithRDSRaw("my-aurora-cluster")},
	} {
		t.Run(tc.shape, func(t *testing.T) {
			checker := dbcSnapCheckerByTarget(t, "dbc")
			result := checker(context.Background(), nil, tc.res, resource.ResourceCache{})

			if result.State() != domain.RelatedUnknown {
				t.Errorf("checkDbcSnapDBC (%s): cold cache: state = %v (Count=%d, IDs=%v), want Unknown — nothing read the dbc list",
					tc.shape, result.State(), result.Count(), result.ResourceIDs())
			}
		})
	}
}

// A parent confirmed on a truncated page is a lower bound: a page nobody read
// may carry another cluster of the same name.
func TestRelated_DbcSnap_DBC_MatchOnTruncatedPageIsALowerBound(t *testing.T) {
	for _, tc := range []struct {
		shape     string
		clusterID string
		res       resource.Resource
	}{
		{"docdb", "my-docdb-cluster", dbcSnapDBC_SnapshotWithDocDBRaw("my-docdb-cluster")},
		{"rds", "my-aurora-cluster", dbcSnapDBC_SnapshotWithRDSRaw("my-aurora-cluster")},
	} {
		t.Run(tc.shape, func(t *testing.T) {
			cache := resource.ResourceCache{
				"dbc": resource.ResourceCacheEntry{
					IsTruncated: true,
					Resources: []resource.Resource{
						{ID: "other-cluster", Name: "other-cluster"},
						{ID: tc.clusterID, Name: tc.clusterID},
					},
				},
			}

			checker := dbcSnapCheckerByTarget(t, "dbc")
			result := checker(context.Background(), nil, tc.res, cache)

			if result.State() != domain.RelatedResolved {
				t.Fatalf("checkDbcSnapDBC (%s): state = %v, want Resolved", tc.shape, result.State())
			}
			if result.Count() != 1 || result.ResourceIDs()[0] != tc.clusterID {
				t.Errorf("checkDbcSnapDBC (%s): ResourceIDs = %v, want [%s]", tc.shape, result.ResourceIDs(), tc.clusterID)
			}
			if !result.Truncated() {
				t.Errorf("checkDbcSnapDBC (%s): Truncated = false, want true — the parent was confirmed on a page that was cut short", tc.shape)
			}
		})
	}
}
