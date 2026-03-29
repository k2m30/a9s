package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("ng", []string{"nodegroup_name", "cluster_name", "status", "instance_types", "desired_size"})

	resource.RegisterPaginated("ng", func(ctx context.Context, clients interface{}, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		resources, err := FetchNodeGroups(ctx, c.EKS, c.EKS, c.EKS)
		if err != nil {
			return resource.FetchResult{}, err
		}
		return resource.FetchResult{
			Resources:  resources,
			Pagination: &resource.PaginationMeta{IsTruncated: false, TotalHint: len(resources), PageSize: len(resources)},
		}, nil
	})
}

// FetchNodeGroups performs a three-step fetch:
// 1. ListClusters to get cluster names (paginated)
// 2. ListNodegroups per cluster to get node group names (paginated)
// 3. DescribeNodegroup per node group to get full details
func FetchNodeGroups(
	ctx context.Context,
	listClustersAPI EKSListClustersAPI,
	listNodegroupsAPI EKSListNodegroupsAPI,
	describeNodegroupAPI EKSDescribeNodegroupAPI,
) ([]resource.Resource, error) {
	// Step 1: List all clusters (paginated)
	var allClusters []string
	var clusterNextToken *string

	for {
		listOutput, err := listClustersAPI.ListClusters(ctx, &eks.ListClustersInput{
			NextToken: clusterNextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("listing EKS clusters: %w", err)
		}

		allClusters = append(allClusters, listOutput.Clusters...)

		if listOutput.NextToken == nil {
			break
		}
		clusterNextToken = listOutput.NextToken
	}

	var resources []resource.Resource

	// Step 2: For each cluster, list its node groups (paginated)
	for _, clusterName := range allClusters {
		var allNodegroups []string
		var ngNextToken *string

		for {
			ngListOutput, err := listNodegroupsAPI.ListNodegroups(ctx, &eks.ListNodegroupsInput{
				ClusterName: aws.String(clusterName),
				NextToken:   ngNextToken,
			})
			if err != nil {
				return nil, fmt.Errorf("listing node groups for cluster %s: %w", clusterName, err)
			}

			allNodegroups = append(allNodegroups, ngListOutput.Nodegroups...)

			if ngListOutput.NextToken == nil {
				break
			}
			ngNextToken = ngListOutput.NextToken
		}

		// Step 3: For each node group, describe it
		for _, ngName := range allNodegroups {
			descOutput, err := describeNodegroupAPI.DescribeNodegroup(ctx, &eks.DescribeNodegroupInput{
				ClusterName:   aws.String(clusterName),
				NodegroupName: aws.String(ngName),
			})
			if err != nil {
				return nil, fmt.Errorf("describing node group %s: %w", ngName, err)
			}

			ng := descOutput.Nodegroup

			nodegroupName := ""
			if ng.NodegroupName != nil {
				nodegroupName = *ng.NodegroupName
			}

			ngClusterName := ""
			if ng.ClusterName != nil {
				ngClusterName = *ng.ClusterName
			}

			status := string(ng.Status)

			instanceTypes := strings.Join(ng.InstanceTypes, ", ")

			// Guard for nil ScalingConfig
			desiredSize := ""
			if ng.ScalingConfig != nil {
				if ng.ScalingConfig.DesiredSize != nil {
					desiredSize = fmt.Sprintf("%d", *ng.ScalingConfig.DesiredSize)
				}
			}

			r := resource.Resource{
				ID:     nodegroupName,
				Name:   nodegroupName,
				Status: status,
				Fields: map[string]string{
					"nodegroup_name": nodegroupName,
					"cluster_name":   ngClusterName,
					"status":         status,
					"instance_types": instanceTypes,
					"desired_size":   desiredSize,
				},
				RawStruct: ng,
			}

			resources = append(resources, r)
		}
	}

	return resources, nil
}
