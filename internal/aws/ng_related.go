// ng_related.go contains EKS Node Group related-resource checker functions.
package aws

import (
	"context"
	"errors"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	autoscalingPkg "github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	smithy "github.com/aws/smithy-go"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkNGEKS extracts ClusterName from the Node Group RawStruct and searches
// the eks cache for a matching cluster by name.
func checkNGEKS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	clusterName := res.Fields["cluster_name"]
	if ng, ok := assertStruct[ekstypes.Nodegroup](res.RawStruct); ok {
		if ng.ClusterName != nil && *ng.ClusterName != "" {
			clusterName = *ng.ClusterName
		}
	}
	if clusterName == "" {
		return resource.RelatedCheckResult{TargetType: "eks", Count: 0}
	}

	eksList, _, err := ngRelatedResources(ctx, clients, cache, "eks")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "eks", Count: -1, Err: err}
	}
	if eksList == nil {
		return resource.RelatedCheckResult{TargetType: "eks", Count: -1}
	}

	var ids []string
	for _, eksRes := range eksList {
		if eksRes.Name == clusterName || eksRes.Fields["cluster_name"] == clusterName {
			ids = append(ids, eksRes.ID)
		}
	}
	return relatedResult("eks", ids)
}

// checkNGRole extracts the NodeRole ARN from the Node Group RawStruct, derives
// the role name from the last "/" segment, and searches the role cache by name.
func checkNGRole(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	ng, ok := assertStruct[ekstypes.Nodegroup](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "role", Count: 0}
	}
	if ng.NodeRole == nil || *ng.NodeRole == "" {
		return resource.RelatedCheckResult{TargetType: "role", Count: 0}
	}
	roleVal := *ng.NodeRole
	roleName := roleVal
	if idx := strings.LastIndex(roleVal, "/"); idx >= 0 && idx < len(roleVal)-1 {
		roleName = roleVal[idx+1:]
	}

	roleList, _, err := ngRelatedResources(ctx, clients, cache, "role")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "role", Count: -1, Err: err}
	}
	if roleList == nil {
		return resource.RelatedCheckResult{TargetType: "role", Count: -1}
	}

	var ids []string
	for _, roleRes := range roleList {
		if roleRes.Name == roleName || roleRes.Fields["role_name"] == roleName {
			ids = append(ids, roleRes.ID)
		}
	}
	return relatedResult("role", ids)
}

// checkNGASG extracts Resources.AutoScalingGroups from the Node Group RawStruct
// and searches the asg cache for matching ASGs by name.
func checkNGASG(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	ng, ok := assertStruct[ekstypes.Nodegroup](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "asg", Count: 0}
	}
	if ng.Resources == nil || len(ng.Resources.AutoScalingGroups) == 0 {
		return resource.RelatedCheckResult{TargetType: "asg", Count: 0}
	}

	asgNames := make(map[string]struct{}, len(ng.Resources.AutoScalingGroups))
	for _, asg := range ng.Resources.AutoScalingGroups {
		if asg.Name != nil && *asg.Name != "" {
			asgNames[*asg.Name] = struct{}{}
		}
	}
	if len(asgNames) == 0 {
		return resource.RelatedCheckResult{TargetType: "asg", Count: 0}
	}

	asgList, truncated, err := ngRelatedResources(ctx, clients, cache, "asg")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "asg", Count: -1, Err: err}
	}
	if asgList == nil {
		return resource.RelatedCheckResult{TargetType: "asg", Count: -1}
	}

	var ids []string
	for _, asgRes := range asgList {
		if _, found := asgNames[asgRes.ID]; found {
			ids = append(ids, asgRes.ID)
			continue
		}
		if _, found := asgNames[asgRes.Name]; found {
			ids = append(ids, asgRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("asg")
	}
	return relatedResult("asg", ids)
}

// checkNGEC2 scans the EC2 instance cache for instances tagged with this node
// group's name via "eks:nodegroup-name" and optionally "eks:cluster-name".
// Pattern C: tag-based cache scan.
func checkNGEC2(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	nodegroupName := res.Fields["nodegroup_name"]
	clusterName := res.Fields["cluster_name"]
	if ng, ok := assertStruct[ekstypes.Nodegroup](res.RawStruct); ok {
		if ng.NodegroupName != nil && *ng.NodegroupName != "" {
			nodegroupName = *ng.NodegroupName
		}
		if ng.ClusterName != nil && *ng.ClusterName != "" {
			clusterName = *ng.ClusterName
		}
	}
	if nodegroupName == "" {
		return resource.RelatedCheckResult{TargetType: "ec2", Count: 0}
	}

	ec2List, truncated, err := ngRelatedResources(ctx, clients, cache, "ec2")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "ec2", Count: -1, Err: err}
	}
	if ec2List == nil {
		return resource.RelatedCheckResult{TargetType: "ec2", Count: -1}
	}

	var ids []string
	for _, ec2Res := range ec2List {
		inst, ok := assertStruct[ec2types.Instance](ec2Res.RawStruct)
		if !ok {
			continue
		}
		if tagValue(inst.Tags, "eks:nodegroup-name") != nodegroupName {
			continue
		}
		if clusterName != "" {
			instCluster := tagValue(inst.Tags, "eks:cluster-name")
			if instCluster != "" && instCluster != clusterName {
				continue
			}
		}
		ids = append(ids, ec2Res.ID)
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("ec2")
	}
	return relatedResult("ec2", ids)
}

// checkNGSG extracts the remote access security group from the EKS Node Group's
// Resources.RemoteAccessSecurityGroup field (present when the node group is not
// using a launch template and SSH access is configured).
// Pattern F — no cache needed.
func checkNGSG(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	ng, ok := assertStruct[ekstypes.Nodegroup](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "sg", Count: -1}
	}
	if ng.Resources == nil || ng.Resources.RemoteAccessSecurityGroup == nil ||
		*ng.Resources.RemoteAccessSecurityGroup == "" {
		return resource.RelatedCheckResult{TargetType: "sg", Count: 0}
	}
	return relatedResult("sg", []string{*ng.Resources.RemoteAccessSecurityGroup})
}

// checkNGAMI resolves the AMI used by this node group's launch template.
// Pattern A — ec2:DescribeLaunchTemplateVersions if LaunchTemplate is set.
// Managed NGs without a custom launch template: AMI resolution via SSM is deferred; returns Count:0.
func checkNGAMI(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	ng, ok := assertStruct[ekstypes.Nodegroup](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "ami", Count: -1}
	}
	if ng.LaunchTemplate == nil || ng.LaunchTemplate.Id == nil || *ng.LaunchTemplate.Id == "" {
		// Managed NG without custom LT — AMI resolution via SSM deferred.
		return resource.RelatedCheckResult{TargetType: "ami", Count: 0}
	}

	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.EC2 == nil {
		return resource.RelatedCheckResult{TargetType: "ami", Count: -1}
	}

	version := aws.String("$Latest")
	if ng.LaunchTemplate.Version != nil && *ng.LaunchTemplate.Version != "" {
		version = ng.LaunchTemplate.Version
	}

	ltOut, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*ec2.DescribeLaunchTemplateVersionsOutput, error) {
		return c.EC2.DescribeLaunchTemplateVersions(ctx, &ec2.DescribeLaunchTemplateVersionsInput{
			LaunchTemplateId: ng.LaunchTemplate.Id,
			Versions:         []string{*version},
		})
	})
	if err != nil {
		// Launch template deleted upstream — that is a true zero, not a
		// fetch failure: there is no AMI for this NG to relate to.
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) && apiErr.ErrorCode() == "InvalidLaunchTemplateId.NotFound" {
			return resource.RelatedCheckResult{TargetType: "ami", Count: 0}
		}
		return resource.RelatedCheckResult{TargetType: "ami", Count: -1, Err: err}
	}
	for _, v := range ltOut.LaunchTemplateVersions {
		if v.LaunchTemplateData != nil && v.LaunchTemplateData.ImageId != nil && *v.LaunchTemplateData.ImageId != "" {
			return relatedResult("ami", []string{*v.LaunchTemplateData.ImageId})
		}
	}
	return resource.RelatedCheckResult{TargetType: "ami", Count: 0}
}

// checkNGEBS resolves EBS volume IDs for instances in this node group.
// Path: NG.Resources.AutoScalingGroups → autoscaling:DescribeAutoScalingGroups →
// Instances[].InstanceId → ec2:DescribeInstances → BlockDeviceMappings[].Ebs.VolumeId.
func checkNGEBS(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	ng, ok := assertStruct[ekstypes.Nodegroup](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "ebs", Count: -1}
	}
	if ng.Resources == nil || len(ng.Resources.AutoScalingGroups) == 0 {
		return resource.RelatedCheckResult{TargetType: "ebs", Count: 0}
	}

	var asgNames []string
	for _, asg := range ng.Resources.AutoScalingGroups {
		if asg.Name != nil && *asg.Name != "" {
			asgNames = append(asgNames, *asg.Name)
		}
	}
	if len(asgNames) == 0 {
		return resource.RelatedCheckResult{TargetType: "ebs", Count: 0}
	}

	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.AutoScaling == nil || c.EC2 == nil {
		return resource.RelatedCheckResult{TargetType: "ebs", Count: -1}
	}

	asgOut, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*autoscalingPkg.DescribeAutoScalingGroupsOutput, error) {
		return c.AutoScaling.DescribeAutoScalingGroups(ctx, &autoscalingPkg.DescribeAutoScalingGroupsInput{
			AutoScalingGroupNames: asgNames,
		})
	})
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "ebs", Count: -1, Err: err}
	}

	var instanceIDs []string
	for _, asg := range asgOut.AutoScalingGroups {
		for _, inst := range asg.Instances {
			if inst.InstanceId != nil && *inst.InstanceId != "" {
				instanceIDs = append(instanceIDs, *inst.InstanceId)
			}
		}
	}
	if len(instanceIDs) == 0 {
		return resource.RelatedCheckResult{TargetType: "ebs", Count: 0}
	}

	ec2Out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*ec2.DescribeInstancesOutput, error) {
		return c.EC2.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
			InstanceIds: instanceIDs,
		})
	})
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "ebs", Count: -1, Err: err}
	}

	seen := make(map[string]struct{})
	for _, res := range ec2Out.Reservations {
		for _, inst := range res.Instances {
			for _, bdm := range inst.BlockDeviceMappings {
				if bdm.Ebs != nil && bdm.Ebs.VolumeId != nil && *bdm.Ebs.VolumeId != "" {
					seen[*bdm.Ebs.VolumeId] = struct{}{}
				}
			}
		}
	}
	var ids []string
	for id := range seen {
		ids = append(ids, id)
	}
	return relatedResult("ebs", ids)
}

// checkNGSubnet returns the subnet IDs this node group deploys into.
// Pattern F — no AWS call needed; data is in Subnets[] on the Nodegroup struct.
func checkNGSubnet(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	ng, ok := assertStruct[ekstypes.Nodegroup](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "subnet", Count: -1}
	}
	var ids []string
	for _, s := range ng.Subnets {
		if s != "" {
			ids = append(ids, s)
		}
	}
	if len(ids) == 0 {
		return resource.RelatedCheckResult{TargetType: "subnet", Count: 0}
	}
	return relatedResult("subnet", ids)
}

// ngRelatedResources returns the resource list for target from cache or by fetching the first page.
func ngRelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}
