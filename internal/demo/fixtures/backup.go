package fixtures

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	backuptypes "github.com/aws/aws-sdk-go-v2/service/backup/types"
)

// BackupFixtures holds typed fixture data for AWS Backup.
type BackupFixtures struct {
	Plans []backuptypes.BackupPlansListMember
}

func mustParseBackupTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

// NewBackupFixtures constructs BackupFixtures from the canonical demo data.
func NewBackupFixtures() *BackupFixtures {
	return &BackupFixtures{
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
