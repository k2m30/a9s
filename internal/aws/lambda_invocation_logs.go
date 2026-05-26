package aws

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"

	"github.com/k2m30/a9s/v3/internal/resource"
)


// maxInvocationLogLines caps the result set for a single invocation's logs.
const maxInvocationLogLines = 500

// FetchLambdaInvocationLogs calls the CloudWatchLogs FilterLogEvents API with
// a filter pattern containing the request ID, returning individual log lines
// for a specific Lambda invocation as a FetchResult. It paginates through
// empty pages (CloudWatch Logs returns these when scanning across log streams
// that don't match) until events are found or the API signals no more pages.
func FetchLambdaInvocationLogs(ctx context.Context, api CWLogsFilterLogEventsAPI, logGroup, requestID string, continuationToken string) (resource.FetchResult, error) {
	filterPattern := fmt.Sprintf("%q", requestID)
	startTime := time.Now().Add(-24 * time.Hour).UnixMilli()

	var nextToken *string
	if continuationToken != "" {
		nextToken = &continuationToken
	}

	var resources []resource.Resource

	for {
		input := &cloudwatchlogs.FilterLogEventsInput{
			LogGroupName:  &logGroup,
			FilterPattern: &filterPattern,
			StartTime:     &startTime,
			NextToken:     nextToken,
		}

		output, err := api.FilterLogEvents(ctx, input)
		if err != nil {
			return resource.FetchResult{}, fmt.Errorf("fetching lambda invocation logs: %w", err)
		}

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

			resources = append(resources, resource.Resource{
				ID:       id,
				Name:     name,
				Findings: logEventFindings(status),
				Fields: map[string]string{
					"timestamp": ts,
					"message":   message,
					"status":    status,
				},
				RawStruct: event,
			})
		}

		if len(resources) >= maxInvocationLogLines {
			apiNextToken := ""
			if output.NextToken != nil {
				apiNextToken = *output.NextToken
			}
			return resource.FetchResult{
				Resources: resources,
				Pagination: &resource.PaginationMeta{
					IsTruncated: true,
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
