// ng_related.go contains EKS Node Group related-resource checker functions.
package aws

import (
	"context"
	"errors"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	smithy "github.com/aws/smithy-go"

	"github.com/k2m30/a9s/v3/internal/resource"
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
		return resource.RelatedCheckResult{TargetType: "eks", Count: 0}
	}

	eksList, _, err := ngRelatedResources(ctx, clients, cache, "eks")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "eks", Count: -1, Err: err}
	}
	if eksList == nil {
		return resource.RelatedCheckResult{TargetType: "eks", Count: -1}
	}

	var ids []string
	for _, eksRes := range eksList {
		if eksRes.Name == clusterName || eksRes.Fields["cluster_name"] == clusterName {
			ids = append(ids, eksRes.ID)
		}
	}
	return relatedResult("eks", ids)
}

// checkNGRole extracts the NodeRole ARN from the Node Group RawStruct, derives
// the role name from the last "/" segment, and searches the role cache by name.
func checkNGRole(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	ng, ok := assertStruct[ekstypes.Nodegroup](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "role", Count: 0}
	}
	if ng.NodeRole == nil || *ng.NodeRole == "" {
		return resource.RelatedCheckResult{TargetType: "role", Count: 0}
	}
	roleVal := *ng.NodeRole
	roleName := roleVal
	if idx := strings.LastIndex(roleVal, "/"); idx >= 0 && idx < len(roleVal)-1 {
		roleName = roleVal[idx+1:]
	}

	roleList, _, err := ngRelatedResources(ctx, clients, cache, "role")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "role", Count: -1, Err: err}
	}
	if roleList == nil {
		return resource.RelatedCheckResult{TargetType: "role", Count: -1}
	}

	var ids []string
	for _, roleRes := range roleList {
		if roleRes.Name == roleName || roleRes.Fields["role_name"] == roleName {
			ids = append(ids, roleRes.ID)
		}
	}
	return relatedResult("role", ids)
}

// checkNGASG extracts Resources.AutoScalingGroups from the Node Group RawStruct
// and searches the asg cache for matching ASGs by name.
func checkNGASG(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	ng, ok := assertStruct[ekstypes.Nodegroup](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "asg", Count: 0}
	}
	if ng.Resources == nil || len(ng.Resources.AutoScalingGroups) == 0 {
		return resource.RelatedCheckResult{TargetType: "asg", Count: 0}
	}

	asgNames := make(map[string]struct{}, len(ng.Resources.AutoScalingGroups))
	for _, asg := range ng.Resources.AutoScalingGroups {
		if asg.Name != nil && *asg.Name != "" {
			asgNames[*asg.Name] = struct{}{}
		}
	}
	if len(asgNames) == 0 {
		return resource.RelatedCheckResult{TargetType: "asg", Count: 0}
	}

	asgList, truncated, err := ngRelatedResources(ctx, clients, cache, "asg")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "asg", Count: -1, Err: err}
	}
	if asgList == nil {
		return resource.RelatedCheckResult{TargetType: "asg", Count: -1}
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
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("asg")
	}
	return relatedResult("asg", ids)
}

// checkNGEC2 scans the EC2 instance cache for instances tagged with this node
// group's name via "eks:nodegroup-name" and optionally "eks:cluster-name".
// Pattern C: tag-based cache scan, reading the cache directly — a cold or
// missing "ec2" cache entry must never trigger a live fetch (see
// checkNGEBS/ngCachedEC2Instances for the shared contract).
func checkNGEC2(_ context.Context, _ any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	nodegroupName, clusterName := ngIdentity(res)
	if nodegroupName == "" {
		return resource.RelatedCheckResult{TargetType: "ec2", Count: 0}
	}

	ec2List, truncated, ok := ngCachedEC2Instances(cache)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "ec2", Count: -1}
	}

	matches := matchingNGInstances(ec2List, nodegroupName, clusterName)
	var ids []string
	for _, m := range matches {
		ids = append(ids, m.resource.ID)
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("ec2")
	}
	return relatedResult("ec2", ids)
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

// ngInstanceMatch pairs a matched EC2 resource.Resource with its typed
// ec2types.Instance so callers can read whichever fields they need
// (checkNGEC2 needs the resource ID; checkNGEBS needs BlockDeviceMappings)
// without re-asserting RawStruct a second time.
type ngInstanceMatch struct {
	resource resource.Resource
	instance ec2types.Instance
}

// matchingNGInstances scans ec2List for instances tagged with nodegroupName
// via "eks:nodegroup-name" and, when clusterName is non-empty, also matching
// "eks:cluster-name". Shared by checkNGEC2 and checkNGEBS, which previously
// each carried their own verbatim copy of this tag-matching loop.
func matchingNGInstances(ec2List []resource.Resource, nodegroupName, clusterName string) []ngInstanceMatch {
	var matches []ngInstanceMatch
	for _, ec2Res := range ec2List {
		inst, ok := assertStruct[ec2types.Instance](ec2Res.RawStruct)
		if !ok {
			continue
		}
		if tagValue(inst.Tags, "eks:nodegroup-name") != nodegroupName {
			continue
		}
		if clusterName != "" {
			instCluster := tagValue(inst.Tags, "eks:cluster-name")
			if instCluster != "" && instCluster != clusterName {
				continue
			}
		}
		matches = append(matches, ngInstanceMatch{resource: ec2Res, instance: inst})
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
		return resource.RelatedCheckResult{TargetType: "sg", Count: -1}
	}
	if ng.Resources == nil || ng.Resources.RemoteAccessSecurityGroup == nil ||
		*ng.Resources.RemoteAccessSecurityGroup == "" {
		return resource.RelatedCheckResult{TargetType: "sg", Count: 0}
	}
	return relatedResult("sg", []string{*ng.Resources.RemoteAccessSecurityGroup})
}

// checkNGAMI resolves the AMI used by this node group's launch template.
// Pattern A — ec2:DescribeLaunchTemplateVersions if LaunchTemplate is set.
// Managed NGs without a custom launch template: AMI resolution via SSM is deferred; returns Count:0.
func checkNGAMI(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	ng, ok := assertStruct[ekstypes.Nodegroup](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "ami", Count: -1}
	}
	if ng.LaunchTemplate == nil || ng.LaunchTemplate.Id == nil || *ng.LaunchTemplate.Id == "" {
		// Managed NG without custom LT — AMI resolution via SSM deferred.
		return resource.RelatedCheckResult{TargetType: "ami", Count: 0}
	}

	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.EC2 == nil {
		return resource.RelatedCheckResult{TargetType: "ami", Count: -1}
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
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) && apiErr.ErrorCode() == "InvalidLaunchTemplateId.NotFound" {
			return resource.RelatedCheckResult{TargetType: "ami", Count: 0}
		}
		return resource.RelatedCheckResult{TargetType: "ami", Count: -1, Err: err}
	}
	for _, v := range ltOut.LaunchTemplateVersions {
		if v.LaunchTemplateData != nil && v.LaunchTemplateData.ImageId != nil && *v.LaunchTemplateData.ImageId != "" {
			return relatedResult("ami", []string{*v.LaunchTemplateData.ImageId})
		}
	}
	return resource.RelatedCheckResult{TargetType: "ami", Count: 0}
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
		return resource.RelatedCheckResult{TargetType: "ebs", Count: 0}
	}

	ec2List, truncated, ok := ngCachedEC2Instances(cache)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "ebs", Count: -1}
	}

	matches := matchingNGInstances(ec2List, nodegroupName, clusterName)
	seen := make(map[string]struct{})
	var ids []string
	for _, m := range matches {
		for _, bdm := range m.instance.BlockDeviceMappings {
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
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("ebs")
	}
	return relatedResult("ebs", ids)
}

// checkNGSubnet returns the subnet IDs this node group deploys into.
// Pattern F — no AWS call needed; data is in Subnets[] on the Nodegroup struct.
func checkNGSubnet(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	ng, ok := assertStruct[ekstypes.Nodegroup](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "subnet", Count: -1}
	}
	var ids []string
	for _, s := range ng.Subnets {
		if s != "" {
			ids = append(ids, s)
		}
	}
	if len(ids) == 0 {
		return resource.RelatedCheckResult{TargetType: "subnet", Count: 0}
	}
	return relatedResult("subnet", ids)
}

// ngCachedEC2Instances reads the "ec2" entry directly from cache — it never
// fetches. checkNGEC2 and checkNGEBS both join against the EC2 cache purely
// as a tag scan (Pattern C), so a cold or missing cache must surface as
// unknown (Count:-1, "?") rather than trigger a live DescribeInstances call.
//
// ok is false (unknown) when:
//   - no "ec2" entry exists in cache at all, or
//   - the entry exists but holds at least one row and NONE of them assert to
//     ec2types.Instance — a disk-seeded cache entry carries rows without
//     RawStruct, so the tag-matching scan below would silently match nothing
//     and be indistinguishable from a genuine zero. An empty (len==0) entry
//     is a legitimate exact-zero source list and is NOT treated as unknown.
func ngCachedEC2Instances(cache resource.ResourceCache) (rows []resource.Resource, truncated bool, ok bool) {
	entry, present := cache["ec2"]
	if !present {
		return nil, false, false
	}
	if len(entry.Resources) > 0 {
		structOK := false
		for _, r := range entry.Resources {
			if _, asserted := assertStruct[ec2types.Instance](r.RawStruct); asserted {
				structOK = true
				break
			}
		}
		if !structOK {
			return nil, false, false
		}
	}
	return entry.Resources, entry.IsTruncated, true
}

// ngRelatedResources returns the resource list for target from cache or by fetching the first page.
func ngRelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}
