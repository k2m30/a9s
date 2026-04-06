package unit_test

import (
	"context"
	"testing"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// iamGroupCheckerByTarget returns the RelatedChecker for the given target type
// registered under "iam-group". Fails immediately if the checker is nil or not found.
func iamGroupCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("iam-group") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("iam-group related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("iam-group related checker for %s not found", target)
	return nil
}

// --- Navigable Fields ---

func TestNavigableFields_IAMGroup_None(t *testing.T) {
	fields := resource.GetNavigableFields("iam-group")
	if len(fields) != 0 {
		t.Errorf("expected no navigable fields for iam-group, got %d: %v", len(fields), fields)
	}
}

// --- iam-group→iam-user (IAM API: GetGroup) ---

func TestRelated_IAMGroup_User_NonNil(t *testing.T) {
	checker := iamGroupCheckerByTarget(t, "iam-user")
	_ = checker
}

func TestRelated_IAMGroup_User_NilClients(t *testing.T) {
	source := resource.Resource{
		ID:   "dev-team",
		Name: "dev-team",
	}
	checker := iamGroupCheckerByTarget(t, "iam-user")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil clients)", result.Count)
	}
	if result.TargetType != "iam-user" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "iam-user")
	}
}

func TestRelated_IAMGroup_User_EmptyID(t *testing.T) {
	source := resource.Resource{
		ID:   "",
		Name: "",
	}
	checker := iamGroupCheckerByTarget(t, "iam-user")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil clients)", result.Count)
	}
}

// --- iam-group→policy (IAM API: ListAttachedGroupPolicies) ---

func TestRelated_IAMGroup_Policy_NonNil(t *testing.T) {
	checker := iamGroupCheckerByTarget(t, "policy")
	_ = checker
}

func TestRelated_IAMGroup_Policy_NilClients(t *testing.T) {
	source := resource.Resource{
		ID:   "dev-team",
		Name: "dev-team",
	}
	checker := iamGroupCheckerByTarget(t, "policy")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil clients)", result.Count)
	}
	if result.TargetType != "policy" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "policy")
	}
}

// --- Demo Checker ---

func TestRelatedDemo_IAMGroup_Registered(t *testing.T) {
	_ = demo.GetResources // ensure demo package is initialized
	checker := resource.GetRelatedDemo("iam-group")
	if checker == nil {
		t.Fatal("no demo checker registered for iam-group")
	}

	results := checker(resource.Resource{ID: "dev-team"})
	if len(results) == 0 {
		t.Fatal("demo checker returned no results")
	}
	for _, r := range results {
		if r.TargetType == "" {
			t.Error("demo result has empty TargetType")
		}
	}

	// Verify all expected target types are present.
	wantTargets := map[string]bool{"iam-user": false, "policy": false}
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
