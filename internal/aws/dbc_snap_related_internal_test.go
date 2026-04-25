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
