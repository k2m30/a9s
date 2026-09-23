// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// dbi_snap_issue_enrichment.go — Wave 1 cross-ref enricher for dbi-snap.
//
// Thin wrapper around EnrichSnapshotCrossRef (snapshot_cross_ref.go) configured
// with the dbi-snap parent (rds DBInstance) extractors. The enricher detects
// two signals (docs/resources/dbi-snap.md):
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
	"context"
	"slices"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// dbi-snap canonical FindingCodes emitted by the cross-ref enricher.
const (
	dbiSnapOrphanCode        domain.FindingCode = "dbi-snap.orphan"
	dbiSnapPastRetentionCode domain.FindingCode = "dbi-snap.past-retention"
	dbiSnapPublicCode        domain.FindingCode = "dbi-snap.public"
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
	ParentIsLocal: func(raw any) bool {
		snap, ok := assertStruct[rdstypes.DBSnapshot](raw)
		return !ok || dbiSnapParentIsLocal(snap)
	},
	IsParent: func(raw any, parent resource.Resource) bool {
		snap, ok := assertStruct[rdstypes.DBSnapshot](raw)
		return ok && dbiSnapTakenFrom(snap, parent)
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
	ParentRowLabel:    "Source DB",
	ParentNoun:        "instance",
	RetentionEnabled:  true,
	OrphanCode:        dbiSnapOrphanCode,
	PastRetentionCode: dbiSnapPastRetentionCode,
	PublicAttr:        dbiSnapShareAttributes,
	PublicCode:        dbiSnapPublicCode,
})

// dbiSnapShareAttributes reads one DB snapshot's share attributes.
func dbiSnapShareAttributes(ctx context.Context, clients *ServiceClients, snap resource.Resource) ([]snapshotAttribute, error) {
	if clients.RDS == nil {
		return nil, nil
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*rds.DescribeDBSnapshotAttributesOutput, error) {
		return clients.RDS.DescribeDBSnapshotAttributes(ctx, &rds.DescribeDBSnapshotAttributesInput{
			DBSnapshotIdentifier: aws.String(snap.ID),
		})
	})
	if err != nil {
		return nil, err
	}
	if out.DBSnapshotAttributesResult == nil {
		return nil, UnusableAnswerErr{Call: "DescribeDBSnapshotAttributes", Field: "attributes result"}
	}
	attrs := make([]snapshotAttribute, 0, len(out.DBSnapshotAttributesResult.DBSnapshotAttributes))
	for _, a := range out.DBSnapshotAttributesResult.DBSnapshotAttributes {
		attrs = append(attrs, snapshotAttribute{Name: aws.ToString(a.AttributeName), Values: a.AttributeValues})
	}
	return attrs, nil
}

// dbiSnapTakenFrom reports whether db is the instance snap was taken from.
// DbiResourceId stays with an instance through a rename and is never given to
// another instance, so a snapshot that carries one is matched on it alone.
func dbiSnapTakenFrom(snap rdstypes.DBSnapshot, db resource.Resource) bool {
	if !dbiSnapParentIsLocal(snap) {
		return false
	}
	if rid := aws.ToString(snap.DbiResourceId); rid != "" {
		inst, ok := assertStruct[rdstypes.DBInstance](db.RawStruct)
		return ok && aws.ToString(inst.DbiResourceId) == rid
	}
	name := aws.ToString(snap.DBInstanceIdentifier)
	return name != "" && (db.ID == name || db.Name == name)
}

// dbiSnapParentIsLocal reports whether the snapshot's instance can be in this
// region's dbi list: SourceDBSnapshotIdentifier has a value only on a
// cross-account or cross-region copy.
func dbiSnapParentIsLocal(snap rdstypes.DBSnapshot) bool {
	return aws.ToString(snap.SourceDBSnapshotIdentifier) == ""
}

// dbiSnapParentRow is the dbi row the snapshot was taken from, the one the
// DB Instances related row counts. With no dbi list loaded, a local
// snapshot's DBInstanceIdentifier is the only name there is.
func dbiSnapParentRow(src resource.Resource, _ string, dbis []resource.Resource) string {
	snap, ok := assertStruct[rdstypes.DBSnapshot](src.RawStruct)
	if !ok || !dbiSnapParentIsLocal(snap) {
		return ""
	}
	if dbis == nil {
		return aws.ToString(snap.DBInstanceIdentifier)
	}
	if i := slices.IndexFunc(dbis, func(db resource.Resource) bool { return dbiSnapTakenFrom(snap, db) }); i >= 0 {
		return dbis[i].ID
	}
	return ""
}
