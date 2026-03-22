package unit

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
)

// ---------------------------------------------------------------------------
// NAT Gateway fetcher tests
// ---------------------------------------------------------------------------

func TestFetchNatGateways_ParsesMultipleNatGateways(t *testing.T) {
	createTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	mock := &mockEC2DescribeNatGatewaysClient{
		output: &ec2.DescribeNatGatewaysOutput{
			NatGateways: []ec2types.NatGateway{
				{
					NatGatewayId:     aws.String("nat-0001"),
					VpcId:            aws.String("vpc-aaa"),
					SubnetId:         aws.String("subnet-0001"),
					State:            ec2types.NatGatewayStateAvailable,
					ConnectivityType: ec2types.ConnectivityTypePublic,
					CreateTime:       &createTime,
					NatGatewayAddresses: []ec2types.NatGatewayAddress{
						{PublicIp: aws.String("1.2.3.4")},
					},
					Tags: []ec2types.Tag{
						{Key: aws.String("Name"), Value: aws.String("main-nat")},
						{Key: aws.String("Env"), Value: aws.String("prod")},
					},
				},
				{
					NatGatewayId:        aws.String("nat-0002"),
					VpcId:               aws.String("vpc-bbb"),
					SubnetId:            aws.String("subnet-0002"),
					State:               ec2types.NatGatewayStatePending,
					ConnectivityType:    ec2types.ConnectivityTypePublic,
					NatGatewayAddresses: []ec2types.NatGatewayAddress{},
					Tags:                []ec2types.Tag{},
				},
			},
		},
	}

	resources, err := awsclient.FetchNatGateways(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	// Verify first NAT gateway
	r0 := resources[0]
	if r0.ID != "nat-0001" {
		t.Errorf("resource[0].ID: expected %q, got %q", "nat-0001", r0.ID)
	}
	if r0.Name != "main-nat" {
		t.Errorf("resource[0].Name: expected %q, got %q", "main-nat", r0.Name)
	}
	if r0.Status != "available" {
		t.Errorf("resource[0].Status: expected %q, got %q", "available", r0.Status)
	}

	// Verify Fields on all resources
	requiredFields := []string{"nat_gateway_id", "name", "vpc_id", "subnet_id", "state", "public_ip"}
	for i, r := range resources {
		for _, key := range requiredFields {
			if _, ok := r.Fields[key]; !ok {
				t.Errorf("resource[%d].Fields missing key %q", i, key)
			}
		}
	}

	// Verify specific field values on first NAT gateway
	if r0.Fields["nat_gateway_id"] != "nat-0001" {
		t.Errorf("resource[0].Fields[\"nat_gateway_id\"]: expected %q, got %q", "nat-0001", r0.Fields["nat_gateway_id"])
	}
	if r0.Fields["name"] != "main-nat" {
		t.Errorf("resource[0].Fields[\"name\"]: expected %q, got %q", "main-nat", r0.Fields["name"])
	}
	if r0.Fields["vpc_id"] != "vpc-aaa" {
		t.Errorf("resource[0].Fields[\"vpc_id\"]: expected %q, got %q", "vpc-aaa", r0.Fields["vpc_id"])
	}
	if r0.Fields["subnet_id"] != "subnet-0001" {
		t.Errorf("resource[0].Fields[\"subnet_id\"]: expected %q, got %q", "subnet-0001", r0.Fields["subnet_id"])
	}
	if r0.Fields["state"] != "available" {
		t.Errorf("resource[0].Fields[\"state\"]: expected %q, got %q", "available", r0.Fields["state"])
	}
	if r0.Fields["public_ip"] != "1.2.3.4" {
		t.Errorf("resource[0].Fields[\"public_ip\"]: expected %q, got %q", "1.2.3.4", r0.Fields["public_ip"])
	}

	// Verify second NAT gateway (no Name tag, pending state, no public IP)
	r1 := resources[1]
	if r1.ID != "nat-0002" {
		t.Errorf("resource[1].ID: expected %q, got %q", "nat-0002", r1.ID)
	}
	if r1.Name != "" {
		t.Errorf("resource[1].Name: expected empty string, got %q", r1.Name)
	}
	if r1.Status != "pending" {
		t.Errorf("resource[1].Status: expected %q, got %q", "pending", r1.Status)
	}
	if r1.Fields["public_ip"] != "" {
		t.Errorf("resource[1].Fields[\"public_ip\"]: expected empty string, got %q", r1.Fields["public_ip"])
	}
}

func TestFetchNatGateways_ErrorResponse(t *testing.T) {
	mock := &mockEC2DescribeNatGatewaysClient{
		output: nil,
		err:    fmt.Errorf("AWS API error: access denied"),
	}

	resources, err := awsclient.FetchNatGateways(context.Background(), mock)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if resources != nil {
		t.Errorf("expected nil resources on error, got %d resources", len(resources))
	}
}

func TestFetchNatGateways_EmptyResponse(t *testing.T) {
	mock := &mockEC2DescribeNatGatewaysClient{
		output: &ec2.DescribeNatGatewaysOutput{
			NatGateways: []ec2types.NatGateway{},
		},
	}

	resources, err := awsclient.FetchNatGateways(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}
