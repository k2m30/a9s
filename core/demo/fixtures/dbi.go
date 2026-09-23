// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// Package fixtures provides RDS DB Instance fixture data for the RDS fake.
package fixtures

import (
	"sync"
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

	// ProdDbiResourceID is prod-dbi-1's DbiResourceId, which AWS keeps with
	// the instance through a rename and never reissues.
	ProdDbiResourceID = "db-XKZQ4T7NPWYB2MHR6VJD8LSCFA"

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

	// DBINotInBackupPlan is the only demo instance no backup plan selects;
	// every other instance is covered by acme-daily-backup.
	DBINotInBackupPlan    = "sandbox-db-02"
	DBINotInBackupPlanARN = "arn:aws:rds:us-east-1:123456789012:db:sandbox-db-02"

	// DBIDocDBMember is a DocumentDB instance of acme-docdb-prod whose own
	// ARN acme-fleet-wide excludes. AWS Backup protects it through its
	// cluster, which acme-prod-db names, so it is covered.
	DBIDocDBMember    = ProdDbcID + "-01"
	DBIDocDBMemberARN = "arn:aws:rds:us-east-1:123456789012:db:" + DBIDocDBMember

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
	// Parent of the dbi-snap "automated snapshot past retention" fixture.
	ProdDbiRetentionParentID  = "prod-dbi-retention-parent"
	ProdDbiRetentionParentARN = "arn:aws:rds:us-east-1:123456789012:db:prod-dbi-retention-parent"

	// broken-dbi-incompatible-network — Broken (DBInstanceStatus=incompatible-network)
	BrokenDbiIncompatibleNetworkID  = "broken-dbi-incompatible-network"
	BrokenDbiIncompatibleNetworkARN = "arn:aws:rds:us-east-1:123456789012:db:broken-dbi-incompatible-network"

	// broken-dbi-incompatible-option-group — Broken (DBInstanceStatus=incompatible-option-group)
	BrokenDbiIncompatibleOptionGroupID  = "broken-dbi-incompatible-option-group"
	BrokenDbiIncompatibleOptionGroupARN = "arn:aws:rds:us-east-1:123456789012:db:broken-dbi-incompatible-option-group"

	// broken-dbi-incompatible-restore — Broken (DBInstanceStatus=incompatible-restore)
	BrokenDbiIncompatibleRestoreID  = "broken-dbi-incompatible-restore"
	BrokenDbiIncompatibleRestoreARN = "arn:aws:rds:us-east-1:123456789012:db:broken-dbi-incompatible-restore"

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

	dbiMonitoringRoleARN  = "arn:aws:iam::123456789012:role/rds-monitoring-role"
	dbiEnhancedMonitorARN = "arn:aws:iam::123456789012:role/rds-enhanced-monitoring"

	// One carrier per security-posture finding. Every other instance takes
	// the healthy value from dbiBaselineHealthy, so the demo bench shows
	// exactly one row per finding.

	// DBISingleAZ runs in a single Availability Zone.
	DBISingleAZ    = "warn-dbi-single-az"
	dbiSingleAZARN = "arn:aws:rds:us-east-1:123456789012:db:warn-dbi-single-az"
	// DBIMinorUpgradeOff has automatic minor version upgrades disabled.
	DBIMinorUpgradeOff    = "warn-dbi-minor-upgrade-off"
	dbiMinorUpgradeOffARN = "arn:aws:rds:us-east-1:123456789012:db:warn-dbi-minor-upgrade-off"
	// DBIIAMAuthOff has IAM database authentication disabled.
	DBIIAMAuthOff    = "warn-dbi-iam-auth-off"
	dbiIAMAuthOffARN = "arn:aws:rds:us-east-1:123456789012:db:warn-dbi-iam-auth-off"
	// DBIDefaultMasterUser keeps the vendor default administrative username.
	DBIDefaultMasterUser    = "warn-dbi-default-master-user"
	dbiDefaultMasterUserARN = "arn:aws:rds:us-east-1:123456789012:db:warn-dbi-default-master-user"
	// DBICACertExpiring has a server certificate inside the 90-day window.
	DBICACertExpiring    = "warn-dbi-ca-cert-expiring"
	dbiCACertExpiringARN = "arn:aws:rds:us-east-1:123456789012:db:warn-dbi-ca-cert-expiring"
	// DBICACertUrgent has one inside the 30-day window, where the same
	// countdown stops being a task to schedule and becomes an outage on a
	// clock — a separate code and a separate row colour.
	DBICACertUrgent    = "broken-dbi-ca-cert-urgent"
	dbiCACertUrgentARN = "arn:aws:rds:us-east-1:123456789012:db:broken-dbi-ca-cert-urgent"
	// DBIEngineDeprecated runs an engine version AWS no longer supports.
	DBIEngineDeprecated    = "broken-dbi-engine-deprecated"
	dbiEngineDeprecatedARN = "arn:aws:rds:us-east-1:123456789012:db:broken-dbi-engine-deprecated"

	// DBIHealthyEngineVersion is the engine version every healthy instance
	// runs; DBIDeprecatedEngineVersion is the retired one DBIEngineDeprecated runs.
	// The RDS fake answers DescribeDBEngineVersions from these two.
	DBIHealthyEngineVersion    = "16.2"
	DBIDeprecatedEngineVersion = "11.22"
	// DBICurrentCAIdentifier is the CA the healthy instances use.
	DBICurrentCAIdentifier = "rds-ca-rsa2048-g1"
)

// NewDBIFixtures builds and returns a fully-populated DBIFixtures struct.
// PendingMaintenanceActions contains the Wave 2 enrichment data for maint-dbi-scheduled.
var sharedDBIFixtures = sync.OnceValue(func() *DBIFixtures {
	return &DBIFixtures{
		Instances:                 buildDBIInstances(),
		PendingMaintenanceActions: buildDBIPendingMaintenance(),
	}
})

func NewDBIFixtures() *DBIFixtures {
	return sharedDBIFixtures()
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
		// Healthy for every security-posture predicate: only the dedicated
		// posture instances below turn one of these off.
		AutoMinorVersionUpgrade:          aws.Bool(true),
		IAMDatabaseAuthenticationEnabled: aws.Bool(true),
		CertificateDetails: &rdstypes.CertificateDetails{
			CAIdentifier: aws.String(DBICurrentCAIdentifier),
			ValidTill:    aws.Time(time.Now().Add(3 * 365 * 24 * time.Hour)),
		},
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
	prodDbi1 := dbiBaselineHealthy(ProdDbiID, ProdDbiARN)
	prodDbi1.MasterUserSecret = &rdstypes.MasterUserSecret{
		SecretArn: aws.String(ProdDbiMasterSecretARN),
	}
	prodDbi1.AssociatedRoles = []rdstypes.DBInstanceRole{
		{RoleArn: aws.String(dbiMonitoringRoleARN), FeatureName: aws.String("Monitoring")},
	}
	prodDbi1.MonitoringRoleArn = aws.String(dbiEnhancedMonitorARN)
	prodDbi1.EnabledCloudwatchLogsExports = []string{"postgresql", "upgrade"}
	// The identifier that stays with the instance across a rename. A snapshot
	// taken before one carries the name of the day and this id, and only the
	// id still leads back here.
	prodDbi1.DbiResourceId = aws.String(ProdDbiResourceID)

	// prod-dbi-aurora-1 — Aurora cluster member, Healthy, and the graph
	// root for dbi: every related pivot is non-zero. Aurora
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

	modifying := dbiBaselineHealthy(StagingDbiModifyingID, StagingDbiModifyingARN)
	modifying.DBInstanceStatus = aws.String("modifying")
	modifying.PendingModifiedValues = &rdstypes.PendingModifiedValues{
		DBInstanceClass: aws.String("db.r6g.xlarge"),
	}
	modifying.TagList = []rdstypes.Tag{
		{Key: aws.String("Environment"), Value: aws.String("staging")},
	}

	rebooting := dbiBaselineHealthy(StagingDbiRebootingID, StagingDbiRebootingARN)
	rebooting.DBInstanceStatus = aws.String("rebooting")
	rebooting.TagList = []rdstypes.Tag{
		{Key: aws.String("Environment"), Value: aws.String("staging")},
	}

	storageFull := dbiBaselineHealthy(BrokenDbiStorageFullID, BrokenDbiStorageFullARN)
	storageFull.DBInstanceStatus = aws.String("storage-full")

	encLocked := dbiBaselineHealthy(BrokenDbiEncryptionLockedID, BrokenDbiEncryptionLockedARN)
	encLocked.DBInstanceStatus = aws.String("inaccessible-encryption-credentials")
	encLocked.StorageEncrypted = aws.Bool(true)
	encLocked.KmsKeyId = aws.String(dbiDeadbeefKeyARN)

	noBackups := dbiBaselineHealthy(WarnDbiNoBackupsID, WarnDbiNoBackupsARN)
	noBackups.BackupRetentionPeriod = aws.Int32(0)

	public := dbiBaselineHealthy(WarnDbiPublicID, WarnDbiPublicARN)
	public.PubliclyAccessible = aws.Bool(true)

	unencrypted := dbiBaselineHealthy(WarnDbiUnencryptedID, WarnDbiUnencryptedARN)
	unencrypted.StorageEncrypted = aws.Bool(false)
	unencrypted.KmsKeyId = nil

	unprotected := dbiBaselineHealthy(WarnDbiUnprotectedID, WarnDbiUnprotectedARN)
	unprotected.DeletionProtection = aws.Bool(false)

	maintScheduled := dbiBaselineHealthy(MaintDbiScheduledID, MaintDbiScheduledARN)

	// DeletionProtection=true so only 3 of the 4 warnings fire.
	// Expected Wave 1 Status: "no automated backups (+2)"
	warnMulti := dbiBaselineHealthy(WarnDbiMultiID, WarnDbiMultiARN)
	warnMulti.BackupRetentionPeriod = aws.Int32(0)
	warnMulti.PubliclyAccessible = aws.Bool(true)
	warnMulti.StorageEncrypted = aws.Bool(false)
	warnMulti.KmsKeyId = nil
	warnMulti.DeletionProtection = aws.Bool(true)

	// Expected Wave 1 Status: "publicly accessible"
	// Expected Status after Wave 2 enrichment: "publicly accessible (+1)"
	warnPublicMaint := dbiBaselineHealthy(WarnDbiPublicMaintID, WarnDbiPublicMaintARN)
	warnPublicMaint.PubliclyAccessible = aws.Bool(true)

	// Used as the parent of WarnDBISnapPastRetentionID in dbi-snap fixtures to
	// trigger the "automated snapshot past BackupRetentionPeriod" enricher signal.
	retentionParent := dbiBaselineHealthy(ProdDbiRetentionParentID, ProdDbiRetentionParentARN)
	retentionParent.BackupRetentionPeriod = aws.Int32(7)

	incompatNetwork := dbiBaselineHealthy(BrokenDbiIncompatibleNetworkID, BrokenDbiIncompatibleNetworkARN)
	incompatNetwork.DBInstanceStatus = aws.String("incompatible-network")

	incompatOptionGroup := dbiBaselineHealthy(BrokenDbiIncompatibleOptionGroupID, BrokenDbiIncompatibleOptionGroupARN)
	incompatOptionGroup.DBInstanceStatus = aws.String("incompatible-option-group")

	incompatRestore := dbiBaselineHealthy(BrokenDbiIncompatibleRestoreID, BrokenDbiIncompatibleRestoreARN)
	incompatRestore.DBInstanceStatus = aws.String("incompatible-restore")

	singleAZ := dbiBaselineHealthy(DBISingleAZ, dbiSingleAZARN)
	singleAZ.MultiAZ = aws.Bool(false)

	minorUpgradeOff := dbiBaselineHealthy(DBIMinorUpgradeOff, dbiMinorUpgradeOffARN)
	minorUpgradeOff.AutoMinorVersionUpgrade = aws.Bool(false)

	iamAuthOff := dbiBaselineHealthy(DBIIAMAuthOff, dbiIAMAuthOffARN)
	iamAuthOff.IAMDatabaseAuthenticationEnabled = aws.Bool(false)

	defaultMasterUser := dbiBaselineHealthy(DBIDefaultMasterUser, dbiDefaultMasterUserARN)
	defaultMasterUser.MasterUsername = aws.String("postgres")

	// The extra hour keeps the day count from rounding down to 59 as the
	// fixture is built.
	caCertExpiring := dbiBaselineHealthy(DBICACertExpiring, dbiCACertExpiringARN)
	caCertExpiring.CertificateDetails = &rdstypes.CertificateDetails{
		CAIdentifier: aws.String("rds-ca-2019"),
		ValidTill:    aws.Time(time.Now().Add(60*24*time.Hour + time.Hour)),
	}

	caCertUrgent := dbiBaselineHealthy(DBICACertUrgent, dbiCACertUrgentARN)
	caCertUrgent.CertificateDetails = &rdstypes.CertificateDetails{
		CAIdentifier: aws.String("rds-ca-2019"),
		ValidTill:    aws.Time(time.Now().Add(20*24*time.Hour + time.Hour)),
	}

	engineDeprecated := dbiBaselineHealthy(DBIEngineDeprecated, dbiEngineDeprecatedARN)
	engineDeprecated.EngineVersion = aws.String(DBIDeprecatedEngineVersion)

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
		incompatNetwork,
		incompatOptionGroup,
		incompatRestore,
		singleAZ,
		minorUpgradeOff,
		iamAuthOff,
		defaultMasterUser,
		caCertExpiring,
		caCertUrgent,
		engineDeprecated,
	}
}

// One witness per DB instance lifecycle status that has a finding of its
// own. They carry Wave-1 findings only, so they sit after the bulk pool and
// leave the Wave-2 witnesses inside EnrichmentCap.
const (
	DBIEncryptionRecoverable = "broken-dbi-encryption-recoverable"
	DBIIncompatibleCreate    = "broken-dbi-incompatible-create"
	DBIInsufficientCapacity  = "broken-dbi-insufficient-capacity"
	DBIUpgradeFailed         = "broken-dbi-upgrade-failed"
	// DBIUnrecognisedStatus reports a status in neither AWS DB instance
	// status table.
	DBIUnrecognisedStatus = "warn-dbi-unrecognised-status"
)

func dbiLifecycleWitnesses() []rdstypes.DBInstance {
	var out []rdstypes.DBInstance
	for _, w := range []struct{ id, status string }{
		{DBIEncryptionRecoverable, "inaccessible-encryption-credentials-recoverable"},
		{DBIIncompatibleCreate, "incompatible-create"},
		{DBIInsufficientCapacity, "insufficient-capacity"},
		{DBIUpgradeFailed, "upgrade-failed"},
		{DBIUnrecognisedStatus, "storage-rebalancing"},
	} {
		db := dbiBaselineHealthy(w.id, "arn:aws:rds:us-east-1:123456789012:db:"+w.id)
		db.DBInstanceStatus = aws.String(w.status)
		out = append(out, db)
	}
	return out
}

func dbiDocDBClusterMember() rdstypes.DBInstance {
	db := dbiBaselineHealthy(DBIDocDBMember, DBIDocDBMemberARN)
	db.Engine = aws.String("docdb")
	db.EngineVersion = aws.String("5.0.0")
	db.DBClusterIdentifier = aws.String(ProdDbcID)
	db.Endpoint.Port = aws.Int32(27017)
	return db
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
