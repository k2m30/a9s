// vpc_related.go contains VPC related-resource checker functions.
package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkVPCSubnet searches the subnet cache for subnets whose vpc_id field
// matches this VPC's ID.
func checkVPCSubnet(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	vpcID := vpcIDFromResource(res)
	if vpcID == "" {
		return resource.RelatedCheckResult{TargetType: "subnet", Count: 0}
	}

	list, truncated, err := relatedResourcesFor(ctx, clients, cache, "subnet")
	if err != nil {
		return resource.ErrorRelated("subnet", err)
	}
	if list == nil {
		return resource.UnknownRelated("subnet")
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
		return resource.RelatedCheckResult{TargetType: "sg", Count: 0}
	}

	list, truncated, err := relatedResourcesFor(ctx, clients, cache, "sg")
	if err != nil {
		return resource.ErrorRelated("sg", err)
	}
	if list == nil {
		return resource.UnknownRelated("sg")
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
		return resource.RelatedCheckResult{TargetType: "ec2", Count: 0}
	}

	list, truncated, err := relatedResourcesFor(ctx, clients, cache, "ec2")
	if err != nil {
		return resource.ErrorRelated("ec2", err)
	}
	if list == nil {
		return resource.UnknownRelated("ec2")
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
		return resource.RelatedCheckResult{TargetType: "elb", Count: 0}
	}

	list, truncated, err := relatedResourcesFor(ctx, clients, cache, "elb")
	if err != nil {
		return resource.ErrorRelated("elb", err)
	}
	if list == nil {
		return resource.UnknownRelated("elb")
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
		return resource.RelatedCheckResult{TargetType: "nat", Count: 0}
	}

	list, truncated, err := relatedResourcesFor(ctx, clients, cache, "nat")
	if err != nil {
		return resource.ErrorRelated("nat", err)
	}
	if list == nil {
		return resource.UnknownRelated("nat")
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
		return resource.RelatedCheckResult{TargetType: "igw", Count: 0}
	}

	list, truncated, err := relatedResourcesFor(ctx, clients, cache, "igw")
	if err != nil {
		return resource.ErrorRelated("igw", err)
	}
	if list == nil {
		return resource.UnknownRelated("igw")
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
		return resource.RelatedCheckResult{TargetType: "rtb", Count: 0}
	}

	list, truncated, err := relatedResourcesFor(ctx, clients, cache, "rtb")
	if err != nil {
		return resource.ErrorRelated("rtb", err)
	}
	if list == nil {
		return resource.UnknownRelated("rtb")
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
		return resource.RelatedCheckResult{TargetType: "vpce", Count: 0}
	}

	list, truncated, err := relatedResourcesFor(ctx, clients, cache, "vpce")
	if err != nil {
		return resource.ErrorRelated("vpce", err)
	}
	if list == nil {
		return resource.UnknownRelated("vpce")
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
// No cache access needed — the tag carries the stack name directly.
func checkVPCCFN(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ec2types.Vpc](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("cfn")
	}
	stackName := tagValue(raw.Tags, "aws:cloudformation:stack-name")
	if stackName == "" {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: 0}
	}
	return relatedResult("cfn", []string{stackName})
}

// checkVPCENI searches the eni cache for network interfaces whose vpc_id
// matches this VPC's ID.
func checkVPCENI(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	vpcID := vpcIDFromResource(res)
	if vpcID == "" {
		return resource.RelatedCheckResult{TargetType: "eni", Count: 0}
	}

	list, truncated, err := relatedResourcesFor(ctx, clients, cache, "eni")
	if err != nil {
		return resource.ErrorRelated("eni", err)
	}
	if list == nil {
		return resource.UnknownRelated("eni")
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
		return resource.RelatedCheckResult{TargetType: "tgw", Count: 0}
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.EC2 == nil {
		return resource.UnknownRelated("tgw")
	}
	resIDName := "resource-id"
	resTypeName := "resource-type"
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*ec2.DescribeTransitGatewayAttachmentsOutput, error) {
		return c.EC2.DescribeTransitGatewayAttachments(ctx, &ec2.DescribeTransitGatewayAttachmentsInput{
			Filters: []ec2types.Filter{
				{Name: &resIDName, Values: []string{vpcID}},
				{Name: &resTypeName, Values: []string{"vpc"}},
			},
		})
	})
	if err != nil {
		return resource.ErrorRelated("tgw", err)
	}
	seen := make(map[string]bool)
	var ids []string
	for _, att := range out.TransitGatewayAttachments {
		if att.TransitGatewayId == nil || *att.TransitGatewayId == "" {
			continue
		}
		if seen[*att.TransitGatewayId] {
			continue
		}
		seen[*att.TransitGatewayId] = true
		ids = append(ids, *att.TransitGatewayId)
	}
	return relatedResult("tgw", ids)
}

// vpcIDFromResource extracts the VPC ID from a VPC resource.
// The VPC's own ID is the vpc_id itself.
func vpcIDFromResource(res resource.Resource) string {
	if res.ID != "" {
		return res.ID
	}
	return res.Fields["vpc_id"]
}
