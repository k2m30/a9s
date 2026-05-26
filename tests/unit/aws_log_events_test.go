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
// Log Events fetcher tests (child of Log Streams)
// ---------------------------------------------------------------------------

// TestFetchLogEvents_Basic verifies parsing of multiple log events with correct
// ID, Name, Fields, and RawStruct.
func TestFetchLogEvents_Basic(t *testing.T) {
	mock := &mockCWLogsGetLogEventsClient{
		output: &cloudwatchlogs.GetLogEventsOutput{
			Events: []cwlogstypes.OutputLogEvent{
				{
					Timestamp:     aws.Int64(1711065600000),
					Message:       aws.String("INFO Starting application server on port 8080"),
					IngestionTime: aws.Int64(1711065601000),
				},
				{
					Timestamp:     aws.Int64(1711065610000),
					Message:       aws.String("ERROR Failed to connect to database: connection refused"),
					IngestionTime: aws.Int64(1711065611000),
				},
				{
					Timestamp:     aws.Int64(1711065620000),
					Message:       aws.String("WARN High memory usage detected: 85%"),
					IngestionTime: aws.Int64(1711065621000),
				},
			},
		},
	}

	result, err := awsclient.FetchLogEvents(context.Background(), mock, "/aws/lambda/my-func", "stream-1", "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	resources := result.Resources

	if len(resources) != 3 {
		t.Fatalf("expected 3 resources, got %d", len(resources))
	}

	t.Run("event_0_ID_not_empty", func(t *testing.T) {
		if resources[0].ID == "" {
			t.Error("ID should not be empty")
		}
	})

	t.Run("event_0_Name", func(t *testing.T) {
		if resources[0].Name != "INFO Starting application server on port 8080" {
			t.Errorf("Name: expected message text, got %q", resources[0].Name)
		}
	})

	t.Run("event_0_Fields_message", func(t *testing.T) {
		if resources[0].Fields["message"] != "INFO Starting application server on port 8080" {
			t.Errorf("Fields[message]: expected full message, got %q", resources[0].Fields["message"])
		}
	})

	t.Run("event_0_Fields_timestamp", func(t *testing.T) {
		if resources[0].Fields["timestamp"] == "" {
			t.Error("Fields[timestamp] should not be empty")
		}
	})

	t.Run("event_0_Fields_ingestion_time", func(t *testing.T) {
		if resources[0].Fields["ingestion_time"] == "" {
			t.Error("Fields[ingestion_time] should not be empty")
		}
	})

	t.Run("event_0_RawStruct", func(t *testing.T) {
		r := resources[0]
		if r.RawStruct == nil {
			t.Fatal("RawStruct must not be nil")
		}
		raw, ok := r.RawStruct.(cwlogstypes.OutputLogEvent)
		if !ok {
			t.Fatalf("RawStruct should be cwlogstypes.OutputLogEvent, got %T", r.RawStruct)
		}
		if raw.Message == nil || *raw.Message != "INFO Starting application server on port 8080" {
			t.Errorf("RawStruct.Message not preserved correctly")
		}
	})

	// Verify all events have required fields
	t.Run("required_fields_present", func(t *testing.T) {
		requiredFields := []string{"timestamp", "message", "ingestion_time"}
		for i, r := range resources {
			for _, key := range requiredFields {
				if _, ok := r.Fields[key]; !ok {
					t.Errorf("resource[%d].Fields missing key %q", i, key)
				}
			}
		}
	})
}

// TestFetchLogEvents_Empty verifies that an empty response returns an empty
// slice with no error.
func TestFetchLogEvents_Empty(t *testing.T) {
	mock := &mockCWLogsGetLogEventsClient{
		output: &cloudwatchlogs.GetLogEventsOutput{
			Events: []cwlogstypes.OutputLogEvent{},
		},
	}

	result, err := awsclient.FetchLogEvents(context.Background(), mock, "/aws/lambda/empty", "stream-1", "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
}

// TestFetchLogEvents_APIError verifies that API errors are propagated correctly.
func TestFetchLogEvents_APIError(t *testing.T) {
	mock := &mockCWLogsGetLogEventsClient{
		err: fmt.Errorf("AWS API error: resource not found"),
	}

	result, err := awsclient.FetchLogEvents(context.Background(), mock, "/aws/lambda/err", "stream-1", "")
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources on error, got %d", len(result.Resources))
	}
}

// TestFetchLogEvents_StatusClassification verifies that event Status is classified
// based on message content:
// - Lines containing "ERROR", "FATAL", "Exception", "Traceback" → "ERROR"
// - Lines containing "WARN" → "WARN"
// - Lines containing "REPORT" → "REPORT"
// - Lines containing "START" or "END" → "META"
// - All other lines → ""
func TestFetchLogEvents_StatusClassification(t *testing.T) {
	mock := &mockCWLogsGetLogEventsClient{
		output: &cloudwatchlogs.GetLogEventsOutput{
			Events: []cwlogstypes.OutputLogEvent{
				{Timestamp: aws.Int64(1000), Message: aws.String("ERROR something went wrong")},
				{Timestamp: aws.Int64(2000), Message: aws.String("FATAL out of memory")},
				{Timestamp: aws.Int64(3000), Message: aws.String("java.lang.NullPointerException: null")},
				{Timestamp: aws.Int64(4000), Message: aws.String("Traceback (most recent call last):")},
				{Timestamp: aws.Int64(5000), Message: aws.String("WARN disk usage at 90%")},
				{Timestamp: aws.Int64(6000), Message: aws.String("REPORT RequestId: abc Duration: 100.0 ms")},
				{Timestamp: aws.Int64(7000), Message: aws.String("START RequestId: abc Version: $LATEST")},
				{Timestamp: aws.Int64(8000), Message: aws.String("END RequestId: abc")},
				{Timestamp: aws.Int64(9000), Message: aws.String("INFO All systems nominal")},
			},
		},
	}

	result, err := awsclient.FetchLogEvents(context.Background(), mock, "/aws/lambda/status", "stream-1", "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	resources := result.Resources

	if len(resources) != 9 {
		t.Fatalf("expected 9 resources, got %d", len(resources))
	}

	cases := []struct {
		idx            int
		expectedStatus string
		description    string
	}{
		{0, "ERROR", "ERROR keyword"},
		{1, "ERROR", "FATAL keyword"},
		{2, "ERROR", "Exception keyword"},
		{3, "ERROR", "Traceback keyword"},
		{4, "WARN", "WARN keyword"},
		{5, "REPORT", "REPORT keyword"},
		{6, "META", "START keyword"},
		{7, "META", "END keyword"},
		{8, "", "plain INFO message"},
	}

	for _, tc := range cases {
		t.Run(tc.description, func(t *testing.T) {
			got := resources[tc.idx].Fields["status"]
			if got != tc.expectedStatus {
				t.Errorf("resource[%d] Fields[\"status\"]: expected %q, got %q (message: %q)",
					tc.idx, tc.expectedStatus, got,
					resources[tc.idx].Fields["message"])
			}
		})
	}
}

// TestFetchLogEvents_MessageTruncation verifies that the Name field is truncated
// to at most 80 characters when the message exceeds that length.
func TestFetchLogEvents_MessageTruncation(t *testing.T) {
	longMessage := strings.Repeat("a", 150)

	mock := &mockCWLogsGetLogEventsClient{
		output: &cloudwatchlogs.GetLogEventsOutput{
			Events: []cwlogstypes.OutputLogEvent{
				{
					Timestamp: aws.Int64(1711065600000),
					Message:   aws.String(longMessage),
				},
			},
		},
	}

	result, err := awsclient.FetchLogEvents(context.Background(), mock, "/aws/lambda/trunc", "stream-1", "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	resources := result.Resources

	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	t.Run("Name_truncated_to_80", func(t *testing.T) {
		if len(resources[0].Name) > 80 {
			t.Errorf("Name length should be <= 80, got %d", len(resources[0].Name))
		}
	})

	t.Run("Fields_message_full", func(t *testing.T) {
		if resources[0].Fields["message"] != longMessage {
			t.Errorf("Fields[message] should contain the full message, got length %d",
				len(resources[0].Fields["message"]))
		}
	})
}

// TestFetchLogEvents_NilFields verifies that events with nil Message and nil
// Timestamp do not panic and produce reasonable defaults.
func TestFetchLogEvents_NilFields(t *testing.T) {
	mock := &mockCWLogsGetLogEventsClient{
		output: &cloudwatchlogs.GetLogEventsOutput{
			Events: []cwlogstypes.OutputLogEvent{
				{
					// All fields nil
				},
			},
		},
	}

	result, err := awsclient.FetchLogEvents(context.Background(), mock, "/aws/lambda/nil", "stream-1", "")
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

	t.Run("ID_not_empty", func(t *testing.T) {
		if r.ID == "" {
			t.Error("ID should not be empty even with nil timestamp")
		}
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

// TestFetchLogEvents_RawStruct verifies that RawStruct is the original
// cwlogstypes.OutputLogEvent, preserving all SDK fields.
func TestFetchLogEvents_RawStruct(t *testing.T) {
	mock := &mockCWLogsGetLogEventsClient{
		output: &cloudwatchlogs.GetLogEventsOutput{
			Events: []cwlogstypes.OutputLogEvent{
				{
					Timestamp:     aws.Int64(1711065600000),
					Message:       aws.String("test event"),
					IngestionTime: aws.Int64(1711065601000),
				},
			},
		},
	}

	result, err := awsclient.FetchLogEvents(context.Background(), mock, "/aws/lambda/raw", "stream-1", "")
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

	raw, ok := r.RawStruct.(cwlogstypes.OutputLogEvent)
	if !ok {
		t.Fatalf("RawStruct should be cwlogstypes.OutputLogEvent, got %T", r.RawStruct)
	}

	t.Run("Timestamp_preserved", func(t *testing.T) {
		if raw.Timestamp == nil || *raw.Timestamp != 1711065600000 {
			t.Errorf("RawStruct.Timestamp not preserved correctly")
		}
	})

	t.Run("Message_preserved", func(t *testing.T) {
		if raw.Message == nil || *raw.Message != "test event" {
			t.Errorf("RawStruct.Message not preserved correctly")
		}
	})

	t.Run("IngestionTime_preserved", func(t *testing.T) {
		if raw.IngestionTime == nil || *raw.IngestionTime != 1711065601000 {
			t.Errorf("RawStruct.IngestionTime not preserved correctly")
		}
	})
}

// TestLogEventColumns verifies that LogEventColumns returns the expected
// columns with correct keys: timestamp, message, ingestion_time.
func TestLogEventColumns(t *testing.T) {
	cols := resource.LogEventColumns()

	t.Run("column_count", func(t *testing.T) {
		if len(cols) < 2 {
			t.Fatalf("expected at least 2 columns, got %d", len(cols))
		}
	})

	t.Run("has_timestamp_column", func(t *testing.T) {
		found := false
		for _, col := range cols {
			if col.Key == "timestamp" {
				found = true
				break
			}
		}
		if !found {
			t.Error("missing 'timestamp' column")
		}
	})

	t.Run("has_message_column", func(t *testing.T) {
		found := false
		for _, col := range cols {
			if col.Key == "message" {
				found = true
				break
			}
		}
		if !found {
			t.Error("missing 'message' column")
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

	t.Run("message_column_wide_for_log_lines", func(t *testing.T) {
		for _, col := range cols {
			if col.Key == "message" && col.Width < 80 {
				t.Errorf("message column Width = %d, want >= 80 for log lines", col.Width)
			}
		}
	})
}

// TestFetchLogEvents_NewestFirst verifies that log events are fetched
// newest-first (StartFromHead=false) so the most recent activity appears
// at the top of the list during incidents.
func TestFetchLogEvents_NewestFirst(t *testing.T) {
	mock := &mockCWLogsGetLogEventsClient{
		output: &cloudwatchlogs.GetLogEventsOutput{},
	}

	_, _ = awsclient.FetchLogEvents(context.Background(), mock, "/aws/lambda/test", "stream-1", "")

	if mock.lastInput == nil {
		t.Fatal("expected GetLogEvents to be called")
	}
	if mock.lastInput.StartFromHead == nil || *mock.lastInput.StartFromHead != false {
		t.Errorf("StartFromHead should be false (newest first), got %v", mock.lastInput.StartFromHead)
	}
}
