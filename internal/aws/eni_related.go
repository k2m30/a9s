// eni_related.go contains Network Interface related-resource checker functions.
package aws

import (
	"context"
	"slices"
	"strings"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterNavigableFields("eni", []resource.NavigableField{
		{FieldPath: "VpcId", TargetType: "vpc"},
		{FieldPath: "SubnetId", TargetType: "subnet"},
		{FieldPath: "Groups.GroupId", TargetType: "sg"},
		{FieldPath: "Attachment.InstanceId", TargetType: "ec2"},
		{FieldPath: "Association.AllocationId", TargetType: "eip"},
	})

	resource.RegisterRelated("eni", []resource.RelatedDef{
		{TargetType: "ec2", DisplayName: "EC2 Instances", Checker: checkENIEC2, NeedsTargetCache: true},
		{TargetType: "sg", DisplayName: "Security Groups", Checker: checkENISG, NeedsTargetCache: true},
		{TargetType: "eip", DisplayName: "Elastic IPs", Checker: checkENIEIP, NeedsTargetCache: true},
		{TargetType: "vpc", DisplayName: "VPC", Checker: checkENIVPC},
		{TargetType: "subnet", DisplayName: "Subnet", Checker: checkENISubnet},
		{TargetType: "elb", DisplayName: "Load Balancers", Checker: checkENIELB},
		{TargetType: "lambda", DisplayName: "Lambda Functions", Checker: checkENILambda},
		{TargetType: "nat", DisplayName: "NAT Gateways", Checker: checkENINAT, NeedsTargetCache: true},
		{TargetType: "vpce", DisplayName: "VPC Endpoints", Checker: checkENIVPCE, NeedsTargetCache: true},
	})
}

// checkENIEC2 extracts Attachment.InstanceId from the ENI RawStruct and searches
// the ec2 cache for a matching instance.
func checkENIEC2(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ec2types.NetworkInterface](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "ec2", Count: 0}
	}
	if raw.Attachment == nil || raw.Attachment.InstanceId == nil || *raw.Attachment.InstanceId == "" {
		return resource.RelatedCheckResult{TargetType: "ec2", Count: 0}
	}
	instanceID := *raw.Attachment.InstanceId

	ec2List, truncated, err := eniRelatedResources(ctx, clients, cache, "ec2")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "ec2", Count: -1, Err: err}
	}
	if ec2List == nil {
		return resource.RelatedCheckResult{TargetType: "ec2", Count: -1}
	}
	var ids []string
	for _, ec2Res := range ec2List {
		if ec2Res.ID == instanceID {
			ids = append(ids, ec2Res.ID)
			break
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("ec2")
	}
	return relatedResult("ec2", ids)
}

// checkENISG extracts Groups[].GroupId from the ENI RawStruct and searches
// the sg cache for matching security groups.
func checkENISG(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ec2types.NetworkInterface](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "sg", Count: 0}
	}
	if len(raw.Groups) == 0 {
		return resource.RelatedCheckResult{TargetType: "sg", Count: 0}
	}
	groupIDs := make(map[string]struct{}, len(raw.Groups))
	for _, g := range raw.Groups {
		if g.GroupId != nil && *g.GroupId != "" {
			groupIDs[*g.GroupId] = struct{}{}
		}
	}
	if len(groupIDs) == 0 {
		return resource.RelatedCheckResult{TargetType: "sg", Count: 0}
	}

	sgList, truncated, err := eniRelatedResources(ctx, clients, cache, "sg")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "sg", Count: -1, Err: err}
	}
	if sgList == nil {
		return resource.RelatedCheckResult{TargetType: "sg", Count: -1}
	}
	var ids []string
	for _, sgRes := range sgList {
		if _, found := groupIDs[sgRes.ID]; found {
			ids = append(ids, sgRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("sg")
	}
	return relatedResult("sg", ids)
}

// checkENIEIP extracts Association.AllocationId from the ENI RawStruct and searches
// the eip cache for a matching Elastic IP.
func checkENIEIP(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ec2types.NetworkInterface](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "eip", Count: 0}
	}
	if raw.Association == nil || raw.Association.AllocationId == nil || *raw.Association.AllocationId == "" {
		return resource.RelatedCheckResult{TargetType: "eip", Count: 0}
	}
	allocationID := *raw.Association.AllocationId

	eipList, truncated, err := eniRelatedResources(ctx, clients, cache, "eip")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "eip", Count: -1, Err: err}
	}
	if eipList == nil {
		return resource.RelatedCheckResult{TargetType: "eip", Count: -1}
	}
	var ids []string
	for _, eipRes := range eipList {
		if eipRes.ID == allocationID {
			ids = append(ids, eipRes.ID)
			break
		}
		eipRaw, eipOk := assertStruct[ec2types.Address](eipRes.RawStruct)
		if eipOk && eipRaw.AllocationId != nil && *eipRaw.AllocationId == allocationID {
			ids = append(ids, eipRes.ID)
			break
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("eip")
	}
	return relatedResult("eip", ids)
}

// checkENIVPC returns the VPC this network interface belongs to (Pattern F).
// Reads vpc_id from Fields which is populated by the ENI fetcher.
func checkENIVPC(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	vpcID := res.Fields["vpc_id"]
	if vpcID == "" {
		return resource.RelatedCheckResult{TargetType: "vpc", Count: 0}
	}
	return relatedResult("vpc", []string{vpcID})
}






// checkENISubnet returns the subnet this ENI sits in (Pattern F).
func checkENISubnet(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ec2types.NetworkInterface](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "subnet", Count: -1}
	}
	if raw.SubnetId == nil || *raw.SubnetId == "" {
		return resource.RelatedCheckResult{TargetType: "subnet", Count: 0}
	}
	return relatedResult("subnet", []string{*raw.SubnetId})
}

// checkENIELB reports load balancers that own this ENI. ELBs create "owned"
// ENIs via RequesterId "amazon-elb" with Description like "ELB app/NAME/HASH".
// We detect the ELB name from the Description when RequesterManaged+RequesterId
// indicates ELB, and map to the ELB resource by name.
func checkENIELB(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ec2types.NetworkInterface](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "elb", Count: -1}
	}
	// ELB-owned ENIs are marked by RequesterId "amazon-elb" and their
	// Description starts with "ELB " — the name segment follows.
	if raw.RequesterId == nil || *raw.RequesterId != "amazon-elb" {
		return resource.RelatedCheckResult{TargetType: "elb", Count: 0}
	}
	if raw.Description == nil || *raw.Description == "" {
		// ENI is owned by ELB but no description — the specific ELB cannot be
		// identified from the ENI alone without cross-referencing the ELB cache.
		return resource.RelatedCheckResult{TargetType: "elb", Count: -1}
	}
	desc := *raw.Description
	// Example: "ELB app/my-alb/abcdef1234567890"
	if !strings.HasPrefix(desc, "ELB ") {
		return resource.RelatedCheckResult{TargetType: "elb", Count: 0}
	}
	rest := desc[4:]
	parts := strings.Split(rest, "/")
	if len(parts) < 2 || parts[1] == "" {
		return resource.RelatedCheckResult{TargetType: "elb", Count: 0}
	}
	return relatedResult("elb", []string{parts[1]})
}

// checkENILambda reports Lambda functions that own this ENI. Lambda-owned ENIs
// are marked by RequesterId "*:awslambda_*" and Description contains the
// function name. Without a stable parse contract on description, the function
// cannot always be identified; returns Count: -1 when the ENI is clearly
// Lambda-managed but the function name isn't directly derivable.
func checkENILambda(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ec2types.NetworkInterface](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "lambda", Count: -1}
	}
	reqID := ""
	if raw.RequesterId != nil {
		reqID = *raw.RequesterId
	}
	desc := ""
	if raw.Description != nil {
		desc = *raw.Description
	}
	// Not Lambda-owned → no relationship.
	if !isLambdaENI(reqID, desc) {
		return resource.RelatedCheckResult{TargetType: "lambda", Count: 0}
	}
	// Parse function name from Description: "AWS Lambda VPC ENI-<name>-<uuid>".
	name := lambdaFunctionNameFromENIDescription(desc)
	if name == "" {
		return resource.RelatedCheckResult{TargetType: "lambda", Count: -1}
	}
	return relatedResult("lambda", []string{name})
}

// lambdaFunctionNameFromENIDescription extracts the Lambda function name from
// the ENI Description field. Returns "" when it cannot parse reliably.
func lambdaFunctionNameFromENIDescription(desc string) string {
	const prefix = "AWS Lambda VPC ENI"
	if !strings.HasPrefix(desc, prefix) {
		return ""
	}
	rest := strings.TrimPrefix(desc, prefix)
	rest = strings.TrimLeft(rest, "- ")
	// The trailing segment is a UUID (8-4-4-4-12 hex + dashes = 36 chars).
	if len(rest) >= 37 && rest[len(rest)-37] == '-' {
		uuidPart := rest[len(rest)-36:]
		if uuidPart[8] == '-' && uuidPart[13] == '-' && uuidPart[18] == '-' && uuidPart[23] == '-' {
			return rest[:len(rest)-37]
		}
	}
	if idx := strings.LastIndex(rest, "-"); idx > 0 {
		return rest[:idx]
	}
	return rest
}

// checkENINAT reports NAT gateways whose NatGatewayAddresses include this
// ENI's ID. Scans the nat cache.
func checkENINAT(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	eniID := res.ID
	raw, ok := assertStruct[ec2types.NetworkInterface](res.RawStruct)
	if ok && raw.NetworkInterfaceId != nil && *raw.NetworkInterfaceId != "" {
		eniID = *raw.NetworkInterfaceId
	}
	if eniID == "" {
		return resource.RelatedCheckResult{TargetType: "nat", Count: 0}
	}

	natList, truncated, err := eniRelatedResources(ctx, clients, cache, "nat")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "nat", Count: -1, Err: err}
	}
	if natList == nil {
		return resource.RelatedCheckResult{TargetType: "nat", Count: -1}
	}

	var ids []string
	for _, natRes := range natList {
		natRaw, nOk := assertStruct[ec2types.NatGateway](natRes.RawStruct)
		if !nOk {
			continue
		}
		for _, addr := range natRaw.NatGatewayAddresses {
			if addr.NetworkInterfaceId != nil && *addr.NetworkInterfaceId == eniID {
				ids = append(ids, natRes.ID)
				break
			}
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("nat")
	}
	return relatedResult("nat", ids)
}

// checkENIVPCE reports VPC endpoints that own this ENI via its
// NetworkInterfaceIds field. Scans the vpce cache.
func checkENIVPCE(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	eniID := res.ID
	raw, ok := assertStruct[ec2types.NetworkInterface](res.RawStruct)
	if ok && raw.NetworkInterfaceId != nil && *raw.NetworkInterfaceId != "" {
		eniID = *raw.NetworkInterfaceId
	}
	if eniID == "" {
		return resource.RelatedCheckResult{TargetType: "vpce", Count: 0}
	}

	vpceList, truncated, err := eniRelatedResources(ctx, clients, cache, "vpce")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "vpce", Count: -1, Err: err}
	}
	if vpceList == nil {
		return resource.RelatedCheckResult{TargetType: "vpce", Count: -1}
	}

	var ids []string
	for _, vpceRes := range vpceList {
		vpceRaw, vOk := assertStruct[ec2types.VpcEndpoint](vpceRes.RawStruct)
		if !vOk {
			continue
		}
		if slices.Contains(vpceRaw.NetworkInterfaceIds, eniID) {
			ids = append(ids, vpceRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("vpce")
	}
	return relatedResult("vpce", ids)
}

// isLambdaENI reports whether an ENI is owned by AWS Lambda based on
// RequesterId/Description markers.
func isLambdaENI(requesterID, description string) bool {
	// Typical Lambda RequesterId forms: "<account>:awslambda_*" or contains "awslambda".
	if requesterID != "" && (requesterID == "lambda.amazonaws.com" || strings.Contains(requesterID, "awslambda")) {
		return true
	}
	// Description pattern: "AWS Lambda VPC ENI-<funcname>-<uuid>".
	if description != "" && strings.Contains(description, "Lambda") {
		return true
	}
	return false
}

// eniRelatedResources returns the resource list for target from cache or fetches
// the first page via the registered paginated fetcher.
func eniRelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}
