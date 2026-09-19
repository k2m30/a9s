package unit

import (
	"slices"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/iampolicy"
)

// IAM evaluates every statement of a policy and an explicit Deny overrides any
// Allow for the principal/action pair it matches
// (docs.aws.amazon.com/IAM/latest/UserGuide/reference_policies_evaluation-logic.html).
// A Deny whose condition only fires when a restrictive key is NOT the named
// value ("deny everyone unless they come through vpce-X") turns the wildcard
// Allow beside it into a scoped grant; a Deny on some other axis (plain HTTP)
// or on one principal leaves the wildcard grant public.
func TestEvaluateExplicitDenySubtractsFromExposure(t *testing.T) {
	cases := []struct {
		name            string
		doc             string
		wantPublic      bool
		wantConditioned bool
		wantActions     []string
	}{
		{
			name: "deny unless the request comes through the named VPC endpoint",
			doc: `{"Version":"2012-10-17","Statement":[
				{"Sid":"AllowRead","Effect":"Allow","Principal":"*","Action":["dynamodb:GetItem","dynamodb:Query"],
				 "Resource":"arn:aws:dynamodb:eu-central-1:123456789012:table/acme-orders"},
				{"Sid":"DenyOutsideVpce","Effect":"Deny","Principal":"*","Action":"dynamodb:*",
				 "Resource":"arn:aws:dynamodb:eu-central-1:123456789012:table/acme-orders",
				 "Condition":{"StringNotEquals":{"aws:SourceVpce":"vpce-0acme1234567890ab"}}}]}`,
			wantPublic:      false,
			wantConditioned: true,
		},
		{
			name: "the AWS-documented network perimeter deny that exempts service principals",
			doc: `{"Version":"2012-10-17","Statement":[
				{"Effect":"Allow","Principal":"*","Action":"sqs:SendMessage","Resource":"arn:aws:sqs:us-east-1:123456789012:acme-orders"},
				{"Effect":"Deny","Principal":"*","Action":"sqs:*","Resource":"arn:aws:sqs:us-east-1:123456789012:acme-orders",
				 "Condition":{"StringNotEquals":{"aws:SourceVpce":"vpce-0acme1234567890ab"},
				              "BoolIfExists":{"aws:PrincipalIsAWSService":"false"}}}]}`,
			wantPublic:      false,
			wantConditioned: true,
		},
		{
			name: "deny unless the caller is inside the organisation",
			doc: `{"Version":"2012-10-17","Statement":[
				{"Effect":"Allow","Principal":{"AWS":"*"},"Action":"secretsmanager:GetSecretValue","Resource":"*"},
				{"Effect":"Deny","Principal":{"AWS":"*"},"Action":"secretsmanager:GetSecretValue","Resource":"*",
				 "Condition":{"StringNotEquals":{"aws:PrincipalOrgID":"o-acme12345"}}}]}`,
			wantPublic:      false,
			wantConditioned: true,
		},
		{
			name: "unconditional deny of every action leaves nothing granted",
			doc: `{"Version":"2012-10-17","Statement":[
				{"Effect":"Allow","Principal":"*","Action":["sqs:SendMessage","sqs:ReceiveMessage"],"Resource":"*"},
				{"Effect":"Deny","Principal":"*","Action":"sqs:*","Resource":"*"}]}`,
			wantPublic:      false,
			wantConditioned: false,
		},
		{
			name: "deny of one action leaves the other one public",
			doc: `{"Version":"2012-10-17","Statement":[
				{"Effect":"Allow","Principal":"*","Action":["sqs:SendMessage","sqs:ReceiveMessage"],"Resource":"*"},
				{"Effect":"Deny","Principal":"*","Action":"sqs:ReceiveMessage","Resource":"*"}]}`,
			wantPublic:  true,
			wantActions: []string{"sqs:SendMessage"},
		},
		{
			name: "an action wildcard in the deny matches the actions it covers",
			doc: `{"Version":"2012-10-17","Statement":[
				{"Effect":"Allow","Principal":"*","Action":["sqs:SendMessage","sqs:ReceiveMessage"],"Resource":"*"},
				{"Effect":"Deny","Principal":"*","Action":"sqs:Receive*","Resource":"*"}]}`,
			wantPublic:  true,
			wantActions: []string{"sqs:SendMessage"},
		},
		{
			name: "denying plain HTTP scopes nobody: anyone may still call over TLS",
			doc: `{"Version":"2012-10-17","Statement":[
				{"Effect":"Allow","Principal":"*","Action":"s3:GetObject","Resource":"arn:aws:s3:::acme-assets/*"},
				{"Effect":"Deny","Principal":"*","Action":"s3:*","Resource":"arn:aws:s3:::acme-assets/*",
				 "Condition":{"Bool":{"aws:SecureTransport":"false"}}}]}`,
			wantPublic:  true,
			wantActions: []string{"s3:GetObject"},
		},
		{
			name: "denying only the traffic that comes through one endpoint leaves everyone else in",
			doc: `{"Version":"2012-10-17","Statement":[
				{"Effect":"Allow","Principal":"*","Action":"sqs:SendMessage","Resource":"*"},
				{"Effect":"Deny","Principal":"*","Action":"sqs:SendMessage","Resource":"*",
				 "Condition":{"StringEquals":{"aws:SourceVpce":"vpce-0acme1234567890ab"}}}]}`,
			wantPublic:  true,
			wantActions: []string{"sqs:SendMessage"},
		},
		{
			name: "denying one named account leaves the wildcard grant public for the rest",
			doc: `{"Version":"2012-10-17","Statement":[
				{"Effect":"Allow","Principal":"*","Action":"sns:Publish","Resource":"*"},
				{"Effect":"Deny","Principal":{"AWS":"arn:aws:iam::210987654321:root"},"Action":"sns:Publish","Resource":"*"}]}`,
			wantPublic:  true,
			wantActions: []string{"sns:Publish"},
		},
		{
			name: "the healthy counterpart: allow everyone with no deny",
			doc: `{"Version":"2012-10-17","Statement":[
				{"Effect":"Allow","Principal":"*","Action":["dynamodb:GetItem","dynamodb:Query"],"Resource":"*"}]}`,
			wantPublic:  true,
			wantActions: []string{"dynamodb:GetItem", "dynamodb:Query"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ex := iampolicy.Evaluate(mustParse(t, tc.doc), "123456789012")
			if ex.Public != tc.wantPublic {
				t.Errorf("Public = %v, want %v", ex.Public, tc.wantPublic)
			}
			if ex.Conditioned != tc.wantConditioned {
				t.Errorf("Conditioned = %v, want %v", ex.Conditioned, tc.wantConditioned)
			}
			if !strSliceEqual(ex.PublicActions, tc.wantActions) {
				t.Errorf("PublicActions = %v, want %v", ex.PublicActions, tc.wantActions)
			}
		})
	}
}

// A foreign account named by an Allow and denied the same action by an
// explicit Deny has no access, so it is not a cross-account grant; the other
// partner the Deny does not name still is.
func TestEvaluateExplicitDenyRemovesADeniedForeignAccount(t *testing.T) {
	doc := mustParse(t, `{"Version":"2012-10-17","Statement":[
		{"Sid":"AllowPartners","Effect":"Allow",
		 "Principal":{"AWS":["arn:aws:iam::210987654321:root","arn:aws:iam::987654321098:role/acme-partner-reader"]},
		 "Action":"secretsmanager:GetSecretValue","Resource":"*"},
		{"Sid":"RevokeFormerPartner","Effect":"Deny",
		 "Principal":{"AWS":"arn:aws:iam::210987654321:root"},
		 "Action":"secretsmanager:GetSecretValue","Resource":"*"}]}`)

	ex := iampolicy.Evaluate(doc, "123456789012")
	if want := []string{"987654321098"}; !strSliceEqual(ex.CrossAccount, want) {
		t.Errorf("CrossAccount = %v, want %v", ex.CrossAccount, want)
	}
}

// aws:PrincipalIsAWSService is true only when an AWS service principal calls
// the resource directly, and anonymous requests do not carry the key at all
// (docs.aws.amazon.com/IAM/latest/UserGuide/reference_policies_condition-keys.html).
// A wildcard grant fenced by it at "true" admits AWS services only, which is
// not public; the same key at "false" admits every signed caller that is not
// a service, which is.
func TestEvaluatePrincipalIsAWSServiceIsARestrictiveKey(t *testing.T) {
	const tmpl = `{"Version":"2012-10-17","Statement":[{"Sid":"AllowServices","Effect":"Allow","Principal":"*",
		"Action":["codeartifact:GetAuthorizationToken","codeartifact:ReadFromRepository"],"Resource":"*",
		"Condition":{"Bool":{"aws:PrincipalIsAWSService":"VALUE"}}}]}`

	servicesOnly := iampolicy.Evaluate(mustParse(t, strings.Replace(tmpl, "VALUE", "true", 1)), "123456789012")
	if servicesOnly.Public || !servicesOnly.Conditioned {
		t.Errorf("aws:PrincipalIsAWSService=true: Public=%v Conditioned=%v, want Public=false Conditioned=true",
			servicesOnly.Public, servicesOnly.Conditioned)
	}

	everyoneElse := iampolicy.Evaluate(mustParse(t, strings.Replace(tmpl, "VALUE", "false", 1)), "123456789012")
	if !everyoneElse.Public {
		t.Errorf("aws:PrincipalIsAWSService=false admits every non-service caller, yet Public=false")
	}
}

func TestAccountFromARN(t *testing.T) {
	cases := []struct{ arn, want string }{
		{"arn:aws:kms:eu-central-1:123456789012:key/1a2b3c4d-1111-2222-3333-444455556666", "123456789012"},
		{"arn:aws:secretsmanager:us-east-1:123456789012:secret:acme-db-AbCdEf", "123456789012"},
		{"arn:aws:iam::123456789012:role/service-role/acme-deploy", "123456789012"},
		{"arn:aws-us-gov:sns:us-gov-west-1:123456789012:acme-alerts", "123456789012"},
		{"arn:aws-cn:dynamodb:cn-north-1:123456789012:table/acme-orders", "123456789012"},
		// S3 bucket ARNs carry no account; an AWS-managed policy's account
		// field is the word "aws", not an account.
		{"arn:aws:s3:::acme-assets", ""},
		{"arn:aws:iam::aws:policy/ReadOnlyAccess", ""},
		{"acme-orders", ""},
		{"", ""},
	}
	for _, tc := range cases {
		if got := iampolicy.AccountFromARN(tc.arn); got != tc.want {
			t.Errorf("AccountFromARN(%q) = %q, want %q", tc.arn, got, tc.want)
		}
	}
}

func sortedPrincipals(ps []iampolicy.Principal) []iampolicy.Principal {
	out := slices.Clone(ps)
	slices.SortFunc(out, func(a, b iampolicy.Principal) int {
		return strings.Compare(a.Kind+"|"+a.Value, b.Kind+"|"+b.Value)
	})
	return out
}

// AllowedPrincipals is the list the related panels read, so it must answer
// the same question access does: granted by an Allow and not taken away by
// an explicit Deny on the same action. A Deny on another action, or one that
// only fires under a condition the principal can avoid, takes nothing away.
func TestAllowedPrincipalsSubtractsExplicitDeny(t *testing.T) {
	cases := []struct {
		name string
		doc  string
		want []iampolicy.Principal
	}{
		{
			name: "a denied principal is not listed, the others keep their account",
			doc: `{"Version":"2012-10-17","Statement":[
				{"Effect":"Allow","Principal":{"AWS":["arn:aws:iam::123456789012:user/alice","arn:aws:iam::123456789012:user/bob"]},"Action":"sts:AssumeRole"},
				{"Effect":"Allow","Principal":{"Service":"ec2.amazonaws.com"},"Action":"sts:AssumeRole"},
				{"Effect":"Allow","Principal":{"Federated":"arn:aws:iam::123456789012:saml-provider/AcmeIdP"},"Action":"sts:AssumeRoleWithSAML"},
				{"Effect":"Deny","Principal":{"AWS":"arn:aws:iam::123456789012:user/bob"},"Action":"sts:AssumeRole"}]}`,
			want: []iampolicy.Principal{
				{Kind: "AWS", Value: "arn:aws:iam::123456789012:user/alice", Account: "123456789012"},
				{Kind: "Federated", Value: "arn:aws:iam::123456789012:saml-provider/AcmeIdP", Account: "123456789012"},
				{Kind: "Service", Value: "ec2.amazonaws.com", Account: ""},
			},
		},
		{
			name: "a bare account id and a foreign role carry their own account",
			doc: `{"Version":"2012-10-17","Statement":[
				{"Effect":"Allow","Principal":{"AWS":["210987654321","arn:aws:iam::987654321098:role/acme-partner-reader"]},"Action":"sts:AssumeRole"}]}`,
			want: []iampolicy.Principal{
				{Kind: "AWS", Value: "210987654321", Account: "210987654321"},
				{Kind: "AWS", Value: "arn:aws:iam::987654321098:role/acme-partner-reader", Account: "987654321098"},
			},
		},
		{
			name: "a wildcard principal is listed as the wildcard kind",
			doc: `{"Version":"2012-10-17","Statement":[
				{"Effect":"Allow","Principal":"*","Action":"sns:Publish","Resource":"*"}]}`,
			want: []iampolicy.Principal{{Kind: "*", Value: "*", Account: ""}},
		},
		{
			name: "a deny on a different action does not take the principal away",
			doc: `{"Version":"2012-10-17","Statement":[
				{"Effect":"Allow","Principal":{"AWS":"arn:aws:iam::123456789012:user/bob"},"Action":["sts:AssumeRole","sts:TagSession"]},
				{"Effect":"Deny","Principal":{"AWS":"arn:aws:iam::123456789012:user/bob"},"Action":"sts:TagSession"}]}`,
			want: []iampolicy.Principal{
				{Kind: "AWS", Value: "arn:aws:iam::123456789012:user/bob", Account: "123456789012"},
			},
		},
		{
			name: "a deny the principal can satisfy (MFA) does not take the principal away",
			doc: `{"Version":"2012-10-17","Statement":[
				{"Effect":"Allow","Principal":{"AWS":"arn:aws:iam::123456789012:user/bob"},"Action":"sts:AssumeRole"},
				{"Effect":"Deny","Principal":{"AWS":"arn:aws:iam::123456789012:user/bob"},"Action":"sts:AssumeRole",
				 "Condition":{"BoolIfExists":{"aws:MultiFactorAuthPresent":"false"}}}]}`,
			want: []iampolicy.Principal{
				{Kind: "AWS", Value: "arn:aws:iam::123456789012:user/bob", Account: "123456789012"},
			},
		},
		{
			name: "an unconditional deny to everyone leaves nobody",
			doc: `{"Version":"2012-10-17","Statement":[
				{"Effect":"Allow","Principal":{"AWS":"arn:aws:iam::123456789012:role/acme-publisher"},"Action":"sns:Publish","Resource":"*"},
				{"Effect":"Deny","Principal":"*","Action":"sns:*","Resource":"*"}]}`,
			want: nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := sortedPrincipals(iampolicy.AllowedPrincipals(mustParse(t, tc.doc), "123456789012"))
			if !slices.Equal(got, sortedPrincipals(tc.want)) {
				t.Errorf("AllowedPrincipals =\n  %+v\nwant\n  %+v", got, sortedPrincipals(tc.want))
			}
		})
	}
}

// A Deny with NotPrincipal applies to every principal except the ones it
// names (docs.aws.amazon.com/IAM/latest/UserGuide/reference_policies_elements_notprincipal.html).
// Beside a wildcard Allow it leaves access only to the named principals, so
// the grant is not public for the actions the Deny covers.
func TestEvaluateDenyNotPrincipalReachesEveryoneItDoesNotName(t *testing.T) {
	const spared = `{"AWS":"arn:aws:iam::123456789012:user/alice"}`
	cases := []struct {
		name        string
		doc         string
		wantPublic  bool
		wantActions []string
	}{
		{
			name: "deny everyone but alice on every action the allow grants",
			doc: `{"Version":"2012-10-17","Statement":[
				{"Effect":"Allow","Principal":"*","Action":["sqs:SendMessage","sqs:ReceiveMessage"],"Resource":"arn:aws:sqs:us-east-1:123456789012:acme-orders"},
				{"Effect":"Deny","NotPrincipal":` + spared + `,"Action":"sqs:*","Resource":"arn:aws:sqs:us-east-1:123456789012:acme-orders"}]}`,
			wantPublic: false,
		},
		{
			name: "deny everyone but alice on one action leaves the other public",
			doc: `{"Version":"2012-10-17","Statement":[
				{"Effect":"Allow","Principal":"*","Action":["sqs:SendMessage","sqs:ReceiveMessage"],"Resource":"arn:aws:sqs:us-east-1:123456789012:acme-orders"},
				{"Effect":"Deny","NotPrincipal":` + spared + `,"Action":"sqs:ReceiveMessage","Resource":"arn:aws:sqs:us-east-1:123456789012:acme-orders"}]}`,
			wantPublic:  true,
			wantActions: []string{"sqs:SendMessage"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ex := iampolicy.Evaluate(mustParse(t, tc.doc), "123456789012")
			if ex.Public != tc.wantPublic {
				t.Errorf("Public = %v, want %v", ex.Public, tc.wantPublic)
			}
			if !strSliceEqual(ex.PublicActions, tc.wantActions) {
				t.Errorf("PublicActions = %v, want %v", ex.PublicActions, tc.wantActions)
			}
		})
	}
}

func TestAllowedPrincipalsDenyNotPrincipalKeepsOnlyTheNamed(t *testing.T) {
	const allow = `{"Effect":"Allow","Principal":{"AWS":["arn:aws:iam::123456789012:user/alice","arn:aws:iam::123456789012:user/bob"]},"Action":"sts:AssumeRole"}`
	cases := []struct {
		name string
		deny string
		want []iampolicy.Principal
	}{
		{
			name: "the named principal is spared, the other is denied",
			deny: `{"Effect":"Deny","NotPrincipal":{"AWS":"arn:aws:iam::123456789012:user/alice"},"Action":"sts:AssumeRole"}`,
			want: []iampolicy.Principal{{Kind: "AWS", Value: "arn:aws:iam::123456789012:user/alice", Account: "123456789012"}},
		},
		{
			name: "naming a third principal denies both",
			deny: `{"Effect":"Deny","NotPrincipal":{"AWS":"arn:aws:iam::123456789012:user/carol"},"Action":"sts:AssumeRole"}`,
			want: nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := `{"Version":"2012-10-17","Statement":[` + allow + `,` + tc.deny + `]}`
			got := sortedPrincipals(iampolicy.AllowedPrincipals(mustParse(t, doc), "123456789012"))
			if !slices.Equal(got, sortedPrincipals(tc.want)) {
				t.Errorf("AllowedPrincipals =\n  %+v\nwant\n  %+v", got, sortedPrincipals(tc.want))
			}
		})
	}
}
