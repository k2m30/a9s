package unit

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	ectypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"

	awsclient "github.com/k2m30/a9s/internal/aws"
)

// mockElastiCacheClient implements awsclient.ElastiCacheDescribeCacheClustersAPI for testing.
type mockElastiCacheClient struct {
	output *elasticache.DescribeCacheClustersOutput
	err    error
}

func (m *mockElastiCacheClient) DescribeCacheClusters(
	ctx context.Context,
	params *elasticache.DescribeCacheClustersInput,
	optFns ...func(*elasticache.Options),
) (*elasticache.DescribeCacheClustersOutput, error) {
	return m.output, m.err
}

// ---------------------------------------------------------------------------
// T057 - Test Redis (ElastiCache) response parsing with client-side filtering
// ---------------------------------------------------------------------------

func TestFetchRedisClusters_FiltersOnlyRedis(t *testing.T) {
	mock := &mockElastiCacheClient{
		output: &elasticache.DescribeCacheClustersOutput{
			CacheClusters: []ectypes.CacheCluster{
				{
					CacheClusterId:      aws.String("redis-prod-001"),
					Engine:              aws.String("redis"),
					EngineVersion:       aws.String("7.0.12"),
					CacheNodeType:       aws.String("cache.r6g.large"),
					CacheClusterStatus:  aws.String("available"),
					NumCacheNodes:       aws.Int32(3),
					ConfigurationEndpoint: &ectypes.Endpoint{
						Address: aws.String("redis-prod-001.abc123.clustercfg.use1.cache.amazonaws.com"),
					},
				},
				{
					CacheClusterId:     aws.String("redis-staging-001"),
					Engine:             aws.String("redis"),
					EngineVersion:      aws.String("6.2.14"),
					CacheNodeType:      aws.String("cache.t3.medium"),
					CacheClusterStatus: aws.String("available"),
					NumCacheNodes:      aws.Int32(1),
					// No ConfigurationEndpoint
				},
				{
					CacheClusterId:     aws.String("memcached-prod-001"),
					Engine:             aws.String("memcached"),
					EngineVersion:      aws.String("1.6.22"),
					CacheNodeType:      aws.String("cache.m5.large"),
					CacheClusterStatus: aws.String("available"),
					NumCacheNodes:      aws.Int32(2),
					ConfigurationEndpoint: &ectypes.Endpoint{
						Address: aws.String("memcached-prod-001.abc123.cfg.use1.cache.amazonaws.com"),
					},
				},
			},
		},
	}

	resources, err := awsclient.FetchRedisClusters(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Should only return 2 redis clusters, not the memcached one
	if len(resources) != 2 {
		t.Fatalf("expected 2 resources (redis only), got %d", len(resources))
	}

	// Verify required fields exist
	requiredFields := []string{"cluster_id", "engine_version", "node_type", "status", "nodes", "endpoint"}
	for i, r := range resources {
		for _, key := range requiredFields {
			if _, ok := r.Fields[key]; !ok {
				t.Errorf("resource[%d].Fields missing key %q", i, key)
			}
		}
	}

	// Verify first redis cluster
	r0 := resources[0]
	if r0.ID != "redis-prod-001" {
		t.Errorf("resource[0].ID: expected %q, got %q", "redis-prod-001", r0.ID)
	}
	if r0.Status != "available" {
		t.Errorf("resource[0].Status: expected %q, got %q", "available", r0.Status)
	}
	if r0.Fields["cluster_id"] != "redis-prod-001" {
		t.Errorf("resource[0].Fields[\"cluster_id\"]: expected %q, got %q", "redis-prod-001", r0.Fields["cluster_id"])
	}
	if r0.Fields["engine_version"] != "7.0.12" {
		t.Errorf("resource[0].Fields[\"engine_version\"]: expected %q, got %q", "7.0.12", r0.Fields["engine_version"])
	}
	if r0.Fields["node_type"] != "cache.r6g.large" {
		t.Errorf("resource[0].Fields[\"node_type\"]: expected %q, got %q", "cache.r6g.large", r0.Fields["node_type"])
	}
	if r0.Fields["nodes"] != "3" {
		t.Errorf("resource[0].Fields[\"nodes\"]: expected %q, got %q", "3", r0.Fields["nodes"])
	}
	if r0.Fields["endpoint"] != "redis-prod-001.abc123.clustercfg.use1.cache.amazonaws.com" {
		t.Errorf("resource[0].Fields[\"endpoint\"]: expected %q, got %q",
			"redis-prod-001.abc123.clustercfg.use1.cache.amazonaws.com", r0.Fields["endpoint"])
	}

	// Verify second redis cluster (no ConfigurationEndpoint)
	r1 := resources[1]
	if r1.ID != "redis-staging-001" {
		t.Errorf("resource[1].ID: expected %q, got %q", "redis-staging-001", r1.ID)
	}
	if r1.Fields["endpoint"] != "" {
		t.Errorf("resource[1].Fields[\"endpoint\"]: expected empty string, got %q", r1.Fields["endpoint"])
	}
}

func TestFetchRedisClusters_ErrorResponse(t *testing.T) {
	mock := &mockElastiCacheClient{
		output: nil,
		err:    fmt.Errorf("AWS API error: access denied"),
	}

	resources, err := awsclient.FetchRedisClusters(context.Background(), mock)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if resources != nil {
		t.Errorf("expected nil resources on error, got %d resources", len(resources))
	}
}

func TestFetchRedisClusters_EmptyResponse(t *testing.T) {
	mock := &mockElastiCacheClient{
		output: &elasticache.DescribeCacheClustersOutput{
			CacheClusters: []ectypes.CacheCluster{},
		},
	}

	resources, err := awsclient.FetchRedisClusters(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}
