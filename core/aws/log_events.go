// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// maxLogEventDisplayNameRunes caps the derived display name length so the
// detail frame title and list identity stay single-line and terminal-width
// friendly, even for verbose JSON log messages.
const maxLogEventDisplayNameRunes = 100

// logEventDisplayName derives a clean, single-line display name from a log
// event message for the detail frame title and list identity. If the message
// is a JSON object carrying a string "message" field, that inner value is used;
// otherwise the first non-empty line is used. Internal whitespace is collapsed
// and the result is capped rune-safely.
func logEventDisplayName(message string) string {
	var name string

	trimmed := strings.TrimSpace(message)
	if strings.HasPrefix(trimmed, "{") {
		var parsed struct {
			Message string `json:"message"`
		}
		if err := json.Unmarshal([]byte(trimmed), &parsed); err == nil && parsed.Message != "" {
			name = parsed.Message
		} else {
			name = firstNonEmptyLine(message)
		}
	} else {
		name = firstNonEmptyLine(message)
	}

	name = collapseWhitespace(name)
	return capRunes(name, maxLogEventDisplayNameRunes)
}

// firstNonEmptyLine returns the first non-blank line of s, or "" if every
// line is blank.
func firstNonEmptyLine(s string) string {
	for line := range strings.SplitSeq(s, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// collapseWhitespace replaces runs of whitespace (including tabs and any
// remaining newlines) with a single space and trims the result.
func collapseWhitespace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// capRunes truncates s to at most n runes, never splitting a multibyte rune.
func capRunes(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}

// logEventFindings returns wave1 findings derived from a classified log status
// (see classifyLogEventStatus). ERROR → broken; WARN → warn; REPORT/META/""
// emit no finding (healthy).
func logEventFindings(status string) []domain.Finding {
	switch status {
	case "ERROR":
		return []domain.Finding{wave1Finding(CodeCWLogError, domain.SevBroken)}
	case "WARN":
		return []domain.Finding{wave1Finding(CodeCWLogWarn, domain.SevWarn)}
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

		name := logEventDisplayName(message)

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
				"log_group":      logGroupName,
				"log_stream":     logStreamName,
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
