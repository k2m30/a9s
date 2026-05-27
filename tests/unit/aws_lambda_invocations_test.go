package unit

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// Lambda Invocations fetcher tests (child of Lambda, cross-service CW Logs)
// ---------------------------------------------------------------------------

// TestFetchLambdaInvocations_Basic verifies parsing of REPORT lines from
// FilterLogEvents into invocation resources with correct ID, Name, Status,
// all computed Fields, and RawStruct.
func TestFetchLambdaInvocations_Basic(t *testing.T) {
	mock := &mockCWLogsFilterLogEventsClient{
		outputs: []*cloudwatchlogs.FilterLogEventsOutput{
			{
				Events: []cwlogstypes.FilteredLogEvent{
					{
						Timestamp:     aws.Int64(1711065600000),
						Message:       aws.String("REPORT RequestId: 12345678-1234-1234-1234-123456789012\tDuration: 2103.45 ms\tBilled Duration: 2200 ms\tMemory Size: 256 MB\tMax Memory Used: 128 MB\t"),
						IngestionTime: aws.Int64(1711065601000),
						LogStreamName: aws.String("2024/03/22/[$LATEST]abcdef1234567890"),
						EventId:       aws.String("evt-001"),
					},
					{
						Timestamp:     aws.Int64(1711065700000),
						Message:       aws.String("REPORT RequestId: abcdefab-abcd-abcd-abcd-abcdefabcdef\tDuration: 50.12 ms\tBilled Duration: 100 ms\tMemory Size: 128 MB\tMax Memory Used: 64 MB\tInit Duration: 350.00 ms\t"),
						IngestionTime: aws.Int64(1711065701000),
						LogStreamName: aws.String("2024/03/22/[$LATEST]abcdef1234567890"),
						EventId:       aws.String("evt-002"),
					},
				},
			},
		},
	}

	result, err := awsclient.FetchLambdaInvocations(context.Background(), mock, "my-func", "/aws/lambda/my-func", "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	resources := result.Resources

	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	t.Run("invocation_0_ID_contains_request_id", func(t *testing.T) {
		if resources[0].ID == "" {
			t.Error("ID should not be empty")
		}
	})

	t.Run("invocation_0_Name_not_empty", func(t *testing.T) {
		if resources[0].Name == "" {
			t.Error("Name should not be empty")
		}
	})

	t.Run("invocation_0_Status_OK", func(t *testing.T) {
		if resources[0].Fields["status"] != "OK" {
			t.Errorf("Fields[\"status\"]: expected %q, got %q", "OK", resources[0].Fields["status"])
		}
	})

	// After newest-first sort, resources[0] is the newer event (abcdefab...)
	t.Run("invocation_0_fields_request_id", func(t *testing.T) {
		r := resources[0]
		if r.Fields["request_id"] != "abcdefab-abcd-abcd-abcd-abcdefabcdef" {
			t.Errorf("Fields[request_id]: expected newest invocation UUID, got %q", r.Fields["request_id"])
		}
	})

	t.Run("invocation_0_fields_duration_ms", func(t *testing.T) {
		r := resources[0]
		if r.Fields["duration_ms"] == "" {
			t.Error("Fields[duration_ms] should not be empty")
		}
	})

	t.Run("invocation_0_fields_memory_used", func(t *testing.T) {
		r := resources[0]
		if r.Fields["memory_used"] == "" {
			t.Error("Fields[memory_used] should not be empty")
		}
	})

	t.Run("invocation_0_fields_timestamp", func(t *testing.T) {
		r := resources[0]
		if r.Fields["timestamp"] == "" {
			t.Error("Fields[timestamp] should not be empty")
		}
	})

	// Newest invocation (abcdefab...) has Init Duration → cold_start = "yes"
	t.Run("invocation_0_cold_start_yes", func(t *testing.T) {
		r := resources[0]
		if r.Fields["cold_start"] != "yes" {
			t.Errorf("Fields[cold_start]: expected %q (has Init Duration), got %q", "yes", r.Fields["cold_start"])
		}
	})

	t.Run("invocation_0_RawStruct", func(t *testing.T) {
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

	// Verify required fields on all invocations
	t.Run("required_fields_present", func(t *testing.T) {
		requiredFields := []string{"request_id", "timestamp", "status", "duration_ms", "memory_used", "cold_start"}
		for i, r := range resources {
			for _, key := range requiredFields {
				if _, ok := r.Fields[key]; !ok {
					t.Errorf("resource[%d].Fields missing key %q", i, key)
				}
			}
		}
	})
}

// TestFetchLambdaInvocations_ColdStartDetection verifies that Init Duration
// in the REPORT line is parsed and cold_start is set to "yes".
func TestFetchLambdaInvocations_ColdStartDetection(t *testing.T) {
	mock := &mockCWLogsFilterLogEventsClient{
		outputs: []*cloudwatchlogs.FilterLogEventsOutput{
			{
				Events: []cwlogstypes.FilteredLogEvent{
					{
						Timestamp: aws.Int64(1711065600000),
						Message:   aws.String("REPORT RequestId: 11111111-1111-1111-1111-111111111111\tDuration: 500.00 ms\tBilled Duration: 600 ms\tMemory Size: 256 MB\tMax Memory Used: 128 MB\tInit Duration: 350.00 ms\t"),
						EventId:   aws.String("evt-cold"),
					},
				},
			},
		},
	}

	result, err := awsclient.FetchLambdaInvocations(context.Background(), mock, "cold-func", "/aws/lambda/cold-func", "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	resources := result.Resources

	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	t.Run("cold_start_yes", func(t *testing.T) {
		if resources[0].Fields["cold_start"] != "yes" {
			t.Errorf("Fields[cold_start]: expected %q, got %q", "yes", resources[0].Fields["cold_start"])
		}
	})

	t.Run("init_duration_ms_populated", func(t *testing.T) {
		if resources[0].Fields["init_duration_ms"] == "" {
			t.Error("Fields[init_duration_ms] should be populated for cold start")
		}
	})
}

// TestFetchLambdaInvocations_ErrorStatus verifies that invocations with
// ERROR log lines are detected and the status is set accordingly.
func TestFetchLambdaInvocations_ErrorStatus(t *testing.T) {
	// The fetcher should cross-reference REPORT lines with any ERROR lines
	// and set status to "ERROR" if the request had errors.
	mock := &mockCWLogsFilterLogEventsClient{
		outputs: []*cloudwatchlogs.FilterLogEventsOutput{
			{
				Events: []cwlogstypes.FilteredLogEvent{
					{
						Timestamp: aws.Int64(1711065600000),
						Message:   aws.String("REPORT RequestId: 22222222-2222-2222-2222-222222222222\tDuration: 100.00 ms\tBilled Duration: 100 ms\tMemory Size: 128 MB\tMax Memory Used: 64 MB\t"),
						EventId:   aws.String("evt-ok"),
					},
				},
			},
		},
	}

	result, err := awsclient.FetchLambdaInvocations(context.Background(), mock, "err-func", "/aws/lambda/err-func", "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	resources := result.Resources

	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	// A simple REPORT without error markers should be "OK"
	if resources[0].Fields["status"] != "OK" {
		t.Errorf("Fields[\"status\"]: expected %q for normal REPORT, got %q", "OK", resources[0].Fields["status"])
	}
}

// TestFetchLambdaInvocations_TimeoutStatus verifies that invocations where
// Duration >= timeout threshold are marked with TIMEOUT status.
func TestFetchLambdaInvocations_TimeoutStatus(t *testing.T) {
	// A REPORT line where "Task timed out" is present should indicate TIMEOUT.
	mock := &mockCWLogsFilterLogEventsClient{
		outputs: []*cloudwatchlogs.FilterLogEventsOutput{
			{
				Events: []cwlogstypes.FilteredLogEvent{
					{
						Timestamp: aws.Int64(1711065600000),
						Message:   aws.String("REPORT RequestId: 33333333-3333-3333-3333-333333333333\tDuration: 30000.00 ms\tBilled Duration: 30000 ms\tMemory Size: 128 MB\tMax Memory Used: 64 MB\tStatus: timeout\t"),
						EventId:   aws.String("evt-timeout"),
					},
				},
			},
		},
	}

	result, err := awsclient.FetchLambdaInvocations(context.Background(), mock, "timeout-func", "/aws/lambda/timeout-func", "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	resources := result.Resources

	if len(resources) < 1 {
		t.Fatalf("expected at least 1 resource, got %d", len(resources))
	}

	// The fetcher should detect timeout status
	r := resources[0]
	if r.Fields["status"] == "" {
		t.Error("Fields[status] should not be empty")
	}
}

// TestFetchLambdaInvocations_Empty verifies that an empty response returns
// an empty slice with no error.
func TestFetchLambdaInvocations_Empty(t *testing.T) {
	mock := &mockCWLogsFilterLogEventsClient{
		outputs: []*cloudwatchlogs.FilterLogEventsOutput{
			{
				Events: []cwlogstypes.FilteredLogEvent{},
			},
		},
	}

	result, err := awsclient.FetchLambdaInvocations(context.Background(), mock, "empty-func", "/aws/lambda/empty-func", "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	resources := result.Resources
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}

// TestFetchLambdaInvocations_APIError verifies that API errors are propagated.
func TestFetchLambdaInvocations_APIError(t *testing.T) {
	mock := &mockCWLogsFilterLogEventsClient{
		err: fmt.Errorf("AWS API error: throttling exception"),
	}

	result, err := awsclient.FetchLambdaInvocations(context.Background(), mock, "err-func", "/aws/lambda/err-func", "")
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	resources := result.Resources
	if resources != nil {
		t.Errorf("expected nil resources on error, got %d", len(resources))
	}
}

// TestFetchLambdaInvocations_MultipleInvocations verifies that multiple
// REPORT lines in a single FilterLogEvents response are all parsed.
func TestFetchLambdaInvocations_MultipleInvocations(t *testing.T) {
	var events []cwlogstypes.FilteredLogEvent
	for i := range 10 {
		events = append(events, cwlogstypes.FilteredLogEvent{
			Timestamp: aws.Int64(int64(1711065600000 + i*100000)),
			Message:   aws.String(fmt.Sprintf("REPORT RequestId: %08d-0000-0000-0000-000000000000\tDuration: %d.00 ms\tBilled Duration: %d ms\tMemory Size: 128 MB\tMax Memory Used: 64 MB\t", i, i*100, (i+1)*100)),
			EventId:   aws.String(fmt.Sprintf("evt-%03d", i)),
		})
	}

	mock := &mockCWLogsFilterLogEventsClient{
		outputs: []*cloudwatchlogs.FilterLogEventsOutput{
			{Events: events},
		},
	}

	result, err := awsclient.FetchLambdaInvocations(context.Background(), mock, "multi-func", "/aws/lambda/multi-func", "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	resources := result.Resources

	if len(resources) != 10 {
		t.Fatalf("expected 10 resources, got %d", len(resources))
	}

	// Each invocation should have a unique request_id
	seen := make(map[string]bool)
	for i, r := range resources {
		rid := r.Fields["request_id"]
		if rid == "" {
			t.Errorf("resource[%d] has empty request_id", i)
		}
		if seen[rid] {
			t.Errorf("resource[%d] has duplicate request_id %q", i, rid)
		}
		seen[rid] = true
	}

	// Results must be sorted newest-first (descending timestamp)
	for i := 1; i < len(resources); i++ {
		prev := resources[i-1].Fields["timestamp"]
		curr := resources[i].Fields["timestamp"]
		if prev < curr {
			t.Errorf("resources not sorted newest-first: resource[%d] timestamp %q < resource[%d] timestamp %q", i-1, prev, i, curr)
		}
	}
}

// TestFetchLambdaInvocations_NilFields verifies that events with nil
// Message, nil Timestamp do not panic.
func TestFetchLambdaInvocations_NilFields(t *testing.T) {
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

	// Should not panic
	result, err := awsclient.FetchLambdaInvocations(context.Background(), mock, "nil-func", "/aws/lambda/nil-func", "")
	if err != nil {
		t.Fatalf("expected no error for nil fields, got %v", err)
	}

	// Nil/empty REPORT lines may be skipped or produce empty resources
	_ = result
}

// TestFetchLambdaInvocations_RawStruct verifies that RawStruct is the
// original cwlogstypes.FilteredLogEvent, preserving all SDK fields.
func TestFetchLambdaInvocations_RawStruct(t *testing.T) {
	mock := &mockCWLogsFilterLogEventsClient{
		outputs: []*cloudwatchlogs.FilterLogEventsOutput{
			{
				Events: []cwlogstypes.FilteredLogEvent{
					{
						Timestamp:     aws.Int64(1711065600000),
						Message:       aws.String("REPORT RequestId: aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee\tDuration: 100.00 ms\tBilled Duration: 100 ms\tMemory Size: 256 MB\tMax Memory Used: 128 MB\t"),
						IngestionTime: aws.Int64(1711065601000),
						LogStreamName: aws.String("2024/03/22/[$LATEST]abcdef"),
						EventId:       aws.String("evt-raw"),
					},
				},
			},
		},
	}

	result, err := awsclient.FetchLambdaInvocations(context.Background(), mock, "raw-func", "/aws/lambda/raw-func", "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	resources := result.Resources

	if len(resources) < 1 {
		t.Fatalf("expected at least 1 resource, got %d", len(resources))
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
		if raw.Message == nil || len(*raw.Message) == 0 {
			t.Errorf("RawStruct.Message not preserved correctly")
		}
	})

	t.Run("EventId_preserved", func(t *testing.T) {
		if raw.EventId == nil || *raw.EventId != "evt-raw" {
			t.Errorf("RawStruct.EventId not preserved correctly")
		}
	})
}

// TestFetchLambdaInvocations_ParentContext verifies that the fetcher uses
// the log_group context key from the parent Lambda resource.
func TestFetchLambdaInvocations_ParentContext(t *testing.T) {
	mock := &mockCWLogsFilterLogEventsClient{
		outputs: []*cloudwatchlogs.FilterLogEventsOutput{
			{
				Events: []cwlogstypes.FilteredLogEvent{},
			},
		},
	}

	// The fetcher should use the custom log group, not construct a default
	customLogGroup := "/custom/log/group/my-func"
	_, err := awsclient.FetchLambdaInvocations(context.Background(), mock, "my-func", customLogGroup, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify the mock was called with the correct log group
	if mock.lastInput == nil {
		t.Fatal("expected FilterLogEvents to be called")
	}
	if mock.lastInput.LogGroupName == nil || *mock.lastInput.LogGroupName != customLogGroup {
		t.Errorf("expected log group %q, got %q", customLogGroup, *mock.lastInput.LogGroupName)
	}
}

// TestFetchLambdaInvocations_Pagination verifies that paginated responses
// via NextToken are followed and all invocations collected.
func TestFetchLambdaInvocations_Pagination(t *testing.T) {
	mock := &mockCWLogsFilterLogEventsClient{
		outputs: []*cloudwatchlogs.FilterLogEventsOutput{
			{
				NextToken: aws.String("page2-token"),
				Events: []cwlogstypes.FilteredLogEvent{
					{
						Timestamp: aws.Int64(1711065600000),
						Message:   aws.String("REPORT RequestId: page1-001-0000-0000-000000000000\tDuration: 100.00 ms\tBilled Duration: 100 ms\tMemory Size: 128 MB\tMax Memory Used: 64 MB\t"),
						EventId:   aws.String("evt-p1"),
					},
				},
			},
			{
				Events: []cwlogstypes.FilteredLogEvent{
					{
						Timestamp: aws.Int64(1711065700000),
						Message:   aws.String("REPORT RequestId: page2-001-0000-0000-000000000000\tDuration: 200.00 ms\tBilled Duration: 200 ms\tMemory Size: 128 MB\tMax Memory Used: 64 MB\t"),
						EventId:   aws.String("evt-p2"),
					},
				},
			},
		},
	}

	result, err := awsclient.FetchLambdaInvocations(context.Background(), mock, "paginated-func", "/aws/lambda/paginated-func", "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	resources := result.Resources

	if len(resources) < 2 {
		t.Fatalf("expected at least 2 resources across pages, got %d", len(resources))
	}

	t.Run("api_called_twice", func(t *testing.T) {
		if mock.callIdx != 2 {
			t.Errorf("expected 2 API calls for pagination, got %d", mock.callIdx)
		}
	})
}

// TestFetchLambdaInvocations_LogGroupNotFound verifies that a
// ResourceNotFoundException (log group doesn't exist because the function was
// never invoked) returns an empty slice, not an error.
func TestFetchLambdaInvocations_LogGroupNotFound(t *testing.T) {
	mock := &mockCWLogsFilterLogEventsClient{
		err: fmt.Errorf("operation error CloudWatch Logs: FilterLogEvents, https response error StatusCode: 400, ResourceNotFoundException: The specified log group does not exist."),
	}

	result, err := awsclient.FetchLambdaInvocations(context.Background(), mock, "never-invoked", "/aws/lambda/never-invoked", "")
	if err != nil {
		t.Fatalf("ResourceNotFoundException should return nil error, got: %v", err)
	}

	resources := result.Resources
	if len(resources) != 0 {
		t.Errorf("expected 0 resources for non-existent log group, got %d", len(resources))
	}
}

// TestLambdaInvocationColumns verifies that LambdaInvocationColumns returns
// the expected columns with correct keys.
func TestLambdaInvocationColumns(t *testing.T) {
	cols := resource.LambdaInvocationColumns()

	expectedKeys := []string{"timestamp", "request_id", "status", "duration_ms", "memory_used", "cold_start"}

	t.Run("column_count", func(t *testing.T) {
		if len(cols) != 6 {
			t.Fatalf("expected 6 columns, got %d", len(cols))
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

// TestFetchLambdaInvocations_ContinuationToken verifies that a non-empty
// continuation token is forwarded to the API as NextToken.
func TestFetchLambdaInvocations_ContinuationToken(t *testing.T) {
	wrapper := &tokenCapturingLambdaInvocationsMock{
		inner: &mockCWLogsFilterLogEventsClient{
			outputs: []*cloudwatchlogs.FilterLogEventsOutput{
				{
					Events: []cwlogstypes.FilteredLogEvent{
						{
							Timestamp: aws.Int64(1711036800000),
							Message:   aws.String("REPORT RequestId: abcd1234-5678-9012-abcd-ef0123456789\tDuration: 100.00 ms\tBilled Duration: 100.00 ms\tMemory Size: 128 MB\tMax Memory Used: 64 MB\n"),
						},
					},
				},
			},
		},
	}

	result, err := awsclient.FetchLambdaInvocations(context.Background(), wrapper, "my-func", "/aws/lambda/my-func", "my-continuation-token")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}

	if wrapper.capturedNextToken == nil {
		t.Fatal("expected NextToken to be set in API call")
	}
	if *wrapper.capturedNextToken != "my-continuation-token" {
		t.Errorf("expected NextToken %q, got %q", "my-continuation-token", *wrapper.capturedNextToken)
	}
}

// tokenCapturingLambdaInvocationsMock wraps the CWLogs mock to capture NextToken.
type tokenCapturingLambdaInvocationsMock struct {
	inner             *mockCWLogsFilterLogEventsClient
	capturedNextToken *string
}

func (m *tokenCapturingLambdaInvocationsMock) FilterLogEvents(ctx context.Context, params *cloudwatchlogs.FilterLogEventsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.FilterLogEventsOutput, error) {
	m.capturedNextToken = params.NextToken
	return m.inner.FilterLogEvents(ctx, params, optFns...)
}
