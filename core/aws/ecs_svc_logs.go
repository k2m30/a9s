// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"

	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// maxLogEvents caps the number of log events shown per service.
const maxLogEvents = 200

// regionalLogsAPI is a CloudWatch Logs client that can hand back its peer in
// another Region, as the session's clients do.
type regionalLogsAPI interface {
	CWLogsFilterLogEventsAPI
	logsIn(region string) CWLogsFilterLogEventsAPI
}

// sessionLogs reads CloudWatch Logs through the session's clients, in any
// Region through the session's per-Region clients.
type sessionLogs struct{ c *ServiceClients }

func (s sessionLogs) FilterLogEvents(ctx context.Context, in *cloudwatchlogs.FilterLogEventsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.FilterLogEventsOutput, error) {
	if s.c.CloudWatchLogs == nil {
		return nil, errClientMissing
	}
	return s.c.CloudWatchLogs.FilterLogEvents(ctx, in, optFns...)
}

func (s sessionLogs) logsIn(region string) CWLogsFilterLogEventsAPI {
	return s.c.InRegion(region).CloudWatchLogs
}

// FetchEcsSvcLogs is a cross-service child fetcher. It calls
// DescribeTaskDefinition, then reads the newest maxLogEvents (200) lines of
// every awslogs container, newest first: each container's awslogs-group,
// narrowed to its own streams by awslogs-stream-prefix, in its
// awslogs-region when cwLogsAPI can reach other Regions. Load More continues
// with the next older ones.
func FetchEcsSvcLogs(
	ctx context.Context,
	taskDefAPI ECSDescribeTaskDefinitionAPI,
	cwLogsAPI CWLogsFilterLogEventsAPI,
	cluster, serviceName, taskDefinition string,
	continuationToken string,
) (resource.FetchResult, error) {
	tdOutput, err := taskDefAPI.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: &taskDefinition,
	})
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("describing task definition for %s: %w", serviceName, err)
	}

	if tdOutput.TaskDefinition == nil || len(tdOutput.TaskDefinition.ContainerDefinitions) == 0 {
		return resource.FetchResult{}, fmt.Errorf("no containers in task definition for %s", serviceName)
	}

	type sourceKey struct{ region, group, streamPrefix string }
	var sources []logSource
	seen := map[sourceKey]bool{}
	for _, container := range tdOutput.TaskDefinition.ContainerDefinitions {
		if container.LogConfiguration == nil || container.LogConfiguration.LogDriver != ecstypes.LogDriverAwslogs {
			continue
		}
		opts := container.LogConfiguration.Options
		// awslogs names each stream <awslogs-stream-prefix>/<container>/<task-id>;
		// without a prefix the stream names carry nothing to select by.
		key := sourceKey{region: opts["awslogs-region"], group: opts["awslogs-group"]}
		if p := opts["awslogs-stream-prefix"]; p != "" {
			key.streamPrefix = p + "/" + aws.ToString(container.Name) + "/"
		}
		if key.group == "" || seen[key] {
			continue
		}
		seen[key] = true
		api := cwLogsAPI
		if r, ok := cwLogsAPI.(regionalLogsAPI); ok {
			api = r.logsIn(key.region)
		}
		sources = append(sources, logSource{api: api, region: key.region, group: key.group, streamPrefix: key.streamPrefix})
	}
	if len(sources) == 0 {
		return resource.FetchResult{}, fmt.Errorf("no container with awslogs log driver in task definition for %s", serviceName)
	}

	cur, err := parseLogCursor(continuationToken, 0)
	if err != nil {
		return resource.FetchResult{}, err
	}
	events, next, err := newestLogEvents(ctx, sources, cur, maxLogEvents)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching log events for %s: %w", serviceName, err)
	}

	resources := make([]resource.Resource, 0, len(events))
	for _, event := range events {
		id := ""
		if event.EventId != nil {
			id = *event.EventId
		}

		timestamp := ""
		if event.Timestamp != nil {
			timestamp = formatEpochMillis(*event.Timestamp)
		}

		ingestionTime := ""
		if event.IngestionTime != nil {
			ingestionTime = formatEpochMillis(*event.IngestionTime)
		}

		streamShort := ""
		logStream := ""
		if event.LogStreamName != nil {
			logStream = *event.LogStreamName
			streamShort = computeStreamShort(logStream)
		}

		message := ""
		if event.Message != nil {
			message = strings.ReplaceAll(*event.Message, "\n", " ")
		}

		name := message
		if len(name) > 80 {
			name = name[:80]
		}

		// Status classification using the shared classifier from
		// log_events.go — ecs_svc_logs pulls the same shape of raw
		// CloudWatch log lines as log_events/lambda_invocation_logs.
		status := classifyLogEventStatus(message)

		r := resource.Resource{
			ID:       id,
			Name:     name,
			Findings: logEventFindings(status),
			Fields: map[string]string{
				"timestamp":      timestamp,
				"ingestion_time": ingestionTime,
				"stream_short":   streamShort,
				"message":        message,
				"status":         status,
				"log_group":      event.group,
				"log_region":     event.region,
				"log_stream":     logStream,
			},
			RawStruct: event.FilteredLogEvent,
		}

		resources = append(resources, r)
	}

	return resource.FetchResult{Resources: resources, Pagination: logReadPagination(len(resources), next)}, nil
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
