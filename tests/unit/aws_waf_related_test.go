package unit_test

import (
	"context"
	"testing"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
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

// --- CF checker returns Count:0 for REGIONAL scope ---

// TestRelated_WAF_CF_ReturnsZero verifies that the cf checker returns Count:0
// for REGIONAL scope WAFs (CloudFront associations only apply to CLOUDFRONT scope).
func TestRelated_WAF_CF_ReturnsZero(t *testing.T) {
	res := wafSrcResource()
	checker := wafCheckerByTarget(t, "cf")
	result := checker(context.Background(), nil, res, nil)
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (REGIONAL scope WAF cannot associate with CloudFront)", result.Count)
	}
}

// --- Demo checker ---

// TestRelatedDemo_WAF_Registered verifies the demo checker is registered and
// returns valid results with all expected target types present and at least one
// positive count.
func TestRelatedDemo_WAF_Registered(t *testing.T) {
	_ = demo.GetResources // ensure demo package is initialized
	checker := resource.GetRelatedDemo("waf")
	if checker == nil {
		t.Fatal("no demo checker registered for waf")
	}

	src := resource.Resource{ID: "a1b2c3d4-5678-90ab-cdef-111111111111"}
	results := checker(src)
	if len(results) == 0 {
		t.Fatal("demo checker returned no results")
	}
	for _, r := range results {
		if r.TargetType == "" {
			t.Error("demo result has empty TargetType")
		}
	}

	// Verify all expected target types are present.
	wantTargets := map[string]bool{
		"elb":   false,
		"apigw": false,
		"cf":    false,
	}
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

	// At least one result should have Count > 0.
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
