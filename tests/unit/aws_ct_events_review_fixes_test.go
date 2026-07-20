package unit

// Regression tests for 4 bugs surfaced during code review of ct-events v2.
//
// Each test is labelled with the bug it locks.
//
// Bug 1 (sort uses display-formatted time string, breaking month boundaries;
// sortColKey="time" → lexicographic compare, fix: sort by event_time
// RFC3339 instead) is ported onto the live seam in list_ports_test.go's
// TestCTEventsSort_RFC3339_AcrossMonthBoundary — this file's
// ResourceListModel.SelectedResource() harness is dead code.
//
// Bug 2 (TestCTVerb_BatchDeleteAttributes_IsDestructive):
//   "Batch" is in the write-prefix table AFTER the BatchGet* short-circuit.
//   BatchDelete* therefore hits "Batch" prefix → W.  Must be D.
//
// Bug 3 (TestCTTarget_BatchGetItem_JoinsTableNames):
//   extractTargetByEventName has no case for BatchGetItem; catch-all cannot
//   handle map-valued requestItems → target falls through to "" / "(none)".
//
// Bug 4 (TestCTActor_CrossAccountRoot_HasCounterpartyPrefix):
//   computeCTActor excludes "ROOT" from the cross-account prefix branch, so
//   a cross-account root event renders "ROOT" instead of "<acct>/ROOT".
//
// Bug 5 (TestFormatCTTarget_EmptyLocalAccount_StripsAccount):
//   When localAccount=="" the condition account != localAccount is always true
//   for any ARN with a non-empty account segment → every ARN gets prefixed.

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/semantics/ctevent"
)

// ===========================================================================
// Bug 2: BatchDelete* classified as W instead of D.
//
// "Batch" appears in the write-prefix table. BatchGet* has an early short-circuit
// that returns "R", but BatchDelete* has no early exit so it falls through to the
// write table and matches "Batch" → W instead of hitting the destructive table.
// ===========================================================================

func TestCTVerb_BatchDeleteAttributes_IsDestructive(t *testing.T) {
	// Primary regression: BatchDeleteAttributes must be D (delete verb).
	got := ctevent.ClassifyCTVerb("BatchDeleteAttributes", "", "")
	if got != "D" {
		t.Errorf("ClassifyCTVerb(%q) = %q, want %q — BatchDelete* must be D, not W; "+
			"bug: \"Batch\" write-prefix matches before destructive prefix table is reached",
			"BatchDeleteAttributes", got, "D")
	}
}

func TestCTVerb_BatchDeleteImage_IsDestructive(t *testing.T) {
	got := ctevent.ClassifyCTVerb("BatchDeleteImage", "", "")
	if got != "D" {
		t.Errorf("ClassifyCTVerb(%q) = %q, want %q — BatchDeleteImage must be D",
			"BatchDeleteImage", got, "D")
	}
}

func TestCTVerb_BatchWriteItem_IsWrite_Regression(t *testing.T) {
	// Regression guard: BatchWriteItem must remain W.
	got := ctevent.ClassifyCTVerb("BatchWriteItem", "", "")
	if got != "W" {
		t.Errorf("ClassifyCTVerb(%q) = %q, want %q — BatchWriteItem regression: must stay W",
			"BatchWriteItem", got, "W")
	}
}

func TestCTVerb_BatchGetItem_IsRead_Regression(t *testing.T) {
	// Regression guard: BatchGetItem must remain R (caught by BatchGet* short-circuit).
	got := ctevent.ClassifyCTVerb("BatchGetItem", "", "")
	if got != "R" {
		t.Errorf("ClassifyCTVerb(%q) = %q, want %q — BatchGetItem regression: must stay R",
			"BatchGetItem", got, "R")
	}
}

// ===========================================================================
// Bug 3: BatchGetItem target fallback missing.
//
// extractTargetByEventName has no case for BatchGetItem. The catch-all scans
// for *Id/*Name/*Arn keys at top level, but requestItems is a map (not a
// string), so the scan yields nothing. The target becomes "" / "(none)".
// Expected: "Users,Sessions" (keys of requestItems joined).
// ===========================================================================

func TestCTTarget_BatchGetItem_JoinsTableNames(t *testing.T) {
	ctJSON := `{` +
		`"eventVersion":"1.08",` +
		`"userIdentity":{"type":"AssumedRole","accountId":"123456789012",` +
		`"sessionContext":{"sessionIssuer":{"userName":"test-role","type":"Role"}}},` +
		`"eventTime":"2026-04-07T17:00:00Z",` +
		`"eventSource":"dynamodb.amazonaws.com",` +
		`"eventName":"BatchGetItem",` +
		`"awsRegion":"us-east-1",` +
		`"sourceIPAddress":"1.2.3.4",` +
		`"userAgent":"aws-cli/2.0",` +
		`"errorCode":"",` +
		`"eventCategory":"Management",` +
		`"eventType":"AwsApiCall",` +
		`"recipientAccountId":"123456789012",` +
		`"requestParameters":{` +
		`"requestItems":{` +
		`"Users":{"Keys":[{"id":{"S":"1"}}]},` +
		`"Sessions":{"Keys":[{"id":{"S":"2"}}]}` +
		`}}}`

	event := cloudtrailtypes.Event{
		EventId:         aws.String("bgi-01"),
		EventName:       aws.String("BatchGetItem"),
		EventTime:       aws.Time(time.Date(2026, 4, 7, 17, 0, 0, 0, time.UTC)),
		EventSource:     aws.String("dynamodb.amazonaws.com"),
		Username:        aws.String("testuser"),
		ReadOnly:        aws.String("true"),
		CloudTrailEvent: aws.String(ctJSON),
		Resources:       nil,
	}

	result, err := awsclient.FetchCloudTrailEventsPage(
		context.Background(), &singleEventCTMock{event: event}, "",
	)
	if err != nil {
		t.Fatalf("FetchCloudTrailEventsPage error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}

	target := result.Resources[0].Fields["_ct.target"]

	// Map iteration order is non-deterministic; assert both table names are present.
	if !strings.Contains(target, "Users") {
		t.Errorf("_ct.target = %q, want to contain %q — BatchGetItem must extract requestItems keys; "+
			"bug: no extractTargetByEventName case for BatchGetItem, catch-all cannot handle map values",
			target, "Users")
	}
	if !strings.Contains(target, "Sessions") {
		t.Errorf("_ct.target = %q, want to contain %q — BatchGetItem must extract requestItems keys",
			target, "Sessions")
	}
	// Must not be the empty / fallback value.
	if target == "" || target == "(none)" {
		t.Errorf("_ct.target = %q, must not be empty or (none) for BatchGetItem with known requestItems",
			target)
	}
}

// ===========================================================================
// Bug 4: Cross-account ROOT actor lacks counterparty prefix.
//
// computeCTActor excludes actor == "ROOT" from the cross-account prefix branch:
//   if crossAccount && actor != "ROOT" && actor != "-" && ...
// A cross-account root event therefore renders "ROOT" instead of "999988887777/ROOT".
// ===========================================================================

func TestCTActor_CrossAccountRoot_HasCounterpartyPrefix(t *testing.T) {
	ctJSON := `{` +
		`"eventVersion":"1.08",` +
		`"userIdentity":{"type":"Root","accountId":"999988887777"},` +
		`"eventTime":"2026-04-07T17:00:00Z",` +
		`"eventSource":"s3.amazonaws.com",` +
		`"eventName":"DeleteBucket",` +
		`"awsRegion":"us-east-1",` +
		`"sourceIPAddress":"1.2.3.4",` +
		`"userAgent":"aws-cli/2.0",` +
		`"errorCode":"",` +
		`"eventCategory":"Management",` +
		`"eventType":"AwsApiCall",` +
		`"recipientAccountId":"123456789012"` +
		`}`

	event := cloudtrailtypes.Event{
		EventId:         aws.String("root-cross-01"),
		EventName:       aws.String("DeleteBucket"),
		EventTime:       aws.Time(time.Date(2026, 4, 7, 17, 0, 0, 0, time.UTC)),
		EventSource:     aws.String("s3.amazonaws.com"),
		Username:        aws.String(""),
		ReadOnly:        aws.String("false"),
		CloudTrailEvent: aws.String(ctJSON),
		Resources:       nil,
	}

	result, err := awsclient.FetchCloudTrailEventsPage(
		context.Background(), &singleEventCTMock{event: event}, "",
	)
	if err != nil {
		t.Fatalf("FetchCloudTrailEventsPage error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}

	actor := result.Resources[0].Fields["_ct.actor"]
	want := "999988887777/ROOT"
	if actor != want {
		t.Errorf("_ct.actor = %q, want %q — cross-account ROOT must carry counterparty prefix; "+
			`bug: computeCTActor has "actor != \"ROOT\"" guard that prevents the prefix`,
			actor, want)
	}
}

// ===========================================================================
// Bug 5: FormatCTTarget spurious account prefix when localAccount is empty.
//
// When localAccount=="" the condition `account != localAccount` evaluates to
// true for ANY ARN with a non-empty account segment (any non-empty string != ""),
// so every ARN gets prefixed with its own account ID. This is wrong: when the
// local account is unknown, strip the account unconditionally (no cross-account
// signal is available).
// ===========================================================================

func TestFormatCTTarget_EmptyLocalAccount_StripsAccount(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		// When localAccount is empty, strip the account segment unconditionally.
		// There is no cross-account signal to preserve — prefix would be misleading.
		{"arn:aws:iam::123456789012:role/Foo", "role/Foo"},
		{"arn:aws:lambda:us-east-1:123456789012:function:my-fn", "function:my-fn"},
		// S3 bucket ARN: no account segment, unchanged behavior.
		{"arn:aws:s3:::bucket", "bucket"},
		// Empty ARN: passthrough.
		{"", ""},
		// Non-ARN: passthrough.
		{"not-an-arn", "not-an-arn"},
	}

	for _, c := range cases {
		got := ctevent.FormatCTTarget(c.in, "")
		if got != c.want {
			t.Errorf("FormatCTTarget(%q, \"\") = %q, want %q — "+
				"when localAccount is empty, ARN account segment must be stripped (not used as cross-account prefix); "+
				`bug: account != "" is always true → every ARN with account segment gets prefixed`,
				c.in, got, c.want)
		}
	}
}
