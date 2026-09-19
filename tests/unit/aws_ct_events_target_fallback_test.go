package unit

// _ct.target falls back to LookupEvents event.Resources when the embedded
// CloudTrailEvent JSON has no resources[] (or is nil); JSON resources[] wins
// when present.

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
)

// buildCTEventForTargetFallback constructs a cloudtrailtypes.Event that has:
//   - a nil CloudTrailEvent (no JSON blob) OR a JSON with empty resources[]
//   - event.Resources populated with a known value via LookupEvents response
func buildCTEventForTargetFallback(
	id string,
	cloudTrailEventJSON *string,
	sdkResources []cloudtrailtypes.Resource,
) cloudtrailtypes.Event {
	return cloudtrailtypes.Event{
		EventId:         aws.String(id),
		EventName:       aws.String("GetObject"),
		EventTime:       aws.Time(time.Date(2026, 3, 28, 14, 0, 0, 0, time.UTC)),
		EventSource:     aws.String("s3.amazonaws.com"),
		Username:        aws.String("alice"),
		ReadOnly:        aws.String("true"),
		CloudTrailEvent: cloudTrailEventJSON,
		Resources:       sdkResources,
	}
}

func TestCTTargetFallback_NilJSON_UsesSDKResources(t *testing.T) {
	sdkResources := []cloudtrailtypes.Resource{
		{
			ResourceName: aws.String("arn:aws:s3:::demo-bucket"),
			ResourceType: aws.String("AWS::S3::Bucket"),
		},
	}
	event := buildCTEventForTargetFallback("tf-01", nil, sdkResources)

	result, err := awsclient.FetchCloudTrailEventsPage(context.Background(), &singleEventCTMock{event: event}, "")
	if err != nil {
		t.Fatalf("FetchCloudTrailEventsPage error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	target := result.Resources[0].Fields["_ct.target"]
	if target == "(none)" {
		t.Errorf("_ct.target = %q; expected non-(none) value from event.Resources (arn:aws:s3:::demo-bucket); "+
			"bug at ct_events.go:224 — ExtractCTTarget(parsed) never falls back to event.Resources", target)
	}
	if target == "" {
		t.Errorf("_ct.target is empty; expected arn:aws:s3:::demo-bucket from LookupEvents event.Resources")
	}
}

func TestCTTargetFallback_EmptyJSONResources_UsesSDKResources(t *testing.T) {
	// JSON has "resources": [] — an explicit empty array, not absent.
	emptyResourcesJSON := `{"eventVersion":"1.08","userIdentity":{"type":"AssumedRole","accountId":"111122223333"}` +
		`,"eventTime":"2026-03-28T14:00:00Z","eventSource":"s3.amazonaws.com","eventName":"GetObject"` +
		`,"awsRegion":"us-east-1","sourceIPAddress":"1.2.3.4","userAgent":"aws-cli/2.0"` +
		`,"errorCode":"","eventCategory":"Management","eventType":"AwsApiCall"` +
		`,"recipientAccountId":"111122223333","resources":[]}`

	sdkResources := []cloudtrailtypes.Resource{
		{
			ResourceName: aws.String("arn:aws:s3:::prod-bucket/key.txt"),
			ResourceType: aws.String("AWS::S3::Object"),
		},
	}
	event := buildCTEventForTargetFallback("tf-02", aws.String(emptyResourcesJSON), sdkResources)

	result, err := awsclient.FetchCloudTrailEventsPage(context.Background(), &singleEventCTMock{event: event}, "")
	if err != nil {
		t.Fatalf("FetchCloudTrailEventsPage error: %v", err)
	}
	target := result.Resources[0].Fields["_ct.target"]
	if target == "(none)" {
		t.Errorf("_ct.target = %q; expected value from event.Resources when JSON resources[] is empty; "+
			"bug at ct_events.go:224", target)
	}
}

// JSON resources[] wins over event.Resources.

func TestCTTargetFallback_JSONResourcesWin_RegressionGuard(t *testing.T) {
	// _ct.target stores the ARN-stripped value: FormatCTTarget runs at fetch time.
	jsonWithResource := `{"eventVersion":"1.08","userIdentity":{"type":"AssumedRole","accountId":"111122223333"}` +
		`,"eventTime":"2026-03-28T14:00:00Z","eventSource":"s3.amazonaws.com","eventName":"GetObject"` +
		`,"awsRegion":"us-east-1","sourceIPAddress":"1.2.3.4","userAgent":"aws-cli/2.0"` +
		`,"errorCode":"","eventCategory":"Management","eventType":"AwsApiCall"` +
		`,"recipientAccountId":"111122223333"` +
		`,"resources":[{"ARN":"arn:aws:s3:::json-wins-bucket","accountId":"111122223333","type":"AWS::S3::Bucket"}]}`

	// event.Resources has a different value — SDK value should NOT win.
	sdkResources := []cloudtrailtypes.Resource{
		{
			ResourceName: aws.String("arn:aws:s3:::sdk-resource-bucket"),
			ResourceType: aws.String("AWS::S3::Bucket"),
		},
	}
	event := buildCTEventForTargetFallback("tf-03", aws.String(jsonWithResource), sdkResources)

	result, err := awsclient.FetchCloudTrailEventsPage(context.Background(), &singleEventCTMock{event: event}, "")
	if err != nil {
		t.Fatalf("FetchCloudTrailEventsPage error: %v", err)
	}
	target := result.Resources[0].Fields["_ct.target"]
	if target != "json-wins-bucket" {
		t.Errorf("_ct.target = %q; expected \"json-wins-bucket\" (ARN stripped per §5 from JSON resources[]); "+
			"JSON resources[] must win over event.Resources (would yield \"sdk-resource-bucket\")", target)
	}
	if target == "sdk-resource-bucket" || target == "arn:aws:s3:::sdk-resource-bucket" {
		t.Errorf("_ct.target = %q; SDK event.Resources must NOT win when JSON resources[] is populated", target)
	}
}

func TestCTTargetFallback_BothEmpty_IsNone_RegressionGuard(t *testing.T) {
	emptyJSON := `{"eventVersion":"1.08","userIdentity":{"type":"AssumedRole","accountId":"111122223333"}` +
		`,"eventTime":"2026-03-28T14:00:00Z","eventSource":"ec2.amazonaws.com","eventName":"DescribeInstances"` +
		`,"awsRegion":"us-east-1","sourceIPAddress":"1.2.3.4","userAgent":"aws-cli/2.0"` +
		`,"errorCode":"","eventCategory":"Management","eventType":"AwsApiCall"` +
		`,"recipientAccountId":"111122223333"}`

	event := buildCTEventForTargetFallback("tf-04", aws.String(emptyJSON), nil)

	result, err := awsclient.FetchCloudTrailEventsPage(context.Background(), &singleEventCTMock{event: event}, "")
	if err != nil {
		t.Fatalf("FetchCloudTrailEventsPage error: %v", err)
	}
	target := result.Resources[0].Fields["_ct.target"]
	// When nothing is available, ExtractCTTarget falls through to a non-resources path
	// (e.g. Management event with no resources → may return "(none)" or a request-id based value).
	// The important invariant is that it is NOT empty string.
	if target == "" {
		t.Errorf("_ct.target is empty string; expected a non-empty fallback (at minimum '(none)')")
	}
}

// Per-event-name fallback table: _ct.target resolves from requestParameters
// (ExtractCTTarget / buildCTResource).

// buildCTEventWithRequestParams constructs a cloudtrailtypes.Event whose
// CloudTrailEvent JSON contains the given requestParameters JSON object.
func buildCTEventWithRequestParams(id, eventName, eventSource, requestParamsJSON string) cloudtrailtypes.Event {
	ctJSON := `{"eventVersion":"1.08","userIdentity":{"type":"AssumedRole","accountId":"123456789012"` +
		`,"sessionContext":{"sessionIssuer":{"userName":"test-role","type":"Role"}}}` +
		`,"eventTime":"2026-04-07T17:00:00Z","eventSource":"` + eventSource +
		`","eventName":"` + eventName +
		`","awsRegion":"us-east-1","sourceIPAddress":"1.2.3.4","userAgent":"aws-cli/2.0"` +
		`,"errorCode":"","eventCategory":"Management","eventType":"AwsApiCall"` +
		`,"recipientAccountId":"123456789012"` +
		`,"requestParameters":` + requestParamsJSON + `}`
	return cloudtrailtypes.Event{
		EventId:         aws.String(id),
		EventName:       aws.String(eventName),
		EventTime:       aws.Time(time.Date(2026, 4, 7, 17, 0, 0, 0, time.UTC)),
		EventSource:     aws.String(eventSource),
		Username:        aws.String("testuser"),
		ReadOnly:        aws.String("false"),
		CloudTrailEvent: aws.String(ctJSON),
		Resources:       nil,
	}
}

// TestCTTargetFallback_DescribeInstances_WithItems — DescribeInstances with
// instancesSet.items populated → "i-abc,i-def".
func TestCTTargetFallback_DescribeInstances_WithItems(t *testing.T) {
	event := buildCTEventWithRequestParams(
		"tf-di-01", "DescribeInstances", "ec2.amazonaws.com",
		`{"instancesSet":{"items":[{"instanceId":"i-abc"},{"instanceId":"i-def"}]}}`,
	)
	result, err := awsclient.FetchCloudTrailEventsPage(context.Background(), &singleEventCTMock{event: event}, "")
	if err != nil {
		t.Fatalf("FetchCloudTrailEventsPage error: %v", err)
	}
	target := result.Resources[0].Fields["_ct.target"]
	if target != "i-abc,i-def" {
		t.Errorf("_ct.target = %q, want %q per §4 DescribeInstances with items", target, "i-abc,i-def")
	}
}

// TestCTTargetFallback_DescribeInstances_EmptyItems — empty instancesSet.items → "(all)".
func TestCTTargetFallback_DescribeInstances_EmptyItems(t *testing.T) {
	event := buildCTEventWithRequestParams(
		"tf-di-02", "DescribeInstances", "ec2.amazonaws.com",
		`{"instancesSet":{"items":[]}}`,
	)
	result, err := awsclient.FetchCloudTrailEventsPage(context.Background(), &singleEventCTMock{event: event}, "")
	if err != nil {
		t.Fatalf("FetchCloudTrailEventsPage error: %v", err)
	}
	target := result.Resources[0].Fields["_ct.target"]
	if target != "(all)" {
		t.Errorf("_ct.target = %q, want %q per §4 DescribeInstances empty items", target, "(all)")
	}
}

// TestCTTargetFallback_UpdateInstanceInformation — instanceId field.
func TestCTTargetFallback_UpdateInstanceInformation(t *testing.T) {
	event := buildCTEventWithRequestParams(
		"tf-uii-01", "UpdateInstanceInformation", "ssm.amazonaws.com",
		`{"instanceId":"i-123"}`,
	)
	result, err := awsclient.FetchCloudTrailEventsPage(context.Background(), &singleEventCTMock{event: event}, "")
	if err != nil {
		t.Fatalf("FetchCloudTrailEventsPage error: %v", err)
	}
	target := result.Resources[0].Fields["_ct.target"]
	if target != "i-123" {
		t.Errorf("_ct.target = %q, want %q per §4 UpdateInstanceInformation", target, "i-123")
	}
}

// TestCTTargetFallback_GetParameter — single SSM parameter name.
func TestCTTargetFallback_GetParameter(t *testing.T) {
	event := buildCTEventWithRequestParams(
		"tf-gp-01", "GetParameter", "ssm.amazonaws.com",
		`{"name":"/foo/bar"}`,
	)
	result, err := awsclient.FetchCloudTrailEventsPage(context.Background(), &singleEventCTMock{event: event}, "")
	if err != nil {
		t.Fatalf("FetchCloudTrailEventsPage error: %v", err)
	}
	target := result.Resources[0].Fields["_ct.target"]
	if target != "/foo/bar" {
		t.Errorf("_ct.target = %q, want %q per §4 GetParameter", target, "/foo/bar")
	}
}

// TestCTTargetFallback_GetParameters — multiple SSM parameter names joined.
func TestCTTargetFallback_GetParameters(t *testing.T) {
	event := buildCTEventWithRequestParams(
		"tf-gps-01", "GetParameters", "ssm.amazonaws.com",
		`{"names":["/a","/b"]}`,
	)
	result, err := awsclient.FetchCloudTrailEventsPage(context.Background(), &singleEventCTMock{event: event}, "")
	if err != nil {
		t.Fatalf("FetchCloudTrailEventsPage error: %v", err)
	}
	target := result.Resources[0].Fields["_ct.target"]
	if target != "/a,/b" {
		t.Errorf("_ct.target = %q, want %q per §4 GetParameters", target, "/a,/b")
	}
}

// TestCTTargetFallback_GetSecretValue — secretId field.
func TestCTTargetFallback_GetSecretValue(t *testing.T) {
	event := buildCTEventWithRequestParams(
		"tf-gsv-01", "GetSecretValue", "secretsmanager.amazonaws.com",
		`{"secretId":"prod/db"}`,
	)
	result, err := awsclient.FetchCloudTrailEventsPage(context.Background(), &singleEventCTMock{event: event}, "")
	if err != nil {
		t.Fatalf("FetchCloudTrailEventsPage error: %v", err)
	}
	target := result.Resources[0].Fields["_ct.target"]
	if target != "prod/db" {
		t.Errorf("_ct.target = %q, want %q per §4 GetSecretValue", target, "prod/db")
	}
}

// TestCTTargetFallback_Decrypt_WithKeyID — keyId present → use it.
func TestCTTargetFallback_Decrypt_WithKeyID(t *testing.T) {
	event := buildCTEventWithRequestParams(
		"tf-dec-01", "Decrypt", "kms.amazonaws.com",
		`{"keyId":"alias/foo"}`,
	)
	result, err := awsclient.FetchCloudTrailEventsPage(context.Background(), &singleEventCTMock{event: event}, "")
	if err != nil {
		t.Fatalf("FetchCloudTrailEventsPage error: %v", err)
	}
	target := result.Resources[0].Fields["_ct.target"]
	if target != "alias/foo" {
		t.Errorf("_ct.target = %q, want %q per §4 Decrypt with keyId", target, "alias/foo")
	}
}

// TestCTTargetFallback_Decrypt_NoKeyID — no keyId → "(by alias)".
func TestCTTargetFallback_Decrypt_NoKeyID(t *testing.T) {
	event := buildCTEventWithRequestParams(
		"tf-dec-02", "Decrypt", "kms.amazonaws.com",
		`{"encryptionContext":{"aws:s3:arn":"arn:aws:s3:::mybucket"}}`,
	)
	result, err := awsclient.FetchCloudTrailEventsPage(context.Background(), &singleEventCTMock{event: event}, "")
	if err != nil {
		t.Fatalf("FetchCloudTrailEventsPage error: %v", err)
	}
	target := result.Resources[0].Fields["_ct.target"]
	if target != "(by alias)" {
		t.Errorf("_ct.target = %q, want %q per §4 Decrypt without keyId", target, "(by alias)")
	}
}

// TestCTTargetFallback_AssumeRole — roleArn, ARN-stripped.
func TestCTTargetFallback_AssumeRole(t *testing.T) {
	// arn:aws:iam::123456789012:role/Admin → "role/Admin" (same-account strip)
	event := buildCTEventWithRequestParams(
		"tf-ar-01", "AssumeRole", "sts.amazonaws.com",
		`{"roleArn":"arn:aws:iam::123456789012:role/Admin","roleSessionName":"mysession"}`,
	)
	result, err := awsclient.FetchCloudTrailEventsPage(context.Background(), &singleEventCTMock{event: event}, "")
	if err != nil {
		t.Fatalf("FetchCloudTrailEventsPage error: %v", err)
	}
	target := result.Resources[0].Fields["_ct.target"]
	if target != "role/Admin" {
		t.Errorf("_ct.target = %q, want %q per §4 AssumeRole (ARN stripped per §5)", target, "role/Admin")
	}
}

// TestCTTargetFallback_BatchGetImage — repositoryName field.
func TestCTTargetFallback_BatchGetImage(t *testing.T) {
	event := buildCTEventWithRequestParams(
		"tf-bgi-01", "BatchGetImage", "ecr.amazonaws.com",
		`{"repositoryName":"myrepo","imageIds":[{"imageTag":"latest"}]}`,
	)
	result, err := awsclient.FetchCloudTrailEventsPage(context.Background(), &singleEventCTMock{event: event}, "")
	if err != nil {
		t.Fatalf("FetchCloudTrailEventsPage error: %v", err)
	}
	target := result.Resources[0].Fields["_ct.target"]
	if target != "myrepo" {
		t.Errorf("_ct.target = %q, want %q per §4 BatchGetImage", target, "myrepo")
	}
}

// TestCTTargetFallback_ListBuckets — ListBuckets has no target → "(none)".
func TestCTTargetFallback_ListBuckets(t *testing.T) {
	event := buildCTEventWithRequestParams(
		"tf-lb-01", "ListBuckets", "s3.amazonaws.com",
		`null`,
	)
	result, err := awsclient.FetchCloudTrailEventsPage(context.Background(), &singleEventCTMock{event: event}, "")
	if err != nil {
		t.Fatalf("FetchCloudTrailEventsPage error: %v", err)
	}
	target := result.Resources[0].Fields["_ct.target"]
	if target != "(none)" {
		t.Errorf("_ct.target = %q, want %q per §4 ListBuckets (no target)", target, "(none)")
	}
}

// TestCTTargetFallback_CatchAll_AnyKeyMatchingID — catch-all: scan for *Id/*Name/*Arn key.
func TestCTTargetFallback_CatchAll_AnyKeyMatchingID(t *testing.T) {
	// FrobnicateThingy has requestParameters.thingyId → "t-1"
	event := buildCTEventWithRequestParams(
		"tf-ca-01", "FrobnicateThingy", "example.amazonaws.com",
		`{"thingyId":"t-1","otherParam":"ignored"}`,
	)
	result, err := awsclient.FetchCloudTrailEventsPage(context.Background(), &singleEventCTMock{event: event}, "")
	if err != nil {
		t.Fatalf("FetchCloudTrailEventsPage error: %v", err)
	}
	target := result.Resources[0].Fields["_ct.target"]
	if target != "t-1" {
		t.Errorf("_ct.target = %q, want %q per §4 catch-all scan for *Id key", target, "t-1")
	}
}
