package unit

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/redshift"
	redshifttypes "github.com/aws/aws-sdk-go-v2/service/redshift/types"

	awsclient "github.com/k2m30/a9s/internal/aws"
)

// ---------------------------------------------------------------------------
// T-RS-001 - Test Redshift Clusters response parsing
// ---------------------------------------------------------------------------

func TestFetchRedshiftClusters_ParsesMultipleClusters(t *testing.T) {
	now := time.Now()
	mock := &mockRedshiftClient{
		output: &redshift.DescribeClustersOutput{
			Clusters: []redshifttypes.Cluster{
				{
					ClusterIdentifier: aws.String("my-cluster-1"),
					ClusterStatus:     aws.String("available"),
					DBName:            aws.String("mydb"),
					NodeType:          aws.String("ra3.4xlarge"),
					NumberOfNodes:     aws.Int32(4),
					ClusterCreateTime: &now,
					Endpoint: &redshifttypes.Endpoint{
						Address: aws.String("my-cluster-1.abc.us-east-1.redshift.amazonaws.com"),
						Port:    aws.Int32(5439),
					},
					MasterUsername: aws.String("admin"),
				},
				{
					ClusterIdentifier: aws.String("my-cluster-2"),
					ClusterStatus:     aws.String("creating"),
					NodeType:          aws.String("dc2.large"),
					NumberOfNodes:     aws.Int32(2),
				},
			},
		},
	}

	resources, err := awsclient.FetchRedshiftClusters(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	r := resources[0]
	if r.Name != "my-cluster-1" {
		t.Errorf("expected Name 'my-cluster-1', got %q", r.Name)
	}
	if r.ID != "my-cluster-1" {
		t.Errorf("expected ID 'my-cluster-1', got %q", r.ID)
	}
	if r.Status != "available" {
		t.Errorf("expected Status 'available', got %q", r.Status)
	}
	if r.Fields["cluster_id"] != "my-cluster-1" {
		t.Errorf("expected Fields[cluster_id] 'my-cluster-1', got %q", r.Fields["cluster_id"])
	}
	if r.Fields["status"] != "available" {
		t.Errorf("expected Fields[status] 'available', got %q", r.Fields["status"])
	}
	if r.Fields["node_type"] != "ra3.4xlarge" {
		t.Errorf("expected Fields[node_type] 'ra3.4xlarge', got %q", r.Fields["node_type"])
	}
	if r.Fields["num_nodes"] != "4" {
		t.Errorf("expected Fields[num_nodes] '4', got %q", r.Fields["num_nodes"])
	}
	if r.Fields["endpoint"] != "my-cluster-1.abc.us-east-1.redshift.amazonaws.com" {
		t.Errorf("expected Fields[endpoint] to match, got %q", r.Fields["endpoint"])
	}
	if r.Fields["db_name"] != "mydb" {
		t.Errorf("expected Fields[db_name] 'mydb', got %q", r.Fields["db_name"])
	}

	if r.RawStruct == nil {
		t.Error("expected RawStruct to be set")
	}

	// Second cluster
	r2 := resources[1]
	if r2.Status != "creating" {
		t.Errorf("expected Status 'creating', got %q", r2.Status)
	}
}

func TestFetchRedshiftClusters_EmptyResponse(t *testing.T) {
	mock := &mockRedshiftClient{
		output: &redshift.DescribeClustersOutput{
			Clusters: []redshifttypes.Cluster{},
		},
	}

	resources, err := awsclient.FetchRedshiftClusters(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 0 {
		t.Fatalf("expected 0 resources, got %d", len(resources))
	}
}

func TestFetchRedshiftClusters_APIError(t *testing.T) {
	mock := &mockRedshiftClient{
		err: &mockAPIError{code: "ClusterNotFound", message: "not found"},
	}

	_, err := awsclient.FetchRedshiftClusters(context.Background(), mock)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestFetchRedshiftClusters_NilEndpoint(t *testing.T) {
	mock := &mockRedshiftClient{
		output: &redshift.DescribeClustersOutput{
			Clusters: []redshifttypes.Cluster{
				{
					ClusterIdentifier: aws.String("no-endpoint"),
					ClusterStatus:     aws.String("creating"),
					NodeType:          aws.String("dc2.large"),
					NumberOfNodes:     aws.Int32(1),
				},
			},
		},
	}

	resources, err := awsclient.FetchRedshiftClusters(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	if resources[0].Fields["endpoint"] != "" {
		t.Errorf("expected empty endpoint, got %q", resources[0].Fields["endpoint"])
	}
}
