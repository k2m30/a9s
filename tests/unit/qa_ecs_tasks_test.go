package unit

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

func TestQA_ECSTasks_FetchSuccess(t *testing.T) {
	listClusters := &mockECSListClustersClient{
		output: &ecs.ListClustersOutput{
			ClusterArns: []string{
				"arn:aws:ecs:us-east-1:123456789012:cluster/my-cluster",
			},
		},
	}

	listTasks := &mockECSListTasksClient{
		outputs: map[string]*ecs.ListTasksOutput{
			"arn:aws:ecs:us-east-1:123456789012:cluster/my-cluster": {
				TaskArns: []string{
					"arn:aws:ecs:us-east-1:123456789012:task/my-cluster/abc123def456",
					"arn:aws:ecs:us-east-1:123456789012:task/my-cluster/xyz789uvw012",
				},
			},
		},
	}

	describeTasks := &mockECSDescribeTasksClient{
		output: &ecs.DescribeTasksOutput{
			Tasks: []ecstypes.Task{
				{
					TaskArn:           aws.String("arn:aws:ecs:us-east-1:123456789012:task/my-cluster/abc123def456"),
					ClusterArn:        aws.String("arn:aws:ecs:us-east-1:123456789012:cluster/my-cluster"),
					LastStatus:        aws.String("RUNNING"),
					TaskDefinitionArn: aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/my-app:5"),
					LaunchType:        ecstypes.LaunchTypeFargate,
					Cpu:               aws.String("256"),
					Memory:            aws.String("512"),
				},
				{
					TaskArn:           aws.String("arn:aws:ecs:us-east-1:123456789012:task/my-cluster/xyz789uvw012"),
					ClusterArn:        aws.String("arn:aws:ecs:us-east-1:123456789012:cluster/my-cluster"),
					LastStatus:        aws.String("STOPPED"),
					TaskDefinitionArn: aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/worker:3"),
					LaunchType:        ecstypes.LaunchTypeEc2,
					Cpu:               aws.String("512"),
					Memory:            aws.String("1024"),
				},
			},
		},
	}

	resources, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchECSTasksPage(context.Background(), listClusters, listTasks, describeTasks, token)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	r := resources[0]
	if r.ID != "abc123def456" {
		t.Errorf("expected ID 'abc123def456' (task UUID from ARN), got %q", r.ID)
	}
	if r.Name != "abc123def456" {
		t.Errorf("expected Name 'abc123def456', got %q", r.Name)
	}
	// Post-PR-03c: fetcher no longer writes Status for RUNNING tasks.
	// State lives in Fields["status"]; RUNNING tasks emit no Finding.
	if r.Fields["status"] != "RUNNING" {
		t.Errorf("Fields[status]: expected %q, got %q", "RUNNING", r.Fields["status"])
	}
	if len(r.Findings) != 0 {
		t.Errorf("Findings: got %d, want 0 for RUNNING task", len(r.Findings))
	}
	if r.Fields["task_id"] != "abc123def456" {
		t.Errorf("expected task_id 'abc123def456', got %q", r.Fields["task_id"])
	}
	if r.Fields["status"] != "RUNNING" {
		t.Errorf("expected status 'RUNNING', got %q", r.Fields["status"])
	}
	if r.Fields["launch_type"] != "FARGATE" {
		t.Errorf("expected launch_type 'FARGATE', got %q", r.Fields["launch_type"])
	}
	if r.Fields["cpu"] != "256" {
		t.Errorf("expected cpu '256', got %q", r.Fields["cpu"])
	}
	if r.Fields["memory"] != "512" {
		t.Errorf("expected memory '512', got %q", r.Fields["memory"])
	}

	r2 := resources[1]
	// Post-PR-03c: fetcher no longer writes Status for STOPPED tasks.
	//
	// RETIRED the old "STOPPED emits no Finding" invariant (stop_code carries
	// actionable info instead): since the color-findings-conformance wave,
	// colorECSTask is colorFromAnyFinding-first (internal/aws/catalog_compute.go)
	// and ecsTaskStructuralFindings (internal/aws/ecs_task_codes.go) now emits
	// CodeECSTaskStateStopped/SevDim for a normal (empty/UserInitiated
	// stop_code) STOPPED task — Color needs its own Finding to color from. See
	// qa_color_findings_conformance_test.go for the standing architectural gate.
	if r2.Fields["status"] != "STOPPED" {
		t.Errorf("r2 Fields[status]: expected %q, got %q", "STOPPED", r2.Fields["status"])
	}
	if len(r2.Findings) != 1 {
		t.Fatalf("r2 Findings: got %d, want 1 for STOPPED task (colorECSTask needs its own Finding to color from)", len(r2.Findings))
	}
	if r2.Findings[0].Code != "ecs-task.state.stopped" {
		t.Errorf("r2 Findings[0].Code: got %q, want %q", r2.Findings[0].Code, "ecs-task.state.stopped")
	}
	if r2.Findings[0].Severity != domain.SevDim {
		t.Errorf("r2 Findings[0].Severity: got %v, want SevDim", r2.Findings[0].Severity)
	}
	if r2.RawStruct == nil {
		t.Error("expected RawStruct to be set")
	}
}

func TestQA_ECSTasks_FetchNoClusters(t *testing.T) {
	listClusters := &mockECSListClustersClient{
		output: &ecs.ListClustersOutput{
			ClusterArns: []string{},
		},
	}
	listTasks := &mockECSListTasksClient{outputs: map[string]*ecs.ListTasksOutput{}}
	describeTasks := &mockECSDescribeTasksClient{}

	resources, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchECSTasksPage(context.Background(), listClusters, listTasks, describeTasks, token)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}

func TestQA_ECSTasks_FetchNoTasksInCluster(t *testing.T) {
	listClusters := &mockECSListClustersClient{
		output: &ecs.ListClustersOutput{
			ClusterArns: []string{"arn:aws:ecs:us-east-1:123456789012:cluster/empty"},
		},
	}
	listTasks := &mockECSListTasksClient{
		outputs: map[string]*ecs.ListTasksOutput{
			"arn:aws:ecs:us-east-1:123456789012:cluster/empty": {TaskArns: []string{}},
		},
	}
	describeTasks := &mockECSDescribeTasksClient{}

	resources, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchECSTasksPage(context.Background(), listClusters, listTasks, describeTasks, token)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}

func TestQA_ECSTasks_FetchListClustersError(t *testing.T) {
	listClusters := &mockECSListClustersClient{
		err: fmt.Errorf("access denied"),
	}
	listTasks := &mockECSListTasksClient{outputs: map[string]*ecs.ListTasksOutput{}}
	describeTasks := &mockECSDescribeTasksClient{}

	_, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchECSTasksPage(context.Background(), listClusters, listTasks, describeTasks, token)
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestQA_ECSTasks_TypeDef(t *testing.T) {
	rt := resource.FindResourceType("ecs-task")
	if rt == nil {
		t.Fatal("resource type 'ecs-task' not found")
	}
	if rt.Name != "ECS Tasks" {
		t.Errorf("expected Name 'ECS Tasks', got %q", rt.Name)
	}
	expected := []struct {
		key   string
		title string
	}{
		{"task_id", "Task ID"},
		{"cluster", "Cluster"},
		{"status", "Status"},
		{"task_definition", "Task Definition"},
		{"launch_type", "Launch"},
		{"cpu", "CPU"},
		{"memory", "Memory"},
	}
	if len(rt.Columns) != len(expected) {
		t.Fatalf("expected %d columns, got %d", len(expected), len(rt.Columns))
	}
	for i, want := range expected {
		if rt.Columns[i].Key != want.key {
			t.Errorf("column %d: expected key %q, got %q", i, want.key, rt.Columns[i].Key)
		}
		if rt.Columns[i].Title != want.title {
			t.Errorf("column %d: expected title %q, got %q", i, want.title, rt.Columns[i].Title)
		}
	}
}

func TestQA_ECSTasks_Aliases(t *testing.T) {
	for _, alias := range []string{"ecs-task", "ecs-tasks", "tasks"} {
		rt := resource.FindResourceType(alias)
		if rt == nil {
			t.Errorf("alias %q should resolve to ecs-task resource type", alias)
		}
	}
}
