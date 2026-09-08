// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// vpcpeer_issue_enrichment.go — Wave 2 cache-scan enrichment for vpc-peer.
//
// Mirrors lt_issue_enrichment.go's EnrichLTDeprecatedAMI layer: zero AWS API
// calls, scanning the already-loaded "rtb" cache instead of the fetcher's own
// client. Both derived signals ship as "~" background checks per the
// established Wave-2 `~`-on-Healthy treatment (the dbi maintenance-scheduled
// / lt deprecated-AMI precedent) — docs/resources/vpc-peer.md §4 note.
package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// vpc-peer.* Wave 2 FindingCodes — docs/resources/vpc-peer.md §3.2/§4.
const (
	vpcPeerCodeNoLocalRoute    domain.FindingCode = "vpc-peer.warn.no_local_route"
	vpcPeerCodeRouteBlackholed domain.FindingCode = "vpc-peer.warn.route_blackholed"
)

// EnrichVpcPeerRoutes cross-references each ACTIVE peering connection
// against the already-loaded "rtb" cache: zero routes referencing it fires
// "no local route to peer"; a route referencing it with State blackhole
// fires "route to peer blackholed" (blackhole takes precedence when a
// connection has both a blackholed route and no other route). Zero AWS API
// calls. Guards: the rtb cache must be present AND not truncated — otherwise
// this is a no-op, never a guess (docs/resources/vpc-peer.md §3.2).
func EnrichVpcPeerRoutes(_ context.Context, _ *ServiceClients, resources []resource.Resource, cache resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:         make(map[string][]domain.Finding),
		AttentionDetails: make(map[string]map[domain.FindingCode]domain.AttentionDetail),
		TruncatedIDs:     make(map[string]bool),
	}

	rtbList, truncated, ok := cachedTypedRows[ec2types.RouteTable](cache, "rtb")
	if !ok || truncated {
		return result, nil
	}

	blackholed := make(map[string]bool)
	routed := make(map[string]bool)
	for _, row := range rtbList {
		for _, route := range row.Raw.Routes {
			pcxID := aws.ToString(route.VpcPeeringConnectionId)
			if pcxID == "" {
				continue
			}
			if route.State == ec2types.RouteStateBlackhole {
				blackholed[pcxID] = true
				continue
			}
			routed[pcxID] = true
		}
	}

	for _, res := range resources {
		raw, asserted := assertStruct[ec2types.VpcPeeringConnection](res.RawStruct)
		if !asserted || raw.Status == nil || raw.Status.Code != ec2types.VpcPeeringConnectionStateReasonCodeActive {
			continue
		}
		switch {
		case blackholed[res.ID]:
			setWave2Finding(&result, res.ID, vpcPeerCodeRouteBlackholed, "vpc-peer", nil)

		case !routed[res.ID]:
			setWave2Finding(&result, res.ID, vpcPeerCodeNoLocalRoute, "vpc-peer", nil)

		}
	}

	return result, nil
}
