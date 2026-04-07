package unit

// Cross-cutting invariant tests for the CloudTrail severity pipeline.
//
// These are property tests — they catch structural bugs that no single
// per-entry test can catch:
//
//  1. Every entry in the §1.3 sensitive-reads allowlist must classify as
//     verb=R. An entry that classifies as W or D is dead weight because
//     the verb-path already escalates those to ct-attention/ct-danger.
//
//  2. isSensitiveRead must use exact service-name matching, not substring
//     or fuzzy matching. Unknown services must NOT be escalated even when
//     their event name collides with a real allowlist entry.

import (
	"context"
	"testing"
	"time"

	cloudtrail "github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
)

// TestSensitiveReads_AreReadVerbs is a cross-cutting invariant: every entry
// in the §1.3 sensitive-reads allowlist must classify as verb=R via
// ClassifyCTVerb. If an entry classifies as W or D, the allowlist entry is
// redundant — verb classification already escalates W→ct-attention and
// D→ct-danger. The redundant entry adds noise without changing behavior and
// should be removed from isSensitiveRead.
//
// This test would have caught the original 4 redundant entries
// (ExportTableToPointInTime, CopyDBSnapshot, CreateSnapshot,
// ModifySnapshotAttribute) before they were flagged by code review.
//
// The allowlist here is an intentional copy of the one in
// TestCTSeverity_AllowlistEntries_AreSensitiveAttention. Keeping them
// separate ensures one test catches a bug the other cannot: a refactor
// that breaks one list won't silently break both.
func TestSensitiveReads_AreReadVerbs(t *testing.T) {
	allowlist := []struct {
		eventSource string
		eventName   string
	}{
		// Secrets / parameters
		{"secretsmanager.amazonaws.com", "GetSecretValue"},
		{"secretsmanager.amazonaws.com", "BatchGetSecretValue"},
		{"secretsmanager.amazonaws.com", "GetRandomPassword"},
		{"secretsmanager.amazonaws.com", "ListSecrets"},
		{"ssm.amazonaws.com", "GetParameter"},
		{"ssm.amazonaws.com", "GetParameters"},
		{"ssm.amazonaws.com", "GetParametersByPath"},
		{"ssm.amazonaws.com", "GetParameterHistory"},
		{"ssm.amazonaws.com", "DescribeParameters"},

		// STS session vending (NOT AssumeRole* — those are exact-match R, not on this allowlist)
		{"sts.amazonaws.com", "GetSessionToken"},
		{"sts.amazonaws.com", "GetFederationToken"},

		// Cognito admin auth
		{"cognito-idp.amazonaws.com", "AdminInitiateAuth"},
		{"cognito-idp.amazonaws.com", "AdminGetUser"},

		// Code signing
		{"signer.amazonaws.com", "GetSigningProfile"},

		// IAM credential / privilege recon
		{"iam.amazonaws.com", "GetAccessKeyLastUsed"},
		{"iam.amazonaws.com", "ListAccessKeys"},
		{"iam.amazonaws.com", "GetCredentialReport"},
		{"iam.amazonaws.com", "GenerateCredentialReport"},
		{"iam.amazonaws.com", "GetLoginProfile"},
		{"iam.amazonaws.com", "GetAccountAuthorizationDetails"},
		{"iam.amazonaws.com", "SimulatePrincipalPolicy"},
		{"iam.amazonaws.com", "SimulateCustomPolicy"},
		{"iam.amazonaws.com", "ListUsers"},
		{"iam.amazonaws.com", "ListRoles"},
		{"iam.amazonaws.com", "ListPolicies"},
		{"iam.amazonaws.com", "ListAttachedRolePolicies"},
		{"iam.amazonaws.com", "ListRolePolicies"},
		{"iam.amazonaws.com", "ListMFADevices"},
		{"iam.amazonaws.com", "ListVirtualMFADevices"},
		{"iam.amazonaws.com", "ListSSHPublicKeys"},
		{"iam.amazonaws.com", "ListServiceSpecificCredentials"},

		// Organizations enumeration
		{"organizations.amazonaws.com", "ListAccounts"},
		{"organizations.amazonaws.com", "DescribeOrganization"},

		// Bulk data exfil
		{"dynamodb.amazonaws.com", "Scan"},
		{"rds.amazonaws.com", "DownloadDBLogFilePortion"},

		// EC2 console / secret exfil
		{"ec2.amazonaws.com", "GetPasswordData"},
		{"ec2.amazonaws.com", "GetConsoleOutput"},
		{"ec2.amazonaws.com", "GetConsoleScreenshot"},

		// Account-wide recon
		{"support.amazonaws.com", "DescribeTrustedAdvisorChecks"},
		{"ce.amazonaws.com", "GetCostAndUsage"},
	}

	for _, e := range allowlist {
		e := e // capture loop variable
		t.Run(e.eventSource+"/"+e.eventName, func(t *testing.T) {
			verb := awsclient.ClassifyCTVerb(e.eventName, "", "")
			// W and D are the only redundant cases: the verb-path already
			// escalates W→ct-attention and D→ct-danger, so an allowlist entry
			// that classifies as W or D adds noise without changing behavior.
			// "?" entries are legitimate — they are the whole reason the
			// allowlist exists (to escalate ops whose names don't follow the
			// standard verb prefix convention).
			if verb == "W" || verb == "D" {
				t.Errorf(
					"%s/%s classifies as verb=%q — this allowlist entry is REDUNDANT. "+
						"Verb classification already escalates W→ct-attention and D→ct-danger. "+
						"REMOVE it from isSensitiveRead.",
					e.eventSource, e.eventName, verb,
				)
			}
		})
	}
}

// unknownServiceCTMock is a CloudTrail mock that returns a single fabricated
// event with a caller-supplied event source and name, surrounded by a
// non-escalating context (same account, IAMUser identity, no error code,
// Management category, AwsApiCall type).
type unknownServiceCTMock struct {
	event cloudtrailtypes.Event
}

func (m *unknownServiceCTMock) LookupEvents(
	_ context.Context,
	_ *cloudtrail.LookupEventsInput,
	_ ...func(*cloudtrail.Options),
) (*cloudtrail.LookupEventsOutput, error) {
	return &cloudtrail.LookupEventsOutput{Events: []cloudtrailtypes.Event{m.event}}, nil
}

// buildUnknownServiceEvent builds a plain non-escalating CT event with the
// given eventSource and eventName. It uses IAMUser identity, same account
// (no cross-account), no errorCode, Management category, AwsApiCall type —
// the minimal non-escalating context so that only isSensitiveRead can
// push the status above ct-info.
func buildUnknownServiceEvent(id, eventName, eventSource string) cloudtrailtypes.Event {
	ctJSON := `{"eventVersion":"1.08","userIdentity":{"type":"IAMUser","accountId":"123456789012"}` +
		`,"eventTime":"2026-04-07T17:00:00Z","eventSource":"` + eventSource +
		`","eventName":"` + eventName +
		`","awsRegion":"us-east-1","sourceIPAddress":"1.2.3.4","userAgent":"aws-cli/2.0"` +
		`,"errorCode":""` +
		`,"eventCategory":"Management","eventType":"AwsApiCall"` +
		`,"recipientAccountId":"123456789012"}`
	idStr := id
	nameStr := eventName
	sourceStr := eventSource
	user := "testuser"
	ts := time.Date(2026, 4, 7, 17, 0, 0, 0, time.UTC)
	return cloudtrailtypes.Event{
		EventId:     &idStr,
		EventName:   &nameStr,
		EventSource: &sourceStr,
		Username:    &user,
		EventTime:   &ts,
		CloudTrailEvent: func() *string {
			s := ctJSON
			return &s
		}(),
	}
}

// fetchUnknownServiceStatus runs buildUnknownServiceEvent through
// FetchCloudTrailEventsPage and returns the resulting Resource.Status.
func fetchUnknownServiceStatus(t *testing.T, eventSource, eventName string) string {
	t.Helper()
	event := buildUnknownServiceEvent("unk-"+eventName, eventName, eventSource)
	result, err := awsclient.FetchCloudTrailEventsPage(
		context.Background(), &unknownServiceCTMock{event: event}, "",
	)
	if err != nil {
		t.Fatalf("FetchCloudTrailEventsPage error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	return result.Resources[0].Status
}

// TestSensitiveReads_RejectsUnknownService verifies that isSensitiveRead's
// service-extraction logic uses exact-match, not substring or fuzzy match.
// A made-up event source must NOT escalate to ct-attention even when its
// event name collides with a real allowlist entry.
//
// Each case is designed so that no OTHER escalator fires:
//   - verb is R (Get*/List*/Describe*) or "?" — never W or D
//   - same account, no ROOT, no errorCode, AwsApiCall
//
// Therefore the only path to ct-attention is isSensitiveRead. If the status
// comes back ct-attention, the service-extraction logic is too loose.
func TestSensitiveReads_RejectsUnknownService(t *testing.T) {
	cases := []struct {
		eventSource string
		eventName   string
		wantStatus  string
		note        string
	}{
		// Made-up service + real allowlist eventName → must NOT match
		{"madeupservice.amazonaws.com", "GetSecretValue", "ct-info",
			"made-up service with real allowlist name"},
		{"fakeservice.amazonaws.com", "GetParameter", "ct-info",
			"made-up service with real allowlist name"},
		{"xsecreetsmanager.amazonaws.com", "GetSecretValue", "ct-info",
			"misspelled (prefix) service with real allowlist name"},

		// Real service + made-up eventName → must NOT match
		{"secretsmanager.amazonaws.com", "FrobnicateWidgets", "ct-info",
			"real service with invented event name (verb=?)"},
		{"iam.amazonaws.com", "ListNonexistentThings", "ct-info",
			"real service with invented List* name (verb=R but not in allowlist)"},

		// Empty source with real allowlist name
		{"", "GetSecretValue", "ct-info",
			"empty eventSource with real allowlist name"},
	}

	for _, c := range cases {
		c := c // capture loop variable
		t.Run(c.eventSource+"/"+c.eventName, func(t *testing.T) {
			got := fetchUnknownServiceStatus(t, c.eventSource, c.eventName)
			if got != c.wantStatus {
				t.Errorf(
					"status = %q, want %q (%s). "+
						"isSensitiveRead may be using fuzzy/substring matching instead of exact service match.",
					got, c.wantStatus, c.note,
				)
			}
		})
	}
}
