// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// ecs_svc_related.go contains ECS service related-resource checker functions.
package aws

import (
	"context"
	"strings"

	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkECSSvcCluster returns the ECS cluster this service belongs to (Pattern F).
// Extracts the cluster name from the Fields["cluster"] key populated by the fetcher.
func checkECSSvcCluster(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	clusterName := res.Fields["cluster"]
	if clusterName == "" {
		return resource.RelatedCheckResult{TargetType: "ecs", Count: 0}
	}
	return relatedResult("ecs", []string{clusterName})
}

// checkECSSvcTargetGroups returns the target groups attached to this ECS service (Pattern F).
// It reads LoadBalancers from the raw ecstypes.Service struct and parses TG names from ARNs.
func checkECSSvcTargetGroups(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ecstypes.Service](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("tg")
	}
	if len(raw.LoadBalancers) == 0 {
		return resource.RelatedCheckResult{TargetType: "tg", Count: 0}
	}

	var ids []string
	for _, lb := range raw.LoadBalancers {
		if lb.TargetGroupArn == nil || *lb.TargetGroupArn == "" {
			continue
		}
		// TG ARN format: arn:aws:elasticloadbalancing:region:account:targetgroup/name/hash
		// Extract the name as the second segment after splitting by "/"
		parts := strings.Split(*lb.TargetGroupArn, "/")
		if len(parts) >= 2 {
			name := parts[len(parts)-2]
			if name != "" {
				ids = append(ids, name)
			}
		}
	}
	return relatedResult("tg", ids)
}

// checkECSSvcAlarms searches the alarm cache for alarms with both ServiceName and ClusterName
// dimensions matching this ECS service (Pattern C).
func checkECSSvcAlarms(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	serviceName := res.ID
	clusterName := res.Fields["cluster"]
	if serviceName == "" {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: 0}
	}

	alarmList, truncated, err := relatedResourcesFor(ctx, clients, cache, "alarm")
	if err != nil {
		return resource.ErrorRelated("alarm", err)
	}
	if alarmList == nil {
		return resource.UnknownRelated("alarm")
	}

	var ids []string
	for _, alarmRes := range alarmList {
		rawAlarm, ok := assertStruct[cwtypes.MetricAlarm](alarmRes.RawStruct)
		if !ok {
			continue
		}
		hasServiceName := false
		hasClusterName := clusterName == ""
		for _, d := range rawAlarm.Dimensions {
			if d.Name == nil || d.Value == nil {
				continue
			}
			if *d.Name == "ServiceName" && *d.Value == serviceName {
				hasServiceName = true
			}
			if clusterName != "" && *d.Name == "ClusterName" && *d.Value == clusterName {
				hasClusterName = true
			}
		}
		if hasServiceName && hasClusterName {
			ids = append(ids, alarmRes.ID)
		}
	}
	return relatedResultTrunc("alarm", ids, truncated)
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
		return resource.RelatedCheckResult{TargetType: "cfn", Count: 0}
	}

	cfnList, truncated, err := relatedResourcesFor(ctx, clients, cache, "cfn")
	if err != nil {
		return resource.ErrorRelated("cfn", err)
	}
	if cfnList == nil {
		return resource.UnknownRelated("cfn")
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
	return relatedResultTrunc("cfn", ids, truncated)
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
		return resource.RelatedCheckResult{TargetType: "elb", Count: 0}
	}
	if len(raw.LoadBalancers) == 0 {
		return resource.RelatedCheckResult{TargetType: "elb", Count: 0}
	}

	// Collect TG ARNs from the service definition.
	tgARNs := make(map[string]struct{})
	for _, lb := range raw.LoadBalancers {
		if lb.TargetGroupArn != nil && *lb.TargetGroupArn != "" {
			tgARNs[*lb.TargetGroupArn] = struct{}{}
		}
	}
	if len(tgARNs) == 0 {
		return resource.RelatedCheckResult{TargetType: "elb", Count: 0}
	}

	// Step 2: scan TG cache for matching target groups.
	tgList, truncatedTG, err := relatedResourcesFor(ctx, clients, cache, "tg")
	if err != nil {
		return resource.ErrorRelated("elb", err)
	}
	if tgList == nil {
		return resource.RelatedCheckResult{TargetType: "elb", Count: 0}
	}

	// Step 3: collect ELB ARNs from matched TGs.
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
			return resource.UnknownRelated("elb")
		}
		return resource.RelatedCheckResult{TargetType: "elb", Count: 0}
	}

	// Step 4: match ELB ARNs against the ELB cache. LoadBalancerArn — not the
	// LB name (res.ID) — is the join key: elbTargetGroup.LoadBalancerArns[]
	// carries full ARNs, so matching against elbRes.ID (the LB name) never
	// hits. The elb fetcher populates Fields["load_balancer_arn"]; RawStruct
	// is the fallback for cache-restored rows that predate the field or lost
	// it to a stale replay.
	elbList, truncatedELB, err := relatedResourcesFor(ctx, clients, cache, "elb")
	if err != nil {
		return resource.ErrorRelated("elb", err)
	}
	if elbList == nil {
		return resource.UnknownRelated("elb")
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
	return relatedResultTrunc("elb", ids, truncatedELB)
}

// checkECSSvcLogs searches the logs cache for log groups matching the ECS service's
// task definition family name.
// Pattern N — convention: scan cache for log groups containing the task def family name.
func checkECSSvcLogs(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ecstypes.Service](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "logs", Count: 0}
	}
	taskDefARN := ""
	if raw.TaskDefinition != nil {
		taskDefARN = *raw.TaskDefinition
	}
	if taskDefARN == "" {
		return resource.RelatedCheckResult{TargetType: "logs", Count: 0}
	}
	// Extract task def family from ARN: arn:aws:ecs:region:account:task-definition/family:revision
	family := arnLastSegment(taskDefARN)
	// Remove revision suffix (e.g. "family:5" -> "family")
	if idx := strings.LastIndex(family, ":"); idx >= 0 {
		family = family[:idx]
	}
	if family == "" {
		return resource.RelatedCheckResult{TargetType: "logs", Count: 0}
	}

	logList, truncated, err := relatedResourcesFor(ctx, clients, cache, "logs")
	if err != nil {
		return resource.ErrorRelated("logs", err)
	}
	if logList == nil {
		return resource.UnknownRelated("logs")
	}

	var ids []string
	for _, logRes := range logList {
		if strings.Contains(logRes.ID, family) {
			ids = append(ids, logRes.ID)
		}
	}
	return relatedResultTrunc("logs", ids, truncated)
}

// checkECSSvcSG extracts security group IDs from the ECS Service's
// NetworkConfiguration.AwsvpcConfiguration.SecurityGroups slice (awsvpc mode).
// Pattern F — no cache needed.
func checkECSSvcSG(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ecstypes.Service](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("sg")
	}
	if raw.NetworkConfiguration == nil ||
		raw.NetworkConfiguration.AwsvpcConfiguration == nil {
		return resource.RelatedCheckResult{TargetType: "sg", Count: 0}
	}
	var ids []string
	for _, sgID := range raw.NetworkConfiguration.AwsvpcConfiguration.SecurityGroups {
		if sgID != "" {
			ids = append(ids, sgID)
		}
	}
	return relatedResult("sg", ids)
}

// checkECSSvcRole extracts the IAM role name from the ECS Service's RoleArn field.
// The RoleArn has the form arn:aws:iam::ACCOUNT:role/ROLE-NAME; the role name is
// the last segment after "/".
func checkECSSvcRole(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ecstypes.Service](res.RawStruct)
	if !ok || raw.RoleArn == nil || *raw.RoleArn == "" {
		return resource.RelatedCheckResult{TargetType: "role", Count: 0}
	}
	arn := *raw.RoleArn
	if idx := strings.LastIndex(arn, "/"); idx >= 0 && idx < len(arn)-1 {
		return relatedResult("role", []string{arn[idx+1:]})
	}
	return resource.RelatedCheckResult{TargetType: "role", Count: 0}
}

// ecsSvcRelatedResources returns the resource list for target from cache or by
// fetching the first page via the registered paginated fetcher.
func ecsSvcRelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	return relatedResourcesFor(ctx, clients, cache, target)
}
