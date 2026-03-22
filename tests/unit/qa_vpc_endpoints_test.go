package unit

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func TestQA_VPCEndpoints_FetchSuccess(t *testing.T) {
	mock := &mockEC2DescribeVpcEndpointsClient{
		output: &ec2.DescribeVpcEndpointsOutput{
			VpcEndpoints: []ec2types.VpcEndpoint{
				{
					VpcEndpointId:   aws.String("vpce-0123456789abcdef0"),
					ServiceName:     aws.String("com.amazonaws.us-east-1.s3"),
					VpcEndpointType: ec2types.VpcEndpointTypeGateway,
					State:           ec2types.StateAvailable,
					VpcId:           aws.String("vpc-12345"),
				},
				{
					VpcEndpointId:   aws.String("vpce-abcdef0123456789a"),
					ServiceName:     aws.String("com.amazonaws.us-east-1.execute-api"),
					VpcEndpointType: ec2types.VpcEndpointTypeInterface,
					State:           ec2types.StatePending,
					VpcId:           aws.String("vpc-67890"),
				},
			},
		},
	}

	resources, err := awsclient.FetchVPCEndpoints(context.Background(), mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	r := resources[0]
	if r.ID != "vpce-0123456789abcdef0" {
		t.Errorf("expected ID 'vpce-0123456789abcdef0', got %q", r.ID)
	}
	if r.Name != "com.amazonaws.us-east-1.s3" {
		t.Errorf("expected Name 'com.amazonaws.us-east-1.s3', got %q", r.Name)
	}
	if r.Fields["vpce_id"] != "vpce-0123456789abcdef0" {
		t.Errorf("expected vpce_id 'vpce-0123456789abcdef0', got %q", r.Fields["vpce_id"])
	}
	// VpcEndpointType enum values are capitalized (Gateway, Interface)
	if r.Fields["type"] != "Gateway" {
		t.Errorf("expected type 'Gateway', got %q", r.Fields["type"])
	}
	if r.Fields["vpc_id"] != "vpc-12345" {
		t.Errorf("expected vpc_id 'vpc-12345', got %q", r.Fields["vpc_id"])
	}
	if r.RawStruct == nil {
		t.Error("expected RawStruct to be set")
	}
}

func TestQA_VPCEndpoints_FetchEmpty(t *testing.T) {
	mock := &mockEC2DescribeVpcEndpointsClient{
		output: &ec2.DescribeVpcEndpointsOutput{
			VpcEndpoints: []ec2types.VpcEndpoint{},
		},
	}

	resources, err := awsclient.FetchVPCEndpoints(context.Background(), mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}

func TestQA_VPCEndpoints_FetchError(t *testing.T) {
	mock := &mockEC2DescribeVpcEndpointsClient{
		err: fmt.Errorf("access denied"),
	}

	_, err := awsclient.FetchVPCEndpoints(context.Background(), mock)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestQA_VPCEndpoints_TypeDef(t *testing.T) {
	rt := resource.FindResourceType("vpce")
	if rt == nil {
		t.Fatal("resource type 'vpce' not found")
	}
	if rt.Name != "VPC Endpoints" {
		t.Errorf("expected Name 'VPC Endpoints', got %q", rt.Name)
	}
	expected := []struct {
		key   string
		title string
	}{
		{"vpce_id", "Endpoint ID"},
		{"service_name", "Service Name"},
		{"type", "Type"},
		{"state", "State"},
		{"vpc_id", "VPC ID"},
	}
	if len(rt.Columns) != len(expected) {
		t.Fatalf("expected %d columns, got %d", len(expected), len(rt.Columns))
	}
	for i, want := range expected {
		if rt.Columns[i].Key != want.key {
			t.Errorf("column %d: expected key %q, got %q", i, want.key, rt.Columns[i].Key)
		}
	}
}
