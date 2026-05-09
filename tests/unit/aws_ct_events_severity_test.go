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
// single synthetic event and returns the Fields["status"] of the first result.
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
	return result.Resources[0].Fields["status"]
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

// TestCTSeverity_SensitiveReadSTSAssumeRole — sts:AssumeRole → ct-info (verb R, no escalation).
// sts:AssumeRole is STS session vending (identity exchange, not state mutation).
// It is NOT on the §1.3 sensitive-reads allowlist and is exact-matched as verb=R
// before the "Assume" W-prefix table runs. With no error, no Root, same account: ct-info.
func TestCTSeverity_SensitiveReadSTSAssumeRole(t *testing.T) {
	// sts:AssumeRole → ct-info because verb=R (exact-match STS session-vending), no escalation.
	// Not a sensitive read, not cross-account, not root, no error.
	event := buildSeverityCTEvent(
		"sev-10", "AssumeRole", "sts.amazonaws.com",
		"123456789012", "123456789012", "AssumedRole", "Management", "AwsApiCall", "",
	)
	got := fetchOneStatus(t, event)
	if got != "ct-info" {
		t.Errorf("Status = %q, want ct-info — sts:AssumeRole is STS session-vending (verb R exact-match); not sensitive-read, same account, no error", got)
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
// §1.3 sensitive-reads allowlist — property test locking all entries.
//
// TestCTSeverity_AllowlistEntries_AreSensitiveAttention replaces 40 individual
// per-entry subtests. It iterates the full expected allowlist inline and asserts
// every entry yields ct-attention for a plain IAMUser, same-account, no-error
// fixture. The 3 original named tests above remain as logic-path coverage.
// ===========================================================================

// TestCTSeverity_AllowlistEntries_AreSensitiveAttention is a property test
// that locks the membership of the §1.3 sensitive-reads allowlist. It builds
// the expected list inline (the source of truth lives in
// internal/aws/ct_events.go::isSensitiveRead) and asserts each entry yields
// ct-attention when surrounded by a plain non-escalating context.
//
// Replaces 40 individual subtests that mirrored each switch case 1:1.
// Failure modes caught:
//   - An entry was removed from isSensitiveRead
//   - svc extraction in isSensitiveRead is broken
//   - The severity ladder no longer escalates sensitive reads to ct-attention
func TestCTSeverity_AllowlistEntries_AreSensitiveAttention(t *testing.T) {
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

	for _, entry := range allowlist {
		t.Run(entry.eventSource+"/"+entry.eventName, func(t *testing.T) {
			event := buildSeverityCTEvent(
				"al-"+entry.eventName, entry.eventName, entry.eventSource,
				"123456789012", "123456789012", "IAMUser", "Management", "AwsApiCall", "",
			)
			got := fetchOneStatus(t, event)
			if got != "ct-attention" {
				t.Errorf("status = %q, want ct-attention — %s:%s must be on §1.3 sensitive-reads allowlist",
					got, entry.eventSource, entry.eventName)
			}
		})
	}
}

// TestCTSeverity_AssumeRoleWithWebIdentity_IsInfo — §1.4 removal.
// AssumeRoleWithWebIdentity is the normal IRSA/OIDC flow (e.g. EKS pod assuming
// its service-account role). It is NOT a sensitive read — it is removed from the
// §1.3 allowlist and classified via exact-match verb=R, yielding ct-info.
// No root identity, same account, no error.
func TestCTSeverity_AssumeRoleWithWebIdentity_IsInfo(t *testing.T) {
	// sts:AssumeRoleWithWebIdentity → ct-info (plain IRSA/OIDC; no escalation)
	event := buildSeverityCTEvent(
		"sr-10", "AssumeRoleWithWebIdentity", "sts.amazonaws.com",
		"123456789012", "123456789012", "AssumedRole", "Management", "AwsApiCall", "",
	)
	got := fetchOneStatus(t, event)
	if got != "ct-info" {
		t.Errorf("Status = %q, want ct-info — sts:AssumeRoleWithWebIdentity removed from §1.3 allowlist; plain IRSA/OIDC is §1.4 ct-info", got)
	}
}

// TestCTSeverity_AssumeRole_IsInfo — sts:AssumeRole yields ct-info (verb R, no escalation).
// AssumeRole is STS session vending (identity exchange, not state mutation).
// It is NOT on the §1.3 sensitive-reads allowlist and is exact-matched as verb=R.
// With no error, same account, no root identity: ct-info.
func TestCTSeverity_AssumeRole_IsInfo(t *testing.T) {
	// sts:AssumeRole → ct-info (verb R exact-match; not sensitive-read, same account, no error).
	event := buildSeverityCTEvent(
		"sr-10a", "AssumeRole", "sts.amazonaws.com",
		"123456789012", "123456789012", "AssumedRole", "Management", "AwsApiCall", "",
	)
	got := fetchOneStatus(t, event)
	if got != "ct-info" {
		t.Errorf("Status = %q, want ct-info — sts:AssumeRole is STS session-vending (verb R exact-match); not sensitive-read, same account, no error", got)
	}
}

// TestCTSeverity_AssumeRoleWithSAML_IsInfo — sts:AssumeRoleWithSAML yields ct-info (verb R, no escalation).
// AssumeRoleWithSAML is STS session vending (SAML federation, not state mutation).
// It is NOT on the §1.3 sensitive-reads allowlist and is exact-matched as verb=R.
// With no error, same account, no root identity: ct-info.
func TestCTSeverity_AssumeRoleWithSAML_IsInfo(t *testing.T) {
	// sts:AssumeRoleWithSAML → ct-info (verb R exact-match; not sensitive-read, same account, no error).
	event := buildSeverityCTEvent(
		"sr-09b", "AssumeRoleWithSAML", "sts.amazonaws.com",
		"123456789012", "123456789012", "AssumedRole", "Management", "AwsApiCall", "",
	)
	got := fetchOneStatus(t, event)
	if got != "ct-info" {
		t.Errorf("Status = %q, want ct-info — sts:AssumeRoleWithSAML is STS session-vending (verb R exact-match); not sensitive-read, same account, no error", got)
	}
}

// ===========================================================================
// §1.4 NOT-sensitive regression tests — noisy read ops that must NOT escalate.
//
// These 4 tests assert that high-volume read ops deliberately excluded from
// the §1.3 allowlist remain at ct-info. They should PASS immediately and
// serve as regression guards to prevent accidental re-escalation.
// ===========================================================================

// TestCTSeverity_NotSensitive_STS_GetCallerIdentity — regression: must remain ct-info.
func TestCTSeverity_NotSensitive_STS_GetCallerIdentity(t *testing.T) {
	// sts:GetCallerIdentity → ct-info (too noisy — every SDK startup; excluded from §1.3)
	event := buildSeverityCTEvent(
		"ns-01", "GetCallerIdentity", "sts.amazonaws.com",
		"123456789012", "123456789012", "IAMUser", "Management", "AwsApiCall", "",
	)
	got := fetchOneStatus(t, event)
	if got != "ct-info" {
		t.Errorf("Status = %q, want ct-info — sts:GetCallerIdentity is deliberately excluded from §1.3 (too noisy)", got)
	}
}

// TestCTSeverity_NotSensitive_ECR_GetAuthorizationToken — regression: must remain ct-info.
func TestCTSeverity_NotSensitive_ECR_GetAuthorizationToken(t *testing.T) {
	// ecr:GetAuthorizationToken → ct-info (too noisy — every docker login; excluded from §1.3)
	event := buildSeverityCTEvent(
		"ns-02", "GetAuthorizationToken", "ecr.amazonaws.com",
		"123456789012", "123456789012", "IAMUser", "Management", "AwsApiCall", "",
	)
	got := fetchOneStatus(t, event)
	if got != "ct-info" {
		t.Errorf("Status = %q, want ct-info — ecr:GetAuthorizationToken is deliberately excluded from §1.3 (too noisy)", got)
	}
}

// TestCTSeverity_NotSensitive_ECR_GetDownloadUrlForLayer — regression: must remain ct-info.
func TestCTSeverity_NotSensitive_ECR_GetDownloadUrlForLayer(t *testing.T) {
	// ecr:GetDownloadUrlForLayer → ct-info (too noisy — every container pull; excluded from §1.3)
	event := buildSeverityCTEvent(
		"ns-03", "GetDownloadUrlForLayer", "ecr.amazonaws.com",
		"123456789012", "123456789012", "IAMUser", "Management", "AwsApiCall", "",
	)
	got := fetchOneStatus(t, event)
	if got != "ct-info" {
		t.Errorf("Status = %q, want ct-info — ecr:GetDownloadUrlForLayer is deliberately excluded from §1.3 (too noisy)", got)
	}
}

// TestCTSeverity_NotSensitive_RDS_DescribeDBSnapshots — regression: must remain ct-info.
func TestCTSeverity_NotSensitive_RDS_DescribeDBSnapshots(t *testing.T) {
	// rds:DescribeDBSnapshots → ct-info (noisy — backup polling; excluded from §1.3)
	event := buildSeverityCTEvent(
		"ns-04", "DescribeDBSnapshots", "rds.amazonaws.com",
		"123456789012", "123456789012", "IAMUser", "Management", "AwsApiCall", "",
	)
	got := fetchOneStatus(t, event)
	if got != "ct-info" {
		t.Errorf("Status = %q, want ct-info — rds:DescribeDBSnapshots is deliberately excluded from §1.3 (noisy backup polling)", got)
	}
}
