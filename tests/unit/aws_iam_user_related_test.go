package unit_test

import (
	"testing"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// --- Navigable Fields ---

func TestNavigableFields_IAMUser_None(t *testing.T) {
	fields := resource.GetNavigableFields("iam-user")
	if len(fields) != 0 {
		t.Errorf("expected no navigable fields for iam-user, got %d: %v", len(fields), fields)
	}
}

// --- Stub Checkers ---

func TestRelated_IAMUser_GroupStub(t *testing.T) {
	defs := resource.GetRelated("iam-user")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for iam-user")
	}
	for _, def := range defs {
		if def.TargetType == "iam-group" {
			if def.Checker != nil {
				t.Errorf("iam-user iam-group Checker should be nil (stub), got non-nil")
			}
			return
		}
	}
	t.Error("expected related def for target iam-group not found for iam-user")
}

func TestRelated_IAMUser_PolicyStub(t *testing.T) {
	defs := resource.GetRelated("iam-user")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for iam-user")
	}
	for _, def := range defs {
		if def.TargetType == "policy" {
			if def.Checker != nil {
				t.Errorf("iam-user policy Checker should be nil (stub), got non-nil")
			}
			return
		}
	}
	t.Error("expected related def for target policy not found for iam-user")
}

// --- Demo Checker ---

func TestRelatedDemo_IAMUser_Registered(t *testing.T) {
	_ = demo.GetResources // ensure demo package is initialized
	checker := resource.GetRelatedDemo("iam-user")
	if checker == nil {
		t.Fatal("no demo checker registered for iam-user")
	}

	results := checker(resource.Resource{ID: "alice"})
	if len(results) == 0 {
		t.Fatal("demo checker returned no results")
	}
	for _, r := range results {
		if r.TargetType == "" {
			t.Error("demo result has empty TargetType")
		}
	}

	// Verify all expected target types are present.
	wantTargets := map[string]bool{"iam-group": false, "policy": false}
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
