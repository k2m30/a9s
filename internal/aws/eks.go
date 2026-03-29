package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("eks", []string{"cluster_name", "version", "status", "endpoint", "platform_version"})

	resource.RegisterPaginated("eks", func(ctx context.Context, clients interface{}, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		resources, err := FetchEKSClusters(ctx, c.EKS, c.EKS)
		if err != nil {
			return resource.FetchResult{}, err
		}
		return resource.FetchResult{
			Resources:  resources,
			Pagination: &resource.PaginationMeta{IsTruncated: false, TotalHint: len(resources), PageSize: len(resources)},
		}, nil
	})
}

// FetchEKSClusters performs a two-step fetch: ListClusters to get cluster names
// (paginated via NextToken), then DescribeCluster for each name to get full details.
func FetchEKSClusters(ctx context.Context, listAPI EKSListClustersAPI, describeAPI EKSDescribeClusterAPI) ([]resource.Resource, error) {
	// Step 1: Collect all cluster names across pages
	var allClusters []string
	var nextToken *string

	for {
		listOutput, err := listAPI.ListClusters(ctx, &eks.ListClustersInput{
			NextToken: nextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("listing EKS clusters: %w", err)
		}

		allClusters = append(allClusters, listOutput.Clusters...)

		if listOutput.NextToken == nil {
			break
		}
		nextToken = listOutput.NextToken
	}

	// Step 2: Describe each cluster
	var resources []resource.Resource

	for _, clusterName := range allClusters {
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
			RawStruct: cluster,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
