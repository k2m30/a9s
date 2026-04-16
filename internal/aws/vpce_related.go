// vpce_related.go contains VPC Endpoint related-resource checker functions.
package aws

import (
	"context"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterNavigableFields("vpce", []resource.NavigableField{
		{FieldPath: "VpcId", TargetType: "vpc"},
		{FieldPath: "SubnetIds", TargetType: "subnet"},
		{FieldPath: "NetworkInterfaceIds", TargetType: "eni"},
		{FieldPath: "Groups.GroupId", TargetType: "sg"},
		{FieldPath: "RouteTableIds", TargetType: "rtb"},
	})

	resource.RegisterRelated("vpce", []resource.RelatedDef{
		{TargetType: "subnet", DisplayName: "Subnets", Checker: checkVPCESubnet, NeedsTargetCache: false},
		{TargetType: "sg", DisplayName: "Security Groups", Checker: checkVPCESG, NeedsTargetCache: false},
		{TargetType: "rtb", DisplayName: "Route Tables", Checker: checkVPCERTB, NeedsTargetCache: false},
		{TargetType: "eni", DisplayName: "Network Interfaces", Checker: checkVPCEENI, NeedsTargetCache: false},
		{TargetType: "vpc", DisplayName: "VPC", Checker: checkVPCEVPC},
	})
}

// checkVPCESubnet reads SubnetIds from the VpcEndpoint RawStruct directly.
// Pattern F: all data is in RawStruct, no cache lookup needed.
func checkVPCESubnet(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	vpce, ok := assertStruct[ec2types.VpcEndpoint](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "subnet", Count: -1}
	}
	if len(vpce.SubnetIds) == 0 {
		return resource.RelatedCheckResult{TargetType: "subnet", Count: 0}
	}
	return relatedResult("subnet", vpce.SubnetIds)
}

// checkVPCESG reads Groups[].GroupId from the VpcEndpoint RawStruct directly.
// Pattern F: all data is in RawStruct, no cache lookup needed.
func checkVPCESG(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	vpce, ok := assertStruct[ec2types.VpcEndpoint](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "sg", Count: -1}
	}
	var ids []string
	for _, g := range vpce.Groups {
		if g.GroupId != nil && *g.GroupId != "" {
			ids = append(ids, *g.GroupId)
		}
	}
	if len(ids) == 0 {
		return resource.RelatedCheckResult{TargetType: "sg", Count: 0}
	}
	return relatedResult("sg", ids)
}

// checkVPCERTB reads RouteTableIds from the VpcEndpoint RawStruct directly.
// Pattern F: all data is in RawStruct, no cache lookup needed (gateway-type endpoints).
func checkVPCERTB(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	vpce, ok := assertStruct[ec2types.VpcEndpoint](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "rtb", Count: -1}
	}
	if len(vpce.RouteTableIds) == 0 {
		return resource.RelatedCheckResult{TargetType: "rtb", Count: 0}
	}
	return relatedResult("rtb", vpce.RouteTableIds)
}

// checkVPCEENI reads NetworkInterfaceIds from the VpcEndpoint RawStruct directly.
// Pattern F: all data is in RawStruct, no cache lookup needed (interface-type endpoints).
func checkVPCEENI(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	vpce, ok := assertStruct[ec2types.VpcEndpoint](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "eni", Count: -1}
	}
	if len(vpce.NetworkInterfaceIds) == 0 {
		return resource.RelatedCheckResult{TargetType: "eni", Count: 0}
	}
	return relatedResult("eni", vpce.NetworkInterfaceIds)
}

// checkVPCEVPC returns the VPC this endpoint is attached to (Pattern F).
// Reads vpc_id from Fields which is populated by the VPC endpoints fetcher.
func checkVPCEVPC(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	vpcID := res.Fields["vpc_id"]
	if vpcID == "" {
		return resource.RelatedCheckResult{TargetType: "vpc", Count: 0}
	}
	return relatedResult("vpc", []string{vpcID})
}
