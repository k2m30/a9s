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
