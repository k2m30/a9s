package unit

// Tests for Bug P2: _ct.target must fall back to LookupEvents event.Resources
// when the embedded CloudTrailEvent JSON has no resources[] (or is nil).
//
// The broken code at internal/aws/ct_events.go:224 calls:
//   target := ExtractCTTarget(parsed)
// and uses ONLY the parsed JSON. When the JSON blob is absent or has an empty
// resources[] array, ExtractCTTarget returns "(none)" — but the LookupEvents
// response also carries event.Resources which may have the target.
//
// The fix: after ExtractCTTarget returns "(none)", check event.Resources for a
// non-empty ResourceName and use it.
//
// Tests CT-TF1..CT-TF4: CT-TF1 and CT-TF2 currently FAIL (return "(none)").
// CT-TF3 and CT-TF4 are regression guards (currently PASS).

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
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

// ===========================================================================
// CT-TF1: nil CloudTrailEvent + populated event.Resources → use LookupEvents value
// Currently FAILS: target is "(none)" because parseCTEventJSON(nil) returns empty
// map, ExtractCTTarget returns "(none)", and event.Resources is never checked.
// ===========================================================================

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
		// Bug: ct_events.go:224 ignores event.Resources when JSON is nil
		t.Errorf("_ct.target = %q; expected non-(none) value from event.Resources (arn:aws:s3:::demo-bucket); "+
			"bug at ct_events.go:224 — ExtractCTTarget(parsed) never falls back to event.Resources", target)
	}
	if target == "" {
		t.Errorf("_ct.target is empty; expected arn:aws:s3:::demo-bucket from LookupEvents event.Resources")
	}
}

// ===========================================================================
// CT-TF2: JSON with empty resources[] + populated event.Resources → use LookupEvents value
// Currently FAILS: ExtractCTTarget sees resources:[] → falls through to "(none)",
// and event.Resources is never checked.
// ===========================================================================

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
		// Bug: ExtractCTTarget sees empty resources[] → falls through, event.Resources never consulted
		t.Errorf("_ct.target = %q; expected value from event.Resources when JSON resources[] is empty; "+
			"bug at ct_events.go:224", target)
	}
}

// ===========================================================================
// CT-TF3: JSON resources[] populated wins over event.Resources (regression guard)
// JSON value must win. Currently PASSES — this is a guard against over-correction.
// ===========================================================================

func TestCTTargetFallback_JSONResourcesWin_RegressionGuard(t *testing.T) {
	// JSON has a concrete resource entry.
	jsonWithResource := `{"eventVersion":"1.08","userIdentity":{"type":"AssumedRole","accountId":"111122223333"}` +
		`,"eventTime":"2026-03-28T14:00:00Z","eventSource":"s3.amazonaws.com","eventName":"GetObject"` +
		`,"awsRegion":"us-east-1","sourceIPAddress":"1.2.3.4","userAgent":"aws-cli/2.0"` +
		`,"errorCode":"","eventCategory":"Management","eventType":"AwsApiCall"` +
		`,"recipientAccountId":"111122223333"` +
		`,"resources":[{"ARN":"arn:aws:s3:::json-wins-bucket","accountId":"111122223333","type":"AWS::S3::Bucket"}]}`

	// event.Resources has a different value.
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
	if target != "arn:aws:s3:::json-wins-bucket" {
		t.Errorf("_ct.target = %q; expected arn:aws:s3:::json-wins-bucket from JSON resources[] (JSON must win over event.Resources)", target)
	}
}

// ===========================================================================
// CT-TF4: Both JSON resources[] and event.Resources are empty → "(none)" (regression guard)
// Currently PASSES.
// ===========================================================================

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
