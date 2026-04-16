// backup_related.go contains AWS Backup related-resource checker functions.
package aws

import (
	"context"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/backup"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterRelated("backup", []resource.RelatedDef{
		{TargetType: "role", DisplayName: "IAM Roles", Checker: checkBackupRole},
		{TargetType: "eb-rule", DisplayName: "EventBridge Rules", Checker: checkBackupEBRule},
		{TargetType: "kms", DisplayName: "KMS Keys", Checker: checkBackupKMS},
		{TargetType: "logs", DisplayName: "Log Groups", Checker: checkBackupLogs},
		{TargetType: "sns", DisplayName: "SNS Topics", Checker: checkBackupSNS},
	})
}

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

// checkBackupEBRule returns Count: -1 (unknown). AWS Backup can react to
// EventBridge events (e.g. scheduled plan invocation), but the linkage lives
// inside the EventBridge rule's targets (arn:aws:backup:...:backup-vault/...
// or startBackupJob target) — not on the backup plan side. Resolving this
// requires scanning every EventBridge rule's targets for backup ARNs, which
// would be a cross-resource traversal not exposed by ListBackupPlans.
func checkBackupEBRule(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "eb-rule", Count: -1}
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

// checkBackupLogs returns Count: -1 (unknown). AWS Backup emits logs to
// CloudWatch via a service-linked log group, but the association is implicit
// and not resolvable from the BackupPlansListMember. The fetcher does not
// query the Backup service's monitoring settings.
func checkBackupLogs(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "logs", Count: -1}
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
	for _, v := range vaults {
		name := v
		out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*backup.GetBackupVaultNotificationsOutput, error) {
			return c.Backup.GetBackupVaultNotifications(ctx, &backup.GetBackupVaultNotificationsInput{
				BackupVaultName: &name,
			})
		})
		if err != nil {
			// ResourceNotFoundException is returned when no notifications are
			// configured on the vault — treat as empty, not an error.
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
	if len(topicARNs) == 0 {
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
		return resource.RelatedCheckResult{TargetType: "sns", Count: -1}
	}
	return relatedResult("sns", ids)
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
