// Package fakes provides in-process fake implementations of AWS service
// interfaces for use in demo mode and tests.
package fakes

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/docdb"

	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
)

// DocDBFake implements aws.DocDBAPI against fixture data loaded at construction time.
type DocDBFake struct {
	fix *fixtures.DocDBFixtures
}

// NewDocDB constructs a DocDBFake backed by fixture data from the fixtures package.
func NewDocDB() *DocDBFake {
	return &DocDBFake{fix: fixtures.NewDocDBFixtures()}
}

func (f *DocDBFake) DescribeDBClusters(_ context.Context, _ *docdb.DescribeDBClustersInput, _ ...func(*docdb.Options)) (*docdb.DescribeDBClustersOutput, error) {
	return &docdb.DescribeDBClustersOutput{DBClusters: f.fix.DBClusters}, nil
}

func (f *DocDBFake) DescribeDBClusterSnapshots(_ context.Context, _ *docdb.DescribeDBClusterSnapshotsInput, _ ...func(*docdb.Options)) (*docdb.DescribeDBClusterSnapshotsOutput, error) {
	return &docdb.DescribeDBClusterSnapshotsOutput{DBClusterSnapshots: f.fix.DBClusterSnapshots}, nil
}
