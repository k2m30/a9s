// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// Package fakes provides in-process fake implementations of AWS service
// interfaces for use in demo mode and tests.
package fakes

import (
	"context"
	"fmt"
	"slices"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	"github.com/k2m30/a9s/v3/core/demo/fixtures"
)

// IAMFake implements aws.IAMAPI against fixture data loaded at construction time.
type IAMFake struct {
	fix *fixtures.IAMFixtures
}

// NewIAM constructs an IAMFake backed by fixture data from the fixtures package.
func NewIAM() *IAMFake {
	return &IAMFake{fix: fixtures.NewIAMFixtures()}
}

// noSuchEntity is the modeled refusal every lookup below answers with. The
// fake holds a whole account, so a key it does not hold is a key the account
// does not have — and answering an empty result for one makes a fixture gap
// read as "this principal has none", the confident zero demo mode exists to
// disprove. The code, not the Go type name, is what IsNotFoundErr reads.
func noSuchEntity(kind, name string) error {
	return &iamtypes.NoSuchEntityException{
		Message: aws.String(kind + " " + name + " cannot be found."),
	}
}

func (f *IAMFake) hasRole(name string) bool {
	return slices.ContainsFunc(f.fix.Roles, func(r iamtypes.Role) bool { return aws.ToString(r.RoleName) == name })
}

func (f *IAMFake) hasUser(name string) bool {
	return slices.ContainsFunc(f.fix.Users, func(u iamtypes.User) bool { return aws.ToString(u.UserName) == name })
}

func (f *IAMFake) hasGroup(name string) bool {
	return slices.ContainsFunc(f.fix.Groups, func(g iamtypes.Group) bool { return aws.ToString(g.GroupName) == name })
}

func (f *IAMFake) ListRoles(_ context.Context, _ *iam.ListRolesInput, _ ...func(*iam.Options)) (*iam.ListRolesOutput, error) {
	return &iam.ListRolesOutput{Roles: f.fix.Roles}, nil
}

func (f *IAMFake) ListPolicies(_ context.Context, input *iam.ListPoliciesInput, _ ...func(*iam.Options)) (*iam.ListPoliciesOutput, error) {
	policies := make([]iamtypes.Policy, 0, len(f.fix.Policies))
	for _, policy := range f.fix.Policies {
		if policy.Arn == nil {
			policies = append(policies, policy)
			continue
		}
		isCustomerManaged := fixtures.IsCustomerManagedPolicyARN(*policy.Arn)
		switch {
		case input == nil || input.Scope == "":
			policies = append(policies, policy)
		case input.Scope == iamtypes.PolicyScopeTypeAll:
			policies = append(policies, policy)
		case input.Scope == iamtypes.PolicyScopeTypeLocal && isCustomerManaged:
			policies = append(policies, policy)
		case input.Scope == iamtypes.PolicyScopeTypeAws && !isCustomerManaged:
			policies = append(policies, policy)
		}
	}
	return &iam.ListPoliciesOutput{Policies: policies}, nil
}

func (f *IAMFake) ListUsers(_ context.Context, _ *iam.ListUsersInput, _ ...func(*iam.Options)) (*iam.ListUsersOutput, error) {
	return &iam.ListUsersOutput{Users: f.fix.Users}, nil
}

func (f *IAMFake) ListGroups(_ context.Context, _ *iam.ListGroupsInput, _ ...func(*iam.Options)) (*iam.ListGroupsOutput, error) {
	return &iam.ListGroupsOutput{Groups: f.fix.Groups}, nil
}

func (f *IAMFake) ListAttachedRolePolicies(_ context.Context, input *iam.ListAttachedRolePoliciesInput, _ ...func(*iam.Options)) (*iam.ListAttachedRolePoliciesOutput, error) {
	if input.RoleName == nil {
		return nil, fmt.Errorf("ListAttachedRolePolicies: role name is required")
	}
	if !f.hasRole(*input.RoleName) {
		return nil, noSuchEntity("Role with name", *input.RoleName)
	}
	return &iam.ListAttachedRolePoliciesOutput{AttachedPolicies: f.fix.AttachedRolePolicies[*input.RoleName]}, nil
}

func (f *IAMFake) ListRolePolicies(_ context.Context, input *iam.ListRolePoliciesInput, _ ...func(*iam.Options)) (*iam.ListRolePoliciesOutput, error) {
	if input.RoleName == nil {
		return nil, fmt.Errorf("ListRolePolicies: role name is required")
	}
	if !f.hasRole(*input.RoleName) {
		return nil, noSuchEntity("Role with name", *input.RoleName)
	}
	return &iam.ListRolePoliciesOutput{PolicyNames: f.fix.InlineRolePolicies[*input.RoleName]}, nil
}

func (f *IAMFake) ListAttachedUserPolicies(_ context.Context, input *iam.ListAttachedUserPoliciesInput, _ ...func(*iam.Options)) (*iam.ListAttachedUserPoliciesOutput, error) {
	if input.UserName == nil {
		return nil, fmt.Errorf("ListAttachedUserPolicies: user name is required")
	}
	if !f.hasUser(*input.UserName) {
		return nil, noSuchEntity("User with name", *input.UserName)
	}
	return &iam.ListAttachedUserPoliciesOutput{AttachedPolicies: f.fix.AttachedUserPolicies[*input.UserName]}, nil
}

func (f *IAMFake) ListAttachedGroupPolicies(_ context.Context, input *iam.ListAttachedGroupPoliciesInput, _ ...func(*iam.Options)) (*iam.ListAttachedGroupPoliciesOutput, error) {
	if input.GroupName == nil {
		return nil, fmt.Errorf("ListAttachedGroupPolicies: group name is required")
	}
	if !f.hasGroup(*input.GroupName) {
		return nil, noSuchEntity("Group with name", *input.GroupName)
	}
	return &iam.ListAttachedGroupPoliciesOutput{AttachedPolicies: f.fix.AttachedGroupPolicies[*input.GroupName]}, nil
}

func (f *IAMFake) ListGroupsForUser(_ context.Context, input *iam.ListGroupsForUserInput, _ ...func(*iam.Options)) (*iam.ListGroupsForUserOutput, error) {
	if input.UserName == nil {
		return nil, fmt.Errorf("ListGroupsForUser: user name is required")
	}
	if !f.hasUser(*input.UserName) {
		return nil, noSuchEntity("User with name", *input.UserName)
	}
	return &iam.ListGroupsForUserOutput{Groups: f.fix.GroupsForUser[*input.UserName]}, nil
}

func (f *IAMFake) ListEntitiesForPolicy(_ context.Context, input *iam.ListEntitiesForPolicyInput, _ ...func(*iam.Options)) (*iam.ListEntitiesForPolicyOutput, error) {
	if input.PolicyArn == nil {
		return nil, fmt.Errorf("ListEntitiesForPolicy: policy ARN is required")
	}
	if err := validateARN(*input.PolicyArn); err != nil {
		return nil, err
	}
	entities, ok := f.fix.EntitiesForPolicy[*input.PolicyArn]
	if !ok {
		return nil, noSuchEntity("Policy", *input.PolicyArn)
	}
	return &iam.ListEntitiesForPolicyOutput{
		PolicyRoles:  entities.Roles,
		PolicyUsers:  entities.Users,
		PolicyGroups: entities.Groups,
	}, nil
}

func (f *IAMFake) ListAccountAliases(_ context.Context, _ *iam.ListAccountAliasesInput, _ ...func(*iam.Options)) (*iam.ListAccountAliasesOutput, error) {
	return &iam.ListAccountAliasesOutput{AccountAliases: f.fix.AccountAliases}, nil
}

func (f *IAMFake) GetGroup(_ context.Context, input *iam.GetGroupInput, _ ...func(*iam.Options)) (*iam.GetGroupOutput, error) {
	if input.GroupName == nil {
		return nil, fmt.Errorf("GetGroup: group name is required")
	}
	users := f.fix.GroupUsers[*input.GroupName]
	// Find the group
	var group *iamtypes.Group
	for i := range f.fix.Groups {
		if f.fix.Groups[i].GroupName != nil && *f.fix.Groups[i].GroupName == *input.GroupName {
			g := f.fix.Groups[i]
			group = &g
			break
		}
	}
	if group == nil {
		return nil, noSuchEntity("Group with name", *input.GroupName)
	}
	return &iam.GetGroupOutput{Group: group, Users: users}, nil
}

func (f *IAMFake) ListGroupPolicies(_ context.Context, input *iam.ListGroupPoliciesInput, _ ...func(*iam.Options)) (*iam.ListGroupPoliciesOutput, error) {
	name := aws.ToString(input.GroupName)
	if !f.hasGroup(name) {
		return nil, noSuchEntity("Group with name", name)
	}
	return &iam.ListGroupPoliciesOutput{PolicyNames: f.fix.InlineGroupPolicies[name]}, nil
}

func (f *IAMFake) GetPolicy(_ context.Context, input *iam.GetPolicyInput, _ ...func(*iam.Options)) (*iam.GetPolicyOutput, error) {
	if input.PolicyArn == nil {
		return nil, fmt.Errorf("GetPolicy: policy ARN is required")
	}
	if err := validateARN(*input.PolicyArn); err != nil {
		return nil, err
	}
	for i := range f.fix.Policies {
		if f.fix.Policies[i].Arn != nil && *f.fix.Policies[i].Arn == *input.PolicyArn {
			p := f.fix.Policies[i]
			return &iam.GetPolicyOutput{Policy: &p}, nil
		}
	}
	return nil, noSuchEntity("Policy", *input.PolicyArn)
}

func (f *IAMFake) GetPolicyVersion(_ context.Context, input *iam.GetPolicyVersionInput, _ ...func(*iam.Options)) (*iam.GetPolicyVersionOutput, error) {
	if input.PolicyArn == nil {
		return nil, fmt.Errorf("GetPolicyVersion: policy ARN is required")
	}
	if err := validateARN(*input.PolicyArn); err != nil {
		return nil, err
	}
	doc, ok := f.fix.PolicyDocuments[*input.PolicyArn]
	if !ok {
		return nil, noSuchEntity("Policy", *input.PolicyArn)
	}
	return &iam.GetPolicyVersionOutput{
		PolicyVersion: &iamtypes.PolicyVersion{
			Document:  aws.String(doc),
			VersionId: input.VersionId,
		},
	}, nil
}

func (f *IAMFake) GetRole(_ context.Context, input *iam.GetRoleInput, _ ...func(*iam.Options)) (*iam.GetRoleOutput, error) {
	if input.RoleName == nil {
		return nil, fmt.Errorf("GetRole: role name is required")
	}
	for i := range f.fix.Roles {
		if f.fix.Roles[i].RoleName != nil && *f.fix.Roles[i].RoleName == *input.RoleName {
			r := f.fix.Roles[i]
			return &iam.GetRoleOutput{Role: &r}, nil
		}
	}
	// Match the real AWS IAM API shape so checkers that ask for the code
	// "NoSuchEntity" (e.g. checkTGWRole, checkXRole service-linked-role
	// probes) can distinguish "role absent" (Count=0) from "unexpected
	// error" (Count=-1). The modeled exception answers the code
	// "NoSuchEntity" — the code is not the type's name.
	return nil, &iamtypes.NoSuchEntityException{
		Message: aws.String("Role with name " + *input.RoleName + " cannot be found."),
	}
}

func (f *IAMFake) GetRolePolicy(_ context.Context, input *iam.GetRolePolicyInput, _ ...func(*iam.Options)) (*iam.GetRolePolicyOutput, error) {
	if input.RoleName == nil {
		return nil, fmt.Errorf("GetRolePolicy: role name is required")
	}
	if input.PolicyName == nil {
		return nil, fmt.Errorf("GetRolePolicy: policy name is required")
	}
	key := *input.RoleName + "/" + *input.PolicyName
	doc, ok := f.fix.InlinePolicyDocuments[key]
	if !ok {
		// Real IAM cannot list a policy name and then have no document for
		// it. Answering an empty string here made a fixture gap look like an
		// empty policy, which the roles fetcher can only read as unparseable.
		return nil, &iamtypes.NoSuchEntityException{
			Message: aws.String("The role policy with name " + *input.PolicyName + " cannot be found."),
		}
	}
	return &iam.GetRolePolicyOutput{
		PolicyName:     input.PolicyName,
		RoleName:       input.RoleName,
		PolicyDocument: aws.String(doc),
	}, nil
}

// GetLoginProfile returns a login profile for fixture-registered console
// users (see IAMFixtures.ConsoleUsers) and NoSuchEntityException for
// everyone else, matching real AWS behavior for a user with no console
// password configured. Required for EnrichIAMUserMFA's Wave-2 issue check.
func (f *IAMFake) GetLoginProfile(_ context.Context, input *iam.GetLoginProfileInput, _ ...func(*iam.Options)) (*iam.GetLoginProfileOutput, error) {
	userName := aws.ToString(input.UserName)
	if !f.fix.ConsoleUsers[userName] {
		return nil, &iamtypes.NoSuchEntityException{
			Message: aws.String(fmt.Sprintf("Login Profile for User %s cannot be found.", userName)),
		}
	}
	return &iam.GetLoginProfileOutput{
		LoginProfile: &iamtypes.LoginProfile{UserName: aws.String(userName)},
	}, nil
}

// ListMFADevices returns the fixture-registered MFA devices for the
// requested user (see IAMFixtures.MFADevicesByUser). Required for
// EnrichIAMUserMFA's Wave-2 issue check.
func (f *IAMFake) ListMFADevices(_ context.Context, input *iam.ListMFADevicesInput, _ ...func(*iam.Options)) (*iam.ListMFADevicesOutput, error) {
	userName := aws.ToString(input.UserName)
	if !f.hasUser(userName) {
		return nil, noSuchEntity("User with name", userName)
	}
	return &iam.ListMFADevicesOutput{MFADevices: f.fix.MFADevicesByUser[userName]}, nil
}

// ListAccessKeys returns the fixture-registered access keys for the
// requested user (see IAMFixtures.AccessKeysByUser). Required for
// EnrichIAMUserMFA's Wave-2 stale-key issue check.
func (f *IAMFake) ListAccessKeys(_ context.Context, input *iam.ListAccessKeysInput, _ ...func(*iam.Options)) (*iam.ListAccessKeysOutput, error) {
	userName := aws.ToString(input.UserName)
	if !f.hasUser(userName) {
		return nil, noSuchEntity("User with name", userName)
	}
	return &iam.ListAccessKeysOutput{AccessKeyMetadata: f.fix.AccessKeysByUser[userName]}, nil
}

// GetAccessKeyLastUsed returns the fixture-registered last-use record for an
// access key (see IAMFixtures.AccessKeyLastUsed). A key with no entry has
// never been used, which is what the real API reports as a nil LastUsedDate.
// Required for EnrichIAMUserMFA's Wave-2 unused-key issue check.
func (f *IAMFake) GetAccessKeyLastUsed(_ context.Context, input *iam.GetAccessKeyLastUsedInput, _ ...func(*iam.Options)) (*iam.GetAccessKeyLastUsedOutput, error) {
	keyID := aws.ToString(input.AccessKeyId)
	last := &iamtypes.AccessKeyLastUsed{ServiceName: aws.String("N/A"), Region: aws.String("N/A")}
	if used, ok := f.fix.AccessKeyLastUsed[keyID]; ok {
		last.LastUsedDate = aws.Time(used)
		last.ServiceName = aws.String("s3")
		last.Region = aws.String("us-east-1")
	}
	return &iam.GetAccessKeyLastUsedOutput{AccessKeyLastUsed: last}, nil
}

// GetInstanceProfile resolves an instance-profile name from fixture data.
// Backs the asg:role and eb:role related-panel pivots (checkASGRole /
// checkEbRole via asgInstanceProfileToRoles).
func (f *IAMFake) GetInstanceProfile(_ context.Context, input *iam.GetInstanceProfileInput, _ ...func(*iam.Options)) (*iam.GetInstanceProfileOutput, error) {
	name := aws.ToString(input.InstanceProfileName)
	profile, ok := f.fix.InstanceProfiles[name]
	if !ok {
		return nil, noSuchEntity("Instance Profile", name)
	}
	return &iam.GetInstanceProfileOutput{InstanceProfile: &profile}, nil
}
