package unit_test

import (
	"context"
	"testing"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// --- iam-user→iam-group ---

func TestRelated_IAMUser_IAMGroup_NonNil(t *testing.T) {
	checker := checkerByTarget(t, "iam-user", "iam-group")
	_ = checker
}

func TestRelated_IAMUser_IAMGroup_NilClients(t *testing.T) {
	source := resource.Resource{
		ID:   "alice",
		Name: "alice",
		Fields: map[string]string{
			"user_name": "alice",
		},
	}
	checker := checkerByTarget(t, "iam-user", "iam-group")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (no clients)", result.Count)
	}
	if result.TargetType != "iam-group" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "iam-group")
	}
}

func TestRelated_IAMUser_IAMGroup_EmptyID(t *testing.T) {
	source := resource.Resource{
		ID:   "",
		Name: "",
	}
	checker := checkerByTarget(t, "iam-user", "iam-group")
	// nil clients must return -1, not panic
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil clients)", result.Count)
	}
}

// Note: iam-user→policy tests (TestRelated_IAMUser_Policy_*) are in aws_iam_user_related_test.go
