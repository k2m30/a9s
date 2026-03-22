package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.Register("eks", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchEKSClusters(ctx, c.EKS, c.EKS)
	})
	resource.RegisterFieldKeys("eks", []string{"cluster_name", "version", "status", "endpoint", "platform_version"})
}

// FetchEKSClusters performs a two-step fetch: ListClusters to get cluster names,
// then DescribeCluster for each name to get full details.
func FetchEKSClusters(ctx context.Context, listAPI EKSListClustersAPI, describeAPI EKSDescribeClusterAPI) ([]resource.Resource, error) {
	listOutput, err := listAPI.ListClusters(ctx, &eks.ListClustersInput{})
	if err != nil {
		return nil, fmt.Errorf("listing EKS clusters: %w", err)
	}

	var resources []resource.Resource

	for _, clusterName := range listOutput.Clusters {
		descOutput, err := describeAPI.DescribeCluster(ctx, &eks.DescribeClusterInput{
			Name: aws.String(clusterName),
		})
		if err != nil {
			return nil, fmt.Errorf("describing EKS cluster %s: %w", clusterName, err)
		}

		cluster := descOutput.Cluster

		name := ""
		if cluster.Name != nil {
			name = *cluster.Name
		}

		version := ""
		if cluster.Version != nil {
			version = *cluster.Version
		}

		status := string(cluster.Status)

		endpoint := ""
		if cluster.Endpoint != nil {
			endpoint = *cluster.Endpoint
		}

		platformVersion := ""
		if cluster.PlatformVersion != nil {
			platformVersion = *cluster.PlatformVersion
		}

		r := resource.Resource{
			ID:     name,
			Name:   name,
			Status: status,
			Fields: map[string]string{
				"cluster_name":     name,
				"version":          version,
				"status":           status,
				"endpoint":         endpoint,
				"platform_version": platformVersion,
			},
			RawStruct:  cluster,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
