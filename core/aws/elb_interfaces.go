package aws

import (
	"context"

	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
)

// ELBv2DescribeLoadBalancersAPI defines the interface for the ELBv2 DescribeLoadBalancers operation.
type ELBv2DescribeLoadBalancersAPI interface {
	DescribeLoadBalancers(ctx context.Context, params *elbv2.DescribeLoadBalancersInput, optFns ...func(*elbv2.Options)) (*elbv2.DescribeLoadBalancersOutput, error)
}

// ELBv2DescribeTargetGroupsAPI defines the interface for the ELBv2 DescribeTargetGroups operation.
type ELBv2DescribeTargetGroupsAPI interface {
	DescribeTargetGroups(ctx context.Context, params *elbv2.DescribeTargetGroupsInput, optFns ...func(*elbv2.Options)) (*elbv2.DescribeTargetGroupsOutput, error)
}

// ELBv2DescribeTargetHealthAPI defines the interface for the ELBv2 DescribeTargetHealth operation.
type ELBv2DescribeTargetHealthAPI interface {
	DescribeTargetHealth(ctx context.Context, params *elbv2.DescribeTargetHealthInput, optFns ...func(*elbv2.Options)) (*elbv2.DescribeTargetHealthOutput, error)
}

// ELBv2DescribeListenersAPI defines the interface for the ELBv2 DescribeListeners operation.
type ELBv2DescribeListenersAPI interface {
	DescribeListeners(ctx context.Context, params *elbv2.DescribeListenersInput, optFns ...func(*elbv2.Options)) (*elbv2.DescribeListenersOutput, error)
}

// ELBv2DescribeRulesAPI defines the interface for the ELBv2 DescribeRules operation.
type ELBv2DescribeRulesAPI interface {
	DescribeRules(ctx context.Context, params *elbv2.DescribeRulesInput, optFns ...func(*elbv2.Options)) (*elbv2.DescribeRulesOutput, error)
}

// ELBv2DescribeLoadBalancerAttributesAPI defines the interface for the ELBv2
// DescribeLoadBalancerAttributes operation.
// Used by EnrichELBAttributes (Wave 2 enrichment).
type ELBv2DescribeLoadBalancerAttributesAPI interface {
	DescribeLoadBalancerAttributes(ctx context.Context, params *elbv2.DescribeLoadBalancerAttributesInput, optFns ...func(*elbv2.Options)) (*elbv2.DescribeLoadBalancerAttributesOutput, error)
}

// ELBv2DescribeTagsAPI lists tags on one or more ELB/TG/listener/rule ARNs.
type ELBv2DescribeTagsAPI interface {
	DescribeTags(ctx context.Context, params *elbv2.DescribeTagsInput, optFns ...func(*elbv2.Options)) (*elbv2.DescribeTagsOutput, error)
}

// ELBv2API is the aggregate interface covering all ELBv2 operations used by a9s fetchers.
// *elbv2.Client structurally satisfies this interface.
type ELBv2API interface {
	ELBv2DescribeLoadBalancersAPI
	ELBv2DescribeTargetGroupsAPI
	ELBv2DescribeTargetHealthAPI
	ELBv2DescribeListenersAPI
	ELBv2DescribeRulesAPI
	ELBv2DescribeLoadBalancerAttributesAPI // Wave 2 enrichment
}
