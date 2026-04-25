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
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

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

// TestEnrichDBCSnapCrossRef_RDSShape_OrphanAndPastRetention verifies that the
// enricher handles rdstypes.DBClusterSnapshot and rdstypes.DBCluster inputs
// correctly, firing orphan and past-retention signals as appropriate.
func TestEnrichDBCSnapCrossRef_RDSShape_OrphanAndPastRetention(t *testing.T) {
	t.Run("orphan_rds_shape", func(t *testing.T) {
		snap := rdstypes.DBClusterSnapshot{
			DBClusterSnapshotIdentifier: aws.String("aurora-orphan"),
			DBClusterIdentifier:         aws.String("deleted-aurora"),
			Engine:                      aws.String("aurora-postgresql"),
			SnapshotType:                aws.String("manual"),
			SnapshotCreateTime:          aws.Time(time.Now().Add(-5 * 24 * time.Hour)),
		}
		res := resource.Resource{
			ID:        "aurora-orphan",
			Name:      "aurora-orphan",
			Status:    "",
			Fields:    map[string]string{},
			RawStruct: snap,
		}

		// dbc cache has an rdstypes.DBCluster but NOT "deleted-aurora".
		otherParent := rdstypes.DBCluster{
			DBClusterIdentifier:   aws.String("other-aurora"),
			BackupRetentionPeriod: aws.Int32(14),
		}
		cache := resource.ResourceCache{
			"dbc": resource.ResourceCacheEntry{
				Resources:   []resource.Resource{{ID: "other-aurora", RawStruct: otherParent}},
				IsTruncated: false,
			},
		}

		result, err := enrichDBCSnapCrossRef(context.Background(), nil, []resource.Resource{res}, cache)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		finding, ok := result.Findings["aurora-orphan"]
		if !ok {
			t.Fatal("expected orphan finding for rdstypes.DBClusterSnapshot, got none")
		}
		if finding.Summary != "orphan: source cluster deleted" {
			t.Errorf("Summary = %q, want %q", finding.Summary, "orphan: source cluster deleted")
		}
		if fu := result.FieldUpdates["aurora-orphan"]; fu == nil || fu["status"] != "orphan: source cluster deleted" {
			t.Errorf("FieldUpdates status = %q, want %q",
				result.FieldUpdates["aurora-orphan"]["status"], "orphan: source cluster deleted")
		}
	})

	t.Run("past_retention_rds_parent", func(t *testing.T) {
		const retentionDays = 7
		const ageDays = 25

		parent := rdstypes.DBCluster{
			DBClusterIdentifier:   aws.String("aurora-prod"),
			BackupRetentionPeriod: aws.Int32(retentionDays),
		}
		cache := resource.ResourceCache{
			"dbc": resource.ResourceCacheEntry{
				Resources: []resource.Resource{{ID: "aurora-prod", RawStruct: parent}},
			},
		}

		snap := rdstypes.DBClusterSnapshot{
			DBClusterSnapshotIdentifier: aws.String("aurora-stale"),
			DBClusterIdentifier:         aws.String("aurora-prod"),
			Engine:                      aws.String("aurora-postgresql"),
			SnapshotType:                aws.String("automated"),
			SnapshotCreateTime:          aws.Time(time.Now().Add(-ageDays * 24 * time.Hour)),
		}
		res := resource.Resource{
			ID:        "aurora-stale",
			Name:      "aurora-stale",
			Status:    "",
			Fields:    map[string]string{},
			RawStruct: snap,
		}

		result, err := enrichDBCSnapCrossRef(context.Background(), nil, []resource.Resource{res}, cache)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		finding, ok := result.Findings["aurora-stale"]
		if !ok {
			t.Fatal("expected past-retention finding for rdstypes automated snapshot")
		}
		if !strings.Contains(finding.Summary, "automated") || !strings.Contains(finding.Summary, "past retention") {
			t.Errorf("Summary = %q; want automated + past retention", finding.Summary)
		}
		// 25 - 7 = 18 days over retention
		if !strings.Contains(finding.Summary, "18d") {
			t.Errorf("Summary days-over should be 18 (25 - 7), got %q", finding.Summary)
		}
	})
}

// TestDbcSnapExtractors_DualShape verifies the four extractor functions handle
// both docdbtypes.DBClusterSnapshot and rdstypes.DBClusterSnapshot inputs.
func TestDbcSnapExtractors_DualShape(t *testing.T) {
	now := time.Now().Truncate(time.Second)

	t.Run("dbcSnapParentID_docdb", func(t *testing.T) {
		snap := docdbtypes.DBClusterSnapshot{
			DBClusterIdentifier: aws.String("prod-docdb"),
		}
		id, ok := dbcSnapParentID(snap)
		if !ok {
			t.Fatal("dbcSnapParentID returned ok=false for docdbtypes with non-nil identifier")
		}
		if id != "prod-docdb" {
			t.Errorf("id = %q, want %q", id, "prod-docdb")
		}
	})

	t.Run("dbcSnapParentID_rds", func(t *testing.T) {
		snap := rdstypes.DBClusterSnapshot{
			DBClusterIdentifier: aws.String("prod-aurora"),
		}
		id, ok := dbcSnapParentID(snap)
		if !ok {
			t.Fatal("dbcSnapParentID returned ok=false for rdstypes with non-nil identifier")
		}
		if id != "prod-aurora" {
			t.Errorf("id = %q, want %q", id, "prod-aurora")
		}
	})

	t.Run("dbcSnapParentID_nil_docdb", func(t *testing.T) {
		snap := docdbtypes.DBClusterSnapshot{DBClusterIdentifier: nil}
		_, ok := dbcSnapParentID(snap)
		if ok {
			t.Error("dbcSnapParentID returned ok=true for nil DBClusterIdentifier")
		}
	})

	t.Run("dbcSnapCreatedAt_docdb", func(t *testing.T) {
		snap := docdbtypes.DBClusterSnapshot{
			SnapshotCreateTime: &now,
		}
		got, ok := dbcSnapCreatedAt(snap)
		if !ok {
			t.Fatal("dbcSnapCreatedAt returned ok=false for docdbtypes with non-nil time")
		}
		if !got.Equal(now) {
			t.Errorf("got %v, want %v", got, now)
		}
	})

	t.Run("dbcSnapCreatedAt_rds", func(t *testing.T) {
		snap := rdstypes.DBClusterSnapshot{
			SnapshotCreateTime: &now,
		}
		got, ok := dbcSnapCreatedAt(snap)
		if !ok {
			t.Fatal("dbcSnapCreatedAt returned ok=false for rdstypes with non-nil time")
		}
		if !got.Equal(now) {
			t.Errorf("got %v, want %v", got, now)
		}
	})

	t.Run("dbcSnapType_docdb", func(t *testing.T) {
		snap := docdbtypes.DBClusterSnapshot{SnapshotType: aws.String("manual")}
		got, ok := dbcSnapType(snap)
		if !ok || got != "manual" {
			t.Errorf("dbcSnapType docdb: got %q, ok=%v, want manual/true", got, ok)
		}
	})

	t.Run("dbcSnapType_rds", func(t *testing.T) {
		snap := rdstypes.DBClusterSnapshot{SnapshotType: aws.String("automated")}
		got, ok := dbcSnapType(snap)
		if !ok || got != "automated" {
			t.Errorf("dbcSnapType rds: got %q, ok=%v, want automated/true", got, ok)
		}
	})

	t.Run("dbcParentRetention_docdb", func(t *testing.T) {
		cluster := docdbtypes.DBCluster{BackupRetentionPeriod: aws.Int32(14)}
		got, ok := dbcParentRetention(cluster)
		if !ok || got != 14 {
			t.Errorf("dbcParentRetention docdb: got %d, ok=%v, want 14/true", got, ok)
		}
	})

	t.Run("dbcParentRetention_rds", func(t *testing.T) {
		cluster := rdstypes.DBCluster{BackupRetentionPeriod: aws.Int32(7)}
		got, ok := dbcParentRetention(cluster)
		if !ok || got != 7 {
			t.Errorf("dbcParentRetention rds: got %d, ok=%v, want 7/true", got, ok)
		}
	})

	t.Run("dbcParentRetention_nil", func(t *testing.T) {
		cluster := docdbtypes.DBCluster{BackupRetentionPeriod: nil}
		_, ok := dbcParentRetention(cluster)
		if ok {
			t.Error("dbcParentRetention returned ok=true for nil BackupRetentionPeriod")
		}
	})
}
