package unit

// DescribeDBClusters also returns Neptune and DocumentDB clusters (see the
// rds DescribeDBClusters API docstring); dbc lists Aurora engines and the
// Multi-AZ "mysql"/"postgres" cluster engines.

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
)

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

func buildRDSCluster(id, engine string) rdstypes.DBCluster {
	return rdstypes.DBCluster{
		DBClusterIdentifier: aws.String(id),
		Engine:              aws.String(engine),
		Status:              aws.String("available"),
		DeletionProtection:  aws.Bool(true),
		StorageEncrypted:    aws.Bool(true),
	}
}

func TestFetchRDSDBClustersPage_FiltersNeptune(t *testing.T) {
	mock := &mockRDSClusterPageClient{
		clusters: []rdstypes.DBCluster{
			buildRDSCluster("aurora-prod", "aurora-postgresql"),
			buildRDSCluster("neptune-prod", "neptune"),
		},
	}

	result, err := awsclient.FetchRDSDBClustersPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

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

// DocumentDB clusters are listed by the DocDB SDK fetcher.
func TestFetchRDSDBClustersPage_SkipsDocDB(t *testing.T) {
	mock := &mockRDSClusterPageClient{
		clusters: []rdstypes.DBCluster{
			buildRDSCluster("aurora-prod", "aurora-mysql"),
			buildRDSCluster("docdb-prod", "docdb"),
		},
	}

	result, err := awsclient.FetchRDSDBClustersPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

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

func TestFetchRDSDBClustersPage_MixedEngines(t *testing.T) {
	mock := &mockRDSClusterPageClient{
		clusters: []rdstypes.DBCluster{
			buildRDSCluster("aurora-pg-prod", "aurora-postgresql"),
			buildRDSCluster("neptune-graph", "neptune"),
			buildRDSCluster("aurora-mysql-staging", "aurora-mysql"),
			buildRDSCluster("docdb-app", "docdb"),
		},
	}

	result, err := awsclient.FetchRDSDBClustersPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

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
