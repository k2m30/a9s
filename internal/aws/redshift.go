package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/service/redshift"

	"github.com/k2m30/a9s/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("redshift", []string{"cluster_id", "status", "node_type", "num_nodes", "db_name", "endpoint"})
	resource.Register("redshift", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchRedshiftClusters(ctx, c.Redshift)
	})
}

// FetchRedshiftClusters calls the Redshift DescribeClusters API and converts the
// response into a slice of generic Resource structs.
func FetchRedshiftClusters(ctx context.Context, api RedshiftDescribeClustersAPI) ([]resource.Resource, error) {
	output, err := api.DescribeClusters(ctx, &redshift.DescribeClustersInput{})
	if err != nil {
		return nil, err
	}

	var resources []resource.Resource

	for _, cluster := range output.Clusters {
		clusterID := ""
		if cluster.ClusterIdentifier != nil {
			clusterID = *cluster.ClusterIdentifier
		}

		status := ""
		if cluster.ClusterStatus != nil {
			status = *cluster.ClusterStatus
		}

		nodeType := ""
		if cluster.NodeType != nil {
			nodeType = *cluster.NodeType
		}

		numNodes := ""
		if cluster.NumberOfNodes != nil {
			numNodes = strconv.Itoa(int(*cluster.NumberOfNodes))
		}

		dbName := ""
		if cluster.DBName != nil {
			dbName = *cluster.DBName
		}

		endpoint := ""
		if cluster.Endpoint != nil && cluster.Endpoint.Address != nil {
			endpoint = *cluster.Endpoint.Address
		}

		masterUser := ""
		if cluster.MasterUsername != nil {
			masterUser = *cluster.MasterUsername
		}

		createTime := ""
		if cluster.ClusterCreateTime != nil {
			createTime = cluster.ClusterCreateTime.Format("2006-01-02 15:04:05")
		}

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
				"cluster_id":  clusterID,
				"status":      status,
				"node_type":   nodeType,
				"num_nodes":   numNodes,
				"db_name":     dbName,
				"endpoint":    endpoint,
				"master_user": masterUser,
				"create_time": createTime,
			},
			RawJSON:   rawJSON,
			RawStruct: cluster,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
