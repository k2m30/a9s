// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// eip_related.go contains Elastic IP related-resource checker functions.
package aws

import (
	"context"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkEIPEC2 returns the EC2 instance associated with this Elastic IP (Pattern F).
func checkEIPEC2(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ec2types.Address](res.RawStruct)
	if !ok {
		return NotRead("ec2")
	}
	if raw.InstanceId == nil || *raw.InstanceId == "" {
		return foundNone("ec2", "raw.InstanceId")
	}
	return relatedResultTrunc("ec2", []string{*raw.InstanceId}, false)
}

// checkEIPENI returns the network interface associated with this Elastic IP (Pattern F).
func checkEIPENI(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ec2types.Address](res.RawStruct)
	if !ok {
		return NotRead("eni")
	}
	if raw.NetworkInterfaceId == nil || *raw.NetworkInterfaceId == "" {
		return foundNone("eni", "raw.NetworkInterfaceId")
	}
	return relatedResultTrunc("eni", []string{*raw.NetworkInterfaceId}, false)
}

// checkEIPNAT checks the NAT gateway cache for NAT gateways using this Elastic IP
// allocation (Pattern C — search target cache by AllocationId).
func checkEIPNAT(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	allocationID := res.ID
	raw, ok := assertStruct[ec2types.Address](res.RawStruct)
	if ok && raw.AllocationId != nil && *raw.AllocationId != "" {
		allocationID = *raw.AllocationId
	}
	if allocationID == "" {
		return foundNone("nat", "allocationID")
	}

	natList, truncated, err := relatedResourcesFor(ctx, clients, cache, "nat")
	if err != nil {
		return ReadFailed("nat", err)
	}
	if natList == nil {
		return NotRead("nat")
	}

	var ids []string
	for _, natRes := range natList {
		natRaw, natOk := assertStruct[ec2types.NatGateway](natRes.RawStruct)
		if natOk {
			for _, addr := range natRaw.NatGatewayAddresses {
				if addr.AllocationId != nil && *addr.AllocationId == allocationID {
					ids = append(ids, natRes.ID)
					break
				}
			}
			continue
		}
		for _, v := range natRes.Fields {
			if v == allocationID {
				ids = append(ids, natRes.ID)
				break
			}
		}
	}
	return relatedResultTrunc("nat", ids, truncated)
}

// checkEIPCFN returns the CloudFormation stack that owns this EIP via
// aws:cloudformation:stack-name tag on the Address Tags slice.
func checkEIPCFN(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ec2types.Address](res.RawStruct)
	if !ok {
		return NotRead("cfn")
	}
	stackName := tagValue(raw.Tags, "aws:cloudformation:stack-name")
	if stackName == "" {
		return foundNone("cfn", "stackName")
	}
	return relatedResultTrunc("cfn", []string{stackName}, false)
}

// checkEIPASG reports Auto Scaling Groups whose instances hold this EIP.
// Cache-based: if this EIP is attached to an EC2 instance, look up that
// instance in the ec2 cache, read its aws:autoscaling:groupName tag, then
// match the name against the asg cache.
func checkEIPASG(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ec2types.Address](res.RawStruct)
	if !ok {
		return NotRead("asg")
	}
	if raw.InstanceId == nil || *raw.InstanceId == "" {
		return foundNone("asg", "raw.InstanceId")
	}
	instanceID := *raw.InstanceId

	ec2List, truncated, err := FetchRelatedTarget(ctx, clients, cache, "ec2")
	if err != nil {
		return ReadFailed("asg", err)
	}
	if ec2List == nil {
		return NotRead("asg")
	}
	asgName := ""
	instanceReadable := false
	for _, ec2Res := range ec2List {
		if ec2Res.ID != instanceID {
			continue
		}
		inst, iok := assertStruct[ec2types.Instance](ec2Res.RawStruct)
		if !iok {
			break
		}
		instanceReadable = true
		asgName = tagValue(inst.Tags, "aws:autoscaling:groupName")
		break
	}
	if asgName != "" {
		return relatedResultTrunc("asg", []string{asgName}, false)
	}
	// The instance was located and read: its ASG membership is definitively
	// empty, regardless of whether other EC2 pages were truncated.
	if instanceReadable {
		return foundNone("asg", "instanceReadable")
	}
	// The instance is not on this page (or its struct was unreadable). A
	// truncated cache may hold it later, so report unknown rather than a
	// false zero; an exhaustive cache means it genuinely has no ASG.
	if truncated {
		return NotRead("asg")
	}
	return foundNone("asg", "the complete ec2 list")
}

// eipENIID resolves the network interface ID this Elastic IP is attached to,
// from RawStruct (preferred) or res.ID as a last resort. Returns "" when no
// ENI association exists.
func eipENIID(res resource.Resource) string {
	raw, ok := assertStruct[ec2types.Address](res.RawStruct)
	if ok && raw.NetworkInterfaceId != nil && *raw.NetworkInterfaceId != "" {
		return *raw.NetworkInterfaceId
	}
	return ""
}

// eipMatchingECSTask scans the already-loaded ecs-task cache for the task
// whose Attachments[].Details carries this ENI's networkInterfaceId
// (Pattern C — zero extra API calls, cache join only). Returns the matching
// task Resource and whether the ecs-task cache was truncated.
func eipMatchingECSTask(ctx context.Context, clients any, cache resource.ResourceCache, eniID string) (resource.Resource, bool, error) {
	taskList, truncated, err := relatedResourcesFor(ctx, clients, cache, "ecs-task")
	if err != nil {
		return resource.Resource{}, truncated, err
	}
	for _, taskRes := range taskList {
		task, ok := assertStruct[ecstypes.Task](taskRes.RawStruct)
		if !ok {
			continue
		}
		for _, att := range task.Attachments {
			for _, d := range att.Details {
				if d.Name != nil && *d.Name == "networkInterfaceId" && d.Value != nil && *d.Value == eniID {
					return taskRes, truncated, nil
				}
			}
		}
	}
	return resource.Resource{}, truncated, nil
}

// checkEIPECSTask reports the ECS task currently holding this EIP, resolved
// via a zero-extra-call join: this EIP's NetworkInterfaceId is matched
// against the ecs-task cache's Task.Attachments[].Details networkInterfaceId.
func checkEIPECSTask(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	eniID := eipENIID(res)
	if eniID == "" {
		return unreadZero(res, foundNone("ecs-task", "eniID"))
	}
	taskRes, truncated, err := eipMatchingECSTask(ctx, clients, cache, eniID)
	if err != nil {
		return ReadFailed("ecs-task", err)
	}
	if taskRes.ID == "" {
		return unreadZero(res, relatedResultTrunc("ecs-task", nil, truncated))
	}
	return unreadZero(res, relatedResultTrunc("ecs-task", []string{taskRes.ID}, false))
}

// checkEIPECSSvc reports the ECS service whose task currently holds this EIP,
// resolved via the same ecs-task cache join as checkEIPECSTask, then reading
// the task's Group field ("service:<name>" convention) to name the service.
func checkEIPECSSvc(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	eniID := eipENIID(res)
	if eniID == "" {
		return unreadZero(res, foundNone("ecs-svc", "eniID"))
	}
	taskRes, truncated, err := eipMatchingECSTask(ctx, clients, cache, eniID)
	if err != nil {
		return ReadFailed("ecs-svc", err)
	}
	if taskRes.ID == "" {
		return unreadZero(res, relatedResultTrunc("ecs-svc", nil, truncated))
	}
	task, ok := assertStruct[ecstypes.Task](taskRes.RawStruct)
	if !ok {
		return unreadZero(res, foundNone("ecs-svc", "task.Group"))
	}
	ref, ofService := ecsSvcRefFromTask(taskRes, task)
	if !ofService {
		return unreadZero(res, foundNone("ecs-svc", "task.Group"))
	}
	return unreadZero(res, relatedRefs("ecs-svc", []string{ref}, refContext(clients, cache, "ecs-svc")))
}

// checkEIPECS reports the ECS cluster whose task currently holds this EIP,
// resolved via the same ecs-task cache join as checkEIPECSTask, then reading
// the task's ClusterArn.
func checkEIPECS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	eniID := eipENIID(res)
	if eniID == "" {
		return unreadZero(res, foundNone("ecs", "eniID"))
	}
	taskRes, truncated, err := eipMatchingECSTask(ctx, clients, cache, eniID)
	if err != nil {
		return ReadFailed("ecs", err)
	}
	if taskRes.ID == "" {
		return unreadZero(res, relatedResultTrunc("ecs", nil, truncated))
	}
	task, ok := assertStruct[ecstypes.Task](taskRes.RawStruct)
	if !ok || task.ClusterArn == nil || *task.ClusterArn == "" {
		return unreadZero(res, foundNone("ecs", "task.ClusterArn"))
	}
	return unreadZero(res, relatedRefs("ecs", []string{*task.ClusterArn}, refContext(clients, cache, "ecs")))
}
