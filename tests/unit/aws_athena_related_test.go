package unit_test

import (
	"testing"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// TestRelated_Athena_Registered verifies all related defs are registered with non-nil checkers.
func TestRelated_Athena_Registered(t *testing.T) {
	defs := resource.GetRelated("athena")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for athena")
	}

	expected := map[string]string{
		"s3":  "S3 Buckets (results)",
		"kms": "KMS Keys",
	}
	for target, wantDisplay := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == target {
				found = true
				if def.Checker == nil {
					t.Errorf("athena %q: Checker should not be nil", target)
				}
				if def.DisplayName != wantDisplay {
					t.Errorf("athena %q: DisplayName = %q, want %q", target, def.DisplayName, wantDisplay)
				}
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found", target)
		}
	}
}

// TestRelatedDemo_Athena_Registered verifies the demo checker is registered and returns valid results.
func TestRelatedDemo_Athena_Registered(t *testing.T) {
	_ = demo.GetResources // ensure demo package is loaded
	checker := resource.GetRelatedDemo("athena")
	if checker == nil {
		t.Fatal("no demo checker registered for athena")
	}

	results := checker(resource.Resource{ID: "demo-workgroup"})
	if len(results) == 0 {
		t.Fatal("demo checker returned no results")
	}
	for _, r := range results {
		if r.TargetType == "" {
			t.Error("demo result has empty TargetType")
		}
	}
}
