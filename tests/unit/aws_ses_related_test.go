package unit_test

import (
	"context"
	"testing"

	_ "github.com/k2m30/a9s/v3/internal/aws"
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

// --- ses→cfn: undeterminable — sesv2.IdentityInfo has no Tags field ---

// TestRelated_SES_CFN_ReturnsUnknown verifies that the ses→cfn checker reports Count=-1
// because the SES v2 IdentityInfo RawStruct carries no Tags — determining CloudFormation
// stack membership would require ListTagsForResource per identity (N+1), which is
// intentionally not implemented.
func TestRelated_SES_CFN_ReturnsUnknown(t *testing.T) {
	source := resource.Resource{
		ID:   "acmecorp.com",
		Name: "acmecorp.com",
	}
	checker := sesCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (undeterminable — no Tags on IdentityInfo)", result.Count)
	}
	if result.TargetType != "cfn" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "cfn")
	}
}
