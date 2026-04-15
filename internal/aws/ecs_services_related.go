// ecs_services_related.go contains ECS service related-resource checker functions.
package aws

import (
	"context"
	"strings"

	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterRelated("ecs-svc", []resource.RelatedDef{
		{TargetType: "ecs", DisplayName: "ECS Clusters", Checker: checkECSSvcCluster},
		{TargetType: "tg", DisplayName: "Target Groups", Checker: checkECSSvcTargetGroups},
		{TargetType: "alarm", DisplayName: "CloudWatch Alarms", Checker: checkECSSvcAlarms, NeedsTargetCache: true},
		{TargetType: "cfn", DisplayName: "CloudFormation Stacks", Checker: checkECSSvcCFN, NeedsTargetCache: true},
		{TargetType: "elb", DisplayName: "Load Balancers", Checker: checkECSSvcELB, NeedsTargetCache: true},
		{TargetType: "logs", DisplayName: "Log Groups", Checker: checkECSSvcLogs, NeedsTargetCache: true},
	})

	// ecstypes.Service: ClusterArn, RoleArn, NetworkConfiguration subnets/SGs, LoadBalancer TG ARNs
	resource.RegisterNavigableFields("ecs-svc", []resource.NavigableField{
		{FieldPath: "ClusterArn", TargetType: "ecs"},
		{FieldPath: "RoleArn", TargetType: "role"},
		{FieldPath: "NetworkConfiguration.AwsvpcConfiguration.Subnets", TargetType: "subnet"},
		{FieldPath: "NetworkConfiguration.AwsvpcConfiguration.SecurityGroups", TargetType: "sg"},
		{FieldPath: "LoadBalancers.TargetGroupArn", TargetType: "tg"},
	})
}

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
		return resource.RelatedCheckResult{TargetType: "tg", Count: -1}
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

	alarmList, truncated, err := ecsSvcRelatedResources(ctx, clients, cache, "alarm")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1, Err: err}
	}
	if alarmList == nil {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1}
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
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1}
	}
	return relatedResult("alarm", ids)
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

	cfnList, truncated, err := ecsSvcRelatedResources(ctx, clients, cache, "cfn")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1, Err: err}
	}
	if cfnList == nil {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1}
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
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1}
	}
	return relatedResult("cfn", ids)
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
		return resource.RelatedCheckResult{TargetType: "elb", Count: -1}
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
	tgList, truncatedTG, err := ecsSvcRelatedResources(ctx, clients, cache, "tg")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "elb", Count: -1, Err: err}
	}
	if tgList == nil {
		return resource.RelatedCheckResult{TargetType: "elb", Count: -1}
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
			return resource.RelatedCheckResult{TargetType: "elb", Count: -1}
		}
		return resource.RelatedCheckResult{TargetType: "elb", Count: 0}
	}

	// Step 4: match ELB ARNs against the ELB cache.
	elbList, truncatedELB, err := ecsSvcRelatedResources(ctx, clients, cache, "elb")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "elb", Count: -1, Err: err}
	}
	if elbList == nil {
		return resource.RelatedCheckResult{TargetType: "elb", Count: -1}
	}

	var ids []string
	for _, elbRes := range elbList {
		if _, found := elbARNs[elbRes.ID]; found {
			ids = append(ids, elbRes.ID)
		}
	}
	if len(ids) == 0 && truncatedELB {
		return resource.RelatedCheckResult{TargetType: "elb", Count: -1}
	}
	return relatedResult("elb", ids)
}

// checkECSSvcLogs searches the logs cache for log groups matching the ECS service's
// task definition family name.
// Pattern N — convention: scan cache for log groups containing the task def family name.
func checkECSSvcLogs(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ecstypes.Service](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "logs", Count: -1}
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

	logList, truncated, err := ecsSvcRelatedResources(ctx, clients, cache, "logs")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "logs", Count: -1, Err: err}
	}
	if logList == nil {
		return resource.RelatedCheckResult{TargetType: "logs", Count: -1}
	}

	var ids []string
	for _, logRes := range logList {
		if strings.Contains(logRes.ID, family) {
			ids = append(ids, logRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "logs", Count: -1}
	}
	return relatedResult("logs", ids)
}

// ecsSvcRelatedResources returns the resource list for target from cache or fetches
// the first page via the registered paginated fetcher.
func ecsSvcRelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}
