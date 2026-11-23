package unit

import (
	"slices"
	"strings"
	"testing"

	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/iampolicy"
)

// A public grant whose condition scopes no caller is open to whatever that
// condition still admits, so the finding's supporting rows state the
// condition found: every AWS service acting for any account under
// aws:PrincipalIsAWSService true, every principal that presents the external
// ID on a wildcard trust. A grant with no condition carries no such row.
func TestT3a_Dev_PublicGrantStatesItsCondition(t *testing.T) {
	ex := iampolicy.Evaluate(mustParse(t, `{"Statement":[{"Effect":"Allow","Principal":"*","Action":"sns:Publish",
		"Condition":{"Bool":{"aws:PrincipalIsAWSService":"true"},"StringLike":{"aws:SourceVpc":["vpc-*","vpc-0a1b"]}}}]}`), "123456789012")
	want := []string{"Bool aws:PrincipalIsAWSService true and StringLike aws:SourceVpc vpc-*, vpc-0a1b"}
	if !slices.Equal(ex.PublicConditions, want) {
		t.Errorf("PublicConditions = %q, want %q", ex.PublicConditions, want)
	}
	if open := iampolicy.Evaluate(mustParse(t, `{"Statement":[{"Effect":"Allow","Principal":"*","Action":"sns:Publish"}]}`), ""); open.PublicConditions != nil {
		t.Errorf("unconditioned grant: PublicConditions = %q, want nil", open.PublicConditions)
	}

	services := enrichCodeArtifactPolicy(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":"*",` +
		`"Action":"codeartifact:ReadFromRepository","Resource":"*","Condition":{"Bool":{"aws:PrincipalIsAWSService":"true"}}}]}`)
	assertConditionRow(t, services.AttentionDetails["acme-artifacts/acme-npm"]["codeartifact.public-access-policy"], "Bool aws:PrincipalIsAWSService true")

	fake := &w4RoleListFake{roles: []iamtypes.Role{w4Role("acme-trust-role", "/", w4TrustObjectStarWithExternalID)}}
	r := w4FetchRole(t, fake, "acme-trust-role")
	assertConditionRow(t, r.AttentionDetails[w4CodeRoleWildcardTrust], "StringEquals sts:ExternalId acme-shared-external-id")
}

// A conditional Deny that covers a public action without fencing it still
// shapes who gets through, so the finding states its condition as written,
// one item per Deny statement, its entries ANDed as IAM evaluates them. A
// Deny that fences the grant, or one that covers none of the public actions,
// is not listed.
func TestT3a_Dev_PublicGrantStatesTheDenyThatDidNotCloseIt(t *testing.T) {
	ex := iampolicy.Evaluate(mustParse(t, policyOf(
		`{"Effect":"Allow","Principal":"*","Action":["sqs:SendMessage","sqs:ReceiveMessage"],"Resource":"*"}`,
		`{"Effect":"Deny","Principal":"*","Action":"sqs:*","Resource":"*","Condition":{"Bool":{"aws:SecureTransport":"false"},"StringEquals":{"aws:SourceVpce":"vpce-0acme1234567890ab"}}}`,
		`{"Effect":"Deny","Principal":"*","Action":"sqs:DeleteQueue","Resource":"*","Condition":{"Bool":{"aws:MultiFactorAuthPresent":"false"}}}`,
	)), "123456789012")
	want := []string{"Bool aws:SecureTransport false and StringEquals aws:SourceVpce vpce-0acme1234567890ab"}
	if !slices.Equal(ex.PublicDenyConditions, want) {
		t.Errorf("PublicDenyConditions = %q, want %q", ex.PublicDenyConditions, want)
	}

	fenced := iampolicy.Evaluate(mustParse(t, policyOf(
		`{"Effect":"Allow","Principal":"*","Action":"sqs:SendMessage","Resource":"*"}`,
		`{"Effect":"Deny","Principal":"*","Action":"sqs:*","Resource":"*","Condition":{"Bool":{"aws:SecureTransport":"false"}}}`,
		`{"Effect":"Deny","Principal":"*","Action":"sqs:ReceiveMessage","Resource":"*","Condition":{"StringNotEquals":{"aws:PrincipalOrgID":"o-acme12345"}}}`,
	)), "123456789012")
	if want := []string{"Bool aws:SecureTransport false"}; !slices.Equal(fenced.PublicDenyConditions, want) {
		t.Errorf("PublicDenyConditions = %q, want %q", fenced.PublicDenyConditions, want)
	}

	perimeter := iampolicy.Evaluate(mustParse(t, policyOf(
		`{"Effect":"Allow","Principal":"*","Action":"sqs:SendMessage","Resource":"*"}`,
		`{"Effect":"Deny","Principal":"*","Action":"sqs:*","Resource":"*","Condition":{"StringNotEquals":{"aws:SourceVpce":"vpce-0acme1234567890ab"},"BoolIfExists":{"aws:PrincipalIsAWSService":"false"}}}`,
	)), "123456789012")
	if want := []string{"BoolIfExists aws:PrincipalIsAWSService false and StringNotEquals aws:SourceVpce vpce-0acme1234567890ab"}; !perimeter.Public || !slices.Equal(perimeter.PublicDenyConditions, want) {
		t.Errorf("perimeter Deny: %+v, want Public with PublicDenyConditions %q", perimeter, want)
	}

	fake := &w4RoleListFake{roles: []iamtypes.Role{w4Role("acme-trust-role", "/", policyOf(
		`{"Effect":"Allow","Principal":{"AWS":"*"},"Action":"sts:AssumeRole"}`,
		`{"Effect":"Deny","Principal":{"AWS":"*"},"Action":"sts:AssumeRole","Condition":{"StringNotEquals":{"sts:ExternalId":"acme-ext-7f3a"}}}`,
	))}}
	r := w4FetchRole(t, fake, "acme-trust-role")
	assertRow(t, r.AttentionDetails[w4CodeRoleWildcardTrust], "Deny when", "StringNotEquals sts:ExternalId acme-ext-7f3a")

	services := enrichCodeArtifactPolicy(policyOf(
		`{"Effect":"Allow","Principal":"*","Action":"codeartifact:ReadFromRepository","Resource":"*"}`,
		`{"Effect":"Deny","Principal":"*","Action":"codeartifact:*","Resource":"*","Condition":{"Bool":{"aws:SecureTransport":"false"}}}`,
	))
	assertRow(t, services.AttentionDetails["acme-artifacts/acme-npm"]["codeartifact.public-access-policy"], "Deny when", "Bool aws:SecureTransport false")
}

func assertConditionRow(t *testing.T, ad domain.AttentionDetail, want string) {
	t.Helper()
	assertRow(t, ad, "Condition", want)
}

func assertRow(t *testing.T, ad domain.AttentionDetail, label, want string) {
	t.Helper()
	i := slices.IndexFunc(ad.Rows, func(r domain.DetailRow) bool { return r.Label == label })
	if i < 0 || !strings.Contains(ad.Rows[i].Value, want) {
		t.Errorf("rows = %+v, want a %s row containing %q", ad.Rows, label, want)
	}
}
