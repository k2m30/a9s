package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/docdb"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.Register("dbc", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchDocDBClusters(ctx, c.DocDB)
	})
	resource.RegisterFieldKeys("dbc", []string{"cluster_id", "engine_version", "status", "instances", "endpoint"})
}

// FetchDocDBClusters calls the DescribeDBClusters API and converts
// the response into a slice of generic Resource structs.
// Returns all DB clusters (Aurora, DocumentDB, Neptune) — no engine filter.
func FetchDocDBClusters(ctx context.Context, api DocDBDescribeDBClustersAPI) ([]resource.Resource, error) {
	output, err := api.DescribeDBClusters(ctx, &docdb.DescribeDBClustersInput{})
	if err != nil {
		return nil, fmt.Errorf("fetching DocumentDB clusters: %w", err)
	}

	var resources []resource.Resource

	for _, cluster := range output.DBClusters {
		clusterID := ""
		if cluster.DBClusterIdentifier != nil {
			clusterID = *cluster.DBClusterIdentifier
		}

		engineVersion := ""
		if cluster.EngineVersion != nil {
			engineVersion = *cluster.EngineVersion
		}

		status := ""
		if cluster.Status != nil {
			status = *cluster.Status
		}

		instances := fmt.Sprintf("%d", len(cluster.DBClusterMembers))

		endpoint := ""
		if cluster.Endpoint != nil {
			endpoint = *cluster.Endpoint
		}

		r := resource.Resource{
			ID:     clusterID,
			Name:   clusterID,
			Status: status,
			Fields: map[string]string{
				"cluster_id":     clusterID,
				"engine_version": engineVersion,
				"status":         status,
				"instances":      instances,
				"endpoint":       endpoint,
			},
			RawStruct:  cluster,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
