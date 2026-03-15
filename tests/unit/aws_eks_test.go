package unit

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"

	awsclient "github.com/k2m30/a9s/internal/aws"
)

// mockEKSListClustersClient implements awsclient.EKSListClustersAPI for testing.
type mockEKSListClustersClient struct {
	output *eks.ListClustersOutput
	err    error
}

func (m *mockEKSListClustersClient) ListClusters(
	ctx context.Context,
	params *eks.ListClustersInput,
	optFns ...func(*eks.Options),
) (*eks.ListClustersOutput, error) {
	return m.output, m.err
}

// mockEKSDescribeClusterClient implements awsclient.EKSDescribeClusterAPI for testing.
type mockEKSDescribeClusterClient struct {
	outputs map[string]*eks.DescribeClusterOutput
	err     error
}

func (m *mockEKSDescribeClusterClient) DescribeCluster(
	ctx context.Context,
	params *eks.DescribeClusterInput,
	optFns ...func(*eks.Options),
) (*eks.DescribeClusterOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	if out, ok := m.outputs[*params.Name]; ok {
		return out, nil
	}
	return nil, fmt.Errorf("cluster %q not found", *params.Name)
}

// ---------------------------------------------------------------------------
// T059 - Test EKS two-step fetch (ListClusters + DescribeCluster)
// ---------------------------------------------------------------------------

func TestFetchEKSClusters_ParsesMultipleClusters(t *testing.T) {
	listMock := &mockEKSListClustersClient{
		output: &eks.ListClustersOutput{
			Clusters: []string{"cluster-a", "cluster-b"},
		},
	}

	describeMock := &mockEKSDescribeClusterClient{
		outputs: map[string]*eks.DescribeClusterOutput{
			"cluster-a": {
				Cluster: &ekstypes.Cluster{
					Name:            aws.String("cluster-a"),
					Version:         aws.String("1.28"),
					Status:          ekstypes.ClusterStatusActive,
					Endpoint:        aws.String("https://ABCDEF1234567890.gr7.us-east-1.eks.amazonaws.com"),
					PlatformVersion: aws.String("eks.5"),
				},
			},
			"cluster-b": {
				Cluster: &ekstypes.Cluster{
					Name:            aws.String("cluster-b"),
					Version:         aws.String("1.27"),
					Status:          ekstypes.ClusterStatusActive,
					Endpoint:        aws.String("https://GHIJKL1234567890.gr7.us-east-1.eks.amazonaws.com"),
					PlatformVersion: aws.String("eks.3"),
				},
			},
		},
	}

	resources, err := awsclient.FetchEKSClusters(context.Background(), listMock, describeMock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	// Verify required fields exist
	requiredFields := []string{"cluster_name", "version", "status", "endpoint", "platform_version"}
	for i, r := range resources {
		for _, key := range requiredFields {
			if _, ok := r.Fields[key]; !ok {
				t.Errorf("resource[%d].Fields missing key %q", i, key)
			}
		}
	}

	// Verify first cluster
	r0 := resources[0]
	if r0.ID != "cluster-a" {
		t.Errorf("resource[0].ID: expected %q, got %q", "cluster-a", r0.ID)
	}
	if r0.Name != "cluster-a" {
		t.Errorf("resource[0].Name: expected %q, got %q", "cluster-a", r0.Name)
	}
	if r0.Status != "ACTIVE" {
		t.Errorf("resource[0].Status: expected %q, got %q", "ACTIVE", r0.Status)
	}
	if r0.Fields["cluster_name"] != "cluster-a" {
		t.Errorf("resource[0].Fields[\"cluster_name\"]: expected %q, got %q", "cluster-a", r0.Fields["cluster_name"])
	}
	if r0.Fields["version"] != "1.28" {
		t.Errorf("resource[0].Fields[\"version\"]: expected %q, got %q", "1.28", r0.Fields["version"])
	}
	if r0.Fields["status"] != "ACTIVE" {
		t.Errorf("resource[0].Fields[\"status\"]: expected %q, got %q", "ACTIVE", r0.Fields["status"])
	}
	if r0.Fields["endpoint"] != "https://ABCDEF1234567890.gr7.us-east-1.eks.amazonaws.com" {
		t.Errorf("resource[0].Fields[\"endpoint\"]: expected %q, got %q",
			"https://ABCDEF1234567890.gr7.us-east-1.eks.amazonaws.com", r0.Fields["endpoint"])
	}
	if r0.Fields["platform_version"] != "eks.5" {
		t.Errorf("resource[0].Fields[\"platform_version\"]: expected %q, got %q", "eks.5", r0.Fields["platform_version"])
	}

	// Verify second cluster
	r1 := resources[1]
	if r1.ID != "cluster-b" {
		t.Errorf("resource[1].ID: expected %q, got %q", "cluster-b", r1.ID)
	}
	if r1.Fields["version"] != "1.27" {
		t.Errorf("resource[1].Fields[\"version\"]: expected %q, got %q", "1.27", r1.Fields["version"])
	}
	if r1.Fields["platform_version"] != "eks.3" {
		t.Errorf("resource[1].Fields[\"platform_version\"]: expected %q, got %q", "eks.3", r1.Fields["platform_version"])
	}
}

func TestFetchEKSClusters_ListClustersError(t *testing.T) {
	listMock := &mockEKSListClustersClient{
		output: nil,
		err:    fmt.Errorf("AWS API error: access denied"),
	}
	describeMock := &mockEKSDescribeClusterClient{}

	resources, err := awsclient.FetchEKSClusters(context.Background(), listMock, describeMock)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if resources != nil {
		t.Errorf("expected nil resources on error, got %d resources", len(resources))
	}
}

func TestFetchEKSClusters_EmptyResponse(t *testing.T) {
	listMock := &mockEKSListClustersClient{
		output: &eks.ListClustersOutput{
			Clusters: []string{},
		},
	}
	describeMock := &mockEKSDescribeClusterClient{}

	resources, err := awsclient.FetchEKSClusters(context.Background(), listMock, describeMock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}
