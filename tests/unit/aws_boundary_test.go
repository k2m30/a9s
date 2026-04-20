// aws_boundary_test.go — US3 boundary semantic tests for implemented
// related-panel checkers. These tests verify generic contract semantics across
// representative checkers (different parent types) rather than duplicating the
// per-pair tests in aws_<parent>_related_test.go.
//
// Test tasks: T110–T114 from specs/019-related-panel-checkers/tasks.md
package unit_test

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	cbtypes "github.com/aws/aws-sdk-go-v2/service/codebuild/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	efstypes "github.com/aws/aws-sdk-go-v2/service/efs/types"

	_ "github.com/k2m30/a9s/v3/internal/aws" // trigger init() registrations
	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// T110 — TestChecker_AccessDenied_ReturnsMinusOne
//
// For 3 representative forward checkers from different parent types, inject a
// fake client that returns AccessDeniedException and verify:
//   - result.Count == -1
//   - result.Err != nil
//
// Covered checkers:
//   asg → vpc  (EC2.DescribeSubnets)
//   ddb → backup (Backup.ListRecoveryPointsByResource)
//   ddb → kinesis (DynamoDB.DescribeKinesisStreamingDestination)
// ---------------------------------------------------------------------------

func TestChecker_AccessDenied_ReturnsMinusOne(t *testing.T) {
	t.Run("asg_vpc", func(t *testing.T) {
		parent := resource.Resource{
			ID:     "my-asg",
			Fields: map[string]string{},
			RawStruct: asgtypes.AutoScalingGroup{
				AutoScalingGroupName: aws.String("my-asg"),
				// Non-empty VPCZoneIdentifier so the checker proceeds to the AWS call.
				VPCZoneIdentifier: aws.String("subnet-0aaa111111111111a"),
			},
		}
		clients := &awsclient.ServiceClients{
			EC2: fakeEC2BoundaryAccessDenied{},
		}
		checker := boundaryCheckerByTarget(t, "asg", "vpc")
		got := checker(context.Background(), clients, parent, nil)
		if got.Count != -1 {
			t.Errorf("Count = %d, want -1 (AccessDenied on DescribeSubnets)", got.Count)
		}
		if got.Err == nil {
			t.Error("Err = nil, want non-nil (AccessDenied must propagate)")
		}
	})

	t.Run("ddb_backup", func(t *testing.T) {
		parent := resource.Resource{
			ID:     "my-table",
			Fields: map[string]string{"arn": "arn:aws:dynamodb:us-east-1:123456789012:table/my-table"},
		}
		clients := &awsclient.ServiceClients{
			Backup: fakeBackupBoundaryAccessDenied{},
		}
		checker := boundaryCheckerByTarget(t, "ddb", "backup")
		got := checker(context.Background(), clients, parent, nil)
		if got.Count != -1 {
			t.Errorf("Count = %d, want -1 (AccessDenied on ListRecoveryPointsByResource)", got.Count)
		}
		if got.Err == nil {
			t.Error("Err = nil, want non-nil (AccessDenied must propagate)")
		}
	})

	t.Run("ddb_kinesis", func(t *testing.T) {
		parent := resource.Resource{
			ID:     "my-table",
			Fields: map[string]string{},
		}
		clients := &awsclient.ServiceClients{
			DynamoDB: fakeDynamoDBBoundaryAccessDenied{},
		}
		checker := boundaryCheckerByTarget(t, "ddb", "kinesis")
		got := checker(context.Background(), clients, parent, nil)
		if got.Count != -1 {
			t.Errorf("Count = %d, want -1 (AccessDenied on DescribeKinesisStreamingDestination)", got.Count)
		}
		if got.Err == nil {
			t.Error("Err = nil, want non-nil (AccessDenied must propagate)")
		}
	})

	t.Run("kms_role", func(t *testing.T) {
		parent := resource.Resource{
			ID:   "a1b2c3d4-0001-0001-0001-000000000077",
			Name: "alias/boundary-key",
		}
		clients := &awsclient.ServiceClients{
			KMS: fakeKMSBoundaryAccessDenied{},
		}
		checker := boundaryCheckerByTarget(t, "kms", "role")
		got := checker(context.Background(), clients, parent, nil)
		if got.Count != -1 {
			t.Errorf("Count = %d, want -1 (AccessDenied on GetKeyPolicy)", got.Count)
		}
		if got.Err == nil {
			t.Error("Err = nil, want non-nil (AccessDenied must propagate)")
		}
	})
}

// ---------------------------------------------------------------------------
// T111 — TestChecker_RetryOnThrottle_WrapsCall
//
// For 2 forward checkers (different services), inject a fake whose first call
// returns a throttling error and subsequent calls succeed. Assert:
//   - result.Count matches the successful response
//   - the fake's call count is > 1 (RetryOnThrottle issued at least one retry)
//
// Covered checkers:
//   asg → vpc  (EC2.DescribeSubnets)
//   ddb → backup (Backup.ListRecoveryPointsByResource)
//
// NOTE: DefaultRetryConfig() uses a 500ms base delay with jitter. We override
// with a 1ms delay via SetRetryConfigForTest so the retry path still executes
// the full backoff-and-retry flow without taking ~500ms per sub-test.
// ---------------------------------------------------------------------------

func TestChecker_RetryOnThrottle_WrapsCall(t *testing.T) {
	restore := awsclient.SetRetryConfigForTest(&awsclient.RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   1 * time.Millisecond,
		MaxDelay:    10 * time.Millisecond,
		Jitter:      false,
	})
	t.Cleanup(restore)

	t.Run("asg_vpc", func(t *testing.T) {
		fakeEC2 := &fakeEC2BoundaryThrottle{
			vpcIDs: []string{"vpc-0abc1234567890000"},
		}
		parent := resource.Resource{
			ID:     "my-asg",
			Fields: map[string]string{},
			RawStruct: asgtypes.AutoScalingGroup{
				AutoScalingGroupName: aws.String("my-asg"),
				VPCZoneIdentifier:    aws.String("subnet-0aaa111111111111a"),
			},
		}
		clients := &awsclient.ServiceClients{
			EC2: fakeEC2,
		}
		checker := boundaryCheckerByTarget(t, "asg", "vpc")
		got := checker(context.Background(), clients, parent, nil)

		// The fake's first call throttled; second call returned one VPC ID.
		calls := fakeEC2.calls.Load()
		if calls < 2 {
			t.Errorf("DescribeSubnets call count = %d, want >= 2 (retry must have fired)", calls)
		}
		if got.Count != 1 {
			t.Errorf("Count = %d, want 1 (successful retry returned one VPC)", got.Count)
		}
		if got.Err != nil {
			t.Errorf("Err = %v, want nil (successful retry should clear error)", got.Err)
		}
	})

	t.Run("ddb_backup", func(t *testing.T) {
		rpARN := "arn:aws:backup:us-east-1:123456789012:recovery-point:rp-0001"
		fakeBackup := &fakeBackupBoundaryThrottle{
			recoveryPoint: rpARN,
		}
		parent := resource.Resource{
			ID:     "my-table",
			Fields: map[string]string{"arn": "arn:aws:dynamodb:us-east-1:123456789012:table/my-table"},
		}
		clients := &awsclient.ServiceClients{
			Backup: fakeBackup,
		}
		checker := boundaryCheckerByTarget(t, "ddb", "backup")
		got := checker(context.Background(), clients, parent, nil)

		calls := fakeBackup.calls.Load()
		if calls < 2 {
			t.Errorf("ListRecoveryPointsByResource call count = %d, want >= 2 (retry must have fired)", calls)
		}
		if got.Count != 1 {
			t.Errorf("Count = %d, want 1 (successful retry returned one recovery point)", got.Count)
		}
		if got.Err != nil {
			t.Errorf("Err = %v, want nil (successful retry should clear error)", got.Err)
		}
	})
}

// ---------------------------------------------------------------------------
// T112 — TestChecker_Approximate_PropagatedFromCache
//
// For 2 reverse-scan checkers, call the checker twice:
//   1. cache has matching resource, IsTruncated=false → Approximate must be false
//   2. same cache entry but IsTruncated=true → Approximate must be true
//
// Both calls must return Count > 0 (real match in cache).
//
// Covered checkers:
//   ecr → ecs  (checkECRECS — reverse-scan via cache["ecs-task"])
//   efs → ecs-task (checkEFSECSTask — reverse-scan via cache["ecs-task"])
// ---------------------------------------------------------------------------

func TestChecker_Approximate_PropagatedFromCache(t *testing.T) {
	t.Run("ecr_ecs", func(t *testing.T) {
		const repoName = "boundary-repo"
		const account = "111111111111"
		const region = "us-east-1"
		imageURI := account + ".dkr.ecr." + region + ".amazonaws.com/" + repoName + ":latest"

		source := resource.Resource{
			ID:   repoName,
			Name: repoName,
			Fields: map[string]string{
				"region": region,
			},
			RawStruct: ecrtypes.Repository{
				RepositoryName: aws.String(repoName),
				RegistryId:     aws.String(account),
			},
		}
		taskRes := resource.Resource{
			ID:   "task-boundary:1",
			Name: "task-boundary:1",
			// checkECRECSTask scans Fields for ".dkr.ecr." + "/repoName" patterns.
			Fields: map[string]string{
				"image_0": imageURI,
			},
			RawStruct: ecstypes.TaskDefinition{
				Family: aws.String("task-boundary"),
				ContainerDefinitions: []ecstypes.ContainerDefinition{
					{Image: aws.String(imageURI)},
				},
			},
		}

		checker := boundaryCheckerByTarget(t, "ecr", "ecs-task")

		// First call: complete cache (IsTruncated=false)
		exact := resource.ResourceCache{
			"ecs-task": resource.ResourceCacheEntry{
				Resources:   []resource.Resource{taskRes},
				IsTruncated: false,
			},
		}
		gotExact := checker(context.Background(), nil, source, exact)
		if gotExact.Count < 1 {
			t.Errorf("exact cache: Count = %d, want >= 1", gotExact.Count)
		}
		if gotExact.Approximate {
			t.Error("exact cache: Approximate = true, want false (IsTruncated=false)")
		}

		// Second call: truncated cache (IsTruncated=true)
		truncated := resource.ResourceCache{
			"ecs-task": resource.ResourceCacheEntry{
				Resources:   []resource.Resource{taskRes},
				IsTruncated: true,
			},
		}
		gotTruncated := checker(context.Background(), nil, source, truncated)
		if gotTruncated.Count < 1 {
			t.Errorf("truncated cache: Count = %d, want >= 1", gotTruncated.Count)
		}
		if !gotTruncated.Approximate {
			t.Error("truncated cache: Approximate = false, want true (IsTruncated=true)")
		}
	})

	t.Run("efs_ecs_task", func(t *testing.T) {
		const fsID = "fs-0boundary1234567"

		efsSource := resource.Resource{
			ID:   fsID,
			Name: fsID,
			RawStruct: efstypes.FileSystemDescription{
				FileSystemId: aws.String(fsID),
			},
		}
		taskRes := resource.Resource{
			ID:   "efs-boundary-task",
			Name: "efs-boundary-task",
			RawStruct: ecstypes.TaskDefinition{
				TaskDefinitionArn: aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/efs-boundary-task:1"),
				Volumes: []ecstypes.Volume{
					{
						Name: aws.String("efs-vol"),
						EfsVolumeConfiguration: &ecstypes.EFSVolumeConfiguration{
							FileSystemId: aws.String(fsID),
						},
					},
				},
			},
		}

		checker := boundaryCheckerByTarget(t, "efs", "ecs-task")

		// First call: complete cache (IsTruncated=false)
		exact := resource.ResourceCache{
			"ecs-task": resource.ResourceCacheEntry{
				Resources:   []resource.Resource{taskRes},
				IsTruncated: false,
			},
		}
		gotExact := checker(context.Background(), nil, efsSource, exact)
		if gotExact.Count < 1 {
			t.Errorf("exact cache: Count = %d, want >= 1", gotExact.Count)
		}
		if gotExact.Approximate {
			t.Error("exact cache: Approximate = true, want false (IsTruncated=false)")
		}

		// Second call: truncated cache (IsTruncated=true)
		truncated := resource.ResourceCache{
			"ecs-task": resource.ResourceCacheEntry{
				Resources:   []resource.Resource{taskRes},
				IsTruncated: true,
			},
		}
		gotTruncated := checker(context.Background(), nil, efsSource, truncated)
		if gotTruncated.Count < 1 {
			t.Errorf("truncated cache: Count = %d, want >= 1", gotTruncated.Count)
		}
		if !gotTruncated.Approximate {
			t.Error("truncated cache: Approximate = false, want true (IsTruncated=true)")
		}
	})
}

// ---------------------------------------------------------------------------
// T113 — TestChecker_DedupsDuplicateIDs
//
// For asg → vpc (checkASGVPC), construct an input where the AWS response
// contains 5 subnets belonging to 2 unique VPC IDs (3 duplicates). Assert
// result.Count == 2, proving relatedResult deduplicates.
// ---------------------------------------------------------------------------

func TestChecker_DedupsDuplicateIDs(t *testing.T) {
	// Subnets: 3 in vpc-0001, 2 in vpc-0002 → after dedup: 2 unique VPCs.
	const vpcA = "vpc-0dedup0000000001"
	const vpcB = "vpc-0dedup0000000002"
	subnetIDs := "subnet-aa01,subnet-aa02,subnet-aa03,subnet-bb01,subnet-bb02"

	fakeEC2 := newFakeEC2WithSubnets([]ec2types.Subnet{
		{SubnetId: aws.String("subnet-aa01"), VpcId: aws.String(vpcA)},
		{SubnetId: aws.String("subnet-aa02"), VpcId: aws.String(vpcA)},
		{SubnetId: aws.String("subnet-aa03"), VpcId: aws.String(vpcA)},
		{SubnetId: aws.String("subnet-bb01"), VpcId: aws.String(vpcB)},
		{SubnetId: aws.String("subnet-bb02"), VpcId: aws.String(vpcB)},
	})
	clients := &awsclient.ServiceClients{
		EC2: fakeEC2,
	}
	parent := resource.Resource{
		ID:     "dedup-asg",
		Fields: map[string]string{},
		RawStruct: asgtypes.AutoScalingGroup{
			AutoScalingGroupName: aws.String("dedup-asg"),
			// 5 subnets across 2 VPCs
			VPCZoneIdentifier: aws.String(subnetIDs),
		},
	}

	checker := boundaryCheckerByTarget(t, "asg", "vpc")
	got := checker(context.Background(), clients, parent, resource.ResourceCache{})

	if got.Count != 2 {
		t.Errorf("Count = %d, want 2 (5 subnets deduped to 2 VPC IDs)", got.Count)
	}
	if len(got.ResourceIDs) != 2 {
		t.Errorf("len(ResourceIDs) = %d, want 2", len(got.ResourceIDs))
	}
	if got.Err != nil {
		t.Errorf("unexpected error: %v", got.Err)
	}
}

// ---------------------------------------------------------------------------
// T114 — TestChecker_NilClients_ReturnsMinusOne
//
// For 3 representative forward checkers (different parent types), call with
// clients == nil and a valid parent. Assert Count == -1 and no panic.
//
// Covered checkers:
//   asg → vpc  (checkASGVPC)
//   ddb → backup (checkDdbBackup)
//   ddb → kinesis (checkDdbKinesis)
// ---------------------------------------------------------------------------

func TestChecker_NilClients_ReturnsMinusOne(t *testing.T) {
	t.Run("asg_vpc", func(t *testing.T) {
		parent := resource.Resource{
			ID:     "my-asg",
			Fields: map[string]string{},
			RawStruct: asgtypes.AutoScalingGroup{
				AutoScalingGroupName: aws.String("my-asg"),
				VPCZoneIdentifier:    aws.String("subnet-0aaa111111111111a"),
			},
		}
		checker := boundaryCheckerByTarget(t, "asg", "vpc")
		// Ensure no panic occurs when clients is nil.
		got := checker(context.Background(), nil, parent, resource.ResourceCache{})
		if got.Count != -1 {
			t.Errorf("Count = %d, want -1 (nil clients must return -1)", got.Count)
		}
	})

	t.Run("ddb_backup", func(t *testing.T) {
		parent := resource.Resource{
			ID:     "my-table",
			Fields: map[string]string{"arn": "arn:aws:dynamodb:us-east-1:123456789012:table/my-table"},
		}
		checker := boundaryCheckerByTarget(t, "ddb", "backup")
		got := checker(context.Background(), nil, parent, resource.ResourceCache{})
		if got.Count != -1 {
			t.Errorf("Count = %d, want -1 (nil clients must return -1)", got.Count)
		}
	})

	t.Run("ddb_kinesis", func(t *testing.T) {
		parent := resource.Resource{
			ID:     "my-table",
			Fields: map[string]string{},
		}
		checker := boundaryCheckerByTarget(t, "ddb", "kinesis")
		got := checker(context.Background(), nil, parent, resource.ResourceCache{})
		if got.Count != -1 {
			t.Errorf("Count = %d, want -1 (nil clients must return -1)", got.Count)
		}
	})
}

// ---------------------------------------------------------------------------
// T115 — TestReverseScans_DoNotPopulateFetchFilter
//
// Locks the contract that reverse-scan checkers (pattern C+reverse) never set
// FetchFilter in their result. Tests 3 representative checkers from different
// parent types with empty-cache inputs (ensures execution reaches the cache-scan
// path, not an early-exit due to missing client).
//
// Covered checkers:
//   cb    → pipeline  (checkCbPipeline — cache-miss path, empty result)
//   ecs-svc → eb-rule (checkECSSvcEbRule — cache-miss path, empty result)
//   secrets → ecs-task (checkSecretsECSTask — wrong RawStruct, early exit, still no FetchFilter)
// ---------------------------------------------------------------------------

func TestReverseScans_DoNotPopulateFetchFilter(t *testing.T) {
	t.Run("cb_pipeline", func(t *testing.T) {
		parent := resource.Resource{
			ID:   "my-project",
			Name: "my-project",
			RawStruct: cbtypes.Project{
				Name: aws.String("my-project"),
			},
		}
		checker := boundaryCheckerByTarget(t, "cb", "pipeline")
		result := checker(context.Background(), nil, parent, resource.ResourceCache{})
		if len(result.FetchFilter) != 0 {
			t.Errorf("cb→pipeline: FetchFilter = %v, want empty (reverse-scan must not set FetchFilter)", result.FetchFilter)
		}
	})

	t.Run("ecs_svc_eb_rule", func(t *testing.T) {
		parent := resource.Resource{
			ID:     "api-svc",
			Fields: map[string]string{"cluster": "prod-cluster"},
		}
		cache := resource.ResourceCache{
			"eb-rule": resource.ResourceCacheEntry{Resources: []resource.Resource{}},
		}
		checker := boundaryCheckerByTarget(t, "ecs-svc", "eb-rule")
		result := checker(context.Background(), nil, parent, cache)
		if len(result.FetchFilter) != 0 {
			t.Errorf("ecs-svc→eb-rule: FetchFilter = %v, want empty (reverse-scan must not set FetchFilter)", result.FetchFilter)
		}
	})

	t.Run("secrets_ecs_task_early_exit", func(t *testing.T) {
		// Wrong RawStruct type triggers early exit at assertStruct check.
		// Even on error paths, FetchFilter must not be set.
		parent := resource.Resource{
			ID:        "arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/key",
			RawStruct: nil,
		}
		checker := boundaryCheckerByTarget(t, "secrets", "ecs-task")
		result := checker(context.Background(), nil, parent, resource.ResourceCache{})
		if len(result.FetchFilter) != 0 {
			t.Errorf("secrets→ecs-task: FetchFilter = %v, want empty (reverse-scan must not set FetchFilter)", result.FetchFilter)
		}
	})
}

// ---------------------------------------------------------------------------
// boundaryCheckerByTarget is a test helper that retrieves the registered
// RelatedChecker for the given parent/target pair. Fails the test if not found
// or if the checker is nil.
// ---------------------------------------------------------------------------

func boundaryCheckerByTarget(t *testing.T, parent, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated(parent) {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("%s → %s: checker is nil", parent, target)
			}
			return def.Checker
		}
	}
	t.Fatalf("%s → %s: no related def registered", parent, target)
	return nil
}
