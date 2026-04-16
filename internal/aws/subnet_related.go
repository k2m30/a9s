// subnet_related.go contains Subnet related-resource checker functions.
package aws

import (
	"context"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterNavigableFields("subnet", []resource.NavigableField{
		{FieldPath: "VpcId", TargetType: "vpc"},
	})

	resource.RegisterRelated("subnet", []resource.RelatedDef{
		{TargetType: "ec2", DisplayName: "EC2 Instances", Checker: checkSubnetEC2, NeedsTargetCache: true},
		{TargetType: "eni", DisplayName: "Network Interfaces", Checker: checkSubnetENI, NeedsTargetCache: true},
		{TargetType: "nat", DisplayName: "NAT Gateways", Checker: checkSubnetNAT, NeedsTargetCache: true},
		{TargetType: "elb", DisplayName: "Load Balancers", Checker: checkSubnetELB, NeedsTargetCache: true},
		{TargetType: "rtb", DisplayName: "Route Tables", Checker: checkSubnetRTB, NeedsTargetCache: true},
		{TargetType: "cfn", DisplayName: "CloudFormation", Checker: checkSubnetCFN, NeedsTargetCache: false},
		{TargetType: "vpc", DisplayName: "VPC", Checker: checkSubnetVPC},
		{TargetType: "asg", DisplayName: "Auto Scaling Groups", Checker: checkSubnetASG},
		{TargetType: "efs", DisplayName: "EFS File Systems", Checker: checkSubnetEFS},
		{TargetType: "eks", DisplayName: "EKS Clusters", Checker: checkSubnetEKS},
		{TargetType: "vpce", DisplayName: "VPC Endpoints", Checker: checkSubnetVPCE},
	})
}

// checkSubnetEC2 searches the ec2 cache for instances whose SubnetId matches
// the subnet's ID. Uses assertStruct since subnet_id is not in EC2 Fields map.
func checkSubnetEC2(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	subnetID := res.ID
	if subnetID == "" {
		return resource.RelatedCheckResult{TargetType: "ec2", Count: 0}
	}

	ec2List, truncated, err := subnetRelatedResources(ctx, clients, cache, "ec2")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "ec2", Count: -1, Err: err}
	}
	if ec2List == nil {
		return resource.RelatedCheckResult{TargetType: "ec2", Count: -1}
	}

	var ids []string
	for _, ec2Res := range ec2List {
		if ec2Res.Fields["subnet_id"] == subnetID {
			ids = append(ids, ec2Res.ID)
			continue
		}
		inst, ok := assertStruct[ec2types.Instance](ec2Res.RawStruct)
		if ok && inst.SubnetId != nil && *inst.SubnetId == subnetID {
			ids = append(ids, ec2Res.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "ec2", Count: -1}
	}
	return relatedResult("ec2", ids)
}

// checkSubnetENI searches the eni cache for network interfaces whose SubnetId
// matches the subnet's ID. Uses assertStruct since subnet_id is not in ENI Fields map.
func checkSubnetENI(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	subnetID := res.ID
	if subnetID == "" {
		return resource.RelatedCheckResult{TargetType: "eni", Count: 0}
	}

	eniList, truncated, err := subnetRelatedResources(ctx, clients, cache, "eni")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "eni", Count: -1, Err: err}
	}
	if eniList == nil {
		return resource.RelatedCheckResult{TargetType: "eni", Count: -1}
	}

	var ids []string
	for _, eniRes := range eniList {
		if eniRes.Fields["subnet_id"] == subnetID {
			ids = append(ids, eniRes.ID)
			continue
		}
		raw, ok := assertStruct[ec2types.NetworkInterface](eniRes.RawStruct)
		if ok && raw.SubnetId != nil && *raw.SubnetId == subnetID {
			ids = append(ids, eniRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "eni", Count: -1}
	}
	return relatedResult("eni", ids)
}

// checkSubnetNAT searches the nat cache for NAT gateways whose subnet_id field
// matches the subnet's ID.
func checkSubnetNAT(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	subnetID := res.ID
	if subnetID == "" {
		return resource.RelatedCheckResult{TargetType: "nat", Count: 0}
	}

	natList, truncated, err := subnetRelatedResources(ctx, clients, cache, "nat")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "nat", Count: -1, Err: err}
	}
	if natList == nil {
		return resource.RelatedCheckResult{TargetType: "nat", Count: -1}
	}

	var ids []string
	for _, natRes := range natList {
		if natRes.Fields["subnet_id"] == subnetID {
			ids = append(ids, natRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "nat", Count: -1}
	}
	return relatedResult("nat", ids)
}

// checkSubnetELB searches the elb cache for load balancers whose AvailabilityZones
// include a reference to the subnet's ID.
func checkSubnetELB(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	subnetID := res.ID
	if subnetID == "" {
		return resource.RelatedCheckResult{TargetType: "elb", Count: 0}
	}

	elbList, truncated, err := subnetRelatedResources(ctx, clients, cache, "elb")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "elb", Count: -1, Err: err}
	}
	if elbList == nil {
		return resource.RelatedCheckResult{TargetType: "elb", Count: -1}
	}

	var ids []string
	for _, elbRes := range elbList {
		lb, ok := assertStruct[elbv2types.LoadBalancer](elbRes.RawStruct)
		if !ok {
			continue
		}
		for _, az := range lb.AvailabilityZones {
			if az.SubnetId != nil && *az.SubnetId == subnetID {
				ids = append(ids, elbRes.ID)
				break
			}
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "elb", Count: -1}
	}
	return relatedResult("elb", ids)
}

// checkSubnetRTB searches the rtb cache for route tables associated with this
// subnet either explicitly (via SubnetId in Associations) or implicitly via the
// main route table for the subnet's VPC.
func checkSubnetRTB(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	subnetID := res.ID
	vpcID := res.Fields["vpc_id"]
	if subnetID == "" {
		return resource.RelatedCheckResult{TargetType: "rtb", Count: 0}
	}

	rtbList, truncated, err := subnetRelatedResources(ctx, clients, cache, "rtb")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "rtb", Count: -1, Err: err}
	}
	if rtbList == nil {
		return resource.RelatedCheckResult{TargetType: "rtb", Count: -1}
	}

	var ids []string
	hasExplicit := false
	var mainRTBID string
	for _, rtbRes := range rtbList {
		raw, ok := assertStruct[ec2types.RouteTable](rtbRes.RawStruct)
		if !ok {
			continue
		}
		rtbVpcID := ""
		if raw.VpcId != nil {
			rtbVpcID = *raw.VpcId
		}
		for _, assoc := range raw.Associations {
			if assoc.SubnetId != nil && *assoc.SubnetId == subnetID {
				ids = append(ids, rtbRes.ID)
				hasExplicit = true
			}
			if assoc.Main != nil && *assoc.Main && rtbVpcID == vpcID {
				mainRTBID = rtbRes.ID
			}
		}
	}
	// If no explicit association, the main route table for the VPC applies.
	if !hasExplicit && mainRTBID != "" {
		ids = append(ids, mainRTBID)
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "rtb", Count: -1}
	}
	return relatedResult("rtb", ids)
}

// checkSubnetCFN checks the subnet's tags for aws:cloudformation:stack-name.
// No cache access needed — the tag carries the stack name directly.
func checkSubnetCFN(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ec2types.Subnet](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1}
	}
	stackName := tagValue(raw.Tags, "aws:cloudformation:stack-name")
	if stackName == "" {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: 0}
	}
	return relatedResult("cfn", []string{stackName})
}

// checkSubnetVPC returns the VPC this subnet belongs to (Pattern F).
// Reads vpc_id from Fields which is populated by the subnet fetcher.
func checkSubnetVPC(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	vpcID := res.Fields["vpc_id"]
	if vpcID == "" {
		return resource.RelatedCheckResult{TargetType: "vpc", Count: 0}
	}
	return relatedResult("vpc", []string{vpcID})
}

// subnetRelatedResources returns the resource list for target from cache or fetches
// the first page via the registered paginated fetcher.
func subnetRelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}

func checkSubnetASG(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "asg", Count: 0}
}

func checkSubnetEFS(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "efs", Count: 0}
}

func checkSubnetEKS(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "eks", Count: 0}
}

func checkSubnetVPCE(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "vpce", Count: 0}
}
