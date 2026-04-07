package unit

// Tests for Bug P1: Resource.Status must be verb-based ("ct-write"/"ct-read").
//
// The broken code in internal/aws/ct_events.go:228-244 uses a priority ladder
// based on identity/error/cross-account/eventType, producing "ct-root", "error",
// "pending", "terminated", or "running" — all wrong.
//
// Design spec (docs/design/ct-event-list.md §4, specs/012-ct-events-list-redesign/spec.md FR-007):
//   - Status must be exactly "ct-write" (verbs W, D) or "ct-read" (verbs R, S, I, N, ?)
//   - Errors, root identity, cross-account, and service events are signalled at the
//     CELL level (OUTCOME / ACTOR / EVENT columns), NOT via Resource.Status
//
// Every test below currently FAILs against HEAD because Status is "running"/"error"/etc.

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
		"111122223333", "111122223333", "1.2.3.4", "us-east-1",
		"aws-cli/2.0", "AssumedRole", eventCategory, eventType, "",
	)
}

// ===========================================================================
// CT1: CreateBucket (verb W) → Status must be "ct-write"
// Even when no error, plain account — NOT "running"
// ===========================================================================

func TestCTStatus_CreateBucket_IsCtWrite(t *testing.T) {
	ctJSON := plainAccountJSON("CreateBucket", "Management", "AwsApiCall")
	status, verb := buildCTEventWithStatus(t, "st-01", "CreateBucket", "s3.amazonaws.com", "alice", ctJSON, nil)
	if verb != "W" {
		t.Errorf("_ct.verb = %q, want W for CreateBucket", verb)
	}
	if status != "ct-write" {
		// Bug: current code returns "running"
		t.Errorf("Status = %q, want ct-write for verb W (CreateBucket); bug in ct_events.go:228-244 priority ladder", status)
	}
}

// ===========================================================================
// CT2: DeleteTable (verb D) → Status must be "ct-write"
// ===========================================================================

func TestCTStatus_DeleteTable_IsCtWrite(t *testing.T) {
	ctJSON := plainAccountJSON("DeleteTable", "Management", "AwsApiCall")
	status, verb := buildCTEventWithStatus(t, "st-02", "DeleteTable", "dynamodb.amazonaws.com", "bob", ctJSON, nil)
	if verb != "D" {
		t.Errorf("_ct.verb = %q, want D for DeleteTable", verb)
	}
	if status != "ct-write" {
		t.Errorf("Status = %q, want ct-write for verb D (DeleteTable); bug in ct_events.go:228-244 priority ladder", status)
	}
}

// ===========================================================================
// CT3: DescribeInstances (verb R) → Status must be "ct-read"
// ===========================================================================

func TestCTStatus_DescribeInstances_IsCtRead(t *testing.T) {
	ctJSON := plainAccountJSON("DescribeInstances", "Management", "AwsApiCall")
	status, verb := buildCTEventWithStatus(t, "st-03", "DescribeInstances", "ec2.amazonaws.com", "carol", ctJSON, nil)
	if verb != "R" {
		t.Errorf("_ct.verb = %q, want R for DescribeInstances", verb)
	}
	if status != "ct-read" {
		t.Errorf("Status = %q, want ct-read for verb R (DescribeInstances); bug in ct_events.go:228-244 priority ladder", status)
	}
}

// ===========================================================================
// CT4: AwsServiceEvent (eventType=AwsServiceEvent, verb S) → Status must be "ct-read"
// The broken priority ladder maps AwsServiceEvent → "terminated"; spec says "ct-read"
// ===========================================================================

func TestCTStatus_AwsServiceEvent_IsCtRead(t *testing.T) {
	ctJSON := buildFullCTEventJSON(
		"111122223333", "111122223333", "1.2.3.4", "us-east-1",
		"aws.amazon.com", "AWSService", "Management", "AwsServiceEvent", "",
	)
	status, verb := buildCTEventWithStatus(t, "st-04", "InvokeExecution", "states.amazonaws.com", "states.amazonaws.com", ctJSON, nil)
	// verb S must map to ct-read, not "terminated"
	if verb != "S" {
		t.Errorf("_ct.verb = %q, want S for AwsServiceEvent", verb)
	}
	if status != "ct-read" {
		// Bug: current code hits case eventType == "AwsServiceEvent": status = "terminated"
		t.Errorf("Status = %q, want ct-read for AwsServiceEvent (verb S); ct_events.go:242-244 sets 'terminated' — wrong", status)
	}
}

// ===========================================================================
// CT5: Insight category (verb I) → Status must be "ct-read"
// ===========================================================================

func TestCTStatus_InsightCategory_IsCtRead(t *testing.T) {
	ctJSON := buildFullCTEventJSON(
		"111122223333", "111122223333", "1.2.3.4", "us-east-1",
		"aws-sdk-go/1.44", "AssumedRole", "Insight", "AwsApiCall", "",
	)
	status, verb := buildCTEventWithStatus(t, "st-05", "DescribeInstances", "ec2.amazonaws.com", "alice", ctJSON, nil)
	if verb != "I" {
		t.Errorf("_ct.verb = %q, want I for Insight category", verb)
	}
	if status != "ct-read" {
		t.Errorf("Status = %q, want ct-read for Insight (verb I)", status)
	}
}

// ===========================================================================
// CT6: NetworkActivity (verb N) → Status must be "ct-read"
// ===========================================================================

func TestCTStatus_NetworkActivity_IsCtRead(t *testing.T) {
	ctJSON := buildFullCTEventJSON(
		"111122223333", "111122223333", "10.0.0.1", "us-east-1",
		"vpc.amazonaws.com", "AssumedRole", "NetworkActivity", "AwsApiCall", "",
	)
	status, verb := buildCTEventWithStatus(t, "st-06", "VpcEndpointConnect", "ec2.amazonaws.com", "alice", ctJSON, nil)
	if verb != "N" {
		t.Errorf("_ct.verb = %q, want N for NetworkActivity category", verb)
	}
	if status != "ct-read" {
		t.Errorf("Status = %q, want ct-read for NetworkActivity (verb N)", status)
	}
}

// ===========================================================================
// CT7: W verb with errorCode=AccessDenied → STILL "ct-write"
// Error must be signalled by OUTCOME cell, NOT by changing Status to "error"
// ===========================================================================

func TestCTStatus_WriteWithError_IsStillCtWrite(t *testing.T) {
	ctJSON := buildFullCTEventJSON(
		"111122223333", "111122223333", "1.2.3.4", "us-east-1",
		"aws-cli/2.0", "AssumedRole", "Management", "AwsApiCall", "AccessDenied",
	)
	status, verb := buildCTEventWithStatus(t, "st-07", "PutBucketPolicy", "s3.amazonaws.com", "eve", ctJSON, nil)
	if verb != "W" {
		t.Errorf("_ct.verb = %q, want W for PutBucketPolicy", verb)
	}
	if status != "ct-write" {
		// Bug: current code hits case errorCode != "": status = "error"
		t.Errorf("Status = %q, want ct-write even when errorCode=AccessDenied; ct_events.go:238-240 sets 'error' — wrong", status)
	}
}

// ===========================================================================
// CT8: D verb by Root identity → STILL "ct-write"
// Root must be signalled by ACTOR cell ("ROOT"), NOT by changing Status to "ct-root"
// ===========================================================================

func TestCTStatus_DeleteByRoot_IsStillCtWrite(t *testing.T) {
	rootJSON := `{"eventVersion":"1.08","userIdentity":{"type":"Root","accountId":"111122223333"},"eventTime":"2026-03-28T14:30:00Z","eventSource":"s3.amazonaws.com","eventName":"DeleteBucket","awsRegion":"us-east-1","sourceIPAddress":"1.2.3.4","userAgent":"aws-cli/2.0","errorCode":"","eventCategory":"Management","eventType":"AwsApiCall","recipientAccountId":"111122223333"}`
	status, verb := buildCTEventWithStatus(t, "st-08", "DeleteBucket", "s3.amazonaws.com", "root", rootJSON, nil)
	if verb != "D" {
		t.Errorf("_ct.verb = %q, want D for DeleteBucket", verb)
	}
	if status != "ct-write" {
		// Bug: current code hits case isRoot == "true": status = "ct-root"
		t.Errorf("Status = %q, want ct-write for Root delete; ct_events.go:236-237 sets 'ct-root' — wrong per FR-007", status)
	}
}

// ===========================================================================
// CT9: R verb cross-account → STILL "ct-read"
// Cross-account must be signalled by ACTOR "[cross] " prefix, NOT Status → "pending"
// ===========================================================================

func TestCTStatus_ReadCrossAccount_IsStillCtRead(t *testing.T) {
	ctJSON := buildFullCTEventJSON(
		"111122223333", "999988887777", "1.2.3.4", "us-east-1",
		"aws-cli/2.0", "AssumedRole", "Management", "AwsApiCall", "",
	)
	// accountId=111122223333, recipientAccountId=999988887777 → crossAccount=true
	status, verb := buildCTEventWithStatus(t, "st-09", "GetBucketAcl", "s3.amazonaws.com", "alice", ctJSON, nil)
	if verb != "R" {
		t.Errorf("_ct.verb = %q, want R for GetBucketAcl", verb)
	}
	if status != "ct-read" {
		// Bug: current code hits case crossAccount == "true": status = "pending"
		t.Errorf("Status = %q, want ct-read for cross-account read; ct_events.go:240-241 sets 'pending' — wrong per FR-007", status)
	}
}

// ===========================================================================
// CT10: Exhaustive guard — all events must have Status exactly "ct-write" or "ct-read"
// Mirrors the pattern in aws_ct_events_redesign_test.go:535-566.
// This slice includes the error/root/cross-account/service cases that the broken
// code classifies differently.
// ===========================================================================

func TestCTStatus_ExhaustiveGuard_OnlyTwoValues(t *testing.T) {
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
		// W verbs
		{"ex-01", "CreateBucket", "Management", "AwsApiCall", "", "111122223333", "111122223333", "AssumedRole"},
		{"ex-02", "PutObject", "Management", "AwsApiCall", "", "111122223333", "111122223333", "AssumedRole"},
		{"ex-03", "UpdateFunctionCode", "Management", "AwsApiCall", "", "111122223333", "111122223333", "AssumedRole"},
		// D verbs
		{"ex-04", "DeleteTable", "Management", "AwsApiCall", "", "111122223333", "111122223333", "AssumedRole"},
		{"ex-05", "TerminateInstances", "Management", "AwsApiCall", "", "111122223333", "111122223333", "AssumedRole"},
		// R verbs
		{"ex-06", "DescribeInstances", "Management", "AwsApiCall", "", "111122223333", "111122223333", "AssumedRole"},
		{"ex-07", "GetObject", "Management", "AwsApiCall", "", "111122223333", "111122223333", "AssumedRole"},
		// W + error → must still be ct-write
		{"ex-08", "CreateBucket", "Management", "AwsApiCall", "AccessDenied", "111122223333", "111122223333", "AssumedRole"},
		// D + root → must still be ct-write
		{"ex-09", "DeleteBucket", "Management", "AwsApiCall", "", "111122223333", "111122223333", "Root"},
		// R + cross-account → must still be ct-read
		{"ex-10", "DescribeInstances", "Management", "AwsApiCall", "", "111122223333", "999988887777", "AssumedRole"},
		// S verb (AwsServiceEvent) → ct-read
		{"ex-11", "InvokeExecution", "Management", "AwsServiceEvent", "", "111122223333", "111122223333", "AWSService"},
		// I verb (Insight)
		{"ex-12", "DescribeInstances", "Insight", "AwsApiCall", "", "111122223333", "111122223333", "AssumedRole"},
		// N verb (NetworkActivity)
		{"ex-13", "VpcEndpointConnect", "NetworkActivity", "AwsApiCall", "", "111122223333", "111122223333", "AssumedRole"},
		// ? verb (unknown name)
		{"ex-14", "SomeFutureApiCall", "Management", "AwsApiCall", "", "111122223333", "111122223333", "AssumedRole"},
	}

	var events []cloudtrailtypes.Event
	for _, c := range cases {
		userTypeForJSON := c.userType
		if userTypeForJSON == "Root" {
			// root JSON has no sessionContext; reuse buildFullCTEventJSON but override the userType
			userTypeForJSON = "Root"
		}
		ctJSON := buildFullCTEventJSON(
			c.account, c.recipient, "1.2.3.4", "us-east-1",
			"aws-cli/2.0", userTypeForJSON, c.category, c.evType, c.errorCode,
		)
		events = append(events, buildSyntheticCTEvent(c.id, c.name, "svc.amazonaws.com", "alice",
			false, time.Now(), ctJSON, nil))
	}

	result, err := awsclient.FetchCloudTrailEventsPage(context.Background(), &multiEventCTMock{events: events}, "")
	if err != nil {
		t.Fatalf("FetchCloudTrailEventsPage error: %v", err)
	}
	for _, r := range result.Resources {
		if r.Status != "ct-write" && r.Status != "ct-read" {
			t.Errorf("Resource %q (Status=%q): must be exactly 'ct-write' or 'ct-read', got %q — bug in ct_events.go:228-244 priority ladder",
				r.ID, r.Status, r.Status)
		}
	}
}

