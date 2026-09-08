package unit

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
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
// - Lines containing "WARN" → "warn"
// - Lines containing "REPORT" → "report"
// - Lines containing "START" or "END" → "meta"
// - All other lines → ""
//
// INVERTED for aws6 row 6 (one humanize owner): the class is a9s's OWN
// judgement about the line, not a value CloudWatch returned, so it is written
// in the words the status cell renders. The old UPPER-CASE expectations are
// not to be restored — an all-caps token here is a9s writing an SDK-shaped
// constant of its own and then having to translate it back on every surface.
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
		{0, "error", "ERROR keyword"},
		{1, "error", "FATAL keyword"},
		{2, "error", "Exception keyword"},
		{3, "error", "Traceback keyword"},
		{4, "warn", "WARN keyword"},
		{5, "report", "REPORT keyword"},
		{6, "meta", "START keyword"},
		{7, "meta", "END keyword"},
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

// TestFetchLogEvents_MessageTruncation verifies that the Name field is capped
// to roughly 100 runes (rune-safe) when the message exceeds that length.
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

	t.Run("Name_capped_at_100_runes", func(t *testing.T) {
		if len([]rune(resources[0].Name)) > 100 {
			t.Errorf("Name length should be <= 100 runes, got %d", len([]rune(resources[0].Name)))
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

// ---------------------------------------------------------------------------
// Clean-Name tests (bug fix): Name must be a clean, human-readable summary —
// never a raw-JSON prefix or a byte-sliced multibyte string. Fields["message"]
// must always retain the full RAW message unchanged.
// ---------------------------------------------------------------------------

// TestFetchLogEvents_Name_JSONMessage_ExtractsInnerMessageField verifies that
// when the raw log message is a JSON object with a string "message" field,
// Resource.Name is that inner value — not the raw "{...}" prefix.
func TestFetchLogEvents_Name_JSONMessage_ExtractsInnerMessageField(t *testing.T) {
	raw := `{"level":"INFO","message":"Lambda invocation event","timestamp":"2026-07-01T17:20:00Z"}`
	mock := &mockCWLogsGetLogEventsClient{
		output: &cloudwatchlogs.GetLogEventsOutput{
			Events: []cwlogstypes.OutputLogEvent{
				{Timestamp: aws.Int64(1711065600000), Message: aws.String(raw)},
			},
		},
	}

	result, err := awsclient.FetchLogEvents(context.Background(), mock, "/aws/lambda/json-msg", "stream-1", "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]

	t.Run("Name_is_inner_message_value", func(t *testing.T) {
		if r.Name != "Lambda invocation event" {
			t.Errorf("Name: expected %q, got %q", "Lambda invocation event", r.Name)
		}
	})

	t.Run("Name_does_not_start_with_brace", func(t *testing.T) {
		if strings.HasPrefix(r.Name, "{") {
			t.Errorf("Name must not be a raw-JSON prefix; got %q", r.Name)
		}
	})

	t.Run("Fields_message_is_full_raw_JSON", func(t *testing.T) {
		if r.Fields["message"] != raw {
			t.Errorf("Fields[message] must equal the full raw message including braces; got %q, want %q", r.Fields["message"], raw)
		}
	})
}

// TestFetchLogEvents_Name_PlainTextMessage_UsesMessageVerbatim verifies that a
// plain-text (non-JSON) message becomes the Name unchanged.
func TestFetchLogEvents_Name_PlainTextMessage_UsesMessageVerbatim(t *testing.T) {
	raw := "START RequestId: abc"
	mock := &mockCWLogsGetLogEventsClient{
		output: &cloudwatchlogs.GetLogEventsOutput{
			Events: []cwlogstypes.OutputLogEvent{
				{Timestamp: aws.Int64(1711065600000), Message: aws.String(raw)},
			},
		},
	}

	result, err := awsclient.FetchLogEvents(context.Background(), mock, "/aws/lambda/plain-msg", "stream-1", "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]

	if r.Name != raw {
		t.Errorf("Name: expected plain-text message verbatim %q, got %q", raw, r.Name)
	}
	if r.Fields["message"] != raw {
		t.Errorf("Fields[message]: expected full raw message %q, got %q", raw, r.Fields["message"])
	}
}

// TestFetchLogEvents_Name_MultiLineMessage_UsesFirstNonEmptyLineOnly verifies
// that a multi-line message produces a Name containing only the first
// non-empty line, with no embedded newline.
func TestFetchLogEvents_Name_MultiLineMessage_UsesFirstNonEmptyLineOnly(t *testing.T) {
	raw := "\n\nERROR something broke\nstack trace line 1\nstack trace line 2\n"
	mock := &mockCWLogsGetLogEventsClient{
		output: &cloudwatchlogs.GetLogEventsOutput{
			Events: []cwlogstypes.OutputLogEvent{
				{Timestamp: aws.Int64(1711065600000), Message: aws.String(raw)},
			},
		},
	}

	result, err := awsclient.FetchLogEvents(context.Background(), mock, "/aws/lambda/multiline-msg", "stream-1", "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]

	t.Run("Name_has_no_newline", func(t *testing.T) {
		if strings.Contains(r.Name, "\n") {
			t.Errorf("Name must not contain an embedded newline; got %q", r.Name)
		}
	})

	t.Run("Name_is_first_non_empty_line", func(t *testing.T) {
		if r.Name != "ERROR something broke" {
			t.Errorf("Name: expected first non-empty line %q, got %q", "ERROR something broke", r.Name)
		}
	})

	t.Run("Fields_message_is_full_raw_multiline", func(t *testing.T) {
		if r.Fields["message"] != raw {
			t.Errorf("Fields[message] must equal the full raw message including newlines; got %q, want %q", r.Fields["message"], raw)
		}
	})
}

// TestFetchLogEvents_Name_LongMessage_CapsAtApprox100RunesRuneSafe verifies
// that a very long message (>100 runes, including multibyte characters) is
// capped to roughly 100 runes without splitting a multibyte rune — a naive
// byte-slice (e.g. name[:100]) would corrupt a multibyte rune straddling the
// cut point.
func TestFetchLogEvents_Name_LongMessage_CapsAtApprox100RunesRuneSafe(t *testing.T) {
	// Build a message where multibyte runes (emoji + CJK) straddle the
	// approx-100-rune cut boundary, so a byte-slice cap would corrupt it.
	prefix := strings.Repeat("a", 95)
	multibyte := "日本語テスト🎉🎉🎉ending-tail-content-that-must-be-truncated-away"
	raw := prefix + multibyte

	mock := &mockCWLogsGetLogEventsClient{
		output: &cloudwatchlogs.GetLogEventsOutput{
			Events: []cwlogstypes.OutputLogEvent{
				{Timestamp: aws.Int64(1711065600000), Message: aws.String(raw)},
			},
		},
	}

	result, err := awsclient.FetchLogEvents(context.Background(), mock, "/aws/lambda/long-msg", "stream-1", "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]

	nameRunes := []rune(r.Name)

	t.Run("Name_capped_to_approx_100_runes", func(t *testing.T) {
		if len(nameRunes) > 100 {
			t.Errorf("Name must be capped to ~100 runes, got %d runes: %q", len(nameRunes), r.Name)
		}
	})

	t.Run("Name_is_valid_utf8_no_corrupted_rune", func(t *testing.T) {
		if !utf8.ValidString(r.Name) {
			t.Errorf("Name must be valid UTF-8 (rune-safe truncation); got invalid string %q", r.Name)
		}
		// A corrupted truncation would introduce the UTF-8 replacement
		// character when a multibyte rune is split mid-sequence.
		if strings.ContainsRune(r.Name, utf8.RuneError) {
			t.Errorf("Name contains utf8.RuneError — truncation split a multibyte rune; got %q", r.Name)
		}
	})

	t.Run("Fields_message_is_full_raw_multibyte", func(t *testing.T) {
		if r.Fields["message"] != raw {
			t.Errorf("Fields[message] must equal the full raw message including all multibyte runes; got %q, want %q", r.Fields["message"], raw)
		}
	})
}
