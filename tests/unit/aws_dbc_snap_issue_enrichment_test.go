package unit

// aws_dbc_snap_issue_enrichment_test.go — Cross-ref enricher tests for dbc-snap.
//
// dbc-snap is wired to the SnapshotCrossRef helper (snapshot_cross_ref.go)
// with parent="dbc". These tests pin the activation: orphan and
// past-retention signals must fire for DBClusterSnapshot inputs whose
// parent is missing or whose retention is exceeded.
//
// The enricher is registered in IssueEnricherRegistry["dbc-snap"]. Tests
// drive it by retrieving the registered function, NOT by importing the
// production file directly.

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// dbcSnapEnricher retrieves the registered dbc-snap IssueEnricherFunc.
func dbcSnapEnricher(t *testing.T) awsclient.IssueEnricherFunc {
	t.Helper()
	e, ok := awsclient.IssueEnricherRegistry["dbc-snap"]
	if !ok {
		t.Fatal("IssueEnricherRegistry[\"dbc-snap\"] not registered")
	}
	if e.Fn == nil {
		t.Fatal("IssueEnricherRegistry[\"dbc-snap\"].Fn is nil")
	}
	return e.Fn
}

// TestDBCSnap_Orphan_DocDB verifies the orphan signal fires for a DocDB
// cluster snapshot whose parent is missing from the dbc cache.
func TestDBCSnap_Orphan_DocDB(t *testing.T) {
	enricher := dbcSnapEnricher(t)

	// dbc cache has "other-cluster" but not "deleted-cluster"; not truncated.
	otherCluster := docdbtypes.DBCluster{
		DBClusterIdentifier:   aws.String("other-cluster"),
		BackupRetentionPeriod: aws.Int32(7),
	}
	cache := resource.ResourceCache{
		"dbc": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{ID: "other-cluster", RawStruct: otherCluster},
			},
			IsTruncated: false,
		},
	}

	snap := docdbtypes.DBClusterSnapshot{
		DBClusterSnapshotIdentifier: aws.String("orphan-dbc-snap"),
		DBClusterIdentifier:         aws.String("deleted-cluster"),
		Engine:                      aws.String("docdb"),
		SnapshotType:                aws.String("manual"),
		SnapshotCreateTime:          aws.Time(time.Now().Add(-3 * 24 * time.Hour)),
	}
	res := resource.Resource{
		ID:        "orphan-dbc-snap",
		RawStruct: snap,
	}

	result, err := enricher(context.Background(), nil, []resource.Resource{res}, cache)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	finding, ok := result.Findings["orphan-dbc-snap"]
	if !ok {
		t.Fatal("expected orphan finding, got none")
	}
	if finding.Severity != "!" {
		t.Errorf("Severity = %q, want %q", finding.Severity, "!")
	}
	if finding.Summary != "orphan: source cluster deleted" {
		t.Errorf("Summary = %q, want %q", finding.Summary, "orphan: source cluster deleted")
	}
	updates, ok := result.FieldUpdates["orphan-dbc-snap"]
	if !ok {
		t.Fatal("expected FieldUpdates, got none")
	}
	if updates["status"] != "orphan: source cluster deleted" {
		t.Errorf("status = %q, want %q", updates["status"], "orphan: source cluster deleted")
	}
}

// TestDBCSnap_Orphan_Aurora verifies the orphan signal fires for an Aurora
// cluster snapshot (rdstypes.DBClusterSnapshot) whose parent is missing from the
// dbc cache. Aurora cluster snapshots arrive via the RDS SDK
// (rdstypes.DBClusterSnapshot), not the DocDB SDK. The enricher's dual-shape
// extractor (dbcSnapParentID) handles both SDK shapes.
func TestDBCSnap_Orphan_Aurora(t *testing.T) {
	enricher := dbcSnapEnricher(t)

	// Cache has an rdstypes.DBCluster parent but NOT the one referenced by the snap.
	otherCluster := rdstypes.DBCluster{
		DBClusterIdentifier:   aws.String("other-aurora"),
		BackupRetentionPeriod: aws.Int32(14),
	}
	cache := resource.ResourceCache{
		"dbc": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{ID: "other-aurora", RawStruct: otherCluster},
			},
			IsTruncated: false,
		},
	}

	// Aurora snapshot uses rdstypes.DBClusterSnapshot — this is the shape the
	// RDS-side fetcher (FetchRDSDBClusterSnapshotsPage) emits.
	snap := rdstypes.DBClusterSnapshot{
		DBClusterSnapshotIdentifier: aws.String("orphan-aurora-snap"),
		DBClusterIdentifier:         aws.String("deleted-aurora"),
		Engine:                      aws.String("aurora-postgresql"),
		SnapshotType:                aws.String("manual"),
		SnapshotCreateTime:          aws.Time(time.Now().Add(-3 * 24 * time.Hour)),
	}
	res := resource.Resource{
		ID:        "orphan-aurora-snap",
		RawStruct: snap,
	}

	result, err := enricher(context.Background(), nil, []resource.Resource{res}, cache)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if finding, ok := result.Findings["orphan-aurora-snap"]; !ok {
		t.Fatal("expected orphan finding for Aurora snapshot (rdstypes.DBClusterSnapshot), got none")
	} else if finding.Summary != "orphan: source cluster deleted" {
		t.Errorf("Summary = %q, want %q", finding.Summary, "orphan: source cluster deleted")
	}
	// FieldUpdates must carry the orphan phrase as the merged status.
	if fu := result.FieldUpdates["orphan-aurora-snap"]; fu == nil || fu["status"] != "orphan: source cluster deleted" {
		t.Errorf("FieldUpdates[\"orphan-aurora-snap\"][\"status\"] = %q, want %q",
			result.FieldUpdates["orphan-aurora-snap"]["status"], "orphan: source cluster deleted")
	}
}

// TestDBCSnap_PastRetention_DocDB verifies the past-retention signal fires
// for an automated DocDB cluster snapshot older than parent's retention.
func TestDBCSnap_PastRetention_DocDB(t *testing.T) {
	enricher := dbcSnapEnricher(t)

	const retentionDays = 7
	const ageDays = 30

	parent := docdbtypes.DBCluster{
		DBClusterIdentifier:   aws.String("prod-cluster"),
		BackupRetentionPeriod: aws.Int32(retentionDays),
	}
	cache := resource.ResourceCache{
		"dbc": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{ID: "prod-cluster", RawStruct: parent},
			},
		},
	}

	snap := docdbtypes.DBClusterSnapshot{
		DBClusterSnapshotIdentifier: aws.String("stale-dbc-snap"),
		DBClusterIdentifier:         aws.String("prod-cluster"),
		Engine:                      aws.String("docdb"),
		SnapshotType:                aws.String("automated"),
		SnapshotCreateTime:          aws.Time(time.Now().Add(-ageDays * 24 * time.Hour)),
	}
	res := resource.Resource{
		ID:        "stale-dbc-snap",
		RawStruct: snap,
	}

	result, err := enricher(context.Background(), nil, []resource.Resource{res}, cache)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	finding, ok := result.Findings["stale-dbc-snap"]
	if !ok {
		t.Fatal("expected past-retention finding, got none")
	}
	if !strings.Contains(finding.Summary, "automated") || !strings.Contains(finding.Summary, "past retention") {
		t.Errorf("Summary = %q, want \"automated, Nd past retention\"", finding.Summary)
	}
	if !strings.Contains(finding.Summary, "23d") {
		t.Errorf("Summary days-over should be 23 (30 - 7), got %q", finding.Summary)
	}
}

// TestDBCSnap_PastRetention_Manual verifies the past-retention rule does
// NOT fire for manual snapshots.
func TestDBCSnap_PastRetention_Manual(t *testing.T) {
	enricher := dbcSnapEnricher(t)

	parent := docdbtypes.DBCluster{
		DBClusterIdentifier:   aws.String("prod-cluster"),
		BackupRetentionPeriod: aws.Int32(7),
	}
	cache := resource.ResourceCache{
		"dbc": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{ID: "prod-cluster", RawStruct: parent},
			},
		},
	}

	snap := docdbtypes.DBClusterSnapshot{
		DBClusterSnapshotIdentifier: aws.String("manual-stale-snap"),
		DBClusterIdentifier:         aws.String("prod-cluster"),
		SnapshotType:                aws.String("manual"),
		SnapshotCreateTime:          aws.Time(time.Now().Add(-30 * 24 * time.Hour)),
	}
	res := resource.Resource{
		ID:        "manual-stale-snap",
		RawStruct: snap,
	}

	result, err := enricher(context.Background(), nil, []resource.Resource{res}, cache)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if finding, ok := result.Findings["manual-stale-snap"]; ok {
		t.Errorf("expected no finding for manual snapshot, got: %+v", finding)
	}
}

// TestDBCSnap_TruncatedCache_NoFalseOrphan verifies the orphan rule skips
// when the dbc cache is truncated and the parent is not in the visible window.
func TestDBCSnap_TruncatedCache_NoFalseOrphan(t *testing.T) {
	enricher := dbcSnapEnricher(t)

	otherCluster := docdbtypes.DBCluster{
		DBClusterIdentifier: aws.String("other-cluster"),
	}
	cache := resource.ResourceCache{
		"dbc": resource.ResourceCacheEntry{
			Resources:   []resource.Resource{{ID: "other-cluster", RawStruct: otherCluster}},
			IsTruncated: true,
		},
	}

	snap := docdbtypes.DBClusterSnapshot{
		DBClusterSnapshotIdentifier: aws.String("snap-1"),
		DBClusterIdentifier:         aws.String("deleted-cluster"),
		SnapshotType:                aws.String("manual"),
		SnapshotCreateTime:          aws.Time(time.Now().Add(-3 * 24 * time.Hour)),
	}
	res := resource.Resource{ID: "snap-1", RawStruct: snap}

	result, err := enricher(context.Background(), nil, []resource.Resource{res}, cache)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if finding, ok := result.Findings["snap-1"]; ok {
		t.Errorf("expected no orphan finding when cache truncated, got: %+v", finding)
	}
}

// TestDBCSnap_NoCache_Skip verifies the enricher skips silently when the
// dbc cache is not loaded (per spec §3.1 skip rule).
func TestDBCSnap_NoCache_Skip(t *testing.T) {
	enricher := dbcSnapEnricher(t)

	snap := docdbtypes.DBClusterSnapshot{
		DBClusterSnapshotIdentifier: aws.String("snap-1"),
		DBClusterIdentifier:         aws.String("any-cluster"),
		SnapshotType:                aws.String("automated"),
	}
	res := resource.Resource{ID: "snap-1", RawStruct: snap}

	result, err := enricher(context.Background(), nil, []resource.Resource{res}, resource.ResourceCache{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings when dbc cache absent, got %d", len(result.Findings))
	}
}
