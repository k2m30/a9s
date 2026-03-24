package aws

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("lambda_invocations", []string{
		"request_id", "timestamp", "status", "duration_ms",
		"billed_duration_ms", "memory_size_mb", "memory_used_mb",
		"memory_used", "init_duration_ms", "cold_start", "xray_trace_id",
	})

	resource.RegisterChildFetcher("lambda_invocations", func(ctx context.Context, clients interface{}, parentCtx resource.ParentContext) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchLambdaInvocations(ctx, c.CloudWatchLogs, parentCtx["function_name"], parentCtx["log_group"])
	})

	resource.RegisterChildType(resource.ResourceTypeDef{
		Name:      "Lambda Invocations",
		ShortName: "lambda_invocations",
		Columns:   resource.LambdaInvocationColumns(),
		Children: []resource.ChildViewDef{
			{
				ChildType:      "lambda_invocation_logs",
				Key:            "enter",
				ContextKeys:    map[string]string{"log_group": "@parent.log_group", "request_id": "request_id"},
				DisplayNameKey: "request_id",
			},
		},
	})
}

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

// FetchLambdaInvocations calls the CloudWatchLogs FilterLogEvents API with a
// "REPORT RequestId" filter pattern, parses each REPORT line, and returns a
// slice of Resource structs representing individual Lambda invocations.
func FetchLambdaInvocations(ctx context.Context, api CWLogsFilterLogEventsAPI, functionName, logGroup string) ([]resource.Resource, error) {
	var resources []resource.Resource
	var nextToken *string

	startTime := time.Now().Add(-invocationLookbackHours * time.Hour).UnixMilli()
	limit := int32(maxInvocations)

	for {
		input := &cloudwatchlogs.FilterLogEventsInput{
			LogGroupName:  &logGroup,
			FilterPattern: strPtr("REPORT RequestId"),
			StartTime:     &startTime,
			Limit:         &limit,
			NextToken:     nextToken,
		}

		output, err := api.FilterLogEvents(ctx, input)
		if err != nil {
			if strings.Contains(err.Error(), "ResourceNotFoundException") {
				return nil, nil
			}
			return nil, fmt.Errorf("fetching lambda invocations: %w", err)
		}

		for _, event := range output.Events {
			message := ""
			if event.Message != nil {
				message = *event.Message
			}

			// Only parse REPORT lines
			matches := reportRegex.FindStringSubmatch(message)
			if matches == nil {
				continue
			}

			requestID := matches[1]
			durationMs := matches[2]
			billedDurationMs := matches[3]
			memorySizeMB := matches[4]
			memoryUsedMB := matches[5]

			// Format duration: strip trailing .00
			formattedDuration := formatDuration(durationMs)
			formattedBilled := formatDuration(billedDurationMs)

			// Timestamp
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

			// XRAY trace
			xrayTraceID := ""
			if xrayMatch := xrayTraceRegex.FindStringSubmatch(message); xrayMatch != nil {
				xrayTraceID = xrayMatch[1]
			}

			// Status detection
			status := "OK"
			if timeoutRegex.MatchString(message) {
				status = "TIMEOUT"
			}

			// Name: truncated request ID (first 8 chars)
			name := requestID
			if len(name) > 8 {
				name = name[:8]
			}

			// Memory used display: "used/total MB"
			memoryUsed := memoryUsedMB + "/" + memorySizeMB + " MB"

			r := resource.Resource{
				ID:     requestID,
				Name:   name,
				Status: status,
				Fields: map[string]string{
					"request_id":       requestID,
					"timestamp":        ts,
					"status":           status,
					"duration_ms":      formattedDuration,
					"billed_duration_ms": formattedBilled,
					"memory_size_mb":   memorySizeMB,
					"memory_used_mb":   memoryUsedMB,
					"memory_used":      memoryUsed,
					"init_duration_ms": initDurationMs,
					"cold_start":       coldStart,
					"xray_trace_id":    xrayTraceID,
				},
				RawStruct: event,
			}

			resources = append(resources, r)
		}

		if output.NextToken == nil || len(resources) >= maxInvocations {
			break
		}
		nextToken = output.NextToken
	}

	// Reverse so newest invocations are first
	for i, j := 0, len(resources)-1; i < j; i, j = i+1, j-1 {
		resources[i], resources[j] = resources[j], resources[i]
	}

	return resources, nil
}

// formatDuration formats a duration string, stripping trailing ".00".
func formatDuration(ms string) string {
	return strings.TrimSuffix(ms, ".00") + " ms"
}

// strPtr returns a pointer to a string.
func strPtr(s string) *string {
	return &s
}
