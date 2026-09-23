// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
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

// restoreDurationRegex matches the Restore Duration a SnapStart function's
// REPORT line carries when the invocation ran in a newly restored execution
// environment, in place of Init Duration, ahead of Billed Restore Duration
// (docs.aws.amazon.com/lambda/latest/dg/snapstart-monitoring.html,
// "Understanding logging and billing behavior with SnapStart").
var restoreDurationRegex = regexp.MustCompile(`Restore Duration:\s*([0-9.]+)\s*ms`)

// statusRegex matches the invocation's status at the end of a REPORT line,
// and the error type that follows a runtime or extension crash:
// "... Status: error Error Type: Runtime.ExitError", "... Status: timeout"
// (docs.aws.amazon.com/lambda/latest/dg/lambda-runtime-environment.html,
// "Failures during the invoke phase"). The older REPORT line carries no
// status.
var statusRegex = regexp.MustCompile(`Status:\s*(\w+)(?:\s+Error Type:\s*(\S+))?`)

// lambdaReportPattern matches an invocation's report in either log format a
// function writes: the text "REPORT RequestId: …" line, and the JSON system
// log event of type platform.report, whose text holds that literal
// (docs.aws.amazon.com/lambda/latest/dg/telemetry-schema-reference.html#platform-report).
// "?" makes each quoted phrase an
// alternative (docs.aws.amazon.com/AmazonCloudWatch/latest/logs/FilterAndPatternSyntax.html,
// "Match optional terms"). Both are read in one query because changing a
// function's format "doesn't affect existing logs stored in CloudWatch Logs"
// (the log-format page): a function that switched has reports of both shapes,
// and each event is parsed by its own.
const lambdaReportPattern = `?"REPORT RequestId" ?"platform.report"`

// FetchLambdaInvocations reads the newest maxInvocations (50) reports of the
// function's log group in the invocationLookbackHours before the list opened,
// newest first, and parses each into an invocation row. Load More continues
// with the next older ones in the same window. In a group other than the
// function's own /aws/lambda/<name>, which other functions may share, only
// the function's streams are read (see lambdaFunctionStreams) when api can
// list them. When that listing stopped at its page cap, the page that ends
// the read comes with an error saying so, so the list is never taken for the
// whole window; the pages before it already offer Load More.
func FetchLambdaInvocations(ctx context.Context, api CWLogsFilterLogEventsAPI, functionName, logGroup string, continuationToken string) (resource.FetchResult, error) {
	cur, err := parseLogCursor(continuationToken, invocationLookbackHours*time.Hour)
	if err != nil {
		return resource.FetchResult{}, err
	}

	streamSets := [][]string{nil}
	streamsPartial := false
	if lister, ok := api.(CWLogsDescribeLogStreamsAPI); ok && logGroup != "/aws/lambda/"+functionName {
		streams, partial, listErr := lambdaFunctionStreams(ctx, lister, logGroup, functionName, cur.start, cur.end)
		if listErr != nil {
			if ErrCodeIs(listErr, "ResourceNotFoundException") {
				return resource.FetchResult{}, nil
			}
			return resource.FetchResult{}, fmt.Errorf("listing log streams of %s for %s: %w", logGroup, functionName, listErr)
		}
		streamSets = slices.Collect(slices.Chunk(streams, maxLogStreamNamesPerFilter))
		streamsPartial = partial
	}
	var sources []logSource
	for _, streams := range streamSets {
		sources = append(sources, logSource{api: api, group: logGroup, streams: streams, pattern: lambdaReportPattern})
	}

	events, next, err := newestLogEvents(ctx, sources, cur, maxInvocations)
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
	result := resource.FetchResult{Resources: resources, Pagination: logReadPagination(len(resources), next)}
	if streamsPartial && next == "" {
		return result, fmt.Errorf("the log streams of %s in %s run past the listing's page cap: invocations in the streams not listed are not shown", functionName, logGroup)
	}
	return result, nil
}

// maxLogStreamNamesPerFilter is the most stream names one FilterLogEvents
// call accepts.
const maxLogStreamNamesPerFilter = 100

// lambdaStreamEventLag is how far a stream's lastEventTimestamp may trail its
// newest event: it "typically updates in less than an hour from ingestion"
// (DescribeLogStreams, docs.aws.amazon.com/AmazonCloudWatchLogs/latest/APIReference/API_DescribeLogStreams.html).
const lambdaStreamEventLag = time.Hour

// lambdaFunctionStreams lists the streams one function writes in a log group
// it may share with others. Lambda names them
// YYYY/MM/DD/<function_name>[<function_version>][<execution_environment_GUID>]
// "so that the mapping between log messages and functions is preserved, even
// if you use the same log group for multiple functions"
// (docs.aws.amazon.com/lambda/latest/dg/monitoring-cloudwatchlogs-loggroups.html).
// The date is the stream's creation day, so a stream opened before the
// window can hold events inside it. Two reads cover [start, end]: the streams
// with the prefix "YYYY/MM/DD/<name>[" for each day of the window, which a
// fresh stream is listed by before its lastEventTimestamp is updated; and the
// group's streams by LastEventTime, newest first, down to start less the
// timestamp's documented lag, which finds a stream opened on an earlier day.
// DescribeLogStreams refuses a prefix with that order, so the second read
// keeps the names whose function part is exactly this function's. partial is
// true when a read stopped at the page cap.
func lambdaFunctionStreams(ctx context.Context, api CWLogsDescribeLogStreamsAPI, group, function string, start, end int64) ([]string, bool, error) {
	var streams []string
	partial := false
	seen := map[string]bool{}
	keep := func(name string) {
		if !seen[name] && lambdaStreamOf(name) == function {
			seen[name] = true
			streams = append(streams, name)
		}
	}
	for day := time.UnixMilli(start).UTC().Truncate(24 * time.Hour); !day.After(time.UnixMilli(end).UTC()); day = day.AddDate(0, 0, 1) {
		prefix := day.Format("2006/01/02") + "/" + function + "["
		got, complete, err := PageAll(ctx, PerParentPageCap, func(ctx context.Context, token *string) ([]cwlogstypes.LogStream, *string, error) {
			out, callErr := api.DescribeLogStreams(ctx, &cloudwatchlogs.DescribeLogStreamsInput{
				LogGroupName:        aws.String(group),
				LogStreamNamePrefix: aws.String(prefix),
				NextToken:           token,
			})
			if callErr != nil {
				return nil, nil, callErr
			}
			return out.LogStreams, out.NextToken, nil
		})
		if err != nil {
			return nil, false, err
		}
		partial = partial || !complete
		for _, s := range got {
			keep(aws.ToString(s.LogStreamName))
		}
	}

	cutoff := start - lambdaStreamEventLag.Milliseconds()
	recent, complete, err := PageAll(ctx, PerParentPageCap, func(ctx context.Context, token *string) ([]cwlogstypes.LogStream, *string, error) {
		out, callErr := api.DescribeLogStreams(ctx, &cloudwatchlogs.DescribeLogStreamsInput{
			LogGroupName: aws.String(group),
			OrderBy:      cwlogstypes.OrderByLastEventTime,
			Descending:   aws.Bool(true),
			NextToken:    token,
		})
		if callErr != nil {
			return nil, nil, callErr
		}
		for i, s := range out.LogStreams {
			if s.LastEventTimestamp != nil && *s.LastEventTimestamp < cutoff {
				return out.LogStreams[:i], nil, nil
			}
		}
		return out.LogStreams, out.NextToken, nil
	})
	if err != nil {
		return nil, false, err
	}
	for _, s := range recent {
		keep(aws.ToString(s.LogStreamName))
	}
	return streams, partial || !complete, nil
}

// lambdaStreamOf returns the function a custom-group stream name
// YYYY/MM/DD/<function_name>[<version>][<guid>] belongs to, or "" for a name
// of any other shape.
func lambdaStreamOf(stream string) string {
	if len(stream) < 11 || stream[4] != '/' || stream[7] != '/' || stream[10] != '/' {
		return ""
	}
	name, _, ok := strings.Cut(stream[11:], "[")
	if !ok {
		return ""
	}
	return name
}

// lambdaReport is one invocation's report, read from either log format.
// status is the report's Status value, "" when the line carries none, and
// errorType the error type that comes with error and failure.
type lambdaReport struct {
	requestID, durationMs, billedMs, memoryMB, usedMB, initMs, restoreMs, xrayTraceID string
	status, errorType                                                                 string
}

// parseTextReport reads a plain-text REPORT line.
func parseTextReport(message string) (lambdaReport, bool) {
	m := reportRegex.FindStringSubmatch(message)
	if m == nil {
		return lambdaReport{}, false
	}
	r := lambdaReport{requestID: m[1], durationMs: m[2], billedMs: m[3], memoryMB: m[4], usedMB: m[5]}
	if im := initDurationRegex.FindStringSubmatch(message); im != nil {
		r.initMs = im[1]
	}
	if rm := restoreDurationRegex.FindStringSubmatch(message); rm != nil {
		r.restoreMs = rm[1]
	}
	if sm := statusRegex.FindStringSubmatch(message); sm != nil {
		r.status, r.errorType = sm[1], sm[2]
	}
	if xm := xrayTraceRegex.FindStringSubmatch(message); xm != nil {
		r.xrayTraceID = xm[1]
	}
	return r, true
}

// lambdaPlatformEvent is the part of a JSON-format system log event the
// invocation views read: its type, and the record's request ID, status and
// the errorType that comes with failure and error, ReportMetrics and trace
// context (docs.aws.amazon.com/lambda/latest/dg/telemetry-schema-reference.html,
// platform.start, platform.report, ReportMetrics, Status, TraceContext).
// The numbers are kept as Lambda wrote them, so they read as a text REPORT
// line's do.
type lambdaPlatformEvent struct {
	Type   string `json:"type"`
	Record struct {
		RequestID string `json:"requestId"`
		Status    string `json:"status"`
		ErrorType string `json:"errorType"`
		Metrics   struct {
			DurationMs        json.Number `json:"durationMs"`
			BilledDurationMs  json.Number `json:"billedDurationMs"`
			MemorySizeMB      json.Number `json:"memorySizeMB"`
			MaxMemoryUsedMB   json.Number `json:"maxMemoryUsedMB"`
			InitDurationMs    json.Number `json:"initDurationMs"`
			RestoreDurationMs json.Number `json:"restoreDurationMs"`
		} `json:"metrics"`
		Tracing struct {
			Value string `json:"value"`
		} `json:"tracing"`
	} `json:"record"`
}

// parsePlatformEvent reads a JSON-format system log event, false for any
// other line.
func parsePlatformEvent(message string) (lambdaPlatformEvent, bool) {
	var e lambdaPlatformEvent
	trimmed := strings.TrimSpace(message)
	if !strings.HasPrefix(trimmed, "{") || json.Unmarshal([]byte(trimmed), &e) != nil || !strings.HasPrefix(e.Type, "platform.") {
		return lambdaPlatformEvent{}, false
	}
	return e, true
}

// parseJSONReport reads a platform.report record.
func parseJSONReport(message string) (lambdaReport, bool) {
	e, ok := parsePlatformEvent(message)
	if !ok || e.Type != "platform.report" || e.Record.RequestID == "" {
		return lambdaReport{}, false
	}
	m := e.Record.Metrics
	r := lambdaReport{
		requestID:  e.Record.RequestID,
		durationMs: m.DurationMs.String(), billedMs: m.BilledDurationMs.String(),
		memoryMB: m.MemorySizeMB.String(), usedMB: m.MaxMemoryUsedMB.String(),
		initMs: m.InitDurationMs.String(), restoreMs: m.RestoreDurationMs.String(),
		status: e.Record.Status, errorType: e.Record.ErrorType,
	}
	// TraceContext.value is the X-Amzn-Trace-Id header, "Root=<trace id>;…";
	// the text REPORT line carries the Root trace ID alone.
	for part := range strings.SplitSeq(e.Record.Tracing.Value, ";") {
		if id, ok := strings.CutPrefix(part, "Root="); ok {
			r.xrayTraceID = id
		}
	}
	return r, true
}

// convertReportEvent parses a single FilteredLogEvent holding an invocation
// report, in either log format, into a Resource plus true, or a zero Resource
// plus false for any other line. logGroup is threaded through to
// Fields["log_group"] alongside the event's own Fields["log_stream"]
// (event.LogStreamName) so the console-link builder can deep-link straight to
// the invocation's own log stream, and the invocation's log reads it.
func convertReportEvent(event cwlogstypes.FilteredLogEvent, logGroup string) (resource.Resource, bool) {
	message := aws.ToString(event.Message)
	report, ok := parseTextReport(message)
	if !ok {
		report, ok = parseJSONReport(message)
	}
	if !ok {
		return resource.Resource{}, false
	}

	ts := ""
	if event.Timestamp != nil {
		ts = formatEpochMillis(*event.Timestamp)
	}

	coldStart := "no"
	initDurationMs, restoreDurationMs := "", ""
	if report.initMs != "" {
		coldStart = "yes"
		initDurationMs = formatDuration(report.initMs)
	}
	if report.restoreMs != "" {
		coldStart = "yes"
		restoreDurationMs = formatDuration(report.restoreMs)
	}

	status := "OK"
	switch report.status {
	case "timeout":
		status = "TIMEOUT"
	case "error", "failure":
		status = "ERROR"
	}

	name := report.requestID
	if len(name) > 8 {
		name = name[:8]
	}

	return resource.Resource{
		ID:   report.requestID,
		Name: name,
		Fields: map[string]string{
			"request_id":             report.requestID,
			"timestamp":              ts,
			"report_ms":              strconv.FormatInt(aws.ToInt64(event.Timestamp), 10),
			"status":                 status,
			"duration_ms":            formatDuration(report.durationMs),
			"duration_ms_raw":        report.durationMs,
			"billed_duration_ms":     formatDuration(report.billedMs),
			"billed_duration_ms_raw": report.billedMs,
			"memory_size_mb":         report.memoryMB,
			"memory_used_mb":         report.usedMB,
			"memory_used":            report.usedMB + "/" + report.memoryMB + " MB",
			"init_duration_ms":       initDurationMs,
			"restore_duration_ms":    restoreDurationMs,
			"cold_start":             coldStart,
			"error_type":             report.errorType,
			"xray_trace_id":          report.xrayTraceID,
			"log_group":              logGroup,
			"log_stream":             aws.ToString(event.LogStreamName),
		},
		Findings:  lambdaInvocationFindings(status),
		RawStruct: event,
	}, true
}

// lambdaInvocationFindings returns the wave1 finding of an invocation that
// timed out or failed.
func lambdaInvocationFindings(status string) []domain.Finding {
	switch status {
	case "TIMEOUT":
		return []domain.Finding{wave1Finding(CodeLambdaInvocationTimeout)}
	case "ERROR":
		return []domain.Finding{wave1Finding(CodeLambdaInvocationError)}
	}
	return nil
}

// formatDuration formats a duration string, stripping trailing ".00".
func formatDuration(ms string) string {
	return strings.TrimSuffix(ms, ".00") + " ms"
}
