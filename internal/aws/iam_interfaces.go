package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/iam"
)

// IAMListRolesAPI defines the interface for the IAM ListRoles operation.
type IAMListRolesAPI interface {
	ListRoles(ctx context.Context, params *iam.ListRolesInput, optFns ...func(*iam.Options)) (*iam.ListRolesOutput, error)
}

// IAMGetRoleAPI defines the interface for the IAM GetRole operation.
type IAMGetRoleAPI interface {
	GetRole(ctx context.Context, params *iam.GetRoleInput, optFns ...func(*iam.Options)) (*iam.GetRoleOutput, error)
}

// IAMListPoliciesAPI defines the interface for the IAM ListPolicies operation.
type IAMListPoliciesAPI interface {
	ListPolicies(ctx context.Context, params *iam.ListPoliciesInput, optFns ...func(*iam.Options)) (*iam.ListPoliciesOutput, error)
}

// IAMListUsersAPI defines the interface for the IAM ListUsers operation.
type IAMListUsersAPI interface {
	ListUsers(ctx context.Context, params *iam.ListUsersInput, optFns ...func(*iam.Options)) (*iam.ListUsersOutput, error)
}

// IAMListGroupsAPI defines the interface for the IAM ListGroups operation.
type IAMListGroupsAPI interface {
	ListGroups(ctx context.Context, params *iam.ListGroupsInput, optFns ...func(*iam.Options)) (*iam.ListGroupsOutput, error)
}

// IAMGetLoginProfileAPI defines the interface for the IAM GetLoginProfile operation.
// Used by Wave 2 EnrichIAMUserMFA to detect console users without MFA (CIS IAM.5).
type IAMGetLoginProfileAPI interface {
	GetLoginProfile(ctx context.Context, params *iam.GetLoginProfileInput, optFns ...func(*iam.Options)) (*iam.GetLoginProfileOutput, error)
}

// IAMListMFADevicesAPI defines the interface for the IAM ListMFADevices operation.
// Used by Wave 2 EnrichIAMUserMFA to detect console users without MFA (CIS IAM.5).
type IAMListMFADevicesAPI interface {
	ListMFADevices(ctx context.Context, params *iam.ListMFADevicesInput, optFns ...func(*iam.Options)) (*iam.ListMFADevicesOutput, error)
}

// IAMListAccessKeysAPI defines the interface for the IAM ListAccessKeys operation.
// Used by Wave 2 EnrichIAMUserMFA to detect stale access keys (>90d rotation).
type IAMListAccessKeysAPI interface {
	ListAccessKeys(ctx context.Context, params *iam.ListAccessKeysInput, optFns ...func(*iam.Options)) (*iam.ListAccessKeysOutput, error)
}

// IAMListAttachedRolePoliciesAPI defines the interface for the IAM ListAttachedRolePolicies operation.
type IAMListAttachedRolePoliciesAPI interface {
	ListAttachedRolePolicies(ctx context.Context, params *iam.ListAttachedRolePoliciesInput, optFns ...func(*iam.Options)) (*iam.ListAttachedRolePoliciesOutput, error)
}

// IAMListRolePoliciesAPI defines the interface for the IAM ListRolePolicies operation.
type IAMListRolePoliciesAPI interface {
	ListRolePolicies(ctx context.Context, params *iam.ListRolePoliciesInput, optFns ...func(*iam.Options)) (*iam.ListRolePoliciesOutput, error)
}

// IAMGetGroupAPI defines the interface for the IAM GetGroup operation.
type IAMGetGroupAPI interface {
	GetGroup(ctx context.Context, params *iam.GetGroupInput, optFns ...func(*iam.Options)) (*iam.GetGroupOutput, error)
}

// IAMListGroupPoliciesAPI defines the interface for the IAM ListGroupPolicies operation.
type IAMListGroupPoliciesAPI interface {
	ListGroupPolicies(ctx context.Context, params *iam.ListGroupPoliciesInput, optFns ...func(*iam.Options)) (*iam.ListGroupPoliciesOutput, error)
}

// IAMGetPolicyAPI defines the interface for the IAM GetPolicy operation.
type IAMGetPolicyAPI interface {
	GetPolicy(ctx context.Context, params *iam.GetPolicyInput, optFns ...func(*iam.Options)) (*iam.GetPolicyOutput, error)
}

// IAMGetPolicyVersionAPI defines the interface for the IAM GetPolicyVersion operation.
type IAMGetPolicyVersionAPI interface {
	GetPolicyVersion(ctx context.Context, params *iam.GetPolicyVersionInput, optFns ...func(*iam.Options)) (*iam.GetPolicyVersionOutput, error)
}

// IAMGetRolePolicyAPI defines the interface for the IAM GetRolePolicy operation.
type IAMGetRolePolicyAPI interface {
	GetRolePolicy(ctx context.Context, params *iam.GetRolePolicyInput, optFns ...func(*iam.Options)) (*iam.GetRolePolicyOutput, error)
}

// IAMListAccountAliasesAPI defines the interface for the IAM ListAccountAliases operation.
type IAMListAccountAliasesAPI interface {
	ListAccountAliases(ctx context.Context, params *iam.ListAccountAliasesInput, optFns ...func(*iam.Options)) (*iam.ListAccountAliasesOutput, error)
}

// IAMListAttachedUserPoliciesAPI defines the interface for the IAM ListAttachedUserPolicies operation.
type IAMListAttachedUserPoliciesAPI interface {
	ListAttachedUserPolicies(ctx context.Context, params *iam.ListAttachedUserPoliciesInput, optFns ...func(*iam.Options)) (*iam.ListAttachedUserPoliciesOutput, error)
}

// IAMListAttachedGroupPoliciesAPI defines the interface for the IAM ListAttachedGroupPolicies operation.
type IAMListAttachedGroupPoliciesAPI interface {
	ListAttachedGroupPolicies(ctx context.Context, params *iam.ListAttachedGroupPoliciesInput, optFns ...func(*iam.Options)) (*iam.ListAttachedGroupPoliciesOutput, error)
}

// IAMListGroupsForUserAPI defines the interface for the IAM ListGroupsForUser operation.
type IAMListGroupsForUserAPI interface {
	ListGroupsForUser(ctx context.Context, params *iam.ListGroupsForUserInput, optFns ...func(*iam.Options)) (*iam.ListGroupsForUserOutput, error)
}

// IAMListEntitiesForPolicyAPI defines the interface for the IAM ListEntitiesForPolicy operation.
type IAMListEntitiesForPolicyAPI interface {
	ListEntitiesForPolicy(ctx context.Context, params *iam.ListEntitiesForPolicyInput, optFns ...func(*iam.Options)) (*iam.ListEntitiesForPolicyOutput, error)
}

// IAMGetInstanceProfileAPI for asg→role and eb→role via IamInstanceProfile.
type IAMGetInstanceProfileAPI interface {
	GetInstanceProfile(ctx context.Context, params *iam.GetInstanceProfileInput, optFns ...func(*iam.Options)) (*iam.GetInstanceProfileOutput, error)
}

// IAMAPI is the aggregate interface covering all IAM operations used by a9s fetchers.
// *iam.Client structurally satisfies this interface.
type IAMAPI interface {
	IAMListRolesAPI
	IAMListPoliciesAPI
	IAMListUsersAPI
	IAMListGroupsAPI
	IAMListAttachedRolePoliciesAPI
	IAMListRolePoliciesAPI
	IAMListAttachedUserPoliciesAPI
	IAMListAttachedGroupPoliciesAPI
	IAMListGroupsForUserAPI
	IAMListEntitiesForPolicyAPI
	IAMListAccountAliasesAPI
	IAMGetGroupAPI
	IAMListGroupPoliciesAPI
	IAMGetPolicyAPI
	IAMGetPolicyVersionAPI
	IAMGetRolePolicyAPI
	// Wave 2 enrichment interfaces.
	IAMGetLoginProfileAPI
	IAMListMFADevicesAPI
	IAMListAccessKeysAPI
	IAMGetInstanceProfileAPI // asg→role, eb→role via IamInstanceProfile
}
