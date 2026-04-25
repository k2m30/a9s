package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func rdsSnapCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("rds-snap") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("rds-snap related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("rds-snap related checker for %s not found", target)
	return nil
}

// --- Navigable Fields ---

func TestNavigableFields_RDSSnap_Registered(t *testing.T) {
	nav := resource.IsFieldNavigable("rds-snap", "DBInstanceIdentifier")
	if nav == nil {
		t.Error("expected navigable field DBInstanceIdentifier for rds-snap, got nil")
	} else if nav.TargetType != "dbi" {
		t.Errorf("DBInstanceIdentifier TargetType = %q, want %q", nav.TargetType, "dbi")
	}
}

func TestNavigableFields_RDSSnap_FieldPathsResolve(t *testing.T) {
	fields := resource.GetNavigableFields("rds-snap")
	if len(fields) == 0 {
		t.Fatal("no navigable fields registered for rds-snap")
	}

	// DBInstanceIdentifier must resolve to dbi.
	found := false
	for _, f := range fields {
		if f.FieldPath == "DBInstanceIdentifier" && f.TargetType == "dbi" {
			found = true
		}
	}
	if !found {
		t.Error("navigable field DBInstanceIdentifier → dbi not registered for rds-snap")
	}
}

// --- DBI checker (Pattern C — cache-based, matches DBInstanceIdentifier) ---

func TestRelated_RDSSnap_DBI_Found(t *testing.T) {
	dbiRes := resource.Resource{
		ID:   "mydb",
		Name: "mydb",
		RawStruct: rdstypes.DBInstance{
			DBInstanceIdentifier: aws.String("mydb"),
		},
	}
	cache := resource.ResourceCache{
		"dbi": resource.ResourceCacheEntry{Resources: []resource.Resource{dbiRes}},
	}
	source := resource.Resource{
		ID:   "rds:mydb:2025-01-15-03-00",
		Name: "rds:mydb:2025-01-15-03-00",
		Fields: map[string]string{
			"db_instance_identifier": "mydb",
		},
		RawStruct: rdstypes.DBSnapshot{
			DBSnapshotIdentifier: aws.String("rds:mydb:2025-01-15-03-00"),
			DBInstanceIdentifier: aws.String("mydb"),
		},
	}

	checker := rdsSnapCheckerByTarget(t, "dbi")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "mydb" {
		t.Errorf("ResourceIDs = %v, want [mydb]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_RDSSnap_DBI_NotFound(t *testing.T) {
	dbiRes := resource.Resource{
		ID:   "otherdb",
		Name: "otherdb",
		RawStruct: rdstypes.DBInstance{
			DBInstanceIdentifier: aws.String("otherdb"),
		},
	}
	cache := resource.ResourceCache{
		"dbi": resource.ResourceCacheEntry{Resources: []resource.Resource{dbiRes}},
	}
	source := resource.Resource{
		ID:   "rds:mydb:2025-01-15-03-00",
		Name: "rds:mydb:2025-01-15-03-00",
		Fields: map[string]string{
			"db_instance_identifier": "mydb",
		},
		RawStruct: rdstypes.DBSnapshot{
			DBSnapshotIdentifier: aws.String("rds:mydb:2025-01-15-03-00"),
			DBInstanceIdentifier: aws.String("mydb"),
		},
	}

	checker := rdsSnapCheckerByTarget(t, "dbi")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_RDSSnap_DBI_CacheMissNoClients(t *testing.T) {
	source := resource.Resource{
		ID:   "rds:mydb:2025-01-15-03-00",
		Name: "rds:mydb:2025-01-15-03-00",
		Fields: map[string]string{
			"db_instance_identifier": "mydb",
		},
		RawStruct: rdstypes.DBSnapshot{
			DBSnapshotIdentifier: aws.String("rds:mydb:2025-01-15-03-00"),
			DBInstanceIdentifier: aws.String("mydb"),
		},
	}

	checker := rdsSnapCheckerByTarget(t, "dbi")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown/cache miss)", result.Count)
	}
}

// --- KMS checker (Pattern C — cache-based, KmsKeyId ARN suffix) ---

func TestRelated_RDSSnap_KMS_Found(t *testing.T) {
	const keyID = "d4e5f6a7-8901-23de-fghi-444444444444"
	arn := "arn:aws:kms:us-east-1:123456789012:key/" + keyID

	kmsRes := resource.Resource{
		ID:   keyID,
		Name: "alias/rds-snap-key",
		Fields: map[string]string{
			"key_id": keyID,
		},
	}
	cache := resource.ResourceCache{
		"kms": resource.ResourceCacheEntry{Resources: []resource.Resource{kmsRes}},
	}
	source := resource.Resource{
		ID:   "rds:mydb:2025-01-15-03-00",
		Name: "rds:mydb:2025-01-15-03-00",
		Fields: map[string]string{
			"kms_key_id": arn,
		},
		RawStruct: rdstypes.DBSnapshot{
			DBSnapshotIdentifier: aws.String("rds:mydb:2025-01-15-03-00"),
			DBInstanceIdentifier: aws.String("mydb"),
			KmsKeyId:             aws.String(arn),
		},
	}

	checker := rdsSnapCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != keyID {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, keyID)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_RDSSnap_KMS_NotFound(t *testing.T) {
	const keyID = "d4e5f6a7-8901-23de-fghi-444444444444"

	kmsRes := resource.Resource{
		ID:   "different-key-id",
		Name: "alias/other-key",
		Fields: map[string]string{
			"key_id": "different-key-id",
		},
	}
	cache := resource.ResourceCache{
		"kms": resource.ResourceCacheEntry{Resources: []resource.Resource{kmsRes}},
	}
	source := resource.Resource{
		ID:   "rds:mydb:2025-01-15-03-00",
		Name: "rds:mydb:2025-01-15-03-00",
		Fields: map[string]string{
			"kms_key_id": "arn:aws:kms:us-east-1:123456789012:key/" + keyID,
		},
		RawStruct: rdstypes.DBSnapshot{
			DBSnapshotIdentifier: aws.String("rds:mydb:2025-01-15-03-00"),
			DBInstanceIdentifier: aws.String("mydb"),
			KmsKeyId:             aws.String("arn:aws:kms:us-east-1:123456789012:key/" + keyID),
		},
	}

	checker := rdsSnapCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_RDSSnap_KMS_CacheMissNoClients(t *testing.T) {
	const keyID = "d4e5f6a7-8901-23de-fghi-444444444444"
	arn := "arn:aws:kms:us-east-1:123456789012:key/" + keyID

	source := resource.Resource{
		ID:   "rds:mydb:2025-01-15-03-00",
		Name: "rds:mydb:2025-01-15-03-00",
		Fields: map[string]string{
			"kms_key_id": arn,
		},
		RawStruct: rdstypes.DBSnapshot{
			DBSnapshotIdentifier: aws.String("rds:mydb:2025-01-15-03-00"),
			DBInstanceIdentifier: aws.String("mydb"),
			KmsKeyId:             aws.String(arn),
		},
	}

	checker := rdsSnapCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown/cache miss)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkRDSSnapBackup — Pattern C: cache scan of backup PLAN list,
// matching snapshot ARN against each plan's Fields["resources"] / ["not_resources"].
// ---------------------------------------------------------------------------

// TestRelated_RDSSnap_Backup_Match verifies that the checker returns plan IDs
// (not recovery-point ARNs) when the loaded backup PLAN cache contains plans
// whose Resources include the snapshot's PARENT DB ARN. AWS Backup tracks the
// parent DB instance, not individual snapshots, so the checker resolves
// snap.DBInstanceIdentifier through the dbi cache to get DBInstanceArn, then
// reverse-scans plan selections for that parent ARN. Drill-through requires
// plan IDs because the backup target's Resource.ID space is plan IDs.
func TestRelated_RDSSnap_Backup_Match(t *testing.T) {
	const parentDBName = "mydb"
	const parentDBARN = "arn:aws:rds:us-east-1:123456789012:db:mydb"

	src := resource.Resource{
		ID:   "rds:mydb-2025-01-15-03-00",
		Name: "rds:mydb-2025-01-15-03-00",
		Fields: map[string]string{
			"arn": "arn:aws:rds:us-east-1:123456789012:snapshot:rds:mydb-2025-01-15-03-00",
		},
		RawStruct: rdstypes.DBSnapshot{
			DBSnapshotIdentifier: aws.String("rds:mydb-2025-01-15-03-00"),
			DBInstanceIdentifier: aws.String(parentDBName),
		},
	}
	cache := resource.ResourceCache{
		// dbi cache resolves the parent DB → ARN.
		"dbi": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{
					ID:   parentDBName,
					Name: parentDBName,
					RawStruct: rdstypes.DBInstance{
						DBInstanceIdentifier: aws.String(parentDBName),
						DBInstanceArn:        aws.String(parentDBARN),
					},
				},
			},
		},
		// backup plan cache — Resources lists the parent DB ARN (real
		// AWS Backup behaviour), NOT the snapshot ARN.
		"backup": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{
					ID:   "plan-covers-parent-A",
					Name: "covers-parent-A",
					Fields: map[string]string{
						"resources":     parentDBARN,
						"not_resources": "",
					},
				},
				{
					ID:   "plan-covers-parent-B",
					Name: "covers-parent-B",
					Fields: map[string]string{
						"resources":     parentDBARN,
						"not_resources": "",
					},
				},
				{
					ID:   "plan-other-target",
					Name: "other",
					Fields: map[string]string{
						"resources":     "arn:aws:s3:::unrelated",
						"not_resources": "",
					},
				},
			},
		},
	}

	checker := rdsSnapCheckerByTarget(t, "backup")
	result := checker(context.Background(), nil, src, cache)

	if result.Count != 2 {
		t.Errorf("Count = %d, want 2 (two plans cover the snapshot's parent DB)", result.Count)
	}
	if len(result.ResourceIDs) != 2 {
		t.Errorf("ResourceIDs = %v, want 2 plan IDs", result.ResourceIDs)
	}
	for _, id := range result.ResourceIDs {
		if id == "plan-other-target" {
			t.Errorf("ResourceIDs unexpectedly contains plan-other-target (its Resources do not match the parent DB ARN)")
		}
	}
}

// TestRelated_RDSSnap_Backup_NoParentInDbi verifies that an orphan snapshot
// (parent not in the loaded dbi cache) returns Count=0 — there's no parent ARN
// to match against plan selections, and AWS Backup would never have covered a
// snapshot whose parent is gone.
func TestRelated_RDSSnap_Backup_NoParentInDbi(t *testing.T) {
	src := resource.Resource{
		ID:   "rds:mydb-2025-01-15-03-00",
		Name: "rds:mydb-2025-01-15-03-00",
		Fields: map[string]string{
			"arn": "arn:aws:rds:us-east-1:123456789012:snapshot:rds:mydb-2025-01-15-03-00",
		},
		RawStruct: rdstypes.DBSnapshot{
			DBSnapshotIdentifier: aws.String("rds:mydb-2025-01-15-03-00"),
			DBInstanceIdentifier: aws.String("orphan-parent"),
		},
	}
	cache := resource.ResourceCache{
		"dbi": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{
					ID:   "other-db",
					Name: "other-db",
					RawStruct: rdstypes.DBInstance{
						DBInstanceIdentifier: aws.String("other-db"),
						DBInstanceArn:        aws.String("arn:aws:rds:us-east-1:123456789012:db:other-db"),
					},
				},
			},
		},
		"backup": resource.ResourceCacheEntry{Resources: []resource.Resource{}},
	}
	checker := rdsSnapCheckerByTarget(t, "backup")
	result := checker(context.Background(), nil, src, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (orphan snapshot — parent gone)", result.Count)
	}
}

// TestRelated_RDSSnap_Backup_NoDbiCacheLoaded verifies that the checker returns
// UnknownRelated (Count=-1) when the dbi cache hasn't been loaded yet — without
// the parent DB ARN we can't say whether any plan covers the snapshot.
func TestRelated_RDSSnap_Backup_NoDbiCacheLoaded(t *testing.T) {
	src := resource.Resource{
		ID:   "rds:mydb-2025-01-15-03-00",
		Name: "rds:mydb-2025-01-15-03-00",
		Fields: map[string]string{
			"arn": "arn:aws:rds:us-east-1:123456789012:snapshot:rds:mydb-2025-01-15-03-00",
		},
		RawStruct: rdstypes.DBSnapshot{
			DBSnapshotIdentifier: aws.String("rds:mydb-2025-01-15-03-00"),
			DBInstanceIdentifier: aws.String("mydb"),
		},
	}
	checker := rdsSnapCheckerByTarget(t, "backup")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (UnknownRelated when dbi cache not loaded)", result.Count)
	}
}

// TestRelated_RDSSnap_Backup_NoPlansLoaded verifies that the checker returns
// UnknownRelated (Count=-1) when the dbi cache resolves the parent ARN but the
// backup PLAN cache hasn't been loaded yet.
func TestRelated_RDSSnap_Backup_NoPlansLoaded(t *testing.T) {
	const parentDBName = "mydb"
	src := resource.Resource{
		ID:   "rds:mydb-2025-01-15-03-00",
		Name: "rds:mydb-2025-01-15-03-00",
		Fields: map[string]string{
			"arn": "arn:aws:rds:us-east-1:123456789012:snapshot:rds:mydb-2025-01-15-03-00",
		},
		RawStruct: rdstypes.DBSnapshot{
			DBSnapshotIdentifier: aws.String("rds:mydb-2025-01-15-03-00"),
			DBInstanceIdentifier: aws.String(parentDBName),
		},
	}
	cache := resource.ResourceCache{
		"dbi": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{
					ID:   parentDBName,
					Name: parentDBName,
					RawStruct: rdstypes.DBInstance{
						DBInstanceIdentifier: aws.String(parentDBName),
						DBInstanceArn:        aws.String("arn:aws:rds:us-east-1:123456789012:db:" + parentDBName),
					},
				},
			},
		},
		// backup cache absent — checker must report UnknownRelated.
	}
	checker := rdsSnapCheckerByTarget(t, "backup")
	result := checker(context.Background(), nil, src, cache)

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (UnknownRelated when backup cache not loaded)", result.Count)
	}
}

// TestRelated_RDSSnap_Backup_NoParentReference verifies a snapshot with no
// DBInstanceIdentifier returns Count=0 (cannot pivot without a parent reference).
func TestRelated_RDSSnap_Backup_NoParentReference(t *testing.T) {
	src := resource.Resource{
		ID:     "rds:orphan-no-parent-ref",
		Name:   "rds:orphan-no-parent-ref",
		Fields: map[string]string{},
		RawStruct: rdstypes.DBSnapshot{
			DBSnapshotIdentifier: aws.String("rds:orphan-no-parent-ref"),
			// no DBInstanceIdentifier
		},
	}
	checker := rdsSnapCheckerByTarget(t, "backup")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no parent reference)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkRDSSnapDBC — two-hop: snap→dbi→dbc
// ---------------------------------------------------------------------------

// TestRelated_RDSSnap_DBC_InvalidRawStruct verifies Count=-1 when the
// RawStruct is not a DBSnapshot.
func TestRelated_RDSSnap_DBC_InvalidRawStruct(t *testing.T) {
	src := resource.Resource{ID: "rds:mydb-snap", RawStruct: "not-a-snapshot"}
	checker := rdsSnapCheckerByTarget(t, "dbc")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (bad raw struct)", result.Count)
	}
}

// TestRelated_RDSSnap_DBC_NoInstanceIDReturnsZero verifies Count=0 when the
// snapshot has no DBInstanceIdentifier (manual/shared snapshot).
func TestRelated_RDSSnap_DBC_NoInstanceIDReturnsZero(t *testing.T) {
	src := resource.Resource{
		ID:        "rds:mydb-snap",
		RawStruct: rdstypes.DBSnapshot{DBSnapshotIdentifier: aws.String("rds:mydb-snap")},
	}
	checker := rdsSnapCheckerByTarget(t, "dbc")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no instance ID)", result.Count)
	}
}

// TestRelated_RDSSnap_DBC_StandaloneInstanceReturnsZero verifies Count=0 when
// the DBI cache contains the source instance but it has no DBClusterIdentifier
// (standalone instance, not part of an Aurora cluster).
func TestRelated_RDSSnap_DBC_StandaloneInstanceReturnsZero(t *testing.T) {
	dbiRes := resource.Resource{
		ID:   "mydb",
		Name: "mydb",
		RawStruct: rdstypes.DBInstance{
			DBInstanceIdentifier: aws.String("mydb"),
			// DBClusterIdentifier intentionally nil — standalone instance.
		},
	}
	cache := resource.ResourceCache{
		"dbi": resource.ResourceCacheEntry{Resources: []resource.Resource{dbiRes}},
	}
	src := resource.Resource{
		ID: "rds:mydb-snap",
		RawStruct: rdstypes.DBSnapshot{
			DBInstanceIdentifier: aws.String("mydb"),
		},
	}
	checker := rdsSnapCheckerByTarget(t, "dbc")
	result := checker(context.Background(), nil, src, cache)
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (standalone instance, no cluster)", result.Count)
	}
}

// TestRelated_RDSSnap_DBC_ClusterInstanceFoundViaCache verifies Count=1 when
// the DBI cache resolves the instance's DBClusterIdentifier.
func TestRelated_RDSSnap_DBC_ClusterInstanceFoundViaCache(t *testing.T) {
	const clusterID = "acme-aurora-cluster"
	dbiRes := resource.Resource{
		ID:   "mydb",
		Name: "mydb",
		RawStruct: rdstypes.DBInstance{
			DBInstanceIdentifier: aws.String("mydb"),
			DBClusterIdentifier:  aws.String(clusterID),
		},
	}
	cache := resource.ResourceCache{
		"dbi": resource.ResourceCacheEntry{Resources: []resource.Resource{dbiRes}},
	}
	src := resource.Resource{
		ID: "rds:mydb-snap",
		RawStruct: rdstypes.DBSnapshot{
			DBInstanceIdentifier: aws.String("mydb"),
		},
	}
	checker := rdsSnapCheckerByTarget(t, "dbc")
	result := checker(context.Background(), nil, src, cache)
	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != clusterID {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, clusterID)
	}
}

// TestRelated_RDSSnap_DBC_CacheMissNoClients verifies Count=-1 when the
// DBI cache is empty and no clients are available to fetch it.
func TestRelated_RDSSnap_DBC_CacheMissNoClients(t *testing.T) {
	src := resource.Resource{
		ID: "rds:mydb-snap",
		RawStruct: rdstypes.DBSnapshot{
			DBInstanceIdentifier: aws.String("mydb"),
		},
	}
	checker := rdsSnapCheckerByTarget(t, "dbc")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (cache miss, no clients)", result.Count)
	}
}
