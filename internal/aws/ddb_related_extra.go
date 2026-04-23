// ddb_related_extra.go contains additional DynamoDB related-resource
// checkers required by docs/related-resources.md.
package aws

import (
	"context"
	"strings"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkDdbLogs scans the logs cache for log groups that are part of the
// ContributorInsights convention: /aws/dynamodb/tables/<table-name>/
// Uses a strict prefix match to avoid false positives from Lambda or other
// services whose log group names may contain the table name as a substring.
func checkDdbLogs(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	name := res.ID
	if name == "" {
		return resource.RelatedCheckResult{TargetType: "logs", Count: 0}
	}
	logList, truncated, err := ddbRelatedResources(ctx, clients, cache, "logs")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "logs", Count: -1, Err: err}
	}
	if logList == nil {
		return resource.ApproximateZero("logs")
	}
	prefix := "/aws/dynamodb/tables/" + name + "/"
	var ids []string
	for _, logRes := range logList {
		if strings.HasPrefix(logRes.ID, prefix) {
			ids = append(ids, logRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("logs")
	}
	return relatedResult("logs", ids)
}

// checkDdbVPCE scans the vpce cache for DynamoDB Gateway endpoints in this
// region. DynamoDB endpoints are service-scoped (not per-table), so every
// matching gateway endpoint is surfaced. Matching requires both:
//   - service_name ending in ".dynamodb" (e.g. "com.amazonaws.us-east-1.dynamodb")
//   - type == "Gateway" (the vpce fetcher stores VpcEndpointType in Fields["type"])
func checkDdbVPCE(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	_ = res
	vpceList, truncated, err := ddbRelatedResources(ctx, clients, cache, "vpce")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "vpce", Count: -1, Err: err}
	}
	if vpceList == nil {
		return resource.ApproximateZero("vpce")
	}
	var ids []string
	for _, vpceRes := range vpceList {
		if strings.HasSuffix(vpceRes.Fields["service_name"], ".dynamodb") &&
			vpceRes.Fields["type"] == "Gateway" {
			ids = append(ids, vpceRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("vpce")
	}
	return relatedResult("vpce", ids)
}
