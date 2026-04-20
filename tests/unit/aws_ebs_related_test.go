package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func ebsCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("ebs") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("ebs related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("ebs related checker for %s not found", target)
	return nil
}

// --- Navigable Field Registration ---

func TestNavigableFields_EBS_Registered(t *testing.T) {
	fields := resource.GetNavigableFields("ebs")
	if len(fields) == 0 {
		t.Fatal("no navigable fields registered for ebs")
	}

	expected := map[string]string{
		"Attachments.InstanceId": "ec2",
	}
	for path, targetType := range expected {
		nav := resource.IsFieldNavigable("ebs", path)
		if nav == nil {
			t.Errorf("expected navigable field %q not found", path)
			continue
		}
		if nav.TargetType != targetType {
			t.Errorf("field %q: TargetType = %q, want %q", path, nav.TargetType, targetType)
		}
	}
}

// --- EC2 checker (Pattern F) ---

func TestRelated_EBS_EC2_Found(t *testing.T) {
	source := resource.Resource{
		ID:     "vol-abc",
		Fields: map[string]string{"attached_to": "i-0a1b2c3d4e5f60001"},
	}
	checker := ebsCheckerByTarget(t, "ec2")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "i-0a1b2c3d4e5f60001" {
		t.Errorf("ResourceIDs = %v, want [i-0a1b2c3d4e5f60001]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_EBS_EC2_NotAttached(t *testing.T) {
	source := resource.Resource{
		ID:     "vol-abc",
		Fields: map[string]string{"attached_to": ""},
	}
	checker := ebsCheckerByTarget(t, "ec2")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_EBS_EC2_EmptyID(t *testing.T) {
	source := resource.Resource{ID: "", Fields: map[string]string{"attached_to": ""}}
	checker := ebsCheckerByTarget(t, "ec2")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

// --- ebs-snap checker (Pattern C) ---

func TestRelated_EBS_Snap_Found(t *testing.T) {
	snap := resource.Resource{
		ID:     "snap-0a1b2c3d4e5f60001",
		Fields: map[string]string{"volume_id": "vol-0a1b2c3d4e5f60001"},
	}
	cache := resource.ResourceCache{
		"ebs-snap": resource.ResourceCacheEntry{Resources: []resource.Resource{snap}},
	}
	source := resource.Resource{ID: "vol-0a1b2c3d4e5f60001", Fields: map[string]string{}}

	checker := ebsCheckerByTarget(t, "ebs-snap")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "snap-0a1b2c3d4e5f60001" {
		t.Errorf("ResourceIDs = %v, want [snap-0a1b2c3d4e5f60001]", result.ResourceIDs)
	}
}

func TestRelated_EBS_Snap_NotFound(t *testing.T) {
	snap := resource.Resource{
		ID:     "snap-other",
		Fields: map[string]string{"volume_id": "vol-other"},
	}
	cache := resource.ResourceCache{
		"ebs-snap": resource.ResourceCacheEntry{Resources: []resource.Resource{snap}},
	}
	source := resource.Resource{ID: "vol-0a1b2c3d4e5f60001", Fields: map[string]string{}}

	checker := ebsCheckerByTarget(t, "ebs-snap")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_EBS_Snap_CacheMissNoClients(t *testing.T) {
	source := resource.Resource{ID: "vol-abc", Fields: map[string]string{}}
	checker := ebsCheckerByTarget(t, "ebs-snap")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown)", result.Count)
	}
}

func TestRelated_EBS_Snap_MultipleSnaps(t *testing.T) {
	snap1 := resource.Resource{ID: "snap-001", Fields: map[string]string{"volume_id": "vol-abc"}}
	snap2 := resource.Resource{ID: "snap-002", Fields: map[string]string{"volume_id": "vol-abc"}}
	snap3 := resource.Resource{ID: "snap-003", Fields: map[string]string{"volume_id": "vol-other"}}
	cache := resource.ResourceCache{
		"ebs-snap": resource.ResourceCacheEntry{Resources: []resource.Resource{snap1, snap2, snap3}},
	}
	source := resource.Resource{ID: "vol-abc", Fields: map[string]string{}}

	checker := ebsCheckerByTarget(t, "ebs-snap")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 2 {
		t.Errorf("Count = %d, want 2", result.Count)
	}
}

// --- KMS checker (Pattern F) ---

func TestRelated_EBS_KMS_Found(t *testing.T) {
	source := resource.Resource{
		ID:     "vol-abc",
		Fields: map[string]string{},
		RawStruct: ec2types.Volume{
			KmsKeyId: aws.String("arn:aws:kms:us-east-1:123456789012:key/a1b2c3d4-5678-90ab-cdef-111111111111"),
		},
	}
	checker := ebsCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "a1b2c3d4-5678-90ab-cdef-111111111111" {
		t.Errorf("ResourceIDs = %v, want [a1b2c3d4-5678-90ab-cdef-111111111111]", result.ResourceIDs)
	}
}

func TestRelated_EBS_KMS_NotEncrypted(t *testing.T) {
	source := resource.Resource{
		ID:        "vol-abc",
		Fields:    map[string]string{},
		RawStruct: ec2types.Volume{KmsKeyId: nil},
	}
	checker := ebsCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_EBS_KMS_BadRawStruct(t *testing.T) {
	source := resource.Resource{
		ID:        "vol-abc",
		Fields:    map[string]string{},
		RawStruct: nil,
	}
	checker := ebsCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkEBSAlarm — Pattern D: VolumeId dimension search in alarm cache
// ---------------------------------------------------------------------------

// TestRelated_EBS_Alarm_MatchByVolumeId verifies that a cache alarm whose
// VolumeId dimension matches the volume ID is returned.
func TestRelated_EBS_Alarm_MatchByVolumeId(t *testing.T) {
	const volID = "vol-0a1b2c3d4e5f60001"
	const alarmName = "ebs-vol-high-queue-depth"
	dimName := "VolumeId"
	dimVal := volID
	alarmRes := resource.Resource{
		ID: alarmName,
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String(alarmName),
			Dimensions: []cwtypes.Dimension{
				{Name: &dimName, Value: &dimVal},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}
	source := resource.Resource{ID: volID, Fields: map[string]string{}}

	checker := ebsCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != alarmName {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, alarmName)
	}
}

// TestRelated_EBS_Alarm_NoMatchOtherDimension verifies Count=0 when the cache
// alarm's VolumeId dimension does not match this volume's ID.
func TestRelated_EBS_Alarm_NoMatchOtherDimension(t *testing.T) {
	dimName := "VolumeId"
	dimVal := "vol-other"
	alarmRes := resource.Resource{
		ID:     "other-alarm",
		RawStruct: cwtypes.MetricAlarm{
			Dimensions: []cwtypes.Dimension{{Name: &dimName, Value: &dimVal}},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}
	source := resource.Resource{ID: "vol-0a1b2c3d4e5f60001", Fields: map[string]string{}}

	checker := ebsCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no matching VolumeId)", result.Count)
	}
}

// TestRelated_EBS_Alarm_EmptyVolumeIDReturnsZero verifies Count=0 when the
// source volume has no ID (short-circuit before cache lookup).
func TestRelated_EBS_Alarm_EmptyVolumeIDReturnsZero(t *testing.T) {
	source := resource.Resource{ID: "", Fields: map[string]string{}}
	checker := ebsCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty volume ID)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkEBSCFN — Pattern C: aws:cloudformation:stack-name tag on Volume.Tags
// ---------------------------------------------------------------------------

// TestRelated_EBS_CFN_MatchByStackTag verifies that a volume with a
// cloudformation stack-name tag resolves the stack from the cfn cache.
func TestRelated_EBS_CFN_MatchByStackTag(t *testing.T) {
	const stackName = "acme-infra-stack"
	tagKey := "aws:cloudformation:stack-name"
	tagVal := stackName
	cfnRes := resource.Resource{
		ID:   stackName,
		Name: stackName,
		RawStruct: cfntypes.Stack{
			StackName: aws.String(stackName),
		},
	}
	cache := resource.ResourceCache{
		"cfn": resource.ResourceCacheEntry{Resources: []resource.Resource{cfnRes}},
	}
	source := resource.Resource{
		ID:     "vol-0a1b2c3d4e5f60001",
		Fields: map[string]string{},
		RawStruct: ec2types.Volume{
			Tags: []ec2types.Tag{{Key: &tagKey, Value: &tagVal}},
		},
	}

	checker := ebsCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != stackName {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, stackName)
	}
}

// TestRelated_EBS_CFN_NoStackTagReturnsZero verifies Count=0 when the volume
// has no aws:cloudformation:stack-name tag.
func TestRelated_EBS_CFN_NoStackTagReturnsZero(t *testing.T) {
	source := resource.Resource{
		ID:        "vol-0a1b2c3d4e5f60001",
		Fields:    map[string]string{},
		RawStruct: ec2types.Volume{Tags: []ec2types.Tag{}},
	}
	checker := ebsCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no CFN tag)", result.Count)
	}
}

// TestRelated_EBS_CFN_InvalidRawStructReturnsZero verifies Count=0 when the
// RawStruct is not an ec2types.Volume (cannot read tags).
func TestRelated_EBS_CFN_InvalidRawStructReturnsZero(t *testing.T) {
	source := resource.Resource{
		ID:        "vol-abc",
		Fields:    map[string]string{},
		RawStruct: "not-a-volume",
	}
	checker := ebsCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (bad raw struct, no tag possible)", result.Count)
	}
}
