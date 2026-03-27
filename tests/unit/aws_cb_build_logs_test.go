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
	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// CodeBuild Build Logs fetcher tests (Level 2 child of CodeBuild Builds,
// cross-service to CloudWatch Logs)
// ---------------------------------------------------------------------------

// TestFetchCBBuildLogs_Basic verifies parsing of 2 log events with correct
// timestamp formatting, message, and status classification.
func TestFetchCBBuildLogs_Basic(t *testing.T) {
	mock := &mockCWLogsGetLogEventsClient{
		output: &cloudwatchlogs.GetLogEventsOutput{
			Events: []cwlogstypes.OutputLogEvent{
				{
					Timestamp:     aws.Int64(1718445600000), // 2024-06-15 10:00:00 UTC
					Message:       aws.String("[Container] Running command echo hello"),
					IngestionTime: aws.Int64(1718445601000),
				},
				{
					Timestamp:     aws.Int64(1718445610000), // 2024-06-15 10:00:10 UTC
					Message:       aws.String("Phase complete: BUILD. Status: SUCCEEDED"),
					IngestionTime: aws.Int64(1718445611000),
				},
			},
		},
	}

	result, err := awsclient.FetchCBBuildLogs(
		context.Background(),
		mock,
		"/aws/codebuild/my-project",
		"build-id-001",
		"",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	resources := result.Resources

	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	t.Run("first_event_ID_synthetic", func(t *testing.T) {
		// ID should be synthetic "evt-{timestamp}-{index}" format
		if resources[0].ID == "" {
			t.Error("ID should not be empty")
		}
		if !strings.HasPrefix(resources[0].ID, "evt-") {
			t.Errorf("ID should start with 'evt-', got %q", resources[0].ID)
		}
	})

	t.Run("first_event_Name_from_message", func(t *testing.T) {
		if resources[0].Name == "" {
			t.Error("Name should not be empty")
		}
		if !strings.Contains(resources[0].Name, "Running command") {
			t.Errorf("Name should contain message text, got %q", resources[0].Name)
		}
	})

	t.Run("first_event_Status_in_progress", func(t *testing.T) {
		// "Running command" maps to IN_PROGRESS
		if resources[0].Status != "IN_PROGRESS" {
			t.Errorf("Status: expected %q, got %q", "IN_PROGRESS", resources[0].Status)
		}
	})

	t.Run("second_event_Status_succeeded", func(t *testing.T) {
		// "Phase complete" and "SUCCEEDED" maps to SUCCEEDED
		if resources[1].Status != "SUCCEEDED" {
			t.Errorf("Status: expected %q, got %q", "SUCCEEDED", resources[1].Status)
		}
	})

	t.Run("Fields_timestamp", func(t *testing.T) {
		ts := resources[0].Fields["timestamp"]
		if ts == "" {
			t.Error("Fields[timestamp] should not be empty")
		}
		// Should be formatted from epoch ms 1718445600000
		if !strings.Contains(ts, "2024") {
			t.Errorf("Fields[timestamp] should contain year, got %q", ts)
		}
	})

	t.Run("Fields_message", func(t *testing.T) {
		msg := resources[0].Fields["message"]
		if !strings.Contains(msg, "Running command") {
			t.Errorf("Fields[message] should contain message text, got %q", msg)
		}
	})

	t.Run("Fields_ingestion_time", func(t *testing.T) {
		it := resources[0].Fields["ingestion_time"]
		if it == "" {
			t.Error("Fields[ingestion_time] should not be empty")
		}
	})

	t.Run("Fields_event_id", func(t *testing.T) {
		eid := resources[0].Fields["event_id"]
		if eid == "" {
			t.Error("Fields[event_id] should not be empty")
		}
	})

	t.Run("required_fields_present", func(t *testing.T) {
		requiredFields := []string{
			"timestamp", "message", "ingestion_time", "event_id",
		}
		for _, key := range requiredFields {
			if _, ok := resources[0].Fields[key]; !ok {
				t.Errorf("Fields missing key %q", key)
			}
		}
	})
}

// TestFetchCBBuildLogs_Empty verifies that empty log events return an empty
// slice with no error.
func TestFetchCBBuildLogs_Empty(t *testing.T) {
	mock := &mockCWLogsGetLogEventsClient{
		output: &cloudwatchlogs.GetLogEventsOutput{
			Events: []cwlogstypes.OutputLogEvent{},
		},
	}

	result, err := awsclient.FetchCBBuildLogs(
		context.Background(),
		mock,
		"/aws/codebuild/empty",
		"stream-empty",
		"",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
}

// TestFetchCBBuildLogs_Error verifies that API errors are propagated.
func TestFetchCBBuildLogs_Error(t *testing.T) {
	mock := &mockCWLogsGetLogEventsClient{
		err: fmt.Errorf("AWS API error: access denied"),
	}

	result, err := awsclient.FetchCBBuildLogs(
		context.Background(),
		mock,
		"/aws/codebuild/err",
		"stream-err",
		"",
	)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "access denied") {
		t.Errorf("error should contain 'access denied', got %q", err.Error())
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources on error, got %d", len(result.Resources))
	}
}

// TestFetchCBBuildLogs_NilFields verifies that events with all nil pointers
// do not cause a panic.
func TestFetchCBBuildLogs_NilFields(t *testing.T) {
	mock := &mockCWLogsGetLogEventsClient{
		output: &cloudwatchlogs.GetLogEventsOutput{
			Events: []cwlogstypes.OutputLogEvent{
				{
					// All fields nil
				},
			},
		},
	}

	// Should not panic
	result, err := awsclient.FetchCBBuildLogs(
		context.Background(),
		mock,
		"/aws/codebuild/nil",
		"stream-nil",
		"",
	)
	if err != nil {
		t.Fatalf("expected no error for nil fields, got %v", err)
	}
	resources := result.Resources
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	// Message should be empty or placeholder
	_ = resources[0].Name
	_ = resources[0].Fields["message"]
	_ = resources[0].Fields["timestamp"]
}

// TestFetchCBBuildLogs_TimestampFormatting verifies that a known epoch ms
// value is formatted to the expected human-readable string.
func TestFetchCBBuildLogs_TimestampFormatting(t *testing.T) {
	// 1718445600000 ms = 2024-06-15 10:00:00 UTC
	mock := &mockCWLogsGetLogEventsClient{
		output: &cloudwatchlogs.GetLogEventsOutput{
			Events: []cwlogstypes.OutputLogEvent{
				{
					Timestamp:     aws.Int64(1718445600000),
					Message:       aws.String("test message"),
					IngestionTime: aws.Int64(1718445601000),
				},
			},
		},
	}

	result, err := awsclient.FetchCBBuildLogs(
		context.Background(),
		mock,
		"/aws/codebuild/ts-test",
		"stream-ts",
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
	if !strings.Contains(ts, "2024-06-15") {
		t.Errorf("Fields[timestamp] should contain '2024-06-15', got %q", ts)
	}
	if !strings.Contains(ts, "10:00") {
		t.Errorf("Fields[timestamp] should contain '10:00', got %q", ts)
	}
}

// TestFetchCBBuildLogs_StatusClassification verifies all status classification
// cases: ERROR, SUCCEEDED, IN_PROGRESS, and default empty.
func TestFetchCBBuildLogs_StatusClassification(t *testing.T) {
	tests := []struct {
		name       string
		message    string
		wantStatus string
	}{
		// ERROR cases
		{"FAIL_keyword", "FAIL: test_something failed", "ERROR"},
		{"ERROR_uppercase", "ERROR: something went wrong", "ERROR"},
		{"error_lowercase", "error: connection refused", "ERROR"},
		{"Error_mixed_case", "Error: unexpected EOF", "ERROR"},
		{"did_not_exit_successfully", "Command did not exit successfully", "ERROR"},

		// SUCCEEDED cases
		{"Phase_complete", "Phase complete: BUILD. Status: SUCCEEDED", "SUCCEEDED"},
		{"SUCCEEDED_keyword", "SUCCEEDED: all tests passed", "SUCCEEDED"},

		// IN_PROGRESS cases
		{"Entering_phase", "Entering phase BUILD", "IN_PROGRESS"},
		{"Running_command", "Running command echo hello", "IN_PROGRESS"},

		// Default -- empty status
		{"plain_message", "Just a regular log line", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mock := &mockCWLogsGetLogEventsClient{
				output: &cloudwatchlogs.GetLogEventsOutput{
					Events: []cwlogstypes.OutputLogEvent{
						{
							Timestamp: aws.Int64(1718445600000),
							Message:   aws.String(tc.message),
						},
					},
				},
			}

			result, err := awsclient.FetchCBBuildLogs(
				context.Background(),
				mock,
				"/aws/codebuild/status-test",
				"stream-status",
				"",
			)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			resources := result.Resources
			if len(resources) != 1 {
				t.Fatalf("expected 1 resource, got %d", len(resources))
			}
			if resources[0].Status != tc.wantStatus {
				t.Errorf("Status for %q: expected %q, got %q",
					tc.message, tc.wantStatus, resources[0].Status)
			}
		})
	}
}

// TestFetchCBBuildLogs_MessageStripping verifies that newlines in messages
// are stripped.
func TestFetchCBBuildLogs_MessageStripping(t *testing.T) {
	mock := &mockCWLogsGetLogEventsClient{
		output: &cloudwatchlogs.GetLogEventsOutput{
			Events: []cwlogstypes.OutputLogEvent{
				{
					Timestamp: aws.Int64(1718445600000),
					Message:   aws.String("line1\nline2\nline3"),
				},
			},
		},
	}

	result, err := awsclient.FetchCBBuildLogs(
		context.Background(),
		mock,
		"/aws/codebuild/strip-test",
		"stream-strip",
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

// TestFetchCBBuildLogs_RawStruct verifies that RawStruct is the original
// cwlogstypes.OutputLogEvent.
func TestFetchCBBuildLogs_RawStruct(t *testing.T) {
	mock := &mockCWLogsGetLogEventsClient{
		output: &cloudwatchlogs.GetLogEventsOutput{
			Events: []cwlogstypes.OutputLogEvent{
				{
					Timestamp:     aws.Int64(1718445600000),
					Message:       aws.String("test raw struct"),
					IngestionTime: aws.Int64(1718445601000),
				},
			},
		},
	}

	result, err := awsclient.FetchCBBuildLogs(
		context.Background(),
		mock,
		"/aws/codebuild/raw-test",
		"stream-raw",
		"",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	resources := result.Resources
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	if resources[0].RawStruct == nil {
		t.Fatal("RawStruct must not be nil")
	}
	raw, ok := resources[0].RawStruct.(cwlogstypes.OutputLogEvent)
	if !ok {
		t.Fatalf("RawStruct should be cwlogstypes.OutputLogEvent, got %T", resources[0].RawStruct)
	}
	if raw.Message == nil || *raw.Message != "test raw struct" {
		t.Error("RawStruct.Message not preserved correctly")
	}
}

// TestFetchCBBuildLogs_RegistrationExists verifies that "cb_build_logs" is
// registered as a child resource type.
func TestFetchCBBuildLogs_RegistrationExists(t *testing.T) {
	td := resource.GetChildType("cb_build_logs")
	if td == nil {
		t.Fatal("cb_build_logs child resource type not registered")
	}
	if td.ShortName != "cb_build_logs" {
		t.Errorf("child type ShortName: expected %q, got %q", "cb_build_logs", td.ShortName)
	}
	if td.Name == "" {
		t.Error("child type Name should not be empty")
	}
}

// ---------------------------------------------------------------------------
// Column definitions test
// ---------------------------------------------------------------------------

// TestCBBuildLogColumns verifies that CBBuildLogColumns returns columns with
// the expected keys.
func TestCBBuildLogColumns(t *testing.T) {
	cols := resource.CBBuildLogColumns()

	if len(cols) == 0 {
		t.Fatal("CBBuildLogColumns() returned no columns")
	}

	wantKeys := []string{"timestamp", "message"}
	for _, wantKey := range wantKeys {
		found := false
		for _, col := range cols {
			if col.Key == wantKey {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("CBBuildLogColumns() missing key %q", wantKey)
		}
	}
}

// TestCBBuildLogs_PaginatedChildFetcherRegistered verifies that the paginated
// child fetcher is registered under the correct short name.
func TestCBBuildLogs_PaginatedChildFetcherRegistered(t *testing.T) {
	f := resource.GetPaginatedChildFetcher("cb_build_logs")
	if f == nil {
		t.Fatal("cb_build_logs paginated child fetcher not registered")
	}
}

// TestCBBuildLogs_ParentHasChildDef verifies that the parent cb_builds child
// type has a child view definition for cb_build_logs.
func TestCBBuildLogs_ParentHasChildDef(t *testing.T) {
	td := resource.GetChildType("cb_builds")
	if td == nil {
		t.Fatal("cb_builds child type not found")
	}

	found := false
	for _, child := range td.Children {
		if child.ChildType == "cb_build_logs" {
			found = true
			if child.Key != "enter" {
				t.Errorf("child Key: expected %q, got %q", "enter", child.Key)
			}
			if child.ContextKeys["log_group_name"] != "log_group_name" {
				t.Errorf("ContextKeys[log_group_name]: expected %q, got %q",
					"log_group_name", child.ContextKeys["log_group_name"])
			}
			if child.ContextKeys["log_stream_name"] != "log_stream_name" {
				t.Errorf("ContextKeys[log_stream_name]: expected %q, got %q",
					"log_stream_name", child.ContextKeys["log_stream_name"])
			}
			break
		}
	}
	if !found {
		t.Error("cb_builds should have child view def for cb_build_logs")
	}
}

// ---------------------------------------------------------------------------
// Config defaults test
// ---------------------------------------------------------------------------

// TestConfigDefaultViewDef_CBBuildLogs verifies that the cb_build_logs view
// definition has the expected list columns and non-empty detail paths.
func TestConfigDefaultViewDef_CBBuildLogs(t *testing.T) {
	vd := config.DefaultViewDef("cb_build_logs")

	t.Run("list_columns", func(t *testing.T) {
		if len(vd.List) < 2 {
			t.Fatalf("expected at least 2 list columns for cb_build_logs default, got %d", len(vd.List))
		}
	})

	t.Run("detail_paths", func(t *testing.T) {
		if len(vd.Detail) == 0 {
			t.Error("expected non-empty Detail paths for cb_build_logs")
		}
	})
}

// Ensure all imports are used.
var _ = cloudwatchlogs.GetLogEventsOutput{}
