package aws

import (
	"context"
	"fmt"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"

	"github.com/k2m30/a9s/v3/internal/catalog"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// colorEKSCluster classifies an EKS cluster. Prefers colorFromWave1 so real
// fetched resources (Findings populated by buildEKSResource, eks.go, Source:
// "wave1") color from their own Finding; the raw-field checks below are the
// identical-precedence fallback for callers that construct a Resource with
// only Fields set (e.g. qa_eks_color_test.go).
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
		Color:   colorEKSCluster,
		Fetcher: fetcherWithClients(FetchEKSClustersPage),
		FieldKeys: []string{
			"cluster_name", "version", "status", "endpoint", "platform_version",
			"arn", "health_issues_count", "health_issues", "subnet_ids",
		},
		// In-fetcher Wave 2: the eks fetcher already issues per-cluster
		// DescribeCluster calls and populates health_issues_count / health_issues
		// at fetch time. InFetcherWave2Sentinel makes the contract explicit so
		// TestAttentionSignalsDoc sees a Wave 2 wiring.
		Wave2: IssueEnricher{Fn: InFetcherWave2Sentinel, Priority: 100},
		Related: []domain.RelatedDef{
			{TargetType: "ng", DisplayName: "Node Groups", Checker: checkEKSNodeGroups, NeedsTargetCache: true},
			{TargetType: "alarm", DisplayName: "CloudWatch Alarms", Checker: checkEKSAlarms, NeedsTargetCache: true},
			{TargetType: "cfn", DisplayName: "CloudFormation Stacks", Checker: checkEKSCFN, NeedsTargetCache: true},
			{TargetType: "logs", DisplayName: "Log Groups", Checker: checkEKSLogs, NeedsTargetCache: true},
			{TargetType: "sg", DisplayName: "Security Groups", Checker: checkEKSSG},
			{TargetType: "vpc", DisplayName: "VPC", Checker: checkEKSVPC},
			{TargetType: "role", DisplayName: "IAM Role", Checker: checkEKSRole},
			{TargetType: "kms", DisplayName: "KMS Key", Checker: checkEKSKMS},
			{TargetType: "subnet", DisplayName: "Subnets", Checker: checkEKSSubnet},
			{TargetType: "ami", DisplayName: "AMI", Checker: checkEKSAMI},
			{TargetType: "asg", DisplayName: "Auto Scaling Groups", Checker: checkEKSASG, NeedsTargetCache: true},
			{TargetType: "ec2", DisplayName: "EC2 Instances", Checker: checkEKSEC2},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: checkEKSCTEvents, NeedsTargetCache: true},
		},
		Navigable: []domain.NavigableField{
			{FieldPath: "ResourcesVpcConfig.VpcId", TargetType: "vpc"},
			{FieldPath: "ResourcesVpcConfig.ClusterSecurityGroupId", TargetType: "sg"},
			{FieldPath: "ResourcesVpcConfig.SubnetIds", TargetType: "subnet"},
			{FieldPath: "ResourcesVpcConfig.SecurityGroupIds", TargetType: "sg"},
			{FieldPath: "RoleArn", TargetType: "role"},
		},
		Findings: []catalog.FindingDef{
			{Code: CodeEKSStateCreating, Phrase: "creating", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeEKSStateUpdating, Phrase: "updating", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeEKSStateFailed, Phrase: "failed", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeEKSHealthIssue, Phrase: "issue: <Issue.Code>", Severity: domain.SevWarn, Source: "wave1"},
			DetailsDeniedFindingDef("eks"),
		},
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
		// In-fetcher Wave 2: the ng fetcher already issues per-node-group
		// DescribeNodegroup calls and populates health_issues_count /
		// health_issues at fetch time. InFetcherWave2Sentinel records the contract
		// explicitly so TestAttentionSignalsDoc sees a Wave 2 wiring.
		Wave2: IssueEnricher{Fn: InFetcherWave2Sentinel, Priority: 100},
		FieldKeys: []string{
			"nodegroup_name", "cluster_name", "status", "instance_types",
			"desired_size", "health_issues_count", "health_issues", "image_id",
		},
		Related: []domain.RelatedDef{
			{TargetType: "eks", DisplayName: "EKS Clusters", Checker: checkNGEKS, NeedsTargetCache: true},
			{TargetType: "role", DisplayName: "IAM Roles", Checker: checkNGRole},
			{TargetType: "asg", DisplayName: "Auto Scaling Groups", Checker: checkNGASG, NeedsTargetCache: true},
			{TargetType: "ec2", DisplayName: "EC2 Instances", Checker: checkNGEC2},
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
		Findings: []catalog.FindingDef{
			{Code: CodeNGStateCreating, Phrase: "creating", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeNGStateUpdating, Phrase: "updating", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeNGStateDeleting, Phrase: "deleting", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeNGStateCreateFailed, Phrase: "create failed", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeNGStateDeleteFailed, Phrase: "delete failed", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeNGStateDegraded, Phrase: "degraded", Severity: domain.SevBroken, Source: "wave1"},
			DetailsDeniedFindingDef("ng"),
		},
	},
}

// fetchNodeGroupsPage is the registered Wave 1 fetcher for the ng resource
// type. It walks ListClusters → ListNodegroups → DescribeNodegroup with
// per-call retry-on-throttle, capping the page at DefaultPageSize so the
// background fetcher pool keeps a bounded blast radius regardless of how many
// clusters/nodegroups exist in the account.
func fetchNodeGroupsPage(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
	c, err := svcClients(clients)
	if err != nil {
		return resource.FetchResult{}, err
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
				resources = append(resources, DegradedDetailsDenied("ng", ngName, &ekstypes.Nodegroup{ClusterName: aws.String(cluster), NodegroupName: aws.String(ngName)}))
				continue
			}
			if descOutput.Nodegroup == nil {
				failures = append(failures, fmt.Sprintf("%s/%s: nil nodegroup in response", cluster, ngName))
				resources = append(resources, DegradedDetailsDenied("ng", ngName, &ekstypes.Nodegroup{ClusterName: aws.String(cluster), NodegroupName: aws.String(ngName)}))
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

// containersChildTypes is the declarative child-type catalog for the CONTAINERS
// category. Each per-category `<cat>ChildTypes` slice is appended to
// allChildTypes() in install.go.
var containersChildTypes = []catalog.ResourceTypeDef{ //nolint:gochecknoglobals // static catalog: intentional package-level var
	{
		Name:      "ECR Images",
		ShortName: "ecr_images",
		Columns:   resource.ECRImageColumns(),
		CopyField: "image_uri",
		Color:     colorWave1OrHealthy,
		FieldKeys: []string{
			"image_tags", "digest_short", "pushed_at", "image_size",
			"scan_status", "finding_counts", "image_uri", "image_digest",
			"repository_name",
		},
		ChildFetcher: childFetcherWithClients(func(ctx context.Context, c *ServiceClients, parentCtx resource.ParentContext, continuationToken string) (resource.FetchResult, error) {
			return FetchECRImages(ctx, c.ECR, parentCtx, continuationToken)
		}),
		Findings: []catalog.FindingDef{
			{Code: CodeECRImageScanFailed, Phrase: "scan failed", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeECRImageCritical, Phrase: "<N> critical vulnerabilities", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeECRImageHigh, Phrase: "<N> high vulnerabilities", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeECRImageUntagged, Phrase: "untagged", Severity: domain.SevDim, Source: "wave1"},
		},
	},
	{
		Name:      "Service Tasks",
		ShortName: "ecs_tasks",
		Columns:   resource.EcsSvcTaskColumns(),
		FieldKeys: []string{
			"task_id_short", "status", "health", "task_def_short",
			"started_at", "stopped_reason", "stop_code",
		},
		Color: func(r domain.Resource) domain.Color {
			// Structural broken overrides (precedence over wave1).
			if r.Fields["health"] == "UNHEALTHY" {
				return domain.ColorBroken
			}
			if r.Fields["status"] == "STOPPED" {
				sc := r.Fields["stop_code"]
				if sc != "" && sc != "UserInitiated" {
					return domain.ColorBroken
				}
			}
			// Wave1 Findings.
			for _, f := range r.Findings {
				if f.Source == "wave1" {
					return resource.ColorFromSeverity(f.Severity)
				}
			}
			// Structural lifecycle fallback.
			switch r.Fields["status"] {
			case "RUNNING":
				return domain.ColorHealthy
			case "STOPPED":
				return domain.ColorDim
			case "PROVISIONING", "PENDING", "ACTIVATING", "DEACTIVATING", "STOPPING", "DEPROVISIONING":
				return domain.ColorWarning
			}
			return domain.ColorHealthy
		},
		ChildFetcher: childFetcherWithClients(func(ctx context.Context, c *ServiceClients, parentCtx resource.ParentContext, continuationToken string) (resource.FetchResult, error) {
			return FetchEcsSvcTasks(ctx, c.ECS, c.ECS, parentCtx["cluster"], parentCtx["service_name"], continuationToken)
		}),
	},
	{
		Name:      "Service Events",
		ShortName: "ecs_svc_events",
		Columns:   resource.EcsSvcEventColumns(),
		FieldKeys: []string{"timestamp", "message"},
		ChildFetcher: childFetcherWithClients(func(ctx context.Context, c *ServiceClients, parentCtx resource.ParentContext, continuationToken string) (resource.FetchResult, error) {
			return FetchEcsSvcEvents(ctx, c.ECS, parentCtx["cluster"], parentCtx["service_name"], continuationToken)
		}),
	},
	{
		Name:      "Service Logs",
		ShortName: "ecs_svc_logs",
		Columns:   resource.EcsSvcLogColumns(),
		Color:     colorWave1OrHealthy,
		FieldKeys: []string{"timestamp", "stream_short", "message"},
		ChildFetcher: childFetcherWithClients(func(ctx context.Context, c *ServiceClients, parentCtx resource.ParentContext, continuationToken string) (resource.FetchResult, error) {
			return FetchEcsSvcLogs(ctx, c.ECS, c.CloudWatchLogs, parentCtx["cluster"], parentCtx["service_name"], parentCtx["task_definition"], continuationToken)
		}),
		Findings: []catalog.FindingDef{
			{Code: CodeCWLogError, Phrase: "error", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeCWLogWarn, Phrase: "warning", Severity: domain.SevWarn, Source: "wave1"},
		},
	},
}
