package unit

// aws_dbc_rds_test.go — Regression pins for Issue 4 (P2):
// RDS DescribeDBClusters returns Neptune / DocDB rows unfiltered.
//
// Bug location: internal/aws/dbc_rds.go:121-186 (FetchRDSDBClustersPage).
// The loop iterates output.DBClusters and emits ALL clusters as dbc resources.
// Per AWS SDK docstring (rds@v1.116.3/api_op_DescribeDBClusters.go:19-28),
// this API may return Neptune and DocDB rows alongside Aurora/Multi-AZ rows.
//
// Impact:
//   - Neptune rows surface as unsupported "dbc" entries (Engine="neptune")
//   - DocDB rows appear duplicated (already fetched from the DocDB SDK side)
//
// Fix contract: FetchRDSDBClustersPage must filter to Aurora / Multi-AZ engines.
// Specifically: keep engines that start with "aurora" (aurora-mysql, aurora-postgresql)
// or are "mysql"/"postgres" (Multi-AZ DB clusters per AWS SDK docstring); skip
// "neptune", "docdb", and any other non-Aurora engine.
//
// Test strategy: each test builds a fake RDS API returning a mix of engines,
// then calls FetchRDSDBClustersPage directly and asserts on the returned resources.
// Tests FAIL today because the function emits all clusters.

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
)

// mockRDSClusterPageClient implements RDSDescribeDBClustersAPI for a fixed page.
type mockRDSClusterPageClient struct {
	clusters []rdstypes.DBCluster
	marker   *string
	err      error
}

func (m *mockRDSClusterPageClient) DescribeDBClusters(
	_ context.Context,
	_ *rds.DescribeDBClustersInput,
	_ ...func(*rds.Options),
) (*rds.DescribeDBClustersOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &rds.DescribeDBClustersOutput{
		DBClusters: m.clusters,
		Marker:     m.marker,
	}, nil
}

// buildRDSCluster is a minimal builder so test cases stay concise.
func buildRDSCluster(id, engine string) rdstypes.DBCluster {
	return rdstypes.DBCluster{
		DBClusterIdentifier: aws.String(id),
		Engine:              aws.String(engine),
		Status:              aws.String("available"),
		DeletionProtection:  aws.Bool(true),
		StorageEncrypted:    aws.Bool(true),
	}
}

// TestFetchRDSDBClustersPage_FiltersNeptune verifies that Neptune clusters are
// NOT emitted by FetchRDSDBClustersPage.
//
// FAILS today: FetchRDSDBClustersPage emits all clusters regardless of engine,
// so a "neptune" cluster appears in the result — Count=2 instead of Count=1.
func TestFetchRDSDBClustersPage_FiltersNeptune(t *testing.T) {
	mock := &mockRDSClusterPageClient{
		clusters: []rdstypes.DBCluster{
			buildRDSCluster("aurora-prod", "aurora-postgresql"),
			buildRDSCluster("neptune-prod", "neptune"), // must be filtered out
		},
	}

	result, err := awsclient.FetchRDSDBClustersPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// FAILS today: len(result.Resources) == 2 (neptune cluster included).
	if len(result.Resources) != 1 {
		t.Fatalf(
			"FetchRDSDBClustersPage: expected 1 resource (aurora-postgresql), got %d — "+
				"DBC-RDS-UNFILTERED BUG: neptune cluster must be filtered out (it is not a dbc type)",
			len(result.Resources),
		)
	}
	if result.Resources[0].ID != "aurora-prod" {
		t.Errorf("FetchRDSDBClustersPage: expected aurora-prod, got %q", result.Resources[0].ID)
	}
}

// TestFetchRDSDBClustersPage_SkipsDocDB verifies that DocDB clusters returned by
// the RDS API are NOT emitted — they are fetched separately via the DocDB SDK.
//
// FAILS today: FetchRDSDBClustersPage emits all clusters — DocDB rows appear
// duplicated (once from DocDB SDK, once from RDS SDK).
func TestFetchRDSDBClustersPage_SkipsDocDB(t *testing.T) {
	mock := &mockRDSClusterPageClient{
		clusters: []rdstypes.DBCluster{
			buildRDSCluster("aurora-prod", "aurora-mysql"),
			buildRDSCluster("docdb-prod", "docdb"), // must be filtered out (fetched via DocDB SDK)
		},
	}

	result, err := awsclient.FetchRDSDBClustersPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// FAILS today: len(result.Resources) == 2 (docdb cluster duplicated).
	if len(result.Resources) != 1 {
		t.Fatalf(
			"FetchRDSDBClustersPage: expected 1 resource (aurora-mysql), got %d — "+
				"DBC-RDS-UNFILTERED BUG: docdb cluster must be skipped (already fetched via DocDB SDK)",
			len(result.Resources),
		)
	}
	if result.Resources[0].ID != "aurora-prod" {
		t.Errorf("FetchRDSDBClustersPage: expected aurora-prod, got %q", result.Resources[0].ID)
	}
}

// TestFetchRDSDBClustersPage_KeepsAuroraVariants verifies that all legitimate
// Aurora and Multi-AZ DB cluster engine variants are emitted.
//
// These should PASS today (the function currently emits everything). This test
// pins the allowed-engine allowlist so the filter does not accidentally drop
// legitimate clusters.
func TestFetchRDSDBClustersPage_KeepsAuroraVariants(t *testing.T) {
	allowedEngines := []struct {
		id     string
		engine string
	}{
		{"aurora-mysql-cluster", "aurora-mysql"},
		{"aurora-pg-cluster", "aurora-postgresql"},
		{"mysql-multiaz-cluster", "mysql"},       // Multi-AZ DB cluster (per AWS SDK docstring)
		{"postgres-multiaz-cluster", "postgres"}, // Multi-AZ DB cluster
	}

	clusters := make([]rdstypes.DBCluster, len(allowedEngines))
	for i, e := range allowedEngines {
		clusters[i] = buildRDSCluster(e.id, e.engine)
	}

	mock := &mockRDSClusterPageClient{clusters: clusters}

	result, err := awsclient.FetchRDSDBClustersPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Resources) != len(allowedEngines) {
		t.Fatalf(
			"FetchRDSDBClustersPage: expected %d Aurora/Multi-AZ clusters, got %d — "+
				"engine filter must keep all Aurora variants (aurora-mysql, aurora-postgresql, mysql, postgres)",
			len(allowedEngines), len(result.Resources),
		)
	}

	// Verify each expected cluster is present.
	got := make(map[string]bool, len(result.Resources))
	for _, r := range result.Resources {
		got[r.ID] = true
	}
	for _, e := range allowedEngines {
		if !got[e.id] {
			t.Errorf("FetchRDSDBClustersPage: expected cluster %q (engine=%q) not in result", e.id, e.engine)
		}
	}
}

// TestFetchRDSDBClustersPage_MixedEngines verifies the complete filtering
// scenario: aurora kept, neptune and docdb filtered.
//
// FAILS today: all 4 clusters emitted; only 2 should be.
func TestFetchRDSDBClustersPage_MixedEngines(t *testing.T) {
	mock := &mockRDSClusterPageClient{
		clusters: []rdstypes.DBCluster{
			buildRDSCluster("aurora-pg-prod", "aurora-postgresql"),
			buildRDSCluster("neptune-graph", "neptune"),   // must be filtered
			buildRDSCluster("aurora-mysql-staging", "aurora-mysql"),
			buildRDSCluster("docdb-app", "docdb"),         // must be filtered
		},
	}

	result, err := awsclient.FetchRDSDBClustersPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// FAILS today: len(result.Resources) == 4.
	if len(result.Resources) != 2 {
		t.Fatalf(
			"FetchRDSDBClustersPage: expected 2 resources (aurora variants), got %d — "+
				"DBC-RDS-UNFILTERED BUG: neptune and docdb must be filtered out",
			len(result.Resources),
		)
	}

	got := make(map[string]bool, len(result.Resources))
	for _, r := range result.Resources {
		got[r.ID] = true
	}
	for _, wantID := range []string{"aurora-pg-prod", "aurora-mysql-staging"} {
		if !got[wantID] {
			t.Errorf("FetchRDSDBClustersPage: expected cluster %q not in result; got %v", wantID, got)
		}
	}
	for _, badID := range []string{"neptune-graph", "docdb-app"} {
		if got[badID] {
			t.Errorf("FetchRDSDBClustersPage: cluster %q must be filtered out but was included", badID)
		}
	}
}
