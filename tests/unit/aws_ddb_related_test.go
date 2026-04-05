package unit_test

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func TestRelated_DDB_Registered(t *testing.T) {
	defs := resource.GetRelated("ddb")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for ddb")
	}

	type expectation struct {
		displayName string
		hasChecker  bool
	}
	expected := map[string]expectation{
		"kms":    {"KMS Key", true},
		"lambda": {"Lambda Functions", false},
		"alarm":  {"CloudWatch Alarms", false},
	}
	for target, want := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == target {
				found = true
				if want.hasChecker && def.Checker == nil {
					t.Errorf("ddb %q: Checker should not be nil", target)
				}
				if !want.hasChecker && def.Checker != nil {
					t.Errorf("ddb %q: Checker should be nil (stub)", target)
				}
				if def.DisplayName != want.displayName {
					t.Errorf("ddb %q: DisplayName = %q, want %q", target, def.DisplayName, want.displayName)
				}
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found", target)
		}
	}
}

func TestRelatedDemo_DDB_Registered(t *testing.T) {
	_ = demo.GetResources
	checker := resource.GetRelatedDemo("ddb")
	if checker == nil {
		t.Fatal("no demo checker registered for ddb")
	}

	results := checker(resource.Resource{ID: "acme-orders-prod"})
	if len(results) == 0 {
		t.Fatal("demo checker returned no results")
	}
	for _, r := range results {
		if r.TargetType == "" {
			t.Error("demo result has empty TargetType")
		}
	}
}
