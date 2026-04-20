package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/redshift"
)

// RedshiftDescribeClustersAPI defines the interface for the Redshift DescribeClusters operation.
type RedshiftDescribeClustersAPI interface {
	DescribeClusters(ctx context.Context, params *redshift.DescribeClustersInput, optFns ...func(*redshift.Options)) (*redshift.DescribeClustersOutput, error)
}

// RedshiftDescribeLoggingStatusAPI defines the interface for the Redshift
// DescribeLoggingStatus operation. Used by redshift→s3 (audit bucket) and
// redshift→logs (CloudWatch log group).
type RedshiftDescribeLoggingStatusAPI interface {
	DescribeLoggingStatus(ctx context.Context, params *redshift.DescribeLoggingStatusInput, optFns ...func(*redshift.Options)) (*redshift.DescribeLoggingStatusOutput, error)
}

// RedshiftDescribeClusterSubnetGroupsAPI defines the interface for the Redshift
// DescribeClusterSubnetGroups operation. Used by redshift→subnet to resolve
// the subnets inside a ClusterSubnetGroupName.
type RedshiftDescribeClusterSubnetGroupsAPI interface {
	DescribeClusterSubnetGroups(ctx context.Context, params *redshift.DescribeClusterSubnetGroupsInput, optFns ...func(*redshift.Options)) (*redshift.DescribeClusterSubnetGroupsOutput, error)
}

// RedshiftAPI is the aggregate interface covering all Redshift operations used by a9s fetchers.
// *redshift.Client structurally satisfies this interface.
type RedshiftAPI interface {
	RedshiftDescribeClustersAPI
	RedshiftDescribeLoggingStatusAPI
	RedshiftDescribeClusterSubnetGroupsAPI
}
