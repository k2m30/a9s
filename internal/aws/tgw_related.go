// tgw_related.go contains related-resource checker functions for Transit Gateways.
package aws

import (
	"context"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterRelated("tgw", []resource.RelatedDef{
		{
			TargetType:       "vpc",
			DisplayName:      "VPCs",
			Checker:          nil,
			NeedsTargetCache: true,
		},
		{
			TargetType:       "rtb",
			DisplayName:      "Route Tables",
			Checker:          checkTGWRTB,
			NeedsTargetCache: true,
		},
		{
			TargetType:       "cfn",
			DisplayName:      "CloudFormation",
			Checker:          nil,
			NeedsTargetCache: true,
		},
	})
}

// checkTGWRTB checks the rtb cache for route tables that have routes
// targeting this transit gateway (Pattern C).
func checkTGWRTB(ctx context.Context, clients interface{}, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	tgwID := res.ID
	if tgwID == "" {
		return resource.RelatedCheckResult{TargetType: "rtb", Count: 0}
	}

	rtbList, truncated, err := tgwRelatedResources(ctx, clients, cache, "rtb")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "rtb", Count: -1, Err: err}
	}
	if rtbList == nil {
		return resource.RelatedCheckResult{TargetType: "rtb", Count: -1}
	}

	var ids []string
	for _, rtbRes := range rtbList {
		rtb, ok := assertStruct[ec2types.RouteTable](rtbRes.RawStruct)
		if !ok {
			continue
		}
		for _, r := range rtb.Routes {
			if r.TransitGatewayId != nil && *r.TransitGatewayId == tgwID {
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

// tgwRelatedResources returns the cached resource list for the given target type,
// or fetches the first page via the registered paginated fetcher.
func tgwRelatedResources(ctx context.Context, clients interface{}, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}
