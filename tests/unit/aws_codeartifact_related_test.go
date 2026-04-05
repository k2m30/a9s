package unit_test

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func TestRelated_Codeartifact_Stubs(t *testing.T) {
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
				if def.Checker != nil {
					t.Errorf("codeartifact %q: Checker should be nil (stub)", target)
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
