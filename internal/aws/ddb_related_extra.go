// ddb_related_extra.go contains additional DynamoDB related-resource
// checkers required by docs/related-resources.md.
package aws

import (
	"context"
	"strings"

	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkDdbLogs scans logs cache for log groups named like the table. No
// standard convention; match substring.
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
		return resource.RelatedCheckResult{TargetType: "logs", Count: -1}
	}
	var ids []string
	for _, logRes := range logList {
		if strings.Contains(logRes.ID, name) {
			ids = append(ids, logRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("logs")
	}
	return relatedResult("logs", ids)
}

// checkDdbVPCE scans the vpce cache for DynamoDB Gateway endpoints in this
// account. The table isn't tied to a specific endpoint; we match by service
// type.
func checkDdbVPCE(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	_ = res
	vpceList, truncated, err := ddbRelatedResources(ctx, clients, cache, "vpce")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "vpce", Count: -1, Err: err}
	}
	if vpceList == nil {
		return resource.RelatedCheckResult{TargetType: "vpce", Count: -1}
	}
	var ids []string
	for _, vpceRes := range vpceList {
		if strings.Contains(vpceRes.Fields["service_name"], ".dynamodb") {
			ids = append(ids, vpceRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("vpce")
	}
	return relatedResult("vpce", ids)
}

// keep ddbtypes imported for future additions.
var _ = ddbtypes.TableDescription{}
