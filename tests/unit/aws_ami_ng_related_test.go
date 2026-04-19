package unit_test

// aws_ami_ng_related_test.go — Failing tests for the AMI→NG related checker.
//
// checkAMING currently has a comment noting it "will never match" because
// Fields["image_id"] is absent from nodegroup resources. These tests pin the
// expected behaviour AFTER the coder:
//
//  1. Extends FetchNodeGroups to resolve and populate Fields["image_id"] from the
//     nodegroup's custom LaunchTemplate via EC2 DescribeLaunchTemplateVersions.
//  2. Removes the TODO comment from checkAMING in ami_related_extra.go.
//
// Tests are RED until both changes land.

import (
	"context"
	"testing"

	_ "github.com/k2m30/a9s/v3/internal/aws" // ensure all related registrations run
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// Helper: find the AMI→NG checker from the registry
// ---------------------------------------------------------------------------

func findCheckAMING(t *testing.T) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("ami") {
		if def.TargetType == "ng" {
			return def.Checker
		}
	}
	t.Fatal("checkAMING not registered under ami→ng")
	return nil
}

// ---------------------------------------------------------------------------
// T-AMI-NG01: Checker matches when NG has Fields["image_id"] == AMI ID
// ---------------------------------------------------------------------------

func TestCheckAMING_MatchesWhenNGImageIDMatches(t *testing.T) {
	checker := findCheckAMING(t)

	amiResource := resource.Resource{
		ID:   "ami-xyz",
		Name: "my-golden-ami",
		Fields: map[string]string{
			"state":      "available",
			"image_type": "machine",
		},
	}

	cache := resource.ResourceCache{
		"ng": {
			Resources: []resource.Resource{
				{
					ID:   "ng-custom",
					Name: "ng-custom",
					Fields: map[string]string{
						"nodegroup_name": "ng-custom",
						"cluster_name":   "prod-cluster",
						"status":         "ACTIVE",
						"instance_types": "m5.large",
						"desired_size":   "2",
						"image_id":       "ami-xyz", // resoled from custom LaunchTemplate
					},
				},
			},
			IsTruncated: false,
		},
	}

	result := checker(context.Background(), nil, amiResource, cache)

	if result.TargetType != "ng" {
		t.Errorf("TargetType = %q, want \"ng\"", result.TargetType)
	}
	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (ng-custom uses ami-xyz)", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "ng-custom" {
		t.Errorf("ResourceIDs = %v, want [\"ng-custom\"]", result.ResourceIDs)
	}
	if result.Approximate {
		t.Error("Approximate = true, want false (non-truncated cache with full match)")
	}
	if result.Err != nil {
		t.Errorf("Err = %v, want nil", result.Err)
	}
}

// ---------------------------------------------------------------------------
// T-AMI-NG02: No match when image_id differs
// ---------------------------------------------------------------------------

func TestCheckAMING_NoMatchWhenImageIDDiffers(t *testing.T) {
	checker := findCheckAMING(t)

	amiResource := resource.Resource{
		ID:   "ami-xyz",
		Name: "my-golden-ami",
		Fields: map[string]string{
			"state": "available",
		},
	}

	cache := resource.ResourceCache{
		"ng": {
			Resources: []resource.Resource{
				{
					ID:   "ng-other",
					Name: "ng-other",
					Fields: map[string]string{
						"nodegroup_name": "ng-other",
						"cluster_name":   "prod-cluster",
						"status":         "ACTIVE",
						"instance_types": "t3.large",
						"desired_size":   "3",
						"image_id":       "ami-other", // different AMI
					},
				},
			},
			IsTruncated: false,
		},
	}

	result := checker(context.Background(), nil, amiResource, cache)

	if result.TargetType != "ng" {
		t.Errorf("TargetType = %q, want \"ng\"", result.TargetType)
	}
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (different AMI ID)", result.Count)
	}
	if result.Approximate {
		t.Error("Approximate = true, want false (non-truncated cache — definitive zero)")
	}
}

// ---------------------------------------------------------------------------
// T-AMI-NG03: Approximate=true when cache is truncated and no match found
// ---------------------------------------------------------------------------

func TestCheckAMING_ApproximateWhenCacheTruncatedAndNoMatch(t *testing.T) {
	checker := findCheckAMING(t)

	amiResource := resource.Resource{
		ID:   "ami-xyz",
		Name: "my-golden-ami",
		Fields: map[string]string{
			"state": "available",
		},
	}

	// Truncated NG cache — partial page with a nodegroup that uses a different AMI.
	// The checker cannot guarantee that the full fleet has no NG using ami-xyz.
	cache := resource.ResourceCache{
		"ng": {
			Resources: []resource.Resource{
				{
					ID:   "ng-page1",
					Name: "ng-page1",
					Fields: map[string]string{
						"nodegroup_name": "ng-page1",
						"cluster_name":   "prod-cluster",
						"status":         "ACTIVE",
						"instance_types": "r5.large",
						"desired_size":   "5",
						"image_id":       "ami-other", // different AMI
					},
				},
			},
			IsTruncated: true, // more pages exist — cannot be certain
		},
	}

	result := checker(context.Background(), nil, amiResource, cache)

	if result.TargetType != "ng" {
		t.Errorf("TargetType = %q, want \"ng\"", result.TargetType)
	}
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
	if !result.Approximate {
		t.Errorf("Approximate = false, want true (truncated cache — lower bound only). Result: %+v", result)
	}
	if result.Err != nil {
		t.Errorf("Err = %v, want nil", result.Err)
	}
}

// ---------------------------------------------------------------------------
// T-AMI-NG04: Multiple NGs — only matching ones counted
// ---------------------------------------------------------------------------

func TestCheckAMING_CountsOnlyMatchingNGs(t *testing.T) {
	checker := findCheckAMING(t)

	amiResource := resource.Resource{
		ID:   "ami-shared",
		Name: "shared-ami",
		Fields: map[string]string{
			"state": "available",
		},
	}

	cache := resource.ResourceCache{
		"ng": {
			Resources: []resource.Resource{
				{
					ID:     "ng-a",
					Name:   "ng-a",
					Fields: map[string]string{"image_id": "ami-shared"},
				},
				{
					ID:     "ng-b",
					Name:   "ng-b",
					Fields: map[string]string{"image_id": "ami-other"},
				},
				{
					ID:     "ng-c",
					Name:   "ng-c",
					Fields: map[string]string{"image_id": "ami-shared"},
				},
				{
					ID:     "ng-d",
					Name:   "ng-d",
					Fields: map[string]string{"image_id": ""}, // EKS-managed, no image_id
				},
			},
			IsTruncated: false,
		},
	}

	result := checker(context.Background(), nil, amiResource, cache)

	if result.Count != 2 {
		t.Errorf("Count = %d, want 2 (ng-a and ng-c use ami-shared)", result.Count)
	}
	if len(result.ResourceIDs) != 2 {
		t.Errorf("ResourceIDs length = %d, want 2; got %v", len(result.ResourceIDs), result.ResourceIDs)
	}
	// Verify both matching nodegroups are present in ResourceIDs
	found := make(map[string]bool)
	for _, id := range result.ResourceIDs {
		found[id] = true
	}
	if !found["ng-a"] {
		t.Error("ResourceIDs missing \"ng-a\"")
	}
	if !found["ng-c"] {
		t.Error("ResourceIDs missing \"ng-c\"")
	}
}

// ---------------------------------------------------------------------------
// T-AMI-NG05: AMI with empty ID returns Count=0 without touching cache
// ---------------------------------------------------------------------------

func TestCheckAMING_EmptyAMIIDReturnsZero(t *testing.T) {
	checker := findCheckAMING(t)

	amiResource := resource.Resource{
		ID:   "", // degenerate — should never happen in practice
		Name: "unnamed-ami",
		Fields: map[string]string{
			"state": "available",
		},
	}

	cache := resource.ResourceCache{
		"ng": {
			Resources: []resource.Resource{
				{
					ID:     "ng-a",
					Name:   "ng-a",
					Fields: map[string]string{"image_id": "ami-real"},
				},
			},
			IsTruncated: false,
		},
	}

	result := checker(context.Background(), nil, amiResource, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty AMI ID is an immediate non-match)", result.Count)
	}
	if result.Approximate {
		t.Error("Approximate = true, want false (empty ID is a definitive non-match)")
	}
}
