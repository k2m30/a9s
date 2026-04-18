// Package fakes provides in-process fake implementations of AWS service
// interfaces for use in demo mode and tests.
package fakes

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/elasticache"

	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
)

// ElastiCacheFake implements aws.ElastiCacheAPI against fixture data loaded at construction time.
type ElastiCacheFake struct {
	fix *fixtures.ElastiCacheFixtures
}

// NewElastiCache constructs an ElastiCacheFake backed by fixture data from the fixtures package.
func NewElastiCache() *ElastiCacheFake {
	return &ElastiCacheFake{fix: fixtures.NewElastiCacheFixtures()}
}

func (f *ElastiCacheFake) DescribeCacheClusters(_ context.Context, _ *elasticache.DescribeCacheClustersInput, _ ...func(*elasticache.Options)) (*elasticache.DescribeCacheClustersOutput, error) {
	return &elasticache.DescribeCacheClustersOutput{CacheClusters: f.fix.CacheClusters}, nil
}

// DescribeReplicationGroups is a stub that returns an empty list. Fixture
// data for replication groups is not yet modeled; adding real fixtures later
// will not require re-wiring this method.
func (f *ElastiCacheFake) DescribeReplicationGroups(_ context.Context, _ *elasticache.DescribeReplicationGroupsInput, _ ...func(*elasticache.Options)) (*elasticache.DescribeReplicationGroupsOutput, error) {
	return &elasticache.DescribeReplicationGroupsOutput{}, nil
}

// DescribeCacheSubnetGroups is a stub that returns an empty list.
func (f *ElastiCacheFake) DescribeCacheSubnetGroups(_ context.Context, _ *elasticache.DescribeCacheSubnetGroupsInput, _ ...func(*elasticache.Options)) (*elasticache.DescribeCacheSubnetGroupsOutput, error) {
	return &elasticache.DescribeCacheSubnetGroupsOutput{}, nil
}

// ListTagsForResource is a stub that returns an empty tag list.
func (f *ElastiCacheFake) ListTagsForResource(_ context.Context, _ *elasticache.ListTagsForResourceInput, _ ...func(*elasticache.Options)) (*elasticache.ListTagsForResourceOutput, error) {
	return &elasticache.ListTagsForResourceOutput{}, nil
}
