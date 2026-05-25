// eks_related_extra.go — additional EKS related-resource checkers.
package aws

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	autoscalingPkg "github.com/aws/aws-sdk-go-v2/service/autoscaling"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	smithy "github.com/aws/smithy-go"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkEKSSubnet extracts subnet IDs from ResourcesVpcConfig.SubnetIds.
func checkEKSSubnet(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	cluster, ok := assertStruct[ekstypes.Cluster](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "subnet", Count: -1}
	}
	if cluster.ResourcesVpcConfig == nil {
		return resource.RelatedCheckResult{TargetType: "subnet", Count: 0}
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
		return resource.RelatedCheckResult{TargetType: "asg", Count: 0}
	}
	ngList, truncated, err := eksRelatedResourcesExtra(ctx, clients, cache, "ng")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "asg", Count: -1, Err: err}
	}
	if ngList == nil {
		return resource.RelatedCheckResult{TargetType: "asg", Count: 0}
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
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("asg")
	}
	return relatedResult("asg", ids)
}

// checkEKSCTEvents scans ct-events for events involving this cluster.
func checkEKSCTEvents(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	clusterName := res.ID
	if clusterName == "" {
		return resource.RelatedCheckResult{TargetType: "ct-events", Count: 0}
	}
	evList, truncated, err := eksRelatedResourcesExtra(ctx, clients, cache, "ct-events")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "ct-events", Count: -1, Err: err}
	}
	if evList == nil {
		return resource.RelatedCheckResult{TargetType: "ct-events", Count: -1}
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
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("ct-events")
	}
	return relatedResult("ct-events", ids)
}

// checkEKSAMI resolves the AMI(s) used by all node groups in this EKS cluster.
// For each node group: if LaunchTemplate is set, call ec2:DescribeLaunchTemplateVersions
// to get LaunchTemplateData.ImageId. Managed NGs without a LT use AmiType+ReleaseVersion
// — SSM resolution is deferred; those return no AMI. Returns distinct AMI IDs.
func checkEKSAMI(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	cluster, ok := assertStruct[ekstypes.Cluster](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "ami", Count: -1}
	}
	clusterName := res.ID
	if clusterName == "" && cluster.Name != nil {
		clusterName = *cluster.Name
	}
	if clusterName == "" {
		return resource.RelatedCheckResult{TargetType: "ami", Count: 0}
	}

	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.EKS == nil {
		return resource.RelatedCheckResult{TargetType: "ami", Count: -1}
	}

	ngOut, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*eks.ListNodegroupsOutput, error) {
		return c.EKS.ListNodegroups(ctx, &eks.ListNodegroupsInput{
			ClusterName: aws.String(clusterName),
		})
	})
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "ami", Count: -1, Err: err}
	}

	amiSet := make(map[string]struct{})
	var failures []string
	total := len(ngOut.Nodegroups)
	for _, ngName := range ngOut.Nodegroups {
		descOut, descErr := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*eks.DescribeNodegroupOutput, error) {
			return c.EKS.DescribeNodegroup(ctx, &eks.DescribeNodegroupInput{
				ClusterName:   aws.String(clusterName),
				NodegroupName: aws.String(ngName),
			})
		})
		if descErr != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", ngName, descErr))
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
			var apiErr smithy.APIError
			if errors.As(ltErr, &apiErr) && apiErr.ErrorCode() == "InvalidLaunchTemplateId.NotFound" {
				failures = append(failures, fmt.Sprintf("%s: launch template deleted", ngName))
				continue
			}
			failures = append(failures, fmt.Sprintf("%s/lt: %v", ngName, ltErr))
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
	result := relatedResult("ami", ids)
	result.Err = AggregateFailures("eks-related: DescribeNodegroup", failures, total)
	return result
}

// checkEKSEC2 resolves EC2 instances running in this EKS cluster via node group ASGs.
// For each node group: get Resources.AutoScalingGroups, then call
// autoscaling:DescribeAutoScalingGroups to read Instances[].InstanceId.
func checkEKSEC2(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	cluster, ok := assertStruct[ekstypes.Cluster](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "ec2", Count: -1}
	}
	clusterName := res.ID
	if clusterName == "" && cluster.Name != nil {
		clusterName = *cluster.Name
	}
	if clusterName == "" {
		return resource.RelatedCheckResult{TargetType: "ec2", Count: 0}
	}

	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.EKS == nil {
		return resource.RelatedCheckResult{TargetType: "ec2", Count: -1}
	}

	ngOut, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*eks.ListNodegroupsOutput, error) {
		return c.EKS.ListNodegroups(ctx, &eks.ListNodegroupsInput{
			ClusterName: aws.String(clusterName),
		})
	})
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "ec2", Count: -1, Err: err}
	}

	var asgNames []string
	var ngFailures []string
	ngTotal := len(ngOut.Nodegroups)
	for _, ngName := range ngOut.Nodegroups {
		descOut, descErr := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*eks.DescribeNodegroupOutput, error) {
			return c.EKS.DescribeNodegroup(ctx, &eks.DescribeNodegroupInput{
				ClusterName:   aws.String(clusterName),
				NodegroupName: aws.String(ngName),
			})
		})
		if descErr != nil {
			ngFailures = append(ngFailures, fmt.Sprintf("%s: %v", ngName, descErr))
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
			return resource.RelatedCheckResult{TargetType: "ec2", Count: -1, Err: ngAggErr}
		}
		return resource.RelatedCheckResult{TargetType: "ec2", Count: 0}
	}
	if c.AutoScaling == nil {
		return resource.RelatedCheckResult{TargetType: "ec2", Count: -1, Err: ngAggErr}
	}

	asgOut, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*autoscalingPkg.DescribeAutoScalingGroupsOutput, error) {
		return c.AutoScaling.DescribeAutoScalingGroups(ctx, &autoscalingPkg.DescribeAutoScalingGroupsInput{
			AutoScalingGroupNames: asgNames,
		})
	})
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "ec2", Count: -1, Err: err}
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
	result := relatedResult("ec2", ids)
	result.Err = ngAggErr
	return result
}

// eksRelatedResourcesExtra — companion helper so we don't duplicate the
// pattern from eks_related.go.
func eksRelatedResourcesExtra(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}

// autoscalingEC2InstanceID is a local type helper to keep ec2types in scope.
var _ ec2types.Instance
