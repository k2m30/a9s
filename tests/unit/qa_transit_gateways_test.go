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

func TestQA_TransitGateways_FetchSuccess(t *testing.T) {
	mock := &mockEC2DescribeTransitGatewaysClient{
		output: &ec2.DescribeTransitGatewaysOutput{
			TransitGateways: []ec2types.TransitGateway{
				{
					TransitGatewayId:  aws.String("tgw-0123456789abcdef0"),
					State:             ec2types.TransitGatewayStateAvailable,
					OwnerId:           aws.String("123456789012"),
					Description:       aws.String("Main transit gateway"),
					Tags: []ec2types.Tag{
						{Key: aws.String("Name"), Value: aws.String("main-tgw")},
					},
				},
				{
					TransitGatewayId: aws.String("tgw-abcdef0123456789a"),
					State:            ec2types.TransitGatewayStatePending,
					OwnerId:          aws.String("123456789012"),
					Description:      aws.String("Secondary TGW"),
				},
			},
		},
	}

	resources, err := awsclient.FetchTransitGateways(context.Background(), mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	r := resources[0]
	if r.ID != "tgw-0123456789abcdef0" {
		t.Errorf("expected ID 'tgw-0123456789abcdef0', got %q", r.ID)
	}
	if r.Name != "main-tgw" {
		t.Errorf("expected Name 'main-tgw', got %q", r.Name)
	}
	if r.Status != "available" {
		t.Errorf("expected Status 'available', got %q", r.Status)
	}
	if r.Fields["tgw_id"] != "tgw-0123456789abcdef0" {
		t.Errorf("expected tgw_id 'tgw-0123456789abcdef0', got %q", r.Fields["tgw_id"])
	}
	if r.Fields["name"] != "main-tgw" {
		t.Errorf("expected name 'main-tgw', got %q", r.Fields["name"])
	}
	if r.Fields["description"] != "Main transit gateway" {
		t.Errorf("expected description 'Main transit gateway', got %q", r.Fields["description"])
	}

	// Second TGW has no Name tag
	r2 := resources[1]
	if r2.Name != "" {
		t.Errorf("expected empty Name for second TGW, got %q", r2.Name)
	}
	if r2.RawStruct == nil {
		t.Error("expected RawStruct to be set")
	}
}

func TestQA_TransitGateways_FetchEmpty(t *testing.T) {
	mock := &mockEC2DescribeTransitGatewaysClient{
		output: &ec2.DescribeTransitGatewaysOutput{
			TransitGateways: []ec2types.TransitGateway{},
		},
	}

	resources, err := awsclient.FetchTransitGateways(context.Background(), mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}

func TestQA_TransitGateways_FetchError(t *testing.T) {
	mock := &mockEC2DescribeTransitGatewaysClient{
		err: fmt.Errorf("access denied"),
	}

	_, err := awsclient.FetchTransitGateways(context.Background(), mock)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestQA_TransitGateways_TypeDef(t *testing.T) {
	rt := resource.FindResourceType("tgw")
	if rt == nil {
		t.Fatal("resource type 'tgw' not found")
	}
	if rt.Name != "Transit Gateways" {
		t.Errorf("expected Name 'Transit Gateways', got %q", rt.Name)
	}
	expected := []struct {
		key   string
		title string
	}{
		{"tgw_id", "TGW ID"},
		{"name", "Name"},
		{"state", "State"},
		{"owner_id", "Owner"},
		{"description", "Description"},
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
