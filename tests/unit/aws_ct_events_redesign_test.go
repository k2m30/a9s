package unit

// Tests for the redesigned CloudTrail Events fetcher (T020Q, T024Q, T030Q).
//
// These tests are written BEFORE implementation exists (TDD).
// They will fail to compile until the coder exports:
//   - aws.ClassifyCTVerb(eventName, eventCategory, eventType string) string
//   - aws.ExtractCTTarget(parsed map[string]any) string
//   - (implicitly) FetchCloudTrailEventsPage writes _ct.* keys into Resource.Fields
//     and sets Resource.Status to "ct-write" or "ct-read"
//
// Bug vectors covered:
//   - Verb classifier maps wrong prefix (e.g. "StopInstances" → "?" instead of "W")
//   - Resource.Status set to old "true"/"false" instead of "ct-write"/"ct-read"
//   - Verb → Status mapping wrong (e.g. "D" classified as ct-read)
//   - _ct.* keys absent from Resource.Fields after fetch
//   - _ct.is_root = "true" for non-Root identity
//   - _ct.cross_account = "true" when accounts match
//   - _ct.outcome = "OK" when errorCode is non-empty
//   - Fetcher reverses LookupEvents newest-first order
//   - Missing userIdentity panics instead of producing safe defaults
//   - Unparseable CloudTrailEvent JSON panics instead of graceful fallback

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	demo "github.com/k2m30/a9s/v3/internal/demo"
)

// ===========================================================================
// T020Q — Verb classifier: ClassifyCTVerb
// ===========================================================================

func TestCTVerb_ReadPrefixes(t *testing.T) {
	cases := []struct {
		eventName string
		want      string
	}{
		{"DescribeInstances", "R"},
		{"DescribeSecurityGroups", "R"},
		{"GetObject", "R"},
		{"GetBucketAcl", "R"},
		{"ListBuckets", "R"},
		{"ListObjects", "R"},
		{"HeadObject", "R"},
		{"HeadBucket", "R"},
		{"LookupEvents", "R"},
		{"LookupPolicy", "R"},
	}
	for _, tc := range cases {
		got := awsclient.ClassifyCTVerb(tc.eventName, "Management", "AwsApiCall")
		if got != tc.want {
			t.Errorf("ClassifyCTVerb(%q, Management, AwsApiCall) = %q, want %q", tc.eventName, got, tc.want)
		}
	}
}

func TestCTVerb_WritePrefixes(t *testing.T) {
	cases := []struct {
		eventName string
		want      string
	}{
		{"CreateBucket", "W"},
		{"CreateTable", "W"},
		{"PutObject", "W"},
		{"PutBucketPolicy", "W"},
		{"UpdateFunctionCode", "W"},
		{"UpdateAccessKey", "W"},
		{"ModifyDBInstance", "W"},
		{"ModifySubnetAttribute", "W"},
		{"AttachRolePolicy", "W"},
		{"AttachGroupPolicy", "W"},
		{"RunInstances", "W"},
		{"AssumeRole", "W"},
		{"AssumeRoleWithWebIdentity", "W"},
	}
	for _, tc := range cases {
		got := awsclient.ClassifyCTVerb(tc.eventName, "Management", "AwsApiCall")
		if got != tc.want {
			t.Errorf("ClassifyCTVerb(%q, Management, AwsApiCall) = %q, want %q", tc.eventName, got, tc.want)
		}
	}
}

func TestCTVerb_DestructivePrefixes(t *testing.T) {
	cases := []struct {
		eventName string
		want      string
	}{
		{"DeleteBucket", "D"},
		{"DeleteTable", "D"},
		{"DeleteSecurityGroup", "D"},
		{"TerminateInstances", "D"},
		{"RevokeSecurityGroupIngress", "D"},
		{"DetachRolePolicy", "D"},
		{"CancelExportTask", "D"},
	}
	for _, tc := range cases {
		got := awsclient.ClassifyCTVerb(tc.eventName, "Management", "AwsApiCall")
		if got != tc.want {
			t.Errorf("ClassifyCTVerb(%q, Management, AwsApiCall) = %q, want %q", tc.eventName, got, tc.want)
		}
	}
}

func TestCTVerb_InsightCategory(t *testing.T) {
	// eventCategory "Insight" dominates event-name prefix matching.
	insightNames := []string{"DescribeInstances", "CreateBucket", "DeleteTable", "SomeUnknownEvent"}
	for _, name := range insightNames {
		got := awsclient.ClassifyCTVerb(name, "Insight", "AwsApiCall")
		if got != "I" {
			t.Errorf("ClassifyCTVerb(%q, Insight, AwsApiCall) = %q, want I", name, got)
		}
	}
}

func TestCTVerb_NetworkActivityCategory(t *testing.T) {
	// eventCategory "NetworkActivity" dominates event-name prefix matching.
	naNames := []string{"CreateNetworkInterface", "VpcEndpointConnect", "SomeNetEvent"}
	for _, name := range naNames {
		got := awsclient.ClassifyCTVerb(name, "NetworkActivity", "AwsApiCall")
		if got != "N" {
			t.Errorf("ClassifyCTVerb(%q, NetworkActivity, AwsApiCall) = %q, want N", name, got)
		}
	}
}

func TestCTVerb_AwsServiceEventType(t *testing.T) {
	// eventType "AwsServiceEvent" → "S".
	got := awsclient.ClassifyCTVerb("DescribeVolumes", "Management", "AwsServiceEvent")
	if got != "S" {
		t.Errorf("ClassifyCTVerb(DescribeVolumes, Management, AwsServiceEvent) = %q, want S", got)
	}
}

func TestCTVerb_UnknownEventName_ReturnsQuestionMark(t *testing.T) {
	got := awsclient.ClassifyCTVerb("SomeFutureUnknownApiCall", "Management", "AwsApiCall")
	if got != "?" {
		t.Errorf("ClassifyCTVerb(SomeFutureUnknownApiCall, Management, AwsApiCall) = %q, want ?", got)
	}
}

func TestCTVerb_Deterministic_SamePrecedenceOrder(t *testing.T) {
	// Insight category must win over an event name that would otherwise classify as R.
	gotInsight := awsclient.ClassifyCTVerb("DescribeInstances", "Insight", "AwsApiCall")
	if gotInsight != "I" {
		t.Errorf("Insight should beat R prefix: got %q, want I", gotInsight)
	}
	// NetworkActivity must win over an event name that would classify as W.
	gotNA := awsclient.ClassifyCTVerb("CreateNetworkInterface", "NetworkActivity", "AwsApiCall")
	if gotNA != "N" {
		t.Errorf("NetworkActivity should beat W prefix: got %q, want N", gotNA)
	}
	// AwsServiceEvent must win over R prefix.
	gotSvc := awsclient.ClassifyCTVerb("DescribeInstances", "Management", "AwsServiceEvent")
	if gotSvc != "S" {
		t.Errorf("AwsServiceEvent type should beat R prefix: got %q, want S", gotSvc)
	}
}

// ===========================================================================
// T020Q — Fetcher flattening: _ct.* fields written into Resource.Fields
// ===========================================================================

// buildSyntheticCTEvent constructs a cloudtrailtypes.Event with embedded JSON.
func buildSyntheticCTEvent(
	id, eventName, eventSource, username string,
	readOnly bool,
	eventTime time.Time,
	cloudTrailEventJSON string,
	resources []cloudtrailtypes.Resource,
) cloudtrailtypes.Event {
	roStr := "false"
	if readOnly {
		roStr = "true"
	}
	return cloudtrailtypes.Event{
		EventId:         aws.String(id),
		EventName:       aws.String(eventName),
		EventTime:       aws.Time(eventTime),
		EventSource:     aws.String(eventSource),
		Username:        aws.String(username),
		ReadOnly:        aws.String(roStr),
		CloudTrailEvent: aws.String(cloudTrailEventJSON),
		Resources:       resources,
	}
}

// buildFullCTEventJSON returns a complete CloudTrailEvent JSON string.
func buildFullCTEventJSON(
	accountID, recipientAccountID, sourceIP, region, userAgent,
	userType, eventCategory, eventType, errorCode string,
) string {
	roleField := ""
	if userType == "AssumedRole" || userType == "Role" {
		roleField = `,"sessionContext":{"sessionIssuer":{"userName":"test-role","type":"Role"}}`
	}
	return `{"eventVersion":"1.08","userIdentity":{"type":"` + userType + `","accountId":"` + accountID + `"` + roleField + `},"eventTime":"2026-03-28T14:30:15Z","eventSource":"ec2.amazonaws.com","eventName":"DescribeInstances","awsRegion":"` + region + `","sourceIPAddress":"` + sourceIP + `","userAgent":"` + userAgent + `","errorCode":"` + errorCode + `","eventCategory":"` + eventCategory + `","eventType":"` + eventType + `","recipientAccountId":"` + recipientAccountID + `"}`
}

// singleEventCTMock is a mock CloudTrail client that returns one event.
type singleEventCTMock struct {
	event cloudtrailtypes.Event
}

func (m *singleEventCTMock) LookupEvents(_ context.Context, _ *cloudtrail.LookupEventsInput, _ ...func(*cloudtrail.Options)) (*cloudtrail.LookupEventsOutput, error) {
	return &cloudtrail.LookupEventsOutput{Events: []cloudtrailtypes.Event{m.event}}, nil
}

// multiEventCTMock is a mock CloudTrail client that returns multiple events.
type multiEventCTMock struct {
	events []cloudtrailtypes.Event
}

func (m *multiEventCTMock) LookupEvents(_ context.Context, _ *cloudtrail.LookupEventsInput, _ ...func(*cloudtrail.Options)) (*cloudtrail.LookupEventsOutput, error) {
	return &cloudtrail.LookupEventsOutput{Events: m.events}, nil
}

func TestCTFlatten_AllCTFieldsPopulated(t *testing.T) {
	ctJSON := buildFullCTEventJSON(
		"111122223333", "111122223333", "203.0.113.42", "eu-west-1",
		"aws-cli/2.15.0", "AssumedRole", "Management", "AwsApiCall", "",
	)
	event := buildSyntheticCTEvent(
		"evt-flatten-001", "DescribeInstances", "ec2.amazonaws.com", "alice",
		true, time.Date(2026, 3, 28, 14, 30, 15, 0, time.UTC),
		ctJSON, []cloudtrailtypes.Resource{},
	)

	result, err := awsclient.FetchCloudTrailEventsPage(context.Background(), &singleEventCTMock{event: event}, "")
	if err != nil {
		t.Fatalf("FetchCloudTrailEventsPage returned error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]

	requiredKeys := []string{
		"_ct.verb", "_ct.actor", "_ct.origin", "_ct.target",
		"_ct.outcome", "_ct.error_code", "_ct.account_id",
		"_ct.recipient_account", "_ct.is_root", "_ct.cross_account",
		"_ct.event_category", "_ct.event_type", "_ct.source_ip", "_ct.region",
	}
	for _, k := range requiredKeys {
		if _, ok := r.Fields[k]; !ok {
			t.Errorf("Resource.Fields missing required key %q", k)
		}
	}
}

func TestCTFlatten_VerbFieldMatchesClassifier(t *testing.T) {
	ctJSON := buildFullCTEventJSON("111122223333", "111122223333", "1.2.3.4", "us-east-1",
		"aws-cli/2.0", "AssumedRole", "Management", "AwsApiCall", "")
	event := buildSyntheticCTEvent("evt-verb-001", "CreateBucket", "s3.amazonaws.com", "alice",
		false, time.Now(), ctJSON, nil)

	result, err := awsclient.FetchCloudTrailEventsPage(context.Background(), &singleEventCTMock{event: event}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	r := result.Resources[0]
	if r.Fields["_ct.verb"] != "W" {
		t.Errorf("_ct.verb = %q, want W for CreateBucket", r.Fields["_ct.verb"])
	}
}

func TestCTFlatten_OutcomeOKWhenNoErrorCode(t *testing.T) {
	ctJSON := buildFullCTEventJSON("111122223333", "111122223333", "1.2.3.4", "us-east-1",
		"aws-cli/2.0", "AssumedRole", "Management", "AwsApiCall", "")
	event := buildSyntheticCTEvent("evt-ok-001", "GetObject", "s3.amazonaws.com", "bob",
		true, time.Now(), ctJSON, nil)

	result, err := awsclient.FetchCloudTrailEventsPage(context.Background(), &singleEventCTMock{event: event}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	r := result.Resources[0]
	if r.Fields["_ct.outcome"] != "OK" {
		t.Errorf("_ct.outcome = %q, want OK when errorCode is empty", r.Fields["_ct.outcome"])
	}
	if r.Fields["_ct.error_code"] != "" {
		t.Errorf("_ct.error_code = %q, want empty string when no error", r.Fields["_ct.error_code"])
	}
}

func TestCTFlatten_OutcomeIsErrorCodeWhenPresent(t *testing.T) {
	ctJSON := buildFullCTEventJSON("111122223333", "111122223333", "1.2.3.4", "us-east-1",
		"aws-cli/2.0", "AssumedRole", "Management", "AwsApiCall", "AccessDenied")
	event := buildSyntheticCTEvent("evt-err-001", "DeleteBucket", "s3.amazonaws.com", "eve",
		false, time.Now(), ctJSON, nil)

	result, err := awsclient.FetchCloudTrailEventsPage(context.Background(), &singleEventCTMock{event: event}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	r := result.Resources[0]
	if r.Fields["_ct.outcome"] == "OK" {
		t.Error("_ct.outcome must not be OK when errorCode is non-empty")
	}
	if r.Fields["_ct.error_code"] != "AccessDenied" {
		t.Errorf("_ct.error_code = %q, want AccessDenied", r.Fields["_ct.error_code"])
	}
}

func TestCTFlatten_IsRootTrue(t *testing.T) {
	// Root identity: embed JSON with userIdentity.type = "Root"
	rootJSON := `{"eventVersion":"1.08","userIdentity":{"type":"Root","accountId":"111122223333","arn":"arn:aws:iam::111122223333:root"},"awsRegion":"us-east-1","sourceIPAddress":"1.2.3.4","eventCategory":"Management","eventType":"AwsApiCall","recipientAccountId":"111122223333"}`
	event := buildSyntheticCTEvent("evt-root-001", "CreateAccessKey", "iam.amazonaws.com", "",
		false, time.Now(), rootJSON, nil)

	result, err := awsclient.FetchCloudTrailEventsPage(context.Background(), &singleEventCTMock{event: event}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	r := result.Resources[0]
	if r.Fields["_ct.is_root"] != "true" {
		t.Errorf("_ct.is_root = %q, want true for Root identity", r.Fields["_ct.is_root"])
	}
}

func TestCTFlatten_IsRootFalseForNonRoot(t *testing.T) {
	ctJSON := buildFullCTEventJSON("111122223333", "111122223333", "1.2.3.4", "us-east-1",
		"aws-cli/2.0", "AssumedRole", "Management", "AwsApiCall", "")
	event := buildSyntheticCTEvent("evt-norootcheck-001", "DescribeInstances", "ec2.amazonaws.com", "alice",
		true, time.Now(), ctJSON, nil)

	result, err := awsclient.FetchCloudTrailEventsPage(context.Background(), &singleEventCTMock{event: event}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	r := result.Resources[0]
	if r.Fields["_ct.is_root"] != "false" {
		t.Errorf("_ct.is_root = %q, want false for non-Root identity", r.Fields["_ct.is_root"])
	}
}

func TestCTFlatten_CrossAccountTrue(t *testing.T) {
	// Different account IDs → cross_account = "true"
	ctJSON := buildFullCTEventJSON("111122223333", "444455556666", "1.2.3.4", "us-east-1",
		"aws-cli/2.0", "AssumedRole", "Management", "AwsApiCall", "")
	event := buildSyntheticCTEvent("evt-cross-001", "GetObject", "s3.amazonaws.com", "crossuser",
		true, time.Now(), ctJSON, nil)

	result, err := awsclient.FetchCloudTrailEventsPage(context.Background(), &singleEventCTMock{event: event}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	r := result.Resources[0]
	if r.Fields["_ct.cross_account"] != "true" {
		t.Errorf("_ct.cross_account = %q, want true when accountId != recipientAccountId", r.Fields["_ct.cross_account"])
	}
	if r.Fields["_ct.account_id"] != "111122223333" {
		t.Errorf("_ct.account_id = %q, want 111122223333", r.Fields["_ct.account_id"])
	}
	if r.Fields["_ct.recipient_account"] != "444455556666" {
		t.Errorf("_ct.recipient_account = %q, want 444455556666", r.Fields["_ct.recipient_account"])
	}
}

func TestCTFlatten_CrossAccountFalse(t *testing.T) {
	// Same account → cross_account = "false"
	ctJSON := buildFullCTEventJSON("111122223333", "111122223333", "1.2.3.4", "us-east-1",
		"aws-cli/2.0", "AssumedRole", "Management", "AwsApiCall", "")
	event := buildSyntheticCTEvent("evt-same-acct-001", "ListBuckets", "s3.amazonaws.com", "alice",
		true, time.Now(), ctJSON, nil)

	result, err := awsclient.FetchCloudTrailEventsPage(context.Background(), &singleEventCTMock{event: event}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	r := result.Resources[0]
	if r.Fields["_ct.cross_account"] != "false" {
		t.Errorf("_ct.cross_account = %q, want false when accounts match", r.Fields["_ct.cross_account"])
	}
}

func TestCTFlatten_RegionAndSourceIPExtracted(t *testing.T) {
	ctJSON := buildFullCTEventJSON("111122223333", "111122223333", "203.0.113.99", "ap-southeast-1",
		"aws-cli/2.0", "AssumedRole", "Management", "AwsApiCall", "")
	event := buildSyntheticCTEvent("evt-region-001", "DescribeVpcs", "ec2.amazonaws.com", "alice",
		true, time.Now(), ctJSON, nil)

	result, err := awsclient.FetchCloudTrailEventsPage(context.Background(), &singleEventCTMock{event: event}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	r := result.Resources[0]
	if r.Fields["_ct.region"] != "ap-southeast-1" {
		t.Errorf("_ct.region = %q, want ap-southeast-1", r.Fields["_ct.region"])
	}
	if r.Fields["_ct.source_ip"] != "203.0.113.99" {
		t.Errorf("_ct.source_ip = %q, want 203.0.113.99", r.Fields["_ct.source_ip"])
	}
}

func TestCTFlatten_EventCategoryAndTypeExtracted(t *testing.T) {
	ctJSON := buildFullCTEventJSON("111122223333", "111122223333", "1.2.3.4", "us-east-1",
		"aws-cli/2.0", "AssumedRole", "Data", "AwsApiCall", "")
	event := buildSyntheticCTEvent("evt-cat-001", "GetObject", "s3.amazonaws.com", "alice",
		true, time.Now(), ctJSON, nil)

	result, err := awsclient.FetchCloudTrailEventsPage(context.Background(), &singleEventCTMock{event: event}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	r := result.Resources[0]
	if r.Fields["_ct.event_category"] != "Data" {
		t.Errorf("_ct.event_category = %q, want Data", r.Fields["_ct.event_category"])
	}
	if r.Fields["_ct.event_type"] != "AwsApiCall" {
		t.Errorf("_ct.event_type = %q, want AwsApiCall", r.Fields["_ct.event_type"])
	}
}

// ===========================================================================
// T020Q — Resource.Status mapping: ct-write vs ct-read
// ===========================================================================

func TestCTFlatten_StatusCTWrite_ForVerbW(t *testing.T) {
	ctJSON := buildFullCTEventJSON("111122223333", "111122223333", "1.2.3.4", "us-east-1",
		"aws-cli/2.0", "AssumedRole", "Management", "AwsApiCall", "")
	event := buildSyntheticCTEvent("evt-status-w", "CreateBucket", "s3.amazonaws.com", "alice",
		false, time.Now(), ctJSON, nil)

	result, err := awsclient.FetchCloudTrailEventsPage(context.Background(), &singleEventCTMock{event: event}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Resources[0].Status != "ct-write" {
		t.Errorf("Resource.Status = %q, want ct-write for verb W (CreateBucket)", result.Resources[0].Status)
	}
}

func TestCTFlatten_StatusCTWrite_ForVerbD(t *testing.T) {
	ctJSON := buildFullCTEventJSON("111122223333", "111122223333", "1.2.3.4", "us-east-1",
		"aws-cli/2.0", "AssumedRole", "Management", "AwsApiCall", "")
	event := buildSyntheticCTEvent("evt-status-d", "DeleteTable", "dynamodb.amazonaws.com", "alice",
		false, time.Now(), ctJSON, nil)

	result, err := awsclient.FetchCloudTrailEventsPage(context.Background(), &singleEventCTMock{event: event}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Resources[0].Status != "ct-write" {
		t.Errorf("Resource.Status = %q, want ct-write for verb D (DeleteTable)", result.Resources[0].Status)
	}
}

func TestCTFlatten_StatusCTRead_ForVerbR(t *testing.T) {
	ctJSON := buildFullCTEventJSON("111122223333", "111122223333", "1.2.3.4", "us-east-1",
		"aws-cli/2.0", "AssumedRole", "Management", "AwsApiCall", "")
	event := buildSyntheticCTEvent("evt-status-r", "DescribeInstances", "ec2.amazonaws.com", "alice",
		true, time.Now(), ctJSON, nil)

	result, err := awsclient.FetchCloudTrailEventsPage(context.Background(), &singleEventCTMock{event: event}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Resources[0].Status != "ct-read" {
		t.Errorf("Resource.Status = %q, want ct-read for verb R (DescribeInstances)", result.Resources[0].Status)
	}
}

func TestCTFlatten_StatusCTRead_ForVerbS(t *testing.T) {
	// AwsServiceEvent → verb S → ct-read
	svcJSON := buildFullCTEventJSON("111122223333", "111122223333", "kms.amazonaws.com", "us-east-1",
		"kms.amazonaws.com", "AWSService", "Management", "AwsServiceEvent", "")
	event := buildSyntheticCTEvent("evt-status-s", "GenerateDataKey", "kms.amazonaws.com", "",
		false, time.Now(), svcJSON, nil)

	result, err := awsclient.FetchCloudTrailEventsPage(context.Background(), &singleEventCTMock{event: event}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Resources[0].Status != "ct-read" {
		t.Errorf("Resource.Status = %q, want ct-read for verb S (AwsServiceEvent)", result.Resources[0].Status)
	}
}

func TestCTFlatten_StatusCTRead_ForVerbI(t *testing.T) {
	ctJSON := buildFullCTEventJSON("111122223333", "111122223333", "1.2.3.4", "us-east-1",
		"cloudtrail.amazonaws.com", "AssumedRole", "Insight", "AwsApiCall", "")
	event := buildSyntheticCTEvent("evt-status-i", "DescribeInstances", "ec2.amazonaws.com", "alice",
		false, time.Now(), ctJSON, nil)

	result, err := awsclient.FetchCloudTrailEventsPage(context.Background(), &singleEventCTMock{event: event}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Resources[0].Status != "ct-read" {
		t.Errorf("Resource.Status = %q, want ct-read for verb I (Insight category)", result.Resources[0].Status)
	}
}

func TestCTFlatten_StatusCTRead_ForVerbN(t *testing.T) {
	ctJSON := buildFullCTEventJSON("111122223333", "111122223333", "10.0.0.1", "us-east-1",
		"vpc.amazonaws.com", "AssumedRole", "NetworkActivity", "AwsApiCall", "")
	event := buildSyntheticCTEvent("evt-status-n", "CreateNetworkInterface", "ec2.amazonaws.com", "alice",
		false, time.Now(), ctJSON, nil)

	result, err := awsclient.FetchCloudTrailEventsPage(context.Background(), &singleEventCTMock{event: event}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Resources[0].Status != "ct-read" {
		t.Errorf("Resource.Status = %q, want ct-read for verb N (NetworkActivity category)", result.Resources[0].Status)
	}
}

func TestCTFlatten_StatusCTRead_ForVerbQuestionMark(t *testing.T) {
	// Unknown event name → verb "?" → ct-read
	ctJSON := buildFullCTEventJSON("111122223333", "111122223333", "1.2.3.4", "us-east-1",
		"aws-cli/2.0", "AssumedRole", "Management", "AwsApiCall", "")
	event := buildSyntheticCTEvent("evt-status-q", "SomeFutureUnknownApiCall", "ec2.amazonaws.com", "alice",
		false, time.Now(), ctJSON, nil)

	result, err := awsclient.FetchCloudTrailEventsPage(context.Background(), &singleEventCTMock{event: event}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Resources[0].Status != "ct-read" {
		t.Errorf("Resource.Status = %q, want ct-read for verb ? (unknown event)", result.Resources[0].Status)
	}
}

func TestCTFlatten_StatusOnlyTwoValues(t *testing.T) {
	// Exhaustive check: Status must only ever be "ct-write" or "ct-read" —
	// never the legacy "true"/"false" or empty string.
	eventDefs := []struct {
		id        string
		eventName string
	}{
		{"evt-multi-01", "CreateBucket"},
		{"evt-multi-02", "DescribeInstances"},
		{"evt-multi-03", "DeleteTable"},
		{"evt-multi-04", "GetObject"},
		{"evt-multi-05", "PutObject"},
		{"evt-multi-06", "ListBuckets"},
	}
	var events []cloudtrailtypes.Event
	for _, d := range eventDefs {
		ctJSON := buildFullCTEventJSON("111122223333", "111122223333", "1.2.3.4", "us-east-1",
			"aws-cli/2.0", "AssumedRole", "Management", "AwsApiCall", "")
		events = append(events, buildSyntheticCTEvent(d.id, d.eventName, "s3.amazonaws.com", "alice",
			false, time.Now(), ctJSON, nil))
	}

	result, err := awsclient.FetchCloudTrailEventsPage(context.Background(), &multiEventCTMock{events: events}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, r := range result.Resources {
		if r.Status != "ct-write" && r.Status != "ct-read" {
			t.Errorf("Resource %q: Status = %q, must be exactly ct-write or ct-read (not legacy true/false)", r.ID, r.Status)
		}
	}
}

// ===========================================================================
// T020Q — Newest-first order preserved
// ===========================================================================

func TestCTFlatten_NewestFirstOrderPreserved(t *testing.T) {
	// LookupEvents returns newest first. The fetcher must NOT reverse this order.
	t1 := time.Date(2026, 3, 28, 15, 0, 0, 0, time.UTC) // newest
	t2 := time.Date(2026, 3, 28, 14, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 3, 28, 13, 0, 0, 0, time.UTC) // oldest

	ctJSON := buildFullCTEventJSON("111122223333", "111122223333", "1.2.3.4", "us-east-1",
		"aws-cli/2.0", "AssumedRole", "Management", "AwsApiCall", "")

	events := []cloudtrailtypes.Event{
		buildSyntheticCTEvent("evt-order-01", "DescribeInstances", "ec2.amazonaws.com", "a", true, t1, ctJSON, nil),
		buildSyntheticCTEvent("evt-order-02", "DescribeVpcs", "ec2.amazonaws.com", "b", true, t2, ctJSON, nil),
		buildSyntheticCTEvent("evt-order-03", "ListBuckets", "s3.amazonaws.com", "c", true, t3, ctJSON, nil),
	}

	result, err := awsclient.FetchCloudTrailEventsPage(context.Background(), &multiEventCTMock{events: events}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Resources) != 3 {
		t.Fatalf("expected 3 resources, got %d", len(result.Resources))
	}
	// First resource must be the newest event.
	if result.Resources[0].ID != "evt-order-01" {
		t.Errorf("result[0].ID = %q, want evt-order-01 (newest first)", result.Resources[0].ID)
	}
	if result.Resources[2].ID != "evt-order-03" {
		t.Errorf("result[2].ID = %q, want evt-order-03 (oldest last)", result.Resources[2].ID)
	}
}

// ===========================================================================
// T020Q — Edge cases: missing/empty userIdentity, unparseable JSON
// ===========================================================================

func TestCTFlatten_MissingUserIdentity_ActorIsNotBlank(t *testing.T) {
	// CloudTrailEvent JSON with no userIdentity field.
	noIdentityJSON := `{"eventVersion":"1.08","eventName":"GetObject","awsRegion":"us-east-1","sourceIPAddress":"1.2.3.4","eventCategory":"Management","eventType":"AwsApiCall","recipientAccountId":"111122223333"}`
	event := buildSyntheticCTEvent("evt-noidentity-001", "GetObject", "s3.amazonaws.com", "",
		true, time.Now(), noIdentityJSON, nil)

	result, err := awsclient.FetchCloudTrailEventsPage(context.Background(), &singleEventCTMock{event: event}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	r := result.Resources[0]
	// Actor must not be blank; data-model says never blank (use "-" as safe default).
	if r.Fields["_ct.actor"] == "" {
		t.Error("_ct.actor must not be empty string when userIdentity is missing; use \"-\"")
	}
	if r.Fields["_ct.is_root"] != "false" {
		t.Errorf("_ct.is_root = %q, want false when userIdentity is missing", r.Fields["_ct.is_root"])
	}
}

func TestCTFlatten_UnparseableCTEventJSON_NoPanic(t *testing.T) {
	// Unparseable JSON must not panic; the row must still be constructed.
	event := buildSyntheticCTEvent("evt-badJSON-001", "GetObject", "s3.amazonaws.com", "bob",
		true, time.Now(), "THIS IS NOT JSON {{{", nil)

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("FetchCloudTrailEventsPage panicked on bad CloudTrailEvent JSON: %v", r)
		}
	}()
	result, err := awsclient.FetchCloudTrailEventsPage(context.Background(), &singleEventCTMock{event: event}, "")
	if err != nil {
		// An error return is also acceptable — but no panic.
		return
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource even on bad JSON, got %d", len(result.Resources))
	}
	// _ct.target must be "(none)" not blank.
	if result.Resources[0].Fields["_ct.target"] == "" {
		t.Error("_ct.target must not be empty on bad JSON; expect \"(none)\"")
	}
}

func TestCTFlatten_NilCTEventPointer_NoPanic(t *testing.T) {
	// CloudTrailEvent field is nil (API may omit it).
	event := cloudtrailtypes.Event{
		EventId:         aws.String("evt-nilJSON-001"),
		EventName:       aws.String("GetObject"),
		EventTime:       aws.Time(time.Now()),
		EventSource:     aws.String("s3.amazonaws.com"),
		Username:        aws.String("bob"),
		ReadOnly:        aws.String("true"),
		CloudTrailEvent: nil,
		Resources:       nil,
	}

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("FetchCloudTrailEventsPage panicked on nil CloudTrailEvent: %v", r)
		}
	}()
	result, err := awsclient.FetchCloudTrailEventsPage(context.Background(), &singleEventCTMock{event: event}, "")
	if err != nil {
		return
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource even on nil CloudTrailEvent, got %d", len(result.Resources))
	}
	if result.Resources[0].Fields["_ct.actor"] == "" {
		t.Error("_ct.actor must not be blank even when CloudTrailEvent is nil")
	}
}

// ===========================================================================
// T030Q — TARGET extraction: ExtractCTTarget
// ===========================================================================

func TestExtractCTTarget_ResourcesArrayNonEmpty_ReturnsFirstResource(t *testing.T) {
	// When resources[] has entries, the first resource ARN/name is returned.
	parsed := map[string]any{
		"resources": []any{
			map[string]any{
				"ARN":          "arn:aws:s3:::my-bucket",
				"accountId":    "111122223333",
				"type":         "AWS::S3::Bucket",
				"resourceName": "my-bucket",
			},
			map[string]any{
				"ARN":          "arn:aws:s3:::second-bucket",
				"accountId":    "111122223333",
				"type":         "AWS::S3::Bucket",
				"resourceName": "second-bucket",
			},
		},
		"eventCategory": "Management",
		"eventType":     "AwsApiCall",
	}
	got := awsclient.ExtractCTTarget(parsed)
	if got == "" || got == "(none)" {
		t.Errorf("ExtractCTTarget with resources[] = %q, want first resource ARN/name", got)
	}
	// Must not return the second resource.
	if got == "arn:aws:s3:::second-bucket" || got == "second-bucket" {
		t.Errorf("ExtractCTTarget returned second resource %q, want first", got)
	}
}

func TestExtractCTTarget_InsightCategory_ReturnsEventNameWithRatio(t *testing.T) {
	// Insight events: return "<eventName> ×<ratio>" from insightDetails.
	parsed := map[string]any{
		"resources":     []any{},
		"eventCategory": "Insight",
		"eventType":     "AwsApiCall",
		"eventName":     "DescribeInstances",
		"insightDetails": map[string]any{
			"state": "Start",
			"insightContext": map[string]any{
				"statistics": map[string]any{
					"baseline": map[string]any{
						"average": float64(2.5),
					},
					"insight": map[string]any{
						"average": float64(12.0),
					},
				},
			},
		},
	}
	got := awsclient.ExtractCTTarget(parsed)
	if got == "" || got == "(none)" {
		t.Errorf("ExtractCTTarget Insight = %q, want non-empty target with event name and ratio", got)
	}
	if !strContains(got, "DescribeInstances") {
		t.Errorf("ExtractCTTarget Insight = %q, must contain eventName DescribeInstances", got)
	}
}

func TestExtractCTTarget_NetworkActivity_ReturnsVpceAndService(t *testing.T) {
	// NetworkActivity: "<vpce-id> → <svc>" format.
	parsed := map[string]any{
		"resources":     []any{},
		"eventCategory": "NetworkActivity",
		"eventType":     "AwsApiCall",
		"eventSource":   "s3.amazonaws.com",
		"vpcEndpointId": "vpce-0a1b2c3d4e5f60001",
	}
	got := awsclient.ExtractCTTarget(parsed)
	if got == "" || got == "(none)" {
		t.Errorf("ExtractCTTarget NetworkActivity = %q, want vpce → service format", got)
	}
	if !strContains(got, "vpce-0a1b2c3d4e5f60001") {
		t.Errorf("ExtractCTTarget NetworkActivity = %q, must contain vpce ID", got)
	}
	// Service prefix (strip .amazonaws.com).
	if !strContains(got, "s3") {
		t.Errorf("ExtractCTTarget NetworkActivity = %q, must contain service prefix s3", got)
	}
}

func TestExtractCTTarget_AwsServiceEvent_ReturnsServicePrincipal(t *testing.T) {
	// AwsServiceEvent: return the eventSource (service principal).
	parsed := map[string]any{
		"resources":     []any{},
		"eventCategory": "Management",
		"eventType":     "AwsServiceEvent",
		"eventSource":   "kms.amazonaws.com",
	}
	got := awsclient.ExtractCTTarget(parsed)
	if got == "" || got == "(none)" {
		t.Errorf("ExtractCTTarget AwsServiceEvent = %q, want service principal", got)
	}
	if !strContains(got, "kms") {
		t.Errorf("ExtractCTTarget AwsServiceEvent = %q, must reference kms service", got)
	}
}

func TestExtractCTTarget_ManagementNoResources_ReturnsNone(t *testing.T) {
	// Management event with empty resources[] → "(none)".
	parsed := map[string]any{
		"resources":     []any{},
		"eventCategory": "Management",
		"eventType":     "AwsApiCall",
	}
	got := awsclient.ExtractCTTarget(parsed)
	if got != "(none)" {
		t.Errorf("ExtractCTTarget management+no resources = %q, want (none)", got)
	}
}

func TestExtractCTTarget_ManagementNilResources_ReturnsNone(t *testing.T) {
	// Management event with absent resources key → "(none)".
	parsed := map[string]any{
		"eventCategory": "Management",
		"eventType":     "AwsApiCall",
	}
	got := awsclient.ExtractCTTarget(parsed)
	if got != "(none)" {
		t.Errorf("ExtractCTTarget management+nil resources = %q, want (none)", got)
	}
}

func TestExtractCTTarget_EmptyMap_ReturnsNoneNeverBlank(t *testing.T) {
	// Empty map — must return "(none)", never blank, never panic.
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("ExtractCTTarget panicked on empty input: %v", r)
		}
	}()
	got := awsclient.ExtractCTTarget(map[string]any{})
	if got == "" {
		t.Error("ExtractCTTarget empty input returned blank string, want (none)")
	}
}

func TestExtractCTTarget_NilInput_ReturnsNoneNeverPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("ExtractCTTarget panicked on nil input: %v", r)
		}
	}()
	got := awsclient.ExtractCTTarget(nil)
	if got == "" {
		t.Error("ExtractCTTarget nil input returned blank string, want (none)")
	}
}

// ===========================================================================
// T024Q — Demo fixture coverage
// ===========================================================================

// loadCTEventsFixtures returns the ct-events demo resources and fails the test
// if none are found.
func loadCTEventsFixtures(t *testing.T) []interface{ getRawEvent() (cloudtrailtypes.Event, bool) } {
	t.Helper()
	// This signature is a documentation stub. The tests below use demo.GetResources
	// and resource.Resource directly (no wrapper interface).
	return nil
}

// The actual T024Q tests call demo.GetResources("ct-events") directly.

func TestCTEventsFixtureCoverage_AllVerbsPresent(t *testing.T) {
	// The demo fixture for ct-events must contain at least one event for each
	// verb class: R, W, D, S, I, N.
	//
	// Pre-T025C: the fixtures still use old Status values ("true"/"false") and
	// do not have _ct.* keys. We derive the verb from the event name + category/type
	// embedded in the CloudTrailEvent JSON.
	// Post-T025C: we read _ct.verb directly.

	resources, ok := demo.GetResources("ct-events")
	if !ok || len(resources) == 0 {
		t.Fatal("demo.GetResources(\"ct-events\") returned no fixtures")
	}

	verbBuckets := map[string]bool{
		"R": false, "W": false, "D": false,
		"S": false, "I": false, "N": false,
	}

	for _, r := range resources {
		// Post-T025C path: read _ct.verb directly.
		if v, hasCTVerb := r.Fields["_ct.verb"]; hasCTVerb {
			if _, known := verbBuckets[v]; known {
				verbBuckets[v] = true
			}
			continue
		}
		// Pre-T025C path: inspect the embedded CloudTrailEvent JSON.
		event, ok := r.RawStruct.(cloudtrailtypes.Event)
		if !ok || event.EventName == nil {
			continue
		}
		category, eType := ctTestParseEventCategoryType(event.CloudTrailEvent)
		v := awsclient.ClassifyCTVerb(*event.EventName, category, eType)
		if _, known := verbBuckets[v]; known {
			verbBuckets[v] = true
		}
	}

	for verb, present := range verbBuckets {
		if !present {
			t.Errorf("demo ct-events fixture missing at least one event with verb %q; add a fixture covering this verb class", verb)
		}
	}
}

func TestCTEventsFixtureCoverage_AllTargetFallbackCategoriesPresent(t *testing.T) {
	// The demo fixture must cover each TARGET-fallback category per research D5:
	//   - standard resources[] (any event with non-empty resources)
	//   - Insight (eventCategory == Insight)
	//   - NetworkActivity (eventCategory == NetworkActivity)
	//   - AwsServiceEvent (eventType == AwsServiceEvent)
	//   - management (none) (Management + no resources[])

	resources, ok := demo.GetResources("ct-events")
	if !ok || len(resources) == 0 {
		t.Fatal("demo.GetResources(\"ct-events\") returned no fixtures")
	}

	hasResources := false
	hasInsight := false
	hasNetworkActivity := false
	hasAwsServiceEvent := false
	hasManagementNone := false

	for _, r := range resources {
		event, ok := r.RawStruct.(cloudtrailtypes.Event)
		if !ok {
			continue
		}
		if len(event.Resources) > 0 {
			hasResources = true
		}
		category, eType := ctTestParseEventCategoryType(event.CloudTrailEvent)
		switch {
		case category == "Insight":
			hasInsight = true
		case category == "NetworkActivity":
			hasNetworkActivity = true
		case eType == "AwsServiceEvent":
			hasAwsServiceEvent = true
		case category == "Management" && len(event.Resources) == 0:
			hasManagementNone = true
		}
	}

	if !hasResources {
		t.Error("demo ct-events fixture missing TARGET-fallback: standard resources[] — add a fixture with non-empty Resources")
	}
	if !hasInsight {
		t.Error("demo ct-events fixture missing TARGET-fallback: Insight category (eventCategory=Insight)")
	}
	if !hasNetworkActivity {
		t.Error("demo ct-events fixture missing TARGET-fallback: NetworkActivity category (eventCategory=NetworkActivity)")
	}
	if !hasAwsServiceEvent {
		t.Error("demo ct-events fixture missing TARGET-fallback: AwsServiceEvent (eventType=AwsServiceEvent)")
	}
	if !hasManagementNone {
		t.Error("demo ct-events fixture missing TARGET-fallback: Management with no resources → (none)")
	}
}

func TestCTEventsFixtureCoverage_AtLeastOneRootEvent(t *testing.T) {
	resources, ok := demo.GetResources("ct-events")
	if !ok || len(resources) == 0 {
		t.Fatal("demo.GetResources(\"ct-events\") returned no fixtures")
	}
	for _, r := range resources {
		// Post-T025C path.
		if r.Fields["_ct.is_root"] == "true" {
			return
		}
		// Pre-T025C path: check CloudTrailEvent JSON.
		event, ok := r.RawStruct.(cloudtrailtypes.Event)
		if !ok || event.CloudTrailEvent == nil {
			continue
		}
		if ctTestIsRoot(event.CloudTrailEvent) {
			return
		}
	}
	t.Error("demo ct-events fixture missing at least one Root identity event (userIdentity.type=Root)")
}

func TestCTEventsFixtureCoverage_AtLeastOneErrorCodeEvent(t *testing.T) {
	resources, ok := demo.GetResources("ct-events")
	if !ok || len(resources) == 0 {
		t.Fatal("demo.GetResources(\"ct-events\") returned no fixtures")
	}
	for _, r := range resources {
		// Post-T025C path.
		if r.Fields["_ct.error_code"] != "" {
			return
		}
		// Pre-T025C path.
		event, ok := r.RawStruct.(cloudtrailtypes.Event)
		if !ok || event.CloudTrailEvent == nil {
			continue
		}
		if ctTestHasErrorCode(event.CloudTrailEvent) {
			return
		}
	}
	t.Error("demo ct-events fixture missing at least one event with non-empty errorCode")
}

func TestCTEventsFixtureCoverage_AtLeastOneCrossAccountEvent(t *testing.T) {
	resources, ok := demo.GetResources("ct-events")
	if !ok || len(resources) == 0 {
		t.Fatal("demo.GetResources(\"ct-events\") returned no fixtures")
	}
	for _, r := range resources {
		// Post-T025C path.
		if r.Fields["_ct.cross_account"] == "true" {
			return
		}
		// Pre-T025C path.
		event, ok := r.RawStruct.(cloudtrailtypes.Event)
		if !ok || event.CloudTrailEvent == nil {
			continue
		}
		if ctTestIsCrossAccount(event.CloudTrailEvent) {
			return
		}
	}
	t.Error("demo ct-events fixture missing at least one cross-account event (accountId != recipientAccountId)")
}

// ---------------------------------------------------------------------------
// T024Q helpers: lightweight JSON field extractors for pre-T025C fixture inspection
// ---------------------------------------------------------------------------

// ctTestParseEventCategoryType parses a CloudTrailEvent JSON string and returns
// (eventCategory, eventType). Returns ("", "") on nil/empty/parse-error.
func ctTestParseEventCategoryType(s *string) (category, eType string) {
	if s == nil || *s == "" {
		return "", ""
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(*s), &m); err != nil {
		return "", ""
	}
	if v, ok := m["eventCategory"].(string); ok {
		category = v
	}
	if v, ok := m["eventType"].(string); ok {
		eType = v
	}
	return category, eType
}

// ctTestIsRoot returns true if the CloudTrailEvent JSON has userIdentity.type == "Root".
func ctTestIsRoot(s *string) bool {
	if s == nil || *s == "" {
		return false
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(*s), &m); err != nil {
		return false
	}
	ui, ok := m["userIdentity"].(map[string]any)
	if !ok {
		return false
	}
	return ui["type"] == "Root"
}

// ctTestHasErrorCode returns true if the CloudTrailEvent JSON has a non-empty errorCode.
func ctTestHasErrorCode(s *string) bool {
	if s == nil || *s == "" {
		return false
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(*s), &m); err != nil {
		return false
	}
	v, ok := m["errorCode"].(string)
	return ok && v != ""
}

// ctTestIsCrossAccount returns true if the CloudTrailEvent JSON has
// userIdentity.accountId != recipientAccountId.
func ctTestIsCrossAccount(s *string) bool {
	if s == nil || *s == "" {
		return false
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(*s), &m); err != nil {
		return false
	}
	ui, ok := m["userIdentity"].(map[string]any)
	if !ok {
		return false
	}
	actorAccount, _ := ui["accountId"].(string)
	recipientAccount, _ := m["recipientAccountId"].(string)
	return actorAccount != "" && recipientAccount != "" && actorAccount != recipientAccount
}

// strContains is a small helper so the test file avoids importing strings.
func strContains(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Ensure demo import is used (drives init() to register demo fixtures).
var _ = demo.GetResources
