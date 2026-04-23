package fakes

import (
	"context"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/backup"
	backuptypes "github.com/aws/aws-sdk-go-v2/service/backup/types"

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

// ListBackupSelections returns the list of selection summaries for the plan.
// Each entry is derived from the fixture's BackupSelection objects so
// GetBackupSelection can look up the full record by (planID, selectionID).
func (f *BackupFake) ListBackupSelections(_ context.Context, input *backup.ListBackupSelectionsInput, _ ...func(*backup.Options)) (*backup.ListBackupSelectionsOutput, error) {
	if input == nil || input.BackupPlanId == nil {
		return &backup.ListBackupSelectionsOutput{}, nil
	}
	sels, ok := f.fix.Selections[*input.BackupPlanId]
	if !ok {
		return &backup.ListBackupSelectionsOutput{}, nil
	}
	summaries := make([]backuptypes.BackupSelectionsListMember, 0, len(sels))
	for i, sel := range sels {
		summaries = append(summaries, backuptypes.BackupSelectionsListMember{
			BackupPlanId:  input.BackupPlanId,
			SelectionId:   aws.String(selectionIDFor(*input.BackupPlanId, i)),
			SelectionName: sel.SelectionName,
			IamRoleArn:    sel.IamRoleArn,
		})
	}
	return &backup.ListBackupSelectionsOutput{BackupSelectionsList: summaries}, nil
}

// GetBackupSelection returns the full BackupSelection (including the
// Resources list) for the (planID, selectionID) pair produced by
// ListBackupSelections.
func (f *BackupFake) GetBackupSelection(_ context.Context, input *backup.GetBackupSelectionInput, _ ...func(*backup.Options)) (*backup.GetBackupSelectionOutput, error) {
	if input == nil || input.BackupPlanId == nil || input.SelectionId == nil {
		return &backup.GetBackupSelectionOutput{}, nil
	}
	sels, ok := f.fix.Selections[*input.BackupPlanId]
	if !ok {
		return &backup.GetBackupSelectionOutput{}, nil
	}
	for i, sel := range sels {
		if selectionIDFor(*input.BackupPlanId, i) != *input.SelectionId {
			continue
		}
		selCopy := sel
		return &backup.GetBackupSelectionOutput{
			BackupPlanId: input.BackupPlanId,
			SelectionId:  input.SelectionId,
			BackupSelection: &selCopy,
		}, nil
	}
	return &backup.GetBackupSelectionOutput{}, nil
}

func selectionIDFor(planID string, idx int) string {
	// Deterministic synthetic selection ID — stable across calls so
	// ListBackupSelections summaries round-trip to GetBackupSelection.
	return planID + "-sel-" + strconv.Itoa(idx)
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
