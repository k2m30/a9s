// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// ec2_related.go contains EC2 related-resource checker functions and shared helpers.
package aws

import (
	"cmp"
	"context"
	"slices"
	"sort"

	"github.com/aws/aws-sdk-go-v2/aws"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkEC2TargetGroups counts the instance target groups the instance is
// registered in, read from each group's DescribeTargetHealth. A target group
// holds only targets in its own VPC (DescribeTargetHealth's InvalidTarget: "is
// not in the same VPC as the target group"), so only the instance's VPC's
// groups are asked.
func checkEC2TargetGroups(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	instanceID, vpcID, _ := ec2Identity(res)
	if instanceID == "" {
		return foundNone("tg", "instanceID")
	}
	tgList, truncated, err := relatedResourcesFor(ctx, clients, cache, "tg")
	if err != nil {
		return ReadFailed("tg", err)
	}
	if tgList == nil {
		return NotRead("tg")
	}
	var candidates []resource.Resource
	for _, tgRes := range tgList {
		raw, ok := assertStruct[elbv2types.TargetGroup](tgRes.RawStruct)
		targetType, tgVpcID := tgRes.Fields["target_type"], tgRes.Fields["vpc_id"]
		if ok {
			targetType, tgVpcID = string(raw.TargetType), cmp.Or(aws.ToString(raw.VpcId), tgVpcID)
		}
		if targetType == string(elbv2types.TargetTypeEnumInstance) && (vpcID == "" || tgVpcID == vpcID) {
			candidates = append(candidates, tgRes)
		}
	}
	registered := tgsRegistering(ctx, clients, candidates, "ec2-related: DescribeTargetHealth", func(id string) bool { return id == instanceID })
	return relatedAnswer("tg", joinReads(registered, relatedRead{partial: truncated}))
}

// checkEC2ASG checks the cache for ASGs containing this EC2 instance.
func checkEC2ASG(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	instanceID, _, _ := ec2Identity(res)
	if instanceID == "" {
		return foundNone("asg", "instanceID")
	}
	asgList, truncated, err := relatedResourcesFor(ctx, clients, cache, "asg")
	if err != nil {
		return ReadFailed("asg", err)
	}
	if asgList == nil {
		return NotRead("asg")
	}
	var ids []string
	for _, asgRes := range asgList {
		raw, ok := assertStruct[asgtypes.AutoScalingGroup](asgRes.RawStruct)
		if !ok {
			continue
		}
		if slices.Contains(asgMembers(raw), instanceID) {
			ids = append(ids, asgRes.ID)
		}
	}
	return relatedResultTrunc("asg", ids, truncated)
}

func checkEC2Alarms(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return alarmIDsByDimension(ctx, clients, cache, "ec2", res)
}

// checkEC2CFN checks instance tags for aws:cloudformation:stack-name.
func checkEC2CFN(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	if res.RawStruct == nil {
		return NotRead("cfn")
	}
	_, _, stackName := ec2Identity(res)
	if stackName == "" {
		return unreadZero(res, foundNone("cfn", "stackName"))
	}
	cfnList, truncated, err := relatedResourcesFor(ctx, clients, cache, "cfn")
	if err != nil {
		return ReadFailed("cfn", err)
	}
	if cfnList == nil {
		return NotRead("cfn")
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
		return foundNone("eip", "instanceID")
	}
	eipList, truncated, err := relatedResourcesFor(ctx, clients, cache, "eip")
	if err != nil {
		return ReadFailed("eip", err)
	}
	if eipList == nil {
		return NotRead("eip")
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
		return unreadZero(res, foundNone("ebs", "ids"))
	}
	ordered := make([]string, 0, len(ids))
	for id := range ids {
		ordered = append(ordered, id)
	}
	sort.Strings(ordered)
	return unreadZero(res, relatedResultTrunc("ebs", ordered, false))
}

// checkEC2NodeGroups checks for EKS node groups associated with this EC2 instance.
// Returns an unknown result when the cache is truncated and no match was found
// in the partial list.
func checkEC2NodeGroups(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	if res.RawStruct == nil {
		return NotRead("ng")
	}
	instanceID, _, _ := ec2Identity(res)
	if instanceID == "" {
		return unreadZero(res, foundNone("ng", "instanceID"))
	}
	inst, ok := assertStruct[ec2types.Instance](res.RawStruct)
	if !ok {
		return NotRead("ng")
	}
	if tagValue(inst.Tags, "eks:nodegroup-name") == "" {
		return unreadZero(res, foundNone("ng", "the eks:nodegroup-name tag"))
	}
	ngList, truncated, err := relatedRowsByID(ctx, clients, cache, "ng")
	if err != nil {
		return ReadFailed("ng", err)
	}
	if ngList == nil {
		return NotRead("ng")
	}
	var ids []string
	for _, ngRes := range ngList {
		nodegroupName, clusterName := ngIdentity(ngRes)
		if ngOwnsInstance(inst.Tags, nodegroupName, clusterName) {
			ids = append(ids, ngRes.ID)
		}
	}
	return unreadZeroScanned(res, len(ngList), relatedAnswer("ng", relatedRead{ids: ids, partial: truncated, atMostOne: true}))
}

// checkEC2EBSSnap counts the snapshots of the instance's volumes and the
// snapshots its AMI is registered from (Image.BlockDeviceMappings[].Ebs.SnapshotId).
func checkEC2EBSSnap(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	volumeIDs := ec2VolumeIDs(res)
	image := ec2AMISnapshots(ctx, clients, cache, res)
	if len(volumeIDs) == 0 {
		return unreadZero(res, relatedAnswer("ebs-snap", image))
	}
	snapList, truncated, err := relatedResourcesFor(ctx, clients, cache, "ebs-snap")
	if snapList == nil {
		return relatedAnswer("ebs-snap", joinReads(image, unreadBy(err)))
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
	return unreadZeroScanned(res, len(snapList), relatedAnswer("ebs-snap", joinReads(image, relatedRead{ids: ids, partial: truncated})))
}

// ec2AMISnapshots reads the snapshots the instance's AMI is registered from,
// off the AMI's row in the ami list, as the ami → ebs-snap pivot counts them.
// An AMI the list does not hold is not the account's, and neither are its
// snapshots.
func ec2AMISnapshots(ctx context.Context, clients any, cache resource.ResourceCache, res resource.Resource) relatedRead {
	inst, _ := assertStruct[ec2types.Instance](res.RawStruct)
	imageID := cmp.Or(aws.ToString(inst.ImageId), res.Fields["image_id"])
	if imageID == "" {
		return relatedRead{}
	}
	amis, truncated, err := FetchRelatedTarget(ctx, clients, cache, "ami")
	if amis == nil {
		return unreadBy(err)
	}
	for _, a := range amis {
		if img, ok := assertStruct[ec2types.Image](a.RawStruct); ok && a.ID == imageID {
			return relatedRead{ids: amiSnapshotIDs(img), failure: err}
		}
	}
	return relatedRead{partial: truncated, failure: err}
}

// checkEC2SG extracts security group IDs from the EC2 Instance's SecurityGroups slice.
// Pattern F — no cache needed.
func checkEC2SG(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ec2types.Instance](res.RawStruct)
	if !ok {
		return NotRead("sg")
	}
	var ids []string
	for _, sg := range raw.SecurityGroups {
		if sg.GroupId != nil && *sg.GroupId != "" {
			ids = append(ids, *sg.GroupId)
		}
	}
	return relatedResultTrunc("sg", ids, false)
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
		return foundNone("vpc", "vpcID")
	}
	return relatedResultTrunc("vpc", []string{vpcID}, false)
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
			return NotRead("role")
		}
		return foundNone("role", "inst.IamInstanceProfile.Arn")
	}
	profileName := instanceProfileName(*inst.IamInstanceProfile.Arn)
	if profileName == "" {
		return foundNone("role", "profileName")
	}
	c, err := svcClients(clients)
	if err != nil {
		return ReadFailed("role", err)
	}
	// no finding: without the IAM client nothing was read.
	if c.IAM == nil {
		return NotRead("role")
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*iam.GetInstanceProfileOutput, error) {
		return c.IAM.GetInstanceProfile(ctx, &iam.GetInstanceProfileInput{
			InstanceProfileName: aws.String(profileName),
		})
	})
	if err != nil {
		return ReadFailed("role", err)
	}
	var refs []string
	if out.InstanceProfile != nil {
		for _, r := range out.InstanceProfile.Roles {
			refs = append(refs, cmp.Or(aws.ToString(r.Arn), aws.ToString(r.RoleName)))
		}
	}
	ids, dropped := resolveRefs("role", refs, refContext(clients, cache, "role"))
	return relatedResultTrunc("role", ids, dropped)
}
