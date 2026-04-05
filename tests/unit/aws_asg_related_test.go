package unit_test

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// TestRelated_ASG_Registered verifies all 5 related defs are registered with correct checker presence.
func TestRelated_ASG_Registered(t *testing.T) {
	defs := resource.GetRelated("asg")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for asg")
	}

	checkerExpected := map[string]bool{
		"ec2":    true,  // non-nil
		"tg":     true,  // non-nil
		"subnet": true,  // non-nil
		"alarm":  false, // nil stub
		"ng":     false, // nil stub
	}
	for target, wantChecker := range checkerExpected {
		found := false
		for _, def := range defs {
			if def.TargetType == target {
				found = true
				hasChecker := def.Checker != nil
				if hasChecker != wantChecker {
					t.Errorf("asg %q: Checker presence = %v, want %v", target, hasChecker, wantChecker)
				}
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found", target)
		}
	}
}

// TestRelatedDemo_ASG_Registered verifies the demo checker is registered and returns valid results.
func TestRelatedDemo_ASG_Registered(t *testing.T) {
	_ = demo.GetResources // ensure demo package is loaded
	checker := resource.GetRelatedDemo("asg")
	if checker == nil {
		t.Fatal("no demo checker registered for asg")
	}

	results := checker(resource.Resource{ID: "acme-web-prod-asg"})
	if len(results) == 0 {
		t.Fatal("demo checker returned no results")
	}
	for _, r := range results {
		if r.TargetType == "" {
			t.Error("demo result has empty TargetType")
		}
	}

	// Verify ec2, tg, subnet return Count > 0
	counts := map[string]int{}
	for _, r := range results {
		counts[r.TargetType] = r.Count
	}
	for _, target := range []string{"ec2", "tg", "subnet"} {
		if counts[target] == 0 {
			t.Errorf("asg demo: %q should have Count > 0", target)
		}
	}
}
