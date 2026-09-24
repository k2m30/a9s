// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// subnet_related.go contains Subnet related-resource checker functions.
package aws

import (
	"context"
	"slices"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkSubnetEC2 searches the ec2 cache for instances whose SubnetId matches
// the subnet's ID. Uses assertStruct since subnet_id is not in EC2 Fields map.
func checkSubnetEC2(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	subnetID := res.ID
	if subnetID == "" {
		return foundNone("ec2", "subnetID")
	}

	ec2List, truncated, err := relatedResourcesFor(ctx, clients, cache, "ec2")
	if err != nil {
		return ReadFailed("ec2", err)
	}
	if ec2List == nil {
		return NotRead("ec2")
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
	return relatedResultTrunc("ec2", ids, truncated)
}

// checkSubnetENI searches the eni cache for network interfaces whose SubnetId
// matches the subnet's ID. Uses assertStruct since subnet_id is not in ENI Fields map.
func checkSubnetENI(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	subnetID := res.ID
	if subnetID == "" {
		return foundNone("eni", "subnetID")
	}

	eniList, truncated, err := relatedResourcesFor(ctx, clients, cache, "eni")
	if err != nil {
		return ReadFailed("eni", err)
	}
	if eniList == nil {
		return NotRead("eni")
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
	return relatedResultTrunc("eni", ids, truncated)
}

// checkSubnetNAT searches the nat cache for NAT gateways whose subnet_id field
// matches the subnet's ID.
func checkSubnetNAT(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	subnetID := res.ID
	if subnetID == "" {
		return foundNone("nat", "subnetID")
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
		if natRes.Fields["subnet_id"] == subnetID {
			ids = append(ids, natRes.ID)
		}
	}
	return relatedResultTrunc("nat", ids, truncated)
}

// checkSubnetELB searches the elb cache for load balancers whose AvailabilityZones
// include a reference to the subnet's ID.
func checkSubnetELB(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	subnetID := res.ID
	if subnetID == "" {
		return foundNone("elb", "subnetID")
	}

	elbList, truncated, err := relatedResourcesFor(ctx, clients, cache, "elb")
	if err != nil {
		return ReadFailed("elb", err)
	}
	if elbList == nil {
		return NotRead("elb")
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
	return relatedResultTrunc("elb", ids, truncated)
}

// subnetRouteTableIDs names the route tables in rtbList that a subnet routes
// through: the tables whose Associations name it, or, when none does, its
// VPC's main table — AWS associates a subnet with no explicit association to
// the main table, and RouteTableAssociation carries no SubnetId for that
// implicit association. Both directions of the rtb ↔ subnet pair read it.
//
// rtbComplete says whether rtbList is the whole account's tables: the main-table
// fallback reads "no table names this subnet", which an unread page can falsify.
func subnetRouteTableIDs(subnetID, vpcID string, rtbList []resource.Resource, rtbComplete bool) []string {
	var ids []string
	mainRTBID := ""
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
			}
			if assoc.Main != nil && *assoc.Main && rtbVpcID == vpcID {
				mainRTBID = rtbRes.ID
			}
		}
	}
	if len(ids) == 0 && mainRTBID != "" && rtbComplete {
		ids = append(ids, mainRTBID)
	}
	return ids
}

// checkSubnetRTB searches the rtb cache for route tables associated with this
// subnet either explicitly (via SubnetId in Associations) or implicitly via the
// main route table for the subnet's VPC.
func checkSubnetRTB(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	subnetID := res.ID
	if subnetID == "" {
		return foundNone("rtb", "subnetID")
	}

	rtbList, truncated, err := relatedResourcesFor(ctx, clients, cache, "rtb")
	if err != nil {
		return ReadFailed("rtb", err)
	}
	if rtbList == nil {
		return NotRead("rtb")
	}
	return relatedResultTrunc("rtb", subnetRouteTableIDs(subnetID, res.Fields["vpc_id"], rtbList, !truncated), truncated)
}

// checkSubnetCFN checks the subnet's tags for aws:cloudformation:stack-name.
func checkSubnetCFN(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ec2types.Subnet](res.RawStruct)
	if !ok {
		return NotRead("cfn")
	}
	stackName := tagValue(raw.Tags, "aws:cloudformation:stack-name")
	if stackName == "" {
		return foundNone("cfn", "stackName")
	}
	return relatedResultTrunc("cfn", []string{stackName}, false)
}

// checkSubnetVPC returns the VPC this subnet belongs to (Pattern F).
// Reads vpc_id from Fields which is populated by the subnet fetcher.
func checkSubnetVPC(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	vpcID := res.Fields["vpc_id"]
	if vpcID == "" {
		return foundNone("vpc", "vpcID")
	}
	return relatedResultTrunc("vpc", []string{vpcID}, false)
}

// checkSubnetASG scans the ASG cache for groups whose VPCZoneIdentifier
// references this subnet (comma-separated subnet ids).
func checkSubnetASG(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	subnetID := res.ID
	if subnetID == "" {
		return foundNone("asg", "subnetID")
	}

	asgList, truncated, err := relatedResourcesFor(ctx, clients, cache, "asg")
	if err != nil {
		return ReadFailed("asg", err)
	}
	if asgList == nil {
		return NotRead("asg")
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
	return relatedResultTrunc("asg", ids, truncated)
}

// checkSubnetEFS reports EFS file systems mounted into this subnet. Pattern
// C — zero extra API calls: scans the already-loaded eni cache for the
// mount-target ENIs whose SubnetId matches this subnet, reading the file
// system each one names and cross-checking it against the efs cache.
func checkSubnetEFS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	subnetID := res.ID
	if subnetID == "" {
		return foundNone("efs", "subnetID")
	}

	eniList, eniTruncated, err := relatedResourcesFor(ctx, clients, cache, "eni")
	if err != nil {
		return ReadFailed("efs", err)
	}
	if eniList == nil {
		return NotRead("efs")
	}

	fsIDSet := make(map[string]struct{})
	for _, eniRes := range eniList {
		eni, ok := assertStruct[ec2types.NetworkInterface](eniRes.RawStruct)
		if !ok {
			continue
		}
		if eni.SubnetId == nil || *eni.SubnetId != subnetID {
			continue
		}
		if fsID, ok := efsIDFromENIDescription(aws.ToString(eni.Description)); ok {
			fsIDSet[fsID] = struct{}{}
		}
	}
	if len(fsIDSet) == 0 {
		if eniTruncated {
			return relatedResultTrunc("efs", nil, true)
		}
		return foundNone("efs", "fsIDSet")
	}

	efsList, efsTruncated, err := relatedResourcesFor(ctx, clients, cache, "efs")
	if err != nil {
		return ReadFailed("efs", err)
	}
	if efsList == nil {
		return NotRead("efs")
	}

	var ids []string
	for _, efsRes := range efsList {
		if _, found := fsIDSet[efsRes.ID]; found {
			ids = append(ids, efsRes.ID)
		}
	}
	return relatedResultTrunc("efs", ids, efsTruncated || eniTruncated)
}

// checkSubnetEKS reports EKS clusters whose VpcConfig.SubnetIds includes
// this subnet. Scans the eks cache looking at the subnets field.
func checkSubnetEKS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	subnetID := res.ID
	if subnetID == "" {
		return foundNone("eks", "subnetID")
	}

	eksList, truncated, err := relatedResourcesFor(ctx, clients, cache, "eks")
	if err != nil {
		return ReadFailed("eks", err)
	}
	if eksList == nil {
		return NotRead("eks")
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
	return relatedResultTrunc("eks", ids, truncated)
}

// checkSubnetVPCE reports VPC endpoints whose SubnetIds include this subnet
// (interface-type endpoints). Scans the vpce cache.
func checkSubnetVPCE(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	subnetID := res.ID
	if subnetID == "" {
		return foundNone("vpce", "subnetID")
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
		vpceRaw, ok := assertStruct[ec2types.VpcEndpoint](vpceRes.RawStruct)
		if !ok {
			continue
		}
		if slices.Contains(vpceRaw.SubnetIds, subnetID) {
			ids = append(ids, vpceRes.ID)
		}
	}
	return relatedResultTrunc("vpce", ids, truncated)
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
