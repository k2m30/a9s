package unit

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	awsclient "github.com/k2m30/a9s/internal/aws"
)

// ---------------------------------------------------------------------------
// Internet Gateway fetcher tests
// ---------------------------------------------------------------------------

func TestFetchInternetGateways_ParsesMultipleIGWs(t *testing.T) {
	mock := &mockEC2DescribeInternetGatewaysClient{
		output: &ec2.DescribeInternetGatewaysOutput{
			InternetGateways: []ec2types.InternetGateway{
				{
					InternetGatewayId: aws.String("igw-0001"),
					OwnerId:           aws.String("123456789012"),
					Attachments: []ec2types.InternetGatewayAttachment{
						{
							VpcId: aws.String("vpc-aaa"),
							State: ec2types.AttachmentStatusAttached,
						},
					},
					Tags: []ec2types.Tag{
						{Key: aws.String("Name"), Value: aws.String("main-igw")},
						{Key: aws.String("Env"), Value: aws.String("prod")},
					},
				},
				{
					InternetGatewayId: aws.String("igw-0002"),
					OwnerId:           aws.String("123456789012"),
					Attachments: []ec2types.InternetGatewayAttachment{
						{
							VpcId: aws.String("vpc-bbb"),
							State: ec2types.AttachmentStatusAttached,
						},
					},
					Tags: []ec2types.Tag{},
				},
			},
		},
	}

	resources, err := awsclient.FetchInternetGateways(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	// Verify first IGW
	r0 := resources[0]
	if r0.ID != "igw-0001" {
		t.Errorf("resource[0].ID: expected %q, got %q", "igw-0001", r0.ID)
	}
	if r0.Name != "main-igw" {
		t.Errorf("resource[0].Name: expected %q, got %q", "main-igw", r0.Name)
	}
	if r0.Status != "attached" {
		t.Errorf("resource[0].Status: expected %q, got %q", "attached", r0.Status)
	}

	// Verify Fields on all resources
	requiredFields := []string{"igw_id", "name", "vpc_id", "state"}
	for i, r := range resources {
		for _, key := range requiredFields {
			if _, ok := r.Fields[key]; !ok {
				t.Errorf("resource[%d].Fields missing key %q", i, key)
			}
		}
	}

	// Verify specific field values on first IGW
	if r0.Fields["igw_id"] != "igw-0001" {
		t.Errorf("resource[0].Fields[\"igw_id\"]: expected %q, got %q", "igw-0001", r0.Fields["igw_id"])
	}
	if r0.Fields["name"] != "main-igw" {
		t.Errorf("resource[0].Fields[\"name\"]: expected %q, got %q", "main-igw", r0.Fields["name"])
	}
	if r0.Fields["vpc_id"] != "vpc-aaa" {
		t.Errorf("resource[0].Fields[\"vpc_id\"]: expected %q, got %q", "vpc-aaa", r0.Fields["vpc_id"])
	}
	if r0.Fields["state"] != "attached" {
		t.Errorf("resource[0].Fields[\"state\"]: expected %q, got %q", "attached", r0.Fields["state"])
	}

	// Verify second IGW (no Name tag)
	r1 := resources[1]
	if r1.ID != "igw-0002" {
		t.Errorf("resource[1].ID: expected %q, got %q", "igw-0002", r1.ID)
	}
	if r1.Name != "" {
		t.Errorf("resource[1].Name: expected empty string, got %q", r1.Name)
	}
	if r1.Status != "attached" {
		t.Errorf("resource[1].Status: expected %q, got %q", "attached", r1.Status)
	}
	if r1.Fields["vpc_id"] != "vpc-bbb" {
		t.Errorf("resource[1].Fields[\"vpc_id\"]: expected %q, got %q", "vpc-bbb", r1.Fields["vpc_id"])
	}
}

func TestFetchInternetGateways_ErrorResponse(t *testing.T) {
	mock := &mockEC2DescribeInternetGatewaysClient{
		output: nil,
		err:    fmt.Errorf("AWS API error: access denied"),
	}

	resources, err := awsclient.FetchInternetGateways(context.Background(), mock)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if resources != nil {
		t.Errorf("expected nil resources on error, got %d resources", len(resources))
	}
}

func TestFetchInternetGateways_EmptyResponse(t *testing.T) {
	mock := &mockEC2DescribeInternetGatewaysClient{
		output: &ec2.DescribeInternetGatewaysOutput{
			InternetGateways: []ec2types.InternetGateway{},
		},
	}

	resources, err := awsclient.FetchInternetGateways(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}
