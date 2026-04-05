package unit_test

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// TestRelated_CB_Stubs verifies all 3 related defs are registered as stubs (nil checkers).
func TestRelated_CB_Stubs(t *testing.T) {
	defs := resource.GetRelated("cb")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for cb")
	}

	expected := map[string]string{
		"logs":     "Log Groups",
		"role":     "IAM Roles",
		"pipeline": "CodePipelines",
	}
	for target, wantDisplay := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == target {
				found = true
				if def.Checker != nil {
					t.Errorf("cb %q: Checker should be nil (stub)", target)
				}
				if def.DisplayName != wantDisplay {
					t.Errorf("cb %q: DisplayName = %q, want %q", target, def.DisplayName, wantDisplay)
				}
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found", target)
		}
	}
}

// TestRelatedDemo_CB_Registered verifies the demo checker is registered and returns valid results.
func TestRelatedDemo_CB_Registered(t *testing.T) {
	_ = demo.GetResources // ensure demo package is loaded
	checker := resource.GetRelatedDemo("cb")
	if checker == nil {
		t.Fatal("no demo checker registered for cb")
	}

	results := checker(resource.Resource{ID: "acme-api-build"})
	if len(results) == 0 {
		t.Fatal("demo checker returned no results")
	}
	for _, r := range results {
		if r.TargetType == "" {
			t.Error("demo result has empty TargetType")
		}
	}
}
