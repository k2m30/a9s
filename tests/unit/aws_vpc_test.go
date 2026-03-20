package unit

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	awsclient "github.com/k2m30/a9s/internal/aws"
	"github.com/k2m30/a9s/tests/testdata"
)

// ---------------------------------------------------------------------------
// VPC fetcher tests
// ---------------------------------------------------------------------------

func TestFetchVPCs_ParsesMultipleVPCs(t *testing.T) {
	mock := &mockEC2DescribeVpcsClient{
		output: &ec2.DescribeVpcsOutput{
			Vpcs: []ec2types.Vpc{
				{
					VpcId:           aws.String("vpc-0001"),
					CidrBlock:       aws.String("10.0.0.0/16"),
					State:           ec2types.VpcStateAvailable,
					IsDefault:       aws.Bool(true),
					DhcpOptionsId:   aws.String("dopt-abc123"),
					InstanceTenancy: ec2types.TenancyDefault,
					OwnerId:         aws.String("123456789012"),
					Tags: []ec2types.Tag{
						{Key: aws.String("Name"), Value: aws.String("main-vpc")},
						{Key: aws.String("Env"), Value: aws.String("prod")},
					},
				},
				{
					VpcId:           aws.String("vpc-0002"),
					CidrBlock:       aws.String("172.16.0.0/16"),
					State:           ec2types.VpcStatePending,
					IsDefault:       aws.Bool(false),
					DhcpOptionsId:   aws.String("dopt-def456"),
					InstanceTenancy: ec2types.TenancyDedicated,
					OwnerId:         aws.String("123456789012"),
					Tags:            []ec2types.Tag{},
				},
			},
		},
	}

	resources, err := awsclient.FetchVPCs(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	// Verify first VPC
	r0 := resources[0]
	if r0.ID != "vpc-0001" {
		t.Errorf("resource[0].ID: expected %q, got %q", "vpc-0001", r0.ID)
	}
	if r0.Name != "main-vpc" {
		t.Errorf("resource[0].Name: expected %q, got %q", "main-vpc", r0.Name)
	}
	if r0.Status != "available" {
		t.Errorf("resource[0].Status: expected %q, got %q", "available", r0.Status)
	}

	// Verify Fields
	requiredFields := []string{"vpc_id", "name", "cidr_block", "state", "is_default"}
	for i, r := range resources {
		for _, key := range requiredFields {
			if _, ok := r.Fields[key]; !ok {
				t.Errorf("resource[%d].Fields missing key %q", i, key)
			}
		}
	}

	// Verify specific field values on first VPC
	if r0.Fields["vpc_id"] != "vpc-0001" {
		t.Errorf("resource[0].Fields[\"vpc_id\"]: expected %q, got %q", "vpc-0001", r0.Fields["vpc_id"])
	}
	if r0.Fields["name"] != "main-vpc" {
		t.Errorf("resource[0].Fields[\"name\"]: expected %q, got %q", "main-vpc", r0.Fields["name"])
	}
	if r0.Fields["cidr_block"] != "10.0.0.0/16" {
		t.Errorf("resource[0].Fields[\"cidr_block\"]: expected %q, got %q", "10.0.0.0/16", r0.Fields["cidr_block"])
	}
	if r0.Fields["state"] != "available" {
		t.Errorf("resource[0].Fields[\"state\"]: expected %q, got %q", "available", r0.Fields["state"])
	}
	if r0.Fields["is_default"] != "true" {
		t.Errorf("resource[0].Fields[\"is_default\"]: expected %q, got %q", "true", r0.Fields["is_default"])
	}

	// Verify second VPC (no Name tag, pending state, not default)
	r1 := resources[1]
	if r1.ID != "vpc-0002" {
		t.Errorf("resource[1].ID: expected %q, got %q", "vpc-0002", r1.ID)
	}
	if r1.Name != "" {
		t.Errorf("resource[1].Name: expected empty string, got %q", r1.Name)
	}
	if r1.Status != "pending" {
		t.Errorf("resource[1].Status: expected %q, got %q", "pending", r1.Status)
	}
	if r1.Fields["is_default"] != "false" {
		t.Errorf("resource[1].Fields[\"is_default\"]: expected %q, got %q", "false", r1.Fields["is_default"])
	}
}

func TestFetchVPCs_ErrorResponse(t *testing.T) {
	mock := &mockEC2DescribeVpcsClient{
		output: nil,
		err:    fmt.Errorf("AWS API error: access denied"),
	}

	resources, err := awsclient.FetchVPCs(context.Background(), mock)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if resources != nil {
		t.Errorf("expected nil resources on error, got %d resources", len(resources))
	}
}

func TestFetchVPCs_EmptyResponse(t *testing.T) {
	mock := &mockEC2DescribeVpcsClient{
		output: &ec2.DescribeVpcsOutput{
			Vpcs: []ec2types.Vpc{},
		},
	}

	resources, err := awsclient.FetchVPCs(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}

func TestFetchVPCs_RawStructPopulated(t *testing.T) {
	mock := &mockEC2DescribeVpcsClient{
		output: &ec2.DescribeVpcsOutput{
			Vpcs: []ec2types.Vpc{
				{
					VpcId:     aws.String("vpc-raw123"),
					CidrBlock: aws.String("10.0.0.0/16"),
					State:     ec2types.VpcStateAvailable,
					IsDefault: aws.Bool(false),
				},
			},
		},
	}

	resources, err := awsclient.FetchVPCs(context.Background(), mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	r := resources[0]

	// Verify RawStruct is populated
	if r.RawStruct == nil {
		t.Fatal("RawStruct must not be nil")
	}

	// Verify it's the correct type (ec2types.Vpc)
	vpc, ok := r.RawStruct.(ec2types.Vpc)
	if !ok {
		t.Fatalf("RawStruct should be ec2types.Vpc, got %T", r.RawStruct)
	}
	if vpc.VpcId == nil || *vpc.VpcId != "vpc-raw123" {
		t.Errorf("RawStruct.VpcId: expected %q, got %v", "vpc-raw123", vpc.VpcId)
	}

}

// ---------------------------------------------------------------------------
// T-VPC-REAL - Test VPC fetcher with real sanitized fixture data
// ---------------------------------------------------------------------------

func TestFetchVPCs_RealAWSData(t *testing.T) {
	mock := &mockEC2DescribeVpcsClient{
		output: &ec2.DescribeVpcsOutput{
			Vpcs: testdata.RealVPCs(),
		},
	}

	resources, err := awsclient.FetchVPCs(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Real data has exactly 2 VPCs
	if len(resources) != 2 {
		t.Fatalf("expected 2 resources from real data, got %d", len(resources))
	}

	// --- VPC 1: dev-vpc (non-default, 10.0.0.0/16) ---
	r0 := resources[0]
	if r0.ID != "vpc-0aaa1111bbb2222cc" {
		t.Errorf("resource[0].ID: expected %q, got %q", "vpc-0aaa1111bbb2222cc", r0.ID)
	}
	if r0.Name != "dev-vpc" {
		t.Errorf("resource[0].Name: expected %q, got %q", "dev-vpc", r0.Name)
	}
	if r0.Status != "available" {
		t.Errorf("resource[0].Status: expected %q, got %q", "available", r0.Status)
	}
	if r0.Fields["vpc_id"] != "vpc-0aaa1111bbb2222cc" {
		t.Errorf("resource[0].Fields[\"vpc_id\"]: expected %q, got %q", "vpc-0aaa1111bbb2222cc", r0.Fields["vpc_id"])
	}
	if r0.Fields["cidr_block"] != "10.0.0.0/16" {
		t.Errorf("resource[0].Fields[\"cidr_block\"]: expected %q, got %q", "10.0.0.0/16", r0.Fields["cidr_block"])
	}
	if r0.Fields["state"] != "available" {
		t.Errorf("resource[0].Fields[\"state\"]: expected %q, got %q", "available", r0.Fields["state"])
	}
	if r0.Fields["is_default"] != "false" {
		t.Errorf("resource[0].Fields[\"is_default\"]: expected %q, got %q", "false", r0.Fields["is_default"])
	}

	// RawStruct verification for VPC 1
	if r0.RawStruct == nil {
		t.Fatal("resource[0].RawStruct must not be nil")
	}
	vpc0, ok := r0.RawStruct.(ec2types.Vpc)
	if !ok {
		t.Fatalf("resource[0].RawStruct should be ec2types.Vpc, got %T", r0.RawStruct)
	}
	if len(vpc0.CidrBlockAssociationSet) != 1 {
		t.Errorf("resource[0].RawStruct.CidrBlockAssociationSet: expected 1, got %d", len(vpc0.CidrBlockAssociationSet))
	}
	if vpc0.CidrBlockAssociationSet[0].CidrBlock == nil || *vpc0.CidrBlockAssociationSet[0].CidrBlock != "10.0.0.0/16" {
		t.Errorf("resource[0].RawStruct.CidrBlockAssociationSet[0].CidrBlock: expected %q", "10.0.0.0/16")
	}

	// --- VPC 2: default VPC (172.31.0.0/16, no Name tag) ---
	r1 := resources[1]
	if r1.ID != "vpc-0ddd3333eee4444ff" {
		t.Errorf("resource[1].ID: expected %q, got %q", "vpc-0ddd3333eee4444ff", r1.ID)
	}
	if r1.Name != "" {
		t.Errorf("resource[1].Name: expected empty (no Name tag on default VPC), got %q", r1.Name)
	}
	if r1.Status != "available" {
		t.Errorf("resource[1].Status: expected %q, got %q", "available", r1.Status)
	}
	if r1.Fields["cidr_block"] != "172.31.0.0/16" {
		t.Errorf("resource[1].Fields[\"cidr_block\"]: expected %q, got %q", "172.31.0.0/16", r1.Fields["cidr_block"])
	}
	if r1.Fields["is_default"] != "true" {
		t.Errorf("resource[1].Fields[\"is_default\"]: expected %q, got %q", "true", r1.Fields["is_default"])
	}

	// Verify the default VPC has an empty Tags slice (Tags exist but Name tag absent)
	vpc1, ok := r1.RawStruct.(ec2types.Vpc)
	if !ok {
		t.Fatalf("resource[1].RawStruct should be ec2types.Vpc, got %T", r1.RawStruct)
	}
	if len(vpc1.Tags) != 0 {
		t.Errorf("resource[1].RawStruct should have 0 tags, got %d", len(vpc1.Tags))
	}

	// Verify DHCP Options ID is shared between both VPCs
}
