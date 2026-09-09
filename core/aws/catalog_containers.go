// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"

	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/consolelink"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

func colorEKSCluster(r domain.Resource) domain.Color {
	if c, ok := colorFromAnyFinding(r); ok {
		return c
	}
	issues, _ := strconv.Atoi(r.Fields["health_issues_count"])
	findings, _ := eksClusterFindings(
		r.Fields["status"], r.Fields[DegradedFindingField], issues, nil, r.Fields["version"], eksPosture{
			PublicEndpoint:      r.Fields["public_endpoint"],
			ControlPlaneLogging: r.Fields["control_plane_logging"],
			SecretsEncryption:   r.Fields["secrets_encryption"],
			VersionSupport:      r.Fields["version_support"],
		})
	return colorFromFindings(findings)
}

func colorEKSNodeGroup(r domain.Resource) domain.Color {
	if c, ok := colorFromAnyFinding(r); ok {
		return c
	}
	issues, _ := strconv.Atoi(r.Fields["health_issues_count"])
	findings, _ := ngFindings(r.Fields["status"], r.Fields[DegradedFindingField], issues, nil)
	return colorFromFindings(findings)
}

var containersTypes = []catalog.ResourceTypeDef{
	{
		Name:           "EKS Clusters",
		ShortName:      "eks",
		HumanizeFields: []string{"status"},
		Aliases:        []string{"eks", "kubernetes", "k8s"},
		Category:       "CONTAINERS",
		CloudTrailKey:  "ResourceName:Fields.arn",
		LifecycleKey:   "status",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			return consolelink.Regional(region, "eks/home?region="+region+"#/clusters/"+url.PathEscape(r.ID))
		},
		Columns: []domain.Column{
			{Key: "cluster_name", Title: "Cluster Name", Path: "Name", Width: 28},
			{Key: "version", Title: "Version", Path: "Version", Width: 10},
			{Key: "status", Title: "Status", Path: "Status", Width: 14},
			{Key: "endpoint", Title: "Endpoint", Path: "Endpoint", Width: 48},
			{Key: "platform_version", Title: "Platform Version", Path: "PlatformVersion", Width: 18},
		},
		Color:   colorEKSCluster,
		Fetcher: fetcherWithClients(FetchEKSClustersPage),
		FieldKeys: []string{
			"cluster_name", "version", "status", "endpoint", "platform_version",
			"arn", "health_issues_count", "health_issues", "subnet_ids",
		},
		// In-fetcher Wave 2: the eks fetcher already issues per-cluster
		// DescribeCluster calls and populates health_issues_count / health_issues
		// at fetch time. InFetcherWave2Sentinel records that contract in the
		// catalog; see its doc comment in issue_enrichment.go for what does
		// (and does not) guard this wiring.
		Wave2: IssueEnricher{Fn: InFetcherWave2Sentinel, Priority: 100},
		Related: []domain.RelatedDef{
			{TargetType: "ng", DisplayName: "Node Groups", Checker: checkEKSNodeGroups, NeedsTargetCache: true, Truncated: true},
			{TargetType: "alarm", DisplayName: "CloudWatch Alarms", Checker: checkEKSAlarms, NeedsTargetCache: true, Truncated: true},
			{TargetType: "cfn", DisplayName: "CloudFormation Stacks", Checker: checkEKSCFN, NeedsTargetCache: true, Truncated: true},
			{TargetType: "logs", DisplayName: "Log Groups", Checker: checkEKSLogs, NeedsTargetCache: true, Truncated: true},
			{TargetType: "sg", DisplayName: "Security Groups", Checker: checkEKSSG},
			{TargetType: "vpc", DisplayName: "VPC", Checker: checkEKSVPC},
			{TargetType: "role", DisplayName: "IAM Role", Checker: checkEKSRole},
			{TargetType: "kms", DisplayName: "KMS Key", Checker: checkEKSKMS},
			{TargetType: "subnet", DisplayName: "Subnets", Checker: checkEKSSubnet},
			{TargetType: "ami", DisplayName: "AMI", Checker: checkEKSAMI},
			{TargetType: "asg", DisplayName: "Auto Scaling Groups", Checker: checkEKSASG, NeedsTargetCache: true, Truncated: true},
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
			{Code: CodeEKSStateDeleting, Phrase: "deleting", Severity: domain.SevWarn, Source: "wave1", Detail: "The cluster is being torn down; its workloads are going with it and nothing else about it is worth reporting until it is gone."},
			{Code: CodeEKSStatePending, Phrase: "pending", Severity: domain.SevWarn, Source: "wave1", Detail: "The cluster has been created but its control plane is not serving yet; nothing can be scheduled on it until it becomes active."},
			{Code: CodeEKSStateFailed, Phrase: "failed", Severity: domain.SevBroken, Source: "wave1", Detail: "The cluster is in a failed state and will not recover on its own; every health issue AWS reports is a row under this finding. Open a support case or recreate the cluster."},
			{Code: CodeEKSHealthIssue, Phrase: "health issue", Severity: domain.SevWarn, Source: "wave1", Detail: "The control plane reports at least one health issue; every reported code is a row under this finding, and the EKS console carries the message. Add-ons and nodes may misbehave until it clears."},
			{Code: CodeEKSPublicEndpoint, Phrase: "cluster endpoint reachable from the internet", Severity: domain.SevBroken, Source: "wave1", Detail: "The cluster's Kubernetes endpoint answers from the public internet, so its authentication is the only thing between the control plane and every scanner on the network. Turn off public endpoint access and reach the cluster over the VPC, or at minimum restrict public access to the office and build ranges."},
			{Code: CodeEKSPublicEndpointRestricted, Phrase: "cluster endpoint reachable from listed networks", Severity: domain.SevWarn, Source: "wave1", Detail: "The cluster's Kubernetes endpoint is public but only the listed address ranges may reach it, so the exposure is bounded by a list somebody has to keep correct. Check the ranges are still the ones you meant, and prefer reaching the cluster over the VPC."},
			{Code: CodeEKSControlPlaneLoggingOff, Phrase: "control plane logging incomplete", Severity: domain.SevWarn, Source: "wave1", Detail: "Some control-plane log types are not being sent to CloudWatch, so an authentication attempt or an admission decision made during an incident leaves no record to investigate. Enable all five control-plane log types on the cluster."},
			{Code: CodeEKSSecretsNotKMS, Phrase: "secrets not encrypted with KMS", Severity: domain.SevWarn, Source: "wave1", Detail: "Kubernetes secrets in this cluster are stored in etcd with only the AWS-managed default protection and no envelope encryption of their own. Attach a KMS key to the cluster's secrets encryption configuration so a copy of etcd is useless without that key."},
			{Code: CodeEKSVersionUnsupported, Phrase: "Kubernetes <version> is out of standard support", Severity: domain.SevBroken, Source: "wave1", Detail: "This Kubernetes minor is past standard support, so it no longer receives the full patch stream and AWS will upgrade it on its own schedule if you do not. Plan an upgrade to a version in standard support before the automatic one lands during business hours."},
			DetailsDeniedFindingDef("eks", ""),
			DetailsUnavailableFindingDef("eks"),
		},
	},
	{
		Name:           "EKS Node Groups",
		ShortName:      "ng",
		HumanizeFields: []string{"status"},
		Aliases:        []string{"ng", "nodegroups", "node-groups"},
		Category:       "CONTAINERS",
		CloudTrailKey:  "ResourceName:ID",
		LifecycleKey:   "status",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			cluster := r.Fields["cluster_name"]
			if cluster == "" {
				return ""
			}
			return consolelink.Regional(region, "eks/home?region="+region+"#/clusters/"+url.PathEscape(cluster)+"/nodegroups/"+url.PathEscape(r.ID))
		},
		Columns: []domain.Column{
			{Key: "nodegroup_name", Title: "Node Group", Path: "NodegroupName", Width: 28},
			{Key: "cluster_name", Title: "Cluster", Path: "ClusterName", Width: 24},
			{Key: "status", Title: "Status", Path: "Status", Width: 14},
			{Key: "instance_types", Title: "Instance Types", Path: "InstanceTypes", Width: 20},
			{Key: "desired_size", Title: "Desired", Path: "ScalingConfig.DesiredSize", Width: 9},
		},
		Color:   colorEKSNodeGroup,
		Fetcher: fetchNodeGroupsPage,
		// In-fetcher Wave 2: the ng fetcher already issues per-node-group
		// DescribeNodegroup calls and populates health_issues_count /
		// health_issues at fetch time. InFetcherWave2Sentinel records that
		// contract in the catalog; see its doc comment in issue_enrichment.go
		// for what does (and does not) guard this wiring.
		Wave2: IssueEnricher{Fn: InFetcherWave2Sentinel, Priority: 100},
		FieldKeys: []string{
			"nodegroup_name", "cluster_name", "status", "instance_types",
			"desired_size", "health_issues_count", "health_issues", "image_id",
		},
		Related: []domain.RelatedDef{
			{TargetType: "eks", DisplayName: "EKS Clusters", Checker: checkNGEKS, NeedsTargetCache: true, Truncated: true},
			{TargetType: "role", DisplayName: "IAM Roles", Checker: checkNGRole},
			{TargetType: "asg", DisplayName: "Auto Scaling Groups", Checker: checkNGASG, NeedsTargetCache: true, Truncated: true},
			{TargetType: "ec2", DisplayName: "EC2 Instances", Checker: checkNGEC2, Truncated: true},
			{TargetType: "sg", DisplayName: "Security Groups", Checker: checkNGSG},
			{TargetType: "ami", DisplayName: "AMI", Checker: checkNGAMI},
			{TargetType: "ebs", DisplayName: "EBS Volumes", Checker: checkNGEBS, Truncated: true},
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
			{Code: CodeNGStateDegraded, Phrase: "degraded", Severity: domain.SevBroken, Source: "wave1", Detail: "The node group is degraded, so some nodes are failing or not joining; every health issue AWS reports is a row under this finding. Fix the cause, usually IAM, subnet capacity or the launch template, and let the group reconcile."},
			{Code: CodeNGHealthIssue, Phrase: "health issue", Severity: domain.SevWarn, Source: "wave1", Detail: "The node group reports a health issue while its state says nothing is wrong; every reported code is a row under this finding. Nodes may be failing to join or to stay healthy until it clears."},
			DetailsDeniedFindingDef("ng", ""),
			DetailsUnavailableFindingDef("ng"),
		},
	},
}

// ngResumeTokenPrefix marks a fetchNodeGroupsPage continuation token as the
// versioned composite resume format below, as opposed to a plain EKS
// ListClusters NextToken — the only format this token held before the
// result-cap-loses-continuation fix, and still the format returned whenever
// the cap isn't hit. EKS's own NextToken opaque strings are not documented to
// ever take this shape, so any token lacking the prefix — old or freshly
// minted — falls through decodeNGResumeToken's ok=false path and is used
// exactly as before: a plain ListClusters continuation.
const ngResumeTokenPrefix = "ng-resume/v1?" //nolint:gosec // not a credential — a fixed marker prefix for the composite pagination resume token format

// ngResumeState is the parsed form of a composite fetchNodeGroupsPage resume
// token, built only when the DefaultPageSize result cap cuts a cluster's
// ListNodegroups drain short:
//   - resumeCluster: the cluster whose drain was interrupted.
//   - pendingNGNames: node group names already listed but not yet described —
//     the tail of the ListNodegroups page in flight when the cap hit.
//   - ngToken: that cluster's own ListNodegroups NextToken for any pages
//     after pendingNGNames; empty once the cluster has no more pages.
//   - pendingClusters: clusters from the same ListClusters page not yet
//     visited, in ListClusters' own order.
//   - outerToken: the outer ListClusters NextToken, for continuing past this
//     page once resumeCluster and every pendingClusters entry are drained.
type ngResumeState struct {
	resumeCluster   string
	ngToken         string
	pendingNGNames  []string
	pendingClusters []string
	outerToken      string
}

// encodeNGResumeToken serializes s as url.Values — a collision-safe,
// self-describing encoding, so caller-controlled cluster/node-group names
// (which could otherwise contain "=" or "&") are always percent-encoded
// rather than risk being misread as token structure.
func encodeNGResumeToken(s ngResumeState) string {
	v := url.Values{}
	v.Set("rc", s.resumeCluster)
	if s.ngToken != "" {
		v.Set("ngt", s.ngToken)
	}
	for _, name := range s.pendingNGNames {
		v.Add("pn", name)
	}
	for _, cluster := range s.pendingClusters {
		v.Add("pc", cluster)
	}
	if s.outerToken != "" {
		v.Set("ot", s.outerToken)
	}
	return ngResumeTokenPrefix + v.Encode()
}

// decodeNGResumeToken reports ok=false for any token not carrying
// ngResumeTokenPrefix (including a malformed query or one missing the
// required resume-cluster field), which is the compatibility path for a
// plain ListClusters token minted before this composite format existed.
func decodeNGResumeToken(token string) (ngResumeState, bool) {
	if !strings.HasPrefix(token, ngResumeTokenPrefix) {
		return ngResumeState{}, false
	}
	v, err := url.ParseQuery(strings.TrimPrefix(token, ngResumeTokenPrefix))
	if err != nil || v.Get("rc") == "" {
		return ngResumeState{}, false
	}
	return ngResumeState{
		resumeCluster:   v.Get("rc"),
		ngToken:         v.Get("ngt"),
		pendingNGNames:  v["pn"],
		pendingClusters: v["pc"],
		outerToken:      v.Get("ot"),
	}, true
}

// fetchNodeGroupsPage is the registered Wave 1 fetcher for the ng resource
// type. It walks ListClusters → ListNodegroups → DescribeNodegroup with
// per-call retry-on-throttle, capping the page at DefaultPageSize so the
// background fetcher pool keeps a bounded blast radius regardless of how many
// clusters/nodegroups exist in the account. When the cap cuts a cluster's
// ListNodegroups drain short, the returned continuation token is a composite
// ngResumeState (encodeNGResumeToken) that resumes exactly where the drain
// stopped, instead of the outer ListClusters token alone — which cannot
// represent a paused per-cluster drain and, on a single-page cluster list,
// would come back empty even though the result is truncated. If the resume
// cluster no longer exists by the time the token is redeemed, its
// ListNodegroups/DescribeNodegroup calls fail like any other AWS error:
// recorded via AggregateFailures, not a hard error, and every other cluster
// named in the token is still visited.
func fetchNodeGroupsPage(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
	c, err := svcClients(clients)
	if err != nil {
		return resource.FetchResult{}, err
	}

	var clusters []string
	var pendingNGNames []string
	var resumeNGToken string
	var resuming bool
	var outerToken string

	if state, ok := decodeNGResumeToken(continuationToken); ok {
		resuming = true
		clusters = append([]string{state.resumeCluster}, state.pendingClusters...)
		pendingNGNames = state.pendingNGNames
		resumeNGToken = state.ngToken
		outerToken = state.outerToken
	} else {
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
		clusters = clusterOutput.Clusters
		if clusterOutput.NextToken != nil {
			outerToken = *clusterOutput.NextToken
		}
	}

	var resources []resource.Resource
	var failures []Failure
	totalAttempted := 0

	// processNGNames describes and appends resources for names, stopping the
	// instant the DefaultPageSize result cap is reached and returning the
	// slice tail that never got a DescribeNodegroup call — the exact set a
	// resume token must carry forward so no name is skipped or repeated.
	processNGNames := func(cluster string, names []string) (leftover []string, hitCap bool) {
		for i, ngName := range names {
			if len(resources) >= DefaultPageSize {
				return names[i:], true
			}
			totalAttempted++
			descOutput, descErr := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*eks.DescribeNodegroupOutput, error) {
				return c.EKS.DescribeNodegroup(ctx, &eks.DescribeNodegroupInput{
					ClusterName:   aws.String(cluster),
					NodegroupName: aws.String(ngName),
				})
			})
			if descErr != nil {
				failures = append(failures, FailedCall(cluster+"/"+ngName, descErr))
				resources = append(resources, degradedNodeGroup(cluster, ngName, descErr))
				continue
			}
			if descOutput.Nodegroup == nil {
				failures = append(failures, UnusableAnswer(cluster+"/"+ngName, "nil nodegroup in response"))
				resources = append(resources, degradedNodeGroup(cluster, ngName, nil))
				continue
			}
			res := buildNodeGroupResource(cluster, ngName, descOutput.Nodegroup)
			if lt := descOutput.Nodegroup.LaunchTemplate; lt != nil && lt.Id != nil {
				res.Fields["image_id"] = resolveNGImageID(ctx, c.EC2, lt)
			}
			resources = append(resources, res)
		}
		return nil, false
	}

	capResult := func(cluster string, leftoverNames []string, ngtok string, remainingClusters []string) resource.FetchResult {
		var pending []string
		if len(remainingClusters) > 0 {
			pending = append([]string{}, remainingClusters...)
		}
		nextToken := encodeNGResumeToken(ngResumeState{
			resumeCluster:   cluster,
			ngToken:         ngtok,
			pendingNGNames:  leftoverNames,
			pendingClusters: pending,
			outerToken:      outerToken,
		})
		return resource.FetchResult{
			Resources: resources,
			Pagination: &resource.PaginationMeta{
				IsTruncated: true,
				NextToken:   nextToken,
				PageSize:    len(resources),
				TotalHint:   -1,
			},
		}
	}

	for i, cluster := range clusters {
		var ngToken *string
		if i == 0 && resuming {
			if len(pendingNGNames) > 0 {
				leftover, hitCap := processNGNames(cluster, pendingNGNames)
				if hitCap {
					return capResult(cluster, leftover, resumeNGToken, clusters[i+1:]), AggregateFailures("ng: DescribeNodegroup", failures, totalAttempted)
				}
			}
			if resumeNGToken == "" {
				continue
			}
			ngToken = aws.String(resumeNGToken)
		}

		for {
			ngOutput, ngErr := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*eks.ListNodegroupsOutput, error) {
				return c.EKS.ListNodegroups(ctx, &eks.ListNodegroupsInput{
					ClusterName: aws.String(cluster),
					MaxResults:  aws.Int32(DefaultPageSize),
					NextToken:   ngToken,
				})
			})
			if ngErr != nil {
				failures = append(failures, FailedCall(cluster, ngErr))
				break
			}
			leftover, hitCap := processNGNames(cluster, ngOutput.Nodegroups)
			if hitCap {
				var nextNGToken string
				if ngOutput.NextToken != nil {
					nextNGToken = *ngOutput.NextToken
				}
				return capResult(cluster, leftover, nextNGToken, clusters[i+1:]), AggregateFailures("ng: DescribeNodegroup", failures, totalAttempted)
			}
			if ngOutput.NextToken == nil {
				break
			}
			ngToken = ngOutput.NextToken
		}
	}

	return resource.FetchResult{
		Resources: resources,
		Pagination: &resource.PaginationMeta{
			IsTruncated: outerToken != "",
			NextToken:   outerToken,
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
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			repo := r.Fields["repository_name"]
			if repo == "" {
				return ""
			}
			return consolelink.Regional(region, "ecr/repositories/"+url.PathEscape(repo)+"/?region="+region)
		},
		Columns:   resource.ECRImageColumns(),
		CopyField: "image_uri",
		Color:     colorAnyFindingOrHealthy,
		FieldKeys: []string{
			"image_tags", "digest_short", "pushed_at", "image_size", "image_size_raw",
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
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			arn := r.Fields["task_arn"]
			if arn == "" {
				return ""
			}
			return consolelink.Regional(region, "ecs/v2/redirect?arn="+url.QueryEscape(arn)+"&region="+region)
		},
		Columns: resource.EcsSvcTaskColumns(),
		FieldKeys: []string{
			"task_id", "status", "health", "task_def_short",
			"started_at", "stopped_reason", "stop_code", "task_arn",
		},
		Color: func(r domain.Resource) domain.Color {
			if c, ok := colorFromAnyFinding(r); ok {
				return c
			}
			return colorFromFindings(ecsTaskStructuralFindings(
				r.Fields["status"], r.Fields["stop_code"], r.Fields["health"]))
		},
		ChildFetcher: childFetcherWithClients(func(ctx context.Context, c *ServiceClients, parentCtx resource.ParentContext, continuationToken string) (resource.FetchResult, error) {
			return FetchEcsSvcTasks(ctx, c.ECS, c.ECS, parentCtx["cluster"], parentCtx["service_name"], continuationToken)
		}),
		Findings: []catalog.FindingDef{
			{Code: CodeECSTaskHealthUnhealthy, Phrase: "unhealthy", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeECSTaskStopCodeFailed, Phrase: "stopped: <reason>", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeECSTaskStateStopped, Phrase: "stopped", Severity: domain.SevDim, Source: "wave1"},
			{Code: CodeECSTaskStateProvisioning, Phrase: "provisioning", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeECSTaskStatePending, Phrase: "pending", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeECSTaskStateActivating, Phrase: "activating", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeECSTaskStateDeactivating, Phrase: "deactivating", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeECSTaskStateStopping, Phrase: "stopping", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeECSTaskStateDeprovisioning, Phrase: "deprovisioning", Severity: domain.SevWarn, Source: "wave1"},
		},
	},
	{
		Name:      "Service Events",
		ShortName: "ecs_svc_events",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			arn := r.Fields["service_arn"]
			if arn == "" {
				return ""
			}
			return consolelink.Regional(region, "ecs/v2/redirect?arn="+url.QueryEscape(arn)+"&region="+region)
		},
		Columns:   resource.EcsSvcEventColumns(),
		FieldKeys: []string{"timestamp", "message", "service_arn"},
		ChildFetcher: childFetcherWithClients(func(ctx context.Context, c *ServiceClients, parentCtx resource.ParentContext, continuationToken string) (resource.FetchResult, error) {
			return FetchEcsSvcEvents(ctx, c.ECS, parentCtx["cluster"], parentCtx["service_name"], continuationToken)
		}),
	},
	{
		Name:      "Service Logs",
		ShortName: "ecs_svc_logs",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			return cloudWatchLogStreamConsoleURL(region, r.Fields["log_group"], r.Fields["log_stream"])
		},
		Columns:   resource.EcsSvcLogColumns(),
		Color:     colorAnyFindingOrHealthy,
		FieldKeys: []string{"timestamp", "stream_short", "message", "log_group", "log_stream"},
		ChildFetcher: childFetcherWithClients(func(ctx context.Context, c *ServiceClients, parentCtx resource.ParentContext, continuationToken string) (resource.FetchResult, error) {
			return FetchEcsSvcLogs(ctx, c.ECS, c.CloudWatchLogs, parentCtx["cluster"], parentCtx["service_name"], parentCtx["task_definition"], continuationToken)
		}),
		Findings: []catalog.FindingDef{
			{Code: CodeCWLogError, Phrase: "error", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeCWLogWarn, Phrase: "warning", Severity: domain.SevWarn, Source: "wave1"},
		},
	},
}
