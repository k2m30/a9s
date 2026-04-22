package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/docdb"
)

// DocDBDescribeDBClustersAPI defines the interface for the DocumentDB DescribeDBClusters operation.
type DocDBDescribeDBClustersAPI interface {
	DescribeDBClusters(ctx context.Context, params *docdb.DescribeDBClustersInput, optFns ...func(*docdb.Options)) (*docdb.DescribeDBClustersOutput, error)
}

// DocDBDescribeDBClusterSnapshotsAPI defines the interface for the DocumentDB DescribeDBClusterSnapshots operation.
type DocDBDescribeDBClusterSnapshotsAPI interface {
	DescribeDBClusterSnapshots(ctx context.Context, params *docdb.DescribeDBClusterSnapshotsInput, optFns ...func(*docdb.Options)) (*docdb.DescribeDBClusterSnapshotsOutput, error)
}

// DocDBDescribeDBSubnetGroupsAPI defines the interface for the DocumentDB
// DescribeDBSubnetGroups operation. Used by dbc→subnet/vpc.
type DocDBDescribeDBSubnetGroupsAPI interface {
	DescribeDBSubnetGroups(ctx context.Context, params *docdb.DescribeDBSubnetGroupsInput, optFns ...func(*docdb.Options)) (*docdb.DescribeDBSubnetGroupsOutput, error)
}

// DocDBDescribePendingMaintenanceActionsAPI defines the interface for the DocumentDB
// DescribePendingMaintenanceActions operation.
type DocDBDescribePendingMaintenanceActionsAPI interface {
	DescribePendingMaintenanceActions(ctx context.Context, params *docdb.DescribePendingMaintenanceActionsInput, optFns ...func(*docdb.Options)) (*docdb.DescribePendingMaintenanceActionsOutput, error)
}

// DocDBAPI is the aggregate interface covering all DocumentDB operations used by a9s fetchers.
// *docdb.Client structurally satisfies this interface.
type DocDBAPI interface {
	DocDBDescribeDBClustersAPI
	DocDBDescribeDBClusterSnapshotsAPI
	DocDBDescribeDBSubnetGroupsAPI
	DocDBDescribePendingMaintenanceActionsAPI
}
