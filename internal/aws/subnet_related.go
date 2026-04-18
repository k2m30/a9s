// subnet_related.go contains Subnet related-resource checker functions.
package aws

import (
	"context"
	"slices"
	"strings"

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
		{TargetType: "asg", DisplayName: "Auto Scaling Groups", Checker: checkSubnetASG, NeedsTargetCache: true},
		{TargetType: "efs", DisplayName: "EFS File Systems", Checker: checkSubnetEFS},
		{TargetType: "eks", DisplayName: "EKS Clusters", Checker: checkSubnetEKS, NeedsTargetCache: true},
		{TargetType: "vpce", DisplayName: "VPC Endpoints", Checker: checkSubnetVPCE, NeedsTargetCache: true},
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

// checkSubnetASG scans the ASG cache for groups whose VPCZoneIdentifier
// references this subnet (comma-separated subnet ids).
func checkSubnetASG(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	subnetID := res.ID
	if subnetID == "" {
		return resource.RelatedCheckResult{TargetType: "asg", Count: 0}
	}

	asgList, truncated, err := subnetRelatedResources(ctx, clients, cache, "asg")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "asg", Count: -1, Err: err}
	}
	if asgList == nil {
		return resource.RelatedCheckResult{TargetType: "asg", Count: -1}
	}

	var ids []string
	for _, asgRes := range asgList {
		zones := asgRes.Fields["vpc_zone_identifier"]
		if zones == "" {
			zones = asgRes.Fields["subnets"]
		}
		if zones == "" {
			continue
		}
		if slices.Contains(splitCSV(zones), subnetID) {
			ids = append(ids, asgRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "asg", Count: -1}
	}
	return relatedResult("asg", ids)
}

// checkSubnetEFS reports EFS file systems mounted into this subnet.
// EFS mount targets are listed per-file-system by DescribeMountTargets; the
// EFS list cache carries only FileSystemDescription which lacks mount targets.
// Determining the relationship requires DescribeMountTargets per file system —
// outside the 1-call budget. Returns Count: -1.
func checkSubnetEFS(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	if res.ID == "" {
		return resource.RelatedCheckResult{TargetType: "efs", Count: 0}
	}
	return resource.RelatedCheckResult{TargetType: "efs", Count: -1}
}

// checkSubnetEKS reports EKS clusters whose VpcConfig.SubnetIds includes
// this subnet. Scans the eks cache looking at the subnets field.
func checkSubnetEKS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	subnetID := res.ID
	if subnetID == "" {
		return resource.RelatedCheckResult{TargetType: "eks", Count: 0}
	}

	eksList, truncated, err := subnetRelatedResources(ctx, clients, cache, "eks")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "eks", Count: -1, Err: err}
	}
	if eksList == nil {
		return resource.RelatedCheckResult{TargetType: "eks", Count: -1}
	}

	var ids []string
	for _, eksRes := range eksList {
		subs := eksRes.Fields["subnets"]
		if subs == "" {
			subs = eksRes.Fields["subnet_ids"]
		}
		if subs == "" {
			continue
		}
		if slices.Contains(splitCSV(subs), subnetID) {
			ids = append(ids, eksRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "eks", Count: -1}
	}
	return relatedResult("eks", ids)
}

// checkSubnetVPCE reports VPC endpoints whose SubnetIds include this subnet
// (interface-type endpoints). Scans the vpce cache.
func checkSubnetVPCE(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	subnetID := res.ID
	if subnetID == "" {
		return resource.RelatedCheckResult{TargetType: "vpce", Count: 0}
	}

	vpceList, truncated, err := subnetRelatedResources(ctx, clients, cache, "vpce")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "vpce", Count: -1, Err: err}
	}
	if vpceList == nil {
		return resource.RelatedCheckResult{TargetType: "vpce", Count: -1}
	}

	var ids []string
	for _, vpceRes := range vpceList {
		vpceRaw, ok := assertStruct[ec2types.VpcEndpoint](vpceRes.RawStruct)
		if !ok {
			continue
		}
		if slices.Contains(vpceRaw.SubnetIds, subnetID) {
			ids = append(ids, vpceRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "vpce", Count: -1}
	}
	return relatedResult("vpce", ids)
}

// splitCSV splits a comma-separated list and trims whitespace from each element.
func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
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




