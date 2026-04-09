package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func TestRelated_CFN_Registered(t *testing.T) {
	defs := resource.GetRelated("cfn")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for cfn")
	}

	type expectation struct {
		displayName string
		hasChecker  bool
	}
	expected := map[string]expectation{
		"role": {"IAM Roles", true},
	}
	for target, want := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == target {
				found = true
				if want.hasChecker && def.Checker == nil {
					t.Errorf("cfn %q: Checker should not be nil", target)
				}
				if !want.hasChecker && def.Checker != nil {
					t.Errorf("cfn %q: Checker should be nil (stub)", target)
				}
				if def.DisplayName != want.displayName {
					t.Errorf("cfn %q: DisplayName = %q, want %q", target, def.DisplayName, want.displayName)
				}
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found", target)
		}
	}
}

func cfnCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("cfn") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("cfn related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("cfn related checker for %s not found", target)
	return nil
}

// --- checkCfnRole tests (Pattern F — forward field lookup by ARN last segment) ---

func TestRelated_CFN_Role_MatchByRoleARN(t *testing.T) {
	roleRes := resource.Resource{
		ID:     "cfn-exec-role",
		Name:   "cfn-exec-role",
		Fields: map[string]string{},
	}
	cache := resource.ResourceCache{
		"role": resource.ResourceCacheEntry{Resources: []resource.Resource{roleRes}},
	}

	res := resource.Resource{
		ID:     "acme-prod-stack",
		Fields: map[string]string{},
		RawStruct: cfntypes.Stack{
			RoleARN: aws.String("arn:aws:iam::123456789012:role/cfn-exec-role"),
		},
	}

	checker := cfnCheckerByTarget(t, "role")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "cfn-exec-role" {
		t.Errorf("ResourceIDs = %v, want [cfn-exec-role]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_CFN_Role_NoMatch(t *testing.T) {
	roleRes := resource.Resource{
		ID:     "different-role",
		Name:   "different-role",
		Fields: map[string]string{},
	}
	cache := resource.ResourceCache{
		"role": resource.ResourceCacheEntry{Resources: []resource.Resource{roleRes}},
	}

	res := resource.Resource{
		ID:     "acme-prod-stack",
		Fields: map[string]string{},
		RawStruct: cfntypes.Stack{
			RoleARN: aws.String("arn:aws:iam::123456789012:role/cfn-exec-role"),
		},
	}

	checker := cfnCheckerByTarget(t, "role")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_CFN_Role_NilRoleARN(t *testing.T) {
	roleRes := resource.Resource{
		ID:     "cfn-exec-role",
		Name:   "cfn-exec-role",
		Fields: map[string]string{},
	}
	cache := resource.ResourceCache{
		"role": resource.ResourceCacheEntry{Resources: []resource.Resource{roleRes}},
	}

	res := resource.Resource{
		ID:     "acme-prod-stack",
		Fields: map[string]string{},
		RawStruct: cfntypes.Stack{
			RoleARN: nil,
		},
	}

	checker := cfnCheckerByTarget(t, "role")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (nil RoleARN)", result.Count)
	}
}

func TestRelated_CFN_Role_NilCache(t *testing.T) {
	cache := resource.ResourceCache{}

	res := resource.Resource{
		ID:     "acme-prod-stack",
		Fields: map[string]string{},
		RawStruct: cfntypes.Stack{
			RoleARN: aws.String("arn:aws:iam::123456789012:role/cfn-exec-role"),
		},
	}

	checker := cfnCheckerByTarget(t, "role")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (empty cache, no clients)", result.Count)
	}
}
