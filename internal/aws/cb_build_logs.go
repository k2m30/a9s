package aws

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("cb_build_logs", []string{"timestamp", "message", "ingestion_time", "event_id"})

	resource.RegisterPaginatedChild("cb_build_logs", func(ctx context.Context, clients any, parentCtx resource.ParentContext, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchCBBuildLogs(ctx, c.CloudWatchLogs, parentCtx["log_group_name"], parentCtx["log_stream_name"], continuationToken)
	})

	resource.RegisterChildType(resource.ResourceTypeDef{
		Name:      "Build Logs",
		ShortName: "cb_build_logs",
		Columns:   resource.CBBuildLogColumns(),
		CopyField: "message",
	})
}

// FetchCBBuildLogs calls the CloudWatch Logs GetLogEvents API for a given
// log group and stream (from a CodeBuild build), converting the response
// into a FetchResult. This is a single-call API, but uses FetchResult for consistency.
func FetchCBBuildLogs(
	ctx context.Context,
	api CWLogsGetLogEventsAPI,
	logGroupName, logStreamName string,
	continuationToken string,
) (resource.FetchResult, error) {
	input := &cloudwatchlogs.GetLogEventsInput{
		LogGroupName:  &logGroupName,
		LogStreamName: &logStreamName,
		StartFromHead: aws.Bool(false),
	}

	output, err := api.GetLogEvents(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching build log events: %w", err)
	}

	var resources []resource.Resource

	for i, event := range output.Events {
		message := ""
		if event.Message != nil {
			message = strings.ReplaceAll(strings.TrimRight(*event.Message, "\n"), "\n", " ")
		}

		ts := ""
		tsVal := int64(0)
		if event.Timestamp != nil {
			tsVal = *event.Timestamp
			ts = formatEpochMillisSec(tsVal)
		}

		ingestionTime := ""
		if event.IngestionTime != nil {
			ingestionTime = formatEpochMillisSec(*event.IngestionTime)
		}

		id := fmt.Sprintf("evt-%d-%d", tsVal, i)

		name := message
		if len(name) > 80 {
			name = name[:80]
		}

		status := classifyBuildLogStatus(message)

		r := resource.Resource{
			ID:     id,
			Name:   name,
			Status: status,
			Fields: map[string]string{
				"timestamp":      ts,
				"message":        message,
				"ingestion_time": ingestionTime,
				"event_id":       id,
			},
			RawStruct: event,
		}

		resources = append(resources, r)
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

// formatEpochMillisSec converts epoch milliseconds to a human-readable
// timestamp string including seconds.
func formatEpochMillisSec(ms int64) string {
	return time.UnixMilli(ms).UTC().Format("2006-01-02 15:04")
}

// classifyBuildLogStatus classifies a build log message into a status category.
func classifyBuildLogStatus(message string) string {
	switch {
	case strings.Contains(message, "FAIL") ||
		strings.Contains(message, "ERROR") ||
		strings.Contains(message, "error") ||
		strings.Contains(message, "Error") ||
		strings.Contains(message, "did not exit successfully"):
		return "ERROR"
	case strings.Contains(message, "Phase complete") ||
		strings.Contains(message, "SUCCEEDED"):
		return "SUCCEEDED"
	case strings.Contains(message, "Entering phase") ||
		strings.Contains(message, "Running command"):
		return "IN_PROGRESS"
	default:
		return ""
	}
}
