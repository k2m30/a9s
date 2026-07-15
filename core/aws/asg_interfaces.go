package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
)

// ASGDescribeAutoScalingGroupsAPI defines the interface for the AutoScaling DescribeAutoScalingGroups operation.
type ASGDescribeAutoScalingGroupsAPI interface {
	DescribeAutoScalingGroups(ctx context.Context, params *autoscaling.DescribeAutoScalingGroupsInput, optFns ...func(*autoscaling.Options)) (*autoscaling.DescribeAutoScalingGroupsOutput, error)
}

// ASGDescribeScalingActivitiesAPI defines the interface for the AutoScaling DescribeScalingActivities operation.
type ASGDescribeScalingActivitiesAPI interface {
	DescribeScalingActivities(ctx context.Context, params *autoscaling.DescribeScalingActivitiesInput, optFns ...func(*autoscaling.Options)) (*autoscaling.DescribeScalingActivitiesOutput, error)
}

// ASGDescribeLaunchConfigurationsAPI for asg→ami, asg→role, asg→sg via LaunchConfiguration.
type ASGDescribeLaunchConfigurationsAPI interface {
	DescribeLaunchConfigurations(ctx context.Context, params *autoscaling.DescribeLaunchConfigurationsInput, optFns ...func(*autoscaling.Options)) (*autoscaling.DescribeLaunchConfigurationsOutput, error)
}

// ASGDescribeNotificationConfigurationsAPI for asg→sns via NotificationConfigurations.
type ASGDescribeNotificationConfigurationsAPI interface {
	DescribeNotificationConfigurations(ctx context.Context, params *autoscaling.DescribeNotificationConfigurationsInput, optFns ...func(*autoscaling.Options)) (*autoscaling.DescribeNotificationConfigurationsOutput, error)
}

// ASGDescribeLifecycleHooksAPI for asg→sns via LifecycleHooks.NotificationTargetARN.
type ASGDescribeLifecycleHooksAPI interface {
	DescribeLifecycleHooks(ctx context.Context, params *autoscaling.DescribeLifecycleHooksInput, optFns ...func(*autoscaling.Options)) (*autoscaling.DescribeLifecycleHooksOutput, error)
}

// ASGAPI is the aggregate interface covering all AutoScaling operations used by a9s fetchers.
// *autoscaling.Client structurally satisfies this interface.
type ASGAPI interface {
	ASGDescribeAutoScalingGroupsAPI
	ASGDescribeScalingActivitiesAPI
	ASGDescribeLaunchConfigurationsAPI
	ASGDescribeNotificationConfigurationsAPI
	ASGDescribeLifecycleHooksAPI
}
