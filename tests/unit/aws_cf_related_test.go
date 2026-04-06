package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// TestRelated_CF_Registered verifies all related defs are registered with correct checker presence.
func TestRelated_CF_Registered(t *testing.T) {
	defs := resource.GetRelated("cf")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for cf")
	}

	type expectation struct {
		displayName string
		hasChecker  bool
	}
	expected := map[string]expectation{
		"s3":  {"S3 Buckets (origin)", true},
		"elb": {"Load Balancers (origin)", true},
		"waf": {"WAF Web ACLs", true},
		"acm": {"ACM Certificates", true},
		"r53": {"Route 53 Zones", false},
	}
	for target, want := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == target {
				found = true
				if want.hasChecker && def.Checker == nil {
					t.Errorf("cf %q: Checker should not be nil", target)
				}
				if !want.hasChecker && def.Checker != nil {
					t.Errorf("cf %q: Checker should be nil (stub)", target)
				}
				if def.DisplayName != want.displayName {
					t.Errorf("cf %q: DisplayName = %q, want %q", target, def.DisplayName, want.displayName)
				}
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found", target)
		}
	}
}

// cfCheckerByTarget returns the RelatedChecker for the given target type registered
// under "cf". It fails the test immediately if the checker is nil or not found.
func cfCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("cf") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("cf related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("cf related checker for %s not found", target)
	return nil
}

// --- checkCfS3 tests (match by S3 origin domain) ---

func TestRelated_CF_S3_MatchByOriginDomain(t *testing.T) {
	s3Res := resource.Resource{
		ID:     "my-bucket",
		Name:   "my-bucket",
		Fields: map[string]string{},
	}
	cache := resource.ResourceCache{
		"s3": resource.ResourceCacheEntry{Resources: []resource.Resource{s3Res}},
	}

	res := resource.Resource{
		ID:     "E1A2B3C4D5E6F7",
		Fields: map[string]string{},
		RawStruct: cftypes.DistributionSummary{
			Origins: &cftypes.Origins{
				Items: []cftypes.Origin{
					{DomainName: aws.String("my-bucket.s3.amazonaws.com")},
				},
				Quantity: aws.Int32(1),
			},
		},
	}

	checker := cfCheckerByTarget(t, "s3")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_CF_S3_NoMatch(t *testing.T) {
	s3Res := resource.Resource{
		ID:     "different-bucket",
		Name:   "different-bucket",
		Fields: map[string]string{},
	}
	cache := resource.ResourceCache{
		"s3": resource.ResourceCacheEntry{Resources: []resource.Resource{s3Res}},
	}

	res := resource.Resource{
		ID:     "E1A2B3C4D5E6F7",
		Fields: map[string]string{},
		RawStruct: cftypes.DistributionSummary{
			Origins: &cftypes.Origins{
				Items: []cftypes.Origin{
					{DomainName: aws.String("my-bucket.s3.amazonaws.com")},
				},
				Quantity: aws.Int32(1),
			},
		},
	}

	checker := cfCheckerByTarget(t, "s3")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_CF_S3_NilCache(t *testing.T) {
	cache := resource.ResourceCache{}

	res := resource.Resource{
		ID:     "E1A2B3C4D5E6F7",
		Fields: map[string]string{},
		RawStruct: cftypes.DistributionSummary{
			Origins: &cftypes.Origins{
				Items: []cftypes.Origin{
					{DomainName: aws.String("my-bucket.s3.amazonaws.com")},
				},
				Quantity: aws.Int32(1),
			},
		},
	}

	checker := cfCheckerByTarget(t, "s3")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (empty cache)", result.Count)
	}
}

// --- checkCfELB tests (match by ELB origin domain) ---

func TestRelated_CF_ELB_MatchByOriginDomain(t *testing.T) {
	elbRes := resource.Resource{
		ID:   "my-alb",
		Name: "my-alb",
		Fields: map[string]string{
			"dns_name": "my-alb-123.us-east-1.elb.amazonaws.com",
		},
	}
	cache := resource.ResourceCache{
		"elb": resource.ResourceCacheEntry{Resources: []resource.Resource{elbRes}},
	}

	res := resource.Resource{
		ID:     "E1A2B3C4D5E6F7",
		Fields: map[string]string{},
		RawStruct: cftypes.DistributionSummary{
			Origins: &cftypes.Origins{
				Items: []cftypes.Origin{
					{DomainName: aws.String("my-alb-123.us-east-1.elb.amazonaws.com")},
				},
				Quantity: aws.Int32(1),
			},
		},
	}

	checker := cfCheckerByTarget(t, "elb")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_CF_ELB_NoMatch(t *testing.T) {
	elbRes := resource.Resource{
		ID:   "other-alb",
		Name: "other-alb",
		Fields: map[string]string{
			"dns_name": "other-alb-999.us-east-1.elb.amazonaws.com",
		},
	}
	cache := resource.ResourceCache{
		"elb": resource.ResourceCacheEntry{Resources: []resource.Resource{elbRes}},
	}

	res := resource.Resource{
		ID:     "E1A2B3C4D5E6F7",
		Fields: map[string]string{},
		RawStruct: cftypes.DistributionSummary{
			Origins: &cftypes.Origins{
				Items: []cftypes.Origin{
					{DomainName: aws.String("my-alb-123.us-east-1.elb.amazonaws.com")},
				},
				Quantity: aws.Int32(1),
			},
		},
	}

	checker := cfCheckerByTarget(t, "elb")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_CF_ELB_NilCache(t *testing.T) {
	cache := resource.ResourceCache{}

	res := resource.Resource{
		ID:     "E1A2B3C4D5E6F7",
		Fields: map[string]string{},
		RawStruct: cftypes.DistributionSummary{
			Origins: &cftypes.Origins{
				Items: []cftypes.Origin{
					{DomainName: aws.String("my-alb-123.us-east-1.elb.amazonaws.com")},
				},
				Quantity: aws.Int32(1),
			},
		},
	}

	checker := cfCheckerByTarget(t, "elb")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (empty cache)", result.Count)
	}
}

// --- checkCfWAF tests (match by WebACLId) ---

func TestRelated_CF_WAF_MatchByWebACLId(t *testing.T) {
	wafRes := resource.Resource{
		ID:   "arn:aws:wafv2:us-east-1:123:regional/webacl/my-acl/id123",
		Name: "my-acl",
		Fields: map[string]string{
			"arn": "arn:aws:wafv2:us-east-1:123:regional/webacl/my-acl/id123",
		},
	}
	cache := resource.ResourceCache{
		"waf": resource.ResourceCacheEntry{Resources: []resource.Resource{wafRes}},
	}

	res := resource.Resource{
		ID:     "E1A2B3C4D5E6F7",
		Fields: map[string]string{},
		RawStruct: cftypes.DistributionSummary{
			WebACLId: aws.String("arn:aws:wafv2:us-east-1:123:regional/webacl/my-acl/id123"),
		},
	}

	checker := cfCheckerByTarget(t, "waf")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_CF_WAF_NoWebACL(t *testing.T) {
	wafRes := resource.Resource{
		ID:   "arn:aws:wafv2:us-east-1:123:regional/webacl/my-acl/id123",
		Name: "my-acl",
		Fields: map[string]string{
			"arn": "arn:aws:wafv2:us-east-1:123:regional/webacl/my-acl/id123",
		},
	}
	cache := resource.ResourceCache{
		"waf": resource.ResourceCacheEntry{Resources: []resource.Resource{wafRes}},
	}

	res := resource.Resource{
		ID:     "E1A2B3C4D5E6F7",
		Fields: map[string]string{},
		RawStruct: cftypes.DistributionSummary{
			WebACLId: nil,
		},
	}

	checker := cfCheckerByTarget(t, "waf")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (nil WebACLId)", result.Count)
	}
}

func TestRelated_CF_WAF_NilCache(t *testing.T) {
	cache := resource.ResourceCache{}

	res := resource.Resource{
		ID:     "E1A2B3C4D5E6F7",
		Fields: map[string]string{},
		RawStruct: cftypes.DistributionSummary{
			WebACLId: aws.String("arn:aws:wafv2:us-east-1:123:regional/webacl/my-acl/id123"),
		},
	}

	checker := cfCheckerByTarget(t, "waf")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (empty cache)", result.Count)
	}
}

// --- checkCfACM tests (match by ViewerCertificate ACMCertificateArn) ---

func TestRelated_CF_ACM_MatchByCertARN(t *testing.T) {
	acmRes := resource.Resource{
		ID:     "arn:aws:acm:us-east-1:123:certificate/abc-123",
		Name:   "abc-123",
		Fields: map[string]string{},
	}
	cache := resource.ResourceCache{
		"acm": resource.ResourceCacheEntry{Resources: []resource.Resource{acmRes}},
	}

	res := resource.Resource{
		ID:     "E1A2B3C4D5E6F7",
		Fields: map[string]string{},
		RawStruct: cftypes.DistributionSummary{
			ViewerCertificate: &cftypes.ViewerCertificate{
				ACMCertificateArn: aws.String("arn:aws:acm:us-east-1:123:certificate/abc-123"),
			},
		},
	}

	checker := cfCheckerByTarget(t, "acm")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_CF_ACM_NoCert(t *testing.T) {
	acmRes := resource.Resource{
		ID:     "arn:aws:acm:us-east-1:123:certificate/abc-123",
		Name:   "abc-123",
		Fields: map[string]string{},
	}
	cache := resource.ResourceCache{
		"acm": resource.ResourceCacheEntry{Resources: []resource.Resource{acmRes}},
	}

	res := resource.Resource{
		ID:        "E1A2B3C4D5E6F7",
		Fields:    map[string]string{},
		RawStruct: cftypes.DistributionSummary{},
	}

	checker := cfCheckerByTarget(t, "acm")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (nil ViewerCertificate)", result.Count)
	}
}

func TestRelated_CF_ACM_NilCache(t *testing.T) {
	cache := resource.ResourceCache{}

	res := resource.Resource{
		ID:     "E1A2B3C4D5E6F7",
		Fields: map[string]string{},
		RawStruct: cftypes.DistributionSummary{
			ViewerCertificate: &cftypes.ViewerCertificate{
				ACMCertificateArn: aws.String("arn:aws:acm:us-east-1:123:certificate/abc-123"),
			},
		},
	}

	checker := cfCheckerByTarget(t, "acm")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (empty cache)", result.Count)
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
