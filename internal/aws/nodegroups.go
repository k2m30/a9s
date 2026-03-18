package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"

	"github.com/k2m30/a9s/internal/resource"
)

func init() {
	resource.Register("ng", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchNodeGroups(ctx, c.EKS, c.EKS, c.EKS)
	})
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
		return nil, err
	}

	var resources []resource.Resource

	// Step 2: For each cluster, list its node groups
	for _, clusterName := range listOutput.Clusters {
		ngListOutput, err := listNodegroupsAPI.ListNodegroups(ctx, &eks.ListNodegroupsInput{
			ClusterName: aws.String(clusterName),
		})
		if err != nil {
			return nil, err
		}

		// Step 3: For each node group, describe it
		for _, ngName := range ngListOutput.Nodegroups {
			descOutput, err := describeNodegroupAPI.DescribeNodegroup(ctx, &eks.DescribeNodegroupInput{
				ClusterName:   aws.String(clusterName),
				NodegroupName: aws.String(ngName),
			})
			if err != nil {
				return nil, err
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
			minSize := ""
			maxSize := ""
			if ng.ScalingConfig != nil {
				if ng.ScalingConfig.DesiredSize != nil {
					desiredSize = fmt.Sprintf("%d", *ng.ScalingConfig.DesiredSize)
				}
				if ng.ScalingConfig.MinSize != nil {
					minSize = fmt.Sprintf("%d", *ng.ScalingConfig.MinSize)
				}
				if ng.ScalingConfig.MaxSize != nil {
					maxSize = fmt.Sprintf("%d", *ng.ScalingConfig.MaxSize)
				}
			}

			// Build DetailData
			detail := map[string]string{
				"Node Group Name":    nodegroupName,
				"Cluster Name":      ngClusterName,
				"Status":            status,
				"Instance Types":    instanceTypes,
				"AMI Type":          string(ng.AmiType),
				"Capacity Type":     string(ng.CapacityType),
				"Desired Size":      desiredSize,
				"Min Size":          minSize,
				"Max Size":          maxSize,
				"Subnets":           strings.Join(ng.Subnets, ", "),
			}

			// Disk Size (nil if launch template used)
			if ng.DiskSize != nil {
				detail["Disk Size"] = fmt.Sprintf("%d", *ng.DiskSize)
			} else {
				detail["Disk Size"] = ""
			}

			// Node Role
			if ng.NodeRole != nil {
				detail["Node Role"] = *ng.NodeRole
			} else {
				detail["Node Role"] = ""
			}

			// Node Group ARN
			if ng.NodegroupArn != nil {
				detail["Node Group ARN"] = *ng.NodegroupArn
			} else {
				detail["Node Group ARN"] = ""
			}

			// Release Version
			if ng.ReleaseVersion != nil {
				detail["Release Version"] = *ng.ReleaseVersion
			} else {
				detail["Release Version"] = ""
			}

			// Kubernetes Version
			if ng.Version != nil {
				detail["Kubernetes Version"] = *ng.Version
			} else {
				detail["Kubernetes Version"] = ""
			}

			// Created At
			if ng.CreatedAt != nil {
				detail["Created At"] = ng.CreatedAt.Format("2006-01-02T15:04:05Z07:00")
			} else {
				detail["Created At"] = ""
			}

			// Labels
			for k, v := range ng.Labels {
				detail["Label: "+k] = v
			}

			// Tags
			for k, v := range ng.Tags {
				detail["Tag: "+k] = v
			}

			// Build RawJSON
			rawJSON := ""
			if jsonBytes, err := json.MarshalIndent(ng, "", "  "); err == nil {
				rawJSON = string(jsonBytes)
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
				DetailData: detail,
				RawJSON:    rawJSON,
				RawStruct:  ng,
			}

			resources = append(resources, r)
		}
	}

	return resources, nil
}
