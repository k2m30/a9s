package aws

import (
	"context"
	"strings"

	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkASGEC2 reads Instances[] from the ASG RawStruct and returns their IDs.
// Pattern F — no cache needed.
func checkASGEC2(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	asg, ok := assertStruct[asgtypes.AutoScalingGroup](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "ec2", Count: -1}
	}
	var ids []string
	for _, inst := range asg.Instances {
		if inst.InstanceId != nil && *inst.InstanceId != "" {
			ids = append(ids, *inst.InstanceId)
		}
	}
	if len(ids) == 0 {
		return resource.RelatedCheckResult{TargetType: "ec2", Count: 0}
	}
	return relatedResult("ec2", ids)
}

// checkASGTG checks the cache for target groups referencing this ASG via TargetGroupARNs.
// Pattern C: ASG RawStruct has TargetGroupARNs; match against tg cache by ARN from RawStruct.
func checkASGTG(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	asg, ok := assertStruct[asgtypes.AutoScalingGroup](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "tg", Count: -1}
	}
	if len(asg.TargetGroupARNs) == 0 {
		return resource.RelatedCheckResult{TargetType: "tg", Count: 0}
	}

	arnSet := map[string]bool{}
	for _, arn := range asg.TargetGroupARNs {
		if arn != "" {
			arnSet[arn] = true
		}
	}

	tgList, truncated, err := asgRelatedResources(ctx, clients, cache, "tg")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "tg", Count: -1, Err: err}
	}
	if tgList == nil {
		return resource.RelatedCheckResult{TargetType: "tg", Count: -1}
	}

	var ids []string
	for _, tgRes := range tgList {
		raw, ok := assertStruct[elbv2types.TargetGroup](tgRes.RawStruct)
		if ok && raw.TargetGroupArn != nil && arnSet[*raw.TargetGroupArn] {
			ids = append(ids, tgRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "tg", Count: -1}
	}
	return relatedResult("tg", ids)
}

// checkASGSubnets parses VPCZoneIdentifier (comma-separated subnet IDs) from the ASG.
// Pattern F — no cache needed.
func checkASGSubnets(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	asg, ok := assertStruct[asgtypes.AutoScalingGroup](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "subnet", Count: -1}
	}
	if asg.VPCZoneIdentifier == nil || *asg.VPCZoneIdentifier == "" {
		return resource.RelatedCheckResult{TargetType: "subnet", Count: 0}
	}
	parts := strings.Split(*asg.VPCZoneIdentifier, ",")
	var ids []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			ids = append(ids, p)
		}
	}
	if len(ids) == 0 {
		return resource.RelatedCheckResult{TargetType: "subnet", Count: 0}
	}
	return relatedResult("subnet", ids)
}

// checkASGAlarm searches the alarm cache for alarms with an "AutoScalingGroupName" dimension
// matching this ASG's name.
// Pattern D — dimension-based lookup.
func checkASGAlarm(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	asgName := res.ID
	if asgName == "" {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: 0}
	}

	alarmList, truncated, err := asgRelatedResources(ctx, clients, cache, "alarm")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1, Err: err}
	}
	if alarmList == nil {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1}
	}

	var ids []string
	for _, alarmRes := range alarmList {
		alarm, ok := assertStruct[cwtypes.MetricAlarm](alarmRes.RawStruct)
		if !ok {
			continue
		}
		for _, d := range alarm.Dimensions {
			if d.Name != nil && *d.Name == "AutoScalingGroupName" && d.Value != nil && *d.Value == asgName {
				ids = append(ids, alarmRes.ID)
				break
			}
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1}
	}
	return relatedResult("alarm", ids)
}

// checkASGNG searches the node group cache for EKS node groups whose AutoScalingGroups
// include this ASG by name.
// Pattern C — reverse cache lookup.
func checkASGNG(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	asgName := res.ID
	if asgName == "" {
		return resource.RelatedCheckResult{TargetType: "ng", Count: 0}
	}

	ngList, truncated, err := asgRelatedResources(ctx, clients, cache, "ng")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "ng", Count: -1, Err: err}
	}
	if ngList == nil {
		return resource.RelatedCheckResult{TargetType: "ng", Count: -1}
	}

	var ids []string
	for _, ngRes := range ngList {
		ng, ok := assertStruct[ekstypes.Nodegroup](ngRes.RawStruct)
		if !ok {
			continue
		}
		if ng.Resources == nil {
			continue
		}
		for _, asgItem := range ng.Resources.AutoScalingGroups {
			if asgItem.Name != nil && *asgItem.Name == asgName {
				ids = append(ids, ngRes.ID)
				break
			}
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "ng", Count: -1}
	}
	return relatedResult("ng", ids)
}

// checkASGSG returns Count: 0 because AutoScalingGroup list response does not
// include security group IDs directly — they are defined on the launch
// template/configuration and are not surfaced in the DescribeAutoScalingGroups
// response payload.
func checkASGSG(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "sg", Count: 0}
}

// asgRelatedResources returns the resource list for target from cache or fetches it.
func asgRelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}

// checkASGVPC — ASG has no direct VPC field; VPCZoneIdentifier is CSV subnet IDs.
// Resolving VPC requires subnet cache lookup. Stub for now.
func checkASGVPC(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "vpc", Count: 0}
}

// checkASGRole returns Count: 0 because the IAM role is on the launch template
// or launch configuration, not on the ASG struct itself — the relationship
// cannot be determined from the ASG list response alone.
func checkASGRole(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "role", Count: 0}
}
