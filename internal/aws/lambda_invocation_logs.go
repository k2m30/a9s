package aws

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("lambda_invocation_logs", []string{"timestamp", "message"})

	resource.RegisterChildFetcher("lambda_invocation_logs", func(ctx context.Context, clients interface{}, parentCtx resource.ParentContext) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchLambdaInvocationLogs(ctx, c.CloudWatchLogs, parentCtx["log_group"], parentCtx["request_id"])
	})

	resource.RegisterChildType(resource.ResourceTypeDef{
		Name:      "Lambda Invocation Logs",
		ShortName: "lambda_invocation_logs",
		Columns:   resource.LambdaInvocationLogColumns(),
	})
}

// FetchLambdaInvocationLogs calls the CloudWatchLogs FilterLogEvents API with
// a filter pattern containing the request ID, returning individual log lines
// for a specific Lambda invocation.
func FetchLambdaInvocationLogs(ctx context.Context, api CWLogsFilterLogEventsAPI, logGroup, requestID string) ([]resource.Resource, error) {
	filterPattern := fmt.Sprintf("%q", requestID)
	startTime := time.Now().Add(-24 * time.Hour).UnixMilli()

	input := &cloudwatchlogs.FilterLogEventsInput{
		LogGroupName:  &logGroup,
		FilterPattern: &filterPattern,
		StartTime:     &startTime,
	}

	output, err := api.FilterLogEvents(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("fetching lambda invocation logs: %w", err)
	}

	var resources []resource.Resource

	for _, event := range output.Events {
		message := ""
		if event.Message != nil {
			message = strings.TrimRight(*event.Message, "\n\r")
		}

		ts := ""
		if event.Timestamp != nil {
			ts = formatEpochMillis(*event.Timestamp)
		}

		// ID: use EventId if available, otherwise generate
		id := ""
		if event.EventId != nil {
			id = *event.EventId
		} else if event.Timestamp != nil {
			id = fmt.Sprintf("evt-%d", *event.Timestamp)
		}

		// Name: message (truncated to 80 chars)
		name := message
		if len(name) > 80 {
			name = name[:80]
		}

		// Status classification using shared function from log_events.go
		status := classifyLogEventStatus(message)

		r := resource.Resource{
			ID:     id,
			Name:   name,
			Status: status,
			Fields: map[string]string{
				"timestamp": ts,
				"message":   message,
			},
			RawStruct: event,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
