package aws

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/docdb"

	"github.com/k2m30/a9s/internal/resource"
)

// FetchDocDBClusters calls the DocumentDB DescribeDBClusters API and converts
// the response into a slice of generic Resource structs.
func FetchDocDBClusters(ctx context.Context, api DocDBDescribeDBClustersAPI) ([]resource.Resource, error) {
	output, err := api.DescribeDBClusters(ctx, &docdb.DescribeDBClustersInput{})
	if err != nil {
		return nil, err
	}

	var resources []resource.Resource

	for _, cluster := range output.DBClusters {
		clusterID := ""
		if cluster.DBClusterIdentifier != nil {
			clusterID = *cluster.DBClusterIdentifier
		}

		engineStr := ""
		if cluster.Engine != nil {
			engineStr = *cluster.Engine
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

		// Build DetailData
		detail := map[string]string{
			"Cluster ID":     clusterID,
			"Engine":         engineStr,
			"Engine Version": engineVersion,
			"Status":         status,
			"Endpoint":       endpoint,
			"Instance Count": instances,
		}

		// Reader Endpoint
		readerEndpoint := ""
		if cluster.ReaderEndpoint != nil {
			readerEndpoint = *cluster.ReaderEndpoint
		}
		detail["Reader Endpoint"] = readerEndpoint

		// Port
		port := ""
		if cluster.Port != nil {
			port = fmt.Sprintf("%d", *cluster.Port)
		}
		detail["Port"] = port

		// Storage Encrypted
		storageEncrypted := "No"
		if cluster.StorageEncrypted != nil && *cluster.StorageEncrypted {
			storageEncrypted = "Yes"
		}
		detail["Storage Encrypted"] = storageEncrypted

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
				"status":         status,
				"instances":      instances,
				"endpoint":       endpoint,
			},
			DetailData: detail,
			RawJSON:    rawJSON,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
