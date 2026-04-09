package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func ebsSnapCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("ebs-snap") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("ebs-snap related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("ebs-snap related checker for %s not found", target)
	return nil
}

// --- Navigable Field Registration ---

func TestNavigableFields_EBSSnap_Registered(t *testing.T) {
	fields := resource.GetNavigableFields("ebs-snap")
	if len(fields) == 0 {
		t.Fatal("no navigable fields registered for ebs-snap")
	}

	expected := map[string]string{
		"VolumeId": "ebs",
		"KmsKeyId": "kms",
	}
	for path, targetType := range expected {
		nav := resource.IsFieldNavigable("ebs-snap", path)
		if nav == nil {
			t.Errorf("expected navigable field %q not found", path)
			continue
		}
		if nav.TargetType != targetType {
			t.Errorf("field %q: TargetType = %q, want %q", path, nav.TargetType, targetType)
		}
	}
}

// --- AMI checker (Pattern C — cache-based) ---

func TestRelated_EBSSnap_AMI_Found(t *testing.T) {
	snapID := "snap-abc"
	amiRes := resource.Resource{
		ID: "ami-0a1b2c3d4e5f60001",
		RawStruct: ec2types.Image{
			ImageId: aws.String("ami-0a1b2c3d4e5f60001"),
			BlockDeviceMappings: []ec2types.BlockDeviceMapping{
				{
					Ebs: &ec2types.EbsBlockDevice{
						SnapshotId: aws.String(snapID),
					},
				},
			},
		},
	}
	cache := resource.ResourceCache{
		"ami": resource.ResourceCacheEntry{Resources: []resource.Resource{amiRes}},
	}
	source := resource.Resource{ID: snapID, Fields: map[string]string{}}

	checker := ebsSnapCheckerByTarget(t, "ami")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "ami-0a1b2c3d4e5f60001" {
		t.Errorf("ResourceIDs = %v, want [ami-0a1b2c3d4e5f60001]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_EBSSnap_AMI_NotFound(t *testing.T) {
	amiRes := resource.Resource{
		ID: "ami-0a1b2c3d4e5f60002",
		RawStruct: ec2types.Image{
			ImageId: aws.String("ami-0a1b2c3d4e5f60002"),
			BlockDeviceMappings: []ec2types.BlockDeviceMapping{
				{
					Ebs: &ec2types.EbsBlockDevice{
						SnapshotId: aws.String("snap-different"),
					},
				},
			},
		},
	}
	cache := resource.ResourceCache{
		"ami": resource.ResourceCacheEntry{Resources: []resource.Resource{amiRes}},
	}
	source := resource.Resource{ID: "snap-abc", Fields: map[string]string{}}

	checker := ebsSnapCheckerByTarget(t, "ami")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_EBSSnap_AMI_CacheMissNoClients(t *testing.T) {
	source := resource.Resource{ID: "snap-abc", Fields: map[string]string{}}

	checker := ebsSnapCheckerByTarget(t, "ami")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown)", result.Count)
	}
}

func TestRelated_EBSSnap_AMI_EmptySnapID(t *testing.T) {
	amiRes := resource.Resource{
		ID: "ami-0a1b2c3d4e5f60001",
		RawStruct: ec2types.Image{
			ImageId: aws.String("ami-0a1b2c3d4e5f60001"),
			BlockDeviceMappings: []ec2types.BlockDeviceMapping{
				{
					Ebs: &ec2types.EbsBlockDevice{
						SnapshotId: aws.String("snap-abc"),
					},
				},
			},
		},
	}
	cache := resource.ResourceCache{
		"ami": resource.ResourceCacheEntry{Resources: []resource.Resource{amiRes}},
	}
	source := resource.Resource{ID: "", Fields: map[string]string{}}

	checker := ebsSnapCheckerByTarget(t, "ami")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 for empty snap ID", result.Count)
	}
}

// --- EBS Volume checker (Pattern F) ---

func TestRelated_EBSSnap_EBS_Found(t *testing.T) {
	source := resource.Resource{
		ID:     "snap-abc",
		Fields: map[string]string{"volume_id": "vol-abc"},
	}

	checker := ebsSnapCheckerByTarget(t, "ebs")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "vol-abc" {
		t.Errorf("ResourceIDs = %v, want [vol-abc]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_EBSSnap_EBS_NoVolume(t *testing.T) {
	source := resource.Resource{
		ID:     "snap-abc",
		Fields: map[string]string{"volume_id": ""},
	}

	checker := ebsSnapCheckerByTarget(t, "ebs")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

// --- EC2 checker (Pattern F — parse Description for CreateImage pattern) ---

func TestRelated_EBSSnap_EC2_Found(t *testing.T) {
	source := resource.Resource{
		ID:     "snap-abc",
		Fields: map[string]string{"description": "Created by CreateImage(i-0a1b2c3d4e5f60001) for ami-xxx"},
	}

	checker := ebsSnapCheckerByTarget(t, "ec2")
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

func TestRelated_EBSSnap_EC2_NotFound(t *testing.T) {
	source := resource.Resource{
		ID:     "snap-abc",
		Fields: map[string]string{"description": "Quarterly backup of web-prod-01 root volume"},
	}

	checker := ebsSnapCheckerByTarget(t, "ec2")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_EBSSnap_EC2_EmptyDescription(t *testing.T) {
	source := resource.Resource{
		ID:     "snap-abc",
		Fields: map[string]string{"description": ""},
	}

	checker := ebsSnapCheckerByTarget(t, "ec2")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

// --- KMS checker (Pattern F — assertStruct[ec2types.Snapshot], extract key ID after last "/") ---

func TestRelated_EBSSnap_KMS_Found(t *testing.T) {
	source := resource.Resource{
		ID:        "snap-abc",
		Fields:    map[string]string{},
		RawStruct: ec2types.Snapshot{KmsKeyId: aws.String("arn:aws:kms:us-east-1:123456789012:key/abc-123")},
	}

	checker := ebsSnapCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "abc-123" {
		t.Errorf("ResourceIDs = %v, want [abc-123]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_EBSSnap_KMS_NotEncrypted(t *testing.T) {
	source := resource.Resource{
		ID:        "snap-abc",
		Fields:    map[string]string{},
		RawStruct: ec2types.Snapshot{KmsKeyId: nil},
	}

	checker := ebsSnapCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_EBSSnap_KMS_NoRawStruct(t *testing.T) {
	source := resource.Resource{
		ID:        "snap-abc",
		Fields:    map[string]string{},
		RawStruct: nil,
	}

	checker := ebsSnapCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1", result.Count)
	}
}
