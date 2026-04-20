package unit_test

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// Inline fake CloudFront client used by checkCfLambda / checkCfLogs tests.
// Implements CloudFrontAPI (ListDistributions + GetDistributionConfig).
// ---------------------------------------------------------------------------

type fakeCFClient struct {
	listOut *cloudfront.ListDistributionsOutput
	listErr error
	getOut  *cloudfront.GetDistributionConfigOutput
	getErr  error
}

func (f *fakeCFClient) ListDistributions(_ context.Context, _ *cloudfront.ListDistributionsInput, _ ...func(*cloudfront.Options)) (*cloudfront.ListDistributionsOutput, error) {
	return f.listOut, f.listErr
}

func (f *fakeCFClient) GetDistributionConfig(_ context.Context, _ *cloudfront.GetDistributionConfigInput, _ ...func(*cloudfront.Options)) (*cloudfront.GetDistributionConfigOutput, error) {
	return f.getOut, f.getErr
}

// fakeCFServiceClients builds a *awsclient.ServiceClients with only CloudFront populated.
func fakeCFServiceClients(cf *fakeCFClient) *awsclient.ServiceClients {
	return &awsclient.ServiceClients{CloudFront: cf}
}

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
		"r53": {"Route 53 Zones", true},
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

// --- checkCfR53 tests (cache-only zone-name suffix match) ---

// TestRelated_CF_R53_NoAliasesReturnsZero: distribution without any alias
// domains has no possible zone match — returns Count: 0 (definitive).
func TestRelated_CF_R53_NoAliasesReturnsZero(t *testing.T) {
	res := resource.Resource{
		ID:     "E1A2B3C4D5E6F7",
		Fields: map[string]string{},
	}
	checker := cfCheckerByTarget(t, "r53")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (distribution has no aliases — nothing to match)", result.Count)
	}
	if result.TargetType != "r53" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "r53")
	}
}

// TestRelated_CF_R53_EmptyInput: empty distribution id → Count: 0.
func TestRelated_CF_R53_EmptyInput(t *testing.T) {
	res := resource.Resource{ID: "", Fields: map[string]string{}}
	checker := cfCheckerByTarget(t, "r53")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty distribution id)", result.Count)
	}
}

// --- checkCfR53: zone suffix match from RawStruct aliases ---

// TestRelated_CF_R53_MatchByRawStructAlias: distribution with alias "www.example.com"
// matches zone "example.com." (suffix match after normalization).
func TestRelated_CF_R53_MatchByRawStructAlias(t *testing.T) {
	zoneRes := resource.Resource{
		ID:     "/hostedzone/Z123456ABCDEF",
		Name:   "example.com.",
		Fields: map[string]string{"name": "example.com."},
	}
	cache := resource.ResourceCache{
		"r53": resource.ResourceCacheEntry{Resources: []resource.Resource{zoneRes}},
	}

	res := resource.Resource{
		ID:     "E1A2B3C4D5E6F7",
		Fields: map[string]string{},
		RawStruct: cftypes.DistributionSummary{
			Aliases: &cftypes.Aliases{
				Items:    []string{"www.example.com"},
				Quantity: aws.Int32(1),
			},
		},
	}

	checker := cfCheckerByTarget(t, "r53")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "/hostedzone/Z123456ABCDEF" {
		t.Errorf("ResourceIDs = %v, want [/hostedzone/Z123456ABCDEF]", result.ResourceIDs)
	}
}

// TestRelated_CF_R53_MatchByFieldsFallback: distribution with no RawStruct but
// aliases in Fields["aliases"] (comma-joined) still produces a zone match.
func TestRelated_CF_R53_MatchByFieldsFallback(t *testing.T) {
	zoneRes := resource.Resource{
		ID:     "/hostedzone/ZFALLBACK",
		Name:   "fallback.io",
		Fields: map[string]string{"name": "fallback.io"},
	}
	cache := resource.ResourceCache{
		"r53": resource.ResourceCacheEntry{Resources: []resource.Resource{zoneRes}},
	}

	// No RawStruct — fallback to Fields["aliases"].
	res := resource.Resource{
		ID:     "EFALLBACK123",
		Fields: map[string]string{"aliases": "app.fallback.io, www.fallback.io"},
	}

	checker := cfCheckerByTarget(t, "r53")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "/hostedzone/ZFALLBACK" {
		t.Errorf("ResourceIDs = %v, want [/hostedzone/ZFALLBACK]", result.ResourceIDs)
	}
}

// TestRelated_CF_R53_NilCacheWithAliases: aliases present but nil zone cache → Count: -1.
func TestRelated_CF_R53_NilCacheWithAliases(t *testing.T) {
	res := resource.Resource{
		ID:     "E1A2B3C4D5E6F7",
		Fields: map[string]string{},
		RawStruct: cftypes.DistributionSummary{
			Aliases: &cftypes.Aliases{
				Items:    []string{"www.example.com"},
				Quantity: aws.Int32(1),
			},
		},
	}

	checker := cfCheckerByTarget(t, "r53")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (aliases present but nil zone cache)", result.Count)
	}
}

// TestRelated_CF_R53_TruncatedCacheNoMatch: truncated zone cache, alias doesn't
// match any loaded zone → ApproximateZero.
func TestRelated_CF_R53_TruncatedCacheNoMatch(t *testing.T) {
	zoneRes := resource.Resource{
		ID:     "/hostedzone/ZOTHER",
		Name:   "other.net",
		Fields: map[string]string{"name": "other.net"},
	}
	cache := resource.ResourceCache{
		"r53": resource.ResourceCacheEntry{
			Resources:   []resource.Resource{zoneRes},
			IsTruncated: true,
		},
	}

	res := resource.Resource{
		ID:     "ETRUNC123",
		Fields: map[string]string{},
		RawStruct: cftypes.DistributionSummary{
			Aliases: &cftypes.Aliases{
				Items:    []string{"www.example.com"},
				Quantity: aws.Int32(1),
			},
		},
	}

	checker := cfCheckerByTarget(t, "r53")
	result := checker(context.Background(), nil, res, cache)
	if !result.Approximate {
		t.Errorf("IsApproximate = false, want true (truncated cache, no match)")
	}
}

// TestRelated_CF_R53_ExactMatch: alias equals zone name exactly (no subdomain).
func TestRelated_CF_R53_ExactMatch(t *testing.T) {
	zoneRes := resource.Resource{
		ID:     "/hostedzone/ZEXACT",
		Name:   "example.com",
		Fields: map[string]string{"name": "example.com"},
	}
	cache := resource.ResourceCache{
		"r53": resource.ResourceCacheEntry{Resources: []resource.Resource{zoneRes}},
	}

	res := resource.Resource{
		ID:     "EEXACT456",
		Fields: map[string]string{},
		RawStruct: cftypes.DistributionSummary{
			Aliases: &cftypes.Aliases{
				Items:    []string{"example.com"},
				Quantity: aws.Int32(1),
			},
		},
	}

	checker := cfCheckerByTarget(t, "r53")
	result := checker(context.Background(), nil, res, cache)
	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (exact alias == zone name)", result.Count)
	}
}

// --- checkCfAlarm: cache scan by DistributionId dimension ---

// cfAlarmCheckerByTarget returns the alarm checker registered under "cf".
func cfAlarmCheckerByTarget(t *testing.T) resource.RelatedChecker {
	t.Helper()
	return cfCheckerByTarget(t, "alarm")
}

// TestRelated_CF_Alarm_MatchByDistributionId: alarm with DistributionId dimension
// matching the distribution ID is returned.
func TestRelated_CF_Alarm_MatchByDistributionId(t *testing.T) {
	const distID = "E1TESTDISTID"

	alarmRes := resource.Resource{
		ID: "cf-error-rate-alarm",
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("cf-error-rate-alarm"),
			Dimensions: []cwtypes.Dimension{
				{Name: aws.String("DistributionId"), Value: aws.String(distID)},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}

	res := resource.Resource{ID: distID, Fields: map[string]string{}}

	checker := cfAlarmCheckerByTarget(t)
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "cf-error-rate-alarm" {
		t.Errorf("ResourceIDs = %v, want [cf-error-rate-alarm]", result.ResourceIDs)
	}
}

// TestRelated_CF_Alarm_WrongDimension: alarm with a non-matching dimension is
// not returned.
func TestRelated_CF_Alarm_WrongDimension(t *testing.T) {
	alarmRes := resource.Resource{
		ID: "other-alarm",
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("other-alarm"),
			Dimensions: []cwtypes.Dimension{
				{Name: aws.String("DistributionId"), Value: aws.String("E9DIFFERENT")},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}

	res := resource.Resource{ID: "E1TESTDISTID", Fields: map[string]string{}}

	checker := cfAlarmCheckerByTarget(t)
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (dimension mismatch)", result.Count)
	}
}

// TestRelated_CF_Alarm_EmptyDistID: empty distribution ID → Count: 0.
func TestRelated_CF_Alarm_EmptyDistID(t *testing.T) {
	res := resource.Resource{ID: "", Fields: map[string]string{}}
	checker := cfAlarmCheckerByTarget(t)
	result := checker(context.Background(), nil, res, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty distribution ID)", result.Count)
	}
}

// TestRelated_CF_Alarm_NilCache: alarm cache miss → Count: -1.
func TestRelated_CF_Alarm_NilCache(t *testing.T) {
	res := resource.Resource{ID: "E1TESTDISTID", Fields: map[string]string{}}
	checker := cfAlarmCheckerByTarget(t)
	result := checker(context.Background(), nil, res, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (empty alarm cache)", result.Count)
	}
}

// TestRelated_CF_Alarm_TruncatedCacheNoMatch: truncated alarm cache, no match → ApproximateZero.
func TestRelated_CF_Alarm_TruncatedCacheNoMatch(t *testing.T) {
	alarmRes := resource.Resource{
		ID: "other-alarm",
		RawStruct: cwtypes.MetricAlarm{
			AlarmName:  aws.String("other-alarm"),
			Dimensions: []cwtypes.Dimension{{Name: aws.String("DistributionId"), Value: aws.String("EOTHER")}},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{
			Resources:   []resource.Resource{alarmRes},
			IsTruncated: true,
		},
	}
	res := resource.Resource{ID: "E1TESTDISTID", Fields: map[string]string{}}
	checker := cfAlarmCheckerByTarget(t)
	result := checker(context.Background(), nil, res, cache)
	if !result.Approximate {
		t.Errorf("IsApproximate = false, want true (truncated cache, no match)")
	}
}

// TestRelated_CF_Alarm_AlarmWithNoRawStruct: alarm entry without a MetricAlarm
// RawStruct is skipped gracefully.
func TestRelated_CF_Alarm_AlarmWithNoRawStruct(t *testing.T) {
	const distID = "E1TESTDISTID"
	noRaw := resource.Resource{
		ID: "bare-alarm",
		// RawStruct is nil — should be skipped, not panic.
	}
	matchingAlarm := resource.Resource{
		ID: "matching-alarm",
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("matching-alarm"),
			Dimensions: []cwtypes.Dimension{
				{Name: aws.String("DistributionId"), Value: aws.String(distID)},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{noRaw, matchingAlarm}},
	}
	res := resource.Resource{ID: distID, Fields: map[string]string{}}
	checker := cfAlarmCheckerByTarget(t)
	result := checker(context.Background(), nil, res, cache)
	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (only matching alarm should be returned)", result.Count)
	}
}

// --- checkCfLambda: nil client path ---

// TestRelated_CF_Lambda_NilClients: no CloudFront client → Count: -1.
func TestRelated_CF_Lambda_NilClients(t *testing.T) {
	res := resource.Resource{ID: "E1A2B3C4D5E6F7", Fields: map[string]string{}}
	checker := cfCheckerByTarget(t, "lambda")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (no CloudFront client)", result.Count)
	}
	if result.TargetType != "lambda" {
		t.Errorf("TargetType = %q, want lambda", result.TargetType)
	}
}

// TestRelated_CF_Lambda_EmptyDistID: empty distribution ID → Count: 0 (no API call).
func TestRelated_CF_Lambda_EmptyDistID(t *testing.T) {
	res := resource.Resource{ID: "", Fields: map[string]string{}}
	checker := cfCheckerByTarget(t, "lambda")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty distribution ID)", result.Count)
	}
}

// --- checkCfLogs: nil client path ---

// TestRelated_CF_Logs_EmptyDistID: empty distribution ID → Count: 0 (no API call).
func TestRelated_CF_Logs_EmptyDistID(t *testing.T) {
	res := resource.Resource{ID: "", Fields: map[string]string{}}
	checker := cfCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty distribution ID)", result.Count)
	}
}

// --- checkCfS3: no origins / wrong struct ---

// TestRelated_CF_S3_NoOrigins: distribution with nil Origins → Count: 0.
func TestRelated_CF_S3_NoOrigins(t *testing.T) {
	res := resource.Resource{
		ID:        "ENOORIGINS",
		Fields:    map[string]string{},
		RawStruct: cftypes.DistributionSummary{Origins: nil},
	}
	checker := cfCheckerByTarget(t, "s3")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (nil Origins)", result.Count)
	}
}

// TestRelated_CF_S3_OriginWithNilDomainName: origin with nil DomainName is skipped.
func TestRelated_CF_S3_OriginWithNilDomainName(t *testing.T) {
	s3Res := resource.Resource{ID: "some-bucket", Fields: map[string]string{}}
	cache := resource.ResourceCache{
		"s3": resource.ResourceCacheEntry{Resources: []resource.Resource{s3Res}},
	}
	res := resource.Resource{
		ID:     "ENILORIGIN",
		Fields: map[string]string{},
		RawStruct: cftypes.DistributionSummary{
			Origins: &cftypes.Origins{
				Items:    []cftypes.Origin{{DomainName: nil}},
				Quantity: aws.Int32(1),
			},
		},
	}
	checker := cfCheckerByTarget(t, "s3")
	result := checker(context.Background(), nil, res, cache)
	// nil domain name → no bucket name extracted → Count: 0
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (nil DomainName in origin)", result.Count)
	}
}

// TestRelated_CF_S3_RegionalOriginFormat: regional S3 origin
// "{bucket}.s3.{region}.amazonaws.com" is correctly parsed.
func TestRelated_CF_S3_RegionalOriginFormat(t *testing.T) {
	s3Res := resource.Resource{ID: "regional-bucket", Fields: map[string]string{}}
	cache := resource.ResourceCache{
		"s3": resource.ResourceCacheEntry{Resources: []resource.Resource{s3Res}},
	}
	res := resource.Resource{
		ID:     "EREGIONAL",
		Fields: map[string]string{},
		RawStruct: cftypes.DistributionSummary{
			Origins: &cftypes.Origins{
				Items: []cftypes.Origin{
					{DomainName: aws.String("regional-bucket.s3.us-west-2.amazonaws.com")},
				},
				Quantity: aws.Int32(1),
			},
		},
	}
	checker := cfCheckerByTarget(t, "s3")
	result := checker(context.Background(), nil, res, cache)
	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (regional S3 origin format)", result.Count)
	}
}

// TestRelated_CF_S3_WrongRawStruct: non-DistributionSummary RawStruct → Count: -1.
func TestRelated_CF_S3_WrongRawStruct(t *testing.T) {
	res := resource.Resource{
		ID:        "EWRONGRAW",
		Fields:    map[string]string{},
		RawStruct: "not-a-distribution",
	}
	checker := cfCheckerByTarget(t, "s3")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (wrong RawStruct type)", result.Count)
	}
}

// --- checkCfELB: no ELB origins / wrong struct ---

// TestRelated_CF_ELB_NoOrigins: distribution with nil Origins → Count: 0.
func TestRelated_CF_ELB_NoOrigins(t *testing.T) {
	res := resource.Resource{
		ID:        "ENOORIGINS",
		Fields:    map[string]string{},
		RawStruct: cftypes.DistributionSummary{Origins: nil},
	}
	checker := cfCheckerByTarget(t, "elb")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (nil Origins)", result.Count)
	}
}

// TestRelated_CF_ELB_NonELBOriginsOnly: origin domains without ".elb.amazonaws.com"
// result in Count: 0 without touching the cache.
func TestRelated_CF_ELB_NonELBOriginsOnly(t *testing.T) {
	res := resource.Resource{
		ID:     "ENONS3",
		Fields: map[string]string{},
		RawStruct: cftypes.DistributionSummary{
			Origins: &cftypes.Origins{
				Items: []cftypes.Origin{
					{DomainName: aws.String("custom-origin.example.com")},
				},
				Quantity: aws.Int32(1),
			},
		},
	}
	checker := cfCheckerByTarget(t, "elb")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (non-ELB origin, no cache lookup needed)", result.Count)
	}
}

// --- checkCfWAF: empty WebACLId ---

// TestRelated_CF_WAF_EmptyWebACLId: WebACLId is empty string → Count: 0.
func TestRelated_CF_WAF_EmptyWebACLId(t *testing.T) {
	res := resource.Resource{
		ID:        "EWAFEMPTY",
		Fields:    map[string]string{},
		RawStruct: cftypes.DistributionSummary{WebACLId: aws.String("")},
	}
	checker := cfCheckerByTarget(t, "waf")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty WebACLId string)", result.Count)
	}
}

// --- checkCfACM: ID-based match fallback ---

// TestRelated_CF_ACM_MatchByID: when resource ID equals the cert ARN (no certificate_arn field).
func TestRelated_CF_ACM_MatchByID(t *testing.T) {
	const certARN = "arn:aws:acm:us-east-1:123:certificate/xyz-789"
	acmRes := resource.Resource{
		ID:     certARN,
		Name:   "xyz-789",
		Fields: map[string]string{}, // no certificate_arn field — ID fallback
	}
	cache := resource.ResourceCache{
		"acm": resource.ResourceCacheEntry{Resources: []resource.Resource{acmRes}},
	}

	res := resource.Resource{
		ID:     "E1A2B3C4D5E6F7",
		Fields: map[string]string{},
		RawStruct: cftypes.DistributionSummary{
			ViewerCertificate: &cftypes.ViewerCertificate{
				ACMCertificateArn: aws.String(certARN),
			},
		},
	}

	checker := cfCheckerByTarget(t, "acm")
	result := checker(context.Background(), nil, res, cache)
	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (ID fallback match)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkCfLambda — fake client tests (GetDistributionConfig path)
// ---------------------------------------------------------------------------

// TestRelated_CF_Lambda_DefaultBehaviorAssociation: distribution has a Lambda@Edge
// function in DefaultCacheBehavior; checkCfLambda extracts its name.
func TestRelated_CF_Lambda_DefaultBehaviorAssociation(t *testing.T) {
	const distID = "E1LAMBDA123"
	const lambdaARN = "arn:aws:lambda:us-east-1:123456789012:function:my-edge-fn:3"

	clients := fakeCFServiceClients(&fakeCFClient{
		getOut: &cloudfront.GetDistributionConfigOutput{
			DistributionConfig: &cftypes.DistributionConfig{
				DefaultCacheBehavior: &cftypes.DefaultCacheBehavior{
					LambdaFunctionAssociations: &cftypes.LambdaFunctionAssociations{
						Quantity: aws.Int32(1),
						Items: []cftypes.LambdaFunctionAssociation{
							{LambdaFunctionARN: aws.String(lambdaARN)},
						},
					},
				},
			},
		},
	})

	res := resource.Resource{ID: distID, Fields: map[string]string{}}
	checker := cfCheckerByTarget(t, "lambda")
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "my-edge-fn" {
		t.Errorf("ResourceIDs = %v, want [my-edge-fn]", result.ResourceIDs)
	}
}

// TestRelated_CF_Lambda_CacheBehaviorAssociation: Lambda@Edge in a CacheBehavior
// item (not DefaultCacheBehavior) is also extracted.
func TestRelated_CF_Lambda_CacheBehaviorAssociation(t *testing.T) {
	const distID = "E1LAMBDA456"
	const lambdaARN = "arn:aws:lambda:us-east-1:123456789012:function:cb-edge-fn:1"

	clients := fakeCFServiceClients(&fakeCFClient{
		getOut: &cloudfront.GetDistributionConfigOutput{
			DistributionConfig: &cftypes.DistributionConfig{
				CacheBehaviors: &cftypes.CacheBehaviors{
					Quantity: aws.Int32(1),
					Items: []cftypes.CacheBehavior{
						{
							LambdaFunctionAssociations: &cftypes.LambdaFunctionAssociations{
								Quantity: aws.Int32(1),
								Items: []cftypes.LambdaFunctionAssociation{
									{LambdaFunctionARN: aws.String(lambdaARN)},
								},
							},
						},
					},
				},
			},
		},
	})

	res := resource.Resource{ID: distID, Fields: map[string]string{}}
	checker := cfCheckerByTarget(t, "lambda")
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "cb-edge-fn" {
		t.Errorf("ResourceIDs = %v, want [cb-edge-fn]", result.ResourceIDs)
	}
}

// TestRelated_CF_Lambda_NoAssociations: DistributionConfig has no Lambda@Edge
// associations → Count: 0.
func TestRelated_CF_Lambda_NoAssociations(t *testing.T) {
	clients := fakeCFServiceClients(&fakeCFClient{
		getOut: &cloudfront.GetDistributionConfigOutput{
			DistributionConfig: &cftypes.DistributionConfig{
				DefaultCacheBehavior: &cftypes.DefaultCacheBehavior{},
			},
		},
	})

	res := resource.Resource{ID: "E1NOLAMBDA", Fields: map[string]string{}}
	checker := cfCheckerByTarget(t, "lambda")
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no Lambda@Edge associations)", result.Count)
	}
}

// TestRelated_CF_Lambda_NilDistributionConfig: GetDistributionConfig returns nil
// DistributionConfig → Count: 0 (no panic).
func TestRelated_CF_Lambda_NilDistributionConfig(t *testing.T) {
	clients := fakeCFServiceClients(&fakeCFClient{
		getOut: &cloudfront.GetDistributionConfigOutput{DistributionConfig: nil},
	})

	res := resource.Resource{ID: "E1NILCFG", Fields: map[string]string{}}
	checker := cfCheckerByTarget(t, "lambda")
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (nil DistributionConfig)", result.Count)
	}
}

// TestRelated_CF_Lambda_APIError: GetDistributionConfig returns an error
// → Count: -1, Err set.
func TestRelated_CF_Lambda_APIError(t *testing.T) {
	clients := fakeCFServiceClients(&fakeCFClient{
		getErr: errors.New("cloudfront: GetDistributionConfig throttled"),
	})

	res := resource.Resource{ID: "E1APIERR", Fields: map[string]string{}}
	checker := cfCheckerByTarget(t, "lambda")
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (API error)", result.Count)
	}
	if result.Err == nil {
		t.Error("Err = nil, want non-nil on API error")
	}
}

// ---------------------------------------------------------------------------
// checkCfLogs — fake client tests (GetDistributionConfig → Logging path)
// ---------------------------------------------------------------------------

// TestRelated_CF_Logs_LoggingEnabled: Logging.Enabled=true, bucket set →
// bucket name is extracted (stripping ".s3..." suffix).
func TestRelated_CF_Logs_LoggingEnabled(t *testing.T) {
	clients := fakeCFServiceClients(&fakeCFClient{
		getOut: &cloudfront.GetDistributionConfigOutput{
			DistributionConfig: &cftypes.DistributionConfig{
				Logging: &cftypes.LoggingConfig{
					Enabled: aws.Bool(true),
					Bucket:  aws.String("cf-access-logs.s3.amazonaws.com"),
				},
			},
		},
	})

	res := resource.Resource{ID: "E1LOGS123", Fields: map[string]string{}}
	checker := cfCheckerByTarget(t, "logs")
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "cf-access-logs" {
		t.Errorf("ResourceIDs = %v, want [cf-access-logs]", result.ResourceIDs)
	}
}

// TestRelated_CF_Logs_LoggingDisabled: Logging.Enabled=false → Count: 0.
func TestRelated_CF_Logs_LoggingDisabled(t *testing.T) {
	clients := fakeCFServiceClients(&fakeCFClient{
		getOut: &cloudfront.GetDistributionConfigOutput{
			DistributionConfig: &cftypes.DistributionConfig{
				Logging: &cftypes.LoggingConfig{
					Enabled: aws.Bool(false),
					Bucket:  aws.String("some-bucket.s3.amazonaws.com"),
				},
			},
		},
	})

	res := resource.Resource{ID: "E1LOGSDIS", Fields: map[string]string{}}
	checker := cfCheckerByTarget(t, "logs")
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (logging disabled)", result.Count)
	}
}

// TestRelated_CF_Logs_NilLoggingConfig: Logging is nil → Count: 0.
func TestRelated_CF_Logs_NilLoggingConfig(t *testing.T) {
	clients := fakeCFServiceClients(&fakeCFClient{
		getOut: &cloudfront.GetDistributionConfigOutput{
			DistributionConfig: &cftypes.DistributionConfig{Logging: nil},
		},
	})

	res := resource.Resource{ID: "E1NILLOG", Fields: map[string]string{}}
	checker := cfCheckerByTarget(t, "logs")
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (nil Logging)", result.Count)
	}
}

// TestRelated_CF_Logs_NilClientPath: nil clients → Count: -1.
func TestRelated_CF_Logs_NilClientPath(t *testing.T) {
	res := resource.Resource{ID: "E1NILCLIENT", Fields: map[string]string{}}
	checker := cfCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil clients)", result.Count)
	}
}
