package unit_test

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// TestRelated_ACM_Stubs verifies all 4 related defs are registered as stubs (nil checkers).
func TestRelated_ACM_Stubs(t *testing.T) {
	defs := resource.GetRelated("acm")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for acm")
	}

	expected := map[string]string{
		"elb":   "Load Balancers",
		"cf":    "CloudFront Distros",
		"apigw": "API Gateways",
		"r53":   "Route 53 Zones",
	}
	for target, wantDisplay := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == target {
				found = true
				if def.Checker != nil {
					t.Errorf("acm related def for %q: Checker should be nil (stub), got non-nil", target)
				}
				if def.DisplayName != wantDisplay {
					t.Errorf("acm related def for %q: DisplayName = %q, want %q", target, def.DisplayName, wantDisplay)
				}
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found", target)
		}
	}
}

// TestRelatedDemo_ACM_Registered verifies the demo checker is registered and returns valid results.
func TestRelatedDemo_ACM_Registered(t *testing.T) {
	_ = demo.GetResources // ensure demo package is loaded
	checker := resource.GetRelatedDemo("acm")
	if checker == nil {
		t.Fatal("no demo checker registered for acm")
	}

	results := checker(resource.Resource{ID: "demo-cert.example.com"})
	if len(results) == 0 {
		t.Fatal("demo checker returned no results")
	}
	for _, r := range results {
		if r.TargetType == "" {
			t.Error("demo result has empty TargetType")
		}
	}
}
