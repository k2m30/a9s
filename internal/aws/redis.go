package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("redis", []string{"cluster_id", "engine_version", "node_type", "status", "nodes", "endpoint"})

	resource.RegisterPaginated("redis", func(ctx context.Context, clients interface{}, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchRedisClustersPage(ctx, c.ElastiCache, continuationToken)
	})
}

// FetchRedisClusters calls the ElastiCache DescribeCacheClusters API and converts
// the response into a slice of generic Resource structs.
// Only clusters with engine "redis" are returned (client-side filter).
func FetchRedisClusters(ctx context.Context, api ElastiCacheDescribeCacheClustersAPI) ([]resource.Resource, error) {
	var all []resource.Resource
	token := ""
	for {
		result, err := FetchRedisClustersPage(ctx, api, token)
		if err != nil {
			return nil, err
		}
		all = append(all, result.Resources...)
		if result.Pagination == nil || !result.Pagination.IsTruncated {
			break
		}
		token = result.Pagination.NextToken
	}
	return all, nil
}

// FetchRedisClustersPage fetches a single page of Redis clusters.
func FetchRedisClustersPage(ctx context.Context, api ElastiCacheDescribeCacheClustersAPI, continuationToken string) (resource.FetchResult, error) {
	input := &elasticache.DescribeCacheClustersInput{
		ShowCacheNodeInfo: aws.Bool(true),
	}
	if continuationToken != "" {
		input.Marker = &continuationToken
	}

	output, err := api.DescribeCacheClusters(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching Redis clusters: %w", err)
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
			RawStruct: cluster,
		}

		resources = append(resources, r)
	}

	nextToken := ""
	isTruncated := false
	if output.Marker != nil {
		nextToken = *output.Marker
		isTruncated = true
	}

	totalHint := len(resources)
	if isTruncated {
		totalHint = -1
	}

	return resource.FetchResult{
		Resources: resources,
		Pagination: &resource.PaginationMeta{
			IsTruncated: isTruncated,
			NextToken:   nextToken,
			PageSize:    len(resources),
			TotalHint:   totalHint,
		},
	}, nil
}
