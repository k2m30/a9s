// dbc_snap_issue_enrichment.go — Wave 1 cross-ref enricher for dbc-snap.
//
// Thin wrapper around EnrichSnapshotCrossRef (snapshot_cross_ref.go) configured
// with the dbc-snap parent (DBCluster) extractors.
//
// Both DocumentDB and Aurora cluster snapshots arrive through the DocDB SDK's
// DescribeDBClusterSnapshots — the docdb and rds SDKs both target
// rds.{region}.amazonaws.com, so a single docdbtypes deserializer covers both
// engines. The enricher detects two signals from docs/resources/dbc-snap.md §3.1:
//
//  1. orphan: parent DBClusterIdentifier NOT found in the loaded dbc cache.
//     Phrase: "orphan: source cluster deleted"
//
//  2. past-retention: automated snapshot older than the parent cluster's
//     BackupRetentionPeriod (1.0× — no multiplier; same threshold as dbi-snap).
//     Phrase: "automated, <N>d past retention"
//
// Wave classification: zero AWS API calls. The enricher scans the in-memory
// dbc ResourceCache only.
package aws

import (
	"fmt"
	"time"

	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"
)

func init() {
	registerIssueEnricher("dbc-snap", enrichDBCSnapCrossRef, 100)
}

// enrichDBCSnapCrossRef is the IssueEnricherFunc registered for dbc-snap.
var enrichDBCSnapCrossRef = EnrichSnapshotCrossRef(SnapshotCrossRefConfig{
	ParentShortName:    "dbc",
	GetParentID:        dbcSnapParentID,
	GetCreatedAt:       dbcSnapCreatedAt,
	GetSnapshotType:    dbcSnapType,
	GetParentRetention: dbcParentRetention,
	OrphanPhrase:       "orphan: source cluster deleted",
	ParentRowLabel:     "Source Cluster",
	RetentionPhrase:    func(d int) string { return fmt.Sprintf("automated, %dd past retention", d) },
	RetentionEnabled:   true,
})

// dbcSnapParentID extracts DBClusterIdentifier from a docdbtypes.DBClusterSnapshot.
func dbcSnapParentID(raw any) (string, bool) {
	snap, ok := assertStruct[docdbtypes.DBClusterSnapshot](raw)
	if !ok || snap.DBClusterIdentifier == nil || *snap.DBClusterIdentifier == "" {
		return "", false
	}
	return *snap.DBClusterIdentifier, true
}

// dbcSnapCreatedAt extracts SnapshotCreateTime from a docdbtypes.DBClusterSnapshot.
func dbcSnapCreatedAt(raw any) (time.Time, bool) {
	snap, ok := assertStruct[docdbtypes.DBClusterSnapshot](raw)
	if !ok || snap.SnapshotCreateTime == nil {
		return time.Time{}, false
	}
	return *snap.SnapshotCreateTime, true
}

// dbcSnapType extracts SnapshotType from a docdbtypes.DBClusterSnapshot.
func dbcSnapType(raw any) (string, bool) {
	snap, ok := assertStruct[docdbtypes.DBClusterSnapshot](raw)
	if !ok || snap.SnapshotType == nil {
		return "", false
	}
	return *snap.SnapshotType, true
}

// dbcParentRetention extracts BackupRetentionPeriod from a docdbtypes.DBCluster.
func dbcParentRetention(raw any) (int32, bool) {
	c, ok := assertStruct[docdbtypes.DBCluster](raw)
	if !ok || c.BackupRetentionPeriod == nil {
		return 0, false
	}
	return *c.BackupRetentionPeriod, true
}
