// routetables_related.go contains Route Table related-resource checker functions.
package aws

import (
	"context"
	"strings"

	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterNavigableFields("rtb", []resource.NavigableField{
		{FieldPath: "VpcId", TargetType: "vpc"},
		{FieldPath: "Associations.SubnetId", TargetType: "subnet"},
		{FieldPath: "Routes.NatGatewayId", TargetType: "nat"},
		{FieldPath: "Routes.GatewayId", TargetType: "igw"},
		{FieldPath: "Routes.NetworkInterfaceId", TargetType: "eni"},
		{FieldPath: "Routes.TransitGatewayId", TargetType: "tgw"},
		{FieldPath: "Routes.VpcPeeringConnectionId", TargetType: "vpc"},
	})

	resource.RegisterRelated("rtb", []resource.RelatedDef{
		{TargetType: "subnet", DisplayName: "Subnets", Checker: checkRTBSubnet, NeedsTargetCache: true},
		{TargetType: "nat", DisplayName: "NAT Gateways", Checker: checkRTBNAT, NeedsTargetCache: true},
		{TargetType: "igw", DisplayName: "Internet Gateways", Checker: checkRTBIGW, NeedsTargetCache: true},
		{TargetType: "cfn", DisplayName: "CloudFormation", Checker: checkRTBCFN, NeedsTargetCache: true},
		{TargetType: "vpc", DisplayName: "VPC", Checker: checkRTBVPC},
		{TargetType: "eni", DisplayName: "Network Interfaces", Checker: checkRTBENI, NeedsTargetCache: true},
		{TargetType: "tgw", DisplayName: "Transit Gateways", Checker: checkRTBTGW, NeedsTargetCache: true},
		{TargetType: "vpce", DisplayName: "VPC Endpoints", Checker: checkRTBVPCE, NeedsTargetCache: true},
	})
}

// checkRTBSubnet searches the subnet cache for subnets associated with this route table.
// It extracts SubnetIds from ec2types.RouteTable.Associations[] (Pattern C — cache lookup).
func checkRTBSubnet(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	rtb, ok := assertStruct[ec2types.RouteTable](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "subnet", Count: -1}
	}

	subnetIDs := make(map[string]bool)
	for _, assoc := range rtb.Associations {
		if assoc.SubnetId != nil && *assoc.SubnetId != "" {
			subnetIDs[*assoc.SubnetId] = true
		}
	}
	if len(subnetIDs) == 0 {
		return resource.RelatedCheckResult{TargetType: "subnet", Count: 0}
	}

	subnetList, truncated, err := rtbRelatedResources(ctx, clients, cache, "subnet")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "subnet", Count: -1, Err: err}
	}
	if subnetList == nil {
		return resource.RelatedCheckResult{TargetType: "subnet", Count: -1}
	}

	var ids []string
	for _, subnetRes := range subnetList {
		if subnetIDs[subnetRes.ID] {
			ids = append(ids, subnetRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "subnet", Count: -1}
	}
	return relatedResult("subnet", ids)
}

// checkRTBNAT searches the nat cache for NAT gateways referenced in this route table's routes.
// It extracts NatGatewayIds from ec2types.RouteTable.Routes[] (Pattern C — cache lookup).
func checkRTBNAT(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	rtb, ok := assertStruct[ec2types.RouteTable](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "nat", Count: -1}
	}

	natIDs := make(map[string]bool)
	for _, route := range rtb.Routes {
		if route.NatGatewayId != nil && *route.NatGatewayId != "" {
			natIDs[*route.NatGatewayId] = true
		}
	}
	if len(natIDs) == 0 {
		return resource.RelatedCheckResult{TargetType: "nat", Count: 0}
	}

	natList, truncated, err := rtbRelatedResources(ctx, clients, cache, "nat")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "nat", Count: -1, Err: err}
	}
	if natList == nil {
		return resource.RelatedCheckResult{TargetType: "nat", Count: -1}
	}

	var ids []string
	for _, natRes := range natList {
		if natIDs[natRes.ID] {
			ids = append(ids, natRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "nat", Count: -1}
	}
	return relatedResult("nat", ids)
}

// checkRTBIGW searches the igw cache for Internet Gateways referenced in this route table's routes.
// It extracts GatewayIds from Routes[] that start with "igw-" (Pattern C — cache lookup).
func checkRTBIGW(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	rtb, ok := assertStruct[ec2types.RouteTable](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "igw", Count: -1}
	}

	igwIDs := make(map[string]bool)
	for _, route := range rtb.Routes {
		if route.GatewayId != nil && strings.HasPrefix(*route.GatewayId, "igw-") {
			igwIDs[*route.GatewayId] = true
		}
	}
	if len(igwIDs) == 0 {
		return resource.RelatedCheckResult{TargetType: "igw", Count: 0}
	}

	igwList, truncated, err := rtbRelatedResources(ctx, clients, cache, "igw")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "igw", Count: -1, Err: err}
	}
	if igwList == nil {
		return resource.RelatedCheckResult{TargetType: "igw", Count: -1}
	}

	var ids []string
	for _, igwRes := range igwList {
		if igwIDs[igwRes.ID] {
			ids = append(ids, igwRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "igw", Count: -1}
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

	cfnList, truncated, err := rtbRelatedResources(ctx, clients, cache, "cfn")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1, Err: err}
	}
	if cfnList == nil {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1}
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
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1}
	}
	return relatedResult("cfn", ids)
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
func checkRTBENI(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	rtb, ok := assertStruct[ec2types.RouteTable](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "eni", Count: -1}
	}
	eniIDs := make(map[string]bool)
	for _, route := range rtb.Routes {
		if route.NetworkInterfaceId != nil && *route.NetworkInterfaceId != "" {
			eniIDs[*route.NetworkInterfaceId] = true
		}
	}
	if len(eniIDs) == 0 {
		return resource.RelatedCheckResult{TargetType: "eni", Count: 0}
	}

	eniList, truncated, err := rtbRelatedResources(ctx, clients, cache, "eni")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "eni", Count: -1, Err: err}
	}
	if eniList == nil {
		return resource.RelatedCheckResult{TargetType: "eni", Count: -1}
	}
	var ids []string
	for _, eniRes := range eniList {
		if eniIDs[eniRes.ID] {
			ids = append(ids, eniRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "eni", Count: -1}
	}
	return relatedResult("eni", ids)
}

// checkRTBTGW searches the tgw cache for transit gateways referenced by this
// route table's routes via Routes[].TransitGatewayId.
func checkRTBTGW(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	rtb, ok := assertStruct[ec2types.RouteTable](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "tgw", Count: -1}
	}
	tgwIDs := make(map[string]bool)
	for _, route := range rtb.Routes {
		if route.TransitGatewayId != nil && *route.TransitGatewayId != "" {
			tgwIDs[*route.TransitGatewayId] = true
		}
	}
	if len(tgwIDs) == 0 {
		return resource.RelatedCheckResult{TargetType: "tgw", Count: 0}
	}

	tgwList, truncated, err := rtbRelatedResources(ctx, clients, cache, "tgw")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "tgw", Count: -1, Err: err}
	}
	if tgwList == nil {
		return resource.RelatedCheckResult{TargetType: "tgw", Count: -1}
	}
	var ids []string
	for _, tgwRes := range tgwList {
		if tgwIDs[tgwRes.ID] {
			ids = append(ids, tgwRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "tgw", Count: -1}
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

	vpceList, truncated, err := rtbRelatedResources(ctx, clients, cache, "vpce")
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
		for _, rid := range vpceRaw.RouteTableIds {
			if rid == rtbID {
				ids = append(ids, vpceRes.ID)
				break
			}
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "vpce", Count: -1}
	}
	return relatedResult("vpce", ids)
}

// rtbRelatedResources returns the resource list for target from cache or fetches
// the first page via the registered paginated fetcher.
func rtbRelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}



