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

// ListBackupJobs returns all fixture jobs, optionally filtered by ByCreatedAfter.
// The enricher queries with ByCreatedAfter=now-24h to scope to the recent window.
func (f *BackupFake) ListBackupJobs(_ context.Context, input *backup.ListBackupJobsInput, _ ...func(*backup.Options)) (*backup.ListBackupJobsOutput, error) {
	jobs := f.fix.Jobs
	if input != nil && input.ByCreatedAfter != nil {
		cutoff := *input.ByCreatedAfter
		filtered := make([]backuptypes.BackupJob, 0, len(jobs))
		for _, j := range jobs {
			if j.CreationDate != nil && !j.CreationDate.Before(cutoff) {
				filtered = append(filtered, j)
			}
		}
		jobs = filtered
	}
	return &backup.ListBackupJobsOutput{BackupJobs: jobs}, nil
}

// GetBackupPlan returns the plan's rules from the PlanRules fixture map.
// The KMS and SNS related checkers read Rules[].TargetBackupVaultName from the response.
func (f *BackupFake) GetBackupPlan(_ context.Context, input *backup.GetBackupPlanInput, _ ...func(*backup.Options)) (*backup.GetBackupPlanOutput, error) {
	if input == nil || input.BackupPlanId == nil {
		return &backup.GetBackupPlanOutput{}, nil
	}
	rules, ok := f.fix.PlanRules[*input.BackupPlanId]
	if !ok {
		return &backup.GetBackupPlanOutput{BackupPlan: &backuptypes.BackupPlan{}}, nil
	}
	return &backup.GetBackupPlanOutput{
		BackupPlanId: input.BackupPlanId,
		BackupPlan: &backuptypes.BackupPlan{
			BackupPlanName: aws.String("fixture-plan-" + *input.BackupPlanId),
			Rules:          rules,
		},
	}, nil
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
			BackupPlanId:    input.BackupPlanId,
			SelectionId:     input.SelectionId,
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

// DescribeBackupVault returns the vault descriptor from the VaultEncryptionKeys fixture map.
// The KMS related checker reads EncryptionKeyArn from the response.
// Vaults not in the map return an empty response (no customer-managed key).
func (f *BackupFake) DescribeBackupVault(_ context.Context, input *backup.DescribeBackupVaultInput, _ ...func(*backup.Options)) (*backup.DescribeBackupVaultOutput, error) {
	if input == nil || input.BackupVaultName == nil {
		return &backup.DescribeBackupVaultOutput{}, nil
	}
	out := &backup.DescribeBackupVaultOutput{
		BackupVaultName: input.BackupVaultName,
		BackupVaultArn:  aws.String("arn:aws:backup:us-east-1:123456789012:backup-vault:" + *input.BackupVaultName),
	}
	if keyARN, ok := f.fix.VaultEncryptionKeys[*input.BackupVaultName]; ok && keyARN != "" {
		out.EncryptionKeyArn = aws.String(keyARN)
	}
	return out, nil
}

// GetBackupVaultNotifications returns the SNS notification config from the
// VaultSNSTopics fixture map. When a vault is absent from the map, it returns
// a ResourceNotFoundException-style error, matching the real AWS Backup API
// behaviour when no notifications are configured on the vault.
// The SNS related checker (checkBackupSNS) handles this error by skipping the vault.
func (f *BackupFake) GetBackupVaultNotifications(_ context.Context, input *backup.GetBackupVaultNotificationsInput, _ ...func(*backup.Options)) (*backup.GetBackupVaultNotificationsOutput, error) {
	if input == nil || input.BackupVaultName == nil {
		return &backup.GetBackupVaultNotificationsOutput{}, nil
	}
	topicARN, ok := f.fix.VaultSNSTopics[*input.BackupVaultName]
	if !ok {
		// Simulate ResourceNotFoundException — vault has no SNS notifications configured.
		return nil, &backuptypes.ResourceNotFoundException{
			Message: aws.String("Vault " + *input.BackupVaultName + " has no notification configuration"),
		}
	}
	return &backup.GetBackupVaultNotificationsOutput{
		BackupVaultName: input.BackupVaultName,
		SNSTopicArn:     aws.String(topicARN),
	}, nil
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
