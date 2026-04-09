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
