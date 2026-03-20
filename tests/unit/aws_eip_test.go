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
// EIP - Test FetchElasticIPs response parsing
// ---------------------------------------------------------------------------

func TestFetchElasticIPs_ParsesMultipleAddresses(t *testing.T) {
	mock := &mockEC2DescribeAddressesClient{
		output: &ec2.DescribeAddressesOutput{
			Addresses: []ec2types.Address{
				{
					AllocationId:  aws.String("eipalloc-0a1b2c3d4e5f00001"),
					PublicIp:      aws.String("54.10.20.30"),
					AssociationId: aws.String("eipassoc-0a1b2c3d4e5f00001"),
					InstanceId:    aws.String("i-0abcdef1234567890"),
					Domain:        ec2types.DomainTypeVpc,
					Tags: []ec2types.Tag{
						{Key: aws.String("Name"), Value: aws.String("prod-nat-eip")},
					},
					PrivateIpAddress:   aws.String("10.0.1.50"),
					NetworkInterfaceId: aws.String("eni-0a1b2c3d4e5f00001"),
				},
				{
					AllocationId: aws.String("eipalloc-0a1b2c3d4e5f00002"),
					PublicIp:     aws.String("52.20.30.40"),
					Domain:       ec2types.DomainTypeVpc,
					Tags: []ec2types.Tag{
						{Key: aws.String("Name"), Value: aws.String("staging-nat-eip")},
					},
				},
			},
		},
	}

	resources, err := awsclient.FetchElasticIPs(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	// Verify required fields exist
	requiredFields := []string{"allocation_id", "public_ip", "association_id", "instance_id", "domain"}
	for i, r := range resources {
		for _, key := range requiredFields {
			if _, ok := r.Fields[key]; !ok {
				t.Errorf("resource[%d].Fields missing key %q", i, key)
			}
		}
	}

	// Verify first EIP
	r0 := resources[0]
	if r0.ID != "eipalloc-0a1b2c3d4e5f00001" {
		t.Errorf("resource[0].ID: expected %q, got %q", "eipalloc-0a1b2c3d4e5f00001", r0.ID)
	}
	if r0.Name != "prod-nat-eip" {
		t.Errorf("resource[0].Name: expected %q, got %q", "prod-nat-eip", r0.Name)
	}
	if r0.Fields["allocation_id"] != "eipalloc-0a1b2c3d4e5f00001" {
		t.Errorf("resource[0].Fields[\"allocation_id\"]: expected %q, got %q", "eipalloc-0a1b2c3d4e5f00001", r0.Fields["allocation_id"])
	}
	if r0.Fields["public_ip"] != "54.10.20.30" {
		t.Errorf("resource[0].Fields[\"public_ip\"]: expected %q, got %q", "54.10.20.30", r0.Fields["public_ip"])
	}
	if r0.Fields["association_id"] != "eipassoc-0a1b2c3d4e5f00001" {
		t.Errorf("resource[0].Fields[\"association_id\"]: expected %q, got %q", "eipassoc-0a1b2c3d4e5f00001", r0.Fields["association_id"])
	}
	if r0.Fields["instance_id"] != "i-0abcdef1234567890" {
		t.Errorf("resource[0].Fields[\"instance_id\"]: expected %q, got %q", "i-0abcdef1234567890", r0.Fields["instance_id"])
	}
	if r0.Fields["domain"] != "vpc" {
		t.Errorf("resource[0].Fields[\"domain\"]: expected %q, got %q", "vpc", r0.Fields["domain"])
	}

	// Verify second EIP (unassociated)
	r1 := resources[1]
	if r1.ID != "eipalloc-0a1b2c3d4e5f00002" {
		t.Errorf("resource[1].ID: expected %q, got %q", "eipalloc-0a1b2c3d4e5f00002", r1.ID)
	}
	if r1.Fields["public_ip"] != "52.20.30.40" {
		t.Errorf("resource[1].Fields[\"public_ip\"]: expected %q, got %q", "52.20.30.40", r1.Fields["public_ip"])
	}
	if r1.Fields["association_id"] != "" {
		t.Errorf("resource[1].Fields[\"association_id\"]: expected empty, got %q", r1.Fields["association_id"])
	}
	if r1.Fields["instance_id"] != "" {
		t.Errorf("resource[1].Fields[\"instance_id\"]: expected empty, got %q", r1.Fields["instance_id"])
	}
}

func TestFetchElasticIPs_ErrorResponse(t *testing.T) {
	mock := &mockEC2DescribeAddressesClient{
		output: nil,
		err:    fmt.Errorf("AWS API error: access denied"),
	}

	resources, err := awsclient.FetchElasticIPs(context.Background(), mock)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if resources != nil {
		t.Errorf("expected nil resources on error, got %d resources", len(resources))
	}
}

func TestFetchElasticIPs_EmptyResponse(t *testing.T) {
	mock := &mockEC2DescribeAddressesClient{
		output: &ec2.DescribeAddressesOutput{
			Addresses: []ec2types.Address{},
		},
	}

	resources, err := awsclient.FetchElasticIPs(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}
