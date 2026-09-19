package unit

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

// "Batch" is in the write-prefix table and BatchGet* short-circuits to R;
// BatchDelete* must reach the destructive table and classify as D.

func TestCTVerb_BatchDeleteAttributes_IsDestructive(t *testing.T) {
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
	got := ctevent.ClassifyCTVerb("BatchWriteItem", "", "")
	if got != "W" {
		t.Errorf("ClassifyCTVerb(%q) = %q, want %q — BatchWriteItem regression: must stay W",
			"BatchWriteItem", got, "W")
	}
}

func TestCTVerb_BatchGetItem_IsRead_Regression(t *testing.T) {
	// BatchGetItem is caught by the BatchGet* short-circuit.
	got := ctevent.ClassifyCTVerb("BatchGetItem", "", "")
	if got != "R" {
		t.Errorf("ClassifyCTVerb(%q) = %q, want %q — BatchGetItem regression: must stay R",
			"BatchGetItem", got, "R")
	}
}

// requestItems in a BatchGetItem request is a map keyed by table name, so the
// target is those keys joined: "Users,Sessions".

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
	if target == "" || target == "(none)" {
		t.Errorf("_ct.target = %q, must not be empty or (none) for BatchGetItem with known requestItems",
			target)
	}
}

// A cross-account root event carries the counterparty prefix like any other
// actor: "999988887777/ROOT".

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

// When the local account is unknown, FormatCTTarget strips the account
// segment unconditionally: there is no cross-account signal to preserve.

func TestFormatCTTarget_EmptyLocalAccount_StripsAccount(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		// When localAccount is empty, strip the account segment unconditionally.
		// There is no cross-account signal to preserve — prefix would be misleading.
		{"arn:aws:iam::123456789012:role/Foo", "role/Foo"},
		{"arn:aws:lambda:us-east-1:123456789012:function:my-fn", "function:my-fn"},
		// S3 bucket ARN: no account segment.
		{"arn:aws:s3:::bucket", "bucket"},
		{"", ""},
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
