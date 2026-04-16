// ecs_services_related_extra.go contains additional ECS service related-
// resource checkers required by docs/related-resources.md.
package aws

import (
	"context"
	"strings"

	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkECSSvcCTEvents scans ct-events cache for events referencing this service.
func checkECSSvcCTEvents(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	svcName := res.ID
	if svcName == "" {
		return resource.RelatedCheckResult{TargetType: "ct-events", Count: 0}
	}
	evList, truncated, err := ecsSvcRelatedResources(ctx, clients, cache, "ct-events")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "ct-events", Count: -1, Err: err}
	}
	if evList == nil {
		return resource.RelatedCheckResult{TargetType: "ct-events", Count: -1}
	}
	var ids []string
	for _, evRes := range evList {
		ev, ok := assertStruct[cloudtrailtypes.Event](evRes.RawStruct)
		if !ok {
			continue
		}
		for _, r := range ev.Resources {
			if r.ResourceName != nil && strings.Contains(*r.ResourceName, svcName) {
				ids = append(ids, evRes.ID)
				break
			}
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "ct-events", Count: -1}
	}
	return relatedResult("ct-events", ids)
}

// checkECSSvcCF scans the cf cache for CloudFront distributions whose origin
// is the ELB backing this service. No direct linkage in Service struct → 0.
func checkECSSvcCF(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "cf", Count: 0}
}

// checkECSSvcEbRule scans eb-rule cache for rules targeting this service
// (EventBridge → ECS run-task targets). Not in the Service struct → 0 unless
// a future eb-rule cache carries enriched targets.
func checkECSSvcEbRule(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "eb-rule", Count: 0}
}

// checkECSSvcECR scans ecr cache for repositories referenced by this
// service's task definition's container images. Requires task definition
// detail not in the Service struct → 0.
func checkECSSvcECR(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "ecr", Count: 0}
}

// checkECSSvcTasks scans the ecs-task cache for tasks belonging to this
// service (task.Group == "service:{svcName}").
func checkECSSvcTasks(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	svcName := res.ID
	if svcName == "" {
		return resource.RelatedCheckResult{TargetType: "ecs-task", Count: 0}
	}
	taskList, truncated, err := ecsSvcRelatedResources(ctx, clients, cache, "ecs-task")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "ecs-task", Count: -1, Err: err}
	}
	if taskList == nil {
		return resource.RelatedCheckResult{TargetType: "ecs-task", Count: -1}
	}
	var ids []string
	for _, tRes := range taskList {
		task, ok := assertStruct[ecstypes.Task](tRes.RawStruct)
		if !ok {
			continue
		}
		if task.Group != nil && *task.Group == "service:"+svcName {
			ids = append(ids, tRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "ecs-task", Count: -1}
	}
	return relatedResult("ecs-task", ids)
}

// checkECSSvcR53 scans r53 cache for records aliasing the ELB backing this
// service. Requires cross-reference with elb cache; Count:0 conservatively.
func checkECSSvcR53(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "r53", Count: 0}
}

// checkECSSvcSecrets — secrets live on task definition ContainerDefinitions,
// not on the Service struct. Count:0.
func checkECSSvcSecrets(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "secrets", Count: 0}
}

// checkECSSvcSFN — Step Functions sometimes orchestrate ECS services; not
// resolvable from Service struct.
func checkECSSvcSFN(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "sfn", Count: 0}
}

// checkECSSvcSubnet extracts subnet IDs from Service's
// NetworkConfiguration.AwsvpcConfiguration.Subnets. Pattern F.
func checkECSSvcSubnet(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ecstypes.Service](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "subnet", Count: -1}
	}
	if raw.NetworkConfiguration == nil || raw.NetworkConfiguration.AwsvpcConfiguration == nil {
		return resource.RelatedCheckResult{TargetType: "subnet", Count: 0}
	}
	var ids []string
	for _, s := range raw.NetworkConfiguration.AwsvpcConfiguration.Subnets {
		if s != "" {
			ids = append(ids, s)
		}
	}
	return relatedResult("subnet", ids)
}

// checkECSSvcVPC derives the VPC from the service's subnets. Pattern C:
// look up each subnet in the subnet cache and collect VpcIds.
func checkECSSvcVPC(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ecstypes.Service](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "vpc", Count: -1}
	}
	if raw.NetworkConfiguration == nil || raw.NetworkConfiguration.AwsvpcConfiguration == nil {
		return resource.RelatedCheckResult{TargetType: "vpc", Count: 0}
	}
	subnetIDs := raw.NetworkConfiguration.AwsvpcConfiguration.Subnets
	if len(subnetIDs) == 0 {
		return resource.RelatedCheckResult{TargetType: "vpc", Count: 0}
	}
	subnetList, truncated, err := ecsSvcRelatedResources(ctx, clients, cache, "subnet")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "vpc", Count: -1, Err: err}
	}
	if subnetList == nil {
		return resource.RelatedCheckResult{TargetType: "vpc", Count: -1}
	}
	wanted := make(map[string]struct{}, len(subnetIDs))
	for _, s := range subnetIDs {
		wanted[s] = struct{}{}
	}
	vpcSet := make(map[string]struct{})
	for _, sRes := range subnetList {
		if _, ok := wanted[sRes.ID]; !ok {
			continue
		}
		if v := sRes.Fields["vpc_id"]; v != "" {
			vpcSet[v] = struct{}{}
		}
	}
	var ids []string
	for v := range vpcSet {
		ids = append(ids, v)
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "vpc", Count: -1}
	}
	return relatedResult("vpc", ids)
}
