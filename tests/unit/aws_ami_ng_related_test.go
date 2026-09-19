package unit_test

// checkAMING matches on Fields["image_id"], which FetchNodeGroups resolves
// from the nodegroup's custom LaunchTemplate via EC2
// DescribeLaunchTemplateVersions.

import (
	"context"
	"testing"

	_ "github.com/k2m30/a9s/v3/core/aws" // ensure all related registrations run
	"github.com/k2m30/a9s/v3/core/resource"
)

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
						"image_id":       "ami-xyz", // resolved from custom LaunchTemplate
					},
				},
			},
			IsTruncated: false,
		},
	}

	result := checker(context.Background(), nil, amiResource, cache)

	if result.TargetType() != "ng" {
		t.Errorf("TargetType = %q, want \"ng\"", result.TargetType())
	}
	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1 (ng-custom uses ami-xyz)", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "ng-custom" {
		t.Errorf("ResourceIDs = %v, want [\"ng-custom\"]", result.ResourceIDs())
	}
	if result.Truncated() {
		t.Error("Truncated = true, want false (non-truncated cache with full match)")
	}
	if result.Err() != nil {
		t.Errorf("Err = %v, want nil", result.Err())
	}
}

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
						"image_id":       "ami-other",
					},
				},
			},
			IsTruncated: false,
		},
	}

	result := checker(context.Background(), nil, amiResource, cache)

	if result.TargetType() != "ng" {
		t.Errorf("TargetType = %q, want \"ng\"", result.TargetType())
	}
	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (different AMI ID)", result.Count())
	}
	if result.Truncated() {
		t.Error("Truncated = true, want false (non-truncated cache — definitive zero)")
	}
}

func TestCheckAMING_TruncatedWhenCacheTruncatedAndNoMatch(t *testing.T) {
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
						"image_id":       "ami-other",
					},
				},
			},
			IsTruncated: true, // more pages exist — cannot be certain
		},
	}

	result := checker(context.Background(), nil, amiResource, cache)

	if result.TargetType() != "ng" {
		t.Errorf("TargetType = %q, want \"ng\"", result.TargetType())
	}
	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0", result.Count())
	}
	if !result.Truncated() {
		t.Errorf("Truncated = false, want true (truncated cache — lower bound only). Result: %+v", result)
	}
	if result.Err() != nil {
		t.Errorf("Err = %v, want nil", result.Err())
	}
}

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

	if result.Count() != 2 {
		t.Errorf("Count = %d, want 2 (ng-a and ng-c use ami-shared)", result.Count())
	}
	if len(result.ResourceIDs()) != 2 {
		t.Errorf("ResourceIDs length = %d, want 2; got %v", len(result.ResourceIDs()), result.ResourceIDs())
	}
	found := make(map[string]bool)
	for _, id := range result.ResourceIDs() {
		found[id] = true
	}
	if !found["ng-a"] {
		t.Error("ResourceIDs missing \"ng-a\"")
	}
	if !found["ng-c"] {
		t.Error("ResourceIDs missing \"ng-c\"")
	}
}

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

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (empty AMI ID is an immediate non-match)", result.Count())
	}
	if result.Truncated() {
		t.Error("Truncated = true, want false (empty ID is a definitive non-match)")
	}
}
