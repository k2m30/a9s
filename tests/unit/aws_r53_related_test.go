package unit_test

import (
	"context"
	"testing"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// --- Navigable Fields ---

func TestNavigableFields_R53_None(t *testing.T) {
	fields := resource.GetNavigableFields("r53")
	if len(fields) != 0 {
		t.Errorf("expected no navigable fields for r53, got %d: %v", len(fields), fields)
	}
}

// r53CheckerByTarget returns the RelatedChecker for the given target type registered
// under "r53". It fails the test immediately if the checker is nil or not found.
func r53CheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("r53") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("r53 related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("r53 related checker for %s not found", target)
	return nil
}

// TestRelated_R53_Registered verifies the 3 related defs are registered with
// correct display names and non-nil checkers.
func TestRelated_R53_Registered(t *testing.T) {
	defs := resource.GetRelated("r53")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for r53")
	}

	expected := map[string]string{
		"elb": "Load Balancers",
		"cf":  "CloudFront",
		"acm": "ACM Certificates",
	}
	for target, wantName := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == target {
				found = true
				if def.Checker == nil {
					t.Errorf("r53 %q: Checker should not be nil", target)
				}
				if def.DisplayName != wantName {
					t.Errorf("r53 %q: DisplayName = %q, want %q", target, def.DisplayName, wantName)
				}
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found", target)
		}
	}
}

// --- r53→elb: requires per-zone ListResourceRecordSets (outside budget) ---

func TestRelated_R53_ELB_Unknown(t *testing.T) {
	source := resource.Resource{ID: "Z1ABC123", Name: "example.com."}
	checker := r53CheckerByTarget(t, "elb")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown: alias records per-zone)", result.Count)
	}
	if result.TargetType != "elb" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "elb")
	}
}

func TestRelated_R53_ELB_EmptyInput(t *testing.T) {
	source := resource.Resource{ID: ""}
	checker := r53CheckerByTarget(t, "elb")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty zone id)", result.Count)
	}
}

// --- r53→cf: requires per-zone ListResourceRecordSets (outside budget) ---

func TestRelated_R53_CF_Unknown(t *testing.T) {
	source := resource.Resource{ID: "Z1ABC123", Name: "example.com."}
	checker := r53CheckerByTarget(t, "cf")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown: alias records per-zone)", result.Count)
	}
	if result.TargetType != "cf" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "cf")
	}
}

func TestRelated_R53_CF_EmptyInput(t *testing.T) {
	source := resource.Resource{ID: ""}
	checker := r53CheckerByTarget(t, "cf")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty zone id)", result.Count)
	}
}

// --- r53→acm: requires per-zone ListResourceRecordSets + DescribeCertificate (outside budget) ---

func TestRelated_R53_ACM_Unknown(t *testing.T) {
	source := resource.Resource{ID: "Z1ABC123", Name: "example.com."}
	checker := r53CheckerByTarget(t, "acm")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown: validation records per-zone)", result.Count)
	}
	if result.TargetType != "acm" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "acm")
	}
}

func TestRelated_R53_ACM_EmptyInput(t *testing.T) {
	source := resource.Resource{ID: ""}
	checker := r53CheckerByTarget(t, "acm")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty zone id)", result.Count)
	}
}
