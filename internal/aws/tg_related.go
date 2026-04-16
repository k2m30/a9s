// tg_related.go contains Target Group related-resource checker functions.
package aws

import (
	"context"
	"slices"
	"strings"

	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterRelated("tg", []resource.RelatedDef{
		{TargetType: "elb", DisplayName: "Load Balancers", Checker: checkTGELB, NeedsTargetCache: false},
		{TargetType: "ecs-svc", DisplayName: "ECS Services", Checker: checkTGECSSvc, NeedsTargetCache: true},
		{TargetType: "asg", DisplayName: "Auto Scaling Groups", Checker: checkTGASG, NeedsTargetCache: true},
		{TargetType: "alarm", DisplayName: "CW Alarms", Checker: checkTGAlarm, NeedsTargetCache: true},
		{TargetType: "sg", DisplayName: "Security Groups", Checker: checkTGSG},
		{TargetType: "vpc", DisplayName: "VPC", Checker: checkTGVPC},
		{TargetType: "role", DisplayName: "IAM Role", Checker: checkTGRole},
		{TargetType: "kms", DisplayName: "KMS Key", Checker: checkTGKMS},
		{TargetType: "backup", DisplayName: "AWS Backups", Checker: checkTGBackup},
		{TargetType: "cfn", DisplayName: "CloudFormation Stacks", Checker: checkTGCFN},
		{TargetType: "dbc", DisplayName: "DB Clusters", Checker: checkTGDBC},
		{TargetType: "dbi", DisplayName: "DB Instances", Checker: checkTGDBI},
		{TargetType: "ec2", DisplayName: "EC2 Instances", Checker: checkTGEC2},
		{TargetType: "lambda", DisplayName: "Lambda Functions", Checker: checkTGLambda},
		{TargetType: "logs", DisplayName: "Log Groups", Checker: checkTGLogs},
		{TargetType: "rds-snap", DisplayName: "RDS Snapshots", Checker: checkTGRDSSnap},
		{TargetType: "secrets", DisplayName: "Secrets", Checker: checkTGSecrets},
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

// checkTGSG returns Count: 0 because Target Groups do not have security groups
// — they are associated with load balancers and targets, not security groups directly.
func checkTGSG(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "sg", Count: 0}
}

// checkTGRole returns Count: 0 because Target Groups do not have a directly
// associated IAM role — role associations are on the ECS services or tasks, not on the TG.
func checkTGRole(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "role", Count: 0}
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

// checkTGKMS is a stub. Target Groups (ALB/NLB) do not have a KMS key
// association — encryption at rest for load balancer logs is managed at
// the S3 bucket level, not on the target group resource itself.
func checkTGKMS(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
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

func checkTGBackup(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "backup", Count: 0}
}

func checkTGCFN(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "cfn", Count: 0}
}

func checkTGDBC(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "dbc", Count: 0}
}

func checkTGDBI(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "dbi", Count: 0}
}

func checkTGEC2(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "ec2", Count: 0}
}

func checkTGLambda(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "lambda", Count: 0}
}

func checkTGLogs(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "logs", Count: 0}
}

func checkTGRDSSnap(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "rds-snap", Count: 0}
}

func checkTGSecrets(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "secrets", Count: 0}
}

func checkTGSubnet(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "subnet", Count: 0}
}
