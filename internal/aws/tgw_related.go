// tgw_related.go contains related-resource checker functions for Transit Gateways.
package aws

import (
	"context"
	"errors"

	smithy "github.com/aws/smithy-go"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"

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

// checkTGWRole checks whether the Transit Gateway service-linked role (SLR)
// "AWSServiceRoleForVPCTransitGateway" exists via iam:GetRole.
// Count: 1 with the role ARN if found; Count: 0 if the role does not exist
// (NoSuchEntity); Count: -1 on unexpected errors.
func checkTGWRole(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	if res.ID == "" {
		return resource.RelatedCheckResult{TargetType: "role", Count: 0}
	}

	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.IAM == nil {
		return resource.RelatedCheckResult{TargetType: "role", Count: -1}
	}
	getRoleAPI, ok := c.IAM.(IAMGetRoleAPI)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "role", Count: -1}
	}

	const slrName = "AWSServiceRoleForVPCTransitGateway"
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*iam.GetRoleOutput, error) {
		return getRoleAPI.GetRole(ctx, &iam.GetRoleInput{
			RoleName: aws.String(slrName),
		})
	})
	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) && apiErr.ErrorCode() == "NoSuchEntity" {
			return resource.RelatedCheckResult{TargetType: "role", Count: 0}
		}
		return resource.RelatedCheckResult{TargetType: "role", Count: -1, Err: err}
	}
	if out.Role == nil || out.Role.Arn == nil || *out.Role.Arn == "" {
		return resource.RelatedCheckResult{TargetType: "role", Count: 0}
	}
	return relatedResult("role", []string{*out.Role.Arn})
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
