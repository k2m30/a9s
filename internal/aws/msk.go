package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/kafka"

	"github.com/k2m30/a9s/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("msk", []string{"cluster_name", "cluster_type", "state", "version"})
	resource.Register("msk", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchMSKClusters(ctx, c.MSK)
	})
}

// FetchMSKClusters calls the MSK ListClustersV2 API and returns a slice of
// generic Resource structs.
func FetchMSKClusters(ctx context.Context, api MSKListClustersV2API) ([]resource.Resource, error) {
	output, err := api.ListClustersV2(ctx, &kafka.ListClustersV2Input{})
	if err != nil {
		return nil, fmt.Errorf("fetching MSK clusters: %w", err)
	}

	var resources []resource.Resource

	for _, cluster := range output.ClusterInfoList {
		clusterName := ""
		if cluster.ClusterName != nil {
			clusterName = *cluster.ClusterName
		}

		clusterType := string(cluster.ClusterType)
		state := string(cluster.State)

		version := ""
		if cluster.CurrentVersion != nil {
			version = *cluster.CurrentVersion
		}

		r := resource.Resource{
			ID:     clusterName,
			Name:   clusterName,
			Status: state,
			Fields: map[string]string{
				"cluster_name": clusterName,
				"cluster_type": clusterType,
				"state":        state,
				"version":      version,
			},
			RawStruct:  cluster,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
