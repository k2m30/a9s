package unit

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo/fakes"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/session"
)

// codex2RolePolicyFake is one role's policy lists over an account whose
// documents are t5EmptyIAM's: AWS-managed ones as AWS publishes them, every
// other one scoped.
type codex2RolePolicyFake struct {
	t5EmptyIAM
	attached []iamtypes.AttachedPolicy
	inline   []string
}

func (f *codex2RolePolicyFake) ListAttachedRolePolicies(_ context.Context, _ *iam.ListAttachedRolePoliciesInput, _ ...func(*iam.Options)) (*iam.ListAttachedRolePoliciesOutput, error) {
	return &iam.ListAttachedRolePoliciesOutput{AttachedPolicies: f.attached}, nil
}

func (f *codex2RolePolicyFake) ListRolePolicies(_ context.Context, _ *iam.ListRolePoliciesInput, _ ...func(*iam.Options)) (*iam.ListRolePoliciesOutput, error) {
	return &iam.ListRolePoliciesOutput{PolicyNames: f.inline}, nil
}

// codex2DrillRolePolicies opens the role's policy list through the
// role_policies child fetcher, the path the drill from a role takes.
func codex2DrillRolePolicies(t *testing.T, fake *codex2RolePolicyFake) []resource.Resource {
	t.Helper()
	td := resource.GetChildType("role_policies")
	if td == nil || td.ChildFetcher == nil {
		t.Fatal("role_policies child type has no fetcher")
	}
	clients := &awsclient.ServiceClients{IAM: fake}
	session.New().WireStores(clients)
	res, err := td.ChildFetcher(context.Background(), clients, map[string]string{"role_name": "acme-deploy-role"}, "")
	if err != nil {
		t.Fatalf("role_policies ChildFetcher: %v", err)
	}
	return res.Resources
}

func codex2FetchRolePolicy(t *testing.T, name, arn string) resource.Resource {
	t.Helper()
	rows := codex2DrillRolePolicies(t, &codex2RolePolicyFake{attached: []iamtypes.AttachedPolicy{
		{PolicyName: aws.String(name), PolicyArn: aws.String(arn)},
	}})
	if len(rows) != 1 {
		t.Fatalf("got %d policy rows, want 1", len(rows))
	}
	return rows[0]
}

// A row is judged from the document its ARN resolves to. A customer-managed
// policy can be named AdministratorAccess and carry whatever its account
// gives it; AWS's AdministratorAccess, in any partition, allows everything.
func TestRolePolicy_RowIsJudgedFromTheDocumentItsARNResolvesTo(t *testing.T) {
	for _, tc := range []struct {
		name   string
		policy string
		arn    string
		code   domain.FindingCode
	}{
		{"AWS-owned AdministratorAccess", "AdministratorAccess", "arn:aws:iam::aws:policy/AdministratorAccess", awsclient.CodeRolePolicyAdministrator},
		{"AWS-owned PowerUserAccess", "PowerUserAccess", "arn:aws:iam::aws:policy/PowerUserAccess", awsclient.CodeRolePolicyBroadPower},
		{"GovCloud AdministratorAccess", "AdministratorAccess", "arn:aws-us-gov:iam::aws:policy/AdministratorAccess", awsclient.CodeRolePolicyAdministrator},
		{"customer policy of the same name", "AdministratorAccess", "arn:aws:iam::123456789012:policy/AdministratorAccess", ""},
		{"customer policy named PowerUserAccess", "PowerUserAccess", "arn:aws:iam::123456789012:policy/PowerUserAccess", ""},
		{"an AWS read-only policy", "ReadOnlyAccess", "arn:aws:iam::aws:policy/ReadOnlyAccess", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := codex2FetchRolePolicy(t, tc.policy, tc.arn)
			var got domain.FindingCode
			if len(r.Findings) > 0 {
				got = r.Findings[0].Code
			}
			if got != tc.code {
				t.Errorf("finding code = %q, want %q (findings=%+v): the verdict comes from the document, never the policy's name",
					got, tc.code, r.Findings)
			}
		})
	}
}

// An inline policy's Statement can be a bare object instead of an array, and
// the privilege-escalation check reads it as a one-statement list.
func TestIAMRoles_InlineBareStatementObjectIsRead(t *testing.T) {
	const doc = `{"Version":"2012-10-17","Statement":{"Effect":"Allow","Action":"iam:CreatePolicyVersion","Resource":"*"}}`
	res, _ := w4EnrichRoles(t, &w4RoleListFake{
		roles:  []iamtypes.Role{w4Role("acme-deploy-role", "/", w4TrustEC2Only)},
		inline: map[string]map[string]string{"acme-deploy-role": {"escalate": doc}},
	})
	if !codex2HasCode(res.Findings["acme-deploy-role"], "role.inline-privilege-escalation") {
		t.Errorf("no privilege-escalation finding on a single-object Statement; findings=%+v truncated=%q",
			res.Findings["acme-deploy-role"], res.TruncatedIDs["acme-deploy-role"])
	}
}

// An empty answer for an entity the fixtures do not hold would make a fixture
// gap read as "this principal has none", the confident zero the demo exists to
// disprove.
func TestDemoIAMFake_RefusesAKeyTheFixturesNeverRegistered(t *testing.T) {
	f := fakes.NewIAM()
	ctx := context.Background()

	for _, tc := range []struct {
		name string
		call func() error
	}{
		{"GetPolicy for an unknown ARN", func() error {
			_, err := f.GetPolicy(ctx, &iam.GetPolicyInput{PolicyArn: aws.String("arn:aws:iam::123456789012:policy/no-such-policy")})
			return err
		}},
		{"GetPolicyVersion for an unknown ARN", func() error {
			_, err := f.GetPolicyVersion(ctx, &iam.GetPolicyVersionInput{
				PolicyArn: aws.String("arn:aws:iam::123456789012:policy/no-such-policy"), VersionId: aws.String("v1")})
			return err
		}},
		{"GetInstanceProfile for an unknown name", func() error {
			_, err := f.GetInstanceProfile(ctx, &iam.GetInstanceProfileInput{InstanceProfileName: aws.String("no-such-profile")})
			return err
		}},
		{"GetRolePolicy for a document the fixture never registered", func() error {
			_, err := f.GetRolePolicy(ctx, &iam.GetRolePolicyInput{RoleName: aws.String("no-such-role"), PolicyName: aws.String("no-such-policy")})
			return err
		}},
		{"ListRolePolicies for an unknown role", func() error {
			_, err := f.ListRolePolicies(ctx, &iam.ListRolePoliciesInput{RoleName: aws.String("no-such-role")})
			return err
		}},
		{"ListAttachedRolePolicies for an unknown role", func() error {
			_, err := f.ListAttachedRolePolicies(ctx, &iam.ListAttachedRolePoliciesInput{RoleName: aws.String("no-such-role")})
			return err
		}},
		{"ListAttachedUserPolicies for an unknown user", func() error {
			_, err := f.ListAttachedUserPolicies(ctx, &iam.ListAttachedUserPoliciesInput{UserName: aws.String("no-such-user")})
			return err
		}},
		{"ListAttachedGroupPolicies for an unknown group", func() error {
			_, err := f.ListAttachedGroupPolicies(ctx, &iam.ListAttachedGroupPoliciesInput{GroupName: aws.String("no-such-group")})
			return err
		}},
		{"ListGroupsForUser for an unknown user", func() error {
			_, err := f.ListGroupsForUser(ctx, &iam.ListGroupsForUserInput{UserName: aws.String("no-such-user")})
			return err
		}},
		{"ListGroupPolicies for an unknown group", func() error {
			_, err := f.ListGroupPolicies(ctx, &iam.ListGroupPoliciesInput{GroupName: aws.String("no-such-group")})
			return err
		}},
		{"ListMFADevices for an unknown user", func() error {
			_, err := f.ListMFADevices(ctx, &iam.ListMFADevicesInput{UserName: aws.String("no-such-user")})
			return err
		}},
		{"ListAccessKeys for an unknown user", func() error {
			_, err := f.ListAccessKeys(ctx, &iam.ListAccessKeysInput{UserName: aws.String("no-such-user")})
			return err
		}},
		{"GetGroup for an unknown group", func() error {
			_, err := f.GetGroup(ctx, &iam.GetGroupInput{GroupName: aws.String("no-such-group")})
			return err
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.call()
			if err == nil {
				t.Fatalf("answered for a key the fixtures never registered; want NoSuchEntity")
			}
			// The code, not the Go type name: checkers such as checkTGWRole
			// ask ErrCodeIs for "NoSuchEntity" to tell "absent" from
			// "unexpected error".
			if !awsclient.ErrCodeIs(err, "NoSuchEntity") {
				t.Errorf("err = %v, want the modeled NoSuchEntity code a checker can read", err)
			}
		})
	}
}

// "This principal exists and has none" is a real AWS answer: an empty list,
// not a refusal.
func TestDemoIAMFake_StillAnswersForEveryRegisteredRole(t *testing.T) {
	f := fakes.NewIAM()
	ctx := context.Background()
	roles, err := f.ListRoles(ctx, &iam.ListRolesInput{})
	if err != nil {
		t.Fatalf("ListRoles: %v", err)
	}
	if len(roles.Roles) == 0 {
		t.Fatal("no demo roles; the control proves nothing")
	}
	for _, r := range roles.Roles {
		name := aws.ToString(r.RoleName)
		if _, listErr := f.ListRolePolicies(ctx, &iam.ListRolePoliciesInput{RoleName: aws.String(name)}); listErr != nil {
			t.Errorf("ListRolePolicies(%s): a registered role must answer, got %v", name, listErr)
		}
		if _, attErr := f.ListAttachedRolePolicies(ctx, &iam.ListAttachedRolePoliciesInput{RoleName: aws.String(name)}); attErr != nil {
			t.Errorf("ListAttachedRolePolicies(%s): a registered role must answer, got %v", name, attErr)
		}
	}
}

// A policy the fixture lists answers its entities lookup (empty when it is
// attached to nothing), so the policy → roles pivot never sees a not-found for
// a policy the list just showed.
func TestDemoIAMFake_StillAnswersForEveryRegisteredPolicy(t *testing.T) {
	ctx := context.Background()
	f := fakes.NewIAM()
	out, err := f.ListPolicies(ctx, &iam.ListPoliciesInput{})
	if err != nil {
		t.Fatalf("ListPolicies: %v", err)
	}
	for _, p := range out.Policies {
		if _, entErr := f.ListEntitiesForPolicy(ctx, &iam.ListEntitiesForPolicyInput{PolicyArn: p.Arn}); entErr != nil {
			t.Errorf("ListEntitiesForPolicy(%s): a registered policy must answer, got %v", aws.ToString(p.Arn), entErr)
		}
	}
}
