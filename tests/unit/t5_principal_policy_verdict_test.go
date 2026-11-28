package unit

import (
	"context"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/session"
)

const (
	t5Account = "123456789012"

	t5RoleAdmin  = domain.FindingCode("role.admin-attached")
	t5UserAdmin  = domain.FindingCode("iam-user.admin-attached")
	t5GroupAdmin = domain.FindingCode("iam-group.admin-attached")

	t5AdminDoc     = `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":"*","Resource":"*"}]}`
	t5ScopedDoc    = `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":["s3:GetObject","s3:ListBucket"],"Resource":["arn:aws:s3:::acme-reports","arn:aws:s3:::acme-reports/*"]}]}`
	t5PrivEscDoc   = `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":["iam:CreatePolicyVersion","iam:ListPolicies"],"Resource":"arn:aws:iam::123456789012:policy/*"}]}`
	t5PowerUserDoc = `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","NotAction":["iam:*","organizations:*","account:*"],"Resource":"*"},{"Effect":"Allow","Action":["account:GetAccountInformation","account:ListRegions","iam:CreateServiceLinkedRole","iam:DeleteServiceLinkedRole","iam:ListRoles","organizations:DescribeOrganization"],"Resource":"*"}]}`
	t5ReadOnlyDoc  = `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":["s3:Get*","s3:List*","ec2:Describe*","iam:Get*","iam:List*"],"Resource":"*"}]}`
	t5TrustDoc     = `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":"arn:aws:iam::123456789012:role/acme-ci"},"Action":"sts:AssumeRole"}]}`

	t5AdminARN    = "arn:aws:iam::aws:policy/AdministratorAccess"
	t5PowerARN    = "arn:aws:iam::aws:policy/PowerUserAccess"
	t5ReadOnlyARN = "arn:aws:iam::aws:policy/ReadOnlyAccess"
)

func t5PolicyARN(name string) string { return "arn:aws:iam::" + t5Account + ":policy/" + name }

// t5IAM is one account's IAM as the API reports it. Principals are keyed
// "role/<name>", "user/<name>", "group/<name>"; documents come back
// URL-encoded, as IAM returns them.
type t5IAM struct {
	awsclient.IAMAPI

	docs     map[string]string
	attached map[string][]string
	inline   map[string]map[string]string
	memberOf map[string][]string
	roles    []string
	users    []string
	groups   []string
	refused  map[string]bool

	mu           sync.Mutex
	versionReads map[string]int
}

func newT5IAM() *t5IAM {
	return &t5IAM{
		docs: map[string]string{
			t5AdminARN:    t5AdminDoc,
			t5PowerARN:    t5PowerUserDoc,
			t5ReadOnlyARN: t5ReadOnlyDoc,
		},
		attached:     map[string][]string{},
		inline:       map[string]map[string]string{},
		memberOf:     map[string][]string{},
		refused:      map[string]bool{},
		versionReads: map[string]int{},
	}
}

func t5Denied(op string) error {
	return &smithy.OperationError{ServiceID: "IAM", OperationName: op, Err: &smithy.GenericAPIError{
		Code:    "AccessDenied",
		Message: "User: arn:aws:sts::123456789012:assumed-role/example-readonly/a9s is not authorized to perform: iam:" + op,
	}}
}

func (f *t5IAM) refuse(op, key string) error {
	if f.refused[op+" "+key] {
		return t5Denied(op)
	}
	return nil
}

func t5Created() *time.Time { return aws.Time(time.Now().Add(-400 * 24 * time.Hour)) }

func (f *t5IAM) attachedOut(key string) []iamtypes.AttachedPolicy {
	var out []iamtypes.AttachedPolicy
	for _, arn := range f.attached[key] {
		out = append(out, iamtypes.AttachedPolicy{PolicyArn: aws.String(arn), PolicyName: aws.String(arn[strings.LastIndex(arn, "/")+1:])})
	}
	return out
}

func (f *t5IAM) inlineNames(key string) []string {
	var names []string
	for n := range f.inline[key] {
		names = append(names, n)
	}
	slices.Sort(names)
	return names
}

func (f *t5IAM) inlineDoc(op, key, name string) (*string, error) {
	if err := f.refuse(op, key); err != nil {
		return nil, err
	}
	doc, ok := f.inline[key][name]
	if !ok {
		return nil, &smithy.OperationError{ServiceID: "IAM", OperationName: op, Err: &iamtypes.NoSuchEntityException{Message: aws.String("The " + key + " policy with name " + name + " cannot be found.")}}
	}
	return aws.String(url.PathEscape(doc)), nil
}

func (f *t5IAM) role(name string) iamtypes.Role {
	return iamtypes.Role{
		RoleName:                 aws.String(name),
		RoleId:                   aws.String("AROAEXAMPL" + strings.ToUpper(name)),
		Arn:                      aws.String("arn:aws:iam::" + t5Account + ":role/" + name),
		Path:                     aws.String("/"),
		CreateDate:               t5Created(),
		AssumeRolePolicyDocument: aws.String(url.PathEscape(t5TrustDoc)),
	}
}

func (f *t5IAM) group(name string) iamtypes.Group {
	return iamtypes.Group{
		GroupName:  aws.String(name),
		GroupId:    aws.String("AGPAEXAMPL" + strings.ToUpper(name)),
		Arn:        aws.String("arn:aws:iam::" + t5Account + ":group/" + name),
		Path:       aws.String("/"),
		CreateDate: t5Created(),
	}
}

func (f *t5IAM) ListRoles(_ context.Context, _ *iam.ListRolesInput, _ ...func(*iam.Options)) (*iam.ListRolesOutput, error) {
	out := &iam.ListRolesOutput{}
	for _, n := range f.roles {
		out.Roles = append(out.Roles, f.role(n))
	}
	return out, nil
}

func (f *t5IAM) GetRole(_ context.Context, in *iam.GetRoleInput, _ ...func(*iam.Options)) (*iam.GetRoleOutput, error) {
	r := f.role(aws.ToString(in.RoleName))
	r.RoleLastUsed = &iamtypes.RoleLastUsed{LastUsedDate: aws.Time(time.Now().Add(-time.Hour)), Region: aws.String("us-east-1")}
	return &iam.GetRoleOutput{Role: &r}, nil
}

func (f *t5IAM) ListAttachedRolePolicies(_ context.Context, in *iam.ListAttachedRolePoliciesInput, _ ...func(*iam.Options)) (*iam.ListAttachedRolePoliciesOutput, error) {
	return &iam.ListAttachedRolePoliciesOutput{AttachedPolicies: f.attachedOut("role/" + aws.ToString(in.RoleName))}, nil
}

func (f *t5IAM) ListRolePolicies(_ context.Context, in *iam.ListRolePoliciesInput, _ ...func(*iam.Options)) (*iam.ListRolePoliciesOutput, error) {
	return &iam.ListRolePoliciesOutput{PolicyNames: f.inlineNames("role/" + aws.ToString(in.RoleName))}, nil
}

func (f *t5IAM) GetRolePolicy(_ context.Context, in *iam.GetRolePolicyInput, _ ...func(*iam.Options)) (*iam.GetRolePolicyOutput, error) {
	doc, err := f.inlineDoc("GetRolePolicy", "role/"+aws.ToString(in.RoleName), aws.ToString(in.PolicyName))
	if err != nil {
		return nil, err
	}
	return &iam.GetRolePolicyOutput{RoleName: in.RoleName, PolicyName: in.PolicyName, PolicyDocument: doc}, nil
}

func (f *t5IAM) GetPolicy(_ context.Context, in *iam.GetPolicyInput, _ ...func(*iam.Options)) (*iam.GetPolicyOutput, error) {
	arn := aws.ToString(in.PolicyArn)
	if err := f.refuse("GetPolicy", arn); err != nil {
		return nil, err
	}
	return &iam.GetPolicyOutput{Policy: &iamtypes.Policy{
		Arn:              aws.String(arn),
		PolicyName:       aws.String(arn[strings.LastIndex(arn, "/")+1:]),
		DefaultVersionId: aws.String("v1"),
		AttachmentCount:  aws.Int32(1),
		IsAttachable:     true,
		Path:             aws.String("/"),
		CreateDate:       t5Created(),
	}}, nil
}

func (f *t5IAM) GetPolicyVersion(_ context.Context, in *iam.GetPolicyVersionInput, _ ...func(*iam.Options)) (*iam.GetPolicyVersionOutput, error) {
	arn := aws.ToString(in.PolicyArn)
	f.mu.Lock()
	f.versionReads[arn]++
	f.mu.Unlock()
	if err := f.refuse("GetPolicyVersion", arn); err != nil {
		return nil, err
	}
	return &iam.GetPolicyVersionOutput{PolicyVersion: &iamtypes.PolicyVersion{
		Document:         aws.String(url.PathEscape(f.docs[arn])),
		VersionId:        in.VersionId,
		IsDefaultVersion: true,
		CreateDate:       t5Created(),
	}}, nil
}

func (f *t5IAM) ListUsers(_ context.Context, _ *iam.ListUsersInput, _ ...func(*iam.Options)) (*iam.ListUsersOutput, error) {
	out := &iam.ListUsersOutput{}
	for _, n := range f.users {
		out.Users = append(out.Users, iamtypes.User{
			UserName:   aws.String(n),
			UserId:     aws.String("AIDAEXAMPL" + strings.ToUpper(n)),
			Arn:        aws.String("arn:aws:iam::" + t5Account + ":user/" + n),
			Path:       aws.String("/"),
			CreateDate: t5Created(),
		})
	}
	return out, nil
}

// A user with no console password: GetLoginProfile answers NoSuchEntity.
func (f *t5IAM) GetLoginProfile(_ context.Context, in *iam.GetLoginProfileInput, _ ...func(*iam.Options)) (*iam.GetLoginProfileOutput, error) {
	return nil, &smithy.OperationError{ServiceID: "IAM", OperationName: "GetLoginProfile", Err: &iamtypes.NoSuchEntityException{
		Message: aws.String("Login Profile for User " + aws.ToString(in.UserName) + " cannot be found."),
	}}
}

func (f *t5IAM) ListMFADevices(_ context.Context, _ *iam.ListMFADevicesInput, _ ...func(*iam.Options)) (*iam.ListMFADevicesOutput, error) {
	return &iam.ListMFADevicesOutput{}, nil
}

func (f *t5IAM) ListAccessKeys(_ context.Context, _ *iam.ListAccessKeysInput, _ ...func(*iam.Options)) (*iam.ListAccessKeysOutput, error) {
	return &iam.ListAccessKeysOutput{}, nil
}

func (f *t5IAM) ListAttachedUserPolicies(_ context.Context, in *iam.ListAttachedUserPoliciesInput, _ ...func(*iam.Options)) (*iam.ListAttachedUserPoliciesOutput, error) {
	return &iam.ListAttachedUserPoliciesOutput{AttachedPolicies: f.attachedOut("user/" + aws.ToString(in.UserName))}, nil
}

func (f *t5IAM) ListUserPolicies(_ context.Context, in *iam.ListUserPoliciesInput, _ ...func(*iam.Options)) (*iam.ListUserPoliciesOutput, error) {
	return &iam.ListUserPoliciesOutput{PolicyNames: f.inlineNames("user/" + aws.ToString(in.UserName))}, nil
}

func (f *t5IAM) GetUserPolicy(_ context.Context, in *iam.GetUserPolicyInput, _ ...func(*iam.Options)) (*iam.GetUserPolicyOutput, error) {
	doc, err := f.inlineDoc("GetUserPolicy", "user/"+aws.ToString(in.UserName), aws.ToString(in.PolicyName))
	if err != nil {
		return nil, err
	}
	return &iam.GetUserPolicyOutput{UserName: in.UserName, PolicyName: in.PolicyName, PolicyDocument: doc}, nil
}

func (f *t5IAM) ListGroupsForUser(_ context.Context, in *iam.ListGroupsForUserInput, _ ...func(*iam.Options)) (*iam.ListGroupsForUserOutput, error) {
	out := &iam.ListGroupsForUserOutput{}
	for _, g := range f.memberOf[aws.ToString(in.UserName)] {
		out.Groups = append(out.Groups, f.group(g))
	}
	return out, nil
}

func (f *t5IAM) ListGroups(_ context.Context, _ *iam.ListGroupsInput, _ ...func(*iam.Options)) (*iam.ListGroupsOutput, error) {
	out := &iam.ListGroupsOutput{}
	for _, n := range f.groups {
		out.Groups = append(out.Groups, f.group(n))
	}
	return out, nil
}

func (f *t5IAM) GetGroup(_ context.Context, in *iam.GetGroupInput, _ ...func(*iam.Options)) (*iam.GetGroupOutput, error) {
	name := aws.ToString(in.GroupName)
	g := f.group(name)
	out := &iam.GetGroupOutput{Group: &g}
	for user, groups := range f.memberOf {
		if slices.Contains(groups, name) {
			out.Users = append(out.Users, iamtypes.User{
				UserName:   aws.String(user),
				UserId:     aws.String("AIDAEXAMPL" + strings.ToUpper(user)),
				Arn:        aws.String("arn:aws:iam::" + t5Account + ":user/" + user),
				Path:       aws.String("/"),
				CreateDate: t5Created(),
			})
		}
	}
	return out, nil
}

func (f *t5IAM) ListAttachedGroupPolicies(_ context.Context, in *iam.ListAttachedGroupPoliciesInput, _ ...func(*iam.Options)) (*iam.ListAttachedGroupPoliciesOutput, error) {
	return &iam.ListAttachedGroupPoliciesOutput{AttachedPolicies: f.attachedOut("group/" + aws.ToString(in.GroupName))}, nil
}

func (f *t5IAM) ListGroupPolicies(_ context.Context, in *iam.ListGroupPoliciesInput, _ ...func(*iam.Options)) (*iam.ListGroupPoliciesOutput, error) {
	return &iam.ListGroupPoliciesOutput{PolicyNames: f.inlineNames("group/" + aws.ToString(in.GroupName))}, nil
}

func (f *t5IAM) GetGroupPolicy(_ context.Context, in *iam.GetGroupPolicyInput, _ ...func(*iam.Options)) (*iam.GetGroupPolicyOutput, error) {
	doc, err := f.inlineDoc("GetGroupPolicy", "group/"+aws.ToString(in.GroupName), aws.ToString(in.PolicyName))
	if err != nil {
		return nil, err
	}
	return &iam.GetGroupPolicyOutput{GroupName: in.GroupName, PolicyName: in.PolicyName, PolicyDocument: doc}, nil
}

// t5EmptyIAM answers the principal policy reads of an account where no
// principal is in a group and every policy it does not name is scoped, and
// serves AWS-managed documents as AWS publishes them in every partition.
type t5EmptyIAM struct{ awsclient.IAMAPI }

func (t5EmptyIAM) ListRolePolicies(_ context.Context, _ *iam.ListRolePoliciesInput, _ ...func(*iam.Options)) (*iam.ListRolePoliciesOutput, error) {
	return &iam.ListRolePoliciesOutput{}, nil
}

func (t5EmptyIAM) ListUserPolicies(_ context.Context, _ *iam.ListUserPoliciesInput, _ ...func(*iam.Options)) (*iam.ListUserPoliciesOutput, error) {
	return &iam.ListUserPoliciesOutput{}, nil
}

func (t5EmptyIAM) ListGroupPolicies(_ context.Context, _ *iam.ListGroupPoliciesInput, _ ...func(*iam.Options)) (*iam.ListGroupPoliciesOutput, error) {
	return &iam.ListGroupPoliciesOutput{}, nil
}

func (t5EmptyIAM) ListGroupsForUser(_ context.Context, _ *iam.ListGroupsForUserInput, _ ...func(*iam.Options)) (*iam.ListGroupsForUserOutput, error) {
	return &iam.ListGroupsForUserOutput{}, nil
}

func (t5EmptyIAM) GetRolePolicy(_ context.Context, in *iam.GetRolePolicyInput, _ ...func(*iam.Options)) (*iam.GetRolePolicyOutput, error) {
	return &iam.GetRolePolicyOutput{RoleName: in.RoleName, PolicyName: in.PolicyName, PolicyDocument: aws.String(url.PathEscape(t5ScopedDoc))}, nil
}

func (t5EmptyIAM) GetUserPolicy(_ context.Context, in *iam.GetUserPolicyInput, _ ...func(*iam.Options)) (*iam.GetUserPolicyOutput, error) {
	return &iam.GetUserPolicyOutput{UserName: in.UserName, PolicyName: in.PolicyName, PolicyDocument: aws.String(url.PathEscape(t5ScopedDoc))}, nil
}

func (t5EmptyIAM) GetGroupPolicy(_ context.Context, in *iam.GetGroupPolicyInput, _ ...func(*iam.Options)) (*iam.GetGroupPolicyOutput, error) {
	return &iam.GetGroupPolicyOutput{GroupName: in.GroupName, PolicyName: in.PolicyName, PolicyDocument: aws.String(url.PathEscape(t5ScopedDoc))}, nil
}

func (t5EmptyIAM) GetPolicy(_ context.Context, in *iam.GetPolicyInput, _ ...func(*iam.Options)) (*iam.GetPolicyOutput, error) {
	arn := aws.ToString(in.PolicyArn)
	return &iam.GetPolicyOutput{Policy: &iamtypes.Policy{
		Arn: in.PolicyArn, PolicyName: aws.String(arn[strings.LastIndex(arn, "/")+1:]),
		DefaultVersionId: aws.String("v1"), AttachmentCount: aws.Int32(1), IsAttachable: true,
	}}, nil
}

func (t5EmptyIAM) GetPolicyVersion(_ context.Context, in *iam.GetPolicyVersionInput, _ ...func(*iam.Options)) (*iam.GetPolicyVersionOutput, error) {
	arn := aws.ToString(in.PolicyArn)
	doc := t5ScopedDoc
	switch {
	case strings.HasSuffix(arn, ":iam::aws:policy/AdministratorAccess"):
		doc = t5AdminDoc
	case strings.HasSuffix(arn, ":iam::aws:policy/PowerUserAccess"):
		doc = t5PowerUserDoc
	case strings.HasSuffix(arn, ":iam::aws:policy/ReadOnlyAccess"):
		doc = t5ReadOnlyDoc
	}
	return &iam.GetPolicyVersionOutput{PolicyVersion: &iamtypes.PolicyVersion{
		Document: aws.String(url.PathEscape(doc)), VersionId: in.VersionId, IsDefaultVersion: true,
	}}, nil
}

// t5Verdict is what the operator sees for one principal list: every finding a
// row carries from its fetch and from its issue enricher, and the rows left
// not inspected.
type t5Verdict struct {
	findings  map[string][]domain.Finding
	truncated map[string]string
}

func t5Clients(f *t5IAM) *awsclient.ServiceClients {
	clients := &awsclient.ServiceClients{IAM: f}
	session.New().WireStores(clients)
	return clients
}

func t5Enrich(t *testing.T, clients *awsclient.ServiceClients, f *t5IAM, short string) t5Verdict {
	t.Helper()
	ctx := context.Background()
	var page resource.FetchResult
	var err error
	var enrich func(context.Context, *awsclient.ServiceClients, []resource.Resource, resource.ResourceCache) (awsclient.IssueEnricherResult, error)
	switch short {
	case "role":
		page, err = awsclient.FetchIAMRolesPage(ctx, f, "")
		enrich = awsclient.EnrichIAMRoleLastUsed
	case "iam-user":
		page, err = awsclient.FetchIAMUsersPage(ctx, f, "")
		enrich = awsclient.EnrichIAMUserMFA
	case "iam-group":
		page, err = awsclient.FetchIAMGroupsPage(ctx, f, "")
		enrich = awsclient.EnrichIAMGroup
	}
	if err != nil {
		t.Fatalf("%s fetch: %v", short, err)
	}
	res, enrichErr := enrich(ctx, clients, page.Resources, nil)
	if enrichErr != nil {
		t.Logf("%s enricher: %v", short, enrichErr)
	}
	v := t5Verdict{findings: map[string][]domain.Finding{}, truncated: res.TruncatedIDs}
	for _, r := range page.Resources {
		v.findings[r.ID] = append(slices.Clone(r.Findings), res.Findings[r.ID]...)
	}
	return v
}

func t5Run(t *testing.T, f *t5IAM, short string) t5Verdict {
	t.Helper()
	return t5Enrich(t, t5Clients(f), f, short)
}

func t5Has(fs []domain.Finding, code domain.FindingCode) bool {
	return slices.ContainsFunc(fs, func(f domain.Finding) bool { return f.Code == code })
}

func t5PrivEsc(fs []domain.Finding) (domain.Finding, bool) {
	i := slices.IndexFunc(fs, func(f domain.Finding) bool { return strings.Contains(f.Phrase, "privilege escalation") })
	if i < 0 {
		return domain.Finding{}, false
	}
	return fs[i], true
}

func t5AssertAdmin(t *testing.T, v t5Verdict, id string, code domain.FindingCode) {
	t.Helper()
	if !t5Has(v.findings[id], code) {
		t.Errorf("%s: no %s; findings=%+v truncated=%q", id, code, v.findings[id], v.truncated[id])
		return
	}
	i := slices.IndexFunc(v.findings[id], func(f domain.Finding) bool { return f.Code == code })
	if f := v.findings[id][i]; f.Phrase != "has an administrator policy" || f.Severity != domain.SevWarn {
		t.Errorf("%s: %s = {Phrase %q, Severity %v}, want {%q, SevWarn}", id, code, f.Phrase, f.Severity, "has an administrator policy")
	}
}

func t5AssertClean(t *testing.T, v t5Verdict, id string) {
	t.Helper()
	if len(v.findings[id]) != 0 {
		t.Errorf("%s: findings=%+v, want none", id, v.findings[id])
	}
	if check, ok := v.truncated[id]; ok {
		t.Errorf("%s: not inspected (%q), want inspected", id, check)
	}
}

// IAM evaluates a principal's permissions as the union of every policy that
// applies to it, so a customer-managed Allow * on * makes the role exactly
// as much an administrator as AWS's AdministratorAccess does; the policy row
// and the role row read the same document.
func TestT5RoleWithCustomerAdminPolicyIsAdministrator(t *testing.T) {
	f := newT5IAM()
	f.docs[t5PolicyARN("acme-break-glass")] = t5AdminDoc
	f.docs[t5PolicyARN("acme-reports-read")] = t5ScopedDoc
	f.roles = []string{"acme-break-glass-role", "acme-reports-role"}
	f.attached["role/acme-break-glass-role"] = []string{t5PolicyARN("acme-break-glass")}
	f.attached["role/acme-reports-role"] = []string{t5PolicyARN("acme-reports-read")}

	v := t5Run(t, f, "role")
	t5AssertAdmin(t, v, "acme-break-glass-role", t5RoleAdmin)
	t5AssertClean(t, v, "acme-reports-role")

	policyRow := []resource.Resource{{
		ID: t5PolicyARN("acme-break-glass"), Name: "acme-break-glass",
		Fields: map[string]string{"policy_name": "acme-break-glass", "attachment_count": "1", "arn": t5PolicyARN("acme-break-glass")},
		RawStruct: iamtypes.Policy{
			Arn: aws.String(t5PolicyARN("acme-break-glass")), PolicyName: aws.String("acme-break-glass"),
			DefaultVersionId: aws.String("v1"), AttachmentCount: aws.Int32(1), IsAttachable: true,
		},
	}}
	pres, err := awsclient.EnrichIAMPolicy(context.Background(), t5Clients(f), policyRow, nil)
	if err != nil {
		t.Fatalf("EnrichIAMPolicy: %v", err)
	}
	if !t5Has(pres.Findings[t5PolicyARN("acme-break-glass")], "iam-policy.admin-star") {
		t.Errorf("policy row: no iam-policy.admin-star; findings=%+v", pres.Findings[t5PolicyARN("acme-break-glass")])
	}
}

func TestT5RoleWithInlineAdminPolicyIsAdministrator(t *testing.T) {
	f := newT5IAM()
	f.roles = []string{"acme-ops-role", "acme-etl-role"}
	f.inline["role/acme-ops-role"] = map[string]string{"acme-everything": t5AdminDoc}
	f.inline["role/acme-etl-role"] = map[string]string{"acme-reports": t5ScopedDoc}

	v := t5Run(t, f, "role")
	t5AssertAdmin(t, v, "acme-ops-role", t5RoleAdmin)
	t5AssertClean(t, v, "acme-etl-role")
}

func TestT5UserAdministratorThroughGroupAttachedPolicy(t *testing.T) {
	f := newT5IAM()
	f.docs[t5PolicyARN("acme-admins")] = t5AdminDoc
	f.docs[t5PolicyARN("acme-reports-read")] = t5ScopedDoc
	f.users = []string{"acme-alice", "acme-bob"}
	f.groups = []string{"acme-admins", "acme-analysts"}
	f.memberOf["acme-alice"] = []string{"acme-analysts", "acme-admins"}
	f.memberOf["acme-bob"] = []string{"acme-analysts"}
	f.attached["group/acme-admins"] = []string{t5PolicyARN("acme-admins")}
	f.attached["group/acme-analysts"] = []string{t5PolicyARN("acme-reports-read")}

	users := t5Run(t, f, "iam-user")
	t5AssertAdmin(t, users, "acme-alice", t5UserAdmin)
	t5AssertClean(t, users, "acme-bob")

	groups := t5Run(t, f, "iam-group")
	t5AssertAdmin(t, groups, "acme-admins", t5GroupAdmin)
	t5AssertClean(t, groups, "acme-analysts")
}

func TestT5UserAndGroupInlineAdminPolicy(t *testing.T) {
	f := newT5IAM()
	f.users = []string{"acme-carol", "acme-dave"}
	f.groups = []string{"acme-oncall", "acme-readers"}
	f.memberOf["acme-erin"] = []string{"acme-oncall", "acme-readers"}
	f.inline["user/acme-carol"] = map[string]string{"acme-full": t5AdminDoc}
	f.inline["user/acme-dave"] = map[string]string{"acme-reports": t5ScopedDoc}
	f.inline["group/acme-oncall"] = map[string]string{"acme-full": t5AdminDoc}
	f.inline["group/acme-readers"] = map[string]string{"acme-reports": t5ScopedDoc}

	users := t5Run(t, f, "iam-user")
	t5AssertAdmin(t, users, "acme-carol", t5UserAdmin)
	t5AssertClean(t, users, "acme-dave")

	groups := t5Run(t, f, "iam-group")
	t5AssertAdmin(t, groups, "acme-oncall", t5GroupAdmin)
	t5AssertClean(t, groups, "acme-readers")
}

// AdministratorAccess is administrator because its document allows every
// action on every resource; PowerUserAccess allows every action except IAM,
// Organizations and Account, so it is not. Without the document there is no
// verdict, whatever the policy is called.
func TestT5AWSManagedAdministratorAccessDecidedFromDocument(t *testing.T) {
	for _, tc := range []struct {
		short, key string
		code       domain.FindingCode
	}{
		{"role", "role/", t5RoleAdmin},
		{"iam-user", "user/", t5UserAdmin},
		{"iam-group", "group/", t5GroupAdmin},
	} {
		t.Run(tc.short, func(t *testing.T) {
			f := newT5IAM()
			names := []string{"acme-admin", "acme-power", "acme-readonly"}
			f.roles, f.users, f.groups = names, names, names
			for _, n := range names {
				f.memberOf["acme-member-"+n] = []string{n}
			}
			f.attached[tc.key+"acme-admin"] = []string{t5AdminARN}
			f.attached[tc.key+"acme-power"] = []string{t5PowerARN}
			f.attached[tc.key+"acme-readonly"] = []string{t5ReadOnlyARN}

			v := t5Run(t, f, tc.short)
			t5AssertAdmin(t, v, "acme-admin", tc.code)
			for _, n := range []string{"acme-power", "acme-readonly"} {
				if t5Has(v.findings[n], tc.code) {
					t.Errorf("%s: %s, want none (document is not Allow * on *)", n, tc.code)
				}
			}

			f.refused["GetPolicyVersion "+t5AdminARN] = true
			denied := t5Run(t, f, tc.short)
			if t5Has(denied.findings["acme-admin"], tc.code) {
				t.Errorf("acme-admin: %s with its document refused, want no verdict", tc.code)
			}
			if got := denied.truncated["acme-admin"]; got != "GetPolicyVersion" {
				t.Errorf("acme-admin not-inspected check = %q, want %q", got, "GetPolicyVersion")
			}
		})
	}
}

func TestT5UserAndGroupPrivilegeEscalation(t *testing.T) {
	f := newT5IAM()
	f.docs[t5PolicyARN("acme-policy-editor")] = t5PrivEscDoc
	f.users = []string{"acme-frank", "acme-grace", "acme-heidi", "acme-ivan"}
	f.groups = []string{"acme-platform", "acme-editors", "acme-viewers"}
	f.attached["user/acme-frank"] = []string{t5PolicyARN("acme-policy-editor")}
	f.inline["user/acme-grace"] = map[string]string{"acme-editor": t5PrivEscDoc}
	f.memberOf["acme-heidi"] = []string{"acme-platform"}
	f.memberOf["acme-ivan"] = []string{"acme-viewers"}
	f.memberOf["acme-judy"] = []string{"acme-editors"}
	f.attached["group/acme-platform"] = []string{t5PolicyARN("acme-policy-editor")}
	f.inline["group/acme-editors"] = map[string]string{"acme-editor": t5PrivEscDoc}
	f.inline["group/acme-viewers"] = map[string]string{"acme-reports": t5ScopedDoc}

	users := t5Run(t, f, "iam-user")
	for _, id := range []string{"acme-frank", "acme-grace", "acme-heidi"} {
		pe, ok := t5PrivEsc(users.findings[id])
		if !ok {
			t.Errorf("user %s: no privilege-escalation finding; findings=%+v truncated=%q", id, users.findings[id], users.truncated[id])
			continue
		}
		if pe.Severity != domain.SevBroken {
			t.Errorf("user %s: privilege escalation severity = %v, want SevBroken", id, pe.Severity)
		}
		if t5Has(users.findings[id], t5UserAdmin) {
			t.Errorf("user %s: administrator finding on a document that is not Allow * on *", id)
		}
	}
	t5AssertClean(t, users, "acme-ivan")

	groups := t5Run(t, f, "iam-group")
	for _, id := range []string{"acme-platform", "acme-editors"} {
		pe, ok := t5PrivEsc(groups.findings[id])
		if !ok {
			t.Errorf("group %s: no privilege-escalation finding; findings=%+v truncated=%q", id, groups.findings[id], groups.truncated[id])
			continue
		}
		if pe.Severity != domain.SevBroken {
			t.Errorf("group %s: privilege escalation severity = %v, want SevBroken", id, pe.Severity)
		}
	}
	t5AssertClean(t, groups, "acme-viewers")
}

func TestT5ScopedPoliciesOnlyProduceNoFinding(t *testing.T) {
	f := newT5IAM()
	f.docs[t5PolicyARN("acme-reports-read")] = t5ScopedDoc
	f.roles = []string{"acme-app-role"}
	f.users = []string{"acme-kim"}
	f.groups = []string{"acme-analysts"}
	f.memberOf["acme-kim"] = []string{"acme-analysts"}
	for _, key := range []string{"role/acme-app-role", "user/acme-kim", "group/acme-analysts"} {
		f.attached[key] = []string{t5PolicyARN("acme-reports-read"), t5ReadOnlyARN}
		f.inline[key] = map[string]string{"acme-reports": t5ScopedDoc}
	}

	t5AssertClean(t, t5Run(t, f, "role"), "acme-app-role")
	t5AssertClean(t, t5Run(t, f, "iam-user"), "acme-kim")
	t5AssertClean(t, t5Run(t, f, "iam-group"), "acme-analysts")
}

// A document nobody could read may be the one that grants administrator, so
// the principal is not inspected, never clean; the mark names the refused call.
func TestT5RefusedDocumentReadMarksPrincipalNotInspected(t *testing.T) {
	for _, tc := range []struct {
		name, short, id, wantCheck string
		setup                      func(f *t5IAM)
	}{
		{"role attached", "role", "acme-app-role", "GetPolicyVersion", func(f *t5IAM) {
			f.roles = []string{"acme-app-role"}
			f.attached["role/acme-app-role"] = []string{t5PolicyARN("acme-app")}
			f.refused["GetPolicyVersion "+t5PolicyARN("acme-app")] = true
		}},
		{"user inline", "iam-user", "acme-leo", "GetUserPolicy", func(f *t5IAM) {
			f.users = []string{"acme-leo"}
			f.inline["user/acme-leo"] = map[string]string{"acme-app": t5ScopedDoc}
			f.refused["GetUserPolicy user/acme-leo"] = true
		}},
		{"user through group", "iam-user", "acme-mia", "GetPolicyVersion", func(f *t5IAM) {
			f.users = []string{"acme-mia"}
			f.memberOf["acme-mia"] = []string{"acme-deployers"}
			f.attached["group/acme-deployers"] = []string{t5PolicyARN("acme-deploy")}
			f.refused["GetPolicyVersion "+t5PolicyARN("acme-deploy")] = true
		}},
		{"group inline", "iam-group", "acme-deployers", "GetGroupPolicy", func(f *t5IAM) {
			f.groups = []string{"acme-deployers"}
			f.memberOf["acme-mia"] = []string{"acme-deployers"}
			f.inline["group/acme-deployers"] = map[string]string{"acme-deploy": t5ScopedDoc}
			f.refused["GetGroupPolicy group/acme-deployers"] = true
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newT5IAM()
			f.docs[t5PolicyARN("acme-app")] = t5AdminDoc
			f.docs[t5PolicyARN("acme-deploy")] = t5AdminDoc
			tc.setup(f)
			v := t5Run(t, f, tc.short)
			if got, ok := v.truncated[tc.id]; !ok || got != tc.wantCheck {
				t.Errorf("%s not-inspected check = %q (marked %v), want %q", tc.id, got, ok, tc.wantCheck)
			}
			for _, fd := range v.findings[tc.id] {
				if fd.Code == t5RoleAdmin || fd.Code == t5UserAdmin || fd.Code == t5GroupAdmin {
					t.Errorf("%s: %s from a document that was never read", tc.id, fd.Code)
				}
			}
		})
	}
}

// IAM stores one document per policy version, so twenty roles holding the
// same policy need its document once per session, across Wave 2 passes too.
func TestT5PolicyAttachedToTwentyRolesIsReadOncePerSession(t *testing.T) {
	f := newT5IAM()
	shared := t5PolicyARN("acme-shared-admin")
	f.docs[shared] = t5AdminDoc
	for i := range 20 {
		name := "acme-svc-" + string(rune('a'+i))
		f.roles = append(f.roles, name)
		f.attached["role/"+name] = []string{shared}
	}
	clients := t5Clients(f)

	v := t5Enrich(t, clients, f, "role")
	for _, name := range f.roles {
		t5AssertAdmin(t, v, name, t5RoleAdmin)
	}
	if got := f.versionReads[shared]; got != 1 {
		t.Errorf("GetPolicyVersion(%s) called %d times in one pass over 20 roles, want 1", shared, got)
	}

	t5Enrich(t, clients, f, "role")
	if got := f.versionReads[shared]; got != 1 {
		t.Errorf("GetPolicyVersion(%s) called %d times after a second pass in the same session, want 1", shared, got)
	}
}
