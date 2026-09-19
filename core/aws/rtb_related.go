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
// It extracts SubnetIds from ec2types.RouteTable.Associations[].
func checkRTBSubnet(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	rtb, ok := assertStruct[ec2types.RouteTable](res.RawStruct)
	if !ok {
		if res.RawStruct == nil {
			return resource.UnknownRelated("subnet")
		}
		return resource.KnownRelated("subnet", nil, false)
	}
	// RouteTable.Associations[].SubnetId are the associated subnets.
	var ids []string
	for _, assoc := range rtb.Associations {
		if assoc.SubnetId != nil && *assoc.SubnetId != "" {
			ids = append(ids, *assoc.SubnetId)
		}
	}
	return relatedResultTrunc("subnet", ids, false)
}

// checkRTBNAT searches the nat cache for NAT gateways referenced in this route table's routes.
// It extracts NatGatewayIds from ec2types.RouteTable.Routes[].
func checkRTBNAT(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	rtb, ok := assertStruct[ec2types.RouteTable](res.RawStruct)
	if !ok {
		if res.RawStruct == nil {
			return resource.UnknownRelated("nat")
		}
		return resource.KnownRelated("nat", nil, false)
	}
	// RouteTable.Routes[].NatGatewayId are the referenced NAT gateways.
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
	return relatedResultTrunc("nat", ids, false)
}

// checkRTBIGW searches the igw cache for Internet Gateways referenced in this route table's routes.
// It extracts GatewayIds from Routes[] that start with "igw-".
func checkRTBIGW(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	rtb, ok := assertStruct[ec2types.RouteTable](res.RawStruct)
	if !ok {
		if res.RawStruct == nil {
			return resource.UnknownRelated("igw")
		}
		return resource.KnownRelated("igw", nil, false)
	}
	// RouteTable.Routes[].GatewayId with the igw- prefix are the IGWs.
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
	return relatedResultTrunc("igw", ids, false)
}

// checkRTBCFN checks EC2 RouteTable tags for aws:cloudformation:stack-name
// and matches against the CFN stack cache.
func checkRTBCFN(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	stackName := rtbCFNStackName(res)
	if stackName == "" {
		return unreadZero(res, resource.ProvenZero("cfn", "stackName"))
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
	return unreadZeroScanned(res, len(cfnList), relatedResultTrunc("cfn", ids, truncated))
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

// checkRTBVPC returns the VPC this route table belongs to.
// Reads vpc_id from Fields which is populated by the route tables fetcher.
func checkRTBVPC(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	vpcID := res.Fields["vpc_id"]
	if vpcID == "" {
		return resource.ProvenZero("vpc", "vpcID")
	}
	return relatedResultTrunc("vpc", []string{vpcID}, false)
}

// checkRTBENI searches the eni cache for interfaces referenced by this route
// table's routes via Routes[].NetworkInterfaceId.
func checkRTBENI(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	rtb, ok := assertStruct[ec2types.RouteTable](res.RawStruct)
	if !ok {
		if res.RawStruct == nil {
			return resource.UnknownRelated("eni")
		}
		return resource.KnownRelated("eni", nil, false)
	}
	// RouteTable.Routes[].NetworkInterfaceId are the referenced ENIs.
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
	return relatedResultTrunc("eni", ids, false)
}

// checkRTBTGW searches the tgw cache for transit gateways referenced by this
// route table's routes via Routes[].TransitGatewayId.
func checkRTBTGW(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	rtb, ok := assertStruct[ec2types.RouteTable](res.RawStruct)
	if !ok {
		if res.RawStruct == nil {
			return resource.UnknownRelated("tgw")
		}
		return resource.KnownRelated("tgw", nil, false)
	}
	// RouteTable.Routes[].TransitGatewayId are the referenced TGWs.
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
	return relatedResultTrunc("tgw", ids, false)
}

// checkRTBVPCE searches the vpce cache for Gateway-type VPC endpoints that
// reference this route table via VpcEndpoint.RouteTableIds.
func checkRTBVPCE(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	rtbID := res.ID
	if rtbID == "" {
		return resource.ProvenZero("vpce", "rtbID")
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
