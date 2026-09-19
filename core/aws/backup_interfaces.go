// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

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

// BackupGetBackupSelectionAPI defines the interface for the Backup
// GetBackupSelection operation. Used by the backup fetcher to read each
// selection whole onto the plan row, so the coverage join and every backup
// pivot can evaluate it from the cache.
type BackupGetBackupSelectionAPI interface {
	GetBackupSelection(ctx context.Context, params *backup.GetBackupSelectionInput, optFns ...func(*backup.Options)) (*backup.GetBackupSelectionOutput, error)
}

// BackupDescribeRegionSettingsAPI defines the interface for the Backup
// DescribeRegionSettings operation: the Region's per-resource-type opt-in,
// which decides whether a selection by "*", by service name or by tags alone
// includes a resource.
type BackupDescribeRegionSettingsAPI interface {
	DescribeRegionSettings(ctx context.Context, params *backup.DescribeRegionSettingsInput, optFns ...func(*backup.Options)) (*backup.DescribeRegionSettingsOutput, error)
}

// BackupAPI is the aggregate interface covering all Backup operations used by
// a9s fetchers. *backup.Client structurally satisfies this interface.
// BackupGetBackupSelectionAPI and BackupDescribeRegionSettingsAPI sit outside
// the aggregate: fetchers that need them type-assert on the concrete client at
// call time, so a fake implementing only the aggregate still satisfies it.
type BackupAPI interface {
	BackupListBackupPlansAPI
	BackupListBackupJobsAPI
	BackupGetBackupPlanAPI
	BackupListBackupSelectionsAPI
	BackupDescribeBackupVaultAPI
	BackupGetBackupVaultNotificationsAPI
	BackupListRecoveryPointsByResourceAPI
}
