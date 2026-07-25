package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"

	_ "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// aws_ng_ebs_cache_join_test.go pins the target behavior for a rewrite of
// checkNGEBS (core/aws/ng_related.go:241) from a two-AWS-call checker
// (autoscaling:DescribeAutoScalingGroups + ec2:DescribeInstances) to a
// zero-call cache join mirroring checkNGEC2: scan the "ec2" cache entry,
// match instances by tag "eks:nodegroup-name" (guarded by "eks:cluster-name"
// when present), then collect BlockDeviceMappings[].Ebs.VolumeId.
//
// These pins are RED against the current two-AWS-call implementation.

func TestRelated_NG_EBS_CacheJoin_MatchByNodegroupTag(t *testing.T) {
	const ngName = "general-pool"
	const clusterName = "prod-cluster"

	matchedInst1 := resource.Resource{
		ID:   "i-0abc111111111aaaa",
		Name: "i-0abc111111111aaaa",
		RawStruct: ec2types.Instance{
			InstanceId: aws.String("i-0abc111111111aaaa"),
			Tags: []ec2types.Tag{
				{Key: aws.String("eks:nodegroup-name"), Value: aws.String(ngName)},
				{Key: aws.String("eks:cluster-name"), Value: aws.String(clusterName)},
			},
			BlockDeviceMappings: []ec2types.InstanceBlockDeviceMapping{
				{
					DeviceName: aws.String("/dev/xvda"),
					Ebs:        &ec2types.EbsInstanceBlockDevice{VolumeId: aws.String("vol-0abc000000000shared")},
				},
				{
					DeviceName: aws.String("/dev/xvdb"),
					Ebs:        &ec2types.EbsInstanceBlockDevice{VolumeId: aws.String("vol-0abc000000000data1")},
				},
			},
		},
	}
	matchedInst2 := resource.Resource{
		ID:   "i-0abc222222222bbbb",
		Name: "i-0abc222222222bbbb",
		RawStruct: ec2types.Instance{
			InstanceId: aws.String("i-0abc222222222bbbb"),
			Tags: []ec2types.Tag{
				{Key: aws.String("eks:nodegroup-name"), Value: aws.String(ngName)},
				{Key: aws.String("eks:cluster-name"), Value: aws.String(clusterName)},
			},
			BlockDeviceMappings: []ec2types.InstanceBlockDeviceMapping{
				{
					// Shared volume ID with matchedInst1 to prove dedup.
					DeviceName: aws.String("/dev/xvda"),
					Ebs:        &ec2types.EbsInstanceBlockDevice{VolumeId: aws.String("vol-0abc000000000shared")},
				},
			},
		},
	}
	otherNGInst := resource.Resource{
		ID:   "i-0abc999999999zzzz",
		Name: "i-0abc999999999zzzz",
		RawStruct: ec2types.Instance{
			InstanceId: aws.String("i-0abc999999999zzzz"),
			Tags: []ec2types.Tag{
				{Key: aws.String("eks:nodegroup-name"), Value: aws.String("other-pool")},
				{Key: aws.String("eks:cluster-name"), Value: aws.String(clusterName)},
			},
			BlockDeviceMappings: []ec2types.InstanceBlockDeviceMapping{
				{
					DeviceName: aws.String("/dev/xvda"),
					Ebs:        &ec2types.EbsInstanceBlockDevice{VolumeId: aws.String("vol-0abc000000000other1")},
				},
			},
		},
	}

	cache := resource.ResourceCache{
		"ec2": resource.ResourceCacheEntry{
			Resources: []resource.Resource{matchedInst1, matchedInst2, otherNGInst},
		},
	}
	source := resource.Resource{
		ID:   ngName,
		Name: ngName,
		Fields: map[string]string{
			"nodegroup_name": ngName,
			"cluster_name":   clusterName,
		},
		RawStruct: ekstypes.Nodegroup{
			NodegroupName: aws.String(ngName),
			ClusterName:   aws.String(clusterName),
		},
	}

	checker := ngCheckerByTarget(t, "ebs")
	// New contract needs NO AWS clients — pass nil.
	result := checker(context.Background(), nil, source, cache)

	if result.Err() != nil {
		t.Fatalf("unexpected error: %v", result.Err())
	}
	if result.Count() != 2 {
		t.Errorf("Count = %d, want 2 (deduped volumes)", result.Count())
	}
	wantIDs := map[string]bool{
		"vol-0abc000000000shared": true,
		"vol-0abc000000000data1":  true,
	}
	if len(result.ResourceIDs()) != len(wantIDs) {
		t.Fatalf("ResourceIDs = %v, want 2 deduped volume IDs matching %v", result.ResourceIDs(), wantIDs)
	}
	for _, id := range result.ResourceIDs() {
		if !wantIDs[id] {
			t.Errorf("unexpected volume ID in result: %q", id)
		}
	}
	for id := range wantIDs {
		found := false
		for _, got := range result.ResourceIDs() {
			if got == id {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected volume ID %q missing from result %v", id, result.ResourceIDs())
		}
	}
}

// TestRelated_NG_EBS_CacheJoin_CrossClusterNodegroupNameCollision pins the
// "eks:cluster-name" guard: an instance tagged with the SAME nodegroup name
// as the source ("general-pool") but a DIFFERENT cluster ("other-cluster")
// must be excluded from the volume join, even though the source nodegroup
// belongs to "prod-cluster". Without the cluster-name guard, two clusters
// that happen to name a nodegroup identically would leak each other's EBS
// volumes into the RELATED panel.
func TestRelated_NG_EBS_CacheJoin_CrossClusterNodegroupNameCollision(t *testing.T) {
	const ngName = "general-pool"
	const sourceCluster = "prod-cluster"
	const otherCluster = "other-cluster"

	matchedInst := resource.Resource{
		ID:   "i-0abc111111111aaaa",
		Name: "i-0abc111111111aaaa",
		RawStruct: ec2types.Instance{
			InstanceId: aws.String("i-0abc111111111aaaa"),
			Tags: []ec2types.Tag{
				{Key: aws.String("eks:nodegroup-name"), Value: aws.String(ngName)},
				{Key: aws.String("eks:cluster-name"), Value: aws.String(sourceCluster)},
			},
			BlockDeviceMappings: []ec2types.InstanceBlockDeviceMapping{
				{
					DeviceName: aws.String("/dev/xvda"),
					Ebs:        &ec2types.EbsInstanceBlockDevice{VolumeId: aws.String("vol-0abc000000000prod01")},
				},
			},
		},
	}
	crossClusterInst := resource.Resource{
		ID:   "i-0abc333333333cccc",
		Name: "i-0abc333333333cccc",
		RawStruct: ec2types.Instance{
			InstanceId: aws.String("i-0abc333333333cccc"),
			Tags: []ec2types.Tag{
				{Key: aws.String("eks:nodegroup-name"), Value: aws.String(ngName)},
				{Key: aws.String("eks:cluster-name"), Value: aws.String(otherCluster)},
			},
			BlockDeviceMappings: []ec2types.InstanceBlockDeviceMapping{
				{
					DeviceName: aws.String("/dev/xvda"),
					Ebs:        &ec2types.EbsInstanceBlockDevice{VolumeId: aws.String("vol-0abc000000000other99")},
				},
			},
		},
	}

	cache := resource.ResourceCache{
		"ec2": resource.ResourceCacheEntry{
			Resources: []resource.Resource{matchedInst, crossClusterInst},
		},
	}
	source := resource.Resource{
		ID:   ngName,
		Name: ngName,
		Fields: map[string]string{
			"nodegroup_name": ngName,
			"cluster_name":   sourceCluster,
		},
		RawStruct: ekstypes.Nodegroup{
			NodegroupName: aws.String(ngName),
			ClusterName:   aws.String(sourceCluster),
		},
	}

	checker := ngCheckerByTarget(t, "ebs")
	result := checker(context.Background(), nil, source, cache)

	if result.Err() != nil {
		t.Fatalf("unexpected error: %v", result.Err())
	}
	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1 (cross-cluster nodegroup-name collision must be excluded)", result.Count())
	}
	for _, id := range result.ResourceIDs() {
		if id == "vol-0abc000000000other99" {
			t.Errorf("ResourceIDs = %v — volume from cross-cluster instance %q (cluster %q) leaked into join for source cluster %q",
				result.ResourceIDs(), crossClusterInst.ID, otherCluster, sourceCluster)
		}
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "vol-0abc000000000prod01" {
		t.Errorf("ResourceIDs = %v, want [%q]", result.ResourceIDs(), "vol-0abc000000000prod01")
	}
}

func TestRelated_NG_EBS_CacheJoin_TruncatedNoMatch_TruncatedResult(t *testing.T) {
	const ngName = "general-pool"

	otherNGInst := resource.Resource{
		ID:   "i-0abc999999999zzzz",
		Name: "i-0abc999999999zzzz",
		RawStruct: ec2types.Instance{
			InstanceId: aws.String("i-0abc999999999zzzz"),
			Tags: []ec2types.Tag{
				{Key: aws.String("eks:nodegroup-name"), Value: aws.String("other-pool")},
			},
			BlockDeviceMappings: []ec2types.InstanceBlockDeviceMapping{
				{
					DeviceName: aws.String("/dev/xvda"),
					Ebs:        &ec2types.EbsInstanceBlockDevice{VolumeId: aws.String("vol-0abc000000000other1")},
				},
			},
		},
	}
	cache := resource.ResourceCache{
		"ec2": resource.ResourceCacheEntry{
			Resources:   []resource.Resource{otherNGInst},
			IsTruncated: true,
		},
	}
	source := resource.Resource{
		ID:   ngName,
		Name: ngName,
		Fields: map[string]string{
			"nodegroup_name": ngName,
		},
		RawStruct: ekstypes.Nodegroup{
			NodegroupName: aws.String(ngName),
		},
	}

	checker := ngCheckerByTarget(t, "ebs")
	result := checker(context.Background(), nil, source, cache)

	want := resource.KnownRelated("ebs", nil, true)
	if result.TargetType() != want.TargetType() {
		t.Errorf("TargetType = %q, want %q", result.TargetType(), want.TargetType())
	}
	if result.Count() != want.Count() {
		t.Errorf("Count = %d, want %d", result.Count(), want.Count())
	}
	if result.Truncated() != want.Truncated() {
		t.Errorf("Truncated = %v, want %v", result.Truncated(), want.Truncated())
	}
	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
	}
}

func TestRelated_NG_EBS_CacheJoin_NoEC2CacheEntry(t *testing.T) {
	const ngName = "general-pool"

	source := resource.Resource{
		ID:   ngName,
		Name: ngName,
		Fields: map[string]string{
			"nodegroup_name": ngName,
		},
		RawStruct: ekstypes.Nodegroup{
			NodegroupName: aws.String(ngName),
		},
	}
	// No "ec2" key present in the cache map at all.
	cache := resource.ResourceCache{}

	checker := ngCheckerByTarget(t, "ebs")
	result := checker(context.Background(), nil, source, cache)

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (no ec2 cache entry, cannot fetch with nil clients)", result.Count())
	}
}

func TestRelated_NG_EBS_CacheJoin_MatchedInstanceNoBlockDeviceMappings(t *testing.T) {
	const ngName = "general-pool"

	matchedNoBDM := resource.Resource{
		ID:   "i-0abc333333333cccc",
		Name: "i-0abc333333333cccc",
		RawStruct: ec2types.Instance{
			InstanceId: aws.String("i-0abc333333333cccc"),
			Tags: []ec2types.Tag{
				{Key: aws.String("eks:nodegroup-name"), Value: aws.String(ngName)},
			},
			BlockDeviceMappings: nil,
		},
	}
	cache := resource.ResourceCache{
		"ec2": resource.ResourceCacheEntry{
			Resources:   []resource.Resource{matchedNoBDM},
			IsTruncated: false,
		},
	}
	source := resource.Resource{
		ID:   ngName,
		Name: ngName,
		Fields: map[string]string{
			"nodegroup_name": ngName,
		},
		RawStruct: ekstypes.Nodegroup{
			NodegroupName: aws.String(ngName),
			// Non-empty Resources.AutoScalingGroups so the OLD two-call
			// implementation proceeds past its early Count:0 guard and
			// instead falls through to its nil-clients State: RelatedUnknown
			// branch — making this a true RED pin against current HEAD rather than
			// an accidental match on the old short-circuit path. The new
			// cache-join contract must ignore this field entirely.
			Resources: &ekstypes.NodegroupResources{
				AutoScalingGroups: []ekstypes.AutoScalingGroup{
					{Name: aws.String("eks-general-pool-asg")},
				},
			},
		},
	}

	checker := ngCheckerByTarget(t, "ebs")
	result := checker(context.Background(), nil, source, cache)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (matched instance has no BlockDeviceMappings, cache not truncated)", result.Count())
	}
	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
	}
}
