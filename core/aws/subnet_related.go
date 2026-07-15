// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// subnet_related.go contains Subnet related-resource checker functions.
package aws

import (
	"context"
	"slices"
	"strings"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkSubnetEC2 searches the ec2 cache for instances whose SubnetId matches
// the subnet's ID. Uses assertStruct since subnet_id is not in EC2 Fields map.
func checkSubnetEC2(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	subnetID := res.ID
	if subnetID == "" {
		return resource.RelatedCheckResult{TargetType: "ec2", Count: 0}
	}

	ec2List, truncated, err := relatedResourcesFor(ctx, clients, cache, "ec2")
	if err != nil {
		return resource.ErrorRelated("ec2", err)
	}
	if ec2List == nil {
		return resource.UnknownRelated("ec2")
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
		return resource.RelatedCheckResult{TargetType: "eni", Count: 0}
	}

	eniList, truncated, err := relatedResourcesFor(ctx, clients, cache, "eni")
	if err != nil {
		return resource.ErrorRelated("eni", err)
	}
	if eniList == nil {
		return resource.UnknownRelated("eni")
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
		return resource.RelatedCheckResult{TargetType: "nat", Count: 0}
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
		return resource.RelatedCheckResult{TargetType: "elb", Count: 0}
	}

	elbList, truncated, err := relatedResourcesFor(ctx, clients, cache, "elb")
	if err != nil {
		return resource.ErrorRelated("elb", err)
	}
	if elbList == nil {
		return resource.UnknownRelated("elb")
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

// checkSubnetRTB searches the rtb cache for route tables associated with this
// subnet either explicitly (via SubnetId in Associations) or implicitly via the
// main route table for the subnet's VPC.
func checkSubnetRTB(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	subnetID := res.ID
	vpcID := res.Fields["vpc_id"]
	if subnetID == "" {
		return resource.RelatedCheckResult{TargetType: "rtb", Count: 0}
	}

	rtbList, truncated, err := relatedResourcesFor(ctx, clients, cache, "rtb")
	if err != nil {
		return resource.ErrorRelated("rtb", err)
	}
	if rtbList == nil {
		return resource.UnknownRelated("rtb")
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
	return relatedResultTrunc("rtb", ids, truncated)
}

// checkSubnetCFN checks the subnet's tags for aws:cloudformation:stack-name.
// No cache access needed — the tag carries the stack name directly.
func checkSubnetCFN(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ec2types.Subnet](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("cfn")
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

	asgList, truncated, err := relatedResourcesFor(ctx, clients, cache, "asg")
	if err != nil {
		return resource.ErrorRelated("asg", err)
	}
	if asgList == nil {
		return resource.UnknownRelated("asg")
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
// C — zero extra API calls: scans the already-loaded eni cache for mount-
// target ENIs (Description "EFS mount target for <fsID>") whose SubnetId
// matches this subnet, extracting the filesystem ID and cross-checking it
// against the efs cache. Mirrors checkEFSSubnet's reverse direction
// (efs_related.go).
func checkSubnetEFS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	subnetID := res.ID
	if subnetID == "" {
		return resource.RelatedCheckResult{TargetType: "efs", Count: 0}
	}

	eniList, eniTruncated, err := relatedResourcesFor(ctx, clients, cache, "eni")
	if err != nil {
		return resource.ErrorRelated("efs", err)
	}
	if eniList == nil {
		return resource.UnknownRelated("efs")
	}

	const mountTargetPrefix = "EFS mount target for "
	fsIDSet := make(map[string]struct{})
	for _, eniRes := range eniList {
		eni, ok := assertStruct[ec2types.NetworkInterface](eniRes.RawStruct)
		if !ok {
			continue
		}
		if eni.SubnetId == nil || *eni.SubnetId != subnetID {
			continue
		}
		if eni.Description == nil || !strings.HasPrefix(*eni.Description, mountTargetPrefix) {
			continue
		}
		fsID := strings.TrimPrefix(*eni.Description, mountTargetPrefix)
		if fsID != "" {
			fsIDSet[fsID] = struct{}{}
		}
	}
	if len(fsIDSet) == 0 {
		if eniTruncated {
			return relatedResultTrunc("efs", nil, true)
		}
		return resource.RelatedCheckResult{TargetType: "efs", Count: 0}
	}

	efsList, efsTruncated, err := relatedResourcesFor(ctx, clients, cache, "efs")
	if err != nil {
		return resource.ErrorRelated("efs", err)
	}
	if efsList == nil {
		return resource.UnknownRelated("efs")
	}

	var ids []string
	for _, efsRes := range efsList {
		if _, found := fsIDSet[efsRes.ID]; found {
			ids = append(ids, efsRes.ID)
		}
	}
	return relatedResultTrunc("efs", ids, efsTruncated)
}

// checkSubnetEKS reports EKS clusters whose VpcConfig.SubnetIds includes
// this subnet. Scans the eks cache looking at the subnets field.
func checkSubnetEKS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	subnetID := res.ID
	if subnetID == "" {
		return resource.RelatedCheckResult{TargetType: "eks", Count: 0}
	}

	eksList, truncated, err := relatedResourcesFor(ctx, clients, cache, "eks")
	if err != nil {
		return resource.ErrorRelated("eks", err)
	}
	if eksList == nil {
		return resource.UnknownRelated("eks")
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
