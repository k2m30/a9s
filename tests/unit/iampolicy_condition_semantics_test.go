package unit

import (
	"slices"
	"testing"

	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	"github.com/k2m30/a9s/v3/core/iampolicy"
	"github.com/k2m30/a9s/v3/core/resource"
)

const conditionQueueARN = "arn:aws:sqs:us-east-1:123456789012:acme-orders"

func allowAllSendUnder(condition string) string {
	return `{"Sid":"AllowSend","Effect":"Allow","Principal":"*","Action":"sqs:SendMessage","Resource":"` + conditionQueueARN + `"` +
		condition + `}`
}

func denyAllUnder(condition string) string {
	return `{"Sid":"Fence","Effect":"Deny","Principal":"*","Action":"sqs:*","Resource":"` + conditionQueueARN + `"` +
		condition + `}`
}

// A ForAllValues condition is true when the request does not carry the key
// at all, and an anonymous request carries no aws:SourceAccount or
// aws:PrincipalOrgID. A ForAnyValue condition is false when the key is
// absent, so a ForAnyValue negated Deny does not fire for an anonymous caller
// (docs.aws.amazon.com/IAM/latest/UserGuide/reference_policies_condition-single-vs-multi-valued-context-keys.html).
// Resource-side keys (aws:ResourceAccount, aws:ResourceOrgID,
// aws:ResourceOrgPaths, s3:ResourceAccount) describe the resource being
// called; on that resource's own policy they hold for every caller.
func TestEvaluateConditionOperatorsAndKeysThatScopeNobody(t *testing.T) {
	cases := []struct {
		name       string
		doc        string
		wantPublic bool
	}{
		{
			name:       "ForAllValues:StringEquals aws:SourceAccount on a wildcard allow",
			doc:        policyOf(allowAllSendUnder(`,"Condition":{"ForAllValues:StringEquals":{"aws:SourceAccount":"123456789012"}}`)),
			wantPublic: true,
		},
		{
			name:       "ForAllValues:StringEquals aws:PrincipalOrgID on a wildcard allow",
			doc:        policyOf(allowAllSendUnder(`,"Condition":{"ForAllValues:StringEquals":{"aws:PrincipalOrgID":["o-acme12345"]}}`)),
			wantPublic: true,
		},
		{
			name: "ForAnyValue:StringNotEquals aws:PrincipalOrgID deny beside a wildcard allow",
			doc: policyOf(allowAllSendUnder(""),
				denyAllUnder(`,"Condition":{"ForAnyValue:StringNotEquals":{"aws:PrincipalOrgID":["o-acme12345"]}}`)),
			wantPublic: true,
		},
		{
			name:       "aws:ResourceAccount equal to the owner on a wildcard allow",
			doc:        policyOf(allowAllSendUnder(`,"Condition":{"StringEquals":{"aws:ResourceAccount":"123456789012"}}`)),
			wantPublic: true,
		},
		{
			name:       "aws:ResourceOrgID equal to the owner's organisation on a wildcard allow",
			doc:        policyOf(allowAllSendUnder(`,"Condition":{"StringEquals":{"aws:ResourceOrgID":"o-acme12345"}}`)),
			wantPublic: true,
		},
		{
			name:       "aws:ResourceOrgPaths under the owner's organisation on a wildcard allow",
			doc:        policyOf(allowAllSendUnder(`,"Condition":{"ForAnyValue:StringLike":{"aws:ResourceOrgPaths":["o-acme12345/r-ab12/ou-ab12-11111111/*"]}}`)),
			wantPublic: true,
		},
		{
			name: "s3:ResourceAccount equal to the owner on a wildcard bucket grant",
			doc: `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":"*","Action":"s3:GetObject",` +
				`"Resource":"arn:aws:s3:::acme-assets/*","Condition":{"StringEquals":{"s3:ResourceAccount":"123456789012"}}}]}`,
			wantPublic: true,
		},
		{
			name: "a deny unless aws:ResourceAccount is the owner never fires on the owner's resource",
			doc: policyOf(allowAllSendUnder(""),
				denyAllUnder(`,"Condition":{"StringNotEquals":{"aws:ResourceAccount":"123456789012"}}`)),
			wantPublic: true,
		},
		{
			name:       "ForAnyValue:StringEquals aws:SourceAccount on a wildcard allow scopes it",
			doc:        policyOf(allowAllSendUnder(`,"Condition":{"ForAnyValue:StringEquals":{"aws:SourceAccount":"123456789012"}}`)),
			wantPublic: false,
		},
		{
			name: "ForAllValues:StringNotEquals aws:PrincipalOrgID deny fences a wildcard allow",
			doc: policyOf(allowAllSendUnder(""),
				denyAllUnder(`,"Condition":{"ForAllValues:StringNotEquals":{"aws:PrincipalOrgID":["o-acme12345"]}}`)),
			wantPublic: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := iampolicy.Evaluate(mustParse(t, tc.doc), "123456789012").Public; got != tc.wantPublic {
				t.Errorf("Public = %v, want %v", got, tc.wantPublic)
			}
		})
	}
}

func TestQueueAndKeyOpenUnderConditionsThatScopeNobodyArePublic(t *testing.T) {
	callers := map[string]policyCaller{}
	for _, c := range policyCallers() {
		callers[c.shortName] = c
	}
	sqsCaller, kmsCaller := callers["sqs"], callers["kms"]

	for name, doc := range map[string]string{
		"ForAllValues:StringEquals aws:SourceAccount": policyOf(
			allowAllSendUnder(`,"Condition":{"ForAllValues:StringEquals":{"aws:SourceAccount":"123456789012"}}`)),
		"ForAnyValue:StringNotEquals aws:PrincipalOrgID deny": policyOf(allowAllSendUnder(""),
			denyAllUnder(`,"Condition":{"ForAnyValue:StringNotEquals":{"aws:PrincipalOrgID":["o-acme12345"]}}`)),
		"aws:ResourceAccount": policyOf(
			allowAllSendUnder(`,"Condition":{"StringEquals":{"aws:ResourceAccount":"123456789012"}}`)),
	} {
		if fs, _ := sqsCaller.run(t, doc); !hasCode(fs, sqsCaller.code) {
			t.Errorf("sqs %s: a queue anyone can send to raised no %s; got %v", name, sqsCaller.code, w2Codes(fs))
		}
	}

	key := policyOf(
		`{"Sid":"Enable IAM User Permissions","Effect":"Allow","Principal":{"AWS":"arn:aws:iam::123456789012:root"},"Action":"kms:*","Resource":"*"}`,
		`{"Sid":"AllowUse","Effect":"Allow","Principal":{"AWS":"*"},"Action":["kms:Encrypt","kms:Decrypt"],"Resource":"*",`+
			`"Condition":{"StringEquals":{"aws:ResourceOrgID":"o-acme12345"}}}`)
	if fs, _ := kmsCaller.run(t, key); !hasCode(fs, kmsCaller.code) {
		t.Errorf("kms aws:ResourceOrgID: a key anyone can use raised no %s; got %v", kmsCaller.code, w2Codes(fs))
	}
}

// A VPC endpoint policy governs the requests that leave through the endpoint
// to other resources, so a resource-side key there does narrow what the
// endpoint reaches.
func TestVPCEndpointPolicyScopedByResourceAccountIsNotOpen(t *testing.T) {
	doc := `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":"*","Action":"s3:*","Resource":"*",` +
		`"Condition":{"StringEquals":{"aws:ResourceAccount":"123456789012"}}}]}`
	if fs := d3FetchVPCE(t, doc).Findings; hasCode(fs, "vpce.policy-open") {
		t.Errorf("an endpoint policy limited to the owner's own resources raised vpce.policy-open")
	}
}

func TestRoleTrustUnderConditionsThatScopeNobodyIsWildcard(t *testing.T) {
	for name, cond := range map[string]string{
		"ForAllValues:StringEquals aws:PrincipalOrgID": `{"ForAllValues:StringEquals":{"aws:PrincipalOrgID":["o-acme12345"]}}`,
		"aws:ResourceAccount":                          `{"StringEquals":{"aws:ResourceAccount":"123456789012"}}`,
	} {
		trust := policyOf(`{"Effect":"Allow","Principal":{"AWS":"*"},"Action":"sts:AssumeRole","Condition":` + cond + `}`)
		role := w4FetchRole(t, &w4RoleListFake{roles: []iamtypes.Role{w4Role("acme-partner-access", "/", trust)}}, "acme-partner-access")
		if !hasCode(role.Findings, "role.trust.wildcard-principal") {
			t.Errorf("%s: a trust policy any AWS principal satisfies raised no role.trust.wildcard-principal; got %v",
				name, w2Codes(role.Findings))
		}
	}
}

// For a REST API method with no IAM authorization, API Gateway admits a caller
// only when the resource policy explicitly allows it and no Deny matches.
func TestAPIGatewayPolicyWithNoSurvivingWildcardGrantIsScoped(t *testing.T) {
	const api = "arn:aws:execute-api:eu-central-1:123456789012:rst556scp/*"
	cases := []struct {
		name        string
		policy      string
		wantFinding bool
	}{
		{
			name:   "a named account only",
			policy: policyOf(`{"Effect":"Allow","Principal":{"AWS":"arn:aws:iam::210987654321:root"},"Action":"execute-api:Invoke","Resource":"` + api + `"}`),
		},
		{
			name:   "deny only",
			policy: policyOf(`{"Effect":"Deny","Principal":"*","Action":"execute-api:Invoke","Resource":"` + api + `","Condition":{"NotIpAddress":{"aws:SourceIp":"203.0.113.0/24"}}}`),
		},
		{
			name: "a wildcard allow removed by an unconditional deny",
			policy: policyOf(
				`{"Effect":"Allow","Principal":"*","Action":"execute-api:Invoke","Resource":"`+api+`"}`,
				`{"Effect":"Deny","Principal":"*","Action":"execute-api:Invoke","Resource":"`+api+`"}`),
		},
		{
			name:        "an unscoped wildcard allow",
			policy:      policyOf(`{"Effect":"Allow","Principal":"*","Action":"execute-api:Invoke","Resource":"` + api + `"}`),
			wantFinding: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fs, unknown := enrichRESTAPIPolicy(t, "rst556scp", tc.policy)
			if unknown {
				t.Fatalf("a readable policy left the API unknown")
			}
			public, private := hasCode(fs, "apigw.no-authorizer-public"), hasCode(fs, "apigw.no-authorizer")
			if tc.wantFinding && !public {
				t.Errorf("an unscoped wildcard grant raised no apigw.no-authorizer-public; got %v", w2Codes(fs))
			}
			if !tc.wantFinding && (public || private) {
				t.Errorf("a policy no anonymous caller satisfies raised %v", w2Codes(fs))
			}
		})
	}
}

// A Deny with NotAction applies to every action except the ones it lists, so
// Deny NotAction [s3:*, logs:*] removes every iam action a wildcard Allow
// grants.
func TestDenyNotActionRemovesEscalationCombos(t *testing.T) {
	const guardrail = `{"Sid":"OnlyS3AndLogs","Effect":"Deny","NotAction":["s3:*","logs:*"],"Resource":"*"}`
	inline := policyOf(`{"Sid":"All","Effect":"Allow","Action":"*","Resource":"*"}`, guardrail)
	role := w4FetchRole(t, &w4RoleListFake{
		roles:  []iamtypes.Role{w4Role("acme-etl", "/", w4TrustLambdaNoSourceScope)},
		inline: map[string]map[string]string{"acme-etl": {"acme-etl-guarded": inline}},
	}, "acme-etl")
	if hasCode(role.Findings, "role.inline-privilege-escalation") {
		t.Errorf("role inline: a policy that denies every action but s3 and logs raised role.inline-privilege-escalation: %v",
			role.AttentionDetails["role.inline-privilege-escalation"].Rows)
	}

	arn := w4CustomerPolicyARN("acme-guarded-ops")
	managed := policyOf(`{"Sid":"Ops","Effect":"Allow","Action":["iam:*","s3:*","logs:*"],"Resource":"*"}`, guardrail)
	res := w4EnrichPolicies(t, &w4PolicyFake{docs: map[string]string{arn: managed}},
		[]resource.Resource{w4PolicyResource("acme-guarded-ops", arn)})
	if hasCode(res.Findings[arn], "policy.privilege-escalation") {
		t.Errorf("managed policy: a policy that denies every action but s3 and logs raised policy.privilege-escalation: %v",
			res.AttentionDetails[arn]["policy.privilege-escalation"].Rows)
	}
}

// An Allow with NotAction grants every action it does not list; only the
// concrete combo actions count as granted by it, so the wildcard-action
// combos are not read into it.
func TestAllowNotActionEscalationReadingIsConcreteOnly(t *testing.T) {
	combos := mustParse(t, policyOf(`{"Effect":"Allow","NotAction":["s3:*"],"Resource":"*"}`)).PrivilegeEscalation()
	if !slices.Contains(combos, "AssumeRole+PutRolePolicy") {
		t.Errorf("Allow NotAction [s3:*] grants iam:PutRolePolicy and sts:AssumeRole, yet AssumeRole+PutRolePolicy is not reported; got %v", combos)
	}
	for _, wildcard := range []string{"IAMPut", "OverPermissiveIAM"} {
		if slices.Contains(combos, wildcard) {
			t.Errorf("Allow NotAction [s3:*] reported the wildcard-action combo %s", wildcard)
		}
	}
}

const (
	adminStatement      = `{"Sid":"All","Effect":"Allow","Action":"*","Resource":"*"}`
	onlyS3AndLogsDeny   = `{"Sid":"OnlyS3AndLogs","Effect":"Deny","NotAction":["s3:*","logs:*"],"Resource":"*"}`
	noBucketDeletesDeny = `{"Sid":"NoBucketDeletes","Effect":"Deny","Action":"s3:DeleteBucket","Resource":"*"}`
)

// An explicit Deny removes the actions it matches from every Allow, so Allow *
// on * beside any unconditional Deny no longer grants every action.
func TestIsAdminAppliesExplicitDeny(t *testing.T) {
	cases := []struct {
		name string
		doc  string
		want bool
	}{
		{"allow everything", policyOf(adminStatement), true},
		{"allow everything, deny all but s3 and logs", policyOf(adminStatement, onlyS3AndLogsDeny), false},
		{"allow everything, deny bucket deletion", policyOf(adminStatement, noBucketDeletesDeny), false},
	}
	for _, tc := range cases {
		if got := mustParse(t, tc.doc).IsAdmin(); got != tc.want {
			t.Errorf("%s: IsAdmin = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestManagedPolicyAdminVerdictAppliesExplicitDeny(t *testing.T) {
	docs := map[string]string{
		"acme-admin":            policyOf(adminStatement),
		"acme-s3-and-logs-only": policyOf(adminStatement, onlyS3AndLogsDeny),
		"acme-admin-no-deletes": policyOf(adminStatement, noBucketDeletesDeny),
	}
	fake := &w4PolicyFake{docs: map[string]string{}}
	var rows []resource.Resource
	for name, doc := range docs {
		arn := w4CustomerPolicyARN(name)
		fake.docs[arn] = doc
		rows = append(rows, w4PolicyResource(name, arn))
	}
	res := w4EnrichPolicies(t, fake, rows)
	codes := func(name string) []string { return w2Codes(res.Findings[w4CustomerPolicyARN(name)]) }

	if !slices.Equal(codes("acme-admin"), []string{"iam-policy.admin-star"}) {
		t.Errorf("Allow * on *: findings %v, want [iam-policy.admin-star]", codes("acme-admin"))
	}
	if got := codes("acme-s3-and-logs-only"); len(got) != 0 {
		t.Errorf("Allow * on * with every action but s3 and logs denied: findings %v, want none", got)
	}
	if got := codes("acme-admin-no-deletes"); !slices.Equal(got, []string{"policy.privilege-escalation"}) {
		t.Errorf("Allow * on * less s3:DeleteBucket: findings %v, want [policy.privilege-escalation] (not full admin, still able to escalate)", got)
	}
}
