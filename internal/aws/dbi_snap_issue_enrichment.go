// dbi_snap_issue_enrichment.go — Wave 1 cross-ref enricher for dbi-snap.
//
// Thin wrapper around EnrichSnapshotCrossRef (snapshot_cross_ref.go) configured
// with the dbi-snap parent (rds DBInstance) extractors. The enricher detects
// two signals from docs/resources/dbi-snap.md §3.1:
//
//  1. orphan: parent DBInstanceIdentifier NOT found in the loaded dbi cache.
//     Phrase: "orphan: source DB deleted"
//
//  2. past-retention: automated snapshot older than the parent's
//     BackupRetentionPeriod (1.0× — no multiplier).
//     Phrase: "automated, <N>d past retention"
//
// Wave classification: zero AWS API calls. The enricher scans the in-memory
// dbi ResourceCache only.
package aws

import (
	"fmt"
	"time"

	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
)

// enrichDBISnapCrossRef is the IssueEnricherFunc registered for dbi-snap.
// It is the SnapshotCrossRef helper instantiated with rds.DBSnapshot /
// rds.DBInstance extractors.
var enrichDBISnapCrossRef = EnrichSnapshotCrossRef(SnapshotCrossRefConfig{
	ParentShortName: "dbi",
	GetParentID: func(raw any) (string, bool) {
		snap, ok := assertStruct[rdstypes.DBSnapshot](raw)
		if !ok || snap.DBInstanceIdentifier == nil || *snap.DBInstanceIdentifier == "" {
			return "", false
		}
		return *snap.DBInstanceIdentifier, true
	},
	GetCreatedAt: func(raw any) (time.Time, bool) {
		snap, ok := assertStruct[rdstypes.DBSnapshot](raw)
		if !ok || snap.SnapshotCreateTime == nil {
			return time.Time{}, false
		}
		return *snap.SnapshotCreateTime, true
	},
	GetSnapshotType: func(raw any) (string, bool) {
		snap, ok := assertStruct[rdstypes.DBSnapshot](raw)
		if !ok || snap.SnapshotType == nil {
			return "", false
		}
		return *snap.SnapshotType, true
	},
	GetParentRetention: func(raw any) (int32, bool) {
		db, ok := assertStruct[rdstypes.DBInstance](raw)
		if !ok || db.BackupRetentionPeriod == nil {
			return 0, false
		}
		return *db.BackupRetentionPeriod, true
	},
	OrphanPhrase:     "orphan: source DB deleted",
	ParentRowLabel:   "Source DB",
	RetentionPhrase:  func(d int) string { return fmt.Sprintf("automated, %dd past retention", d) },
	RetentionEnabled: true,
	Severity:         "!",
})
