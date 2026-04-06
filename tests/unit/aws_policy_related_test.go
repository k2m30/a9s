package unit_test

import (
	"context"
	"testing"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// --- Navigable Fields ---

func TestNavigableFields_Policy_None(t *testing.T) {
	fields := resource.GetNavigableFields("policy")
	if len(fields) != 0 {
		t.Errorf("expected no navigable fields for policy, got %d: %v", len(fields), fields)
	}
}

// --- policy→iam-user (smoke: non-nil checker) ---

func TestRelated_Policy_User_NonNil(t *testing.T) {
	checker := checkerByTarget(t, "policy", "iam-user")
	_ = checker
}

func TestRelated_Policy_User_NilClients(t *testing.T) {
	source := resource.Resource{
		ID:   "arn:aws:iam::111122223333:policy/test-policy",
		Name: "test-policy",
	}
	checker := checkerByTarget(t, "policy", "iam-user")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil clients)", result.Count)
	}
	if result.TargetType != "iam-user" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "iam-user")
	}
}

// --- policy→iam-group (smoke: non-nil checker) ---

func TestRelated_Policy_Group_NonNil(t *testing.T) {
	checker := checkerByTarget(t, "policy", "iam-group")
	_ = checker
}

func TestRelated_Policy_Group_NilClients(t *testing.T) {
	source := resource.Resource{
		ID:   "arn:aws:iam::111122223333:policy/test-policy",
		Name: "test-policy",
	}
	checker := checkerByTarget(t, "policy", "iam-group")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil clients)", result.Count)
	}
	if result.TargetType != "iam-group" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "iam-group")
	}
}

// --- Demo Checker ---

func TestRelatedDemo_Policy_Registered(t *testing.T) {
	_ = demo.GetResources // ensure demo package is initialized
	checker := resource.GetRelatedDemo("policy")
	if checker == nil {
		t.Fatal("no demo checker registered for policy")
	}

	results := checker(resource.Resource{ID: "arn:aws:iam::aws:policy/ReadOnlyAccess"})
	if len(results) == 0 {
		t.Fatal("demo checker returned no results")
	}
	for _, r := range results {
		if r.TargetType == "" {
			t.Error("demo result has empty TargetType")
		}
	}

	// Verify all expected target types are present.
	wantTargets := map[string]bool{"role": false, "iam-user": false, "iam-group": false}
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
