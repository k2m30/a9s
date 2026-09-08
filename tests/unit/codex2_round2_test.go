package unit

// codex2_round2_test.go — one owner of "which AWS-managed policies are
// dangerous", one parse of a role's inline policy documents, and a demo IAM
// fake that refuses a key the fixtures never registered.

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
)

const codex2OverPrivileged = domain.FindingCode("role-policy.broken.over_privileged")

// ─── item 1 — the dangerous-policy set is matched by the AWS-owned ARN ──────

type codex2RolePolicyFake struct {
	awsclient.IAMAPI
	attached []iamtypes.AttachedPolicy
}

func (f *codex2RolePolicyFake) ListAttachedRolePolicies(_ context.Context, _ *iam.ListAttachedRolePoliciesInput, _ ...func(*iam.Options)) (*iam.ListAttachedRolePoliciesOutput, error) {
	return &iam.ListAttachedRolePoliciesOutput{AttachedPolicies: f.attached}, nil
}

func (f *codex2RolePolicyFake) ListRolePolicies(_ context.Context, _ *iam.ListRolePoliciesInput, _ ...func(*iam.Options)) (*iam.ListRolePoliciesOutput, error) {
	return &iam.ListRolePoliciesOutput{}, nil
}

func codex2FetchRolePolicy(t *testing.T, name, arn string) resource.Resource {
	t.Helper()
	fake := &codex2RolePolicyFake{attached: []iamtypes.AttachedPolicy{
		{PolicyName: aws.String(name), PolicyArn: aws.String(arn)},
	}}
	res, err := awsclient.FetchRolePolicies(context.Background(), fake, fake,
		map[string]string{"role_name": "acme-deploy-role"}, "")
	if err != nil {
		t.Fatalf("FetchRolePolicies: %v", err)
	}
	if len(res.Resources) != 1 {
		t.Fatalf("got %d policy rows, want 1", len(res.Resources))
	}
	return res.Resources[0]
}

// TestRolePolicy_DangerousSetIsMatchedByTheAWSOwnedARN pins the fix for the
// second dangerous-policy list: the child view matched by bare PolicyName, so
// a customer-managed policy an operator happened to call AdministratorAccess
// rendered red while carrying whatever permissions its account gave it.
func TestRolePolicy_DangerousSetIsMatchedByTheAWSOwnedARN(t *testing.T) {
	for _, tc := range []struct {
		name    string
		policy  string
		arn     string
		flagged bool
	}{
		{"AWS-owned AdministratorAccess", "AdministratorAccess", "arn:aws:iam::aws:policy/AdministratorAccess", true},
		{"AWS-owned PowerUserAccess", "PowerUserAccess", "arn:aws:iam::aws:policy/PowerUserAccess", true},
		{"GovCloud AdministratorAccess", "AdministratorAccess", "arn:aws-us-gov:iam::aws:policy/AdministratorAccess", true},
		{"customer policy of the same name", "AdministratorAccess", "arn:aws:iam::123456789012:policy/AdministratorAccess", false},
		{"customer policy named PowerUserAccess", "PowerUserAccess", "arn:aws:iam::123456789012:policy/PowerUserAccess", false},
		{"an AWS read-only policy", "ReadOnlyAccess", "arn:aws:iam::aws:policy/ReadOnlyAccess", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := codex2FetchRolePolicy(t, tc.policy, tc.arn)
			if got := codex2HasCode(r.Findings, codex2OverPrivileged); got != tc.flagged {
				t.Errorf("over-privileged = %v, want %v (findings=%+v)\n"+
					"the dangerous-policy set is the one in policy_findings.go, matched by the "+
					"AWS-owned ARN — never by the policy's bare name", got, tc.flagged, r.Findings)
			}
		})
	}
}

// ─── item 2 — one parse of a role's inline policy documents ─────────────────

// TestIAMRoles_InlineResourcesReadOffTheOneParse pins that policy_resources
// comes off the same parsed document the privilege-escalation check reads.
// The separate struct-based reader accepted only an array Statement, so a
// role whose inline policy uses the bare-object form lost its resources and
// the s3→role pivot behind that field went quiet.
func TestIAMRoles_InlineResourcesReadOffTheOneParse(t *testing.T) {
	const doc = `{"Version":"2012-10-17","Statement":{"Effect":"Allow","Action":"s3:GetObject","Resource":"arn:aws:s3:::acme-reports/*"}}`
	fake := &codex2RoleFake{
		roles:    []iamtypes.Role{codex2Role("acme-deploy-role")},
		policies: []string{"reports"},
		docs:     map[string]string{"reports": doc},
	}
	res, err := awsclient.FetchIAMRolesPage(context.Background(), fake, "")
	if err != nil {
		t.Fatalf("FetchIAMRolesPage: %v", err)
	}
	if got := res.Resources[0].Fields["policy_resources"]; got != "arn:aws:s3:::acme-reports/*" {
		t.Errorf("policy_resources = %q, want %q — a single-object Statement is as legal as an array",
			got, "arn:aws:s3:::acme-reports/*")
	}
}

// ─── item 4 — the demo IAM fake refuses a key it does not hold ──────────────

// TestDemoIAMFake_RefusesAKeyTheFixturesNeverRegistered pins the fake's
// honesty rule: a GET for an entity the fixtures do not hold answers
// NoSuchEntity, and a LIST for one does too. Answering an empty result made a
// fixture gap read as "this principal has none", which is exactly the
// confident zero the demo exists to disprove.
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

// TestDemoIAMFake_StillAnswersForEveryRegisteredRole is the negative control:
// "this principal exists and has none" is a real AWS answer and must stay an
// empty list, not a refusal. Every role the fixtures hold must answer.
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
