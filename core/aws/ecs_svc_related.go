// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// ecs_svc_related.go contains ECS service related-resource checker functions.
package aws

import (
	"context"
	"errors"

	"github.com/aws/aws-sdk-go-v2/aws"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkECSSvcCluster returns the ECS cluster this service belongs to (Pattern F).
// Extracts the cluster name from the Fields["cluster"] key populated by the fetcher.
func checkECSSvcCluster(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	clusterName := res.Fields["cluster"]
	if clusterName == "" {
		return keyMissing("ecs", "clusterName")
	}
	return relatedResultTrunc("ecs", []string{clusterName}, false)
}

// checkECSSvcTargetGroups returns the target groups attached to this ECS service (Pattern F).
// It reads the TargetGroupArn of each LoadBalancers entry through the tg resolver.
func checkECSSvcTargetGroups(_ context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ecstypes.Service](res.RawStruct)
	if !ok {
		return NotRead("tg")
	}
	if len(raw.LoadBalancers) == 0 {
		return foundNone("tg", "raw.LoadBalancers")
	}

	var arns []string
	for _, lb := range raw.LoadBalancers {
		arns = append(arns, aws.ToString(lb.TargetGroupArn))
	}
	return relatedRefs("tg", arns, refContext(clients, cache, "tg"))
}

// checkECSSvcAlarms reports the CloudWatch alarms on this service. A service
// name is unique within its cluster only, so an alarm that names a cluster
// must name this service's.
func checkECSSvcAlarms(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return alarmIDsByDimension(ctx, clients, cache, "ecs-svc", res)
}

// checkECSSvcCFN checks the ECS service's tags for aws:cloudformation:stack-name and finds the
// matching CFN stack in cache (Pattern C).
func checkECSSvcCFN(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	stackName := ""
	raw, ok := assertStruct[ecstypes.Service](res.RawStruct)
	if ok {
		for _, tag := range raw.Tags {
			if tag.Key != nil && *tag.Key == "aws:cloudformation:stack-name" && tag.Value != nil {
				stackName = *tag.Value
				break
			}
		}
	}
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
		rawCFN, cfnOk := assertStruct[cfntypes.Stack](cfnRes.RawStruct)
		if cfnOk && rawCFN.StackName != nil && *rawCFN.StackName == stackName {
			ids = append(ids, cfnRes.ID)
		}
	}
	return unreadZeroScanned(res, len(cfnList), relatedResultTrunc("cfn", ids, truncated))
}

// checkECSSvcELB finds the load balancers attached to this ECS service via a two-hop
// cache lookup (Pattern F+C):
// 1. Read TargetGroupArns from the ecstypes.Service LoadBalancers slice.
// 2. Scan the TG cache for those target groups.
// 3. From each matched TG, read LoadBalancerArns via elbv2types.TargetGroup.
// 4. Match those ARNs against the ELB cache.
func checkECSSvcELB(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ecstypes.Service](res.RawStruct)
	if !ok {
		return NotRead("elb")
	}
	if len(raw.LoadBalancers) == 0 {
		return foundNone("elb", "raw.LoadBalancers")
	}

	tgARNs := make(map[string]struct{})
	for _, lb := range raw.LoadBalancers {
		if lb.TargetGroupArn != nil && *lb.TargetGroupArn != "" {
			tgARNs[*lb.TargetGroupArn] = struct{}{}
		}
	}
	if len(tgARNs) == 0 {
		return foundNone("elb", "tgARNs")
	}

	tgList, truncatedTG, err := relatedResourcesFor(ctx, clients, cache, "tg")
	if err != nil {
		return ReadFailed("elb", err)
	}
	if tgList == nil {
		return NotRead("elb")
	}

	elbARNs := make(map[string]struct{})
	for _, tgRes := range tgList {
		tg, tgOk := assertStruct[elbv2types.TargetGroup](tgRes.RawStruct)
		if !tgOk {
			continue
		}
		if tg.TargetGroupArn == nil {
			continue
		}
		if _, matched := tgARNs[*tg.TargetGroupArn]; !matched {
			continue
		}
		for _, elbARN := range tg.LoadBalancerArns {
			if elbARN != "" {
				elbARNs[elbARN] = struct{}{}
			}
		}
	}
	if len(elbARNs) == 0 {
		if truncatedTG {
			return NotRead("elb")
		}
		return foundNone("elb", "elbARNs")
	}

	// LoadBalancerArn — not the LB name (res.ID) — is the join key:
	// elbTargetGroup.LoadBalancerArns[] carries full ARNs, so matching against
	// elbRes.ID (the LB name) never hits. The elb fetcher populates
	// Fields["load_balancer_arn"]; RawStruct is the fallback for cache-restored
	// rows without the field.
	elbList, truncatedELB, err := relatedResourcesFor(ctx, clients, cache, "elb")
	if err != nil {
		return ReadFailed("elb", err)
	}
	if elbList == nil {
		return NotRead("elb")
	}

	var ids []string
	for _, elbRes := range elbList {
		arn := elbRes.Fields["load_balancer_arn"]
		if arn == "" {
			if raw, ok := assertStruct[elbv2types.LoadBalancer](elbRes.RawStruct); ok && raw.LoadBalancerArn != nil {
				arn = *raw.LoadBalancerArn
			}
		}
		if arn == "" {
			continue
		}
		if _, found := elbARNs[arn]; found {
			ids = append(ids, elbRes.ID)
		}
	}
	return relatedResultTrunc("elb", ids, truncatedELB || truncatedTG)
}

// checkECSSvcLogs reports the log groups the containers of the service's
// task definition write to.
func checkECSSvcLogs(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ecstypes.Service](res.RawStruct)
	if !ok {
		return NotRead("logs")
	}
	taskDefARN := aws.ToString(raw.TaskDefinition)
	if taskDefARN == "" {
		return foundNone("logs", "raw.TaskDefinition")
	}
	return ecsTaskDefLogGroups(ctx, clients, cache, taskDefARN)
}

// checkECSSvcSG extracts security group IDs from the ECS Service's
// NetworkConfiguration.AwsvpcConfiguration.SecurityGroups slice (awsvpc mode).
// Pattern F — no cache needed.
func checkECSSvcSG(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ecstypes.Service](res.RawStruct)
	if !ok {
		return NotRead("sg")
	}
	if raw.NetworkConfiguration == nil ||
		raw.NetworkConfiguration.AwsvpcConfiguration == nil {
		return foundNone("sg", "NetworkConfiguration.AwsvpcConfiguration")
	}
	var ids []string
	for _, sgID := range raw.NetworkConfiguration.AwsvpcConfiguration.SecurityGroups {
		if sgID != "" {
			ids = append(ids, sgID)
		}
	}
	return relatedResultTrunc("sg", ids, false)
}

// checkECSSvcRole counts the service's roles: Service.RoleArn, and its task
// definition's taskRoleArn (what the containers call AWS as) and
// executionRoleArn (what the container agent calls AWS as)
// (https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_TaskDefinition.html).
func checkECSSvcRole(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ecstypes.Service](res.RawStruct)
	if !ok {
		return NotRead("role")
	}
	refs := []string{aws.ToString(raw.RoleArn)}
	var def relatedRead
	if td := aws.ToString(raw.TaskDefinition); td != "" {
		switch taskDef, err := ecsTaskDefinition(ctx, clients, td); {
		case errors.Is(err, errClientMissing):
			return NotRead("role")
		case err != nil || taskDef == nil:
			def = unreadBy(err)
		default:
			refs = append(refs, aws.ToString(taskDef.TaskRoleArn), aws.ToString(taskDef.ExecutionRoleArn))
		}
	}
	return relatedAnswer("role", joinReads(def, readOf(relatedRefs("role", refs, refContext(clients, cache, "role")))))
}
