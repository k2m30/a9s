package unit

// A failed DescribeTaskDefinition join marks the task with
// Fields["task_def_join_error"]="true" and leaves the page untruncated;
// checkEFSECSTask turns such a task into Truncated=true, an honest lower bound
// instead of a confident zero.

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	smithy "github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

type fullECSAPI struct {
	listClustersFn    func(*ecs.ListClustersInput) (*ecs.ListClustersOutput, error)
	listTasksFn       func(*ecs.ListTasksInput) (*ecs.ListTasksOutput, error)
	describeTasksFn   func(*ecs.DescribeTasksInput) (*ecs.DescribeTasksOutput, error)
	describeTaskDefFn func(*ecs.DescribeTaskDefinitionInput) (*ecs.DescribeTaskDefinitionOutput, error)
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

func (f *fullECSAPI) DescribeClusters(_ context.Context, _ *ecs.DescribeClustersInput, _ ...func(*ecs.Options)) (*ecs.DescribeClustersOutput, error) {
	return &ecs.DescribeClustersOutput{}, nil
}

func (f *fullECSAPI) ListServices(_ context.Context, _ *ecs.ListServicesInput, _ ...func(*ecs.Options)) (*ecs.ListServicesOutput, error) {
	return &ecs.ListServicesOutput{}, nil
}

func (f *fullECSAPI) DescribeServices(_ context.Context, _ *ecs.DescribeServicesInput, _ ...func(*ecs.Options)) (*ecs.DescribeServicesOutput, error) {
	return &ecs.DescribeServicesOutput{}, nil
}

var _ awsclient.ECSAPI = (*fullECSAPI)(nil)

// The registered paginated fetcher is the path that joins
// DescribeTaskDefinition.
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

	if result.Pagination == nil {
		t.Fatal("Pagination must not be nil")
	}
	if result.Pagination.IsTruncated {
		t.Errorf("IsTruncated must be false when DescribeTaskDefinition fails — " +
			"the fetcher must set Fields[task_def_join_error] instead of marking pagination truncated. " +
			"Got IsTruncated=true (OLD BUG: would surface misleading 'm: load more' in TUI).")
	}
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken must be empty on join failure (no pagination in play); got %q", result.Pagination.NextToken)
	}

	if task.Fields["task_def_join_error"] != "true" {
		t.Errorf("Fields[task_def_join_error]: want %q, got %q", "true", task.Fields["task_def_join_error"])
	}
}

func TestFetchECSTasksPage_JoinSucceeds_NoErrorField(t *testing.T) {
	mock := buildECSTaskMockAPI(func(_ *ecs.DescribeTaskDefinitionInput) (*ecs.DescribeTaskDefinitionOutput, error) {
		return &ecs.DescribeTaskDefinitionOutput{
			TaskDefinition: &ecstypes.TaskDefinition{
				Volumes: []ecstypes.Volume{},
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

func TestCheckEFSECSTask_JoinIncompleteTask_MarksTruncated(t *testing.T) {
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

	task1 := resource.Resource{
		ID:     "task-with-ids-001",
		Fields: map[string]string{"efs_file_system_ids": "fs-foo"},
	}
	task2 := resource.Resource{
		ID:     "task-join-fail-002",
		Fields: map[string]string{"task_def_join_error": "true"},
	}

	cache := resource.ResourceCache{
		"ecs-task": {
			Resources:   []resource.Resource{task1, task2},
			IsTruncated: false, // not page-truncated; the Truncated comes from joinIncomplete
		},
	}

	result := checker(context.Background(), nil, sourceEFS, cache)

	if result.Count() != 0 {
		t.Errorf("Count: want 0 (no task matches fs-bar), got %d", result.Count())
	}
	if !result.Truncated() {
		t.Errorf("Truncated: want true (task2 has join error → result is a lower bound, not definitive zero); got false. " +
			"This means the checker is not propagating joinIncomplete into result.Truncated.")
	}
}
