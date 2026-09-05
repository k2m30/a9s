package unit

// prowler_w4_secrets_test.go — behavioural tests for the batch-w4 secrets
// rows: a secret whose resource policy is open to anyone, and one that hands
// read access to another AWS account.

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/session"
)

const (
	w4CodeSecretPublicPolicy domain.FindingCode = "secrets.public-policy"
	w4CodeSecretCrossAccount domain.FindingCode = "secrets.cross-account-policy"

	w4PhraseSecretPublic    = "resource policy open to anyone"
	w4PhraseSecretCrossAcct = "resource policy grants another account"
	w4SourceSecretsWave2    = "wave2:secrets"
	w4OwnAccount            = "123456789012"
	w4ForeignAccount        = "210987654321"
	w4SecondForeignAccount  = "987654321098"
)

const w4SecretPublicPolicyDoc = `{"Version":"2012-10-17","Statement":[{"Sid":"AllowAnyone",` +
	`"Effect":"Allow","Principal":{"AWS":"*"},"Action":["secretsmanager:GetSecretValue",` +
	`"secretsmanager:DescribeSecret"],"Resource":"*"}]}`

const w4SecretCrossAccountPolicyDoc = `{"Version":"2012-10-17","Statement":[{"Sid":"AllowPartner",` +
	`"Effect":"Allow","Principal":{"AWS":["arn:aws:iam::210987654321:root",` +
	`"arn:aws:iam::987654321098:role/acme-partner-reader"]},` +
	`"Action":"secretsmanager:GetSecretValue","Resource":"*"}]}`

// Same-account grant: an explicit statement naming the owning account is the
// normal shape of a resource policy, not a cross-account share.
const w4SecretOwnAccountPolicyDoc = `{"Version":"2012-10-17","Statement":[{"Sid":"AllowSelf",` +
	`"Effect":"Allow","Principal":{"AWS":"arn:aws:iam::123456789012:role/acme-app-role"},` +
	`"Action":"secretsmanager:GetSecretValue","Resource":"*"}]}`

// Open to anyone AND naming a foreign account: the open grant is the whole
// story, so it is reported once as public.
const w4SecretPublicAndCrossDoc = `{"Version":"2012-10-17","Statement":[` +
	`{"Sid":"AllowAnyone","Effect":"Allow","Principal":{"AWS":"*"},` +
	`"Action":"secretsmanager:GetSecretValue","Resource":"*"},` +
	`{"Sid":"AllowPartner","Effect":"Allow","Principal":{"AWS":"arn:aws:iam::210987654321:root"},` +
	`"Action":"secretsmanager:DescribeSecret","Resource":"*"}]}`

// w4SecretsFake serves GetResourcePolicy keyed by secret id. A secret absent
// from policies has no resource policy at all, which the API reports as a
// successful call with a nil document.
type w4SecretsFake struct {
	awsclient.SecretsManagerAPI
	policies map[string]string
	errByID  map[string]error
}

func (f *w4SecretsFake) GetResourcePolicy(
	_ context.Context, in *secretsmanager.GetResourcePolicyInput, _ ...func(*secretsmanager.Options),
) (*secretsmanager.GetResourcePolicyOutput, error) {
	// GetResourcePolicy accepts either the secret name or its ARN; callers
	// legitimately pass the ARN, so the fake resolves both to the name.
	id := w4SecretNameFromID(aws.ToString(in.SecretId))
	if err, ok := f.errByID[id]; ok {
		return nil, err
	}
	out := &secretsmanager.GetResourcePolicyOutput{
		Name: aws.String(id),
		ARN:  aws.String(w4SecretARN(id)),
	}
	if doc, ok := f.policies[id]; ok {
		out.ResourcePolicy = aws.String(doc)
	}
	return out, nil
}

var _ awsclient.SecretsManagerAPI = (*w4SecretsFake)(nil)

func w4SecretARN(name string) string {
	return "arn:aws:secretsmanager:us-east-1:123456789012:secret:" + name + "-AbCdEf"
}

// w4SecretNameFromID reverses w4SecretARN, mirroring how Secrets Manager
// resolves an ARN back to the secret it names.
func w4SecretNameFromID(id string) string {
	const prefix = "arn:aws:secretsmanager:us-east-1:123456789012:secret:"
	if !strings.HasPrefix(id, prefix) {
		return id
	}
	return strings.TrimSuffix(strings.TrimPrefix(id, prefix), "-AbCdEf")
}

func w4SecretResource(name string) resource.Resource {
	return resource.Resource{
		ID: name, Name: name, Type: "secrets",
		Fields: map[string]string{
			"secret_name":      name,
			"arn":              w4SecretARN(name),
			"status":           "OK",
			"rotation_enabled": "true",
		},
	}
}

func w4EnrichSecrets(t *testing.T, fake *w4SecretsFake, rs []resource.Resource) awsclient.IssueEnricherResult {
	t.Helper()
	clients := &awsclient.ServiceClients{SecretsManager: fake, Region: "us-east-1"}
	identity := session.NewIdentityStore()
	identity.Set(w4OwnAccount, nil)
	clients.SetIdentityStore(identity)
	res, err := awsclient.EnrichSecretsPolicy(context.Background(), clients, rs, nil)
	if err != nil {
		t.Fatalf("EnrichSecretsPolicy: %v", err)
	}
	return res
}

// TestW4SecretPublicPolicy pins the open secret: anyone can read its value.
// The secret beside it, with no resource policy at all, is not flagged —
// absent is the normal, safe state.
func TestW4SecretPublicPolicy(t *testing.T) {
	fake := &w4SecretsFake{policies: map[string]string{
		"acme-open-api-key": w4SecretPublicPolicyDoc,
	}}
	res := w4EnrichSecrets(t, fake, []resource.Resource{
		w4SecretResource("acme-open-api-key"),
		w4SecretResource("acme-db-password"),
	})

	w4AssertFinding(t, res.Findings["acme-open-api-key"], w4CodeSecretPublicPolicy,
		w4PhraseSecretPublic, domain.SevBroken, w4SourceSecretsWave2)
	w4AssertRows(t, res.AttentionDetails["acme-open-api-key"], w4CodeSecretPublicPolicy, []domain.DetailRow{
		{Label: "Principal", Value: "*"},
		{Label: "Actions", Value: "secretsmanager:DescribeSecret, secretsmanager:GetSecretValue"},
	})
	w4AssertNoCode(t, res.Findings["acme-db-password"], w4CodeSecretPublicPolicy)
	w4AssertNoCode(t, res.Findings["acme-db-password"], w4CodeSecretCrossAccount)
}

// TestW4SecretPublicPolicyDetailIsAdvice pins the redaction rule: the
// finding describes the exposure and never carries a value.
func TestW4SecretPublicPolicyDetailIsAdvice(t *testing.T) {
	fake := &w4SecretsFake{policies: map[string]string{"acme-open-api-key": w4SecretPublicPolicyDoc}}
	res := w4EnrichSecrets(t, fake, []resource.Resource{w4SecretResource("acme-open-api-key")})

	f, ok := w4FindingByCode(t, res.Findings["acme-open-api-key"], w4CodeSecretPublicPolicy)
	if !ok {
		t.Fatalf("no %q finding", w4CodeSecretPublicPolicy)
	}
	if f.Detail == f.Phrase {
		t.Errorf("Detail merely repeats the phrase: %q", f.Detail)
	}
}

// TestW4SecretCrossAccountPolicy pins the shared secret: named foreign
// accounts can read it. Every account is listed so the operator can tell
// which share is intentional.
func TestW4SecretCrossAccountPolicy(t *testing.T) {
	fake := &w4SecretsFake{policies: map[string]string{
		"acme-partner-token": w4SecretCrossAccountPolicyDoc,
	}}
	res := w4EnrichSecrets(t, fake, []resource.Resource{w4SecretResource("acme-partner-token")})

	w4AssertFinding(t, res.Findings["acme-partner-token"], w4CodeSecretCrossAccount,
		w4PhraseSecretCrossAcct, domain.SevWarn, w4SourceSecretsWave2)
	w4AssertRows(t, res.AttentionDetails["acme-partner-token"], w4CodeSecretCrossAccount,
		[]domain.DetailRow{{Label: "Accounts", Value: w4ForeignAccount + ", " + w4SecondForeignAccount}})
	w4AssertNoCode(t, res.Findings["acme-partner-token"], w4CodeSecretPublicPolicy)
}

// TestW4SecretOwnAccountIsNotCrossAccount pins that a policy naming the
// account that owns the secret shares nothing.
func TestW4SecretOwnAccountIsNotCrossAccount(t *testing.T) {
	fake := &w4SecretsFake{policies: map[string]string{
		"acme-app-secret": w4SecretOwnAccountPolicyDoc,
	}}
	res := w4EnrichSecrets(t, fake, []resource.Resource{w4SecretResource("acme-app-secret")})
	w4AssertNoCode(t, res.Findings["acme-app-secret"], w4CodeSecretCrossAccount)
	w4AssertNoCode(t, res.Findings["acme-app-secret"], w4CodeSecretPublicPolicy)
}

// TestW4SecretPublicSupersedesCrossAccount pins that a policy open to
// everyone is reported once, as public. Naming a partner alongside "*" adds
// nothing an operator needs to act on separately.
func TestW4SecretPublicSupersedesCrossAccount(t *testing.T) {
	fake := &w4SecretsFake{policies: map[string]string{
		"acme-open-api-key": w4SecretPublicAndCrossDoc,
	}}
	res := w4EnrichSecrets(t, fake, []resource.Resource{w4SecretResource("acme-open-api-key")})

	w4AssertFinding(t, res.Findings["acme-open-api-key"], w4CodeSecretPublicPolicy,
		w4PhraseSecretPublic, domain.SevBroken, w4SourceSecretsWave2)
	w4AssertNoCode(t, res.Findings["acme-open-api-key"], w4CodeSecretCrossAccount)
}

// TestW4SecretPolicyFetchFailureIsUnknown pins that a secret whose policy
// could not be read is marked unknown, not clean, and that the rest of the
// batch is still judged.
func TestW4SecretPolicyFetchFailureIsUnknown(t *testing.T) {
	fake := &w4SecretsFake{
		policies: map[string]string{"acme-open-api-key": w4SecretPublicPolicyDoc},
		errByID: map[string]error{
			"acme-denied-secret": errors.New("AccessDeniedException: secretsmanager:GetResourcePolicy"),
		},
	}
	clients := &awsclient.ServiceClients{SecretsManager: fake, Region: "us-east-1"}
	identity := session.NewIdentityStore()
	identity.Set(w4OwnAccount, nil)
	clients.SetIdentityStore(identity)
	// A per-item failure is reported as a composite error alongside the
	// partial result; the batch is still delivered.
	res, err := awsclient.EnrichSecretsPolicy(context.Background(), clients, []resource.Resource{
		w4SecretResource("acme-denied-secret"),
		w4SecretResource("acme-open-api-key"),
	}, nil)
	if err == nil {
		t.Errorf("a denied GetResourcePolicy was reported as a clean pass")
	} else if !strings.Contains(err.Error(), "acme-denied-secret") {
		t.Errorf("composite error does not name the failed secret: %v", err)
	}

	if !res.TruncatedIDs["acme-denied-secret"] {
		t.Errorf("TruncatedIDs[acme-denied-secret] = false, want true")
	}
	if len(res.Findings["acme-denied-secret"]) != 0 {
		t.Errorf("a failed call produced findings: %s", w4CodesOf(res.Findings["acme-denied-secret"]))
	}
	w4AssertFinding(t, res.Findings["acme-open-api-key"], w4CodeSecretPublicPolicy,
		w4PhraseSecretPublic, domain.SevBroken, w4SourceSecretsWave2)
}

// TestW4SecretPolicyBeyondCapIsTruncated pins the cap semantics for an
// enricher that emits a "!" finding: past the budget the issue count is a
// lower bound and the result says so.
func TestW4SecretPolicyBeyondCapIsTruncated(t *testing.T) {
	policies := map[string]string{}
	rs := make([]resource.Resource, 0, awsclient.EnrichmentCap+1)
	for i := range awsclient.EnrichmentCap + 1 {
		name := fmt.Sprintf("acme-secret-%03d", i)
		policies[name] = w4SecretPublicPolicyDoc
		rs = append(rs, w4SecretResource(name))
	}
	res := w4EnrichSecrets(t, &w4SecretsFake{policies: policies}, rs)

	if !res.Truncated {
		t.Errorf("Truncated = false with %d secrets and a cap of %d", len(rs), awsclient.EnrichmentCap)
	}
	if len(res.Findings) != awsclient.EnrichmentCap {
		t.Errorf("evaluated %d secrets, want exactly the cap %d", len(res.Findings), awsclient.EnrichmentCap)
	}
}

// TestW4SecretPolicyNilClient pins the nil-client contract.
func TestW4SecretPolicyNilClient(t *testing.T) {
	res, err := awsclient.EnrichSecretsPolicy(context.Background(), &awsclient.ServiceClients{},
		[]resource.Resource{w4SecretResource("acme-open-api-key")}, nil)
	if err != nil {
		t.Fatalf("EnrichSecretsPolicy: %v", err)
	}
	if res.Findings == nil || res.TruncatedIDs == nil || res.FieldUpdates == nil {
		t.Fatalf("result maps must be non-nil")
	}
	if len(res.Findings) != 0 {
		t.Errorf("Findings = %v, want empty", res.Findings)
	}
}

// TestW4SecretsEnricherIsRegistered pins the wiring: an enricher the catalog
// does not know about never runs, so the two rows would never reach a row.
func TestW4SecretsEnricherIsRegistered(t *testing.T) {
	enricher, ok := awsclient.Wave2EnricherFor("secrets")
	if !ok || enricher.Fn == nil {
		t.Fatalf("secrets has no Wave 2 enricher registered")
	}
	if enricher.Priority != 100 {
		t.Errorf("Priority = %d, want 100", enricher.Priority)
	}
}

// TestW4SecretsFindingDefs pins the registry rows for the two new codes.
func TestW4SecretsFindingDefs(t *testing.T) {
	cases := []struct {
		code   domain.FindingCode
		phrase string
		sev    domain.Severity
	}{
		{w4CodeSecretPublicPolicy, w4PhraseSecretPublic, domain.SevBroken},
		{w4CodeSecretCrossAccount, w4PhraseSecretCrossAcct, domain.SevWarn},
	}
	for _, tc := range cases {
		t.Run(string(tc.code), func(t *testing.T) {
			def := w4FindingDef(t, "secrets", tc.code)
			if def.Phrase != tc.phrase {
				t.Errorf("Phrase = %q, want %q", def.Phrase, tc.phrase)
			}
			if def.Severity != tc.sev {
				t.Errorf("Severity = %v, want %v", def.Severity, tc.sev)
			}
			if def.Source != "wave2" {
				t.Errorf("Source = %q, want wave2", def.Source)
			}
		})
	}
}

// A wildcard principal scoped by a condition whose values arrive as an array
// rather than a scalar. Both shapes are legal IAM and mean the same thing.
const w4SecretConditionArrayDoc = `{"Version":"2012-10-17","Statement":[{"Sid":"AllowOrg",` +
	`"Effect":"Allow","Principal":{"AWS":"*"},"Action":"secretsmanager:GetSecretValue","Resource":"*",` +
	`"Condition":{"StringEquals":{"aws:PrincipalOrgID":["o-acme12345","o-acme67890"]}}}]}`

// TestW4SecretConditionValuesAsArray pins that a scoping condition is read the
// same whether its values are written as one string or a list.
func TestW4SecretConditionValuesAsArray(t *testing.T) {
	fake := &w4SecretsFake{policies: map[string]string{
		"acme-org-shared-secret": w4SecretConditionArrayDoc,
	}}
	res := w4EnrichSecrets(t, fake, []resource.Resource{w4SecretResource("acme-org-shared-secret")})
	w4AssertNoCode(t, res.Findings["acme-org-shared-secret"], w4CodeSecretPublicPolicy)
}

// TestW4SecretScheduledForDeletionEmitsNoPostureFinding pins that a secret
// already scheduled for deletion is not reported for its resource policy. The
// operator's action is to wait or restore, and a posture row on a resource on
// its way out is noise that outlives the resource.
func TestW4SecretScheduledForDeletionEmitsNoPostureFinding(t *testing.T) {
	deleted := w4SecretResource("acme-retired-api-key")
	deleted.Fields["status"] = "DELETED"
	fake := &w4SecretsFake{policies: map[string]string{
		"acme-retired-api-key": w4SecretPublicPolicyDoc,
	}}
	res := w4EnrichSecrets(t, fake, []resource.Resource{deleted})
	w4AssertNoCode(t, res.Findings["acme-retired-api-key"], w4CodeSecretPublicPolicy)
	w4AssertNoCode(t, res.Findings["acme-retired-api-key"], w4CodeSecretCrossAccount)
}
