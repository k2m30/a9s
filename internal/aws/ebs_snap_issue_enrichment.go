// ebs_snap_issue_enrichment.go — Wave 1 cross-ref enricher for ebs-snap.
//
// Thin wrapper around EnrichSnapshotCrossRef (snapshot_cross_ref.go) configured
// with the ebs-snap parent (ec2 Volume) extractors. The enricher detects one
// signal from docs/attention-signals.md's ebs-snap row:
//
//   - orphan: source VolumeId NOT found in the loaded ebs cache.
//     Phrase: "orphan: source volume deleted"
//
// ec2.Volume carries no retention-period concept, so RetentionEnabled is
// false — only the orphan rule fires; GetCreatedAt/GetSnapshotType/
// GetParentRetention are left nil per SnapshotCrossRefConfig's contract.
//
// Wave classification: zero AWS API calls. The enricher scans the in-memory
// ebs ResourceCache only.
package aws

import (
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// enrichEBSSnapCrossRef is the IssueEnricherFunc registered for ebs-snap.
// It is the SnapshotCrossRef helper instantiated with ec2types.Snapshot /
// ec2types.Volume extractors.
var enrichEBSSnapCrossRef = EnrichSnapshotCrossRef(SnapshotCrossRefConfig{
	ParentShortName: "ebs",
	GetParentID: func(raw any) (string, bool) {
		snap, ok := assertStruct[ec2types.Snapshot](raw)
		if !ok || snap.VolumeId == nil || *snap.VolumeId == "" {
			return "", false
		}
		return *snap.VolumeId, true
	},
	OrphanPhrase:     "orphan: source volume deleted",
	ParentRowLabel:   "Source Volume",
	RetentionEnabled: false,
	Severity:         "~",
	ShortName:        "ebs-snap",
	OrphanCode:       CodeEBSSnapOrphan,
})
