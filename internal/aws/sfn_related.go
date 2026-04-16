// sfn_related.go contains Step Functions related-resource checker functions.
package aws

import (
	"context"

	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkSFNLogs searches the logs cache for the vendedlogs log group associated
// with this state machine by naming convention.
// Pattern N — naming convention: /aws/vendedlogs/states/{sfnName}
func checkSFNLogs(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	sfnName := res.ID
	if sfnName == "" {
		return resource.RelatedCheckResult{TargetType: "logs", Count: 0}
	}

	expectedLogGroup := "/aws/vendedlogs/states/" + sfnName

	logList, truncated, err := sfnRelatedResources(ctx, clients, cache, "logs")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "logs", Count: -1, Err: err}
	}
	if logList == nil {
		return resource.RelatedCheckResult{TargetType: "logs", Count: -1}
	}

	var ids []string
	for _, logRes := range logList {
		if logRes.ID == expectedLogGroup {
			ids = append(ids, logRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "logs", Count: -1}
	}
	return relatedResult("logs", ids)
}

// checkSFNAlarm searches the alarm cache for alarms with a "StateMachineArn" dimension
// matching this state machine's ARN.
// Pattern D — dimension-based lookup.
func checkSFNAlarm(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	sfnARN := res.Fields["arn"]
	if sfnARN == "" {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1}
	}

	alarmList, truncated, err := sfnRelatedResources(ctx, clients, cache, "alarm")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1, Err: err}
	}
	if alarmList == nil {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1}
	}

	var ids []string
	for _, alarmRes := range alarmList {
		alarm, ok := assertStruct[cwtypes.MetricAlarm](alarmRes.RawStruct)
		if !ok {
			continue
		}
		for _, d := range alarm.Dimensions {
			if d.Name != nil && *d.Name == "StateMachineArn" && d.Value != nil && *d.Value == sfnARN {
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





// sfnRelatedResources returns the resource list for target from cache or by fetching the first page.
func sfnRelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}

