// eip_related.go contains Elastic IP related-resource checker functions.
package aws

import (
	"context"
	"strings"

	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkEIPEC2 returns the EC2 instance associated with this Elastic IP (Pattern F).
func checkEIPEC2(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ec2types.Address](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("ec2")
	}
	if raw.InstanceId == nil || *raw.InstanceId == "" {
		return resource.RelatedCheckResult{TargetType: "ec2", Count: 0}
	}
	return relatedResult("ec2", []string{*raw.InstanceId})
}

// checkEIPENI returns the network interface associated with this Elastic IP (Pattern F).
func checkEIPENI(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ec2types.Address](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("eni")
	}
	if raw.NetworkInterfaceId == nil || *raw.NetworkInterfaceId == "" {
		return resource.RelatedCheckResult{TargetType: "eni", Count: 0}
	}
	return relatedResult("eni", []string{*raw.NetworkInterfaceId})
}

// checkEIPNAT checks the NAT gateway cache for NAT gateways using this Elastic IP
// allocation (Pattern C — search target cache by AllocationId).
func checkEIPNAT(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	// Resolve the allocation ID from the RawStruct or from res.ID.
	allocationID := res.ID
	raw, ok := assertStruct[ec2types.Address](res.RawStruct)
	if ok && raw.AllocationId != nil && *raw.AllocationId != "" {
		allocationID = *raw.AllocationId
	}
	if allocationID == "" {
		return resource.RelatedCheckResult{TargetType: "nat", Count: 0}
	}

	natList, truncated, err := eipRelatedResources(ctx, clients, cache, "nat")
	if err != nil {
		return resource.ErrorRelated("nat", err)
	}
	if natList == nil {
		return resource.UnknownRelated("nat")
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
		// Fallback: check Fields keys for allocation ID values.
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
		return resource.UnknownRelated("cfn")
	}
	stackName := tagValue(raw.Tags, "aws:cloudformation:stack-name")
	if stackName == "" {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: 0}
	}
	return relatedResult("cfn", []string{stackName})
}

// checkEIPAlarm reports CloudWatch alarms on entities this EIP is attached
// to. EIPs have no CW dimension of their own; alarms operationally related
// to an EIP target the InstanceId or NetworkInterfaceId it's attached to.
// Scans the alarm cache for those dimension values.
func checkEIPAlarm(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ec2types.Address](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("alarm")
	}
	wanted := map[string]string{}
	if raw.InstanceId != nil && *raw.InstanceId != "" {
		wanted["InstanceId"] = *raw.InstanceId
	}
	if raw.NetworkInterfaceId != nil && *raw.NetworkInterfaceId != "" {
		wanted["NetworkInterfaceId"] = *raw.NetworkInterfaceId
	}
	if len(wanted) == 0 {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: 0}
	}
	alarmList, _, err := FetchRelatedTarget(ctx, clients, cache, "alarm")
	if err != nil {
		if _, sok := clients.(*ServiceClients); !sok {
			return resource.UnknownRelated("alarm")
		}
		return resource.ErrorRelated("alarm", err)
	}
	if alarmList == nil {
		return resource.UnknownRelated("alarm")
	}
	seen := map[string]bool{}
	var ids []string
	for _, alarmRes := range alarmList {
		a, aok := assertStruct[cwtypes.MetricAlarm](alarmRes.RawStruct)
		if !aok {
			continue
		}
		for _, d := range a.Dimensions {
			if d.Name == nil || d.Value == nil {
				continue
			}
			if v, exists := wanted[*d.Name]; exists && v == *d.Value {
				if !seen[alarmRes.ID] {
					seen[alarmRes.ID] = true
					ids = append(ids, alarmRes.ID)
				}
				break
			}
		}
	}
	return relatedResult("alarm", ids)
}

// checkEIPASG reports Auto Scaling Groups whose instances hold this EIP.
// Cache-based: if this EIP is attached to an EC2 instance, look up that
// instance in the ec2 cache, read its aws:autoscaling:groupName tag, then
// match the name against the asg cache.
func checkEIPASG(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ec2types.Address](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("asg")
	}
	if raw.InstanceId == nil || *raw.InstanceId == "" {
		return resource.RelatedCheckResult{TargetType: "asg", Count: 0}
	}
	instanceID := *raw.InstanceId

	ec2List, _, err := FetchRelatedTarget(ctx, clients, cache, "ec2")
	if err != nil {
		if _, sok := clients.(*ServiceClients); !sok {
			return resource.UnknownRelated("asg")
		}
		return resource.ErrorRelated("asg", err)
	}
	if ec2List == nil {
		return resource.UnknownRelated("asg")
	}
	asgName := ""
	for _, ec2Res := range ec2List {
		if ec2Res.ID != instanceID {
			continue
		}
		inst, iok := assertStruct[ec2types.Instance](ec2Res.RawStruct)
		if !iok {
			break
		}
		asgName = tagValue(inst.Tags, "aws:autoscaling:groupName")
		break
	}
	if asgName == "" {
		return resource.RelatedCheckResult{TargetType: "asg", Count: 0}
	}
	return relatedResult("asg", []string{asgName})
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
	taskList, truncated, err := eipRelatedResources(ctx, clients, cache, "ecs-task")
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
		return resource.RelatedCheckResult{TargetType: "ecs-task", Count: 0}
	}
	taskRes, truncated, err := eipMatchingECSTask(ctx, clients, cache, eniID)
	if err != nil {
		return resource.ErrorRelated("ecs-task", err)
	}
	if taskRes.ID == "" {
		if truncated {
			return relatedResultTrunc("ecs-task", nil, true)
		}
		return resource.RelatedCheckResult{TargetType: "ecs-task", Count: 0}
	}
	return relatedResult("ecs-task", []string{taskRes.ID})
}

// checkEIPECSSvc reports the ECS service whose task currently holds this EIP,
// resolved via the same ecs-task cache join as checkEIPECSTask, then reading
// the task's Group field ("service:<name>" convention) to name the service.
func checkEIPECSSvc(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	eniID := eipENIID(res)
	if eniID == "" {
		return resource.RelatedCheckResult{TargetType: "ecs-svc", Count: 0}
	}
	taskRes, truncated, err := eipMatchingECSTask(ctx, clients, cache, eniID)
	if err != nil {
		return resource.ErrorRelated("ecs-svc", err)
	}
	if taskRes.ID == "" {
		if truncated {
			return relatedResultTrunc("ecs-svc", nil, true)
		}
		return resource.RelatedCheckResult{TargetType: "ecs-svc", Count: 0}
	}
	task, ok := assertStruct[ecstypes.Task](taskRes.RawStruct)
	if !ok || task.Group == nil || !strings.HasPrefix(*task.Group, "service:") {
		return resource.RelatedCheckResult{TargetType: "ecs-svc", Count: 0}
	}
	svcName := strings.TrimPrefix(*task.Group, "service:")
	if svcName == "" {
		return resource.RelatedCheckResult{TargetType: "ecs-svc", Count: 0}
	}
	return relatedResult("ecs-svc", []string{svcName})
}

// checkEIPECS reports the ECS cluster whose task currently holds this EIP,
// resolved via the same ecs-task cache join as checkEIPECSTask, then reading
// the task's ClusterArn.
func checkEIPECS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	eniID := eipENIID(res)
	if eniID == "" {
		return resource.RelatedCheckResult{TargetType: "ecs", Count: 0}
	}
	taskRes, truncated, err := eipMatchingECSTask(ctx, clients, cache, eniID)
	if err != nil {
		return resource.ErrorRelated("ecs", err)
	}
	if taskRes.ID == "" {
		if truncated {
			return relatedResultTrunc("ecs", nil, true)
		}
		return resource.RelatedCheckResult{TargetType: "ecs", Count: 0}
	}
	task, ok := assertStruct[ecstypes.Task](taskRes.RawStruct)
	if !ok || task.ClusterArn == nil || *task.ClusterArn == "" {
		return resource.RelatedCheckResult{TargetType: "ecs", Count: 0}
	}
	clusterName := arnLastSegment(*task.ClusterArn)
	if clusterName == "" {
		return resource.RelatedCheckResult{TargetType: "ecs", Count: 0}
	}
	return relatedResult("ecs", []string{clusterName})
}

// eipRelatedResources returns the resource list for target from cache or fetches
// the first page via the registered paginated fetcher.
func eipRelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}
