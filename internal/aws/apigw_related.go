// apigw_related.go contains API Gateway related-resource checker functions.
package aws

import (
	"context"
	"strings"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkApigwLogs searches the logs cache for log groups associated with this
// API Gateway by naming convention:
//   - API-Gateway-Execution-Logs_{apiID}/ prefix (default execution log group)
//   - /aws/apigateway/{apiName} (custom access log group convention)
//
// Pattern N — naming convention.
func checkApigwLogs(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	apiID := res.ID
	apiName := res.Name
	if apiName == "" {
		apiName = res.Fields["name"]
	}
	if apiID == "" && apiName == "" {
		return resource.RelatedCheckResult{TargetType: "logs", Count: 0}
	}

	logList, truncated, err := apigwRelatedResources(ctx, clients, cache, "logs")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "logs", Count: -1, Err: err}
	}
	if logList == nil {
		return resource.RelatedCheckResult{TargetType: "logs", Count: -1}
	}

	executionPrefix := "API-Gateway-Execution-Logs_" + apiID + "/"
	accessLogPrefix := "/aws/apigateway/" + apiName

	var ids []string
	for _, logRes := range logList {
		if (apiID != "" && strings.HasPrefix(logRes.ID, executionPrefix)) ||
			(apiName != "" && strings.HasPrefix(logRes.ID, accessLogPrefix)) {
			ids = append(ids, logRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "logs", Count: -1}
	}
	return relatedResult("logs", ids)
}













// apigwRelatedResources returns the resource list for target from cache or by fetching the first page.
func apigwRelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}
