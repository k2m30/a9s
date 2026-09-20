// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// nat_related.go contains NAT Gateway related-resource checker functions.
package aws

import (
	"context"
	"slices"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkNATVPC extracts VpcId from the NAT Gateway RawStruct and searches the
// vpc cache for a matching resource.
func checkNATVPC(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ec2types.NatGateway](res.RawStruct)
	if !ok {
		if res.RawStruct == nil {
			return resource.UnknownRelated("vpc")
		}
		return resource.KnownRelated("vpc", nil, false)
	}
	if raw.VpcId == nil || *raw.VpcId == "" {
		return resource.ProvenZero("vpc", "raw.VpcId")
	}
	// The NAT gateway's own VpcId is the related VPC; it resolves by
	// identity.
	return relatedResultTrunc("vpc", []string{*raw.VpcId}, false)
}

// checkNATSubnet extracts SubnetId from the NAT Gateway RawStruct and searches
// the subnet cache for a matching resource.
func checkNATSubnet(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ec2types.NatGateway](res.RawStruct)
	if !ok {
		if res.RawStruct == nil {
			return resource.UnknownRelated("subnet")
		}
		return resource.KnownRelated("subnet", nil, false)
	}
	if raw.SubnetId == nil || *raw.SubnetId == "" {
		return resource.ProvenZero("subnet", "raw.SubnetId")
	}
	// The NAT gateway's own SubnetId is the related subnet.
	return relatedResultTrunc("subnet", []string{*raw.SubnetId}, false)
}

// checkNATRTB searches the rtb cache for route tables that contain a route
// with a NatGatewayId matching this NAT gateway's ID.
func checkNATRTB(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	natID := res.ID
	raw, ok := assertStruct[ec2types.NatGateway](res.RawStruct)
	if ok && raw.NatGatewayId != nil && *raw.NatGatewayId != "" {
		natID = *raw.NatGatewayId
	}
	if natID == "" {
		return resource.ProvenZero("rtb", "natID")
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
		if slices.ContainsFunc(rtbRaw.Routes, func(route ec2types.Route) bool { return routeTargetsGateway(route, natID) }) {
			ids = append(ids, rtbRes.ID)
		}
	}
	return relatedResultTrunc("rtb", ids, truncated)
}

// checkNATEIP extracts AllocationId values from the NAT gateway's
// NatGatewayAddresses slice and searches the eip cache for matching EIPs.
func checkNATEIP(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ec2types.NatGateway](res.RawStruct)
	if !ok {
		if res.RawStruct == nil {
			return resource.UnknownRelated("eip")
		}
		return resource.KnownRelated("eip", nil, false)
	}
	// NatGatewayAddresses[].AllocationId is the eip resource id (eip
	// resources are keyed by AllocationId — see eip.go).
	var ids []string
	for _, addr := range raw.NatGatewayAddresses {
		if addr.AllocationId != nil && *addr.AllocationId != "" {
			ids = append(ids, *addr.AllocationId)
		}
	}
	return relatedResultTrunc("eip", ids, false)
}

// checkNATENI extracts NetworkInterfaceId values from the NAT gateway's
// NatGatewayAddresses slice and searches the eni cache for matching interfaces.
func checkNATENI(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ec2types.NatGateway](res.RawStruct)
	if !ok {
		if res.RawStruct == nil {
			return resource.UnknownRelated("eni")
		}
		return resource.KnownRelated("eni", nil, false)
	}
	// NatGatewayAddresses[].NetworkInterfaceId is the eni resource id.
	var ids []string
	for _, addr := range raw.NatGatewayAddresses {
		if addr.NetworkInterfaceId != nil && *addr.NetworkInterfaceId != "" {
			ids = append(ids, *addr.NetworkInterfaceId)
		}
	}
	return relatedResultTrunc("eni", ids, false)
}

// checkNATAlarm reports CloudWatch alarms for this NAT Gateway.
// NAT Gateway metrics use dimension "NatGatewayId" (e.g. ActiveConnectionCount).
// Scans the alarm cache for MetricAlarm.Dimensions with that name/value.
func checkNATAlarm(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	natID := res.ID
	raw, ok := assertStruct[ec2types.NatGateway](res.RawStruct)
	if ok && raw.NatGatewayId != nil && *raw.NatGatewayId != "" {
		natID = *raw.NatGatewayId
	}
	return alarmIDsByDimension(ctx, clients, cache, "", "NatGatewayId", natID)
}
