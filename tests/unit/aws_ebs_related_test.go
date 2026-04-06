package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/fieldpath"
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

func TestNavigableFields_EBS_FieldPathsResolve(t *testing.T) {
	resources, ok := demo.GetResources("ebs")
	if !ok {
		t.Fatal("no demo fixture registered for ebs — fixtures_compute.go must register it")
	}
	if len(resources) == 0 {
		t.Fatal("demo fixture returned no resources for ebs")
	}
	r := resources[0]

	fields := resource.GetNavigableFields("ebs")
	if len(fields) == 0 {
		t.Fatal("no navigable fields registered for ebs")
	}

	for _, nav := range fields {
		items := fieldpath.ExtractFieldList(r.RawStruct, r.Fields, []string{nav.FieldPath}, nil)
		found := false
		for _, item := range items {
			if item.Value != "" && item.Value != "-" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("NavigableField.FieldPath %q resolved to empty/missing value in demo fixture", nav.FieldPath)
		}
	}
}

// --- Demo Checker ---

func TestRelatedDemo_EBS_Registered(t *testing.T) {
	_ = demo.GetResources
	checker := resource.GetRelatedDemo("ebs")
	if checker == nil {
		t.Fatal("no demo checker registered for ebs")
	}

	results := checker(resource.Resource{ID: "vol-0a1b2c3d4e5f60001"})
	if len(results) == 0 {
		t.Fatal("demo checker returned no results")
	}
	for _, r := range results {
		if r.TargetType == "" {
			t.Error("demo result has empty TargetType")
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
