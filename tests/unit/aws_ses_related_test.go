package unit_test

import (
	"context"
	"testing"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// sesCheckerByTarget returns the RelatedChecker for the given target type registered
// under "ses". It fails the test immediately if the checker is nil or not found.
func sesCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("ses") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("ses related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("ses related checker for %s not found", target)
	return nil
}

// --- checkSESR53 tests (Pattern N — naming convention, searches R53 cache) ---

func TestRelated_SES_R53_DomainMatch(t *testing.T) {
	zoneRes := resource.Resource{
		ID:   "/hostedzone/Z123",
		Name: "example.com.",
	}
	cache := resource.ResourceCache{
		"r53": resource.ResourceCacheEntry{Resources: []resource.Resource{zoneRes}},
	}

	src := resource.Resource{
		ID:     "example.com",
		Fields: map[string]string{"identity_type": "DOMAIN"},
	}
	checker := sesCheckerByTarget(t, "r53")
	result := checker(context.Background(), nil, src, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "/hostedzone/Z123" {
		t.Errorf("ResourceIDs = %v, want [/hostedzone/Z123]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_SES_R53_EmailMatch(t *testing.T) {
	zoneRes := resource.Resource{
		ID:   "/hostedzone/Z123",
		Name: "example.com.",
	}
	cache := resource.ResourceCache{
		"r53": resource.ResourceCacheEntry{Resources: []resource.Resource{zoneRes}},
	}

	src := resource.Resource{
		ID:     "user@example.com",
		Fields: map[string]string{"identity_type": "EMAIL_ADDRESS"},
	}
	checker := sesCheckerByTarget(t, "r53")
	result := checker(context.Background(), nil, src, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "/hostedzone/Z123" {
		t.Errorf("ResourceIDs = %v, want [/hostedzone/Z123]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_SES_R53_SubdomainMatch(t *testing.T) {
	zoneRes := resource.Resource{
		ID:   "/hostedzone/Z123",
		Name: "example.com.",
	}
	cache := resource.ResourceCache{
		"r53": resource.ResourceCacheEntry{Resources: []resource.Resource{zoneRes}},
	}

	src := resource.Resource{
		ID:     "sub.example.com",
		Fields: map[string]string{"identity_type": "DOMAIN"},
	}
	checker := sesCheckerByTarget(t, "r53")
	result := checker(context.Background(), nil, src, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (subdomain suffix match)", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "/hostedzone/Z123" {
		t.Errorf("ResourceIDs = %v, want [/hostedzone/Z123]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_SES_R53_NoMatch(t *testing.T) {
	zoneRes := resource.Resource{
		ID:   "/hostedzone/Z123",
		Name: "example.com.",
	}
	cache := resource.ResourceCache{
		"r53": resource.ResourceCacheEntry{Resources: []resource.Resource{zoneRes}},
	}

	src := resource.Resource{
		ID:     "other.com",
		Fields: map[string]string{"identity_type": "DOMAIN"},
	}
	checker := sesCheckerByTarget(t, "r53")
	result := checker(context.Background(), nil, src, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_SES_R53_EmptyID(t *testing.T) {
	zoneRes := resource.Resource{
		ID:   "/hostedzone/Z123",
		Name: "example.com.",
	}
	cache := resource.ResourceCache{
		"r53": resource.ResourceCacheEntry{Resources: []resource.Resource{zoneRes}},
	}

	src := resource.Resource{
		ID:     "",
		Fields: map[string]string{"identity_type": "DOMAIN"},
	}
	checker := sesCheckerByTarget(t, "r53")
	result := checker(context.Background(), nil, src, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty ID)", result.Count)
	}
}

func TestRelated_SES_R53_CacheMissNoClients(t *testing.T) {
	cache := resource.ResourceCache{}

	src := resource.Resource{
		ID:     "example.com",
		Fields: map[string]string{"identity_type": "DOMAIN"},
	}
	checker := sesCheckerByTarget(t, "r53")
	result := checker(context.Background(), nil, src, cache)

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown — empty cache, no clients)", result.Count)
	}
}

// --- ses→cfn: undeterminable from cache, returns Count: 0 ---

func TestRelated_SES_CFN_ReturnsZero(t *testing.T) {
	source := resource.Resource{
		ID:   "acmecorp.com",
		Name: "acmecorp.com",
	}
	checker := sesCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (undeterminable from cache)", result.Count)
	}
	if result.TargetType != "cfn" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "cfn")
	}
}

// --- Demo Checker ---

func TestRelatedDemo_SES_Registered(t *testing.T) {
	_ = demo.GetResources // ensure demo package is initialized
	checker := resource.GetRelatedDemo("ses")
	if checker == nil {
		t.Fatal("no demo checker registered for ses")
	}

	results := checker(resource.Resource{ID: "acmecorp.com"})
	if len(results) == 0 {
		t.Fatal("demo checker returned no results")
	}
	for _, r := range results {
		if r.TargetType == "" {
			t.Error("demo result has empty TargetType")
		}
	}

	// Verify all expected target types are present.
	wantTargets := map[string]bool{"r53": false, "cfn": false}
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

	// At least one result must have Count > 0.
	hasPositive := false
	for _, r := range results {
		if r.Count > 0 {
			hasPositive = true
			break
		}
	}
	if !hasPositive {
		t.Error("demo checker returned no result with Count > 0")
	}
}
