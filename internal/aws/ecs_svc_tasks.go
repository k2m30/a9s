package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("ecs_tasks", []string{
		"task_id_short", "status", "health", "task_def_short",
		"started_at", "stopped_reason",
	})

	resource.RegisterChildFetcher("ecs_tasks", func(ctx context.Context, clients interface{}, parentCtx resource.ParentContext) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchEcsSvcTasks(ctx, c.ECS, c.ECS, parentCtx["cluster"], parentCtx["service_name"])
	})

	resource.RegisterChildType(resource.ResourceTypeDef{
		Name:      "Service Tasks",
		ShortName: "ecs_tasks",
		Columns:   resource.EcsSvcTaskColumns(),
	})
}

// maxEcsTasks caps the total number of tasks fetched per service.
const maxEcsTasks = 200

// FetchEcsSvcTasks calls ListTasks for both RUNNING and STOPPED statuses,
// then DescribeTasks for full details. Returns at most maxEcsTasks results.
func FetchEcsSvcTasks(
	ctx context.Context,
	listAPI ECSListTasksAPI,
	describeAPI ECSDescribeTasksAPI,
	cluster, serviceName string,
) ([]resource.Resource, error) {
	var allTaskArns []string

	// Collect task ARNs for both RUNNING and STOPPED statuses
	for _, status := range []ecstypes.DesiredStatus{ecstypes.DesiredStatusRunning, ecstypes.DesiredStatusStopped} {
		var nextToken *string
		for {
			input := &ecs.ListTasksInput{
				Cluster:       aws.String(cluster),
				ServiceName:   aws.String(serviceName),
				DesiredStatus: status,
				NextToken:     nextToken,
			}

			output, err := listAPI.ListTasks(ctx, input)
			if err != nil {
				return nil, fmt.Errorf("listing ECS tasks for %s: %w", serviceName, err)
			}

			allTaskArns = append(allTaskArns, output.TaskArns...)

			if output.NextToken == nil || len(allTaskArns) >= maxEcsTasks {
				break
			}
			nextToken = output.NextToken
		}

		if len(allTaskArns) >= maxEcsTasks {
			allTaskArns = allTaskArns[:maxEcsTasks]
			break
		}
	}

	if len(allTaskArns) == 0 {
		return []resource.Resource{}, nil
	}

	// DescribeTasks API accepts max 100 ARNs per call — batch if needed.
	const descBatchSize = 100
	var allTasks []ecstypes.Task
	for i := 0; i < len(allTaskArns); i += descBatchSize {
		end := i + descBatchSize
		if end > len(allTaskArns) {
			end = len(allTaskArns)
		}
		descOutput, err := describeAPI.DescribeTasks(ctx, &ecs.DescribeTasksInput{
			Cluster: aws.String(cluster),
			Tasks:   allTaskArns[i:end],
		})
		if err != nil {
			return nil, fmt.Errorf("describing ECS tasks for %s: %w", serviceName, err)
		}
		allTasks = append(allTasks, descOutput.Tasks...)
	}

	var resources []resource.Resource

	for _, task := range allTasks {
		taskIDShort := ""
		taskArn := ""
		if task.TaskArn != nil {
			taskArn = *task.TaskArn
			parts := strings.Split(taskArn, "/")
			taskIDShort = parts[len(parts)-1]
		}

		status := ""
		if task.LastStatus != nil {
			status = *task.LastStatus
		}

		health := ""
		if task.HealthStatus != "" {
			health = strings.ToUpper(string(task.HealthStatus))
		}

		taskDefShort := ""
		if task.TaskDefinitionArn != nil {
			// Extract "family:revision" from ARN like
			// "arn:aws:ecs:us-east-1:123456789012:task-definition/web-app:5"
			tdArn := *task.TaskDefinitionArn
			parts := strings.Split(tdArn, "/")
			if len(parts) > 0 {
				taskDefShort = parts[len(parts)-1]
			}
		}

		startedAt := ""
		if task.StartedAt != nil {
			startedAt = task.StartedAt.UTC().Format("2006-01-02 15:04:05")
		}

		stoppedReason := ""
		if task.StoppedReason != nil {
			stoppedReason = strings.ReplaceAll(*task.StoppedReason, "\n", " ")
		}

		r := resource.Resource{
			ID:     taskIDShort,
			Name:   taskIDShort,
			Status: status,
			Fields: map[string]string{
				"task_id_short":  taskIDShort,
				"status":         status,
				"health":         health,
				"task_def_short": taskDefShort,
				"started_at":     startedAt,
				"stopped_reason": stoppedReason,
			},
			RawStruct: task,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
