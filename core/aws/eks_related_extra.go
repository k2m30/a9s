// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// eks_related_extra.go — additional EKS related-resource checkers.
package aws

import (
	"context"
	"slices"

	"github.com/aws/aws-sdk-go-v2/aws"
	autoscalingPkg "github.com/aws/aws-sdk-go-v2/service/autoscaling"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
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
		return resource.ProvenZero("subnet", "cluster.ResourcesVpcConfig")
	}
	var ids []string
	for _, s := range cluster.ResourcesVpcConfig.SubnetIds {
		if s != "" {
			ids = append(ids, s)
		}
	}
	return relatedResultTrunc("subnet", ids, false)
}

// checkEKSASG — ASGs are owned by NodeGroups; derive by scanning ng cache.
func checkEKSASG(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	clusterName := res.ID
	if clusterName == "" {
		return resource.ProvenZero("asg", "clusterName")
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

// listClusterNodegroups walks the ListNodegroups pages of one cluster.
// complete is false when the walk stopped at the page cap.
func listClusterNodegroups(ctx context.Context, api EKSAPI, clusterName string) (names []string, complete bool, err error) {
	return PageAll(ctx, PerParentPageCap, func(ctx context.Context, token *string) ([]string, *string, error) {
		out, err := api.ListNodegroups(ctx, &eks.ListNodegroupsInput{
			ClusterName: aws.String(clusterName),
			NextToken:   token,
		})
		if err != nil {
			return nil, nil, err
		}
		return out.Nodegroups, out.NextToken, nil
	})
}

// checkEKSAMI resolves the AMI(s) used by all node groups in this EKS cluster.
// For each node group: if LaunchTemplate is set, call ec2:DescribeLaunchTemplateVersions
// to get LaunchTemplateData.ImageId. Managed NGs without a LT use AmiType+ReleaseVersion
// — those return no AMI, since resolving them needs SSM. Returns distinct AMI IDs.
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
		return resource.ProvenZero("ami", "clusterName")
	}

	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.EKS == nil {
		return resource.UnknownRelated("ami")
	}

	ngNames, ngComplete, err := listClusterNodegroups(ctx, c.EKS, clusterName)
	if err != nil {
		return resource.ErrorRelated("ami", err)
	}

	amiSet := make(map[string]struct{})
	var failures []Failure
	for _, ngName := range ngNames {
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
			// Managed NG without custom LT: its AMI is only resolvable via SSM.
			continue
		}

		versions, ltErr := launchTemplateVersions(ctx, c.EC2, ng.LaunchTemplate.Id, ng.LaunchTemplate.Version)
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
		for _, v := range versions {
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
		if aggErr := AggregateFailures("eks-related: DescribeNodegroup/DescribeLaunchTemplateVersions", failures, len(ngNames)); aggErr != nil {
			return resource.ErrorRelated("ami", aggErr)
		}
		return relatedResultTrunc("ami", nil, !ngComplete)
	}
	// Some DescribeNodegroup/DescribeLaunchTemplateVersions calls may have
	// failed: ids is a proven subset, not necessarily exhaustive. Truncated
	// (not Errored) keeps the row actionable rather than discarding confirmed
	// matches as a dead end.
	return relatedResultTrunc("ami", ids, len(failures) > 0 || !ngComplete)
}

// asgNamesPerDescribe is the maximum number of names DescribeAutoScalingGroups
// accepts in one request; a longer list is refused outright, which would turn a
// large cluster's whole EC2 row into an error.
const asgNamesPerDescribe = 50

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
		return resource.ProvenZero("ec2", "clusterName")
	}

	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.EKS == nil {
		return resource.UnknownRelated("ec2")
	}

	ngNames, ngComplete, err := listClusterNodegroups(ctx, c.EKS, clusterName)
	if err != nil {
		return resource.ErrorRelated("ec2", err)
	}

	var asgNames []string
	var ngFailures []Failure
	ngTotal := len(ngNames)
	for _, ngName := range ngNames {
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
		return relatedResultTrunc("ec2", nil, !ngComplete)
	}
	if c.AutoScaling == nil {
		// ASG names are known but cannot be resolved to instances without an
		// AutoScaling client — Unknown, not an error (any DescribeNodegroup
		// failures are subsumed: we could not have proceeded past this point
		// regardless of ngAggErr).
		return resource.UnknownRelated("ec2")
	}

	seen := make(map[string]struct{})
	complete := ngComplete
	for batch := range slices.Chunk(asgNames, asgNamesPerDescribe) {
		asgs, batchComplete, err := PageAll(ctx, PerParentPageCap, func(ctx context.Context, token *string) ([]asgtypes.AutoScalingGroup, *string, error) {
			out, err := c.AutoScaling.DescribeAutoScalingGroups(ctx, &autoscalingPkg.DescribeAutoScalingGroupsInput{
				AutoScalingGroupNames: batch,
				NextToken:             token,
			})
			if err != nil {
				return nil, nil, err
			}
			return out.AutoScalingGroups, out.NextToken, nil
		})
		if err != nil {
			return resource.ErrorRelated("ec2", err)
		}
		complete = complete && batchComplete
		for _, asg := range asgs {
			for _, inst := range asg.Instances {
				if inst.InstanceId != nil && *inst.InstanceId != "" {
					seen[*inst.InstanceId] = struct{}{}
				}
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
	return relatedResultTrunc("ec2", ids, ngAggErr != nil || !complete)
}

// Keeps the ec2types import in use.
var _ ec2types.Instance
