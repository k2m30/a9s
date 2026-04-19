// fakes_us1_batch2_test.go contains lightweight fake implementations of AWS
// service client interfaces used by the US1 batch-2 checker tests (T009–T014
// for asg, T020–T024 for eb). All types are in package unit_test (external
// test package).
package unit_test

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk/types"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/aws-sdk-go-v2/service/sns"
)

// ---------------------------------------------------------------------------
// fakeASGBatch2 — implements ASGAPI for batch-2 tests
// Controllable methods: DescribeLaunchConfigurations, DescribeNotificationConfigurations
// Other methods return safe empty stubs.
// ---------------------------------------------------------------------------

type fakeASGBatch2 struct {
	describeLaunchConfigsFn       func(*autoscaling.DescribeLaunchConfigurationsInput) (*autoscaling.DescribeLaunchConfigurationsOutput, error)
	describeNotificationConfigsFn func(*autoscaling.DescribeNotificationConfigurationsInput) (*autoscaling.DescribeNotificationConfigurationsOutput, error)
}

func (f *fakeASGBatch2) DescribeAutoScalingGroups(_ context.Context, _ *autoscaling.DescribeAutoScalingGroupsInput, _ ...func(*autoscaling.Options)) (*autoscaling.DescribeAutoScalingGroupsOutput, error) {
	return &autoscaling.DescribeAutoScalingGroupsOutput{}, nil
}

func (f *fakeASGBatch2) DescribeScalingActivities(_ context.Context, _ *autoscaling.DescribeScalingActivitiesInput, _ ...func(*autoscaling.Options)) (*autoscaling.DescribeScalingActivitiesOutput, error) {
	return &autoscaling.DescribeScalingActivitiesOutput{}, nil
}

func (f *fakeASGBatch2) DescribeLaunchConfigurations(_ context.Context, input *autoscaling.DescribeLaunchConfigurationsInput, _ ...func(*autoscaling.Options)) (*autoscaling.DescribeLaunchConfigurationsOutput, error) {
	if f.describeLaunchConfigsFn != nil {
		return f.describeLaunchConfigsFn(input)
	}
	return &autoscaling.DescribeLaunchConfigurationsOutput{}, nil
}

func (f *fakeASGBatch2) DescribeNotificationConfigurations(_ context.Context, input *autoscaling.DescribeNotificationConfigurationsInput, _ ...func(*autoscaling.Options)) (*autoscaling.DescribeNotificationConfigurationsOutput, error) {
	if f.describeNotificationConfigsFn != nil {
		return f.describeNotificationConfigsFn(input)
	}
	return &autoscaling.DescribeNotificationConfigurationsOutput{}, nil
}

func (f *fakeASGBatch2) DescribeLifecycleHooks(_ context.Context, _ *autoscaling.DescribeLifecycleHooksInput, _ ...func(*autoscaling.Options)) (*autoscaling.DescribeLifecycleHooksOutput, error) {
	return &autoscaling.DescribeLifecycleHooksOutput{}, nil
}

// newFakeASGWithLaunchConfig returns a fakeASGBatch2 that returns the given
// LaunchConfiguration (with SecurityGroups and IamInstanceProfile) from
// DescribeLaunchConfigurations.
func newFakeASGWithLaunchConfig(lcName string, iamProfile string, sgs []string) *fakeASGBatch2 {
	return &fakeASGBatch2{
		describeLaunchConfigsFn: func(_ *autoscaling.DescribeLaunchConfigurationsInput) (*autoscaling.DescribeLaunchConfigurationsOutput, error) {
			lc := asgtypes.LaunchConfiguration{
				LaunchConfigurationName: &lcName,
				IamInstanceProfile:      &iamProfile,
				SecurityGroups:          sgs,
			}
			return &autoscaling.DescribeLaunchConfigurationsOutput{
				LaunchConfigurations: []asgtypes.LaunchConfiguration{lc},
			}, nil
		},
	}
}

// newFakeASGWithNotifications returns a fakeASGBatch2 that returns the given
// notification configs from DescribeNotificationConfigurations.
func newFakeASGWithNotifications(configs []asgtypes.NotificationConfiguration) *fakeASGBatch2 {
	return &fakeASGBatch2{
		describeNotificationConfigsFn: func(_ *autoscaling.DescribeNotificationConfigurationsInput) (*autoscaling.DescribeNotificationConfigurationsOutput, error) {
			return &autoscaling.DescribeNotificationConfigurationsOutput{
				NotificationConfigurations: configs,
			}, nil
		},
	}
}

// ---------------------------------------------------------------------------
// fakeEC2Batch2 — implements EC2API for batch-2 tests
// Controllable method: DescribeSubnets, DescribeLaunchTemplateVersions
// Other methods return safe empty stubs.
// ---------------------------------------------------------------------------

type fakeEC2Batch2 struct {
	describeSubnetsFn                func(*ec2.DescribeSubnetsInput) (*ec2.DescribeSubnetsOutput, error)
	describeLaunchTemplateVersionsFn func(*ec2.DescribeLaunchTemplateVersionsInput) (*ec2.DescribeLaunchTemplateVersionsOutput, error)
}

func (f *fakeEC2Batch2) DescribeInstances(_ context.Context, _ *ec2.DescribeInstancesInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	return &ec2.DescribeInstancesOutput{}, nil
}

func (f *fakeEC2Batch2) DescribeInstanceStatus(_ context.Context, _ *ec2.DescribeInstanceStatusInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstanceStatusOutput, error) {
	return &ec2.DescribeInstanceStatusOutput{}, nil
}

func (f *fakeEC2Batch2) DescribeVpcs(_ context.Context, _ *ec2.DescribeVpcsInput, _ ...func(*ec2.Options)) (*ec2.DescribeVpcsOutput, error) {
	return &ec2.DescribeVpcsOutput{}, nil
}

func (f *fakeEC2Batch2) DescribeSecurityGroups(_ context.Context, _ *ec2.DescribeSecurityGroupsInput, _ ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error) {
	return &ec2.DescribeSecurityGroupsOutput{}, nil
}

func (f *fakeEC2Batch2) DescribeSubnets(_ context.Context, input *ec2.DescribeSubnetsInput, _ ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error) {
	if f.describeSubnetsFn != nil {
		return f.describeSubnetsFn(input)
	}
	return &ec2.DescribeSubnetsOutput{}, nil
}

func (f *fakeEC2Batch2) DescribeRouteTables(_ context.Context, _ *ec2.DescribeRouteTablesInput, _ ...func(*ec2.Options)) (*ec2.DescribeRouteTablesOutput, error) {
	return &ec2.DescribeRouteTablesOutput{}, nil
}

func (f *fakeEC2Batch2) DescribeNatGateways(_ context.Context, _ *ec2.DescribeNatGatewaysInput, _ ...func(*ec2.Options)) (*ec2.DescribeNatGatewaysOutput, error) {
	return &ec2.DescribeNatGatewaysOutput{}, nil
}

func (f *fakeEC2Batch2) DescribeInternetGateways(_ context.Context, _ *ec2.DescribeInternetGatewaysInput, _ ...func(*ec2.Options)) (*ec2.DescribeInternetGatewaysOutput, error) {
	return &ec2.DescribeInternetGatewaysOutput{}, nil
}

func (f *fakeEC2Batch2) DescribeAddresses(_ context.Context, _ *ec2.DescribeAddressesInput, _ ...func(*ec2.Options)) (*ec2.DescribeAddressesOutput, error) {
	return &ec2.DescribeAddressesOutput{}, nil
}

func (f *fakeEC2Batch2) DescribeTransitGateways(_ context.Context, _ *ec2.DescribeTransitGatewaysInput, _ ...func(*ec2.Options)) (*ec2.DescribeTransitGatewaysOutput, error) {
	return &ec2.DescribeTransitGatewaysOutput{}, nil
}

func (f *fakeEC2Batch2) DescribeTransitGatewayAttachments(_ context.Context, _ *ec2.DescribeTransitGatewayAttachmentsInput, _ ...func(*ec2.Options)) (*ec2.DescribeTransitGatewayAttachmentsOutput, error) {
	return &ec2.DescribeTransitGatewayAttachmentsOutput{}, nil
}

func (f *fakeEC2Batch2) DescribeVpcEndpoints(_ context.Context, _ *ec2.DescribeVpcEndpointsInput, _ ...func(*ec2.Options)) (*ec2.DescribeVpcEndpointsOutput, error) {
	return &ec2.DescribeVpcEndpointsOutput{}, nil
}

func (f *fakeEC2Batch2) DescribeNetworkInterfaces(_ context.Context, _ *ec2.DescribeNetworkInterfacesInput, _ ...func(*ec2.Options)) (*ec2.DescribeNetworkInterfacesOutput, error) {
	return &ec2.DescribeNetworkInterfacesOutput{}, nil
}

func (f *fakeEC2Batch2) DescribeVolumes(_ context.Context, _ *ec2.DescribeVolumesInput, _ ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error) {
	return &ec2.DescribeVolumesOutput{}, nil
}

func (f *fakeEC2Batch2) DescribeSnapshots(_ context.Context, _ *ec2.DescribeSnapshotsInput, _ ...func(*ec2.Options)) (*ec2.DescribeSnapshotsOutput, error) {
	return &ec2.DescribeSnapshotsOutput{}, nil
}

func (f *fakeEC2Batch2) DescribeImages(_ context.Context, _ *ec2.DescribeImagesInput, _ ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
	return &ec2.DescribeImagesOutput{}, nil
}

func (f *fakeEC2Batch2) DescribeVolumeStatus(_ context.Context, _ *ec2.DescribeVolumeStatusInput, _ ...func(*ec2.Options)) (*ec2.DescribeVolumeStatusOutput, error) {
	return &ec2.DescribeVolumeStatusOutput{}, nil
}

func (f *fakeEC2Batch2) DescribeFlowLogs(_ context.Context, _ *ec2.DescribeFlowLogsInput, _ ...func(*ec2.Options)) (*ec2.DescribeFlowLogsOutput, error) {
	return &ec2.DescribeFlowLogsOutput{}, nil
}

func (f *fakeEC2Batch2) DescribeTransitGatewayVpcAttachments(_ context.Context, _ *ec2.DescribeTransitGatewayVpcAttachmentsInput, _ ...func(*ec2.Options)) (*ec2.DescribeTransitGatewayVpcAttachmentsOutput, error) {
	return &ec2.DescribeTransitGatewayVpcAttachmentsOutput{}, nil
}

func (f *fakeEC2Batch2) DescribeTransitGatewayRouteTables(_ context.Context, _ *ec2.DescribeTransitGatewayRouteTablesInput, _ ...func(*ec2.Options)) (*ec2.DescribeTransitGatewayRouteTablesOutput, error) {
	return &ec2.DescribeTransitGatewayRouteTablesOutput{}, nil
}

func (f *fakeEC2Batch2) DescribeLaunchTemplateVersions(_ context.Context, input *ec2.DescribeLaunchTemplateVersionsInput, _ ...func(*ec2.Options)) (*ec2.DescribeLaunchTemplateVersionsOutput, error) {
	if f.describeLaunchTemplateVersionsFn != nil {
		return f.describeLaunchTemplateVersionsFn(input)
	}
	return &ec2.DescribeLaunchTemplateVersionsOutput{}, nil
}

// newFakeEC2WithSubnets returns a fakeEC2Batch2 whose DescribeSubnets returns
// the supplied subnets.
func newFakeEC2WithSubnets(subnets []ec2types.Subnet) *fakeEC2Batch2 {
	return &fakeEC2Batch2{
		describeSubnetsFn: func(_ *ec2.DescribeSubnetsInput) (*ec2.DescribeSubnetsOutput, error) {
			return &ec2.DescribeSubnetsOutput{Subnets: subnets}, nil
		},
	}
}

// newFakeEC2WithLaunchTemplateVersions returns a fakeEC2Batch2 whose
// DescribeLaunchTemplateVersions returns the supplied versions.
func newFakeEC2WithLaunchTemplateVersions(versions []ec2types.LaunchTemplateVersion) *fakeEC2Batch2 {
	return &fakeEC2Batch2{
		describeLaunchTemplateVersionsFn: func(_ *ec2.DescribeLaunchTemplateVersionsInput) (*ec2.DescribeLaunchTemplateVersionsOutput, error) {
			return &ec2.DescribeLaunchTemplateVersionsOutput{LaunchTemplateVersions: versions}, nil
		},
	}
}

// ---------------------------------------------------------------------------
// fakeIAMBatch2 — implements IAMAPI for batch-2 tests
// Controllable method: GetInstanceProfile
// Other methods return safe empty stubs.
// ---------------------------------------------------------------------------

type fakeIAMBatch2 struct {
	getInstanceProfileFn func(*iam.GetInstanceProfileInput) (*iam.GetInstanceProfileOutput, error)
}

func (f *fakeIAMBatch2) ListRoles(_ context.Context, _ *iam.ListRolesInput, _ ...func(*iam.Options)) (*iam.ListRolesOutput, error) {
	return &iam.ListRolesOutput{}, nil
}

func (f *fakeIAMBatch2) ListPolicies(_ context.Context, _ *iam.ListPoliciesInput, _ ...func(*iam.Options)) (*iam.ListPoliciesOutput, error) {
	return &iam.ListPoliciesOutput{}, nil
}

func (f *fakeIAMBatch2) ListUsers(_ context.Context, _ *iam.ListUsersInput, _ ...func(*iam.Options)) (*iam.ListUsersOutput, error) {
	return &iam.ListUsersOutput{}, nil
}

func (f *fakeIAMBatch2) ListGroups(_ context.Context, _ *iam.ListGroupsInput, _ ...func(*iam.Options)) (*iam.ListGroupsOutput, error) {
	return &iam.ListGroupsOutput{}, nil
}

func (f *fakeIAMBatch2) ListAttachedRolePolicies(_ context.Context, _ *iam.ListAttachedRolePoliciesInput, _ ...func(*iam.Options)) (*iam.ListAttachedRolePoliciesOutput, error) {
	return &iam.ListAttachedRolePoliciesOutput{}, nil
}

func (f *fakeIAMBatch2) ListRolePolicies(_ context.Context, _ *iam.ListRolePoliciesInput, _ ...func(*iam.Options)) (*iam.ListRolePoliciesOutput, error) {
	return &iam.ListRolePoliciesOutput{}, nil
}

func (f *fakeIAMBatch2) ListAttachedUserPolicies(_ context.Context, _ *iam.ListAttachedUserPoliciesInput, _ ...func(*iam.Options)) (*iam.ListAttachedUserPoliciesOutput, error) {
	return &iam.ListAttachedUserPoliciesOutput{}, nil
}

func (f *fakeIAMBatch2) ListAttachedGroupPolicies(_ context.Context, _ *iam.ListAttachedGroupPoliciesInput, _ ...func(*iam.Options)) (*iam.ListAttachedGroupPoliciesOutput, error) {
	return &iam.ListAttachedGroupPoliciesOutput{}, nil
}

func (f *fakeIAMBatch2) ListGroupsForUser(_ context.Context, _ *iam.ListGroupsForUserInput, _ ...func(*iam.Options)) (*iam.ListGroupsForUserOutput, error) {
	return &iam.ListGroupsForUserOutput{}, nil
}

func (f *fakeIAMBatch2) ListEntitiesForPolicy(_ context.Context, _ *iam.ListEntitiesForPolicyInput, _ ...func(*iam.Options)) (*iam.ListEntitiesForPolicyOutput, error) {
	return &iam.ListEntitiesForPolicyOutput{}, nil
}

func (f *fakeIAMBatch2) ListAccountAliases(_ context.Context, _ *iam.ListAccountAliasesInput, _ ...func(*iam.Options)) (*iam.ListAccountAliasesOutput, error) {
	return &iam.ListAccountAliasesOutput{}, nil
}

func (f *fakeIAMBatch2) GetGroup(_ context.Context, _ *iam.GetGroupInput, _ ...func(*iam.Options)) (*iam.GetGroupOutput, error) {
	return &iam.GetGroupOutput{Group: &iamtypes.Group{}}, nil
}

func (f *fakeIAMBatch2) ListGroupPolicies(_ context.Context, _ *iam.ListGroupPoliciesInput, _ ...func(*iam.Options)) (*iam.ListGroupPoliciesOutput, error) {
	return &iam.ListGroupPoliciesOutput{}, nil
}

func (f *fakeIAMBatch2) GetPolicy(_ context.Context, _ *iam.GetPolicyInput, _ ...func(*iam.Options)) (*iam.GetPolicyOutput, error) {
	return &iam.GetPolicyOutput{}, nil
}

func (f *fakeIAMBatch2) GetPolicyVersion(_ context.Context, _ *iam.GetPolicyVersionInput, _ ...func(*iam.Options)) (*iam.GetPolicyVersionOutput, error) {
	return &iam.GetPolicyVersionOutput{}, nil
}

func (f *fakeIAMBatch2) GetRole(_ context.Context, _ *iam.GetRoleInput, _ ...func(*iam.Options)) (*iam.GetRoleOutput, error) {
	return &iam.GetRoleOutput{Role: &iamtypes.Role{}}, nil
}

func (f *fakeIAMBatch2) GetRolePolicy(_ context.Context, _ *iam.GetRolePolicyInput, _ ...func(*iam.Options)) (*iam.GetRolePolicyOutput, error) {
	return &iam.GetRolePolicyOutput{}, nil
}

func (f *fakeIAMBatch2) GetLoginProfile(_ context.Context, _ *iam.GetLoginProfileInput, _ ...func(*iam.Options)) (*iam.GetLoginProfileOutput, error) {
	return &iam.GetLoginProfileOutput{}, nil
}

func (f *fakeIAMBatch2) ListMFADevices(_ context.Context, _ *iam.ListMFADevicesInput, _ ...func(*iam.Options)) (*iam.ListMFADevicesOutput, error) {
	return &iam.ListMFADevicesOutput{}, nil
}

func (f *fakeIAMBatch2) ListAccessKeys(_ context.Context, _ *iam.ListAccessKeysInput, _ ...func(*iam.Options)) (*iam.ListAccessKeysOutput, error) {
	return &iam.ListAccessKeysOutput{}, nil
}

func (f *fakeIAMBatch2) GetInstanceProfile(_ context.Context, input *iam.GetInstanceProfileInput, _ ...func(*iam.Options)) (*iam.GetInstanceProfileOutput, error) {
	if f.getInstanceProfileFn != nil {
		return f.getInstanceProfileFn(input)
	}
	return &iam.GetInstanceProfileOutput{InstanceProfile: &iamtypes.InstanceProfile{}}, nil
}

// newFakeIAMWithInstanceProfile returns a fakeIAMBatch2 whose GetInstanceProfile
// returns an InstanceProfile containing the given roles.
func newFakeIAMWithInstanceProfile(roles []iamtypes.Role) *fakeIAMBatch2 {
	return &fakeIAMBatch2{
		getInstanceProfileFn: func(_ *iam.GetInstanceProfileInput) (*iam.GetInstanceProfileOutput, error) {
			return &iam.GetInstanceProfileOutput{
				InstanceProfile: &iamtypes.InstanceProfile{
					Roles: roles,
				},
			}, nil
		},
	}
}

// ---------------------------------------------------------------------------
// fakeEBBatch2 — implements ElasticBeanstalkAPI for batch-2 tests
// Controllable methods: DescribeEnvironmentResources, DescribeConfigurationSettings,
// DescribeApplicationVersions
// ---------------------------------------------------------------------------

type fakeEBBatch2 struct {
	describeEnvResourcesFn        func(*elasticbeanstalk.DescribeEnvironmentResourcesInput) (*elasticbeanstalk.DescribeEnvironmentResourcesOutput, error)
	describeConfigSettingsFn      func(*elasticbeanstalk.DescribeConfigurationSettingsInput) (*elasticbeanstalk.DescribeConfigurationSettingsOutput, error)
	describeApplicationVersionsFn func(*elasticbeanstalk.DescribeApplicationVersionsInput) (*elasticbeanstalk.DescribeApplicationVersionsOutput, error)
}

func (f *fakeEBBatch2) DescribeEnvironments(_ context.Context, _ *elasticbeanstalk.DescribeEnvironmentsInput, _ ...func(*elasticbeanstalk.Options)) (*elasticbeanstalk.DescribeEnvironmentsOutput, error) {
	return &elasticbeanstalk.DescribeEnvironmentsOutput{}, nil
}

func (f *fakeEBBatch2) DescribeEnvironmentHealth(_ context.Context, _ *elasticbeanstalk.DescribeEnvironmentHealthInput, _ ...func(*elasticbeanstalk.Options)) (*elasticbeanstalk.DescribeEnvironmentHealthOutput, error) {
	return &elasticbeanstalk.DescribeEnvironmentHealthOutput{}, nil
}

func (f *fakeEBBatch2) DescribeConfigurationSettings(_ context.Context, input *elasticbeanstalk.DescribeConfigurationSettingsInput, _ ...func(*elasticbeanstalk.Options)) (*elasticbeanstalk.DescribeConfigurationSettingsOutput, error) {
	if f.describeConfigSettingsFn != nil {
		return f.describeConfigSettingsFn(input)
	}
	return &elasticbeanstalk.DescribeConfigurationSettingsOutput{}, nil
}

func (f *fakeEBBatch2) DescribeEnvironmentResources(_ context.Context, input *elasticbeanstalk.DescribeEnvironmentResourcesInput, _ ...func(*elasticbeanstalk.Options)) (*elasticbeanstalk.DescribeEnvironmentResourcesOutput, error) {
	if f.describeEnvResourcesFn != nil {
		return f.describeEnvResourcesFn(input)
	}
	return &elasticbeanstalk.DescribeEnvironmentResourcesOutput{}, nil
}

func (f *fakeEBBatch2) DescribeApplicationVersions(_ context.Context, input *elasticbeanstalk.DescribeApplicationVersionsInput, _ ...func(*elasticbeanstalk.Options)) (*elasticbeanstalk.DescribeApplicationVersionsOutput, error) {
	if f.describeApplicationVersionsFn != nil {
		return f.describeApplicationVersionsFn(input)
	}
	return &elasticbeanstalk.DescribeApplicationVersionsOutput{}, nil
}

// newFakeEBWithEnvironmentResources returns a fakeEBBatch2 whose
// DescribeEnvironmentResources returns the given EnvironmentResourceDescription.
func newFakeEBWithEnvironmentResources(resources ebtypes.EnvironmentResourceDescription) *fakeEBBatch2 {
	return &fakeEBBatch2{
		describeEnvResourcesFn: func(_ *elasticbeanstalk.DescribeEnvironmentResourcesInput) (*elasticbeanstalk.DescribeEnvironmentResourcesOutput, error) {
			return &elasticbeanstalk.DescribeEnvironmentResourcesOutput{
				EnvironmentResources: &resources,
			}, nil
		},
	}
}

// newFakeEBWithConfigSettings returns a fakeEBBatch2 whose
// DescribeConfigurationSettings returns the given option settings.
func newFakeEBWithConfigSettings(settings []ebtypes.ConfigurationOptionSetting) *fakeEBBatch2 {
	return &fakeEBBatch2{
		describeConfigSettingsFn: func(_ *elasticbeanstalk.DescribeConfigurationSettingsInput) (*elasticbeanstalk.DescribeConfigurationSettingsOutput, error) {
			return &elasticbeanstalk.DescribeConfigurationSettingsOutput{
				ConfigurationSettings: []ebtypes.ConfigurationSettingsDescription{
					{OptionSettings: settings},
				},
			}, nil
		},
	}
}

// newFakeEBWithAppVersions returns a fakeEBBatch2 whose DescribeApplicationVersions
// returns the given application versions.
func newFakeEBWithAppVersions(versions []ebtypes.ApplicationVersionDescription) *fakeEBBatch2 {
	return &fakeEBBatch2{
		describeApplicationVersionsFn: func(_ *elasticbeanstalk.DescribeApplicationVersionsInput) (*elasticbeanstalk.DescribeApplicationVersionsOutput, error) {
			return &elasticbeanstalk.DescribeApplicationVersionsOutput{
				ApplicationVersions: versions,
			}, nil
		},
	}
}

// ---------------------------------------------------------------------------
// fakeELBv2Batch2 — implements ELBv2API for batch-2 tests
// Controllable methods: DescribeLoadBalancers, DescribeListeners
// ---------------------------------------------------------------------------

type fakeELBv2Batch2 struct {
	describeLoadBalancersFn func(*elbv2.DescribeLoadBalancersInput) (*elbv2.DescribeLoadBalancersOutput, error)
	describeListenersFn     func(*elbv2.DescribeListenersInput) (*elbv2.DescribeListenersOutput, error)
}

func (f *fakeELBv2Batch2) DescribeLoadBalancers(_ context.Context, input *elbv2.DescribeLoadBalancersInput, _ ...func(*elbv2.Options)) (*elbv2.DescribeLoadBalancersOutput, error) {
	if f.describeLoadBalancersFn != nil {
		return f.describeLoadBalancersFn(input)
	}
	return &elbv2.DescribeLoadBalancersOutput{}, nil
}

func (f *fakeELBv2Batch2) DescribeTargetGroups(_ context.Context, _ *elbv2.DescribeTargetGroupsInput, _ ...func(*elbv2.Options)) (*elbv2.DescribeTargetGroupsOutput, error) {
	return &elbv2.DescribeTargetGroupsOutput{}, nil
}

func (f *fakeELBv2Batch2) DescribeTargetHealth(_ context.Context, _ *elbv2.DescribeTargetHealthInput, _ ...func(*elbv2.Options)) (*elbv2.DescribeTargetHealthOutput, error) {
	return &elbv2.DescribeTargetHealthOutput{}, nil
}

func (f *fakeELBv2Batch2) DescribeListeners(_ context.Context, input *elbv2.DescribeListenersInput, _ ...func(*elbv2.Options)) (*elbv2.DescribeListenersOutput, error) {
	if f.describeListenersFn != nil {
		return f.describeListenersFn(input)
	}
	return &elbv2.DescribeListenersOutput{}, nil
}

func (f *fakeELBv2Batch2) DescribeRules(_ context.Context, _ *elbv2.DescribeRulesInput, _ ...func(*elbv2.Options)) (*elbv2.DescribeRulesOutput, error) {
	return &elbv2.DescribeRulesOutput{}, nil
}

func (f *fakeELBv2Batch2) DescribeLoadBalancerAttributes(_ context.Context, _ *elbv2.DescribeLoadBalancerAttributesInput, _ ...func(*elbv2.Options)) (*elbv2.DescribeLoadBalancerAttributesOutput, error) {
	return &elbv2.DescribeLoadBalancerAttributesOutput{}, nil
}

// newFakeELBv2WithListeners returns a fakeELBv2Batch2 whose DescribeListeners
// returns the given listeners (DescribeLoadBalancers returns empty).
func newFakeELBv2WithListeners(listeners []elbv2types.Listener) *fakeELBv2Batch2 {
	return &fakeELBv2Batch2{
		describeListenersFn: func(_ *elbv2.DescribeListenersInput) (*elbv2.DescribeListenersOutput, error) {
			return &elbv2.DescribeListenersOutput{Listeners: listeners}, nil
		},
	}
}

// newFakeELBv2WithLBsAndListeners returns a fakeELBv2Batch2 that resolves
// LB names to ARNs via DescribeLoadBalancers and returns matching listeners
// via DescribeListeners (keyed by LoadBalancerArn).
func newFakeELBv2WithLBsAndListeners(lbs []elbv2types.LoadBalancer, listeners []elbv2types.Listener) *fakeELBv2Batch2 {
	return &fakeELBv2Batch2{
		describeLoadBalancersFn: func(_ *elbv2.DescribeLoadBalancersInput) (*elbv2.DescribeLoadBalancersOutput, error) {
			return &elbv2.DescribeLoadBalancersOutput{LoadBalancers: lbs}, nil
		},
		describeListenersFn: func(_ *elbv2.DescribeListenersInput) (*elbv2.DescribeListenersOutput, error) {
			return &elbv2.DescribeListenersOutput{Listeners: listeners}, nil
		},
	}
}

// ---------------------------------------------------------------------------
// fakeSNSBatch2 — implements SNSAPI for batch-2 tests (stub-only, SNS not
// directly needed for these checkers but required to satisfy ServiceClients).
// ---------------------------------------------------------------------------

type fakeSNSBatch2 struct{}

func (f *fakeSNSBatch2) ListSubscriptionsByTopic(_ context.Context, _ *sns.ListSubscriptionsByTopicInput, _ ...func(*sns.Options)) (*sns.ListSubscriptionsByTopicOutput, error) {
	return &sns.ListSubscriptionsByTopicOutput{}, nil
}

func (f *fakeSNSBatch2) GetTopicAttributes(_ context.Context, _ *sns.GetTopicAttributesInput, _ ...func(*sns.Options)) (*sns.GetTopicAttributesOutput, error) {
	return &sns.GetTopicAttributesOutput{Attributes: map[string]string{}}, nil
}

func (f *fakeSNSBatch2) ListTagsForResource(_ context.Context, _ *sns.ListTagsForResourceInput, _ ...func(*sns.Options)) (*sns.ListTagsForResourceOutput, error) {
	return &sns.ListTagsForResourceOutput{}, nil
}

func (f *fakeSNSBatch2) GetSubscriptionAttributes(_ context.Context, _ *sns.GetSubscriptionAttributesInput, _ ...func(*sns.Options)) (*sns.GetSubscriptionAttributesOutput, error) {
	return &sns.GetSubscriptionAttributesOutput{Attributes: map[string]string{}}, nil
}
