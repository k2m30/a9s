package unit

// aws_ecs_task_join_incomplete_test.go — Regression pins for ECS task definition
// join failure behaviour in FetchECSTasksPage (via fetchECSTasksPageWithJoin)
// and checkEFSECSTask.
//
// OLD behaviour (bug):
//   When DescribeTaskDefinition returned a non-ClientException error, the fetcher
//   set Pagination.IsTruncated=true, which caused the TUI to show "m: load more"
//   and hid the real resource list.
//
// NEW behaviour (fix):
//   The fetcher sets Fields["task_def_join_error"]="true" on the affected task
//   and keeps Pagination.IsTruncated=false. The reverse-scan checker
//   (checkEFSECSTask) sets result.Approximate=true when any task carries that
//   field, surfacing an honest "~0" instead of a silently wrong "0".
//
// Tests A and B exercise the fetcher via the registered paginated closure
// (resource.GetPaginatedFetcher("ecs-task")) with a mock ECSAPI whose
// DescribeTaskDefinition returns an error (Test A) or success (Test B).
//
// Test C exercises checkEFSECSTask directly via resource.GetRelated("efs")
// with a hand-crafted ResourceCache.

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	smithy "github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// fullECSAPI — a complete ECSAPI mock. Only the methods relevant to a given
// test are overridden via function fields; the rest return empty success.
// ---------------------------------------------------------------------------

type fullECSAPI struct {
	listClustersFn         func(*ecs.ListClustersInput) (*ecs.ListClustersOutput, error)
	listTasksFn            func(*ecs.ListTasksInput) (*ecs.ListTasksOutput, error)
	describeTasksFn        func(*ecs.DescribeTasksInput) (*ecs.DescribeTasksOutput, error)
	describeTaskDefFn      func(*ecs.DescribeTaskDefinitionInput) (*ecs.DescribeTaskDefinitionOutput, error)
}

func (f *fullECSAPI) ListClusters(_ context.Context, in *ecs.ListClustersInput, _ ...func(*ecs.Options)) (*ecs.ListClustersOutput, error) {
	if f.listClustersFn != nil {
		return f.listClustersFn(in)
	}
	return &ecs.ListClustersOutput{}, nil
}

func (f *fullECSAPI) ListTasks(_ context.Context, in *ecs.ListTasksInput, _ ...func(*ecs.Options)) (*ecs.ListTasksOutput, error) {
	if f.listTasksFn != nil {
		return f.listTasksFn(in)
	}
	return &ecs.ListTasksOutput{}, nil
}

func (f *fullECSAPI) DescribeTasks(_ context.Context, in *ecs.DescribeTasksInput, _ ...func(*ecs.Options)) (*ecs.DescribeTasksOutput, error) {
	if f.describeTasksFn != nil {
		return f.describeTasksFn(in)
	}
	return &ecs.DescribeTasksOutput{}, nil
}

func (f *fullECSAPI) DescribeTaskDefinition(_ context.Context, in *ecs.DescribeTaskDefinitionInput, _ ...func(*ecs.Options)) (*ecs.DescribeTaskDefinitionOutput, error) {
	if f.describeTaskDefFn != nil {
		return f.describeTaskDefFn(in)
	}
	return &ecs.DescribeTaskDefinitionOutput{TaskDefinition: &ecstypes.TaskDefinition{}}, nil
}

// Stubs for methods not used by the ECS task fetcher.

func (f *fullECSAPI) DescribeClusters(_ context.Context, _ *ecs.DescribeClustersInput, _ ...func(*ecs.Options)) (*ecs.DescribeClustersOutput, error) {
	return &ecs.DescribeClustersOutput{}, nil
}

func (f *fullECSAPI) ListServices(_ context.Context, _ *ecs.ListServicesInput, _ ...func(*ecs.Options)) (*ecs.ListServicesOutput, error) {
	return &ecs.ListServicesOutput{}, nil
}

func (f *fullECSAPI) DescribeServices(_ context.Context, _ *ecs.DescribeServicesInput, _ ...func(*ecs.Options)) (*ecs.DescribeServicesOutput, error) {
	return &ecs.DescribeServicesOutput{}, nil
}

// Compile-time: fullECSAPI satisfies ECSAPI.
var _ awsclient.ECSAPI = (*fullECSAPI)(nil)

// ecsTaskPaginatedFetcher returns the registered paginated fetcher for "ecs-task".
// This is the production path that calls fetchECSTasksPageWithJoin (which includes
// the DescribeTaskDefinition join).
func ecsTaskPaginatedFetcher(t *testing.T) resource.PaginatedFetcher {
	t.Helper()
	f := resource.GetPaginatedFetcher("ecs-task")
	if f == nil {
		t.Fatal("ecs-task paginated fetcher not registered")
	}
	return f
}

const (
	testClusterARN = "arn:aws:ecs:us-east-1:123456789012:cluster/test-cluster"
	testTaskARN    = "arn:aws:ecs:us-east-1:123456789012:task/test-cluster/task-foo-001"
	testTaskDefARN = "arn:aws:ecs:us-east-1:123456789012:task-definition/test-app:1"
)

// buildECSTaskMockAPI returns a fully-wired fullECSAPI where:
//   - ListClusters returns one cluster.
//   - ListTasks returns one task ARN.
//   - DescribeTasks returns one task with a TaskDefinitionArn.
//   - DescribeTaskDefinition behaviour is controlled by the describeTaskDefFn param.
func buildECSTaskMockAPI(describeTaskDefFn func(*ecs.DescribeTaskDefinitionInput) (*ecs.DescribeTaskDefinitionOutput, error)) *fullECSAPI {
	return &fullECSAPI{
		listClustersFn: func(_ *ecs.ListClustersInput) (*ecs.ListClustersOutput, error) {
			return &ecs.ListClustersOutput{
				ClusterArns: []string{testClusterARN},
			}, nil
		},
		listTasksFn: func(_ *ecs.ListTasksInput) (*ecs.ListTasksOutput, error) {
			return &ecs.ListTasksOutput{
				TaskArns: []string{testTaskARN},
			}, nil
		},
		describeTasksFn: func(_ *ecs.DescribeTasksInput) (*ecs.DescribeTasksOutput, error) {
			return &ecs.DescribeTasksOutput{
				Tasks: []ecstypes.Task{
					{
						TaskArn:           aws.String(testTaskARN),
						ClusterArn:        aws.String(testClusterARN),
						LastStatus:        aws.String("RUNNING"),
						TaskDefinitionArn: aws.String(testTaskDefARN),
						LaunchType:        ecstypes.LaunchTypeFargate,
						Cpu:               aws.String("256"),
						Memory:            aws.String("512"),
					},
				},
			}, nil
		},
		describeTaskDefFn: describeTaskDefFn,
	}
}

// TestFetchECSTasksPage_JoinFailure_SetsTaskDefJoinErrorField verifies that when
// DescribeTaskDefinition returns a non-ClientException error:
//   - The fetcher does NOT propagate the error (returns the task resource).
//   - Fields["task_def_join_error"] == "true" on the affected task.
//   - Pagination.IsTruncated == false  (KEY regression pin — was true before fix).
//   - Pagination.NextToken == ""
func TestFetchECSTasksPage_JoinFailure_SetsTaskDefJoinErrorField(t *testing.T) {
	mock := buildECSTaskMockAPI(func(_ *ecs.DescribeTaskDefinitionInput) (*ecs.DescribeTaskDefinitionOutput, error) {
		return nil, &smithy.GenericAPIError{Code: "AccessDenied", Message: "denied"}
	})

	clients := &awsclient.ServiceClients{ECS: mock}
	fetcher := ecsTaskPaginatedFetcher(t)

	result, err := fetcher(context.Background(), clients, "")
	if err != nil {
		t.Fatalf("fetcher must not return an error on DescribeTaskDefinition join failure; got: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("want 1 resource (task returned despite join failure), got %d", len(result.Resources))
	}

	task := result.Resources[0]

	// KEY regression pin: before the fix this was IsTruncated=true.
	if result.Pagination == nil {
		t.Fatal("Pagination must not be nil")
	}
	if result.Pagination.IsTruncated {
		t.Errorf("IsTruncated must be false when DescribeTaskDefinition fails — "+
			"the fetcher must set Fields[task_def_join_error] instead of marking pagination truncated. "+
			"Got IsTruncated=true (OLD BUG: would surface misleading 'm: load more' in TUI).")
	}
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken must be empty on join failure (no pagination in play); got %q", result.Pagination.NextToken)
	}

	// The join error must be recorded on the task itself.
	if task.Fields["task_def_join_error"] != "true" {
		t.Errorf("Fields[task_def_join_error]: want %q, got %q", "true", task.Fields["task_def_join_error"])
	}
}

// TestFetchECSTasksPage_JoinSucceeds_NoErrorField verifies the happy path:
// when DescribeTaskDefinition succeeds, no error field is set and IsTruncated=false.
func TestFetchECSTasksPage_JoinSucceeds_NoErrorField(t *testing.T) {
	mock := buildECSTaskMockAPI(func(_ *ecs.DescribeTaskDefinitionInput) (*ecs.DescribeTaskDefinitionOutput, error) {
		return &ecs.DescribeTaskDefinitionOutput{
			TaskDefinition: &ecstypes.TaskDefinition{
				Volumes: []ecstypes.Volume{}, // no EFS volumes
			},
		}, nil
	})

	clients := &awsclient.ServiceClients{ECS: mock}
	fetcher := ecsTaskPaginatedFetcher(t)

	result, err := fetcher(context.Background(), clients, "")
	if err != nil {
		t.Fatalf("fetcher returned unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("want 1 resource, got %d", len(result.Resources))
	}

	task := result.Resources[0]

	if task.Fields["task_def_join_error"] != "" {
		t.Errorf("Fields[task_def_join_error] must be absent on success; got %q", task.Fields["task_def_join_error"])
	}
	if result.Pagination == nil {
		t.Fatal("Pagination must not be nil")
	}
	if result.Pagination.IsTruncated {
		t.Errorf("IsTruncated must be false when join succeeds; got true")
	}
}

// TestCheckEFSECSTask_JoinIncompleteTask_MarksApproximate verifies that the
// checkEFSECSTask checker (accessed via resource.GetRelated("efs")) returns
// Approximate=true when any task in the cache carries Fields["task_def_join_error"]="true".
//
// Setup:
//   - Source EFS resource ID: "fs-bar" (does not match any task's efs_file_system_ids).
//   - Cache "ecs-task" entry has two tasks:
//       task1: efs_file_system_ids="fs-foo" (no join error, does not match source).
//       task2: task_def_join_error="true"   (join incomplete, no efs ids).
//   - Expected result: Count==0 (no match), Approximate==true (join incomplete).
//
// This proves the zero is approximate (honest lower bound), not definitive.
func TestCheckEFSECSTask_JoinIncompleteTask_MarksApproximate(t *testing.T) {
	// Locate the efs→ecs-task checker via the registered related defs.
	var checker resource.RelatedChecker
	for _, def := range resource.GetRelated("efs") {
		if def.TargetType == "ecs-task" {
			checker = def.Checker
			break
		}
	}
	if checker == nil {
		t.Fatal("efs→ecs-task related checker not registered; cannot run Test C")
	}

	sourceEFS := resource.Resource{
		ID:   "fs-bar",
		Name: "fs-bar",
	}

	// task1: has efs ids, but for a different FS — does not match source "fs-bar".
	task1 := resource.Resource{
		ID:     "task-with-ids-001",
		Fields: map[string]string{"efs_file_system_ids": "fs-foo"},
	}
	// task2: join failed — efs ids unknown, marks result as approximate.
	task2 := resource.Resource{
		ID:     "task-join-fail-002",
		Fields: map[string]string{"task_def_join_error": "true"},
	}

	cache := resource.ResourceCache{
		"ecs-task": {
			Resources:   []resource.Resource{task1, task2},
			IsTruncated: false, // not page-truncated; the Approximate comes from joinIncomplete
		},
	}

	result := checker(context.Background(), nil, sourceEFS, cache)

	if result.Count != 0 {
		t.Errorf("Count: want 0 (no task matches fs-bar), got %d", result.Count)
	}
	if !result.Approximate {
		t.Errorf("Approximate: want true (task2 has join error → result is a lower bound, not definitive zero); got false. "+
			"This means the checker is not propagating joinIncomplete into result.Approximate.")
	}
}
