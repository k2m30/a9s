// Package fixtures provides RDS DB Instance fixture data for the RDS fake.
package fixtures

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
)

// DBIFixtures holds typed fixture data for RDS DB Instances.
type DBIFixtures struct {
	// Instances is the full list returned by DescribeDBInstances.
	Instances []rdstypes.DBInstance
	// PendingMaintenanceActions is returned by DescribePendingMaintenanceActions.
	PendingMaintenanceActions []rdstypes.ResourcePendingMaintenanceActions
}

// Stable IDs and ARNs for DBI fixtures — imported by sibling fixture files.
const (
	// prod-dbi-1 — baseline Healthy, graph-connected
	ProdDbiID  = "prod-dbi-1"
	ProdDbiARN = "arn:aws:rds:us-east-1:123456789012:db:prod-dbi-1"

	// prod-dbi-aurora-1 — Aurora cluster member
	ProdDbiAuroraID  = "prod-dbi-aurora-1"
	ProdDbiAuroraARN = "arn:aws:rds:us-east-1:123456789012:db:prod-dbi-aurora-1"

	// staging-dbi-modifying — Warning (transitional with pending class change)
	StagingDbiModifyingID  = "staging-dbi-modifying"
	StagingDbiModifyingARN = "arn:aws:rds:us-east-1:123456789012:db:staging-dbi-modifying"

	// staging-dbi-rebooting — Warning (transitional, no pending values)
	StagingDbiRebootingID  = "staging-dbi-rebooting"
	StagingDbiRebootingARN = "arn:aws:rds:us-east-1:123456789012:db:staging-dbi-rebooting"

	// broken-dbi-storage-full — Broken
	BrokenDbiStorageFullID  = "broken-dbi-storage-full"
	BrokenDbiStorageFullARN = "arn:aws:rds:us-east-1:123456789012:db:broken-dbi-storage-full"

	// broken-dbi-encryption-locked — Broken (inaccessible-encryption-credentials)
	BrokenDbiEncryptionLockedID  = "broken-dbi-encryption-locked"
	BrokenDbiEncryptionLockedARN = "arn:aws:rds:us-east-1:123456789012:db:broken-dbi-encryption-locked"

	// warn-dbi-no-backups — Warning (BackupRetentionPeriod=0)
	WarnDbiNoBackupsID  = "warn-dbi-no-backups"
	WarnDbiNoBackupsARN = "arn:aws:rds:us-east-1:123456789012:db:warn-dbi-no-backups"

	// warn-dbi-public — Warning (CIS RDS.2)
	WarnDbiPublicID  = "warn-dbi-public"
	WarnDbiPublicARN = "arn:aws:rds:us-east-1:123456789012:db:warn-dbi-public"

	// warn-dbi-unencrypted — Warning (CIS RDS.3)
	WarnDbiUnencryptedID  = "warn-dbi-unencrypted"
	WarnDbiUnencryptedARN = "arn:aws:rds:us-east-1:123456789012:db:warn-dbi-unencrypted"

	// warn-dbi-unprotected — Warning (DeletionProtection=false)
	WarnDbiUnprotectedID  = "warn-dbi-unprotected"
	WarnDbiUnprotectedARN = "arn:aws:rds:us-east-1:123456789012:db:warn-dbi-unprotected"

	// maint-dbi-scheduled — Healthy + pending maintenance action
	MaintDbiScheduledID  = "maint-dbi-scheduled"
	MaintDbiScheduledARN = "arn:aws:rds:us-east-1:123456789012:db:maint-dbi-scheduled"

	// warn-dbi-multi — 3 Wave 1 warnings stacked (no-backups + public + unencrypted)
	WarnDbiMultiID  = "warn-dbi-multi"
	WarnDbiMultiARN = "arn:aws:rds:us-east-1:123456789012:db:warn-dbi-multi"

	// warn-dbi-public-maint — Wave 1 warning (publicly accessible) + Wave 2 maintenance
	WarnDbiPublicMaintID  = "warn-dbi-public-maint"
	WarnDbiPublicMaintARN = "arn:aws:rds:us-east-1:123456789012:db:warn-dbi-public-maint"

	// ProdDbiRetentionParentID — healthy DBI with BackupRetentionPeriod=7.
	// Used by rds-snap fixtures to test the "automated snapshot past retention" enricher signal.
	ProdDbiRetentionParentID  = "prod-dbi-retention-parent"
	ProdDbiRetentionParentARN = "arn:aws:rds:us-east-1:123456789012:db:prod-dbi-retention-parent"

	// Shared fixture constants
	dbiKMSKeyID       = "arn:aws:kms:us-east-1:123456789012:key/a1b2c3d4-5678-90ab-cdef-111111111111"
	dbiDeadbeefKeyARN = "arn:aws:kms:us-east-1:123456789012:key/deadbeef-0000-0000-0000-000000000000"

	dbiProdSGID        = "sg-0ccc333333333333c"
	dbiProdVPCID       = "vpc-0abc123def456789a"
	dbiSubnetGroup     = "acme-rds-subnet-group"
	dbiProdSubnetA     = "subnet-0aaa111111111111a"
	dbiProdSubnetB     = "subnet-0ccc333333333333c"
	dbiAuroraClusterID = "prod-aurora-cluster"

	// ProdDbiMasterSecretARN is the Secrets Manager ARN for prod-dbi-1's RDS-managed password.
	ProdDbiMasterSecretARN = "arn:aws:secretsmanager:us-east-1:123456789012:secret:rds!db-prod-dbi-1-ABCDEF"

	// ProdDbiAuroraMasterSecretARN is the Secrets Manager ARN for
	// prod-dbi-aurora-1's RDS-managed password (used so the Aurora fixture
	// covers the dbi→secrets pivot on a single fixture).
	ProdDbiAuroraMasterSecretARN = "arn:aws:secretsmanager:us-east-1:123456789012:secret:rds!db-prod-dbi-aurora-1-GHIJKL"

	dbiMonitoringRoleARN    = "arn:aws:iam::123456789012:role/rds-monitoring-role"
	dbiEnhancedMonitorARN   = "arn:aws:iam::123456789012:role/rds-enhanced-monitoring"
)

// NewDBIFixtures builds and returns a fully-populated DBIFixtures struct.
// The Instances slice contains all non-adversarial fixtures from the spec §2.
// PendingMaintenanceActions contains the Wave 2 enrichment data for maint-dbi-scheduled.
func NewDBIFixtures() *DBIFixtures {
	return &DBIFixtures{
		Instances:                 buildDBIInstances(),
		PendingMaintenanceActions: buildDBIPendingMaintenance(),
	}
}

// dbiBaselineHealthy builds a fully-configured healthy DBInstance.
// Callers mutate specific fields for variant fixtures.
func dbiBaselineHealthy(id, arn string) rdstypes.DBInstance {
	return rdstypes.DBInstance{
		DBInstanceIdentifier:       aws.String(id),
		DBInstanceArn:              aws.String(arn),
		Engine:                     aws.String("postgres"),
		EngineVersion:              aws.String("16.2"),
		DBInstanceStatus:           aws.String("available"),
		DBInstanceClass:            aws.String("db.r6g.large"),
		MasterUsername:             aws.String("pgadmin"),
		AvailabilityZone:           aws.String("us-east-1a"),
		AllocatedStorage:           aws.Int32(100),
		StorageType:                aws.String("gp3"),
		StorageEncrypted:           aws.Bool(true),
		KmsKeyId:                   aws.String(dbiKMSKeyID),
		Iops:                       aws.Int32(3000),
		BackupRetentionPeriod:      aws.Int32(7),
		PreferredBackupWindow:      aws.String("03:00-04:00"),
		PreferredMaintenanceWindow: aws.String("sun:05:00-sun:06:00"),
		DeletionProtection:         aws.Bool(true),
		PubliclyAccessible:         aws.Bool(false),
		MultiAZ:                    aws.Bool(true),
		PerformanceInsightsEnabled: aws.Bool(true),
		Endpoint: &rdstypes.Endpoint{
			Address: aws.String(id + ".xxxxxxx.us-east-1.rds.amazonaws.com"),
			Port:    aws.Int32(5432),
		},
		VpcSecurityGroups: []rdstypes.VpcSecurityGroupMembership{
			{VpcSecurityGroupId: aws.String(dbiProdSGID), Status: aws.String("active")},
		},
		DBSubnetGroup: &rdstypes.DBSubnetGroup{
			DBSubnetGroupName: aws.String(dbiSubnetGroup),
			VpcId:             aws.String(dbiProdVPCID),
			Subnets: []rdstypes.Subnet{
				{SubnetIdentifier: aws.String(dbiProdSubnetA)},
				{SubnetIdentifier: aws.String(dbiProdSubnetB)},
			},
		},
		TagList: []rdstypes.Tag{
			{Key: aws.String("Environment"), Value: aws.String("production")},
		},
	}
}

func buildDBIInstances() []rdstypes.DBInstance {
	// 1. prod-dbi-1 — baseline Healthy, graph-connected
	prodDbi1 := dbiBaselineHealthy(ProdDbiID, ProdDbiARN)
	prodDbi1.MasterUserSecret = &rdstypes.MasterUserSecret{
		SecretArn: aws.String(ProdDbiMasterSecretARN),
	}
	prodDbi1.AssociatedRoles = []rdstypes.DBInstanceRole{
		{RoleArn: aws.String(dbiMonitoringRoleARN), FeatureName: aws.String("Monitoring")},
	}
	prodDbi1.MonitoringRoleArn = aws.String(dbiEnhancedMonitorARN)
	prodDbi1.EnabledCloudwatchLogsExports = []string{"postgresql", "upgrade"}

	// 2. prod-dbi-aurora-1 — Aurora cluster member, Healthy.
	// This fixture is the "all related pivots non-zero" graph-root for dbi
	// (asserted in tests/integration/scenario_dbi_visual_test.go). Aurora
	// supports all the optional fields below — cluster-level secret + instance
	// MasterUserSecret may coexist, Enhanced Monitoring works per-instance,
	// associated roles support S3 import/export workflows, and log exports
	// are valid per-instance.
	auroraBase := dbiBaselineHealthy(ProdDbiAuroraID, ProdDbiAuroraARN)
	auroraBase.Engine = aws.String("aurora-postgresql")
	auroraBase.EngineVersion = aws.String("16.4")
	auroraBase.DBClusterIdentifier = aws.String(dbiAuroraClusterID)
	auroraBase.StorageType = aws.String("aurora")
	auroraBase.MasterUserSecret = &rdstypes.MasterUserSecret{
		SecretArn: aws.String(ProdDbiAuroraMasterSecretARN),
	}
	auroraBase.AssociatedRoles = []rdstypes.DBInstanceRole{
		{RoleArn: aws.String(dbiMonitoringRoleARN), FeatureName: aws.String("Monitoring")},
	}
	auroraBase.MonitoringRoleArn = aws.String(dbiEnhancedMonitorARN)
	auroraBase.EnabledCloudwatchLogsExports = []string{"postgresql", "upgrade"}

	// 3. staging-dbi-modifying — Warning (transitional with pending class change)
	modifying := dbiBaselineHealthy(StagingDbiModifyingID, StagingDbiModifyingARN)
	modifying.DBInstanceStatus = aws.String("modifying")
	modifying.PendingModifiedValues = &rdstypes.PendingModifiedValues{
		DBInstanceClass: aws.String("db.r6g.xlarge"),
	}
	modifying.TagList = []rdstypes.Tag{
		{Key: aws.String("Environment"), Value: aws.String("staging")},
	}

	// 4. staging-dbi-rebooting — Warning (transitional, no pending values)
	rebooting := dbiBaselineHealthy(StagingDbiRebootingID, StagingDbiRebootingARN)
	rebooting.DBInstanceStatus = aws.String("rebooting")
	rebooting.TagList = []rdstypes.Tag{
		{Key: aws.String("Environment"), Value: aws.String("staging")},
	}

	// 5. broken-dbi-storage-full — Broken
	storageFull := dbiBaselineHealthy(BrokenDbiStorageFullID, BrokenDbiStorageFullARN)
	storageFull.DBInstanceStatus = aws.String("storage-full")

	// 6. broken-dbi-encryption-locked — Broken (inaccessible-encryption-credentials)
	encLocked := dbiBaselineHealthy(BrokenDbiEncryptionLockedID, BrokenDbiEncryptionLockedARN)
	encLocked.DBInstanceStatus = aws.String("inaccessible-encryption-credentials")
	encLocked.StorageEncrypted = aws.Bool(true)
	encLocked.KmsKeyId = aws.String(dbiDeadbeefKeyARN)

	// 7. warn-dbi-no-backups — Warning (BackupRetentionPeriod=0)
	noBackups := dbiBaselineHealthy(WarnDbiNoBackupsID, WarnDbiNoBackupsARN)
	noBackups.BackupRetentionPeriod = aws.Int32(0)

	// 8. warn-dbi-public — Warning (CIS RDS.2 — publicly accessible)
	public := dbiBaselineHealthy(WarnDbiPublicID, WarnDbiPublicARN)
	public.PubliclyAccessible = aws.Bool(true)

	// 9. warn-dbi-unencrypted — Warning (CIS RDS.3 — unencrypted storage)
	unencrypted := dbiBaselineHealthy(WarnDbiUnencryptedID, WarnDbiUnencryptedARN)
	unencrypted.StorageEncrypted = aws.Bool(false)
	unencrypted.KmsKeyId = nil

	// 10. warn-dbi-unprotected — Warning (DeletionProtection=false)
	unprotected := dbiBaselineHealthy(WarnDbiUnprotectedID, WarnDbiUnprotectedARN)
	unprotected.DeletionProtection = aws.Bool(false)

	// 11. maint-dbi-scheduled — Healthy + pending maintenance (Wave 2 enricher target)
	maintScheduled := dbiBaselineHealthy(MaintDbiScheduledID, MaintDbiScheduledARN)

	// 12. warn-dbi-multi — 3 Wave 1 warnings stacked (backups=0, public=true, unencrypted)
	// DeletionProtection=true so only 3 of the 4 warnings fire.
	// Expected Wave 1 Status: "no automated backups (+2)"
	warnMulti := dbiBaselineHealthy(WarnDbiMultiID, WarnDbiMultiARN)
	warnMulti.BackupRetentionPeriod = aws.Int32(0)
	warnMulti.PubliclyAccessible = aws.Bool(true)
	warnMulti.StorageEncrypted = aws.Bool(false)
	warnMulti.KmsKeyId = nil
	warnMulti.DeletionProtection = aws.Bool(true)

	// 13. warn-dbi-public-maint — Wave 1 warning (public) + Wave 2 maintenance
	// Expected Wave 1 Status: "publicly accessible"
	// Expected Status after Wave 2 enrichment: "publicly accessible (+1)"
	warnPublicMaint := dbiBaselineHealthy(WarnDbiPublicMaintID, WarnDbiPublicMaintARN)
	warnPublicMaint.PubliclyAccessible = aws.Bool(true)

	// 14. prod-dbi-retention-parent — Healthy DBI with BackupRetentionPeriod=7.
	// Used as the parent of WarnRDSSnapPastRetentionID in rds-snap fixtures to
	// trigger the "automated snapshot past BackupRetentionPeriod" enricher signal.
	retentionParent := dbiBaselineHealthy(ProdDbiRetentionParentID, ProdDbiRetentionParentARN)
	retentionParent.BackupRetentionPeriod = aws.Int32(7)

	return []rdstypes.DBInstance{
		prodDbi1,
		auroraBase,
		modifying,
		rebooting,
		storageFull,
		encLocked,
		noBackups,
		public,
		unencrypted,
		unprotected,
		maintScheduled,
		warnMulti,
		warnPublicMaint,
		retentionParent,
	}
}

func buildDBIPendingMaintenance() []rdstypes.ResourcePendingMaintenanceActions {
	autoApplied := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	// warn-dbi-public-maint auto-apply date is in the past to trigger "overdue" summary.
	publicMaintDate := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)
	return []rdstypes.ResourcePendingMaintenanceActions{
		{
			ResourceIdentifier: aws.String(MaintDbiScheduledARN),
			PendingMaintenanceActionDetails: []rdstypes.PendingMaintenanceAction{
				{
					Action:               aws.String("system-update"),
					Description:          aws.String("New minor engine patch 16.2.3"),
					AutoAppliedAfterDate: aws.Time(autoApplied),
				},
			},
		},
		// warn-dbi-public-maint: Wave 1 (publicly accessible) + Wave 2 (maintenance overdue).
		// Expected Status after enrichment: "publicly accessible (+1)".
		{
			ResourceIdentifier: aws.String(WarnDbiPublicMaintARN),
			PendingMaintenanceActionDetails: []rdstypes.PendingMaintenanceAction{
				{
					Action:               aws.String("os-upgrade"),
					Description:          aws.String("Kernel security patch"),
					AutoAppliedAfterDate: aws.Time(publicMaintDate),
				},
			},
		},
	}
}
