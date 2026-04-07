package unit

// Tests for §1.4 cross-account actor format.
//
// §1.4: When accountId != recipientAccountId, the ACTOR cell text is prefixed
// with the counterparty account ID using slash separator: <accountID>/<actor>.
// The legacy "[cross] " literal prefix is removed.
//
// These tests assert Resource.Fields["_ct.actor"] values after FetchCloudTrailEventsPage.
// They are expected to FAIL until the P1 coder updates computeCTActor in
// internal/aws/ct_events.go (currently uses "[cross] " prefix at line ~369).

import (
	"context"
	"testing"
	"time"

	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
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

// ===========================================================================
// CT-CA1: IAMUser cross-account — actor must be "999988887777/alice"
// §1.4: <accountID>/<actor>, no "[cross] " prefix.
// ===========================================================================

func TestCTCrossAccountActor_IAMUser_CrossAccount(t *testing.T) {
	// Spec: §1.4 — cross-account actor format is "999988887777/alice", not "[cross] alice"
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
	// Verify the legacy "[cross] " prefix is NOT present.
	if len(actor) >= 7 && actor[:7] == "[cross]" {
		t.Errorf("_ct.actor = %q: legacy [cross] prefix must be removed per §1.4", actor)
	}
}

// ===========================================================================
// CT-CA2: AssumedRole cross-account — actor must be "999988887777/AdminRole/session-xyz"
// §1.4: accountID/roleName/sessionName format for AssumedRole.
// ===========================================================================

func TestCTCrossAccountActor_AssumedRole_CrossAccount(t *testing.T) {
	// Spec: §1.4 — cross-account AssumedRole format: "999988887777/AdminRole/session-xyz"
	// ARN: arn:aws:sts::999988887777:assumed-role/AdminRole/session-xyz
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
	// Verify legacy "[cross] " prefix is NOT present.
	if len(actor) >= 7 && actor[:7] == "[cross]" {
		t.Errorf("_ct.actor = %q: legacy [cross] prefix must be removed per §1.4", actor)
	}
}

// ===========================================================================
// CT-CA3: Same-account IAMUser — actor must be "alice" (no prefix at all).
// §1.4: prefix only when cross-account.
// ===========================================================================

func TestCTCrossAccountActor_SameAccount_NoPrefix(t *testing.T) {
	// Spec: §1.4 — no prefix when accountId == recipientAccountId
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

// ===========================================================================
// CT-CA4: Root identity same-account — actor must be "ROOT" (unchanged by §1.4).
// §1.4 exempts ROOT from the cross-account prefix (Root has no account prefix).
// ===========================================================================

func TestCTCrossAccountActor_SameAccountRoot_Preserved(t *testing.T) {
	// Spec: §1.4 — Root format is "ROOT", unchanged (existing code already exempts ROOT)
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
