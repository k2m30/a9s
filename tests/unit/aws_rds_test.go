package unit

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	awsclient "github.com/k2m30/a9s/internal/aws"
)

// ---------------------------------------------------------------------------
// T056 - Test RDS response parsing
// ---------------------------------------------------------------------------

func TestFetchRDSInstances_ParsesMultipleInstances(t *testing.T) {
	mock := &mockRDSClient{
		output: &rds.DescribeDBInstancesOutput{
			DBInstances: []rdstypes.DBInstance{
				{
					DBInstanceIdentifier: aws.String("prod-db-01"),
					Engine:               aws.String("mysql"),
					EngineVersion:        aws.String("8.0.35"),
					DBInstanceStatus:     aws.String("available"),
					DBInstanceClass:      aws.String("db.r5.large"),
					Endpoint: &rdstypes.Endpoint{
						Address: aws.String("prod-db-01.abc123.us-east-1.rds.amazonaws.com"),
					},
					MultiAZ: aws.Bool(true),
				},
				{
					DBInstanceIdentifier: aws.String("staging-db-01"),
					Engine:               aws.String("postgres"),
					EngineVersion:        aws.String("15.4"),
					DBInstanceStatus:     aws.String("available"),
					DBInstanceClass:      aws.String("db.t3.medium"),
					Endpoint: &rdstypes.Endpoint{
						Address: aws.String("staging-db-01.abc123.us-east-1.rds.amazonaws.com"),
					},
					MultiAZ: aws.Bool(false),
				},
			},
		},
	}

	resources, err := awsclient.FetchRDSInstances(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	// Verify first instance
	r0 := resources[0]
	if r0.ID != "prod-db-01" {
		t.Errorf("resource[0].ID: expected %q, got %q", "prod-db-01", r0.ID)
	}
	if r0.Status != "available" {
		t.Errorf("resource[0].Status: expected %q, got %q", "available", r0.Status)
	}

	// Verify required fields exist and have correct values
	requiredFields := []string{"db_identifier", "engine", "engine_version", "status", "class", "endpoint", "multi_az"}
	for i, r := range resources {
		for _, key := range requiredFields {
			if _, ok := r.Fields[key]; !ok {
				t.Errorf("resource[%d].Fields missing key %q", i, key)
			}
		}
	}

	// Verify specific field values on first instance
	if r0.Fields["db_identifier"] != "prod-db-01" {
		t.Errorf("resource[0].Fields[\"db_identifier\"]: expected %q, got %q", "prod-db-01", r0.Fields["db_identifier"])
	}
	if r0.Fields["engine"] != "mysql" {
		t.Errorf("resource[0].Fields[\"engine\"]: expected %q, got %q", "mysql", r0.Fields["engine"])
	}
	if r0.Fields["engine_version"] != "8.0.35" {
		t.Errorf("resource[0].Fields[\"engine_version\"]: expected %q, got %q", "8.0.35", r0.Fields["engine_version"])
	}
	if r0.Fields["status"] != "available" {
		t.Errorf("resource[0].Fields[\"status\"]: expected %q, got %q", "available", r0.Fields["status"])
	}
	if r0.Fields["class"] != "db.r5.large" {
		t.Errorf("resource[0].Fields[\"class\"]: expected %q, got %q", "db.r5.large", r0.Fields["class"])
	}
	if r0.Fields["endpoint"] != "prod-db-01.abc123.us-east-1.rds.amazonaws.com" {
		t.Errorf("resource[0].Fields[\"endpoint\"]: expected %q, got %q", "prod-db-01.abc123.us-east-1.rds.amazonaws.com", r0.Fields["endpoint"])
	}
	if r0.Fields["multi_az"] != "Yes" {
		t.Errorf("resource[0].Fields[\"multi_az\"]: expected %q, got %q", "Yes", r0.Fields["multi_az"])
	}

	// Verify second instance
	r1 := resources[1]
	if r1.Fields["db_identifier"] != "staging-db-01" {
		t.Errorf("resource[1].Fields[\"db_identifier\"]: expected %q, got %q", "staging-db-01", r1.Fields["db_identifier"])
	}
	if r1.Fields["engine"] != "postgres" {
		t.Errorf("resource[1].Fields[\"engine\"]: expected %q, got %q", "postgres", r1.Fields["engine"])
	}
	if r1.Fields["multi_az"] != "No" {
		t.Errorf("resource[1].Fields[\"multi_az\"]: expected %q, got %q", "No", r1.Fields["multi_az"])
	}
}

func TestFetchRDSInstances_ErrorResponse(t *testing.T) {
	mock := &mockRDSClient{
		output: nil,
		err:    fmt.Errorf("AWS API error: access denied"),
	}

	resources, err := awsclient.FetchRDSInstances(context.Background(), mock)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if resources != nil {
		t.Errorf("expected nil resources on error, got %d resources", len(resources))
	}
}

func TestFetchRDSInstances_EmptyResponse(t *testing.T) {
	mock := &mockRDSClient{
		output: &rds.DescribeDBInstancesOutput{
			DBInstances: []rdstypes.DBInstance{},
		},
	}

	resources, err := awsclient.FetchRDSInstances(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}
