package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// acmCheckerByTarget returns the RelatedChecker for the given target registered under "acm".
func acmCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("acm") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("acm related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("acm related checker for %s not found", target)
	return nil
}

// TestRelated_ACM_Registered verifies all 4 related defs are registered with correct checker presence.
func TestRelated_ACM_Registered(t *testing.T) {
	defs := resource.GetRelated("acm")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for acm")
	}

	type expectation struct {
		displayName string
		hasChecker  bool
	}
	expected := map[string]expectation{
		"elb":   {"Load Balancers", true},
		"cf":    {"CloudFront Distros", true},
		"apigw": {"API Gateways", true},
		"r53":   {"Route 53 Zones", true},
	}
	for target, want := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == target {
				found = true
				if want.hasChecker && def.Checker == nil {
					t.Errorf("acm %q: Checker should not be nil", target)
				}
				if def.DisplayName != want.displayName {
					t.Errorf("acm %q: DisplayName = %q, want %q", target, def.DisplayName, want.displayName)
				}
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found", target)
		}
	}
}

// --- acm→elb: undeterminable from cache, returns Count: 0 ---

func TestRelated_ACM_ELB_ReturnsZero(t *testing.T) {
	source := resource.Resource{
		ID:   "arn:aws:acm::111122223333:certificate/abc-123",
		Name: "example.com",
	}
	checker := acmCheckerByTarget(t, "elb")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (undeterminable from cache)", result.Count)
	}
	if result.TargetType != "elb" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "elb")
	}
}

// --- acm→cf: cache-based (ViewerCertificate.ACMCertificateArn == certARN) ---

func TestRelated_ACM_CF_Found(t *testing.T) {
	certARN := "arn:aws:acm:us-east-1:111122223333:certificate/abc-123"
	source := resource.Resource{
		ID:   certARN,
		Name: "example.com",
	}

	matchingDist := resource.Resource{
		ID:   "E1ABCDEF123456",
		Name: "my-distribution",
		RawStruct: cftypes.DistributionSummary{
			Id: aws.String("E1ABCDEF123456"),
			ViewerCertificate: &cftypes.ViewerCertificate{
				ACMCertificateArn: aws.String(certARN),
			},
		},
	}
	otherDist := resource.Resource{
		ID:   "E2XXXXXX999999",
		Name: "other-distribution",
		RawStruct: cftypes.DistributionSummary{
			Id: aws.String("E2XXXXXX999999"),
			ViewerCertificate: &cftypes.ViewerCertificate{
				ACMCertificateArn: aws.String("arn:aws:acm:us-east-1:111122223333:certificate/different-cert"),
			},
		},
	}
	cache := resource.ResourceCache{
		"cf": resource.ResourceCacheEntry{Resources: []resource.Resource{matchingDist, otherDist}},
	}

	checker := acmCheckerByTarget(t, "cf")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "E1ABCDEF123456" {
		t.Errorf("ResourceIDs = %v, want [E1ABCDEF123456]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_ACM_CF_NotFound(t *testing.T) {
	certARN := "arn:aws:acm:us-east-1:111122223333:certificate/abc-123"
	source := resource.Resource{
		ID:   certARN,
		Name: "example.com",
	}

	otherDist := resource.Resource{
		ID:   "E2XXXXXX999999",
		Name: "other-distribution",
		RawStruct: cftypes.DistributionSummary{
			Id: aws.String("E2XXXXXX999999"),
			ViewerCertificate: &cftypes.ViewerCertificate{
				ACMCertificateArn: aws.String("arn:aws:acm:us-east-1:111122223333:certificate/different-cert"),
			},
		},
	}
	cache := resource.ResourceCache{
		"cf": resource.ResourceCacheEntry{Resources: []resource.Resource{otherDist}},
	}

	checker := acmCheckerByTarget(t, "cf")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_ACM_CF_CacheMiss(t *testing.T) {
	source := resource.Resource{
		ID:   "arn:aws:acm:us-east-1:111122223333:certificate/abc-123",
		Name: "example.com",
	}

	checker := acmCheckerByTarget(t, "cf")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (empty cache, nil clients)", result.Count)
	}
}

func TestRelated_ACM_CF_EmptyCertARN(t *testing.T) {
	source := resource.Resource{
		ID:   "",
		Name: "",
	}
	cache := resource.ResourceCache{
		"cf": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "E1ABCDEF123456"},
		}},
	}

	checker := acmCheckerByTarget(t, "cf")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty cert ARN)", result.Count)
	}
}

// --- acm→apigw: undeterminable from cache, returns Count: 0 ---

func TestRelated_ACM_APIGW_ReturnsZero(t *testing.T) {
	source := resource.Resource{
		ID:   "arn:aws:acm::111122223333:certificate/abc-123",
		Name: "example.com",
	}
	checker := acmCheckerByTarget(t, "apigw")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (undeterminable from cache)", result.Count)
	}
	if result.TargetType != "apigw" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "apigw")
	}
}

// --- acm→r53: undeterminable from cache, returns Count: 0 ---

func TestRelated_ACM_R53_ReturnsZero(t *testing.T) {
	source := resource.Resource{
		ID:   "arn:aws:acm::111122223333:certificate/abc-123",
		Name: "example.com",
	}
	checker := acmCheckerByTarget(t, "r53")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (undeterminable from cache)", result.Count)
	}
	if result.TargetType != "r53" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "r53")
	}
}
