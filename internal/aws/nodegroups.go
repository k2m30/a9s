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
	resource.Register("ng", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchNodeGroups(ctx, c.EKS, c.EKS, c.EKS)
	})
	resource.RegisterFieldKeys("ng", []string{"nodegroup_name", "cluster_name", "status", "instance_types", "desired_size"})
}

// FetchNodeGroups performs a three-step fetch:
// 1. ListClusters to get cluster names
// 2. ListNodegroups per cluster to get node group names
// 3. DescribeNodegroup per node group to get full details
func FetchNodeGroups(
	ctx context.Context,
	listClustersAPI EKSListClustersAPI,
	listNodegroupsAPI EKSListNodegroupsAPI,
	describeNodegroupAPI EKSDescribeNodegroupAPI,
) ([]resource.Resource, error) {
	// Step 1: List all clusters
	listOutput, err := listClustersAPI.ListClusters(ctx, &eks.ListClustersInput{})
	if err != nil {
		return nil, fmt.Errorf("listing EKS clusters: %w", err)
	}

	var resources []resource.Resource

	// Step 2: For each cluster, list its node groups
	for _, clusterName := range listOutput.Clusters {
		ngListOutput, err := listNodegroupsAPI.ListNodegroups(ctx, &eks.ListNodegroupsInput{
			ClusterName: aws.String(clusterName),
		})
		if err != nil {
			return nil, fmt.Errorf("listing node groups for cluster %s: %w", clusterName, err)
		}

		// Step 3: For each node group, describe it
		for _, ngName := range ngListOutput.Nodegroups {
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
				RawStruct:  ng,
			}

			resources = append(resources, r)
		}
	}

	return resources, nil
}
