// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package fixtures

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	backuptypes "github.com/aws/aws-sdk-go-v2/service/backup/types"
)

// Exported plan IDs and ARNs — referenced by tests and sibling fixtures by symbol.
const (
	// plan-healthy-daily
	HealthyDailyPlanID  = "11111111-1111-1111-1111-111111111111"
	HealthyDailyPlanARN = "arn:aws:backup:us-east-1:123456789012:backup-plan:11111111-1111-1111-1111-111111111111"

	// plan-never-ran
	NeverRanPlanID  = "22222222-2222-2222-2222-222222222222"
	NeverRanPlanARN = "arn:aws:backup:us-east-1:123456789012:backup-plan:22222222-2222-2222-2222-222222222222"

	// plan-broken-1failed
	ProdCriticalPlanID  = "33333333-3333-3333-3333-333333333333"
	ProdCriticalPlanARN = "arn:aws:backup:us-east-1:123456789012:backup-plan:33333333-3333-3333-3333-333333333333"

	// plan-broken-2failed (graph root: every related pivot resolves ≥1)
	ProdDatabasePlanID  = "44444444-4444-4444-4444-444444444444"
	ProdDatabasePlanARN = "arn:aws:backup:us-east-1:123456789012:backup-plan:44444444-4444-4444-4444-444444444444"

	// plan-broken-aborted
	StagingHourlyPlanID  = "55555555-5555-5555-5555-555555555555"
	StagingHourlyPlanARN = "arn:aws:backup:us-east-1:123456789012:backup-plan:55555555-5555-5555-5555-555555555555"

	// plan-warning-partial
	AppDataPlanID  = "66666666-6666-6666-6666-666666666666"
	AppDataPlanARN = "arn:aws:backup:us-east-1:123456789012:backup-plan:66666666-6666-6666-6666-666666666666"

	// plan-broken-mixed (! beats ~)
	ComplianceMixedPlanID  = "77777777-7777-7777-7777-777777777777"
	ComplianceMixedPlanARN = "arn:aws:backup:us-east-1:123456789012:backup-plan:77777777-7777-7777-7777-777777777777"

	// plan-old-failure (its job is 48h+ old, outside the window)
	DevSporadicPlanID  = "88888888-8888-8888-8888-888888888888"
	DevSporadicPlanARN = "arn:aws:backup:us-east-1:123456789012:backup-plan:88888888-8888-8888-8888-888888888888"

	// plan-fleet-wide selects every volume, database, cluster and table by
	// wildcard and excludes the uncovered resources by name, so
	// "not covered by a backup plan" has exactly one carrier per type.
	FleetWidePlanID  = "99999999-9999-9999-9999-999999999999"
	FleetWidePlanARN = "arn:aws:backup:us-east-1:123456789012:backup-plan:99999999-9999-9999-9999-999999999999"

	// FleetWideReselectedTableARN is excluded by the fleet-wide plan's first
	// selection and taken back in by its second, and no other plan names it:
	// an exclusion holds within its own selection only, so the table stays
	// covered.
	FleetWideReselectedTableARN = ddbDeletionProtectionOffARN

	BackupDefaultVaultName = "acme-default-vault"
	BackupProdVaultName    = "acme-prod-vault"

	// KMS key for acme-prod-vault — must exist in kms.go.
	// DescribeBackupVault("acme-prod-vault").EncryptionKeyArn uses this ARN;
	// the KMS checker extracts the key ID (last "/" segment = BackupProdVaultKMSKeyID).
	BackupProdVaultKMSKeyID  = "acme-prod-master-key"
	BackupProdVaultKMSKeyARN = "arn:aws:kms:us-east-1:123456789012:key/acme-prod-master-key"

	// SNS topic for acme-prod-vault notifications — must exist in sns.go.
	// GetBackupVaultNotifications("acme-prod-vault").SNSTopicArn points here.
	BackupAlertsSNSTopicName = "acme-backup-alerts"
	BackupAlertsSNSTopicARN  = "arn:aws:sns:us-east-1:123456789012:acme-backup-alerts"

	// IAM role for backup selections on broken plans — must exist in iam.go.
	// checkBackupRole extracts "AcmeBackupRoleProd" as the last "/" segment.
	AcmeBackupRoleARN = "arn:aws:iam::123456789012:role/AcmeBackupRoleProd"
)

// BackupFixtures holds typed fixture data for AWS Backup.
type BackupFixtures struct {
	Plans []backuptypes.BackupPlansListMember
	// RecoveryPoints maps resource ARN → []RecoveryPointByResource.
	RecoveryPoints map[string][]backuptypes.RecoveryPointByResource
	// Selections maps plan ID → list of full BackupSelection objects (each
	// already carries SelectionId + IamRoleArn + Resources). The fetcher
	// keeps them on the plan row so sibling pivots (s3, ddb, efs, …) can
	// match via cache scan.
	Selections map[string][]backuptypes.BackupSelection
	// Jobs is the account-wide list returned by ListBackupJobs.
	// The enricher filters this by CreatedBy.BackupPlanId and timestamp window.
	Jobs []backuptypes.BackupJob
	// PlanRules maps plan ID → slice of BackupRule (returned by GetBackupPlan).
	// The KMS and SNS related checkers read Rules[].TargetBackupVaultName.
	PlanRules map[string][]backuptypes.BackupRule
	// VaultEncryptionKeys maps vault name → EncryptionKeyArn.
	// Empty string means no customer-managed key (AWS-managed default key).
	// The fake constructs DescribeBackupVaultOutput envelopes at call time.
	VaultEncryptionKeys map[string]string
	// VaultSNSTopics maps vault name → SNSTopicArn.
	// Vaults absent from this map return ResourceNotFoundException on
	// GetBackupVaultNotifications, matching the real AWS Backup API behaviour.
	VaultSNSTopics map[string]string
	// RegionOptIn is DescribeRegionSettings' ResourceTypeOptInPreference.
	RegionOptIn map[string]bool
}

func mustParseBackupTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

// buildBackupRecoveryPoints returns recovery point fixtures keyed by resource ARN.
// The acme-shared-data EFS filesystem has recent daily recovery points demonstrating
// the EFS→Backup related-panel relationship.
func buildBackupRecoveryPoints() map[string][]backuptypes.RecoveryPointByResource {
	efsARN := "arn:aws:elasticfilesystem:us-east-1:123456789012:file-system/fs-0abc111111111111a"
	return map[string][]backuptypes.RecoveryPointByResource{
		HealthyBucketARN: {
			{
				RecoveryPointArn: aws.String("arn:aws:backup:us-east-1:123456789012:recovery-point:rp-s3-daily-20260416"),
				BackupVaultName:  aws.String("Default"),
				Status:           backuptypes.RecoveryPointStatusCompleted,
				CreationDate:     aws.Time(mustParseBackupTime("2026-04-16T03:00:00Z")),
			},
		},
		// orders-prod DynamoDB table recovery point (checkDdbBackup pivot).
		OrdersProdARN: {
			{
				RecoveryPointArn: aws.String("arn:aws:backup:us-east-1:123456789012:recovery-point:rp-ddb-weekly-20260420"),
				BackupVaultName:  aws.String("Default"),
				Status:           backuptypes.RecoveryPointStatusCompleted,
				CreationDate:     aws.Time(mustParseBackupTime("2026-04-20T03:00:00Z")),
			},
		},
		efsARN: {
			{
				RecoveryPointArn: aws.String("arn:aws:backup:us-east-1:123456789012:recovery-point:rp-efs-daily-20260416"),
				BackupVaultName:  aws.String("Default"),
				Status:           backuptypes.RecoveryPointStatusCompleted,
				CreationDate:     aws.Time(mustParseBackupTime("2026-04-16T02:00:00Z")),
			},
			{
				RecoveryPointArn: aws.String("arn:aws:backup:us-east-1:123456789012:recovery-point:rp-efs-daily-20260415"),
				BackupVaultName:  aws.String("Default"),
				Status:           backuptypes.RecoveryPointStatusCompleted,
				CreationDate:     aws.Time(mustParseBackupTime("2026-04-15T02:00:00Z")),
			},
		},
		// EFS prod-app-data recovery points — required for efs→backup related-panel pivot (Count = 2).
		// checkEFSBackup calls ListRecoveryPointsByResource(ResourceArn=ProdEFSARN).
		ProdEFSARN: {
			{
				RecoveryPointArn: aws.String(ProdEFSBackupARecoveryARN),
				BackupVaultName:  aws.String(BackupProdVaultName),
				Status:           backuptypes.RecoveryPointStatusCompleted,
				CreationDate:     aws.Time(mustParseBackupTime("2026-04-16T02:00:00Z")),
			},
			{
				RecoveryPointArn: aws.String(ProdEFSBackupBRecoveryARN),
				BackupVaultName:  aws.String(BackupProdVaultName),
				Status:           backuptypes.RecoveryPointStatusCompleted,
				CreationDate:     aws.Time(mustParseBackupTime("2026-04-15T02:00:00Z")),
			},
		},
		// dbi-snap pivot — required for the dbi-snap→backup related-panel pivot.
		// checkDBISnapBackup calls ListRecoveryPointsByResource(ResourceArn=res.Fields["arn"]).
		// ProdDBISnapARN gets 1 recovery point, BackupCoveredDBISnapARN gets 2.
		// dbi-snap pivots are 1:1 by AWS data model: a snapshot has exactly one
		// source instance and one encryption key, and Aurora cluster snapshots
		// live in dbc-snap.
		ProdDBISnapARN: {
			{
				RecoveryPointArn: aws.String("arn:aws:backup:us-east-1:123456789012:recovery-point:rp-rds1-daily-20260415"),
				BackupVaultName:  aws.String(BackupProdVaultName),
				Status:           backuptypes.RecoveryPointStatusCompleted,
				CreationDate:     aws.Time(mustParseBackupTime("2026-04-15T03:00:00Z")),
			},
		},
		BackupCoveredDBISnapARN: {
			{
				RecoveryPointArn: aws.String("arn:aws:backup:us-east-1:123456789012:recovery-point:rp-bkcov-daily-20260418"),
				BackupVaultName:  aws.String(BackupProdVaultName),
				Status:           backuptypes.RecoveryPointStatusCompleted,
				CreationDate:     aws.Time(mustParseBackupTime("2026-04-18T03:00:00Z")),
			},
			{
				RecoveryPointArn: aws.String("arn:aws:backup:us-east-1:123456789012:recovery-point:rp-bkcov-weekly-20260420"),
				BackupVaultName:  aws.String(BackupProdVaultName),
				Status:           backuptypes.RecoveryPointStatusCompleted,
				CreationDate:     aws.Time(mustParseBackupTime("2026-04-20T03:00:00Z")),
			},
		},
		// dbc-snap → backup pivot: AWS Backup can produce DBClusterSnapshots
		// (DocumentDB and Aurora both). Recovery points keyed on the snapshot
		// ARN drive the dbc-snap→backup pivot to Count ≥ 1 on the graph-roots.
		ProdDBCSnapAuroraARN: {
			{
				RecoveryPointArn: aws.String("arn:aws:backup:us-east-1:123456789012:recovery-point:rp-aurora-cluster-daily-20260415"),
				BackupVaultName:  aws.String(BackupProdVaultName),
				Status:           backuptypes.RecoveryPointStatusCompleted,
				CreationDate:     aws.Time(mustParseBackupTime("2026-04-15T04:00:00Z")),
			},
		},
		ProdDBCSnapDocDBARN: {
			{
				RecoveryPointArn: aws.String("arn:aws:backup:us-east-1:123456789012:recovery-point:rp-docdb-cluster-daily-20260320"),
				BackupVaultName:  aws.String(BackupProdVaultName),
				Status:           backuptypes.RecoveryPointStatusCompleted,
				CreationDate:     aws.Time(mustParseBackupTime("2026-03-20T04:00:00Z")),
			},
		},
	}
}

// buildBackupPlanRules returns the plan rules map used by GetBackupPlan.
// Keyed by plan ID. The KMS and SNS related checkers traverse these to find vault names.
func buildBackupPlanRules() map[string][]backuptypes.BackupRule {
	daily := backuptypes.BackupRule{
		RuleName:              aws.String("Daily"),
		TargetBackupVaultName: aws.String(BackupDefaultVaultName),
		ScheduleExpression:    aws.String("cron(0 5 ? * * *)"),
	}
	prod := backuptypes.BackupRule{
		RuleName:              aws.String("ProdDaily"),
		TargetBackupVaultName: aws.String(BackupProdVaultName),
		ScheduleExpression:    aws.String("cron(0 3 ? * * *)"),
	}
	return map[string][]backuptypes.BackupRule{
		HealthyDailyPlanID:    {daily},
		NeverRanPlanID:        {daily},
		ProdCriticalPlanID:    {prod},
		ProdDatabasePlanID:    {prod},
		StagingHourlyPlanID:   {daily},
		AppDataPlanID:         {daily},
		ComplianceMixedPlanID: {prod},
		DevSporadicPlanID:     {daily},
	}
}

// buildBackupVaultEncryptionKeys returns vault name → EncryptionKeyArn.
// The fake uses this to construct DescribeBackupVaultOutput at call time.
// acme-prod-vault uses the customer-managed key that also exists in kms.go.
// acme-default-vault has no entry → EncryptionKeyArn will be omitted (AWS-managed key).
func buildBackupVaultEncryptionKeys() map[string]string {
	return map[string]string{
		BackupProdVaultName: BackupProdVaultKMSKeyARN,
	}
}

// buildBackupVaultSNSTopics returns vault name → SNSTopicArn.
// The fake uses this to construct GetBackupVaultNotificationsOutput at call time.
// Vaults absent from this map produce ResourceNotFoundException (no SNS configured).
func buildBackupVaultSNSTopics() map[string]string {
	return map[string]string{
		BackupProdVaultName: BackupAlertsSNSTopicARN,
	}
}

// buildBackupJobs returns the account-wide BackupJob list.
// The enricher filters by CreatedBy.BackupPlanId and the 24h cutoff window.
//
// Timestamps are computed relative to time.Now() at fixture-construction time
// so the demo (and the integration scenario harness) always sees jobs inside
// the enricher's rolling 24h window — no matter what date the test runs on.
// The plan-old-failure job is 48h old, outside that window.
func buildBackupJobs() []backuptypes.BackupJob {
	now := time.Now()
	inWindow := func(hoursAgo int) *time.Time {
		t := now.Add(-time.Duration(hoursAgo) * time.Hour)
		return &t
	}
	outOfWindow := func(hoursAgo int) *time.Time { return inWindow(hoursAgo) }
	return []backuptypes.BackupJob{
		// plan-broken-1failed (ProdCriticalPlanID): one FAILED job ~10h ago.
		{
			BackupJobId:   aws.String("job-33-a"),
			State:         backuptypes.BackupJobStateFailed,
			CreationDate:  inWindow(10),
			StatusMessage: aws.String("Backup vault access denied — check KMS key policy"),
			IamRoleArn:    aws.String(AcmeBackupRoleARN),
			CreatedBy: &backuptypes.RecoveryPointCreator{
				BackupPlanId: aws.String(ProdCriticalPlanID),
			},
		},

		// plan-broken-2failed (ProdDatabasePlanID): two failed jobs in window.
		{
			BackupJobId:   aws.String("job-44-a"),
			State:         backuptypes.BackupJobStateFailed,
			CreationDate:  inWindow(10),
			StatusMessage: aws.String("KMSKeyNotAccessibleException: CMK access denied for vault encryption"),
			IamRoleArn:    aws.String(AcmeBackupRoleARN),
			CreatedBy: &backuptypes.RecoveryPointCreator{
				BackupPlanId: aws.String(ProdDatabasePlanID),
			},
		},
		{
			BackupJobId:   aws.String("job-44-b"),
			State:         backuptypes.BackupJobStateExpired,
			CreationDate:  inWindow(4),
			StatusMessage: aws.String("backup job expired past completion window"),
			IamRoleArn:    aws.String(AcmeBackupRoleARN),
			CreatedBy: &backuptypes.RecoveryPointCreator{
				BackupPlanId: aws.String(ProdDatabasePlanID),
			},
		},

		// plan-broken-aborted (StagingHourlyPlanID): one ABORTED job ~2h ago.
		{
			BackupJobId:   aws.String("job-55-a"),
			State:         backuptypes.BackupJobStateAborted,
			CreationDate:  inWindow(2),
			StatusMessage: aws.String("Backup job aborted by user"),
			IamRoleArn:    aws.String("arn:aws:iam::123456789012:role/service-role/AWSBackupDefaultServiceRole"),
			CreatedBy: &backuptypes.RecoveryPointCreator{
				BackupPlanId: aws.String(StagingHourlyPlanID),
			},
		},

		// plan-warning-partial (AppDataPlanID): 2 COMPLETED + 1 PARTIAL in window.
		{
			BackupJobId:  aws.String("job-66-a"),
			State:        backuptypes.BackupJobStateCompleted,
			CreationDate: inWindow(12),
			IamRoleArn:   aws.String("arn:aws:iam::123456789012:role/service-role/AWSBackupDefaultServiceRole"),
			CreatedBy: &backuptypes.RecoveryPointCreator{
				BackupPlanId: aws.String(AppDataPlanID),
			},
		},
		{
			BackupJobId:  aws.String("job-66-b"),
			State:        backuptypes.BackupJobStateCompleted,
			CreationDate: inWindow(11),
			IamRoleArn:   aws.String("arn:aws:iam::123456789012:role/service-role/AWSBackupDefaultServiceRole"),
			CreatedBy: &backuptypes.RecoveryPointCreator{
				BackupPlanId: aws.String(AppDataPlanID),
			},
		},
		{
			BackupJobId:   aws.String("job-66-c"),
			State:         backuptypes.BackupJobStatePartial,
			CreationDate:  inWindow(10),
			StatusMessage: aws.String("1 of 3 resources was not backed up"),
			IamRoleArn:    aws.String("arn:aws:iam::123456789012:role/service-role/AWSBackupDefaultServiceRole"),
			CreatedBy: &backuptypes.RecoveryPointCreator{
				BackupPlanId: aws.String(AppDataPlanID),
			},
		},

		// plan-broken-mixed (ComplianceMixedPlanID): FAILED + PARTIAL + COMPLETED — ! beats ~.
		{
			BackupJobId:   aws.String("job-77-a"),
			State:         backuptypes.BackupJobStateFailed,
			CreationDate:  inWindow(18),
			StatusMessage: aws.String("Resource not accessible"),
			IamRoleArn:    aws.String(AcmeBackupRoleARN),
			CreatedBy: &backuptypes.RecoveryPointCreator{
				BackupPlanId: aws.String(ComplianceMixedPlanID),
			},
		},
		{
			BackupJobId:   aws.String("job-77-b"),
			State:         backuptypes.BackupJobStatePartial,
			CreationDate:  inWindow(15),
			StatusMessage: aws.String("2 of 5 resources were not backed up"),
			IamRoleArn:    aws.String(AcmeBackupRoleARN),
			CreatedBy: &backuptypes.RecoveryPointCreator{
				BackupPlanId: aws.String(ComplianceMixedPlanID),
			},
		},
		{
			BackupJobId:  aws.String("job-77-c"),
			State:        backuptypes.BackupJobStateCompleted,
			CreationDate: inWindow(12),
			IamRoleArn:   aws.String(AcmeBackupRoleARN),
			CreatedBy: &backuptypes.RecoveryPointCreator{
				BackupPlanId: aws.String(ComplianceMixedPlanID),
			},
		},

		// plan-old-failure (DevSporadicPlanID): FAILED job 48h+ ago — outside the 24h window.
		// The enricher ignores this job.
		{
			BackupJobId:   aws.String("job-88-a"),
			State:         backuptypes.BackupJobStateFailed,
			CreationDate:  outOfWindow(48),
			StatusMessage: aws.String("Network connectivity failure"),
			IamRoleArn:    aws.String("arn:aws:iam::123456789012:role/service-role/AWSBackupDefaultServiceRole"),
			CreatedBy: &backuptypes.RecoveryPointCreator{
				BackupPlanId: aws.String(DevSporadicPlanID),
			},
		},
	}
}

// buildBackupSelections returns the selections map used by ListBackupSelections / GetBackupSelection.
// Keyed by plan ID. The role related-checker reads IamRoleArn from these.
// The fetcher keeps each selection on the plan row for sibling pivots (s3, efs).
func buildBackupSelections() map[string][]backuptypes.BackupSelection {
	return map[string][]backuptypes.BackupSelection{
		// plan-healthy-daily: selects healthy S3 bucket, the shared
		// EFS, the graph-root EFS (ProdEFSARN), and the orders-prod DynamoDB
		// table — backs the s3→backup, efs→backup, and ddb→backup pivots via
		// cache scan of the plan's selections. Also includes the Aurora parent DB
		// (ProdDbiAuroraARN) so the dbi-snap→backup pivot resolves Count ≥ 2
		// for snapshots whose parent is that DB (this plan + ProdDatabasePlanID
		// below).  AWS Backup selects parent DB instances, not individual
		// snapshots — checkDBISnapBackup walks the snapshot's parent DB ARN.
		HealthyDailyPlanID: {
			{
				SelectionName: aws.String("acme-daily-multi-selection"),
				IamRoleArn:    aws.String("arn:aws:iam::123456789012:role/service-role/AWSBackupDefaultServiceRole"),
				Resources: []string{
					HealthyBucketARN,
					"arn:aws:elasticfilesystem:us-east-1:123456789012:file-system/fs-0abc111111111111a",
					ProdEFSARN,
					OrdersProdARN,
					ProdDbiAuroraARN,
					// vol-0a1b2c3d4e5f60001 is the EBS volume whose
					// AWS-Backup-created snapshot backs the ebs-snap:backup
					// related-panel pivot (ec2.go snapshot fixture).
					"arn:aws:ec2:us-east-1:123456789012:volume/vol-0a1b2c3d4e5f60001",
				},
				// ListOfTags — tag-based selection required for the ec2:backup
				// and ebs:backup related-panel pivots. Matches the
				// backup=daily tag on i-0a1b2c3d4e5f60001 (ec2.go) and its
				// root volume vol-0a1b2c3d4e5f60001 (ec2.go).
				ListOfTags: []backuptypes.Condition{
					{
						ConditionKey:   aws.String("aws:ResourceTag/backup"),
						ConditionValue: aws.String("daily"),
						ConditionType:  backuptypes.ConditionTypeStringequals,
					},
				},
			},
		},

		// plan-broken-1failed: uses AcmeBackupRoleProd so role pivot resolves.
		ProdCriticalPlanID: {
			{
				SelectionName: aws.String("acme-prod-critical-selection"),
				IamRoleArn:    aws.String(AcmeBackupRoleARN),
				Resources:     []string{"arn:aws:rds:us-east-1:123456789012:db:acme-prod-secondary"},
			},
		},

		// plan-broken-2failed (graph-root for backup; also covers dbi-snap and
		// dbc-snap pivots): uses AcmeBackupRoleProd, so the role pivot resolves ≥1.
		// Resources covers the dbi-snap parent DBs (ProdDbiID + ProdDbiAuroraID)
		// AND the dbc-snap parent clusters (ProdDbcARN + Aurora cluster ARN), so
		// every snapshot whose parent is one of those resources gets the backup
		// pivot Count ≥ 1. AWS Backup selects parent DBs/clusters, not snapshots.
		ProdDatabasePlanID: {
			{
				SelectionName: aws.String("acme-prod-db-selection"),
				IamRoleArn:    aws.String(AcmeBackupRoleARN),
				Resources: []string{
					"arn:aws:rds:us-east-1:123456789012:db:acme-prod-primary",
					ProdDbiARN,
					ProdDbiAuroraARN,
					ProdDbcARN,
					"arn:aws:rds:us-east-1:123456789012:cluster:prod-aurora-cluster",
				},
			},
		},

		// plan-fleet-wide: one wildcard selection per service the coverage
		// join reads, minus the four uncovered resources, so every other demo
		// resource is covered by a plan. The table a second selection takes
		// back in and a DocumentDB instance, which AWS Backup protects through
		// its cluster, are excluded as well.
		FleetWidePlanID: {
			{
				SelectionName: aws.String("acme-fleet-wide-selection"),
				IamRoleArn:    aws.String("arn:aws:iam::123456789012:role/service-role/AWSBackupDefaultServiceRole"),
				Resources: []string{
					"arn:aws:ec2:*:*:volume/*",
					"arn:aws:rds:*:*:db:*",
					"arn:aws:rds:*:*:cluster:*",
					"arn:aws:dynamodb:*:*:table/*",
				},
				NotResources: []string{
					EBSNotInBackupPlanARN,
					DBINotInBackupPlanARN,
					DBCNotInBackupPlanARN,
					DDBNotInBackupPlanARN,
					FleetWideReselectedTableARN,
					DBIDocDBMemberARN,
				},
			},
			{
				SelectionName: aws.String("acme-fleet-wide-reselect"),
				IamRoleArn:    aws.String("arn:aws:iam::123456789012:role/service-role/AWSBackupDefaultServiceRole"),
				Resources:     []string{FleetWideReselectedTableARN},
			},
		},

		// plan-broken-aborted: uses default service role.
		StagingHourlyPlanID: {
			{
				SelectionName: aws.String("acme-staging-hourly-selection"),
				IamRoleArn:    aws.String("arn:aws:iam::123456789012:role/service-role/AWSBackupDefaultServiceRole"),
				Resources:     []string{"arn:aws:ec2:us-east-1:123456789012:instance/i-staging001"},
			},
		},

		// plan-warning-partial: uses default service role.
		AppDataPlanID: {
			{
				SelectionName: aws.String("acme-app-data-selection"),
				IamRoleArn:    aws.String("arn:aws:iam::123456789012:role/service-role/AWSBackupDefaultServiceRole"),
				Resources:     []string{"arn:aws:dynamodb:us-east-1:123456789012:table/acme-app-sessions"},
			},
		},

		// plan-broken-mixed: uses AcmeBackupRoleProd. Also selects the
		// graph-root EFS so efs→backup resolves to ≥2 plans.
		ComplianceMixedPlanID: {
			{
				SelectionName: aws.String("acme-compliance-mixed-selection"),
				IamRoleArn:    aws.String(AcmeBackupRoleARN),
				Resources: []string{
					"arn:aws:ec2:us-east-1:123456789012:instance/i-compliance001",
					ProdEFSARN,
				},
			},
		},

		// plan-old-failure: uses default service role.
		DevSporadicPlanID: {
			{
				SelectionName: aws.String("acme-dev-sporadic-selection"),
				IamRoleArn:    aws.String("arn:aws:iam::123456789012:role/service-role/AWSBackupDefaultServiceRole"),
				Resources:     []string{S3OtherRegionBucketARN},
			},
		},
	}
}

// NewBackupFixtures constructs BackupFixtures from the canonical demo data.
var sharedBackupFixtures = shared(func() *BackupFixtures {
	return &BackupFixtures{
		RecoveryPoints:      buildBackupRecoveryPoints(),
		Selections:          buildBackupSelections(),
		Jobs:                buildBackupJobs(),
		PlanRules:           buildBackupPlanRules(),
		VaultEncryptionKeys: buildBackupVaultEncryptionKeys(),
		VaultSNSTopics:      buildBackupVaultSNSTopics(),
		// EFS is opted out: a file system only an exact ARN names is still
		// backed up, and one selected by tag alone (EFSNoBackupPolicy) is not.
		RegionOptIn: map[string]bool{
			"Aurora": true, "CloudFormation": true, "DocumentDB": true, "DynamoDB": true, "EBS": true,
			"EC2": true, "EFS": false, "FSx": true, "Neptune": true, "RDS": true, "Redshift": true,
			"S3": true, "SAP HANA on Amazon EC2": true, "Storage Gateway": true, "Timestream": true,
			"VirtualMachine": true,
		},
		Plans: []backuptypes.BackupPlansListMember{
			// plan-fleet-wide: the blanket selection behind the coverage join.
			{
				BackupPlanName:   aws.String("acme-fleet-wide"),
				BackupPlanId:     aws.String(FleetWidePlanID),
				BackupPlanArn:    aws.String(FleetWidePlanARN),
				CreationDate:     aws.Time(mustParseBackupTime("2025-01-15T09:00:00Z")),
				VersionId:        aws.String("v1"),
				CreatorRequestId: aws.String("acme-fleet-wide-init"),
			},
			// plan-healthy-daily: no jobs in 24h window → Healthy silence.
			{
				BackupPlanName:    aws.String("acme-daily-backup"),
				BackupPlanId:      aws.String(HealthyDailyPlanID),
				BackupPlanArn:     aws.String(HealthyDailyPlanARN),
				CreationDate:      aws.Time(mustParseBackupTime("2025-01-15T09:00:00Z")),
				LastExecutionDate: aws.Time(mustParseBackupTime("2026-04-22T02:00:00Z")),
				VersionId:         aws.String("v1"),
				CreatorRequestId:  aws.String("acme-daily-backup-init"),
				AdvancedBackupSettings: []backuptypes.AdvancedBackupSetting{
					{
						ResourceType:  aws.String("EC2"),
						BackupOptions: map[string]string{"WindowsVSS": "enabled"},
					},
				},
			},

			// plan-never-ran: no jobs ever → Healthy.
			{
				BackupPlanName:   aws.String("acme-newly-created"),
				BackupPlanId:     aws.String(NeverRanPlanID),
				BackupPlanArn:    aws.String(NeverRanPlanARN),
				CreationDate:     aws.Time(mustParseBackupTime("2026-04-22T18:00:00Z")),
				VersionId:        aws.String("v1"),
				CreatorRequestId: aws.String("acme-newly-created-init"),
			},

			// plan-broken-1failed: one FAILED job in window → !.
			{
				BackupPlanName:    aws.String("acme-prod-critical"),
				BackupPlanId:      aws.String(ProdCriticalPlanID),
				BackupPlanArn:     aws.String(ProdCriticalPlanARN),
				CreationDate:      aws.Time(mustParseBackupTime("2025-06-01T10:00:00Z")),
				LastExecutionDate: aws.Time(mustParseBackupTime("2026-04-22T07:12:00Z")),
				VersionId:         aws.String("v1"),
				CreatorRequestId:  aws.String("acme-prod-critical-init"),
			},

			// plan-broken-2failed (graph-root): two failed jobs (FAILED + EXPIRED) → !.
			// Related pivots: kms≥1 (via acme-prod-vault→acme-prod-master-key),
			//                 role≥1 (via AcmeBackupRoleProd),
			//                 sns≥1  (via acme-prod-vault→acme-backup-alerts).
			{
				BackupPlanName:    aws.String("acme-prod-database"),
				BackupPlanId:      aws.String(ProdDatabasePlanID),
				BackupPlanArn:     aws.String(ProdDatabasePlanARN),
				CreationDate:      aws.Time(mustParseBackupTime("2025-04-10T09:00:00Z")),
				LastExecutionDate: aws.Time(mustParseBackupTime("2026-04-22T12:30:00Z")),
				VersionId:         aws.String("v1"),
				CreatorRequestId:  aws.String("acme-prod-database-init"),
			},

			// plan-broken-aborted: one ABORTED job → ! (ABORTED counts as failed).
			{
				BackupPlanName:    aws.String("acme-staging-hourly"),
				BackupPlanId:      aws.String(StagingHourlyPlanID),
				BackupPlanArn:     aws.String(StagingHourlyPlanARN),
				CreationDate:      aws.Time(mustParseBackupTime("2025-09-01T08:00:00Z")),
				LastExecutionDate: aws.Time(mustParseBackupTime("2026-04-22T15:00:00Z")),
				VersionId:         aws.String("v1"),
				CreatorRequestId:  aws.String("acme-staging-hourly-init"),
			},

			// plan-warning-partial: 2 COMPLETED + 1 PARTIAL → ~.
			{
				BackupPlanName:    aws.String("acme-app-data"),
				BackupPlanId:      aws.String(AppDataPlanID),
				BackupPlanArn:     aws.String(AppDataPlanARN),
				CreationDate:      aws.Time(mustParseBackupTime("2025-03-15T11:00:00Z")),
				LastExecutionDate: aws.Time(mustParseBackupTime("2026-04-22T06:02:00Z")),
				VersionId:         aws.String("v1"),
				CreatorRequestId:  aws.String("acme-app-data-init"),
			},

			// plan-broken-mixed: FAILED + PARTIAL + COMPLETED → ! beats ~.
			{
				BackupPlanName:    aws.String("acme-compliance-mixed"),
				BackupPlanId:      aws.String(ComplianceMixedPlanID),
				BackupPlanArn:     aws.String(ComplianceMixedPlanARN),
				CreationDate:      aws.Time(mustParseBackupTime("2025-07-20T09:00:00Z")),
				LastExecutionDate: aws.Time(mustParseBackupTime("2026-04-22T12:00:00Z")),
				VersionId:         aws.String("v1"),
				CreatorRequestId:  aws.String("acme-compliance-mixed-init"),
			},

			// plan-old-failure: job is 48h+ old → Healthy (window exclusion).
			{
				BackupPlanName:    aws.String("acme-dev-sporadic"),
				BackupPlanId:      aws.String(DevSporadicPlanID),
				BackupPlanArn:     aws.String(DevSporadicPlanARN),
				CreationDate:      aws.Time(mustParseBackupTime("2024-11-10T14:00:00Z")),
				LastExecutionDate: aws.Time(mustParseBackupTime("2026-04-20T15:00:00Z")),
				VersionId:         aws.String("v1"),
				CreatorRequestId:  aws.String("acme-dev-sporadic-init"),
			},
		},
	}
})

func NewBackupFixtures() *BackupFixtures {
	return sharedBackupFixtures()
}

func init() {
	Register(Pin{ShortName: "backup", Rows: 9, Issues: 0, CoverageGaps: []string{"dim"}})
}
