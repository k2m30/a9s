package aws

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("ng", []string{"nodegroup_name", "cluster_name", "status", "instance_types", "desired_size", "health_issues_count", "health_issues"})

	resource.RegisterRelated("ng", []resource.RelatedDef{
		{TargetType: "eks", DisplayName: "EKS Clusters", Checker: checkNGEKS, NeedsTargetCache: true},
		{TargetType: "role", DisplayName: "IAM Roles", Checker: checkNGRole, NeedsTargetCache: true},
		{TargetType: "asg", DisplayName: "Auto Scaling Groups", Checker: checkNGASG, NeedsTargetCache: true},
		{TargetType: "ec2", DisplayName: "EC2 Instances", Checker: checkNGEC2, NeedsTargetCache: true},
	})

	resource.RegisterNavigableFields("ng", []resource.NavigableField{
		{FieldPath: "ClusterName", TargetType: "eks"},
		{FieldPath: "NodeRole", TargetType: "role"},
	})

	resource.RegisterPaginated("ng", func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}

		clusterInput := &eks.ListClustersInput{MaxResults: aws.Int32(DefaultPageSize)}
		if continuationToken != "" {
			clusterInput.NextToken = aws.String(continuationToken)
		}

		clusterOutput, err := c.EKS.ListClusters(ctx, clusterInput)
		if err != nil {
			return resource.FetchResult{}, fmt.Errorf("listing EKS clusters: %w", err)
		}

		moreClusters := clusterOutput.NextToken != nil
		moreNodegroups := false
		hitCap := false
		var resources []resource.Resource

		for _, cluster := range clusterOutput.Clusters {
			if hitCap {
				moreNodegroups = true
				break
			}
			ngOutput, err := c.EKS.ListNodegroups(ctx, &eks.ListNodegroupsInput{
				ClusterName: aws.String(cluster),
				MaxResults:  aws.Int32(DefaultPageSize),
			})
			if err != nil {
				continue
			}
			if ngOutput.NextToken != nil {
				moreNodegroups = true
			}
			for _, ngName := range ngOutput.Nodegroups {
				if len(resources) >= DefaultPageSize {
					hitCap = true
					moreNodegroups = true
					break
				}
				descOutput, err := c.EKS.DescribeNodegroup(ctx, &eks.DescribeNodegroupInput{
					ClusterName:   aws.String(cluster),
					NodegroupName: aws.String(ngName),
				})
				if err != nil || descOutput.Nodegroup == nil {
					continue
				}
				resources = append(resources, buildNodeGroupResource(cluster, ngName, descOutput.Nodegroup))
			}
		}

		isTruncated := moreClusters || moreNodegroups
		var nextToken string
		if clusterOutput.NextToken != nil {
			nextToken = *clusterOutput.NextToken
		}

		return resource.FetchResult{
			Resources: resources,
			Pagination: &resource.PaginationMeta{
				IsTruncated: isTruncated,
				NextToken:   nextToken,
				PageSize:    len(resources),
				TotalHint:   -1,
			},
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
			if descOutput.Nodegroup == nil {
				continue
			}
			resources = append(resources, buildNodeGroupResource(clusterName, ngName, descOutput.Nodegroup))
		}
	}

	return resources, nil
}

// buildNodeGroupResource constructs a Resource from cluster name, nodegroup name, and EKS Nodegroup struct.
func buildNodeGroupResource(clusterName, ngName string, ng *ekstypes.Nodegroup) resource.Resource {
	nodegroupName := ngName
	if ng.NodegroupName != nil {
		nodegroupName = *ng.NodegroupName
	}

	ngClusterName := clusterName
	if ng.ClusterName != nil {
		ngClusterName = *ng.ClusterName
	}

	status := string(ng.Status)
	instanceTypes := strings.Join(ng.InstanceTypes, ", ")

	desiredSize := ""
	if ng.ScalingConfig != nil && ng.ScalingConfig.DesiredSize != nil {
		desiredSize = fmt.Sprintf("%d", *ng.ScalingConfig.DesiredSize)
	}

	// Wave 2: health.issues[] — populated by DescribeNodegroup (called per node group in fetcher).
	healthIssuesCount := 0
	var issueCodes []string
	if ng.Health != nil {
		for _, issue := range ng.Health.Issues {
			healthIssuesCount++
			issueCodes = append(issueCodes, string(issue.Code))
		}
	}

	return resource.Resource{
		ID:     nodegroupName,
		Name:   nodegroupName,
		Status: status,
		Fields: map[string]string{
			"nodegroup_name":      nodegroupName,
			"cluster_name":        ngClusterName,
			"status":              status,
			"instance_types":      instanceTypes,
			"desired_size":        desiredSize,
			"health_issues_count": strconv.Itoa(healthIssuesCount),
			"health_issues":       strings.Join(issueCodes, ","),
		},
		RawStruct: ng,
	}
}
