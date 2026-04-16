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

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterRelated("tg", []resource.RelatedDef{
		{TargetType: "elb", DisplayName: "Load Balancers", Checker: checkTGELB, NeedsTargetCache: false},
		{TargetType: "ecs-svc", DisplayName: "ECS Services", Checker: checkTGECSSvc, NeedsTargetCache: true},
		{TargetType: "asg", DisplayName: "Auto Scaling Groups", Checker: checkTGASG, NeedsTargetCache: true},
		{TargetType: "alarm", DisplayName: "CW Alarms", Checker: checkTGAlarm, NeedsTargetCache: true},
		{TargetType: "vpc", DisplayName: "VPC", Checker: checkTGVPC},
		{TargetType: "backup", DisplayName: "Backup Plans", Checker: checkTGBackup},
		{TargetType: "cfn", DisplayName: "CloudFormation", Checker: checkTGCFN},
		{TargetType: "dbc", DisplayName: "DocumentDB Clusters", Checker: checkTGDBC},
		{TargetType: "dbi", DisplayName: "RDS Instances", Checker: checkTGDBI},
		{TargetType: "ec2", DisplayName: "EC2 Instances", Checker: checkTGEC2},
		{TargetType: "kms", DisplayName: "KMS Keys", Checker: checkTGKMS},
		{TargetType: "lambda", DisplayName: "Lambda Functions", Checker: checkTGLambda},
		{TargetType: "logs", DisplayName: "Log Groups", Checker: checkTGLogs},
		{TargetType: "rds-snap", DisplayName: "RDS Snapshots", Checker: checkTGRDSSnap},
		{TargetType: "role", DisplayName: "IAM Roles", Checker: checkTGRole},
		{TargetType: "secrets", DisplayName: "Secrets Manager", Checker: checkTGSecrets},
		{TargetType: "sg", DisplayName: "Security Groups", Checker: checkTGSG},
		{TargetType: "subnet", DisplayName: "Subnets", Checker: checkTGSubnet},
	})

	resource.RegisterNavigableFields("tg", []resource.NavigableField{
		{FieldPath: "VpcId", TargetType: "vpc"},
		{FieldPath: "LoadBalancerArns", TargetType: "elb"},
	})
}

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

	elbList, truncated, err := tgRelatedResources(ctx, clients, cache, "elb")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "elb", Count: -1, Err: err}
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
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "elb", Count: -1}
	}
	return relatedResult("elb", ids)
}

// checkTGECSSvc searches the ECS service cache for services whose LoadBalancers
// include a TargetGroupArn matching this TG (Pattern C).
func checkTGECSSvc(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	tgArn := tgARN(res)
	if tgArn == "" {
		return resource.RelatedCheckResult{TargetType: "ecs-svc", Count: 0}
	}

	svcList, truncated, err := tgRelatedResources(ctx, clients, cache, "ecs-svc")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "ecs-svc", Count: -1, Err: err}
	}
	if svcList == nil {
		return resource.RelatedCheckResult{TargetType: "ecs-svc", Count: -1}
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
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "ecs-svc", Count: -1}
	}
	return relatedResult("ecs-svc", ids)
}

// checkTGASG searches the ASG cache for auto scaling groups whose TargetGroupARNs
// include this TG's ARN (Pattern C).
func checkTGASG(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	tgArn := tgARN(res)
	if tgArn == "" {
		return resource.RelatedCheckResult{TargetType: "asg", Count: 0}
	}

	asgList, truncated, err := tgRelatedResources(ctx, clients, cache, "asg")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "asg", Count: -1, Err: err}
	}
	if asgList == nil {
		return resource.RelatedCheckResult{TargetType: "asg", Count: -1}
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
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "asg", Count: -1}
	}
	return relatedResult("asg", ids)
}

// checkTGAlarm searches the alarm cache for CloudWatch alarms targeting this
// target group via the TargetGroup dimension.
func checkTGAlarm(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	tgARNVal := tgARN(res)
	if tgARNVal == "" {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: 0}
	}

	alarmList, truncated, err := tgRelatedResources(ctx, clients, cache, "alarm")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1, Err: err}
	}
	if alarmList == nil {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1}
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
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1}
	}
	return relatedResult("alarm", ids)
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


// checkTGBackup reports backup plans that cover this target group.
// TGs are not a protectable AWS Backup resource directly — the DevOps link is
// via backup plans on the targets' instances/dbs. Not determinable from
// caches alone. Returns Count: -1.
func checkTGBackup(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	if tgARN(res) == "" {
		return resource.RelatedCheckResult{TargetType: "backup", Count: 0}
	}
	return resource.RelatedCheckResult{TargetType: "backup", Count: -1}
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
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1}
	}
	api, ok := c.ELBv2.(ELBv2DescribeTagsAPI)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1}
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*elbv2.DescribeTagsOutput, error) {
		return api.DescribeTags(ctx, &elbv2.DescribeTagsInput{ResourceArns: []string{arn}})
	})
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1, Err: err}
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

// checkTGDBC reports DocumentDB clusters targeted by this TG. DB clusters are
// not directly registered as TG targets in AWS ELBv2; target identity requires
// DescribeTargetHealth per TG and matching IP addresses against DocDB ENIs —
// outside the 1-call budget. Returns Count: -1.
func checkTGDBC(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	if tgARN(res) == "" {
		return resource.RelatedCheckResult{TargetType: "dbc", Count: 0}
	}
	return resource.RelatedCheckResult{TargetType: "dbc", Count: -1}
}

// checkTGDBI reports RDS instances targeted by this TG. Same limitation as dbc:
// DescribeTargetHealth required. Returns Count: -1.
func checkTGDBI(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	if tgARN(res) == "" {
		return resource.RelatedCheckResult{TargetType: "dbi", Count: 0}
	}
	return resource.RelatedCheckResult{TargetType: "dbi", Count: -1}
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
		return resource.RelatedCheckResult{TargetType: "ec2", Count: -1}
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*elbv2.DescribeTargetHealthOutput, error) {
		return c.ELBv2.DescribeTargetHealth(ctx, &elbv2.DescribeTargetHealthInput{TargetGroupArn: &tgArn})
	})
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "ec2", Count: -1, Err: err}
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

// checkTGKMS reports KMS keys encrypting resources behind this TG.
// TargetGroup has no KMS reference in DescribeTargetGroups. Returns Count: 0
// (real: no direct AWS API field ties a TG to a KMS key).
func checkTGKMS(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
}

// checkTGLambda reports Lambda functions registered as targets (lambda-type TG).
// Pattern C: one elbv2:DescribeTargetHealth call; targets are Lambda invoke
// ARNs — extract the function name.
func checkTGLambda(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[elbv2types.TargetGroup](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "lambda", Count: -1}
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
		return resource.RelatedCheckResult{TargetType: "lambda", Count: -1}
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*elbv2.DescribeTargetHealthOutput, error) {
		return c.ELBv2.DescribeTargetHealth(ctx, &elbv2.DescribeTargetHealthInput{TargetGroupArn: &tgArn})
	})
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "lambda", Count: -1, Err: err}
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

// checkTGLogs reports CloudWatch log groups related to this TG.
// Target groups themselves do not emit logs; the relevant logs are on the
// parent ELB (access logs). Not directly determinable from TG fields alone.
// Returns Count: -1 when the TG has an ARN.
func checkTGLogs(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	if tgARN(res) == "" {
		return resource.RelatedCheckResult{TargetType: "logs", Count: 0}
	}
	return resource.RelatedCheckResult{TargetType: "logs", Count: -1}
}

// checkTGRDSSnap reports RDS snapshots related to RDS instances targeted by
// this TG. Requires two hops (target instance → snapshots) and per-TG target
// enumeration. Returns Count: -1.
func checkTGRDSSnap(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	if tgARN(res) == "" {
		return resource.RelatedCheckResult{TargetType: "rds-snap", Count: 0}
	}
	return resource.RelatedCheckResult{TargetType: "rds-snap", Count: -1}
}

// checkTGRole reports IAM roles associated with this TG. TGs do not carry
// IAM role fields directly in DescribeTargetGroups. Returns Count: 0.
func checkTGRole(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "role", Count: 0}
}

// checkTGSecrets reports Secrets Manager secrets used by targets of this TG.
// No direct TG→Secrets field exists in DescribeTargetGroups. Returns 0.
func checkTGSecrets(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "secrets", Count: 0}
}

// checkTGSG reports security groups of the TG's targets. DescribeTargetGroups
// does not carry SG references directly (they are on the ENIs of targets).
// Returns Count: -1 when TG has VPC scope.
func checkTGSG(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	if res.Fields["vpc_id"] == "" {
		return resource.RelatedCheckResult{TargetType: "sg", Count: 0}
	}
	return resource.RelatedCheckResult{TargetType: "sg", Count: -1}
}

// checkTGSubnet reports the subnets a TG's targets reside in. Target
// networking requires DescribeTargetHealth + ENI lookup — outside budget.
// Returns Count: -1 when TG has a VPC; 0 otherwise.
func checkTGSubnet(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	if res.Fields["vpc_id"] == "" {
		return resource.RelatedCheckResult{TargetType: "subnet", Count: 0}
	}
	return resource.RelatedCheckResult{TargetType: "subnet", Count: -1}
}

// tgRelatedResources returns the resource list for target from cache or by
// fetching the first page via the registered paginated fetcher.
func tgRelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}










