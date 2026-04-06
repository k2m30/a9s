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
		{TargetType: "rtb", DisplayName: "Route Tables", Checker: nil, NeedsTargetCache: true},
		{TargetType: "cfn", DisplayName: "CloudFormation", Checker: nil, NeedsTargetCache: true},
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
