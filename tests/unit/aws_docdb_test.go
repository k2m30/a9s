package unit

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/docdb"
	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"

	awsclient "github.com/k2m30/a9s/internal/aws"
)

// ---------------------------------------------------------------------------
// T058 - Test DocumentDB response parsing
// ---------------------------------------------------------------------------

func TestFetchDocDBClusters_ParsesMultipleClusters(t *testing.T) {
	mock := &mockDocDBClient{
		output: &docdb.DescribeDBClustersOutput{
			DBClusters: []docdbtypes.DBCluster{
				{
					DBClusterIdentifier: aws.String("docdb-prod-cluster"),
					EngineVersion:       aws.String("5.0.0"),
					Status:              aws.String("available"),
					DBClusterMembers: []docdbtypes.DBClusterMember{
						{DBInstanceIdentifier: aws.String("docdb-prod-instance-1")},
						{DBInstanceIdentifier: aws.String("docdb-prod-instance-2")},
						{DBInstanceIdentifier: aws.String("docdb-prod-instance-3")},
					},
					Endpoint: aws.String("docdb-prod-cluster.cluster-abc123.us-east-1.docdb.amazonaws.com"),
				},
				{
					DBClusterIdentifier: aws.String("docdb-staging-cluster"),
					EngineVersion:       aws.String("4.0.0"),
					Status:              aws.String("available"),
					DBClusterMembers: []docdbtypes.DBClusterMember{
						{DBInstanceIdentifier: aws.String("docdb-staging-instance-1")},
					},
					Endpoint: aws.String("docdb-staging-cluster.cluster-abc123.us-east-1.docdb.amazonaws.com"),
				},
			},
		},
	}

	resources, err := awsclient.FetchDocDBClusters(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	// Verify required fields exist
	requiredFields := []string{"cluster_id", "engine_version", "status", "instances", "endpoint"}
	for i, r := range resources {
		for _, key := range requiredFields {
			if _, ok := r.Fields[key]; !ok {
				t.Errorf("resource[%d].Fields missing key %q", i, key)
			}
		}
	}

	// Verify first cluster
	r0 := resources[0]
	if r0.ID != "docdb-prod-cluster" {
		t.Errorf("resource[0].ID: expected %q, got %q", "docdb-prod-cluster", r0.ID)
	}
	if r0.Status != "available" {
		t.Errorf("resource[0].Status: expected %q, got %q", "available", r0.Status)
	}
	if r0.Fields["cluster_id"] != "docdb-prod-cluster" {
		t.Errorf("resource[0].Fields[\"cluster_id\"]: expected %q, got %q", "docdb-prod-cluster", r0.Fields["cluster_id"])
	}
	if r0.Fields["engine_version"] != "5.0.0" {
		t.Errorf("resource[0].Fields[\"engine_version\"]: expected %q, got %q", "5.0.0", r0.Fields["engine_version"])
	}
	if r0.Fields["status"] != "available" {
		t.Errorf("resource[0].Fields[\"status\"]: expected %q, got %q", "available", r0.Fields["status"])
	}
	if r0.Fields["instances"] != "3" {
		t.Errorf("resource[0].Fields[\"instances\"]: expected %q, got %q", "3", r0.Fields["instances"])
	}
	if r0.Fields["endpoint"] != "docdb-prod-cluster.cluster-abc123.us-east-1.docdb.amazonaws.com" {
		t.Errorf("resource[0].Fields[\"endpoint\"]: expected %q, got %q",
			"docdb-prod-cluster.cluster-abc123.us-east-1.docdb.amazonaws.com", r0.Fields["endpoint"])
	}

	// Verify second cluster
	r1 := resources[1]
	if r1.ID != "docdb-staging-cluster" {
		t.Errorf("resource[1].ID: expected %q, got %q", "docdb-staging-cluster", r1.ID)
	}
	if r1.Fields["instances"] != "1" {
		t.Errorf("resource[1].Fields[\"instances\"]: expected %q, got %q", "1", r1.Fields["instances"])
	}
}

func TestFetchDocDBClusters_ErrorResponse(t *testing.T) {
	mock := &mockDocDBClient{
		output: nil,
		err:    fmt.Errorf("AWS API error: access denied"),
	}

	resources, err := awsclient.FetchDocDBClusters(context.Background(), mock)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if resources != nil {
		t.Errorf("expected nil resources on error, got %d resources", len(resources))
	}
}

func TestFetchDocDBClusters_EmptyResponse(t *testing.T) {
	mock := &mockDocDBClient{
		output: &docdb.DescribeDBClustersOutput{
			DBClusters: []docdbtypes.DBCluster{},
		},
	}

	resources, err := awsclient.FetchDocDBClusters(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}
