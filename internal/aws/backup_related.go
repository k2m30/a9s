// backup_related.go contains AWS Backup related-resource checker functions.
package aws

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/backup"
	backuptypes "github.com/aws/aws-sdk-go-v2/service/backup/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkBackupRole resolves the IAM roles used by this plan's selections via
// a single backup:ListBackupSelections call (Pattern C). Each
// BackupSelectionsListMember exposes IamRoleArn directly — the role the
// Backup service assumes to protect the selection's resources.
func checkBackupRole(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	planID := res.ID
	if planID == "" {
		return resource.RelatedCheckResult{TargetType: "role", Count: -1}
	}
	c, cok := clients.(*ServiceClients)
	if !cok || c == nil || c.Backup == nil {
		return resource.RelatedCheckResult{TargetType: "role", Count: -1}
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*backup.ListBackupSelectionsOutput, error) {
		return c.Backup.ListBackupSelections(ctx, &backup.ListBackupSelectionsInput{
			BackupPlanId: &planID,
		})
	})
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "role", Count: -1, Err: err}
	}
	seen := make(map[string]struct{})
	var ids []string
	for _, sel := range out.BackupSelectionsList {
		if sel.IamRoleArn == nil || *sel.IamRoleArn == "" {
			continue
		}
		arn := *sel.IamRoleArn
		name := arn
		if idx := strings.LastIndex(arn, "/"); idx >= 0 && idx < len(arn)-1 {
			name = arn[idx+1:]
		}
		if _, dup := seen[name]; dup {
			continue
		}
		seen[name] = struct{}{}
		ids = append(ids, name)
	}
	return relatedResult("role", ids)
}

// checkBackupKMS resolves the KMS key(s) encrypting this plan's target
// vaults via backup:GetBackupPlan → backup:DescribeBackupVault (Pattern C,
// bounded N+1 where N = unique vaults referenced by plan rules, typically 1).
func checkBackupKMS(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	vaults := backupPlanVaults(ctx, clients, res)
	if vaults == nil {
		return resource.RelatedCheckResult{TargetType: "kms", Count: -1}
	}
	if len(vaults) == 0 {
		return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
	}
	c, cok := clients.(*ServiceClients)
	if !cok || c == nil || c.Backup == nil {
		return resource.RelatedCheckResult{TargetType: "kms", Count: -1}
	}
	seen := make(map[string]struct{})
	var ids []string
	for _, v := range vaults {
		name := v
		out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*backup.DescribeBackupVaultOutput, error) {
			return c.Backup.DescribeBackupVault(ctx, &backup.DescribeBackupVaultInput{
				BackupVaultName: &name,
			})
		})
		if err != nil {
			return resource.RelatedCheckResult{TargetType: "kms", Count: -1, Err: err}
		}
		if out == nil || out.EncryptionKeyArn == nil || *out.EncryptionKeyArn == "" {
			continue
		}
		arn := *out.EncryptionKeyArn
		keyID := arn
		if idx := strings.LastIndex(arn, "/"); idx >= 0 && idx < len(arn)-1 {
			keyID = arn[idx+1:]
		}
		if _, dup := seen[keyID]; dup {
			continue
		}
		seen[keyID] = struct{}{}
		ids = append(ids, keyID)
	}
	return relatedResult("kms", ids)
}

// checkBackupSNS resolves the SNS topic(s) configured for this plan's target
// vaults via backup:GetBackupPlan → backup:GetBackupVaultNotifications
// (Pattern C, bounded N+1 where N = unique vaults).
func checkBackupSNS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	vaults := backupPlanVaults(ctx, clients, res)
	if vaults == nil {
		return resource.RelatedCheckResult{TargetType: "sns", Count: -1}
	}
	if len(vaults) == 0 {
		return resource.RelatedCheckResult{TargetType: "sns", Count: 0}
	}
	c, cok := clients.(*ServiceClients)
	if !cok || c == nil || c.Backup == nil {
		return resource.RelatedCheckResult{TargetType: "sns", Count: -1}
	}
	seen := make(map[string]struct{})
	var topicARNs []string
	var failures []string
	for _, v := range vaults {
		name := v
		out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*backup.GetBackupVaultNotificationsOutput, error) {
			return c.Backup.GetBackupVaultNotifications(ctx, &backup.GetBackupVaultNotificationsInput{
				BackupVaultName: &name,
			})
		})
		if err != nil {
			// ResourceNotFoundException means the vault has no notifications configured — treat as empty.
			if _, ok := errors.AsType[*backuptypes.ResourceNotFoundException](err); ok {
				continue
			}
			failures = append(failures, fmt.Sprintf("%s: %v", name, err))
			continue
		}
		if out == nil || out.SNSTopicArn == nil || *out.SNSTopicArn == "" {
			continue
		}
		arn := *out.SNSTopicArn
		if _, dup := seen[arn]; dup {
			continue
		}
		seen[arn] = struct{}{}
		topicARNs = append(topicARNs, arn)
	}
	aggErr := AggregateFailures("backup-related: GetBackupVaultNotifications", failures, len(vaults))
	if len(topicARNs) == 0 {
		if aggErr != nil {
			return resource.RelatedCheckResult{TargetType: "sns", Count: -1, Err: aggErr}
		}
		return resource.RelatedCheckResult{TargetType: "sns", Count: 0}
	}

	// Resolve topic names against sns cache (topic name is last segment of ARN).
	snsList, truncated, err := FetchRelatedTarget(ctx, clients, cache, "sns")
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			snsList = nil
		}
	}
	var ids []string
	for _, arn := range topicARNs {
		name := arn
		if idx := strings.LastIndex(arn, ":"); idx >= 0 && idx < len(arn)-1 {
			name = arn[idx+1:]
		}
		matched := false
		for _, snsRes := range snsList {
			if snsRes.ID == name || snsRes.ID == arn || snsRes.Name == name || snsRes.Fields["arn"] == arn {
				ids = append(ids, snsRes.ID)
				matched = true
				break
			}
		}
		if !matched {
			ids = append(ids, name)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("sns")
	}
	result := relatedResult("sns", ids)
	result.Err = aggErr
	return result
}

// backupPlanVaults returns the unique TargetBackupVaultName values from the
// plan's rules by calling backup:GetBackupPlan once. Returns nil on API
// failure (so callers can distinguish "unknown" from "empty").
func backupPlanVaults(ctx context.Context, clients any, res resource.Resource) []string {
	planID := res.ID
	if planID == "" {
		return nil
	}
	c, cok := clients.(*ServiceClients)
	if !cok || c == nil || c.Backup == nil {
		return nil
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*backup.GetBackupPlanOutput, error) {
		return c.Backup.GetBackupPlan(ctx, &backup.GetBackupPlanInput{
			BackupPlanId: &planID,
		})
	})
	if err != nil || out == nil || out.BackupPlan == nil {
		return nil
	}
	seen := make(map[string]struct{})
	var names []string
	for _, rule := range out.BackupPlan.Rules {
		if rule.TargetBackupVaultName == nil || *rule.TargetBackupVaultName == "" {
			continue
		}
		n := *rule.TargetBackupVaultName
		if _, dup := seen[n]; dup {
			continue
		}
		seen[n] = struct{}{}
		names = append(names, n)
	}
	return names
}
