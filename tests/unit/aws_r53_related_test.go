package unit_test

import (
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
