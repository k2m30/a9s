// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// ng_related.go contains EKS Node Group related-resource checker functions.
package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
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
		return keyMissing("eks", "clusterName")
	}

	eksList, truncated, err := relatedRowsByID(ctx, clients, cache, "eks")
	if err != nil {
		return ReadFailed("eks", err)
	}
	if eksList == nil {
		return NotRead("eks")
	}

	var ids []string
	for _, eksRes := range eksList {
		if eksRes.Name == clusterName || eksRes.Fields["cluster_name"] == clusterName {
			ids = append(ids, eksRes.ID)
		}
	}
	return relatedAnswer("eks", relatedRead{ids: ids, partial: truncated, atMostOne: true})
}

// checkNGRole extracts the NodeRole ARN from the Node Group RawStruct, derives
// the role name from the last "/" segment, and searches the role cache by name.
func checkNGRole(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	ng, ok := assertStruct[ekstypes.Nodegroup](res.RawStruct)
	if !ok {
		return NotRead("role")
	}
	if ng.NodeRole == nil || *ng.NodeRole == "" {
		return foundNone("role", "ng.NodeRole")
	}
	// The node group's NodeRole ARN normalizes to the role name (== the
	// role's Resource.ID), so it resolves by identity.
	return relatedRefs("role", []string{*ng.NodeRole}, refContext(clients, cache, "role"))
}

// checkNGASG extracts Resources.AutoScalingGroups from the Node Group RawStruct
// and searches the asg cache for matching ASGs by name.
func checkNGASG(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	ng, ok := assertStruct[ekstypes.Nodegroup](res.RawStruct)
	if !ok {
		return NotRead("asg")
	}
	if ng.Resources == nil || len(ng.Resources.AutoScalingGroups) == 0 {
		return foundNone("asg", "ng.Resources.AutoScalingGroups")
	}

	asgNames := make(map[string]struct{}, len(ng.Resources.AutoScalingGroups))
	for _, asg := range ng.Resources.AutoScalingGroups {
		if asg.Name != nil && *asg.Name != "" {
			asgNames[*asg.Name] = struct{}{}
		}
	}
	if len(asgNames) == 0 {
		return foundNone("asg", "asgNames")
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
// It reads the cache directly — a cold or
// missing "ec2" cache entry must never trigger a live fetch (see
// checkNGEBS/cachedTypedRows for the shared contract).
func checkNGEC2(_ context.Context, _ any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	nodegroupName, clusterName := ngIdentity(res)
	if nodegroupName == "" {
		return foundNone("ec2", "nodegroupName")
	}

	ec2List, truncated, ok := cachedTypedRows[ec2types.Instance](cache, "ec2")
	if !ok {
		return NotRead("ec2")
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
		if ec2InstanceLive(row.Raw) && ngOwnsInstance(row.Raw.Tags, nodegroupName, clusterName) {
			matches = append(matches, row)
		}
	}
	return matches
}

// checkNGSG returns the security groups on the node group's nodes: the
// remote-access group EKS creates and the client groups it admits
// (docs/resources/ng.md), and the groups the launch template names, on the
// instance or on its network interfaces.
func checkNGSG(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	ng, ok := assertStruct[ekstypes.Nodegroup](res.RawStruct)
	if !ok {
		return NotRead("sg")
	}
	var ids []string
	if ng.Resources != nil {
		ids = append(ids, aws.ToString(ng.Resources.RemoteAccessSecurityGroup))
	}
	if ng.RemoteAccess != nil {
		ids = append(ids, ng.RemoteAccess.SourceSecurityGroups...)
	}
	data, err := ngLaunchTemplateData(ctx, ngTemplateAPI(clients), ng.LaunchTemplate)
	template := relatedRead{}
	if err != nil {
		template = unreadBy(err)
	}
	if data != nil {
		template.ids = append(template.ids, data.SecurityGroupIds...)
		for _, ni := range data.NetworkInterfaces {
			template.ids = append(template.ids, ni.Groups...)
		}
	}
	return relatedAnswer("sg", joinReads(relatedRead{ids: ids}, template))
}

// checkNGAMI resolves the AMI of the launch template the node group names,
// with ec2:DescribeLaunchTemplateVersions. A node group on no template of its
// own, or on one that names no image, runs the EKS-optimised image of its
// AmiType and release version, which no image id here names.
func checkNGAMI(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	ng, ok := assertStruct[ekstypes.Nodegroup](res.RawStruct)
	if !ok {
		return NotRead("ami")
	}
	data, err := ngLaunchTemplateData(ctx, ngTemplateAPI(clients), ng.LaunchTemplate)
	switch {
	case ErrCodeIs(err, "InvalidLaunchTemplateId.NotFound"):
		// Launch template deleted upstream — that is a true zero, not a
		// fetch failure: there is no AMI for this NG to relate to.
		return foundNone("ami", "the API answered that none is configured")
	case err != nil && !isAWSRefusal(err):
		return ReadFailed("ami", err)
	case err != nil || data == nil || aws.ToString(data.ImageId) == "":
		return relatedAnswer("ami", relatedRead{unread: true})
	}
	return relatedRefs("ami", []string{*data.ImageId}, refContext(clients, nil, "ami"))
}

// ngTemplateAPI is the EC2 client a node group's launch template is read
// with, nil when the session has none.
func ngTemplateAPI(clients any) EC2DescribeLaunchTemplateVersionsAPI {
	if c, ok := clients.(*ServiceClients); ok && c != nil && c.EC2 != nil {
		return c.EC2
	}
	return nil
}

// checkNGEBS scans the EC2 instance cache for instances tagged with this node
// group's name via "eks:nodegroup-name" (and, when known, "eks:cluster-name"),
// then collects the EBS volume IDs from each matched instance's
// BlockDeviceMappings, as checkNGEC2 does — zero extra AWS calls; a cold or
// missing "ec2" cache entry reads as unknown ("?"), never as a fetch
// trigger (docs/resources/ng.md).
func checkNGEBS(_ context.Context, _ any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	nodegroupName, clusterName := ngIdentity(res)
	if nodegroupName == "" {
		return foundNone("ebs", "nodegroupName")
	}

	ec2List, truncated, ok := cachedTypedRows[ec2types.Instance](cache, "ec2")
	if !ok {
		return NotRead("ebs")
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
// The data is in Subnets[] on the Nodegroup struct.
func checkNGSubnet(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	ng, ok := assertStruct[ekstypes.Nodegroup](res.RawStruct)
	if !ok {
		return NotRead("subnet")
	}
	var ids []string
	for _, s := range ng.Subnets {
		if s != "" {
			ids = append(ids, s)
		}
	}
	if len(ids) == 0 {
		return foundNone("subnet", "ids")
	}
	return relatedResultTrunc("subnet", ids, false)
}
