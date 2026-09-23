// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// maxInvocationLogLines caps the result set for a single invocation's logs.
const maxInvocationLogLines = 500

// maxInvocationLogScanPages caps the number of FilterLogEvents calls. Empty
// pages carrying a NextToken are the documented normal case here (scanning
// across log streams that don't match the request ID), so maxInvocationLogLines
// alone never fires while that happens; without this cap a request ID with no
// matching logs scans the full 24h lookback window page by page forever.
const maxInvocationLogScanPages = 100

// lambdaMaxInvocation is the longest an invocation can run: a function's
// timeout goes "up to a maximum value of 900 seconds (15 minutes)"
// (docs.aws.amazon.com/lambda/latest/dg/configuration-timeout.html).
const lambdaMaxInvocation = 15 * time.Minute

// FetchLambdaInvocationLogs reads the log of an invocation whose stream is not
// known: the lines of the group in the last 24 hours that carry its request
// ID.
func FetchLambdaInvocationLogs(ctx context.Context, api CWLogsFilterLogEventsAPI, logGroup, requestID string, continuationToken string) (resource.FetchResult, error) {
	input := &cloudwatchlogs.FilterLogEventsInput{
		LogGroupName:  aws.String(logGroup),
		FilterPattern: aws.String(fmt.Sprintf("%q", requestID)),
		StartTime:     aws.Int64(time.Now().Add(-24 * time.Hour).UnixMilli()),
	}
	return readInvocationLog(ctx, api, input, logGroup, "", continuationToken)
}

// fetchInvocationStreamLog reads an invocation's log from its own stream:
// every line from its START to its REPORT, those its runtime wrote without
// the request ID (a print, an uncaught stack trace) included. An execution
// environment runs one invocation at a time and writes one stream, so what
// lies between the two lines is this invocation's. The START is found by the
// request ID in the stream within the longest an invocation can run before
// the REPORT; the lines are then the stream's in [START, REPORT].
func fetchInvocationStreamLog(ctx context.Context, api CWLogsFilterLogEventsAPI, logGroup, logStream, requestID string, reportMs int64, continuationToken string) (resource.FetchResult, error) {
	find := &cloudwatchlogs.FilterLogEventsInput{
		LogGroupName:   aws.String(logGroup),
		LogStreamNames: []string{logStream},
		FilterPattern:  aws.String(fmt.Sprintf("%q", requestID)),
		StartTime:      aws.Int64(reportMs - lambdaMaxInvocation.Milliseconds() - time.Minute.Milliseconds()),
		EndTime:        aws.Int64(reportMs),
	}
	startMs := aws.ToInt64(find.StartTime)
	calls := 0
	for calls < maxInvocationLogScanPages {
		out, err := api.FilterLogEvents(ctx, find)
		calls++
		if err != nil {
			return resource.FetchResult{}, fmt.Errorf("fetching lambda invocation logs: %w", err)
		}
		if i := slices.IndexFunc(out.Events, func(e cwlogstypes.FilteredLogEvent) bool {
			return invocationMarker(aws.ToString(e.Message), requestID) == "start"
		}); i >= 0 {
			startMs = aws.ToInt64(out.Events[i].Timestamp)
			break
		}
		if aws.ToString(out.NextToken) == "" {
			break
		}
		find.NextToken = out.NextToken
	}

	input := &cloudwatchlogs.FilterLogEventsInput{
		LogGroupName:   aws.String(logGroup),
		LogStreamNames: []string{logStream},
		StartTime:      aws.Int64(startMs),
		EndTime:        aws.Int64(reportMs),
	}
	return readInvocationLog(ctx, api, input, logGroup, requestID, continuationToken)
}

// invocationMarker reports whether message is the "start" or "report" line of
// requestID, in either log format: text "START RequestId: <id>" and
// "REPORT RequestId: <id>", or JSON platform.start and platform.report
// (docs.aws.amazon.com/lambda/latest/dg/telemetry-schema-reference.html).
func invocationMarker(message, requestID string) string {
	switch {
	case strings.HasPrefix(message, "START RequestId: "+requestID):
		return "start"
	case strings.HasPrefix(message, "REPORT RequestId: "+requestID):
		return "report"
	}
	if e, ok := parsePlatformEvent(message); ok && e.Record.RequestID == requestID {
		switch e.Type {
		case "platform.start":
			return "start"
		case "platform.report":
			return "report"
		}
	}
	return ""
}

// readInvocationLog pages through input from continuationToken into log rows.
// With bounded set, rows begin at that invocation's START line and end at
// its REPORT line: the edges of [START, REPORT] can share a millisecond with
// the invocations before and after it.
func readInvocationLog(ctx context.Context, api CWLogsFilterLogEventsAPI, input *cloudwatchlogs.FilterLogEventsInput, logGroup, bounded, continuationToken string) (resource.FetchResult, error) {
	if continuationToken != "" {
		input.NextToken = aws.String(continuationToken)
	}
	started := bounded == "" || continuationToken != ""
	var resources []resource.Resource

	pages := 0
	for {
		output, err := api.FilterLogEvents(ctx, input)
		if err != nil {
			return resource.FetchResult{}, fmt.Errorf("fetching lambda invocation logs: %w", err)
		}
		pages++

		for _, event := range output.Events {
			message := ""
			if event.Message != nil {
				message = strings.TrimRight(*event.Message, "\n\r")
			}
			marker := ""
			if bounded != "" {
				marker = invocationMarker(message, bounded)
			}
			if !started {
				if marker != "start" {
					continue
				}
				started = true
			}

			ts := ""
			if event.Timestamp != nil {
				ts = formatEpochMillis(*event.Timestamp)
			}

			id := ""
			if event.EventId != nil {
				id = *event.EventId
			} else if event.Timestamp != nil {
				id = eventRowIDFromMillis(*event.Timestamp, message)
			}

			status := classifyLogEventStatus(message)

			resources = append(resources, resource.Resource{
				ID:       id,
				Name:     logEventDisplayName(message),
				Findings: logEventFindings(status),
				Fields: map[string]string{
					"timestamp":  ts,
					"message":    message,
					"status":     status,
					"log_group":  logGroup,
					"log_stream": aws.ToString(event.LogStreamName),
				},
				RawStruct: event,
			})
			if marker == "report" {
				return resource.FetchResult{Resources: resources, Pagination: logReadPagination(len(resources), "")}, nil
			}
		}

		apiNextToken := aws.ToString(output.NextToken)
		if len(resources) >= maxInvocationLogLines || pages >= maxInvocationLogScanPages || apiNextToken == "" {
			// IsTruncated only ever claims what apiNextToken can resume: a
			// dead cursor (IsTruncated=true, NextToken="") would make Load
			// More restart from page 1 forever.
			return resource.FetchResult{Resources: resources, Pagination: logReadPagination(len(resources), apiNextToken)}, nil
		}
		input.NextToken = output.NextToken
	}
}
