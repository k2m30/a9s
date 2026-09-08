package unit

// aws5_ecs_task_status_test.go — the ECS task's lifecycle state is one field.
//
// The fetcher wrote one variable under "status" and "last_status", and the
// readers had already drifted apart: the column and LifecycleKey read "status"
// while the colour classifier read "last_status". The project settled which
// name survives when the backup type went through this same collapse
// (tests/unit/aws_backup_test.go:273, "column keyed 'last_status' is banned
// per spec §4 — replace with 'status'").

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// TestECSTask_StateIsOneField pins the collapse at the fetcher and follows it
// through every reader the catalog registers.
func TestECSTask_StateIsOneField(t *testing.T) {
	listClusters := &mockECSListClustersClient{
		output: &ecs.ListClustersOutput{
			ClusterArns: []string{"arn:aws:ecs:us-east-1:123456789012:cluster/acme-prod"},
		},
	}
	listTasks := &mockECSListTasksClient{
		outputs: map[string]*ecs.ListTasksOutput{
			"arn:aws:ecs:us-east-1:123456789012:cluster/acme-prod": {
				TaskArns: []string{"arn:aws:ecs:us-east-1:123456789012:task/acme-prod/abc123def456"},
			},
		},
	}
	describeTasks := &mockECSDescribeTasksClient{
		output: &ecs.DescribeTasksOutput{
			Tasks: []ecstypes.Task{{
				TaskArn:           aws.String("arn:aws:ecs:us-east-1:123456789012:task/acme-prod/abc123def456"),
				ClusterArn:        aws.String("arn:aws:ecs:us-east-1:123456789012:cluster/acme-prod"),
				LastStatus:        aws.String("RUNNING"),
				TaskDefinitionArn: aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/acme-api:4"),
			}},
		},
	}

	res, err := qaECSTaskFetch(context.Background(), listClusters, listTasks, describeTasks, "")
	if err != nil {
		t.Fatalf("ecs-task fetch: %v", err)
	}
	if len(res.Resources) != 1 {
		t.Fatalf("got %d resources, want 1", len(res.Resources))
	}
	aws5NoSecondKey(t, res.Resources[0], "last_status", "status", "RUNNING")

	td := aws5TypeDef(t, "ecs-task")
	for _, k := range td.FieldKeys {
		if k == "last_status" {
			t.Errorf("ecs-task FieldKeys still names the dropped key %q", k)
		}
	}
	if td.LifecycleKey != "status" {
		t.Errorf("ecs-task LifecycleKey = %q, want %q — the key the surviving field carries", td.LifecycleKey, "status")
	}
}

// TestECSTask_ColourReadsTheSurvivingKey pins the reader that had drifted. It
// coloured from "last_status", so a row rebuilt from the cache under the key
// the column persists would have coloured healthy whatever its state.
func TestECSTask_ColourReadsTheSurvivingKey(t *testing.T) {
	td := aws5TypeDef(t, "ecs-task")
	if td.Color == nil {
		t.Fatal("ecs-task registers no Color function")
	}
	stopped := domain.Resource{
		ID:   "abc123def456",
		Name: "abc123def456",
		Fields: map[string]string{
			"status":        "STOPPED",
			"stop_code":     "TaskFailedToStart",
			"health_status": "UNKNOWN",
		},
	}
	healthy := domain.Resource{
		ID:     "xyz789uvw012",
		Name:   "xyz789uvw012",
		Fields: map[string]string{"status": "RUNNING"},
	}
	if got := td.Color(stopped); got == td.Color(healthy) {
		t.Errorf("Color(status=STOPPED, stop_code=TaskFailedToStart) = %v, the same as a running task: "+
			"the classifier is reading a key the fetcher no longer writes", got)
	}
}

var _ = resource.GetPaginatedFetcher
