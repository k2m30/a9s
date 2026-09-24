// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// tg_related.go contains Target Group related-resource checker functions.
package aws

import (
	"context"
	"slices"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
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

// tgsRegistering reads the target groups in tgs a target is registered in:
// one elbv2:DescribeTargetHealth per group, which lists every registered
// target (https://docs.aws.amazon.com/elasticloadbalancing/latest/APIReference/API_DescribeTargetHealth.html),
// kept to the ones names accepts. A group whose read failed leaves the answer
// short; none read leaves it unread.
func tgsRegistering(ctx context.Context, clients any, tgs []resource.Resource, op string, names func(targetID string) bool) relatedRead {
	if len(tgs) == 0 {
		return relatedRead{}
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.ELBv2 == nil {
		return relatedRead{unread: true}
	}
	tgs, capped := fanOut(tgs)
	var read relatedRead
	var failures []Failure
	for _, tg := range tgs {
		arn := tgARN(tg)
		out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*elbv2.DescribeTargetHealthOutput, error) {
			return c.ELBv2.DescribeTargetHealth(ctx, &elbv2.DescribeTargetHealthInput{TargetGroupArn: &arn})
		})
		if err != nil {
			failures = append(failures, FailedCall(tg.ID, err))
			continue
		}
		if slices.ContainsFunc(out.TargetHealthDescriptions, func(d elbv2types.TargetHealthDescription) bool {
			return d.Target != nil && names(aws.ToString(d.Target.Id))
		}) {
			read.ids = append(read.ids, tg.ID)
		}
	}
	read.partial = capped || len(failures) > 0
	read.unread = len(failures) == len(tgs)
	read.failure = AggregateFailures(op, failures, len(tgs))
	return read
}

// checkTGELB counts the load balancers the TG's LoadBalancerArns name, a
// closed set, as the elb list holds them (Pattern F).
func checkTGELB(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[elbv2types.TargetGroup](res.RawStruct)
	if !ok {
		return NotRead("elb")
	}
	return listedRelated(ctx, clients, cache, "elb", raw.LoadBalancerArns, false)
}

// checkTGECSSvc searches the ECS service cache for services whose LoadBalancers
// include a TargetGroupArn matching this TG (Pattern C).
func checkTGECSSvc(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	tgArn := tgARN(res)
	if tgArn == "" {
		return foundNone("ecs-svc", "tgArn")
	}

	svcList, truncated, err := relatedResourcesFor(ctx, clients, cache, "ecs-svc")
	if err != nil {
		return ReadFailed("ecs-svc", err)
	}
	if svcList == nil {
		return NotRead("ecs-svc")
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
		return foundNone("asg", "tgArn")
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

func checkTGAlarm(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return alarmIDsByDimension(ctx, clients, cache, "tg", res)
}

// checkTGVPC returns the VPC this target group is scoped to (Pattern F).
// Reads vpc_id from Fields which is populated by the target groups fetcher.
func checkTGVPC(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	vpcID := res.Fields["vpc_id"]
	if vpcID == "" {
		return foundNone("vpc", "vpcID")
	}
	return relatedResultTrunc("vpc", []string{vpcID}, false)
}

// checkTGCFN reports the CloudFormation stack owning this TG via
// aws:cloudformation:stack-name tag. Pattern C: one elbv2:DescribeTags call
// keyed by the TargetGroup ARN.
func checkTGCFN(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	arn := tgARN(res)
	if arn == "" {
		return foundNone("cfn", "arn")
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.ELBv2 == nil {
		return NotRead("cfn")
	}
	api, ok := c.ELBv2.(ELBv2DescribeTagsAPI)
	if !ok {
		return NotRead("cfn")
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*elbv2.DescribeTagsOutput, error) {
		return api.DescribeTags(ctx, &elbv2.DescribeTagsInput{ResourceArns: []string{arn}})
	})
	if err != nil {
		return ReadFailed("cfn", err)
	}
	for _, td := range out.TagDescriptions {
		for _, tag := range td.Tags {
			if tag.Key != nil && *tag.Key == "aws:cloudformation:stack-name" && tag.Value != nil && *tag.Value != "" {
				return relatedResultTrunc("cfn", []string{*tag.Value}, false)
			}
		}
	}
	return foundNone("cfn", "the aws:cloudformation:stack-name tag")
}

// checkTGEC2 reports EC2 instances registered as targets of this TG.
// Pattern C: one elbv2:DescribeTargetHealth call; filter targets whose ID
// starts with "i-" (EC2 instance IDs).
func checkTGEC2(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	tgArn := tgARN(res)
	if tgArn == "" {
		return foundNone("ec2", "tgArn")
	}
	// Skip the API call if the TG is a lambda/IP-only TG; EC2 targets only
	// apply to target_type=instance.
	raw, ok := assertStruct[elbv2types.TargetGroup](res.RawStruct)
	if ok && raw.TargetType != "" && raw.TargetType != elbv2types.TargetTypeEnumInstance {
		return foundNone("ec2", "raw.TargetType")
	}
	c, cok := clients.(*ServiceClients)
	if !cok || c == nil || c.ELBv2 == nil {
		return NotRead("ec2")
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*elbv2.DescribeTargetHealthOutput, error) {
		return c.ELBv2.DescribeTargetHealth(ctx, &elbv2.DescribeTargetHealthInput{TargetGroupArn: &tgArn})
	})
	if err != nil {
		return ReadFailed("ec2", err)
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
	return relatedResultTrunc("ec2", ids, false)
}

// checkTGLambda reports Lambda functions registered as targets (lambda-type TG).
// Pattern C: one elbv2:DescribeTargetHealth call; targets are Lambda invoke
// ARNs, read through the lambda resolver.
func checkTGLambda(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[elbv2types.TargetGroup](res.RawStruct)
	if !ok {
		return NotRead("lambda")
	}
	if raw.TargetType != elbv2types.TargetTypeEnumLambda {
		return foundNone("lambda", "raw.TargetType")
	}
	tgArn := tgARN(res)
	if tgArn == "" {
		return foundNone("lambda", "tgArn")
	}
	c, cok := clients.(*ServiceClients)
	if !cok || c == nil || c.ELBv2 == nil {
		return NotRead("lambda")
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*elbv2.DescribeTargetHealthOutput, error) {
		return c.ELBv2.DescribeTargetHealth(ctx, &elbv2.DescribeTargetHealthInput{TargetGroupArn: &tgArn})
	})
	if err != nil {
		return ReadFailed("lambda", err)
	}
	var arns []string
	for _, t := range out.TargetHealthDescriptions {
		if t.Target == nil || t.Target.Id == nil {
			continue
		}
		if _, isFunction := ARNForService(*t.Target.Id, "lambda"); isFunction {
			arns = append(arns, *t.Target.Id)
		}
	}
	return relatedRefs("lambda", arns, refContext(clients, cache, "lambda"))
}
