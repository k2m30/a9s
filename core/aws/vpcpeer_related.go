// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// vpcpeer_related.go contains VPC Peering Connection related-resource
// checker functions.
//
// rtb and vpc are cache-scan checkers that read the sibling resource cache
// directly (never RawStruct alone) — zero extra AWS calls, mirroring
// lt_related.go's asg/ng/ec2 checkers via the shared cachedTypedRows
// tri-state helper (related_common.go). ct-events uses the universal
// ctEventsCheckerFor pivot, registered inline in catalog_networking.go — no
// dedicated function here, matching the asg/lt/ebs convention. No sg checker
// exists: docs/related-resources.md § vpc-peer excludes it (no declared
// link; cross-region peers cannot reference SGs at all).
package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkVpcPeerRTB scans the loaded rtb cache for routes referencing this
// peering connection via Routes[].VpcPeeringConnectionId — "who actually
// routes to this peer", the pivot that makes vpc-peer worth adding
// (docs/resources/vpc-peer.md §2 rtb bullet). Zero extra API calls.
//
// A present-but-truncated rtb cache is trusted enough to scan — the house
// fleet convention (lt's asg/ng/ec2 checkers): State stays RelatedResolved,
// Count/ResourceIDs report the honest partial scan, and Truncated=true
// carries through so the row renders "N+". Only a wholly absent cache
// entry (never loaded this session) is Unknown.
func checkVpcPeerRTB(_ context.Context, _ any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	rtbList, truncated, ok := cachedTypedRows[ec2types.RouteTable](cache, "rtb")
	if !ok {
		return resource.UnknownRelated("rtb")
	}
	var ids []string
	for _, row := range rtbList {
		for _, route := range row.Raw.Routes {
			if aws.ToString(route.VpcPeeringConnectionId) == res.ID {
				ids = append(ids, row.ID)
				break
			}
		}
	}
	return relatedResultTrunc("rtb", ids, truncated)
}

// checkVpcPeerVPC is the symmetric CACHE-MEMBERSHIP GATE: it pivots for
// whichever side's VpcId (requester and/or accepter) is present in the
// loaded local vpc cache. A cross-account remote side is never in the
// cache — it stays a plain fact (VpcId + OwnerId + Region, visible via the
// raw detail fields), never a pivot. Never hardcode requester-is-local
// (docs/resources/vpc-peer.md §2 vpc bullet).
//
// A present-but-truncated vpc cache is trusted enough to scan, mirroring
// checkVpcPeerRTB's house fleet convention — only a wholly absent cache
// entry is Unknown.
func checkVpcPeerVPC(_ context.Context, _ any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	vpcList, truncated, ok := cachedTypedRows[ec2types.Vpc](cache, "vpc")
	if !ok {
		return resource.UnknownRelated("vpc")
	}
	raw, asserted := assertStruct[ec2types.VpcPeeringConnection](res.RawStruct)
	if !asserted {
		return resource.UnknownRelated("vpc")
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
