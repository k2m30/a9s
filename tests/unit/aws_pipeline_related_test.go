package unit_test

import (
	"testing"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// --- Navigable Fields ---

func TestNavigableFields_Pipeline_None(t *testing.T) {
	fields := resource.GetNavigableFields("pipeline")
	if len(fields) != 0 {
		t.Errorf("expected no navigable fields for pipeline, got %d: %v", len(fields), fields)
	}
}

// --- Stub Checkers ---

func TestRelated_Pipeline_CB_IsStub(t *testing.T) {
	defs := resource.GetRelated("pipeline")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for pipeline")
	}
	for _, def := range defs {
		if def.TargetType == "cb" {
			if def.Checker != nil {
				t.Errorf("pipeline cb Checker should be nil (stub), got non-nil")
			}
			return
		}
	}
	t.Error("expected related def for target cb not found for pipeline")
}

func TestRelated_Pipeline_Role_IsStub(t *testing.T) {
	defs := resource.GetRelated("pipeline")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for pipeline")
	}
	for _, def := range defs {
		if def.TargetType == "role" {
			if def.Checker != nil {
				t.Errorf("pipeline role Checker should be nil (stub), got non-nil")
			}
			return
		}
	}
	t.Error("expected related def for target role not found for pipeline")
}

// --- Demo Checker ---

func TestRelatedDemo_Pipeline_Registered(t *testing.T) {
	_ = demo.GetResources // ensure demo package is initialized
	checker := resource.GetRelatedDemo("pipeline")
	if checker == nil {
		t.Fatal("no demo checker registered for pipeline")
	}

	results := checker(resource.Resource{ID: "acme-pipeline"})
	if len(results) == 0 {
		t.Fatal("demo checker returned no results")
	}
	for _, r := range results {
		if r.TargetType == "" {
			t.Error("demo result has empty TargetType")
		}
	}

	// Verify all expected target types are present.
	wantTargets := map[string]bool{"cb": false, "role": false}
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
