// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"

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
// (see classifyLogEventStatus). error → broken; warn → warn; report/meta/""
// emit no finding (healthy).
func logEventFindings(status string) []domain.Finding {
	switch status {
	case logStatusError:
		return []domain.Finding{wave1Finding(CodeCWLogError)}
	case logStatusWarn:
		return []domain.Finding{wave1Finding(CodeCWLogWarn)}
	}
	return nil
}

// readStreamPage reads one GetLogEvents page of a stream: the newest page
// for token "", otherwise the page before the one token came with. Events
// are in chronological order within the page. next is the token of the page
// before it, or "" at the stream's start, where AWS hands back the token it
// was sent.
func readStreamPage(ctx context.Context, api CWLogsGetLogEventsAPI, group, stream, token string) (events []cwlogstypes.OutputLogEvent, next string, err error) {
	input := &cloudwatchlogs.GetLogEventsInput{
		LogGroupName:  aws.String(group),
		LogStreamName: aws.String(stream),
		StartFromHead: aws.Bool(false),
	}
	if token != "" {
		input.NextToken = aws.String(token)
	}
	out, err := api.GetLogEvents(ctx, input)
	if err != nil {
		return nil, "", err
	}
	back := aws.ToString(out.NextBackwardToken)
	if back == "" || back == token {
		return out.Events, "", nil
	}
	// A backward token is handed out on every page, the first page of the
	// stream included; only asking for the page before tells whether it
	// exists, and one event of it is enough. A refused probe leaves Load More
	// on offer, where the refusal surfaces.
	probeInput := *input
	probeInput.NextToken = aws.String(back)
	probeInput.Limit = aws.Int32(1)
	probe, err := api.GetLogEvents(ctx, &probeInput)
	if err == nil && len(probe.Events) == 0 && aws.ToString(probe.NextBackwardToken) == back {
		return out.Events, "", nil
	}
	return out.Events, back, nil
}

// FetchLogEvents reads one page of a log stream, the newest first and each
// Load More the page before, converting it into a FetchResult.
func FetchLogEvents(ctx context.Context, api CWLogsGetLogEventsAPI, logGroupName, logStreamName string, continuationToken string) (resource.FetchResult, error) {
	events, next, err := readStreamPage(ctx, api, logGroupName, logStreamName, continuationToken)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching log events: %w", err)
	}

	var resources []resource.Resource
	ids := streamPageRowIDs(continuationToken, events)

	for _, event := range events {
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

		id := ids.at(tsVal, message)

		name := logEventDisplayName(message)

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

	return resource.FetchResult{Resources: resources, Pagination: logReadPagination(len(resources), next)}, nil
}

// The classes classifyLogEventStatus sorts a log line into. These are a9s's
// own judgement about a line, not a value CloudWatch returned, so they are
// written in the words the status cell renders rather than in the shape of an
// AWS constant that would have to be translated back on every surface.
const (
	logStatusError  = "error"
	logStatusWarn   = "warn"
	logStatusReport = "report"
	logStatusMeta   = "meta"
)

// classifyLogEventStatus classifies a log event message into a status category.
// The tokens it matches on are what a runtime writes into the LINE; the class
// it returns is what a9s says about the line. A Lambda system log event in
// JSON format (a platform.* type) is classed as its text-format line is: a
// report (platform.report, platform.initReport, platform.restoreReport) as
// REPORT, any other platform event as START and END.
func classifyLogEventStatus(message string) string {
	if e, ok := parsePlatformEvent(message); ok {
		if strings.HasSuffix(strings.ToLower(e.Type), "report") {
			return logStatusReport
		}
		return logStatusMeta
	}
	switch {
	case strings.Contains(message, "ERROR") ||
		strings.Contains(message, "FATAL") ||
		strings.Contains(message, "Exception") ||
		strings.Contains(message, "Traceback"):
		return logStatusError
	case strings.Contains(message, "WARN"):
		return logStatusWarn
	case strings.Contains(message, "REPORT"):
		return logStatusReport
	case strings.Contains(message, "START") ||
		strings.Contains(message, "END"):
		return logStatusMeta
	default:
		return ""
	}
}
