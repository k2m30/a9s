package unit_test

import (
	"testing"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// --- Navigable Fields ---

func TestNavigableFields_IAMGroup_None(t *testing.T) {
	fields := resource.GetNavigableFields("iam-group")
	if len(fields) != 0 {
		t.Errorf("expected no navigable fields for iam-group, got %d: %v", len(fields), fields)
	}
}

// --- Stub Checkers ---

func TestRelated_IAMGroup_UserStub(t *testing.T) {
	defs := resource.GetRelated("iam-group")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for iam-group")
	}
	for _, def := range defs {
		if def.TargetType == "iam-user" {
			if def.Checker != nil {
				t.Errorf("iam-group iam-user Checker should be nil (stub), got non-nil")
			}
			return
		}
	}
	t.Error("expected related def for target iam-user not found for iam-group")
}

func TestRelated_IAMGroup_PolicyStub(t *testing.T) {
	defs := resource.GetRelated("iam-group")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for iam-group")
	}
	for _, def := range defs {
		if def.TargetType == "policy" {
			if def.Checker != nil {
				t.Errorf("iam-group policy Checker should be nil (stub), got non-nil")
			}
			return
		}
	}
	t.Error("expected related def for target policy not found for iam-group")
}

// --- Demo Checker ---

func TestRelatedDemo_IAMGroup_Registered(t *testing.T) {
	_ = demo.GetResources // ensure demo package is initialized
	checker := resource.GetRelatedDemo("iam-group")
	if checker == nil {
		t.Fatal("no demo checker registered for iam-group")
	}

	results := checker(resource.Resource{ID: "dev-team"})
	if len(results) == 0 {
		t.Fatal("demo checker returned no results")
	}
	for _, r := range results {
		if r.TargetType == "" {
			t.Error("demo result has empty TargetType")
		}
	}

	// Verify all expected target types are present.
	wantTargets := map[string]bool{"iam-user": false, "policy": false}
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
