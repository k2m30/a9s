// tgw_related.go contains related-resource checker functions for Transit Gateways.
package aws

import (
	"context"

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
		{
			TargetType:       "role",
			DisplayName:      "IAM Role",
			Checker:          checkTGWRole,
			NeedsTargetCache: false,
		},
		{
			TargetType:       "subnet",
			DisplayName:      "Subnets",
			Checker:          checkTGWSubnet,
			NeedsTargetCache: false,
		},
	})
}

// checkTGWVPC calls ec2:DescribeTransitGatewayVpcAttachments filtered by the
// TGW id and collects the VpcId of each returned attachment (Pattern A —
// direct API call). DevOps consensus (5/5 reviewers) agrees this is the
// canonical API for tgw→vpc.
func checkTGWVPC(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ec2types.TransitGateway](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "vpc", Count: -1}
	}
	tgwID := res.ID
	if tgwID == "" && raw.TransitGatewayId != nil {
		tgwID = *raw.TransitGatewayId
	}
	if tgwID == "" {
		return resource.RelatedCheckResult{TargetType: "vpc", Count: 0}
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.EC2 == nil {
		return resource.RelatedCheckResult{TargetType: "vpc", Count: -1}
	}
	api, ok := c.EC2.(EC2DescribeTransitGatewayVpcAttachmentsAPI)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "vpc", Count: -1}
	}
	filterName := "transit-gateway-id"
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*ec2.DescribeTransitGatewayVpcAttachmentsOutput, error) {
		return api.DescribeTransitGatewayVpcAttachments(ctx, &ec2.DescribeTransitGatewayVpcAttachmentsInput{
			Filters: []ec2types.Filter{
				{Name: &filterName, Values: []string{tgwID}},
			},
		})
	})
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "vpc", Count: -1, Err: err}
	}
	var ids []string
	for _, att := range out.TransitGatewayVpcAttachments {
		if att.VpcId != nil && *att.VpcId != "" {
			ids = append(ids, *att.VpcId)
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

// checkTGWRole reports the IAM role that TGW assumes for cross-account/service
// attachments. Transit Gateways themselves do not carry a service role in
// DescribeTransitGateways output — role-based attachments are per-attachment
// configuration and would require DescribeTransitGatewayAttachments with
// per-attachment resolution — outside the 1-call budget.
// Returns Count: -1 (unknown).
func checkTGWRole(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	if res.ID == "" {
		return resource.RelatedCheckResult{TargetType: "role", Count: 0}
	}
	return resource.RelatedCheckResult{TargetType: "role", Count: -1}
}

// checkTGWSubnet reports subnets this transit gateway is attached to via VPC
// attachments. Pattern C: one ec2:DescribeTransitGatewayVpcAttachments call
// filtered by the TGW id.
func checkTGWSubnet(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	tgwID := res.ID
	if tgwID == "" {
		return resource.RelatedCheckResult{TargetType: "subnet", Count: 0}
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.EC2 == nil {
		return resource.RelatedCheckResult{TargetType: "subnet", Count: -1}
	}
	api, ok := c.EC2.(EC2DescribeTransitGatewayVpcAttachmentsAPI)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "subnet", Count: -1}
	}
	filterName := "transit-gateway-id"
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*ec2.DescribeTransitGatewayVpcAttachmentsOutput, error) {
		return api.DescribeTransitGatewayVpcAttachments(ctx, &ec2.DescribeTransitGatewayVpcAttachmentsInput{
			Filters: []ec2types.Filter{
				{Name: &filterName, Values: []string{tgwID}},
			},
		})
	})
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "subnet", Count: -1, Err: err}
	}
	seen := make(map[string]bool)
	var ids []string
	for _, att := range out.TransitGatewayVpcAttachments {
		for _, sID := range att.SubnetIds {
			if sID == "" || seen[sID] {
				continue
			}
			seen[sID] = true
			ids = append(ids, sID)
		}
	}
	return relatedResult("subnet", ids)
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
