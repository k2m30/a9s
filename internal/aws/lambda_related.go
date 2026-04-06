// lambda_related.go contains Lambda function related-resource checker functions.
package aws

import (
	"context"
	"strings"

	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkLambdaRole extracts the Role ARN from the Lambda FunctionConfiguration RawStruct.
// It extracts the role name from the last path segment of the ARN (after the last "/")
// and searches the role cache by name.
func checkLambdaRole(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	fn, ok := assertStruct[lambdatypes.FunctionConfiguration](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "role", Count: -1}
	}
	if fn.Role == nil || *fn.Role == "" {
		return resource.RelatedCheckResult{TargetType: "role", Count: 0}
	}
	roleARN := *fn.Role
	roleName := roleARN
	if idx := strings.LastIndex(roleARN, "/"); idx >= 0 && idx < len(roleARN)-1 {
		roleName = roleARN[idx+1:]
	}

	roleList, _, err := lambdaRelatedResources(ctx, clients, cache, "role")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "role", Count: -1, Err: err}
	}
	if roleList == nil {
		return resource.RelatedCheckResult{TargetType: "role", Count: -1}
	}

	var ids []string
	for _, roleRes := range roleList {
		if roleRes.Name == roleName || roleRes.Fields["role_name"] == roleName {
			ids = append(ids, roleRes.ID)
		}
	}
	return relatedResult("role", ids)
}

// checkLambdaAlarms searches the alarm cache for alarms with a "FunctionName" dimension
// matching this Lambda function's name (res.ID).
func checkLambdaAlarms(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	functionName := res.ID
	if functionName == "" {
		functionName = res.Name
	}
	if functionName == "" {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: 0}
	}

	alarmList, truncated, err := lambdaRelatedResources(ctx, clients, cache, "alarm")
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
			if d.Name != nil && *d.Name == "FunctionName" && d.Value != nil && *d.Value == functionName {
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

// lambdaRelatedResources returns the resource list for target from cache or by fetching the first page.
func lambdaRelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}
