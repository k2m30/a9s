package fixtures

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	backuptypes "github.com/aws/aws-sdk-go-v2/service/backup/types"
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
				CreationDate:     aws.Time(mustParseBackupTime("2026-04-16T03:00:00+00:00")),
			},
		},
		efsARN: {
			{
				RecoveryPointArn: aws.String("arn:aws:backup:us-east-1:123456789012:recovery-point:rp-efs-daily-20260416"),
				BackupVaultName:  aws.String("Default"),
				Status:           backuptypes.RecoveryPointStatusCompleted,
				CreationDate:     aws.Time(mustParseBackupTime("2026-04-16T02:00:00+00:00")),
			},
			{
				RecoveryPointArn: aws.String("arn:aws:backup:us-east-1:123456789012:recovery-point:rp-efs-daily-20260415"),
				BackupVaultName:  aws.String("Default"),
				Status:           backuptypes.RecoveryPointStatusCompleted,
				CreationDate:     aws.Time(mustParseBackupTime("2026-04-15T02:00:00+00:00")),
			},
		},
	}
}

// NewBackupFixtures constructs BackupFixtures from the canonical demo data.
func NewBackupFixtures() *BackupFixtures {
	dailyPlanID := "1a2b3c4d-5e6f-7890-abcd-111111111111"
	return &BackupFixtures{
		RecoveryPoints: buildBackupRecoveryPoints(),
		Selections: map[string][]backuptypes.BackupSelection{
			// The daily plan selects the healthy bucket (and the acme-shared
			// EFS filesystem) so the s3→backup and efs→backup pivots resolve
			// via cache scan of Fields["resources"].
			dailyPlanID: {
				{
					SelectionName: aws.String("acme-daily-s3-efs-selection"),
					IamRoleArn:    aws.String("arn:aws:iam::123456789012:role/service-role/AWSBackupDefaultServiceRole"),
					Resources: []string{
						HealthyBucketARN,
						"arn:aws:elasticfilesystem:us-east-1:123456789012:file-system/fs-0abc111111111111a",
					},
				},
			},
		},
		Plans: []backuptypes.BackupPlansListMember{
			{
				BackupPlanName:    aws.String("acme-daily-backup"),
				BackupPlanId:      aws.String("1a2b3c4d-5e6f-7890-abcd-111111111111"),
				BackupPlanArn:     aws.String("arn:aws:backup:us-east-1:123456789012:backup-plan:1a2b3c4d-5e6f-7890-abcd-111111111111"),
				CreationDate:      aws.Time(mustParseBackupTime("2025-01-15T09:00:00+00:00")),
				LastExecutionDate: aws.Time(mustParseBackupTime("2026-03-21T02:00:00+00:00")),
				DeletionDate:      aws.Time(mustParseBackupTime("2026-12-31T23:59:59+00:00")),
				VersionId:         aws.String("v1"),
				CreatorRequestId:  aws.String("acme-daily-backup-init"),
				AdvancedBackupSettings: []backuptypes.AdvancedBackupSetting{
					{
						ResourceType:  aws.String("EC2"),
						BackupOptions: map[string]string{"WindowsVSS": "enabled"},
					},
				},
			},
			{
				BackupPlanName:    aws.String("acme-weekly-full-backup"),
				BackupPlanId:      aws.String("1a2b3c4d-5e6f-7890-abcd-222222222222"),
				BackupPlanArn:     aws.String("arn:aws:backup:us-east-1:123456789012:backup-plan:1a2b3c4d-5e6f-7890-abcd-222222222222"),
				CreationDate:      aws.Time(mustParseBackupTime("2025-01-15T09:15:00+00:00")),
				LastExecutionDate: aws.Time(mustParseBackupTime("2026-03-16T03:00:00+00:00")),
				VersionId:         aws.String("v1"),
				CreatorRequestId:  aws.String("acme-weekly-full-backup-init"),
			},
			{
				BackupPlanName:    aws.String("acme-compliance-30day"),
				BackupPlanId:      aws.String("1a2b3c4d-5e6f-7890-abcd-333333333333"),
				BackupPlanArn:     aws.String("arn:aws:backup:us-east-1:123456789012:backup-plan:1a2b3c4d-5e6f-7890-abcd-333333333333"),
				CreationDate:      aws.Time(mustParseBackupTime("2025-06-01T10:00:00+00:00")),
				LastExecutionDate: aws.Time(mustParseBackupTime("2026-03-20T04:30:00+00:00")),
				VersionId:         aws.String("v1"),
				CreatorRequestId:  aws.String("acme-compliance-30day-init"),
			},
		},
	}
}
