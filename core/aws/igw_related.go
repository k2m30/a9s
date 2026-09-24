// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// igw_related.go contains Internet Gateway related-resource checker functions.
package aws

import (
	"context"
	"slices"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkIGWVPC extracts Attachments[0].VpcId from the IGW RawStruct and searches
// the vpc cache for a matching resource.
func checkIGWVPC(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ec2types.InternetGateway](res.RawStruct)
	if !ok {
		return NotRead("vpc")
	}
	if len(raw.Attachments) == 0 || raw.Attachments[0].VpcId == nil || *raw.Attachments[0].VpcId == "" {
		return foundNone("vpc", "raw.Attachments[0].VpcId")
	}
	// The IGW's own Attachments[0].VpcId is the attached VPC.
	return relatedResultTrunc("vpc", []string{*raw.Attachments[0].VpcId}, false)
}

// checkIGWRTB searches the rtb cache for route tables that contain a route
// with a GatewayId matching this IGW's ID.
func checkIGWRTB(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	igwID := res.ID
	raw, ok := assertStruct[ec2types.InternetGateway](res.RawStruct)
	if ok && raw.InternetGatewayId != nil && *raw.InternetGatewayId != "" {
		igwID = *raw.InternetGatewayId
	}
	if igwID == "" {
		return keyMissing("rtb", "igwID")
	}

	rtbList, truncated, err := relatedResourcesFor(ctx, clients, cache, "rtb")
	if err != nil {
		return ReadFailed("rtb", err)
	}
	if rtbList == nil {
		return NotRead("rtb")
	}

	var ids []string
	for _, rtbRes := range rtbList {
		rtbRaw, rtbOk := assertStruct[ec2types.RouteTable](rtbRes.RawStruct)
		if !rtbOk {
			continue
		}
		if slices.ContainsFunc(rtbRaw.Routes, func(route ec2types.Route) bool { return routeTargetsGateway(route, igwID) }) {
			ids = append(ids, rtbRes.ID)
		}
	}
	return relatedResultTrunc("rtb", ids, truncated)
}
