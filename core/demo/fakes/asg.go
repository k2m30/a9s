// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// Package fakes provides in-process fake implementations of AWS service
// interfaces for use in demo mode and tests.
package fakes

import (
	"context"
	"slices"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	"github.com/aws/smithy-go"

	"github.com/k2m30/a9s/v3/core/demo/fixtures"
)

// ASGFake implements aws.ASGAPI against fixture data loaded at construction time.
type ASGFake struct {
	fix *fixtures.ASGFixtures
}

// NewASG constructs an ASGFake backed by fixture data from the fixtures package.
func NewASG() *ASGFake {
	return &ASGFake{fix: fixtures.NewASGFixtures()}
}

func (f *ASGFake) DescribeAutoScalingGroups(_ context.Context, input *autoscaling.DescribeAutoScalingGroupsInput, _ ...func(*autoscaling.Options)) (*autoscaling.DescribeAutoScalingGroupsOutput, error) {
	if len(input.AutoScalingGroupNames) == 0 {
		return &autoscaling.DescribeAutoScalingGroupsOutput{AutoScalingGroups: f.fix.AutoScalingGroups}, nil
	}
	wanted := toSet(input.AutoScalingGroupNames)
	var result []asgtypes.AutoScalingGroup
	for _, g := range f.fix.AutoScalingGroups {
		if wanted[aws.ToString(g.AutoScalingGroupName)] {
			result = append(result, g)
		}
	}
	return &autoscaling.DescribeAutoScalingGroupsOutput{AutoScalingGroups: result}, nil
}

func (f *ASGFake) DescribeScalingActivities(_ context.Context, input *autoscaling.DescribeScalingActivitiesInput, _ ...func(*autoscaling.Options)) (*autoscaling.DescribeScalingActivitiesOutput, error) {
	asgName := aws.ToString(input.AutoScalingGroupName)
	if asgName == "" {
		var all []asgtypes.Activity
		for _, acts := range f.fix.Activities {
			all = append(all, acts...)
		}
		return &autoscaling.DescribeScalingActivitiesOutput{Activities: all}, nil
	}
	if !f.hasGroup(asgName) {
		// Auto Scaling is a query-protocol API with no modeled not-found
		// shape: a group that does not exist comes back as ValidationError.
		return nil, &smithy.GenericAPIError{
			Code:    "ValidationError",
			Message: "AutoScalingGroup name not found - AutoScalingGroup " + asgName + " not found",
			Fault:   smithy.FaultClient,
		}
	}
	return &autoscaling.DescribeScalingActivitiesOutput{Activities: f.fix.Activities[asgName]}, nil
}

// DescribeLaunchConfigurations returns launch configurations by name from fixture data.
// If no names are requested, all known LCs are returned.
func (f *ASGFake) DescribeLaunchConfigurations(_ context.Context, input *autoscaling.DescribeLaunchConfigurationsInput, _ ...func(*autoscaling.Options)) (*autoscaling.DescribeLaunchConfigurationsOutput, error) {
	var result []asgtypes.LaunchConfiguration
	if len(input.LaunchConfigurationNames) == 0 {
		for _, lc := range f.fix.LaunchConfigurations {
			result = append(result, lc)
		}
		return &autoscaling.DescribeLaunchConfigurationsOutput{LaunchConfigurations: result}, nil
	}
	for _, name := range input.LaunchConfigurationNames {
		if lc, ok := f.fix.LaunchConfigurations[name]; ok {
			result = append(result, lc)
		}
	}
	return &autoscaling.DescribeLaunchConfigurationsOutput{LaunchConfigurations: result}, nil
}

// DescribeNotificationConfigurations returns notification configurations by
// ASG name from fixture data. Backs the asg:sns related-panel pivot.
func (f *ASGFake) DescribeNotificationConfigurations(_ context.Context, input *autoscaling.DescribeNotificationConfigurationsInput, _ ...func(*autoscaling.Options)) (*autoscaling.DescribeNotificationConfigurationsOutput, error) {
	var result []asgtypes.NotificationConfiguration
	for _, name := range input.AutoScalingGroupNames {
		result = append(result, f.fix.NotificationConfigurations[name]...)
	}
	return &autoscaling.DescribeNotificationConfigurationsOutput{NotificationConfigurations: result}, nil
}

// DescribeLifecycleHooks returns lifecycle hooks for the given ASG name from
// fixture data. Backs the asg:sns related-panel pivot (SNS-ARN reverse scan).
func (f *ASGFake) DescribeLifecycleHooks(_ context.Context, input *autoscaling.DescribeLifecycleHooksInput, _ ...func(*autoscaling.Options)) (*autoscaling.DescribeLifecycleHooksOutput, error) {
	name := aws.ToString(input.AutoScalingGroupName)
	return &autoscaling.DescribeLifecycleHooksOutput{LifecycleHooks: f.fix.LifecycleHooks[name]}, nil
}

// hasGroup reports whether the fixtures register this Auto Scaling group.
func (f *ASGFake) hasGroup(name string) bool {
	return slices.ContainsFunc(f.fix.AutoScalingGroups, func(g asgtypes.AutoScalingGroup) bool {
		return aws.ToString(g.AutoScalingGroupName) == name
	})
}
