package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/backup"
)

// BackupListBackupPlansAPI defines the interface for the Backup ListBackupPlans operation.
type BackupListBackupPlansAPI interface {
	ListBackupPlans(ctx context.Context, params *backup.ListBackupPlansInput, optFns ...func(*backup.Options)) (*backup.ListBackupPlansOutput, error)
}

// BackupListBackupJobsAPI defines the interface for the Backup ListBackupJobs operation.
type BackupListBackupJobsAPI interface {
	ListBackupJobs(ctx context.Context, params *backup.ListBackupJobsInput, optFns ...func(*backup.Options)) (*backup.ListBackupJobsOutput, error)
}

// BackupGetBackupPlanAPI defines the interface for the Backup GetBackupPlan
// operation. Used by backup→role / backup→kms / backup→sns to read the plan's
// rules (target vault names) and associated IAM role/KMS/SNS config.
type BackupGetBackupPlanAPI interface {
	GetBackupPlan(ctx context.Context, params *backup.GetBackupPlanInput, optFns ...func(*backup.Options)) (*backup.GetBackupPlanOutput, error)
}

// BackupListBackupSelectionsAPI defines the interface for the Backup
// ListBackupSelections operation. Used by backup→role to enumerate the
// plan's selections (each carries the IAM role ARN used to perform backups).
type BackupListBackupSelectionsAPI interface {
	ListBackupSelections(ctx context.Context, params *backup.ListBackupSelectionsInput, optFns ...func(*backup.Options)) (*backup.ListBackupSelectionsOutput, error)
}

// BackupDescribeBackupVaultAPI defines the interface for the Backup
// DescribeBackupVault operation. Used by backup→kms to resolve the KMS key
// ARN encrypting the plan's target vault.
type BackupDescribeBackupVaultAPI interface {
	DescribeBackupVault(ctx context.Context, params *backup.DescribeBackupVaultInput, optFns ...func(*backup.Options)) (*backup.DescribeBackupVaultOutput, error)
}

// BackupGetBackupVaultNotificationsAPI defines the interface for the Backup
// GetBackupVaultNotifications operation. Used by backup→sns to resolve the
// SNS topic ARN configured for job-event notifications on the vault.
type BackupGetBackupVaultNotificationsAPI interface {
	GetBackupVaultNotifications(ctx context.Context, params *backup.GetBackupVaultNotificationsInput, optFns ...func(*backup.Options)) (*backup.GetBackupVaultNotificationsOutput, error)
}

// BackupListRecoveryPointsByResourceAPI defines the interface for the Backup
// ListRecoveryPointsByResource operation. Used by {rds,docdb}-snap→backup to
// trace a snapshot back to the backup plan (via RecoveryPoint.CreatedBy.BackupPlanId).
type BackupListRecoveryPointsByResourceAPI interface {
	ListRecoveryPointsByResource(ctx context.Context, params *backup.ListRecoveryPointsByResourceInput, optFns ...func(*backup.Options)) (*backup.ListRecoveryPointsByResourceOutput, error)
}

// BackupAPI is the aggregate interface covering all Backup operations used by a9s fetchers.
// *backup.Client structurally satisfies this interface.
type BackupAPI interface {
	BackupListBackupPlansAPI
	BackupListBackupJobsAPI
	BackupGetBackupPlanAPI
	BackupListBackupSelectionsAPI
	BackupDescribeBackupVaultAPI
	BackupGetBackupVaultNotificationsAPI
	BackupListRecoveryPointsByResourceAPI
}
