// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// Package fixtures provides RDS Snapshot fixture data for the RDS fake.
package fixtures

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
)

// DBISnapFixtures holds typed fixture data for RDS DB Snapshots.
type DBISnapFixtures struct {
	// Instances is the full list returned by DescribeDBSnapshots.
	Instances []rdstypes.DBSnapshot
}

// Stable IDs and ARNs for RDS Snapshot fixtures — imported by sibling fixture files.
//
// dbi-snap pivots are 1:1 for dbi and kms (a
// snapshot has exactly one source instance and one encryption key); the dbc
// pivot has no realistic non-zero case for dbi-snap because Aurora cluster
// snapshots live in dbc-snap (real AWS rejects CreateDBSnapshot on Aurora
// cluster members). The graph-root ProdDBISnapID
// achieves Count ≥ 1 on every counted pivot except dbc.
const (
	// ProdDBISnapID — graph root: Healthy non-Aurora automated snapshot of prod-dbi-1.
	// dbi pivot: Count=1 (ProdDbiID). kms pivot: Count=1 (dbiKMSKeyID).
	// dbc pivot: Count=0 (non-Aurora, no DBClusterIdentifier on parent — AWS-API truth).
	// backup pivot: Count=1 (one recovery point in backup.go).
	// ct-events pivot: count "unknown" (windowed).
	ProdDBISnapID  = "rds:prod-dbi-1-2026-04-15"
	ProdDBISnapARN = "arn:aws:rds:us-east-1:123456789012:snapshot:rds:prod-dbi-1-2026-04-15"

	// ResourceIDDBISnapID is the manual snapshot that carries DbiResourceId,
	// the field a snapshot's parent is matched on when it has one.
	ResourceIDDBISnapID  = "prod-dbi-1-manual-2026-02-11"
	ResourceIDDBISnapARN = "arn:aws:rds:us-east-1:123456789012:snapshot:prod-dbi-1-manual-2026-02-11"

	// WarnDBISnapCopyingID carries dbi-snap.warn.transitional: a cross-Region
	// copy from us-west-2 still being copied, whose source instance is not in
	// this Region's dbi list.
	WarnDBISnapCopyingID  = "cross-region-copy-snap"
	WarnDBISnapCopyingARN = "arn:aws:rds:us-east-1:123456789012:snapshot:cross-region-copy-snap"

	// WarnDBISnapCreatingID — Wave-1 warning: Status=creating, PercentProgress=42.
	WarnDBISnapCreatingID  = "dev-feature-branch-snap"
	WarnDBISnapCreatingARN = "arn:aws:rds:us-east-1:123456789012:snapshot:dev-feature-branch-snap"

	// BrokenDBISnapFailedID — Broken: Status=failed.
	BrokenDBISnapFailedID  = "prod-dbi-1-failed-snap"
	BrokenDBISnapFailedARN = "arn:aws:rds:us-east-1:123456789012:snapshot:prod-dbi-1-failed-snap"

	// BrokenDBISnapIncompatibleID — Broken: Status=incompatible-restore.
	BrokenDBISnapIncompatibleID  = "legacy-mysql-snap-incompatible"
	BrokenDBISnapIncompatibleARN = "arn:aws:rds:us-east-1:123456789012:snapshot:legacy-mysql-snap-incompatible"

	// WarnDBISnapUnencryptedID — Warning: Encrypted=false.
	WarnDBISnapUnencryptedID  = "unenc-pre-migration-snap"
	WarnDBISnapUnencryptedARN = "arn:aws:rds:us-east-1:123456789012:snapshot:unenc-pre-migration-snap"

	// WarnDBISnapOrphanID — Warning orphan: parent DBInstanceIdentifier not in dbi list.
	WarnDBISnapOrphanID  = "orphan-deleted-db-snap"
	WarnDBISnapOrphanARN = "arn:aws:rds:us-east-1:123456789012:snapshot:orphan-deleted-db-snap"

	// WarnDBISnapPastRetentionID — Warning: automated snapshot older than parent's
	// BackupRetentionPeriod (parent = ProdDbiRetentionParentID with retention=7).
	// SnapshotCreateTime is set to now-30d at fixture construction time so the
	// retention check always fires regardless of test date.
	WarnDBISnapPastRetentionID  = "rds:retention-test-2026-03-25"
	WarnDBISnapPastRetentionARN = "arn:aws:rds:us-east-1:123456789012:snapshot:rds:retention-test-2026-03-25"

	// MultiW1DBISnapID — Encrypted=false + orphan (parent not in dbi list).
	// Expected Status: "unencrypted (+1)".
	MultiW1DBISnapID  = "multi-orphan-unenc-snap"
	MultiW1DBISnapARN = "arn:aws:rds:us-east-1:123456789012:snapshot:multi-orphan-unenc-snap"

	// BackupCoveredDBISnapID — AWS Backup-prefixed identifier; backup pivot target.
	BackupCoveredDBISnapID  = "awsbackup:job-deadbeef-snap"
	BackupCoveredDBISnapARN = "arn:aws:rds:us-east-1:123456789012:snapshot:awsbackup:job-deadbeef-snap"

	// SeverityBrokenWarnDBISnapID — Broken beats Warning.
	// Status=failed + Encrypted=false → Status phrase = "failed" (Broken wins;
	// Encrypted=false is suppressed when Status is a non-available end-state).
	SeverityBrokenWarnDBISnapID  = "failed-with-unenc-snap"
	SeverityBrokenWarnDBISnapARN = "arn:aws:rds:us-east-1:123456789012:snapshot:failed-with-unenc-snap"
)

// NewDBISnapFixtures constructs DBISnapFixtures from the canonical demo data.
var sharedDBISnapFixtures = shared(func() *DBISnapFixtures {
	return &DBISnapFixtures{
		Instances: buildDBISnapInstances(),
	}
})

func NewDBISnapFixtures() *DBISnapFixtures {
	return sharedDBISnapFixtures()
}

func buildDBISnapInstances() []rdstypes.DBSnapshot {
	// now-30d for WarnDBISnapPastRetentionID — always 30 days old relative to runtime.
	pastRetentionTime := time.Now().UTC().Add(-30 * 24 * time.Hour)
	// now-3d for healthy automated snapshots — always within the 7-day BackupRetentionPeriod.
	recentSnapTime := time.Now().UTC().Add(-3 * 24 * time.Hour)

	return []rdstypes.DBSnapshot{
		// DBISnapPublic carries dbi-snap.public: the RDS fake reports
		// its restore attribute as granting the "all" group.
		{
			DBSnapshotIdentifier: aws.String(DBISnapPublic),
			DBSnapshotArn:        aws.String("arn:aws:rds:us-east-1:123456789012:snapshot:" + DBISnapPublic),
			DBInstanceIdentifier: aws.String(ProdDbiID),
			Status:               aws.String("available"),
			Engine:               aws.String("postgres"),
			EngineVersion:        aws.String("16.2"),
			SnapshotType:         aws.String("manual"),
			SnapshotCreateTime:   aws.Time(recentSnapTime),
			AllocatedStorage:     aws.Int32(100),
			StorageType:          aws.String("gp3"),
			Encrypted:            aws.Bool(true),
			KmsKeyId:             aws.String(dbiKMSKeyID),
			AvailabilityZone:     aws.String("us-east-1a"),
			MasterUsername:       aws.String("pgadmin"),
			LicenseModel:         aws.String("postgresql-license"),
			PercentProgress:      aws.Int32(100),
			SourceRegion:         aws.String("us-east-1"),
		},

		// ProdDBISnapID's SnapshotCreateTime is dynamic (now-3d) to stay within
		// the parent's 7-day retention.
		{
			DBSnapshotIdentifier: aws.String(ProdDBISnapID),
			DBSnapshotArn:        aws.String(ProdDBISnapARN),
			DBInstanceIdentifier: aws.String(ProdDbiID),
			Status:               aws.String("available"),
			Engine:               aws.String("postgres"),
			EngineVersion:        aws.String("16.2"),
			SnapshotType:         aws.String("automated"),
			SnapshotCreateTime:   aws.Time(recentSnapTime),
			AllocatedStorage:     aws.Int32(100),
			StorageType:          aws.String("gp3"),
			Encrypted:            aws.Bool(true),
			KmsKeyId:             aws.String(dbiKMSKeyID),
			AvailabilityZone:     aws.String("us-east-1a"),
			MasterUsername:       aws.String("pgadmin"),
			LicenseModel:         aws.String("postgresql-license"),
			PercentProgress:      aws.Int32(100),
			SourceRegion:         aws.String("us-east-1"),
		},

		// Carries DbiResourceId, which AWS keeps with an instance across a
		// rename and never reissues, so it is matched on that alone and the
		// name beside it is not read.
		{
			DBSnapshotIdentifier: aws.String(ResourceIDDBISnapID),
			DBSnapshotArn:        aws.String(ResourceIDDBISnapARN),
			DBInstanceIdentifier: aws.String(ProdDbiID),
			DbiResourceId:        aws.String(ProdDbiResourceID),
			Status:               aws.String("available"),
			Engine:               aws.String("postgres"),
			EngineVersion:        aws.String("16.2"),
			SnapshotType:         aws.String("manual"),
			SnapshotCreateTime:   aws.Time(mustTime("2026-02-11T02:15:00Z")),
			AllocatedStorage:     aws.Int32(100),
			StorageType:          aws.String("gp3"),
			Encrypted:            aws.Bool(true),
			KmsKeyId:             aws.String(dbiKMSKeyID),
			AvailabilityZone:     aws.String("us-east-1a"),
			MasterUsername:       aws.String("pgadmin"),
			LicenseModel:         aws.String("postgresql-license"),
			PercentProgress:      aws.Int32(100),
			SourceRegion:         aws.String("us-east-1"),
		},

		{
			DBSnapshotIdentifier: aws.String(WarnDBISnapCreatingID),
			DBSnapshotArn:        aws.String(WarnDBISnapCreatingARN),
			DBInstanceIdentifier: aws.String("dev-feature-branch"),
			Status:               aws.String("creating"),
			Engine:               aws.String("aurora-postgresql"),
			EngineVersion:        aws.String("16.4"),
			SnapshotType:         aws.String("manual"),
			SnapshotCreateTime:   aws.Time(mustTime("2026-04-25T10:30:00Z")),
			AllocatedStorage:     aws.Int32(20),
			StorageType:          aws.String("aurora"),
			Encrypted:            aws.Bool(true),
			KmsKeyId:             aws.String(dbiKMSKeyID),
			AvailabilityZone:     aws.String("us-east-1a"),
			MasterUsername:       aws.String("pgadmin"),
			LicenseModel:         aws.String("postgresql-license"),
			PercentProgress:      aws.Int32(42),
			SourceRegion:         aws.String("us-east-1"),
		},

		// Status=copying is a state the predicate does not name. Encrypted
		// so it carries the transitional finding alone.
		{
			DBSnapshotIdentifier:       aws.String(WarnDBISnapCopyingID),
			DBSnapshotArn:              aws.String(WarnDBISnapCopyingARN),
			DBInstanceIdentifier:       aws.String(ProdDbiID),
			Status:                     aws.String("copying"),
			Engine:                     aws.String("aurora-postgresql"),
			EngineVersion:              aws.String("16.4"),
			SnapshotType:               aws.String("manual"),
			SnapshotCreateTime:         aws.Time(mustTime("2026-04-26T09:00:00Z")),
			AllocatedStorage:           aws.Int32(20),
			StorageType:                aws.String("aurora"),
			Encrypted:                  aws.Bool(true),
			KmsKeyId:                   aws.String(dbiKMSKeyID),
			AvailabilityZone:           aws.String("us-east-1a"),
			MasterUsername:             aws.String("pgadmin"),
			LicenseModel:               aws.String("postgresql-license"),
			PercentProgress:            aws.Int32(70),
			SourceRegion:               aws.String("us-west-2"),
			SourceDBSnapshotIdentifier: aws.String("arn:aws:rds:us-west-2:123456789012:snapshot:prod-dbi-1-2026-04-26"),
		},

		{
			DBSnapshotIdentifier: aws.String(BrokenDBISnapFailedID),
			DBSnapshotArn:        aws.String(BrokenDBISnapFailedARN),
			DBInstanceIdentifier: aws.String(ProdDbiID),
			Status:               aws.String("failed"),
			Engine:               aws.String("postgres"),
			EngineVersion:        aws.String("16.2"),
			SnapshotType:         aws.String("manual"),
			SnapshotCreateTime:   aws.Time(mustTime("2026-04-22T10:00:00Z")),
			AllocatedStorage:     aws.Int32(100),
			StorageType:          aws.String("gp3"),
			Encrypted:            aws.Bool(true),
			KmsKeyId:             aws.String(dbiKMSKeyID),
			AvailabilityZone:     aws.String("us-east-1a"),
			MasterUsername:       aws.String("pgadmin"),
			LicenseModel:         aws.String("postgresql-license"),
			PercentProgress:      aws.Int32(0),
			SourceRegion:         aws.String("us-east-1"),
		},

		{
			DBSnapshotIdentifier: aws.String(BrokenDBISnapIncompatibleID),
			DBSnapshotArn:        aws.String(BrokenDBISnapIncompatibleARN),
			DBInstanceIdentifier: aws.String(ProdDbiID),
			Status:               aws.String("incompatible-restore"),
			Engine:               aws.String("mysql"),
			EngineVersion:        aws.String("8.0.36"),
			SnapshotType:         aws.String("manual"),
			SnapshotCreateTime:   aws.Time(mustTime("2026-04-10T22:00:00Z")),
			AllocatedStorage:     aws.Int32(100),
			StorageType:          aws.String("gp2"),
			Encrypted:            aws.Bool(true),
			KmsKeyId:             aws.String(dbiKMSKeyID),
			AvailabilityZone:     aws.String("us-east-1a"),
			MasterUsername:       aws.String("mysqladmin"),
			LicenseModel:         aws.String("general-public-license"),
			PercentProgress:      aws.Int32(0),
			SourceRegion:         aws.String("us-east-1"),
		},

		{
			DBSnapshotIdentifier: aws.String(WarnDBISnapUnencryptedID),
			DBSnapshotArn:        aws.String(WarnDBISnapUnencryptedARN),
			DBInstanceIdentifier: aws.String(ProdDbiID),
			Status:               aws.String("available"),
			Engine:               aws.String("postgres"),
			EngineVersion:        aws.String("16.2"),
			SnapshotType:         aws.String("manual"),
			SnapshotCreateTime:   aws.Time(mustTime("2026-04-10T22:00:00Z")),
			AllocatedStorage:     aws.Int32(100),
			StorageType:          aws.String("gp3"),
			Encrypted:            aws.Bool(false),
			KmsKeyId:             nil,
			AvailabilityZone:     aws.String("us-east-1a"),
			MasterUsername:       aws.String("pgadmin"),
			LicenseModel:         aws.String("postgresql-license"),
			PercentProgress:      aws.Int32(100),
			SourceRegion:         aws.String("us-east-1"),
		},

		{
			DBSnapshotIdentifier: aws.String(WarnDBISnapOrphanID),
			DBSnapshotArn:        aws.String(WarnDBISnapOrphanARN),
			DBInstanceIdentifier: aws.String("deleted-legacy-db"),
			Status:               aws.String("available"),
			Engine:               aws.String("mysql"),
			EngineVersion:        aws.String("8.0.36"),
			SnapshotType:         aws.String("manual"),
			SnapshotCreateTime:   aws.Time(mustTime("2026-03-15T03:00:00Z")),
			AllocatedStorage:     aws.Int32(50),
			StorageType:          aws.String("gp2"),
			Encrypted:            aws.Bool(true),
			KmsKeyId:             aws.String(dbiKMSKeyID),
			AvailabilityZone:     aws.String("us-east-1a"),
			MasterUsername:       aws.String("mysqladmin"),
			LicenseModel:         aws.String("general-public-license"),
			PercentProgress:      aws.Int32(100),
			SourceRegion:         aws.String("us-east-1"),
		},

		// SnapshotCreateTime computed relative to time.Now() so the enricher
		// always sees this as past-retention regardless of test date.
		{
			DBSnapshotIdentifier: aws.String(WarnDBISnapPastRetentionID),
			DBSnapshotArn:        aws.String(WarnDBISnapPastRetentionARN),
			DBInstanceIdentifier: aws.String(ProdDbiRetentionParentID),
			Status:               aws.String("available"),
			Engine:               aws.String("postgres"),
			EngineVersion:        aws.String("16.2"),
			SnapshotType:         aws.String("automated"),
			SnapshotCreateTime:   aws.Time(pastRetentionTime),
			AllocatedStorage:     aws.Int32(100),
			StorageType:          aws.String("gp3"),
			Encrypted:            aws.Bool(true),
			KmsKeyId:             aws.String(dbiKMSKeyID),
			AvailabilityZone:     aws.String("us-east-1a"),
			MasterUsername:       aws.String("pgadmin"),
			LicenseModel:         aws.String("postgresql-license"),
			PercentProgress:      aws.Int32(100),
			SourceRegion:         aws.String("us-east-1"),
		},

		// DBInstanceIdentifier="deleted-legacy-db" is NOT in the dbi list.
		// Expected Status: "unencrypted (+1)".
		{
			DBSnapshotIdentifier: aws.String(MultiW1DBISnapID),
			DBSnapshotArn:        aws.String(MultiW1DBISnapARN),
			DBInstanceIdentifier: aws.String("deleted-legacy-db"),
			Status:               aws.String("available"),
			Engine:               aws.String("mysql"),
			EngineVersion:        aws.String("8.0.36"),
			SnapshotType:         aws.String("manual"),
			SnapshotCreateTime:   aws.Time(mustTime("2026-04-01T03:00:00Z")),
			AllocatedStorage:     aws.Int32(50),
			StorageType:          aws.String("gp2"),
			Encrypted:            aws.Bool(false),
			KmsKeyId:             nil,
			AvailabilityZone:     aws.String("us-east-1a"),
			MasterUsername:       aws.String("mysqladmin"),
			LicenseModel:         aws.String("general-public-license"),
			PercentProgress:      aws.Int32(100),
			SourceRegion:         aws.String("us-east-1"),
		},

		// backup.go holds 2 recovery points for BackupCoveredDBISnapARN.
		{
			DBSnapshotIdentifier: aws.String(BackupCoveredDBISnapID),
			DBSnapshotArn:        aws.String(BackupCoveredDBISnapARN),
			DBInstanceIdentifier: aws.String(ProdDbiID),
			Status:               aws.String("available"),
			Engine:               aws.String("postgres"),
			EngineVersion:        aws.String("16.2"),
			SnapshotType:         aws.String("manual"),
			SnapshotCreateTime:   aws.Time(mustTime("2026-04-18T03:00:00Z")),
			AllocatedStorage:     aws.Int32(100),
			StorageType:          aws.String("gp3"),
			Encrypted:            aws.Bool(true),
			KmsKeyId:             aws.String(dbiKMSKeyID),
			AvailabilityZone:     aws.String("us-east-1a"),
			MasterUsername:       aws.String("pgadmin"),
			LicenseModel:         aws.String("postgresql-license"),
			PercentProgress:      aws.Int32(100),
			SourceRegion:         aws.String("us-east-1"),
		},

		// Status=failed + Encrypted=false → phrase = "failed" (Broken wins;
		// Encrypted=false suppressed when Status is a non-available end-state).
		{
			DBSnapshotIdentifier: aws.String(SeverityBrokenWarnDBISnapID),
			DBSnapshotArn:        aws.String(SeverityBrokenWarnDBISnapARN),
			DBInstanceIdentifier: aws.String(ProdDbiID),
			Status:               aws.String("failed"),
			Engine:               aws.String("postgres"),
			EngineVersion:        aws.String("16.2"),
			SnapshotType:         aws.String("manual"),
			SnapshotCreateTime:   aws.Time(mustTime("2026-04-23T10:00:00Z")),
			AllocatedStorage:     aws.Int32(100),
			StorageType:          aws.String("gp3"),
			Encrypted:            aws.Bool(false),
			KmsKeyId:             nil,
			AvailabilityZone:     aws.String("us-east-1a"),
			MasterUsername:       aws.String("pgadmin"),
			LicenseModel:         aws.String("postgresql-license"),
			PercentProgress:      aws.Int32(0),
			SourceRegion:         aws.String("us-east-1"),
		},
	}
}
