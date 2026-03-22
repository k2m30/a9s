package unit

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
)

// ---------------------------------------------------------------------------
// Route Table fetcher tests
// ---------------------------------------------------------------------------

func TestFetchRouteTables_ParsesMultipleRouteTables(t *testing.T) {
	mock := &mockEC2DescribeRouteTablesClient{
		output: &ec2.DescribeRouteTablesOutput{
			RouteTables: []ec2types.RouteTable{
				{
					RouteTableId: aws.String("rtb-0001"),
					VpcId:        aws.String("vpc-aaa"),
					OwnerId:      aws.String("123456789012"),
					Routes: []ec2types.Route{
						{DestinationCidrBlock: aws.String("0.0.0.0/0")},
						{DestinationCidrBlock: aws.String("10.0.0.0/16")},
					},
					Associations: []ec2types.RouteTableAssociation{
						{Main: aws.Bool(true)},
					},
					Tags: []ec2types.Tag{
						{Key: aws.String("Name"), Value: aws.String("main-rtb")},
						{Key: aws.String("Env"), Value: aws.String("prod")},
					},
				},
				{
					RouteTableId: aws.String("rtb-0002"),
					VpcId:        aws.String("vpc-bbb"),
					OwnerId:      aws.String("123456789012"),
					Routes: []ec2types.Route{
						{DestinationCidrBlock: aws.String("10.1.0.0/16")},
					},
					Associations: []ec2types.RouteTableAssociation{},
					Tags:         []ec2types.Tag{},
				},
			},
		},
	}

	resources, err := awsclient.FetchRouteTables(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	// Verify first route table
	r0 := resources[0]
	if r0.ID != "rtb-0001" {
		t.Errorf("resource[0].ID: expected %q, got %q", "rtb-0001", r0.ID)
	}
	if r0.Name != "main-rtb" {
		t.Errorf("resource[0].Name: expected %q, got %q", "main-rtb", r0.Name)
	}
	// Status is "true" when Main association exists, "false" otherwise
	if r0.Status != "true" {
		t.Errorf("resource[0].Status: expected %q, got %q", "true", r0.Status)
	}

	// Verify Fields on all resources
	requiredFields := []string{"route_table_id", "name", "vpc_id", "routes_count", "associations_count"}
	for i, r := range resources {
		for _, key := range requiredFields {
			if _, ok := r.Fields[key]; !ok {
				t.Errorf("resource[%d].Fields missing key %q", i, key)
			}
		}
	}

	// Verify specific field values on first route table
	if r0.Fields["route_table_id"] != "rtb-0001" {
		t.Errorf("resource[0].Fields[\"route_table_id\"]: expected %q, got %q", "rtb-0001", r0.Fields["route_table_id"])
	}
	if r0.Fields["name"] != "main-rtb" {
		t.Errorf("resource[0].Fields[\"name\"]: expected %q, got %q", "main-rtb", r0.Fields["name"])
	}
	if r0.Fields["vpc_id"] != "vpc-aaa" {
		t.Errorf("resource[0].Fields[\"vpc_id\"]: expected %q, got %q", "vpc-aaa", r0.Fields["vpc_id"])
	}
	if r0.Fields["routes_count"] != "2" {
		t.Errorf("resource[0].Fields[\"routes_count\"]: expected %q, got %q", "2", r0.Fields["routes_count"])
	}
	if r0.Fields["associations_count"] != "1" {
		t.Errorf("resource[0].Fields[\"associations_count\"]: expected %q, got %q", "1", r0.Fields["associations_count"])
	}

	// Verify second route table (no Name tag, not main)
	r1 := resources[1]
	if r1.ID != "rtb-0002" {
		t.Errorf("resource[1].ID: expected %q, got %q", "rtb-0002", r1.ID)
	}
	if r1.Name != "" {
		t.Errorf("resource[1].Name: expected empty string, got %q", r1.Name)
	}
	if r1.Status != "false" {
		t.Errorf("resource[1].Status: expected %q, got %q", "false", r1.Status)
	}
	if r1.Fields["vpc_id"] != "vpc-bbb" {
		t.Errorf("resource[1].Fields[\"vpc_id\"]: expected %q, got %q", "vpc-bbb", r1.Fields["vpc_id"])
	}
	if r1.Fields["routes_count"] != "1" {
		t.Errorf("resource[1].Fields[\"routes_count\"]: expected %q, got %q", "1", r1.Fields["routes_count"])
	}
	if r1.Fields["associations_count"] != "0" {
		t.Errorf("resource[1].Fields[\"associations_count\"]: expected %q, got %q", "0", r1.Fields["associations_count"])
	}
}

func TestFetchRouteTables_ErrorResponse(t *testing.T) {
	mock := &mockEC2DescribeRouteTablesClient{
		output: nil,
		err:    fmt.Errorf("AWS API error: access denied"),
	}

	resources, err := awsclient.FetchRouteTables(context.Background(), mock)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if resources != nil {
		t.Errorf("expected nil resources on error, got %d resources", len(resources))
	}
}

func TestFetchRouteTables_EmptyResponse(t *testing.T) {
	mock := &mockEC2DescribeRouteTablesClient{
		output: &ec2.DescribeRouteTablesOutput{
			RouteTables: []ec2types.RouteTable{},
		},
	}

	resources, err := awsclient.FetchRouteTables(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}
