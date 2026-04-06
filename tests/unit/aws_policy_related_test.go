package unit_test

import (
	"testing"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// --- Navigable Fields ---

func TestNavigableFields_Policy_None(t *testing.T) {
	fields := resource.GetNavigableFields("policy")
	if len(fields) != 0 {
		t.Errorf("expected no navigable fields for policy, got %d: %v", len(fields), fields)
	}
}

// --- Stub Checkers ---

func TestRelated_Policy_Role_IsStub(t *testing.T) {
	defs := resource.GetRelated("policy")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for policy")
	}
	for _, def := range defs {
		if def.TargetType == "role" {
			if def.Checker != nil {
				t.Errorf("policy role Checker should be nil (stub), got non-nil")
			}
			return
		}
	}
	t.Error("expected related def for target role not found for policy")
}

func TestRelated_Policy_User_IsStub(t *testing.T) {
	defs := resource.GetRelated("policy")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for policy")
	}
	for _, def := range defs {
		if def.TargetType == "iam-user" {
			if def.Checker != nil {
				t.Errorf("policy iam-user Checker should be nil (stub), got non-nil")
			}
			return
		}
	}
	t.Error("expected related def for target iam-user not found for policy")
}

func TestRelated_Policy_Group_IsStub(t *testing.T) {
	defs := resource.GetRelated("policy")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for policy")
	}
	for _, def := range defs {
		if def.TargetType == "iam-group" {
			if def.Checker != nil {
				t.Errorf("policy iam-group Checker should be nil (stub), got non-nil")
			}
			return
		}
	}
	t.Error("expected related def for target iam-group not found for policy")
}

// --- Demo Checker ---

func TestRelatedDemo_Policy_Registered(t *testing.T) {
	_ = demo.GetResources // ensure demo package is initialized
	checker := resource.GetRelatedDemo("policy")
	if checker == nil {
		t.Fatal("no demo checker registered for policy")
	}

	results := checker(resource.Resource{ID: "arn:aws:iam::aws:policy/ReadOnlyAccess"})
	if len(results) == 0 {
		t.Fatal("demo checker returned no results")
	}
	for _, r := range results {
		if r.TargetType == "" {
			t.Error("demo result has empty TargetType")
		}
	}

	// Verify all expected target types are present.
	wantTargets := map[string]bool{"role": false, "iam-user": false, "iam-group": false}
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
