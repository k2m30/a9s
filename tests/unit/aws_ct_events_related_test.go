package unit_test

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func TestRelated_CtEvents_Stubs(t *testing.T) {
	defs := resource.GetRelated("ct-events")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for ct-events")
	}

	expected := map[string]string{
		"role":     "IAM Roles",
		"iam-user": "IAM Users",
	}
	for target, wantDisplay := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == target {
				found = true
				if def.Checker != nil {
					t.Errorf("ct-events %q: Checker should be nil (stub)", target)
				}
				if def.DisplayName != wantDisplay {
					t.Errorf("ct-events %q: DisplayName = %q, want %q", target, def.DisplayName, wantDisplay)
				}
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found", target)
		}
	}
}

func TestRelatedDemo_CtEvents_Registered(t *testing.T) {
	_ = demo.GetResources
	checker := resource.GetRelatedDemo("ct-events")
	if checker == nil {
		t.Fatal("no demo checker registered for ct-events")
	}

	results := checker(resource.Resource{ID: "evt-0a1b2c3d4e5f60001"})
	if len(results) == 0 {
		t.Fatal("demo checker returned no results")
	}
	for _, r := range results {
		if r.TargetType == "" {
			t.Error("demo result has empty TargetType")
		}
	}
}
