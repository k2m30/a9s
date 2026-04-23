// aws_backup_related_test.go — related-target discovery tests for backup.
//
// Covers TEST: related_pivots_resolve_nonzero_on_graph_root (U9).
//
// Graph-root is ProdDatabasePlanID (plan-broken-2failed):
//   - Rules → TargetBackupVaultName = "acme-prod-vault"
//   - acme-prod-vault → EncryptionKeyArn = BackupProdVaultKMSKeyARN → ID "acme-prod-master-key"
//   - acme-prod-vault → SNSTopicArn = BackupAlertsSNSTopicARN → name "acme-backup-alerts"
//   - Selections → IamRoleArn = AcmeBackupRoleARN → name "AcmeBackupRoleProd"
//
// Non-graph-root baseline: HealthyDailyPlanID (plan-healthy-daily):
//   - Rules → TargetBackupVaultName = "acme-default-vault"
//   - acme-default-vault → no EncryptionKeyArn → kms count == 0
//   - acme-default-vault → no SNS topic → sns count == 0
//   - Selections → AWSBackupDefaultServiceRole → role count >= 1
//
// ct-events pivot is count-shown: unknown (spec §2) — exempt from assertions here.
package unit

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	_ "github.com/k2m30/a9s/v3/internal/aws" // register enrichers/related via init()
	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo/fakes"
	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// backupCheckerByTarget returns the RelatedChecker for the given target type
// from the backup related registry. Fails if not found or checker is nil.
func backupCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("backup") {
		if def.TargetType == target {
			require.NotNil(t, def.Checker,
				"backup related checker for %s is registered but nil", target)
			return def.Checker
		}
	}
	t.Fatalf("backup related checker for %s not found in registry", target)
	return nil
}

// backupClientsFake returns ServiceClients with only the Backup fake populated.
// Other service clients remain nil; the backup related checkers only use Backup.
func backupClientsFake() *awsclient.ServiceClients {
	return &awsclient.ServiceClients{Backup: fakes.NewBackup()}
}

// graphRootResource builds a resource.Resource for the graph-root plan
// (plan-broken-2failed) using the fixture plan member as RawStruct.
// The related checkers use res.ID for plan lookups, not RawStruct.
func graphRootResource() resource.Resource {
	fix := fixtures.NewBackupFixtures()
	var raw interface{}
	for _, p := range fix.Plans {
		if p.BackupPlanId != nil && *p.BackupPlanId == fixtures.ProdDatabasePlanID {
			raw = p
			break
		}
	}
	return resource.Resource{
		ID:        fixtures.ProdDatabasePlanID,
		Name:      "acme-prod-database",
		RawStruct: raw,
		Fields:    map[string]string{},
	}
}

// healthyPlanResource builds a resource.Resource for the healthy daily plan.
func healthyPlanResource() resource.Resource {
	fix := fixtures.NewBackupFixtures()
	var raw interface{}
	for _, p := range fix.Plans {
		if p.BackupPlanId != nil && *p.BackupPlanId == fixtures.HealthyDailyPlanID {
			raw = p
			break
		}
	}
	return resource.Resource{
		ID:        fixtures.HealthyDailyPlanID,
		Name:      "acme-daily-backup",
		RawStruct: raw,
		Fields:    map[string]string{},
	}
}

// sliceContainsSubstr returns true if any element of haystack contains needle as a substring.
func sliceContainsSubstr(haystack []string, needle string) bool {
	for _, s := range haystack {
		if strings.Contains(s, needle) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// TEST: related_pivots_resolve_nonzero_on_graph_root (U9) — role pivot
// ---------------------------------------------------------------------------

// TestBackup_Related_GraphRoot_RoleResolvesAtLeastOne verifies that the role
// checker returns Count >= 1 for the graph-root plan and that the result
// mentions "AcmeBackupRoleProd" in ResourceIDs (extracted from ARN by checker).
func TestBackup_Related_GraphRoot_RoleResolvesAtLeastOne(t *testing.T) {
	checker := backupCheckerByTarget(t, "role")
	res := graphRootResource()
	clients := backupClientsFake()

	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	require.GreaterOrEqual(t, result.Count, 1,
		"role pivot must resolve >= 1 for graph-root plan (plan-broken-2failed); got Count=%d", result.Count)
	require.NotEmpty(t, result.ResourceIDs,
		"role pivot must return non-empty ResourceIDs when Count >= 1")
	require.True(t, sliceContainsSubstr(result.ResourceIDs, "AcmeBackupRoleProd"),
		"role pivot ResourceIDs must contain 'AcmeBackupRoleProd'; got %v", result.ResourceIDs)
	require.NoError(t, result.Err, "role pivot must not return an error for graph-root")
}

// ---------------------------------------------------------------------------
// TEST: related_pivots_resolve_nonzero_on_graph_root (U9) — kms pivot
// ---------------------------------------------------------------------------

// TestBackup_Related_GraphRoot_KMSResolvesAtLeastOne verifies that the kms
// checker returns Count >= 1 for the graph-root plan, resolving through
// acme-prod-vault → EncryptionKeyArn → key ID "acme-prod-master-key".
func TestBackup_Related_GraphRoot_KMSResolvesAtLeastOne(t *testing.T) {
	checker := backupCheckerByTarget(t, "kms")
	res := graphRootResource()
	clients := backupClientsFake()

	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	require.GreaterOrEqual(t, result.Count, 1,
		"kms pivot must resolve >= 1 for graph-root plan; got Count=%d", result.Count)
	require.NotEmpty(t, result.ResourceIDs,
		"kms pivot must return non-empty ResourceIDs when Count >= 1")
	require.True(t, sliceContainsSubstr(result.ResourceIDs, fixtures.BackupProdVaultKMSKeyID),
		"kms pivot ResourceIDs must contain key ID %q; got %v",
		fixtures.BackupProdVaultKMSKeyID, result.ResourceIDs)
	require.NoError(t, result.Err, "kms pivot must not return an error for graph-root")
}

// ---------------------------------------------------------------------------
// TEST: related_pivots_resolve_nonzero_on_graph_root (U9) — sns pivot
// ---------------------------------------------------------------------------

// TestBackup_Related_GraphRoot_SNSResolvesAtLeastOne verifies that the sns
// checker returns Count >= 1 for the graph-root plan, resolving through
// acme-prod-vault → GetBackupVaultNotifications → SNSTopicArn → "acme-backup-alerts".
func TestBackup_Related_GraphRoot_SNSResolvesAtLeastOne(t *testing.T) {
	checker := backupCheckerByTarget(t, "sns")
	res := graphRootResource()
	clients := backupClientsFake()

	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	require.GreaterOrEqual(t, result.Count, 1,
		"sns pivot must resolve >= 1 for graph-root plan; got Count=%d", result.Count)
	require.NotEmpty(t, result.ResourceIDs,
		"sns pivot must return non-empty ResourceIDs when Count >= 1")
	require.True(t, sliceContainsSubstr(result.ResourceIDs, fixtures.BackupAlertsSNSTopicName),
		"sns pivot ResourceIDs must contain topic name %q; got %v",
		fixtures.BackupAlertsSNSTopicName, result.ResourceIDs)
	require.NoError(t, result.Err, "sns pivot must not return an error for graph-root")
}

// ---------------------------------------------------------------------------
// Baseline: healthy plan (acme-default-vault) — kms count == 0
// ---------------------------------------------------------------------------

// TestBackup_Related_HealthyPlan_KMS_DefaultVault_CountZero verifies that the
// kms checker returns Count == 0 for the healthy daily plan, whose rules point
// to acme-default-vault which has NO customer-managed EncryptionKeyArn (the
// VaultEncryptionKeys map does not contain BackupDefaultVaultName).
func TestBackup_Related_HealthyPlan_KMS_DefaultVault_CountZero(t *testing.T) {
	checker := backupCheckerByTarget(t, "kms")
	res := healthyPlanResource()
	clients := backupClientsFake()

	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	require.Equal(t, 0, result.Count,
		"kms pivot must return Count=0 for healthy plan using acme-default-vault (no customer-managed key)")
}

// ---------------------------------------------------------------------------
// Baseline: healthy plan (acme-default-vault) — sns count == 0
// ---------------------------------------------------------------------------

// TestBackup_Related_HealthyPlan_SNS_DefaultVault_CountZero verifies that the
// sns checker returns Count == 0 for the healthy daily plan, whose vault has no
// SNS notification configured (VaultSNSTopics does not contain BackupDefaultVaultName).
// GetBackupVaultNotifications returns ResourceNotFoundException for this vault.
func TestBackup_Related_HealthyPlan_SNS_DefaultVault_CountZero(t *testing.T) {
	checker := backupCheckerByTarget(t, "sns")
	res := healthyPlanResource()
	clients := backupClientsFake()

	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	require.Equal(t, 0, result.Count,
		"sns pivot must return Count=0 for healthy plan using acme-default-vault (no SNS topic configured)")
}

// ---------------------------------------------------------------------------
// Baseline: healthy plan — role count >= 1 (uses AWSBackupDefaultServiceRole)
// ---------------------------------------------------------------------------

// TestBackup_Related_HealthyPlan_Role_DefaultServiceRole_Resolves verifies that
// the role checker returns Count >= 1 for the healthy daily plan.
// Its selection uses AWSBackupDefaultServiceRole, which is a valid non-empty ARN.
func TestBackup_Related_HealthyPlan_Role_DefaultServiceRole_Resolves(t *testing.T) {
	checker := backupCheckerByTarget(t, "role")
	res := healthyPlanResource()
	clients := backupClientsFake()

	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	require.GreaterOrEqual(t, result.Count, 1,
		"role pivot must resolve >= 1 for healthy plan (uses AWSBackupDefaultServiceRole)")
	require.True(t, sliceContainsSubstr(result.ResourceIDs, "AWSBackupDefaultServiceRole"),
		"role pivot ResourceIDs must contain 'AWSBackupDefaultServiceRole'; got %v",
		result.ResourceIDs)
}

// ---------------------------------------------------------------------------
// Edge: empty plan ID — all pivots return Count == -1
// ---------------------------------------------------------------------------

// TestBackup_Related_EmptyPlanID_AllPivotsReturnUnknown verifies that when
// the resource has an empty ID, every pivot returns Count == -1 (unknown)
// rather than panicking or returning Count == 0 (which would be misleading).
func TestBackup_Related_EmptyPlanID_AllPivotsReturnUnknown(t *testing.T) {
	pivots := []string{"role", "kms", "sns"}
	emptyRes := resource.Resource{
		ID:     "", // empty plan ID
		Name:   "orphan",
		Fields: map[string]string{},
	}
	clients := backupClientsFake()

	for _, pivot := range pivots {
		pivot := pivot
		t.Run(pivot, func(t *testing.T) {
			checker := backupCheckerByTarget(t, pivot)
			result := checker(context.Background(), clients, emptyRes, resource.ResourceCache{})
			require.Equal(t, -1, result.Count,
				"pivot %q must return Count=-1 for empty plan ID (unknown, not zero)", pivot)
		})
	}
}

// ---------------------------------------------------------------------------
// Registry: all registered backup pivots have non-nil checkers
// ---------------------------------------------------------------------------

// TestBackup_Related_RegistryComplete verifies that backup has registered
// related definitions for role, kms, and sns — and that none have nil checkers.
// ct-events is auto-registered via the universal zzz_ct_events_all_related.go init.
func TestBackup_Related_RegistryComplete(t *testing.T) {
	defs := resource.GetRelated("backup")
	require.NotEmpty(t, defs, "backup must have related definitions registered")

	required := map[string]bool{
		"role": false,
		"kms":  false,
		"sns":  false,
	}

	for _, def := range defs {
		if _, ok := required[def.TargetType]; ok {
			required[def.TargetType] = true
		}
		require.NotNil(t, def.Checker,
			"registered backup related def for %q has nil Checker — structural bug", def.TargetType)
		require.NotEmpty(t, def.DisplayName,
			"registered backup related def for %q has empty DisplayName", def.TargetType)
	}

	for target, found := range required {
		require.True(t, found,
			"backup related registry missing required target %q", target)
	}
}
