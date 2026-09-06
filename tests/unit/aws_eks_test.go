package unit

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
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

	clients := &awsclient.ServiceClients{EKS: newMockEKSFull(listMock, describeMock, nil, nil)}
	result, err := awsclient.FetchEKSClustersPage(context.Background(), clients, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	resources := result.Resources

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
	// Post-fold contract: ACTIVE state is healthy → no lifecycle Finding.
	//
	// Inverted deliberately: this counted Findings and expected 0. It cannot,
	// now that posture signals share the slice — this fixture declares no
	// control-plane logging and no encryption configuration, so it legitimately
	// carries those two findings. Counting the whole slice makes every future
	// posture row look like a regression here, so the assertion names the
	// lifecycle codes it is actually about.
	for _, code := range []domain.FindingCode{
		awsclient.CodeEKSStateCreating, awsclient.CodeEKSStateUpdating,
		awsclient.CodeEKSStateFailed, awsclient.CodeEKSHealthIssue,
	} {
		if _, found := pw1FindFinding(r0.Findings, code); found {
			t.Errorf("resource[0].Findings: ACTIVE cluster carries lifecycle finding %q (all: %+v)", code, r0.Findings)
		}
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

	clients := &awsclient.ServiceClients{EKS: newMockEKSFull(listMock, describeMock, nil, nil)}
	result, err := awsclient.FetchEKSClustersPage(context.Background(), clients, "")
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected no resources on error, got %d resources", len(result.Resources))
	}
}

func TestFetchEKSClusters_EmptyResponse(t *testing.T) {
	listMock := &mockEKSListClustersClient{
		output: &eks.ListClustersOutput{
			Clusters: []string{},
		},
	}
	describeMock := &mockEKSDescribeClusterClient{}

	clients := &awsclient.ServiceClients{EKS: newMockEKSFull(listMock, describeMock, nil, nil)}
	result, err := awsclient.FetchEKSClustersPage(context.Background(), clients, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
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
			// A plain (non-typed) transport-class error — not an authorization
			// denial — so the shared DegradedDetails classifier renders the
			// neutral "details unavailable" row (denied is covered by the typed
			// AccessDenied cases in mwaa/transfer and the classifier contract test).
			"cluster-bad": fmt.Errorf("eks: DescribeCluster: internal server error"),
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

	// Both clusters appear: the successful one in full, the failed one as a
	// name-only degraded row (a listed cluster must never vanish).
	if len(result.Resources) != 2 {
		t.Fatalf(
			"FetchEKSClustersPage must keep the denied cluster as a degraded row — got %d rows, want 2",
			len(result.Resources),
		)
	}
	byID := map[string]resource.Resource{}
	for _, r := range result.Resources {
		byID[r.ID] = r
	}
	if _, ok := byID["cluster-ok"]; !ok {
		t.Errorf("partial result must contain \"cluster-ok\", got %v", result.Resources)
	}
	bad, ok := byID["cluster-bad"]
	if !ok {
		t.Fatalf("denied cluster \"cluster-bad\" must remain as a degraded row, got %v", result.Resources)
	}
	if got := bad.Fields["status"]; got != "details unavailable" {
		t.Errorf("degraded row status = %q, want %q", got, "details unavailable")
	}
}
