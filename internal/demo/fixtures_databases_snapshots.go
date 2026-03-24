package demo

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	demoData["rds-snap"] = rdsSnapshotFixtures
	demoData["docdb-snap"] = docdbSnapshotFixtures
}

// rdsSnapshotFixtures returns demo RDS DB snapshot fixtures.
// Field keys: snapshot_id, db_instance, status, engine, snapshot_type, created
func rdsSnapshotFixtures() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "rds:prod-api-primary-2026-03-20",
			Name:   "rds:prod-api-primary-2026-03-20",
			Status: "available",
			Fields: map[string]string{
				"snapshot_id":   "rds:prod-api-primary-2026-03-20",
				"db_instance":   "prod-api-primary",
				"status":        "available",
				"engine":        "aurora-postgresql",
				"snapshot_type": "automated",
				"created":       "2026-03-20T03:00:00Z",
			},
			RawStruct: rdstypes.DBSnapshot{
				DBSnapshotIdentifier: aws.String("rds:prod-api-primary-2026-03-20"),
				DBInstanceIdentifier: aws.String("prod-api-primary"),
				Status:               aws.String("available"),
				Engine:               aws.String("aurora-postgresql"),
				EngineVersion:        aws.String("16.4"),
				SnapshotType:         aws.String("automated"),
				SnapshotCreateTime:   aws.Time(mustParseTime("2026-03-20T03:00:00+00:00")),
				AllocatedStorage:     aws.Int32(100),
				DBSnapshotArn:        aws.String("arn:aws:rds:us-east-1:123456789012:snapshot:rds:prod-api-primary-2026-03-20"),
			},
		},
		{
			ID:     "rds:analytics-warehouse-2026-03-20",
			Name:   "rds:analytics-warehouse-2026-03-20",
			Status: "available",
			Fields: map[string]string{
				"snapshot_id":   "rds:analytics-warehouse-2026-03-20",
				"db_instance":   "analytics-warehouse",
				"status":        "available",
				"engine":        "postgres",
				"snapshot_type": "automated",
				"created":       "2026-03-20T03:30:00Z",
			},
			RawStruct: rdstypes.DBSnapshot{
				DBSnapshotIdentifier: aws.String("rds:analytics-warehouse-2026-03-20"),
				DBInstanceIdentifier: aws.String("analytics-warehouse"),
				Status:               aws.String("available"),
				Engine:               aws.String("postgres"),
				EngineVersion:        aws.String("16.2"),
				SnapshotType:         aws.String("automated"),
				SnapshotCreateTime:   aws.Time(mustParseTime("2026-03-20T03:30:00+00:00")),
				AllocatedStorage:     aws.Int32(500),
				DBSnapshotArn:        aws.String("arn:aws:rds:us-east-1:123456789012:snapshot:rds:analytics-warehouse-2026-03-20"),
			},
		},
		{
			ID:     "pre-migration-snapshot",
			Name:   "pre-migration-snapshot",
			Status: "available",
			Fields: map[string]string{
				"snapshot_id":   "pre-migration-snapshot",
				"db_instance":   "staging-mysql",
				"status":        "available",
				"engine":        "mysql",
				"snapshot_type": "manual",
				"created":       "2026-03-15T22:00:00Z",
			},
			RawStruct: rdstypes.DBSnapshot{
				DBSnapshotIdentifier: aws.String("pre-migration-snapshot"),
				DBInstanceIdentifier: aws.String("staging-mysql"),
				Status:               aws.String("available"),
				Engine:               aws.String("mysql"),
				EngineVersion:        aws.String("8.0.36"),
				SnapshotType:         aws.String("manual"),
				SnapshotCreateTime:   aws.Time(mustParseTime("2026-03-15T22:00:00+00:00")),
				AllocatedStorage:     aws.Int32(50),
				DBSnapshotArn:        aws.String("arn:aws:rds:us-east-1:123456789012:snapshot:pre-migration-snapshot"),
			},
		},
		{
			ID:     "dev-feature-branch-snap",
			Name:   "dev-feature-branch-snap",
			Status: "creating",
			Fields: map[string]string{
				"snapshot_id":   "dev-feature-branch-snap",
				"db_instance":   "dev-feature-branch",
				"status":        "creating",
				"engine":        "aurora-postgresql",
				"snapshot_type": "manual",
				"created":       "2026-03-21T10:30:00Z",
			},
			RawStruct: rdstypes.DBSnapshot{
				DBSnapshotIdentifier: aws.String("dev-feature-branch-snap"),
				DBInstanceIdentifier: aws.String("dev-feature-branch"),
				Status:               aws.String("creating"),
				Engine:               aws.String("aurora-postgresql"),
				EngineVersion:        aws.String("16.4"),
				SnapshotType:         aws.String("manual"),
				SnapshotCreateTime:   aws.Time(mustParseTime("2026-03-21T10:30:00+00:00")),
				AllocatedStorage:     aws.Int32(20),
				DBSnapshotArn:        aws.String("arn:aws:rds:us-east-1:123456789012:snapshot:dev-feature-branch-snap"),
			},
		},
	}
}

// docdbSnapshotFixtures returns demo DocumentDB cluster snapshot fixtures.
// Field keys: snapshot_id, cluster_id, status, engine, snapshot_type, snapshot_create_time, storage_type
func docdbSnapshotFixtures() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "rds:acme-docdb-prod-2026-03-20",
			Name:   "rds:acme-docdb-prod-2026-03-20",
			Status: "available",
			Fields: map[string]string{
				"snapshot_id":          "rds:acme-docdb-prod-2026-03-20",
				"cluster_id":           "acme-docdb-prod",
				"status":               "available",
				"engine":               "docdb",
				"snapshot_type":        "automated",
				"snapshot_create_time": "2026-03-20T04:00:00Z",
				"storage_type":         "standard",
			},
			RawStruct: docdbtypes.DBClusterSnapshot{
				DBClusterSnapshotIdentifier: aws.String("rds:acme-docdb-prod-2026-03-20"),
				DBClusterIdentifier:         aws.String("acme-docdb-prod"),
				DBClusterSnapshotArn:        aws.String("arn:aws:rds:us-east-1:123456789012:cluster-snapshot:rds:acme-docdb-prod-2026-03-20"),
				Status:                      aws.String("available"),
				Engine:                      aws.String("docdb"),
				EngineVersion:               aws.String("5.0.0"),
				SnapshotType:                aws.String("automated"),
				SnapshotCreateTime:          aws.Time(mustParseTime("2026-03-20T04:00:00+00:00")),
				StorageType:                 aws.String("standard"),
				StorageEncrypted:            aws.Bool(true),
				VpcId:                       aws.String("vpc-0abc123def456789a"),
			},
		},
		{
			ID:     "pre-upgrade-docdb-snap",
			Name:   "pre-upgrade-docdb-snap",
			Status: "available",
			Fields: map[string]string{
				"snapshot_id":          "pre-upgrade-docdb-snap",
				"cluster_id":           "staging-docdb",
				"status":               "available",
				"engine":               "docdb",
				"snapshot_type":        "manual",
				"snapshot_create_time": "2026-03-18T20:00:00Z",
				"storage_type":         "standard",
			},
			RawStruct: docdbtypes.DBClusterSnapshot{
				DBClusterSnapshotIdentifier: aws.String("pre-upgrade-docdb-snap"),
				DBClusterIdentifier:         aws.String("staging-docdb"),
				DBClusterSnapshotArn:        aws.String("arn:aws:rds:us-east-1:123456789012:cluster-snapshot:pre-upgrade-docdb-snap"),
				Status:                      aws.String("available"),
				Engine:                      aws.String("docdb"),
				EngineVersion:               aws.String("4.0.0"),
				SnapshotType:                aws.String("manual"),
				SnapshotCreateTime:          aws.Time(mustParseTime("2026-03-18T20:00:00+00:00")),
				StorageType:                 aws.String("standard"),
				StorageEncrypted:            aws.Bool(true),
				VpcId:                       aws.String("vpc-0abc123def456789a"),
			},
		},
		{
			ID:     "rds:analytics-docdb-2026-03-20",
			Name:   "rds:analytics-docdb-2026-03-20",
			Status: "available",
			Fields: map[string]string{
				"snapshot_id":          "rds:analytics-docdb-2026-03-20",
				"cluster_id":           "analytics-docdb",
				"status":               "available",
				"engine":               "docdb",
				"snapshot_type":        "automated",
				"snapshot_create_time": "2026-03-20T04:30:00Z",
				"storage_type":         "iopt1",
			},
			RawStruct: docdbtypes.DBClusterSnapshot{
				DBClusterSnapshotIdentifier: aws.String("rds:analytics-docdb-2026-03-20"),
				DBClusterIdentifier:         aws.String("analytics-docdb"),
				DBClusterSnapshotArn:        aws.String("arn:aws:rds:us-east-1:123456789012:cluster-snapshot:rds:analytics-docdb-2026-03-20"),
				Status:                      aws.String("available"),
				Engine:                      aws.String("docdb"),
				EngineVersion:               aws.String("5.0.0"),
				SnapshotType:                aws.String("automated"),
				SnapshotCreateTime:          aws.Time(mustParseTime("2026-03-20T04:30:00+00:00")),
				StorageType:                 aws.String("iopt1"),
				StorageEncrypted:            aws.Bool(true),
				VpcId:                       aws.String("vpc-0abc123def456789a"),
			},
		},
	}
}
