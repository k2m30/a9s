// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

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
	"context"
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
