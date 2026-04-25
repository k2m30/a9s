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
// DescribeDBSubnetGroups operation. Used by dbi→eni path for VPC/subnet
// resolution when the subnet group is needed.
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

// RDSAPI is the aggregate interface covering all RDS operations used by a9s fetchers.
// *rds.Client structurally satisfies this interface.
type RDSAPI interface {
	RDSDescribeDBInstancesAPI
	RDSDescribeDBSnapshotsAPI
	RDSDescribeEventsAPI
	RDSDescribePendingMaintenanceAPI // Wave 2 enrichment
	RDSDescribeDBSubnetGroupsAPI
	RDSDescribeDBClustersAPI
	RDSDescribeDBClusterSnapshotsAPI
}
