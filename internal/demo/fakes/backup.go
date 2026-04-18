package fakes

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/backup"

	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
)

// BackupFake implements aws.BackupAPI against fixture data loaded at construction time.
type BackupFake struct {
	fix *fixtures.BackupFixtures
}

// NewBackup constructs a BackupFake backed by fixture data from the fixtures package.
func NewBackup() *BackupFake {
	return &BackupFake{fix: fixtures.NewBackupFixtures()}
}

func (f *BackupFake) ListBackupPlans(_ context.Context, _ *backup.ListBackupPlansInput, _ ...func(*backup.Options)) (*backup.ListBackupPlansOutput, error) {
	return &backup.ListBackupPlansOutput{BackupPlansList: f.fix.Plans}, nil
}

func (f *BackupFake) ListBackupJobs(_ context.Context, _ *backup.ListBackupJobsInput, _ ...func(*backup.Options)) (*backup.ListBackupJobsOutput, error) {
	return &backup.ListBackupJobsOutput{}, nil
}

// GetBackupPlan returns an empty plan — demo mode does not model plan rules.
func (f *BackupFake) GetBackupPlan(_ context.Context, _ *backup.GetBackupPlanInput, _ ...func(*backup.Options)) (*backup.GetBackupPlanOutput, error) {
	return &backup.GetBackupPlanOutput{}, nil
}

// ListBackupSelections returns an empty list — demo mode does not model
// backup selections.
func (f *BackupFake) ListBackupSelections(_ context.Context, _ *backup.ListBackupSelectionsInput, _ ...func(*backup.Options)) (*backup.ListBackupSelectionsOutput, error) {
	return &backup.ListBackupSelectionsOutput{}, nil
}

// DescribeBackupVault returns an empty vault description.
func (f *BackupFake) DescribeBackupVault(_ context.Context, _ *backup.DescribeBackupVaultInput, _ ...func(*backup.Options)) (*backup.DescribeBackupVaultOutput, error) {
	return &backup.DescribeBackupVaultOutput{}, nil
}

// GetBackupVaultNotifications returns an empty notification config.
func (f *BackupFake) GetBackupVaultNotifications(_ context.Context, _ *backup.GetBackupVaultNotificationsInput, _ ...func(*backup.Options)) (*backup.GetBackupVaultNotificationsOutput, error) {
	return &backup.GetBackupVaultNotificationsOutput{}, nil
}

// ListRecoveryPointsByResource returns recovery points for the given resource ARN.
func (f *BackupFake) ListRecoveryPointsByResource(_ context.Context, input *backup.ListRecoveryPointsByResourceInput, _ ...func(*backup.Options)) (*backup.ListRecoveryPointsByResourceOutput, error) {
	if input.ResourceArn == nil {
		return &backup.ListRecoveryPointsByResourceOutput{}, nil
	}
	rps, ok := f.fix.RecoveryPoints[*input.ResourceArn]
	if !ok {
		return &backup.ListRecoveryPointsByResourceOutput{}, nil
	}
	return &backup.ListRecoveryPointsByResourceOutput{RecoveryPoints: rps}, nil
}
