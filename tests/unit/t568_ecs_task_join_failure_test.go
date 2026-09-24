package unit_test

// A task whose task definition the fetcher could not read keeps its row: the
// ecs-task list stays whole, and the refusal is the page's per-item failure,
// reported once at list load. A log group's pivot over that list is a lower
// bound and carries no failure of its own, so the refusal is not repeated on
// every log group opened.

import (
	"context"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

const (
	t568JoinCluster = "arn:aws:ecs:us-east-1:123456789012:cluster/prod"
	t568JoinAPITD   = "arn:aws:ecs:us-east-1:123456789012:task-definition/api:42"
	t568JoinDenyTD  = "arn:aws:ecs:us-east-1:123456789012:task-definition/billing:3"
)

// t568JoinECS lists one cluster with two tasks and refuses DescribeTaskDefinition
// for one of their definitions.
type t568JoinECS struct {
	awsclient.ECSAPI
	defCalls int
}

func (f *t568JoinECS) ListClusters(context.Context, *ecs.ListClustersInput, ...func(*ecs.Options)) (*ecs.ListClustersOutput, error) {
	return &ecs.ListClustersOutput{ClusterArns: []string{t568JoinCluster}}, nil
}

func (f *t568JoinECS) ListTasks(context.Context, *ecs.ListTasksInput, ...func(*ecs.Options)) (*ecs.ListTasksOutput, error) {
	return &ecs.ListTasksOutput{TaskArns: []string{
		"arn:aws:ecs:us-east-1:123456789012:task/prod/0f1e2d3c4b5a49788a9b0c1d2e3f4a5b",
		"arn:aws:ecs:us-east-1:123456789012:task/prod/1a2b3c4d5e6f4a7b8c9d0e1f2a3b4c5d",
	}}, nil
}

func (f *t568JoinECS) DescribeTasks(_ context.Context, in *ecs.DescribeTasksInput, _ ...func(*ecs.Options)) (*ecs.DescribeTasksOutput, error) {
	out := &ecs.DescribeTasksOutput{}
	for i, arn := range in.Tasks {
		td := t568JoinAPITD
		if i == 1 {
			td = t568JoinDenyTD
		}
		out.Tasks = append(out.Tasks, ecstypes.Task{TaskArn: aws.String(arn), ClusterArn: aws.String(t568JoinCluster),
			TaskDefinitionArn: aws.String(td), LastStatus: aws.String("RUNNING"), DesiredStatus: aws.String("RUNNING")})
	}
	return out, nil
}

func (f *t568JoinECS) DescribeTaskDefinition(_ context.Context, in *ecs.DescribeTaskDefinitionInput, _ ...func(*ecs.Options)) (*ecs.DescribeTaskDefinitionOutput, error) {
	f.defCalls++
	if aws.ToString(in.TaskDefinition) == t568JoinDenyTD {
		return nil, &smithy.GenericAPIError{Code: "AccessDeniedException",
			Message: "User: arn:aws:sts::123456789012:assumed-role/example-readonly/a9s is not authorized to perform: ecs:DescribeTaskDefinition on resource: " + t568JoinDenyTD}
	}
	return &ecs.DescribeTaskDefinitionOutput{TaskDefinition: &ecstypes.TaskDefinition{Family: aws.String("api"), Revision: 42,
		ContainerDefinitions: []ecstypes.ContainerDefinition{{Name: aws.String("api"), LogConfiguration: &ecstypes.LogConfiguration{
			LogDriver: ecstypes.LogDriverAwslogs, Options: map[string]string{"awslogs-group": "/aws/ecs/acme-api", "awslogs-region": "us-east-1"}}}}}}, nil
}

func TestT568_TaskDefinitionJoinFailureIsThePagesItemFailure(t *testing.T) {
	fake := &t568JoinECS{}
	clients := &awsclient.ServiceClients{ECS: fake, Region: "us-east-1"}
	result, err := resource.GetPaginatedFetcher("ecs-task")(context.Background(), clients, "")

	if len(result.Resources) != 2 {
		t.Fatalf("rows = %d, want both tasks: a task whose definition was refused is still a task", len(result.Resources))
	}
	if err == nil {
		t.Fatal("the refused DescribeTaskDefinition is not reported with the page")
	}
	if msg := err.Error(); !strings.Contains(msg, "1 of 2") || strings.Count(msg, "DescribeTaskDefinition failed") != 1 {
		t.Errorf("page error = %q, want one aggregate naming 1 of 2 tasks", msg)
	}
	if !awsclient.IsAccessDenied(err) {
		t.Errorf("page error = %v, want it classed access-denied", err)
	}
	if awsclient.FetchIsPartial(result, err) {
		t.Error("FetchIsPartial = true: a per-item join failure on a row the page holds makes the list a subset")
	}
	if p := awsclient.StoredPagination(result, err); p != nil && (p.IsTruncated || p.LowerBoundOnly) {
		t.Errorf("stored pagination %+v, want the list stored whole", *p)
	}
	if fake.defCalls != 2 {
		t.Errorf("DescribeTaskDefinition calls = %d, want one per definition", fake.defCalls)
	}

	cache := resource.ResourceCache{"ecs-task": {Resources: result.Resources}}
	for _, group := range []string{"/aws/ecs/acme-api", "/aws/ecs/other"} {
		lg := resource.Resource{ID: group, Name: group, Type: "logs", Fields: map[string]string{"log_group_name": group}}
		got := refChecker(t, "logs", "ecs-task")(context.Background(), clients, lg, cache)
		if got.Failure() != nil || got.Err() != nil {
			t.Errorf("logs %s → ecs-task repeats the list's failure: failure %v, err %v", group, got.Failure(), got.Err())
		}
		if got.EffectiveState() != domain.RelatedResolved || got.Coverage() == domain.CoverageComplete {
			t.Errorf("logs %s → ecs-task = %v (state %v, coverage %v), want a lower bound: one task's definition was not read", group, got.ResourceIDs(), got.EffectiveState(), got.Coverage())
		}
	}
	if fake.defCalls != 2 {
		t.Errorf("DescribeTaskDefinition calls after opening two log groups = %d, want 2: the pivot reads the list's join", fake.defCalls)
	}
}
