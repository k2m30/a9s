package unit_test

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func TestRelated_CF_Stubs(t *testing.T) {
	defs := resource.GetRelated("cf")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for cf")
	}

	expected := map[string]string{
		"s3":  "S3 Buckets (origin)",
		"elb": "Load Balancers (origin)",
		"waf": "WAF Web ACLs",
		"acm": "ACM Certificates",
		"r53": "Route 53 Zones",
	}
	for target, wantDisplay := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == target {
				found = true
				if def.Checker != nil {
					t.Errorf("cf %q: Checker should be nil (stub)", target)
				}
				if def.DisplayName != wantDisplay {
					t.Errorf("cf %q: DisplayName = %q, want %q", target, def.DisplayName, wantDisplay)
				}
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found", target)
		}
	}
}

func TestRelatedDemo_CF_Registered(t *testing.T) {
	_ = demo.GetResources
	checker := resource.GetRelatedDemo("cf")
	if checker == nil {
		t.Fatal("no demo checker registered for cf")
	}

	results := checker(resource.Resource{ID: "E1A2B3C4D5E6F7"})
	if len(results) == 0 {
		t.Fatal("demo checker returned no results")
	}
	for _, r := range results {
		if r.TargetType == "" {
			t.Error("demo result has empty TargetType")
		}
	}
}
