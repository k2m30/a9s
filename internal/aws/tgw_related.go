// tgw_related.go contains related-resource checker functions for Transit Gateways.
package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterRelated("tgw", []resource.RelatedDef{
		{
			TargetType:       "vpc",
			DisplayName:      "VPCs",
			Checker:          checkTGWVPC,
			NeedsTargetCache: false,
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
			Checker:          checkTGWCFN,
			NeedsTargetCache: false,
		},
		{TargetType: "role", DisplayName: "IAM Role", Checker: checkTGWRole},
		{TargetType: "subnet", DisplayName: "Subnets", Checker: checkTGWSubnet},
	})
}

// checkTGWVPC calls ec2:DescribeTransitGatewayAttachments to find VPCs attached to
// this transit gateway (Pattern A — direct API call).
func checkTGWVPC(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	tgwID := res.ID
	if tgwID == "" {
		return resource.RelatedCheckResult{TargetType: "vpc", Count: 0}
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil {
		return resource.RelatedCheckResult{TargetType: "vpc", Count: -1}
	}
	resourceType := "vpc"
	out, err := c.EC2.DescribeTransitGatewayAttachments(ctx, &ec2.DescribeTransitGatewayAttachmentsInput{
		Filters: []ec2types.Filter{
			{Name: aws.String("transit-gateway-id"), Values: []string{tgwID}},
			{Name: aws.String("resource-type"), Values: []string{resourceType}},
		},
	})
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "vpc", Count: -1, Err: err}
	}
	var ids []string
	for _, att := range out.TransitGatewayAttachments {
		if att.ResourceId != nil {
			ids = append(ids, *att.ResourceId)
		}
	}
	return relatedResult("vpc", ids)
}

// checkTGWCFN checks the transit gateway's tags for aws:cloudformation:stack-name.
// No cache access needed — the tag carries the stack name directly.
func checkTGWCFN(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ec2types.TransitGateway](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1}
	}
	stackName := tagValue(raw.Tags, "aws:cloudformation:stack-name")
	if stackName == "" {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: 0}
	}
	return relatedResult("cfn", []string{stackName})
}

// checkTGWRTB checks the rtb cache for route tables that have routes
// targeting this transit gateway (Pattern C).
func checkTGWRTB(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
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

// checkTGWRole returns Count: 0 because Transit Gateways do not expose an IAM role
// ARN in the DescribeTransitGateways response.
func checkTGWRole(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "role", Count: 0}
}

// tgwRelatedResources returns the cached resource list for the given target type,
// or fetches the first page via the registered paginated fetcher.
func tgwRelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}

func checkTGWSubnet(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "subnet", Count: 0}
}
