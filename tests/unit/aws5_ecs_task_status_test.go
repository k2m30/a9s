package unit

// The ECS task's lifecycle state is one field, "status": the column,
// and LifecycleKey both read it.

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// The fetcher writes the state once, and every reader the catalog registers
// reads that key.
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

var _ = resource.GetPaginatedFetcher
