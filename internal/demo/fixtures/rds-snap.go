// Package fixtures provides RDS Snapshot fixture data for the RDS fake.
package fixtures

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
)

// RDSSnapFixtures holds typed fixture data for RDS DB Snapshots.
type RDSSnapFixtures struct {
	// Instances is the full list returned by DescribeDBSnapshots.
	Instances []rdstypes.DBSnapshot
}

// Stable IDs and ARNs for RDS Snapshot fixtures — imported by sibling fixture files.
//
// §9.3 structural cap: rds-snap pivot data model is 1:1 for dbi, kms, and dbc (a
// snapshot has exactly one source instance, one encryption key, and at most one
// cluster). The universal "≥50% Count ≥ 2" rule is structurally unsatisfiable for
// this resource type. The graph-root ProdRDSSnapAuroraID achieves backup=2 which is
// the realistic maximum for any Count-≥-2 pivot. Phase 9.3 must accept this as a
// documented structural exemption and not require ≥50% of count-shown:yes pivots
// to reach Count ≥ 2.
const (
	// ProdRDSSnapID — Healthy non-Aurora automated snapshot of prod-dbi-1.
	// dbi pivot: Count=1 (ProdDbiID). kms pivot: Count=1 (dbiKMSKeyID).
	// dbc pivot: Count=0 (non-Aurora, no DBClusterIdentifier on parent).
	ProdRDSSnapID  = "rds:prod-dbi-1-2026-04-15"
	ProdRDSSnapARN = "arn:aws:rds:us-east-1:123456789012:snapshot:rds:prod-dbi-1-2026-04-15"

	// ProdRDSSnapAuroraID — graph-root for §9.3: Healthy Aurora automated snapshot.
	// dbi pivot: Count=1 (ProdDbiAuroraID).
	// kms pivot: Count=1 (dbiKMSKeyID).
	// dbc pivot: Count=1 (prod-aurora-cluster / ProdDbcID).
	// backup pivot: Count=2 (two recovery points added to backup.go).
	// ct-events pivot: ≥3 events added to cloudtrail.go (count "unknown" / exempt).
	ProdRDSSnapAuroraID  = "rds:prod-dbi-aurora-1-2026-04-15"
	ProdRDSSnapAuroraARN = "arn:aws:rds:us-east-1:123456789012:snapshot:rds:prod-dbi-aurora-1-2026-04-15"

	// WarnRDSSnapCreatingID — Wave-1 warning: Status=creating, PercentProgress=42.
	WarnRDSSnapCreatingID  = "dev-feature-branch-snap"
	WarnRDSSnapCreatingARN = "arn:aws:rds:us-east-1:123456789012:snapshot:dev-feature-branch-snap"

	// BrokenRDSSnapFailedID — Broken: Status=failed.
	BrokenRDSSnapFailedID  = "prod-dbi-1-failed-snap"
	BrokenRDSSnapFailedARN = "arn:aws:rds:us-east-1:123456789012:snapshot:prod-dbi-1-failed-snap"

	// BrokenRDSSnapIncompatibleID — Broken: Status=incompatible-restore.
	BrokenRDSSnapIncompatibleID  = "legacy-mysql-snap-incompatible"
	BrokenRDSSnapIncompatibleARN = "arn:aws:rds:us-east-1:123456789012:snapshot:legacy-mysql-snap-incompatible"

	// WarnRDSSnapUnencryptedID — Warning: Encrypted=false.
	WarnRDSSnapUnencryptedID  = "unenc-pre-migration-snap"
	WarnRDSSnapUnencryptedARN = "arn:aws:rds:us-east-1:123456789012:snapshot:unenc-pre-migration-snap"

	// WarnRDSSnapOrphanID — Warning orphan: parent DBInstanceIdentifier not in dbi list.
	WarnRDSSnapOrphanID  = "orphan-deleted-db-snap"
	WarnRDSSnapOrphanARN = "arn:aws:rds:us-east-1:123456789012:snapshot:orphan-deleted-db-snap"

	// WarnRDSSnapPastRetentionID — Warning: automated snapshot older than parent's
	// BackupRetentionPeriod (parent = WarnDbiPastRetentionParentID with retention=7).
	// SnapshotCreateTime is set to now-30d at fixture construction time so the
	// retention check always fires regardless of test date.
	WarnRDSSnapPastRetentionID  = "rds:retention-test-2026-03-25"
	WarnRDSSnapPastRetentionARN = "arn:aws:rds:us-east-1:123456789012:snapshot:rds:retention-test-2026-03-25"

	// MultiW1RDSSnapID — U7a multi-W1: Encrypted=false + orphan (parent not in dbi list).
	// Expected Status: "unencrypted (+1)".
	MultiW1RDSSnapID  = "multi-orphan-unenc-snap"
	MultiW1RDSSnapARN = "arn:aws:rds:us-east-1:123456789012:snapshot:multi-orphan-unenc-snap"

	// BackupCoveredRDSSnapID — AWS Backup-prefixed identifier; backup pivot pivot target.
	BackupCoveredRDSSnapID  = "awsbackup:job-deadbeef-snap"
	BackupCoveredRDSSnapARN = "arn:aws:rds:us-east-1:123456789012:snapshot:awsbackup:job-deadbeef-snap"

	// SeverityBrokenWarnRDSSnapID — U8 severity: Broken beats Warning.
	// Status=failed + Encrypted=false → Status phrase = "failed" (Broken wins;
	// Encrypted=false is suppressed when Status is a non-available end-state).
	SeverityBrokenWarnRDSSnapID  = "failed-with-unenc-snap"
	SeverityBrokenWarnRDSSnapARN = "arn:aws:rds:us-east-1:123456789012:snapshot:failed-with-unenc-snap"
)

// NewRDSSnapFixtures constructs RDSSnapFixtures from the canonical demo data.
// Every fixture in the impl-plan §2 is represented here; adversarial fixtures
// (nil DBSnapshotIdentifier, nil Status, malformed ARN, nil SnapshotCreateTime)
// stay inline in tests/unit/aws_rds_snap_test.go.
func NewRDSSnapFixtures() *RDSSnapFixtures {
	return &RDSSnapFixtures{
		Instances: buildRDSSnapInstances(),
	}
}

func buildRDSSnapInstances() []rdstypes.DBSnapshot {
	// now-30d for WarnRDSSnapPastRetentionID — always 30 days old relative to runtime.
	pastRetentionTime := time.Now().UTC().Add(-30 * 24 * time.Hour)
	// now-3d for healthy automated snapshots — always within the 7-day BackupRetentionPeriod.
	recentSnapTime := time.Now().UTC().Add(-3 * 24 * time.Hour)

	return []rdstypes.DBSnapshot{
		// 1. ProdRDSSnapID — Healthy non-Aurora automated snapshot of prod-dbi-1.
		// SnapshotCreateTime is dynamic (now-3d) to stay within the parent's 7-day retention.
		{
			DBSnapshotIdentifier: aws.String(ProdRDSSnapID),
			DBSnapshotArn:        aws.String(ProdRDSSnapARN),
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

		// 2. ProdRDSSnapAuroraID — graph-root for §9.3. Healthy Aurora automated snapshot.
		// dbi=ProdDbiAuroraID, kms=dbiKMSKeyID, dbc=prod-aurora-cluster (via auroraBase.DBClusterIdentifier).
		// backup: 2 recovery points added to backup.go pointing at ProdRDSSnapAuroraARN.
		// ct-events: 3 events added to cloudtrail.go with ResourceName=ProdRDSSnapAuroraID.
		// SnapshotCreateTime is dynamic (now-3d) to stay within the parent's 7-day retention.
		{
			DBSnapshotIdentifier: aws.String(ProdRDSSnapAuroraID),
			DBSnapshotArn:        aws.String(ProdRDSSnapAuroraARN),
			DBInstanceIdentifier: aws.String(ProdDbiAuroraID),
			Status:               aws.String("available"),
			Engine:               aws.String("aurora-postgresql"),
			EngineVersion:        aws.String("16.4"),
			SnapshotType:         aws.String("automated"),
			SnapshotCreateTime:   aws.Time(recentSnapTime),
			AllocatedStorage:     aws.Int32(100),
			StorageType:          aws.String("aurora"),
			Encrypted:            aws.Bool(true),
			KmsKeyId:             aws.String(dbiKMSKeyID),
			AvailabilityZone:     aws.String("us-east-1a"),
			MasterUsername:       aws.String("pgadmin"),
			LicenseModel:         aws.String("postgresql-license"),
			PercentProgress:      aws.Int32(100),
			SourceRegion:         aws.String("us-east-1"),
		},

		// 3. WarnRDSSnapCreatingID — Wave-1 warning: Status=creating, PercentProgress=42.
		{
			DBSnapshotIdentifier: aws.String(WarnRDSSnapCreatingID),
			DBSnapshotArn:        aws.String(WarnRDSSnapCreatingARN),
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

		// 4. BrokenRDSSnapFailedID — Broken: Status=failed.
		{
			DBSnapshotIdentifier: aws.String(BrokenRDSSnapFailedID),
			DBSnapshotArn:        aws.String(BrokenRDSSnapFailedARN),
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

		// 5. BrokenRDSSnapIncompatibleID — Broken: Status=incompatible-restore.
		{
			DBSnapshotIdentifier: aws.String(BrokenRDSSnapIncompatibleID),
			DBSnapshotArn:        aws.String(BrokenRDSSnapIncompatibleARN),
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

		// 6. WarnRDSSnapUnencryptedID — Warning: Encrypted=false, parent ProdDbiID present.
		{
			DBSnapshotIdentifier: aws.String(WarnRDSSnapUnencryptedID),
			DBSnapshotArn:        aws.String(WarnRDSSnapUnencryptedARN),
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

		// 7. WarnRDSSnapOrphanID — Warning orphan: parent "deleted-legacy-db" NOT in dbi list.
		{
			DBSnapshotIdentifier: aws.String(WarnRDSSnapOrphanID),
			DBSnapshotArn:        aws.String(WarnRDSSnapOrphanARN),
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

		// 8. WarnRDSSnapPastRetentionID — Warning: automated, 30 days old,
		// parent WarnDbiPastRetentionParentID with BackupRetentionPeriod=7.
		// SnapshotCreateTime computed relative to time.Now() so the enricher
		// always sees this as past-retention regardless of test date.
		{
			DBSnapshotIdentifier: aws.String(WarnRDSSnapPastRetentionID),
			DBSnapshotArn:        aws.String(WarnRDSSnapPastRetentionARN),
			DBInstanceIdentifier: aws.String(WarnDbiPastRetentionParentID),
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

		// 9. MultiW1RDSSnapID — U7a multi-W1: Encrypted=false + orphan.
		// DBInstanceIdentifier="deleted-legacy-db" is NOT in the dbi list.
		// Expected Status: "unencrypted (+1)".
		{
			DBSnapshotIdentifier: aws.String(MultiW1RDSSnapID),
			DBSnapshotArn:        aws.String(MultiW1RDSSnapARN),
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

		// 10. BackupCoveredRDSSnapID — AWS Backup-prefixed identifier.
		// Verifies that identifiers with the "awsbackup:" prefix are handled correctly.
		// backup pivot: 2 recovery points added to backup.go pointing at BackupCoveredRDSSnapARN.
		{
			DBSnapshotIdentifier: aws.String(BackupCoveredRDSSnapID),
			DBSnapshotArn:        aws.String(BackupCoveredRDSSnapARN),
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

		// 11. SeverityBrokenWarnRDSSnapID — U8 severity: Broken beats Warning.
		// Status=failed + Encrypted=false → phrase = "failed" (Broken wins;
		// Encrypted=false suppressed when Status is a non-available end-state).
		{
			DBSnapshotIdentifier: aws.String(SeverityBrokenWarnRDSSnapID),
			DBSnapshotArn:        aws.String(SeverityBrokenWarnRDSSnapARN),
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
