package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	backuptypes "github.com/aws/aws-sdk-go-v2/service/backup/types"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	efstypes "github.com/aws/aws-sdk-go-v2/service/efs/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	awsclient "github.com/k2m30/a9s/v3/internal/aws"
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

// --- Backup checker tests (Pattern A — direct API call) ---

const efsTestFSARN = "arn:aws:elasticfilesystem:us-east-1:123456789012:file-system/fs-0a1b2c3d4e5f60001"
const efsBackupRecoveryARN1 = "arn:aws:backup:us-east-1:123456789012:recovery-point:rp-efs-aaa"
const efsBackupRecoveryARN2 = "arn:aws:backup:us-east-1:123456789012:recovery-point:rp-efs-bbb"

func efsSrcResourceWithARN() resource.Resource {
	return resource.Resource{
		ID:   "fs-0a1b2c3d4e5f60001",
		Name: "prod-shared-efs",
		Fields: map[string]string{},
		RawStruct: efstypes.FileSystemDescription{
			FileSystemId:  aws.String("fs-0a1b2c3d4e5f60001"),
			FileSystemArn: aws.String(efsTestFSARN),
		},
	}
}

// TestRelated_EFS_Backup_Match verifies that two recovery points returned by
// the fake produce Count=2 with both ARNs in ResourceIDs.
func TestRelated_EFS_Backup_Match(t *testing.T) {
	fake := newFakeBackupWithRecoveryPoints([]backuptypes.RecoveryPointByResource{
		{RecoveryPointArn: aws.String(efsBackupRecoveryARN1)},
		{RecoveryPointArn: aws.String(efsBackupRecoveryARN2)},
	})
	clients := &awsclient.ServiceClients{Backup: fake}
	res := efsSrcResourceWithARN()

	checker := efsCheckerByTarget(t, "backup")
	result := checker(context.Background(), clients, res, nil)

	if result.Count != 2 {
		t.Fatalf("Count = %d, want 2", result.Count)
	}
	if len(result.ResourceIDs) != 2 {
		t.Fatalf("ResourceIDs length = %d, want 2: %v", len(result.ResourceIDs), result.ResourceIDs)
	}
	seen := map[string]bool{}
	for _, id := range result.ResourceIDs {
		seen[id] = true
	}
	for _, want := range []string{efsBackupRecoveryARN1, efsBackupRecoveryARN2} {
		if !seen[want] {
			t.Errorf("ResourceIDs missing %q; got %v", want, result.ResourceIDs)
		}
	}
	if result.Err != nil {
		t.Errorf("unexpected Err: %v", result.Err)
	}
}

// TestRelated_EFS_Backup_Empty verifies that zero recovery points produce Count=0.
func TestRelated_EFS_Backup_Empty(t *testing.T) {
	fake := newFakeBackupWithRecoveryPoints([]backuptypes.RecoveryPointByResource{})
	clients := &awsclient.ServiceClients{Backup: fake}
	res := efsSrcResourceWithARN()

	checker := efsCheckerByTarget(t, "backup")
	result := checker(context.Background(), clients, res, nil)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no recovery points)", result.Count)
	}
	if len(result.ResourceIDs) != 0 {
		t.Errorf("ResourceIDs = %v, want empty", result.ResourceIDs)
	}
}

// TestRelated_EFS_Backup_WrongRawStruct verifies that a wrong RawStruct type
// returns Count=-1 (defensive guard, assertStruct fails).
func TestRelated_EFS_Backup_WrongRawStruct(t *testing.T) {
	res := resource.Resource{
		ID:        "fs-0a1b2c3d4e5f60001",
		RawStruct: "not-a-filesystem",
	}
	checker := efsCheckerByTarget(t, "backup")
	result := checker(context.Background(), nil, res, nil)

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (wrong RawStruct type)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// efs→ecs-task (Pattern C+reverse: cache["ecs-task"] scan, EfsVolumeConfiguration.FileSystemId)
// ---------------------------------------------------------------------------

// efsTaskDefWithVolume builds an ecs-task cache entry whose TaskDefinition has
// a volume with EfsVolumeConfiguration.FileSystemId = fsID.
func efsTaskDefWithVolume(taskID, fsID string) resource.Resource {
	return resource.Resource{
		ID:   taskID,
		Name: taskID,
		RawStruct: ecstypes.TaskDefinition{
			TaskDefinitionArn: aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/" + taskID + ":1"),
			Volumes: []ecstypes.Volume{
				{
					Name: aws.String("efs-data"),
					EfsVolumeConfiguration: &ecstypes.EFSVolumeConfiguration{
						FileSystemId: aws.String(fsID),
					},
				},
			},
		},
	}
}

// efsTaskDefNoEFSVolume builds an ecs-task cache entry with no EFS volume.
func efsTaskDefNoEFSVolume(taskID string) resource.Resource {
	return resource.Resource{
		ID:   taskID,
		Name: taskID,
		RawStruct: ecstypes.TaskDefinition{
			TaskDefinitionArn: aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/" + taskID + ":1"),
			Volumes: []ecstypes.Volume{
				{
					Name: aws.String("local-tmp"),
					Host: &ecstypes.HostVolumeProperties{
						SourcePath: aws.String("/tmp"),
					},
				},
			},
		},
	}
}

// efsSourceResource builds an EFS filesystem resource used as the parent.
func efsSourceResource(fsID string) resource.Resource {
	return resource.Resource{
		ID:   fsID,
		Name: fsID,
		RawStruct: efstypes.FileSystemDescription{
			FileSystemId: aws.String(fsID),
		},
	}
}

// TestRelated_EFS_ECSTask_Match verifies that a task definition whose
// EfsVolumeConfiguration.FileSystemId matches the parent filesystem is returned
// with Count=1.
func TestRelated_EFS_ECSTask_Match(t *testing.T) {
	const fsID = "fs-0a1b2c3d4e5f60001"
	const taskID = "api-task"

	cache := resource.ResourceCache{
		"ecs-task": resource.ResourceCacheEntry{
			Resources: []resource.Resource{efsTaskDefWithVolume(taskID, fsID)},
		},
	}

	checker := efsCheckerByTarget(t, "ecs-task")
	result := checker(context.Background(), nil, efsSourceResource(fsID), cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != taskID {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, taskID)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

// TestRelated_EFS_ECSTask_Match_Truncated verifies that IsTruncated propagates to
// Approximate=true while Count still reflects found matches.
func TestRelated_EFS_ECSTask_Match_Truncated(t *testing.T) {
	const fsID = "fs-0a1b2c3d4e5f60001"
	const taskID = "api-task"

	cache := resource.ResourceCache{
		"ecs-task": resource.ResourceCacheEntry{
			Resources:   []resource.Resource{efsTaskDefWithVolume(taskID, fsID)},
			IsTruncated: true,
		},
	}

	checker := efsCheckerByTarget(t, "ecs-task")
	result := checker(context.Background(), nil, efsSourceResource(fsID), cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if !result.Approximate {
		t.Error("Approximate = false, want true (cache is truncated)")
	}
}

// TestRelated_EFS_ECSTask_Empty verifies that a cache containing only task defs
// without EFS volumes returns Count=0.
func TestRelated_EFS_ECSTask_Empty(t *testing.T) {
	const fsID = "fs-0a1b2c3d4e5f60001"

	cache := resource.ResourceCache{
		"ecs-task": resource.ResourceCacheEntry{
			Resources: []resource.Resource{efsTaskDefNoEFSVolume("worker-task")},
		},
	}

	checker := efsCheckerByTarget(t, "ecs-task")
	result := checker(context.Background(), nil, efsSourceResource(fsID), cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no task mounts this filesystem)", result.Count)
	}
}

// TestRelated_EFS_ECSTask_DifferentFS verifies that task defs mounting a different
// EFS filesystem are not returned.
func TestRelated_EFS_ECSTask_DifferentFS(t *testing.T) {
	const fsID = "fs-0a1b2c3d4e5f60001"
	const otherFSID = "fs-9999999999999999"

	cache := resource.ResourceCache{
		"ecs-task": resource.ResourceCacheEntry{
			Resources: []resource.Resource{efsTaskDefWithVolume("other-task", otherFSID)},
		},
	}

	checker := efsCheckerByTarget(t, "ecs-task")
	result := checker(context.Background(), nil, efsSourceResource(fsID), cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (task mounts different filesystem)", result.Count)
	}
}

// TestRelated_EFS_ECSTask_MissingCache verifies that a missing "ecs-task" cache
// key returns the zero-value (Count=0), not Count=-1.
func TestRelated_EFS_ECSTask_MissingCache(t *testing.T) {
	checker := efsCheckerByTarget(t, "ecs-task")
	result := checker(context.Background(), nil, efsSourceResource("fs-0a1b2c3d4e5f60001"), resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (cache key missing returns zero-value)", result.Count)
	}
}

// TestRelated_EFS_ECSTask_FetchFilter verifies that the checker does NOT populate
// FetchFilter — reverse-scan checkers must not set FetchFilter (Fix 3).
func TestRelated_EFS_ECSTask_FetchFilter(t *testing.T) {
	const fsID = "fs-0a1b2c3d4e5f60001"

	cache := resource.ResourceCache{
		"ecs-task": resource.ResourceCacheEntry{Resources: []resource.Resource{}},
	}

	checker := efsCheckerByTarget(t, "ecs-task")
	result := checker(context.Background(), nil, efsSourceResource(fsID), cache)

	if len(result.FetchFilter) != 0 {
		t.Errorf("FetchFilter = %v, want empty (reverse-scan checkers must not set FetchFilter)", result.FetchFilter)
	}
}

// TestRelated_EFS_ECSTask_MultipleMatches verifies that multiple task defs
// mounting the same filesystem are all returned.
func TestRelated_EFS_ECSTask_MultipleMatches(t *testing.T) {
	const fsID = "fs-0a1b2c3d4e5f60001"

	cache := resource.ResourceCache{
		"ecs-task": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				efsTaskDefWithVolume("api-task", fsID),
				efsTaskDefWithVolume("worker-task", fsID),
				efsTaskDefNoEFSVolume("unrelated-task"),
			},
		},
	}

	checker := efsCheckerByTarget(t, "ecs-task")
	result := checker(context.Background(), nil, efsSourceResource(fsID), cache)

	if result.Count != 2 {
		t.Errorf("Count = %d, want 2", result.Count)
	}
	if len(result.ResourceIDs) != 2 {
		t.Errorf("ResourceIDs = %v, want 2 entries", result.ResourceIDs)
	}
}
