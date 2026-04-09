// Package fakes provides in-process fake implementations of AWS service
// interfaces for use in demo mode and tests.
package fakes

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"

	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
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
	return &autoscaling.DescribeScalingActivitiesOutput{Activities: f.fix.Activities[asgName]}, nil
}
