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
