package aws

// dbc_snap_partial_failure_internal_test.go — internal package regression pin for
// Rule E5: when the DocDB phase of the dbc-snap paginated fetcher succeeds but the
// RDS phase fails, the caller must receive the DocDB rows PLUS a composite error
// rather than discarding everything.
//
// Pins the fix in dbc_snap.go: rdsErr != nil branch returns docResult.Resources
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
// Fake DocDBAPI — only DescribeDBClusterSnapshots is exercised here; all other
// methods panic to catch accidental calls.
// ---------------------------------------------------------------------------

type fakeDocDBSnapClient struct {
	snapOut *docdb.DescribeDBClusterSnapshotsOutput
	snapErr error
}

func (f *fakeDocDBSnapClient) DescribeDBClusters(_ context.Context, _ *docdb.DescribeDBClustersInput, _ ...func(*docdb.Options)) (*docdb.DescribeDBClustersOutput, error) {
	panic("DescribeDBClusters should not be called in dbc-snap partial-failure tests")
}
func (f *fakeDocDBSnapClient) DescribeDBClusterSnapshots(_ context.Context, _ *docdb.DescribeDBClusterSnapshotsInput, _ ...func(*docdb.Options)) (*docdb.DescribeDBClusterSnapshotsOutput, error) {
	return f.snapOut, f.snapErr
}
func (f *fakeDocDBSnapClient) DescribeDBSubnetGroups(_ context.Context, _ *docdb.DescribeDBSubnetGroupsInput, _ ...func(*docdb.Options)) (*docdb.DescribeDBSubnetGroupsOutput, error) {
	panic("DescribeDBSubnetGroups should not be called in dbc-snap partial-failure tests")
}
func (f *fakeDocDBSnapClient) DescribePendingMaintenanceActions(_ context.Context, _ *docdb.DescribePendingMaintenanceActionsInput, _ ...func(*docdb.Options)) (*docdb.DescribePendingMaintenanceActionsOutput, error) {
	panic("DescribePendingMaintenanceActions should not be called in dbc-snap partial-failure tests")
}

// ---------------------------------------------------------------------------
// Fake RDSAPI — only DescribeDBClusterSnapshots is exercised here; all other
// methods panic to catch accidental calls.
// ---------------------------------------------------------------------------

type fakeRDSSnapErrClient struct {
	snapErr error
}

func (f *fakeRDSSnapErrClient) DescribeDBInstances(_ context.Context, _ *rds.DescribeDBInstancesInput, _ ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error) {
	panic("DescribeDBInstances should not be called in dbc-snap partial-failure tests")
}
func (f *fakeRDSSnapErrClient) DescribeDBSnapshots(_ context.Context, _ *rds.DescribeDBSnapshotsInput, _ ...func(*rds.Options)) (*rds.DescribeDBSnapshotsOutput, error) {
	panic("DescribeDBSnapshots should not be called in dbc-snap partial-failure tests")
}
func (f *fakeRDSSnapErrClient) DescribeEvents(_ context.Context, _ *rds.DescribeEventsInput, _ ...func(*rds.Options)) (*rds.DescribeEventsOutput, error) {
	panic("DescribeEvents should not be called in dbc-snap partial-failure tests")
}
func (f *fakeRDSSnapErrClient) DescribePendingMaintenanceActions(_ context.Context, _ *rds.DescribePendingMaintenanceActionsInput, _ ...func(*rds.Options)) (*rds.DescribePendingMaintenanceActionsOutput, error) {
	panic("DescribePendingMaintenanceActions should not be called in dbc-snap partial-failure tests")
}
func (f *fakeRDSSnapErrClient) DescribeDBSubnetGroups(_ context.Context, _ *rds.DescribeDBSubnetGroupsInput, _ ...func(*rds.Options)) (*rds.DescribeDBSubnetGroupsOutput, error) {
	panic("DescribeDBSubnetGroups should not be called in dbc-snap partial-failure tests")
}
func (f *fakeRDSSnapErrClient) DescribeDBClusters(_ context.Context, _ *rds.DescribeDBClustersInput, _ ...func(*rds.Options)) (*rds.DescribeDBClustersOutput, error) {
	panic("DescribeDBClusters should not be called in dbc-snap partial-failure tests")
}
func (f *fakeRDSSnapErrClient) DescribeDBClusterSnapshots(_ context.Context, _ *rds.DescribeDBClusterSnapshotsInput, _ ...func(*rds.Options)) (*rds.DescribeDBClusterSnapshotsOutput, error) {
	return nil, f.snapErr
}

// ---------------------------------------------------------------------------
// Test
// ---------------------------------------------------------------------------

// TestRegisterPaginatedDBCSnap_RDSError_PreservesDocDBRows pins Rule E5:
// when the DocDB phase returns N rows (no more pages) and the RDS phase
// returns an error, the registered "dbc-snap" paginated fetcher must:
//
//   - return exactly the DocDB rows (not an empty result)
//   - return a non-nil error whose message contains the composite sentinel
//   - set IsTruncated=true (signals the operator that data is incomplete)
//   - set NextToken="rds:" (allows a retry of the RDS side)
func TestRegisterPaginatedDBCSnap_RDSError_PreservesDocDBRows(t *testing.T) {
	const wantLen = 2
	const wantErrSubstr = "dbc-snap: RDS-side cluster snapshot fetch failed"

	docdbFake := &fakeDocDBSnapClient{
		snapOut: &docdb.DescribeDBClusterSnapshotsOutput{
			DBClusterSnapshots: []docdbtypes.DBClusterSnapshot{
				{
					DBClusterSnapshotIdentifier: aws.String("snap-docdb-1"),
					DBClusterIdentifier:         aws.String("cluster-a"),
					Status:                      aws.String("available"),
					Engine:                      aws.String("docdb"),
				},
				{
					DBClusterSnapshotIdentifier: aws.String("snap-docdb-2"),
					DBClusterIdentifier:         aws.String("cluster-b"),
					Status:                      aws.String("available"),
					Engine:                      aws.String("docdb"),
				},
			},
			// Marker nil → IsTruncated=false, so fetcher proceeds to RDS phase.
			Marker: nil,
		},
	}

	rdsFake := &fakeRDSSnapErrClient{
		snapErr: errors.New("AccessDenied: User is not authorized to perform rds:DescribeDBClusterSnapshots"),
	}

	clients := &ServiceClients{
		DocDB: docdbFake,
		RDS:   rdsFake,
	}

	fetcher := resource.GetPaginatedFetcher("dbc-snap")
	if fetcher == nil {
		t.Fatal("GetPaginatedFetcher(\"dbc-snap\") returned nil — is dbc_snap.go compiled into this package?")
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
	wantIDs := []string{"snap-docdb-1", "snap-docdb-2"}
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
