package unit_test

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func TestRelated_DBI_Registered(t *testing.T) {
	defs := resource.GetRelated("dbi")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for dbi")
	}

	type expectation struct {
		displayName string
		hasChecker  bool
	}
	expected := map[string]expectation{
		"sg":       {"Security Groups", true},
		"kms":      {"KMS Key", true},
		"subnet":   {"Subnets", true},
		"alarm":    {"CloudWatch Alarms", false},
		"rds-snap": {"RDS Snapshots", false},
		"secrets":  {"Secrets Manager", false},
	}
	for target, want := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == target {
				found = true
				if want.hasChecker && def.Checker == nil {
					t.Errorf("dbi %q: Checker should not be nil", target)
				}
				if !want.hasChecker && def.Checker != nil {
					t.Errorf("dbi %q: Checker should be nil (stub)", target)
				}
				if def.DisplayName != want.displayName {
					t.Errorf("dbi %q: DisplayName = %q, want %q", target, def.DisplayName, want.displayName)
				}
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found", target)
		}
	}
}

func TestRelatedDemo_DBI_Registered(t *testing.T) {
	_ = demo.GetResources
	checker := resource.GetRelatedDemo("dbi")
	if checker == nil {
		t.Fatal("no demo checker registered for dbi")
	}

	results := checker(resource.Resource{ID: "demo-db"})
	if len(results) == 0 {
		t.Fatal("demo checker returned no results")
	}
	for _, r := range results {
		if r.TargetType == "" {
			t.Error("demo result has empty TargetType")
		}
	}
}
