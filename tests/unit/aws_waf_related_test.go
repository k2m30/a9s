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

	_ "github.com/k2m30/a9s/v3/core/aws"
	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

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

func TestRelated_WAF_ELB_NilClients(t *testing.T) {
	res := wafSrcResource()
	checker := wafCheckerByTarget(t, "elb")
	result := checker(context.Background(), nil, res, nil)
	if result.State() != domain.RelatedUnknown {
		t.Errorf("State = %v, want RelatedUnknown (nil clients)", result.State())
	}
}

func TestRelated_WAF_APIGW_NilClients(t *testing.T) {
	res := wafSrcResource()
	checker := wafCheckerByTarget(t, "apigw")
	result := checker(context.Background(), nil, res, nil)
	if result.State() != domain.RelatedUnknown {
		t.Errorf("State = %v, want RelatedUnknown (nil clients)", result.State())
	}
}

// A REGIONAL web ACL cannot be associated with a CloudFront distribution.
func TestRelated_WAF_CF_RegionalReturnsZero(t *testing.T) {
	res := wafSrcResource()
	checker := wafCheckerByTarget(t, "cf")
	result := checker(context.Background(), nil, res, nil)
	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (REGIONAL scope cannot bind CloudFront)", result.Count())
	}
	if result.TargetType() != "cf" {
		t.Errorf("TargetType = %q, want %q", result.TargetType(), "cf")
	}
}

// checkWAFCF reads the web ACL ARN from Fields["arn"].
func TestRelated_WAF_CF_CloudfrontScopeUnknown(t *testing.T) {
	res := resource.Resource{
		ID:   "a1b2c3d4-5678-90ab-cdef-222222222222",
		Name: "my-cf-waf",
		Fields: map[string]string{
			"name":  "my-cf-waf",
			"id":    "a1b2c3d4-5678-90ab-cdef-222222222222",
			"arn":   "arn:aws:wafv2:us-east-1:123456789012:global/webacl/my-cf-waf/a1b2c3d4-5678-90ab-cdef-222222222222",
			"scope": "CLOUDFRONT",
		},
	}
	checker := wafCheckerByTarget(t, "cf")
	result := checker(context.Background(), nil, res, nil)
	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (CLOUDFRONT scope: requires ListResourcesForWebACL)", result.Count())
	}
}

func TestRelated_WAF_Alarm_MatchByWebACLDimension(t *testing.T) {
	res := wafSrcResource()

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

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "waf-blocked-requests-alarm" {
		t.Errorf("ResourceIDs = %v, want [waf-blocked-requests-alarm]", result.ResourceIDs())
	}
	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
	}
}

func TestRelated_WAF_Alarm_NoMatch(t *testing.T) {
	res := wafSrcResource()

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

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0", result.Count())
	}
}

func TestRelated_WAF_Alarm_CacheMissNoClients(t *testing.T) {
	res := wafSrcResource()

	checker := wafCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (cache miss, no clients)", result.Count())
	}
}

func TestRelated_WAF_Alarm_EmptyName(t *testing.T) {
	res := resource.Resource{
		ID:     "a1b2c3d4-1234-5678-abcd-111111111111",
		Name:   "",
		Fields: map[string]string{"name": "", "scope": "REGIONAL"},
	}

	checker := wafCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (empty name)", result.Count())
	}
}

func TestRelated_WAF_Logs_EmptyARN(t *testing.T) {
	res := resource.Resource{
		ID:     "a1b2c3d4-1234-5678-abcd-111111111111",
		Name:   "my-waf",
		Fields: map[string]string{"name": "my-waf", "scope": "REGIONAL"},
	}

	checker := wafCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (empty ARN)", result.Count())
	}
}

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

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (nil clients)", result.Count())
	}
}

func TestRelated_WAF_ELB_ExtractsNameFromARN(t *testing.T) {
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

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "my-alb" {
		t.Errorf("ResourceIDs = %v, want [my-alb]", result.ResourceIDs())
	}
}

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

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no ARNs returned)", result.Count())
	}
}

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

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (short ARN skipped)", result.Count())
	}
}

func TestRelated_WAF_APIGW_ExtractsAPIIDFromARN(t *testing.T) {
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

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "abc123def" {
		t.Errorf("ResourceIDs = %v, want [abc123def]", result.ResourceIDs())
	}
}

// The apigw resolver reads both API ARN shapes: "/restapis/<id>" and
// "/apis/<id>".
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

	if ids := result.ResourceIDs(); len(ids) != 1 || ids[0] != "xyz789" {
		t.Errorf("ResourceIDs = %v, want [xyz789]", ids)
	}
}

func TestRelated_WAF_Logs_CWLogGroupNameExtracted(t *testing.T) {
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

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "/aws/waf/my-waf" {
		t.Errorf("ResourceIDs = %v, want [/aws/waf/my-waf]", result.ResourceIDs())
	}
}

// TestRelated_WAF_Logs_FirehoseARNPassthrough: a Firehose destination is no
// log group and is not counted: a stream ARN drills into no log group, so it
// is never the ID.
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

	if result.Count() != 0 {
		t.Errorf("Count = %d (IDs %v), want 0: %s is no log group", result.Count(), result.ResourceIDs(), firehoseARN)
	}
}

// GetLoggingConfiguration answers WAFNonexistentItemException when the web
// ACL has no logging configured.
func TestRelated_WAF_Logs_NoLoggingConfigured(t *testing.T) {
	res := resource.Resource{
		ID:   "a1b2c3d4-5678-90ab-cdef-111111111111",
		Name: "my-waf",
		Fields: map[string]string{
			"arn": "arn:aws:wafv2:us-east-1:123456789012:regional/webacl/my-waf/a1b2c3d4",
		},
	}

	notFoundErr := &wafv2types.WAFNonexistentItemException{Message: aws.String("no logging config")}
	fake := &fakeWAFv2CR{loggingErr: notFoundErr}
	clients := &awsclient.ServiceClients{WAFv2: fake}

	checker := wafCheckerByTarget(t, "logs")
	result := checker(context.Background(), clients, res, nil)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (WAFNonexistentItemException = no logging configured)", result.Count())
	}
	if result.Err() != nil {
		t.Errorf("unexpected Err: %v", result.Err())
	}
}

// checkWAFCF passes the full web ACL ARN to
// cloudfront:ListDistributionsByWebACLId.
func TestRelated_WAF_CF_CloudfrontScopeReturnsDistributionIDs(t *testing.T) {
	res := resource.Resource{
		ID:   "a1b2c3d4-5678-90ab-cdef-222222222222",
		Name: "my-cf-waf",
		Fields: map[string]string{
			"name":  "my-cf-waf",
			"id":    "a1b2c3d4-5678-90ab-cdef-222222222222",
			"arn":   "arn:aws:wafv2:us-east-1:123456789012:global/webacl/my-cf-waf/a1b2c3d4-5678-90ab-cdef-222222222222",
			"scope": "CLOUDFRONT",
		},
	}

	fakeCF := &fakeCloudFrontAPI{
		WebACLOutput: &cloudfront.ListDistributionsByWebACLIdOutput{
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

	if result.Count() != 2 {
		t.Errorf("Count = %d, want 2", result.Count())
	}
	if len(result.ResourceIDs()) != 2 {
		t.Fatalf("ResourceIDs len = %d, want 2 (got %v)", len(result.ResourceIDs()), result.ResourceIDs())
	}
	if result.ResourceIDs()[0] != "E1ABC123DEF456" || result.ResourceIDs()[1] != "E2XYZ789GHI012" {
		t.Errorf("ResourceIDs = %v, want [E1ABC123DEF456, E2XYZ789GHI012]", result.ResourceIDs())
	}
}

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

	fakeCF := &fakeCloudFrontAPI{
		WebACLOutput: &cloudfront.ListDistributionsByWebACLIdOutput{
			DistributionList: &cftypes.DistributionList{
				Items: []cftypes.DistributionSummary{},
			},
		},
	}
	clients := &awsclient.ServiceClients{CloudFront: fakeCF}

	checker := wafCheckerByTarget(t, "cf")
	result := checker(context.Background(), clients, res, nil)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (empty distribution list)", result.Count())
	}
}

// An empty DistributionList is CloudFront saying this web ACL protects no
// distribution, and a zero is the truth. A nil one is a successful call that
// carried no list at all, which is not a count of any kind.
func TestRelated_WAF_CF_NilDistributionListIsUnknown(t *testing.T) {
	res := resource.Resource{
		ID:   "a1b2c3d4-5678-90ab-cdef-222222222222",
		Name: "my-cf-waf",
		Fields: map[string]string{
			"name":  "my-cf-waf",
			"id":    "a1b2c3d4-5678-90ab-cdef-222222222222",
			"scope": "CLOUDFRONT",
		},
	}
	clients := &awsclient.ServiceClients{CloudFront: &fakeCloudFrontAPI{
		WebACLOutput: &cloudfront.ListDistributionsByWebACLIdOutput{},
	}}

	result := wafCheckerByTarget(t, "cf")(context.Background(), clients, res, nil)

	if result.State() != domain.RelatedUnknown {
		t.Errorf("nil DistributionList: state = %v (Count=%d), want Unknown — the response carried no list, which is not a zero",
			result.State(), result.Count())
	}
}
