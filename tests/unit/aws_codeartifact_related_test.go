package unit_test

import (
	"context"
	"testing"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// TestRelated_Codeartifact_CB_ReturnsUnknown verifies codeartifact→cb reports Count=-1
// because CodeBuild buildspecs (which name the CodeArtifact repo/domain) are stored as
// inline YAML or external-file references on cbtypes.Project — they are not exposed
// as structured fields, so reverse lookup from cache is not possible.
func TestRelated_Codeartifact_CB_ReturnsUnknown(t *testing.T) {
	var checker resource.RelatedChecker
	for _, def := range resource.GetRelated("codeartifact") {
		if def.TargetType == "cb" {
			checker = def.Checker
			break
		}
	}
	if checker == nil {
		t.Fatal("codeartifact→cb checker not registered")
	}
	res := resource.Resource{
		ID:     "my-repo",
		Name:   "my-repo",
		Fields: map[string]string{},
	}
	cache := resource.ResourceCache{
		"cb": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "some-project", Name: "some-project"},
		}},
	}
	result := checker(context.Background(), nil, res, cache)
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (undeterminable — buildspecs not structured)", result.Count)
	}
	if result.TargetType != "cb" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "cb")
	}
}

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
