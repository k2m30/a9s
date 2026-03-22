package unit

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
)

// ---------------------------------------------------------------------------
// T-ELB01 - Test Load Balancers response parsing
// ---------------------------------------------------------------------------

func TestFetchLoadBalancers_ParsesMultipleLoadBalancers(t *testing.T) {
	createdTime := time.Now()

	mock := &mockELBv2DescribeLoadBalancersClient{
		output: &elbv2.DescribeLoadBalancersOutput{
			LoadBalancers: []elbv2types.LoadBalancer{
				{
					LoadBalancerName: aws.String("prod-alb"),
					DNSName:          aws.String("prod-alb-123456789.us-east-1.elb.amazonaws.com"),
					Type:             elbv2types.LoadBalancerTypeEnumApplication,
					Scheme:           elbv2types.LoadBalancerSchemeEnumInternetFacing,
					State: &elbv2types.LoadBalancerState{
						Code: elbv2types.LoadBalancerStateEnumActive,
					},
					VpcId:               aws.String("vpc-abc123"),
					LoadBalancerArn:     aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/prod-alb/abc123"),
					CanonicalHostedZoneId: aws.String("Z35SXDOTRQ7X7K"),
					CreatedTime:         &createdTime,
					IpAddressType:       elbv2types.IpAddressTypeIpv4,
				},
				{
					LoadBalancerName: aws.String("internal-nlb"),
					DNSName:          aws.String("internal-nlb-987654321.us-east-1.elb.amazonaws.com"),
					Type:             elbv2types.LoadBalancerTypeEnumNetwork,
					Scheme:           elbv2types.LoadBalancerSchemeEnumInternal,
					State: &elbv2types.LoadBalancerState{
						Code: elbv2types.LoadBalancerStateEnumActive,
					},
					VpcId:           aws.String("vpc-def456"),
					LoadBalancerArn: aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/net/internal-nlb/def456"),
					IpAddressType:   elbv2types.IpAddressTypeIpv4,
				},
			},
		},
	}

	resources, err := awsclient.FetchLoadBalancers(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	// Verify required fields exist
	requiredFields := []string{"name", "dns_name", "type", "scheme", "state", "vpc_id"}
	for i, r := range resources {
		for _, key := range requiredFields {
			if _, ok := r.Fields[key]; !ok {
				t.Errorf("resource[%d].Fields missing key %q", i, key)
			}
		}
	}

	// Verify first load balancer
	r0 := resources[0]
	if r0.ID != "prod-alb" {
		t.Errorf("resource[0].ID: expected %q, got %q", "prod-alb", r0.ID)
	}
	if r0.Name != "prod-alb" {
		t.Errorf("resource[0].Name: expected %q, got %q", "prod-alb", r0.Name)
	}
	if r0.Status != "active" {
		t.Errorf("resource[0].Status: expected %q, got %q", "active", r0.Status)
	}
	if r0.Fields["name"] != "prod-alb" {
		t.Errorf("resource[0].Fields[\"name\"]: expected %q, got %q", "prod-alb", r0.Fields["name"])
	}
	if r0.Fields["dns_name"] != "prod-alb-123456789.us-east-1.elb.amazonaws.com" {
		t.Errorf("resource[0].Fields[\"dns_name\"]: expected %q, got %q",
			"prod-alb-123456789.us-east-1.elb.amazonaws.com", r0.Fields["dns_name"])
	}
	if r0.Fields["type"] != "application" {
		t.Errorf("resource[0].Fields[\"type\"]: expected %q, got %q", "application", r0.Fields["type"])
	}
	if r0.Fields["scheme"] != "internet-facing" {
		t.Errorf("resource[0].Fields[\"scheme\"]: expected %q, got %q", "internet-facing", r0.Fields["scheme"])
	}
	if r0.Fields["state"] != "active" {
		t.Errorf("resource[0].Fields[\"state\"]: expected %q, got %q", "active", r0.Fields["state"])
	}
	if r0.Fields["vpc_id"] != "vpc-abc123" {
		t.Errorf("resource[0].Fields[\"vpc_id\"]: expected %q, got %q", "vpc-abc123", r0.Fields["vpc_id"])
	}

	// Verify second load balancer
	r1 := resources[1]
	if r1.ID != "internal-nlb" {
		t.Errorf("resource[1].ID: expected %q, got %q", "internal-nlb", r1.ID)
	}
	if r1.Fields["type"] != "network" {
		t.Errorf("resource[1].Fields[\"type\"]: expected %q, got %q", "network", r1.Fields["type"])
	}
	if r1.Fields["scheme"] != "internal" {
		t.Errorf("resource[1].Fields[\"scheme\"]: expected %q, got %q", "internal", r1.Fields["scheme"])
	}
	if r1.Fields["vpc_id"] != "vpc-def456" {
		t.Errorf("resource[1].Fields[\"vpc_id\"]: expected %q, got %q", "vpc-def456", r1.Fields["vpc_id"])
	}
}

func TestFetchLoadBalancers_ErrorResponse(t *testing.T) {
	mock := &mockELBv2DescribeLoadBalancersClient{
		output: nil,
		err:    fmt.Errorf("AWS API error: access denied"),
	}

	resources, err := awsclient.FetchLoadBalancers(context.Background(), mock)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if resources != nil {
		t.Errorf("expected nil resources on error, got %d resources", len(resources))
	}
}

func TestFetchLoadBalancers_EmptyResponse(t *testing.T) {
	mock := &mockELBv2DescribeLoadBalancersClient{
		output: &elbv2.DescribeLoadBalancersOutput{
			LoadBalancers: []elbv2types.LoadBalancer{},
		},
	}

	resources, err := awsclient.FetchLoadBalancers(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}
