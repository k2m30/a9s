// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// tg_related.go contains Target Group related-resource checker functions.
package aws

import (
	"context"
	"slices"
	"strings"

	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// tgARN returns the Target Group ARN from Fields or RawStruct.
func tgARN(res resource.Resource) string {
	if arn := res.Fields["target_group_arn"]; arn != "" {
		return arn
	}
	raw, ok := assertStruct[elbv2types.TargetGroup](res.RawStruct)
	if ok && raw.TargetGroupArn != nil {
		return *raw.TargetGroupArn
	}
	return ""
}

// checkTGELB extracts LoadBalancerArns from the TG's RawStruct directly (Pattern F).
func checkTGELB(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[elbv2types.TargetGroup](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "elb", Count: 0}
	}
	if len(raw.LoadBalancerArns) == 0 {
		return resource.RelatedCheckResult{TargetType: "elb", Count: 0}
	}

	elbList, truncated, err := relatedResourcesFor(ctx, clients, cache, "elb")
	if err != nil {
		return resource.ErrorRelated("elb", err)
	}
	if elbList == nil {
		// No ELB cache available — fall back to count from ARN slice.
		return resource.RelatedCheckResult{TargetType: "elb", Count: len(raw.LoadBalancerArns)}
	}

	// Build a set of ARNs from the TG's LoadBalancerArns.
	arnSet := make(map[string]struct{}, len(raw.LoadBalancerArns))
	for _, arn := range raw.LoadBalancerArns {
		arnSet[arn] = struct{}{}
	}

	var ids []string
	for _, elbRes := range elbList {
		elbARN := elbRes.Fields["load_balancer_arn"]
		if elbARN == "" {
			lb, ok2 := assertStruct[elbv2types.LoadBalancer](elbRes.RawStruct)
			if ok2 && lb.LoadBalancerArn != nil {
				elbARN = *lb.LoadBalancerArn
			}
		}
		if _, matched := arnSet[elbARN]; matched {
			ids = append(ids, elbRes.ID)
		}
	}
	return relatedResultTrunc("elb", ids, truncated)
}

// checkTGECSSvc searches the ECS service cache for services whose LoadBalancers
// include a TargetGroupArn matching this TG (Pattern C).
func checkTGECSSvc(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	tgArn := tgARN(res)
	if tgArn == "" {
		return resource.RelatedCheckResult{TargetType: "ecs-svc", Count: 0}
	}

	svcList, truncated, err := relatedResourcesFor(ctx, clients, cache, "ecs-svc")
	if err != nil {
		return resource.ErrorRelated("ecs-svc", err)
	}
	if svcList == nil {
		return resource.UnknownRelated("ecs-svc")
	}

	var ids []string
	for _, svcRes := range svcList {
		svc, ok := assertStruct[ecstypes.Service](svcRes.RawStruct)
		if !ok {
			continue
		}
		for _, lb := range svc.LoadBalancers {
			if lb.TargetGroupArn != nil && *lb.TargetGroupArn == tgArn {
				ids = append(ids, svcRes.ID)
				break
			}
		}
	}
	return relatedResultTrunc("ecs-svc", ids, truncated)
}

// checkTGASG searches the ASG cache for auto scaling groups whose TargetGroupARNs
// include this TG's ARN (Pattern C).
func checkTGASG(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	tgArn := tgARN(res)
	if tgArn == "" {
		return resource.RelatedCheckResult{TargetType: "asg", Count: 0}
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
		asg, ok := assertStruct[asgtypes.AutoScalingGroup](asgRes.RawStruct)
		if !ok {
			continue
		}
		if slices.Contains(asg.TargetGroupARNs, tgArn) {
			ids = append(ids, asgRes.ID)
		}
	}
	return relatedResultTrunc("asg", ids, truncated)
}

// checkTGAlarm searches the alarm cache for CloudWatch alarms targeting this
// target group via the TargetGroup dimension.
func checkTGAlarm(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	tgARNVal := tgARN(res)
	if tgARNVal == "" {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: 0}
	}

	alarmList, truncated, err := relatedResourcesFor(ctx, clients, cache, "alarm")
	if err != nil {
		return resource.ErrorRelated("alarm", err)
	}
	if alarmList == nil {
		return resource.UnknownRelated("alarm")
	}

	// Extract the TG suffix for dimension matching: "targetgroup/name/hash"
	tgSuffix := tgARNVal
	if idx := strings.Index(tgARNVal, "targetgroup/"); idx >= 0 {
		tgSuffix = tgARNVal[idx:]
	}

	var ids []string
	for _, alarmRes := range alarmList {
		alarm, ok := assertStruct[cwtypes.MetricAlarm](alarmRes.RawStruct)
		if !ok {
			continue
		}
		for _, d := range alarm.Dimensions {
			if d.Name != nil && *d.Name == "TargetGroup" && d.Value != nil {
				if strings.Contains(*d.Value, tgSuffix) || strings.Contains(tgARNVal, *d.Value) {
					ids = append(ids, alarmRes.ID)
					break
				}
			}
		}
	}
	return relatedResultTrunc("alarm", ids, truncated)
}

// checkTGVPC returns the VPC this target group is scoped to (Pattern F).
// Reads vpc_id from Fields which is populated by the target groups fetcher.
func checkTGVPC(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	vpcID := res.Fields["vpc_id"]
	if vpcID == "" {
		return resource.RelatedCheckResult{TargetType: "vpc", Count: 0}
	}
	return relatedResult("vpc", []string{vpcID})
}

// checkTGCFN reports the CloudFormation stack owning this TG via
// aws:cloudformation:stack-name tag. Pattern C: one elbv2:DescribeTags call
// keyed by the TargetGroup ARN.
func checkTGCFN(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	arn := tgARN(res)
	if arn == "" {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: 0}
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.ELBv2 == nil {
		return resource.UnknownRelated("cfn")
	}
	api, ok := c.ELBv2.(ELBv2DescribeTagsAPI)
	if !ok {
		return resource.UnknownRelated("cfn")
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*elbv2.DescribeTagsOutput, error) {
		return api.DescribeTags(ctx, &elbv2.DescribeTagsInput{ResourceArns: []string{arn}})
	})
	if err != nil {
		return resource.ErrorRelated("cfn", err)
	}
	for _, td := range out.TagDescriptions {
		for _, tag := range td.Tags {
			if tag.Key != nil && *tag.Key == "aws:cloudformation:stack-name" && tag.Value != nil && *tag.Value != "" {
				return relatedResult("cfn", []string{*tag.Value})
			}
		}
	}
	return resource.RelatedCheckResult{TargetType: "cfn", Count: 0}
}

// checkTGEC2 reports EC2 instances registered as targets of this TG.
// Pattern C: one elbv2:DescribeTargetHealth call; filter targets whose ID
// starts with "i-" (EC2 instance IDs).
func checkTGEC2(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	tgArn := tgARN(res)
	if tgArn == "" {
		return resource.RelatedCheckResult{TargetType: "ec2", Count: 0}
	}
	// Skip the API call if the TG is a lambda/IP-only TG; EC2 targets only
	// apply to target_type=instance.
	raw, ok := assertStruct[elbv2types.TargetGroup](res.RawStruct)
	if ok && raw.TargetType != "" && raw.TargetType != elbv2types.TargetTypeEnumInstance {
		return resource.RelatedCheckResult{TargetType: "ec2", Count: 0}
	}
	c, cok := clients.(*ServiceClients)
	if !cok || c == nil || c.ELBv2 == nil {
		return resource.UnknownRelated("ec2")
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*elbv2.DescribeTargetHealthOutput, error) {
		return c.ELBv2.DescribeTargetHealth(ctx, &elbv2.DescribeTargetHealthInput{TargetGroupArn: &tgArn})
	})
	if err != nil {
		return resource.ErrorRelated("ec2", err)
	}
	seen := make(map[string]bool)
	var ids []string
	for _, t := range out.TargetHealthDescriptions {
		if t.Target == nil || t.Target.Id == nil {
			continue
		}
		id := *t.Target.Id
		if !strings.HasPrefix(id, "i-") || seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	return relatedResult("ec2", ids)
}

// checkTGLambda reports Lambda functions registered as targets (lambda-type TG).
// Pattern C: one elbv2:DescribeTargetHealth call; targets are Lambda invoke
// ARNs — extract the function name.
func checkTGLambda(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[elbv2types.TargetGroup](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("lambda")
	}
	if raw.TargetType != elbv2types.TargetTypeEnumLambda {
		return resource.RelatedCheckResult{TargetType: "lambda", Count: 0}
	}
	tgArn := tgARN(res)
	if tgArn == "" {
		return resource.RelatedCheckResult{TargetType: "lambda", Count: 0}
	}
	c, cok := clients.(*ServiceClients)
	if !cok || c == nil || c.ELBv2 == nil {
		return resource.UnknownRelated("lambda")
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*elbv2.DescribeTargetHealthOutput, error) {
		return c.ELBv2.DescribeTargetHealth(ctx, &elbv2.DescribeTargetHealthInput{TargetGroupArn: &tgArn})
	})
	if err != nil {
		return resource.ErrorRelated("lambda", err)
	}
	seen := make(map[string]bool)
	var ids []string
	for _, t := range out.TargetHealthDescriptions {
		if t.Target == nil || t.Target.Id == nil {
			continue
		}
		arn := *t.Target.Id
		// Lambda invoke ARN: arn:aws:lambda:REGION:ACCT:function:NAME[:VERSION]
		if !strings.Contains(arn, ":function:") {
			continue
		}
		idx := strings.LastIndex(arn, ":function:")
		rest := arn[idx+len(":function:"):]
		if colon := strings.Index(rest, ":"); colon >= 0 {
			rest = rest[:colon]
		}
		if rest != "" && !seen[rest] {
			seen[rest] = true
			ids = append(ids, rest)
		}
	}
	return relatedResult("lambda", ids)
}
