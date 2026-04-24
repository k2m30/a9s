package fakes

import (
	"context"
	"fmt"

	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
)

// ELBFake implements the ELBv2 interfaces against fixture data.
type ELBFake struct {
	fix *fixtures.ELBFixtures
}

// NewELB constructs an ELBFake backed by fixture data.
func NewELB() *ELBFake {
	return &ELBFake{fix: fixtures.NewELBFixtures()}
}

func (f *ELBFake) DescribeLoadBalancers(_ context.Context, _ *elbv2.DescribeLoadBalancersInput, _ ...func(*elbv2.Options)) (*elbv2.DescribeLoadBalancersOutput, error) {
	return &elbv2.DescribeLoadBalancersOutput{LoadBalancers: f.fix.LoadBalancers}, nil
}

func (f *ELBFake) DescribeTargetGroups(_ context.Context, _ *elbv2.DescribeTargetGroupsInput, _ ...func(*elbv2.Options)) (*elbv2.DescribeTargetGroupsOutput, error) {
	return &elbv2.DescribeTargetGroupsOutput{TargetGroups: f.fix.TargetGroups}, nil
}

func (f *ELBFake) DescribeTargetHealth(_ context.Context, input *elbv2.DescribeTargetHealthInput, _ ...func(*elbv2.Options)) (*elbv2.DescribeTargetHealthOutput, error) {
	if input.TargetGroupArn == nil {
		return &elbv2.DescribeTargetHealthOutput{}, nil
	}
	// Only error if the target group itself does not exist. A TG with no
	// registered targets legitimately returns an empty health list on the
	// real AWS API, not an error.
	if !f.hasTargetGroup(*input.TargetGroupArn) {
		return nil, fmt.Errorf("target group %q not found", *input.TargetGroupArn)
	}
	health := f.fix.TargetHealth[*input.TargetGroupArn]
	return &elbv2.DescribeTargetHealthOutput{TargetHealthDescriptions: health}, nil
}

func (f *ELBFake) hasTargetGroup(arn string) bool {
	for _, tg := range f.fix.TargetGroups {
		if tg.TargetGroupArn != nil && *tg.TargetGroupArn == arn {
			return true
		}
	}
	return false
}

func (f *ELBFake) DescribeListeners(_ context.Context, input *elbv2.DescribeListenersInput, _ ...func(*elbv2.Options)) (*elbv2.DescribeListenersOutput, error) {
	if input.LoadBalancerArn == nil {
		// return all listeners
		var all []elbv2types.Listener
		for _, ls := range f.fix.Listeners {
			all = append(all, ls...)
		}
		return &elbv2.DescribeListenersOutput{Listeners: all}, nil
	}
	listeners, ok := f.fix.Listeners[*input.LoadBalancerArn]
	if !ok {
		return &elbv2.DescribeListenersOutput{}, nil
	}
	return &elbv2.DescribeListenersOutput{Listeners: listeners}, nil
}

func (f *ELBFake) DescribeRules(_ context.Context, input *elbv2.DescribeRulesInput, _ ...func(*elbv2.Options)) (*elbv2.DescribeRulesOutput, error) {
	if input.ListenerArn == nil {
		return &elbv2.DescribeRulesOutput{}, nil
	}
	rules, ok := f.fix.Rules[*input.ListenerArn]
	if !ok {
		return nil, fmt.Errorf("listener %q not found", *input.ListenerArn)
	}
	return &elbv2.DescribeRulesOutput{Rules: rules}, nil
}

// DescribeLoadBalancerAttributes is a no-op stub for demo mode.
// Wave 2 enrichment is skipped in demo mode; this satisfies the ELBv2API interface.
func (f *ELBFake) DescribeLoadBalancerAttributes(_ context.Context, _ *elbv2.DescribeLoadBalancerAttributesInput, _ ...func(*elbv2.Options)) (*elbv2.DescribeLoadBalancerAttributesOutput, error) {
	return &elbv2.DescribeLoadBalancerAttributesOutput{}, nil
}
