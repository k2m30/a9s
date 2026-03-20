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
// Subnet fetcher tests
// ---------------------------------------------------------------------------

func TestFetchSubnets_ParsesMultipleSubnets(t *testing.T) {
	mock := &mockEC2DescribeSubnetsClient{
		output: &ec2.DescribeSubnetsOutput{
			Subnets: []ec2types.Subnet{
				{
					SubnetId:                aws.String("subnet-0001"),
					VpcId:                   aws.String("vpc-aaa"),
					CidrBlock:               aws.String("10.0.1.0/24"),
					AvailabilityZone:        aws.String("us-east-1a"),
					State:                   ec2types.SubnetStateAvailable,
					AvailableIpAddressCount: aws.Int32(251),
					MapPublicIpOnLaunch:     aws.Bool(true),
					DefaultForAz:            aws.Bool(false),
					OwnerId:                 aws.String("123456789012"),
					SubnetArn:               aws.String("arn:aws:ec2:us-east-1:123456789012:subnet/subnet-0001"),
					Tags: []ec2types.Tag{
						{Key: aws.String("Name"), Value: aws.String("public-subnet-1a")},
						{Key: aws.String("Env"), Value: aws.String("prod")},
					},
				},
				{
					SubnetId:                aws.String("subnet-0002"),
					VpcId:                   aws.String("vpc-bbb"),
					CidrBlock:               aws.String("10.0.2.0/24"),
					AvailabilityZone:        aws.String("us-east-1b"),
					State:                   ec2types.SubnetStatePending,
					AvailableIpAddressCount: aws.Int32(245),
					MapPublicIpOnLaunch:     aws.Bool(false),
					DefaultForAz:            aws.Bool(false),
					OwnerId:                 aws.String("123456789012"),
					Tags:                    []ec2types.Tag{},
				},
			},
		},
	}

	resources, err := awsclient.FetchSubnets(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	// Verify first subnet
	r0 := resources[0]
	if r0.ID != "subnet-0001" {
		t.Errorf("resource[0].ID: expected %q, got %q", "subnet-0001", r0.ID)
	}
	if r0.Name != "public-subnet-1a" {
		t.Errorf("resource[0].Name: expected %q, got %q", "public-subnet-1a", r0.Name)
	}
	if r0.Status != "available" {
		t.Errorf("resource[0].Status: expected %q, got %q", "available", r0.Status)
	}

	// Verify Fields on all resources
	requiredFields := []string{"subnet_id", "name", "vpc_id", "cidr_block", "availability_zone", "state", "available_ips"}
	for i, r := range resources {
		for _, key := range requiredFields {
			if _, ok := r.Fields[key]; !ok {
				t.Errorf("resource[%d].Fields missing key %q", i, key)
			}
		}
	}

	// Verify specific field values on first subnet
	if r0.Fields["subnet_id"] != "subnet-0001" {
		t.Errorf("resource[0].Fields[\"subnet_id\"]: expected %q, got %q", "subnet-0001", r0.Fields["subnet_id"])
	}
	if r0.Fields["name"] != "public-subnet-1a" {
		t.Errorf("resource[0].Fields[\"name\"]: expected %q, got %q", "public-subnet-1a", r0.Fields["name"])
	}
	if r0.Fields["vpc_id"] != "vpc-aaa" {
		t.Errorf("resource[0].Fields[\"vpc_id\"]: expected %q, got %q", "vpc-aaa", r0.Fields["vpc_id"])
	}
	if r0.Fields["cidr_block"] != "10.0.1.0/24" {
		t.Errorf("resource[0].Fields[\"cidr_block\"]: expected %q, got %q", "10.0.1.0/24", r0.Fields["cidr_block"])
	}
	if r0.Fields["availability_zone"] != "us-east-1a" {
		t.Errorf("resource[0].Fields[\"availability_zone\"]: expected %q, got %q", "us-east-1a", r0.Fields["availability_zone"])
	}
	if r0.Fields["state"] != "available" {
		t.Errorf("resource[0].Fields[\"state\"]: expected %q, got %q", "available", r0.Fields["state"])
	}
	if r0.Fields["available_ips"] != "251" {
		t.Errorf("resource[0].Fields[\"available_ips\"]: expected %q, got %q", "251", r0.Fields["available_ips"])
	}

	// Verify second subnet (no Name tag, pending state)
	r1 := resources[1]
	if r1.ID != "subnet-0002" {
		t.Errorf("resource[1].ID: expected %q, got %q", "subnet-0002", r1.ID)
	}
	if r1.Name != "" {
		t.Errorf("resource[1].Name: expected empty string, got %q", r1.Name)
	}
	if r1.Status != "pending" {
		t.Errorf("resource[1].Status: expected %q, got %q", "pending", r1.Status)
	}
	if r1.Fields["vpc_id"] != "vpc-bbb" {
		t.Errorf("resource[1].Fields[\"vpc_id\"]: expected %q, got %q", "vpc-bbb", r1.Fields["vpc_id"])
	}
	if r1.Fields["cidr_block"] != "10.0.2.0/24" {
		t.Errorf("resource[1].Fields[\"cidr_block\"]: expected %q, got %q", "10.0.2.0/24", r1.Fields["cidr_block"])
	}
}

func TestFetchSubnets_ErrorResponse(t *testing.T) {
	mock := &mockEC2DescribeSubnetsClient{
		output: nil,
		err:    fmt.Errorf("AWS API error: access denied"),
	}

	resources, err := awsclient.FetchSubnets(context.Background(), mock)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if resources != nil {
		t.Errorf("expected nil resources on error, got %d resources", len(resources))
	}
}

func TestFetchSubnets_EmptyResponse(t *testing.T) {
	mock := &mockEC2DescribeSubnetsClient{
		output: &ec2.DescribeSubnetsOutput{
			Subnets: []ec2types.Subnet{},
		},
	}

	resources, err := awsclient.FetchSubnets(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}
