// Package fixtures provides DocumentDB cluster fixture data for the DocDB fake.
package fixtures

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"
)

// DBCFixtures holds all DocumentDB cluster domain objects served by the fake.
type DBCFixtures struct {
	// DBClusters is the full list returned by DescribeDBClusters.
	DBClusters []docdbtypes.DBCluster
	// DBClusterSnapshots is the full list returned by DescribeDBClusterSnapshots.
	DBClusterSnapshots []docdbtypes.DBClusterSnapshot
	// DBSubnetGroups is returned by DescribeDBSubnetGroups (filtered by name).
	DBSubnetGroups []docdbtypes.DBSubnetGroup
	// PendingMaintenanceActions is returned by DescribePendingMaintenanceActions.
	PendingMaintenanceActions []docdbtypes.ResourcePendingMaintenanceActions
}

// Stable IDs and ARNs for DBC fixtures — imported by sibling fixture files.
const (
	// ProdDbcID / ProdDbcARN — healthy baseline, graph-connected.
	ProdDbcID  = "acme-docdb-prod"
	ProdDbcARN = "arn:aws:rds:us-east-1:123456789012:cluster:acme-docdb-prod"

	// ProdDbcMasterSecretARN — matches secrets.go entry for acme-docdb-prod.
	ProdDbcMasterSecretARN = "arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/docdb/acme-docdb-prod-XyZaBc"

	// ProdDbcAuroraMasterSecretARN — Secrets Manager ARN for the Aurora
	// cluster (prod-aurora-cluster) master user. Used so the Aurora dbc
	// fixture covers the dbc→secrets pivot.
	ProdDbcAuroraMasterSecretARN = "arn:aws:secretsmanager:us-east-1:123456789012:secret:rds!cluster-prod-aurora-cluster-MNOPQR"

	// MaintDbcOverdueID / MaintDbcOverdueARN — healthy + overdue maintenance.
	MaintDbcOverdueID  = "healthy-dbc-maint-overdue"
	MaintDbcOverdueARN = "arn:aws:rds:us-east-1:123456789012:cluster:healthy-dbc-maint-overdue"

	// WarnDbcNoBkpMaintID / WarnDbcNoBkpMaintARN — Wave-1 warn + overdue maintenance.
	WarnDbcNoBkpMaintID  = "warn-dbc-no-bkp-plus-maint"
	WarnDbcNoBkpMaintARN = "arn:aws:rds:us-east-1:123456789012:cluster:warn-dbc-no-bkp-plus-maint"

	// ProdDBCSnapAuroraID — Aurora cluster snapshot for prod-aurora-cluster.
	// Imported by tests/integration/scenario_related_drill_through_test.go as
	// the dbc-snap graph-root: drilling its dbc back-pivot lands on
	// prod-aurora-cluster (the Aurora dbc graph-root).
	ProdDBCSnapAuroraID  = "rds:prod-aurora-cluster-2026-04-15"
	ProdDBCSnapAuroraARN = "arn:aws:rds:us-east-1:123456789012:cluster-snapshot:rds:prod-aurora-cluster-2026-04-15"

	// ProdDBCSnapDocDBID — DocumentDB cluster snapshot for acme-docdb-prod.
	// Provides a non-Aurora dbc-snap drill-through case.
	ProdDBCSnapDocDBID  = "rds:acme-docdb-prod-2026-03-20"
	ProdDBCSnapDocDBARN = "arn:aws:rds:us-east-1:123456789012:cluster-snapshot:rds:acme-docdb-prod-2026-03-20"

	// shared internal constants
	dbcKMSKeyID = "arn:aws:kms:us-east-1:123456789012:key/a1b2c3d4-5678-90ab-cdef-111111111111"
	dbcSGID     = "sg-0ccc333333333333c"
	dbcVPCID    = "vpc-0abc123def456789a"

	// subnet IDs used in acme-docdb-subnet-group — match ec2.go fixtProdPrivateSubnetA/B.
	dbcSubnetA = "subnet-0ccc333333333333c"
	dbcSubnetB = "subnet-0ddd444444444444d"
)

// NewDBCFixtures builds and returns a fully-populated DBCFixtures struct.
func NewDBCFixtures() *DBCFixtures {
	return &DBCFixtures{
		DBClusters:                buildDBCClusters(),
		DBClusterSnapshots:        buildDBCSnapshots(),
		DBSubnetGroups:            buildDBCSubnetGroups(),
		PendingMaintenanceActions: buildDBCPendingMaintenance(),
	}
}

// dbcBaseline returns a healthy DocumentDB cluster with all fields set.
// Callers mutate specific fields to produce issue-state variants.
func dbcBaseline(id string) docdbtypes.DBCluster {
	return docdbtypes.DBCluster{
		DBClusterIdentifier:        aws.String(id),
		DBClusterArn:               aws.String("arn:aws:rds:us-east-1:123456789012:cluster:" + id),
		Engine:                     aws.String("docdb"),
		EngineVersion:              aws.String("5.0.0"),
		Status:                     aws.String("available"),
		Endpoint:                   aws.String(id + ".cluster-c9xyz123.us-east-1.docdb.amazonaws.com"),
		ReaderEndpoint:             aws.String(id + ".cluster-ro-c9xyz123.us-east-1.docdb.amazonaws.com"),
		Port:                       aws.Int32(27017),
		StorageEncrypted:           aws.Bool(true),
		KmsKeyId:                   aws.String(dbcKMSKeyID),
		DeletionProtection:         aws.Bool(true),
		BackupRetentionPeriod:      aws.Int32(7),
		PreferredMaintenanceWindow: aws.String("sun:04:00-sun:04:30"),
		DBSubnetGroup:              aws.String("acme-docdb-subnet-group"),
		VpcSecurityGroups: []docdbtypes.VpcSecurityGroupMembership{
			{VpcSecurityGroupId: aws.String(dbcSGID), Status: aws.String("active")},
		},
		DBClusterMembers: []docdbtypes.DBClusterMember{
			{DBInstanceIdentifier: aws.String(id + "-01"), IsClusterWriter: aws.Bool(true)},
			{DBInstanceIdentifier: aws.String(id + "-02"), IsClusterWriter: aws.Bool(false)},
		},
		MasterUsername:    aws.String("docdbadmin"),
		MultiAZ:           aws.Bool(true),
		ClusterCreateTime: aws.Time(mustTime("2025-04-15T10:20:00Z")),
	}
}

func buildDBCClusters() []docdbtypes.DBCluster {
	// 1. acme-docdb-prod — Healthy baseline: every §2 pivot returns ≥1 row.
	prod := dbcBaseline(ProdDbcID)
	prod.DBClusterArn = aws.String(ProdDbcARN)
	prod.DBClusterMembers = []docdbtypes.DBClusterMember{
		{DBInstanceIdentifier: aws.String("acme-docdb-prod-01"), IsClusterWriter: aws.Bool(true)},
		{DBInstanceIdentifier: aws.String("acme-docdb-prod-02"), IsClusterWriter: aws.Bool(false)},
		{DBInstanceIdentifier: aws.String("acme-docdb-prod-03"), IsClusterWriter: aws.Bool(false)},
	}
	prod.MasterUserSecret = &docdbtypes.ClusterMasterUserSecret{
		SecretArn: aws.String(ProdDbcMasterSecretARN),
	}
	prod.ClusterCreateTime = aws.Time(mustTime("2025-04-15T10:20:00Z"))

	// 2. warn-dbc-modifying — Status=modifying; all else healthy.
	modifying := dbcBaseline("warn-dbc-modifying")
	modifying.Status = aws.String("modifying")
	modifying.EngineVersion = aws.String("4.0.0")
	modifying.ClusterCreateTime = aws.Time(mustTime("2025-11-05T08:30:00Z"))

	// 3. broken-dbc-failed — Status=failed.
	failed := dbcBaseline("broken-dbc-failed")
	failed.Status = aws.String("failed")
	failed.DBClusterMembers = []docdbtypes.DBClusterMember{}
	failed.DeletionProtection = aws.Bool(false)
	failed.BackupRetentionPeriod = aws.Int32(1)
	failed.ClusterCreateTime = aws.Time(mustTime("2026-04-10T12:00:00Z"))

	// 4. broken-dbc-enc-unreachable — Status=inaccessible-encryption-credentials.
	encUnreachable := dbcBaseline("broken-dbc-enc-unreachable")
	encUnreachable.Status = aws.String("inaccessible-encryption-credentials")
	encUnreachable.ClusterCreateTime = aws.Time(mustTime("2026-01-15T09:00:00Z"))

	// 5. broken-dbc-incompat-params — Status=incompatible-parameters.
	incompatParams := dbcBaseline("broken-dbc-incompat-params")
	incompatParams.Status = aws.String("incompatible-parameters")
	incompatParams.ClusterCreateTime = aws.Time(mustTime("2025-12-20T14:00:00Z"))

	// 6. broken-dbc-no-writer — available, two readers, zero writers.
	noWriter := dbcBaseline("broken-dbc-no-writer")
	noWriter.Status = aws.String("available")
	noWriter.DeletionProtection = aws.Bool(false)
	noWriter.BackupRetentionPeriod = aws.Int32(7)
	noWriter.DBClusterMembers = []docdbtypes.DBClusterMember{
		{DBInstanceIdentifier: aws.String("broken-dbc-no-writer-01"), IsClusterWriter: aws.Bool(false)},
		{DBInstanceIdentifier: aws.String("broken-dbc-no-writer-02"), IsClusterWriter: aws.Bool(false)},
	}
	noWriter.ClusterCreateTime = aws.Time(mustTime("2025-09-01T08:00:00Z"))

	// 7. warn-dbc-no-prot — available, writer, encrypted, retention=7, DeletionProtection=false.
	noProt := dbcBaseline("warn-dbc-no-prot")
	noProt.DeletionProtection = aws.Bool(false)
	noProt.ClusterCreateTime = aws.Time(mustTime("2025-07-10T11:00:00Z"))

	// 8. warn-dbc-unenc — available, writer, retention=7, DeletionProtection=true, StorageEncrypted=false.
	unenc := dbcBaseline("warn-dbc-unenc")
	unenc.StorageEncrypted = aws.Bool(false)
	unenc.KmsKeyId = nil
	unenc.ClusterCreateTime = aws.Time(mustTime("2025-08-20T16:45:00Z"))

	// 9. warn-dbc-no-bkp — available, writer, encrypted, DeletionProtection=true, BackupRetentionPeriod=0.
	noBkp := dbcBaseline("warn-dbc-no-bkp")
	noBkp.BackupRetentionPeriod = aws.Int32(0)
	noBkp.ClusterCreateTime = aws.Time(mustTime("2025-06-01T10:00:00Z"))

	// 10. warn-dbc-multi — available, writer, StorageEncrypted=false + DeletionProtection=false + BackupRetentionPeriod=0.
	// Expected Status="delete-protection off (+2)".
	multi := dbcBaseline("warn-dbc-multi")
	multi.StorageEncrypted = aws.Bool(false)
	multi.KmsKeyId = nil
	multi.DeletionProtection = aws.Bool(false)
	multi.BackupRetentionPeriod = aws.Int32(0)
	multi.ClusterCreateTime = aws.Time(mustTime("2025-05-15T09:00:00Z"))

	// 11. healthy-dbc-maint-overdue — healthy baseline; paired with overdue maintenance action.
	maintOverdue := dbcBaseline(MaintDbcOverdueID)
	maintOverdue.DBClusterArn = aws.String(MaintDbcOverdueARN)
	maintOverdue.ClusterCreateTime = aws.Time(mustTime("2025-03-01T12:00:00Z"))

	// 12. warn-dbc-no-bkp-plus-maint — Wave-1 no-bkp + overdue maintenance.
	noBkpPlusMaint := dbcBaseline(WarnDbcNoBkpMaintID)
	noBkpPlusMaint.DBClusterArn = aws.String(WarnDbcNoBkpMaintARN)
	noBkpPlusMaint.BackupRetentionPeriod = aws.Int32(0)
	noBkpPlusMaint.ClusterCreateTime = aws.Time(mustTime("2025-02-10T08:00:00Z"))

	// prod-aurora-cluster — Aurora PostgreSQL cluster. Acts as the
	// "all pivots non-zero" graph-root for dbc (asserted in
	// tests/integration/scenario_dbc_visual_test.go). Every registered
	// dbc pivot resolves on this single fixture: sg (VpcSecurityGroups),
	// alarm (cloudwatch fixture), logs (cwlogs fixture), kms (KmsKeyId),
	// secrets (MasterUserSecret below), dbi (prod-dbi-aurora-1 is a member),
	// dbc-snap (cluster snapshot fixture), subnet+vpc (subnet group).
	aurora := docdbtypes.DBCluster{
		DBClusterIdentifier:        aws.String("prod-aurora-cluster"),
		DBClusterArn:               aws.String("arn:aws:rds:us-east-1:123456789012:cluster:prod-aurora-cluster"),
		Engine:                     aws.String("aurora-postgresql"),
		EngineVersion:              aws.String("16.4"),
		Status:                     aws.String("available"),
		Endpoint:                   aws.String("prod-aurora-cluster.cluster-c9xyz123.us-east-1.rds.amazonaws.com"),
		ReaderEndpoint:             aws.String("prod-aurora-cluster.cluster-ro-xyz.us-east-1.rds.amazonaws.com"),
		Port:                       aws.Int32(5432),
		StorageEncrypted:           aws.Bool(true),
		KmsKeyId:                   aws.String(dbcKMSKeyID),
		DeletionProtection:         aws.Bool(true),
		BackupRetentionPeriod:      aws.Int32(7),
		PreferredMaintenanceWindow: aws.String("sun:05:00-sun:06:00"),
		DBSubnetGroup:              aws.String(rdsSubnetGroup),
		VpcSecurityGroups: []docdbtypes.VpcSecurityGroupMembership{
			{VpcSecurityGroupId: aws.String(dbcSGID), Status: aws.String("active")},
		},
		DBClusterMembers: []docdbtypes.DBClusterMember{
			{DBInstanceIdentifier: aws.String("prod-dbi-aurora-1"), IsClusterWriter: aws.Bool(true)},
		},
		MasterUsername: aws.String("pgadmin"),
		MasterUserSecret: &docdbtypes.ClusterMasterUserSecret{
			SecretArn: aws.String(ProdDbcAuroraMasterSecretARN),
		},
		MultiAZ:           aws.Bool(true),
		ClusterCreateTime: aws.Time(mustTime("2025-03-01T12:00:00Z")),
	}

	return []docdbtypes.DBCluster{
		prod,
		modifying,
		failed,
		encUnreachable,
		incompatParams,
		noWriter,
		noProt,
		unenc,
		noBkp,
		multi,
		maintOverdue,
		noBkpPlusMaint,
		aurora,
	}
}

func buildDBCSnapshots() []docdbtypes.DBClusterSnapshot {
	return []docdbtypes.DBClusterSnapshot{
		// Automated daily snapshot for acme-docdb-prod — satisfies dbc→dbc-snap pivot (count≥1).
		{
			DBClusterSnapshotIdentifier: aws.String(ProdDBCSnapDocDBID),
			DBClusterIdentifier:         aws.String(ProdDbcID),
			DBClusterSnapshotArn:        aws.String(ProdDBCSnapDocDBARN),
			Status:                      aws.String("available"),
			Engine:                      aws.String("docdb"),
			EngineVersion:               aws.String("5.0.0"),
			SnapshotType:                aws.String("automated"),
			SnapshotCreateTime:          aws.Time(mustTime("2026-03-20T04:00:00Z")),
			ClusterCreateTime:           aws.Time(mustTime("2025-01-10T09:00:00Z")),
			MasterUsername:              aws.String("docdbadmin"),
			Port:                        aws.Int32(27017),
			KmsKeyId:                    aws.String(dbcKMSKeyID),
			PercentProgress:             aws.Int32(100),
			SourceDBClusterSnapshotArn:  aws.String("arn:aws:rds:us-east-1:123456789012:cluster-snapshot:rds:acme-docdb-prod-2026-03-19"),
			AvailabilityZones:           []string{"us-east-1a", "us-east-1b", "us-east-1c"},
			StorageType:                 aws.String("standard"),
			StorageEncrypted:            aws.Bool(true),
			VpcId:                       aws.String(dbcVPCID),
		},
		// Automated snapshot for prod-aurora-cluster — required for the
		// dbc→dbc-snap pivot on the Aurora "all pivots non-zero"
		// graph-root. Aurora cluster snapshots share the DocDB API surface
		// (DescribeDBClusterSnapshots) so they land in the same cache.
		{
			DBClusterSnapshotIdentifier: aws.String(ProdDBCSnapAuroraID),
			DBClusterIdentifier:         aws.String("prod-aurora-cluster"),
			DBClusterSnapshotArn:        aws.String(ProdDBCSnapAuroraARN),
			Status:                      aws.String("available"),
			Engine:                      aws.String("aurora-postgresql"),
			EngineVersion:               aws.String("16.4"),
			SnapshotType:                aws.String("automated"),
			SnapshotCreateTime:          aws.Time(mustTime("2026-04-15T04:00:00Z")),
			ClusterCreateTime:           aws.Time(mustTime("2025-03-01T12:00:00Z")),
			MasterUsername:              aws.String("pgadmin"),
			Port:                        aws.Int32(5432),
			KmsKeyId:                    aws.String(dbcKMSKeyID),
			PercentProgress:             aws.Int32(100),
			StorageType:                 aws.String("aurora"),
			StorageEncrypted:            aws.Bool(true),
			VpcId:                       aws.String(dbcVPCID),
		},
		// Manual pre-upgrade snapshot for warn-dbc-modifying.
		{
			DBClusterSnapshotIdentifier: aws.String("pre-upgrade-dbc-snap"),
			DBClusterIdentifier:         aws.String("warn-dbc-modifying"),
			DBClusterSnapshotArn:        aws.String("arn:aws:rds:us-east-1:123456789012:cluster-snapshot:pre-upgrade-dbc-snap"),
			Status:                      aws.String("available"),
			Engine:                      aws.String("docdb"),
			EngineVersion:               aws.String("4.0.0"),
			SnapshotType:                aws.String("manual"),
			SnapshotCreateTime:          aws.Time(mustTime("2026-03-18T20:00:00Z")),
			StorageType:                 aws.String("standard"),
			StorageEncrypted:            aws.Bool(true),
			VpcId:                       aws.String(dbcVPCID),
		},
		// Automated snapshot for analytics cluster (unrelated, populates dbc-snap list count).
		{
			DBClusterSnapshotIdentifier: aws.String("rds:analytics-docdb-2026-03-20"),
			DBClusterIdentifier:         aws.String("analytics-docdb"),
			DBClusterSnapshotArn:        aws.String("arn:aws:rds:us-east-1:123456789012:cluster-snapshot:rds:analytics-docdb-2026-03-20"),
			Status:                      aws.String("available"),
			Engine:                      aws.String("docdb"),
			EngineVersion:               aws.String("5.0.0"),
			SnapshotType:                aws.String("automated"),
			SnapshotCreateTime:          aws.Time(mustTime("2026-03-20T04:30:00Z")),
			StorageType:                 aws.String("iopt1"),
			StorageEncrypted:            aws.Bool(true),
			VpcId:                       aws.String(dbcVPCID),
		},
	}
}

// buildDBCSubnetGroups returns the subnet group for acme-docdb-prod.
// Subnets reference existing EC2 subnet IDs (fixtProdPrivateSubnetA/B in ec2.go).
func buildDBCSubnetGroups() []docdbtypes.DBSubnetGroup {
	return []docdbtypes.DBSubnetGroup{
		{
			DBSubnetGroupName:        aws.String("acme-docdb-subnet-group"),
			DBSubnetGroupDescription: aws.String("Subnet group for acme-docdb-prod cluster"),
			DBSubnetGroupArn:         aws.String("arn:aws:rds:us-east-1:123456789012:subgrp:acme-docdb-subnet-group"),
			VpcId:                    aws.String(dbcVPCID),
			SubnetGroupStatus:        aws.String("Complete"),
			Subnets: []docdbtypes.Subnet{
				{
					SubnetIdentifier: aws.String(dbcSubnetA),
					SubnetStatus:     aws.String("Active"),
					SubnetAvailabilityZone: &docdbtypes.AvailabilityZone{
						Name: aws.String("us-east-1a"),
					},
				},
				{
					SubnetIdentifier: aws.String(dbcSubnetB),
					SubnetStatus:     aws.String("Active"),
					SubnetAvailabilityZone: &docdbtypes.AvailabilityZone{
						Name: aws.String("us-east-1b"),
					},
				},
			},
		},
		// Subnet group for prod-aurora-cluster — required for the dbc→subnet
		// and dbc→vpc pivots on the Aurora "all pivots non-zero" graph-root.
		// Aurora uses rdsSubnetGroup ("acme-rds-subnet-group") in the dbc
		// fixture; the group must exist in the DocDB-API-backed cache since
		// the checker calls DescribeDBSubnetGroups on c.DocDB.
		{
			DBSubnetGroupName:        aws.String(rdsSubnetGroup),
			DBSubnetGroupDescription: aws.String("Subnet group for prod-aurora-cluster"),
			DBSubnetGroupArn:         aws.String("arn:aws:rds:us-east-1:123456789012:subgrp:" + rdsSubnetGroup),
			VpcId:                    aws.String(dbcVPCID),
			SubnetGroupStatus:        aws.String("Complete"),
			Subnets: []docdbtypes.Subnet{
				{
					SubnetIdentifier: aws.String(dbcSubnetA),
					SubnetStatus:     aws.String("Active"),
					SubnetAvailabilityZone: &docdbtypes.AvailabilityZone{
						Name: aws.String("us-east-1a"),
					},
				},
				{
					SubnetIdentifier: aws.String(dbcSubnetB),
					SubnetStatus:     aws.String("Active"),
					SubnetAvailabilityZone: &docdbtypes.AvailabilityZone{
						Name: aws.String("us-east-1b"),
					},
				},
			},
		},
	}
}

// buildDBCPendingMaintenance returns ResourcePendingMaintenanceActions entries for:
//   - healthy-dbc-maint-overdue: os-upgrade, AutoAppliedAfterDate 2026-03-15 (past)
//   - warn-dbc-no-bkp-plus-maint: system-update, AutoAppliedAfterDate 2026-03-10 (past)
func buildDBCPendingMaintenance() []docdbtypes.ResourcePendingMaintenanceActions {
	overdueDate := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)
	noBkpDate := time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)
	return []docdbtypes.ResourcePendingMaintenanceActions{
		{
			ResourceIdentifier: aws.String(MaintDbcOverdueARN),
			PendingMaintenanceActionDetails: []docdbtypes.PendingMaintenanceAction{
				{
					Action:               aws.String("os-upgrade"),
					Description:          aws.String("DocumentDB patch — security"),
					AutoAppliedAfterDate: aws.Time(overdueDate),
				},
			},
		},
		{
			ResourceIdentifier: aws.String(WarnDbcNoBkpMaintARN),
			PendingMaintenanceActionDetails: []docdbtypes.PendingMaintenanceAction{
				{
					Action:               aws.String("system-update"),
					Description:          aws.String("Cluster parameter upgrade"),
					AutoAppliedAfterDate: aws.Time(noBkpDate),
				},
			},
		},
	}
}
