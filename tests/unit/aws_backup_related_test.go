package unit_test

import (
	"testing"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// TestRelated_Backup_Registered verifies all related defs are registered with non-nil checkers.
func TestRelated_Backup_Registered(t *testing.T) {
	defs := resource.GetRelated("backup")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for backup")
	}

	expected := map[string]string{
		"role": "IAM Roles",
	}
	for target, wantDisplay := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == target {
				found = true
				if def.Checker == nil {
					t.Errorf("backup %q: Checker should not be nil", target)
				}
				if def.DisplayName != wantDisplay {
					t.Errorf("backup %q: DisplayName = %q, want %q", target, def.DisplayName, wantDisplay)
				}
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found", target)
		}
	}
}

// TestRelatedDemo_Backup_Registered verifies the demo checker is registered and returns valid results.
func TestRelatedDemo_Backup_Registered(t *testing.T) {
	_ = demo.GetResources // ensure demo package is loaded
	checker := resource.GetRelatedDemo("backup")
	if checker == nil {
		t.Fatal("no demo checker registered for backup")
	}

	results := checker(resource.Resource{ID: "demo-vault"})
	if len(results) == 0 {
		t.Fatal("demo checker returned no results")
	}
	for _, r := range results {
		if r.TargetType == "" {
			t.Error("demo result has empty TargetType")
		}
	}
}
