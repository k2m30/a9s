// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// Package fakes provides in-process fake implementations of AWS service
// interfaces for use in demo mode and tests.
package fakes

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	"github.com/k2m30/a9s/v3/core/demo/fixtures"
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

// DescribePendingMaintenanceActions returns the maintenance actions from fixture data.
func (f *RDSFake) DescribePendingMaintenanceActions(_ context.Context, _ *rds.DescribePendingMaintenanceActionsInput, _ ...func(*rds.Options)) (*rds.DescribePendingMaintenanceActionsOutput, error) {
	dbi := fixtures.NewDBIFixtures()
	return &rds.DescribePendingMaintenanceActionsOutput{
		PendingMaintenanceActions: dbi.PendingMaintenanceActions,
	}, nil
}

// DescribeDBSubnetGroups returns matching subnet groups from fixture data.
// When DBSubnetGroupName is set in the input, it filters to that single group.
// This supports the dbc related checker's Aurora-side subnet-group resolution
// (c.RDS.DescribeDBSubnetGroups for rdstypes.DBCluster shapes).
func (f *RDSFake) DescribeDBSubnetGroups(_ context.Context, in *rds.DescribeDBSubnetGroupsInput, _ ...func(*rds.Options)) (*rds.DescribeDBSubnetGroupsOutput, error) {
	if in != nil && in.DBSubnetGroupName != nil && *in.DBSubnetGroupName != "" {
		name := *in.DBSubnetGroupName
		for _, sg := range f.fix.DBSubnetGroups {
			if sg.DBSubnetGroupName != nil && *sg.DBSubnetGroupName == name {
				return &rds.DescribeDBSubnetGroupsOutput{DBSubnetGroups: []rdstypes.DBSubnetGroup{sg}}, nil
			}
		}
		return &rds.DescribeDBSubnetGroupsOutput{}, nil
	}
	return &rds.DescribeDBSubnetGroupsOutput{DBSubnetGroups: f.fix.DBSubnetGroups}, nil
}

// DescribeDBClusters returns the Aurora + Multi-AZ DB clusters from fixture data.
func (f *RDSFake) DescribeDBClusters(_ context.Context, _ *rds.DescribeDBClustersInput, _ ...func(*rds.Options)) (*rds.DescribeDBClustersOutput, error) {
	return &rds.DescribeDBClustersOutput{DBClusters: f.fix.DBClusters}, nil
}

// DescribeDBClusterSnapshots returns the Aurora + Multi-AZ DB cluster snapshots from fixture data.
func (f *RDSFake) DescribeDBClusterSnapshots(_ context.Context, _ *rds.DescribeDBClusterSnapshotsInput, _ ...func(*rds.Options)) (*rds.DescribeDBClusterSnapshotsOutput, error) {
	return &rds.DescribeDBClusterSnapshotsOutput{DBClusterSnapshots: f.fix.DBClusterSnapshots}, nil
}

// DescribeDBEngineVersions answers the engine-deprecation check. Every
// version except fixtures.DBIDeprecatedEngineVersion is available; that one
// comes back with Status "deprecated".
func (f *RDSFake) DescribeDBEngineVersions(_ context.Context, in *rds.DescribeDBEngineVersionsInput, _ ...func(*rds.Options)) (*rds.DescribeDBEngineVersionsOutput, error) {
	engine, version := "", ""
	if in != nil {
		if in.Engine != nil {
			engine = *in.Engine
		}
		if in.EngineVersion != nil {
			version = *in.EngineVersion
		}
	}
	status := "available"
	if version == fixtures.DBIDeprecatedEngineVersion {
		status = "deprecated"
	}
	return &rds.DescribeDBEngineVersionsOutput{
		DBEngineVersions: []rdstypes.DBEngineVersion{{
			Engine:        &engine,
			EngineVersion: &version,
			Status:        &status,
		}},
	}, nil
}

// DescribeDBSnapshotAttributes reports the restore grant on a DB snapshot.
// Only fixtures.DBISnapPublic is shared with the "all" group.
func (f *RDSFake) DescribeDBSnapshotAttributes(_ context.Context, in *rds.DescribeDBSnapshotAttributesInput, _ ...func(*rds.Options)) (*rds.DescribeDBSnapshotAttributesOutput, error) {
	id := ""
	if in != nil && in.DBSnapshotIdentifier != nil {
		id = *in.DBSnapshotIdentifier
	}
	return &rds.DescribeDBSnapshotAttributesOutput{
		DBSnapshotAttributesResult: &rdstypes.DBSnapshotAttributesResult{
			DBSnapshotIdentifier: &id,
			DBSnapshotAttributes: []rdstypes.DBSnapshotAttribute{{
				AttributeName:   restoreAttributeName(),
				AttributeValues: restoreAttributeValues(id == fixtures.DBISnapPublic),
			}},
		},
	}, nil
}

// DescribeDBClusterSnapshotAttributes reports the restore grant on an Aurora
// cluster snapshot. Only fixtures.DBCSnapPublic is shared with the "all" group.
func (f *RDSFake) DescribeDBClusterSnapshotAttributes(_ context.Context, in *rds.DescribeDBClusterSnapshotAttributesInput, _ ...func(*rds.Options)) (*rds.DescribeDBClusterSnapshotAttributesOutput, error) {
	id := ""
	if in != nil && in.DBClusterSnapshotIdentifier != nil {
		id = *in.DBClusterSnapshotIdentifier
	}
	return &rds.DescribeDBClusterSnapshotAttributesOutput{
		DBClusterSnapshotAttributesResult: &rdstypes.DBClusterSnapshotAttributesResult{
			DBClusterSnapshotIdentifier: &id,
			DBClusterSnapshotAttributes: []rdstypes.DBClusterSnapshotAttribute{{
				AttributeName:   restoreAttributeName(),
				AttributeValues: restoreAttributeValues(id == fixtures.DBCSnapPublic),
			}},
		},
	}, nil
}

// restoreAttributeName is the attribute AWS uses to record who may restore a
// snapshot.
func restoreAttributeName() *string {
	name := "restore"
	return &name
}

// restoreAttributeValues returns the restore grant list: the "all" group when
// the snapshot is shared with every account, otherwise an empty list (AWS's
// representation of a snapshot shared with nobody).
func restoreAttributeValues(public bool) []string {
	if public {
		return []string{"all"}
	}
	return nil
}
