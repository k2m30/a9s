// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// ddb_related_extra.go contains additional DynamoDB related-resource
// checkers required by docs/related-resources.md.
package aws

import (
	"context"
	"strings"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkDdbVPCE scans the vpce cache for DynamoDB endpoints in this region.
// DynamoDB endpoints are service-scoped (not per-table), so every matching
// endpoint is surfaced. Matching is on service_name ending in ".dynamodb"
// (e.g. "com.amazonaws.us-east-1.dynamodb"), which both the gateway endpoint
// and the PrivateLink interface endpoint carry.
func checkDdbVPCE(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	_ = res
	vpceList, truncated, err := relatedResourcesFor(ctx, clients, cache, "vpce")
	if err != nil {
		return ReadFailed("vpce", err)
	}
	if vpceList == nil {
		return NotRead("vpce")
	}
	var ids []string
	for _, vpceRes := range vpceList {
		if strings.HasSuffix(vpceRes.Fields["service_name"], ".dynamodb") {
			ids = append(ids, vpceRes.ID)
		}
	}
	return relatedResultTrunc("vpce", ids, truncated)
}
