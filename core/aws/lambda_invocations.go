// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	cwlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// maxInvocations caps the result set to keep load times fast.
const maxInvocations = 50

// invocationLookbackHours limits the FilterLogEvents scan window.
const invocationLookbackHours = 24

// reportRegex matches the standard REPORT line from Lambda runtime.
var reportRegex = regexp.MustCompile(
	`REPORT RequestId:\s*([0-9a-zA-Z-]+)` +
		`\s*Duration:\s*([0-9.]+)\s*ms` +
		`\s*Billed Duration:\s*([0-9.]+)\s*ms` +
		`\s*Memory Size:\s*([0-9]+)\s*MB` +
		`\s*Max Memory Used:\s*([0-9]+)\s*MB`,
)

// initDurationRegex matches the optional Init Duration field.
var initDurationRegex = regexp.MustCompile(`Init Duration:\s*([0-9.]+)\s*ms`)

// xrayTraceRegex matches an optional XRAY TraceId field.
var xrayTraceRegex = regexp.MustCompile(`XRAY TraceId:\s*(\S+)`)

// timeoutRegex matches timeout status in the REPORT line.
var timeoutRegex = regexp.MustCompile(`Status:\s*timeout`)

// FetchLambdaInvocations reads the newest maxInvocations (50) REPORT lines of
// the function's log group in the last invocationLookbackHours, newest first,
// and parses each into an invocation row. Load More continues with the next
// older ones in the same window.
func FetchLambdaInvocations(ctx context.Context, api CWLogsFilterLogEventsAPI, functionName, logGroup string, continuationToken string) (resource.FetchResult, error) {
	cur, err := parseLogCursor(continuationToken)
	if err != nil {
		return resource.FetchResult{}, err
	}
	start := time.Now().Add(-invocationLookbackHours * time.Hour).UnixMilli()
	source := logSource{api: api, group: logGroup, pattern: "REPORT RequestId"}
	events, next, err := newestLogEvents(ctx, []logSource{source}, start, cur, maxInvocations)
	if err != nil {
		// ResourceNotFoundException means the log group doesn't exist → 0
		if ErrCodeIs(err, "ResourceNotFoundException") {
			return resource.FetchResult{}, nil
		}
		return resource.FetchResult{}, fmt.Errorf("fetching invocations for %s: %w", functionName, err)
	}

	var resources []resource.Resource
	for _, event := range events {
		if r, ok := convertReportEvent(event.FilteredLogEvent, logGroup); ok {
			resources = append(resources, r)
		}
	}
	return resource.FetchResult{Resources: resources, Pagination: logReadPagination(len(resources), next)}, nil
}

// convertReportEvent parses a single FilteredLogEvent for a REPORT line and returns
// a Resource plus true if the event matched, or a zero Resource plus false if not.
// logGroup is threaded through to Fields["log_group"] alongside the event's own
// Fields["log_stream"] (event.LogStreamName) so the console-link builder can
// deep-link straight to the invocation's own log stream.
func convertReportEvent(event cwlogstypes.FilteredLogEvent, logGroup string) (resource.Resource, bool) {
	message := ""
	if event.Message != nil {
		message = *event.Message
	}

	matches := reportRegex.FindStringSubmatch(message)
	if matches == nil {
		return resource.Resource{}, false
	}

	requestID := matches[1]
	durationMs := matches[2]
	billedDurationMs := matches[3]
	memorySizeMB := matches[4]
	memoryUsedMB := matches[5]

	formattedDuration := formatDuration(durationMs)
	formattedBilled := formatDuration(billedDurationMs)

	ts := ""
	if event.Timestamp != nil {
		ts = formatEpochMillis(*event.Timestamp)
	}

	// Init Duration (cold start detection)
	coldStart := "no"
	initDurationMs := ""
	if initMatch := initDurationRegex.FindStringSubmatch(message); initMatch != nil {
		coldStart = "yes"
		initDurationMs = formatDuration(initMatch[1])
	}

	xrayTraceID := ""
	if xrayMatch := xrayTraceRegex.FindStringSubmatch(message); xrayMatch != nil {
		xrayTraceID = xrayMatch[1]
	}

	status := "OK"
	if timeoutRegex.MatchString(message) {
		status = "TIMEOUT"
	}

	name := requestID
	if len(name) > 8 {
		name = name[:8]
	}

	memoryUsed := memoryUsedMB + "/" + memorySizeMB + " MB"

	logStream := ""
	if event.LogStreamName != nil {
		logStream = *event.LogStreamName
	}

	return resource.Resource{
		ID:   requestID,
		Name: name,
		Fields: map[string]string{
			"request_id":             requestID,
			"timestamp":              ts,
			"status":                 status,
			"duration_ms":            formattedDuration,
			"duration_ms_raw":        durationMs,
			"billed_duration_ms":     formattedBilled,
			"billed_duration_ms_raw": billedDurationMs,
			"memory_size_mb":         memorySizeMB,
			"memory_used_mb":         memoryUsedMB,
			"memory_used":            memoryUsed,
			"init_duration_ms":       initDurationMs,
			"cold_start":             coldStart,
			"xray_trace_id":          xrayTraceID,
			"log_group":              logGroup,
			"log_stream":             logStream,
		},
		Findings:  lambdaInvocationFindings(status),
		RawStruct: event,
	}, true
}

// lambdaInvocationFindings returns a wave1 finding for an invocation whose
// REPORT line status is TIMEOUT (the only non-OK status timeoutRegex
// classifies — Lambda's runtime REPORT line carries no other structured
// error/status signal for a successful-vs-failed invocation split).
func lambdaInvocationFindings(status string) []domain.Finding {
	if status == "TIMEOUT" {
		return []domain.Finding{wave1Finding(CodeLambdaInvocationTimeout)}
	}
	return nil
}

// formatDuration formats a duration string, stripping trailing ".00".
func formatDuration(ms string) string {
	return strings.TrimSuffix(ms, ".00") + " ms"
}
