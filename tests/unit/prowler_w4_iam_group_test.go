package unit

// prowler_w4_iam_group_test.go — behavioural test for the batch-w4 iam-group
// row: an admin-equivalent managed policy attached to a group hands that
// power to every current and future member.

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

const (
	w4CodeGroupAdminAttached domain.FindingCode = "iam-group.admin-attached"
	w4SourceGroupWave2                          = "wave2:iam-group"
)

// w4GroupFake serves the three calls the group enricher makes, keyed by
// group name.
type w4GroupFake struct {
	awsclient.IAMAPI
	members map[string][]string
	// attached maps group name → attached policy name → ARN.
	attached map[string]map[string]string
	inline   map[string][]string
}

func (f *w4GroupFake) GetGroup(
	_ context.Context, in *iam.GetGroupInput, _ ...func(*iam.Options),
) (*iam.GetGroupOutput, error) {
	name := aws.ToString(in.GroupName)
	var users []iamtypes.User
	for _, u := range f.members[name] {
		users = append(users, iamtypes.User{UserName: aws.String(u)})
	}
	return &iam.GetGroupOutput{
		Group: &iamtypes.Group{GroupName: in.GroupName},
		Users: users,
	}, nil
}

func (f *w4GroupFake) ListAttachedGroupPolicies(
	_ context.Context, in *iam.ListAttachedGroupPoliciesInput, _ ...func(*iam.Options),
) (*iam.ListAttachedGroupPoliciesOutput, error) {
	var out []iamtypes.AttachedPolicy
	for name, arn := range f.attached[aws.ToString(in.GroupName)] {
		out = append(out, iamtypes.AttachedPolicy{
			PolicyName: aws.String(name), PolicyArn: aws.String(arn),
		})
	}
	return &iam.ListAttachedGroupPoliciesOutput{AttachedPolicies: out}, nil
}

func (f *w4GroupFake) ListGroupPolicies(
	_ context.Context, in *iam.ListGroupPoliciesInput, _ ...func(*iam.Options),
) (*iam.ListGroupPoliciesOutput, error) {
	return &iam.ListGroupPoliciesOutput{PolicyNames: f.inline[aws.ToString(in.GroupName)]}, nil
}

var _ awsclient.IAMAPI = (*w4GroupFake)(nil)

func w4GroupResource(name string) resource.Resource {
	return resource.Resource{
		ID: name, Name: name, Type: "iam-group",
		Fields: map[string]string{"group_name": name},
	}
}

func w4EnrichGroups(t *testing.T, fake *w4GroupFake, rs []resource.Resource) awsclient.IssueEnricherResult {
	t.Helper()
	clients := &awsclient.ServiceClients{IAM: fake, Region: "us-east-1"}
	res, err := awsclient.EnrichIAMGroup(context.Background(), clients, rs, nil)
	if err != nil {
		t.Fatalf("EnrichIAMGroup: %v", err)
	}
	return res
}

// TestW4GroupAdminAttached pins that a group carrying an admin-equivalent
// managed policy is flagged, and a group with a scoped policy is not.
func TestW4GroupAdminAttached(t *testing.T) {
	fake := &w4GroupFake{
		members: map[string][]string{
			"acme-admins":  {"acme-ops-user"},
			"acme-readers": {"acme-reports-user"},
		},
		attached: map[string]map[string]string{
			"acme-admins":  {"AdministratorAccess": w4AdminAccessARN},
			"acme-readers": {"AmazonS3ReadOnlyAccess": "arn:aws:iam::aws:policy/AmazonS3ReadOnlyAccess"},
		},
	}
	res := w4EnrichGroups(t, fake, []resource.Resource{
		w4GroupResource("acme-admins"), w4GroupResource("acme-readers"),
	})

	w4AssertFinding(t, res.Findings["acme-admins"], w4CodeGroupAdminAttached,
		"has an administrator policy", domain.SevWarn, w4SourceGroupWave2)
	w4AssertRows(t, res.AttentionDetails["acme-admins"], w4CodeGroupAdminAttached,
		[]domain.DetailRow{{Label: "Policy", Value: "AdministratorAccess"}})
	w4AssertNoCode(t, res.Findings["acme-readers"], w4CodeGroupAdminAttached)
}

// TestW4GroupAdminAndOrphanAreIndependent pins that the new admin row does
// not displace the group enricher's existing membership finding: an empty
// admin group reports both, each with its own code and rows.
func TestW4GroupAdminAndOrphanAreIndependent(t *testing.T) {
	fake := &w4GroupFake{
		attached: map[string]map[string]string{
			"acme-empty-admins": {"AdministratorAccess": w4AdminAccessARN},
		},
	}
	res := w4EnrichGroups(t, fake, []resource.Resource{w4GroupResource("acme-empty-admins")})

	w4AssertFinding(t, res.Findings["acme-empty-admins"], w4CodeGroupAdminAttached,
		"has an administrator policy", domain.SevWarn, w4SourceGroupWave2)
	if _, ok := w4FindingByCode(t, res.Findings["acme-empty-admins"], "iam-group.orphan-or-noop"); !ok {
		t.Errorf("the existing membership finding was displaced; got %s",
			w4CodesOf(res.Findings["acme-empty-admins"]))
	}
}

// TestW4GroupFindingDef pins the registry row for the new group code.
func TestW4GroupFindingDef(t *testing.T) {
	def := w4FindingDef(t, "iam-group", w4CodeGroupAdminAttached)
	if def.Phrase != "has an administrator policy" {
		t.Errorf("Phrase = %q, want %q", def.Phrase, "has an administrator policy")
	}
	if def.Severity != domain.SevWarn {
		t.Errorf("Severity = %v, want SevWarn", def.Severity)
	}
	if def.Source != "wave2" {
		t.Errorf("Source = %q, want wave2", def.Source)
	}
}

// TestW4AdminPolicySetIsSharedAcrossPrincipals pins the architectural half of
// rows 3/6/10: role, user and group answer "is this admin?" from one ARN set.
// PowerUserAccess is the probe — a per-file copy of the set is exactly what
// would let one principal type recognise it and the other two miss it.
func TestW4AdminPolicySetIsSharedAcrossPrincipals(t *testing.T) {
	t.Run("role", func(t *testing.T) {
		fake := &w4RoleAdminFake{attached: map[string]map[string]string{
			"acme-power-role": {"PowerUserAccess": w4PowerUserAccessARN},
		}}
		clients := &awsclient.ServiceClients{IAM: fake, Region: "us-east-1"}
		res, err := awsclient.EnrichIAMRoleLastUsed(context.Background(), clients,
			[]resource.Resource{w4RoleResource("acme-power-role", "/")}, nil)
		if err != nil {
			t.Fatalf("EnrichIAMRoleLastUsed: %v", err)
		}
		w4AssertFinding(t, res.Findings["acme-power-role"], w4CodeRoleAdminAttached,
			w4PhraseRoleAdminAttached, domain.SevWarn, w4SourceRoleWave2)
		w4AssertRows(t, res.AttentionDetails["acme-power-role"], w4CodeRoleAdminAttached,
			[]domain.DetailRow{{Label: "Policy", Value: "PowerUserAccess"}})
	})

	t.Run("iam-user", func(t *testing.T) {
		fake := &w4UserFake{
			mfaUsers: map[string]bool{"acme-power-user": true},
			attached: map[string]map[string]string{
				"acme-power-user": {"PowerUserAccess": w4PowerUserAccessARN},
			},
		}
		res := w4EnrichUsers(t, fake, []resource.Resource{
			w4UserResource("acme-power-user", 400, "2026-01-02 09:00"),
		})
		w4AssertFinding(t, res.Findings["acme-power-user"], w4CodeUserAdminAttached,
			"has an administrator policy", domain.SevWarn, w4SourceUserWave2)
		w4AssertRows(t, res.AttentionDetails["acme-power-user"], w4CodeUserAdminAttached,
			[]domain.DetailRow{{Label: "Policy", Value: "PowerUserAccess"}})
	})

	t.Run("iam-group", func(t *testing.T) {
		fake := &w4GroupFake{
			members: map[string][]string{"acme-power-group": {"acme-power-user"}},
			attached: map[string]map[string]string{
				"acme-power-group": {"PowerUserAccess": w4PowerUserAccessARN},
			},
		}
		res := w4EnrichGroups(t, fake, []resource.Resource{w4GroupResource("acme-power-group")})
		w4AssertFinding(t, res.Findings["acme-power-group"], w4CodeGroupAdminAttached,
			"has an administrator policy", domain.SevWarn, w4SourceGroupWave2)
		w4AssertRows(t, res.AttentionDetails["acme-power-group"], w4CodeGroupAdminAttached,
			[]domain.DetailRow{{Label: "Policy", Value: "PowerUserAccess"}})
	})
}
