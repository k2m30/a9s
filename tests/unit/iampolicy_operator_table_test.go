// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package unit

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/iampolicy"
)

// condCase is one row of the operator table that decides, for both the
// public-exposure verdict and the confused-deputy verdict, whether a
// condition positively narrows who may call. An operator narrows only when
// it asserts that a supported scoping key equals/matches a concrete value:
// Null asserts the key is *absent*, a negated operator asserts the key is
// anything *but* the listed value, IfExists lets a caller omit the key
// entirely, and an operator IAM does not define cannot be assumed to
// constrain anything. None of those narrow the principal.
type condCase struct {
	name string
	// cond is the JSON value of the statement's Condition block.
	cond string
	// restricts is true when the condition genuinely narrows the caller.
	restricts bool
	// sourceScope marks the cases whose key is a confused-deputy source
	// key, i.e. the cases the service-trust path must agree on.
	sourceScope bool
}

var operatorTable = []condCase{
	{
		name:        "Null on SourceArn requires the key to be absent",
		cond:        `{"Null":{"aws:SourceArn":"true"}}`,
		sourceScope: true,
	},
	{
		name:        "Null on SourceAccount",
		cond:        `{"Null":{"aws:SourceAccount":"false"}}`,
		sourceScope: true,
	},
	{
		name: "Null on SourceIp",
		cond: `{"Null":{"aws:SourceIp":"false"}}`,
	},
	{
		name:        "StringNotEquals on SourceArn allows every other ARN",
		cond:        `{"StringNotEquals":{"aws:SourceArn":"arn:aws:s3:::example-bucket"}}`,
		sourceScope: true,
	},
	{
		name:        "ArnNotLike on SourceArn",
		cond:        `{"ArnNotLike":{"aws:SourceArn":"arn:aws:s3:::example-bucket"}}`,
		sourceScope: true,
	},
	{
		name: "NotIpAddress on SourceIp",
		cond: `{"NotIpAddress":{"aws:SourceIp":"10.0.0.0/8"}}`,
	},
	{
		name: "Bool on SecureTransport constrains the channel, not the caller",
		cond: `{"Bool":{"aws:SecureTransport":"true"}}`,
	},
	{
		name:        "IfExists lets the caller omit the key",
		cond:        `{"StringEqualsIfExists":{"aws:SourceAccount":"123456789012"}}`,
		sourceScope: true,
	},
	{
		name:        "an operator IAM does not define",
		cond:        `{"UnknownFutureOperator":{"aws:SourceArn":"arn:aws:s3:::example-bucket"}}`,
		sourceScope: true,
	},
	{
		name:        "StringEquals on SourceArn with a wildcard value matches every ARN",
		cond:        `{"StringEquals":{"aws:SourceArn":"*"}}`,
		sourceScope: true,
	},
	{
		name:        "StringEquals on SourceAccount",
		cond:        `{"StringEquals":{"aws:SourceAccount":"123456789012"}}`,
		restricts:   true,
		sourceScope: true,
	},
	{
		name:        "ArnLike on SourceArn",
		cond:        `{"ArnLike":{"aws:SourceArn":"arn:aws:s3:::example-bucket"}}`,
		restricts:   true,
		sourceScope: true,
	},
	{
		name:      "ForAllValues:StringEquals on PrincipalOrgID",
		cond:      `{"ForAllValues:StringEquals":{"aws:PrincipalOrgID":"o-abc123"}}`,
		restricts: true,
	},
	{
		name:      "ForAnyValue:StringEquals on PrincipalOrgID",
		cond:      `{"ForAnyValue:StringEquals":{"aws:PrincipalOrgID":"o-abc123"}}`,
		restricts: true,
	},
	{
		name:      "IpAddress on a scoped SourceIp range",
		cond:      `{"IpAddress":{"aws:SourceIp":"203.0.113.0/24"}}`,
		restricts: true,
	},
	{
		name: "StringEquals on SourceIp compares an address to a literal, not to a range",
		cond: `{"StringEquals":{"aws:SourceIp":"203.0.113.10"}}`,
	},
	{
		name:        "StringEqualsIgnoreCase on SourceAccount",
		cond:        `{"StringEqualsIgnoreCase":{"aws:SourceAccount":"123456789012"}}`,
		restricts:   true,
		sourceScope: true,
	},
	{
		name:        "StringEquals on SourceOrgID",
		cond:        `{"StringEquals":{"aws:SourceOrgID":"o-abc123"}}`,
		restricts:   true,
		sourceScope: true,
	},
	{
		name:        "a SourceArn value whose letters spell the ARN scaffolding is still a concrete value",
		cond:        `{"StringEquals":{"aws:SourceArn":"arnaws"}}`,
		restricts:   true,
		sourceScope: true,
	},
	{
		name:        "ArnLike on SourceArn with a bare partition wildcard matches every ARN",
		cond:        `{"ArnLike":{"aws:SourceArn":"arn:*"}}`,
		sourceScope: true,
	},
	{
		name:        "a set prefix IAM does not define is an unknown operator",
		cond:        `{"Bogus:StringEquals":{"aws:SourceArn":"arn:aws:s3:::example-bucket"}}`,
		sourceScope: true,
	},
	{
		name: "FunctionUrlAuthType NONE lets anyone invoke the URL",
		cond: `{"StringEquals":{"lambda:FunctionUrlAuthType":"NONE"}}`,
	},
	{
		name:      "FunctionUrlAuthType AWS_IAM demands a signed request",
		cond:      `{"StringEquals":{"lambda:FunctionUrlAuthType":"AWS_IAM"}}`,
		restricts: true,
	},
	{
		name: "ViaService names the service the request comes through, not the account behind it",
		cond: `{"StringEquals":{"kms:ViaService":"s3.us-east-1.amazonaws.com"}}`,
	},
	{
		name: "Endpoint names where a notification is delivered, not who may subscribe",
		cond: `{"StringEquals":{"sns:Endpoint":"https://hooks.example.com/sns"}}`,
	},
	{
		name:      "the default KMS key policy pairs ViaService with CallerAccount, which does scope",
		cond:      `{"StringEquals":{"kms:CallerAccount":"123456789012","kms:ViaService":"s3.us-east-1.amazonaws.com"}}`,
		restricts: true,
	},
}

// A wildcard-principal Allow is public unless its condition positively
// narrows who may call. Anything short of that leaves the grant open to
// every AWS caller, so it must be reported public rather than conditioned.
func TestEvaluate_OperatorTableDecidesRestrictiveness(t *testing.T) {
	for _, tt := range operatorTable {
		t.Run(tt.name, func(t *testing.T) {
			doc := `{"Statement":{"Effect":"Allow","Principal":{"AWS":"*"},"Action":"s3:GetObject","Resource":"arn:aws:s3:::example-bucket/*","Condition":` + tt.cond + `}}`
			ex := iampolicy.Evaluate(mustParse(t, doc), "123456789012")
			if ex.Public == tt.restricts {
				t.Errorf("Public = %v, want %v for Condition %s", ex.Public, !tt.restricts, tt.cond)
			}
			if ex.Conditioned != tt.restricts {
				t.Errorf("Conditioned = %v, want %v for Condition %s", ex.Conditioned, tt.restricts, tt.cond)
			}
		})
	}
}

// A service principal is protected against the confused-deputy problem only
// when the source-scope key is positively constrained. Presence of the key
// under Null or a negated operator protects nothing, so the same table that
// decides public exposure must decide this verdict too.
func TestHasServicePrincipalWithoutSourceScope_OperatorTableDecidesScoping(t *testing.T) {
	for _, tt := range operatorTable {
		if !tt.sourceScope {
			continue
		}
		t.Run(tt.name, func(t *testing.T) {
			doc := `{"Statement":{"Effect":"Allow","Principal":{"Service":"lambda.amazonaws.com"},"Action":"sts:AssumeRole","Condition":` + tt.cond + `}}`
			var want []string
			if !tt.restricts {
				want = []string{"lambda"}
			}
			got := mustParse(t, doc).HasServicePrincipalWithoutSourceScope()
			if !strSliceEqual(got, want) {
				t.Errorf("HasServicePrincipalWithoutSourceScope() = %v, want %v for Condition %s", got, want, tt.cond)
			}
		})
	}
}

// The source-scope keys are the ones naming the account or resource a
// service acts on behalf of. Other keys narrow a wildcard principal on a
// resource policy without answering the confused-deputy question at all: a
// service calls the role from AWS-owned addresses whoever asked it to, and
// an organisation ID says nothing about which of that organisation's
// resources triggered the call. These are the cases where the two lanes must
// answer differently, which is why they cannot be rows of the shared table.
//
// Together with the shared table's three source-key rows, which pin that
// each source key scopes on both lanes, this fixes the membership of the
// derived source list from outside the package in both directions.
func TestHasServicePrincipalWithoutSourceScope_CallerKeysAreNotSourceScoping(t *testing.T) {
	conds := []string{
		`{"IpAddress":{"aws:SourceIp":"203.0.113.0/24"}}`,
		`{"StringEquals":{"aws:PrincipalOrgID":"o-abc123"}}`,
	}
	for _, cond := range conds {
		t.Run(cond, func(t *testing.T) {
			doc := `{"Statement":{"Effect":"Allow","Principal":{"Service":"lambda.amazonaws.com"},"Action":"sts:AssumeRole","Condition":` + cond + `}}`
			got := mustParse(t, doc).HasServicePrincipalWithoutSourceScope()
			if !strSliceEqual(got, []string{"lambda"}) {
				t.Errorf("HasServicePrincipalWithoutSourceScope() = %v, want [lambda] for %s", got, cond)
			}
		})
	}
}

// sts:ExternalId is restrictive only on a trust policy, and only when it is
// positively constrained. Null on it demands the caller pass no external ID
// at all, which is the opposite of the scoping pattern it stands for.
func TestEvaluateTrust_ExternalIDOperatorAware(t *testing.T) {
	tests := []struct {
		name       string
		cond       string
		wantPublic bool
	}{
		{
			name:       "StringEquals on sts:ExternalId scopes the trust",
			cond:       `{"StringEquals":{"sts:ExternalId":"a9s-external-id"}}`,
			wantPublic: false,
		},
		{
			name:       "Null on sts:ExternalId scopes nothing",
			cond:       `{"Null":{"sts:ExternalId":"true"}}`,
			wantPublic: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := `{"Statement":{"Effect":"Allow","Principal":{"AWS":"*"},"Action":"sts:AssumeRole","Condition":` + tt.cond + `}}`
			ex := iampolicy.EvaluateTrust(mustParse(t, doc), "123456789012")
			if ex.Public != tt.wantPublic {
				t.Errorf("EvaluateTrust Public = %v, want %v", ex.Public, tt.wantPublic)
			}
			if ex.Conditioned == tt.wantPublic {
				t.Errorf("EvaluateTrust Conditioned = %v, want %v", ex.Conditioned, !tt.wantPublic)
			}
		})
	}
}

// An IAM ARN naming account "*" and resource "root" is every account's root
// user in whichever partition it names. GovCloud and China policies carry
// their own partition, so recognising only the commercial one lets an
// equally public grant read as private.
func TestEvaluate_WildcardRootPrincipalInEveryPartition(t *testing.T) {
	tests := []struct {
		name       string
		principal  string
		wantPublic bool
		wantCross  []string
	}{
		{
			name:       "commercial partition",
			principal:  "arn:aws:iam::*:root",
			wantPublic: true,
		},
		{
			name:       "china partition",
			principal:  "arn:aws-cn:iam::*:root",
			wantPublic: true,
		},
		{
			name:       "govcloud partition",
			principal:  "arn:aws-us-gov:iam::*:root",
			wantPublic: true,
		},
		{
			name:       "isolated partition",
			principal:  "arn:aws-iso-b:iam::*:root",
			wantPublic: true,
		},
		{
			name:      "named account root is one account, not everyone",
			principal: "arn:aws:iam::210987654321:root",
			wantCross: []string{"210987654321"},
		},
		{
			name:      "named account root in the china partition",
			principal: "arn:aws-cn:iam::210987654321:root",
			wantCross: []string{"210987654321"},
		},
		{
			name:      "wildcard account but a named user, not root",
			principal: "arn:aws:iam::*:user/admin",
		},
		{
			name:      "wildcard resource on a non-iam service",
			principal: "arn:aws:s3:::*",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := `{"Statement":{"Effect":"Allow","Principal":{"AWS":"` + tt.principal + `"},"Action":"s3:GetObject","Resource":"arn:aws:s3:::example-bucket/*"}}`
			ex := iampolicy.Evaluate(mustParse(t, doc), "123456789012")
			if ex.Public != tt.wantPublic {
				t.Errorf("Public = %v, want %v for principal %s", ex.Public, tt.wantPublic, tt.principal)
			}
			if !strSliceEqual(ex.CrossAccount, tt.wantCross) {
				t.Errorf("CrossAccount = %v, want %v for principal %s", ex.CrossAccount, tt.wantCross, tt.principal)
			}
		})
	}
}

// pw1FunctionURLPolicy is the resource policy AWS writes when a function URL
// is created with the given auth type: a wildcard principal narrowed only by
// lambda:FunctionUrlAuthType. With NONE the grant is what it says, an
// endpoint anyone may invoke, and the enricher that reads it through the
// engine has to say so.
func pw1FunctionURLPolicy(name, authType string) string {
	return `{"Version":"2012-10-17","Id":"default","Statement":[{"Sid":"FunctionURLAllowPublicAccess",` +
		`"Effect":"Allow","Principal":"*","Action":"lambda:InvokeFunctionUrl",` +
		`"Resource":"arn:aws:lambda:us-east-1:123456789012:function:` + name + `",` +
		`"Condition":{"StringEquals":{"lambda:FunctionUrlAuthType":"` + authType + `"}}}]}`
}

func TestLambda_PublicPolicy_FunctionURLAuthTypeNoneIsPublic(t *testing.T) {
	const name = "acme-url-policy-none"
	fake := &pw1LambdaPostureFake{policies: map[string]string{name: pw1FunctionURLPolicy(name, "NONE")}}
	res := pw1EnrichLambda(t, fake, name)
	pw1RequireFinding(t, res.Findings[name], pw1LambdaCodePublicPolicy,
		"invokable by anyone", domain.SevBroken, "wave2:lambda")
}

func TestLambda_PublicPolicy_FunctionURLAuthTypeIAMIsHealthy(t *testing.T) {
	const name = "acme-url-policy-iam"
	fake := &pw1LambdaPostureFake{policies: map[string]string{name: pw1FunctionURLPolicy(name, "AWS_IAM")}}
	res := pw1EnrichLambda(t, fake, name)
	pw1RequireNoFinding(t, res.Findings[name], pw1LambdaCodePublicPolicy)
}
