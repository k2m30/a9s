package unit_test

import (
	"context"
	"testing"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// pipelineCheckerByTarget returns the RelatedChecker for the given target registered under "pipeline".
func pipelineCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("pipeline") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("pipeline related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("pipeline related checker for %s not found", target)
	return nil
}

// --- Navigable Fields ---

func TestNavigableFields_Pipeline_None(t *testing.T) {
	fields := resource.GetNavigableFields("pipeline")
	if len(fields) != 0 {
		t.Errorf("expected no navigable fields for pipeline, got %d: %v", len(fields), fields)
	}
}

// --- pipeline→cb: undeterminable from cache, returns Count: 0 ---

func TestRelated_Pipeline_CB_ReturnsZero(t *testing.T) {
	source := resource.Resource{
		ID:   "acme-pipeline",
		Name: "acme-pipeline",
	}
	checker := pipelineCheckerByTarget(t, "cb")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (undeterminable from cache)", result.Count)
	}
	if result.TargetType != "cb" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "cb")
	}
}

// --- pipeline→role: undeterminable from cache, returns Count: 0 ---

func TestRelated_Pipeline_Role_ReturnsZero(t *testing.T) {
	source := resource.Resource{
		ID:   "acme-pipeline",
		Name: "acme-pipeline",
	}
	checker := pipelineCheckerByTarget(t, "role")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (undeterminable from cache)", result.Count)
	}
	if result.TargetType != "role" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "role")
	}
}

// --- Demo Checker ---

func TestRelatedDemo_Pipeline_Registered(t *testing.T) {
	_ = demo.GetResources // ensure demo package is initialized
	checker := resource.GetRelatedDemo("pipeline")
	if checker == nil {
		t.Fatal("no demo checker registered for pipeline")
	}

	results := checker(resource.Resource{ID: "acme-pipeline"})
	if len(results) == 0 {
		t.Fatal("demo checker returned no results")
	}
	for _, r := range results {
		if r.TargetType == "" {
			t.Error("demo result has empty TargetType")
		}
	}

	// Verify all expected target types are present.
	wantTargets := map[string]bool{"cb": false, "role": false}
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
