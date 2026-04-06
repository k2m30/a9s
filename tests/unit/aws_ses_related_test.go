package unit_test

import (
	"testing"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// --- Stub checker tests ---

func TestRelated_SES_R53_IsStub(t *testing.T) {
	defs := resource.GetRelated("ses")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for ses")
	}
	for _, def := range defs {
		if def.TargetType == "r53" {
			if def.Checker != nil {
				t.Errorf("ses r53 Checker should be nil (stub)")
			}
			return
		}
	}
	t.Error("expected related def for target r53 not found for ses")
}

func TestRelated_SES_CFN_IsStub(t *testing.T) {
	defs := resource.GetRelated("ses")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for ses")
	}
	for _, def := range defs {
		if def.TargetType == "cfn" {
			if def.Checker != nil {
				t.Errorf("ses cfn Checker should be nil (stub)")
			}
			return
		}
	}
	t.Error("expected related def for target cfn not found for ses")
}

// --- Demo Checker ---

func TestRelatedDemo_SES_Registered(t *testing.T) {
	_ = demo.GetResources // ensure demo package is initialized
	checker := resource.GetRelatedDemo("ses")
	if checker == nil {
		t.Fatal("no demo checker registered for ses")
	}

	results := checker(resource.Resource{ID: "acmecorp.com"})
	if len(results) == 0 {
		t.Fatal("demo checker returned no results")
	}
	for _, r := range results {
		if r.TargetType == "" {
			t.Error("demo result has empty TargetType")
		}
	}

	// Verify all expected target types are present.
	wantTargets := map[string]bool{"r53": false, "cfn": false}
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

	// At least one result must have Count > 0.
	hasPositive := false
	for _, r := range results {
		if r.Count > 0 {
			hasPositive = true
			break
		}
	}
	if !hasPositive {
		t.Error("demo checker returned no result with Count > 0")
	}
}
