package unit_test

import (
	"context"
	"testing"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// --- Navigable Fields ---

func TestNavigableFields_R53_None(t *testing.T) {
	fields := resource.GetNavigableFields("r53")
	if len(fields) != 0 {
		t.Errorf("expected no navigable fields for r53, got %d: %v", len(fields), fields)
	}
}

// r53CheckerByTarget returns the RelatedChecker for the given target registered under "r53".
func r53CheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("r53") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("r53 related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("r53 related checker for %s not found", target)
	return nil
}

// --- r53→elb: undeterminable from cache, returns Count: 0 ---

func TestRelated_R53_ELB_ReturnsZero(t *testing.T) {
	source := resource.Resource{
		ID:   "/hostedzone/Z0123456789ABCDEFGHIJ",
		Name: "example.com.",
	}
	checker := r53CheckerByTarget(t, "elb")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (undeterminable from cache)", result.Count)
	}
	if result.TargetType != "elb" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "elb")
	}
}

// --- r53→cf: undeterminable from cache, returns Count: 0 ---

func TestRelated_R53_CF_ReturnsZero(t *testing.T) {
	source := resource.Resource{
		ID:   "/hostedzone/Z0123456789ABCDEFGHIJ",
		Name: "example.com.",
	}
	checker := r53CheckerByTarget(t, "cf")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (undeterminable from cache)", result.Count)
	}
	if result.TargetType != "cf" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "cf")
	}
}

// --- r53→acm: undeterminable from cache, returns Count: 0 ---

func TestRelated_R53_ACM_ReturnsZero(t *testing.T) {
	source := resource.Resource{
		ID:   "/hostedzone/Z0123456789ABCDEFGHIJ",
		Name: "example.com.",
	}
	checker := r53CheckerByTarget(t, "acm")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (undeterminable from cache)", result.Count)
	}
	if result.TargetType != "acm" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "acm")
	}
}

// --- Demo Checker ---

func TestRelatedDemo_R53_Registered(t *testing.T) {
	_ = demo.GetResources // ensure demo package is initialized
	checker := resource.GetRelatedDemo("r53")
	if checker == nil {
		t.Fatal("no demo checker registered for r53")
	}

	results := checker(resource.Resource{ID: "/hostedzone/Z0123456789ABCDEFGHIJ"})
	if len(results) == 0 {
		t.Fatal("demo checker returned no results")
	}
	for _, r := range results {
		if r.TargetType == "" {
			t.Error("demo result has empty TargetType")
		}
	}

	// Verify all expected target types are present.
	wantTargets := map[string]bool{"elb": false, "cf": false, "acm": false}
	for _, r := range results {
		if _, ok := wantTargets[r.TargetType]; ok {
			wantTargets[r.TargetType] = true
		}
	}
	for target, found := range wantTargets {
		if !found {
			t.Errorf("demo checker missing result for target %q", target)
		}
	}
}
