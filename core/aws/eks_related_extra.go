// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// eks_related_extra.go — additional EKS related-resource checkers.
package aws

import (
	"context"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	autoscalingPkg "github.com/aws/aws-sdk-go-v2/service/autoscaling"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkEKSSubnet extracts subnet IDs from ResourcesVpcConfig.SubnetIds.
func checkEKSSubnet(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	cluster, ok := assertStruct[ekstypes.Cluster](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("subnet")
	}
	if cluster.ResourcesVpcConfig == nil {
		return resource.KnownRelated("subnet", nil, false)
	}
	var ids []string
	for _, s := range cluster.ResourcesVpcConfig.SubnetIds {
		if s != "" {
			ids = append(ids, s)
		}
	}
	return relatedResult("subnet", ids)
}

// checkEKSASG — ASGs are owned by NodeGroups; derive by scanning ng cache.
func checkEKSASG(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	clusterName := res.ID
	if clusterName == "" {
		return resource.KnownRelated("asg", nil, false)
	}
	ngList, truncated, err := relatedResourcesFor(ctx, clients, cache, "ng")
	if err != nil {
		return resource.ErrorRelated("asg", err)
	}
	if ngList == nil {
		return resource.UnknownRelated("asg")
	}
	seen := make(map[string]struct{})
	for _, ngRes := range ngList {
		ng, ok := assertStruct[ekstypes.Nodegroup](ngRes.RawStruct)
		if !ok {
			continue
		}
		if ng.ClusterName == nil || *ng.ClusterName != clusterName {
			continue
		}
		if ng.Resources != nil {
			for _, a := range ng.Resources.AutoScalingGroups {
				if a.Name != nil && *a.Name != "" {
					seen[*a.Name] = struct{}{}
				}
			}
		}
	}
	var ids []string
	for id := range seen {
		ids = append(ids, id)
	}
	return relatedResultTrunc("asg", ids, truncated)
}

// checkEKSCTEvents scans ct-events for events involving this cluster.
func checkEKSCTEvents(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	clusterName := res.ID
	if clusterName == "" {
		return resource.KnownRelated("ct-events", nil, false)
	}
	evList, truncated, err := relatedResourcesFor(ctx, clients, cache, "ct-events")
	if err != nil {
		return resource.ErrorRelated("ct-events", err)
	}
	if evList == nil {
		return resource.UnknownRelated("ct-events")
	}
	var ids []string
	for _, evRes := range evList {
		ev, ok := assertStruct[cloudtrailtypes.Event](evRes.RawStruct)
		if !ok {
			continue
		}
		for _, r := range ev.Resources {
			if r.ResourceName != nil && strings.Contains(*r.ResourceName, clusterName) {
				ids = append(ids, evRes.ID)
				break
			}
		}
	}
	return relatedResultTrunc("ct-events", ids, truncated)
}

// checkEKSAMI resolves the AMI(s) used by all node groups in this EKS cluster.
// For each node group: if LaunchTemplate is set, call ec2:DescribeLaunchTemplateVersions
// to get LaunchTemplateData.ImageId. Managed NGs without a LT use AmiType+ReleaseVersion
// — SSM resolution is deferred; those return no AMI. Returns distinct AMI IDs.
func checkEKSAMI(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	cluster, ok := assertStruct[ekstypes.Cluster](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("ami")
	}
	clusterName := res.ID
	if clusterName == "" && cluster.Name != nil {
		clusterName = *cluster.Name
	}
	if clusterName == "" {
		return resource.KnownRelated("ami", nil, false)
	}

	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.EKS == nil {
		return resource.UnknownRelated("ami")
	}

	ngOut, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*eks.ListNodegroupsOutput, error) {
		return c.EKS.ListNodegroups(ctx, &eks.ListNodegroupsInput{
			ClusterName: aws.String(clusterName),
		})
	})
	if err != nil {
		return resource.ErrorRelated("ami", err)
	}

	amiSet := make(map[string]struct{})
	var failures []Failure
	for _, ngName := range ngOut.Nodegroups {
		descOut, descErr := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*eks.DescribeNodegroupOutput, error) {
			return c.EKS.DescribeNodegroup(ctx, &eks.DescribeNodegroupInput{
				ClusterName:   aws.String(clusterName),
				NodegroupName: aws.String(ngName),
			})
		})
		if descErr != nil {
			failures = append(failures, FailedCall(ngName, descErr))
			continue
		}
		if descOut.Nodegroup == nil {
			continue
		}
		ng := descOut.Nodegroup
		if ng.LaunchTemplate == nil || ng.LaunchTemplate.Id == nil || *ng.LaunchTemplate.Id == "" {
			// Managed NG without custom LT — AMI resolution via SSM deferred.
			continue
		}

		version := aws.String("$Latest")
		if ng.LaunchTemplate.Version != nil && *ng.LaunchTemplate.Version != "" {
			version = ng.LaunchTemplate.Version
		}
		ltOut, ltErr := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*ec2.DescribeLaunchTemplateVersionsOutput, error) {
			return c.EC2.DescribeLaunchTemplateVersions(ctx, &ec2.DescribeLaunchTemplateVersionsInput{
				LaunchTemplateId: ng.LaunchTemplate.Id,
				Versions:         []string{*version},
			})
		})
		if ltErr != nil {
			// Soft-skip when the launch template has been deleted upstream:
			// AWS returns InvalidLaunchTemplateId.NotFound, which is a true
			// "no AMI to relate to" rather than a fetch failure.
			if ErrCodeIs(ltErr, "InvalidLaunchTemplateId.NotFound") {
				continue
			}
			failures = append(failures, FailedCall(ngName+"/lt", ltErr))
			continue
		}
		for _, v := range ltOut.LaunchTemplateVersions {
			if v.LaunchTemplateData != nil && v.LaunchTemplateData.ImageId != nil && *v.LaunchTemplateData.ImageId != "" {
				amiSet[*v.LaunchTemplateData.ImageId] = struct{}{}
			}
		}
	}

	var ids []string
	for id := range amiSet {
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		// Nothing was confirmed. A failure here means the resolution attempt
		// itself failed — that does not establish "the population is larger
		// than what we saw" (Truncated's contract); it establishes nothing.
		// Surface it as an error, not a lower bound of zero.
		if aggErr := AggregateFailures("eks-related: DescribeNodegroup/DescribeLaunchTemplateVersions", failures, len(ngOut.Nodegroups)); aggErr != nil {
			return resource.ErrorRelated("ami", aggErr)
		}
		return relatedResultTrunc("ami", nil, false)
	}
	// Some DescribeNodegroup/DescribeLaunchTemplateVersions calls may have
	// failed: ids is a proven subset, not necessarily exhaustive. Truncated
	// (not Errored) keeps the row actionable rather than discarding confirmed
	// matches as a dead end.
	return relatedResultTrunc("ami", ids, len(failures) > 0)
}

// checkEKSEC2 resolves EC2 instances running in this EKS cluster via node group ASGs.
// For each node group: get Resources.AutoScalingGroups, then call
// autoscaling:DescribeAutoScalingGroups to read Instances[].InstanceId.
func checkEKSEC2(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	cluster, ok := assertStruct[ekstypes.Cluster](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("ec2")
	}
	clusterName := res.ID
	if clusterName == "" && cluster.Name != nil {
		clusterName = *cluster.Name
	}
	if clusterName == "" {
		return resource.KnownRelated("ec2", nil, false)
	}

	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.EKS == nil {
		return resource.UnknownRelated("ec2")
	}

	ngOut, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*eks.ListNodegroupsOutput, error) {
		return c.EKS.ListNodegroups(ctx, &eks.ListNodegroupsInput{
			ClusterName: aws.String(clusterName),
		})
	})
	if err != nil {
		return resource.ErrorRelated("ec2", err)
	}

	var asgNames []string
	var ngFailures []Failure
	ngTotal := len(ngOut.Nodegroups)
	for _, ngName := range ngOut.Nodegroups {
		descOut, descErr := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*eks.DescribeNodegroupOutput, error) {
			return c.EKS.DescribeNodegroup(ctx, &eks.DescribeNodegroupInput{
				ClusterName:   aws.String(clusterName),
				NodegroupName: aws.String(ngName),
			})
		})
		if descErr != nil {
			ngFailures = append(ngFailures, FailedCall(ngName, descErr))
			continue
		}
		if descOut.Nodegroup == nil || descOut.Nodegroup.Resources == nil {
			continue
		}
		for _, asg := range descOut.Nodegroup.Resources.AutoScalingGroups {
			if asg.Name != nil && *asg.Name != "" {
				asgNames = append(asgNames, *asg.Name)
			}
		}
	}

	ngAggErr := AggregateFailures("eks-related: DescribeNodegroup", ngFailures, ngTotal)
	if len(asgNames) == 0 {
		if ngAggErr != nil {
			return resource.ErrorRelated("ec2", ngAggErr)
		}
		return resource.KnownRelated("ec2", nil, false)
	}
	if c.AutoScaling == nil {
		// ASG names are known but cannot be resolved to instances without an
		// AutoScaling client — Unknown, not an error (any DescribeNodegroup
		// failures are subsumed: we could not have proceeded past this point
		// regardless of ngAggErr).
		return resource.UnknownRelated("ec2")
	}

	asgOut, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*autoscalingPkg.DescribeAutoScalingGroupsOutput, error) {
		return c.AutoScaling.DescribeAutoScalingGroups(ctx, &autoscalingPkg.DescribeAutoScalingGroupsInput{
			AutoScalingGroupNames: asgNames,
		})
	})
	if err != nil {
		return resource.ErrorRelated("ec2", err)
	}

	seen := make(map[string]struct{})
	for _, asg := range asgOut.AutoScalingGroups {
		for _, inst := range asg.Instances {
			if inst.InstanceId != nil && *inst.InstanceId != "" {
				seen[*inst.InstanceId] = struct{}{}
			}
		}
	}
	var ids []string
	for id := range seen {
		ids = append(ids, id)
	}
	// Some DescribeNodegroup calls may have failed: ids is a proven subset of
	// the cluster's EC2 instances, not necessarily exhaustive. Truncated (not
	// Errored) keeps the row actionable rather than discarding confirmed
	// matches as a dead end.
	return relatedResultTrunc("ec2", ids, ngAggErr != nil)
}

// autoscalingEC2InstanceID is a local type helper to keep ec2types in scope.
var _ ec2types.Instance
