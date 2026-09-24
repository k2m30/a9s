// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// vpcpeer_related.go contains VPC Peering Connection related-resource
// checker functions.
//
// rtb and vpc are cache-scan checkers that read the sibling resource cache
// directly (never RawStruct alone) — zero extra AWS calls, mirroring
// lt_related.go's asg/ng/ec2 checkers via the shared cachedTypedRows
// tri-state helper (related_common.go).
package aws

import (
	"context"
	"slices"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkVpcPeerRTB scans the loaded rtb cache for live routes to this peering
// connection via Routes[].VpcPeeringConnectionId — "who actually routes to
// this peer". Zero extra API calls.
//
// A present-but-truncated rtb cache is trusted enough to scan — the house
// fleet convention (lt's asg/ng/ec2 checkers): State stays RelatedResolved,
// Count/ResourceIDs report the honest partial scan, and Truncated=true
// carries through so the row renders "N+". Only a wholly absent cache
// entry (never loaded this session) is Unknown.
func checkVpcPeerRTB(_ context.Context, _ any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	rtbList, truncated, ok := cachedTypedRows[ec2types.RouteTable](cache, "rtb")
	if !ok {
		return NotRead("rtb")
	}
	var ids []string
	for _, row := range rtbList {
		if slices.ContainsFunc(row.Raw.Routes, func(route ec2types.Route) bool { return routeTargetsGateway(route, res.ID) }) {
			ids = append(ids, row.ID)
		}
	}
	return relatedResultTrunc("rtb", ids, truncated)
}

// checkVpcPeerVPC is the symmetric CACHE-MEMBERSHIP GATE: it pivots for
// whichever side's VpcId (requester and/or accepter) is present in the
// loaded local vpc cache. A cross-account remote side is never in the
// cache — it stays a plain fact (VpcId + OwnerId + Region, visible via the
// raw detail fields), never a pivot. Either side may be the local one.
//
// A present-but-truncated vpc cache is trusted enough to scan, mirroring
// checkVpcPeerRTB's house fleet convention — only a wholly absent cache
// entry is Unknown.
func checkVpcPeerVPC(_ context.Context, _ any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	vpcList, truncated, ok := cachedTypedRows[ec2types.Vpc](cache, "vpc")
	if !ok {
		return NotRead("vpc")
	}
	raw, asserted := assertStruct[ec2types.VpcPeeringConnection](res.RawStruct)
	if !asserted {
		return NotRead("vpc")
	}

	present := make(map[string]struct{}, len(vpcList))
	for _, row := range vpcList {
		present[row.ID] = struct{}{}
	}

	var ids []string
	for _, side := range []*ec2types.VpcPeeringConnectionVpcInfo{raw.RequesterVpcInfo, raw.AccepterVpcInfo} {
		if side == nil {
			continue
		}
		id := aws.ToString(side.VpcId)
		if id == "" {
			continue
		}
		if _, member := present[id]; member {
			ids = append(ids, id)
		}
	}
	return relatedResultTrunc("vpc", ids, truncated)
}
