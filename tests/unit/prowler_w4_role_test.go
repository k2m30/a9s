package unit

// prowler_w4_role_test.go — behavioural tests for the batch-w4 role rows:
// the trust-policy verdict migration onto core/iampolicy, the
// confused-deputy trust finding, the inline privilege-escalation finding,
// and the admin-policy-attached wave-2 finding.

import (
	"context"
	"errors"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/iampolicy"
	"github.com/k2m30/a9s/v3/core/resource"
)

const (
	w4CodeRoleWildcardTrust  domain.FindingCode = "role.trust.wildcard-principal"
	w4CodeRoleConfusedDeputy domain.FindingCode = "role.trust.confused-deputy"
	w4CodeRoleInlinePrivEsc  domain.FindingCode = "role.inline-privilege-escalation"
	w4CodeRoleAdminAttached  domain.FindingCode = "role.admin-attached"

	w4PhraseRoleWildcardTrust  = "anyone can assume this role"
	w4PhraseRoleConfusedDeputy = "service can assume without source scoping"
	w4PhraseRoleAdminAttached  = "has an administrator policy"

	// The role enricher stamps its Wave-2 findings with the "iam-role"
	// short name; a second finding from the same enricher keeps that
	// provenance rather than inventing a second one for the same type.
	w4SourceRoleWave2 = "wave2:iam-role"

	w4AdminAccessARN     = "arn:aws:iam::aws:policy/AdministratorAccess"
	w4PowerUserAccessARN = "arn:aws:iam::aws:policy/PowerUserAccess"
)

// --- trust policy documents -------------------------------------------------

// The bare-string principal form. AWS returns it verbatim for
// {"Principal": "*"}; a typed unmarshal into a struct whose Principal is an
// object cannot see it, which is why the pre-migration code needed a
// substring check alongside the parse.
const w4TrustBareStarNoCondition = `{"Version":"2012-10-17","Statement":[` +
	`{"Effect":"Allow","Principal":"*","Action":"sts:AssumeRole"}]}`

const w4TrustBareStarWithExternalID = `{"Version":"2012-10-17","Statement":[` +
	`{"Effect":"Allow","Principal":"*","Action":"sts:AssumeRole",` +
	`"Condition":{"StringEquals":{"sts:ExternalId":"acme-shared-external-id"}}}]}`

const w4TrustObjectStarNoCondition = `{"Version":"2012-10-17","Statement":[` +
	`{"Effect":"Allow","Principal":{"AWS":"*"},"Action":"sts:AssumeRole"}]}`

const w4TrustObjectStarWithExternalID = `{"Version":"2012-10-17","Statement":[` +
	`{"Effect":"Allow","Principal":{"AWS":"*"},"Action":"sts:AssumeRole",` +
	`"Condition":{"StringEquals":{"sts:ExternalId":"acme-shared-external-id"}}}]}`

// A wildcard principal narrowed to the organisation. Restrictive under the
// shared evaluator, invisible to a check that only knows sts:ExternalId.
const w4TrustBareStarWithOrgID = `{"Version":"2012-10-17","Statement":[` +
	`{"Effect":"Allow","Principal":"*","Action":"sts:AssumeRole",` +
	`"Condition":{"StringEquals":{"aws:PrincipalOrgID":"o-acme12345"}}}]}`

// Statement is a single object rather than an array — legal IAM, and the
// shape a hand-written trust policy often takes.
const w4TrustStatementObjectStar = `{"Version":"2012-10-17","Statement":` +
	`{"Effect":"Allow","Principal":{"AWS":"*"},"Action":"sts:AssumeRole"}}`

const w4TrustLambdaNoSourceScope = `{"Version":"2012-10-17","Statement":[` +
	`{"Effect":"Allow","Principal":{"Service":"lambda.amazonaws.com"},"Action":"sts:AssumeRole"}]}`

const w4TrustLambdaWithSourceAccount = `{"Version":"2012-10-17","Statement":[` +
	`{"Effect":"Allow","Principal":{"Service":"lambda.amazonaws.com"},"Action":"sts:AssumeRole",` +
	`"Condition":{"StringEquals":{"aws:SourceAccount":"123456789012"}}}]}`

const w4TrustEC2Only = `{"Version":"2012-10-17","Statement":[` +
	`{"Effect":"Allow","Principal":{"Service":"ec2.amazonaws.com"},"Action":"sts:AssumeRole"}]}`

const w4TrustEC2AndLambda = `{"Version":"2012-10-17","Statement":[` +
	`{"Effect":"Allow","Principal":{"Service":["ec2.amazonaws.com","lambda.amazonaws.com"]},` +
	`"Action":"sts:AssumeRole"}]}`

// Both a wildcard AWS principal and an unscoped service principal, in two
// statements of one document.
const w4TrustStarAndLambda = `{"Version":"2012-10-17","Statement":[` +
	`{"Effect":"Allow","Principal":{"AWS":"*"},"Action":"sts:AssumeRole"},` +
	`{"Effect":"Allow","Principal":{"Service":"lambda.amazonaws.com"},"Action":"sts:AssumeRole"}]}`

// --- inline policy documents ------------------------------------------------

const w4InlinePrivEscDoc = `{"Version":"2012-10-17","Statement":[` +
	`{"Effect":"Allow","Action":"iam:CreateLoginProfile","Resource":"*"}]}`

const w4InlineBenignDoc = `{"Version":"2012-10-17","Statement":[` +
	`{"Effect":"Allow","Action":["s3:GetObject","s3:ListBucket"],"Resource":"arn:aws:s3:::acme-reports/*"}]}`

// --- fakes ------------------------------------------------------------------

// w4RoleListFake serves ListRoles plus the two optional inline-policy
// interfaces FetchIAMRolesPage type-asserts on.
type w4RoleListFake struct {
	awsclient.IAMAPI
	roles []iamtypes.Role
	// inline maps RoleName → PolicyName → document.
	inline map[string]map[string]string
	// listErr, when set for a role name, fails ListRolePolicies for it.
	listErr map[string]error
}

func (f *w4RoleListFake) ListRoles(
	_ context.Context, _ *iam.ListRolesInput, _ ...func(*iam.Options),
) (*iam.ListRolesOutput, error) {
	return &iam.ListRolesOutput{Roles: f.roles}, nil
}

func (f *w4RoleListFake) ListRolePolicies(
	_ context.Context, in *iam.ListRolePoliciesInput, _ ...func(*iam.Options),
) (*iam.ListRolePoliciesOutput, error) {
	name := aws.ToString(in.RoleName)
	if err, ok := f.listErr[name]; ok {
		return nil, err
	}
	var names []string
	for policyName := range f.inline[name] {
		names = append(names, policyName)
	}
	return &iam.ListRolePoliciesOutput{PolicyNames: names}, nil
}

func (f *w4RoleListFake) GetRolePolicy(
	_ context.Context, in *iam.GetRolePolicyInput, _ ...func(*iam.Options),
) (*iam.GetRolePolicyOutput, error) {
	doc, ok := f.inline[aws.ToString(in.RoleName)][aws.ToString(in.PolicyName)]
	if !ok {
		return nil, errors.New("NoSuchEntity: inline policy not found")
	}
	return &iam.GetRolePolicyOutput{
		RoleName:   in.RoleName,
		PolicyName: in.PolicyName,
		// AWS returns the inline document URL-encoded.
		PolicyDocument: aws.String(url.QueryEscape(doc)),
	}, nil
}

var _ awsclient.IAMAPI = (*w4RoleListFake)(nil)

// w4Role builds a ListRoles entry with a URL-encoded trust document, the
// shape the IAM API actually returns.
func w4Role(name, path, trustDoc string) iamtypes.Role {
	created := time.Now().Add(-365 * 24 * time.Hour)
	return iamtypes.Role{
		RoleName:                 aws.String(name),
		RoleId:                   aws.String("AROA" + strings.ToUpper(name)),
		Arn:                      aws.String("arn:aws:iam::123456789012:role" + path + name),
		Path:                     aws.String(path),
		CreateDate:               &created,
		AssumeRolePolicyDocument: aws.String(url.QueryEscape(trustDoc)),
	}
}

// w4FetchRole runs the real role fetcher over one role and returns it.
func w4FetchRole(t *testing.T, fake *w4RoleListFake, name string) resource.Resource {
	t.Helper()
	res, err := awsclient.FetchIAMRolesPage(context.Background(), fake, "")
	if err != nil {
		t.Fatalf("FetchIAMRolesPage: %v", err)
	}
	for _, r := range res.Resources {
		if r.ID == name {
			return r
		}
	}
	t.Fatalf("role %q not in fetch result", name)
	return resource.Resource{}
}

// --- row 1: the trust verdict comes from iampolicy -------------------------

// TestW4RoleWildcardTrustVerdict pins which trust policies are "anyone can
// assume this role". The verdict is the shared policy evaluator's, so a
// wildcard principal narrowed by any restrictive condition — sts:ExternalId
// on a trust policy, aws:PrincipalOrgID anywhere — is a scoped grant and not
// a finding, whichever JSON shape the principal is written in.
func TestW4RoleWildcardTrustVerdict(t *testing.T) {
	cases := []struct {
		name string
		doc  string
		want bool
	}{
		{"bare_star_no_condition", w4TrustBareStarNoCondition, true},
		{"object_star_no_condition", w4TrustObjectStarNoCondition, true},
		{"statement_as_object", w4TrustStatementObjectStar, true},
		{"bare_star_external_id", w4TrustBareStarWithExternalID, false},
		{"object_star_external_id", w4TrustObjectStarWithExternalID, false},
		{"bare_star_org_id", w4TrustBareStarWithOrgID, false},
		{"service_principal_only", w4TrustLambdaNoSourceScope, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := &w4RoleListFake{roles: []iamtypes.Role{w4Role("acme-trust-role", "/", tc.doc)}}
			r := w4FetchRole(t, fake, "acme-trust-role")
			if !tc.want {
				w4AssertNoCode(t, r.Findings, w4CodeRoleWildcardTrust)
				if got := resource.FindResourceType("role").ResolveColor(r); got == domain.ColorBroken {
					t.Errorf("color = Broken for a scoped trust policy")
				}
				return
			}
			w4AssertFinding(t, r.Findings, w4CodeRoleWildcardTrust,
				w4PhraseRoleWildcardTrust, domain.SevBroken, "wave1")
			if got := resource.FindResourceType("role").ResolveColor(r); got != domain.ColorBroken {
				t.Errorf("color = %v, want Broken", got)
			}
		})
	}
}

// TestW4ColorRoleHasNoRawDocumentFallback pins that the role row color is
// derived from findings alone. A resource carrying a raw trust document with
// a wildcard principal but no finding is a resource nothing flagged — the
// classifier must not re-derive the verdict from the field text and disagree
// with the Findings list.
func TestW4ColorRoleHasNoRawDocumentFallback(t *testing.T) {
	td := resource.FindResourceType("role")
	if td == nil {
		t.Fatal("role type not registered")
	}
	r := domain.Resource{
		ID:   "acme-no-finding-role",
		Name: "acme-no-finding-role",
		Type: "role",
		Fields: map[string]string{
			"role_name":                   "acme-no-finding-role",
			"assume_role_policy_document": w4TrustBareStarNoCondition,
		},
	}
	if got := td.ResolveColor(r); got != domain.ColorHealthy {
		t.Fatalf("color = %v, want Healthy: no finding means nothing to color", got)
	}
}

// TestW4TrustExternalIDIsTrustOnly pins that widening sts:ExternalId to
// "restrictive" applies to trust policies only. On a bucket or key policy the
// same key scopes nothing, so the resource-policy verdict must stay public.
func TestW4TrustExternalIDIsTrustOnly(t *testing.T) {
	doc, err := iampolicy.Parse(w4TrustObjectStarWithExternalID)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if ex := iampolicy.Evaluate(doc, ""); !ex.Public {
		t.Errorf("Evaluate().Public = false; sts:ExternalId must not scope a resource policy")
	}
	ex := iampolicy.EvaluateTrust(doc, "")
	if ex.Public {
		t.Errorf("EvaluateTrust().Public = true; sts:ExternalId scopes a trust policy")
	}
	if !ex.Conditioned {
		t.Errorf("EvaluateTrust().Conditioned = false, want true")
	}
}

// TestW4NoPrincipalStringMatchingLeftInRoleCode pins the architectural half of
// the migration: the trust verdict has exactly one implementation. A second
// copy in the fetcher or the classifier is what let the Findings list and the
// row color disagree in the first place.
func TestW4NoPrincipalStringMatchingLeftInRoleCode(t *testing.T) {
	for _, path := range []string{"../../core/aws/iam_roles.go", "../../core/aws/catalog_security.go"} {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		for _, needle := range []string{`"Principal":"*"`, `"Principal": "*"`} {
			if strings.Contains(string(b), needle) {
				t.Errorf("%s still string-matches %s; the trust verdict belongs to core/iampolicy", path, needle)
			}
		}
	}
}

// w4RoleGetFake serves GetRole for the by-ID path.
type w4RoleGetFake struct {
	awsclient.IAMAPI
	roles map[string]iamtypes.Role
}

func (f *w4RoleGetFake) GetRole(
	_ context.Context, in *iam.GetRoleInput, _ ...func(*iam.Options),
) (*iam.GetRoleOutput, error) {
	role, ok := f.roles[aws.ToString(in.RoleName)]
	if !ok {
		return nil, errors.New("NoSuchEntity: role not found")
	}
	return &iam.GetRoleOutput{Role: &role}, nil
}

var _ awsclient.IAMAPI = (*w4RoleGetFake)(nil)

// TestW4RoleByIDCarriesTheSameTrustVerdict pins that a role opened by id —
// the related-panel drill path — reaches the same verdict as one that came
// from a list page. Two construction sites re-deriving the trust findings
// separately is how a role reads clean in one view and broken in the other.
func TestW4RoleByIDCarriesTheSameTrustVerdict(t *testing.T) {
	fake := &w4RoleGetFake{roles: map[string]iamtypes.Role{
		"acme-double-role": w4Role("acme-double-role", "/", w4TrustStarAndLambda),
	}}
	got, err := awsclient.FetchRolesByIDs(context.Background(), fake, []string{"acme-double-role"})
	if err != nil {
		t.Fatalf("FetchRolesByIDs: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d resources, want 1", len(got))
	}
	w4AssertFinding(t, got[0].Findings, w4CodeRoleWildcardTrust,
		w4PhraseRoleWildcardTrust, domain.SevBroken, "wave1")
	w4AssertFinding(t, got[0].Findings, w4CodeRoleConfusedDeputy,
		w4PhraseRoleConfusedDeputy, domain.SevWarn, "wave1")
	w4AssertRows(t, got[0].AttentionDetails, w4CodeRoleConfusedDeputy, []domain.DetailRow{
		{Label: "Services", Value: "lambda"},
	})
}

// --- row 2: confused deputy -------------------------------------------------

// TestW4RoleConfusedDeputy pins the trust-policy row for a service principal
// that can assume the role on behalf of any caller.
func TestW4RoleConfusedDeputy(t *testing.T) {
	fake := &w4RoleListFake{roles: []iamtypes.Role{
		w4Role("acme-lambda-exec-role", "/", w4TrustLambdaNoSourceScope),
	}}
	r := w4FetchRole(t, fake, "acme-lambda-exec-role")

	w4AssertFinding(t, r.Findings, w4CodeRoleConfusedDeputy,
		w4PhraseRoleConfusedDeputy, domain.SevWarn, "wave1")
	w4AssertRows(t, r.AttentionDetails, w4CodeRoleConfusedDeputy, []domain.DetailRow{
		{Label: "Services", Value: "lambda"},
	})
}

// TestW4RoleConfusedDeputyNegatives pins the three cases that are not
// actionable: the caller is already scoped, AWS owns the document, or the
// only consumer is the instance the role is attached to.
func TestW4RoleConfusedDeputyNegatives(t *testing.T) {
	cases := []struct {
		name string
		path string
		doc  string
	}{
		{"source_account_condition", "/", w4TrustLambdaWithSourceAccount},
		{"service_linked_role", "/aws-service-role/", w4TrustLambdaNoSourceScope},
		{"ec2_instance_profile_only", "/", w4TrustEC2Only},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := &w4RoleListFake{roles: []iamtypes.Role{w4Role("acme-scoped-role", tc.path, tc.doc)}}
			r := w4FetchRole(t, fake, "acme-scoped-role")
			w4AssertNoCode(t, r.Findings, w4CodeRoleConfusedDeputy)
		})
	}
}

// TestW4RoleConfusedDeputyEC2WithAnotherService pins that the EC2 exclusion is
// about EC2 being the only consumer. Once a second service is trusted, the
// role is reachable through that service and the finding stands.
func TestW4RoleConfusedDeputyEC2WithAnotherService(t *testing.T) {
	fake := &w4RoleListFake{roles: []iamtypes.Role{
		w4Role("acme-shared-role", "/", w4TrustEC2AndLambda),
	}}
	r := w4FetchRole(t, fake, "acme-shared-role")

	w4AssertFinding(t, r.Findings, w4CodeRoleConfusedDeputy,
		w4PhraseRoleConfusedDeputy, domain.SevWarn, "wave1")
	w4AssertRows(t, r.AttentionDetails, w4CodeRoleConfusedDeputy, []domain.DetailRow{
		{Label: "Services", Value: "ec2, lambda"},
	})
}

// TestW4RoleTwoTrustConditionsTwoFindings pins independence: a document that
// is both open to everyone and unscoped for a service reports both, so
// fixing one does not hide the other.
func TestW4RoleTwoTrustConditionsTwoFindings(t *testing.T) {
	fake := &w4RoleListFake{roles: []iamtypes.Role{
		w4Role("acme-double-role", "/", w4TrustStarAndLambda),
	}}
	r := w4FetchRole(t, fake, "acme-double-role")

	w4AssertFinding(t, r.Findings, w4CodeRoleWildcardTrust,
		w4PhraseRoleWildcardTrust, domain.SevBroken, "wave1")
	w4AssertFinding(t, r.Findings, w4CodeRoleConfusedDeputy,
		w4PhraseRoleConfusedDeputy, domain.SevWarn, "wave1")
	if got := resource.FindResourceType("role").ResolveColor(r); got != domain.ColorBroken {
		t.Errorf("color = %v, want Broken: the worst finding wins", got)
	}
}

// --- row 5: inline privilege escalation -------------------------------------

// TestW4RoleInlinePrivilegeEscalation pins the inline-policy row: the finding
// names both the policy that carries the grant and the escalation it adds up
// to, so the operator knows which document to edit.
func TestW4RoleInlinePrivilegeEscalation(t *testing.T) {
	fake := &w4RoleListFake{
		roles: []iamtypes.Role{w4Role("acme-inline-escalate-role", "/", w4TrustEC2Only)},
		inline: map[string]map[string]string{
			"acme-inline-escalate-role": {"acme-break-glass": w4InlinePrivEscDoc},
		},
	}
	r := w4FetchRole(t, fake, "acme-inline-escalate-role")

	w4AssertFinding(t, r.Findings, w4CodeRoleInlinePrivEsc,
		"inline policy allows privilege escalation: iam:CreateLoginProfile",
		domain.SevBroken, "wave1")
	w4AssertRows(t, r.AttentionDetails, w4CodeRoleInlinePrivEsc, []domain.DetailRow{
		{Label: "Policy", Value: "acme-break-glass"},
		{Label: "Combo", Value: "iam:CreateLoginProfile"},
	})
}

// TestW4RoleInlinePrivEscNegatives pins the healthy counterparts: an inline
// policy that grants only data access, and a role with no inline policies at
// all.
func TestW4RoleInlinePrivEscNegatives(t *testing.T) {
	t.Run("benign_inline_policy", func(t *testing.T) {
		fake := &w4RoleListFake{
			roles: []iamtypes.Role{w4Role("acme-reader-role", "/", w4TrustEC2Only)},
			inline: map[string]map[string]string{
				"acme-reader-role": {"acme-read-reports": w4InlineBenignDoc},
			},
		}
		r := w4FetchRole(t, fake, "acme-reader-role")
		w4AssertNoCode(t, r.Findings, w4CodeRoleInlinePrivEsc)
	})

	t.Run("no_inline_policies", func(t *testing.T) {
		fake := &w4RoleListFake{roles: []iamtypes.Role{w4Role("acme-bare-role", "/", w4TrustEC2Only)}}
		r := w4FetchRole(t, fake, "acme-bare-role")
		w4AssertNoCode(t, r.Findings, w4CodeRoleInlinePrivEsc)
	})

	// A role whose inline policies cannot be listed is unknown, not clean and
	// not flagged: an API failure must never read as a verdict.
	t.Run("list_inline_policies_fails", func(t *testing.T) {
		fake := &w4RoleListFake{
			roles:   []iamtypes.Role{w4Role("acme-denied-role", "/", w4TrustEC2Only)},
			listErr: map[string]error{"acme-denied-role": errors.New("AccessDenied")},
		}
		r := w4FetchRole(t, fake, "acme-denied-role")
		w4AssertNoCode(t, r.Findings, w4CodeRoleInlinePrivEsc)
	})
}

// --- row 3: admin policy attached (wave 2) ----------------------------------

// w4RoleAdminFake serves the role enricher: GetRole for the dormancy check
// and ListAttachedRolePolicies for the admin check.
type w4RoleAdminFake struct {
	awsclient.IAMAPI
	// attached maps RoleName → attached policy name → policy ARN.
	attached map[string]map[string]string
	// attachErr maps RoleName → error for ListAttachedRolePolicies.
	attachErr map[string]error
}

func (f *w4RoleAdminFake) GetRole(
	_ context.Context, in *iam.GetRoleInput, _ ...func(*iam.Options),
) (*iam.GetRoleOutput, error) {
	recent := time.Now().Add(-1 * time.Hour)
	return &iam.GetRoleOutput{Role: &iamtypes.Role{
		RoleName:     in.RoleName,
		Path:         aws.String("/"),
		RoleLastUsed: &iamtypes.RoleLastUsed{LastUsedDate: &recent},
	}}, nil
}

func (f *w4RoleAdminFake) ListAttachedRolePolicies(
	_ context.Context, in *iam.ListAttachedRolePoliciesInput, _ ...func(*iam.Options),
) (*iam.ListAttachedRolePoliciesOutput, error) {
	name := aws.ToString(in.RoleName)
	if err, ok := f.attachErr[name]; ok {
		return nil, err
	}
	var out []iamtypes.AttachedPolicy
	for policyName, arn := range f.attached[name] {
		out = append(out, iamtypes.AttachedPolicy{
			PolicyName: aws.String(policyName),
			PolicyArn:  aws.String(arn),
		})
	}
	return &iam.ListAttachedRolePoliciesOutput{AttachedPolicies: out}, nil
}

var _ awsclient.IAMAPI = (*w4RoleAdminFake)(nil)

func w4RoleResource(name, path string) resource.Resource {
	return resource.Resource{
		ID: name, Name: name, Type: "role",
		Fields: map[string]string{"role_name": name, "path": path},
	}
}

// TestW4RoleAdminAttached pins the wave-2 row: a role holding a managed
// admin policy is reported on the role row itself, naming the policy.
func TestW4RoleAdminAttached(t *testing.T) {
	fake := &w4RoleAdminFake{attached: map[string]map[string]string{
		"acme-deploy-admin-role": {"AdministratorAccess": w4AdminAccessARN},
		"acme-reports-role":      {"AmazonS3ReadOnlyAccess": "arn:aws:iam::aws:policy/AmazonS3ReadOnlyAccess"},
	}}
	clients := &awsclient.ServiceClients{IAM: fake, Region: "us-east-1"}
	res, err := awsclient.EnrichIAMRoleLastUsed(context.Background(), clients, []resource.Resource{
		w4RoleResource("acme-deploy-admin-role", "/"),
		w4RoleResource("acme-reports-role", "/"),
	}, nil)
	if err != nil {
		t.Fatalf("EnrichIAMRoleLastUsed: %v", err)
	}

	w4AssertFinding(t, res.Findings["acme-deploy-admin-role"], w4CodeRoleAdminAttached,
		w4PhraseRoleAdminAttached, domain.SevWarn, w4SourceRoleWave2)
	w4AssertRows(t, res.AttentionDetails["acme-deploy-admin-role"], w4CodeRoleAdminAttached,
		[]domain.DetailRow{{Label: "Policy", Value: "AdministratorAccess"}})
	w4AssertNoCode(t, res.Findings["acme-reports-role"], w4CodeRoleAdminAttached)
}

// TestW4RoleAdminAttachedSkipsServiceLinked pins that AWS-owned roles are out
// of scope: the operator cannot detach a service-linked role's policies.
func TestW4RoleAdminAttachedSkipsServiceLinked(t *testing.T) {
	fake := &w4RoleAdminFake{attached: map[string]map[string]string{
		"AWSServiceRoleForAutoScaling": {"AdministratorAccess": w4AdminAccessARN},
	}}
	clients := &awsclient.ServiceClients{IAM: fake, Region: "us-east-1"}
	res, err := awsclient.EnrichIAMRoleLastUsed(context.Background(), clients, []resource.Resource{
		w4RoleResource("AWSServiceRoleForAutoScaling", "/aws-service-role/autoscaling.amazonaws.com/"),
	}, nil)
	if err != nil {
		t.Fatalf("EnrichIAMRoleLastUsed: %v", err)
	}
	w4AssertNoCode(t, res.Findings["AWSServiceRoleForAutoScaling"], w4CodeRoleAdminAttached)
}

// TestW4RoleAdminAttachedAPIErrorIsUnknown pins that one role's failed
// attached-policy listing marks that role unknown and leaves the rest of the
// batch evaluated. A failed call must not read as "no admin policy".
func TestW4RoleAdminAttachedAPIErrorIsUnknown(t *testing.T) {
	fake := &w4RoleAdminFake{
		attached: map[string]map[string]string{
			"acme-ops-admin-role": {"PowerUserAccess": w4PowerUserAccessARN},
		},
		attachErr: map[string]error{"acme-denied-role": errors.New("AccessDenied: not authorized")},
	}
	clients := &awsclient.ServiceClients{IAM: fake, Region: "us-east-1"}
	res, err := awsclient.EnrichIAMRoleLastUsed(context.Background(), clients, []resource.Resource{
		w4RoleResource("acme-denied-role", "/"),
		w4RoleResource("acme-ops-admin-role", "/"),
	}, nil)
	if err != nil {
		t.Fatalf("EnrichIAMRoleLastUsed: %v", err)
	}
	if !res.TruncatedIDs["acme-denied-role"] {
		t.Errorf("TruncatedIDs[acme-denied-role] = false, want true")
	}
	w4AssertNoCode(t, res.Findings["acme-denied-role"], w4CodeRoleAdminAttached)
	w4AssertFinding(t, res.Findings["acme-ops-admin-role"], w4CodeRoleAdminAttached,
		w4PhraseRoleAdminAttached, domain.SevWarn, w4SourceRoleWave2)
}

// TestW4RoleAdminAttachedNilClient pins the wave-2 nil-client contract: an
// empty, non-nil result and no error.
func TestW4RoleAdminAttachedNilClient(t *testing.T) {
	res, err := awsclient.EnrichIAMRoleLastUsed(context.Background(),
		&awsclient.ServiceClients{}, []resource.Resource{w4RoleResource("acme-deploy-admin-role", "/")}, nil)
	if err != nil {
		t.Fatalf("EnrichIAMRoleLastUsed: %v", err)
	}
	if res.Findings == nil || res.TruncatedIDs == nil {
		t.Fatalf("result maps must be non-nil")
	}
	if len(res.Findings) != 0 {
		t.Errorf("Findings = %v, want empty", res.Findings)
	}
}

// --- catalog registration ---------------------------------------------------

// TestW4RoleFindingDefs pins that every role code the batch emits has a
// registry row whose phrase and severity match the emitted finding. Without
// it the code renders with no phrase in the menu and misses the docs table.
func TestW4RoleFindingDefs(t *testing.T) {
	cases := []struct {
		code   domain.FindingCode
		phrase string
		sev    domain.Severity
		source string
	}{
		{w4CodeRoleWildcardTrust, w4PhraseRoleWildcardTrust, domain.SevBroken, "wave1"},
		{w4CodeRoleConfusedDeputy, w4PhraseRoleConfusedDeputy, domain.SevWarn, "wave1"},
		{w4CodeRoleInlinePrivEsc, "inline policy allows privilege escalation: <combo>", domain.SevBroken, "wave1"},
		{w4CodeRoleAdminAttached, w4PhraseRoleAdminAttached, domain.SevWarn, "wave2"},
	}
	for _, tc := range cases {
		t.Run(string(tc.code), func(t *testing.T) {
			def := w4FindingDef(t, "role", tc.code)
			if def.Phrase != tc.phrase {
				t.Errorf("Phrase = %q, want %q", def.Phrase, tc.phrase)
			}
			if def.Severity != tc.sev {
				t.Errorf("Severity = %v, want %v", def.Severity, tc.sev)
			}
			if def.Source != tc.source {
				t.Errorf("Source = %q, want %q", def.Source, tc.source)
			}
		})
	}
}
