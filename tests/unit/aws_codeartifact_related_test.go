package unit_test

import (
	"context"
	"testing"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func TestRelated_Codeartifact_Registered(t *testing.T) {
	defs := resource.GetRelated("codeartifact")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for codeartifact")
	}

	expected := map[string]string{
		"cb": "CodeBuild Projects",
	}
	for target, wantDisplay := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == target {
				found = true
				if def.Checker == nil {
					t.Errorf("codeartifact %q: Checker should not be nil", target)
				}
				if def.DisplayName != wantDisplay {
					t.Errorf("codeartifact %q: DisplayName = %q, want %q", target, def.DisplayName, wantDisplay)
				}
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found", target)
		}
	}
}

// --- codeartifact→cb: undeterminable from cache, returns Count: 0 ---

func TestRelated_Codeartifact_CB_ReturnsZero(t *testing.T) {
	source := resource.Resource{
		ID:   "acme-npm",
		Name: "acme-npm",
	}
	var checker resource.RelatedChecker
	for _, def := range resource.GetRelated("codeartifact") {
		if def.TargetType == "cb" {
			checker = def.Checker
			break
		}
	}
	if checker == nil {
		t.Fatal("codeartifact cb checker is nil")
	}
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (undeterminable from cache)", result.Count)
	}
	if result.TargetType != "cb" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "cb")
	}
}

func TestRelatedDemo_Codeartifact_Registered(t *testing.T) {
	_ = demo.GetResources
	checker := resource.GetRelatedDemo("codeartifact")
	if checker == nil {
		t.Fatal("no demo checker registered for codeartifact")
	}

	results := checker(resource.Resource{ID: "acme-npm"})
	if len(results) == 0 {
		t.Fatal("demo checker returned no results")
	}
	for _, r := range results {
		if r.TargetType == "" {
			t.Error("demo result has empty TargetType")
		}
	}
}
