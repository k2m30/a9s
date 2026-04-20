// nat_related.go contains NAT Gateway related-resource checker functions.
package aws

import (
	"context"

	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkNATVPC extracts VpcId from the NAT Gateway RawStruct and searches the
// vpc cache for a matching resource (Pattern F + C hybrid).
func checkNATVPC(_ context.Context, _ any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ec2types.NatGateway](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "vpc", Count: 0}
	}
	if raw.VpcId == nil || *raw.VpcId == "" {
		return resource.RelatedCheckResult{TargetType: "vpc", Count: 0}
	}
	vpcID := *raw.VpcId

	entry, hasEntry := cache["vpc"]
	if !hasEntry {
		return resource.RelatedCheckResult{TargetType: "vpc", Count: -1}
	}
	for _, vpcRes := range entry.Resources {
		if vpcRes.ID == vpcID {
			return relatedResult("vpc", []string{vpcID})
		}
	}
	if entry.IsTruncated {
		return resource.ApproximateZero("vpc")
	}
	return resource.RelatedCheckResult{TargetType: "vpc", Count: 0}
}

// checkNATSubnet extracts SubnetId from the NAT Gateway RawStruct and searches
// the subnet cache for a matching resource (Pattern F + C hybrid).
func checkNATSubnet(_ context.Context, _ any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ec2types.NatGateway](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "subnet", Count: 0}
	}
	if raw.SubnetId == nil || *raw.SubnetId == "" {
		return resource.RelatedCheckResult{TargetType: "subnet", Count: 0}
	}
	subnetID := *raw.SubnetId

	entry, hasEntry := cache["subnet"]
	if !hasEntry {
		return resource.RelatedCheckResult{TargetType: "subnet", Count: -1}
	}
	for _, subnetRes := range entry.Resources {
		if subnetRes.ID == subnetID {
			return relatedResult("subnet", []string{subnetID})
		}
	}
	if entry.IsTruncated {
		return resource.ApproximateZero("subnet")
	}
	return resource.RelatedCheckResult{TargetType: "subnet", Count: 0}
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

	rtbList, truncated, err := natRelatedResources(ctx, clients, cache, "rtb")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "rtb", Count: -1, Err: err}
	}
	if rtbList == nil {
		return resource.RelatedCheckResult{TargetType: "rtb", Count: -1}
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
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("rtb")
	}
	return relatedResult("rtb", ids)
}

// checkNATEIP extracts AllocationId values from the NAT gateway's
// NatGatewayAddresses slice and searches the eip cache for matching EIPs.
func checkNATEIP(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ec2types.NatGateway](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "eip", Count: 0}
	}
	allocIDs := make(map[string]struct{})
	for _, addr := range raw.NatGatewayAddresses {
		if addr.AllocationId != nil && *addr.AllocationId != "" {
			allocIDs[*addr.AllocationId] = struct{}{}
		}
	}
	if len(allocIDs) == 0 {
		return resource.RelatedCheckResult{TargetType: "eip", Count: 0}
	}

	eipList, truncated, err := natRelatedResources(ctx, clients, cache, "eip")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "eip", Count: -1, Err: err}
	}
	if eipList == nil {
		return resource.RelatedCheckResult{TargetType: "eip", Count: -1}
	}
	var ids []string
	for _, eipRes := range eipList {
		if _, found := allocIDs[eipRes.ID]; found {
			ids = append(ids, eipRes.ID)
			continue
		}
		eipRaw, eipOk := assertStruct[ec2types.Address](eipRes.RawStruct)
		if eipOk && eipRaw.AllocationId != nil {
			if _, found := allocIDs[*eipRaw.AllocationId]; found {
				ids = append(ids, eipRes.ID)
			}
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("eip")
	}
	return relatedResult("eip", ids)
}

// checkNATENI extracts NetworkInterfaceId values from the NAT gateway's
// NatGatewayAddresses slice and searches the eni cache for matching interfaces.
func checkNATENI(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ec2types.NatGateway](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "eni", Count: 0}
	}
	eniIDs := make(map[string]struct{})
	for _, addr := range raw.NatGatewayAddresses {
		if addr.NetworkInterfaceId != nil && *addr.NetworkInterfaceId != "" {
			eniIDs[*addr.NetworkInterfaceId] = struct{}{}
		}
	}
	if len(eniIDs) == 0 {
		return resource.RelatedCheckResult{TargetType: "eni", Count: 0}
	}

	eniList, truncated, err := natRelatedResources(ctx, clients, cache, "eni")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "eni", Count: -1, Err: err}
	}
	if eniList == nil {
		return resource.RelatedCheckResult{TargetType: "eni", Count: -1}
	}
	var ids []string
	for _, eniRes := range eniList {
		if _, found := eniIDs[eniRes.ID]; found {
			ids = append(ids, eniRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("eni")
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

	alarmList, truncated, err := natRelatedResources(ctx, clients, cache, "alarm")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1, Err: err}
	}
	if alarmList == nil {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1}
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
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("alarm")
	}
	return relatedResult("alarm", ids)
}

// natRelatedResources returns the resource list for target from cache or fetches
// the first page via the registered paginated fetcher.
func natRelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}
