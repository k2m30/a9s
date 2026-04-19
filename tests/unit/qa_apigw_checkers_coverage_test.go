// qa_apigw_checkers_coverage_test.go — Behavioral coverage tests for APIGW related-resource checkers.
//
// Tests cover functions with zero coverage: checkApigwACM, checkApigwAlarm, checkApigwCF,
// checkApigwELB, checkApigwR53, checkApigwRole, checkApigwSFN, checkApigwSNS, checkApigwVPCE.
//
// Each test exercises the real checker logic (no mocking the checker itself).
// Tests in this file should PASS against current main — they cover existing, correct code.
package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	apigwv2types "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// checkApigwACM — stub: returns Count:-1 for non-empty API ID.
// ---------------------------------------------------------------------------

func TestRelated_APIGW_ACM_Unknown(t *testing.T) {
	checker := apigwCheckerByTarget(t, "acm")
	res := resource.Resource{ID: "abc123xyz", Fields: map[string]string{}}
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.TargetType != "acm" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "acm")
	}
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (ACM certs on custom domain require GetDomainNames, not in GetApis)", result.Count)
	}
}

func TestRelated_APIGW_ACM_EmptyID(t *testing.T) {
	checker := apigwCheckerByTarget(t, "acm")
	res := resource.Resource{ID: "", Fields: map[string]string{}}
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty ID means no resource)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkApigwAlarm — scans alarm cache for MetricAlarm.Dimensions[name=ApiId, value=apiID].
// ---------------------------------------------------------------------------

func TestRelated_APIGW_Alarm_Match(t *testing.T) {
	const apiID = "api-test-alarm123"

	// Build a CloudWatch MetricAlarm with dimension ApiId = apiID.
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
	// Second alarm for a different API — must NOT match.
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

	if result.TargetType != "alarm" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "alarm")
	}
	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "apigw-5xx-alarm" {
		t.Errorf("ResourceIDs = %v, want [apigw-5xx-alarm]", result.ResourceIDs)
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

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no alarm dimensions match this API)", result.Count)
	}
}

func TestRelated_APIGW_Alarm_CacheNotLoaded(t *testing.T) {
	// Empty cache + nil clients → Count:-1 (unknown).
	checker := apigwCheckerByTarget(t, "alarm")
	res := resource.Resource{ID: "api-xyz987", Fields: map[string]string{}}
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (alarm cache not loaded, no clients)", result.Count)
	}
}

func TestRelated_APIGW_Alarm_EmptyID(t *testing.T) {
	checker := apigwCheckerByTarget(t, "alarm")
	res := resource.Resource{ID: "", Fields: map[string]string{}}
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty API ID)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkApigwCF — scans CF cache for DistributionSummary.Origins with apiID.execute-api.
// ---------------------------------------------------------------------------

func TestRelated_APIGW_CF_Match(t *testing.T) {
	const apiID = "a1b2c3d4e5"

	// CloudFront distribution with an origin pointing at this API's invoke URL.
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

	// Another distribution pointing at a different API — must NOT match.
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

	if result.TargetType != "cf" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "cf")
	}
	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "E1EXAMPLE" {
		t.Errorf("ResourceIDs = %v, want [E1EXAMPLE]", result.ResourceIDs)
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

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no CF origins point at this API)", result.Count)
	}
}

func TestRelated_APIGW_CF_CacheNotLoaded(t *testing.T) {
	checker := apigwCheckerByTarget(t, "cf")
	res := resource.Resource{ID: "api-no-cache", Fields: map[string]string{}}
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (CF cache not loaded)", result.Count)
	}
}

func TestRelated_APIGW_CF_EmptyID(t *testing.T) {
	checker := apigwCheckerByTarget(t, "cf")
	res := resource.Resource{ID: "", Fields: map[string]string{}}
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty API ID)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkApigwELB — stub: returns Count:-1 for non-empty API ID.
// ---------------------------------------------------------------------------

func TestRelated_APIGW_ELB_Unknown(t *testing.T) {
	checker := apigwCheckerByTarget(t, "elb")
	res := resource.Resource{ID: "api-elb-test", Fields: map[string]string{}}
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.TargetType != "elb" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "elb")
	}
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (ELB links via VPC link require GetVpcLinks, not in budget)", result.Count)
	}
}

func TestRelated_APIGW_ELB_EmptyID(t *testing.T) {
	checker := apigwCheckerByTarget(t, "elb")
	res := resource.Resource{ID: "", Fields: map[string]string{}}
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty ID)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkApigwR53 — stub: returns Count:-1 for non-empty API ID.
// ---------------------------------------------------------------------------

func TestRelated_APIGW_R53_Unknown(t *testing.T) {
	checker := apigwCheckerByTarget(t, "r53")
	res := resource.Resource{ID: "api-r53-test", Fields: map[string]string{}}
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.TargetType != "r53" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "r53")
	}
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (R53 alias records live per-zone, not in GetApis)", result.Count)
	}
}

func TestRelated_APIGW_R53_EmptyID(t *testing.T) {
	checker := apigwCheckerByTarget(t, "r53")
	res := resource.Resource{ID: "", Fields: map[string]string{}}
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty ID)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkApigwRole — stub: returns Count:-1 for non-empty API ID.
// ---------------------------------------------------------------------------

func TestRelated_APIGW_Role_Unknown(t *testing.T) {
	checker := apigwCheckerByTarget(t, "role")
	res := resource.Resource{ID: "api-role-test", Fields: map[string]string{}}
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.TargetType != "role" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "role")
	}
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (IAM role refs per route/authorizer, not in GetApis)", result.Count)
	}
}

func TestRelated_APIGW_Role_EmptyID(t *testing.T) {
	checker := apigwCheckerByTarget(t, "role")
	res := resource.Resource{ID: "", Fields: map[string]string{}}
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty ID)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkApigwSFN — Pattern C: GetIntegrations → look for :states:action/ URIs.
// When SFN integration found, Count:-1 (state machine name requires template parsing).
// When no SFN integration, Count:0.
// ---------------------------------------------------------------------------

func TestRelated_APIGW_SFN_FoundIntegration_ReturnsUnknown(t *testing.T) {
	// An integration URI pointing at Step Functions.
	sfnURI := "arn:aws:apigateway:us-east-1:states:action/StartExecution"
	fake := &fakeAPIGWV2US1{
		integrations: []apigwv2types.Integration{
			{
				IntegrationId:  aws.String("integration-sfn-001"),
				IntegrationUri: aws.String(sfnURI),
			},
		},
	}
	clients := &awsclient.ServiceClients{
		APIGatewayV2: fake,
	}

	checker := apigwCheckerByTarget(t, "sfn")
	res := resource.Resource{ID: "api-sfn-test", Fields: map[string]string{}}
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.TargetType != "sfn" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "sfn")
	}
	// SFN integration found but state machine name requires template parsing → Count:-1.
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (SFN found but state machine name requires template parsing)", result.Count)
	}
}

func TestRelated_APIGW_SFN_NoSFNIntegration_ReturnsZero(t *testing.T) {
	// Integration URI pointing at Lambda, not SFN.
	lambdaURI := "arn:aws:apigateway:us-east-1:lambda:path/2015-03-31/functions/arn:aws:lambda:us-east-1:123456789012:function:my-fn/invocations"
	fake := &fakeAPIGWV2US1{
		integrations: []apigwv2types.Integration{
			{
				IntegrationId:  aws.String("integration-lambda-001"),
				IntegrationUri: aws.String(lambdaURI),
			},
		},
	}
	clients := &awsclient.ServiceClients{
		APIGatewayV2: fake,
	}

	checker := apigwCheckerByTarget(t, "sfn")
	res := resource.Resource{ID: "api-sfn-none", Fields: map[string]string{}}
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no SFN integrations found)", result.Count)
	}
}

func TestRelated_APIGW_SFN_NilClients_ReturnsUnknown(t *testing.T) {
	checker := apigwCheckerByTarget(t, "sfn")
	res := resource.Resource{ID: "api-sfn-nil", Fields: map[string]string{}}
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil clients → can't call GetIntegrations)", result.Count)
	}
}

func TestRelated_APIGW_SFN_EmptyID(t *testing.T) {
	checker := apigwCheckerByTarget(t, "sfn")
	res := resource.Resource{ID: "", Fields: map[string]string{}}
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty API ID)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkApigwSNS — Pattern C: GetIntegrations → look for :sns:action/ URIs.
// When SNS integration found, Count:-1 (topic ARN requires template parsing).
// When no SNS integration, Count:0.
// ---------------------------------------------------------------------------

func TestRelated_APIGW_SNS_FoundIntegration_ReturnsUnknown(t *testing.T) {
	snsURI := "arn:aws:apigateway:us-east-1:sns:action/Publish"
	fake := &fakeAPIGWV2US1{
		integrations: []apigwv2types.Integration{
			{
				IntegrationId:  aws.String("integration-sns-001"),
				IntegrationUri: aws.String(snsURI),
			},
		},
	}
	clients := &awsclient.ServiceClients{
		APIGatewayV2: fake,
	}

	checker := apigwCheckerByTarget(t, "sns")
	res := resource.Resource{ID: "api-sns-test", Fields: map[string]string{}}
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.TargetType != "sns" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "sns")
	}
	// Topic ARN lives in the route request template → Count:-1.
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (SNS topic ARN requires template parsing)", result.Count)
	}
}

func TestRelated_APIGW_SNS_NoSNSIntegration_ReturnsZero(t *testing.T) {
	// Integration pointing at DynamoDB, not SNS.
	ddbURI := "arn:aws:apigateway:us-east-1:dynamodb:action/PutItem"
	fake := &fakeAPIGWV2US1{
		integrations: []apigwv2types.Integration{
			{
				IntegrationId:  aws.String("integration-ddb-001"),
				IntegrationUri: aws.String(ddbURI),
			},
		},
	}
	clients := &awsclient.ServiceClients{
		APIGatewayV2: fake,
	}

	checker := apigwCheckerByTarget(t, "sns")
	res := resource.Resource{ID: "api-no-sns", Fields: map[string]string{}}
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no SNS integrations)", result.Count)
	}
}

func TestRelated_APIGW_SNS_NilClients_ReturnsUnknown(t *testing.T) {
	checker := apigwCheckerByTarget(t, "sns")
	res := resource.Resource{ID: "api-sns-nil", Fields: map[string]string{}}
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil clients → can't call GetIntegrations)", result.Count)
	}
}

func TestRelated_APIGW_SNS_EmptyID(t *testing.T) {
	checker := apigwCheckerByTarget(t, "sns")
	res := resource.Resource{ID: "", Fields: map[string]string{}}
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty API ID)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkApigwVPCE — stub: returns Count:-1 for non-empty API ID.
// ---------------------------------------------------------------------------

func TestRelated_APIGW_VPCE_Unknown(t *testing.T) {
	checker := apigwCheckerByTarget(t, "vpce")
	res := resource.Resource{ID: "api-vpce-test", Fields: map[string]string{}}
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.TargetType != "vpce" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "vpce")
	}
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (endpoint_configuration is v1-only, not available via v2 GetApis)", result.Count)
	}
}

func TestRelated_APIGW_VPCE_EmptyID(t *testing.T) {
	checker := apigwCheckerByTarget(t, "vpce")
	res := resource.Resource{ID: "", Fields: map[string]string{}}
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty ID)", result.Count)
	}
}
