package aws

import (
	"context"
	"strings"

	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkASGEC2 reads Instances[] from the ASG RawStruct and returns their IDs.
// Pattern F — no cache needed.
func checkASGEC2(_ context.Context, _ interface{}, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
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
func checkASGTG(ctx context.Context, clients interface{}, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
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
func checkASGSubnets(_ context.Context, _ interface{}, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
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

// asgRelatedResources returns the resource list for target from cache or fetches it.
func asgRelatedResources(ctx context.Context, clients interface{}, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}
