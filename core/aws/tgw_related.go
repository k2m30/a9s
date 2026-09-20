// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// tgw_related.go contains related-resource checker functions for Transit Gateways.
package aws

import (
	"context"
	"slices"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkTGWVPC calls ec2:DescribeTransitGatewayVpcAttachments filtered by the
// TGW id and collects the VpcId of each returned attachment (Pattern A —
// direct API call).
func checkTGWVPC(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ec2types.TransitGateway](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("vpc")
	}
	tgwID := res.ID
	if tgwID == "" && raw.TransitGatewayId != nil {
		tgwID = *raw.TransitGatewayId
	}
	if tgwID == "" {
		return resource.ProvenZero("vpc", "tgwID")
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.EC2 == nil {
		return resource.UnknownRelated("vpc")
	}
	api, ok := c.EC2.(EC2DescribeTransitGatewayVpcAttachmentsAPI)
	if !ok {
		return resource.UnknownRelated("vpc")
	}
	atts, complete, err := tgwVpcAttachments(ctx, api, tgwID)
	if err != nil {
		return resource.ErrorRelated("vpc", err)
	}
	var ids []string
	for _, att := range atts {
		if att.VpcId != nil && *att.VpcId != "" {
			ids = append(ids, *att.VpcId)
		}
	}
	return relatedResultTrunc("vpc", ids, !complete)
}

// tgwVpcAttachments walks the VPC attachments of one transit gateway.
func tgwVpcAttachments(ctx context.Context, api EC2DescribeTransitGatewayVpcAttachmentsAPI, tgwID string) ([]ec2types.TransitGatewayVpcAttachment, bool, error) {
	return PageAll(ctx, PerParentPageCap, func(ctx context.Context, token *string) ([]ec2types.TransitGatewayVpcAttachment, *string, error) {
		out, err := api.DescribeTransitGatewayVpcAttachments(ctx, &ec2.DescribeTransitGatewayVpcAttachmentsInput{
			Filters:   []ec2types.Filter{{Name: aws.String("transit-gateway-id"), Values: []string{tgwID}}},
			NextToken: token,
		})
		if err != nil {
			return nil, nil, err
		}
		return out.TransitGatewayVpcAttachments, out.NextToken, nil
	})
}

// checkTGWRTB checks the rtb cache for route tables that have routes
// targeting this transit gateway (Pattern C).
func checkTGWRTB(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	tgwID := res.ID
	if tgwID == "" {
		return resource.ProvenZero("rtb", "tgwID")
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
		rtb, ok := assertStruct[ec2types.RouteTable](rtbRes.RawStruct)
		if !ok {
			continue
		}
		if slices.ContainsFunc(rtb.Routes, func(r ec2types.Route) bool { return routeTargetsGateway(r, tgwID) }) {
			ids = append(ids, rtbRes.ID)
		}
	}
	return relatedResultTrunc("rtb", ids, truncated)
}

// checkTGWRole checks whether the Transit Gateway service-linked role (SLR)
// "AWSServiceRoleForVPCTransitGateway" exists via iam:GetRole.
// Count: 1 with the role if found; Count: 0 if the role does not exist
// (NoSuchEntity); unknown state on unexpected errors.
func checkTGWRole(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	if res.ID == "" {
		return resource.KnownRelated("role", nil, false)
	}

	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.IAM == nil {
		return resource.UnknownRelated("role")
	}
	getRoleAPI, ok := c.IAM.(IAMGetRoleAPI)
	if !ok {
		return resource.UnknownRelated("role")
	}

	const slrName = "AWSServiceRoleForVPCTransitGateway"
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*iam.GetRoleOutput, error) {
		return getRoleAPI.GetRole(ctx, &iam.GetRoleInput{
			RoleName: aws.String(slrName),
		})
	})
	if err != nil {
		if ErrCodeIs(err, "NoSuchEntity") {
			return resource.ProvenZero("role", "the API answered that none is configured")
		}
		return resource.ErrorRelated("role", err)
	}
	if out.Role == nil || out.Role.Arn == nil || *out.Role.Arn == "" {
		return resource.ProvenZero("role", "out.Role.Arn")
	}
	ids, dropped := resolveRefs("role", []string{*out.Role.Arn}, refContext(clients, cache, "role"))
	return heuristicResult("role", ids, dropped)
}

// checkTGWSubnet reports subnets this transit gateway is attached to via VPC
// attachments. Pattern C: one ec2:DescribeTransitGatewayVpcAttachments call
// filtered by the TGW id.
func checkTGWSubnet(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	tgwID := res.ID
	if tgwID == "" {
		return resource.ProvenZero("subnet", "tgwID")
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.EC2 == nil {
		return resource.UnknownRelated("subnet")
	}
	api, ok := c.EC2.(EC2DescribeTransitGatewayVpcAttachmentsAPI)
	if !ok {
		return resource.UnknownRelated("subnet")
	}
	atts, complete, err := tgwVpcAttachments(ctx, api, tgwID)
	if err != nil {
		return resource.ErrorRelated("subnet", err)
	}
	seen := make(map[string]bool)
	var ids []string
	for _, att := range atts {
		for _, sID := range att.SubnetIds {
			if sID == "" || seen[sID] {
				continue
			}
			seen[sID] = true
			ids = append(ids, sID)
		}
	}
	return relatedResultTrunc("subnet", ids, !complete)
}
