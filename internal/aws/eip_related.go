// eip_related.go contains Elastic IP related-resource checker functions.
package aws

import (
	"context"

	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterNavigableFields("eip", []resource.NavigableField{
		{FieldPath: "InstanceId", TargetType: "ec2"},
		{FieldPath: "NetworkInterfaceId", TargetType: "eni"},
	})

	resource.RegisterRelated("eip", []resource.RelatedDef{
		{TargetType: "ec2", DisplayName: "EC2 Instances", Checker: checkEIPEC2},
		{TargetType: "eni", DisplayName: "Network Interfaces", Checker: checkEIPENI},
		{TargetType: "nat", DisplayName: "NAT Gateways", Checker: checkEIPNAT, NeedsTargetCache: true},
		{TargetType: "alarm", DisplayName: "CloudWatch Alarms", Checker: checkEIPAlarm},
		{TargetType: "asg", DisplayName: "Auto Scaling Groups", Checker: checkEIPASG},
		{TargetType: "cfn", DisplayName: "CloudFormation", Checker: checkEIPCFN},
		{TargetType: "ecs", DisplayName: "ECS Clusters", Checker: checkEIPECS},
		{TargetType: "ecs-svc", DisplayName: "ECS Services", Checker: checkEIPECSSvc},
		{TargetType: "ecs-task", DisplayName: "ECS Tasks", Checker: checkEIPECSTask},
		{TargetType: "logs", DisplayName: "Log Groups", Checker: checkEIPLogs},
	})
}

// checkEIPEC2 returns the EC2 instance associated with this Elastic IP (Pattern F).
func checkEIPEC2(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ec2types.Address](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "ec2", Count: -1}
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
		return resource.RelatedCheckResult{TargetType: "eni", Count: -1}
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
		return resource.RelatedCheckResult{TargetType: "nat", Count: -1, Err: err}
	}
	if natList == nil {
		return resource.RelatedCheckResult{TargetType: "nat", Count: -1}
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
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("nat")
	}
	return relatedResult("nat", ids)
}









// checkEIPCFN returns the CloudFormation stack that owns this EIP via
// aws:cloudformation:stack-name tag on the Address Tags slice.
func checkEIPCFN(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ec2types.Address](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1}
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
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1}
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
			return resource.RelatedCheckResult{TargetType: "alarm", Count: -1}
		}
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1, Err: err}
	}
	if alarmList == nil {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1}
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
		return resource.RelatedCheckResult{TargetType: "asg", Count: -1}
	}
	if raw.InstanceId == nil || *raw.InstanceId == "" {
		return resource.RelatedCheckResult{TargetType: "asg", Count: 0}
	}
	instanceID := *raw.InstanceId

	ec2List, _, err := FetchRelatedTarget(ctx, clients, cache, "ec2")
	if err != nil {
		if _, sok := clients.(*ServiceClients); !sok {
			return resource.RelatedCheckResult{TargetType: "asg", Count: -1}
		}
		return resource.RelatedCheckResult{TargetType: "asg", Count: -1, Err: err}
	}
	if ec2List == nil {
		return resource.RelatedCheckResult{TargetType: "asg", Count: -1}
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

// checkEIPECS reports ECS clusters whose tasks currently hold this EIP.
// ECS tasks that use awsvpc networking get ENIs but do not attach EIPs
// directly; association requires DescribeTasks per cluster — outside the
// 1-call budget. Returns Count: -1.
func checkEIPECS(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	if res.ID == "" {
		return resource.RelatedCheckResult{TargetType: "ecs", Count: 0}
	}
	return resource.RelatedCheckResult{TargetType: "ecs", Count: -1}
}

// checkEIPECSSvc reports ECS services whose tasks currently hold this EIP.
// Same limitation as checkEIPECS — task networking is not in the list caches.
func checkEIPECSSvc(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	if res.ID == "" {
		return resource.RelatedCheckResult{TargetType: "ecs-svc", Count: 0}
	}
	return resource.RelatedCheckResult{TargetType: "ecs-svc", Count: -1}
}

// checkEIPECSTask reports ECS tasks currently holding this EIP.
// ECS task ENIs are not in the EIP list response; resolving requires
// DescribeTasks per cluster — outside the 1-call budget.
func checkEIPECSTask(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	if res.ID == "" {
		return resource.RelatedCheckResult{TargetType: "ecs-task", Count: 0}
	}
	return resource.RelatedCheckResult{TargetType: "ecs-task", Count: -1}
}

// checkEIPLogs reports CloudWatch log groups related to this EIP.
// EIPs themselves do not emit logs; flow logs attached to the associated
// ENI/subnet/VPC cover the traffic but are not identifiable from EIP ID
// without per-ENI DescribeFlowLogs — outside the 1-call budget.
func checkEIPLogs(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	if res.ID == "" {
		return resource.RelatedCheckResult{TargetType: "logs", Count: 0}
	}
	return resource.RelatedCheckResult{TargetType: "logs", Count: -1}
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
