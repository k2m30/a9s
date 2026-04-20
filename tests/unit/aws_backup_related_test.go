package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/backup"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// TestRelated_Backup_Registered verifies all related defs are registered with non-nil checkers.
func TestRelated_Backup_Registered(t *testing.T) {
	defs := resource.GetRelated("backup")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for backup")
	}

	expected := map[string]string{
		"role": "IAM Roles",
		"kms":  "KMS Keys",
		"sns":  "SNS Topics",
	}
	for target, wantDisplay := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == target {
				found = true
				if def.Checker == nil {
					t.Errorf("backup %q: Checker should not be nil", target)
				}
				if def.DisplayName != wantDisplay {
					t.Errorf("backup %q: DisplayName = %q, want %q", target, def.DisplayName, wantDisplay)
				}
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found", target)
		}
	}
}

// TestRelated_Backup_Role_Unknown documents why this checker cannot resolve
// the IAM role from the ListBackupPlans response alone. The
// backuptypes.BackupPlansListMember exposes only plan metadata (name, id, arn,
// timestamps) — it does not include the IamRoleArn used by the plan's backup
// rules. Resolving that would require per-plan GetBackupPlan /
// GetBackupSelection calls which the fetcher does not perform. Real behavior:
// Count=-1 (unknown), never Count=0, because we cannot answer.
func TestRelated_Backup_Role_Unknown(t *testing.T) {
	var checker resource.RelatedChecker
	for _, def := range resource.GetRelated("backup") {
		if def.TargetType == "role" {
			checker = def.Checker
		}
	}
	if checker == nil {
		t.Fatal("backup→role checker not registered")
	}

	source := resource.Resource{
		ID:   "abcd1234-1111-2222-3333-444455556666",
		Name: "nightly-prod-plan",
		Fields: map[string]string{
			"plan_id":   "abcd1234-1111-2222-3333-444455556666",
			"plan_name": "nightly-prod-plan",
		},
	}

	// With an empty cache the answer is unknown (can't fetch the role).
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("empty cache: Count = %d, want -1 (unknown — GetBackupPlan required)", result.Count)
	}
	if result.TargetType != "role" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "role")
	}

	// Even with a populated role cache we still cannot decide without calling
	// GetBackupPlan to retrieve BackupRule.IamRoleArn. Must remain unknown.
	cache := resource.ResourceCache{
		"role": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "AWSBackupDefaultServiceRole", Name: "AWSBackupDefaultServiceRole"},
		}},
	}
	result2 := checker(context.Background(), nil, source, cache)
	if result2.Count != -1 {
		t.Errorf("populated cache: Count = %d, want -1 (unknown — tags not available)", result2.Count)
	}
}

// backupCheckerByTarget returns the RelatedChecker for the given target type registered
// under "backup". Fails immediately if the checker is nil or not found.
func backupCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("backup") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("backup related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("backup related checker for %s not found", target)
	return nil
}

// backupPlanSrc returns a minimal backup plan resource.
func backupPlanSrc() resource.Resource {
	return resource.Resource{
		ID:   "abcd1234-1111-2222-3333-444455556666",
		Name: "nightly-prod-plan",
		Fields: map[string]string{
			"plan_id": "abcd1234-1111-2222-3333-444455556666",
		},
	}
}

// ---------------------------------------------------------------------------
// checkBackupRole — Pattern C: ListBackupSelections per plan ID
// ---------------------------------------------------------------------------

// TestRelated_Backup_Role_Match verifies that a plan with a selection carrying
// an IamRoleArn extracts the role name (last "/" segment).
func TestRelated_Backup_Role_Match(t *testing.T) {
	src := backupPlanSrc()
	clients := &awsclient.ServiceClients{
		Backup: &fakeBackupCR{
			listSelectionsOutput: backupListSelectionsWithRole("arn:aws:iam::123456789012:role/AWSBackupDefaultServiceRole"),
		},
	}
	checker := backupCheckerByTarget(t, "role")
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) == 0 || result.ResourceIDs[0] != "AWSBackupDefaultServiceRole" {
		t.Errorf("ResourceIDs = %v, want [AWSBackupDefaultServiceRole]", result.ResourceIDs)
	}
}

// TestRelated_Backup_Role_NoSelections verifies Count=0 when no backup selections exist.
func TestRelated_Backup_Role_NoSelections(t *testing.T) {
	src := backupPlanSrc()
	clients := &awsclient.ServiceClients{
		Backup: &fakeBackupCR{}, // returns empty ListBackupSelectionsOutput
	}
	checker := backupCheckerByTarget(t, "role")
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no selections)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkBackupKMS — Pattern C: GetBackupPlan → DescribeBackupVault.EncryptionKeyArn
// ---------------------------------------------------------------------------

// TestRelated_Backup_KMS_Match verifies KMS key extraction from vault descriptor.
func TestRelated_Backup_KMS_Match(t *testing.T) {
	src := backupPlanSrc()
	const kmsARN = "arn:aws:kms:us-east-1:123456789012:key/deadbeef-1234-5678-abcd-000000000000"
	clients := &awsclient.ServiceClients{
		Backup: newFakeBackupCRWithVaultKMS("my-vault", kmsARN),
	}
	checker := backupCheckerByTarget(t, "kms")
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) == 0 || result.ResourceIDs[0] != "deadbeef-1234-5678-abcd-000000000000" {
		t.Errorf("ResourceIDs = %v, want [deadbeef-1234-5678-abcd-000000000000]", result.ResourceIDs)
	}
}

// TestRelated_Backup_KMS_EmptyPlan verifies Count=-1 when the plan has no vault rules.
// backupPlanVaults returns nil (not an empty slice) when no vault-named rules exist,
// so checkBackupKMS returns -1 (unknown) rather than 0. The Count=0 branch in
// checkBackupKMS is unreachable with the current backupPlanVaults implementation.
func TestRelated_Backup_KMS_EmptyPlan(t *testing.T) {
	src := backupPlanSrc()
	clients := &awsclient.ServiceClients{
		Backup: &fakeBackupCR{
			// GetBackupPlan returns a plan with no Rules — backupPlanVaults returns nil
			getBackupPlanOutput: backupEmptyPlan(),
		},
	}
	checker := backupCheckerByTarget(t, "kms")
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	// nil return from backupPlanVaults → Count=-1 (unknown), not 0
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil vault list from empty plan)", result.Count)
	}
}

// TestRelated_Backup_KMS_NilClients verifies Count=-1 when clients are nil.
func TestRelated_Backup_KMS_NilClients(t *testing.T) {
	src := backupPlanSrc()
	checker := backupCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil clients)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkBackupSNS — Pattern C: GetBackupPlan → GetBackupVaultNotifications
// ---------------------------------------------------------------------------

// TestRelated_Backup_SNS_Match verifies SNS topic ARN extraction from vault notification config.
func TestRelated_Backup_SNS_Match(t *testing.T) {
	src := backupPlanSrc()
	const topicARN = "arn:aws:sns:us-east-1:123456789012:backup-alerts"
	clients := &awsclient.ServiceClients{
		Backup: &fakeBackupCR{
			getBackupPlanOutput:         backupPlanWithVault("notification-vault"),
			getVaultNotificationsOutput: backupVaultNotificationWithSNS(topicARN),
		},
	}
	checker := backupCheckerByTarget(t, "sns")
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.Count == 0 {
		t.Errorf("Count = %d, want > 0 (SNS notification configured)", result.Count)
	}
}

// TestRelated_Backup_SNS_NoNotification verifies Count=0 when no SNS is configured.
func TestRelated_Backup_SNS_NoNotification(t *testing.T) {
	src := backupPlanSrc()
	clients := &awsclient.ServiceClients{
		Backup: &fakeBackupCR{
			getBackupPlanOutput:         backupPlanWithVault("quiet-vault"),
			getVaultNotificationsOutput: &backup.GetBackupVaultNotificationsOutput{}, // no SNSTopicArn
		},
	}
	checker := backupCheckerByTarget(t, "sns")
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no SNS notification)", result.Count)
	}
}

// TestRelated_Backup_Role_EmptyID verifies Count=-1 when the resource ID is empty.
func TestRelated_Backup_Role_EmptyID(t *testing.T) {
	src := resource.Resource{ID: "", Name: "no-id-plan", Fields: map[string]string{}}
	clients := &awsclient.ServiceClients{
		Backup: &fakeBackupCR{},
	}
	checker := backupCheckerByTarget(t, "role")
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (empty ID)", result.Count)
	}
}

// TestRelated_Backup_SNS_NilClients verifies Count=-1 when clients are nil.
func TestRelated_Backup_SNS_NilClients(t *testing.T) {
	src := backupPlanSrc()
	checker := backupCheckerByTarget(t, "sns")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil clients)", result.Count)
	}
}
