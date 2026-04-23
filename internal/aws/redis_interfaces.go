package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/elasticache"
)

// ElastiCacheDescribeCacheClustersAPI defines the interface for the ElastiCache DescribeCacheClusters operation.
type ElastiCacheDescribeCacheClustersAPI interface {
	DescribeCacheClusters(ctx context.Context, params *elasticache.DescribeCacheClustersInput, optFns ...func(*elasticache.Options)) (*elasticache.DescribeCacheClustersOutput, error)
}

// ElastiCacheDescribeReplicationGroupsAPI defines the interface for the
// ElastiCache DescribeReplicationGroups operation. This is the primary list API
// for the redis resource type — each row represents one ReplicationGroup.
// Also used by related checkers for member-cluster resolution.
type ElastiCacheDescribeReplicationGroupsAPI interface {
	DescribeReplicationGroups(ctx context.Context, params *elasticache.DescribeReplicationGroupsInput, optFns ...func(*elasticache.Options)) (*elasticache.DescribeReplicationGroupsOutput, error)
}

// ElastiCacheDescribeCacheSubnetGroupsAPI defines the interface for the
// ElastiCache DescribeCacheSubnetGroups operation. Used by redis→subnet/vpc.
type ElastiCacheDescribeCacheSubnetGroupsAPI interface {
	DescribeCacheSubnetGroups(ctx context.Context, params *elasticache.DescribeCacheSubnetGroupsInput, optFns ...func(*elasticache.Options)) (*elasticache.DescribeCacheSubnetGroupsOutput, error)
}

// ElastiCacheListTagsForResourceAPI defines the interface for the
// ElastiCache ListTagsForResource operation. Used by redis→cfn for
// extracting the aws:cloudformation:stack-name tag.
type ElastiCacheListTagsForResourceAPI interface {
	ListTagsForResource(ctx context.Context, params *elasticache.ListTagsForResourceInput, optFns ...func(*elasticache.Options)) (*elasticache.ListTagsForResourceOutput, error)
}

// ElastiCacheAPI is the aggregate interface covering all ElastiCache operations used by a9s fetchers.
// *elasticache.Client structurally satisfies this interface.
type ElastiCacheAPI interface {
	ElastiCacheDescribeCacheClustersAPI
	ElastiCacheDescribeReplicationGroupsAPI
	ElastiCacheDescribeCacheSubnetGroupsAPI
	ElastiCacheListTagsForResourceAPI
}
