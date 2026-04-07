package unit

// Tests for the §1.2 severity ladder.
//
// Each test builds a cloudtrailtypes.Event with a hand-crafted CloudTrailEvent
// JSON string, fetches it via FetchCloudTrailEventsPage, and asserts
// Resource.Status matches the expected value from §1.2.
//
// These tests are expected to FAIL until the P1 coder rewrites buildCTResource
// in internal/aws/ct_events.go to implement the three-tier severity model.

import (
	"context"
	"testing"
	"time"

	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
)

// buildSeverityCTEvent constructs a cloudtrailtypes.Event whose CloudTrailEvent
// JSON carries exactly the fields required by the severity ladder tests.
// accountID is userIdentity.accountId; recipientAccountID is the top-level
// recipientAccountId. userType sets userIdentity.type.
func buildSeverityCTEvent(
	id, eventName, eventSource string,
	accountID, recipientAccountID, userType, eventCategory, eventType, errorCode string,
) cloudtrailtypes.Event {
	sessionCtx := ""
	if userType == "AssumedRole" || userType == "Role" {
		sessionCtx = `,"sessionContext":{"sessionIssuer":{"userName":"test-role","type":"Role"}}`
	}
	ctJSON := `{"eventVersion":"1.08","userIdentity":{"type":"` + userType +
		`","accountId":"` + accountID + `"` + sessionCtx +
		`},"eventTime":"2026-04-07T17:00:00Z","eventSource":"` + eventSource +
		`","eventName":"` + eventName +
		`","awsRegion":"us-east-1","sourceIPAddress":"1.2.3.4","userAgent":"aws-cli/2.0"` +
		`,"errorCode":"` + errorCode +
		`","eventCategory":"` + eventCategory +
		`","eventType":"` + eventType +
		`","recipientAccountId":"` + recipientAccountID + `"}`
	return buildSyntheticCTEvent(
		id, eventName, eventSource, "testuser", false,
		time.Date(2026, 4, 7, 17, 0, 0, 0, time.UTC),
		ctJSON, nil,
	)
}

// fetchOneStatus is a test helper that calls FetchCloudTrailEventsPage with a
// single synthetic event and returns the Resource.Status of the first result.
func fetchOneStatus(t *testing.T, event cloudtrailtypes.Event) string {
	t.Helper()
	result, err := awsclient.FetchCloudTrailEventsPage(
		context.Background(), &singleEventCTMock{event: event}, "",
	)
	if err != nil {
		t.Fatalf("FetchCloudTrailEventsPage error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	return result.Resources[0].Status
}

// ===========================================================================
// Severity table tests — one per §1.2 ladder row.
// ===========================================================================

// TestCTSeverity_ErrorOverridesReadVerb — §1.2 row 1: errorCode → ct-danger wins.
// errorCode=AccessDenied on a plain GetObject (verb R) must still be ct-danger.
func TestCTSeverity_ErrorOverridesReadVerb(t *testing.T) {
	// Spec: §1.2 — errorCode != "" → ct-danger (overrides everything else)
	event := buildSeverityCTEvent(
		"sev-01", "GetObject", "s3.amazonaws.com",
		"123456789012", "123456789012", "AssumedRole", "Management", "AwsApiCall", "AccessDenied",
	)
	got := fetchOneStatus(t, event)
	if got != "ct-danger" {
		t.Errorf("Status = %q, want ct-danger — errorCode=AccessDenied on R verb must escalate to ct-danger per §1.2", got)
	}
}

// TestCTSeverity_ErrorOverridesWriteVerb — §1.2 row 1: errorCode beats write verb.
func TestCTSeverity_ErrorOverridesWriteVerb(t *testing.T) {
	// Spec: §1.2 — errorCode != "" → ct-danger (overrides everything else)
	event := buildSeverityCTEvent(
		"sev-02", "CreateBucket", "s3.amazonaws.com",
		"123456789012", "123456789012", "AssumedRole", "Management", "AwsApiCall", "AccessDenied",
	)
	got := fetchOneStatus(t, event)
	if got != "ct-danger" {
		t.Errorf("Status = %q, want ct-danger — errorCode=AccessDenied on W verb per §1.2", got)
	}
}

// TestCTSeverity_DestructiveVerbNoError — §1.2 row 1: verb D, no error → ct-danger.
func TestCTSeverity_DestructiveVerbNoError(t *testing.T) {
	// Spec: §1.2 — Verb is D → ct-danger
	event := buildSeverityCTEvent(
		"sev-03", "TerminateInstances", "ec2.amazonaws.com",
		"123456789012", "123456789012", "AssumedRole", "Management", "AwsApiCall", "",
	)
	got := fetchOneStatus(t, event)
	if got != "ct-danger" {
		t.Errorf("Status = %q, want ct-danger — TerminateInstances (verb D) per §1.2", got)
	}
}

// TestCTSeverity_DestructiveVerbWithError — §1.2: D verb + error still ct-danger.
func TestCTSeverity_DestructiveVerbWithError(t *testing.T) {
	// Spec: §1.2 — errorCode != "" → ct-danger (and verb D is also ct-danger; highest wins)
	event := buildSeverityCTEvent(
		"sev-04", "DeleteBucket", "s3.amazonaws.com",
		"123456789012", "123456789012", "AssumedRole", "Management", "AwsApiCall", "AccessDenied",
	)
	got := fetchOneStatus(t, event)
	if got != "ct-danger" {
		t.Errorf("Status = %q, want ct-danger — DeleteBucket with AccessDenied per §1.2", got)
	}
}

// TestCTSeverity_RootReadIsAttention — §1.2 row 2: Root identity on a read verb → ct-attention.
func TestCTSeverity_RootReadIsAttention(t *testing.T) {
	// Spec: §1.2 — userIdentity.type == "Root" → ct-attention (no error, no D verb)
	rootJSON := `{"eventVersion":"1.08","userIdentity":{"type":"Root","accountId":"123456789012"}` +
		`,"eventTime":"2026-04-07T17:00:00Z","eventSource":"ec2.amazonaws.com","eventName":"DescribeInstances"` +
		`,"awsRegion":"us-east-1","sourceIPAddress":"1.2.3.4","userAgent":"aws-cli/2.0"` +
		`,"errorCode":"","eventCategory":"Management","eventType":"AwsApiCall"` +
		`,"recipientAccountId":"123456789012"}`
	event := buildSyntheticCTEvent(
		"sev-05", "DescribeInstances", "ec2.amazonaws.com", "", false,
		time.Date(2026, 4, 7, 17, 0, 0, 0, time.UTC), rootJSON, nil,
	)
	got := fetchOneStatus(t, event)
	if got != "ct-attention" {
		t.Errorf("Status = %q, want ct-attention — Root identity on R verb per §1.2", got)
	}
}

// TestCTSeverity_RootDestructiveIsDanger — §1.2: D verb wins over Root.
// Root identity on a destructive event → ct-danger (danger wins over attention).
func TestCTSeverity_RootDestructiveIsDanger(t *testing.T) {
	// Spec: §1.2 — Verb D → ct-danger; Root only escalates to ct-attention, D is higher
	rootJSON := `{"eventVersion":"1.08","userIdentity":{"type":"Root","accountId":"123456789012"}` +
		`,"eventTime":"2026-04-07T17:00:00Z","eventSource":"s3.amazonaws.com","eventName":"DeleteBucket"` +
		`,"awsRegion":"us-east-1","sourceIPAddress":"1.2.3.4","userAgent":"aws-cli/2.0"` +
		`,"errorCode":"","eventCategory":"Management","eventType":"AwsApiCall"` +
		`,"recipientAccountId":"123456789012"}`
	event := buildSyntheticCTEvent(
		"sev-06", "DeleteBucket", "s3.amazonaws.com", "root", false,
		time.Date(2026, 4, 7, 17, 0, 0, 0, time.UTC), rootJSON, nil,
	)
	got := fetchOneStatus(t, event)
	if got != "ct-danger" {
		t.Errorf("Status = %q, want ct-danger — Root + verb D, danger wins per §1.2", got)
	}
}

// TestCTSeverity_WriteVerbIsAttention — §1.2 row 2: verb W, no special conditions → ct-attention.
func TestCTSeverity_WriteVerbIsAttention(t *testing.T) {
	// Spec: §1.2 — Verb is W → ct-attention
	event := buildSeverityCTEvent(
		"sev-07", "CreateBucket", "s3.amazonaws.com",
		"123456789012", "123456789012", "AssumedRole", "Management", "AwsApiCall", "",
	)
	got := fetchOneStatus(t, event)
	if got != "ct-attention" {
		t.Errorf("Status = %q, want ct-attention — CreateBucket (verb W) per §1.2", got)
	}
}

// TestCTSeverity_SensitiveReadSecretsManager — §1.2 row 2: sensitive-read → ct-attention.
// secretsmanager:GetSecretValue is on the sensitive-reads allowlist in §1.3.
func TestCTSeverity_SensitiveReadSecretsManager(t *testing.T) {
	// Spec: §1.2 + §1.3 — secretsmanager:GetSecretValue → ct-attention even though verb is R
	event := buildSeverityCTEvent(
		"sev-08", "GetSecretValue", "secretsmanager.amazonaws.com",
		"123456789012", "123456789012", "AssumedRole", "Management", "AwsApiCall", "",
	)
	got := fetchOneStatus(t, event)
	if got != "ct-attention" {
		t.Errorf("Status = %q, want ct-attention — secretsmanager:GetSecretValue is on §1.3 sensitive-reads allowlist", got)
	}
}

// TestCTSeverity_SensitiveReadSSMGetParameter — §1.3: ssm:GetParameter → ct-attention.
func TestCTSeverity_SensitiveReadSSMGetParameter(t *testing.T) {
	// Spec: §1.3 sensitive-reads allowlist: ssm:GetParameter
	event := buildSeverityCTEvent(
		"sev-09", "GetParameter", "ssm.amazonaws.com",
		"123456789012", "123456789012", "AssumedRole", "Management", "AwsApiCall", "",
	)
	got := fetchOneStatus(t, event)
	if got != "ct-attention" {
		t.Errorf("Status = %q, want ct-attention — ssm:GetParameter is on §1.3 sensitive-reads allowlist", got)
	}
}

// TestCTSeverity_SensitiveReadSTSAssumeRole — §1.3: sts:AssumeRole → ct-attention.
func TestCTSeverity_SensitiveReadSTSAssumeRole(t *testing.T) {
	// Spec: §1.3 sensitive-reads allowlist: sts:AssumeRole
	// Note: ClassifyCTVerb currently classifies AssumeRole as W (Assume prefix).
	// Under the v2 spec §2.1 the Assume prefix stays in W, but §1.3 independently
	// escalates sts:AssumeRole to ct-attention regardless of verb.
	// Since W already yields ct-attention, this test verifies the combined outcome.
	event := buildSeverityCTEvent(
		"sev-10", "AssumeRole", "sts.amazonaws.com",
		"123456789012", "123456789012", "AssumedRole", "Management", "AwsApiCall", "",
	)
	got := fetchOneStatus(t, event)
	if got != "ct-attention" {
		t.Errorf("Status = %q, want ct-attention — sts:AssumeRole per §1.3 sensitive-reads (verb W also yields ct-attention)", got)
	}
}

// TestCTSeverity_CrossAccountReadIsAttention — §1.2 row 2: cross-account read → ct-attention.
// accountId=999988887777, recipientAccountId=123456789012.
func TestCTSeverity_CrossAccountReadIsAttention(t *testing.T) {
	// Spec: §1.2 — cross-account (accountId != recipientAccountId) → ct-attention
	event := buildSeverityCTEvent(
		"sev-11", "GetObject", "s3.amazonaws.com",
		"999988887777", "123456789012", "IAMUser", "Management", "AwsApiCall", "",
	)
	got := fetchOneStatus(t, event)
	if got != "ct-attention" {
		t.Errorf("Status = %q, want ct-attention — cross-account read (accountId != recipientAccountId) per §1.2", got)
	}
}

// TestCTSeverity_PlainReadIsInfo — §1.2 row 3: plain read, no special conditions → ct-info.
// No error, no D/W verb, not Root, not cross-account, not sensitive-read.
func TestCTSeverity_PlainReadIsInfo(t *testing.T) {
	// Spec: §1.2 — ct-info otherwise (plain R verb, same account, not root)
	event := buildSeverityCTEvent(
		"sev-12", "GetObject", "s3.amazonaws.com",
		"123456789012", "123456789012", "AssumedRole", "Management", "AwsApiCall", "",
	)
	got := fetchOneStatus(t, event)
	if got != "ct-info" {
		t.Errorf("Status = %q, want ct-info — plain GetObject (R verb, same acct, no error) per §1.2", got)
	}
}

// TestCTSeverity_PlainDescribeIsInfo — §1.2 row 3: plain describe → ct-info.
func TestCTSeverity_PlainDescribeIsInfo(t *testing.T) {
	// Spec: §1.2 — ct-info otherwise
	event := buildSeverityCTEvent(
		"sev-13", "DescribeInstances", "ec2.amazonaws.com",
		"123456789012", "123456789012", "AssumedRole", "Management", "AwsApiCall", "",
	)
	got := fetchOneStatus(t, event)
	if got != "ct-info" {
		t.Errorf("Status = %q, want ct-info — plain DescribeInstances (R verb) per §1.2", got)
	}
}

// TestCTSeverity_InsightCategoryIsInfo — §1.2 row 3: Insight category → ct-info.
func TestCTSeverity_InsightCategoryIsInfo(t *testing.T) {
	// Spec: §1.2 — ct-info otherwise; Insight events have verb I (not W/D), no error
	event := buildSeverityCTEvent(
		"sev-14", "ApiCallRateInsight", "cloudtrail.amazonaws.com",
		"123456789012", "123456789012", "AssumedRole", "Insight", "AwsApiCall", "",
	)
	got := fetchOneStatus(t, event)
	if got != "ct-info" {
		t.Errorf("Status = %q, want ct-info — Insight category (verb I) per §1.2", got)
	}
}

// TestCTSeverity_AwsServiceEventIsInfo — §1.2 row 3: AwsServiceEvent → ct-info.
func TestCTSeverity_AwsServiceEventIsInfo(t *testing.T) {
	// Spec: §1.2 — ct-info otherwise; AwsServiceEvent has verb S, no error
	event := buildSeverityCTEvent(
		"sev-15", "InvokeExecution", "states.amazonaws.com",
		"123456789012", "123456789012", "AWSService", "Management", "AwsServiceEvent", "",
	)
	got := fetchOneStatus(t, event)
	if got != "ct-info" {
		t.Errorf("Status = %q, want ct-info — AwsServiceEvent (verb S) per §1.2", got)
	}
}

// ===========================================================================
// §1.3 sensitive-reads allowlist — exhaustive coverage of all 16 entries.
//
// Each test sends a read verb event that would otherwise be ct-info, but is
// escalated to ct-attention because it appears in the sensitive-reads allowlist.
// Test IDs: sr-01..sr-16, matching the order in the allowlist.
// ===========================================================================

// TestCTSeverity_SensitiveRead_BatchGetSecretValue — §1.3 entry 2.
func TestCTSeverity_SensitiveRead_BatchGetSecretValue(t *testing.T) {
	// secretsmanager:BatchGetSecretValue → ct-attention (sensitive-read; verb is R from BatchGet prefix)
	event := buildSeverityCTEvent(
		"sr-02", "BatchGetSecretValue", "secretsmanager.amazonaws.com",
		"123456789012", "123456789012", "AssumedRole", "Management", "AwsApiCall", "",
	)
	got := fetchOneStatus(t, event)
	if got != "ct-attention" {
		t.Errorf("Status = %q, want ct-attention — secretsmanager:BatchGetSecretValue is §1.3 entry 2", got)
	}
}

// TestCTSeverity_SensitiveRead_GetParameters — §1.3 entry 4.
func TestCTSeverity_SensitiveRead_GetParameters(t *testing.T) {
	// ssm:GetParameters → ct-attention (sensitive-read; verb is R)
	event := buildSeverityCTEvent(
		"sr-04", "GetParameters", "ssm.amazonaws.com",
		"123456789012", "123456789012", "AssumedRole", "Management", "AwsApiCall", "",
	)
	got := fetchOneStatus(t, event)
	if got != "ct-attention" {
		t.Errorf("Status = %q, want ct-attention — ssm:GetParameters is §1.3 entry 4", got)
	}
}

// TestCTSeverity_SensitiveRead_GetParametersByPath — §1.3 entry 5.
func TestCTSeverity_SensitiveRead_GetParametersByPath(t *testing.T) {
	// ssm:GetParametersByPath → ct-attention (sensitive-read; verb is R)
	event := buildSeverityCTEvent(
		"sr-05", "GetParametersByPath", "ssm.amazonaws.com",
		"123456789012", "123456789012", "AssumedRole", "Management", "AwsApiCall", "",
	)
	got := fetchOneStatus(t, event)
	if got != "ct-attention" {
		t.Errorf("Status = %q, want ct-attention — ssm:GetParametersByPath is §1.3 entry 5", got)
	}
}

// TestCTSeverity_SensitiveRead_GetSessionToken — §1.3 entry 6.
func TestCTSeverity_SensitiveRead_GetSessionToken(t *testing.T) {
	// sts:GetSessionToken → ct-attention (sensitive-read; verb is R from Get prefix)
	event := buildSeverityCTEvent(
		"sr-06", "GetSessionToken", "sts.amazonaws.com",
		"123456789012", "123456789012", "AssumedRole", "Management", "AwsApiCall", "",
	)
	got := fetchOneStatus(t, event)
	if got != "ct-attention" {
		t.Errorf("Status = %q, want ct-attention — sts:GetSessionToken is §1.3 entry 6", got)
	}
}

// TestCTSeverity_SensitiveRead_GetFederationToken — §1.3 entry 7.
func TestCTSeverity_SensitiveRead_GetFederationToken(t *testing.T) {
	// sts:GetFederationToken → ct-attention (sensitive-read; verb is R from Get prefix)
	event := buildSeverityCTEvent(
		"sr-07", "GetFederationToken", "sts.amazonaws.com",
		"123456789012", "123456789012", "AssumedRole", "Management", "AwsApiCall", "",
	)
	got := fetchOneStatus(t, event)
	if got != "ct-attention" {
		t.Errorf("Status = %q, want ct-attention — sts:GetFederationToken is §1.3 entry 7", got)
	}
}

// TestCTSeverity_SensitiveRead_AssumeRoleWithSAML — §1.3 entry 9.
// Note: AssumeRoleWithSAML has verb W (Assume* prefix), which already yields
// ct-attention. The sensitive-read list independently escalates it too; this
// test verifies the final outcome is ct-attention.
func TestCTSeverity_SensitiveRead_AssumeRoleWithSAML(t *testing.T) {
	// sts:AssumeRoleWithSAML → ct-attention (both §1.2 verb W and §1.3 sensitive-read)
	event := buildSeverityCTEvent(
		"sr-09", "AssumeRoleWithSAML", "sts.amazonaws.com",
		"123456789012", "123456789012", "AssumedRole", "Management", "AwsApiCall", "",
	)
	got := fetchOneStatus(t, event)
	if got != "ct-attention" {
		t.Errorf("Status = %q, want ct-attention — sts:AssumeRoleWithSAML is §1.3 entry 9", got)
	}
}

// TestCTSeverity_SensitiveRead_AssumeRoleWithWebIdentity — §1.3 entry 10.
func TestCTSeverity_SensitiveRead_AssumeRoleWithWebIdentity(t *testing.T) {
	// sts:AssumeRoleWithWebIdentity → ct-attention
	event := buildSeverityCTEvent(
		"sr-10", "AssumeRoleWithWebIdentity", "sts.amazonaws.com",
		"123456789012", "123456789012", "AssumedRole", "Management", "AwsApiCall", "",
	)
	got := fetchOneStatus(t, event)
	if got != "ct-attention" {
		t.Errorf("Status = %q, want ct-attention — sts:AssumeRoleWithWebIdentity is §1.3 entry 10", got)
	}
}

// TestCTSeverity_SensitiveRead_GetAccessKeyLastUsed — §1.3 entry 11.
func TestCTSeverity_SensitiveRead_GetAccessKeyLastUsed(t *testing.T) {
	// iam:GetAccessKeyLastUsed → ct-attention (sensitive-read; verb is R from Get prefix)
	event := buildSeverityCTEvent(
		"sr-11", "GetAccessKeyLastUsed", "iam.amazonaws.com",
		"123456789012", "123456789012", "AssumedRole", "Management", "AwsApiCall", "",
	)
	got := fetchOneStatus(t, event)
	if got != "ct-attention" {
		t.Errorf("Status = %q, want ct-attention — iam:GetAccessKeyLastUsed is §1.3 entry 11", got)
	}
}

// TestCTSeverity_SensitiveRead_ListAccessKeys — §1.3 entry 12.
func TestCTSeverity_SensitiveRead_ListAccessKeys(t *testing.T) {
	// iam:ListAccessKeys → ct-attention (sensitive-read; verb is R from List prefix)
	event := buildSeverityCTEvent(
		"sr-12", "ListAccessKeys", "iam.amazonaws.com",
		"123456789012", "123456789012", "AssumedRole", "Management", "AwsApiCall", "",
	)
	got := fetchOneStatus(t, event)
	if got != "ct-attention" {
		t.Errorf("Status = %q, want ct-attention — iam:ListAccessKeys is §1.3 entry 12", got)
	}
}

// TestCTSeverity_SensitiveRead_GetCredentialReport — §1.3 entry 13.
func TestCTSeverity_SensitiveRead_GetCredentialReport(t *testing.T) {
	// iam:GetCredentialReport → ct-attention (sensitive-read; verb is R from Get prefix)
	event := buildSeverityCTEvent(
		"sr-13", "GetCredentialReport", "iam.amazonaws.com",
		"123456789012", "123456789012", "AssumedRole", "Management", "AwsApiCall", "",
	)
	got := fetchOneStatus(t, event)
	if got != "ct-attention" {
		t.Errorf("Status = %q, want ct-attention — iam:GetCredentialReport is §1.3 entry 13", got)
	}
}

// TestCTSeverity_SensitiveRead_GenerateCredentialReport — §1.3 entry 14.
// Note: GenerateCredentialReport has verb W (Generate* is NOT in the R table),
// which already yields ct-attention. The sensitive-read list also covers it.
func TestCTSeverity_SensitiveRead_GenerateCredentialReport(t *testing.T) {
	// iam:GenerateCredentialReport → ct-attention
	event := buildSeverityCTEvent(
		"sr-14", "GenerateCredentialReport", "iam.amazonaws.com",
		"123456789012", "123456789012", "AssumedRole", "Management", "AwsApiCall", "",
	)
	got := fetchOneStatus(t, event)
	if got != "ct-attention" {
		t.Errorf("Status = %q, want ct-attention — iam:GenerateCredentialReport is §1.3 entry 14", got)
	}
}

// TestCTSeverity_SensitiveRead_GetLoginProfile — §1.3 entry 15.
func TestCTSeverity_SensitiveRead_GetLoginProfile(t *testing.T) {
	// iam:GetLoginProfile → ct-attention (sensitive-read; verb is R from Get prefix)
	event := buildSeverityCTEvent(
		"sr-15", "GetLoginProfile", "iam.amazonaws.com",
		"123456789012", "123456789012", "AssumedRole", "Management", "AwsApiCall", "",
	)
	got := fetchOneStatus(t, event)
	if got != "ct-attention" {
		t.Errorf("Status = %q, want ct-attention — iam:GetLoginProfile is §1.3 entry 15", got)
	}
}

// TestCTSeverity_SensitiveRead_ExportCertificate — §1.3 entry 16.
// Note: ExportCertificate has verb W (Export* prefix), which already yields
// ct-attention. The sensitive-read list also covers it.
func TestCTSeverity_SensitiveRead_ExportCertificate(t *testing.T) {
	// acm:ExportCertificate → ct-attention (both verb W and §1.3 sensitive-read)
	event := buildSeverityCTEvent(
		"sr-16", "ExportCertificate", "acm.amazonaws.com",
		"123456789012", "123456789012", "AssumedRole", "Management", "AwsApiCall", "",
	)
	got := fetchOneStatus(t, event)
	if got != "ct-attention" {
		t.Errorf("Status = %q, want ct-attention — acm:ExportCertificate is §1.3 entry 16", got)
	}
}
