package unit

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
)

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
	// Post-fold contract: ACTIVE state is healthy → no Status, no Finding.
	if r0.Status != "" {
		t.Errorf("resource[0].Status: expected %q (fetcher must not write Status), got %q", "", r0.Status)
	}
	if len(r0.Findings) != 0 {
		t.Errorf("resource[0].Findings: expected 0 for ACTIVE cluster, got %d", len(r0.Findings))
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

// ---------------------------------------------------------------------------
// TestFetchEKSClusters_DescribeFailureSurfacesError verifies that
// FetchEKSClustersPage returns partial results for successful clusters AND a
// composite error for failed DescribeCluster calls. The failed cluster is not
// silently dropped — the caller receives the error via the second return value.
// ---------------------------------------------------------------------------

// eksTestFake implements awsclient.EKSAPI for registered-fetcher tests.
// It supports per-cluster describe errors via errByName.
type eksDescribeFailFake struct {
	// clusters returned by ListClusters
	clusters []string
	// outputs keyed by cluster name
	outputs map[string]*eks.DescribeClusterOutput
	// errByName maps cluster name → error returned by DescribeCluster
	errByName map[string]error
}

func (f *eksDescribeFailFake) ListClusters(
	_ context.Context,
	_ *eks.ListClustersInput,
	_ ...func(*eks.Options),
) (*eks.ListClustersOutput, error) {
	return &eks.ListClustersOutput{Clusters: f.clusters}, nil
}

func (f *eksDescribeFailFake) DescribeCluster(
	_ context.Context,
	input *eks.DescribeClusterInput,
	_ ...func(*eks.Options),
) (*eks.DescribeClusterOutput, error) {
	name := aws.ToString(input.Name)
	if err, ok := f.errByName[name]; ok {
		return nil, err
	}
	if out, ok := f.outputs[name]; ok {
		return out, nil
	}
	return nil, fmt.Errorf("cluster %q not found", name)
}

func (f *eksDescribeFailFake) ListNodegroups(
	_ context.Context,
	_ *eks.ListNodegroupsInput,
	_ ...func(*eks.Options),
) (*eks.ListNodegroupsOutput, error) {
	return &eks.ListNodegroupsOutput{}, nil
}

func (f *eksDescribeFailFake) DescribeNodegroup(
	_ context.Context,
	_ *eks.DescribeNodegroupInput,
	_ ...func(*eks.Options),
) (*eks.DescribeNodegroupOutput, error) {
	return &eks.DescribeNodegroupOutput{}, nil
}

// Compile-time check: eksDescribeFailFake satisfies awsclient.EKSAPI.
var _ awsclient.EKSAPI = (*eksDescribeFailFake)(nil)

func TestFetchEKSClusters_DescribeFailureSurfacesError(t *testing.T) {
	eksFake := &eksDescribeFailFake{
		clusters: []string{"cluster-ok", "cluster-bad"},
		outputs: map[string]*eks.DescribeClusterOutput{
			"cluster-ok": {
				Cluster: &ekstypes.Cluster{
					Name:            aws.String("cluster-ok"),
					Version:         aws.String("1.28"),
					Status:          ekstypes.ClusterStatusActive,
					Endpoint:        aws.String("https://OK123.gr7.us-east-1.eks.amazonaws.com"),
					PlatformVersion: aws.String("eks.5"),
				},
			},
		},
		errByName: map[string]error{
			"cluster-bad": fmt.Errorf("eks: DescribeCluster: AccessDeniedException"),
		},
	}

	clients := &awsclient.ServiceClients{EKS: eksFake}

	result, err := awsclient.FetchEKSClustersPage(context.Background(), clients, "")

	// The fetcher must surface a composite error for the failing cluster.
	if err == nil {
		t.Fatal("FetchEKSClustersPage must return a non-nil error when DescribeCluster fails for a cluster")
	}
	if errStr := err.Error(); !strings.Contains(errStr, "eks: DescribeCluster") {
		t.Errorf("composite error must contain \"eks: DescribeCluster\", got: %q", errStr)
	}
	if errStr := err.Error(); !strings.Contains(errStr, "cluster-bad") {
		t.Errorf("composite error must contain the failing cluster name \"cluster-bad\", got: %q", errStr)
	}

	// The successful cluster must still appear in partial results.
	if len(result.Resources) != 1 {
		t.Errorf(
			"FetchEKSClustersPage must return partial results — got %d rows, want 1 (cluster-ok only)",
			len(result.Resources),
		)
	}
	if len(result.Resources) == 1 && result.Resources[0].ID != "cluster-ok" {
		t.Errorf("partial result must contain \"cluster-ok\", got ID %q", result.Resources[0].ID)
	}
}
