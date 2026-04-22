// Package fakes provides in-process fake implementations of AWS service
// interfaces for use in demo mode and tests.
package fakes

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/docdb"
	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"

	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
)

// DocDBFake implements aws.DocDBAPI against fixture data loaded at construction time.
type DocDBFake struct {
	fix *fixtures.DBCFixtures
}

// NewDocDB constructs a DocDBFake backed by fixture data from the fixtures package.
func NewDocDB() *DocDBFake {
	return &DocDBFake{fix: fixtures.NewDBCFixtures()}
}

func (f *DocDBFake) DescribeDBClusters(_ context.Context, _ *docdb.DescribeDBClustersInput, _ ...func(*docdb.Options)) (*docdb.DescribeDBClustersOutput, error) {
	return &docdb.DescribeDBClustersOutput{DBClusters: f.fix.DBClusters}, nil
}

func (f *DocDBFake) DescribeDBClusterSnapshots(_ context.Context, _ *docdb.DescribeDBClusterSnapshotsInput, _ ...func(*docdb.Options)) (*docdb.DescribeDBClusterSnapshotsOutput, error) {
	return &docdb.DescribeDBClusterSnapshotsOutput{DBClusterSnapshots: f.fix.DBClusterSnapshots}, nil
}

// DescribeDBSubnetGroups returns the subnet group matching DBSubnetGroupName when set,
// or all subnet groups when no filter is provided.
func (f *DocDBFake) DescribeDBSubnetGroups(_ context.Context, in *docdb.DescribeDBSubnetGroupsInput, _ ...func(*docdb.Options)) (*docdb.DescribeDBSubnetGroupsOutput, error) {
	if in == nil || in.DBSubnetGroupName == nil || *in.DBSubnetGroupName == "" {
		return &docdb.DescribeDBSubnetGroupsOutput{DBSubnetGroups: f.fix.DBSubnetGroups}, nil
	}
	name := *in.DBSubnetGroupName
	var matched []docdbtypes.DBSubnetGroup
	for _, sg := range f.fix.DBSubnetGroups {
		if sg.DBSubnetGroupName != nil && *sg.DBSubnetGroupName == name {
			matched = append(matched, sg)
		}
	}
	return &docdb.DescribeDBSubnetGroupsOutput{DBSubnetGroups: matched}, nil
}

// DescribePendingMaintenanceActions returns all pending maintenance actions from fixtures.
func (f *DocDBFake) DescribePendingMaintenanceActions(_ context.Context, _ *docdb.DescribePendingMaintenanceActionsInput, _ ...func(*docdb.Options)) (*docdb.DescribePendingMaintenanceActionsOutput, error) {
	return &docdb.DescribePendingMaintenanceActionsOutput{
		PendingMaintenanceActions: f.fix.PendingMaintenanceActions,
	}, nil
}
