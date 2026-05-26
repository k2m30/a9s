package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// logEventFindings returns wave1 findings derived from a classified log status
// (see classifyLogEventStatus). ERROR → broken; WARN → warn; REPORT/META/""
// emit no finding (healthy).
func logEventFindings(status string) []domain.Finding {
	switch status {
	case "ERROR":
		return []domain.Finding{{Code: CodeCWLogError, Phrase: "error", Severity: domain.SevBroken, Source: "wave1"}}
	case "WARN":
		return []domain.Finding{{Code: CodeCWLogWarn, Phrase: "warning", Severity: domain.SevWarn, Source: "wave1"}}
	}
	return nil
}

// FetchLogEvents calls the CloudWatchLogs GetLogEvents API for a given
// log group and stream, converting the response into a FetchResult.
// This is a single-call API, but uses FetchResult for consistency.
func FetchLogEvents(ctx context.Context, api CWLogsGetLogEventsAPI, logGroupName, logStreamName string, continuationToken string) (resource.FetchResult, error) {
	input := &cloudwatchlogs.GetLogEventsInput{
		LogGroupName:  &logGroupName,
		LogStreamName: &logStreamName,
		StartFromHead: new(false),
	}

	output, err := api.GetLogEvents(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching log events: %w", err)
	}

	var resources []resource.Resource

	for i, event := range output.Events {
		message := ""
		if event.Message != nil {
			message = *event.Message
		}

		ts := ""
		tsVal := int64(0)
		if event.Timestamp != nil {
			tsVal = *event.Timestamp
			ts = formatEpochMillis(tsVal)
		}

		ingestionTime := ""
		if event.IngestionTime != nil {
			ingestionTime = formatEpochMillis(*event.IngestionTime)
		}

		// ID: use timestamp + index for uniqueness
		id := fmt.Sprintf("evt-%d-%d", tsVal, i)

		// Name: first 80 chars of message
		name := message
		if len(name) > 80 {
			name = name[:80]
		}

		// Status classification based on message content
		status := classifyLogEventStatus(message)

		r := resource.Resource{
			ID:       id,
			Name:     name,
			Findings: logEventFindings(status),
			Fields: map[string]string{
				"timestamp":      ts,
				"message":        message,
				"ingestion_time": ingestionTime,
				"event_id":       id,
				"status":         status,
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

// classifyLogEventStatus classifies a log event message into a status category.
func classifyLogEventStatus(message string) string {
	switch {
	case strings.Contains(message, "ERROR") ||
		strings.Contains(message, "FATAL") ||
		strings.Contains(message, "Exception") ||
		strings.Contains(message, "Traceback"):
		return "ERROR"
	case strings.Contains(message, "WARN"):
		return "WARN"
	case strings.Contains(message, "REPORT"):
		return "REPORT"
	case strings.Contains(message, "START") ||
		strings.Contains(message, "END"):
		return "META"
	default:
		return ""
	}
}
