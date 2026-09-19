package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	_ "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

func dbiSnapCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("dbi-snap") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("dbi-snap related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("dbi-snap related checker for %s not found", target)
	return nil
}

func TestNavigableFields_DBISnap_Registered(t *testing.T) {
	nav := resource.IsFieldNavigableForTest("dbi-snap", "DBInstanceIdentifier")
	if nav == nil {
		t.Error("expected navigable field DBInstanceIdentifier for dbi-snap, got nil")
	} else if nav.TargetType != "dbi" {
		t.Errorf("DBInstanceIdentifier TargetType = %q, want %q", nav.TargetType, "dbi")
	}
}

func TestNavigableFields_DBISnap_FieldPathsResolve(t *testing.T) {
	fields := resource.GetNavigableFields("dbi-snap")
	if len(fields) == 0 {
		t.Fatal("no navigable fields registered for dbi-snap")
	}

	found := false
	for _, f := range fields {
		if f.FieldPath == "DBInstanceIdentifier" && f.TargetType == "dbi" {
			found = true
		}
	}
	if !found {
		t.Error("navigable field DBInstanceIdentifier → dbi not registered for dbi-snap")
	}
}

func TestRelated_DBISnap_DBI_Found(t *testing.T) {
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

	checker := dbiSnapCheckerByTarget(t, "dbi")
	result := checker(context.Background(), nil, source, cache)

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "mydb" {
		t.Errorf("ResourceIDs = %v, want [mydb]", result.ResourceIDs())
	}
	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
	}
}

func TestRelated_DBISnap_DBI_NotFound(t *testing.T) {
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

	checker := dbiSnapCheckerByTarget(t, "dbi")
	result := checker(context.Background(), nil, source, cache)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0", result.Count())
	}
	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
	}
}

func TestRelated_DBISnap_DBI_CacheMissNoClients(t *testing.T) {
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

	checker := dbiSnapCheckerByTarget(t, "dbi")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (unknown/cache miss)", result.Count())
	}
}

func TestRelated_DBISnap_KMS_Found(t *testing.T) {
	const keyID = "d4e5f6a7-8901-23de-fghi-444444444444"
	arn := "arn:aws:kms:us-east-1:123456789012:key/" + keyID

	kmsRes := resource.Resource{
		ID:   keyID,
		Name: "alias/dbi-snap-key",
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

	checker := dbiSnapCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, source, cache)

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != keyID {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs(), keyID)
	}
	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
	}
}

// A key the resource names by ID is counted by that ID whether or not the kms
// list holds it — AWS-managed keys never are — and the related drill
// lazy-adds it.
func TestRelated_DBISnap_KMS_NotFound(t *testing.T) {
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

	checker := dbiSnapCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, source, cache)

	if ids := result.ResourceIDs(); len(ids) != 1 || ids[0] != keyID {
		t.Errorf("ResourceIDs = %v, want [%s]", ids, keyID)
	}
}

// A key the resource names by ID is counted by that ID whether or not the kms
// list holds it — AWS-managed keys never are — and the related drill
// lazy-adds it.
func TestRelated_DBISnap_KMS_CacheMissNoClients(t *testing.T) {
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

	checker := dbiSnapCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if ids := result.ResourceIDs(); result.State() != domain.RelatedResolved || len(ids) != 1 || ids[0] != keyID {
		t.Errorf("State = %v, IDs = %v, want resolved [%s] (the key ARN names it without the list)", result.State(), ids, keyID)
	}
}

// AWS Backup tracks the parent DB instance, not individual snapshots, so the
// checker resolves snap.DBInstanceIdentifier through the dbi cache to its
// DBInstanceArn and reverse-scans plan selections for that ARN. It returns plan
// IDs because the backup target's Resource.ID space is plan IDs.
func TestRelated_DBISnap_Backup_Match(t *testing.T) {
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

	checker := dbiSnapCheckerByTarget(t, "backup")
	result := checker(context.Background(), nil, src, cache)

	if result.Count() != 2 {
		t.Errorf("Count = %d, want 2 (two plans cover the snapshot's parent DB)", result.Count())
	}
	if len(result.ResourceIDs()) != 2 {
		t.Errorf("ResourceIDs = %v, want 2 plan IDs", result.ResourceIDs())
	}
	for _, id := range result.ResourceIDs() {
		if id == "plan-other-target" {
			t.Errorf("ResourceIDs unexpectedly contains plan-other-target (its Resources do not match the parent DB ARN)")
		}
	}
}

// An orphan snapshot has no parent ARN to match against plan selections.
func TestRelated_DBISnap_Backup_NoParentInDbi(t *testing.T) {
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
	checker := dbiSnapCheckerByTarget(t, "backup")
	result := checker(context.Background(), nil, src, cache)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (orphan snapshot — parent gone)", result.Count())
	}
}

// Without the parent DB ARN no plan selection can be matched, so the answer
// is unknown.
func TestRelated_DBISnap_Backup_NoDbiCacheLoaded(t *testing.T) {
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
	checker := dbiSnapCheckerByTarget(t, "backup")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (UnknownRelated when dbi cache not loaded)", result.Count())
	}
}

func TestRelated_DBISnap_Backup_NoPlansLoaded(t *testing.T) {
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
	}
	checker := dbiSnapCheckerByTarget(t, "backup")
	result := checker(context.Background(), nil, src, cache)

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (UnknownRelated when backup cache not loaded)", result.Count())
	}
}

func TestRelated_DBISnap_Backup_NoParentReference(t *testing.T) {
	src := resource.Resource{
		ID:     "rds:orphan-no-parent-ref",
		Name:   "rds:orphan-no-parent-ref",
		Fields: map[string]string{},
		RawStruct: rdstypes.DBSnapshot{
			DBSnapshotIdentifier: aws.String("rds:orphan-no-parent-ref"),
		},
	}
	checker := dbiSnapCheckerByTarget(t, "backup")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no parent reference)", result.Count())
	}
}
