package aws

// dbc_subnet_group_internal_test.go — internal package tests for dbcSubnetGroup,
// dbcRDSSubnetGroup, and dbcDocDBSubnetGroup.
//
// dbcSubnetGroup is unexported and dispatches to one of two engine-specific
// helpers based on the RawStruct shape:
//   - rdstypes.DBCluster  → dbcRDSSubnetGroup  (Aurora / Multi-AZ; RDS API)
//   - docdb_types.DBCluster → dbcDocDBSubnetGroup (DocumentDB; DocDB API)
//
// These tests verify the dispatch and the nil-client short-circuit, using
// minimal fake implementations of the DocDBAPI and RDSAPI interfaces.

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/docdb"
	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// Minimal fake DocDBAPI — satisfies DocDBAPI with only DescribeDBSubnetGroups
// implemented; all other methods panic (should never be called in these tests).
// ---------------------------------------------------------------------------

type fakeDocDBClient struct {
	subnetOut *docdb.DescribeDBSubnetGroupsOutput
	subnetErr error
}

func (f *fakeDocDBClient) DescribeDBClusters(_ context.Context, _ *docdb.DescribeDBClustersInput, _ ...func(*docdb.Options)) (*docdb.DescribeDBClustersOutput, error) {
	panic("DescribeDBClusters should not be called in subnet-group tests")
}
func (f *fakeDocDBClient) DescribeDBClusterSnapshots(_ context.Context, _ *docdb.DescribeDBClusterSnapshotsInput, _ ...func(*docdb.Options)) (*docdb.DescribeDBClusterSnapshotsOutput, error) {
	panic("DescribeDBClusterSnapshots should not be called in subnet-group tests")
}
func (f *fakeDocDBClient) DescribeDBSubnetGroups(_ context.Context, _ *docdb.DescribeDBSubnetGroupsInput, _ ...func(*docdb.Options)) (*docdb.DescribeDBSubnetGroupsOutput, error) {
	return f.subnetOut, f.subnetErr
}
func (f *fakeDocDBClient) DescribePendingMaintenanceActions(_ context.Context, _ *docdb.DescribePendingMaintenanceActionsInput, _ ...func(*docdb.Options)) (*docdb.DescribePendingMaintenanceActionsOutput, error) {
	panic("DescribePendingMaintenanceActions should not be called in subnet-group tests")
}

// ---------------------------------------------------------------------------
// Minimal fake RDSAPI — satisfies RDSAPI with only DescribeDBSubnetGroups
// implemented; all other methods panic.
// ---------------------------------------------------------------------------

type fakeRDSSubnetGroupClient struct {
	subnetOut *rds.DescribeDBSubnetGroupsOutput
	subnetErr error
}

func (f *fakeRDSSubnetGroupClient) DescribeDBInstances(_ context.Context, _ *rds.DescribeDBInstancesInput, _ ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error) {
	panic("DescribeDBInstances should not be called in subnet-group tests")
}
func (f *fakeRDSSubnetGroupClient) DescribeDBSnapshots(_ context.Context, _ *rds.DescribeDBSnapshotsInput, _ ...func(*rds.Options)) (*rds.DescribeDBSnapshotsOutput, error) {
	panic("DescribeDBSnapshots should not be called in subnet-group tests")
}
func (f *fakeRDSSubnetGroupClient) DescribeEvents(_ context.Context, _ *rds.DescribeEventsInput, _ ...func(*rds.Options)) (*rds.DescribeEventsOutput, error) {
	panic("DescribeEvents should not be called in subnet-group tests")
}
func (f *fakeRDSSubnetGroupClient) DescribePendingMaintenanceActions(_ context.Context, _ *rds.DescribePendingMaintenanceActionsInput, _ ...func(*rds.Options)) (*rds.DescribePendingMaintenanceActionsOutput, error) {
	panic("DescribePendingMaintenanceActions should not be called in subnet-group tests")
}
func (f *fakeRDSSubnetGroupClient) DescribeDBSubnetGroups(_ context.Context, _ *rds.DescribeDBSubnetGroupsInput, _ ...func(*rds.Options)) (*rds.DescribeDBSubnetGroupsOutput, error) {
	return f.subnetOut, f.subnetErr
}
func (f *fakeRDSSubnetGroupClient) DescribeDBClusters(_ context.Context, _ *rds.DescribeDBClustersInput, _ ...func(*rds.Options)) (*rds.DescribeDBClustersOutput, error) {
	panic("DescribeDBClusters should not be called in subnet-group tests")
}
func (f *fakeRDSSubnetGroupClient) DescribeDBClusterSnapshots(_ context.Context, _ *rds.DescribeDBClusterSnapshotsInput, _ ...func(*rds.Options)) (*rds.DescribeDBClusterSnapshotsOutput, error) {
	panic("DescribeDBClusterSnapshots should not be called in subnet-group tests")
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestDbcSubnetGroup_DocDBShape verifies dbcSubnetGroup dispatches to
// dbcDocDBSubnetGroup for a docdbtypes.DBCluster shape and returns the
// VpcId and Subnets from the DocDB API response.
func TestDbcSubnetGroup_DocDBShape(t *testing.T) {
	const sngName = "acme-docdb-subnet-group"
	const vpcID = "vpc-0abc1234"
	const subnetA = "subnet-aaa111"
	const subnetB = "subnet-bbb222"

	fakeDocDB := &fakeDocDBClient{
		subnetOut: &docdb.DescribeDBSubnetGroupsOutput{
			DBSubnetGroups: []docdbtypes.DBSubnetGroup{
				{
					VpcId: aws.String(vpcID),
					Subnets: []docdbtypes.Subnet{
						{SubnetIdentifier: aws.String(subnetA)},
						{SubnetIdentifier: aws.String(subnetB)},
					},
				},
			},
		},
	}

	clients := &ServiceClients{DocDB: fakeDocDB}
	res := resource.Resource{
		ID: "prod-docdb",
		RawStruct: docdbtypes.DBCluster{
			DBClusterIdentifier: aws.String("prod-docdb"),
			DBSubnetGroup:       aws.String(sngName),
		},
	}

	info := dbcSubnetGroup(context.Background(), clients, res)
	if info == nil {
		t.Fatal("dbcSubnetGroup returned nil for docdb shape with valid client")
	}
	if info.VpcId == nil || *info.VpcId != vpcID {
		t.Errorf("VpcId = %v, want %q", info.VpcId, vpcID)
	}
	if len(info.Subnets) != 2 {
		t.Fatalf("Subnets len = %d, want 2", len(info.Subnets))
	}
	if info.Subnets[0].SubnetIdentifier == nil || *info.Subnets[0].SubnetIdentifier != subnetA {
		t.Errorf("Subnets[0] = %v, want %q", info.Subnets[0].SubnetIdentifier, subnetA)
	}
}

// TestDbcSubnetGroup_RDSShape verifies dbcSubnetGroup dispatches to
// dbcRDSSubnetGroup for a rdstypes.DBCluster shape and returns the
// VpcId and Subnets from the RDS API response.
func TestDbcSubnetGroup_RDSShape(t *testing.T) {
	const sngName = "acme-aurora-subnet-group"
	const vpcID = "vpc-0def5678"
	const subnetC = "subnet-ccc333"

	fakeRDS := &fakeRDSSubnetGroupClient{
		subnetOut: &rds.DescribeDBSubnetGroupsOutput{
			DBSubnetGroups: []rdstypes.DBSubnetGroup{
				{
					VpcId: aws.String(vpcID),
					Subnets: []rdstypes.Subnet{
						{SubnetIdentifier: aws.String(subnetC)},
					},
				},
			},
		},
	}

	clients := &ServiceClients{RDS: fakeRDS}
	res := resource.Resource{
		ID: "prod-aurora",
		RawStruct: rdstypes.DBCluster{
			DBClusterIdentifier: aws.String("prod-aurora"),
			DBSubnetGroup:       aws.String(sngName),
		},
	}

	info := dbcSubnetGroup(context.Background(), clients, res)
	if info == nil {
		t.Fatal("dbcSubnetGroup returned nil for rds shape with valid client")
	}
	if info.VpcId == nil || *info.VpcId != vpcID {
		t.Errorf("VpcId = %v, want %q", info.VpcId, vpcID)
	}
	if len(info.Subnets) != 1 {
		t.Fatalf("Subnets len = %d, want 1", len(info.Subnets))
	}
	if info.Subnets[0].SubnetIdentifier == nil || *info.Subnets[0].SubnetIdentifier != subnetC {
		t.Errorf("Subnets[0] = %v, want %q", info.Subnets[0].SubnetIdentifier, subnetC)
	}
}

// TestDbcSubnetGroup_NilClient_DocDB verifies dbcSubnetGroup returns nil when
// the DocDB client is nil (for a docdbtypes.DBCluster shape).
func TestDbcSubnetGroup_NilClient_DocDB(t *testing.T) {
	res := resource.Resource{
		ID: "prod-docdb",
		RawStruct: docdbtypes.DBCluster{
			DBClusterIdentifier: aws.String("prod-docdb"),
			DBSubnetGroup:       aws.String("some-sng"),
		},
	}
	// Pass nil clients — dbcDocDBSubnetGroup must short-circuit.
	info := dbcSubnetGroup(context.Background(), nil, res)
	if info != nil {
		t.Errorf("dbcSubnetGroup = %+v, want nil when clients is nil", info)
	}
}

// TestDbcSubnetGroup_NilClient_RDS verifies dbcSubnetGroup returns nil when
// the RDS client is nil (for a rdstypes.DBCluster shape).
func TestDbcSubnetGroup_NilClient_RDS(t *testing.T) {
	res := resource.Resource{
		ID: "prod-aurora",
		RawStruct: rdstypes.DBCluster{
			DBClusterIdentifier: aws.String("prod-aurora"),
			DBSubnetGroup:       aws.String("some-sng"),
		},
	}
	// Pass nil clients — dbcRDSSubnetGroup must short-circuit.
	info := dbcSubnetGroup(context.Background(), nil, res)
	if info != nil {
		t.Errorf("dbcSubnetGroup = %+v, want nil when clients is nil", info)
	}
}

// TestDbcSubnetGroup_UnrecognisedShape verifies dbcSubnetGroup returns nil for
// an unrecognised RawStruct type (neither docdb nor rds cluster shape).
func TestDbcSubnetGroup_UnrecognisedShape(t *testing.T) {
	res := resource.Resource{
		ID:        "unknown",
		RawStruct: "not-a-cluster",
	}
	info := dbcSubnetGroup(context.Background(), nil, res)
	if info != nil {
		t.Errorf("dbcSubnetGroup = %+v, want nil for unrecognised shape", info)
	}
}

// TestDbcSubnetGroup_NoSubnetGroupName_DocDB verifies dbcSubnetGroup returns nil
// when the DBSubnetGroup name is absent on a docdbtypes.DBCluster.
func TestDbcSubnetGroup_NoSubnetGroupName_DocDB(t *testing.T) {
	fakeDocDB := &fakeDocDBClient{}
	clients := &ServiceClients{DocDB: fakeDocDB}
	res := resource.Resource{
		ID:        "prod-docdb-no-sng",
		RawStruct: docdbtypes.DBCluster{DBClusterIdentifier: aws.String("prod-docdb-no-sng"), DBSubnetGroup: nil},
	}
	info := dbcSubnetGroup(context.Background(), clients, res)
	if info != nil {
		t.Errorf("dbcSubnetGroup = %+v, want nil when DBSubnetGroup name is absent", info)
	}
}
