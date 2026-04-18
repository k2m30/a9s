// igw_related.go contains Internet Gateway related-resource checker functions.
package aws

import (
	"context"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkIGWVPC extracts Attachments[0].VpcId from the IGW RawStruct and searches
// the vpc cache for a matching resource (Pattern F + C hybrid: field extraction
// from self, then cache lookup).
func checkIGWVPC(_ context.Context, _ any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ec2types.InternetGateway](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "vpc", Count: 0}
	}
	if len(raw.Attachments) == 0 || raw.Attachments[0].VpcId == nil || *raw.Attachments[0].VpcId == "" {
		return resource.RelatedCheckResult{TargetType: "vpc", Count: 0}
	}
	vpcID := *raw.Attachments[0].VpcId

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

// checkIGWRTB searches the rtb cache for route tables that contain a route
// with a GatewayId matching this IGW's ID (Pattern C — search target cache).
func checkIGWRTB(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	igwID := res.ID
	raw, ok := assertStruct[ec2types.InternetGateway](res.RawStruct)
	if ok && raw.InternetGatewayId != nil && *raw.InternetGatewayId != "" {
		igwID = *raw.InternetGatewayId
	}
	if igwID == "" {
		return resource.RelatedCheckResult{TargetType: "rtb", Count: 0}
	}

	rtbList, truncated, err := igwRelatedResources(ctx, clients, cache, "rtb")
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
			if route.GatewayId != nil && *route.GatewayId == igwID {
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

// igwRelatedResources returns the resource list for target from cache or fetches
// the first page via the registered paginated fetcher.
func igwRelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}
