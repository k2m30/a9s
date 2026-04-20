// fakes_us1_batch3_test.go contains lightweight fake implementations of AWS
// service client interfaces used by the US1 batch-3 checker tests.
// Covered: EKSAPI (eks→ami, eks→ec2), ASG (ng→ebs), IAM (tgw→role).
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

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
)

// ---------------------------------------------------------------------------
// fakeEKSBatch3 — implements EKSAPI for batch-3 tests
// Controllable methods: ListNodegroups, DescribeNodegroup
// Other methods return safe empty stubs.
// ---------------------------------------------------------------------------

type fakeEKSBatch3 struct {
	listNodegroupsFn    func(*eks.ListNodegroupsInput) (*eks.ListNodegroupsOutput, error)
	describeNodegroupFn func(*eks.DescribeNodegroupInput) (*eks.DescribeNodegroupOutput, error)
}

func (f *fakeEKSBatch3) ListClusters(_ context.Context, _ *eks.ListClustersInput, _ ...func(*eks.Options)) (*eks.ListClustersOutput, error) {
	return &eks.ListClustersOutput{}, nil
}

func (f *fakeEKSBatch3) DescribeCluster(_ context.Context, _ *eks.DescribeClusterInput, _ ...func(*eks.Options)) (*eks.DescribeClusterOutput, error) {
	return &eks.DescribeClusterOutput{}, nil
}

func (f *fakeEKSBatch3) ListNodegroups(_ context.Context, input *eks.ListNodegroupsInput, _ ...func(*eks.Options)) (*eks.ListNodegroupsOutput, error) {
	if f.listNodegroupsFn != nil {
		return f.listNodegroupsFn(input)
	}
	return &eks.ListNodegroupsOutput{}, nil
}

func (f *fakeEKSBatch3) DescribeNodegroup(_ context.Context, input *eks.DescribeNodegroupInput, _ ...func(*eks.Options)) (*eks.DescribeNodegroupOutput, error) {
	if f.describeNodegroupFn != nil {
		return f.describeNodegroupFn(input)
	}
	return &eks.DescribeNodegroupOutput{}, nil
}

// Compile-time check: fakeEKSBatch3 satisfies EKSAPI.
var _ awsclient.EKSAPI = (*fakeEKSBatch3)(nil)

// newFakeEKSWithNodegroups returns a fakeEKSBatch3 whose ListNodegroups returns
// the given nodegroup names and whose DescribeNodegroup returns the given
// per-name nodegroup structs (looked up by NodegroupName).
func newFakeEKSWithNodegroups(names []string, byName map[string]*ekstypes.Nodegroup) *fakeEKSBatch3 {
	return &fakeEKSBatch3{
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
// fakeIAMBatch3 — extends fakeIAMBatch2 pattern with controllable GetRole.
// Used for tgw→role tests (the checker uses a type-assertion to IAMGetRoleAPI).
// ---------------------------------------------------------------------------

type fakeIAMBatch3 struct {
	getRoleFn func(*iam.GetRoleInput) (*iam.GetRoleOutput, error)
}

func (f *fakeIAMBatch3) ListRoles(_ context.Context, _ *iam.ListRolesInput, _ ...func(*iam.Options)) (*iam.ListRolesOutput, error) {
	return &iam.ListRolesOutput{}, nil
}

func (f *fakeIAMBatch3) ListPolicies(_ context.Context, _ *iam.ListPoliciesInput, _ ...func(*iam.Options)) (*iam.ListPoliciesOutput, error) {
	return &iam.ListPoliciesOutput{}, nil
}

func (f *fakeIAMBatch3) ListUsers(_ context.Context, _ *iam.ListUsersInput, _ ...func(*iam.Options)) (*iam.ListUsersOutput, error) {
	return &iam.ListUsersOutput{}, nil
}

func (f *fakeIAMBatch3) ListGroups(_ context.Context, _ *iam.ListGroupsInput, _ ...func(*iam.Options)) (*iam.ListGroupsOutput, error) {
	return &iam.ListGroupsOutput{}, nil
}

func (f *fakeIAMBatch3) ListAttachedRolePolicies(_ context.Context, _ *iam.ListAttachedRolePoliciesInput, _ ...func(*iam.Options)) (*iam.ListAttachedRolePoliciesOutput, error) {
	return &iam.ListAttachedRolePoliciesOutput{}, nil
}

func (f *fakeIAMBatch3) ListRolePolicies(_ context.Context, _ *iam.ListRolePoliciesInput, _ ...func(*iam.Options)) (*iam.ListRolePoliciesOutput, error) {
	return &iam.ListRolePoliciesOutput{}, nil
}

func (f *fakeIAMBatch3) ListAttachedUserPolicies(_ context.Context, _ *iam.ListAttachedUserPoliciesInput, _ ...func(*iam.Options)) (*iam.ListAttachedUserPoliciesOutput, error) {
	return &iam.ListAttachedUserPoliciesOutput{}, nil
}

func (f *fakeIAMBatch3) ListAttachedGroupPolicies(_ context.Context, _ *iam.ListAttachedGroupPoliciesInput, _ ...func(*iam.Options)) (*iam.ListAttachedGroupPoliciesOutput, error) {
	return &iam.ListAttachedGroupPoliciesOutput{}, nil
}

func (f *fakeIAMBatch3) ListGroupsForUser(_ context.Context, _ *iam.ListGroupsForUserInput, _ ...func(*iam.Options)) (*iam.ListGroupsForUserOutput, error) {
	return &iam.ListGroupsForUserOutput{}, nil
}

func (f *fakeIAMBatch3) ListEntitiesForPolicy(_ context.Context, _ *iam.ListEntitiesForPolicyInput, _ ...func(*iam.Options)) (*iam.ListEntitiesForPolicyOutput, error) {
	return &iam.ListEntitiesForPolicyOutput{}, nil
}

func (f *fakeIAMBatch3) ListAccountAliases(_ context.Context, _ *iam.ListAccountAliasesInput, _ ...func(*iam.Options)) (*iam.ListAccountAliasesOutput, error) {
	return &iam.ListAccountAliasesOutput{}, nil
}

func (f *fakeIAMBatch3) GetGroup(_ context.Context, _ *iam.GetGroupInput, _ ...func(*iam.Options)) (*iam.GetGroupOutput, error) {
	return &iam.GetGroupOutput{Group: &iamtypes.Group{}}, nil
}

func (f *fakeIAMBatch3) ListGroupPolicies(_ context.Context, _ *iam.ListGroupPoliciesInput, _ ...func(*iam.Options)) (*iam.ListGroupPoliciesOutput, error) {
	return &iam.ListGroupPoliciesOutput{}, nil
}

func (f *fakeIAMBatch3) GetPolicy(_ context.Context, _ *iam.GetPolicyInput, _ ...func(*iam.Options)) (*iam.GetPolicyOutput, error) {
	return &iam.GetPolicyOutput{}, nil
}

func (f *fakeIAMBatch3) GetPolicyVersion(_ context.Context, _ *iam.GetPolicyVersionInput, _ ...func(*iam.Options)) (*iam.GetPolicyVersionOutput, error) {
	return &iam.GetPolicyVersionOutput{}, nil
}

func (f *fakeIAMBatch3) GetRole(_ context.Context, input *iam.GetRoleInput, _ ...func(*iam.Options)) (*iam.GetRoleOutput, error) {
	if f.getRoleFn != nil {
		return f.getRoleFn(input)
	}
	return &iam.GetRoleOutput{Role: &iamtypes.Role{}}, nil
}

func (f *fakeIAMBatch3) GetRolePolicy(_ context.Context, _ *iam.GetRolePolicyInput, _ ...func(*iam.Options)) (*iam.GetRolePolicyOutput, error) {
	return &iam.GetRolePolicyOutput{}, nil
}

func (f *fakeIAMBatch3) GetLoginProfile(_ context.Context, _ *iam.GetLoginProfileInput, _ ...func(*iam.Options)) (*iam.GetLoginProfileOutput, error) {
	return &iam.GetLoginProfileOutput{}, nil
}

func (f *fakeIAMBatch3) ListMFADevices(_ context.Context, _ *iam.ListMFADevicesInput, _ ...func(*iam.Options)) (*iam.ListMFADevicesOutput, error) {
	return &iam.ListMFADevicesOutput{}, nil
}

func (f *fakeIAMBatch3) ListAccessKeys(_ context.Context, _ *iam.ListAccessKeysInput, _ ...func(*iam.Options)) (*iam.ListAccessKeysOutput, error) {
	return &iam.ListAccessKeysOutput{}, nil
}

func (f *fakeIAMBatch3) GetInstanceProfile(_ context.Context, _ *iam.GetInstanceProfileInput, _ ...func(*iam.Options)) (*iam.GetInstanceProfileOutput, error) {
	return &iam.GetInstanceProfileOutput{InstanceProfile: &iamtypes.InstanceProfile{}}, nil
}

// Compile-time check: fakeIAMBatch3 satisfies IAMAPI.
var _ awsclient.IAMAPI = (*fakeIAMBatch3)(nil)

// newFakeIAMWithRole returns a fakeIAMBatch3 whose GetRole returns a Role with
// the given ARN and name.
func newFakeIAMWithRole(roleARN, roleName string) *fakeIAMBatch3 {
	return &fakeIAMBatch3{
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

// newFakeIAMWithNoSuchEntityRole returns a fakeIAMBatch3 whose GetRole returns
// a NoSuchEntity error (simulates the SLR not existing in this account).
func newFakeIAMWithNoSuchEntityRole() *fakeIAMBatch3 {
	return &fakeIAMBatch3{
		getRoleFn: func(_ *iam.GetRoleInput) (*iam.GetRoleOutput, error) {
			return nil, &smithy.GenericAPIError{
				Code:    "NoSuchEntity",
				Message: "the role does not exist",
			}
		},
	}
}

// ---------------------------------------------------------------------------
// fakeASGBatch3 — extends fakeASGBatch2 pattern with controllable
// DescribeAutoScalingGroups. Used by ng→ebs and eks→ec2 tests.
// ---------------------------------------------------------------------------

type fakeASGBatch3 struct {
	describeAutoScalingGroupsFn func(*autoscaling.DescribeAutoScalingGroupsInput) (*autoscaling.DescribeAutoScalingGroupsOutput, error)
}

func (f *fakeASGBatch3) DescribeAutoScalingGroups(_ context.Context, input *autoscaling.DescribeAutoScalingGroupsInput, _ ...func(*autoscaling.Options)) (*autoscaling.DescribeAutoScalingGroupsOutput, error) {
	if f.describeAutoScalingGroupsFn != nil {
		return f.describeAutoScalingGroupsFn(input)
	}
	return &autoscaling.DescribeAutoScalingGroupsOutput{}, nil
}

func (f *fakeASGBatch3) DescribeScalingActivities(_ context.Context, _ *autoscaling.DescribeScalingActivitiesInput, _ ...func(*autoscaling.Options)) (*autoscaling.DescribeScalingActivitiesOutput, error) {
	return &autoscaling.DescribeScalingActivitiesOutput{}, nil
}

func (f *fakeASGBatch3) DescribeLaunchConfigurations(_ context.Context, _ *autoscaling.DescribeLaunchConfigurationsInput, _ ...func(*autoscaling.Options)) (*autoscaling.DescribeLaunchConfigurationsOutput, error) {
	return &autoscaling.DescribeLaunchConfigurationsOutput{}, nil
}

func (f *fakeASGBatch3) DescribeNotificationConfigurations(_ context.Context, _ *autoscaling.DescribeNotificationConfigurationsInput, _ ...func(*autoscaling.Options)) (*autoscaling.DescribeNotificationConfigurationsOutput, error) {
	return &autoscaling.DescribeNotificationConfigurationsOutput{}, nil
}

func (f *fakeASGBatch3) DescribeLifecycleHooks(_ context.Context, _ *autoscaling.DescribeLifecycleHooksInput, _ ...func(*autoscaling.Options)) (*autoscaling.DescribeLifecycleHooksOutput, error) {
	return &autoscaling.DescribeLifecycleHooksOutput{}, nil
}

// Compile-time check: fakeASGBatch3 satisfies ASGAPI.
var _ awsclient.ASGAPI = (*fakeASGBatch3)(nil)

// newFakeASGWithGroups returns a fakeASGBatch3 whose DescribeAutoScalingGroups
// returns the given ASG structs.
func newFakeASGWithGroups(groups []asgtypes.AutoScalingGroup) *fakeASGBatch3 {
	return &fakeASGBatch3{
		describeAutoScalingGroupsFn: func(_ *autoscaling.DescribeAutoScalingGroupsInput) (*autoscaling.DescribeAutoScalingGroupsOutput, error) {
			return &autoscaling.DescribeAutoScalingGroupsOutput{
				AutoScalingGroups: groups,
			}, nil
		},
	}
}
