// nat_related.go contains NAT Gateway related-resource checker functions.
package aws

import (
	"context"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkNATVPC extracts VpcId from the NAT Gateway RawStruct and searches the
// vpc cache for a matching resource (Pattern F + C hybrid).
func checkNATVPC(_ context.Context, _ any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ec2types.NatGateway](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "vpc", Count: -1}
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
		return resource.RelatedCheckResult{TargetType: "vpc", Count: -1}
	}
	return resource.RelatedCheckResult{TargetType: "vpc", Count: 0}
}

// checkNATSubnet extracts SubnetId from the NAT Gateway RawStruct and searches
// the subnet cache for a matching resource (Pattern F + C hybrid).
func checkNATSubnet(_ context.Context, _ any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ec2types.NatGateway](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "subnet", Count: -1}
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
		return resource.RelatedCheckResult{TargetType: "subnet", Count: -1}
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
		return resource.RelatedCheckResult{TargetType: "rtb", Count: -1}
	}
	return relatedResult("rtb", ids)
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
