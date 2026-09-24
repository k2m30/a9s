// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// eni_related.go contains Network Interface related-resource checker functions.
package aws

import (
	"context"
	"slices"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkENIEC2 extracts Attachment.InstanceId from the ENI RawStruct and searches
// the ec2 cache for a matching instance.
func checkENIEC2(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ec2types.NetworkInterface](res.RawStruct)
	if !ok {
		return NotRead("ec2")
	}
	if raw.Attachment == nil || raw.Attachment.InstanceId == nil || *raw.Attachment.InstanceId == "" {
		return foundNone("ec2", "raw.Attachment.InstanceId")
	}
	return relatedResultTrunc("ec2", []string{*raw.Attachment.InstanceId}, false)
}

// checkENISG extracts Groups[].GroupId from the ENI RawStruct and searches
// the sg cache for matching security groups.
func checkENISG(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ec2types.NetworkInterface](res.RawStruct)
	if !ok {
		return NotRead("sg")
	}
	var ids []string
	for _, g := range raw.Groups {
		if g.GroupId != nil && *g.GroupId != "" {
			ids = append(ids, *g.GroupId)
		}
	}
	return relatedResultTrunc("sg", ids, false)
}

// checkENIEIP extracts the allocation IDs associated with the ENI and searches
// the eip cache for matching Elastic IPs. NetworkInterface.Association carries
// the association of the primary private IPv4 address alone; an Elastic IP on
// a secondary private address appears only in that address's own entry under
// PrivateIpAddresses, while Address.NetworkInterfaceId names the interface
// either way.
func checkENIEIP(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ec2types.NetworkInterface](res.RawStruct)
	if !ok {
		return NotRead("eip")
	}
	// In-body: Association.AllocationId IS the eip resource id (eip keyed by AllocationId).
	var ids []string
	add := func(assoc *ec2types.NetworkInterfaceAssociation) {
		if assoc == nil || assoc.AllocationId == nil || *assoc.AllocationId == "" {
			return
		}
		if !slices.Contains(ids, *assoc.AllocationId) {
			ids = append(ids, *assoc.AllocationId)
		}
	}
	add(raw.Association)
	for _, addr := range raw.PrivateIpAddresses {
		add(addr.Association)
	}
	if len(ids) == 0 {
		return foundNone("eip", "raw.Association.AllocationId")
	}
	return relatedResultTrunc("eip", ids, false)
}

// checkENIVPC returns the VPC this network interface belongs to (Pattern F).
// Reads vpc_id from Fields which is populated by the ENI fetcher.
func checkENIVPC(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	vpcID := res.Fields["vpc_id"]
	if vpcID == "" {
		return foundNone("vpc", "vpcID")
	}
	return relatedResultTrunc("vpc", []string{vpcID}, false)
}

// checkENISubnet returns the subnet this ENI sits in (Pattern F).
func checkENISubnet(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ec2types.NetworkInterface](res.RawStruct)
	if !ok {
		return NotRead("subnet")
	}
	if raw.SubnetId == nil || *raw.SubnetId == "" {
		return foundNone("subnet", "raw.SubnetId")
	}
	return relatedResultTrunc("subnet", []string{*raw.SubnetId}, false)
}

// checkENIELB reports load balancers that own this ENI. ELBs create "owned"
// ENIs via RequesterId "amazon-elb" with Description like "ELB app/NAME/HASH".
// We detect the ELB name from the Description when RequesterManaged+RequesterId
// indicates ELB, and map to the ELB resource by name.
func checkENIELB(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ec2types.NetworkInterface](res.RawStruct)
	if !ok {
		return NotRead("elb")
	}
	// ELB-owned ENIs are marked by RequesterId "amazon-elb" and their
	// Description starts with "ELB " — the name segment follows.
	if raw.RequesterId == nil || *raw.RequesterId != "amazon-elb" {
		return foundNone("elb", "raw.RequesterId")
	}
	if raw.Description == nil || *raw.Description == "" {
		// ENI is owned by ELB but no description — the specific ELB cannot be
		// identified from the ENI alone without cross-referencing the ELB cache.
		return NotRead("elb")
	}
	name := elbNameFromENIDescription(*raw.Description)
	if name == "" {
		return foundNone("elb", "name")
	}
	return relatedResultTrunc("elb", []string{name}, false)
}

// checkENILambda reports the Lambda function that owns this ENI. The
// Description carries the function's name by convention only, so an ENI
// Lambda owns whose description does not parse identifies no function and
// the answer is unknown.
func checkENILambda(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ec2types.NetworkInterface](res.RawStruct)
	if !ok {
		return NotRead("lambda")
	}
	if !isLambdaENI(raw) {
		return foundNone("lambda", "the interface type")
	}
	// Parse function name from Description: "AWS Lambda VPC ENI-<name>-<uuid>".
	name := lambdaFunctionNameFromENIDescription(aws.ToString(raw.Description))
	if name == "" {
		return NotRead("lambda")
	}
	return relatedResultTrunc("lambda", []string{name}, false)
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
		return foundNone("nat", "eniID")
	}

	natList, truncated, err := relatedResourcesFor(ctx, clients, cache, "nat")
	if err != nil {
		return ReadFailed("nat", err)
	}
	if natList == nil {
		return NotRead("nat")
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
	return relatedResultTrunc("nat", ids, truncated)
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
		return foundNone("vpce", "eniID")
	}

	vpceList, truncated, err := relatedResourcesFor(ctx, clients, cache, "vpce")
	if err != nil {
		return ReadFailed("vpce", err)
	}
	if vpceList == nil {
		return NotRead("vpce")
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
	return relatedResultTrunc("vpce", ids, truncated)
}
