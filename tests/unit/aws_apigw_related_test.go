package unit_test

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// TestRelated_APIGW_Stubs verifies all 3 related defs are registered as stubs (nil checkers).
func TestRelated_APIGW_Stubs(t *testing.T) {
	defs := resource.GetRelated("apigw")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for apigw")
	}

	expected := map[string]string{
		"lambda": "Lambda Functions",
		"logs":   "Log Groups",
		"waf":    "WAF Web ACLs",
	}
	for target, wantDisplay := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == target {
				found = true
				if def.Checker != nil {
					t.Errorf("apigw %q: Checker should be nil (stub)", target)
				}
				if def.DisplayName != wantDisplay {
					t.Errorf("apigw %q: DisplayName = %q, want %q", target, def.DisplayName, wantDisplay)
				}
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found", target)
		}
	}
}

// TestRelatedDemo_APIGW_Registered verifies the demo checker is registered and returns valid results.
func TestRelatedDemo_APIGW_Registered(t *testing.T) {
	_ = demo.GetResources // ensure demo package is loaded
	checker := resource.GetRelatedDemo("apigw")
	if checker == nil {
		t.Fatal("no demo checker registered for apigw")
	}

	results := checker(resource.Resource{ID: "demo-api-id"})
	if len(results) == 0 {
		t.Fatal("demo checker returned no results")
	}
	for _, r := range results {
		if r.TargetType == "" {
			t.Error("demo result has empty TargetType")
		}
	}
}
