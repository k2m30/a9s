package unit_test

import (
	"testing"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// --- Stub checker assertions ---

// TestRelated_WAF_ELBStub verifies that the elb checker is a nil stub (WAF
// associations require wafv2:ListResourcesForWebACL which is not in the cache
// layer).
func TestRelated_WAF_ELBStub(t *testing.T) {
	for _, def := range resource.GetRelated("waf") {
		if def.TargetType == "elb" {
			if def.Checker != nil {
				t.Error("elb checker for waf should be nil stub, but it is non-nil")
			}
			return
		}
	}
	t.Fatal("no related def found for waf target elb")
}

// TestRelated_WAF_APIGWStub verifies that the apigw checker is a nil stub.
func TestRelated_WAF_APIGWStub(t *testing.T) {
	for _, def := range resource.GetRelated("waf") {
		if def.TargetType == "apigw" {
			if def.Checker != nil {
				t.Error("apigw checker for waf should be nil stub, but it is non-nil")
			}
			return
		}
	}
	t.Fatal("no related def found for waf target apigw")
}

// TestRelated_WAF_CFStub verifies that the cf checker is a nil stub.
func TestRelated_WAF_CFStub(t *testing.T) {
	for _, def := range resource.GetRelated("waf") {
		if def.TargetType == "cf" {
			if def.Checker != nil {
				t.Error("cf checker for waf should be nil stub, but it is non-nil")
			}
			return
		}
	}
	t.Fatal("no related def found for waf target cf")
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
