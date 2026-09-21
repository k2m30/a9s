// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// ddb_related_extra.go contains additional DynamoDB related-resource
// checkers required by docs/related-resources.md.
package aws

import (
	"context"
	"strings"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkDdbLogs scans the logs cache for log groups that are part of the
// ContributorInsights convention: /aws/dynamodb/tables/<table-name>/
// Uses a strict prefix match to avoid false positives from Lambda or other
// services whose log group names may contain the table name as a substring.
func checkDdbLogs(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	name := res.ID
	if name == "" {
		return resource.ProvenZero("logs", "name")
	}
	logList, truncated, err := relatedResourcesFor(ctx, clients, cache, "logs")
	if err != nil {
		return resource.ErrorRelated("logs", err)
	}
	if logList == nil {
		return resource.UnknownRelated("logs")
	}
	prefix := "/aws/dynamodb/tables/" + name + "/"
	var ids []string
	for _, logRes := range logList {
		if strings.HasPrefix(logRes.ID, prefix) {
			ids = append(ids, logRes.ID)
		}
	}
	return relatedResultTrunc("logs", ids, truncated)
}

// checkDdbVPCE scans the vpce cache for DynamoDB endpoints in this region.
// DynamoDB endpoints are service-scoped (not per-table), so every matching
// endpoint is surfaced. Matching is on service_name ending in ".dynamodb"
// (e.g. "com.amazonaws.us-east-1.dynamodb"), which both the gateway endpoint
// and the PrivateLink interface endpoint carry.
func checkDdbVPCE(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	_ = res
	vpceList, truncated, err := relatedResourcesFor(ctx, clients, cache, "vpce")
	if err != nil {
		return resource.ErrorRelated("vpce", err)
	}
	if vpceList == nil {
		return resource.UnknownRelated("vpce")
	}
	var ids []string
	for _, vpceRes := range vpceList {
		if strings.HasSuffix(vpceRes.Fields["service_name"], ".dynamodb") {
			ids = append(ids, vpceRes.ID)
		}
	}
	return relatedResultTrunc("vpce", ids, truncated)
}
