package unit

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// Lambda Invocation Logs fetcher tests (level-2 child of Lambda Invocations)
// ---------------------------------------------------------------------------

// TestFetchLambdaInvocationLogs_Basic verifies filtering log events by
// RequestId, checking ID, Name, Status, all Fields, and RawStruct.
func TestFetchLambdaInvocationLogs_Basic(t *testing.T) {
	mock := &mockCWLogsFilterLogEventsClient{
		outputs: []*cloudwatchlogs.FilterLogEventsOutput{
			{
				Events: []cwlogstypes.FilteredLogEvent{
					{
						Timestamp:     aws.Int64(1711065600000),
						Message:       aws.String("START RequestId: 12345678-1234-1234-1234-123456789012 Version: $LATEST\n"),
						IngestionTime: aws.Int64(1711065600500),
						LogStreamName: aws.String("2024/03/22/[$LATEST]abcdef"),
						EventId:       aws.String("log-001"),
					},
					{
						Timestamp:     aws.Int64(1711065601000),
						Message:       aws.String("INFO Processing request for user abc-123\n"),
						IngestionTime: aws.Int64(1711065601500),
						LogStreamName: aws.String("2024/03/22/[$LATEST]abcdef"),
						EventId:       aws.String("log-002"),
					},
					{
						Timestamp:     aws.Int64(1711065602000),
						Message:       aws.String("ERROR Failed to connect to database: connection refused\n"),
						IngestionTime: aws.Int64(1711065602500),
						LogStreamName: aws.String("2024/03/22/[$LATEST]abcdef"),
						EventId:       aws.String("log-003"),
					},
					{
						Timestamp:     aws.Int64(1711065603000),
						Message:       aws.String("END RequestId: 12345678-1234-1234-1234-123456789012\n"),
						IngestionTime: aws.Int64(1711065603500),
						LogStreamName: aws.String("2024/03/22/[$LATEST]abcdef"),
						EventId:       aws.String("log-004"),
					},
				},
			},
		},
	}

	result, err := awsclient.FetchLambdaInvocationLogs(
		context.Background(),
		mock,
		"/aws/lambda/my-func",
		"12345678-1234-1234-1234-123456789012",
		"",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	resources := result.Resources

	if len(resources) != 4 {
		t.Fatalf("expected 4 resources, got %d", len(resources))
	}

	t.Run("log_0_ID_not_empty", func(t *testing.T) {
		if resources[0].ID == "" {
			t.Error("ID should not be empty")
		}
	})

	t.Run("log_0_Name_is_message", func(t *testing.T) {
		if resources[0].Name == "" {
			t.Error("Name should not be empty")
		}
	})

	t.Run("log_0_Fields_timestamp", func(t *testing.T) {
		if resources[0].Fields["timestamp"] == "" {
			t.Error("Fields[timestamp] should not be empty")
		}
	})

	t.Run("log_0_Fields_message", func(t *testing.T) {
		if resources[0].Fields["message"] == "" {
			t.Error("Fields[message] should not be empty")
		}
	})

	t.Run("log_0_RawStruct", func(t *testing.T) {
		r := resources[0]
		if r.RawStruct == nil {
			t.Fatal("RawStruct must not be nil")
		}
		raw, ok := r.RawStruct.(cwlogstypes.FilteredLogEvent)
		if !ok {
			t.Fatalf("RawStruct should be cwlogstypes.FilteredLogEvent, got %T", r.RawStruct)
		}
		if raw.Message == nil || len(*raw.Message) == 0 {
			t.Error("RawStruct.Message not preserved correctly")
		}
	})

	// Verify required fields on all log lines
	t.Run("required_fields_present", func(t *testing.T) {
		requiredFields := []string{"timestamp", "message"}
		for i, r := range resources {
			for _, key := range requiredFields {
				if _, ok := r.Fields[key]; !ok {
					t.Errorf("resource[%d].Fields missing key %q", i, key)
				}
			}
		}
	})
}

// TestFetchLambdaInvocationLogs_StatusClassification verifies that log line
// Status is classified using classifyLogEventStatus (reused from log_events.go).
func TestFetchLambdaInvocationLogs_StatusClassification(t *testing.T) {
	mock := &mockCWLogsFilterLogEventsClient{
		outputs: []*cloudwatchlogs.FilterLogEventsOutput{
			{
				Events: []cwlogstypes.FilteredLogEvent{
					{Timestamp: aws.Int64(1000), Message: aws.String("ERROR something went wrong")},
					{Timestamp: aws.Int64(2000), Message: aws.String("WARN disk usage at 90%")},
					{Timestamp: aws.Int64(3000), Message: aws.String("REPORT RequestId: abc Duration: 100.0 ms")},
					{Timestamp: aws.Int64(4000), Message: aws.String("START RequestId: abc Version: $LATEST")},
					{Timestamp: aws.Int64(5000), Message: aws.String("END RequestId: abc")},
					{Timestamp: aws.Int64(6000), Message: aws.String("INFO All systems nominal")},
				},
			},
		},
	}

	result, err := awsclient.FetchLambdaInvocationLogs(
		context.Background(),
		mock,
		"/aws/lambda/status-func",
		"abc",
		"",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	resources := result.Resources

	if len(resources) != 6 {
		t.Fatalf("expected 6 resources, got %d", len(resources))
	}

	cases := []struct {
		idx            int
		expectedStatus string
		description    string
	}{
		{0, "ERROR", "ERROR keyword"},
		{1, "WARN", "WARN keyword"},
		{2, "REPORT", "REPORT keyword"},
		{3, "META", "START keyword"},
		{4, "META", "END keyword"},
		{5, "", "plain INFO message"},
	}

	for _, tc := range cases {
		t.Run(tc.description, func(t *testing.T) {
			if resources[tc.idx].Status != tc.expectedStatus {
				t.Errorf("resource[%d] Status: expected %q, got %q (message: %q)",
					tc.idx, tc.expectedStatus, resources[tc.idx].Status,
					resources[tc.idx].Fields["message"])
			}
		})
	}
}

// TestFetchLambdaInvocationLogs_Empty verifies that an empty response
// returns an empty slice with no error.
func TestFetchLambdaInvocationLogs_Empty(t *testing.T) {
	mock := &mockCWLogsFilterLogEventsClient{
		outputs: []*cloudwatchlogs.FilterLogEventsOutput{
			{
				Events: []cwlogstypes.FilteredLogEvent{},
			},
		},
	}

	result, err := awsclient.FetchLambdaInvocationLogs(
		context.Background(),
		mock,
		"/aws/lambda/empty-func",
		"empty-request-id",
		"",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
}

// TestFetchLambdaInvocationLogs_APIError verifies that API errors are
// propagated correctly.
func TestFetchLambdaInvocationLogs_APIError(t *testing.T) {
	mock := &mockCWLogsFilterLogEventsClient{
		err: fmt.Errorf("AWS API error: resource not found"),
	}

	result, err := awsclient.FetchLambdaInvocationLogs(
		context.Background(),
		mock,
		"/aws/lambda/err-func",
		"err-request-id",
		"",
	)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources on error, got %d", len(result.Resources))
	}
}

// TestFetchLambdaInvocationLogs_MessageNewlineStripping verifies that
// trailing newlines are stripped from log messages.
func TestFetchLambdaInvocationLogs_MessageNewlineStripping(t *testing.T) {
	mock := &mockCWLogsFilterLogEventsClient{
		outputs: []*cloudwatchlogs.FilterLogEventsOutput{
			{
				Events: []cwlogstypes.FilteredLogEvent{
					{
						Timestamp: aws.Int64(1711065600000),
						Message:   aws.String("INFO Application started successfully\n"),
						EventId:   aws.String("log-newline"),
					},
				},
			},
		},
	}

	result, err := awsclient.FetchLambdaInvocationLogs(
		context.Background(),
		mock,
		"/aws/lambda/newline-func",
		"newline-request-id",
		"",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	resources := result.Resources

	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	msg := resources[0].Fields["message"]
	if strings.Contains(msg, "\n") {
		t.Errorf("Fields[message] should not contain newlines, got %q", msg)
	}
}

// TestFetchLambdaInvocationLogs_NilFields verifies that events with nil
// Message and nil Timestamp do not panic.
func TestFetchLambdaInvocationLogs_NilFields(t *testing.T) {
	mock := &mockCWLogsFilterLogEventsClient{
		outputs: []*cloudwatchlogs.FilterLogEventsOutput{
			{
				Events: []cwlogstypes.FilteredLogEvent{
					{
						// All fields nil
					},
				},
			},
		},
	}

	result, err := awsclient.FetchLambdaInvocationLogs(
		context.Background(),
		mock,
		"/aws/lambda/nil-func",
		"nil-request-id",
		"",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	resources := result.Resources

	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	r := resources[0]

	t.Run("no_panic", func(t *testing.T) {
		// If we got here, no panic occurred
	})

	t.Run("message_empty", func(t *testing.T) {
		if r.Fields["message"] != "" {
			t.Errorf("Fields[message]: expected empty, got %q", r.Fields["message"])
		}
	})

	t.Run("timestamp_empty", func(t *testing.T) {
		if r.Fields["timestamp"] != "" {
			t.Errorf("Fields[timestamp]: expected empty, got %q", r.Fields["timestamp"])
		}
	})
}

// TestFetchLambdaInvocationLogs_RawStruct verifies that RawStruct is the
// original cwlogstypes.FilteredLogEvent, preserving all SDK fields.
func TestFetchLambdaInvocationLogs_RawStruct(t *testing.T) {
	mock := &mockCWLogsFilterLogEventsClient{
		outputs: []*cloudwatchlogs.FilterLogEventsOutput{
			{
				Events: []cwlogstypes.FilteredLogEvent{
					{
						Timestamp:     aws.Int64(1711065600000),
						Message:       aws.String("test log line"),
						IngestionTime: aws.Int64(1711065601000),
						LogStreamName: aws.String("2024/03/22/[$LATEST]abc"),
						EventId:       aws.String("raw-log-001"),
					},
				},
			},
		},
	}

	result, err := awsclient.FetchLambdaInvocationLogs(
		context.Background(),
		mock,
		"/aws/lambda/raw-func",
		"raw-request-id",
		"",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	resources := result.Resources

	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	r := resources[0]
	if r.RawStruct == nil {
		t.Fatal("RawStruct must not be nil")
	}

	raw, ok := r.RawStruct.(cwlogstypes.FilteredLogEvent)
	if !ok {
		t.Fatalf("RawStruct should be cwlogstypes.FilteredLogEvent, got %T", r.RawStruct)
	}

	t.Run("Timestamp_preserved", func(t *testing.T) {
		if raw.Timestamp == nil || *raw.Timestamp != 1711065600000 {
			t.Errorf("RawStruct.Timestamp not preserved correctly")
		}
	})

	t.Run("Message_preserved", func(t *testing.T) {
		if raw.Message == nil || *raw.Message != "test log line" {
			t.Errorf("RawStruct.Message not preserved correctly")
		}
	})

	t.Run("IngestionTime_preserved", func(t *testing.T) {
		if raw.IngestionTime == nil || *raw.IngestionTime != 1711065601000 {
			t.Errorf("RawStruct.IngestionTime not preserved correctly")
		}
	})

	t.Run("EventId_preserved", func(t *testing.T) {
		if raw.EventId == nil || *raw.EventId != "raw-log-001" {
			t.Errorf("RawStruct.EventId not preserved correctly")
		}
	})
}

// TestFetchLambdaInvocationLogs_ParentContextKeys verifies that the fetcher
// uses @parent.log_group and request_id from the invocation context.
func TestFetchLambdaInvocationLogs_ParentContextKeys(t *testing.T) {
	mock := &mockCWLogsFilterLogEventsClient{
		outputs: []*cloudwatchlogs.FilterLogEventsOutput{
			{
				Events: []cwlogstypes.FilteredLogEvent{},
			},
		},
	}

	logGroup := "/custom/log/group"
	requestID := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"

	_, err := awsclient.FetchLambdaInvocationLogs(
		context.Background(),
		mock,
		logGroup,
		requestID,
		"",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify the log group and request ID filter are passed correctly
	if mock.lastInput == nil {
		t.Fatal("expected FilterLogEvents to be called")
	}
	if mock.lastInput.LogGroupName == nil || *mock.lastInput.LogGroupName != logGroup {
		t.Errorf("expected log group %q, got %q", logGroup, *mock.lastInput.LogGroupName)
	}
	// The filter pattern should contain the request ID
	if mock.lastInput.FilterPattern == nil || !strings.Contains(*mock.lastInput.FilterPattern, requestID) {
		filterPattern := ""
		if mock.lastInput.FilterPattern != nil {
			filterPattern = *mock.lastInput.FilterPattern
		}
		t.Errorf("expected filter pattern containing request ID %q, got %q", requestID, filterPattern)
	}
}

// TestFetchLambdaInvocationLogs_TimestampFormatting verifies that epoch ms
// timestamps are formatted into human-readable strings.
func TestFetchLambdaInvocationLogs_TimestampFormatting(t *testing.T) {
	mock := &mockCWLogsFilterLogEventsClient{
		outputs: []*cloudwatchlogs.FilterLogEventsOutput{
			{
				Events: []cwlogstypes.FilteredLogEvent{
					{
						Timestamp: aws.Int64(1711065600000), // 2024-03-22 00:00:00 UTC
						Message:   aws.String("test event"),
						EventId:   aws.String("ts-test"),
					},
				},
			},
		},
	}

	result, err := awsclient.FetchLambdaInvocationLogs(
		context.Background(),
		mock,
		"/aws/lambda/ts-func",
		"ts-request-id",
		"",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	resources := result.Resources

	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	ts := resources[0].Fields["timestamp"]
	if ts == "" {
		t.Fatal("Fields[timestamp] should not be empty")
	}
	// Should be formatted, not raw epoch ms
	if ts == "1711065600000" {
		t.Errorf("timestamp should be formatted, not raw epoch ms: %q", ts)
	}
	if !strings.Contains(ts, "2024-03-22") {
		t.Errorf("timestamp should contain date '2024-03-22', got %q", ts)
	}
}

// TestFetchLambdaInvocationLogs_StartTimeBound verifies that FilterLogEvents
// is called with a StartTime to avoid scanning the entire log group history.
func TestFetchLambdaInvocationLogs_StartTimeBound(t *testing.T) {
	mock := &mockCWLogsFilterLogEventsClient{
		outputs: []*cloudwatchlogs.FilterLogEventsOutput{
			{Events: []cwlogstypes.FilteredLogEvent{}},
		},
	}

	_, err := awsclient.FetchLambdaInvocationLogs(
		context.Background(),
		mock,
		"/aws/lambda/time-bound-func",
		"aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
		"",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if mock.lastInput == nil {
		t.Fatal("expected FilterLogEvents to be called")
	}
	if mock.lastInput.StartTime == nil {
		t.Fatal("FilterLogEvents must set StartTime to avoid scanning entire log group history")
	}
}

// TestLambdaInvocationLogColumns verifies that LambdaInvocationLogColumns
// returns the expected columns with correct keys.
func TestLambdaInvocationLogColumns(t *testing.T) {
	cols := resource.LambdaInvocationLogColumns()

	expectedKeys := []string{"timestamp", "message"}

	t.Run("column_count", func(t *testing.T) {
		if len(cols) != 2 {
			t.Fatalf("expected 2 columns, got %d", len(cols))
		}
	})

	t.Run("column_keys", func(t *testing.T) {
		for i, expected := range expectedKeys {
			if cols[i].Key != expected {
				t.Errorf("column[%d].Key: expected %q, got %q", i, expected, cols[i].Key)
			}
		}
	})

	t.Run("columns_have_titles", func(t *testing.T) {
		for i, col := range cols {
			if col.Title == "" {
				t.Errorf("column[%d] (%s) has empty Title", i, col.Key)
			}
		}
	})

	t.Run("columns_have_positive_width", func(t *testing.T) {
		for i, col := range cols {
			if col.Width <= 0 {
				t.Errorf("column[%d] (%s) has non-positive Width: %d", i, col.Key, col.Width)
			}
		}
	})
}

// TestFetchLambdaInvocationLogs_ContinuationTokenForwarded verifies that a
// non-empty continuationToken is forwarded to FilterLogEventsInput.NextToken.
func TestFetchLambdaInvocationLogs_ContinuationTokenForwarded(t *testing.T) {
	mock := &mockCWLogsFilterLogEventsClient{
		outputs: []*cloudwatchlogs.FilterLogEventsOutput{
			{Events: []cwlogstypes.FilteredLogEvent{}},
		},
	}

	_, err := awsclient.FetchLambdaInvocationLogs(
		context.Background(),
		mock,
		"/aws/lambda/my-function",
		"abc123requestid",
		"cont-token-page2",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if mock.lastInput == nil {
		t.Fatal("mock was not called")
	}
	if mock.lastInput.NextToken == nil || *mock.lastInput.NextToken != "cont-token-page2" {
		t.Errorf("NextToken not forwarded: got %v, want %q", mock.lastInput.NextToken, "cont-token-page2")
	}
}

// ============================================================================
