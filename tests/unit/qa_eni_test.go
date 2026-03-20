package unit

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	awsclient "github.com/k2m30/a9s/internal/aws"
	"github.com/k2m30/a9s/internal/resource"
)

func TestQA_ENI_FetchSuccess(t *testing.T) {
	mock := &mockEC2DescribeNetworkInterfacesClient{
		output: &ec2.DescribeNetworkInterfacesOutput{
			NetworkInterfaces: []ec2types.NetworkInterface{
				{
					NetworkInterfaceId: aws.String("eni-0123456789abcdef0"),
					Status:             ec2types.NetworkInterfaceStatusInUse,
					InterfaceType:      ec2types.NetworkInterfaceTypeInterface,
					VpcId:              aws.String("vpc-12345"),
					PrivateIpAddress:   aws.String("10.0.1.50"),
					TagSet: []ec2types.Tag{
						{Key: aws.String("Name"), Value: aws.String("my-eni")},
					},
				},
				{
					NetworkInterfaceId: aws.String("eni-abcdef0123456789a"),
					Status:             ec2types.NetworkInterfaceStatusAvailable,
					InterfaceType:      ec2types.NetworkInterfaceTypeNatGateway,
					VpcId:              aws.String("vpc-67890"),
					PrivateIpAddress:   aws.String("10.0.2.100"),
				},
			},
		},
	}

	resources, err := awsclient.FetchNetworkInterfaces(context.Background(), mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	r := resources[0]
	if r.ID != "eni-0123456789abcdef0" {
		t.Errorf("expected ID 'eni-0123456789abcdef0', got %q", r.ID)
	}
	if r.Name != "my-eni" {
		t.Errorf("expected Name 'my-eni', got %q", r.Name)
	}
	if r.Status != "in-use" {
		t.Errorf("expected Status 'in-use', got %q", r.Status)
	}
	if r.Fields["eni_id"] != "eni-0123456789abcdef0" {
		t.Errorf("expected eni_id, got %q", r.Fields["eni_id"])
	}
	if r.Fields["name"] != "my-eni" {
		t.Errorf("expected name 'my-eni', got %q", r.Fields["name"])
	}
	if r.Fields["type"] != "interface" {
		t.Errorf("expected type 'interface', got %q", r.Fields["type"])
	}
	if r.Fields["vpc_id"] != "vpc-12345" {
		t.Errorf("expected vpc_id 'vpc-12345', got %q", r.Fields["vpc_id"])
	}
	if r.Fields["private_ip"] != "10.0.1.50" {
		t.Errorf("expected private_ip '10.0.1.50', got %q", r.Fields["private_ip"])
	}

	r2 := resources[1]
	if r2.Name != "" {
		t.Errorf("expected empty Name for second ENI, got %q", r2.Name)
	}
	if r2.RawStruct == nil {
		t.Error("expected RawStruct to be set")
	}
}

func TestQA_ENI_FetchEmpty(t *testing.T) {
	mock := &mockEC2DescribeNetworkInterfacesClient{
		output: &ec2.DescribeNetworkInterfacesOutput{
			NetworkInterfaces: []ec2types.NetworkInterface{},
		},
	}

	resources, err := awsclient.FetchNetworkInterfaces(context.Background(), mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}

func TestQA_ENI_FetchError(t *testing.T) {
	mock := &mockEC2DescribeNetworkInterfacesClient{
		err: fmt.Errorf("access denied"),
	}

	_, err := awsclient.FetchNetworkInterfaces(context.Background(), mock)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestQA_ENI_TypeDef(t *testing.T) {
	rt := resource.FindResourceType("eni")
	if rt == nil {
		t.Fatal("resource type 'eni' not found")
	}
	if rt.Name != "Network Interfaces" {
		t.Errorf("expected Name 'Network Interfaces', got %q", rt.Name)
	}
	expected := []struct {
		key   string
		title string
	}{
		{"eni_id", "ENI ID"},
		{"name", "Name"},
		{"status", "Status"},
		{"type", "Type"},
		{"vpc_id", "VPC ID"},
		{"private_ip", "Private IP"},
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
