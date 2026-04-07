package unit

// Tests for the §1.1 severity model: Resource.Status must be one of
// "ct-info" / "ct-attention" / "ct-danger" per docs/design/ct-event-list-v2.md §1.1.
//
// Replaces the old ct-write / ct-read binary model (removed in v2 redesign).

import (
	"context"
	"testing"
	"time"

	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
)

// buildCTEventWithStatus is a convenience wrapper that builds a singleEventCTMock
// and calls FetchCloudTrailEventsPage, returning the single resource or failing.
func buildCTEventWithStatus(t *testing.T, id, eventName, eventSource, username string,
	ctJSON string, resources []cloudtrailtypes.Resource,
) (status, verb string) {
	t.Helper()
	event := buildSyntheticCTEvent(id, eventName, eventSource, username, false,
		time.Date(2026, 3, 28, 14, 30, 0, 0, time.UTC), ctJSON, resources)
	result, err := awsclient.FetchCloudTrailEventsPage(context.Background(), &singleEventCTMock{event: event}, "")
	if err != nil {
		t.Fatalf("FetchCloudTrailEventsPage error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]
	return r.Status, r.Fields["_ct.verb"]
}

// plainAccountJSON returns JSON for a regular AssumedRole event with no errorCode.
func plainAccountJSON(eventName, eventCategory, eventType string) string {
	return buildFullCTEventJSON(
		"123456789012", "123456789012", "1.2.3.4", "us-east-1",
		"aws-cli/2.0", "AssumedRole", eventCategory, eventType, "",
	)
}

// ===========================================================================
// CT1: CreateBucket (verb W) → Status must be "ct-attention"
// §1.2 rule: Verb W → ct-attention
// ===========================================================================

func TestCTStatus_CreateBucket_IsCtAttention(t *testing.T) {
	ctJSON := plainAccountJSON("CreateBucket", "Management", "AwsApiCall")
	status, verb := buildCTEventWithStatus(t, "st-01", "CreateBucket", "s3.amazonaws.com", "alice", ctJSON, nil)
	if verb != "W" {
		t.Errorf("_ct.verb = %q, want W for CreateBucket", verb)
	}
	if status != "ct-attention" {
		t.Errorf("Status = %q, want ct-attention for verb W (CreateBucket) per §1.2", status)
	}
}

// ===========================================================================
// CT2: DeleteTable (verb D) → Status must be "ct-danger"
// §1.2 rule: Verb D → ct-danger
// ===========================================================================

func TestCTStatus_DeleteTable_IsCtDanger(t *testing.T) {
	ctJSON := plainAccountJSON("DeleteTable", "Management", "AwsApiCall")
	status, verb := buildCTEventWithStatus(t, "st-02", "DeleteTable", "dynamodb.amazonaws.com", "bob", ctJSON, nil)
	if verb != "D" {
		t.Errorf("_ct.verb = %q, want D for DeleteTable", verb)
	}
	if status != "ct-danger" {
		t.Errorf("Status = %q, want ct-danger for verb D (DeleteTable) per §1.2", status)
	}
}

// ===========================================================================
// CT3: DescribeInstances (verb R) → Status must be "ct-info"
// §1.2 rule: plain read, no sensitive, no root, same-account → ct-info
// ===========================================================================

func TestCTStatus_DescribeInstances_IsCtInfo(t *testing.T) {
	ctJSON := plainAccountJSON("DescribeInstances", "Management", "AwsApiCall")
	status, verb := buildCTEventWithStatus(t, "st-03", "DescribeInstances", "ec2.amazonaws.com", "carol", ctJSON, nil)
	if verb != "R" {
		t.Errorf("_ct.verb = %q, want R for DescribeInstances", verb)
	}
	if status != "ct-info" {
		t.Errorf("Status = %q, want ct-info for verb R (DescribeInstances) per §1.2", status)
	}
}

// ===========================================================================
// CT4: AwsServiceEvent (eventType=AwsServiceEvent, verb S) → Status must be "ct-info"
// §1.2 rule: plain read/service, no error → ct-info
// ===========================================================================

func TestCTStatus_AwsServiceEvent_IsCtInfo(t *testing.T) {
	ctJSON := buildFullCTEventJSON(
		"123456789012", "123456789012", "1.2.3.4", "us-east-1",
		"aws.amazon.com", "AWSService", "Management", "AwsServiceEvent", "",
	)
	status, verb := buildCTEventWithStatus(t, "st-04", "InvokeExecution", "states.amazonaws.com", "states.amazonaws.com", ctJSON, nil)
	if verb != "S" {
		t.Errorf("_ct.verb = %q, want S for AwsServiceEvent", verb)
	}
	if status != "ct-info" {
		t.Errorf("Status = %q, want ct-info for AwsServiceEvent (verb S) per §1.2", status)
	}
}

// ===========================================================================
// CT5: Insight category (verb I) → Status must be "ct-info"
// §1.2 rule: no error, no write/destroy, no root, no cross-account → ct-info
// ===========================================================================

func TestCTStatus_InsightCategory_IsCtInfo(t *testing.T) {
	ctJSON := buildFullCTEventJSON(
		"123456789012", "123456789012", "1.2.3.4", "us-east-1",
		"aws-sdk-go/1.44", "AssumedRole", "Insight", "AwsApiCall", "",
	)
	status, verb := buildCTEventWithStatus(t, "st-05", "ApiCallRateInsight", "ec2.amazonaws.com", "alice", ctJSON, nil)
	if verb != "I" {
		t.Errorf("_ct.verb = %q, want I for Insight category", verb)
	}
	if status != "ct-info" {
		t.Errorf("Status = %q, want ct-info for Insight (verb I) per §1.2", status)
	}
}

// ===========================================================================
// CT6: NetworkActivity (verb N) → Status must be "ct-info"
// §1.2 rule: no error, no write/destroy → ct-info
// ===========================================================================

func TestCTStatus_NetworkActivity_IsCtInfo(t *testing.T) {
	ctJSON := buildFullCTEventJSON(
		"123456789012", "123456789012", "10.0.0.1", "us-east-1",
		"vpc.amazonaws.com", "AssumedRole", "NetworkActivity", "AwsApiCall", "",
	)
	status, verb := buildCTEventWithStatus(t, "st-06", "VpcEndpointConnect", "ec2.amazonaws.com", "alice", ctJSON, nil)
	if verb != "N" {
		t.Errorf("_ct.verb = %q, want N for NetworkActivity category", verb)
	}
	if status != "ct-info" {
		t.Errorf("Status = %q, want ct-info for NetworkActivity (verb N) per §1.2", status)
	}
}

// ===========================================================================
// CT7: W verb with errorCode=AccessDenied → "ct-danger"
// §1.2 rule: errorCode != "" → ct-danger (highest precedence)
// ===========================================================================

func TestCTStatus_WriteWithError_IsCtDanger(t *testing.T) {
	ctJSON := buildFullCTEventJSON(
		"123456789012", "123456789012", "1.2.3.4", "us-east-1",
		"aws-cli/2.0", "AssumedRole", "Management", "AwsApiCall", "AccessDenied",
	)
	status, verb := buildCTEventWithStatus(t, "st-07", "PutBucketPolicy", "s3.amazonaws.com", "eve", ctJSON, nil)
	if verb != "W" {
		t.Errorf("_ct.verb = %q, want W for PutBucketPolicy", verb)
	}
	if status != "ct-danger" {
		t.Errorf("Status = %q, want ct-danger when errorCode=AccessDenied per §1.2 (error overrides everything)", status)
	}
}

// ===========================================================================
// CT8: D verb by Root identity → "ct-danger"
// §1.2 rule: Verb D → ct-danger (danger beats Root attention)
// ===========================================================================

func TestCTStatus_DeleteByRoot_IsCtDanger(t *testing.T) {
	rootJSON := `{"eventVersion":"1.08","userIdentity":{"type":"Root","accountId":"123456789012"},"eventTime":"2026-03-28T14:30:00Z","eventSource":"s3.amazonaws.com","eventName":"DeleteBucket","awsRegion":"us-east-1","sourceIPAddress":"1.2.3.4","userAgent":"aws-cli/2.0","errorCode":"","eventCategory":"Management","eventType":"AwsApiCall","recipientAccountId":"123456789012"}`
	status, verb := buildCTEventWithStatus(t, "st-08", "DeleteBucket", "s3.amazonaws.com", "root", rootJSON, nil)
	if verb != "D" {
		t.Errorf("_ct.verb = %q, want D for DeleteBucket", verb)
	}
	if status != "ct-danger" {
		t.Errorf("Status = %q, want ct-danger for Root delete per §1.2 (D verb wins over root)", status)
	}
}

// ===========================================================================
// CT9: R verb cross-account → "ct-attention"
// §1.2 rule: cross-account → ct-attention (escalates from ct-info)
// ===========================================================================

func TestCTStatus_ReadCrossAccount_IsCtAttention(t *testing.T) {
	ctJSON := buildFullCTEventJSON(
		"999988887777", "123456789012", "1.2.3.4", "us-east-1",
		"aws-cli/2.0", "AssumedRole", "Management", "AwsApiCall", "",
	)
	// accountId=999988887777, recipientAccountId=123456789012 → crossAccount=true
	status, verb := buildCTEventWithStatus(t, "st-09", "GetBucketAcl", "s3.amazonaws.com", "alice", ctJSON, nil)
	if verb != "R" {
		t.Errorf("_ct.verb = %q, want R for GetBucketAcl", verb)
	}
	if status != "ct-attention" {
		t.Errorf("Status = %q, want ct-attention for cross-account read per §1.2", status)
	}
}

// ===========================================================================
// CT10: Root R verb (non-destructive) → "ct-attention"
// §1.2 rule: Root identity → ct-attention
// ===========================================================================

func TestCTStatus_RootRead_IsCtAttention(t *testing.T) {
	rootJSON := `{"eventVersion":"1.08","userIdentity":{"type":"Root","accountId":"123456789012"},"eventTime":"2026-03-28T14:30:00Z","eventSource":"ec2.amazonaws.com","eventName":"DescribeInstances","awsRegion":"us-east-1","sourceIPAddress":"1.2.3.4","userAgent":"aws-cli/2.0","errorCode":"","eventCategory":"Management","eventType":"AwsApiCall","recipientAccountId":"123456789012"}`
	status, verb := buildCTEventWithStatus(t, "st-10", "DescribeInstances", "ec2.amazonaws.com", "", rootJSON, nil)
	if verb != "R" {
		t.Errorf("_ct.verb = %q, want R for DescribeInstances", verb)
	}
	if status != "ct-attention" {
		t.Errorf("Status = %q, want ct-attention for Root read per §1.2", status)
	}
}

// ===========================================================================
// CT11: Exhaustive guard — all events must have Status in {"ct-info","ct-attention","ct-danger"}
// §1.1: three semantic statuses, no others.
// ===========================================================================

func TestCTStatus_ExhaustiveGuard_ThreeValuesOnly(t *testing.T) {
	type evDef struct {
		id        string
		name      string
		category  string
		evType    string
		errorCode string
		account   string
		recipient string
		userType  string
	}
	cases := []evDef{
		// W verbs → ct-attention
		{"ex-01", "CreateBucket", "Management", "AwsApiCall", "", "123456789012", "123456789012", "AssumedRole"},
		{"ex-02", "PutObject", "Management", "AwsApiCall", "", "123456789012", "123456789012", "AssumedRole"},
		{"ex-03", "UpdateFunctionCode", "Management", "AwsApiCall", "", "123456789012", "123456789012", "AssumedRole"},
		// D verbs → ct-danger
		{"ex-04", "DeleteTable", "Management", "AwsApiCall", "", "123456789012", "123456789012", "AssumedRole"},
		{"ex-05", "TerminateInstances", "Management", "AwsApiCall", "", "123456789012", "123456789012", "AssumedRole"},
		// R verbs → ct-info
		{"ex-06", "DescribeInstances", "Management", "AwsApiCall", "", "123456789012", "123456789012", "AssumedRole"},
		{"ex-07", "GetObject", "Management", "AwsApiCall", "", "123456789012", "123456789012", "AssumedRole"},
		// W + error → ct-danger
		{"ex-08", "CreateBucket", "Management", "AwsApiCall", "AccessDenied", "123456789012", "123456789012", "AssumedRole"},
		// D + root → ct-danger (D wins)
		{"ex-09", "DeleteBucket", "Management", "AwsApiCall", "", "123456789012", "123456789012", "Root"},
		// R + cross-account → ct-attention
		{"ex-10", "DescribeInstances", "Management", "AwsApiCall", "", "999988887777", "123456789012", "AssumedRole"},
		// S verb (AwsServiceEvent) → ct-info
		{"ex-11", "InvokeExecution", "Management", "AwsServiceEvent", "", "123456789012", "123456789012", "AWSService"},
		// I verb (Insight) → ct-info
		{"ex-12", "ApiCallRateInsight", "Insight", "AwsApiCall", "", "123456789012", "123456789012", "AssumedRole"},
		// N verb (NetworkActivity) → ct-info
		{"ex-13", "VpcEndpointConnect", "NetworkActivity", "AwsApiCall", "", "123456789012", "123456789012", "AssumedRole"},
		// ? verb (unknown name) → ct-info
		{"ex-14", "SomeFutureApiCall", "Management", "AwsApiCall", "", "123456789012", "123456789012", "AssumedRole"},
	}

	var events []cloudtrailtypes.Event
	for _, c := range cases {
		ctJSON := buildFullCTEventJSON(
			c.account, c.recipient, "1.2.3.4", "us-east-1",
			"aws-cli/2.0", c.userType, c.category, c.evType, c.errorCode,
		)
		events = append(events, buildSyntheticCTEvent(c.id, c.name, "svc.amazonaws.com", "alice",
			false, time.Now(), ctJSON, nil))
	}

	result, err := awsclient.FetchCloudTrailEventsPage(context.Background(), &multiEventCTMock{events: events}, "")
	if err != nil {
		t.Fatalf("FetchCloudTrailEventsPage error: %v", err)
	}
	valid := map[string]bool{"ct-info": true, "ct-attention": true, "ct-danger": true}
	for _, r := range result.Resources {
		if !valid[r.Status] {
			t.Errorf("Resource %q (Status=%q): must be exactly ct-info / ct-attention / ct-danger per §1.1",
				r.ID, r.Status)
		}
	}
}
