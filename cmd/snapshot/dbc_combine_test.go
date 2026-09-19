package main

import (
	"errors"
	"testing"
)

// errThrottled is a stand-in AWS API error used across this file's
// one-side-fails test cases — the combine functions must treat any non-nil
// error uniformly (they don't need to branch on error type), so a plain
// sentinel is sufficient to pin the contract.
var errThrottled = errors.New("ThrottlingException: rate exceeded")

// The dbc/dbc-snap capture keeps the partial-success behavior of the live app
// fetcher: a failed DocDB or RDS source never discards the other source's rows.
//
//   - both sides ok: merged with docdb-first dedup (dedupDBCByID /
//     dedupDBCSnapByID — docdb rows win on ID collision).
//   - one side fails, the other has rows: rows from the healthy side are
//     preserved, the returned error is nil, and PartialErrors records which
//     side failed.
//   - both sides fail: top-level error is non-nil and the caller omits the
//     section from the snapshot.

var _ func([]dbcCluster, error, []dbcCluster, error) (dbcData, error) = combineDBCClusters

var _ func([]dbcSnapshot, error, []dbcSnapshot, error) (dbcSnapData, error) = combineDBCSnapshots

// TestCombineDBCClusters_BothOK_MergesWithDocDBFirstDedup pins the happy
// path: both sides succeed, rows are merged with the docdb-first dedup
// (dedupDBCByID), and no error / no PartialErrors.
func TestCombineDBCClusters_BothOK_MergesWithDocDBFirstDedup(t *testing.T) {
	docdb := []dbcCluster{
		{Source: "docdb", DBClusterIdentifier: "shared-id"},
		{Source: "docdb", DBClusterIdentifier: "docdb-only"},
	}
	rds := []dbcCluster{
		{Source: "rds", DBClusterIdentifier: "shared-id"}, // collides — docdb wins
		{Source: "rds", DBClusterIdentifier: "rds-only"},
	}

	got, err := combineDBCClusters(docdb, nil, rds, nil)
	if err != nil {
		t.Fatalf("combineDBCClusters() error = %v, want nil when both sides succeed", err)
	}
	if len(got.PartialErrors) != 0 {
		t.Errorf("PartialErrors = %v, want empty when both sides succeed", got.PartialErrors)
	}
	if len(got.Clusters) != 3 {
		t.Fatalf("Clusters = %d entries, want 3 (docdb-only + shared[docdb-wins] + rds-only), got %+v", len(got.Clusters), got.Clusters)
	}
	byID := make(map[string]dbcCluster, len(got.Clusters))
	for _, c := range got.Clusters {
		byID[c.DBClusterIdentifier] = c
	}
	if shared, ok := byID["shared-id"]; !ok || shared.Source != "docdb" {
		t.Errorf("shared-id cluster = %+v, want Source=\"docdb\" (docdb-first dedup wins on collision)", shared)
	}
	if _, ok := byID["docdb-only"]; !ok {
		t.Errorf("expected docdb-only cluster preserved, got %+v", got.Clusters)
	}
	if _, ok := byID["rds-only"]; !ok {
		t.Errorf("expected rds-only cluster preserved, got %+v", got.Clusters)
	}
}

// TestCombineDBCClusters_RDSFails_DocDBRowsPreserved pins that when the
// RDS-side call fails but DocDB succeeded and returned rows,
// those rows must be preserved (not discarded), the returned error must be
// nil (not a top-level capture failure), and the failure must be recorded in
// PartialErrors so the operator can see the RDS side was incomplete.
func TestCombineDBCClusters_RDSFails_DocDBRowsPreserved(t *testing.T) {
	docdb := []dbcCluster{
		{Source: "docdb", DBClusterIdentifier: "docdb-cluster-1"},
		{Source: "docdb", DBClusterIdentifier: "docdb-cluster-2"},
	}
	rdsErr := errThrottled

	got, err := combineDBCClusters(docdb, nil, nil, rdsErr)
	if err != nil {
		t.Fatalf("combineDBCClusters() error = %v, want nil — DocDB rows must not be discarded because RDS failed", err)
	}
	if len(got.Clusters) != 2 {
		t.Fatalf("Clusters = %d entries, want 2 (both DocDB rows preserved), got %+v", len(got.Clusters), got.Clusters)
	}
	if len(got.PartialErrors) != 1 {
		t.Fatalf("PartialErrors = %v, want exactly 1 entry recording the RDS-side failure", got.PartialErrors)
	}
}

// TestCombineDBCClusters_DocDBFails_RDSRowsPreserved is the mirror case: the
// DocDB-side call fails but RDS succeeded — RDS rows must be preserved.
func TestCombineDBCClusters_DocDBFails_RDSRowsPreserved(t *testing.T) {
	rds := []dbcCluster{
		{Source: "rds", DBClusterIdentifier: "rds-cluster-1"},
	}
	docdbErr := errThrottled

	got, err := combineDBCClusters(nil, docdbErr, rds, nil)
	if err != nil {
		t.Fatalf("combineDBCClusters() error = %v, want nil — RDS rows must not be discarded because DocDB failed", err)
	}
	if len(got.Clusters) != 1 {
		t.Fatalf("Clusters = %d entries, want 1 (RDS row preserved), got %+v", len(got.Clusters), got.Clusters)
	}
	if len(got.PartialErrors) != 1 {
		t.Fatalf("PartialErrors = %v, want exactly 1 entry recording the DocDB-side failure", got.PartialErrors)
	}
}

// TestCombineDBCClusters_BothFail_TopLevelError pins that a genuine top-level
// failure (both sources errored, nothing usable) returns a non-nil error so
// the caller omits the section entirely.
func TestCombineDBCClusters_BothFail_TopLevelError(t *testing.T) {
	got, err := combineDBCClusters(nil, errThrottled, nil, errThrottled)
	if err == nil {
		t.Fatalf("combineDBCClusters() error = nil, want non-nil when both DocDB and RDS fail — the section must be omitted")
	}
	if len(got.Clusters) != 0 {
		t.Errorf("Clusters = %+v, want empty when both sides failed", got.Clusters)
	}
}

// TestCombineDBCSnapshots_BothOK_MergesWithDocDBFirstDedup mirrors the
// cluster happy-path test for the snapshot variant (dedupDBCSnapByID).
func TestCombineDBCSnapshots_BothOK_MergesWithDocDBFirstDedup(t *testing.T) {
	docdb := []dbcSnapshot{
		{Source: "docdb", DBClusterSnapshotIdentifier: "shared-snap"},
		{Source: "docdb", DBClusterSnapshotIdentifier: "docdb-only-snap"},
	}
	rds := []dbcSnapshot{
		{Source: "rds", DBClusterSnapshotIdentifier: "shared-snap"}, // collides — docdb wins
		{Source: "rds", DBClusterSnapshotIdentifier: "rds-only-snap"},
	}

	got, err := combineDBCSnapshots(docdb, nil, rds, nil)
	if err != nil {
		t.Fatalf("combineDBCSnapshots() error = %v, want nil when both sides succeed", err)
	}
	if len(got.PartialErrors) != 0 {
		t.Errorf("PartialErrors = %v, want empty when both sides succeed", got.PartialErrors)
	}
	if len(got.Snapshots) != 3 {
		t.Fatalf("Snapshots = %d entries, want 3, got %+v", len(got.Snapshots), got.Snapshots)
	}
	byID := make(map[string]dbcSnapshot, len(got.Snapshots))
	for _, s := range got.Snapshots {
		byID[s.DBClusterSnapshotIdentifier] = s
	}
	if shared, ok := byID["shared-snap"]; !ok || shared.Source != "docdb" {
		t.Errorf("shared-snap = %+v, want Source=\"docdb\" (docdb-first dedup wins on collision)", shared)
	}
}

// TestCombineDBCSnapshots_RDSFails_DocDBRowsPreserved pins that an RDS
// DescribeDBClusterSnapshots failure keeps the already-gathered DocDB
// snapshot rows.
func TestCombineDBCSnapshots_RDSFails_DocDBRowsPreserved(t *testing.T) {
	docdb := []dbcSnapshot{
		{Source: "docdb", DBClusterSnapshotIdentifier: "docdb-snap-1"},
		{Source: "docdb", DBClusterSnapshotIdentifier: "docdb-snap-2"},
		{Source: "docdb", DBClusterSnapshotIdentifier: "docdb-snap-3"},
	}
	rdsErr := errThrottled

	got, err := combineDBCSnapshots(docdb, nil, nil, rdsErr)
	if err != nil {
		t.Fatalf("combineDBCSnapshots() error = %v, want nil — DocDB snapshot rows must not be discarded because RDS failed", err)
	}
	if len(got.Snapshots) != 3 {
		t.Fatalf("Snapshots = %d entries, want 3 (all DocDB rows preserved), got %+v", len(got.Snapshots), got.Snapshots)
	}
	if len(got.PartialErrors) != 1 {
		t.Fatalf("PartialErrors = %v, want exactly 1 entry recording the RDS-side failure", got.PartialErrors)
	}
}

// TestCombineDBCSnapshots_DocDBFails_RDSRowsPreserved is the mirror case for
// the snapshot variant.
func TestCombineDBCSnapshots_DocDBFails_RDSRowsPreserved(t *testing.T) {
	rds := []dbcSnapshot{
		{Source: "rds", DBClusterSnapshotIdentifier: "rds-snap-1"},
	}
	docdbErr := errThrottled

	got, err := combineDBCSnapshots(nil, docdbErr, rds, nil)
	if err != nil {
		t.Fatalf("combineDBCSnapshots() error = %v, want nil — RDS snapshot rows must not be discarded because DocDB failed", err)
	}
	if len(got.Snapshots) != 1 {
		t.Fatalf("Snapshots = %d entries, want 1 (RDS row preserved), got %+v", len(got.Snapshots), got.Snapshots)
	}
	if len(got.PartialErrors) != 1 {
		t.Fatalf("PartialErrors = %v, want exactly 1 entry recording the DocDB-side failure", got.PartialErrors)
	}
}

// TestCombineDBCSnapshots_BothFail_TopLevelError pins the both-sources-failed
// top-level error contract for the snapshot variant.
func TestCombineDBCSnapshots_BothFail_TopLevelError(t *testing.T) {
	got, err := combineDBCSnapshots(nil, errThrottled, nil, errThrottled)
	if err == nil {
		t.Fatalf("combineDBCSnapshots() error = nil, want non-nil when both DocDB and RDS fail — the section must be omitted")
	}
	if len(got.Snapshots) != 0 {
		t.Errorf("Snapshots = %+v, want empty when both sides failed", got.Snapshots)
	}
}
