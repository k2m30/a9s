package unit_test

import (
	"context"
	"testing"

	_ "github.com/k2m30/a9s/v3/internal/aws"
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
