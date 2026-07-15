// nat_related.go contains NAT Gateway related-resource checker functions.
package aws

import (
	"context"

	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkNATVPC extracts VpcId from the NAT Gateway RawStruct and searches the
// vpc cache for a matching resource (Pattern F + C hybrid).
func checkNATVPC(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ec2types.NatGateway](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "vpc", Count: 0}
	}
	if raw.VpcId == nil || *raw.VpcId == "" {
		return resource.RelatedCheckResult{TargetType: "vpc", Count: 0}
	}
	// In-body: the NAT gateway's own VpcId IS the related VPC. Resolve by
	// identity — no vpc-list fetch to confirm an id the source already carries.
	return relatedResult("vpc", []string{*raw.VpcId})
}

// checkNATSubnet extracts SubnetId from the NAT Gateway RawStruct and searches
// the subnet cache for a matching resource (Pattern F + C hybrid).
func checkNATSubnet(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ec2types.NatGateway](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "subnet", Count: 0}
	}
	if raw.SubnetId == nil || *raw.SubnetId == "" {
		return resource.RelatedCheckResult{TargetType: "subnet", Count: 0}
	}
	// In-body: the NAT gateway's own SubnetId IS the related subnet.
	return relatedResult("subnet", []string{*raw.SubnetId})
}

// checkNATRTB searches the rtb cache for route tables that contain a route
// with a NatGatewayId matching this NAT gateway's ID (Pattern C — search target cache).
func checkNATRTB(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	natID := res.ID
	raw, ok := assertStruct[ec2types.NatGateway](res.RawStruct)
	if ok && raw.NatGatewayId != nil && *raw.NatGatewayId != "" {
		natID = *raw.NatGatewayId
	}
	if natID == "" {
		return resource.RelatedCheckResult{TargetType: "rtb", Count: 0}
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
			if route.NatGatewayId != nil && *route.NatGatewayId == natID {
				ids = append(ids, rtbRes.ID)
				break
			}
		}
	}
	return relatedResultTrunc("rtb", ids, truncated)
}

// checkNATEIP extracts AllocationId values from the NAT gateway's
// NatGatewayAddresses slice and searches the eip cache for matching EIPs.
func checkNATEIP(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ec2types.NatGateway](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "eip", Count: 0}
	}
	// In-body: NatGatewayAddresses[].AllocationId IS the eip resource id (eip
	// resources are keyed by AllocationId — see eip.go). Resolve by identity.
	var ids []string
	for _, addr := range raw.NatGatewayAddresses {
		if addr.AllocationId != nil && *addr.AllocationId != "" {
			ids = append(ids, *addr.AllocationId)
		}
	}
	return relatedResult("eip", ids)
}

// checkNATENI extracts NetworkInterfaceId values from the NAT gateway's
// NatGatewayAddresses slice and searches the eni cache for matching interfaces.
func checkNATENI(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ec2types.NatGateway](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "eni", Count: 0}
	}
	// In-body: NatGatewayAddresses[].NetworkInterfaceId IS the eni resource id.
	var ids []string
	for _, addr := range raw.NatGatewayAddresses {
		if addr.NetworkInterfaceId != nil && *addr.NetworkInterfaceId != "" {
			ids = append(ids, *addr.NetworkInterfaceId)
		}
	}
	return relatedResult("eni", ids)
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
	if natID == "" {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: 0}
	}

	alarmList, truncated, err := relatedResourcesFor(ctx, clients, cache, "alarm")
	if err != nil {
		return resource.ErrorRelated("alarm", err)
	}
	if alarmList == nil {
		return resource.UnknownRelated("alarm")
	}

	var ids []string
	for _, alarmRes := range alarmList {
		alarmRaw, aOk := assertStruct[cwtypes.MetricAlarm](alarmRes.RawStruct)
		if !aOk {
			continue
		}
		for _, d := range alarmRaw.Dimensions {
			if d.Name != nil && *d.Name == "NatGatewayId" && d.Value != nil && *d.Value == natID {
				ids = append(ids, alarmRes.ID)
				break
			}
		}
	}
	return relatedResultTrunc("alarm", ids, truncated)
}
