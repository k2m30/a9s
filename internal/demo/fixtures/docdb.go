// Package fixtures provides DocumentDB fixture data for the DocDB fake.
package fixtures

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"
)

// DocDBFixtures holds all DocumentDB domain objects served by the fake.
type DocDBFixtures struct {
	// DBClusters is the full list returned by DescribeDBClusters.
	DBClusters []docdbtypes.DBCluster
	// DBClusterSnapshots is the full list returned by DescribeDBClusterSnapshots.
	DBClusterSnapshots []docdbtypes.DBClusterSnapshot
}

// NewDocDBFixtures builds and returns a fully-populated DocDBFixtures struct.
func NewDocDBFixtures() *DocDBFixtures {
	return &DocDBFixtures{
		DBClusters:         buildDocDBClusters(),
		DBClusterSnapshots: buildDocDBSnapshots(),
	}
}

const (
	docdbKMSKeyID = "arn:aws:kms:us-east-1:123456789012:key/a1b2c3d4-5678-90ab-cdef-111111111111"
	docdbSGID     = "sg-0ccc333333333333c"
	docdbVPCID    = "vpc-0abc123def456789a"
)

func buildDocDBClusters() []docdbtypes.DBCluster {
	return []docdbtypes.DBCluster{
		{
			DBClusterIdentifier:        aws.String("acme-docdb-prod"),
			DBClusterArn:               aws.String("arn:aws:rds:us-east-1:123456789012:cluster:acme-docdb-prod"),
			Engine:                     aws.String("docdb"),
			EngineVersion:              aws.String("5.0.0"),
			Status:                     aws.String("available"),
			Endpoint:                   aws.String("acme-docdb-prod.cluster-c9xyz123.us-east-1.docdb.amazonaws.com"),
			ReaderEndpoint:             aws.String("acme-docdb-prod.cluster-ro-xyz.us-east-1.docdb.amazonaws.com"),
			Port:                       aws.Int32(27017),
			StorageEncrypted:           aws.Bool(true),
			KmsKeyId:                   aws.String(docdbKMSKeyID),
			DeletionProtection:         aws.Bool(true),
			BackupRetentionPeriod:      aws.Int32(7),
			PreferredMaintenanceWindow: aws.String("sun:04:00-sun:04:30"),
			DBSubnetGroup:              aws.String("acme-docdb-subnet-group"),
			VpcSecurityGroups: []docdbtypes.VpcSecurityGroupMembership{
				{VpcSecurityGroupId: aws.String(docdbSGID), Status: aws.String("active")},
			},
			DBClusterMembers: []docdbtypes.DBClusterMember{
				{DBInstanceIdentifier: aws.String("acme-docdb-prod-01"), IsClusterWriter: aws.Bool(true)},
				{DBInstanceIdentifier: aws.String("acme-docdb-prod-02"), IsClusterWriter: aws.Bool(false)},
				{DBInstanceIdentifier: aws.String("acme-docdb-prod-03"), IsClusterWriter: aws.Bool(false)},
			},
			MasterUsername:    aws.String("docdbadmin"),
			MultiAZ:           aws.Bool(true),
			ClusterCreateTime: aws.Time(mustTime("2025-04-15T10:20:00Z")),
		},
		{
			DBClusterIdentifier: aws.String("analytics-docdb"),
			DBClusterArn:        aws.String("arn:aws:rds:us-east-1:123456789012:cluster:analytics-docdb"),
			Engine:              aws.String("docdb"),
			EngineVersion:       aws.String("5.0.0"),
			Status:              aws.String("available"),
			Endpoint:            aws.String("analytics-docdb.cluster-c9xyz123.us-east-1.docdb.amazonaws.com"),
			DBClusterMembers: []docdbtypes.DBClusterMember{
				{DBInstanceIdentifier: aws.String("analytics-docdb-01"), IsClusterWriter: aws.Bool(true)},
				{DBInstanceIdentifier: aws.String("analytics-docdb-02"), IsClusterWriter: aws.Bool(false)},
			},
			MasterUsername:    aws.String("analytics"),
			MultiAZ:           aws.Bool(false),
			ClusterCreateTime: aws.Time(mustTime("2025-08-20T16:45:00Z")),
		},
		{
			DBClusterIdentifier:        aws.String("staging-docdb"),
			DBClusterArn:               aws.String("arn:aws:rds:us-east-1:123456789012:cluster:staging-docdb"),
			Engine:                     aws.String("docdb"),
			EngineVersion:              aws.String("4.0.0"),
			Status:                     aws.String("modifying"),
			Endpoint:                   aws.String("staging-docdb.cluster-c9xyz123.us-east-1.docdb.amazonaws.com"),
			ReaderEndpoint:             aws.String("staging-docdb.cluster-ro-c9xyz123.us-east-1.docdb.amazonaws.com"),
			Port:                       aws.Int32(27017),
			StorageEncrypted:           aws.Bool(true),
			KmsKeyId:                   aws.String(docdbKMSKeyID),
			DeletionProtection:         aws.Bool(false),
			BackupRetentionPeriod:      aws.Int32(1),
			PreferredMaintenanceWindow: aws.String("tue:07:00-tue:08:00"),
			DBClusterMembers: []docdbtypes.DBClusterMember{
				{DBInstanceIdentifier: aws.String("staging-docdb-01"), IsClusterWriter: aws.Bool(true)},
			},
			MasterUsername: aws.String("stagingadmin"),
			MultiAZ:        aws.Bool(false),
			VpcSecurityGroups: []docdbtypes.VpcSecurityGroupMembership{
				{VpcSecurityGroupId: aws.String(docdbSGID), Status: aws.String("active")},
			},
			DBSubnetGroup:     aws.String("acme-docdb-subnet-group"),
			ClusterCreateTime: aws.Time(mustTime("2025-11-05T08:30:00Z")),
		},
	}
}

func buildDocDBSnapshots() []docdbtypes.DBClusterSnapshot {
	return []docdbtypes.DBClusterSnapshot{
		{
			DBClusterSnapshotIdentifier: aws.String("rds:acme-docdb-prod-2026-03-20"),
			DBClusterIdentifier:         aws.String("acme-docdb-prod"),
			DBClusterSnapshotArn:        aws.String("arn:aws:rds:us-east-1:123456789012:cluster-snapshot:rds:acme-docdb-prod-2026-03-20"),
			Status:                      aws.String("available"),
			Engine:                      aws.String("docdb"),
			EngineVersion:               aws.String("5.0.0"),
			SnapshotType:                aws.String("automated"),
			SnapshotCreateTime:          aws.Time(mustTime("2026-03-20T04:00:00Z")),
			ClusterCreateTime:           aws.Time(mustTime("2025-01-10T09:00:00Z")),
			MasterUsername:              aws.String("docdbadmin"),
			Port:                        aws.Int32(27017),
			KmsKeyId:                    aws.String(docdbKMSKeyID),
			PercentProgress:             aws.Int32(100),
			SourceDBClusterSnapshotArn:  aws.String("arn:aws:rds:us-east-1:123456789012:cluster-snapshot:rds:acme-docdb-prod-2026-03-19"),
			AvailabilityZones:           []string{"us-east-1a", "us-east-1b", "us-east-1c"},
			StorageType:                 aws.String("standard"),
			StorageEncrypted:            aws.Bool(true),
			VpcId:                       aws.String(docdbVPCID),
		},
		{
			DBClusterSnapshotIdentifier: aws.String("pre-upgrade-docdb-snap"),
			DBClusterIdentifier:         aws.String("staging-docdb"),
			DBClusterSnapshotArn:        aws.String("arn:aws:rds:us-east-1:123456789012:cluster-snapshot:pre-upgrade-docdb-snap"),
			Status:                      aws.String("available"),
			Engine:                      aws.String("docdb"),
			EngineVersion:               aws.String("4.0.0"),
			SnapshotType:                aws.String("manual"),
			SnapshotCreateTime:          aws.Time(mustTime("2026-03-18T20:00:00Z")),
			StorageType:                 aws.String("standard"),
			StorageEncrypted:            aws.Bool(true),
			VpcId:                       aws.String(docdbVPCID),
		},
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
			VpcId:                       aws.String(docdbVPCID),
		},
	}
}
