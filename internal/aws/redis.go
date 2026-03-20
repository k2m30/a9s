package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"

	"github.com/k2m30/a9s/internal/resource"
)

func init() {
	resource.Register("redis", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchRedisClusters(ctx, c.ElastiCache)
	})
	resource.RegisterFieldKeys("redis", []string{"cluster_id", "engine_version", "node_type", "status", "nodes", "endpoint"})
}

// FetchRedisClusters calls the ElastiCache DescribeCacheClusters API and converts
// the response into a slice of generic Resource structs.
// Only clusters with engine "redis" are returned (client-side filter).
func FetchRedisClusters(ctx context.Context, api ElastiCacheDescribeCacheClustersAPI) ([]resource.Resource, error) {
	output, err := api.DescribeCacheClusters(ctx, &elasticache.DescribeCacheClustersInput{
		ShowCacheNodeInfo: aws.Bool(true),
	})
	if err != nil {
		return nil, fmt.Errorf("fetching Redis clusters: %w", err)
	}

	var resources []resource.Resource

	for _, cluster := range output.CacheClusters {
		// Client-side filter: only include redis clusters
		if cluster.Engine == nil || *cluster.Engine != "redis" {
			continue
		}

		clusterID := ""
		if cluster.CacheClusterId != nil {
			clusterID = *cluster.CacheClusterId
		}

		engineVersion := ""
		if cluster.EngineVersion != nil {
			engineVersion = *cluster.EngineVersion
		}

		nodeType := ""
		if cluster.CacheNodeType != nil {
			nodeType = *cluster.CacheNodeType
		}

		status := ""
		if cluster.CacheClusterStatus != nil {
			status = *cluster.CacheClusterStatus
		}

		nodes := "0"
		if cluster.NumCacheNodes != nil {
			nodes = fmt.Sprintf("%d", *cluster.NumCacheNodes)
		}

		endpoint := ""
		if cluster.ConfigurationEndpoint != nil && cluster.ConfigurationEndpoint.Address != nil {
			endpoint = *cluster.ConfigurationEndpoint.Address
		}

		r := resource.Resource{
			ID:     clusterID,
			Name:   clusterID,
			Status: status,
			Fields: map[string]string{
				"cluster_id":     clusterID,
				"engine_version": engineVersion,
				"node_type":      nodeType,
				"status":         status,
				"nodes":          nodes,
				"endpoint":       endpoint,
			},
			RawStruct:  cluster,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
