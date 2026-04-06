package unit_test

import (
	"context"
	"testing"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// iamUserCheckerByTarget returns the RelatedChecker for the given target type
// registered under "iam-user". Fails immediately if the checker is nil or not found.
func iamUserCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("iam-user") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("iam-user related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("iam-user related checker for %s not found", target)
	return nil
}

// --- Navigable Fields ---

func TestNavigableFields_IAMUser_None(t *testing.T) {
	fields := resource.GetNavigableFields("iam-user")
	if len(fields) != 0 {
		t.Errorf("expected no navigable fields for iam-user, got %d: %v", len(fields), fields)
	}
}

// --- iam-user→iam-group (IAM API: ListGroupsForUser) ---

func TestRelated_IAMUser_Group_NonNil(t *testing.T) {
	checker := iamUserCheckerByTarget(t, "iam-group")
	_ = checker
}

func TestRelated_IAMUser_Group_NilClients(t *testing.T) {
	source := resource.Resource{
		ID:   "alice",
		Name: "alice",
	}
	checker := iamUserCheckerByTarget(t, "iam-group")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil clients)", result.Count)
	}
	if result.TargetType != "iam-group" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "iam-group")
	}
}

func TestRelated_IAMUser_Group_EmptyID(t *testing.T) {
	source := resource.Resource{
		ID:   "",
		Name: "",
	}
	checker := iamUserCheckerByTarget(t, "iam-group")
	// nil clients: expect -1 not panic (empty userName triggers early return in impl, but nil clients checked first)
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil clients)", result.Count)
	}
}

// --- iam-user→policy (IAM API: ListAttachedUserPolicies) ---

func TestRelated_IAMUser_Policy_NonNil(t *testing.T) {
	checker := iamUserCheckerByTarget(t, "policy")
	_ = checker
}

func TestRelated_IAMUser_Policy_NilClients(t *testing.T) {
	source := resource.Resource{
		ID:   "alice",
		Name: "alice",
	}
	checker := iamUserCheckerByTarget(t, "policy")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil clients)", result.Count)
	}
	if result.TargetType != "policy" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "policy")
	}
}

// --- Demo Checker ---

func TestRelatedDemo_IAMUser_Registered(t *testing.T) {
	_ = demo.GetResources // ensure demo package is initialized
	checker := resource.GetRelatedDemo("iam-user")
	if checker == nil {
		t.Fatal("no demo checker registered for iam-user")
	}

	results := checker(resource.Resource{ID: "alice"})
	if len(results) == 0 {
		t.Fatal("demo checker returned no results")
	}
	for _, r := range results {
		if r.TargetType == "" {
			t.Error("demo result has empty TargetType")
		}
	}

	// Verify all expected target types are present.
	wantTargets := map[string]bool{"iam-group": false, "policy": false}
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
