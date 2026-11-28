package unit

import (
	"context"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
)

const (
	t5d2PassRoleDoc    = `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":"iam:PassRole","Resource":"*"}]}`
	t5d2LambdaDoc      = `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":["lambda:CreateFunction","lambda:InvokeFunction"],"Resource":"*"}]}`
	t5d2DenyIAMDoc     = `{"Version":"2012-10-17","Statement":[{"Effect":"Deny","Action":"iam:*","Resource":"*"}]}`
	t5d2PrivEscCode    = domain.FindingCode("iam-user.privilege-escalation")
	t5d2RoleInlineCode = domain.FindingCode("role.inline-privilege-escalation")
	t5d2RolePrivEsc    = domain.FindingCode("role.privilege-escalation")
)

// t5d2Bounded serves GetRole and GetUser with a permissions boundary, the
// member ListRoles and ListUsers leave out.
type t5d2Bounded struct {
	*t5IAM
	boundaries map[string]string
}

func (f *t5d2Bounded) bounded(name string) *iamtypes.AttachedPermissionsBoundary {
	arn, ok := f.boundaries[name]
	if !ok {
		return nil
	}
	return &iamtypes.AttachedPermissionsBoundary{PermissionsBoundaryArn: aws.String(arn), PermissionsBoundaryType: iamtypes.PermissionsBoundaryAttachmentTypePolicy}
}

func (f *t5d2Bounded) GetRole(ctx context.Context, in *iam.GetRoleInput, opts ...func(*iam.Options)) (*iam.GetRoleOutput, error) {
	out, err := f.t5IAM.GetRole(ctx, in, opts...)
	if err == nil {
		out.Role.PermissionsBoundary = f.bounded(aws.ToString(in.RoleName))
	}
	return out, err
}

func (f *t5d2Bounded) GetUser(_ context.Context, in *iam.GetUserInput, _ ...func(*iam.Options)) (*iam.GetUserOutput, error) {
	name := aws.ToString(in.UserName)
	return &iam.GetUserOutput{User: &iamtypes.User{UserName: in.UserName, Arn: aws.String("arn:aws:iam::" + t5Account + ":user/" + name), PermissionsBoundary: f.bounded(name)}}, nil
}

func t5d2Run(t *testing.T, f *t5d2Bounded, short string) t5Verdict {
	t.Helper()
	clients := &awsclient.ServiceClients{IAM: f}
	return t5Enrich(t, clients, f.t5IAM, short)
}

// IAM grants a user the union of its own policies and its groups': a path
// split across them is a path.
func TestT5Dev_EscalationPathSplitAcrossUserAndGroupIsReported(t *testing.T) {
	f := newT5IAM()
	f.docs[t5PolicyARN("acme-pass")] = t5d2PassRoleDoc
	f.users = []string{"acme-dev"}
	f.groups = []string{"acme-deployers"}
	f.memberOf["acme-dev"] = []string{"acme-deployers"}
	f.attached["group/acme-deployers"] = []string{t5PolicyARN("acme-pass")}
	f.inline["user/acme-dev"] = map[string]string{"acme-lambda": t5d2LambdaDoc}
	v := t5Run(t, f, "iam-user")
	if !t5Has(v.findings["acme-dev"], t5d2PrivEscCode) {
		t.Errorf("acme-dev: no %s; findings=%+v truncated=%q", t5d2PrivEscCode, v.findings["acme-dev"], v.truncated["acme-dev"])
	}
}

// An explicit Deny in any identity policy wins over an Allow in another.
func TestT5Dev_AdministratorAccessWithADenyElsewhereIsNotAdministrator(t *testing.T) {
	f := newT5IAM()
	f.roles = []string{"acme-fenced"}
	f.attached["role/acme-fenced"] = []string{t5AdminARN}
	f.inline["role/acme-fenced"] = map[string]string{"acme-no-iam": t5d2DenyIAMDoc}
	v := t5Run(t, f, "role")
	if t5Has(v.findings["acme-fenced"], t5RoleAdmin) {
		t.Errorf("acme-fenced: %s although an inline policy denies iam:*", t5RoleAdmin)
	}
}

// One judge for a role's escalation: an inline Allow * on * is the
// administrator finding alone, a path that needs an inline grant is the
// inline code, and one the attached policies grant is the attached code.
func TestT5Dev_RoleEscalationHasOneJudge(t *testing.T) {
	f := newT5IAM()
	f.docs[t5PolicyARN("acme-pass")] = t5d2PassRoleDoc
	f.docs[t5PolicyARN("acme-editor")] = t5PrivEscDoc
	f.roles = []string{"acme-inline-admin", "acme-split", "acme-attached"}
	f.inline["role/acme-inline-admin"] = map[string]string{"acme-all": t5AdminDoc}
	f.attached["role/acme-split"] = []string{t5PolicyARN("acme-pass")}
	f.inline["role/acme-split"] = map[string]string{"acme-lambda": t5d2LambdaDoc}
	f.attached["role/acme-attached"] = []string{t5PolicyARN("acme-editor")}
	v := t5Run(t, f, "role")
	if got := v.findings["acme-inline-admin"]; len(got) != 1 || got[0].Code != t5RoleAdmin {
		t.Errorf("acme-inline-admin: findings=%+v, want %s alone", got, t5RoleAdmin)
	}
	if got := v.findings["acme-split"]; len(got) != 1 || got[0].Code != t5d2RoleInlineCode {
		t.Errorf("acme-split: findings=%+v, want %s alone", got, t5d2RoleInlineCode)
	}
	if got := v.findings["acme-attached"]; len(got) != 1 || got[0].Code != t5d2RolePrivEsc {
		t.Errorf("acme-attached: findings=%+v, want %s alone", got, t5d2RolePrivEsc)
	}
}

// A role's policy row is judged from its document, whoever owns the policy.
func TestT5Dev_RolePolicyRowIsJudgedFromItsDocument(t *testing.T) {
	f := newT5IAM()
	f.docs[t5PolicyARN("acme-break-glass")] = t5AdminDoc
	f.docs[t5PolicyARN("acme-reports")] = t5ScopedDoc
	f.roles = []string{"acme-ops"}
	f.attached["role/acme-ops"] = []string{t5PolicyARN("acme-break-glass"), t5PowerARN, t5PolicyARN("acme-reports")}
	clients := t5Clients(f)
	page, err := awsclient.FetchRolePolicies(context.Background(), f, f, map[string]string{"role_name": "acme-ops"}, "")
	if err != nil {
		t.Fatalf("FetchRolePolicies: %v", err)
	}
	judged, err := awsclient.JudgeRolePolicies(context.Background(), clients, page)
	if err != nil {
		t.Fatalf("JudgeRolePolicies: %v", err)
	}
	want := map[string]domain.FindingCode{
		t5PolicyARN("acme-break-glass"): awsclient.CodeRolePolicyAdministrator,
		t5PowerARN:                      awsclient.CodeRolePolicyBroadPower,
		t5PolicyARN("acme-reports"):     "",
	}
	for _, r := range judged.Resources {
		var got domain.FindingCode
		if len(r.Findings) > 0 {
			got = r.Findings[0].Code
		}
		if got != want[r.ID] {
			t.Errorf("%s: code %q, want %q", r.ID, got, want[r.ID])
		}
	}
}

// A permissions boundary caps what the policies grant: a path counts only
// where the boundary also allows it, and an unread boundary decides nothing.
func TestT5Dev_PermissionsBoundaryCapsTheVerdict(t *testing.T) {
	f := &t5d2Bounded{t5IAM: newT5IAM(), boundaries: map[string]string{
		"acme-bounded-role": t5ReadOnlyARN,
		"acme-open-role":    t5AdminARN,
		"acme-unread-role":  t5PolicyARN("acme-secret-boundary"),
		"acme-bounded-user": t5ReadOnlyARN,
	}}
	f.docs[t5PolicyARN("acme-secret-boundary")] = t5AdminDoc
	f.refused["GetPolicyVersion "+t5PolicyARN("acme-secret-boundary")] = true
	f.roles = []string{"acme-bounded-role", "acme-open-role", "acme-unread-role"}
	for _, r := range f.roles {
		f.attached["role/"+r] = []string{t5PowerARN}
	}
	f.users = []string{"acme-bounded-user", "acme-open-user"}
	for _, u := range f.users {
		f.attached["user/"+u] = []string{t5PowerARN}
	}
	roles := t5d2Run(t, f, "role")
	if t5Has(roles.findings["acme-bounded-role"], t5d2RolePrivEsc) {
		t.Errorf("acme-bounded-role: %s through a ReadOnlyAccess boundary", t5d2RolePrivEsc)
	}
	if !t5Has(roles.findings["acme-open-role"], t5d2RolePrivEsc) {
		t.Errorf("acme-open-role: no %s under a boundary that allows everything", t5d2RolePrivEsc)
	}
	if t5Has(roles.findings["acme-unread-role"], t5d2RolePrivEsc) || roles.truncated["acme-unread-role"] != "GetPolicyVersion" {
		t.Errorf("acme-unread-role: findings=%+v check=%q, want none and GetPolicyVersion", roles.findings["acme-unread-role"], roles.truncated["acme-unread-role"])
	}
	users := t5d2Run(t, f, "iam-user")
	if t5Has(users.findings["acme-bounded-user"], t5d2PrivEscCode) {
		t.Errorf("acme-bounded-user: %s through a ReadOnlyAccess boundary", t5d2PrivEscCode)
	}
	if !t5Has(users.findings["acme-open-user"], t5d2PrivEscCode) {
		t.Errorf("acme-open-user: no %s with no boundary", t5d2PrivEscCode)
	}
}

type t5d2CountingIAM struct {
	*t5IAM
	mu    sync.Mutex
	lists map[string]int
}

func (f *t5d2CountingIAM) ListAttachedGroupPolicies(ctx context.Context, in *iam.ListAttachedGroupPoliciesInput, opts ...func(*iam.Options)) (*iam.ListAttachedGroupPoliciesOutput, error) {
	f.mu.Lock()
	f.lists[aws.ToString(in.GroupName)]++
	f.mu.Unlock()
	return f.t5IAM.ListAttachedGroupPolicies(ctx, in, opts...)
}

func TestT5Dev_GroupPolicyListsAreReadOncePerSession(t *testing.T) {
	f := &t5d2CountingIAM{t5IAM: newT5IAM(), lists: map[string]int{}}
	f.groups = []string{"acme-shared"}
	for _, u := range []string{"acme-a", "acme-b", "acme-c"} {
		f.users = append(f.users, u)
		f.memberOf[u] = []string{"acme-shared"}
	}
	clients := &awsclient.ServiceClients{IAM: f}
	ctx := context.Background()
	users, _ := awsclient.FetchIAMUsersPage(ctx, f, "")
	if _, err := awsclient.EnrichIAMUserMFA(ctx, clients, users.Resources, nil); err != nil {
		t.Fatalf("EnrichIAMUserMFA: %v", err)
	}
	groups, _ := awsclient.FetchIAMGroupsPage(ctx, f, "")
	if _, err := awsclient.EnrichIAMGroup(ctx, clients, groups.Resources, nil); err != nil {
		t.Fatalf("EnrichIAMGroup: %v", err)
	}
	if got := f.lists["acme-shared"]; got != 1 {
		t.Errorf("ListAttachedGroupPolicies(acme-shared) called %d times for three members and the group, want 1", got)
	}
}
