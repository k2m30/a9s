// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// eni_related.go contains Network Interface related-resource checker functions.
package aws

import (
	"context"
	"slices"
	"strings"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkENIEC2 extracts Attachment.InstanceId from the ENI RawStruct and searches
// the ec2 cache for a matching instance.
func checkENIEC2(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ec2types.NetworkInterface](res.RawStruct)
	if !ok {
		if res.RawStruct == nil {
			return resource.UnknownRelated("ec2")
		}
		return resource.KnownRelated("ec2", nil, false)
	}
	if raw.Attachment == nil || raw.Attachment.InstanceId == nil || *raw.Attachment.InstanceId == "" {
		return resource.ProvenZero("ec2", "raw.Attachment.InstanceId")
	}
	return relatedResultTrunc("ec2", []string{*raw.Attachment.InstanceId}, false)
}

// checkENISG extracts Groups[].GroupId from the ENI RawStruct and searches
// the sg cache for matching security groups.
func checkENISG(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ec2types.NetworkInterface](res.RawStruct)
	if !ok {
		if res.RawStruct == nil {
			return resource.UnknownRelated("sg")
		}
		return resource.KnownRelated("sg", nil, false)
	}
	var ids []string
	for _, g := range raw.Groups {
		if g.GroupId != nil && *g.GroupId != "" {
			ids = append(ids, *g.GroupId)
		}
	}
	return relatedResultTrunc("sg", ids, false)
}

// checkENIEIP extracts Association.AllocationId from the ENI RawStruct and searches
// the eip cache for a matching Elastic IP.
func checkENIEIP(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ec2types.NetworkInterface](res.RawStruct)
	if !ok {
		if res.RawStruct == nil {
			return resource.UnknownRelated("eip")
		}
		return resource.KnownRelated("eip", nil, false)
	}
	if raw.Association == nil || raw.Association.AllocationId == nil || *raw.Association.AllocationId == "" {
		return resource.ProvenZero("eip", "raw.Association.AllocationId")
	}
	// In-body: Association.AllocationId IS the eip resource id (eip keyed by AllocationId).
	return relatedResultTrunc("eip", []string{*raw.Association.AllocationId}, false)
}

// checkENIVPC returns the VPC this network interface belongs to (Pattern F).
// Reads vpc_id from Fields which is populated by the ENI fetcher.
func checkENIVPC(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	vpcID := res.Fields["vpc_id"]
	if vpcID == "" {
		return resource.ProvenZero("vpc", "vpcID")
	}
	return relatedResultTrunc("vpc", []string{vpcID}, false)
}

// checkENISubnet returns the subnet this ENI sits in (Pattern F).
func checkENISubnet(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ec2types.NetworkInterface](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("subnet")
	}
	if raw.SubnetId == nil || *raw.SubnetId == "" {
		return resource.ProvenZero("subnet", "raw.SubnetId")
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
		return resource.UnknownRelated("elb")
	}
	// ELB-owned ENIs are marked by RequesterId "amazon-elb" and their
	// Description starts with "ELB " — the name segment follows.
	if raw.RequesterId == nil || *raw.RequesterId != "amazon-elb" {
		return resource.ProvenZero("elb", "raw.RequesterId")
	}
	if raw.Description == nil || *raw.Description == "" {
		// ENI is owned by ELB but no description — the specific ELB cannot be
		// identified from the ENI alone without cross-referencing the ELB cache.
		return resource.UnknownRelated("elb")
	}
	name := elbNameFromENIDescription(*raw.Description)
	if name == "" {
		return resource.ProvenZero("elb", "name")
	}
	return relatedResultTrunc("elb", []string{name}, false)
}

// checkENILambda reports Lambda functions that own this ENI. Lambda-owned ENIs
// are marked by RequesterId "*:awslambda_*" and Description contains the
// function name. Without a stable parse contract on description, the function
// cannot always be identified; returns an unknown result when the ENI is
// clearly Lambda-managed but the function name isn't directly derivable.
func checkENILambda(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ec2types.NetworkInterface](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("lambda")
	}
	reqID := ""
	if raw.RequesterId != nil {
		reqID = *raw.RequesterId
	}
	desc := ""
	if raw.Description != nil {
		desc = *raw.Description
	}
	if !isLambdaENI(reqID, desc) {
		return resource.ProvenZero("lambda", "reqID")
	}
	// Parse function name from Description: "AWS Lambda VPC ENI-<name>-<uuid>".
	name := lambdaFunctionNameFromENIDescription(desc)
	if name == "" {
		return resource.UnknownRelated("lambda")
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
		return resource.ProvenZero("nat", "eniID")
	}

	natList, truncated, err := relatedResourcesFor(ctx, clients, cache, "nat")
	if err != nil {
		return resource.ErrorRelated("nat", err)
	}
	if natList == nil {
		return resource.UnknownRelated("nat")
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
		return resource.ProvenZero("vpce", "eniID")
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
