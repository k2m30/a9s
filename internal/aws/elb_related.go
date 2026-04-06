// elb_related.go contains ELB related-resource checker functions.
package aws

import (
	"context"
	"strings"

	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkELBTargetGroups checks the cache for target groups whose LoadBalancerArns
// contains this ELB's ARN.
func checkELBTargetGroups(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	elbARN := res.Fields["load_balancer_arn"]
	if elbARN == "" {
		raw, ok := assertStruct[elbv2types.LoadBalancer](res.RawStruct)
		if ok && raw.LoadBalancerArn != nil {
			elbARN = *raw.LoadBalancerArn
		}
	}
	if elbARN == "" {
		return resource.RelatedCheckResult{TargetType: "tg", Count: 0}
	}

	tgList, truncated, err := elbRelatedResources(ctx, clients, cache, "tg")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "tg", Count: -1, Err: err}
	}
	if tgList == nil {
		return resource.RelatedCheckResult{TargetType: "tg", Count: -1}
	}

	var ids []string
	for _, tgRes := range tgList {
		raw, ok := assertStruct[elbv2types.TargetGroup](tgRes.RawStruct)
		if !ok {
			continue
		}
		for _, lbARN := range raw.LoadBalancerArns {
			if lbARN == elbARN {
				ids = append(ids, tgRes.ID)
				break
			}
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "tg", Count: -1}
	}
	return relatedResult("tg", ids)
}

// checkELBAlarms checks the cache for CloudWatch alarms with a "LoadBalancer"
// dimension matching the ARN suffix of this ELB (everything after "loadbalancer/").
func checkELBAlarms(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	elbARN := res.Fields["load_balancer_arn"]
	if elbARN == "" {
		raw, ok := assertStruct[elbv2types.LoadBalancer](res.RawStruct)
		if ok && raw.LoadBalancerArn != nil {
			elbARN = *raw.LoadBalancerArn
		}
	}
	if elbARN == "" {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: 0}
	}

	// Compute the ARN suffix: everything after "loadbalancer/"
	const prefix = "loadbalancer/"
	idx := strings.Index(elbARN, prefix)
	arnSuffix := elbARN
	if idx >= 0 {
		arnSuffix = elbARN[idx+len(prefix):]
	}

	alarmList, truncated, err := elbRelatedResources(ctx, clients, cache, "alarm")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1, Err: err}
	}
	if alarmList == nil {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1}
	}

	var ids []string
	for _, alarmRes := range alarmList {
		raw, ok := assertStruct[cwtypes.MetricAlarm](alarmRes.RawStruct)
		if !ok {
			continue
		}
		for _, d := range raw.Dimensions {
			if d.Name != nil && *d.Name == "LoadBalancer" && d.Value != nil && *d.Value == arnSuffix {
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

// checkELBCFN returns Count: 0 because ELBv2 LoadBalancer tags are not included
// in the DescribeLoadBalancers response — the CFN relationship cannot be
// determined from cache alone.
func checkELBCFN(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "cfn", Count: 0}
}

// checkELBR53 returns Count: -1 (unknown) because Route 53 cached resources
// are hosted zones, not record sets. Alias target information (which references
// ELB DNS names) is only available at the record level via ListResourceRecordSets,
// which is not stored in the r53 hosted-zone cache.
func checkELBR53(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "r53", Count: -1}
}

// elbRelatedResources returns the resource list for target from cache or by
// fetching the first page via the registered paginated fetcher.
func elbRelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}
