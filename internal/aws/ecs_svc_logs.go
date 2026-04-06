package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("ecs_svc_logs", []string{"timestamp", "stream_short", "message"})

	resource.RegisterPaginatedChild("ecs_svc_logs", func(ctx context.Context, clients any, parentCtx resource.ParentContext, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchEcsSvcLogs(ctx, c.ECS, c.CloudWatchLogs, parentCtx["cluster"], parentCtx["service_name"], parentCtx["task_definition"], continuationToken)
	})

	resource.RegisterChildType(resource.ResourceTypeDef{
		Name:      "Service Logs",
		ShortName: "ecs_svc_logs",
		Columns:   resource.EcsSvcLogColumns(),
	})
}

// maxLogEvents caps the total number of log events fetched per service.
const maxLogEvents = 200

// FetchEcsSvcLogs is a cross-service child fetcher. It first calls
// DescribeTaskDefinition to extract the awslogs-group and awslogs-stream-prefix
// from the task definition's first container, then calls FilterLogEvents to
// retrieve recent log lines. Returns a FetchResult with pagination support.
// Each call returns up to maxLogEvents (200) items.
func FetchEcsSvcLogs(
	ctx context.Context,
	taskDefAPI ECSDescribeTaskDefinitionAPI,
	cwLogsAPI CWLogsFilterLogEventsAPI,
	cluster, serviceName, taskDefinition string,
	continuationToken string,
) (resource.FetchResult, error) {
	// Step 1: DescribeTaskDefinition to get log configuration
	tdOutput, err := taskDefAPI.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: &taskDefinition,
	})
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("describing task definition for %s: %w", serviceName, err)
	}

	if tdOutput.TaskDefinition == nil || len(tdOutput.TaskDefinition.ContainerDefinitions) == 0 {
		return resource.FetchResult{}, fmt.Errorf("no containers in task definition for %s", serviceName)
	}

	// Find the first container with awslogs driver
	var logGroup string
	var found bool

	for _, container := range tdOutput.TaskDefinition.ContainerDefinitions {
		if container.LogConfiguration == nil {
			continue
		}
		if container.LogConfiguration.LogDriver != ecstypes.LogDriverAwslogs {
			continue
		}
		logGroup = container.LogConfiguration.Options["awslogs-group"]
		found = true
		break
	}

	if !found {
		return resource.FetchResult{}, fmt.Errorf("no container with awslogs log driver in task definition for %s", serviceName)
	}

	// Step 2: FilterLogEvents on the extracted log group
	var resources []resource.Resource
	var nextToken *string
	if continuationToken != "" {
		nextToken = &continuationToken
	}

	for {
		input := &cloudwatchlogs.FilterLogEventsInput{
			LogGroupName: &logGroup,
			NextToken:    nextToken,
		}

		output, err := cwLogsAPI.FilterLogEvents(ctx, input)
		if err != nil {
			return resource.FetchResult{}, fmt.Errorf("fetching log events for %s: %w", serviceName, err)
		}

		for _, event := range output.Events {
			id := ""
			if event.EventId != nil {
				id = *event.EventId
			}

			timestamp := ""
			if event.Timestamp != nil {
				timestamp = formatEpochMillis(*event.Timestamp)
			}

			streamShort := ""
			if event.LogStreamName != nil {
				streamShort = computeStreamShort(*event.LogStreamName)
			}

			message := ""
			if event.Message != nil {
				message = strings.ReplaceAll(*event.Message, "\n", " ")
			}

			name := message
			if len(name) > 80 {
				name = name[:80]
			}

			r := resource.Resource{
				ID:   id,
				Name: name,
				Fields: map[string]string{
					"timestamp":    timestamp,
					"stream_short": streamShort,
					"message":      message,
				},
				RawStruct: event,
			}

			resources = append(resources, r)
		}

		if len(resources) >= maxLogEvents {
			apiNextToken := ""
			if output.NextToken != nil {
				apiNextToken = *output.NextToken
			}
			return resource.FetchResult{
				Resources: resources,
				Pagination: &resource.PaginationMeta{
					IsTruncated: apiNextToken != "",
					NextToken:   apiNextToken,
					PageSize:    len(resources),
				},
			}, nil
		}

		if output.NextToken == nil {
			break
		}
		nextToken = output.NextToken
	}

	return resource.FetchResult{
		Resources: resources,
		Pagination: &resource.PaginationMeta{
			IsTruncated: false,
			TotalHint:   len(resources),
			PageSize:    len(resources),
		},
	}, nil
}

// computeStreamShort extracts "container/short-task-id" from a log stream name
// like "ecs/web/abc123def456789". The result is "web/abc123de" (container name
// plus first 8 chars of the task ID).
func computeStreamShort(streamName string) string {
	parts := strings.Split(streamName, "/")
	if len(parts) < 3 {
		return streamName
	}

	// parts[0] = prefix (e.g., "ecs")
	// parts[1] = container name
	// parts[2] = task ID
	container := parts[1]
	taskID := parts[2]
	if len(taskID) > 8 {
		taskID = taskID[:8]
	}

	return container + "/" + taskID
}
