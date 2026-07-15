// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// rtb_related.go contains Route Table related-resource checker functions.
package aws

import (
	"context"
	"slices"
	"strings"

	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkRTBSubnet searches the subnet cache for subnets associated with this route table.
// It extracts SubnetIds from ec2types.RouteTable.Associations[] (Pattern C — cache lookup).
func checkRTBSubnet(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	rtb, ok := assertStruct[ec2types.RouteTable](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "subnet", Count: 0}
	}
	// In-body: RouteTable.Associations[].SubnetId ARE the associated subnets.
	var ids []string
	for _, assoc := range rtb.Associations {
		if assoc.SubnetId != nil && *assoc.SubnetId != "" {
			ids = append(ids, *assoc.SubnetId)
		}
	}
	return relatedResult("subnet", ids)
}

// checkRTBNAT searches the nat cache for NAT gateways referenced in this route table's routes.
// It extracts NatGatewayIds from ec2types.RouteTable.Routes[] (Pattern C — cache lookup).
func checkRTBNAT(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	rtb, ok := assertStruct[ec2types.RouteTable](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "nat", Count: 0}
	}
	// In-body: RouteTable.Routes[].NatGatewayId ARE the referenced NAT gateways.
	// Skip blackhole routes — AWS leaves the stale target id on a route after the
	// NAT is deleted, so counting it would advertise an unopenable target.
	var ids []string
	for _, route := range rtb.Routes {
		if route.State == ec2types.RouteStateBlackhole {
			continue
		}
		if route.NatGatewayId != nil && *route.NatGatewayId != "" {
			ids = append(ids, *route.NatGatewayId)
		}
	}
	return relatedResult("nat", ids)
}

// checkRTBIGW searches the igw cache for Internet Gateways referenced in this route table's routes.
// It extracts GatewayIds from Routes[] that start with "igw-" (Pattern C — cache lookup).
func checkRTBIGW(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	rtb, ok := assertStruct[ec2types.RouteTable](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "igw", Count: 0}
	}
	// In-body: RouteTable.Routes[].GatewayId with the igw- prefix ARE the IGWs.
	// Skip blackhole routes — the target id is stale once the gateway is gone.
	var ids []string
	for _, route := range rtb.Routes {
		if route.State == ec2types.RouteStateBlackhole {
			continue
		}
		if route.GatewayId != nil && strings.HasPrefix(*route.GatewayId, "igw-") {
			ids = append(ids, *route.GatewayId)
		}
	}
	return relatedResult("igw", ids)
}

// checkRTBCFN checks EC2 RouteTable tags for aws:cloudformation:stack-name
// and matches against the CFN stack cache (Pattern C — tag-based).
func checkRTBCFN(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	stackName := rtbCFNStackName(res)
	if stackName == "" {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: 0}
	}

	cfnList, truncated, err := relatedResourcesFor(ctx, clients, cache, "cfn")
	if err != nil {
		return resource.ErrorRelated("cfn", err)
	}
	if cfnList == nil {
		return resource.UnknownRelated("cfn")
	}

	var ids []string
	for _, cfnRes := range cfnList {
		if cfnRes.ID == stackName || cfnRes.Name == stackName || cfnRes.Fields["stack_name"] == stackName {
			ids = append(ids, cfnRes.ID)
			continue
		}
		raw, ok := assertStruct[cfntypes.Stack](cfnRes.RawStruct)
		if ok && raw.StackName != nil && *raw.StackName == stackName {
			ids = append(ids, cfnRes.ID)
		}
	}
	return relatedResultTrunc("cfn", ids, truncated)
}

// rtbCFNStackName extracts the aws:cloudformation:stack-name tag value from the
// route table's EC2 Tags slice.
func rtbCFNStackName(res resource.Resource) string {
	rtb, ok := assertStruct[ec2types.RouteTable](res.RawStruct)
	if !ok {
		return ""
	}
	return tagValue(rtb.Tags, "aws:cloudformation:stack-name")
}

// checkRTBVPC returns the VPC this route table belongs to (Pattern F).
// Reads vpc_id from Fields which is populated by the route tables fetcher.
func checkRTBVPC(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	vpcID := res.Fields["vpc_id"]
	if vpcID == "" {
		return resource.RelatedCheckResult{TargetType: "vpc", Count: 0}
	}
	return relatedResult("vpc", []string{vpcID})
}

// checkRTBENI searches the eni cache for interfaces referenced by this route
// table's routes via Routes[].NetworkInterfaceId.
func checkRTBENI(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	rtb, ok := assertStruct[ec2types.RouteTable](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "eni", Count: 0}
	}
	// In-body: RouteTable.Routes[].NetworkInterfaceId ARE the referenced ENIs.
	// Skip blackhole routes — the target id is stale once the ENI is gone.
	var ids []string
	for _, route := range rtb.Routes {
		if route.State == ec2types.RouteStateBlackhole {
			continue
		}
		if route.NetworkInterfaceId != nil && *route.NetworkInterfaceId != "" {
			ids = append(ids, *route.NetworkInterfaceId)
		}
	}
	return relatedResult("eni", ids)
}

// checkRTBTGW searches the tgw cache for transit gateways referenced by this
// route table's routes via Routes[].TransitGatewayId.
func checkRTBTGW(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	rtb, ok := assertStruct[ec2types.RouteTable](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "tgw", Count: 0}
	}
	// In-body: RouteTable.Routes[].TransitGatewayId ARE the referenced TGWs.
	// Skip blackhole routes — the target id is stale once the TGW is gone.
	var ids []string
	for _, route := range rtb.Routes {
		if route.State == ec2types.RouteStateBlackhole {
			continue
		}
		if route.TransitGatewayId != nil && *route.TransitGatewayId != "" {
			ids = append(ids, *route.TransitGatewayId)
		}
	}
	return relatedResult("tgw", ids)
}

// checkRTBVPCE searches the vpce cache for Gateway-type VPC endpoints that
// reference this route table via VpcEndpoint.RouteTableIds.
func checkRTBVPCE(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	rtbID := res.ID
	if rtbID == "" {
		return resource.RelatedCheckResult{TargetType: "vpce", Count: 0}
	}

	vpceList, truncated, err := relatedResourcesFor(ctx, clients, cache, "vpce")
	if err != nil {
		return resource.ErrorRelated("vpce", err)
	}
	if vpceList == nil {
		return resource.UnknownRelated("vpce")
	}
	var ids []string
	for _, vpceRes := range vpceList {
		vpceRaw, ok := assertStruct[ec2types.VpcEndpoint](vpceRes.RawStruct)
		if !ok {
			continue
		}
		if slices.Contains(vpceRaw.RouteTableIds, rtbID) {
			ids = append(ids, vpceRes.ID)
		}
	}
	return relatedResultTrunc("vpce", ids, truncated)
}
