// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// vpc_related.go contains VPC related-resource checker functions.
package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkVPCSubnet searches the subnet cache for subnets whose vpc_id field
// matches this VPC's ID.
func checkVPCSubnet(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	vpcID := vpcIDFromResource(res)
	if vpcID == "" {
		return foundNone("subnet", "vpcID")
	}

	list, truncated, err := relatedResourcesFor(ctx, clients, cache, "subnet")
	if err != nil {
		return ReadFailed("subnet", err)
	}
	if list == nil {
		return NotRead("subnet")
	}

	var ids []string
	for _, r := range list {
		if r.Fields["vpc_id"] == vpcID {
			ids = append(ids, r.ID)
		}
	}
	return relatedResultTrunc("subnet", ids, truncated)
}

// checkVPCSG searches the sg cache for security groups whose vpc_id field
// matches this VPC's ID.
func checkVPCSG(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	vpcID := vpcIDFromResource(res)
	if vpcID == "" {
		return foundNone("sg", "vpcID")
	}

	list, truncated, err := relatedResourcesFor(ctx, clients, cache, "sg")
	if err != nil {
		return ReadFailed("sg", err)
	}
	if list == nil {
		return NotRead("sg")
	}

	var ids []string
	for _, r := range list {
		if r.Fields["vpc_id"] == vpcID {
			ids = append(ids, r.ID)
		}
	}
	return relatedResultTrunc("sg", ids, truncated)
}

// checkVPCEC2 searches the ec2 cache for instances whose vpc_id field
// matches this VPC's ID.
func checkVPCEC2(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	vpcID := vpcIDFromResource(res)
	if vpcID == "" {
		return foundNone("ec2", "vpcID")
	}

	list, truncated, err := relatedResourcesFor(ctx, clients, cache, "ec2")
	if err != nil {
		return ReadFailed("ec2", err)
	}
	if list == nil {
		return NotRead("ec2")
	}

	var ids []string
	for _, r := range list {
		if r.Fields["vpc_id"] == vpcID {
			ids = append(ids, r.ID)
		}
	}
	return relatedResultTrunc("ec2", ids, truncated)
}

// checkVPCELB searches the elb cache for load balancers whose vpc_id field
// or RawStruct VpcId matches this VPC's ID.
func checkVPCELB(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	vpcID := vpcIDFromResource(res)
	if vpcID == "" {
		return foundNone("elb", "vpcID")
	}

	list, truncated, err := relatedResourcesFor(ctx, clients, cache, "elb")
	if err != nil {
		return ReadFailed("elb", err)
	}
	if list == nil {
		return NotRead("elb")
	}

	var ids []string
	for _, r := range list {
		if r.Fields["vpc_id"] == vpcID {
			ids = append(ids, r.ID)
			continue
		}
		lb, ok := assertStruct[elbv2types.LoadBalancer](r.RawStruct)
		if ok && lb.VpcId != nil && *lb.VpcId == vpcID {
			ids = append(ids, r.ID)
		}
	}
	return relatedResultTrunc("elb", ids, truncated)
}

// checkVPCNAT searches the nat cache for NAT gateways whose vpc_id field
// matches this VPC's ID.
func checkVPCNAT(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	vpcID := vpcIDFromResource(res)
	if vpcID == "" {
		return foundNone("nat", "vpcID")
	}

	list, truncated, err := relatedResourcesFor(ctx, clients, cache, "nat")
	if err != nil {
		return ReadFailed("nat", err)
	}
	if list == nil {
		return NotRead("nat")
	}

	var ids []string
	for _, r := range list {
		if r.Fields["vpc_id"] == vpcID {
			ids = append(ids, r.ID)
		}
	}
	return relatedResultTrunc("nat", ids, truncated)
}

// checkVPCIGW searches the igw cache for internet gateways whose vpc_id field
// matches this VPC's ID.
func checkVPCIGW(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	vpcID := vpcIDFromResource(res)
	if vpcID == "" {
		return foundNone("igw", "vpcID")
	}

	list, truncated, err := relatedResourcesFor(ctx, clients, cache, "igw")
	if err != nil {
		return ReadFailed("igw", err)
	}
	if list == nil {
		return NotRead("igw")
	}

	var ids []string
	for _, r := range list {
		if r.Fields["vpc_id"] == vpcID {
			ids = append(ids, r.ID)
		}
	}
	return relatedResultTrunc("igw", ids, truncated)
}

// checkVPCRTB searches the rtb cache for route tables whose vpc_id field
// matches this VPC's ID.
func checkVPCRTB(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	vpcID := vpcIDFromResource(res)
	if vpcID == "" {
		return foundNone("rtb", "vpcID")
	}

	list, truncated, err := relatedResourcesFor(ctx, clients, cache, "rtb")
	if err != nil {
		return ReadFailed("rtb", err)
	}
	if list == nil {
		return NotRead("rtb")
	}

	var ids []string
	for _, r := range list {
		if r.Fields["vpc_id"] == vpcID {
			ids = append(ids, r.ID)
		}
	}
	return relatedResultTrunc("rtb", ids, truncated)
}

// checkVPCVPCE searches the vpce cache for VPC endpoints whose vpc_id field
// matches this VPC's ID.
func checkVPCVPCE(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	vpcID := vpcIDFromResource(res)
	if vpcID == "" {
		return foundNone("vpce", "vpcID")
	}

	list, truncated, err := relatedResourcesFor(ctx, clients, cache, "vpce")
	if err != nil {
		return ReadFailed("vpce", err)
	}
	if list == nil {
		return NotRead("vpce")
	}

	var ids []string
	for _, r := range list {
		if r.Fields["vpc_id"] == vpcID {
			ids = append(ids, r.ID)
		}
	}
	return relatedResultTrunc("vpce", ids, truncated)
}

// checkVPCCFN checks the VPC's tags for aws:cloudformation:stack-name.
func checkVPCCFN(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ec2types.Vpc](res.RawStruct)
	if !ok {
		return NotRead("cfn")
	}
	stackName := tagValue(raw.Tags, "aws:cloudformation:stack-name")
	if stackName == "" {
		return foundNone("cfn", "stackName")
	}
	return relatedResultTrunc("cfn", []string{stackName}, false)
}

// checkVPCENI searches the eni cache for network interfaces whose vpc_id
// matches this VPC's ID.
func checkVPCENI(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	vpcID := vpcIDFromResource(res)
	if vpcID == "" {
		return foundNone("eni", "vpcID")
	}

	list, truncated, err := relatedResourcesFor(ctx, clients, cache, "eni")
	if err != nil {
		return ReadFailed("eni", err)
	}
	if list == nil {
		return NotRead("eni")
	}
	var ids []string
	for _, r := range list {
		if r.Fields["vpc_id"] == vpcID {
			ids = append(ids, r.ID)
			continue
		}
		eni, ok := assertStruct[ec2types.NetworkInterface](r.RawStruct)
		if ok && eni.VpcId != nil && *eni.VpcId == vpcID {
			ids = append(ids, r.ID)
		}
	}
	return relatedResultTrunc("eni", ids, truncated)
}

// checkVPCTGW reports transit gateways attached to this VPC.
// Pattern C: one ec2:DescribeTransitGatewayAttachments call filtered by the
// VPC resource-id; deduplicate TransitGatewayId across the attachments.
func checkVPCTGW(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	vpcID := vpcIDFromResource(res)
	if vpcID == "" {
		return foundNone("tgw", "vpcID")
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.EC2 == nil {
		return NotRead("tgw")
	}
	atts, complete, err := PageAll(ctx, PerParentPageCap, func(ctx context.Context, token *string) ([]ec2types.TransitGatewayAttachment, *string, error) {
		out, err := c.EC2.DescribeTransitGatewayAttachments(ctx, &ec2.DescribeTransitGatewayAttachmentsInput{
			Filters: []ec2types.Filter{
				{Name: aws.String("resource-id"), Values: []string{vpcID}},
				{Name: aws.String("resource-type"), Values: []string{"vpc"}},
			},
			NextToken: token,
		})
		if err != nil {
			return nil, nil, err
		}
		return out.TransitGatewayAttachments, out.NextToken, nil
	})
	if err != nil {
		return ReadFailed("tgw", err)
	}
	var ids []string
	for _, att := range atts {
		ids = append(ids, aws.ToString(att.TransitGatewayId))
	}
	return relatedResultTrunc("tgw", ids, !complete)
}

// vpcIDFromResource extracts the VPC ID from a VPC resource.
// The VPC's own ID is the vpc_id itself.
func vpcIDFromResource(res resource.Resource) string {
	if res.ID != "" {
		return res.ID
	}
	return res.Fields["vpc_id"]
}
