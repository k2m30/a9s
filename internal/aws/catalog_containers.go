package aws

import (
	"context"
	"fmt"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"

	"github.com/k2m30/a9s/v3/internal/catalog"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func colorEKSCluster(r domain.Resource) domain.Color {
	if c, ok := colorFromWave1(r); ok {
		return c
	}
	if r.Fields["status"] == "FAILED" {
		return domain.ColorBroken
	}
	hasIssues := false
	if n, err := strconv.Atoi(r.Fields["health_issues_count"]); err == nil && n > 0 {
		hasIssues = true
	}
	switch r.Fields["status"] {
	case "ACTIVE":
		if hasIssues {
			return domain.ColorWarning
		}
		return domain.ColorHealthy
	case "CREATING", "UPDATING", "DELETING":
		return domain.ColorWarning
	}
	if hasIssues {
		return domain.ColorWarning
	}
	return domain.ColorHealthy
}

func colorEKSNodeGroup(r domain.Resource) domain.Color {
	if c, ok := colorFromWave1(r); ok {
		return c
	}
	hasIssues := false
	if n, err := strconv.Atoi(r.Fields["health_issues_count"]); err == nil && n > 0 {
		hasIssues = true
	}
	switch r.Fields["status"] {
	case "ACTIVE":
		if hasIssues {
			return domain.ColorWarning
		}
		return domain.ColorHealthy
	case "CREATING", "UPDATING", "DELETING":
		return domain.ColorWarning
	case "CREATE_FAILED", "DELETE_FAILED", "DEGRADED":
		return domain.ColorBroken
	}
	if hasIssues {
		return domain.ColorWarning
	}
	return domain.ColorHealthy
}

var containersTypes = []catalog.ResourceTypeDef{
	{
		Name:          "EKS Clusters",
		ShortName:     "eks",
		Aliases:       []string{"eks", "kubernetes", "k8s"},
		Category:      "CONTAINERS",
		CloudTrailKey: "ResourceName:Fields.arn",
		LifecycleKey:  "status",
		Columns: []domain.Column{
			{Key: "cluster_name", Title: "Cluster Name", Width: 28, Sortable: true},
			{Key: "version", Title: "Version", Width: 10, Sortable: true},
			{Key: "status", Title: "Status", Width: 14, Sortable: true},
			{Key: "endpoint", Title: "Endpoint", Width: 48, Sortable: false},
			{Key: "platform_version", Title: "Platform Version", Width: 18, Sortable: true},
		},
		Color: colorEKSCluster,
	},
	{
		Name:          "EKS Node Groups",
		ShortName:     "ng",
		Aliases:       []string{"ng", "nodegroups", "node-groups"},
		Category:      "CONTAINERS",
		CloudTrailKey: "ResourceName:ID",
		LifecycleKey:  "status",
		Columns: []domain.Column{
			{Key: "nodegroup_name", Title: "Node Group", Width: 28, Sortable: true},
			{Key: "cluster_name", Title: "Cluster", Width: 24, Sortable: true},
			{Key: "status", Title: "Status", Width: 14, Sortable: true},
			{Key: "instance_types", Title: "Instance Types", Width: 20, Sortable: false},
			{Key: "desired_size", Title: "Desired", Width: 9, Sortable: true},
		},
		Color:   colorEKSNodeGroup,
		Fetcher: fetchNodeGroupsPage,
		FieldKeys: []string{
			"nodegroup_name", "cluster_name", "status", "instance_types",
			"desired_size", "health_issues_count", "health_issues", "image_id",
		},
		Related: []domain.RelatedDef{
			{TargetType: "eks", DisplayName: "EKS Clusters", Checker: checkNGEKS, NeedsTargetCache: true},
			{TargetType: "role", DisplayName: "IAM Roles", Checker: checkNGRole, NeedsTargetCache: true},
			{TargetType: "asg", DisplayName: "Auto Scaling Groups", Checker: checkNGASG, NeedsTargetCache: true},
			{TargetType: "ec2", DisplayName: "EC2 Instances", Checker: checkNGEC2, NeedsTargetCache: true},
			{TargetType: "sg", DisplayName: "Security Groups", Checker: checkNGSG},
			{TargetType: "ami", DisplayName: "AMI", Checker: checkNGAMI},
			{TargetType: "ebs", DisplayName: "EBS Volumes", Checker: checkNGEBS},
			{TargetType: "subnet", DisplayName: "Subnets", Checker: checkNGSubnet},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("ng")},
		},
		Navigable: []domain.NavigableField{
			{FieldPath: "ClusterName", TargetType: "eks"},
			{FieldPath: "NodeRole", TargetType: "role"},
			{FieldPath: "Subnets", TargetType: "subnet"},
		},
	},
}

// fetchNodeGroupsPage is the registered Wave 1 fetcher for the ng resource
// type. It walks ListClusters → ListNodegroups → DescribeNodegroup with
// per-call retry-on-throttle, capping the page at DefaultPageSize so the
// background fetcher pool keeps a bounded blast radius regardless of how many
// clusters/nodegroups exist in the account.
func fetchNodeGroupsPage(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil {
		return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
	}

	clusterInput := &eks.ListClustersInput{MaxResults: aws.Int32(DefaultPageSize)}
	if continuationToken != "" {
		clusterInput.NextToken = aws.String(continuationToken)
	}

	clusterOutput, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*eks.ListClustersOutput, error) {
		return c.EKS.ListClusters(ctx, clusterInput)
	})
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("listing EKS clusters: %w", err)
	}

	moreClusters := clusterOutput.NextToken != nil
	moreNodegroups := false
	hitCap := false
	var resources []resource.Resource
	var failures []string
	totalAttempted := 0

	for _, cluster := range clusterOutput.Clusters {
		if hitCap {
			moreNodegroups = true
			break
		}
		ngOutput, ngErr := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*eks.ListNodegroupsOutput, error) {
			return c.EKS.ListNodegroups(ctx, &eks.ListNodegroupsInput{
				ClusterName: aws.String(cluster),
				MaxResults:  aws.Int32(DefaultPageSize),
			})
		})
		if ngErr != nil {
			failures = append(failures, fmt.Sprintf("%s: %s", cluster, ngErr.Error()))
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
			totalAttempted++
			descOutput, descErr := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*eks.DescribeNodegroupOutput, error) {
				return c.EKS.DescribeNodegroup(ctx, &eks.DescribeNodegroupInput{
					ClusterName:   aws.String(cluster),
					NodegroupName: aws.String(ngName),
				})
			})
			if descErr != nil {
				failures = append(failures, fmt.Sprintf("%s/%s: %s", cluster, ngName, descErr.Error()))
				continue
			}
			if descOutput.Nodegroup == nil {
				failures = append(failures, fmt.Sprintf("%s/%s: nil nodegroup in response", cluster, ngName))
				continue
			}
			res := buildNodeGroupResource(cluster, ngName, descOutput.Nodegroup)
			if lt := descOutput.Nodegroup.LaunchTemplate; lt != nil && lt.Id != nil {
				res.Fields["image_id"] = resolveNGImageID(ctx, c.EC2, lt)
			}
			resources = append(resources, res)
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
	}, AggregateFailures("ng: DescribeNodegroup", failures, totalAttempted)
}
