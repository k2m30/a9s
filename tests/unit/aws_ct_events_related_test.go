package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func TestRelated_CtEvents_Registered(t *testing.T) {
	defs := resource.GetRelated("ct-events")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for ct-events")
	}

	type expectation struct {
		displayName string
		hasChecker  bool
	}
	expected := map[string]expectation{
		"role":     {"IAM Roles", true},
		"iam-user": {"IAM Users", true},
	}
	for target, want := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == target {
				found = true
				if want.hasChecker && def.Checker == nil {
					t.Errorf("ct-events %q: Checker should not be nil", target)
				}
				if !want.hasChecker && def.Checker != nil {
					t.Errorf("ct-events %q: Checker should be nil (stub)", target)
				}
				if def.DisplayName != want.displayName {
					t.Errorf("ct-events %q: DisplayName = %q, want %q", target, def.DisplayName, want.displayName)
				}
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found", target)
		}
	}
}

// ctEventsCheckerByTarget returns the RelatedChecker for the given target type registered
// under "ct-events". It fails the test immediately if the checker is nil or not found.
func ctEventsCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("ct-events") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("ct-events related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("ct-events related checker for %s not found", target)
	return nil
}

// --- IAM User checker tests (match by Fields["user"] == resource.Name) ---

func TestRelated_CtEvents_User_MatchByUsername(t *testing.T) {
	userRes := resource.Resource{
		ID:     "AIDAEXAMPLE",
		Name:   "admin-user",
		Fields: map[string]string{},
	}
	cache := resource.ResourceCache{
		"iam-user": resource.ResourceCacheEntry{Resources: []resource.Resource{userRes}},
	}

	res := resource.Resource{
		ID:     "evt-0a1b2c3d4e5f60001",
		Fields: map[string]string{"user": "admin-user"},
	}

	checker := ctEventsCheckerByTarget(t, "iam-user")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_CtEvents_User_NoMatch(t *testing.T) {
	userRes := resource.Resource{
		ID:     "AIDAEXAMPLE",
		Name:   "other-user",
		Fields: map[string]string{},
	}
	cache := resource.ResourceCache{
		"iam-user": resource.ResourceCacheEntry{Resources: []resource.Resource{userRes}},
	}

	res := resource.Resource{
		ID:     "evt-0a1b2c3d4e5f60002",
		Fields: map[string]string{"user": "admin-user"},
	}

	checker := ctEventsCheckerByTarget(t, "iam-user")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_CtEvents_User_EmptyUser(t *testing.T) {
	userRes := resource.Resource{
		ID:     "AIDAEXAMPLE",
		Name:   "admin-user",
		Fields: map[string]string{},
	}
	cache := resource.ResourceCache{
		"iam-user": resource.ResourceCacheEntry{Resources: []resource.Resource{userRes}},
	}

	res := resource.Resource{
		ID:     "evt-0a1b2c3d4e5f60003",
		Fields: map[string]string{"user": ""},
	}

	checker := ctEventsCheckerByTarget(t, "iam-user")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty user field)", result.Count)
	}
}

func TestRelated_CtEvents_User_NilCache(t *testing.T) {
	cache := resource.ResourceCache{}

	res := resource.Resource{
		ID:     "evt-0a1b2c3d4e5f60004",
		Fields: map[string]string{"user": "admin-user"},
	}

	checker := ctEventsCheckerByTarget(t, "iam-user")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (empty cache)", result.Count)
	}
}

// --- IAM Role checker tests (match by RawStruct Resources list — AWS::IAM::Role) ---

func TestRelated_CtEvents_Role_MatchByResource(t *testing.T) {
	roleRes := resource.Resource{
		ID:     "my-role",
		Name:   "my-role",
		Fields: map[string]string{},
	}
	cache := resource.ResourceCache{
		"role": resource.ResourceCacheEntry{Resources: []resource.Resource{roleRes}},
	}

	res := resource.Resource{
		ID:     "evt-0a1b2c3d4e5f60005",
		Fields: map[string]string{},
		RawStruct: cloudtrailtypes.Event{
			Resources: []cloudtrailtypes.Resource{
				{
					ResourceType: aws.String("AWS::IAM::Role"),
					ResourceName: aws.String("arn:aws:iam::123:role/my-role"),
				},
			},
		},
	}

	checker := ctEventsCheckerByTarget(t, "role")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_CtEvents_Role_NoMatch(t *testing.T) {
	roleRes := resource.Resource{
		ID:     "other-role",
		Name:   "other-role",
		Fields: map[string]string{},
	}
	cache := resource.ResourceCache{
		"role": resource.ResourceCacheEntry{Resources: []resource.Resource{roleRes}},
	}

	res := resource.Resource{
		ID:     "evt-0a1b2c3d4e5f60006",
		Fields: map[string]string{},
		RawStruct: cloudtrailtypes.Event{
			Resources: []cloudtrailtypes.Resource{
				{
					ResourceType: aws.String("AWS::IAM::Role"),
					ResourceName: aws.String("arn:aws:iam::123:role/my-role"),
				},
			},
		},
	}

	checker := ctEventsCheckerByTarget(t, "role")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_CtEvents_Role_NilCache(t *testing.T) {
	cache := resource.ResourceCache{}

	res := resource.Resource{
		ID:     "evt-0a1b2c3d4e5f60007",
		Fields: map[string]string{},
		RawStruct: cloudtrailtypes.Event{
			Resources: []cloudtrailtypes.Resource{
				{
					ResourceType: aws.String("AWS::IAM::Role"),
					ResourceName: aws.String("arn:aws:iam::123:role/my-role"),
				},
			},
		},
	}

	checker := ctEventsCheckerByTarget(t, "role")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (empty cache)", result.Count)
	}
}

func TestRelatedDemo_CtEvents_Registered(t *testing.T) {
	_ = demo.GetResources
	checker := resource.GetRelatedDemo("ct-events")
	if checker == nil {
		t.Fatal("no demo checker registered for ct-events")
	}

	results := checker(resource.Resource{ID: "evt-0a1b2c3d4e5f60001"})
	if len(results) == 0 {
		t.Fatal("demo checker returned no results")
	}
	for _, r := range results {
		if r.TargetType == "" {
			t.Error("demo result has empty TargetType")
		}
	}
}
