package unit

// When accountId != recipientAccountId, the ACTOR cell text is prefixed with
// the counterparty account ID using a slash separator: <accountID>/<actor>.
// computeCTActor in core/aws/ct_events.go writes it to Fields["_ct.actor"].

import (
	"context"
	"testing"
	"time"

	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
)

// buildCrossAccountCTEvent builds a cloudtrailtypes.Event whose CloudTrailEvent
// JSON has the given userIdentity.type, userIdentity.accountId,
// recipientAccountId, and (for IAMUser) userIdentity.userName.
// For AssumedRole with an ARN, the ARN is embedded in userIdentity.arn.
func buildCrossAccountCTEvent(
	id, eventName, userType, userName, arnStr, accountID, recipientAccountID string,
) cloudtrailtypes.Event {
	var userIdentityJSON string
	switch userType {
	case "Root":
		userIdentityJSON = `"type":"Root","accountId":"` + accountID + `"`
	case "IAMUser":
		userIdentityJSON = `"type":"IAMUser","accountId":"` + accountID + `","userName":"` + userName + `"`
	case "AssumedRole":
		sessionIssuer := userName // used as sessionIssuer.userName (role name part)
		userIdentityJSON = `"type":"AssumedRole","accountId":"` + accountID +
			`","arn":"` + arnStr +
			`","sessionContext":{"sessionIssuer":{"userName":"` + sessionIssuer + `","type":"Role"}}`
	default:
		userIdentityJSON = `"type":"` + userType + `","accountId":"` + accountID + `"`
	}

	ctJSON := `{"eventVersion":"1.08","userIdentity":{` + userIdentityJSON +
		`},"eventTime":"2026-04-07T17:00:00Z","eventSource":"ec2.amazonaws.com","eventName":"` + eventName +
		`","awsRegion":"us-east-1","sourceIPAddress":"1.2.3.4","userAgent":"aws-cli/2.0"` +
		`,"errorCode":"","eventCategory":"Management","eventType":"AwsApiCall"` +
		`,"recipientAccountId":"` + recipientAccountID + `"}`

	return buildSyntheticCTEvent(
		id, eventName, "ec2.amazonaws.com", userName, false,
		time.Date(2026, 4, 7, 17, 0, 0, 0, time.UTC),
		ctJSON, nil,
	)
}

func TestCTCrossAccountActor_IAMUser_CrossAccount(t *testing.T) {
	event := buildCrossAccountCTEvent(
		"ca-01", "GetObject", "IAMUser", "alice", "",
		"999988887777", "123456789012",
	)
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
	want := "999988887777/alice"
	if actor != want {
		t.Errorf("_ct.actor = %q, want %q per §1.4 (slash separator, no [cross] prefix)", actor, want)
	}
	if len(actor) >= 7 && actor[:7] == "[cross]" {
		t.Errorf("_ct.actor = %q: legacy [cross] prefix must be removed per §1.4", actor)
	}
}

func TestCTCrossAccountActor_AssumedRole_CrossAccount(t *testing.T) {
	// sessionIssuer.userName = "AdminRole", session name extracted from ARN last segment
	event := buildCrossAccountCTEvent(
		"ca-02", "AssumeRole", "AssumedRole", "AdminRole",
		"arn:aws:sts::999988887777:assumed-role/AdminRole/session-xyz",
		"999988887777", "123456789012",
	)
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
	want := "999988887777/AdminRole/session-xyz"
	if actor != want {
		t.Errorf("_ct.actor = %q, want %q per §1.4 cross-account AssumedRole format", actor, want)
	}
	if len(actor) >= 7 && actor[:7] == "[cross]" {
		t.Errorf("_ct.actor = %q: legacy [cross] prefix must be removed per §1.4", actor)
	}
}

func TestCTCrossAccountActor_SameAccount_NoPrefix(t *testing.T) {
	event := buildCrossAccountCTEvent(
		"ca-03", "GetObject", "IAMUser", "alice", "",
		"123456789012", "123456789012",
	)
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
	want := "alice"
	if actor != want {
		t.Errorf("_ct.actor = %q, want %q — no prefix for same-account per §1.4", actor, want)
	}
}

func TestCTCrossAccountActor_SameAccountRoot_Preserved(t *testing.T) {
	// A same-account ROOT carries no account prefix.
	event := buildCrossAccountCTEvent(
		"ca-04", "DescribeInstances", "Root", "", "",
		"123456789012", "123456789012",
	)
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
	want := "ROOT"
	if actor != want {
		t.Errorf("_ct.actor = %q, want %q — Root identity must be 'ROOT', not prefixed", actor, want)
	}
}
