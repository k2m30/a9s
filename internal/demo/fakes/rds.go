// Package fakes provides in-process fake implementations of AWS service
// interfaces for use in demo mode and tests.
package fakes

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/rds"

	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
)

// RDSFake implements aws.RDSAPI against fixture data loaded at construction time.
type RDSFake struct {
	fix *fixtures.RDSFixtures
}

// NewRDS constructs an RDSFake backed by fixture data from the fixtures package.
func NewRDS() *RDSFake {
	return &RDSFake{fix: fixtures.NewRDSFixtures()}
}

func (f *RDSFake) DescribeDBInstances(_ context.Context, _ *rds.DescribeDBInstancesInput, _ ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error) {
	return &rds.DescribeDBInstancesOutput{DBInstances: f.fix.DBInstances}, nil
}

func (f *RDSFake) DescribeDBSnapshots(_ context.Context, _ *rds.DescribeDBSnapshotsInput, _ ...func(*rds.Options)) (*rds.DescribeDBSnapshotsOutput, error) {
	return &rds.DescribeDBSnapshotsOutput{DBSnapshots: f.fix.DBSnapshots}, nil
}

func (f *RDSFake) DescribeEvents(_ context.Context, _ *rds.DescribeEventsInput, _ ...func(*rds.Options)) (*rds.DescribeEventsOutput, error) {
	return &rds.DescribeEventsOutput{Events: f.fix.Events}, nil
}

// DescribePendingMaintenanceActions is a stub for the Wave 2 enrichment interface.
func (f *RDSFake) DescribePendingMaintenanceActions(_ context.Context, _ *rds.DescribePendingMaintenanceActionsInput, _ ...func(*rds.Options)) (*rds.DescribePendingMaintenanceActionsOutput, error) {
	return &rds.DescribePendingMaintenanceActionsOutput{}, nil
}
