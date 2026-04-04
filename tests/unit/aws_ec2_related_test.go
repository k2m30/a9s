package unit

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func ec2CheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("ec2") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("ec2 related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("ec2 related checker for %s not found", target)
	return nil
}

func TestEC2RelatedCheckers_NoUnknownCounts(t *testing.T) {
	instance := resource.Resource{
		ID: "i-abc123",
		RawStruct: ec2types.Instance{
			InstanceId: aws.String("i-abc123"),
			VpcId:      aws.String("vpc-123"),
			Tags: []ec2types.Tag{
				{Key: aws.String("aws:cloudformation:stack-name"), Value: aws.String("app-stack")},
			},
			BlockDeviceMappings: []ec2types.InstanceBlockDeviceMapping{
				{Ebs: &ec2types.EbsInstanceBlockDevice{VolumeId: aws.String("vol-abc")}},
			},
		},
	}

	cache := resource.ResourceCache{
		"tg": {Resources: []resource.Resource{
			{
				ID: "tg-web",
				RawStruct: elbv2types.TargetGroup{
					VpcId:      aws.String("vpc-123"),
					TargetType: elbv2types.TargetTypeEnumInstance,
				},
			},
		}},
		"asg": {Resources: []resource.Resource{
			{
				ID: "asg-web",
				RawStruct: asgtypes.AutoScalingGroup{
					Instances: []asgtypes.Instance{{InstanceId: aws.String("i-abc123")}},
				},
			},
		}},
		"alarm": {Resources: []resource.Resource{
			{
				ID: "cpu-high",
				RawStruct: cwtypes.MetricAlarm{
					Dimensions: []cwtypes.Dimension{{Name: aws.String("InstanceId"), Value: aws.String("i-abc123")}},
				},
			},
		}},
		"cfn": {Resources: []resource.Resource{
			{
				ID: "app-stack",
				RawStruct: cfntypes.Stack{
					StackName: aws.String("app-stack"),
				},
			},
		}},
		"eip": {Resources: []resource.Resource{
			{
				ID: "eipalloc-abc",
				RawStruct: ec2types.Address{
					InstanceId: aws.String("i-abc123"),
				},
			},
		}},
		"ebs-snap": {Resources: []resource.Resource{
			{
				ID: "snap-abc",
				RawStruct: ec2types.Snapshot{
					VolumeId: aws.String("vol-abc"),
				},
			},
		}},
	}

	targets := []string{"tg", "asg", "alarm", "cfn", "eip", "ebs-snap"}
	for _, target := range targets {
		checker := ec2CheckerByTarget(t, target)
		got := checker(context.Background(), nil, instance, cache)
		if got.Count < 0 {
			t.Fatalf("%s checker returned unknown count: %+v", target, got)
		}
		if got.Count == 0 {
			t.Fatalf("%s checker returned zero count with matching fixture cache: %+v", target, got)
		}
		if len(got.ResourceIDs) == 0 {
			t.Fatalf("%s checker returned empty ResourceIDs with positive count: %+v", target, got)
		}
	}
}

func TestEC2NavigableFields_IncludeSecurityGroupsGroupID(t *testing.T) {
	fields := resource.GetNavigableFields("ec2")
	if len(fields) == 0 {
		t.Fatal("resource.GetNavigableFields(\"ec2\") returned empty")
	}

	found := false
	for _, f := range fields {
		if f.FieldPath == "SecurityGroups.GroupId" && f.TargetType == "sg" {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("ec2 navigable fields must include SecurityGroups.GroupId -> sg; got: %+v", fields)
	}
}

// Bug reveal: real-profile EC2 detail counts currently depend on the destination
// list already being present in cache. Related counts should be derivable without
// pre-warming :asg/:eip first.
func TestEC2RelatedCheckers_ASGDoesNotRequirePrewarmedCache(t *testing.T) {
	instance := resource.Resource{
		ID: "i-real-asg",
		RawStruct: ec2types.Instance{
			InstanceId: aws.String("i-real-asg"),
			VpcId:      aws.String("vpc-123"),
		},
	}

	checker := ec2CheckerByTarget(t, "asg")
	got := checker(context.Background(), nil, instance, resource.ResourceCache{})

	if got.Count == -1 {
		t.Fatalf("asg related count should not be unknown just because the asg list was not preloaded; got %+v", got)
	}
}

// Bug reveal: EIP count is also cache-dependent today.
func TestEC2RelatedCheckers_EIPDoesNotRequirePrewarmedCache(t *testing.T) {
	instance := resource.Resource{
		ID: "i-real-eip",
		RawStruct: ec2types.Instance{
			InstanceId: aws.String("i-real-eip"),
			VpcId:      aws.String("vpc-123"),
		},
	}

	checker := ec2CheckerByTarget(t, "eip")
	got := checker(context.Background(), nil, instance, resource.ResourceCache{})

	if got.Count == -1 {
		t.Fatalf("eip related count should not be unknown just because the eip list was not preloaded; got %+v", got)
	}
}

// Bug reveal: live EC2 detail shows EKS node group rows but they are not backed
// by a checker, so the row never becomes actionable.
func TestEC2RelatedRegistry_NodeGroupsHasChecker(t *testing.T) {
	for _, def := range resource.GetRelated("ec2") {
		if def.TargetType != "ng" {
			continue
		}
		if def.Checker == nil {
			t.Fatal("ec2 related definition for ng must have a checker so EKS node group relationships can be counted and opened")
		}
		return
	}
	t.Fatal("ec2 related definition for ng not found")
}

// Bug reveal: live EC2 detail shows CloudTrail Events but the row is currently a
// non-counted placeholder.
func TestEC2RelatedRegistry_CloudTrailEventsHasChecker(t *testing.T) {
	for _, def := range resource.GetRelated("ec2") {
		if def.TargetType != "ct-events" {
			continue
		}
		if def.Checker == nil {
			t.Fatal("ec2 related definition for ct-events must have a checker so CloudTrail event relationships can be counted and opened")
		}
		return
	}
	t.Fatal("ec2 related definition for ct-events not found")
}

// ---------------------------------------------------------------------------
// checkEC2EBS — direct checker coverage (0% before this test)
// ---------------------------------------------------------------------------

func TestEC2RelatedCheckers_EBS_MatchesVolumeIDs(t *testing.T) {
	instance := resource.Resource{
		ID: "i-ebs-multi",
		RawStruct: ec2types.Instance{
			InstanceId: aws.String("i-ebs-multi"),
			BlockDeviceMappings: []ec2types.InstanceBlockDeviceMapping{
				{Ebs: &ec2types.EbsInstanceBlockDevice{VolumeId: aws.String("vol-abc")}},
				{Ebs: &ec2types.EbsInstanceBlockDevice{VolumeId: aws.String("vol-def")}},
			},
		},
	}

	checker := ec2CheckerByTarget(t, "ebs")
	got := checker(context.Background(), nil, instance, resource.ResourceCache{})

	if got.Count != 2 {
		t.Errorf("checkEC2EBS: expected count=2, got %d", got.Count)
	}
	if len(got.ResourceIDs) != 2 {
		t.Errorf("checkEC2EBS: expected 2 ResourceIDs, got %v", got.ResourceIDs)
	}
	// ResourceIDs should be sorted
	if got.ResourceIDs[0] != "vol-abc" || got.ResourceIDs[1] != "vol-def" {
		t.Errorf("checkEC2EBS: expected sorted [vol-abc, vol-def], got %v", got.ResourceIDs)
	}
}

func TestEC2RelatedCheckers_EBS_NoVolumes(t *testing.T) {
	instance := resource.Resource{
		ID: "i-ebs-none",
		RawStruct: ec2types.Instance{
			InstanceId:          aws.String("i-ebs-none"),
			BlockDeviceMappings: []ec2types.InstanceBlockDeviceMapping{},
		},
	}

	checker := ec2CheckerByTarget(t, "ebs")
	got := checker(context.Background(), nil, instance, resource.ResourceCache{})

	if got.Count != 0 {
		t.Errorf("checkEC2EBS with empty BlockDeviceMappings: expected count=0, got %d", got.Count)
	}
	if len(got.ResourceIDs) != 0 {
		t.Errorf("checkEC2EBS with empty BlockDeviceMappings: expected no ResourceIDs, got %v", got.ResourceIDs)
	}
}

func TestEC2RelatedCheckers_EBS_NilEbs(t *testing.T) {
	instance := resource.Resource{
		ID: "i-ebs-nil",
		RawStruct: ec2types.Instance{
			InstanceId: aws.String("i-ebs-nil"),
			BlockDeviceMappings: []ec2types.InstanceBlockDeviceMapping{
				{Ebs: nil}, // nil Ebs field — should not panic
			},
		},
	}

	checker := ec2CheckerByTarget(t, "ebs")
	// Must not panic
	got := checker(context.Background(), nil, instance, resource.ResourceCache{})

	if got.Count != 0 {
		t.Errorf("checkEC2EBS with nil Ebs: expected count=0, got %d", got.Count)
	}
}

func TestEC2RelatedCheckers_EBS_NilVolumeId(t *testing.T) {
	instance := resource.Resource{
		ID: "i-ebs-nil-volid",
		RawStruct: ec2types.Instance{
			InstanceId: aws.String("i-ebs-nil-volid"),
			BlockDeviceMappings: []ec2types.InstanceBlockDeviceMapping{
				{Ebs: &ec2types.EbsInstanceBlockDevice{VolumeId: nil}}, // Ebs present but VolumeId nil
			},
		},
	}

	checker := ec2CheckerByTarget(t, "ebs")
	got := checker(context.Background(), nil, instance, resource.ResourceCache{})

	if got.Count != 0 {
		t.Errorf("checkEC2EBS with nil VolumeId: expected count=0, got %d", got.Count)
	}
}

func TestEC2RelatedCheckers_EBS_SingleVolume(t *testing.T) {
	instance := resource.Resource{
		ID: "i-ebs-single",
		RawStruct: ec2types.Instance{
			InstanceId: aws.String("i-ebs-single"),
			BlockDeviceMappings: []ec2types.InstanceBlockDeviceMapping{
				{Ebs: &ec2types.EbsInstanceBlockDevice{VolumeId: aws.String("vol-only")}},
			},
		},
	}

	checker := ec2CheckerByTarget(t, "ebs")
	got := checker(context.Background(), nil, instance, resource.ResourceCache{})

	if got.Count != 1 {
		t.Errorf("checkEC2EBS single volume: expected count=1, got %d", got.Count)
	}
	if len(got.ResourceIDs) != 1 || got.ResourceIDs[0] != "vol-only" {
		t.Errorf("checkEC2EBS single volume: expected ResourceIDs=[vol-only], got %v", got.ResourceIDs)
	}
}

func TestEC2RelatedCheckers_EBS_DeduplicatesVolumeIDs(t *testing.T) {
	// If somehow the same volume appears twice in BlockDeviceMappings, it should be counted once.
	instance := resource.Resource{
		ID: "i-ebs-dedup",
		RawStruct: ec2types.Instance{
			InstanceId: aws.String("i-ebs-dedup"),
			BlockDeviceMappings: []ec2types.InstanceBlockDeviceMapping{
				{Ebs: &ec2types.EbsInstanceBlockDevice{VolumeId: aws.String("vol-dup")}},
				{Ebs: &ec2types.EbsInstanceBlockDevice{VolumeId: aws.String("vol-dup")}},
			},
		},
	}

	checker := ec2CheckerByTarget(t, "ebs")
	got := checker(context.Background(), nil, instance, resource.ResourceCache{})

	if got.Count != 1 {
		t.Errorf("checkEC2EBS should deduplicate volume IDs; expected count=1, got %d", got.Count)
	}
}

func TestEC2RelatedCheckers_EBS_NonEC2RawStruct(t *testing.T) {
	// When RawStruct is not an ec2types.Instance, should return count=0 without panic.
	instance := resource.Resource{
		ID:        "i-wrong-type",
		RawStruct: "not-an-ec2-instance",
	}

	checker := ec2CheckerByTarget(t, "ebs")
	got := checker(context.Background(), nil, instance, resource.ResourceCache{})

	if got.Count != 0 {
		t.Errorf("checkEC2EBS with wrong RawStruct type: expected count=0, got %d", got.Count)
	}
}

// TestResourceCacheEntry_IsTruncated_Propagates verifies that when the cache
// has IsTruncated=true for a target type and no matching resources are found,
// the related checker returns Count=-1 ("?") rather than Count=0.
//
// Failing with current code because ResourceCacheEntry type doesn't exist yet
// (Phase 1: #218 ResourceCache type change).
func TestResourceCacheEntry_IsTruncated_Propagates(t *testing.T) {
	instance := resource.Resource{
		ID: "i-truncated-test",
		RawStruct: ec2types.Instance{
			InstanceId: aws.String("i-truncated-test"),
			VpcId:      aws.String("vpc-999"),
		},
	}

	// Cache has alarm data but it's truncated — and none of the alarms match
	// this instance. The checker should return Count=-1 (unknown) not Count=0.
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{
					ID: "alarm-other-instance",
					RawStruct: cwtypes.MetricAlarm{
						Dimensions: []cwtypes.Dimension{
							{Name: aws.String("InstanceId"), Value: aws.String("i-other")},
						},
					},
				},
			},
			IsTruncated: true,
		},
	}

	checker := ec2CheckerByTarget(t, "alarm")
	got := checker(context.Background(), nil, instance, cache)

	if got.Count != -1 {
		t.Errorf("alarm checker with truncated cache and 0 matches: want Count=-1, got Count=%d", got.Count)
	}
}
