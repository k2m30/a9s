// msk_related.go contains MSK cluster related-resource checker functions.
package aws

import (
	"context"

	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterRelated("msk", []resource.RelatedDef{
		{TargetType: "lambda", DisplayName: "Lambda Functions", Checker: checkMSKLambda, NeedsTargetCache: false},
		{TargetType: "alarm", DisplayName: "CW Alarms", Checker: checkMSKAlarms, NeedsTargetCache: true},
		{TargetType: "cfn", DisplayName: "CloudFormation", Checker: checkMSKCFN, NeedsTargetCache: false},
	})
}

// checkMSKLambda returns Count: 0 because MSK cluster event source mappings are
// not available in the list API — the relationship cannot be determined from
// cache alone.
func checkMSKLambda(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "lambda", Count: 0}
}

// checkMSKCFN returns Count: 0 because MSK cluster tags are not included in the
// ListClusters response — the CFN relationship cannot be determined from cache
// alone.
func checkMSKCFN(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "cfn", Count: 0}
}

// checkMSKAlarms checks the cache for CloudWatch alarms with "Cluster Name" dimension matching this cluster.
func checkMSKAlarms(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	clusterName := res.ID
	if clusterName == "" {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: 0}
	}

	alarmList, truncated, err := mskRelatedResources(ctx, clients, cache, "alarm")
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
		for _, d := range rawAlarm.Dimensions {
			if d.Name != nil && *d.Name == "Cluster Name" && d.Value != nil && *d.Value == clusterName {
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

// mskRelatedResources returns the resource list for target from cache or by fetching the first page.
func mskRelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}
