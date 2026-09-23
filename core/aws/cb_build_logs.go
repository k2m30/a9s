// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/k2m30/a9s/v3/core/resource"
)

// FetchCBBuildLogs reads one page of a CodeBuild build's log stream, the
// newest first and each Load More the page before, converting it into a
// FetchResult.
func FetchCBBuildLogs(
	ctx context.Context,
	api CWLogsGetLogEventsAPI,
	logGroupName, logStreamName string,
	continuationToken string,
) (resource.FetchResult, error) {
	events, next, err := readStreamPage(ctx, api, logGroupName, logStreamName, continuationToken)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching build log events: %w", err)
	}

	var resources []resource.Resource
	ids := streamPageRowIDs(continuationToken, events)

	for _, event := range events {
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

		id := ids.at(tsVal, message)

		name := message
		if len(name) > 80 {
			name = name[:80]
		}

		status := classifyBuildLogStatus(message)

		r := resource.Resource{
			ID:   id,
			Name: name,
			Fields: map[string]string{
				"timestamp":       ts,
				"status":          status,
				"message":         message,
				"ingestion_time":  ingestionTime,
				"event_id":        id,
				"log_group_name":  logGroupName,
				"log_stream_name": logStreamName,
			},
			RawStruct: event,
		}

		resources = append(resources, r)
	}

	return resource.FetchResult{Resources: resources, Pagination: logReadPagination(len(resources), next)}, nil
}

// formatEpochMillisSec converts epoch milliseconds to a human-readable
// timestamp string including seconds.
func formatEpochMillisSec(ms int64) string {
	return time.UnixMilli(ms).UTC().Format("2006-01-02 15:04:05")
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
