package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	efstypes "github.com/aws/aws-sdk-go-v2/service/efs/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func efsCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("efs") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("efs related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("efs related checker for %s not found", target)
	return nil
}

// --- Navigable Field Registration ---

func TestNavigableFields_EFS_KmsKeyId(t *testing.T) {
	nav := resource.IsFieldNavigable("efs", "KmsKeyId")
	if nav == nil {
		t.Fatal("expected navigable field KmsKeyId not found for efs")
	}
	if nav.TargetType != "kms" {
		t.Errorf("KmsKeyId TargetType = %q, want %q", nav.TargetType, "kms")
	}
}

// --- KMS checker (Pattern F) ---

func TestRelated_EFS_KMS_Encrypted(t *testing.T) {
	arn := "arn:aws:kms:us-east-1:123456789012:key/efs-key-001"
	source := resource.Resource{
		ID:     "fs-0a1b2c3d4e5f60001",
		Fields: map[string]string{},
		RawStruct: efstypes.FileSystemDescription{
			KmsKeyId: aws.String(arn),
		},
	}
	checker := efsCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "efs-key-001" {
		t.Errorf("ResourceIDs = %v, want [efs-key-001]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_EFS_KMS_NotEncrypted(t *testing.T) {
	source := resource.Resource{
		ID:     "fs-0a1b2c3d4e5f60002",
		Fields: map[string]string{},
		RawStruct: efstypes.FileSystemDescription{
			KmsKeyId: nil,
		},
	}
	checker := efsCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

// --- CFN checker (Pattern C — cache, aws:cloudformation:stack-name tag) ---

func TestRelated_EFS_CFN_FromTags(t *testing.T) {
	cfnRes := resource.Resource{
		ID:   "my-efs-stack",
		Name: "my-efs-stack",
		RawStruct: cfntypes.Stack{
			StackName: aws.String("my-efs-stack"),
		},
	}
	cache := resource.ResourceCache{
		"cfn": resource.ResourceCacheEntry{Resources: []resource.Resource{cfnRes}},
	}
	source := resource.Resource{
		ID:     "fs-0a1b2c3d4e5f60001",
		Fields: map[string]string{},
		RawStruct: efstypes.FileSystemDescription{
			Tags: []efstypes.Tag{
				{Key: aws.String("aws:cloudformation:stack-name"), Value: aws.String("my-efs-stack")},
			},
		},
	}

	checker := efsCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "my-efs-stack" {
		t.Errorf("ResourceIDs = %v, want [my-efs-stack]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_EFS_CFN_NoTag(t *testing.T) {
	cfnRes := resource.Resource{
		ID:   "some-stack",
		Name: "some-stack",
		RawStruct: cfntypes.Stack{
			StackName: aws.String("some-stack"),
		},
	}
	cache := resource.ResourceCache{
		"cfn": resource.ResourceCacheEntry{Resources: []resource.Resource{cfnRes}},
	}
	source := resource.Resource{
		ID:     "fs-0a1b2c3d4e5f60002",
		Fields: map[string]string{},
		RawStruct: efstypes.FileSystemDescription{
			Tags: []efstypes.Tag{},
		},
	}

	checker := efsCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 for file system with no CFN tag", result.Count)
	}
}

func TestRelated_EFS_CFN_CacheMissNoClients(t *testing.T) {
	source := resource.Resource{
		ID:     "fs-0a1b2c3d4e5f60001",
		Fields: map[string]string{},
		RawStruct: efstypes.FileSystemDescription{
			Tags: []efstypes.Tag{
				{Key: aws.String("aws:cloudformation:stack-name"), Value: aws.String("my-efs-stack")},
			},
		},
	}

	checker := efsCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown)", result.Count)
	}
}

// --- efs→lambda: requires live EFS DescribeAccessPoints + lambda cache scan ---

// TestRelated_EFS_Lambda_UnknownWithoutClients verifies the checker reports
// Count=-1 when no live EFS client is available. Lambda FileSystemConfigs
// carry access-point ARNs, not filesystem ARNs, so the link cannot be resolved
// from cache alone.
func TestRelated_EFS_Lambda_UnknownWithoutClients(t *testing.T) {
	source := resource.Resource{
		ID:   "fs-0a1b2c3d4e5f60001",
		Name: "prod-shared-efs",
	}
	checker := efsCheckerByTarget(t, "lambda")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (requires live efs:DescribeAccessPoints)", result.Count)
	}
	if result.TargetType != "lambda" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "lambda")
	}
}

// TestRelated_EFS_Lambda_EmptyIDReturnsZero verifies the checker short-circuits
// with Count=0 for a resource with no filesystem ID — no API call is attempted.
func TestRelated_EFS_Lambda_EmptyIDReturnsZero(t *testing.T) {
	source := resource.Resource{ID: "", Name: ""}
	checker := efsCheckerByTarget(t, "lambda")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no filesystem id)", result.Count)
	}
}
