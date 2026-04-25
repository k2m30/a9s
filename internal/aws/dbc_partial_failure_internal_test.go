package aws

// dbc_partial_failure_internal_test.go — internal package regression pin for
// Rule E5: when the DocDB phase of the dbc paginated fetcher succeeds but the
// RDS phase fails, the caller must receive the DocDB rows PLUS a composite error
// rather than discarding everything.
//
// Pins the fix in dbc.go: rdsErr != nil branch returns docResult.Resources
// with IsTruncated=true and NextToken="rds:" so the operator sees partial data.

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/docdb"
	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"
	"github.com/aws/aws-sdk-go-v2/service/rds"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// Fake DocDBAPI — only DescribeDBClusters is exercised here; all other
// methods panic to catch accidental calls.
// ---------------------------------------------------------------------------

type fakeDocDBClusterClient struct {
	clusterOut *docdb.DescribeDBClustersOutput
	clusterErr error
}

func (f *fakeDocDBClusterClient) DescribeDBClusters(_ context.Context, _ *docdb.DescribeDBClustersInput, _ ...func(*docdb.Options)) (*docdb.DescribeDBClustersOutput, error) {
	return f.clusterOut, f.clusterErr
}
func (f *fakeDocDBClusterClient) DescribeDBClusterSnapshots(_ context.Context, _ *docdb.DescribeDBClusterSnapshotsInput, _ ...func(*docdb.Options)) (*docdb.DescribeDBClusterSnapshotsOutput, error) {
	panic("DescribeDBClusterSnapshots should not be called in dbc partial-failure tests")
}
func (f *fakeDocDBClusterClient) DescribeDBSubnetGroups(_ context.Context, _ *docdb.DescribeDBSubnetGroupsInput, _ ...func(*docdb.Options)) (*docdb.DescribeDBSubnetGroupsOutput, error) {
	panic("DescribeDBSubnetGroups should not be called in dbc partial-failure tests")
}
func (f *fakeDocDBClusterClient) DescribePendingMaintenanceActions(_ context.Context, _ *docdb.DescribePendingMaintenanceActionsInput, _ ...func(*docdb.Options)) (*docdb.DescribePendingMaintenanceActionsOutput, error) {
	panic("DescribePendingMaintenanceActions should not be called in dbc partial-failure tests")
}

// ---------------------------------------------------------------------------
// Fake RDSAPI — only DescribeDBClusters is exercised here; all other
// methods panic to catch accidental calls.
// ---------------------------------------------------------------------------

type fakeRDSClusterErrClient struct {
	clusterErr error
}

func (f *fakeRDSClusterErrClient) DescribeDBInstances(_ context.Context, _ *rds.DescribeDBInstancesInput, _ ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error) {
	panic("DescribeDBInstances should not be called in dbc partial-failure tests")
}
func (f *fakeRDSClusterErrClient) DescribeDBSnapshots(_ context.Context, _ *rds.DescribeDBSnapshotsInput, _ ...func(*rds.Options)) (*rds.DescribeDBSnapshotsOutput, error) {
	panic("DescribeDBSnapshots should not be called in dbc partial-failure tests")
}
func (f *fakeRDSClusterErrClient) DescribeEvents(_ context.Context, _ *rds.DescribeEventsInput, _ ...func(*rds.Options)) (*rds.DescribeEventsOutput, error) {
	panic("DescribeEvents should not be called in dbc partial-failure tests")
}
func (f *fakeRDSClusterErrClient) DescribePendingMaintenanceActions(_ context.Context, _ *rds.DescribePendingMaintenanceActionsInput, _ ...func(*rds.Options)) (*rds.DescribePendingMaintenanceActionsOutput, error) {
	panic("DescribePendingMaintenanceActions should not be called in dbc partial-failure tests")
}
func (f *fakeRDSClusterErrClient) DescribeDBSubnetGroups(_ context.Context, _ *rds.DescribeDBSubnetGroupsInput, _ ...func(*rds.Options)) (*rds.DescribeDBSubnetGroupsOutput, error) {
	panic("DescribeDBSubnetGroups should not be called in dbc partial-failure tests")
}
func (f *fakeRDSClusterErrClient) DescribeDBClusters(_ context.Context, _ *rds.DescribeDBClustersInput, _ ...func(*rds.Options)) (*rds.DescribeDBClustersOutput, error) {
	return nil, f.clusterErr
}
func (f *fakeRDSClusterErrClient) DescribeDBClusterSnapshots(_ context.Context, _ *rds.DescribeDBClusterSnapshotsInput, _ ...func(*rds.Options)) (*rds.DescribeDBClusterSnapshotsOutput, error) {
	panic("DescribeDBClusterSnapshots should not be called in dbc partial-failure tests")
}

// ---------------------------------------------------------------------------
// Test
// ---------------------------------------------------------------------------

// TestRegisterPaginatedDBC_RDSError_PreservesDocDBRows pins Rule E5:
// when the DocDB phase returns N rows (no more pages) and the RDS phase
// returns an error, the registered "dbc" paginated fetcher must:
//
//   - return exactly the DocDB rows (not an empty result)
//   - return a non-nil error whose message contains the composite sentinel
//   - set IsTruncated=true (signals the operator that data is incomplete)
//   - set NextToken="rds:" (allows a retry of the RDS side)
func TestRegisterPaginatedDBC_RDSError_PreservesDocDBRows(t *testing.T) {
	const wantLen = 2
	const wantErrSubstr = "dbc: RDS-side cluster fetch failed"

	docdbFake := &fakeDocDBClusterClient{
		clusterOut: &docdb.DescribeDBClustersOutput{
			DBClusters: []docdbtypes.DBCluster{
				{
					DBClusterIdentifier:   aws.String("docdb-cluster-alpha"),
					Status:                aws.String("available"),
					Engine:                aws.String("docdb"),
					BackupRetentionPeriod: aws.Int32(7),
					DeletionProtection:    aws.Bool(true),
					StorageEncrypted:      aws.Bool(true),
				},
				{
					DBClusterIdentifier:   aws.String("docdb-cluster-beta"),
					Status:                aws.String("available"),
					Engine:                aws.String("docdb"),
					BackupRetentionPeriod: aws.Int32(14),
					DeletionProtection:    aws.Bool(true),
					StorageEncrypted:      aws.Bool(true),
				},
			},
			// Marker nil → IsTruncated=false, so fetcher proceeds to RDS phase.
			Marker: nil,
		},
	}

	rdsFake := &fakeRDSClusterErrClient{
		clusterErr: errors.New("AccessDenied: User is not authorized to perform rds:DescribeDBClusters"),
	}

	clients := &ServiceClients{
		DocDB: docdbFake,
		RDS:   rdsFake,
	}

	fetcher := resource.GetPaginatedFetcher("dbc")
	if fetcher == nil {
		t.Fatal("GetPaginatedFetcher(\"dbc\") returned nil — is dbc.go compiled into this package?")
	}

	result, err := fetcher(context.Background(), clients, "")

	// Must return a non-nil error.
	if err == nil {
		t.Fatal("expected a non-nil error when RDS phase fails, got nil")
	}
	if !strings.Contains(err.Error(), wantErrSubstr) {
		t.Errorf("error message = %q, want it to contain %q", err.Error(), wantErrSubstr)
	}

	// DocDB rows must be preserved (Rule E5: no silent discard).
	if got := len(result.Resources); got != wantLen {
		t.Errorf("len(result.Resources) = %d, want %d (DocDB rows must be preserved)", got, wantLen)
	}

	// Verify identity of the returned rows (catches a bug where rows are returned
	// but from the wrong source or duplicated).
	wantIDs := []string{"docdb-cluster-alpha", "docdb-cluster-beta"}
	for i, want := range wantIDs {
		if got := result.Resources[i].ID; got != want {
			t.Errorf("result.Resources[%d].ID = %q, want %q", i, got, want)
		}
	}

	// Pagination must signal that more data may exist (IsTruncated=true).
	if result.Pagination == nil {
		t.Fatal("result.Pagination is nil, want non-nil pagination metadata")
	}
	if !result.Pagination.IsTruncated {
		t.Errorf("result.Pagination.IsTruncated = false, want true (RDS side is still outstanding)")
	}
	if result.Pagination.NextToken != "rds:" {
		t.Errorf("result.Pagination.NextToken = %q, want %q", result.Pagination.NextToken, "rds:")
	}
}
