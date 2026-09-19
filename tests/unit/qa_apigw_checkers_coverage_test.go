// Behavioural tests for the APIGW
// related-resource checkers checkApigwACM, checkApigwAlarm, checkApigwCF,
// checkApigwELB and checkApigwRole, run against the real checker logic.
package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// ---------------------------------------------------------------------------
// checkApigwACM — resolves ACM certs via GetDomainNames + GetApiMappings.
// With a nil client we exercise the client-missing guard → State: RelatedUnknown.
// ---------------------------------------------------------------------------

func TestRelated_APIGW_ACM_Unknown(t *testing.T) {
	checker := apigwCheckerByTarget(t, "acm")
	res := resource.Resource{ID: "abc123xyz", Fields: map[string]string{}}
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.TargetType() != "acm" {
		t.Errorf("TargetType = %q, want %q", result.TargetType(), "acm")
	}
	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (nil client hits the client-missing guard before any AWS call)", result.Count())
	}
}

func TestRelated_APIGW_ACM_EmptyID(t *testing.T) {
	checker := apigwCheckerByTarget(t, "acm")
	res := resource.Resource{ID: "", Fields: map[string]string{}}
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (empty ID means no resource)", result.Count())
	}
}

// ---------------------------------------------------------------------------
// checkApigwAlarm — scans alarm cache for MetricAlarm.Dimensions[name=ApiId, value=apiID].
// ---------------------------------------------------------------------------

func TestRelated_APIGW_Alarm_Match(t *testing.T) {
	const apiID = "api-test-alarm123"

	alarm := cwtypes.MetricAlarm{
		AlarmName: aws.String("apigw-5xx-alarm"),
		Dimensions: []cwtypes.Dimension{
			{Name: aws.String("ApiId"), Value: aws.String(apiID)},
			{Name: aws.String("Stage"), Value: aws.String("$default")},
		},
	}
	alarmRes := resource.Resource{
		ID:        "apigw-5xx-alarm",
		Fields:    map[string]string{},
		RawStruct: alarm,
	}
	otherAlarm := cwtypes.MetricAlarm{
		AlarmName: aws.String("other-api-alarm"),
		Dimensions: []cwtypes.Dimension{
			{Name: aws.String("ApiId"), Value: aws.String("different-api-id")},
		},
	}
	otherAlarmRes := resource.Resource{
		ID:        "other-api-alarm",
		Fields:    map[string]string{},
		RawStruct: otherAlarm,
	}

	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{
			Resources: []resource.Resource{alarmRes, otherAlarmRes},
		},
	}

	checker := apigwCheckerByTarget(t, "alarm")
	res := resource.Resource{ID: apiID, Fields: map[string]string{}}
	result := checker(context.Background(), nil, res, cache)

	if result.TargetType() != "alarm" {
		t.Errorf("TargetType = %q, want %q", result.TargetType(), "alarm")
	}
	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "apigw-5xx-alarm" {
		t.Errorf("ResourceIDs = %v, want [apigw-5xx-alarm]", result.ResourceIDs())
	}
}

func TestRelated_APIGW_Alarm_NoMatch(t *testing.T) {
	alarm := cwtypes.MetricAlarm{
		AlarmName: aws.String("lambda-error-alarm"),
		Dimensions: []cwtypes.Dimension{
			{Name: aws.String("FunctionName"), Value: aws.String("my-lambda")},
		},
	}
	alarmRes := resource.Resource{
		ID:        "lambda-error-alarm",
		Fields:    map[string]string{},
		RawStruct: alarm,
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}

	checker := apigwCheckerByTarget(t, "alarm")
	res := resource.Resource{ID: "api-xyz987", Fields: map[string]string{}}
	result := checker(context.Background(), nil, res, cache)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no alarm dimensions match this API)", result.Count())
	}
}

func TestRelated_APIGW_Alarm_CacheNotLoaded(t *testing.T) {
	checker := apigwCheckerByTarget(t, "alarm")
	res := resource.Resource{ID: "api-xyz987", Fields: map[string]string{}}
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (alarm cache not loaded, no clients)", result.Count())
	}
}

func TestRelated_APIGW_Alarm_EmptyID(t *testing.T) {
	checker := apigwCheckerByTarget(t, "alarm")
	res := resource.Resource{ID: "", Fields: map[string]string{}}
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (empty API ID)", result.Count())
	}
}

// ---------------------------------------------------------------------------
// checkApigwCF — scans CF cache for DistributionSummary.Origins with apiID.execute-api.
// ---------------------------------------------------------------------------

func TestRelated_APIGW_CF_Match(t *testing.T) {
	const apiID = "a1b2c3d4e5"

	dist := cftypes.DistributionSummary{
		ARN: aws.String("arn:aws:cloudfront::123456789012:distribution/E1EXAMPLE"),
		Origins: &cftypes.Origins{
			Quantity: aws.Int32(1),
			Items: []cftypes.Origin{
				{
					Id:         aws.String("apigw-origin"),
					DomainName: aws.String(apiID + ".execute-api.us-east-1.amazonaws.com"),
				},
			},
		},
	}
	cfRes := resource.Resource{
		ID:        "E1EXAMPLE",
		Fields:    map[string]string{},
		RawStruct: dist,
	}

	otherDist := cftypes.DistributionSummary{
		ARN: aws.String("arn:aws:cloudfront::123456789012:distribution/E2OTHER"),
		Origins: &cftypes.Origins{
			Quantity: aws.Int32(1),
			Items: []cftypes.Origin{
				{
					Id:         aws.String("other-origin"),
					DomainName: aws.String("z9y8x7w6v5.execute-api.us-east-1.amazonaws.com"),
				},
			},
		},
	}
	otherCFRes := resource.Resource{
		ID:        "E2OTHER",
		Fields:    map[string]string{},
		RawStruct: otherDist,
	}

	cache := resource.ResourceCache{
		"cf": resource.ResourceCacheEntry{
			Resources: []resource.Resource{cfRes, otherCFRes},
		},
	}

	checker := apigwCheckerByTarget(t, "cf")
	res := resource.Resource{ID: apiID, Fields: map[string]string{}}
	result := checker(context.Background(), nil, res, cache)

	if result.TargetType() != "cf" {
		t.Errorf("TargetType = %q, want %q", result.TargetType(), "cf")
	}
	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "E1EXAMPLE" {
		t.Errorf("ResourceIDs = %v, want [E1EXAMPLE]", result.ResourceIDs())
	}
}

func TestRelated_APIGW_CF_NoMatch(t *testing.T) {
	dist := cftypes.DistributionSummary{
		ARN: aws.String("arn:aws:cloudfront::123456789012:distribution/E3NOAPIGW"),
		Origins: &cftypes.Origins{
			Quantity: aws.Int32(1),
			Items: []cftypes.Origin{
				{
					Id:         aws.String("s3-origin"),
					DomainName: aws.String("my-bucket.s3.amazonaws.com"),
				},
			},
		},
	}
	cfRes := resource.Resource{
		ID:        "E3NOAPIGW",
		Fields:    map[string]string{},
		RawStruct: dist,
	}
	cache := resource.ResourceCache{
		"cf": resource.ResourceCacheEntry{Resources: []resource.Resource{cfRes}},
	}

	checker := apigwCheckerByTarget(t, "cf")
	res := resource.Resource{ID: "api-no-cf", Fields: map[string]string{}}
	result := checker(context.Background(), nil, res, cache)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no CF origins point at this API)", result.Count())
	}
}

func TestRelated_APIGW_CF_CacheNotLoaded(t *testing.T) {
	checker := apigwCheckerByTarget(t, "cf")
	res := resource.Resource{ID: "api-no-cache", Fields: map[string]string{}}
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (CF cache not loaded)", result.Count())
	}
}

func TestRelated_APIGW_CF_EmptyID(t *testing.T) {
	checker := apigwCheckerByTarget(t, "cf")
	res := resource.Resource{ID: "", Fields: map[string]string{}}
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (empty API ID)", result.Count())
	}
}

// ---------------------------------------------------------------------------
// checkApigwELB — stub: returns State: RelatedUnknown for non-empty API ID.
// ---------------------------------------------------------------------------

func TestRelated_APIGW_ELB_Unknown(t *testing.T) {
	checker := apigwCheckerByTarget(t, "elb")
	res := resource.Resource{ID: "api-elb-test", Fields: map[string]string{}}
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.TargetType() != "elb" {
		t.Errorf("TargetType = %q, want %q", result.TargetType(), "elb")
	}
	if result.State() != domain.RelatedUnknown {
		t.Errorf("State = %v, want RelatedUnknown (ELB links via VPC link require GetVpcLinks, not in budget)", result.State())
	}
}

func TestRelated_APIGW_ELB_EmptyID(t *testing.T) {
	checker := apigwCheckerByTarget(t, "elb")
	res := resource.Resource{ID: "", Fields: map[string]string{}}
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (empty ID)", result.Count())
	}
}

// ---------------------------------------------------------------------------
// checkApigwRole — stub: returns State: RelatedUnknown for non-empty API ID.
// ---------------------------------------------------------------------------

func TestRelated_APIGW_Role_Unknown(t *testing.T) {
	checker := apigwCheckerByTarget(t, "role")
	res := resource.Resource{ID: "api-role-test", Fields: map[string]string{}}
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.TargetType() != "role" {
		t.Errorf("TargetType = %q, want %q", result.TargetType(), "role")
	}
	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (IAM role refs per route/authorizer, not in GetApis)", result.Count())
	}
}

func TestRelated_APIGW_Role_EmptyID(t *testing.T) {
	checker := apigwCheckerByTarget(t, "role")
	res := resource.Resource{ID: "", Fields: map[string]string{}}
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (empty ID)", result.Count())
	}
}
