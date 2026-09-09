package unit

// d3_iam_test.go — rows 1, 2, 3, 4 and 12: a swallowed key error, rows that
// restate their phrase, policy-document attribute names used as labels, a
// drilled role that cannot report what the listed one does, and three policy
// decoders that disagree about a literal '+'.

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/iampolicy"
	"github.com/k2m30/a9s/v3/core/resource"
)

const (
	d3CodeUserKeyUnused domain.FindingCode = "iam-user.access-key-unused"
	d3CodeUserNoMFA     domain.FindingCode = "iam-user.no-mfa"
	d3CodeUserOldKey    domain.FindingCode = "iam-user.old-key"
	d3CodePolicyAdmin   domain.FindingCode = "iam-policy.admin-star"
	d3CodeRoleInline    domain.FindingCode = "role.inline-privilege-escalation"
)

// --- rows 1 and 2: the iam-user enricher ------------------------------------

// d3UserFake serves the user enricher. Every user has a console password and
// no MFA device unless said otherwise, so the rows under test are reachable
// without also arranging a second condition.
type d3UserFake struct {
	awsclient.IAMAPI
	keys map[string][]iamtypes.AccessKeyMetadata
	// lastUsedErr maps AccessKeyId → error from GetAccessKeyLastUsed.
	lastUsedErr map[string]error
	withMFA     map[string]bool
}

func (f *d3UserFake) GetLoginProfile(
	_ context.Context, in *iam.GetLoginProfileInput, _ ...func(*iam.Options),
) (*iam.GetLoginProfileOutput, error) {
	created := time.Now().Add(-400 * 24 * time.Hour)
	return &iam.GetLoginProfileOutput{LoginProfile: &iamtypes.LoginProfile{
		UserName: in.UserName, CreateDate: &created,
	}}, nil
}

func (f *d3UserFake) ListMFADevices(
	_ context.Context, in *iam.ListMFADevicesInput, _ ...func(*iam.Options),
) (*iam.ListMFADevicesOutput, error) {
	if !f.withMFA[aws.ToString(in.UserName)] {
		return &iam.ListMFADevicesOutput{}, nil
	}
	return &iam.ListMFADevicesOutput{MFADevices: []iamtypes.MFADevice{{
		UserName:     in.UserName,
		SerialNumber: aws.String("arn:aws:iam::123456789012:mfa/" + aws.ToString(in.UserName)),
	}}}, nil
}

func (f *d3UserFake) ListAccessKeys(
	_ context.Context, in *iam.ListAccessKeysInput, _ ...func(*iam.Options),
) (*iam.ListAccessKeysOutput, error) {
	return &iam.ListAccessKeysOutput{AccessKeyMetadata: f.keys[aws.ToString(in.UserName)]}, nil
}

func (f *d3UserFake) GetAccessKeyLastUsed(
	_ context.Context, in *iam.GetAccessKeyLastUsedInput, _ ...func(*iam.Options),
) (*iam.GetAccessKeyLastUsedOutput, error) {
	id := aws.ToString(in.AccessKeyId)
	if err, ok := f.lastUsedErr[id]; ok {
		return nil, err
	}
	return &iam.GetAccessKeyLastUsedOutput{
		AccessKeyLastUsed: &iamtypes.AccessKeyLastUsed{
			ServiceName: aws.String("s3"), Region: aws.String("us-east-1"),
		},
	}, nil
}

func (f *d3UserFake) ListAttachedUserPolicies(
	_ context.Context, _ *iam.ListAttachedUserPoliciesInput, _ ...func(*iam.Options),
) (*iam.ListAttachedUserPoliciesOutput, error) {
	return &iam.ListAttachedUserPoliciesOutput{}, nil
}

var _ awsclient.IAMAPI = (*d3UserFake)(nil)

func d3Key(user, id string, ageDays int) iamtypes.AccessKeyMetadata {
	created := time.Now().Add(-time.Duration(ageDays) * 24 * time.Hour)
	return iamtypes.AccessKeyMetadata{
		UserName:    aws.String(user),
		AccessKeyId: aws.String(id),
		Status:      iamtypes.StatusTypeActive,
		CreateDate:  &created,
	}
}

func d3UserResource(name string) resource.Resource {
	created := time.Now().Add(-400 * 24 * time.Hour)
	return resource.Resource{
		ID: name, Name: name, Type: "iam-user",
		Fields: map[string]string{
			"user_name":            name,
			"create_date":          created.Format("2006-01-02 15:04"),
			"password_last_used":   "2026-06-01 09:00",
			"has_console_password": "false",
		},
	}
}

func d3EnrichUsers(t *testing.T, fake *d3UserFake, rs []resource.Resource) awsclient.IssueEnricherResult {
	t.Helper()
	res, err := d3EnrichUsersErr(t, fake, rs)
	if err != nil {
		t.Fatalf("EnrichIAMUserMFA: %v", err)
	}
	return res
}

// d3EnrichUsersErr is d3EnrichUsers for the cases that expect a refusal: a key
// whose last use could not be read is a recorded failure, not a silent skip.
func d3EnrichUsersErr(t *testing.T, fake *d3UserFake, rs []resource.Resource) (awsclient.IssueEnricherResult, error) {
	t.Helper()
	return awsclient.EnrichIAMUserMFA(context.Background(),
		&awsclient.ServiceClients{IAM: fake, Region: "us-east-1"}, rs, nil)
}

// TestD3KeyLastUsedErrorMarksTheUserUnknown pins row 1: a GetAccessKeyLastUsed
// that fails leaves the key's idleness unknown, so the user must read as
// unknown rather than clean. Swallowing the error is how a user with a
// forgotten key renders healthy.
func TestD3KeyLastUsedErrorMarksTheUserUnknown(t *testing.T) {
	fake := &d3UserFake{
		withMFA: map[string]bool{"acme-batch-user": true, "acme-ci-user": true},
		keys: map[string][]iamtypes.AccessKeyMetadata{
			"acme-batch-user": {d3Key("acme-batch-user", "AKIAIOSFODNN7EXAMPL1", 10)},
			"acme-ci-user":    {d3Key("acme-ci-user", "AKIAIOSFODNN7EXAMPL2", 400)},
		},
		lastUsedErr: map[string]error{
			"AKIAIOSFODNN7EXAMPL1": errors.New("Throttling: rate exceeded"),
		},
	}
	res, err := d3EnrichUsersErr(t, fake, []resource.Resource{
		d3UserResource("acme-batch-user"), d3UserResource("acme-ci-user"),
	})

	// A helper that failed the test on any error would let the unreadable key
	// be marked "?" with nothing in the log.
	if err == nil {
		t.Error("a key whose last use could not be read returned no error")
	}
	if _, marked := res.TruncatedIDs["acme-batch-user"]; !marked {
		t.Errorf("TruncatedIDs[acme-batch-user] = false; an unreadable key is unknown, not clean")
	}
	w4AssertNoCode(t, res.Findings["acme-batch-user"], d3CodeUserKeyUnused)
	if _, marked := res.TruncatedIDs["acme-ci-user"]; marked {
		t.Errorf("the sibling user was marked unknown by another user's failure")
	}
	// The idle days are an Idle row, not part of the phrase, so one key's
	// number does not stand for every idle key on the user.
	w4AssertFinding(t, res.Findings["acme-ci-user"], d3CodeUserKeyUnused,
		catalog.Phrase(d3CodeUserKeyUnused), domain.SevWarn, "wave2")
}

// TestD3UserRowsDoNotRestateTheirPhrase pins row 2: a supporting row exists to
// add the fact the phrase leaves out. A row that repeats the phrase word for
// word costs a line and tells the operator nothing.
func TestD3UserRowsDoNotRestateTheirPhrase(t *testing.T) {
	fake := &d3UserFake{
		keys: map[string][]iamtypes.AccessKeyMetadata{
			"acme-legacy-user": {d3Key("acme-legacy-user", "AKIAIOSFODNN7EXAMPLE", 400)},
		},
	}
	res := d3EnrichUsers(t, fake, []resource.Resource{d3UserResource("acme-legacy-user")})
	rows := res.AttentionDetails["acme-legacy-user"]

	t.Run("no_mfa", func(t *testing.T) {
		w4AssertRows(t, rows, d3CodeUserNoMFA, []domain.DetailRow{
			{Label: "MFA device", Value: "none registered"},
		})
	})

	t.Run("old_key", func(t *testing.T) {
		f, ok := w4FindingByCode(t, res.Findings["acme-legacy-user"], d3CodeUserOldKey)
		if !ok {
			t.Fatalf("no %q finding", d3CodeUserOldKey)
		}
		ad, ok := rows[d3CodeUserOldKey]
		if !ok || len(ad.Rows) != 1 {
			t.Fatalf("old-key rows = %+v, want exactly one", ad.Rows)
		}
		if ad.Rows[0].Value == f.Phrase {
			t.Errorf("row value restates the phrase: %q", ad.Rows[0].Value)
		}
		if ad.Rows[0].Value != "…MPLE" {
			t.Errorf("row value = %q, want the key suffix %q", ad.Rows[0].Value, "…MPLE")
		}
	})
}

// --- row 3: the policy enricher's labels ------------------------------------

type d3PolicyFake struct {
	awsclient.IAMAPI
	docs map[string]string
}

func (f *d3PolicyFake) GetPolicy(
	_ context.Context, in *iam.GetPolicyInput, _ ...func(*iam.Options),
) (*iam.GetPolicyOutput, error) {
	if _, ok := f.docs[aws.ToString(in.PolicyArn)]; !ok {
		return nil, errors.New("NoSuchEntity")
	}
	return &iam.GetPolicyOutput{Policy: &iamtypes.Policy{
		Arn: in.PolicyArn, DefaultVersionId: aws.String("v1"),
	}}, nil
}

func (f *d3PolicyFake) GetPolicyVersion(
	_ context.Context, in *iam.GetPolicyVersionInput, _ ...func(*iam.Options),
) (*iam.GetPolicyVersionOutput, error) {
	doc, ok := f.docs[aws.ToString(in.PolicyArn)]
	if !ok {
		return nil, errors.New("NoSuchEntity")
	}
	return &iam.GetPolicyVersionOutput{PolicyVersion: &iamtypes.PolicyVersion{
		VersionId: in.VersionId, Document: aws.String(url.PathEscape(doc)),
	}}, nil
}

var _ awsclient.IAMAPI = (*d3PolicyFake)(nil)

// TestD3AdminPolicyRowsUsePlainWords pins row 3: the two supporting rows on an
// admin policy are read by an operator, so their labels are words rather than
// the attribute names the policy document happens to use.
func TestD3AdminPolicyRowsUsePlainWords(t *testing.T) {
	const arn = "arn:aws:iam::123456789012:policy/acme-break-glass"
	fake := &d3PolicyFake{docs: map[string]string{
		arn: `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":"*","Resource":"*"}]}`,
	}}
	res, err := awsclient.EnrichIAMPolicy(context.Background(),
		&awsclient.ServiceClients{IAM: fake, Region: "us-east-1"},
		[]resource.Resource{{
			ID: arn, Name: "acme-break-glass", Type: "policy",
			Fields:    map[string]string{"attachment_count": "2"},
			RawStruct: iamtypes.Policy{PolicyName: aws.String("acme-break-glass"), Arn: aws.String(arn)},
		}}, nil)
	if err != nil {
		t.Fatalf("EnrichIAMPolicy: %v", err)
	}

	w4AssertRows(t, res.AttentionDetails[arn], d3CodePolicyAdmin, []domain.DetailRow{
		{Label: "Allowed actions", Value: "all (*)"},
		{Label: "On resources", Value: "all (*)"},
	})
	if got := res.FieldUpdates[arn]["risk"]; got != "admin policy" {
		t.Errorf("risk = %q, want %q", got, "admin policy")
	}
}

// TestD3PrivEscStatusReadsAsWords pins the Status cell of a policy that grants
// an escalation path. The cell is read by an operator, so it carries the
// vocabulary word rather than an enum-shaped token.
func TestD3PrivEscStatusReadsAsWords(t *testing.T) {
	const arn = "arn:aws:iam::123456789012:policy/acme-lambda-deployer"
	fake := &d3PolicyFake{docs: map[string]string{
		arn: `{"Version":"2012-10-17","Statement":[{"Effect":"Allow",` +
			`"Action":["iam:PassRole","lambda:CreateFunction","lambda:InvokeFunction"],"Resource":"*"}]}`,
	}}
	res, err := awsclient.EnrichIAMPolicy(context.Background(),
		&awsclient.ServiceClients{IAM: fake, Region: "us-east-1"},
		[]resource.Resource{{
			ID: arn, Name: "acme-lambda-deployer", Type: "policy",
			Fields:    map[string]string{"attachment_count": "2"},
			RawStruct: iamtypes.Policy{PolicyName: aws.String("acme-lambda-deployer"), Arn: aws.String(arn)},
		}}, nil)
	if err != nil {
		t.Fatalf("EnrichIAMPolicy: %v", err)
	}
	if got := res.FieldUpdates[arn]["risk"]; got != "privilege escalation" {
		t.Errorf("risk = %q, want %q", got, "privilege escalation")
	}
}

// --- row 4: a drilled role reports what a listed one does -------------------

const d3InlinePrivEscDoc = `{"Version":"2012-10-17","Statement":[` +
	`{"Effect":"Allow","Action":"iam:CreateLoginProfile","Resource":"*"}]}`

// d3RoleFake serves both the list path and the by-id path over one role, so
// the two can be compared without the fixture differing between them.
type d3RoleFake struct {
	awsclient.IAMAPI
	role   iamtypes.Role
	inline map[string]string
}

func (f *d3RoleFake) ListRoles(
	_ context.Context, _ *iam.ListRolesInput, _ ...func(*iam.Options),
) (*iam.ListRolesOutput, error) {
	return &iam.ListRolesOutput{Roles: []iamtypes.Role{f.role}}, nil
}

func (f *d3RoleFake) GetRole(
	_ context.Context, _ *iam.GetRoleInput, _ ...func(*iam.Options),
) (*iam.GetRoleOutput, error) {
	role := f.role
	return &iam.GetRoleOutput{Role: &role}, nil
}

func (f *d3RoleFake) ListRolePolicies(
	_ context.Context, _ *iam.ListRolePoliciesInput, _ ...func(*iam.Options),
) (*iam.ListRolePoliciesOutput, error) {
	names := make([]string, 0, len(f.inline))
	for name := range f.inline {
		names = append(names, name)
	}
	return &iam.ListRolePoliciesOutput{PolicyNames: names}, nil
}

func (f *d3RoleFake) GetRolePolicy(
	_ context.Context, in *iam.GetRolePolicyInput, _ ...func(*iam.Options),
) (*iam.GetRolePolicyOutput, error) {
	doc, ok := f.inline[aws.ToString(in.PolicyName)]
	if !ok {
		return nil, errors.New("NoSuchEntity")
	}
	return &iam.GetRolePolicyOutput{
		RoleName: in.RoleName, PolicyName: in.PolicyName,
		PolicyDocument: aws.String(url.PathEscape(doc)),
	}, nil
}

var _ awsclient.IAMAPI = (*d3RoleFake)(nil)

// TestD3DrilledRoleCarriesTheSameFindingsAsTheListedOne pins row 4: the same
// role must read the same whether it arrived in a list page or was opened
// from another view. A finding that only one path can compute makes the
// verdict depend on how the operator navigated.
func TestD3DrilledRoleCarriesTheSameFindingsAsTheListedOne(t *testing.T) {
	const name = "acme-pipeline-builder-role"
	created := time.Now().Add(-365 * 24 * time.Hour)
	fake := &d3RoleFake{
		role: iamtypes.Role{
			RoleName:   aws.String(name),
			RoleId:     aws.String("AROAEXAMPLEPIPELINE1"),
			Arn:        aws.String("arn:aws:iam::123456789012:role/" + name),
			Path:       aws.String("/"),
			CreateDate: &created,
			AssumeRolePolicyDocument: aws.String(url.PathEscape(
				`{"Version":"2012-10-17","Statement":[{"Effect":"Allow",` +
					`"Principal":{"Service":"ec2.amazonaws.com"},"Action":"sts:AssumeRole"}]}`)),
		},
		inline: map[string]string{"acme-break-glass": d3InlinePrivEscDoc},
	}

	listed, err := awsclient.FetchIAMRolesPage(context.Background(), fake, "")
	if err != nil {
		t.Fatalf("FetchIAMRolesPage: %v", err)
	}
	if len(listed.Resources) != 1 {
		t.Fatalf("listed %d roles, want 1", len(listed.Resources))
	}
	// The combos are Combo rows, so no combo on the inline policy is left
	// unsayable behind a phrase that names only the first match.
	w4AssertFinding(t, listed.Resources[0].Findings, d3CodeRoleInline,
		catalog.Phrase(d3CodeRoleInline), domain.SevBroken, "wave1")

	drilled, err := awsclient.FetchRolesByIDs(context.Background(), fake, []string{name})
	if err != nil {
		t.Fatalf("FetchRolesByIDs: %v", err)
	}
	if len(drilled) != 1 {
		t.Fatalf("drilled %d roles, want 1", len(drilled))
	}
	w4AssertFinding(t, drilled[0].Findings, d3CodeRoleInline,
		catalog.Phrase(d3CodeRoleInline), domain.SevBroken, "wave1")
}

// --- row 12: one decoder ----------------------------------------------------

// d3PlusDoc carries a literal '+' inside a resource ARN. Path-style
// unescaping keeps it; query-style turns it into a space, which is how the
// same document comes out differently depending on which site read it.
const d3PlusDoc = `{"Version":"2012-10-17","Statement":[{"Effect":"Allow",` +
	`"Action":"s3:GetObject","Resource":"arn:aws:s3:::acme-reports/q1+q2/*"}]}`

// TestD3PolicyDocumentDecodesIdenticallyEverywhere pins row 12: a document
// with a literal '+' must read the same at every site that decodes one.
// Today the role fetcher unescapes query-style and turns the '+' into a
// space, so the same trust policy reads differently there than in iampolicy.
func TestD3PolicyDocumentDecodesIdenticallyEverywhere(t *testing.T) {
	encoded := url.PathEscape(d3PlusDoc)
	const want = "acme-reports/q1+q2/*"

	t.Run("iampolicy_decode", func(t *testing.T) {
		if got := iampolicy.Decode(encoded); !strings.Contains(got, want) {
			t.Errorf("Decode lost the literal '+': %q", got)
		}
	})

	// A document that already is JSON is used as it stands, so a resource
	// name holding a literal '%' is not read as the start of an escape.
	// Decode and Parse have to agree on that, or the same document means two
	// things depending on which entry point the caller reached for.
	t.Run("literal_percent_in_valid_json", func(t *testing.T) {
		const doc = `{"Version":"2012-10-17","Statement":[{"Effect":"Allow",` +
			`"Action":"s3:GetObject","Resource":"arn:aws:s3:::acme-reports/100%2Fyear/*"}]}`
		const resourceWant = "arn:aws:s3:::acme-reports/100%2Fyear/*"

		if got := iampolicy.Decode(doc); got != doc {
			t.Errorf("Decode unescaped a document that was already JSON:\n want %q\n  got %q", doc, got)
		}
		parsed, err := iampolicy.Parse(doc)
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		if len(parsed.Statement) != 1 || len(parsed.Statement[0].Resource) != 1 {
			t.Fatalf("parsed %+v, want one statement with one resource", parsed.Statement)
		}
		if got := parsed.Statement[0].Resource[0]; got != resourceWant {
			t.Errorf("Parse read the resource as %q, want %q", got, resourceWant)
		}
	})

	t.Run("role_fetcher", func(t *testing.T) {
		created := time.Now().Add(-365 * 24 * time.Hour)
		fake := &d3RoleFake{role: iamtypes.Role{
			RoleName:                 aws.String("acme-plus-role"),
			Path:                     aws.String("/"),
			CreateDate:               &created,
			AssumeRolePolicyDocument: aws.String(encoded),
		}}
		out, err := awsclient.FetchIAMRolesPage(context.Background(), fake, "")
		if err != nil {
			t.Fatalf("FetchIAMRolesPage: %v", err)
		}
		got := out.Resources[0].Fields["assume_role_policy_document"]
		if !strings.Contains(got, want) {
			t.Errorf("the role fetcher lost the literal '+': %q", got)
		}

		// The by-id path builds the same field from the same document, so it
		// must not read it differently from the list path.
		drilled, derr := awsclient.FetchRolesByIDs(context.Background(), fake, []string{"acme-plus-role"})
		if derr != nil {
			t.Fatalf("FetchRolesByIDs: %v", derr)
		}
		if drilledDoc := drilled[0].Fields["assume_role_policy_document"]; drilledDoc != got {
			t.Errorf("the drilled role decoded differently:\n list: %q\ndrill: %q", got, drilledDoc)
		}
	})

	t.Run("managed_policy_document", func(t *testing.T) {
		const arn = "arn:aws:iam::123456789012:policy/acme-plus-policy"
		fake := &d3PolicyFake{docs: map[string]string{arn: d3PlusDoc}}
		doc, err := awsclient.FetchManagedPolicyDocument(context.Background(), fake, fake, arn)
		if err != nil {
			t.Fatalf("FetchManagedPolicyDocument: %v", err)
		}
		m, ok := doc.(map[string]any)
		if !ok {
			t.Fatalf("decoded document is %T, want a JSON object", doc)
		}
		if !strings.Contains(d3FirstResource(t, m), want) {
			t.Errorf("the managed-policy decoder lost the literal '+': %v", m)
		}
	})
}

// d3FirstResource pulls Statement[0].Resource out of a decoded document.
func d3FirstResource(t *testing.T, doc map[string]any) string {
	t.Helper()
	stmts, ok := doc["Statement"].([]any)
	if !ok || len(stmts) == 0 {
		t.Fatalf("no Statement in %v", doc)
	}
	first, ok := stmts[0].(map[string]any)
	if !ok {
		t.Fatalf("Statement[0] is not an object: %v", stmts[0])
	}
	res, _ := first["Resource"].(string)
	return res
}
