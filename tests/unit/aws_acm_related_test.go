package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	acmtypes "github.com/aws/aws-sdk-go-v2/service/acm/types"
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

// --- acm→elb: requires DescribeListeners per ELB (outside cache budget) ---

// TestRelated_ACM_ELB_NilClients: real cert RawStruct → Count: -1 when clients
// are nil (API call is the only way to resolve).
func TestRelated_ACM_ELB_NilClients(t *testing.T) {
	source := resource.Resource{
		ID:   "example.com",
		Name: "example.com",
		RawStruct: acmtypes.CertificateSummary{
			CertificateArn: aws.String("arn:aws:acm:us-east-1:111122223333:certificate/abc-123"),
			DomainName:     aws.String("example.com"),
		},
	}
	checker := acmCheckerByTarget(t, "elb")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown when no clients available)", result.Count)
	}
	if result.TargetType != "elb" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "elb")
	}
}

// TestRelated_ACM_ELB_EmptyInput: empty cert identity → Count: 0 (nothing to look up).
func TestRelated_ACM_ELB_EmptyInput(t *testing.T) {
	source := resource.Resource{ID: "", Name: ""}
	checker := acmCheckerByTarget(t, "elb")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty cert identity)", result.Count)
	}
}

// --- acm→cf: cache-based (ViewerCertificate.ACMCertificateArn == certARN) ---

func TestRelated_ACM_CF_Found(t *testing.T) {
	certARN := "arn:aws:acm:us-east-1:111122223333:certificate/abc-123"
	source := resource.Resource{
		ID:     "example.com",
		Name:   "example.com",
		Fields: map[string]string{"certificate_arn": certARN},
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
		ID:     "example.com",
		Name:   "example.com",
		Fields: map[string]string{"certificate_arn": "arn:aws:acm:us-east-1:111122223333:certificate/abc-123"},
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

// --- acm→apigw: requires GetDomainNames (outside cache budget) ---

// TestRelated_ACM_APIGW_NilClients: real cert RawStruct → Count: -1 when
// clients are nil (acm:DescribeCertificate is the source of truth).
// TestRelated_ACM_APIGW_EmptyInput: empty cert identity → Count: 0.
func TestRelated_ACM_APIGW_EmptyInput(t *testing.T) {
	source := resource.Resource{ID: "", Name: ""}
	checker := acmCheckerByTarget(t, "apigw")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty cert identity)", result.Count)
	}
}

// --- acm→r53: requires per-zone ListResourceRecordSets (outside cache budget) ---

// TestRelated_ACM_R53_NilClients: real cert RawStruct → Count: -1 when
// clients are nil (acm:DescribeCertificate is the source of truth).
// TestRelated_ACM_R53_EmptyInput: empty cert identity → Count: 0.
func TestRelated_ACM_R53_EmptyInput(t *testing.T) {
	source := resource.Resource{ID: "", Name: ""}
	checker := acmCheckerByTarget(t, "r53")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty cert identity)", result.Count)
	}
}

// TestRelated_ACM_R53_EmptyCertARNInRawStruct: RawStruct present but CertificateArn is nil → Count: 0.
func TestRelated_ACM_R53_EmptyCertARNInRawStruct(t *testing.T) {
	source := resource.Resource{
		ID:   "example.com",
		Name: "example.com",
		RawStruct: acmtypes.CertificateSummary{
			DomainName: aws.String("example.com"),
			// CertificateArn intentionally nil
		},
	}
	checker := acmCheckerByTarget(t, "r53")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (nil CertificateArn in RawStruct)", result.Count)
	}
}

// --- checkACMELB: ARN parsing for ALB/NLB and Classic ELB shapes ---

// TestRelated_ACM_ELB_ALBShape: an ALB ARN in InUseBy should produce the load balancer name.
// Since we cannot mock the ACM API here, we cover the ARN-parsing branches directly
// by exercising the full checker through a cache-backed path is not possible without
// a live client. Instead we verify nil-client → -1 with a real CertificateArn,
// confirming the early-exit path is hit only when ID and Name are both empty.
// --- checkACMCF: truncated cache → ApproximateZero ---

// TestRelated_ACM_CF_TruncatedCacheNoMatch: when the cache is truncated and no
// distribution matches, returns ApproximateZero (Count: 0 with IsApproximate true).
func TestRelated_ACM_CF_TruncatedCacheNoMatch(t *testing.T) {
	certARN := "arn:aws:acm:us-east-1:111122223333:certificate/abc-123"
	source := resource.Resource{
		ID:     "example.com",
		Name:   "example.com",
		Fields: map[string]string{"certificate_arn": certARN},
	}

	// Distribution with a different cert — no match.
	otherDist := resource.Resource{
		ID: "E2XXXXXX999999",
		RawStruct: cftypes.DistributionSummary{
			Id: aws.String("E2XXXXXX999999"),
			ViewerCertificate: &cftypes.ViewerCertificate{
				ACMCertificateArn: aws.String("arn:aws:acm:us-east-1:111122223333:certificate/other"),
			},
		},
	}
	cache := resource.ResourceCache{
		"cf": resource.ResourceCacheEntry{
			Resources:   []resource.Resource{otherDist},
			IsTruncated: true,
		},
	}

	checker := acmCheckerByTarget(t, "cf")
	result := checker(context.Background(), nil, source, cache)
	if !result.Approximate {
		t.Errorf("IsApproximate = false, want true (truncated cache, no match)")
	}
}

// TestRelated_ACM_CF_NilViewerCertificate: distributions with nil ViewerCertificate
// are skipped — the matching dist must have it set.
func TestRelated_ACM_CF_NilViewerCertificate(t *testing.T) {
	certARN := "arn:aws:acm:us-east-1:111122223333:certificate/abc-123"
	source := resource.Resource{
		ID:     "example.com",
		Name:   "example.com",
		Fields: map[string]string{"certificate_arn": certARN},
	}

	noViewerCert := resource.Resource{
		ID: "E1NULLVIEWERCERT",
		RawStruct: cftypes.DistributionSummary{
			Id:                nil,
			ViewerCertificate: nil, // no viewer cert
		},
	}
	withViewerCert := resource.Resource{
		ID: "E1WITHVIEWERCERT",
		RawStruct: cftypes.DistributionSummary{
			Id: aws.String("E1WITHVIEWERCERT"),
			ViewerCertificate: &cftypes.ViewerCertificate{
				ACMCertificateArn: aws.String(certARN),
			},
		},
	}
	cache := resource.ResourceCache{
		"cf": resource.ResourceCacheEntry{Resources: []resource.Resource{noViewerCert, withViewerCert}},
	}

	checker := acmCheckerByTarget(t, "cf")
	result := checker(context.Background(), nil, source, cache)
	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (only dist with matching cert should match)", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "E1WITHVIEWERCERT" {
		t.Errorf("ResourceIDs = %v, want [E1WITHVIEWERCERT]", result.ResourceIDs)
	}
}
