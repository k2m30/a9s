package unit_test

import (
	"testing"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// --- Stub checker tests ---

func TestRelated_SFN_Alarm_IsStub(t *testing.T) {
	defs := resource.GetRelated("sfn")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for sfn")
	}
	for _, def := range defs {
		if def.TargetType == "alarm" {
			if def.Checker != nil {
				t.Errorf("sfn alarm Checker should be nil (stub)")
			}
			return
		}
	}
	t.Error("expected related def for target alarm not found for sfn")
}

func TestRelated_SFN_Logs_IsStub(t *testing.T) {
	defs := resource.GetRelated("sfn")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for sfn")
	}
	for _, def := range defs {
		if def.TargetType == "logs" {
			if def.Checker != nil {
				t.Errorf("sfn logs Checker should be nil (stub)")
			}
			return
		}
	}
	t.Error("expected related def for target logs not found for sfn")
}

func TestRelated_SFN_Role_IsStub(t *testing.T) {
	defs := resource.GetRelated("sfn")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for sfn")
	}
	for _, def := range defs {
		if def.TargetType == "role" {
			if def.Checker != nil {
				t.Errorf("sfn role Checker should be nil (stub)")
			}
			return
		}
	}
	t.Error("expected related def for target role not found for sfn")
}

func TestRelated_SFN_CFN_IsStub(t *testing.T) {
	defs := resource.GetRelated("sfn")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for sfn")
	}
	for _, def := range defs {
		if def.TargetType == "cfn" {
			if def.Checker != nil {
				t.Errorf("sfn cfn Checker should be nil (stub)")
			}
			return
		}
	}
	t.Error("expected related def for target cfn not found for sfn")
}

// --- Demo Checker ---

func TestRelatedDemo_SFN_Registered(t *testing.T) {
	_ = demo.GetResources // ensure demo package is initialized
	checker := resource.GetRelatedDemo("sfn")
	if checker == nil {
		t.Fatal("no demo checker registered for sfn")
	}

	results := checker(resource.Resource{ID: "order-fulfillment-workflow"})
	if len(results) == 0 {
		t.Fatal("demo checker returned no results")
	}
	for _, r := range results {
		if r.TargetType == "" {
			t.Error("demo result has empty TargetType")
		}
	}

	// Verify all expected target types are present.
	wantTargets := map[string]bool{"alarm": false, "logs": false, "role": false, "cfn": false}
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
