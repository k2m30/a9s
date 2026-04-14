// Package fixtures provides RDS fixture data for the RDS fake.
package fixtures

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
)

// RDSFixtures holds all RDS domain objects served by the fake.
type RDSFixtures struct {
	// DBInstances is the full list returned by DescribeDBInstances.
	DBInstances []rdstypes.DBInstance
	// DBSnapshots is the full list returned by DescribeDBSnapshots.
	DBSnapshots []rdstypes.DBSnapshot
	// Events is a shared list of events returned by DescribeEvents.
	Events []rdstypes.Event
}

// NewRDSFixtures builds and returns a fully-populated RDSFixtures struct.
func NewRDSFixtures() *RDSFixtures {
	return &RDSFixtures{
		DBInstances: buildRDSInstances(),
		DBSnapshots: buildRDSSnapshots(),
		Events:      buildRDSEvents(),
	}
}

const (
	rdsProdRDSSGID = "sg-0ccc333333333333c"
	rdsProdVPCID   = "vpc-0abc123def456789a"
	rdsSubnetGroup = "acme-rds-subnet-group"
	rdsKMSKeyID    = "arn:aws:kms:us-east-1:123456789012:key/a1b2c3d4-5678-90ab-cdef-111111111111"
)

var rdsNamePool = []string{
	"payment-db-01", "payment-db-02", "user-service-db", "inventory-db",
	"order-history-db", "notification-db", "session-db", "audit-db",
	"reporting-db", "metrics-db", "config-db", "integration-db",
	"partner-db", "archive-db", "sandbox-db-01", "sandbox-db-02",
	"canary-db",
}

var rdsEnginePool = []struct {
	engine, version, class string
}{
	{"aurora-postgresql", "16.4", "db.t3.medium"},
	{"aurora-postgresql", "16.4", "db.t3.small"},
	{"mysql", "8.0.36", "db.t3.medium"},
	{"postgres", "16.2", "db.t3.medium"},
	{"aurora-postgresql", "15.6", "db.t3.small"},
	{"mysql", "8.0.36", "db.t3.small"},
	{"aurora-postgresql", "16.4", "db.t3.medium"},
	{"postgres", "15.5", "db.t3.medium"},
	{"mysql", "8.0.36", "db.t3.medium"},
	{"aurora-postgresql", "16.4", "db.t3.small"},
	{"postgres", "16.2", "db.t3.small"},
	{"mysql", "8.0.36", "db.t3.medium"},
	{"aurora-postgresql", "16.4", "db.t3.medium"},
	{"postgres", "15.5", "db.t3.small"},
	{"aurora-postgresql", "15.6", "db.t3.medium"},
	{"mysql", "8.0.36", "db.t3.small"},
	{"aurora-postgresql", "16.4", "db.t3.medium"},
}

func buildRDSInstances() []rdstypes.DBInstance {
	named := []rdstypes.DBInstance{
		{
			DBInstanceIdentifier:       aws.String("prod-api-primary"),
			DBInstanceArn:              aws.String("arn:aws:rds:us-east-1:123456789012:db:prod-api-primary"),
			Engine:                     aws.String("aurora-postgresql"),
			EngineVersion:              aws.String("16.4"),
			DBInstanceStatus:           aws.String("available"),
			DBInstanceClass:            aws.String("db.r6g.xlarge"),
			MasterUsername:             aws.String("pgadmin"),
			AvailabilityZone:           aws.String("us-east-1a"),
			AllocatedStorage:           aws.Int32(100),
			StorageType:                aws.String("aurora"),
			StorageEncrypted:           aws.Bool(true),
			KmsKeyId:                   aws.String(rdsKMSKeyID),
			Iops:                       aws.Int32(0),
			BackupRetentionPeriod:      aws.Int32(7),
			PreferredBackupWindow:      aws.String("03:00-04:00"),
			PreferredMaintenanceWindow: aws.String("sun:05:00-sun:06:00"),
			DeletionProtection:         aws.Bool(true),
			PubliclyAccessible:         aws.Bool(false),
			PerformanceInsightsEnabled: aws.Bool(true),
			Endpoint: &rdstypes.Endpoint{
				Address: aws.String("prod-api-primary.cluster-c9xyz123.us-east-1.rds.amazonaws.com"),
				Port:    aws.Int32(5432),
			},
			MultiAZ: aws.Bool(true),
			VpcSecurityGroups: []rdstypes.VpcSecurityGroupMembership{
				{VpcSecurityGroupId: aws.String(rdsProdRDSSGID), Status: aws.String("active")},
			},
			DBSubnetGroup: &rdstypes.DBSubnetGroup{
				VpcId:             aws.String(rdsProdVPCID),
				DBSubnetGroupName: aws.String(rdsSubnetGroup),
			},
			TagList: []rdstypes.Tag{
				{Key: aws.String("Environment"), Value: aws.String("production")},
				{Key: aws.String("Team"), Value: aws.String("backend")},
			},
		},
		{
			DBInstanceIdentifier:       aws.String("prod-api-replica"),
			DBInstanceArn:              aws.String("arn:aws:rds:us-east-1:123456789012:db:prod-api-replica"),
			Engine:                     aws.String("aurora-postgresql"),
			EngineVersion:              aws.String("16.4"),
			DBInstanceStatus:           aws.String("available"),
			DBInstanceClass:            aws.String("db.r6g.large"),
			MasterUsername:             aws.String("pgadmin"),
			AvailabilityZone:           aws.String("us-east-1b"),
			AllocatedStorage:           aws.Int32(100),
			StorageType:                aws.String("aurora"),
			StorageEncrypted:           aws.Bool(true),
			KmsKeyId:                   aws.String(rdsKMSKeyID),
			Iops:                       aws.Int32(0),
			BackupRetentionPeriod:      aws.Int32(7),
			PreferredBackupWindow:      aws.String("03:00-04:00"),
			PreferredMaintenanceWindow: aws.String("sun:05:00-sun:06:00"),
			DeletionProtection:         aws.Bool(true),
			PubliclyAccessible:         aws.Bool(false),
			PerformanceInsightsEnabled: aws.Bool(true),
			Endpoint: &rdstypes.Endpoint{
				Address: aws.String("prod-api-replica.c9xyz123.us-east-1.rds.amazonaws.com"),
				Port:    aws.Int32(5432),
			},
			MultiAZ: aws.Bool(false),
			VpcSecurityGroups: []rdstypes.VpcSecurityGroupMembership{
				{VpcSecurityGroupId: aws.String(rdsProdRDSSGID), Status: aws.String("active")},
			},
			DBSubnetGroup: &rdstypes.DBSubnetGroup{
				VpcId:             aws.String(rdsProdVPCID),
				DBSubnetGroupName: aws.String(rdsSubnetGroup),
			},
			TagList: []rdstypes.Tag{
				{Key: aws.String("Environment"), Value: aws.String("prod")},
			},
		},
		{
			DBInstanceIdentifier:       aws.String("analytics-warehouse"),
			DBInstanceArn:              aws.String("arn:aws:rds:us-east-1:123456789012:db:analytics-warehouse"),
			Engine:                     aws.String("postgres"),
			EngineVersion:              aws.String("16.2"),
			DBInstanceStatus:           aws.String("available"),
			DBInstanceClass:            aws.String("db.m6g.2xlarge"),
			MasterUsername:             aws.String("analytics"),
			AvailabilityZone:           aws.String("us-east-1a"),
			AllocatedStorage:           aws.Int32(500),
			StorageType:                aws.String("gp3"),
			Iops:                       aws.Int32(3000),
			StorageEncrypted:           aws.Bool(true),
			KmsKeyId:                   aws.String(rdsKMSKeyID),
			BackupRetentionPeriod:      aws.Int32(14),
			PreferredBackupWindow:      aws.String("04:00-05:00"),
			PreferredMaintenanceWindow: aws.String("mon:06:00-mon:07:00"),
			DeletionProtection:         aws.Bool(true),
			PubliclyAccessible:         aws.Bool(false),
			PerformanceInsightsEnabled: aws.Bool(true),
			Endpoint: &rdstypes.Endpoint{
				Address: aws.String("analytics-warehouse.c9xyz123.us-east-1.rds.amazonaws.com"),
				Port:    aws.Int32(5432),
			},
			MultiAZ: aws.Bool(true),
			VpcSecurityGroups: []rdstypes.VpcSecurityGroupMembership{
				{VpcSecurityGroupId: aws.String(rdsProdRDSSGID), Status: aws.String("active")},
			},
			DBSubnetGroup: &rdstypes.DBSubnetGroup{
				VpcId:             aws.String(rdsProdVPCID),
				DBSubnetGroupName: aws.String(rdsSubnetGroup),
			},
			TagList: []rdstypes.Tag{
				{Key: aws.String("Environment"), Value: aws.String("prod")},
			},
		},
		{
			DBInstanceIdentifier:       aws.String("staging-mysql"),
			DBInstanceArn:              aws.String("arn:aws:rds:us-east-1:123456789012:db:staging-mysql"),
			Engine:                     aws.String("mysql"),
			EngineVersion:              aws.String("8.0.36"),
			DBInstanceStatus:           aws.String("stopped"),
			DBInstanceClass:            aws.String("db.t3.medium"),
			MasterUsername:             aws.String("mysqladmin"),
			AvailabilityZone:           aws.String("us-east-1a"),
			AllocatedStorage:           aws.Int32(50),
			StorageType:                aws.String("gp2"),
			StorageEncrypted:           aws.Bool(false),
			BackupRetentionPeriod:      aws.Int32(1),
			PreferredMaintenanceWindow: aws.String("fri:07:00-fri:08:00"),
			DeletionProtection:         aws.Bool(false),
			PubliclyAccessible:         aws.Bool(false),
			Endpoint: &rdstypes.Endpoint{
				Address: aws.String("staging-mysql.c9xyz123.us-east-1.rds.amazonaws.com"),
				Port:    aws.Int32(3306),
			},
			MultiAZ: aws.Bool(false),
			VpcSecurityGroups: []rdstypes.VpcSecurityGroupMembership{
				{VpcSecurityGroupId: aws.String(rdsProdRDSSGID), Status: aws.String("active")},
			},
			DBSubnetGroup: &rdstypes.DBSubnetGroup{
				VpcId:             aws.String(rdsProdVPCID),
				DBSubnetGroupName: aws.String("acme-staging-rds-subnet-group"),
			},
			TagList: []rdstypes.Tag{
				{Key: aws.String("Environment"), Value: aws.String("staging")},
			},
		},
		{
			DBInstanceIdentifier:  aws.String("dev-feature-branch"),
			DBInstanceArn:         aws.String("arn:aws:rds:us-east-1:123456789012:db:dev-feature-branch"),
			Engine:                aws.String("aurora-postgresql"),
			EngineVersion:         aws.String("16.4"),
			DBInstanceStatus:      aws.String("creating"),
			DBInstanceClass:       aws.String("db.t3.medium"),
			MasterUsername:        aws.String("pgadmin"),
			AvailabilityZone:      aws.String("us-east-1a"),
			AllocatedStorage:      aws.Int32(20),
			StorageType:           aws.String("aurora"),
			StorageEncrypted:      aws.Bool(true),
			BackupRetentionPeriod: aws.Int32(1),
			DeletionProtection:    aws.Bool(false),
			PubliclyAccessible:    aws.Bool(false),
			MultiAZ:               aws.Bool(false),
			VpcSecurityGroups: []rdstypes.VpcSecurityGroupMembership{
				{VpcSecurityGroupId: aws.String(rdsProdRDSSGID), Status: aws.String("active")},
			},
			TagList: []rdstypes.Tag{
				{Key: aws.String("Environment"), Value: aws.String("dev")},
			},
		},
		{
			DBInstanceIdentifier:  aws.String("legacy-reports-db"),
			DBInstanceArn:         aws.String("arn:aws:rds:us-east-1:123456789012:db:legacy-reports-db"),
			Engine:                aws.String("mysql"),
			EngineVersion:         aws.String("8.0.36"),
			DBInstanceStatus:      aws.String("failed"),
			DBInstanceClass:       aws.String("db.t3.medium"),
			MasterUsername:        aws.String("mysqladmin"),
			AvailabilityZone:      aws.String("us-east-1b"),
			AllocatedStorage:      aws.Int32(100),
			StorageType:           aws.String("gp2"),
			StorageEncrypted:      aws.Bool(false),
			BackupRetentionPeriod: aws.Int32(0),
			DeletionProtection:    aws.Bool(false),
			PubliclyAccessible:    aws.Bool(false),
			MultiAZ:               aws.Bool(false),
			VpcSecurityGroups: []rdstypes.VpcSecurityGroupMembership{
				{VpcSecurityGroupId: aws.String(rdsProdRDSSGID), Status: aws.String("active")},
			},
			DBSubnetGroup: &rdstypes.DBSubnetGroup{
				VpcId:             aws.String(rdsProdVPCID),
				DBSubnetGroupName: aws.String(rdsSubnetGroup),
			},
			TagList: []rdstypes.Tag{
				{Key: aws.String("Environment"), Value: aws.String("legacy")},
				{Key: aws.String("Team"), Value: aws.String("reporting")},
			},
		},
	}

	// Generate 17 more instances to reach 22 total.
	statuses := []string{"available", "available", "available", "stopped", "available",
		"available", "available", "modifying", "available", "available",
		"available", "available", "available", "stopped", "available",
		"available", "available"}
	for i := range 17 {
		eng := rdsEnginePool[i]
		name := rdsNamePool[i]
		status := statuses[i]
		port := int32(5432)
		if eng.engine == "mysql" {
			port = 3306
		}
		named = append(named, rdstypes.DBInstance{
			DBInstanceIdentifier:  aws.String(name),
			DBInstanceArn:         aws.String("arn:aws:rds:us-east-1:123456789012:db:" + name),
			Engine:                aws.String(eng.engine),
			EngineVersion:         aws.String(eng.version),
			DBInstanceStatus:      aws.String(status),
			DBInstanceClass:       aws.String(eng.class),
			MasterUsername:        aws.String("admin"),
			AvailabilityZone:      aws.String("us-east-1a"),
			AllocatedStorage:      aws.Int32(20),
			StorageType:           aws.String("gp3"),
			StorageEncrypted:      aws.Bool(true),
			BackupRetentionPeriod: aws.Int32(7),
			DeletionProtection:    aws.Bool(false),
			PubliclyAccessible:    aws.Bool(false),
			MultiAZ:               aws.Bool(false),
			Endpoint: &rdstypes.Endpoint{
				Address: aws.String(name + ".c9xyz123.us-east-1.rds.amazonaws.com"),
				Port:    aws.Int32(port),
			},
			VpcSecurityGroups: []rdstypes.VpcSecurityGroupMembership{
				{VpcSecurityGroupId: aws.String(rdsProdRDSSGID), Status: aws.String("active")},
			},
			DBSubnetGroup: &rdstypes.DBSubnetGroup{
				VpcId:             aws.String(rdsProdVPCID),
				DBSubnetGroupName: aws.String(rdsSubnetGroup),
			},
		})
	}

	return named
}

func buildRDSSnapshots() []rdstypes.DBSnapshot {
	return []rdstypes.DBSnapshot{
		{
			DBSnapshotIdentifier: aws.String("rds:prod-api-primary-2026-03-20"),
			DBInstanceIdentifier: aws.String("prod-api-primary"),
			Status:               aws.String("available"),
			Engine:               aws.String("aurora-postgresql"),
			EngineVersion:        aws.String("16.4"),
			SnapshotType:         aws.String("automated"),
			SnapshotCreateTime:   aws.Time(mustTime("2026-03-20T03:00:00Z")),
			AllocatedStorage:     aws.Int32(100),
			StorageType:          aws.String("gp3"),
			Encrypted:            aws.Bool(true),
			KmsKeyId:             aws.String(rdsKMSKeyID),
			AvailabilityZone:     aws.String("us-east-1a"),
			MasterUsername:       aws.String("pgadmin"),
			LicenseModel:         aws.String("postgresql-license"),
			Iops:                 aws.Int32(3000),
			PercentProgress:      aws.Int32(100),
			SourceRegion:         aws.String("us-east-1"),
			DBSnapshotArn:        aws.String("arn:aws:rds:us-east-1:123456789012:snapshot:rds:prod-api-primary-2026-03-20"),
		},
		{
			DBSnapshotIdentifier: aws.String("rds:analytics-warehouse-2026-03-20"),
			DBInstanceIdentifier: aws.String("analytics-warehouse"),
			Status:               aws.String("available"),
			Engine:               aws.String("postgres"),
			EngineVersion:        aws.String("16.2"),
			SnapshotType:         aws.String("automated"),
			SnapshotCreateTime:   aws.Time(mustTime("2026-03-20T03:30:00Z")),
			AllocatedStorage:     aws.Int32(500),
			DBSnapshotArn:        aws.String("arn:aws:rds:us-east-1:123456789012:snapshot:rds:analytics-warehouse-2026-03-20"),
		},
		{
			DBSnapshotIdentifier: aws.String("pre-migration-snapshot"),
			DBInstanceIdentifier: aws.String("staging-mysql"),
			Status:               aws.String("available"),
			Engine:               aws.String("mysql"),
			EngineVersion:        aws.String("8.0.36"),
			SnapshotType:         aws.String("manual"),
			SnapshotCreateTime:   aws.Time(mustTime("2026-03-15T22:00:00Z")),
			AllocatedStorage:     aws.Int32(50),
			DBSnapshotArn:        aws.String("arn:aws:rds:us-east-1:123456789012:snapshot:pre-migration-snapshot"),
		},
		{
			DBSnapshotIdentifier: aws.String("dev-feature-branch-snap"),
			DBInstanceIdentifier: aws.String("dev-feature-branch"),
			Status:               aws.String("creating"),
			Engine:               aws.String("aurora-postgresql"),
			EngineVersion:        aws.String("16.4"),
			SnapshotType:         aws.String("manual"),
			SnapshotCreateTime:   aws.Time(mustTime("2026-03-21T10:30:00Z")),
			AllocatedStorage:     aws.Int32(20),
			DBSnapshotArn:        aws.String("arn:aws:rds:us-east-1:123456789012:snapshot:dev-feature-branch-snap"),
		},
	}
}

func buildRDSEvents() []rdstypes.Event {
	t1 := time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 3, 19, 22, 30, 0, 0, time.UTC)
	t3 := time.Date(2026, 3, 18, 14, 15, 0, 0, time.UTC)
	return []rdstypes.Event{
		{
			SourceIdentifier: aws.String("prod-api-primary"),
			SourceType:       rdstypes.SourceTypeDbInstance,
			Message:          aws.String("Automatic backup completed for DB instance prod-api-primary"),
			EventCategories:  []string{"backup"},
			Date:             aws.Time(t1),
		},
		{
			SourceIdentifier: aws.String("prod-api-primary"),
			SourceType:       rdstypes.SourceTypeDbInstance,
			Message:          aws.String("DB instance restarted: prod-api-primary"),
			EventCategories:  []string{"availability"},
			Date:             aws.Time(t2),
		},
		{
			SourceIdentifier: aws.String("analytics-warehouse"),
			SourceType:       rdstypes.SourceTypeDbInstance,
			Message:          aws.String("Parameter group change requiring restart for analytics-warehouse"),
			EventCategories:  []string{"configuration change"},
			Date:             aws.Time(t3),
		},
	}
}
