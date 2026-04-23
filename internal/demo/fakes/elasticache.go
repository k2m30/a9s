// Package fakes provides in-process fake implementations of AWS service
// interfaces for use in demo mode and tests.
package fakes

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	elasticachetypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"

	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
)

// ElastiCacheFake implements aws.ElastiCacheAPI against fixture data loaded at construction time.
type ElastiCacheFake struct {
	fix *fixtures.RedisFixtures
}

// NewElastiCache constructs an ElastiCacheFake backed by fixture data from the fixtures package.
func NewElastiCache() *ElastiCacheFake {
	return &ElastiCacheFake{fix: fixtures.NewRedisFixtures()}
}

// DescribeReplicationGroups returns all replication groups from the fixture set.
func (f *ElastiCacheFake) DescribeReplicationGroups(_ context.Context, _ *elasticache.DescribeReplicationGroupsInput, _ ...func(*elasticache.Options)) (*elasticache.DescribeReplicationGroupsOutput, error) {
	return &elasticache.DescribeReplicationGroupsOutput{
		ReplicationGroups: f.fix.ReplicationGroups,
	}, nil
}

// DescribeCacheClusters returns cache clusters from the fixture set, optionally
// filtered by CacheClusterId when the input specifies one.
func (f *ElastiCacheFake) DescribeCacheClusters(_ context.Context, input *elasticache.DescribeCacheClustersInput, _ ...func(*elasticache.Options)) (*elasticache.DescribeCacheClustersOutput, error) {
	if input != nil && input.CacheClusterId != nil && *input.CacheClusterId != "" {
		id := *input.CacheClusterId
		var filtered []elasticachetypes.CacheCluster
		for _, cc := range f.fix.CacheClusters {
			if cc.CacheClusterId != nil && *cc.CacheClusterId == id {
				filtered = append(filtered, cc)
			}
		}
		return &elasticache.DescribeCacheClustersOutput{CacheClusters: filtered}, nil
	}
	return &elasticache.DescribeCacheClustersOutput{CacheClusters: f.fix.CacheClusters}, nil
}

// DescribeCacheSubnetGroups returns subnet groups from the fixture set, optionally
// filtered by CacheSubnetGroupName when the input specifies one.
func (f *ElastiCacheFake) DescribeCacheSubnetGroups(_ context.Context, input *elasticache.DescribeCacheSubnetGroupsInput, _ ...func(*elasticache.Options)) (*elasticache.DescribeCacheSubnetGroupsOutput, error) {
	if input != nil && input.CacheSubnetGroupName != nil && *input.CacheSubnetGroupName != "" {
		name := *input.CacheSubnetGroupName
		var filtered []elasticachetypes.CacheSubnetGroup
		for _, sg := range f.fix.SubnetGroups {
			if sg.CacheSubnetGroupName != nil && *sg.CacheSubnetGroupName == name {
				filtered = append(filtered, sg)
			}
		}
		return &elasticache.DescribeCacheSubnetGroupsOutput{CacheSubnetGroups: filtered}, nil
	}
	return &elasticache.DescribeCacheSubnetGroupsOutput{CacheSubnetGroups: f.fix.SubnetGroups}, nil
}

// ListTagsForResource looks up the tag list by ResourceName (replication-group ARN)
// from the fixture TagLists map. Returns an empty tag list when no entry matches.
func (f *ElastiCacheFake) ListTagsForResource(_ context.Context, input *elasticache.ListTagsForResourceInput, _ ...func(*elasticache.Options)) (*elasticache.ListTagsForResourceOutput, error) {
	if input != nil && input.ResourceName != nil {
		if tags, ok := f.fix.TagLists[*input.ResourceName]; ok {
			return &elasticache.ListTagsForResourceOutput{TagList: tags}, nil
		}
	}
	return &elasticache.ListTagsForResourceOutput{}, nil
}
