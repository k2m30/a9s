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

	resource.RegisterPaginatedChild("ecs_tasks", func(ctx context.Context, clients interface{}, parentCtx resource.ParentContext, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchEcsSvcTasks(ctx, c.ECS, c.ECS, parentCtx["cluster"], parentCtx["service_name"], continuationToken)
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
// then DescribeTasks for full details. Returns at most maxEcsTasks results
// as a FetchResult with pagination support. When continuationToken is provided,
// it resumes ListTasks from that token. When the cap is reached and more pages
// exist, FetchResult.Pagination.IsTruncated is set to true with a NextToken
// for continuation.
func FetchEcsSvcTasks(
	ctx context.Context,
	listAPI ECSListTasksAPI,
	describeAPI ECSDescribeTasksAPI,
	cluster, serviceName string,
	continuationToken string,
) (resource.FetchResult, error) {
	var allTaskArns []string
	var nextToken *string
	if continuationToken != "" {
		nextToken = &continuationToken
	}

	var lastAPINextToken string

	// Collect task ARNs for both RUNNING and STOPPED statuses
	for _, status := range []ecstypes.DesiredStatus{ecstypes.DesiredStatusRunning, ecstypes.DesiredStatusStopped} {
		for {
			input := &ecs.ListTasksInput{
				Cluster:       aws.String(cluster),
				ServiceName:   aws.String(serviceName),
				DesiredStatus: status,
				NextToken:     nextToken,
			}

			output, err := listAPI.ListTasks(ctx, input)
			if err != nil {
				return resource.FetchResult{}, fmt.Errorf("listing ECS tasks for %s: %w", serviceName, err)
			}

			allTaskArns = append(allTaskArns, output.TaskArns...)

			if output.NextToken != nil {
				lastAPINextToken = *output.NextToken
			} else {
				lastAPINextToken = ""
			}

			if output.NextToken == nil || len(allTaskArns) >= maxEcsTasks {
				break
			}
			nextToken = output.NextToken
		}

		if len(allTaskArns) >= maxEcsTasks {
			allTaskArns = allTaskArns[:maxEcsTasks]
			break
		}
		// Reset nextToken between status phases
		nextToken = nil
	}

	if len(allTaskArns) == 0 {
		return resource.FetchResult{
			Resources: []resource.Resource{},
			Pagination: &resource.PaginationMeta{
				IsTruncated: false,
				TotalHint:   0,
				PageSize:    0,
			},
		}, nil
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
			return resource.FetchResult{}, fmt.Errorf("describing ECS tasks for %s: %w", serviceName, err)
		}
		allTasks = append(allTasks, descOutput.Tasks...)
	}

	var resources []resource.Resource
	for _, task := range allTasks {
		resources = append(resources, convertEcsTask(task))
	}

	return resource.FetchResult{
		Resources: resources,
		Pagination: &resource.PaginationMeta{
			IsTruncated: lastAPINextToken != "",
			NextToken:   lastAPINextToken,
			PageSize:    len(resources),
			TotalHint:   len(resources),
		},
	}, nil
}

// convertEcsTask converts a single ECS Task into a generic Resource.
func convertEcsTask(task ecstypes.Task) resource.Resource {
	taskIDShort := ""
	if task.TaskArn != nil {
		parts := strings.Split(*task.TaskArn, "/")
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
		parts := strings.Split(*task.TaskDefinitionArn, "/")
		if len(parts) > 0 {
			taskDefShort = parts[len(parts)-1]
		}
	}

	startedAt := ""
	if task.StartedAt != nil {
		startedAt = task.StartedAt.UTC().Format("2006-01-02 15:04")
	}

	stoppedReason := ""
	if task.StoppedReason != nil {
		stoppedReason = strings.ReplaceAll(*task.StoppedReason, "\n", " ")
	}

	return resource.Resource{
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
}
