// dbc_snap_issue_enrichment.go — Wave 1 cross-ref enricher for dbc-snap.
//
// Thin wrapper around EnrichSnapshotCrossRef (snapshot_cross_ref.go) configured
// with the dbc-snap parent (DBCluster) extractors. Covers BOTH DocumentDB and
// Aurora cluster snapshots — they share the AWS API (DescribeDBClusterSnapshots).
// The enricher detects two signals from docs/resources/dbc-snap.md §3.1:
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
//
// Engine note: snapshots in this resource type may be docdbtypes.DBClusterSnapshot
// (DocumentDB engine) OR rdstypes.DBClusterSnapshot (Aurora — the Aurora cluster
// fetcher in the RDS service uses rds.DescribeDBClusterSnapshots). Both shapes
// expose DBClusterIdentifier / SnapshotType / SnapshotCreateTime under the same
// field names, so the extractors below try each in turn.
package aws

import (
	"fmt"
	"time"

	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
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
	OrphanRowLabel:     "Source Cluster",
	RetentionPhrase:    func(d int) string { return fmt.Sprintf("automated, %dd past retention", d) },
	RetentionEnabled:   true,
})

// dbcSnapParentID extracts DBClusterIdentifier from either docdb or rds
// DBClusterSnapshot SDK shape.
func dbcSnapParentID(raw any) (string, bool) {
	if snap, ok := assertStruct[docdbtypes.DBClusterSnapshot](raw); ok {
		if snap.DBClusterIdentifier == nil || *snap.DBClusterIdentifier == "" {
			return "", false
		}
		return *snap.DBClusterIdentifier, true
	}
	if snap, ok := assertStruct[rdstypes.DBClusterSnapshot](raw); ok {
		if snap.DBClusterIdentifier == nil || *snap.DBClusterIdentifier == "" {
			return "", false
		}
		return *snap.DBClusterIdentifier, true
	}
	return "", false
}

// dbcSnapCreatedAt extracts SnapshotCreateTime from either SDK shape.
func dbcSnapCreatedAt(raw any) (time.Time, bool) {
	if snap, ok := assertStruct[docdbtypes.DBClusterSnapshot](raw); ok {
		if snap.SnapshotCreateTime == nil {
			return time.Time{}, false
		}
		return *snap.SnapshotCreateTime, true
	}
	if snap, ok := assertStruct[rdstypes.DBClusterSnapshot](raw); ok {
		if snap.SnapshotCreateTime == nil {
			return time.Time{}, false
		}
		return *snap.SnapshotCreateTime, true
	}
	return time.Time{}, false
}

// dbcSnapType extracts SnapshotType from either SDK shape.
func dbcSnapType(raw any) (string, bool) {
	if snap, ok := assertStruct[docdbtypes.DBClusterSnapshot](raw); ok {
		if snap.SnapshotType == nil {
			return "", false
		}
		return *snap.SnapshotType, true
	}
	if snap, ok := assertStruct[rdstypes.DBClusterSnapshot](raw); ok {
		if snap.SnapshotType == nil {
			return "", false
		}
		return *snap.SnapshotType, true
	}
	return "", false
}

// dbcParentRetention extracts BackupRetentionPeriod from either docdb or rds
// DBCluster SDK shape (the dbc fetcher may emit either depending on engine).
func dbcParentRetention(raw any) (int32, bool) {
	if c, ok := assertStruct[docdbtypes.DBCluster](raw); ok {
		if c.BackupRetentionPeriod == nil {
			return 0, false
		}
		return *c.BackupRetentionPeriod, true
	}
	if c, ok := assertStruct[rdstypes.DBCluster](raw); ok {
		if c.BackupRetentionPeriod == nil {
			return 0, false
		}
		return *c.BackupRetentionPeriod, true
	}
	return 0, false
}
