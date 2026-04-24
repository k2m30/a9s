package fixtures

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	backuptypes "github.com/aws/aws-sdk-go-v2/service/backup/types"
)

// Exported plan IDs and ARNs — referenced by QA tests and sibling fixtures by symbol.
const (
	// plan-healthy-daily (replaces legacy acme-daily-backup)
	HealthyDailyPlanID  = "11111111-1111-1111-1111-111111111111"
	HealthyDailyPlanARN = "arn:aws:backup:us-east-1:123456789012:backup-plan:11111111-1111-1111-1111-111111111111"

	// plan-never-ran
	NeverRanPlanID  = "22222222-2222-2222-2222-222222222222"
	NeverRanPlanARN = "arn:aws:backup:us-east-1:123456789012:backup-plan:22222222-2222-2222-2222-222222222222"

	// plan-broken-1failed
	ProdCriticalPlanID  = "33333333-3333-3333-3333-333333333333"
	ProdCriticalPlanARN = "arn:aws:backup:us-east-1:123456789012:backup-plan:33333333-3333-3333-3333-333333333333"

	// plan-broken-2failed (graph-root for U9 — every count-shown:yes pivot must resolve ≥1)
	ProdDatabasePlanID  = "44444444-4444-4444-4444-444444444444"
	ProdDatabasePlanARN = "arn:aws:backup:us-east-1:123456789012:backup-plan:44444444-4444-4444-4444-444444444444"

	// plan-broken-aborted
	StagingHourlyPlanID  = "55555555-5555-5555-5555-555555555555"
	StagingHourlyPlanARN = "arn:aws:backup:us-east-1:123456789012:backup-plan:55555555-5555-5555-5555-555555555555"

	// plan-warning-partial
	AppDataPlanID  = "66666666-6666-6666-6666-666666666666"
	AppDataPlanARN = "arn:aws:backup:us-east-1:123456789012:backup-plan:66666666-6666-6666-6666-666666666666"

	// plan-broken-mixed (U7d — ! beats ~)
	ComplianceMixedPlanID  = "77777777-7777-7777-7777-777777777777"
	ComplianceMixedPlanARN = "arn:aws:backup:us-east-1:123456789012:backup-plan:77777777-7777-7777-7777-777777777777"

	// plan-old-failure (window-exclusion test — job is 48h+ old)
	DevSporadicPlanID  = "88888888-8888-8888-8888-888888888888"
	DevSporadicPlanARN = "arn:aws:backup:us-east-1:123456789012:backup-plan:88888888-8888-8888-8888-888888888888"

	// Vault names
	BackupDefaultVaultName = "acme-default-vault"
	BackupProdVaultName    = "acme-prod-vault"

	// KMS key for acme-prod-vault — must exist in kms.go (sibling edit).
	// DescribeBackupVault("acme-prod-vault").EncryptionKeyArn uses this ARN;
	// the KMS checker extracts the key ID (last "/" segment = BackupProdVaultKMSKeyID).
	BackupProdVaultKMSKeyID  = "acme-prod-master-key"
	BackupProdVaultKMSKeyARN = "arn:aws:kms:us-east-1:123456789012:key/acme-prod-master-key"

	// SNS topic for acme-prod-vault notifications — must exist in sns.go (sibling edit).
	// GetBackupVaultNotifications("acme-prod-vault").SNSTopicArn points here.
	BackupAlertsSNSTopicName = "acme-backup-alerts"
	BackupAlertsSNSTopicARN  = "arn:aws:sns:us-east-1:123456789012:acme-backup-alerts"

	// IAM role for backup selections on broken plans — must exist in iam.go (sibling edit).
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
	// reads these to populate Fields["resources"] so sibling pivots (s3,
	// ddb, efs, …) can match via cache scan.
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
		// S3 healthy-bucket recovery point (checkS3Backup pivot).
		// checkS3Backup reads bk.Fields["resource_arn"] which is populated in Phase 7.
		// Adding the recovery point now so the graph is ready when Phase 7 wires the field.
		HealthyBucketARN: {
			{
				RecoveryPointArn: aws.String("arn:aws:backup:us-east-1:123456789012:recovery-point:rp-s3-daily-20260416"),
				BackupVaultName:  aws.String("Default"),
				Status:           backuptypes.RecoveryPointStatusCompleted,
				CreationDate:     aws.Time(mustParseBackupTime("2026-04-16T03:00:00Z")),
			},
		},
		// orders-prod DynamoDB table recovery point (checkDdbBackup pivot).
		// The current checkDdbBackup calls ListRecoveryPointsByResource; after phase-7
		// rewrites it to a cache scan, this entry remains as belt-and-suspenders.
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
		// BackupDefaultVaultName intentionally absent — no customer-managed key.
	}
}

// buildBackupVaultSNSTopics returns vault name → SNSTopicArn.
// The fake uses this to construct GetBackupVaultNotificationsOutput at call time.
// Vaults absent from this map produce ResourceNotFoundException (no SNS configured).
func buildBackupVaultSNSTopics() map[string]string {
	return map[string]string{
		BackupProdVaultName: BackupAlertsSNSTopicARN,
		// BackupDefaultVaultName intentionally absent — no SNS topic configured.
	}
}

// buildBackupJobs returns the account-wide BackupJob list.
// The enricher filters by CreatedBy.BackupPlanId and the 24h cutoff window.
//
// Timestamps are computed relative to time.Now() at fixture-construction time
// so the demo (and the integration scenario harness) always sees jobs inside
// the enricher's rolling 24h window — no matter what date the test runs on.
// The plan-old-failure fixture is intentionally 48h old to pin the
// window-exclusion invariant.
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

		// plan-broken-2failed (ProdDatabasePlanID): two failed jobs in window (graph-root U9).
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
		// The enricher must ignore this job (window-exclusion test).
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
// The fetcher reads Resources to populate Fields["resources"] for sibling pivots (s3, efs).
func buildBackupSelections() map[string][]backuptypes.BackupSelection {
	return map[string][]backuptypes.BackupSelection{
		// plan-healthy-daily: selects healthy S3 bucket, the legacy shared
		// EFS, the graph-root EFS (ProdEFSARN), and the orders-prod DynamoDB
		// table — preserves s3→backup, efs→backup, and ddb→backup pivots via
		// cache scan of Fields["resources"].
		HealthyDailyPlanID: {
			{
				SelectionName: aws.String("acme-daily-multi-selection"),
				IamRoleArn:    aws.String("arn:aws:iam::123456789012:role/service-role/AWSBackupDefaultServiceRole"),
				Resources: []string{
					HealthyBucketARN,
					"arn:aws:elasticfilesystem:us-east-1:123456789012:file-system/fs-0abc111111111111a",
					ProdEFSARN,
					OrdersProdARN,
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

		// plan-broken-2failed (graph-root): uses AcmeBackupRoleProd — role pivot resolves ≥1 on U9.
		ProdDatabasePlanID: {
			{
				SelectionName: aws.String("acme-prod-db-selection"),
				IamRoleArn:    aws.String(AcmeBackupRoleARN),
				Resources:     []string{"arn:aws:rds:us-east-1:123456789012:db:acme-prod-primary"},
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
		// graph-root EFS so efs→backup resolves to ≥2 plans (U9 ≥50%
		// Count>=2 requirement).
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
				Resources:     []string{"arn:aws:s3:::acme-dev-bucket"},
			},
		},
	}
}

// NewBackupFixtures constructs BackupFixtures from the canonical demo data.
// Every plan in the impl-plan §2 is represented here; adversarial fixtures
// (nil CreatedBy, nil CreationDate, API errors) stay inline in QA test files.
func NewBackupFixtures() *BackupFixtures {
	return &BackupFixtures{
		RecoveryPoints:      buildBackupRecoveryPoints(),
		Selections:          buildBackupSelections(),
		Jobs:                buildBackupJobs(),
		PlanRules:           buildBackupPlanRules(),
		VaultEncryptionKeys: buildBackupVaultEncryptionKeys(),
		VaultSNSTopics:      buildBackupVaultSNSTopics(),
		Plans: []backuptypes.BackupPlansListMember{
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

			// plan-never-ran: no jobs ever → Healthy (spec §4 "never run is also Healthy").
			{
				BackupPlanName:   aws.String("acme-newly-created"),
				BackupPlanId:     aws.String(NeverRanPlanID),
				BackupPlanArn:    aws.String(NeverRanPlanARN),
				CreationDate:     aws.Time(mustParseBackupTime("2026-04-22T18:00:00Z")),
				VersionId:        aws.String("v1"),
				CreatorRequestId: aws.String("acme-newly-created-init"),
				// LastExecutionDate intentionally nil — never ran.
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

			// plan-broken-aborted: one ABORTED job → ! (ABORTED maps to "failed" bucket per §3.2).
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

			// plan-broken-mixed (U7d): FAILED + PARTIAL + COMPLETED → ! beats ~.
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
}
