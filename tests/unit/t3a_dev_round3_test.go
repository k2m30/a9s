package unit

import (
	"slices"
	"testing"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/iampolicy"
)

// A Deny applies only to requests for a resource it matches (SNS Developer
// Guide, Evaluation logic: the engine evaluates "all the policies that are
// applicable to the request (based on the resource, principal, action, and
// conditions)"; IAM User Guide, NotResource: "resources specified in this
// element are the only resources that are not limited"). A Deny on part of
// a public grant's resources leaves the rest public. Resources compare
// case-sensitively ("In the Resource element, the IAM user name is case
// sensitive", IAM User Guide, IAM JSON policy elements: Resource), actions do
// not.
func TestT3a_Dev_DenyAppliesToTheResourcesItNames(t *testing.T) {
	const allow = `{"Effect":"Allow","Principal":"*","Action":"s3:GetObject","Resource":"arn:aws:s3:::acme-bucket/*"}`
	cases := []struct {
		name   string
		deny   string
		public bool
	}{
		{"subset_resource", `{"Effect":"Deny","Principal":"*","Action":"s3:GetObject","Resource":"arn:aws:s3:::acme-bucket/private/*"}`, true},
		{"not_resource", `{"Effect":"Deny","Principal":"*","Action":"s3:GetObject","NotResource":"arn:aws:s3:::acme-bucket/public/*"}`, true},
		{"subset_resource_fence", `{"Effect":"Deny","Principal":"*","Action":"s3:GetObject","Resource":"arn:aws:s3:::acme-bucket/private/*","Condition":{"StringNotEquals":{"aws:SourceVpce":"vpce-0acme1234567890ab"}}}`, true},
		{"same_resource", `{"Effect":"Deny","Principal":"*","Action":"s3:GetObject","Resource":"arn:aws:s3:::acme-bucket/*"}`, false},
		{"wider_resource", `{"Effect":"Deny","Principal":"*","Action":"s3:*","Resource":"arn:aws:s3:::acme-*"}`, false},
		{"no_resource_element", `{"Effect":"Deny","Principal":"*","Action":"s3:GetObject"}`, false},
		{"question_mark_pattern", `{"Effect":"Deny","Principal":"*","Action":"s3:GetObject","Resource":"arn:aws:s3:::acme-bucket/?"}`, true},
		{"resource_differs_in_case", `{"Effect":"Deny","Principal":"*","Action":"s3:GetObject","Resource":"arn:aws:s3:::ACME-bucket/*"}`, true},
		{"action_differs_in_case", `{"Effect":"Deny","Principal":"*","Action":"S3:getobject","Resource":"arn:aws:s3:::acme-bucket/*"}`, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ex := iampolicy.Evaluate(mustParse(t, policyOf(allow, c.deny)), "123456789012")
			if ex.Public != c.public {
				t.Errorf("Public = %v, want %v (%+v)", ex.Public, c.public, ex)
			}
		})
	}
}

// IAM ANDs a Deny's condition entries, so an aws:PrincipalIsAWSService entry
// confines a Deny to AWS service principals or to every other caller. A
// grant is closed for a class only by the Denies that apply to it. The AWS
// two-statement perimeter (the identity perimeter for callers that are not
// services, and RCPEnforceConfusedDeputyProtection for services, IAM User
// Guide, AWS global condition context keys, aws:SourceOrgID) fences both
// classes; either statement alone leaves the other class open.
func TestT3a_Dev_DenyIsReadPerCallerClass(t *testing.T) {
	const allow = `{"Effect":"Allow","Principal":"*","Action":"sqs:SendMessage","Resource":"*"}`
	const identity = `{"Effect":"Deny","Principal":"*","Action":"sqs:*","Resource":"*","Condition":{"StringNotEqualsIfExists":{"aws:PrincipalOrgID":"o-abc123def4"},"BoolIfExists":{"aws:PrincipalIsAWSService":"false"}}}`
	const deputy = `{"Effect":"Deny","Principal":"*","Action":"sqs:*","Resource":"*","Condition":{"StringNotEqualsIfExists":{"aws:SourceOrgID":"o-abc123def4"},"Null":{"aws:SourceAccount":"false"},"Bool":{"aws:PrincipalIsAWSService":"true"}}}`
	const network = `{"Effect":"Deny","Principal":"*","Action":"sqs:*","Resource":"*","Condition":{"StringNotEquals":{"aws:SourceVpce":"vpce-0acme1234567890ab"},"BoolIfExists":{"aws:PrincipalIsAWSService":"false"}}}`
	const denyServices = `{"Effect":"Deny","Principal":"*","Action":"sqs:*","Resource":"*","Condition":{"Bool":{"aws:PrincipalIsAWSService":"true"}}}`
	const denyOthers = `{"Effect":"Deny","Principal":"*","Action":"sqs:*","Resource":"*","Condition":{"BoolIfExists":{"aws:PrincipalIsAWSService":"false"}}}`
	const denyOthersPlainBool = `{"Effect":"Deny","Principal":"*","Action":"sqs:*","Resource":"*","Condition":{"Bool":{"aws:PrincipalIsAWSService":"false"}}}`
	cases := []struct {
		name   string
		denies []string
		public bool
	}{
		{"two_statement_perimeter", []string{identity, deputy}, false},
		{"identity_alone", []string{identity}, true},
		{"deputy_alone", []string{deputy}, true},
		{"network_perimeter_alone", []string{network}, true},
		{"network_and_deputy", []string{network, deputy}, false},
		{"deny_services_and_deny_others", []string{denyServices, denyOthers}, false},
		{"deny_services_and_deny_signed_others", []string{denyServices, denyOthersPlainBool}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ex := iampolicy.Evaluate(mustParse(t, policyOf(append([]string{allow}, c.denies...)...)), "123456789012")
			if ex.Public != c.public {
				t.Errorf("Public = %v, want %v (%+v)", ex.Public, c.public, ex)
			}
		})
	}
}

// The Condition row and the Deny when row read the same way: the ANDed
// entries of one statement joined by " and ", statements by "; ". A Deny
// that overlaps a public grant's actions is listed even when it covers only
// some of them.
func TestT3a_Dev_RowTextGroupsByStatement(t *testing.T) {
	ex := iampolicy.Evaluate(mustParse(t, policyOf(
		`{"Effect":"Allow","Principal":"*","Action":"sqs:SendMessage","Resource":"*","Condition":{"Bool":{"aws:SecureTransport":"true"},"StringLike":{"aws:SourceArn":["arn:aws:s3:::a-*","arn:aws:s3:::b"]}}}`,
		`{"Effect":"Allow","Principal":"*","Action":"sqs:ReceiveMessage","Resource":"*","Condition":{"IpAddress":{"aws:SourceIp":"0.0.0.0/0"}}}`,
		`{"Effect":"Deny","Principal":"*","Action":"sqs:*","Resource":"*","Condition":{"Bool":{"aws:SecureTransport":"false"},"StringEquals":{"aws:SourceVpce":"vpce-0acme1234567890ab"}}}`,
		`{"Effect":"Deny","Principal":"*","Action":"sqs:ReceiveMessage","Resource":"*","Condition":{"Bool":{"aws:MultiFactorAuthPresent":"false"}}}`,
	)), "123456789012")
	wantCond := []string{
		"Bool aws:SecureTransport true and StringLike aws:SourceArn arn:aws:s3:::a-*, arn:aws:s3:::b",
		"IpAddress aws:SourceIp 0.0.0.0/0",
	}
	if !slices.Equal(ex.PublicConditions, wantCond) {
		t.Errorf("PublicConditions = %q, want %q", ex.PublicConditions, wantCond)
	}
	wantDeny := []string{
		"Bool aws:MultiFactorAuthPresent false",
		"Bool aws:SecureTransport false and StringEquals aws:SourceVpce vpce-0acme1234567890ab",
	}
	if !slices.Equal(ex.PublicDenyConditions, wantDeny) {
		t.Errorf("PublicDenyConditions = %q, want %q", ex.PublicDenyConditions, wantDeny)
	}

	services := enrichCodeArtifactPolicy(policyOf(
		`{"Effect":"Allow","Principal":"*","Action":"codeartifact:ReadFromRepository","Resource":"*","Condition":{"Bool":{"aws:SecureTransport":"true"},"StringLike":{"aws:SourceArn":"arn:aws:s3:::a-*"}}}`,
		`{"Effect":"Allow","Principal":"*","Action":"codeartifact:GetAuthorizationToken","Resource":"*","Condition":{"IpAddress":{"aws:SourceIp":"0.0.0.0/0"}}}`,
	))
	rows := services.AttentionDetails["acme-artifacts/acme-npm"]["codeartifact.public-access-policy"].Rows
	i := slices.IndexFunc(rows, func(r domain.DetailRow) bool { return r.Label == "Condition" })
	if want := "Bool aws:SecureTransport true and StringLike aws:SourceArn arn:aws:s3:::a-*; IpAddress aws:SourceIp 0.0.0.0/0"; i < 0 || rows[i].Value != want {
		t.Errorf("rows = %+v, want Condition %q", rows, want)
	}
}

func TestT3a_Dev_DenyWhenListsADenyThatOverlapsTheGrant(t *testing.T) {
	const tls = `{"Effect":"Deny","Principal":"*","Action":"sqs:SendMessage","Resource":"*","Condition":{"Bool":{"aws:SecureTransport":"false"}}}`
	cases := []struct{ name, allow string }{
		{"service_wildcard_allow", `{"Effect":"Allow","Principal":"*","Action":"sqs:*","Resource":"*"}`},
		{"not_action_allow", `{"Effect":"Allow","Principal":"*","NotAction":"sqs:DeleteQueue","Resource":"*"}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ex := iampolicy.Evaluate(mustParse(t, policyOf(c.allow, tls)), "123456789012")
			if want := []string{"Bool aws:SecureTransport false"}; !ex.Public || !slices.Equal(ex.PublicDenyConditions, want) {
				t.Errorf("%+v, want Public with PublicDenyConditions %q", ex, want)
			}
		})
	}
}
