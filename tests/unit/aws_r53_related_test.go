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

// --- Stub Checkers ---

func TestRelated_R53_ELB_IsStub(t *testing.T) {
	defs := resource.GetRelated("r53")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for r53")
	}
	for _, def := range defs {
		if def.TargetType == "elb" {
			if def.Checker != nil {
				t.Errorf("r53 elb Checker should be nil (stub), got non-nil")
			}
			return
		}
	}
	t.Error("expected related def for target elb not found for r53")
}

func TestRelated_R53_CF_IsStub(t *testing.T) {
	defs := resource.GetRelated("r53")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for r53")
	}
	for _, def := range defs {
		if def.TargetType == "cf" {
			if def.Checker != nil {
				t.Errorf("r53 cf Checker should be nil (stub), got non-nil")
			}
			return
		}
	}
	t.Error("expected related def for target cf not found for r53")
}

func TestRelated_R53_ACM_IsStub(t *testing.T) {
	defs := resource.GetRelated("r53")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for r53")
	}
	for _, def := range defs {
		if def.TargetType == "acm" {
			if def.Checker != nil {
				t.Errorf("r53 acm Checker should be nil (stub), got non-nil")
			}
			return
		}
	}
	t.Error("expected related def for target acm not found for r53")
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
