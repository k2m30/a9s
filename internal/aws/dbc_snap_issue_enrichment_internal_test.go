package aws

// dbc_snap_issue_enrichment_internal_test.go — internal package tests for the
// dbc-snap cross-ref enricher's handling of Broken+orphan stacking.
//
// Lives in internal/aws (not tests/unit) so it can call the unexported
// enrichDBCSnapCrossRef package-level var directly, testing the exact
// function wired into IssueEnricherRegistry["dbc-snap"].
//
// Pins regression B1: a fetcher-emitted Broken phrase ("failed") MUST
// survive cross-ref enrichment. When the enricher adds "orphan: source
// cluster deleted", the merged status must be "failed (+1)", NOT the
// orphan phrase overriding the Broken phrase.

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// TestEnrichDBCSnapCrossRef_FailedPlusOrphan verifies that a snapshot whose
// fetcher set Status="failed" (Broken) retains "failed" as the top phrase
// after the cross-ref enricher adds the orphan signal. The merged status must
// be "failed (+1)", not the orphan phrase.
//
// This is the key correctness contract for WarnDBCSnapFailedAndManualOldID:
// three signals stack (failed + manual-old + orphan); the coder's fixture
// sets parent="deleted-legacy-cluster" (not in dbc cache) so orphan fires.
// This test covers the two-signal case (failed + orphan) as a focused pin.
func TestEnrichDBCSnapCrossRef_FailedPlusOrphan(t *testing.T) {
	snap := docdbtypes.DBClusterSnapshot{
		DBClusterSnapshotIdentifier: aws.String("snap-failed"),
		DBClusterIdentifier:         aws.String("deleted-legacy-cluster"),
		Status:                      aws.String("failed"),
		SnapshotType:                aws.String("manual"),
	}

	// Build the resource as the fetcher would emit it: Status="failed",
	// Issues=["failed"] — the pre-enrichment state.
	res := resource.Resource{
		ID:        "snap-failed",
		Name:      "snap-failed",
		Status:    "failed",
		Issues:    []string{"failed"},
		Fields:    map[string]string{},
		RawStruct: snap,
	}

	// dbc cache is loaded but "deleted-legacy-cluster" is absent → orphan fires.
	// IsTruncated=false so the orphan rule is NOT suppressed.
	otherCluster := docdbtypes.DBCluster{
		DBClusterIdentifier:   aws.String("other-cluster"),
		BackupRetentionPeriod: aws.Int32(7),
	}
	cache := resource.ResourceCache{
		"dbc": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{ID: "other-cluster", Name: "other-cluster", RawStruct: otherCluster},
			},
			IsTruncated: false,
		},
	}

	result, err := enrichDBCSnapCrossRef(context.Background(), nil, []resource.Resource{res}, cache)
	if err != nil {
		t.Fatalf("enrichDBCSnapCrossRef returned unexpected error: %v", err)
	}

	// The orphan finding must be present with the dbc-snap-specific phrase.
	finding, hasFinding := result.Findings["snap-failed"]
	if !hasFinding {
		t.Fatal("Findings[\"snap-failed\"] missing; want orphan finding from cross-ref enricher")
	}
	if finding.Summary != "orphan: source cluster deleted" {
		t.Errorf("Findings[\"snap-failed\"].Summary = %q, want %q",
			finding.Summary, "orphan: source cluster deleted")
	}

	// The merged status must be "failed (+1)" — Broken phrase stays at top,
	// orphan stacked as +1. This is the B1 regression pin.
	if result.FieldUpdates == nil || result.FieldUpdates["snap-failed"] == nil {
		t.Fatal("FieldUpdates[\"snap-failed\"] is nil; want merged status phrase")
	}
	gotStatus := result.FieldUpdates["snap-failed"]["status"]
	if gotStatus != "failed (+1)" {
		t.Errorf("FieldUpdates[\"snap-failed\"][\"status\"] = %q, want %q",
			gotStatus, "failed (+1)")
	}
}

// TestEnrichDBCSnapCrossRef_TruncatedDBC_OrphanSuppressed verifies that the
// orphan rule is skipped when the dbc cache is truncated and the parent is
// not in the visible window. This ensures no false-positive orphan flags
// on healthy snapshots when the dbc list has not been fully loaded.
func TestEnrichDBCSnapCrossRef_TruncatedDBC_OrphanSuppressed(t *testing.T) {
	snap := docdbtypes.DBClusterSnapshot{
		DBClusterSnapshotIdentifier: aws.String("snap-healthy"),
		DBClusterIdentifier:         aws.String("maybe-exists-cluster"),
		Status:                      aws.String("available"),
		SnapshotType:                aws.String("manual"),
	}
	res := resource.Resource{
		ID:        "snap-healthy",
		Name:      "snap-healthy",
		Status:    "",
		Fields:    map[string]string{},
		RawStruct: snap,
	}

	// dbc cache is truncated — parent not visible, but truncation prevents orphan.
	otherCluster := docdbtypes.DBCluster{
		DBClusterIdentifier: aws.String("other-cluster"),
	}
	cache := resource.ResourceCache{
		"dbc": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{ID: "other-cluster", Name: "other-cluster", RawStruct: otherCluster},
			},
			IsTruncated: true,
		},
	}

	result, err := enrichDBCSnapCrossRef(context.Background(), nil, []resource.Resource{res}, cache)
	if err != nil {
		t.Fatalf("enrichDBCSnapCrossRef returned unexpected error: %v", err)
	}

	if _, has := result.Findings["snap-healthy"]; has {
		t.Error("Findings[\"snap-healthy\"] present; want absent (orphan suppressed when dbc cache truncated)")
	}
	if fu := result.FieldUpdates["snap-healthy"]; fu != nil && fu["status"] != "" {
		t.Errorf("FieldUpdates[\"snap-healthy\"][\"status\"] = %q; want empty when orphan suppressed", fu["status"])
	}
}
