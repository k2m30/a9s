// vpc_related.go contains VPC related-resource checker functions.
package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterRelated("vpc", []resource.RelatedDef{
		{TargetType: "subnet", DisplayName: "Subnets", Checker: checkVPCSubnet, NeedsTargetCache: true},
		{TargetType: "sg", DisplayName: "Security Groups", Checker: checkVPCSG, NeedsTargetCache: true},
		{TargetType: "ec2", DisplayName: "EC2 Instances", Checker: checkVPCEC2, NeedsTargetCache: true},
		{TargetType: "elb", DisplayName: "Load Balancers", Checker: checkVPCELB, NeedsTargetCache: true},
		{TargetType: "nat", DisplayName: "NAT Gateways", Checker: checkVPCNAT, NeedsTargetCache: true},
		{TargetType: "igw", DisplayName: "Internet Gateways", Checker: checkVPCIGW, NeedsTargetCache: true},
		{TargetType: "rtb", DisplayName: "Route Tables", Checker: checkVPCRTB, NeedsTargetCache: true},
		{TargetType: "vpce", DisplayName: "VPC Endpoints", Checker: checkVPCVPCE, NeedsTargetCache: true},
		{TargetType: "cfn", DisplayName: "CloudFormation", Checker: checkVPCCFN, NeedsTargetCache: false},
		{TargetType: "eni", DisplayName: "Network Interfaces", Checker: checkVPCENI, NeedsTargetCache: true},
		{TargetType: "tgw", DisplayName: "Transit Gateways", Checker: checkVPCTGW, NeedsTargetCache: false},
	})
}

// checkVPCSubnet searches the subnet cache for subnets whose vpc_id field
// matches this VPC's ID.
func checkVPCSubnet(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	vpcID := vpcIDFromResource(res)
	if vpcID == "" {
		return resource.RelatedCheckResult{TargetType: "subnet", Count: 0}
	}

	list, truncated, err := vpcRelatedResources(ctx, clients, cache, "subnet")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "subnet", Count: -1, Err: err}
	}
	if list == nil {
		return resource.RelatedCheckResult{TargetType: "subnet", Count: -1}
	}

	var ids []string
	for _, r := range list {
		if r.Fields["vpc_id"] == vpcID {
			ids = append(ids, r.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("subnet")
	}
	return relatedResult("subnet", ids)
}

// checkVPCSG searches the sg cache for security groups whose vpc_id field
// matches this VPC's ID.
func checkVPCSG(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	vpcID := vpcIDFromResource(res)
	if vpcID == "" {
		return resource.RelatedCheckResult{TargetType: "sg", Count: 0}
	}

	list, truncated, err := vpcRelatedResources(ctx, clients, cache, "sg")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "sg", Count: -1, Err: err}
	}
	if list == nil {
		return resource.RelatedCheckResult{TargetType: "sg", Count: -1}
	}

	var ids []string
	for _, r := range list {
		if r.Fields["vpc_id"] == vpcID {
			ids = append(ids, r.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("sg")
	}
	return relatedResult("sg", ids)
}

// checkVPCEC2 searches the ec2 cache for instances whose vpc_id field
// matches this VPC's ID.
func checkVPCEC2(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	vpcID := vpcIDFromResource(res)
	if vpcID == "" {
		return resource.RelatedCheckResult{TargetType: "ec2", Count: 0}
	}

	list, truncated, err := vpcRelatedResources(ctx, clients, cache, "ec2")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "ec2", Count: -1, Err: err}
	}
	if list == nil {
		return resource.RelatedCheckResult{TargetType: "ec2", Count: -1}
	}

	var ids []string
	for _, r := range list {
		if r.Fields["vpc_id"] == vpcID {
			ids = append(ids, r.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("ec2")
	}
	return relatedResult("ec2", ids)
}

// checkVPCELB searches the elb cache for load balancers whose vpc_id field
// or RawStruct VpcId matches this VPC's ID.
func checkVPCELB(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	vpcID := vpcIDFromResource(res)
	if vpcID == "" {
		return resource.RelatedCheckResult{TargetType: "elb", Count: 0}
	}

	list, truncated, err := vpcRelatedResources(ctx, clients, cache, "elb")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "elb", Count: -1, Err: err}
	}
	if list == nil {
		return resource.RelatedCheckResult{TargetType: "elb", Count: -1}
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
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("elb")
	}
	return relatedResult("elb", ids)
}

// checkVPCNAT searches the nat cache for NAT gateways whose vpc_id field
// matches this VPC's ID.
func checkVPCNAT(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	vpcID := vpcIDFromResource(res)
	if vpcID == "" {
		return resource.RelatedCheckResult{TargetType: "nat", Count: 0}
	}

	list, truncated, err := vpcRelatedResources(ctx, clients, cache, "nat")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "nat", Count: -1, Err: err}
	}
	if list == nil {
		return resource.RelatedCheckResult{TargetType: "nat", Count: -1}
	}

	var ids []string
	for _, r := range list {
		if r.Fields["vpc_id"] == vpcID {
			ids = append(ids, r.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("nat")
	}
	return relatedResult("nat", ids)
}

// checkVPCIGW searches the igw cache for internet gateways whose vpc_id field
// matches this VPC's ID.
func checkVPCIGW(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	vpcID := vpcIDFromResource(res)
	if vpcID == "" {
		return resource.RelatedCheckResult{TargetType: "igw", Count: 0}
	}

	list, truncated, err := vpcRelatedResources(ctx, clients, cache, "igw")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "igw", Count: -1, Err: err}
	}
	if list == nil {
		return resource.RelatedCheckResult{TargetType: "igw", Count: -1}
	}

	var ids []string
	for _, r := range list {
		if r.Fields["vpc_id"] == vpcID {
			ids = append(ids, r.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("igw")
	}
	return relatedResult("igw", ids)
}

// checkVPCRTB searches the rtb cache for route tables whose vpc_id field
// matches this VPC's ID.
func checkVPCRTB(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	vpcID := vpcIDFromResource(res)
	if vpcID == "" {
		return resource.RelatedCheckResult{TargetType: "rtb", Count: 0}
	}

	list, truncated, err := vpcRelatedResources(ctx, clients, cache, "rtb")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "rtb", Count: -1, Err: err}
	}
	if list == nil {
		return resource.RelatedCheckResult{TargetType: "rtb", Count: -1}
	}

	var ids []string
	for _, r := range list {
		if r.Fields["vpc_id"] == vpcID {
			ids = append(ids, r.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("rtb")
	}
	return relatedResult("rtb", ids)
}

// checkVPCVPCE searches the vpce cache for VPC endpoints whose vpc_id field
// matches this VPC's ID.
func checkVPCVPCE(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	vpcID := vpcIDFromResource(res)
	if vpcID == "" {
		return resource.RelatedCheckResult{TargetType: "vpce", Count: 0}
	}

	list, truncated, err := vpcRelatedResources(ctx, clients, cache, "vpce")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "vpce", Count: -1, Err: err}
	}
	if list == nil {
		return resource.RelatedCheckResult{TargetType: "vpce", Count: -1}
	}

	var ids []string
	for _, r := range list {
		if r.Fields["vpc_id"] == vpcID {
			ids = append(ids, r.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("vpce")
	}
	return relatedResult("vpce", ids)
}

// checkVPCCFN checks the VPC's tags for aws:cloudformation:stack-name.
// No cache access needed — the tag carries the stack name directly.
func checkVPCCFN(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ec2types.Vpc](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1}
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

	list, truncated, err := vpcRelatedResources(ctx, clients, cache, "eni")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "eni", Count: -1, Err: err}
	}
	if list == nil {
		return resource.RelatedCheckResult{TargetType: "eni", Count: -1}
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
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("eni")
	}
	return relatedResult("eni", ids)
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
		return resource.RelatedCheckResult{TargetType: "tgw", Count: -1}
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
		return resource.RelatedCheckResult{TargetType: "tgw", Count: -1, Err: err}
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

// vpcRelatedResources returns the resource list for target from cache or fetches
// the first page via the registered paginated fetcher.
func vpcRelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}


