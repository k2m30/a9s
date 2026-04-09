// Package fakes provides in-process fake implementations of AWS service
// interfaces for use in demo mode and tests.
package fakes

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
)

// IAMFake implements aws.IAMAPI against fixture data loaded at construction time.
type IAMFake struct {
	fix *fixtures.IAMFixtures
}

// NewIAM constructs an IAMFake backed by fixture data from the fixtures package.
func NewIAM() *IAMFake {
	return &IAMFake{fix: fixtures.NewIAMFixtures()}
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
	policies := f.fix.AttachedRolePolicies[*input.RoleName]
	return &iam.ListAttachedRolePoliciesOutput{AttachedPolicies: policies}, nil
}

func (f *IAMFake) ListRolePolicies(_ context.Context, input *iam.ListRolePoliciesInput, _ ...func(*iam.Options)) (*iam.ListRolePoliciesOutput, error) {
	if input.RoleName == nil {
		return nil, fmt.Errorf("ListRolePolicies: role name is required")
	}
	names := f.fix.InlineRolePolicies[*input.RoleName]
	return &iam.ListRolePoliciesOutput{PolicyNames: names}, nil
}

func (f *IAMFake) ListAttachedUserPolicies(_ context.Context, input *iam.ListAttachedUserPoliciesInput, _ ...func(*iam.Options)) (*iam.ListAttachedUserPoliciesOutput, error) {
	if input.UserName == nil {
		return nil, fmt.Errorf("ListAttachedUserPolicies: user name is required")
	}
	policies := f.fix.AttachedUserPolicies[*input.UserName]
	return &iam.ListAttachedUserPoliciesOutput{AttachedPolicies: policies}, nil
}

func (f *IAMFake) ListAttachedGroupPolicies(_ context.Context, input *iam.ListAttachedGroupPoliciesInput, _ ...func(*iam.Options)) (*iam.ListAttachedGroupPoliciesOutput, error) {
	if input.GroupName == nil {
		return nil, fmt.Errorf("ListAttachedGroupPolicies: group name is required")
	}
	policies := f.fix.AttachedGroupPolicies[*input.GroupName]
	return &iam.ListAttachedGroupPoliciesOutput{AttachedPolicies: policies}, nil
}

func (f *IAMFake) ListGroupsForUser(_ context.Context, input *iam.ListGroupsForUserInput, _ ...func(*iam.Options)) (*iam.ListGroupsForUserOutput, error) {
	if input.UserName == nil {
		return nil, fmt.Errorf("ListGroupsForUser: user name is required")
	}
	groups := f.fix.GroupsForUser[*input.UserName]
	return &iam.ListGroupsForUserOutput{Groups: groups}, nil
}

func (f *IAMFake) ListEntitiesForPolicy(_ context.Context, input *iam.ListEntitiesForPolicyInput, _ ...func(*iam.Options)) (*iam.ListEntitiesForPolicyOutput, error) {
	if input.PolicyArn == nil {
		return nil, fmt.Errorf("ListEntitiesForPolicy: policy ARN is required")
	}
	entities := f.fix.EntitiesForPolicy[*input.PolicyArn]
	if entities == nil {
		return &iam.ListEntitiesForPolicyOutput{}, nil
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
		return nil, fmt.Errorf("group %q not found", *input.GroupName)
	}
	return &iam.GetGroupOutput{Group: group, Users: users}, nil
}

func (f *IAMFake) ListGroupPolicies(_ context.Context, input *iam.ListGroupPoliciesInput, _ ...func(*iam.Options)) (*iam.ListGroupPoliciesOutput, error) {
	name := aws.ToString(input.GroupName)
	policies := f.fix.InlineGroupPolicies[name]
	return &iam.ListGroupPoliciesOutput{PolicyNames: policies}, nil
}
