// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/rds"
)

// RDSDescribeDBInstancesAPI defines the interface for the RDS DescribeDBInstances operation.
type RDSDescribeDBInstancesAPI interface {
	DescribeDBInstances(ctx context.Context, params *rds.DescribeDBInstancesInput, optFns ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error)
}

// RDSDescribeDBClustersAPI defines the interface for the RDS DescribeDBClusters
// operation. Used by the dbc fetcher to cover Aurora + Multi-AZ DB clusters
// (see docs/resources/dbc.md §1 Coverage). Per AWS SDK docstring
// (rds@v1.116.3/api_op_DescribeDBClusters.go:19-28), this returns Aurora +
// Multi-AZ explicitly and may also return Neptune / DocumentDB rows.
type RDSDescribeDBClustersAPI interface {
	DescribeDBClusters(ctx context.Context, params *rds.DescribeDBClustersInput, optFns ...func(*rds.Options)) (*rds.DescribeDBClustersOutput, error)
}

// RDSDescribeDBClusterSnapshotsAPI defines the interface for the RDS
// DescribeDBClusterSnapshots operation. Required because the docdb-side call
// is DocDB-scoped — Aurora and Multi-AZ cluster snapshots only return through
// this RDS-side operation per AWS SDK docstring
// (rds@v1.116.3/api_op_DescribeDBClusterSnapshots.go:19-25).
type RDSDescribeDBClusterSnapshotsAPI interface {
	DescribeDBClusterSnapshots(ctx context.Context, params *rds.DescribeDBClusterSnapshotsInput, optFns ...func(*rds.Options)) (*rds.DescribeDBClusterSnapshotsOutput, error)
}

// RDSDescribeDBSubnetGroupsAPI defines the interface for the RDS
// DescribeDBSubnetGroups operation. Used by the dbi→eni path for VPC/subnet
// resolution when the subnet group is needed, and by the dbc fetcher for
// Aurora-side subnet-group resolution (rdstypes.DBCluster rows). DocDB-side
// rows (docdb_types.DBCluster) resolve via c.DocDB.DescribeDBSubnetGroups.
// See docs/resources/dbc.md §1 Coverage.
type RDSDescribeDBSubnetGroupsAPI interface {
	DescribeDBSubnetGroups(ctx context.Context, params *rds.DescribeDBSubnetGroupsInput, optFns ...func(*rds.Options)) (*rds.DescribeDBSubnetGroupsOutput, error)
}

// RDSDescribeDBSnapshotsAPI defines the interface for the RDS DescribeDBSnapshots operation.
type RDSDescribeDBSnapshotsAPI interface {
	DescribeDBSnapshots(ctx context.Context, params *rds.DescribeDBSnapshotsInput, optFns ...func(*rds.Options)) (*rds.DescribeDBSnapshotsOutput, error)
}

// RDSDescribeEventsAPI defines the interface for the RDS DescribeEvents operation.
type RDSDescribeEventsAPI interface {
	DescribeEvents(ctx context.Context, params *rds.DescribeEventsInput, optFns ...func(*rds.Options)) (*rds.DescribeEventsOutput, error)
}

// RDSDescribePendingMaintenanceAPI defines the interface for the RDS DescribePendingMaintenanceActions operation.
type RDSDescribePendingMaintenanceAPI interface {
	DescribePendingMaintenanceActions(ctx context.Context, params *rds.DescribePendingMaintenanceActionsInput, optFns ...func(*rds.Options)) (*rds.DescribePendingMaintenanceActionsOutput, error)
}

// RDSDescribeDBEngineVersionsAPI defines the interface for the RDS
// DescribeDBEngineVersions operation. Used by EnrichDBIMaintenance to decide
// whether an instance's engine version is still supported.
type RDSDescribeDBEngineVersionsAPI interface {
	DescribeDBEngineVersions(ctx context.Context, params *rds.DescribeDBEngineVersionsInput, optFns ...func(*rds.Options)) (*rds.DescribeDBEngineVersionsOutput, error)
}

// RDSDescribeDBSnapshotAttributesAPI defines the interface for the RDS
// DescribeDBSnapshotAttributes operation. Used by the dbi-snap enricher to
// read the "restore" attribute that says who may restore the snapshot.
type RDSDescribeDBSnapshotAttributesAPI interface {
	DescribeDBSnapshotAttributes(ctx context.Context, params *rds.DescribeDBSnapshotAttributesInput, optFns ...func(*rds.Options)) (*rds.DescribeDBSnapshotAttributesOutput, error)
}

// RDSDescribeDBClusterSnapshotAttributesAPI defines the interface for the RDS
// DescribeDBClusterSnapshotAttributes operation. Used by the dbc-snap
// enricher for Aurora / Multi-AZ cluster snapshots.
type RDSDescribeDBClusterSnapshotAttributesAPI interface {
	DescribeDBClusterSnapshotAttributes(ctx context.Context, params *rds.DescribeDBClusterSnapshotAttributesInput, optFns ...func(*rds.Options)) (*rds.DescribeDBClusterSnapshotAttributesOutput, error)
}

// RDSAPI is the aggregate interface covering all RDS operations used by a9s fetchers.
// *rds.Client structurally satisfies this interface.
type RDSAPI interface {
	RDSDescribeDBInstancesAPI
	RDSDescribeDBSnapshotsAPI
	RDSDescribeEventsAPI
	RDSDescribePendingMaintenanceAPI // Wave 2 enrichment
	RDSDescribeDBEngineVersionsAPI   // Wave 2 enrichment
	RDSDescribeDBSnapshotAttributesAPI
	RDSDescribeDBClusterSnapshotAttributesAPI
	RDSDescribeDBSubnetGroupsAPI
	RDSDescribeDBClustersAPI
	RDSDescribeDBClusterSnapshotsAPI
}
