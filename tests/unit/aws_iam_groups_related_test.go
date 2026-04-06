package unit_test

import (
	"context"
	"testing"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// --- iam-group→iam-user ---

func TestRelated_IAMGroup_IAMUser_NonNil(t *testing.T) {
	checker := checkerByTarget(t, "iam-group", "iam-user")
	_ = checker
}

func TestRelated_IAMGroup_IAMUser_NilClients(t *testing.T) {
	source := resource.Resource{
		ID:   "dev-team",
		Name: "dev-team",
		Fields: map[string]string{
			"group_name": "dev-team",
		},
	}
	checker := checkerByTarget(t, "iam-group", "iam-user")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (no clients)", result.Count)
	}
	if result.TargetType != "iam-user" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "iam-user")
	}
}

func TestRelated_IAMGroup_IAMUser_EmptyID(t *testing.T) {
	source := resource.Resource{
		ID:   "",
		Name: "",
	}
	checker := checkerByTarget(t, "iam-group", "iam-user")
	// nil clients must return -1, not panic
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil clients)", result.Count)
	}
}

func TestRelated_IAMGroup_Policy_EmptyID(t *testing.T) {
	source := resource.Resource{
		ID:   "",
		Name: "",
	}
	checker := checkerByTarget(t, "iam-group", "policy")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil clients)", result.Count)
	}
}
