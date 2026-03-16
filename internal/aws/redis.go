package aws

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"

	"github.com/k2m30/a9s/internal/resource"
)

// FetchRedisClusters calls the ElastiCache DescribeCacheClusters API and converts
// the response into a slice of generic Resource structs.
// Only clusters with engine "redis" are returned (client-side filter).
func FetchRedisClusters(ctx context.Context, api ElastiCacheDescribeCacheClustersAPI) ([]resource.Resource, error) {
	output, err := api.DescribeCacheClusters(ctx, &elasticache.DescribeCacheClustersInput{
		ShowCacheNodeInfo: aws.Bool(true),
	})
	if err != nil {
		return nil, err
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

		engineStr := ""
		if cluster.Engine != nil {
			engineStr = *cluster.Engine
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

		port := ""
		if cluster.ConfigurationEndpoint != nil && cluster.ConfigurationEndpoint.Port != nil {
			port = fmt.Sprintf("%d", *cluster.ConfigurationEndpoint.Port)
		}

		// Build DetailData
		detail := map[string]string{
			"Cluster ID":     clusterID,
			"Engine":         engineStr,
			"Engine Version": engineVersion,
			"Status":         status,
			"Node Type":      nodeType,
			"Num Nodes":      nodes,
			"Endpoint":       endpoint,
			"Port":           port,
		}

		// Preferred AZ
		preferredAZ := ""
		if cluster.PreferredAvailabilityZone != nil {
			preferredAZ = *cluster.PreferredAvailabilityZone
		}
		detail["Preferred AZ"] = preferredAZ

		// Cache Subnet Group
		cacheSubnetGroup := ""
		if cluster.CacheSubnetGroupName != nil {
			cacheSubnetGroup = *cluster.CacheSubnetGroupName
		}
		detail["Cache Subnet Group"] = cacheSubnetGroup

		// Build RawJSON
		rawJSON := ""
		if jsonBytes, err := json.MarshalIndent(cluster, "", "  "); err == nil {
			rawJSON = string(jsonBytes)
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
			DetailData: detail,
			RawJSON:    rawJSON,
			RawStruct:  cluster,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
