package unit

// Live (demo-client) coverage tests for checkRolePolicy.
// These complement aws_iam_roles_related_test.go (package unit_test) which only
// exercises nil-client and cache-based paths. Here we use demo.NewServiceClients()
// to cover the two paths not yet covered at 66.7%:
//
//   - RawStruct fallback (lines 150-153): empty ID + valid RawStruct.RoleName → resolved
//   - AWS-managed-only role: acme-eks-node-role has only `:aws:policy/` ARNs → Count=0
//   - Happy path: acme-lambda-execution has 2 customer-managed policies → Count=2

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func iamRolePolicyChecker(t *testing.T) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("role") {
		if def.TargetType == "policy" {
			if def.Checker == nil {
				t.Fatal("role→policy checker is nil")
			}
			return def.Checker
		}
	}
	t.Fatal("role→policy checker not registered")
	return nil
}

// TestCheckRolePolicy_HappyPath verifies that acme-lambda-execution (which has
// acme-cloudwatch-logs + acme-s3-read-only — both customer-managed) returns Count=2.
func TestCheckRolePolicy_HappyPath(t *testing.T) {
	clients := demo.NewServiceClients()
	checker := iamRolePolicyChecker(t)

	res := resource.Resource{
		ID:   "acme-lambda-execution",
		Name: "acme-lambda-execution",
	}
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.TargetType != "policy" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "policy")
	}
	if result.Count != 2 {
		t.Errorf("Count = %d, want 2 (acme-cloudwatch-logs + acme-s3-read-only); "+
			"fixture AttachedRolePolicies[\"acme-lambda-execution\"] has 2 customer-managed entries",
			result.Count)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}

	wantPolicies := map[string]bool{"acme-cloudwatch-logs": false, "acme-s3-read-only": false}
	for _, id := range result.ResourceIDs {
		wantPolicies[id] = true
	}
	for name, found := range wantPolicies {
		if !found {
			t.Errorf("policy %q not found in ResourceIDs %v", name, result.ResourceIDs)
		}
	}
}

// TestCheckRolePolicy_AWSManagedOnlyRole verifies that checkRolePolicy emits
// every attached policy name, including AWS-managed ones. The lazy-add path
// (FetchIAMPoliciesByIDsFull) resolves AWS-managed names to real entries at
// drill time — previously the checker pre-filtered by ARN which hid these
// attachments from the operator entirely.
//
// acme-eks-node-role has AmazonEKSWorkerNodePolicy, AmazonEKS_CNI_Policy,
// and AmazonEC2ContainerRegistryReadOnly attached — all AWS-managed.
// Result must be Count=len(attached), not 0.
func TestCheckRolePolicy_AWSManagedOnlyRole(t *testing.T) {
	clients := demo.NewServiceClients()
	checker := iamRolePolicyChecker(t)

	res := resource.Resource{
		ID:   "acme-eks-node-role",
		Name: "acme-eks-node-role",
	}
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.TargetType != "policy" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "policy")
	}
	if result.Count < 1 {
		t.Errorf("Count = %d, want ≥1; acme-eks-node-role has AWS-managed policies "+
			"(AmazonEKSWorkerNodePolicy, AmazonEKS_CNI_Policy, AmazonEC2ContainerRegistryReadOnly) "+
			"that the checker now emits as attached names (lazy-add resolves them at drill time)",
			result.Count)
	}
	if len(result.ResourceIDs) != result.Count {
		t.Errorf("ResourceIDs length = %d, want %d (every attached policy must surface by name)", len(result.ResourceIDs), result.Count)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

// TestCheckRolePolicy_RawStructFallback exercises the RawStruct fallback path
// (iam_roles_related.go:149-153): when resource.ID is empty but RawStruct holds
// an iamtypes.Role with a valid RoleName, the checker must resolve the name from
// RawStruct and call ListAttachedRolePolicies successfully.
func TestCheckRolePolicy_RawStructFallback(t *testing.T) {
	clients := demo.NewServiceClients()
	checker := iamRolePolicyChecker(t)

	// Empty ID but valid RawStruct — should fall through to RawStruct resolution.
	res := resource.Resource{
		ID:   "",
		Name: "",
		RawStruct: iamtypes.Role{
			RoleName: aws.String("acme-lambda-execution"),
			Arn:      aws.String("arn:aws:iam::123456789012:role/service-role/acme-lambda-execution"),
		},
	}
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.TargetType != "policy" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "policy")
	}
	// acme-lambda-execution has 2 customer-managed policies — same as HappyPath.
	if result.Count != 2 {
		t.Errorf("Count = %d, want 2; RawStruct fallback must resolve RoleName from "+
			"iamtypes.Role.RoleName when resource.ID is empty "+
			"(internal/aws/iam_roles_related.go:149-153)",
			result.Count)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}
