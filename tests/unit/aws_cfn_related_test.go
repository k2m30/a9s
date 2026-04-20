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

// --- checkCFNCFN tests (Pattern F+C — parent/child nested stacks) ---

// TestRelated_CFN_CFN_FindChildStacks verifies that stacks whose ParentId
// matches this stack's StackId are returned as child stacks.
func TestRelated_CFN_CFN_FindChildStacks(t *testing.T) {
	const parentStackID = "arn:aws:cloudformation:us-east-1:123456789012:stack/parent-stack/aaa-111"

	childRes := resource.Resource{
		ID:   "child-stack",
		Name: "child-stack",
		RawStruct: cfntypes.Stack{
			StackId:  aws.String("arn:aws:cloudformation:us-east-1:123456789012:stack/child-stack/bbb-222"),
			ParentId: aws.String(parentStackID),
		},
	}
	otherRes := resource.Resource{
		ID:   "unrelated-stack",
		Name: "unrelated-stack",
		RawStruct: cfntypes.Stack{
			StackId:  aws.String("arn:aws:cloudformation:us-east-1:123456789012:stack/unrelated-stack/ccc-333"),
			ParentId: nil,
		},
	}
	cache := resource.ResourceCache{
		"cfn": resource.ResourceCacheEntry{Resources: []resource.Resource{childRes, otherRes}},
	}

	res := resource.Resource{
		ID:   "parent-stack",
		Name: "parent-stack",
		RawStruct: cfntypes.Stack{
			StackId:  aws.String(parentStackID),
			ParentId: nil,
		},
	}

	checker := cfnCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "child-stack" {
		t.Errorf("ResourceIDs = %v, want [child-stack]", result.ResourceIDs)
	}
}

// TestRelated_CFN_CFN_FindParentStack verifies that the parent stack is
// returned for a nested (child) stack.
func TestRelated_CFN_CFN_FindParentStack(t *testing.T) {
	const parentStackID = "arn:aws:cloudformation:us-east-1:123456789012:stack/parent-stack/aaa-111"

	parentRes := resource.Resource{
		ID:   "parent-stack",
		Name: "parent-stack",
		RawStruct: cfntypes.Stack{
			StackId:  aws.String(parentStackID),
			ParentId: nil,
		},
	}
	cache := resource.ResourceCache{
		"cfn": resource.ResourceCacheEntry{Resources: []resource.Resource{parentRes}},
	}

	// This stack is the child — its ParentId points to the parent.
	res := resource.Resource{
		ID:   "child-stack",
		Name: "child-stack",
		RawStruct: cfntypes.Stack{
			StackId:  aws.String("arn:aws:cloudformation:us-east-1:123456789012:stack/child-stack/bbb-222"),
			ParentId: aws.String(parentStackID),
		},
	}

	checker := cfnCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "parent-stack" {
		t.Errorf("ResourceIDs = %v, want [parent-stack]", result.ResourceIDs)
	}
}

// TestRelated_CFN_CFN_NoRelated verifies that a standalone stack with no
// parent and no children returns Count=0.
func TestRelated_CFN_CFN_NoRelated(t *testing.T) {
	otherRes := resource.Resource{
		ID:   "unrelated-stack",
		Name: "unrelated-stack",
		RawStruct: cfntypes.Stack{
			StackId:  aws.String("arn:aws:cloudformation:us-east-1:123456789012:stack/unrelated-stack/ccc-333"),
			ParentId: nil,
		},
	}
	cache := resource.ResourceCache{
		"cfn": resource.ResourceCacheEntry{Resources: []resource.Resource{otherRes}},
	}

	res := resource.Resource{
		ID:   "standalone-stack",
		Name: "standalone-stack",
		RawStruct: cfntypes.Stack{
			StackId:  aws.String("arn:aws:cloudformation:us-east-1:123456789012:stack/standalone-stack/ddd-444"),
			ParentId: nil,
		},
	}

	checker := cfnCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

// TestRelated_CFN_CFN_CacheMissNoClients verifies that an empty cache with
// no clients returns Count=-1 (unknown).
func TestRelated_CFN_CFN_CacheMissNoClients(t *testing.T) {
	res := resource.Resource{
		ID:   "parent-stack",
		Name: "parent-stack",
		RawStruct: cfntypes.Stack{
			StackId:  aws.String("arn:aws:cloudformation:us-east-1:123456789012:stack/parent-stack/aaa-111"),
			ParentId: nil,
		},
	}

	checker := cfnCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (cache miss, no clients)", result.Count)
	}
}

// TestRelated_CFN_CFN_InvalidRawStruct verifies that a missing/wrong RawStruct
// returns Count=0 (the checker cannot identify any related stacks).
func TestRelated_CFN_CFN_InvalidRawStruct(t *testing.T) {
	res := resource.Resource{
		ID:        "some-stack",
		RawStruct: "not-a-cfn-stack",
	}

	checker := cfnCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 for invalid RawStruct", result.Count)
	}
}

// --- checkCfnSNS tests (Pattern F — reads NotificationARNs from Stack) ---

// TestRelated_CFN_SNS_FoundARNs verifies that notification ARNs are returned.
func TestRelated_CFN_SNS_FoundARNs(t *testing.T) {
	const arn1 = "arn:aws:sns:us-east-1:123456789012:cfn-deploy-topic"
	const arn2 = "arn:aws:sns:us-east-1:123456789012:cfn-alert-topic"

	res := resource.Resource{
		ID:   "acme-prod-stack",
		Name: "acme-prod-stack",
		RawStruct: cfntypes.Stack{
			NotificationARNs: []string{arn1, arn2},
		},
	}

	checker := cfnCheckerByTarget(t, "sns")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 2 {
		t.Errorf("Count = %d, want 2", result.Count)
	}
	found1, found2 := false, false
	for _, id := range result.ResourceIDs {
		if id == arn1 {
			found1 = true
		}
		if id == arn2 {
			found2 = true
		}
	}
	if !found1 || !found2 {
		t.Errorf("ResourceIDs = %v, want both [%s] and [%s]", result.ResourceIDs, arn1, arn2)
	}
}

// TestRelated_CFN_SNS_Empty verifies that an empty NotificationARNs slice
// returns Count=0.
func TestRelated_CFN_SNS_Empty(t *testing.T) {
	res := resource.Resource{
		ID:   "acme-prod-stack",
		Name: "acme-prod-stack",
		RawStruct: cfntypes.Stack{
			NotificationARNs: []string{},
		},
	}

	checker := cfnCheckerByTarget(t, "sns")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

// TestRelated_CFN_SNS_InvalidRawStruct verifies that a wrong RawStruct type
// returns Count=-1.
func TestRelated_CFN_SNS_InvalidRawStruct(t *testing.T) {
	res := resource.Resource{
		ID:        "acme-prod-stack",
		RawStruct: "not-a-stack",
	}

	checker := cfnCheckerByTarget(t, "sns")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 for invalid RawStruct", result.Count)
	}
}

// --- checkCfnS3 tests (Pattern C — nil clients → -1) ---

// TestRelated_CFN_S3_NilClients verifies that nil clients returns Count=-1.
func TestRelated_CFN_S3_NilClients(t *testing.T) {
	res := resource.Resource{
		ID:   "acme-prod-stack",
		Name: "acme-prod-stack",
		RawStruct: cfntypes.Stack{
			StackName: aws.String("acme-prod-stack"),
		},
	}

	checker := cfnCheckerByTarget(t, "s3")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil clients)", result.Count)
	}
}

// --- checkCfnEBRule tests (Pattern C — nil clients → -1) ---

// TestRelated_CFN_EBRule_NilClients verifies that nil clients returns Count=-1.
func TestRelated_CFN_EBRule_NilClients(t *testing.T) {
	res := resource.Resource{
		ID:   "acme-prod-stack",
		Name: "acme-prod-stack",
		RawStruct: cfntypes.Stack{
			StackName: aws.String("acme-prod-stack"),
		},
	}

	checker := cfnCheckerByTarget(t, "eb-rule")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil clients)", result.Count)
	}
}
