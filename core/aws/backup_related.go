// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// backup_related.go contains AWS Backup related-resource checker functions.
package aws

import (
	"context"
	"errors"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/backup"
	backuptypes "github.com/aws/aws-sdk-go-v2/service/backup/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkBackupRole resolves the IAM roles used by this plan's selections via
// a single backup:ListBackupSelections call. Each
// BackupSelectionsListMember exposes IamRoleArn directly — the role the
// Backup service assumes to protect the selection's resources.
func checkBackupRole(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	planID := res.ID
	if planID == "" {
		return resource.UnknownRelated("role")
	}
	c, cok := clients.(*ServiceClients)
	if !cok || c == nil || c.Backup == nil {
		return resource.UnknownRelated("role")
	}
	sels, complete, err := PageAll(ctx, PerParentPageCap, func(ctx context.Context, token *string) ([]backuptypes.BackupSelectionsListMember, *string, error) {
		out, err := c.Backup.ListBackupSelections(ctx, &backup.ListBackupSelectionsInput{
			BackupPlanId: &planID,
			NextToken:    token,
		})
		if err != nil {
			return nil, nil, err
		}
		return out.BackupSelectionsList, out.NextToken, nil
	})
	if err != nil {
		return resource.ErrorRelated("role", err)
	}
	var refs []string
	for _, sel := range sels {
		refs = append(refs, aws.ToString(sel.IamRoleArn))
	}
	ids, dropped := resolveRefs("role", refs, refContext(clients, cache, "role"))
	return relatedResultTrunc("role", ids, dropped || !complete)
}

// checkBackupKMS resolves the KMS key(s) encrypting this plan's target
// vaults via backup:GetBackupPlan → backup:DescribeBackupVault (bounded
// N+1 where N = unique vaults referenced by plan rules, typically 1).
func checkBackupKMS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	vaults := backupPlanVaults(ctx, clients, res)
	if vaults == nil {
		return resource.UnknownRelated("kms")
	}
	if len(vaults) == 0 {
		return resource.ProvenZero("kms", "vaults")
	}
	c, cok := clients.(*ServiceClients)
	if !cok || c == nil || c.Backup == nil {
		return resource.UnknownRelated("kms")
	}
	var refs []string
	for _, v := range vaults {
		name := v
		out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*backup.DescribeBackupVaultOutput, error) {
			return c.Backup.DescribeBackupVault(ctx, &backup.DescribeBackupVaultInput{
				BackupVaultName: &name,
			})
		})
		if err != nil {
			return resource.ErrorRelated("kms", err)
		}
		if out == nil || out.EncryptionKeyArn == nil || *out.EncryptionKeyArn == "" {
			continue
		}
		refs = append(refs, *out.EncryptionKeyArn)
	}
	return kmsRelated(ctx, clients, cache, refs)
}

// checkBackupSNS resolves the SNS topic(s) configured for this plan's target
// vaults via backup:GetBackupPlan → backup:GetBackupVaultNotifications
// (bounded N+1 where N = unique vaults).
func checkBackupSNS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	vaults := backupPlanVaults(ctx, clients, res)
	if vaults == nil {
		return resource.UnknownRelated("sns")
	}
	if len(vaults) == 0 {
		return resource.ProvenZero("sns", "vaults")
	}
	c, cok := clients.(*ServiceClients)
	if !cok || c == nil || c.Backup == nil {
		return resource.UnknownRelated("sns")
	}
	seen := make(map[string]struct{})
	var topicARNs []string
	var failures []Failure
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
			failures = append(failures, FailedCall(name, err))
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
			return resource.ErrorRelated("sns", aggErr)
		}
		return resource.ProvenZero("sns", "topicARNs")
	}

	ids, dropped := resolveRefs("sns", topicARNs, refContext(clients, cache, "sns"))
	if len(ids) == 0 && aggErr != nil {
		// Nothing was confirmed: the vault-notification failures are a plain
		// fetch failure, not a truncation signal.
		return resource.ErrorRelated("sns", aggErr)
	}
	// Some GetBackupVaultNotifications calls may have failed: ids is a proven
	// subset, not necessarily exhaustive. Truncated (not Errored) keeps the
	// row actionable rather than discarding confirmed matches as a dead end.
	return relatedResultTrunc("sns", ids, dropped || aggErr != nil)
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
	// checkBackupKMS and checkBackupSNS both turn this nil into
	// UnknownRelated.
	// no finding: the pivot renders "?" instead of a count nobody read.
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
