// fakes_eks_ng_tgw_related_test.go contains lightweight fake implementations of AWS
// service client interfaces used by the eks, ng, and tgw related-panel
// checker tests. Covered: EKSAPI (eks→ami, eks→ec2), ASG (ng→ebs), IAM (tgw→role).
// BackupAPI fakes are provided by fakes_us1_test.go (newFakeBackupWithRecoveryPoints).
// All types are in package unit_test (external test package).
package unit_test

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	smithy "github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
)

// ---------------------------------------------------------------------------
// fakeEKSForAMIEC2 — implements EKSAPI for batch-3 tests
// Controllable methods: ListNodegroups, DescribeNodegroup
// Other methods return safe empty stubs.
// ---------------------------------------------------------------------------

type fakeEKSForAMIEC2 struct {
	listNodegroupsFn    func(*eks.ListNodegroupsInput) (*eks.ListNodegroupsOutput, error)
	describeNodegroupFn func(*eks.DescribeNodegroupInput) (*eks.DescribeNodegroupOutput, error)
}

func (f *fakeEKSForAMIEC2) ListClusters(_ context.Context, _ *eks.ListClustersInput, _ ...func(*eks.Options)) (*eks.ListClustersOutput, error) {
	return &eks.ListClustersOutput{}, nil
}

func (f *fakeEKSForAMIEC2) DescribeCluster(_ context.Context, _ *eks.DescribeClusterInput, _ ...func(*eks.Options)) (*eks.DescribeClusterOutput, error) {
	return &eks.DescribeClusterOutput{}, nil
}

func (f *fakeEKSForAMIEC2) ListNodegroups(_ context.Context, input *eks.ListNodegroupsInput, _ ...func(*eks.Options)) (*eks.ListNodegroupsOutput, error) {
	if f.listNodegroupsFn != nil {
		return f.listNodegroupsFn(input)
	}
	return &eks.ListNodegroupsOutput{}, nil
}

func (f *fakeEKSForAMIEC2) DescribeNodegroup(_ context.Context, input *eks.DescribeNodegroupInput, _ ...func(*eks.Options)) (*eks.DescribeNodegroupOutput, error) {
	if f.describeNodegroupFn != nil {
		return f.describeNodegroupFn(input)
	}
	return &eks.DescribeNodegroupOutput{}, nil
}

// Compile-time check: fakeEKSForAMIEC2 satisfies EKSAPI.
var _ awsclient.EKSAPI = (*fakeEKSForAMIEC2)(nil)

// newFakeEKSWithNodegroups returns a fakeEKSForAMIEC2 whose ListNodegroups returns
// the given nodegroup names and whose DescribeNodegroup returns the given
// per-name nodegroup structs (looked up by NodegroupName).
func newFakeEKSWithNodegroups(names []string, byName map[string]*ekstypes.Nodegroup) *fakeEKSForAMIEC2 {
	return &fakeEKSForAMIEC2{
		listNodegroupsFn: func(_ *eks.ListNodegroupsInput) (*eks.ListNodegroupsOutput, error) {
			return &eks.ListNodegroupsOutput{Nodegroups: names}, nil
		},
		describeNodegroupFn: func(input *eks.DescribeNodegroupInput) (*eks.DescribeNodegroupOutput, error) {
			name := ""
			if input.NodegroupName != nil {
				name = *input.NodegroupName
			}
			ng, ok := byName[name]
			if !ok {
				return &eks.DescribeNodegroupOutput{}, nil
			}
			return &eks.DescribeNodegroupOutput{Nodegroup: ng}, nil
		},
	}
}

// ---------------------------------------------------------------------------
// fakeIAMForTGW — extends fakeIAMForASG pattern with controllable GetRole.
// Used for tgw→role tests (the checker uses a type-assertion to IAMGetRoleAPI).
// ---------------------------------------------------------------------------

type fakeIAMForTGW struct {
	getRoleFn func(*iam.GetRoleInput) (*iam.GetRoleOutput, error)
}

func (f *fakeIAMForTGW) ListRoles(_ context.Context, _ *iam.ListRolesInput, _ ...func(*iam.Options)) (*iam.ListRolesOutput, error) {
	return &iam.ListRolesOutput{}, nil
}

func (f *fakeIAMForTGW) ListPolicies(_ context.Context, _ *iam.ListPoliciesInput, _ ...func(*iam.Options)) (*iam.ListPoliciesOutput, error) {
	return &iam.ListPoliciesOutput{}, nil
}

func (f *fakeIAMForTGW) ListUsers(_ context.Context, _ *iam.ListUsersInput, _ ...func(*iam.Options)) (*iam.ListUsersOutput, error) {
	return &iam.ListUsersOutput{}, nil
}

func (f *fakeIAMForTGW) ListGroups(_ context.Context, _ *iam.ListGroupsInput, _ ...func(*iam.Options)) (*iam.ListGroupsOutput, error) {
	return &iam.ListGroupsOutput{}, nil
}

func (f *fakeIAMForTGW) ListAttachedRolePolicies(_ context.Context, _ *iam.ListAttachedRolePoliciesInput, _ ...func(*iam.Options)) (*iam.ListAttachedRolePoliciesOutput, error) {
	return &iam.ListAttachedRolePoliciesOutput{}, nil
}

func (f *fakeIAMForTGW) ListRolePolicies(_ context.Context, _ *iam.ListRolePoliciesInput, _ ...func(*iam.Options)) (*iam.ListRolePoliciesOutput, error) {
	return &iam.ListRolePoliciesOutput{}, nil
}

func (f *fakeIAMForTGW) ListAttachedUserPolicies(_ context.Context, _ *iam.ListAttachedUserPoliciesInput, _ ...func(*iam.Options)) (*iam.ListAttachedUserPoliciesOutput, error) {
	return &iam.ListAttachedUserPoliciesOutput{}, nil
}

func (f *fakeIAMForTGW) ListAttachedGroupPolicies(_ context.Context, _ *iam.ListAttachedGroupPoliciesInput, _ ...func(*iam.Options)) (*iam.ListAttachedGroupPoliciesOutput, error) {
	return &iam.ListAttachedGroupPoliciesOutput{}, nil
}

func (f *fakeIAMForTGW) ListGroupsForUser(_ context.Context, _ *iam.ListGroupsForUserInput, _ ...func(*iam.Options)) (*iam.ListGroupsForUserOutput, error) {
	return &iam.ListGroupsForUserOutput{}, nil
}

func (f *fakeIAMForTGW) ListEntitiesForPolicy(_ context.Context, _ *iam.ListEntitiesForPolicyInput, _ ...func(*iam.Options)) (*iam.ListEntitiesForPolicyOutput, error) {
	return &iam.ListEntitiesForPolicyOutput{}, nil
}

func (f *fakeIAMForTGW) ListAccountAliases(_ context.Context, _ *iam.ListAccountAliasesInput, _ ...func(*iam.Options)) (*iam.ListAccountAliasesOutput, error) {
	return &iam.ListAccountAliasesOutput{}, nil
}

func (f *fakeIAMForTGW) GetGroup(_ context.Context, _ *iam.GetGroupInput, _ ...func(*iam.Options)) (*iam.GetGroupOutput, error) {
	return &iam.GetGroupOutput{Group: &iamtypes.Group{}}, nil
}

func (f *fakeIAMForTGW) ListGroupPolicies(_ context.Context, _ *iam.ListGroupPoliciesInput, _ ...func(*iam.Options)) (*iam.ListGroupPoliciesOutput, error) {
	return &iam.ListGroupPoliciesOutput{}, nil
}

func (f *fakeIAMForTGW) GetPolicy(_ context.Context, _ *iam.GetPolicyInput, _ ...func(*iam.Options)) (*iam.GetPolicyOutput, error) {
	return &iam.GetPolicyOutput{}, nil
}

func (f *fakeIAMForTGW) GetPolicyVersion(_ context.Context, _ *iam.GetPolicyVersionInput, _ ...func(*iam.Options)) (*iam.GetPolicyVersionOutput, error) {
	return &iam.GetPolicyVersionOutput{}, nil
}

func (f *fakeIAMForTGW) GetRole(_ context.Context, input *iam.GetRoleInput, _ ...func(*iam.Options)) (*iam.GetRoleOutput, error) {
	if f.getRoleFn != nil {
		return f.getRoleFn(input)
	}
	return &iam.GetRoleOutput{Role: &iamtypes.Role{}}, nil
}

func (f *fakeIAMForTGW) GetRolePolicy(_ context.Context, _ *iam.GetRolePolicyInput, _ ...func(*iam.Options)) (*iam.GetRolePolicyOutput, error) {
	return &iam.GetRolePolicyOutput{}, nil
}

func (f *fakeIAMForTGW) GetLoginProfile(_ context.Context, _ *iam.GetLoginProfileInput, _ ...func(*iam.Options)) (*iam.GetLoginProfileOutput, error) {
	return &iam.GetLoginProfileOutput{}, nil
}

func (f *fakeIAMForTGW) ListMFADevices(_ context.Context, _ *iam.ListMFADevicesInput, _ ...func(*iam.Options)) (*iam.ListMFADevicesOutput, error) {
	return &iam.ListMFADevicesOutput{}, nil
}

func (f *fakeIAMForTGW) ListAccessKeys(_ context.Context, _ *iam.ListAccessKeysInput, _ ...func(*iam.Options)) (*iam.ListAccessKeysOutput, error) {
	return &iam.ListAccessKeysOutput{}, nil
}

func (f *fakeIAMForTGW) GetInstanceProfile(_ context.Context, _ *iam.GetInstanceProfileInput, _ ...func(*iam.Options)) (*iam.GetInstanceProfileOutput, error) {
	return &iam.GetInstanceProfileOutput{InstanceProfile: &iamtypes.InstanceProfile{}}, nil
}

// Compile-time check: fakeIAMForTGW satisfies IAMAPI.
var _ awsclient.IAMAPI = (*fakeIAMForTGW)(nil)

// newFakeIAMWithRole returns a fakeIAMForTGW whose GetRole returns a Role with
// the given ARN and name.
func newFakeIAMWithRole(roleARN, roleName string) *fakeIAMForTGW {
	return &fakeIAMForTGW{
		getRoleFn: func(_ *iam.GetRoleInput) (*iam.GetRoleOutput, error) {
			return &iam.GetRoleOutput{
				Role: &iamtypes.Role{
					Arn:      &roleARN,
					RoleName: &roleName,
				},
			}, nil
		},
	}
}

// newFakeIAMWithNoSuchEntityRole returns a fakeIAMForTGW whose GetRole returns
// a NoSuchEntity error (simulates the SLR not existing in this account).
func newFakeIAMWithNoSuchEntityRole() *fakeIAMForTGW {
	return &fakeIAMForTGW{
		getRoleFn: func(_ *iam.GetRoleInput) (*iam.GetRoleOutput, error) {
			return nil, &smithy.GenericAPIError{
				Code:    "NoSuchEntity",
				Message: "the role does not exist",
			}
		},
	}
}

// ---------------------------------------------------------------------------
// fakeASGForNG — extends fakeASGChecker pattern with controllable
// DescribeAutoScalingGroups. Used by ng→ebs and eks→ec2 tests.
// ---------------------------------------------------------------------------

type fakeASGForNG struct {
	describeAutoScalingGroupsFn func(*autoscaling.DescribeAutoScalingGroupsInput) (*autoscaling.DescribeAutoScalingGroupsOutput, error)
}

func (f *fakeASGForNG) DescribeAutoScalingGroups(_ context.Context, input *autoscaling.DescribeAutoScalingGroupsInput, _ ...func(*autoscaling.Options)) (*autoscaling.DescribeAutoScalingGroupsOutput, error) {
	if f.describeAutoScalingGroupsFn != nil {
		return f.describeAutoScalingGroupsFn(input)
	}
	return &autoscaling.DescribeAutoScalingGroupsOutput{}, nil
}

func (f *fakeASGForNG) DescribeScalingActivities(_ context.Context, _ *autoscaling.DescribeScalingActivitiesInput, _ ...func(*autoscaling.Options)) (*autoscaling.DescribeScalingActivitiesOutput, error) {
	return &autoscaling.DescribeScalingActivitiesOutput{}, nil
}

func (f *fakeASGForNG) DescribeLaunchConfigurations(_ context.Context, _ *autoscaling.DescribeLaunchConfigurationsInput, _ ...func(*autoscaling.Options)) (*autoscaling.DescribeLaunchConfigurationsOutput, error) {
	return &autoscaling.DescribeLaunchConfigurationsOutput{}, nil
}

func (f *fakeASGForNG) DescribeNotificationConfigurations(_ context.Context, _ *autoscaling.DescribeNotificationConfigurationsInput, _ ...func(*autoscaling.Options)) (*autoscaling.DescribeNotificationConfigurationsOutput, error) {
	return &autoscaling.DescribeNotificationConfigurationsOutput{}, nil
}

func (f *fakeASGForNG) DescribeLifecycleHooks(_ context.Context, _ *autoscaling.DescribeLifecycleHooksInput, _ ...func(*autoscaling.Options)) (*autoscaling.DescribeLifecycleHooksOutput, error) {
	return &autoscaling.DescribeLifecycleHooksOutput{}, nil
}

// Compile-time check: fakeASGForNG satisfies ASGAPI.
var _ awsclient.ASGAPI = (*fakeASGForNG)(nil)

// newFakeASGWithGroups returns a fakeASGForNG whose DescribeAutoScalingGroups
// returns the given ASG structs.
func newFakeASGWithGroups(groups []asgtypes.AutoScalingGroup) *fakeASGForNG {
	return &fakeASGForNG{
		describeAutoScalingGroupsFn: func(_ *autoscaling.DescribeAutoScalingGroupsInput) (*autoscaling.DescribeAutoScalingGroupsOutput, error) {
			return &autoscaling.DescribeAutoScalingGroupsOutput{
				AutoScalingGroups: groups,
			}, nil
		},
	}
}
