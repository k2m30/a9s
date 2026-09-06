// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// igw_related.go contains Internet Gateway related-resource checker functions.
package aws

import (
	"context"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkIGWVPC extracts Attachments[0].VpcId from the IGW RawStruct and searches
// the vpc cache for a matching resource (Pattern F + C hybrid: field extraction
// from self, then cache lookup).
func checkIGWVPC(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ec2types.InternetGateway](res.RawStruct)
	if !ok {
		if res.RawStruct == nil {
			return resource.UnknownRelated("vpc")
		}
		return resource.KnownRelated("vpc", nil, false)
	}
	if len(raw.Attachments) == 0 || raw.Attachments[0].VpcId == nil || *raw.Attachments[0].VpcId == "" {
		return resource.KnownRelated("vpc", nil, false)
	}
	// In-body: the IGW's own Attachments[0].VpcId IS the attached VPC.
	return relatedResult("vpc", []string{*raw.Attachments[0].VpcId})
}

// checkIGWRTB searches the rtb cache for route tables that contain a route
// with a GatewayId matching this IGW's ID (Pattern C — search target cache).
func checkIGWRTB(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	igwID := res.ID
	raw, ok := assertStruct[ec2types.InternetGateway](res.RawStruct)
	if ok && raw.InternetGatewayId != nil && *raw.InternetGatewayId != "" {
		igwID = *raw.InternetGatewayId
	}
	if igwID == "" {
		return resource.KnownRelated("rtb", nil, false)
	}

	rtbList, truncated, err := relatedResourcesFor(ctx, clients, cache, "rtb")
	if err != nil {
		return resource.ErrorRelated("rtb", err)
	}
	if rtbList == nil {
		return resource.UnknownRelated("rtb")
	}

	var ids []string
	for _, rtbRes := range rtbList {
		rtbRaw, rtbOk := assertStruct[ec2types.RouteTable](rtbRes.RawStruct)
		if !rtbOk {
			continue
		}
		for _, route := range rtbRaw.Routes {
			if route.GatewayId != nil && *route.GatewayId == igwID {
				ids = append(ids, rtbRes.ID)
				break
			}
		}
	}
	return relatedResultTrunc("rtb", ids, truncated)
}
