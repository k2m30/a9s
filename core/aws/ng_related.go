// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// ng_related.go contains EKS Node Group related-resource checker functions.
package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkNGEKS extracts ClusterName from the Node Group RawStruct and searches
// the eks cache for a matching cluster by name.
func checkNGEKS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	clusterName := res.Fields["cluster_name"]
	if ng, ok := assertStruct[ekstypes.Nodegroup](res.RawStruct); ok {
		if ng.ClusterName != nil && *ng.ClusterName != "" {
			clusterName = *ng.ClusterName
		}
	}
	if clusterName == "" {
		return resource.KnownRelated("eks", nil, false)
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
		if eksRes.Name == clusterName || eksRes.Fields["cluster_name"] == clusterName {
			ids = append(ids, eksRes.ID)
		}
	}
	return relatedResultTrunc("eks", ids, truncated)
}

// checkNGRole extracts the NodeRole ARN from the Node Group RawStruct, derives
// the role name from the last "/" segment, and searches the role cache by name.
func checkNGRole(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	ng, ok := assertStruct[ekstypes.Nodegroup](res.RawStruct)
	if !ok {
		if res.RawStruct == nil {
			return resource.UnknownRelated("role")
		}
		return resource.KnownRelated("role", nil, false)
	}
	if ng.NodeRole == nil || *ng.NodeRole == "" {
		return resource.KnownRelated("role", nil, false)
	}
	// In-body: the node group's NodeRole ARN normalizes to the role name (== the
	// role's Resource.ID). Resolve by identity — no role-list fetch.
	return relatedResult("role", []string{roleNameFromARN(*ng.NodeRole)})
}

// checkNGASG extracts Resources.AutoScalingGroups from the Node Group RawStruct
// and searches the asg cache for matching ASGs by name.
func checkNGASG(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	ng, ok := assertStruct[ekstypes.Nodegroup](res.RawStruct)
	if !ok {
		if res.RawStruct == nil {
			return resource.UnknownRelated("asg")
		}
		return resource.KnownRelated("asg", nil, false)
	}
	if ng.Resources == nil || len(ng.Resources.AutoScalingGroups) == 0 {
		return resource.KnownRelated("asg", nil, false)
	}

	asgNames := make(map[string]struct{}, len(ng.Resources.AutoScalingGroups))
	for _, asg := range ng.Resources.AutoScalingGroups {
		if asg.Name != nil && *asg.Name != "" {
			asgNames[*asg.Name] = struct{}{}
		}
	}
	if len(asgNames) == 0 {
		return resource.KnownRelated("asg", nil, false)
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
		if _, found := asgNames[asgRes.ID]; found {
			ids = append(ids, asgRes.ID)
			continue
		}
		if _, found := asgNames[asgRes.Name]; found {
			ids = append(ids, asgRes.ID)
		}
	}
	return relatedResultTrunc("asg", ids, truncated)
}

// checkNGEC2 scans the EC2 instance cache for instances tagged with this node
// group's name via "eks:nodegroup-name" and optionally "eks:cluster-name".
// Pattern C: tag-based cache scan, reading the cache directly — a cold or
// missing "ec2" cache entry must never trigger a live fetch (see
// checkNGEBS/cachedTypedRows for the shared contract).
func checkNGEC2(_ context.Context, _ any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	nodegroupName, clusterName := ngIdentity(res)
	if nodegroupName == "" {
		return resource.KnownRelated("ec2", nil, false)
	}

	ec2List, truncated, ok := cachedTypedRows[ec2types.Instance](cache, "ec2")
	if !ok {
		return resource.UnknownRelated("ec2")
	}

	matches := matchingNGInstances(ec2List, nodegroupName, clusterName)
	var ids []string
	for _, m := range matches {
		ids = append(ids, m.ID)
	}
	return relatedResultTrunc("ec2", ids, truncated)
}

// ngIdentity resolves a node group's nodegroup name and cluster name,
// preferring the RawStruct's live values over the pre-extracted Fields map.
// Shared by every NG-related checker that needs to match EC2 instances by
// the "eks:nodegroup-name"/"eks:cluster-name" tags.
func ngIdentity(res resource.Resource) (nodegroupName, clusterName string) {
	nodegroupName = res.Fields["nodegroup_name"]
	clusterName = res.Fields["cluster_name"]
	if ng, ok := assertStruct[ekstypes.Nodegroup](res.RawStruct); ok {
		if ng.NodegroupName != nil && *ng.NodegroupName != "" {
			nodegroupName = *ng.NodegroupName
		}
		if ng.ClusterName != nil && *ng.ClusterName != "" {
			clusterName = *ng.ClusterName
		}
	}
	return nodegroupName, clusterName
}

// matchingNGInstances filters ec2List for instances tagged with
// nodegroupName via "eks:nodegroup-name" and, when clusterName is
// non-empty, also matching "eks:cluster-name". Shared by checkNGEC2 and
// checkNGEBS.
func matchingNGInstances(ec2List []typedRow[ec2types.Instance], nodegroupName, clusterName string) []typedRow[ec2types.Instance] {
	var matches []typedRow[ec2types.Instance]
	for _, row := range ec2List {
		if tagValue(row.Raw.Tags, "eks:nodegroup-name") != nodegroupName {
			continue
		}
		if clusterName != "" && tagValue(row.Raw.Tags, "eks:cluster-name") != clusterName {
			continue
		}
		matches = append(matches, row)
	}
	return matches
}

// checkNGSG extracts the remote access security group from the EKS Node Group's
// Resources.RemoteAccessSecurityGroup field (present when the node group is not
// using a launch template and SSH access is configured).
// Pattern F — no cache needed.
func checkNGSG(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	ng, ok := assertStruct[ekstypes.Nodegroup](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("sg")
	}
	if ng.Resources == nil || ng.Resources.RemoteAccessSecurityGroup == nil ||
		*ng.Resources.RemoteAccessSecurityGroup == "" {
		return resource.KnownRelated("sg", nil, false)
	}
	return relatedResult("sg", []string{*ng.Resources.RemoteAccessSecurityGroup})
}

// checkNGAMI resolves the AMI used by this node group's launch template.
// Pattern A — ec2:DescribeLaunchTemplateVersions if LaunchTemplate is set.
// Managed NGs without a custom launch template: AMI resolution via SSM is deferred; returns Count:0.
func checkNGAMI(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	ng, ok := assertStruct[ekstypes.Nodegroup](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("ami")
	}
	if ng.LaunchTemplate == nil || ng.LaunchTemplate.Id == nil || *ng.LaunchTemplate.Id == "" {
		// Managed NG without custom LT — AMI resolution via SSM deferred.
		return resource.KnownRelated("ami", nil, false)
	}

	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.EC2 == nil {
		return resource.UnknownRelated("ami")
	}

	version := aws.String("$Latest")
	if ng.LaunchTemplate.Version != nil && *ng.LaunchTemplate.Version != "" {
		version = ng.LaunchTemplate.Version
	}

	ltOut, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*ec2.DescribeLaunchTemplateVersionsOutput, error) {
		return c.EC2.DescribeLaunchTemplateVersions(ctx, &ec2.DescribeLaunchTemplateVersionsInput{
			LaunchTemplateId: ng.LaunchTemplate.Id,
			Versions:         []string{*version},
		})
	})
	if err != nil {
		// Launch template deleted upstream — that is a true zero, not a
		// fetch failure: there is no AMI for this NG to relate to.
		if ErrCodeIs(err, "InvalidLaunchTemplateId.NotFound") {
			return resource.KnownRelated("ami", nil, false)
		}
		return resource.ErrorRelated("ami", err)
	}
	for _, v := range ltOut.LaunchTemplateVersions {
		if v.LaunchTemplateData != nil && v.LaunchTemplateData.ImageId != nil && *v.LaunchTemplateData.ImageId != "" {
			return relatedResult("ami", []string{*v.LaunchTemplateData.ImageId})
		}
	}
	return resource.KnownRelated("ami", nil, false)
}

// checkNGEBS scans the EC2 instance cache for instances tagged with this node
// group's name via "eks:nodegroup-name" (and, when known, "eks:cluster-name"),
// then collects the EBS volume IDs from each matched instance's
// BlockDeviceMappings. Pattern C: tag-based cache scan, mirroring checkNGEC2 —
// zero extra AWS calls; a cold or missing "ec2" cache entry reads as unknown
// ("?"), never as a fetch trigger (docs/resources/ng.md §2 ebs bullet).
func checkNGEBS(_ context.Context, _ any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	nodegroupName, clusterName := ngIdentity(res)
	if nodegroupName == "" {
		return resource.KnownRelated("ebs", nil, false)
	}

	ec2List, truncated, ok := cachedTypedRows[ec2types.Instance](cache, "ec2")
	if !ok {
		return resource.UnknownRelated("ebs")
	}

	matches := matchingNGInstances(ec2List, nodegroupName, clusterName)
	seen := make(map[string]struct{})
	var ids []string
	for _, m := range matches {
		for _, bdm := range m.Raw.BlockDeviceMappings {
			if bdm.Ebs != nil && bdm.Ebs.VolumeId != nil && *bdm.Ebs.VolumeId != "" {
				volumeID := *bdm.Ebs.VolumeId
				if _, dup := seen[volumeID]; dup {
					continue
				}
				seen[volumeID] = struct{}{}
				ids = append(ids, volumeID)
			}
		}
	}
	return relatedResultTrunc("ebs", ids, truncated)
}

// checkNGSubnet returns the subnet IDs this node group deploys into.
// Pattern F — no AWS call needed; data is in Subnets[] on the Nodegroup struct.
func checkNGSubnet(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	ng, ok := assertStruct[ekstypes.Nodegroup](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("subnet")
	}
	var ids []string
	for _, s := range ng.Subnets {
		if s != "" {
			ids = append(ids, s)
		}
	}
	if len(ids) == 0 {
		return resource.KnownRelated("subnet", nil, false)
	}
	return relatedResult("subnet", ids)
}
