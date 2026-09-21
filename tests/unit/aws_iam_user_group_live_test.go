package unit

import (
	"context"
	"testing"

	_ "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/resource"
)

func iamUserGroupChecker(t *testing.T) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("iam-user") {
		if def.TargetType == "iam-group" {
			if def.Checker == nil {
				t.Fatal("iam-user→iam-group checker is nil")
			}
			return def.Checker
		}
	}
	t.Fatal("iam-user→iam-group checker not registered")
	return nil
}

// TestCheckUserGroup_HappyPath verifies alice.johnson is a member of exactly
// two groups (admins, developers) as defined in demo/fixtures/iam.go.
func TestCheckUserGroup_HappyPath(t *testing.T) {
	clients := demo.NewServiceClients()
	checker := iamUserGroupChecker(t)

	res := resource.Resource{ID: "alice.johnson", Name: "alice.johnson"}
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.TargetType() != "iam-group" {
		t.Errorf("TargetType = %q, want %q", result.TargetType(), "iam-group")
	}
	if result.Count() != 2 {
		t.Errorf("Count = %d, want 2 (admins + developers); fixture GroupsForUser[\"alice.johnson\"] has 2 entries", result.Count())
	}
	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
	}

	wantGroups := map[string]bool{"admins": false, "developers": false}
	for _, id := range result.ResourceIDs() {
		wantGroups[id] = true
	}
	for name, found := range wantGroups {
		if !found {
			t.Errorf("group %q not found in ResourceIDs %v", name, result.ResourceIDs())
		}
	}
}

// TestCheckUserGroup_EmptyID verifies that an empty user ID returns Count=0
// without calling the API.
func TestCheckUserGroup_EmptyID(t *testing.T) {
	clients := demo.NewServiceClients()
	checker := iamUserGroupChecker(t)

	res := resource.Resource{ID: "", Name: ""}
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 for empty user ID", result.Count())
	}
	if result.TargetType() != "iam-group" {
		t.Errorf("TargetType = %q, want %q", result.TargetType(), "iam-group")
	}
}

// TestCheckUserGroup_NoGroups verifies that a user with no groups in the fixture
// returns Count=0 (not -1). "ci-service-account" is defined as a user in the IAM
// fixture and belongs to no group, so ListGroupsForUser returns an empty list.
func TestCheckUserGroup_NoGroups(t *testing.T) {
	clients := demo.NewServiceClients()
	checker := iamUserGroupChecker(t)

	res := resource.Resource{ID: "ci-service-account", Name: "ci-service-account"}
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (ci-service-account belongs to no group)", result.Count())
	}
	if result.TargetType() != "iam-group" {
		t.Errorf("TargetType = %q, want %q", result.TargetType(), "iam-group")
	}
	if result.Err() != nil {
		t.Errorf("unexpected error for user with no groups: %v", result.Err())
	}
}
