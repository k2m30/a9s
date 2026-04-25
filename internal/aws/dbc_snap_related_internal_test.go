package aws

// dbc_snap_related_internal_test.go — internal package tests for checkDbcSnapBackup.
//
// checkDbcSnapBackup is unexported, so these tests must live in the internal/aws
// package. Two cases pin the truncated-dbc-cache contract:
//
//  1. TruncatedDBCCacheReturnsUnknown: dbc cache is truncated AND the parent
//     cluster is not in the visible window → UnknownRelated("backup") (Count=-1).
//     This is the bug the coder is fixing: current code returns Count=0.
//
//  2. TruncatedDBCCacheButParentResolved: dbc cache is truncated BUT the parent
//     cluster IS in the visible window with a populated DBClusterArn → the
//     backup scan proceeds normally (not Unknown).

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// buildDBCSnapBackupResource builds a resource.Resource wrapping a
// docdbtypes.DBClusterSnapshot with the given parent cluster identifier.
// Used by both backup-related tests below.
func buildDBCSnapBackupResource(snapshotID, parentClusterID string) resource.Resource {
	snap := docdbtypes.DBClusterSnapshot{
		DBClusterSnapshotIdentifier: aws.String(snapshotID),
		DBClusterIdentifier:         aws.String(parentClusterID),
		Status:                      aws.String("available"),
		SnapshotType:                aws.String("manual"),
	}
	return resource.Resource{
		ID:        snapshotID,
		Name:      snapshotID,
		Fields:    map[string]string{},
		RawStruct: snap,
	}
}

// TestCheckDbcSnapBackup_TruncatedDBCCacheReturnsUnknown verifies that when
// the dbc cache is truncated and the parent cluster ("missing-cluster") is NOT
// in the visible window, checkDbcSnapBackup returns UnknownRelated("backup")
// (Count=-1, TargetType="backup") rather than Count=0.
//
// Rationale: the parent cluster may exist in the unseen portion of the
// truncated dbc list. Returning Count=0 would be a false "no backup coverage"
// signal. UnknownRelated("backup") renders as "?" in the related panel,
// which is the correct UX for "we don't know".
//
// This test FAILS until the coder's fix to checkDbcSnapBackup is shipped.
func TestCheckDbcSnapBackup_TruncatedDBCCacheReturnsUnknown(t *testing.T) {
	res := buildDBCSnapBackupResource("snap-missing-parent", "missing-cluster")

	// dbc cache is truncated; "missing-cluster" is NOT in the visible window.
	otherCluster := docdbtypes.DBCluster{
		DBClusterIdentifier: aws.String("other-cluster"),
		DBClusterArn:        aws.String("arn:aws:rds:us-east-1:123456789012:cluster:other-cluster"),
	}
	cache := resource.ResourceCache{
		"dbc": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{
					ID:        "other-cluster",
					Name:      "other-cluster",
					RawStruct: otherCluster,
				},
			},
			IsTruncated: true, // <-- truncated: parent might exist beyond the page
		},
		"backup": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{
					ID:     "plan-aaa",
					Name:   "plan-aaa",
					Fields: map[string]string{"resources": "arn:aws:rds:us-east-1:123456789012:cluster:some-cluster"},
				},
			},
			IsTruncated: false,
		},
	}

	result := checkDbcSnapBackup(context.Background(), nil, res, cache)

	// Must return UnknownRelated shape: Count=-1, TargetType="backup".
	// This is the key assertion — current (buggy) code returns Count=0.
	if result.TargetType != "backup" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "backup")
	}
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (UnknownRelated — dbc truncated, parent not in visible window)\n"+
			"NOTE: this test fails until the coder's fix to checkDbcSnapBackup is shipped",
			result.Count)
	}
}

// TestCheckDbcSnapBackup_TruncatedDBCCacheButParentResolved verifies that when
// the dbc cache is truncated BUT the parent cluster IS in the visible window
// with a populated DBClusterArn, the backup scan proceeds normally.
//
// The truncated flag alone must NOT force Unknown — only when the parent
// cannot be resolved (ARN remains empty) should we return UnknownRelated.
func TestCheckDbcSnapBackup_TruncatedDBCCacheButParentResolved(t *testing.T) {
	const parentID = "prod-cluster"
	const parentARN = "arn:aws:rds:us-east-1:123456789012:cluster:prod-cluster"
	const planID = "plan-prod-coverage"

	res := buildDBCSnapBackupResource("snap-prod", parentID)

	// dbc cache is truncated BUT the parent IS present with its ARN.
	parentCluster := docdbtypes.DBCluster{
		DBClusterIdentifier: aws.String(parentID),
		DBClusterArn:        aws.String(parentARN),
	}
	cache := resource.ResourceCache{
		"dbc": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{
					ID:        parentID,
					Name:      parentID,
					RawStruct: parentCluster,
				},
			},
			IsTruncated: true, // truncated, but parent is visible
		},
		"backup": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{
					ID:     planID,
					Name:   planID,
					Fields: map[string]string{"resources": parentARN},
				},
			},
			IsTruncated: false,
		},
	}

	result := checkDbcSnapBackup(context.Background(), nil, res, cache)

	// Parent was found in the visible window — backup scan must proceed.
	// The plan covers the parent ARN → Count=1.
	if result.TargetType != "backup" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "backup")
	}
	if result.Count == -1 {
		t.Errorf("Count = -1 (Unknown), but parent was resolved — should scan backup plans normally")
	}
	// Verify the plan was found (Count should be 1).
	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (plan %q covers parent ARN)", result.Count, planID)
	}
}

// TestDbcSnapHelpers_DualShape pins the dual-shape dispatch in checkDbcSnapDBC,
// checkDbcSnapKMS, checkDbcSnapVPC, dbcSnapParentRefs, and dbcResourceARN
// for both docdbtypes and rdstypes inputs.
func TestDbcSnapHelpers_DualShape(t *testing.T) {
	const kmsARN = "arn:aws:kms:us-east-1:123456789012:key/abcdef12-0000-0000-0000-000000000000"
	const kmsUUID = "abcdef12-0000-0000-0000-000000000000"
	const vpcID = "vpc-0abc1234"
	const parentID = "prod-cluster"
	const parentARN = "arn:aws:rds:us-east-1:123456789012:cluster:prod-cluster"
	emptyCache := resource.ResourceCache{}

	// --- checkDbcSnapDBC ---
	t.Run("checkDbcSnapDBC_docdb", func(t *testing.T) {
		snap := docdbtypes.DBClusterSnapshot{
			DBClusterIdentifier: aws.String(parentID),
		}
		res := resource.Resource{ID: "snap-1", RawStruct: snap}
		result := checkDbcSnapDBC(context.Background(), nil, res, emptyCache)
		if result.TargetType != "dbc" {
			t.Errorf("TargetType = %q, want dbc", result.TargetType)
		}
		if result.Count != 1 {
			t.Errorf("Count = %d, want 1", result.Count)
		}
		if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != parentID {
			t.Errorf("IDs = %v, want [%s]", result.ResourceIDs, parentID)
		}
	})

	t.Run("checkDbcSnapDBC_rds", func(t *testing.T) {
		snap := rdstypes.DBClusterSnapshot{
			DBClusterIdentifier: aws.String(parentID),
		}
		res := resource.Resource{ID: "snap-2", RawStruct: snap}
		result := checkDbcSnapDBC(context.Background(), nil, res, emptyCache)
		if result.Count != 1 {
			t.Errorf("Count = %d, want 1", result.Count)
		}
		if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != parentID {
			t.Errorf("IDs = %v, want [%s]", result.ResourceIDs, parentID)
		}
	})

	t.Run("checkDbcSnapDBC_nil_identifier", func(t *testing.T) {
		snap := docdbtypes.DBClusterSnapshot{DBClusterIdentifier: nil}
		res := resource.Resource{ID: "snap-3", RawStruct: snap}
		result := checkDbcSnapDBC(context.Background(), nil, res, emptyCache)
		if result.Count != 0 {
			t.Errorf("Count = %d, want 0 for nil identifier", result.Count)
		}
	})

	// --- checkDbcSnapKMS ---
	t.Run("checkDbcSnapKMS_docdb", func(t *testing.T) {
		snap := docdbtypes.DBClusterSnapshot{KmsKeyId: aws.String(kmsARN)}
		res := resource.Resource{ID: "snap-4", RawStruct: snap}
		result := checkDbcSnapKMS(context.Background(), nil, res, emptyCache)
		if result.TargetType != "kms" {
			t.Errorf("TargetType = %q, want kms", result.TargetType)
		}
		if result.Count != 1 {
			t.Errorf("Count = %d, want 1", result.Count)
		}
		if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != kmsUUID {
			t.Errorf("IDs = %v, want [%s]", result.ResourceIDs, kmsUUID)
		}
	})

	t.Run("checkDbcSnapKMS_rds", func(t *testing.T) {
		snap := rdstypes.DBClusterSnapshot{KmsKeyId: aws.String(kmsARN)}
		res := resource.Resource{ID: "snap-5", RawStruct: snap}
		result := checkDbcSnapKMS(context.Background(), nil, res, emptyCache)
		if result.Count != 1 {
			t.Errorf("Count = %d, want 1", result.Count)
		}
		if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != kmsUUID {
			t.Errorf("IDs = %v, want [%s]", result.ResourceIDs, kmsUUID)
		}
	})

	t.Run("checkDbcSnapKMS_no_key", func(t *testing.T) {
		snap := docdbtypes.DBClusterSnapshot{KmsKeyId: nil}
		res := resource.Resource{ID: "snap-6", RawStruct: snap}
		result := checkDbcSnapKMS(context.Background(), nil, res, emptyCache)
		if result.Count != 0 {
			t.Errorf("Count = %d, want 0 when KmsKeyId nil", result.Count)
		}
	})

	// --- checkDbcSnapVPC ---
	t.Run("checkDbcSnapVPC_docdb", func(t *testing.T) {
		snap := docdbtypes.DBClusterSnapshot{VpcId: aws.String(vpcID)}
		res := resource.Resource{ID: "snap-7", RawStruct: snap}
		result := checkDbcSnapVPC(context.Background(), nil, res, emptyCache)
		if result.TargetType != "vpc" {
			t.Errorf("TargetType = %q, want vpc", result.TargetType)
		}
		if result.Count != 1 {
			t.Errorf("Count = %d, want 1", result.Count)
		}
		if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != vpcID {
			t.Errorf("IDs = %v, want [%s]", result.ResourceIDs, vpcID)
		}
	})

	t.Run("checkDbcSnapVPC_rds", func(t *testing.T) {
		snap := rdstypes.DBClusterSnapshot{VpcId: aws.String(vpcID)}
		res := resource.Resource{ID: "snap-8", RawStruct: snap}
		result := checkDbcSnapVPC(context.Background(), nil, res, emptyCache)
		if result.Count != 1 {
			t.Errorf("Count = %d, want 1", result.Count)
		}
		if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != vpcID {
			t.Errorf("IDs = %v, want [%s]", result.ResourceIDs, vpcID)
		}
	})

	// --- dbcSnapParentRefs ---
	t.Run("dbcSnapParentRefs_docdb", func(t *testing.T) {
		snap := docdbtypes.DBClusterSnapshot{DBClusterIdentifier: aws.String(parentID)}
		name, arn := dbcSnapParentRefs(snap)
		if name != parentID {
			t.Errorf("name = %q, want %q", name, parentID)
		}
		if arn != "" {
			t.Errorf("arn = %q, want empty (no ARN on snapshot shape)", arn)
		}
	})

	t.Run("dbcSnapParentRefs_rds", func(t *testing.T) {
		snap := rdstypes.DBClusterSnapshot{DBClusterIdentifier: aws.String(parentID)}
		name, arn := dbcSnapParentRefs(snap)
		if name != parentID {
			t.Errorf("name = %q, want %q", name, parentID)
		}
		if arn != "" {
			t.Errorf("arn = %q, want empty (no ARN on snapshot shape)", arn)
		}
	})

	// --- dbcResourceARN ---
	t.Run("dbcResourceARN_docdb", func(t *testing.T) {
		cluster := docdbtypes.DBCluster{DBClusterArn: aws.String(parentARN)}
		got := dbcResourceARN(cluster)
		if got != parentARN {
			t.Errorf("dbcResourceARN docdb = %q, want %q", got, parentARN)
		}
	})

	t.Run("dbcResourceARN_rds", func(t *testing.T) {
		cluster := rdstypes.DBCluster{DBClusterArn: aws.String(parentARN)}
		got := dbcResourceARN(cluster)
		if got != parentARN {
			t.Errorf("dbcResourceARN rds = %q, want %q", got, parentARN)
		}
	})

	t.Run("dbcResourceARN_nil", func(t *testing.T) {
		cluster := docdbtypes.DBCluster{DBClusterArn: nil}
		got := dbcResourceARN(cluster)
		if got != "" {
			t.Errorf("dbcResourceARN nil = %q, want empty", got)
		}
	})
}
