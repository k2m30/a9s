// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// dbc_snap_issue_enrichment.go — Wave 1 cross-ref enricher for dbc-snap.
//
// Thin wrapper around EnrichSnapshotCrossRef (snapshot_cross_ref.go) configured
// with the dbc-snap parent (DBCluster) extractors.
//
// Both DocumentDB AND Aurora cluster snapshots flow through this enricher because
// the dbc-snap fetcher merges results from c.DocDB.DescribeDBClusterSnapshots
// (DocumentDB — docdb@v1.48.12/api_op_DescribeDBClusterSnapshots.go:14) and
// c.RDS.DescribeDBClusterSnapshots (Aurora + Multi-AZ —
// rds@v1.116.3/api_op_DescribeDBClusterSnapshots.go:19-25). Each row's RawStruct
// is whichever SDK it came from (docdbtypes.DBClusterSnapshot or
// rdstypes.DBClusterSnapshot). The extractors below try docdbtypes first, then
// rdstypes, returning zero values if neither matches.
//
// The enricher detects two signals (docs/resources/dbc-snap.md):
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
	"context"
	"slices"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/service/docdb"
	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// dbc-snap canonical FindingCodes emitted by the cross-ref enricher.
const (
	dbcSnapOrphanCode        domain.FindingCode = "dbc-snap.orphan"
	dbcSnapPastRetentionCode domain.FindingCode = "dbc-snap.past-retention"
	dbcSnapPublicCode        domain.FindingCode = "dbc-snap.public"
)

// enrichDBCSnapCrossRef is the IssueEnricherFunc registered for dbc-snap.
var enrichDBCSnapCrossRef = EnrichSnapshotCrossRef(SnapshotCrossRefConfig{
	ParentShortName:    "dbc",
	GetParentID:        dbcSnapParentID,
	ParentIsLocal:      dbcSnapParentIsLocal,
	IsParent:           dbcSnapTakenFrom,
	GetCreatedAt:       dbcSnapCreatedAt,
	GetSnapshotType:    dbcSnapType,
	GetParentRetention: dbcParentRetention,
	ParentRowLabel:     "Source Cluster",
	ParentNoun:         "cluster",
	RetentionEnabled:   true,
	OrphanCode:         dbcSnapOrphanCode,
	PastRetentionCode:  dbcSnapPastRetentionCode,
	PublicAttr:         dbcSnapShareAttributes,
	PublicCode:         dbcSnapPublicCode,
})

// dbcSnapShareAttributes reads one cluster snapshot's share attributes. The
// dbc-snap list merges DocumentDB and Aurora rows, so the RawStruct decides
// which SDK owns the snapshot — the same rule the extractors below follow.
func dbcSnapShareAttributes(ctx context.Context, clients *ServiceClients, snap resource.Resource) ([]snapshotAttribute, error) {
	if _, isDocDB := assertStruct[docdbtypes.DBClusterSnapshot](snap.RawStruct); isDocDB {
		if clients.DocDB == nil {
			return nil, nil
		}
		out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*docdb.DescribeDBClusterSnapshotAttributesOutput, error) {
			return clients.DocDB.DescribeDBClusterSnapshotAttributes(ctx, &docdb.DescribeDBClusterSnapshotAttributesInput{
				DBClusterSnapshotIdentifier: aws.String(snap.ID),
			})
		})
		if err != nil {
			return nil, err
		}
		if out.DBClusterSnapshotAttributesResult == nil {
			return nil, UnusableAnswerErr{Call: "DescribeDBClusterSnapshotAttributes", Field: "attributes result"}
		}
		attrs := make([]snapshotAttribute, 0, len(out.DBClusterSnapshotAttributesResult.DBClusterSnapshotAttributes))
		for _, a := range out.DBClusterSnapshotAttributesResult.DBClusterSnapshotAttributes {
			attrs = append(attrs, snapshotAttribute{Name: aws.ToString(a.AttributeName), Values: a.AttributeValues})
		}
		return attrs, nil
	}
	if clients.RDS == nil {
		return nil, nil
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*rds.DescribeDBClusterSnapshotAttributesOutput, error) {
		return clients.RDS.DescribeDBClusterSnapshotAttributes(ctx, &rds.DescribeDBClusterSnapshotAttributesInput{
			DBClusterSnapshotIdentifier: aws.String(snap.ID),
		})
	})
	if err != nil {
		return nil, err
	}
	if out.DBClusterSnapshotAttributesResult == nil {
		return nil, UnusableAnswerErr{Call: "DescribeDBClusterSnapshotAttributes", Field: "attributes result"}
	}
	attrs := make([]snapshotAttribute, 0, len(out.DBClusterSnapshotAttributesResult.DBClusterSnapshotAttributes))
	for _, a := range out.DBClusterSnapshotAttributesResult.DBClusterSnapshotAttributes {
		attrs = append(attrs, snapshotAttribute{Name: aws.ToString(a.AttributeName), Values: a.AttributeValues})
	}
	return attrs, nil
}

// dbcSnapParent is what a cluster snapshot records about the cluster it was
// taken from, read from the DocumentDB or the RDS snapshot shape. Only the RDS
// shape carries DbClusterResourceId.
type dbcSnapParent struct {
	cluster    string
	resourceID string
	created    *time.Time
	arn        string
	sourceARN  string
}

func dbcSnapParentOf(raw any) (dbcSnapParent, bool) {
	if s, ok := assertStruct[docdbtypes.DBClusterSnapshot](raw); ok {
		return dbcSnapParent{
			cluster:   aws.ToString(s.DBClusterIdentifier),
			created:   s.ClusterCreateTime,
			arn:       aws.ToString(s.DBClusterSnapshotArn),
			sourceARN: aws.ToString(s.SourceDBClusterSnapshotArn),
		}, true
	}
	if s, ok := assertStruct[rdstypes.DBClusterSnapshot](raw); ok {
		return dbcSnapParent{
			cluster:    aws.ToString(s.DBClusterIdentifier),
			resourceID: aws.ToString(s.DbClusterResourceId),
			created:    s.ClusterCreateTime,
			arn:        aws.ToString(s.DBClusterSnapshotArn),
			sourceARN:  aws.ToString(s.SourceDBClusterSnapshotArn),
		}, true
	}
	return dbcSnapParent{}, false
}

// local reports whether the snapshot's cluster can be in the dbc list. AWS
// sets SourceDBClusterSnapshotArn on every copy, same-Region ones included.
// The dbc-snap list holds only this account's snapshots in this Region, so
// the snapshot's own ARN names the session's Region and account, and a copy
// whose source ARN names another Region or account has its cluster elsewhere.
func (p dbcSnapParent) local() bool {
	src, err := arn.Parse(p.sourceARN)
	if err != nil {
		return true
	}
	own, err := arn.Parse(p.arn)
	return err != nil || (src.Region == own.Region && src.AccountID == own.AccountID)
}

func dbcSnapParentID(raw any) (string, bool) {
	p, _ := dbcSnapParentOf(raw)
	return p.cluster, p.cluster != ""
}

func dbcSnapParentIsLocal(raw any) bool {
	p, _ := dbcSnapParentOf(raw)
	return p.local()
}

// dbcSnapParentRow names the dbc row the snapshot's DBClusterIdentifier
// opens. A cluster that was deleted, or that lives in another Region or
// account, has no row here, and the field stays unnavigable rather than
// offering a link to nothing.
func dbcSnapParentRow(src resource.Resource, clusters []resource.Resource) string {
	p, ok := dbcSnapParentOf(src.RawStruct)
	if !ok || !p.local() {
		return ""
	}
	if clusters == nil {
		return p.cluster
	}
	if i := slices.IndexFunc(clusters, func(c resource.Resource) bool { return dbcSnapTakenFrom(src.RawStruct, c) }); i >= 0 {
		return clusters[i].ID
	}
	return ""
}

// dbcSnapTakenFrom reports whether cluster is the cluster snap was taken
// from. A cluster deleted and created again under the same name gets a new
// DbClusterResourceId and ClusterCreateTime. DbClusterResourceId also stays
// with a cluster through a rename, so a snapshot that carries it is matched
// on it alone; otherwise the name is matched, and ClusterCreateTime too when
// both sides carry it.
func dbcSnapTakenFrom(snapRaw any, cluster resource.Resource) bool {
	p, ok := dbcSnapParentOf(snapRaw)
	if !ok || !p.local() {
		return false
	}
	resourceID, created := dbcClusterGeneration(cluster.RawStruct)
	if p.resourceID != "" {
		return p.resourceID == resourceID
	}
	if p.cluster == "" || (p.cluster != cluster.ID && p.cluster != cluster.Name) {
		return false
	}
	return p.created == nil || created == nil || p.created.Equal(*created)
}

func dbcClusterGeneration(raw any) (resourceID string, created *time.Time) {
	if c, ok := assertStruct[docdbtypes.DBCluster](raw); ok {
		return aws.ToString(c.DbClusterResourceId), c.ClusterCreateTime
	}
	if c, ok := assertStruct[rdstypes.DBCluster](raw); ok {
		return aws.ToString(c.DbClusterResourceId), c.ClusterCreateTime
	}
	return "", nil
}

// dbcSnapCreatedAt extracts SnapshotCreateTime from either a
// docdbtypes.DBClusterSnapshot or rdstypes.DBClusterSnapshot.
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

// dbcSnapType extracts SnapshotType from either a
// docdbtypes.DBClusterSnapshot or rdstypes.DBClusterSnapshot.
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

// dbcParentRetention extracts BackupRetentionPeriod from a parent cluster row.
// The parent RawStruct can be either docdbtypes.DBCluster or rdstypes.DBCluster.
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
