package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cbtypes "github.com/aws/aws-sdk-go-v2/service/codebuild/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// TestRelated_CB_Registered verifies all 3 related defs are registered with correct checker presence.
func TestRelated_CB_Registered(t *testing.T) {
	defs := resource.GetRelated("cb")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for cb")
	}

	type expectation struct {
		displayName string
		hasChecker  bool
	}
	expected := map[string]expectation{
		"logs":     {"Log Groups", true},
		"role":     {"IAM Roles", true},
		"pipeline": {"CodePipelines", true},
	}
	for target, want := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == target {
				found = true
				if want.hasChecker && def.Checker == nil {
					t.Errorf("cb %q: Checker should not be nil", target)
				}
				if !want.hasChecker && def.Checker != nil {
					t.Errorf("cb %q: Checker should be nil (stub)", target)
				}
				if def.DisplayName != want.displayName {
					t.Errorf("cb %q: DisplayName = %q, want %q", target, def.DisplayName, want.displayName)
				}
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found", target)
		}
	}
}

// cbCheckerByTarget returns the RelatedChecker for the given target type registered
// under "cb". It fails the test immediately if the checker is nil or not found.
func cbCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("cb") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("cb related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("cb related checker for %s not found", target)
	return nil
}

// --- checkCbRole tests (Pattern F — forward field lookup by ARN last segment) ---

func TestRelated_CB_Role_MatchByServiceRole(t *testing.T) {
	roleRes := resource.Resource{
		ID:     "codebuild-role",
		Name:   "codebuild-role",
		Fields: map[string]string{},
	}
	cache := resource.ResourceCache{
		"role": resource.ResourceCacheEntry{Resources: []resource.Resource{roleRes}},
	}

	res := resource.Resource{
		ID:     "my-project",
		Fields: map[string]string{},
		RawStruct: cbtypes.Project{
			ServiceRole: aws.String("arn:aws:iam::123456789012:role/codebuild-role"),
		},
	}

	checker := cbCheckerByTarget(t, "role")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "codebuild-role" {
		t.Errorf("ResourceIDs = %v, want [codebuild-role]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_CB_Role_NoMatch(t *testing.T) {
	roleRes := resource.Resource{
		ID:     "different-role",
		Name:   "different-role",
		Fields: map[string]string{},
	}
	cache := resource.ResourceCache{
		"role": resource.ResourceCacheEntry{Resources: []resource.Resource{roleRes}},
	}

	res := resource.Resource{
		ID:     "my-project",
		Fields: map[string]string{},
		RawStruct: cbtypes.Project{
			ServiceRole: aws.String("arn:aws:iam::123456789012:role/codebuild-role"),
		},
	}

	checker := cbCheckerByTarget(t, "role")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_CB_Role_NilServiceRole(t *testing.T) {
	roleRes := resource.Resource{
		ID:     "codebuild-role",
		Name:   "codebuild-role",
		Fields: map[string]string{},
	}
	cache := resource.ResourceCache{
		"role": resource.ResourceCacheEntry{Resources: []resource.Resource{roleRes}},
	}

	res := resource.Resource{
		ID:     "my-project",
		Fields: map[string]string{},
		RawStruct: cbtypes.Project{
			ServiceRole: nil,
		},
	}

	checker := cbCheckerByTarget(t, "role")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (nil ServiceRole)", result.Count)
	}
}

func TestRelated_CB_Role_NilCache(t *testing.T) {
	cache := resource.ResourceCache{}

	res := resource.Resource{
		ID:     "my-project",
		Fields: map[string]string{},
		RawStruct: cbtypes.Project{
			ServiceRole: aws.String("arn:aws:iam::123456789012:role/codebuild-role"),
		},
	}

	checker := cbCheckerByTarget(t, "role")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (empty cache, no clients)", result.Count)
	}
}

// --- checkCbLogs tests (Pattern F+N — explicit GroupName or naming convention) ---

func TestRelated_CB_Logs_MatchByExplicitGroupName(t *testing.T) {
	logRes := resource.Resource{
		ID:     "/custom/my-logs",
		Fields: map[string]string{},
	}
	cache := resource.ResourceCache{
		"logs": resource.ResourceCacheEntry{Resources: []resource.Resource{logRes}},
	}

	res := resource.Resource{
		ID:     "my-project",
		Fields: map[string]string{},
		RawStruct: cbtypes.Project{
			LogsConfig: &cbtypes.LogsConfig{
				CloudWatchLogs: &cbtypes.CloudWatchLogsConfig{
					GroupName: aws.String("/custom/my-logs"),
				},
			},
		},
	}

	checker := cbCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "/custom/my-logs" {
		t.Errorf("ResourceIDs = %v, want [/custom/my-logs]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_CB_Logs_MatchByNamingConvention(t *testing.T) {
	logRes := resource.Resource{
		ID:     "/aws/codebuild/my-project",
		Fields: map[string]string{},
	}
	cache := resource.ResourceCache{
		"logs": resource.ResourceCacheEntry{Resources: []resource.Resource{logRes}},
	}

	res := resource.Resource{
		ID:        "my-project",
		Fields:    map[string]string{},
		RawStruct: cbtypes.Project{},
	}

	checker := cbCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "/aws/codebuild/my-project" {
		t.Errorf("ResourceIDs = %v, want [/aws/codebuild/my-project]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_CB_Logs_NoMatch(t *testing.T) {
	logRes := resource.Resource{
		ID:     "/aws/codebuild/other-project",
		Fields: map[string]string{},
	}
	cache := resource.ResourceCache{
		"logs": resource.ResourceCacheEntry{Resources: []resource.Resource{logRes}},
	}

	res := resource.Resource{
		ID:        "my-project",
		Fields:    map[string]string{},
		RawStruct: cbtypes.Project{},
	}

	checker := cbCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_CB_Logs_NilCache(t *testing.T) {
	cache := resource.ResourceCache{}

	res := resource.Resource{
		ID:        "my-project",
		Fields:    map[string]string{},
		RawStruct: cbtypes.Project{},
	}

	checker := cbCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (empty cache, no clients)", result.Count)
	}
}

// TestRelatedDemo_CB_Registered verifies the demo checker is registered and returns valid results.
func TestRelatedDemo_CB_Registered(t *testing.T) {
	_ = demo.GetResources // ensure demo package is loaded
	checker := resource.GetRelatedDemo("cb")
	if checker == nil {
		t.Fatal("no demo checker registered for cb")
	}

	results := checker(resource.Resource{ID: "acme-api-build"})
	if len(results) == 0 {
		t.Fatal("demo checker returned no results")
	}
	for _, r := range results {
		if r.TargetType == "" {
			t.Error("demo result has empty TargetType")
		}
	}
}
