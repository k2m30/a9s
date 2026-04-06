package unit_test

import (
	"context"
	"testing"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// TestRelated_APIGW_Registered verifies all 3 related defs are registered with correct checker presence.
func TestRelated_APIGW_Registered(t *testing.T) {
	defs := resource.GetRelated("apigw")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for apigw")
	}

	type expectation struct {
		displayName string
		hasChecker  bool
	}
	expected := map[string]expectation{
		"lambda": {"Lambda Functions", true},
		"logs":   {"Log Groups", true},
		"waf":    {"WAF Web ACLs", true},
	}
	for target, want := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == target {
				found = true
				if want.hasChecker && def.Checker == nil {
					t.Errorf("apigw %q: Checker should not be nil", target)
				}
				if !want.hasChecker && def.Checker != nil {
					t.Errorf("apigw %q: Checker should be nil (stub)", target)
				}
				if def.DisplayName != want.displayName {
					t.Errorf("apigw %q: DisplayName = %q, want %q", target, def.DisplayName, want.displayName)
				}
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found", target)
		}
	}
}

// apigwCheckerByTarget returns the RelatedChecker for the given target type registered
// under "apigw". It fails the test immediately if the checker is nil or not found.
func apigwCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("apigw") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("apigw related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("apigw related checker for %s not found", target)
	return nil
}

// --- checkApigwLogs tests (Pattern N — naming convention) ---

func TestRelated_APIGW_Logs_MatchByExecutionLogPattern(t *testing.T) {
	logRes := resource.Resource{
		ID:     "API-Gateway-Execution-Logs_abc123/prod",
		Fields: map[string]string{},
	}
	cache := resource.ResourceCache{
		"logs": resource.ResourceCacheEntry{Resources: []resource.Resource{logRes}},
	}

	res := resource.Resource{
		ID:     "abc123",
		Fields: map[string]string{},
	}

	checker := apigwCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "API-Gateway-Execution-Logs_abc123/prod" {
		t.Errorf("ResourceIDs = %v, want [API-Gateway-Execution-Logs_abc123/prod]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_APIGW_Logs_MatchByAccessLogPattern(t *testing.T) {
	logRes := resource.Resource{
		ID:     "/aws/apigateway/my-api",
		Fields: map[string]string{},
	}
	cache := resource.ResourceCache{
		"logs": resource.ResourceCacheEntry{Resources: []resource.Resource{logRes}},
	}

	res := resource.Resource{
		ID:     "some-id",
		Name:   "my-api",
		Fields: map[string]string{},
	}

	checker := apigwCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "/aws/apigateway/my-api" {
		t.Errorf("ResourceIDs = %v, want [/aws/apigateway/my-api]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_APIGW_Logs_NoMatch(t *testing.T) {
	logRes := resource.Resource{
		ID:     "/aws/apigateway/other-api",
		Fields: map[string]string{},
	}
	cache := resource.ResourceCache{
		"logs": resource.ResourceCacheEntry{Resources: []resource.Resource{logRes}},
	}

	res := resource.Resource{
		ID:     "xyz999",
		Name:   "my-api",
		Fields: map[string]string{},
	}

	checker := apigwCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_APIGW_Logs_NilCache(t *testing.T) {
	cache := resource.ResourceCache{}

	res := resource.Resource{
		ID:     "abc123",
		Fields: map[string]string{},
	}

	checker := apigwCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (empty cache, no clients)", result.Count)
	}
}

// TestRelatedDemo_APIGW_Registered verifies the demo checker is registered and returns valid results.
func TestRelatedDemo_APIGW_Registered(t *testing.T) {
	_ = demo.GetResources // ensure demo package is loaded
	checker := resource.GetRelatedDemo("apigw")
	if checker == nil {
		t.Fatal("no demo checker registered for apigw")
	}

	results := checker(resource.Resource{ID: "demo-api-id"})
	if len(results) == 0 {
		t.Fatal("demo checker returned no results")
	}
	for _, r := range results {
		if r.TargetType == "" {
			t.Error("demo result has empty TargetType")
		}
	}
}
