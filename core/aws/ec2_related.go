// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// ec2_related.go contains EC2 related-resource checker functions and shared helpers.
package aws

import (
	"context"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkEC2TargetGroups checks the cache for target groups referencing this EC2 instance.
func checkEC2TargetGroups(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	instanceID, vpcID, _ := ec2Identity(res)
	if instanceID == "" {
		return resource.KnownRelated("tg", nil, false)
	}
	tgList, truncated, err := relatedResourcesFor(ctx, clients, cache, "tg")
	if err != nil {
		return resource.ErrorRelated("tg", err)
	}
	if tgList == nil {
		return resource.UnknownRelated("tg")
	}
	var ids []string
	for _, tgRes := range tgList {
		raw, ok := assertStruct[elbv2types.TargetGroup](tgRes.RawStruct)
		targetType := tgRes.Fields["target_type"]
		tgVpcID := tgRes.Fields["vpc_id"]
		if ok {
			targetType = string(raw.TargetType)
			if raw.VpcId != nil {
				tgVpcID = *raw.VpcId
			}
		}
		if targetType != "instance" {
			continue
		}
		// Without target-health rows in cache, best available approximation is
		// VPC-level matching for instance target groups.
		if tgVpcID == vpcID {
			ids = append(ids, tgRes.ID)
		}
	}
	return relatedResultTrunc("tg", ids, truncated)
}

// checkEC2ASG checks the cache for ASGs containing this EC2 instance.
func checkEC2ASG(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	instanceID, _, _ := ec2Identity(res)
	if instanceID == "" {
		return resource.KnownRelated("asg", nil, false)
	}
	asgList, truncated, err := relatedResourcesFor(ctx, clients, cache, "asg")
	if err != nil {
		return resource.ErrorRelated("asg", err)
	}
	if asgList == nil {
		return resource.UnknownRelated("asg")
	}
	var ids []string
	for _, asgRes := range asgList {
		raw, ok := assertStruct[asgtypes.AutoScalingGroup](asgRes.RawStruct)
		if !ok {
			continue
		}
		for _, inst := range raw.Instances {
			if inst.InstanceId != nil && *inst.InstanceId == instanceID {
				ids = append(ids, asgRes.ID)
				break
			}
		}
	}
	return relatedResultTrunc("asg", ids, truncated)
}

// checkEC2Alarms checks the cache for CloudWatch alarms targeting this EC2 instance.
func checkEC2Alarms(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	instanceID, _, _ := ec2Identity(res)
	return alarmIDsByDimension(ctx, clients, cache, "", "InstanceId", instanceID)
}

// checkEC2CFN checks instance tags for aws:cloudformation:stack-name.
func checkEC2CFN(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	if res.RawStruct == nil {
		return unreadZero(res, resource.KnownRelated("cfn", nil, false))
	}
	_, _, stackName := ec2Identity(res)
	if stackName == "" {
		return unreadZero(res, resource.KnownRelated("cfn", nil, false))
	}
	cfnList, truncated, err := relatedResourcesFor(ctx, clients, cache, "cfn")
	if err != nil {
		return resource.ErrorRelated("cfn", err)
	}
	if cfnList == nil {
		return resource.UnknownRelated("cfn")
	}
	var ids []string
	for _, cfnRes := range cfnList {
		if cfnRes.ID == stackName || cfnRes.Name == stackName || cfnRes.Fields["stack_name"] == stackName {
			ids = append(ids, cfnRes.ID)
			continue
		}
		raw, ok := assertStruct[cfntypes.Stack](cfnRes.RawStruct)
		if ok && raw.StackName != nil && *raw.StackName == stackName {
			ids = append(ids, cfnRes.ID)
		}
	}
	return unreadZeroScanned(res, len(cfnList), relatedResultTrunc("cfn", ids, truncated))
}

// checkEC2EIP checks the cache for Elastic IPs associated with this EC2 instance.
func checkEC2EIP(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	instanceID, _, _ := ec2Identity(res)
	if instanceID == "" {
		return resource.KnownRelated("eip", nil, false)
	}
	eipList, truncated, err := relatedResourcesFor(ctx, clients, cache, "eip")
	if err != nil {
		return resource.ErrorRelated("eip", err)
	}
	if eipList == nil {
		return resource.UnknownRelated("eip")
	}
	var ids []string
	for _, eipRes := range eipList {
		raw, ok := assertStruct[ec2types.Address](eipRes.RawStruct)
		if ok && raw.InstanceId != nil && *raw.InstanceId == instanceID {
			ids = append(ids, eipRes.ID)
			continue
		}
		if eipRes.Fields["instance_id"] == instanceID {
			ids = append(ids, eipRes.ID)
		}
	}
	return relatedResultTrunc("eip", ids, truncated)
}

func checkEC2EBS(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	ids := ec2VolumeIDs(res)
	if len(ids) == 0 {
		return unreadZero(res, resource.KnownRelated("ebs", nil, false))
	}
	ordered := make([]string, 0, len(ids))
	for id := range ids {
		ordered = append(ordered, id)
	}
	sort.Strings(ordered)
	return unreadZero(res, relatedResult("ebs", ordered))
}

// checkEC2NodeGroups checks for EKS node groups associated with this EC2 instance.
// Returns an unknown result when the cache is truncated and no match was found
// in the partial list.
func checkEC2NodeGroups(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	if res.RawStruct == nil {
		return unreadZero(res, resource.KnownRelated("ng", nil, false))
	}
	instanceID, _, _ := ec2Identity(res)
	if instanceID == "" {
		return unreadZero(res, resource.KnownRelated("ng", nil, false))
	}
	tags := ec2Tags(res)
	clusterName := tags["eks:cluster-name"]
	nodegroupName := tags["eks:nodegroup-name"]
	if clusterName == "" && nodegroupName == "" {
		return unreadZero(res, resource.KnownRelated("ng", nil, false))
	}
	ngList, truncated, err := relatedResourcesFor(ctx, clients, cache, "ng")
	if err != nil {
		return resource.ErrorRelated("ng", err)
	}
	if ngList == nil {
		return resource.UnknownRelated("ng")
	}
	var ids []string
	for _, ngRes := range ngList {
		raw, ok := assertStruct[ekstypes.Nodegroup](ngRes.RawStruct)
		rawClusterName := ngRes.Fields["cluster_name"]
		rawNodegroupName := ngRes.Fields["nodegroup_name"]
		if ok {
			if raw.ClusterName != nil {
				rawClusterName = *raw.ClusterName
			}
			if raw.NodegroupName != nil {
				rawNodegroupName = *raw.NodegroupName
			}
		}
		if clusterName != "" && rawClusterName != "" && clusterName != rawClusterName {
			continue
		}
		if nodegroupName != "" && rawNodegroupName != "" && nodegroupName != rawNodegroupName {
			continue
		}
		if rawNodegroupName != "" {
			ids = append(ids, ngRes.ID)
		}
	}
	return unreadZeroScanned(res, len(ngList), relatedResultTrunc("ng", ids, truncated))
}

// checkEC2CloudTrailEvents checks cached CloudTrail events for references to the
// instance. Returns an unknown result when the cache is truncated — the partial
// list cannot yield a definitive count. FetchFilter["ResourceName"] is always set
// so the caller can do a filtered re-fetch.
func checkEC2CloudTrailEvents(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	instanceID, _, _ := ec2Identity(res)
	if instanceID == "" {
		return resource.KnownRelated("ct-events", nil, false)
	}
	eventList, truncated, err := relatedResourcesFor(ctx, clients, cache, "ct-events")
	if err != nil {
		return resource.ErrorRelated("ct-events", err)
	}
	if eventList == nil {
		return resource.UnknownRelated("ct-events")
	}
	var ids []string
	for _, eventRes := range eventList {
		raw, ok := assertStruct[cloudtrailtypes.Event](eventRes.RawStruct)
		if ok {
			if cloudTrailEventMentionsInstance(raw, instanceID) {
				ids = append(ids, eventRes.ID)
			}
			continue
		}
		if eventRes.Fields["resource_name"] == instanceID {
			ids = append(ids, eventRes.ID)
		}
	}
	fetchFilter := map[string]string{"ResourceName": instanceID}
	if truncated {
		// Cache is partial — the filtered fetch will determine the real count.
		return resource.DeferredRelated("ct-events", fetchFilter)
	}
	return relatedResult("ct-events", ids).WithFetchFilter(fetchFilter)
}

// checkEC2EBSSnap checks the cache for EBS snapshots belonging to this EC2 instance.
func checkEC2EBSSnap(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	volumeIDs := ec2VolumeIDs(res)
	if len(volumeIDs) == 0 {
		return unreadZero(res, resource.KnownRelated("ebs-snap", nil, false))
	}
	snapList, truncated, err := relatedResourcesFor(ctx, clients, cache, "ebs-snap")
	if err != nil {
		return resource.ErrorRelated("ebs-snap", err)
	}
	if snapList == nil {
		return resource.UnknownRelated("ebs-snap")
	}
	var ids []string
	for _, snapRes := range snapList {
		raw, ok := assertStruct[ec2types.Snapshot](snapRes.RawStruct)
		volumeID := snapRes.Fields["volume_id"]
		if ok && raw.VolumeId != nil {
			volumeID = *raw.VolumeId
		}
		if _, found := volumeIDs[volumeID]; found {
			ids = append(ids, snapRes.ID)
		}
	}
	return unreadZeroScanned(res, len(snapList), relatedResultTrunc("ebs-snap", ids, truncated))
}

// checkEC2SG extracts security group IDs from the EC2 Instance's SecurityGroups slice.
// Pattern F — no cache needed.
func checkEC2SG(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ec2types.Instance](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("sg")
	}
	var ids []string
	for _, sg := range raw.SecurityGroups {
		if sg.GroupId != nil && *sg.GroupId != "" {
			ids = append(ids, *sg.GroupId)
		}
	}
	return relatedResult("sg", ids)
}

func ec2Identity(res resource.Resource) (instanceID, vpcID, stackName string) {
	instanceID = res.ID
	raw, ok := assertStruct[ec2types.Instance](res.RawStruct)
	if !ok {
		return instanceID, res.Fields["vpc_id"], ""
	}
	if raw.InstanceId != nil {
		instanceID = *raw.InstanceId
	}
	if raw.VpcId != nil {
		vpcID = *raw.VpcId
	}
	for _, tag := range raw.Tags {
		if tag.Key == nil || tag.Value == nil {
			continue
		}
		if *tag.Key == "aws:cloudformation:stack-name" {
			stackName = *tag.Value
			break
		}
	}
	return instanceID, vpcID, stackName
}

func ec2Tags(res resource.Resource) map[string]string {
	tags := map[string]string{}
	raw, ok := assertStruct[ec2types.Instance](res.RawStruct)
	if !ok {
		return tags
	}
	for _, tag := range raw.Tags {
		if tag.Key == nil || tag.Value == nil {
			continue
		}
		tags[*tag.Key] = *tag.Value
	}
	return tags
}

// checkEC2SSM checks whether this EC2 instance is managed by SSM by calling
// ssm:DescribeInstanceInformation filtered by InstanceIds (Pattern C: 1 API call).
// If the response contains at least one entry the instance is SSM-managed and
// the instance ID is returned as the single resource ID.
func checkEC2SSM(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	instanceID, _, _ := ec2Identity(res)
	if instanceID == "" {
		return resource.KnownRelated("ssm", nil, false)
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.SSM == nil {
		return resource.UnknownRelated("ssm")
	}
	api, ok := c.SSM.(SSMDescribeInstanceInformationAPI)
	if !ok {
		return resource.UnknownRelated("ssm")
	}
	filterKey := "InstanceIds"
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*ssm.DescribeInstanceInformationOutput, error) {
		return api.DescribeInstanceInformation(ctx, &ssm.DescribeInstanceInformationInput{
			Filters: []ssmtypes.InstanceInformationStringFilter{
				{Key: &filterKey, Values: []string{instanceID}},
			},
		})
	})
	if err != nil {
		return resource.ErrorRelated("ssm", err)
	}
	if len(out.InstanceInformationList) == 0 {
		return resource.KnownRelated("ssm", nil, false)
	}
	return relatedResult("ssm", []string{instanceID})
}

func cloudTrailEventMentionsInstance(event cloudtrailtypes.Event, instanceID string) bool {
	for _, rr := range event.Resources {
		if rr.ResourceName != nil && *rr.ResourceName == instanceID {
			return true
		}
	}
	return false
}

func ec2VolumeIDs(res resource.Resource) map[string]struct{} {
	ids := map[string]struct{}{}
	raw, ok := assertStruct[ec2types.Instance](res.RawStruct)
	if !ok {
		return ids
	}
	for _, bdm := range raw.BlockDeviceMappings {
		if bdm.Ebs == nil || bdm.Ebs.VolumeId == nil {
			continue
		}
		ids[*bdm.Ebs.VolumeId] = struct{}{}
	}
	return ids
}

// tagValue extracts a tag value from a slice of EC2 tags.
// Used by CFN checkers across multiple resource types.
func tagValue(tags []ec2types.Tag, key string) string {
	for _, t := range tags {
		if t.Key != nil && *t.Key == key && t.Value != nil {
			return *t.Value
		}
	}
	return ""
}

// checkEC2VPC returns the VPC this EC2 instance runs in (Pattern F).
// Reads vpc_id from Fields which is populated by the EC2 fetcher.
func checkEC2VPC(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	vpcID := res.Fields["vpc_id"]
	if vpcID == "" {
		return resource.KnownRelated("vpc", nil, false)
	}
	return relatedResult("vpc", []string{vpcID})
}

// checkEC2Role resolves the IAM role backing the EC2 Instance's instance
// profile (Instance.IamInstanceProfile.Arn, form
// arn:aws:iam::ACCOUNT:instance-profile/PROFILE-NAME). The last ARN segment
// is the PROFILE name, not the role name — they only coincide in manual
// setups; EKS/ASG-generated profiles commonly use a different name for the
// role they carry. Manual setups (profile name == role name) are the common
// case, so the already-loaded role cache is checked first for a zero-call
// match; only on a cache miss does this fall back to a single
// iam:GetInstanceProfile call to resolve the real role(s), mirroring
// asgInstanceProfileToRoles (asg_related.go) which resolves the same
// ARN/name ambiguity for ASG launch configs/templates.
func checkEC2Role(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	inst, ok := assertStruct[ec2types.Instance](res.RawStruct)
	if !ok || inst.IamInstanceProfile == nil || inst.IamInstanceProfile.Arn == nil || *inst.IamInstanceProfile.Arn == "" {
		if res.RawStruct == nil {
			return resource.UnknownRelated("role")
		}
		return resource.KnownRelated("role", nil, false)
	}
	arn := *inst.IamInstanceProfile.Arn
	idx := strings.LastIndex(arn, "/")
	if idx < 0 || idx >= len(arn)-1 {
		return resource.KnownRelated("role", nil, false)
	}
	profileName := arn[idx+1:]

	roleList, truncated, err := relatedResourcesFor(ctx, clients, cache, "role")
	if err != nil {
		return resource.ErrorRelated("role", err)
	}
	for _, roleRes := range roleList {
		if roleRes.Name == profileName || roleRes.Fields["role_name"] == profileName {
			return relatedResult("role", []string{profileName})
		}
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.IAM == nil {
		if truncated {
			return relatedResultTrunc("role", nil, true)
		}
		return resource.KnownRelated("role", nil, false)
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*iam.GetInstanceProfileOutput, error) {
		return c.IAM.GetInstanceProfile(ctx, &iam.GetInstanceProfileInput{
			InstanceProfileName: aws.String(profileName),
		})
	})
	if err != nil {
		return resource.ErrorRelated("role", err)
	}
	if out == nil || out.InstanceProfile == nil || len(out.InstanceProfile.Roles) == 0 {
		return resource.KnownRelated("role", nil, false)
	}
	var ids []string
	for _, r := range out.InstanceProfile.Roles {
		if r.RoleName != nil && *r.RoleName != "" {
			ids = append(ids, *r.RoleName)
		}
	}
	return relatedResult("role", ids)
}
