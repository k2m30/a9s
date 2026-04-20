package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/wafv2"
	wafv2types "github.com/aws/aws-sdk-go-v2/service/wafv2/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// fakeWAFv2CR — implements WAFv2API (ListWebACLs, ListResourcesForWebACL,
// GetLoggingConfiguration) for WAF related checker tests.
// ---------------------------------------------------------------------------

type fakeWAFv2CR struct {
	listResourcesOutput *wafv2.ListResourcesForWebACLOutput
	listResourcesErr    error
	loggingOutput       *wafv2.GetLoggingConfigurationOutput
	loggingErr          error
}

func (f *fakeWAFv2CR) ListWebACLs(_ context.Context, _ *wafv2.ListWebACLsInput, _ ...func(*wafv2.Options)) (*wafv2.ListWebACLsOutput, error) {
	return &wafv2.ListWebACLsOutput{}, nil
}

func (f *fakeWAFv2CR) ListResourcesForWebACL(_ context.Context, _ *wafv2.ListResourcesForWebACLInput, _ ...func(*wafv2.Options)) (*wafv2.ListResourcesForWebACLOutput, error) {
	if f.listResourcesErr != nil {
		return nil, f.listResourcesErr
	}
	if f.listResourcesOutput != nil {
		return f.listResourcesOutput, nil
	}
	return &wafv2.ListResourcesForWebACLOutput{}, nil
}

func (f *fakeWAFv2CR) GetLoggingConfiguration(_ context.Context, _ *wafv2.GetLoggingConfigurationInput, _ ...func(*wafv2.Options)) (*wafv2.GetLoggingConfigurationOutput, error) {
	if f.loggingErr != nil {
		return nil, f.loggingErr
	}
	if f.loggingOutput != nil {
		return f.loggingOutput, nil
	}
	return &wafv2.GetLoggingConfigurationOutput{}, nil
}

var _ awsclient.WAFv2API = (*fakeWAFv2CR)(nil)

// ---------------------------------------------------------------------------
// fakeCloudFrontWAF — implements CloudFrontAPI + CloudFrontListDistributionsByWebACLIdAPI
// for WAF→CF related checker tests. checkWAFCF does a type assertion on c.CloudFront
// so the fake must satisfy CloudFrontAPI (the field type) AND the narrow API.
// ---------------------------------------------------------------------------

type fakeCloudFrontWAF struct {
	output *cloudfront.ListDistributionsByWebACLIdOutput
	err    error
}

func (f *fakeCloudFrontWAF) ListDistributions(_ context.Context, _ *cloudfront.ListDistributionsInput, _ ...func(*cloudfront.Options)) (*cloudfront.ListDistributionsOutput, error) {
	return &cloudfront.ListDistributionsOutput{}, nil
}

func (f *fakeCloudFrontWAF) GetDistributionConfig(_ context.Context, _ *cloudfront.GetDistributionConfigInput, _ ...func(*cloudfront.Options)) (*cloudfront.GetDistributionConfigOutput, error) {
	return &cloudfront.GetDistributionConfigOutput{}, nil
}

func (f *fakeCloudFrontWAF) ListDistributionsByWebACLId(_ context.Context, _ *cloudfront.ListDistributionsByWebACLIdInput, _ ...func(*cloudfront.Options)) (*cloudfront.ListDistributionsByWebACLIdOutput, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.output != nil {
		return f.output, nil
	}
	return &cloudfront.ListDistributionsByWebACLIdOutput{}, nil
}

var _ awsclient.CloudFrontAPI = (*fakeCloudFrontWAF)(nil)
var _ awsclient.CloudFrontListDistributionsByWebACLIdAPI = (*fakeCloudFrontWAF)(nil)

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

// --- Alarm checker (Pattern C — cache scan, WebACL dimension match) ---

// TestRelated_WAF_Alarm_MatchByWebACLDimension verifies that an alarm with
// dimension "WebACL" equal to the WAF resource name is returned.
func TestRelated_WAF_Alarm_MatchByWebACLDimension(t *testing.T) {
	res := wafSrcResource() // name = "my-waf"

	alarmRes := resource.Resource{
		ID: "waf-blocked-requests-alarm",
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("waf-blocked-requests-alarm"),
			Dimensions: []cwtypes.Dimension{
				{Name: aws.String("WebACL"), Value: aws.String("my-waf")},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}

	checker := wafCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "waf-blocked-requests-alarm" {
		t.Errorf("ResourceIDs = %v, want [waf-blocked-requests-alarm]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

// TestRelated_WAF_Alarm_NoMatch verifies that alarms with non-matching
// WebACL dimension return Count=0.
func TestRelated_WAF_Alarm_NoMatch(t *testing.T) {
	res := wafSrcResource() // name = "my-waf"

	alarmRes := resource.Resource{
		ID: "other-waf-alarm",
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("other-waf-alarm"),
			Dimensions: []cwtypes.Dimension{
				{Name: aws.String("WebACL"), Value: aws.String("different-waf")},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}

	checker := wafCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

// TestRelated_WAF_Alarm_CacheMissNoClients verifies that an empty alarm cache
// with no clients returns Count=-1 (unknown).
func TestRelated_WAF_Alarm_CacheMissNoClients(t *testing.T) {
	res := wafSrcResource()

	checker := wafCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (cache miss, no clients)", result.Count)
	}
}

// TestRelated_WAF_Alarm_EmptyName verifies that a WAF resource with no name
// returns Count=0 immediately.
func TestRelated_WAF_Alarm_EmptyName(t *testing.T) {
	res := resource.Resource{
		ID:     "a1b2c3d4-1234-5678-abcd-111111111111",
		Name:   "",
		Fields: map[string]string{"name": "", "scope": "REGIONAL"},
	}

	checker := wafCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty name)", result.Count)
	}
}

// --- Logs checker (Pattern C — getLoggingConfiguration, empty ARN → 0) ---

// TestRelated_WAF_Logs_EmptyARN verifies that a WAF resource without an ARN
// returns Count=0 (no logging configured by definition).
func TestRelated_WAF_Logs_EmptyARN(t *testing.T) {
	res := resource.Resource{
		ID:     "a1b2c3d4-1234-5678-abcd-111111111111",
		Name:   "my-waf",
		Fields: map[string]string{"name": "my-waf", "scope": "REGIONAL"},
	}

	checker := wafCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty ARN)", result.Count)
	}
}

// TestRelated_WAF_Logs_NilClients verifies that a WAF resource with an ARN
// but nil clients returns Count=-1 (cannot call GetLoggingConfiguration).
func TestRelated_WAF_Logs_NilClients(t *testing.T) {
	res := resource.Resource{
		ID:   "a1b2c3d4-1234-5678-abcd-111111111111",
		Name: "my-waf",
		Fields: map[string]string{
			"name":  "my-waf",
			"scope": "REGIONAL",
			"arn":   "arn:aws:wafv2:us-east-1:123456789012:regional/webacl/my-waf/a1b2c3d4-1234",
		},
	}

	checker := wafCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil clients)", result.Count)
	}
}

// --- ELB checker: real dispatch with fake WAFv2 ---

// TestRelated_WAF_ELB_ExtractsNameFromARN verifies that the elb checker correctly
// extracts the load balancer name from the ARN segment parts[len(parts)-2].
func TestRelated_WAF_ELB_ExtractsNameFromARN(t *testing.T) {
	// ARN format: arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/my-alb/abcdef012345
	// parts[len-2] = "my-alb"
	res := resource.Resource{
		ID:   "a1b2c3d4-5678-90ab-cdef-111111111111",
		Name: "my-waf",
		Fields: map[string]string{
			"name":  "my-waf",
			"scope": "REGIONAL",
			"arn":   "arn:aws:wafv2:us-east-1:123456789012:regional/webacl/my-waf/a1b2c3d4",
		},
	}

	fake := &fakeWAFv2CR{
		listResourcesOutput: &wafv2.ListResourcesForWebACLOutput{
			ResourceArns: []string{
				"arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/my-alb/abcdef012345",
			},
		},
	}
	clients := &awsclient.ServiceClients{WAFv2: fake}

	checker := wafCheckerByTarget(t, "elb")
	result := checker(context.Background(), clients, res, nil)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "my-alb" {
		t.Errorf("ResourceIDs = %v, want [my-alb]", result.ResourceIDs)
	}
}

// TestRelated_WAF_ELB_NoMatch verifies that when ListResourcesForWebACL returns
// no ARNs the checker returns Count=0.
func TestRelated_WAF_ELB_NoMatch(t *testing.T) {
	res := resource.Resource{
		ID:     "a1b2c3d4-5678-90ab-cdef-111111111111",
		Name:   "my-waf",
		Fields: map[string]string{"arn": "arn:aws:wafv2:us-east-1:123456789012:regional/webacl/my-waf/a1b2c3d4"},
	}

	fake := &fakeWAFv2CR{
		listResourcesOutput: &wafv2.ListResourcesForWebACLOutput{ResourceArns: []string{}},
	}
	clients := &awsclient.ServiceClients{WAFv2: fake}

	checker := wafCheckerByTarget(t, "elb")
	result := checker(context.Background(), clients, res, nil)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no ARNs returned)", result.Count)
	}
}

// TestRelated_WAF_ELB_ShortARNSkipped verifies that ARNs with fewer than 3
// slash-delimited parts are skipped (no panic, no spurious IDs).
func TestRelated_WAF_ELB_ShortARNSkipped(t *testing.T) {
	res := resource.Resource{
		ID:     "a1b2c3d4-5678-90ab-cdef-111111111111",
		Name:   "my-waf",
		Fields: map[string]string{"arn": "arn:aws:wafv2:us-east-1:123456789012:regional/webacl/my-waf/a1b2c3d4"},
	}

	fake := &fakeWAFv2CR{
		listResourcesOutput: &wafv2.ListResourcesForWebACLOutput{
			ResourceArns: []string{"only/two"},
		},
	}
	clients := &awsclient.ServiceClients{WAFv2: fake}

	checker := wafCheckerByTarget(t, "elb")
	result := checker(context.Background(), clients, res, nil)

	// ARN "only/two" splits to ["only","two"] — len < 3, skipped.
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (short ARN skipped)", result.Count)
	}
}

// --- APIGW checker: real dispatch with fake WAFv2 ---

// TestRelated_WAF_APIGW_ExtractsAPIIDFromARN verifies that the apigw checker
// correctly extracts the API ID from the restapis path segment.
func TestRelated_WAF_APIGW_ExtractsAPIIDFromARN(t *testing.T) {
	// ARN format: arn:aws:apigateway:us-east-1::/restapis/abc123def/stages/prod
	res := resource.Resource{
		ID:   "a1b2c3d4-5678-90ab-cdef-111111111111",
		Name: "my-waf",
		Fields: map[string]string{
			"arn": "arn:aws:wafv2:us-east-1:123456789012:regional/webacl/my-waf/a1b2c3d4",
		},
	}

	fake := &fakeWAFv2CR{
		listResourcesOutput: &wafv2.ListResourcesForWebACLOutput{
			ResourceArns: []string{
				"arn:aws:apigateway:us-east-1::/restapis/abc123def/stages/prod",
			},
		},
	}
	clients := &awsclient.ServiceClients{WAFv2: fake}

	checker := wafCheckerByTarget(t, "apigw")
	result := checker(context.Background(), clients, res, nil)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "abc123def" {
		t.Errorf("ResourceIDs = %v, want [abc123def]", result.ResourceIDs)
	}
}

// TestRelated_WAF_APIGW_NoRestAPIsInARN verifies that ARNs without /restapis/
// are skipped and Count=0 is returned.
func TestRelated_WAF_APIGW_NoRestAPIsInARN(t *testing.T) {
	res := resource.Resource{
		ID:     "a1b2c3d4-5678-90ab-cdef-111111111111",
		Name:   "my-waf",
		Fields: map[string]string{"arn": "arn:aws:wafv2:us-east-1:123456789012:regional/webacl/my-waf/a1b2c3d4"},
	}

	fake := &fakeWAFv2CR{
		listResourcesOutput: &wafv2.ListResourcesForWebACLOutput{
			ResourceArns: []string{"arn:aws:apigateway:us-east-1::/apis/xyz789"},
		},
	}
	clients := &awsclient.ServiceClients{WAFv2: fake}

	checker := wafCheckerByTarget(t, "apigw")
	result := checker(context.Background(), clients, res, nil)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no /restapis/ in ARN)", result.Count)
	}
}

// --- Logs checker: real dispatch with fake WAFv2 ---

// TestRelated_WAF_Logs_CWLogGroupNameExtracted verifies that the logs checker
// extracts the log-group name from a CW Logs ARN containing ":log-group:".
func TestRelated_WAF_Logs_CWLogGroupNameExtracted(t *testing.T) {
	// CW Logs ARN: arn:aws:logs:us-east-1:123456789012:log-group:/aws/waf/my-waf:*
	res := resource.Resource{
		ID:   "a1b2c3d4-5678-90ab-cdef-111111111111",
		Name: "my-waf",
		Fields: map[string]string{
			"arn": "arn:aws:wafv2:us-east-1:123456789012:regional/webacl/my-waf/a1b2c3d4",
		},
	}

	fake := &fakeWAFv2CR{
		loggingOutput: &wafv2.GetLoggingConfigurationOutput{
			LoggingConfiguration: &wafv2types.LoggingConfiguration{
				ResourceArn: aws.String("arn:aws:wafv2:us-east-1:123456789012:regional/webacl/my-waf/a1b2c3d4"),
				LogDestinationConfigs: []string{
					"arn:aws:logs:us-east-1:123456789012:log-group:/aws/waf/my-waf:*",
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{WAFv2: fake}

	checker := wafCheckerByTarget(t, "logs")
	result := checker(context.Background(), clients, res, nil)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "/aws/waf/my-waf" {
		t.Errorf("ResourceIDs = %v, want [/aws/waf/my-waf]", result.ResourceIDs)
	}
}

// TestRelated_WAF_Logs_FirehoseARNPassthrough verifies that non-CW-Logs ARNs
// (e.g. Firehose) are passed through as-is.
func TestRelated_WAF_Logs_FirehoseARNPassthrough(t *testing.T) {
	res := resource.Resource{
		ID:   "a1b2c3d4-5678-90ab-cdef-111111111111",
		Name: "my-waf",
		Fields: map[string]string{
			"arn": "arn:aws:wafv2:us-east-1:123456789012:regional/webacl/my-waf/a1b2c3d4",
		},
	}

	firehoseARN := "arn:aws:firehose:us-east-1:123456789012:deliverystream/aws-waf-logs-my-stream"
	fake := &fakeWAFv2CR{
		loggingOutput: &wafv2.GetLoggingConfigurationOutput{
			LoggingConfiguration: &wafv2types.LoggingConfiguration{
				ResourceArn:           aws.String("arn:aws:wafv2:us-east-1:123456789012:regional/webacl/my-waf/a1b2c3d4"),
				LogDestinationConfigs: []string{firehoseARN},
			},
		},
	}
	clients := &awsclient.ServiceClients{WAFv2: fake}

	checker := wafCheckerByTarget(t, "logs")
	result := checker(context.Background(), clients, res, nil)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != firehoseARN {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, firehoseARN)
	}
}

// TestRelated_WAF_Logs_NoLoggingConfigured verifies that WAFNonexistentItemException
// (no logging configured) causes the checker to return Count=0 instead of -1.
func TestRelated_WAF_Logs_NoLoggingConfigured(t *testing.T) {
	res := resource.Resource{
		ID:   "a1b2c3d4-5678-90ab-cdef-111111111111",
		Name: "my-waf",
		Fields: map[string]string{
			"arn": "arn:aws:wafv2:us-east-1:123456789012:regional/webacl/my-waf/a1b2c3d4",
		},
	}

	// Simulate WAFNonexistentItemException (logging not configured)
	notFoundErr := &wafv2types.WAFNonexistentItemException{Message: aws.String("no logging config")}
	fake := &fakeWAFv2CR{loggingErr: notFoundErr}
	clients := &awsclient.ServiceClients{WAFv2: fake}

	checker := wafCheckerByTarget(t, "logs")
	result := checker(context.Background(), clients, res, nil)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (WAFNonexistentItemException = no logging configured)", result.Count)
	}
	if result.Err != nil {
		t.Errorf("unexpected Err: %v", result.Err)
	}
}

// --- CF checker: real dispatch with fake CloudFront ---

// TestRelated_WAF_CF_CloudfrontScopeReturnsDistributionIDs verifies that
// a CLOUDFRONT-scope WebACL with a fake CloudFront client returns distribution IDs.
func TestRelated_WAF_CF_CloudfrontScopeReturnsDistributionIDs(t *testing.T) {
	res := resource.Resource{
		ID:   "a1b2c3d4-5678-90ab-cdef-222222222222",
		Name: "my-cf-waf",
		Fields: map[string]string{
			"name":  "my-cf-waf",
			"id":    "a1b2c3d4-5678-90ab-cdef-222222222222",
			"scope": "CLOUDFRONT",
		},
	}

	fakeCF := &fakeCloudFrontWAF{
		output: &cloudfront.ListDistributionsByWebACLIdOutput{
			DistributionList: &cftypes.DistributionList{
				Items: []cftypes.DistributionSummary{
					{Id: aws.String("E1ABC123DEF456")},
					{Id: aws.String("E2XYZ789GHI012")},
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{CloudFront: fakeCF}

	checker := wafCheckerByTarget(t, "cf")
	result := checker(context.Background(), clients, res, nil)

	if result.Count != 2 {
		t.Errorf("Count = %d, want 2", result.Count)
	}
	if len(result.ResourceIDs) != 2 {
		t.Errorf("ResourceIDs len = %d, want 2", len(result.ResourceIDs))
	}
	if result.ResourceIDs[0] != "E1ABC123DEF456" || result.ResourceIDs[1] != "E2XYZ789GHI012" {
		t.Errorf("ResourceIDs = %v, want [E1ABC123DEF456, E2XYZ789GHI012]", result.ResourceIDs)
	}
}

// TestRelated_WAF_CF_CloudfrontScopeEmptyDistributionList verifies that
// a CLOUDFRONT-scope WebACL with zero distributions returns Count=0.
func TestRelated_WAF_CF_CloudfrontScopeEmptyDistributionList(t *testing.T) {
	res := resource.Resource{
		ID:   "a1b2c3d4-5678-90ab-cdef-222222222222",
		Name: "my-cf-waf",
		Fields: map[string]string{
			"name":  "my-cf-waf",
			"id":    "a1b2c3d4-5678-90ab-cdef-222222222222",
			"scope": "CLOUDFRONT",
		},
	}

	fakeCF := &fakeCloudFrontWAF{
		output: &cloudfront.ListDistributionsByWebACLIdOutput{
			DistributionList: &cftypes.DistributionList{
				Items: []cftypes.DistributionSummary{},
			},
		},
	}
	clients := &awsclient.ServiceClients{CloudFront: fakeCF}

	checker := wafCheckerByTarget(t, "cf")
	result := checker(context.Background(), clients, res, nil)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty distribution list)", result.Count)
	}
}
