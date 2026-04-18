package unit_test

import (
	"context"
	"testing"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// wafCheckerByTarget retrieves the RelatedChecker for the given targetType
// and fails the test if the checker is nil or not found.
func wafCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("waf") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("waf related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("waf related checker for %s not found", target)
	return nil
}

// wafSrcResource returns a canonical REGIONAL WAF Web ACL test resource.
func wafSrcResource() resource.Resource {
	return resource.Resource{
		ID:   "a1b2c3d4-5678-90ab-cdef-111111111111",
		Name: "my-waf",
		Fields: map[string]string{
			"name":  "my-waf",
			"id":    "a1b2c3d4-5678-90ab-cdef-111111111111",
			"scope": "REGIONAL",
		},
	}
}

// --- ELB checker nil-clients test ---

// TestRelated_WAF_ELB_NilClients verifies that the elb checker returns Count:-1
// when clients are nil (ListResourcesForWebACL cannot be called).
func TestRelated_WAF_ELB_NilClients(t *testing.T) {
	res := wafSrcResource()
	checker := wafCheckerByTarget(t, "elb")
	result := checker(context.Background(), nil, res, nil)
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil clients)", result.Count)
	}
}

// --- APIGW checker nil-clients test ---

// TestRelated_WAF_APIGW_NilClients verifies that the apigw checker returns Count:-1
// when clients are nil (ListResourcesForWebACL cannot be called).
func TestRelated_WAF_APIGW_NilClients(t *testing.T) {
	res := wafSrcResource()
	checker := wafCheckerByTarget(t, "apigw")
	result := checker(context.Background(), nil, res, nil)
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil clients)", result.Count)
	}
}

// --- CF checker: real scope-based dispatch ---

// TestRelated_WAF_CF_RegionalReturnsZero: REGIONAL scope → definitively no CF association.
func TestRelated_WAF_CF_RegionalReturnsZero(t *testing.T) {
	res := wafSrcResource()
	checker := wafCheckerByTarget(t, "cf")
	result := checker(context.Background(), nil, res, nil)
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (REGIONAL scope cannot bind CloudFront)", result.Count)
	}
	if result.TargetType != "cf" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "cf")
	}
}

// TestRelated_WAF_CF_CloudfrontScopeUnknown: CLOUDFRONT scope → Count: -1 (would need API).
func TestRelated_WAF_CF_CloudfrontScopeUnknown(t *testing.T) {
	res := resource.Resource{
		ID:   "a1b2c3d4-5678-90ab-cdef-222222222222",
		Name: "my-cf-waf",
		Fields: map[string]string{
			"name":  "my-cf-waf",
			"id":    "a1b2c3d4-5678-90ab-cdef-222222222222",
			"scope": "CLOUDFRONT",
		},
	}
	checker := wafCheckerByTarget(t, "cf")
	result := checker(context.Background(), nil, res, nil)
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (CLOUDFRONT scope: requires ListResourcesForWebACL)", result.Count)
	}
}

