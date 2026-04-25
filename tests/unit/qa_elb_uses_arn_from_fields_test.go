package unit

// qa_elb_uses_arn_from_fields_test.go — Regression: EnrichELBAttributes must
// call DescribeLoadBalancerAttributes with the load balancer ARN from
// r.Fields["load_balancer_arn"], NOT the bare name in r.ID.
//
// Same shape as the tg and sfn bugs: the elb fetcher (elb.go:111) sets
// `ID: lbName` and stores the ARN in Fields["load_balancer_arn"]. The
// enricher passing r.ID directly to LoadBalancerArn produces a ValidationError
// against real AWS exactly like tg did.

import (
	"context"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	smithy "github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// strictELBv2Fake mirrors AWS: rejects DescribeLoadBalancerAttributes when
// LoadBalancerArn is not a valid ARN.
type strictELBv2Fake struct {
	awsclient.ELBv2API
	calledWith string
}

func (f *strictELBv2Fake) DescribeLoadBalancerAttributes(
	_ context.Context,
	input *elbv2.DescribeLoadBalancerAttributesInput,
	_ ...func(*elbv2.Options),
) (*elbv2.DescribeLoadBalancerAttributesOutput, error) {
	got := aws.ToString(input.LoadBalancerArn)
	f.calledWith = got
	if !strings.HasPrefix(got, "arn:aws:") {
		return nil, &smithy.GenericAPIError{
			Code:    "ValidationError",
			Message: "'" + got + "' is not a valid load balancer ARN",
		}
	}
	return &elbv2.DescribeLoadBalancerAttributesOutput{}, nil
}

// TestEnrichELBAttributes_UsesARNFromFields verifies the enricher passes
// r.Fields["load_balancer_arn"] (the full LB ARN) to DescribeLoadBalancerAttributes,
// not r.ID (the bare LB name set by the elb fetcher).
func TestEnrichELBAttributes_UsesARNFromFields(t *testing.T) {
	const lbName = "prod-app-alb"
	const lbARN = "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/prod-app-alb/abc123def456"

	fake := &strictELBv2Fake{}
	clients := &awsclient.ServiceClients{ELBv2: fake}
	resources := []resource.Resource{{
		ID:     lbName,
		Name:   lbName,
		Fields: map[string]string{"load_balancer_arn": lbARN},
	}}

	_, err := awsclient.EnrichELBAttributes(context.Background(), clients, resources)
	if err != nil && strings.Contains(err.Error(), "ValidationError") {
		t.Fatalf("enricher passed bare name to AWS instead of ARN; got: %v", err)
	}
	if fake.calledWith != lbARN {
		t.Errorf("DescribeLoadBalancerAttributes was called with %q, want %q (the ARN from Fields[\"load_balancer_arn\"])",
			fake.calledWith, lbARN)
	}
}
