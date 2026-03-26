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
// Log Streams fetcher tests (child of Log Groups)
// ---------------------------------------------------------------------------

// TestFetchLogStreams_Basic verifies parsing of 3 log streams with varied data,
// checking ID, Name, Status, all Fields, and RawStruct.
func TestFetchLogStreams_Basic(t *testing.T) {
	mock := &mockCWLogsDescribeLogStreamsClient{
		outputs: []*cloudwatchlogs.DescribeLogStreamsOutput{
			{
				LogStreams: []cwlogstypes.LogStream{
					{
						LogStreamName:       aws.String("stream-alpha"),
						FirstEventTimestamp: aws.Int64(1711065600000), // 2024-03-22 00:00 UTC
						LastEventTimestamp:  aws.Int64(1711152000000), // 2024-03-23 00:00 UTC
						StoredBytes:         aws.Int64(14336),
					},
					{
						LogStreamName:       aws.String("stream-beta"),
						FirstEventTimestamp: aws.Int64(1711065600000),
						LastEventTimestamp:  aws.Int64(1711065600000),
						StoredBytes:         aws.Int64(2415919),
					},
					{
						LogStreamName:       aws.String("stream-gamma"),
						FirstEventTimestamp: aws.Int64(1711065600000),
						LastEventTimestamp:  aws.Int64(1711065600000),
						StoredBytes:         aws.Int64(0),
					},
				},
			},
		},
	}

	result, err := awsclient.FetchLogStreams(context.Background(), mock, "/aws/lambda/my-func", "",
)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

		resources := result.Resources

	if len(resources) != 3 {
		t.Fatalf("expected 3 resources, got %d", len(resources))
	}

	t.Run("stream_alpha_ID", func(t *testing.T) {
		if resources[0].ID != "stream-alpha" {
			t.Errorf("ID: expected %q, got %q", "stream-alpha", resources[0].ID)
		}
	})

	t.Run("stream_alpha_Name", func(t *testing.T) {
		if resources[0].Name != "stream-alpha" {
			t.Errorf("Name: expected %q, got %q", "stream-alpha", resources[0].Name)
		}
	})

	t.Run("stream_alpha_Status", func(t *testing.T) {
		if resources[0].Status != "" {
			t.Errorf("Status: expected empty string, got %q", resources[0].Status)
		}
	})

	t.Run("stream_alpha_fields", func(t *testing.T) {
		r := resources[0]
		if r.Fields["stream_name"] != "stream-alpha" {
			t.Errorf("Fields[stream_name]: expected %q, got %q", "stream-alpha", r.Fields["stream_name"])
		}
	})

	t.Run("stream_alpha_RawStruct", func(t *testing.T) {
		r := resources[0]
		if r.RawStruct == nil {
			t.Fatal("RawStruct must not be nil")
		}
		raw, ok := r.RawStruct.(cwlogstypes.LogStream)
		if !ok {
			t.Fatalf("RawStruct should be cwlogstypes.LogStream, got %T", r.RawStruct)
		}
		if raw.LogStreamName == nil || *raw.LogStreamName != "stream-alpha" {
			t.Errorf("RawStruct.LogStreamName: expected %q", "stream-alpha")
		}
	})

	t.Run("stream_beta_ID", func(t *testing.T) {
		if resources[1].ID != "stream-beta" {
			t.Errorf("ID: expected %q, got %q", "stream-beta", resources[1].ID)
		}
	})

	t.Run("stream_gamma_ID", func(t *testing.T) {
		if resources[2].ID != "stream-gamma" {
			t.Errorf("ID: expected %q, got %q", "stream-gamma", resources[2].ID)
		}
	})

	// Verify all streams have required fields
	t.Run("required_fields_present", func(t *testing.T) {
		requiredFields := []string{"stream_name", "last_event", "first_event"}
		for i, r := range resources {
			for _, key := range requiredFields {
				if _, ok := r.Fields[key]; !ok {
					t.Errorf("resource[%d].Fields missing key %q", i, key)
				}
			}
		}
	})
}

// TestFetchLogStreams_Empty verifies that an empty response returns an empty
// slice with no error.
func TestFetchLogStreams_Empty(t *testing.T) {
	mock := &mockCWLogsDescribeLogStreamsClient{
		outputs: []*cloudwatchlogs.DescribeLogStreamsOutput{
			{
				LogStreams: []cwlogstypes.LogStream{},
			},
		},
	}

	result, err := awsclient.FetchLogStreams(context.Background(), mock, "/aws/lambda/empty", "",
)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

		resources := result.Resources
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}

// TestFetchLogStreams_APIError verifies that API errors are propagated correctly.
func TestFetchLogStreams_APIError(t *testing.T) {
	mock := &mockCWLogsDescribeLogStreamsClient{
		err: fmt.Errorf("AWS API error: throttling exception"),
	}

	result, err := awsclient.FetchLogStreams(context.Background(), mock, "/aws/lambda/err", "",
)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

		resources := result.Resources
	if resources != nil {
		t.Errorf("expected nil resources on error, got %d", len(resources))
	}
}

// TestFetchLogStreams_Pagination verifies that paginated responses via NextToken
// are followed and all streams collected across pages.
func TestFetchLogStreams_Pagination(t *testing.T) {
	mock := &mockCWLogsDescribeLogStreamsClient{
		outputs: []*cloudwatchlogs.DescribeLogStreamsOutput{
			{
				NextToken: aws.String("page2-token"),
				LogStreams: []cwlogstypes.LogStream{
					{
						LogStreamName:       aws.String("page1-stream-1"),
						LastEventTimestamp:  aws.Int64(1711065600000),
						FirstEventTimestamp: aws.Int64(1711065600000),
						StoredBytes:         aws.Int64(100),
					},
					{
						LogStreamName:       aws.String("page1-stream-2"),
						LastEventTimestamp:  aws.Int64(1711065600000),
						FirstEventTimestamp: aws.Int64(1711065600000),
						StoredBytes:         aws.Int64(200),
					},
				},
			},
			{
				LogStreams: []cwlogstypes.LogStream{
					{
						LogStreamName:       aws.String("page2-stream-1"),
						LastEventTimestamp:  aws.Int64(1711065600000),
						FirstEventTimestamp: aws.Int64(1711065600000),
						StoredBytes:         aws.Int64(300),
					},
				},
			},
		},
	}

	result, err := awsclient.FetchLogStreams(context.Background(), mock, "/aws/lambda/paginated", "",
)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

		resources := result.Resources

	t.Run("total_count", func(t *testing.T) {
		if len(resources) != 3 {
			t.Fatalf("expected 3 resources across 2 pages, got %d", len(resources))
		}
	})

	t.Run("page1_first_stream", func(t *testing.T) {
		if resources[0].Name != "page1-stream-1" {
			t.Errorf("resources[0].Name: expected %q, got %q", "page1-stream-1", resources[0].Name)
		}
	})

	t.Run("page1_second_stream", func(t *testing.T) {
		if resources[1].Name != "page1-stream-2" {
			t.Errorf("resources[1].Name: expected %q, got %q", "page1-stream-2", resources[1].Name)
		}
	})

	t.Run("page2_stream", func(t *testing.T) {
		if resources[2].Name != "page2-stream-1" {
			t.Errorf("resources[2].Name: expected %q, got %q", "page2-stream-1", resources[2].Name)
		}
	})

	t.Run("api_called_twice", func(t *testing.T) {
		if mock.callIdx != 2 {
			t.Errorf("expected 2 API calls for pagination, got %d", mock.callIdx)
		}
	})
}

// TestFetchLogStreams_TimestampFormatting verifies that epoch milliseconds are
// correctly formatted into human-readable timestamps.
// 1711065600000 ms = 2024-03-22 00:00:00 UTC
func TestFetchLogStreams_TimestampFormatting(t *testing.T) {
	mock := &mockCWLogsDescribeLogStreamsClient{
		outputs: []*cloudwatchlogs.DescribeLogStreamsOutput{
			{
				LogStreams: []cwlogstypes.LogStream{
					{
						LogStreamName:       aws.String("ts-stream"),
						FirstEventTimestamp: aws.Int64(1711065600000),
						LastEventTimestamp:  aws.Int64(1711065600000),
						StoredBytes:         aws.Int64(0),
					},
				},
			},
		},
	}

	result, err := awsclient.FetchLogStreams(context.Background(), mock, "/aws/lambda/ts", "",
)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

		resources := result.Resources

	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	r := resources[0]

	t.Run("first_event_formatted", func(t *testing.T) {
		expected := "2024-03-22 00:00"
		if r.Fields["first_event"] != expected {
			t.Errorf("Fields[first_event]: expected %q, got %q", expected, r.Fields["first_event"])
		}
	})

	t.Run("last_event_formatted", func(t *testing.T) {
		expected := "2024-03-22 00:00"
		if r.Fields["last_event"] != expected {
			t.Errorf("Fields[last_event]: expected %q, got %q", expected, r.Fields["last_event"])
		}
	})
}

// TestFetchLogStreams_NilFields verifies that a stream with nil optional fields
// (LastEventTimestamp, FirstEventTimestamp, StoredBytes) does not panic and
// produces empty strings for those fields.
func TestFetchLogStreams_NilFields(t *testing.T) {
	mock := &mockCWLogsDescribeLogStreamsClient{
		outputs: []*cloudwatchlogs.DescribeLogStreamsOutput{
			{
				LogStreams: []cwlogstypes.LogStream{
					{
						LogStreamName: aws.String("nil-fields-stream"),
						// All other fields are nil
					},
				},
			},
		},
	}

	result, err := awsclient.FetchLogStreams(context.Background(), mock, "/aws/lambda/nil", "",
)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

		resources := result.Resources

	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	r := resources[0]

	t.Run("ID_set", func(t *testing.T) {
		if r.ID != "nil-fields-stream" {
			t.Errorf("ID: expected %q, got %q", "nil-fields-stream", r.ID)
		}
	})

	t.Run("last_event_empty", func(t *testing.T) {
		if r.Fields["last_event"] != "" {
			t.Errorf("Fields[last_event]: expected empty, got %q", r.Fields["last_event"])
		}
	})

	t.Run("first_event_empty", func(t *testing.T) {
		if r.Fields["first_event"] != "" {
			t.Errorf("Fields[first_event]: expected empty, got %q", r.Fields["first_event"])
		}
	})

}

// TestFetchLogStreams_RawStruct verifies that RawStruct is the original
// cwlogstypes.LogStream, preserving all SDK fields.
func TestFetchLogStreams_RawStruct(t *testing.T) {
	mock := &mockCWLogsDescribeLogStreamsClient{
		outputs: []*cloudwatchlogs.DescribeLogStreamsOutput{
			{
				LogStreams: []cwlogstypes.LogStream{
					{
						LogStreamName:       aws.String("raw-stream"),
						Arn:                 aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/test:log-stream:raw-stream"),
						FirstEventTimestamp: aws.Int64(1711065600000),
						LastEventTimestamp:  aws.Int64(1711152000000),
						StoredBytes:         aws.Int64(4096),
						CreationTime:        aws.Int64(1711000000000),
					},
				},
			},
		},
	}

	result, err := awsclient.FetchLogStreams(context.Background(), mock, "/test", "",
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

	raw, ok := r.RawStruct.(cwlogstypes.LogStream)
	if !ok {
		t.Fatalf("RawStruct should be cwlogstypes.LogStream, got %T", r.RawStruct)
	}

	t.Run("Arn_preserved", func(t *testing.T) {
		if raw.Arn == nil || *raw.Arn != "arn:aws:logs:us-east-1:123456789012:log-group:/test:log-stream:raw-stream" {
			t.Errorf("RawStruct.Arn not preserved correctly")
		}
	})

	t.Run("CreationTime_preserved", func(t *testing.T) {
		if raw.CreationTime == nil || *raw.CreationTime != 1711000000000 {
			t.Errorf("RawStruct.CreationTime not preserved correctly")
		}
	})

}

// TestLogStreamColumns verifies that LogStreamColumns returns the expected
// 3 columns with the correct keys: stream_name, last_event, first_event.
func TestLogStreamColumns(t *testing.T) {
	cols := resource.LogStreamColumns()

	expectedKeys := []string{"stream_name", "last_event", "first_event"}

	t.Run("column_count", func(t *testing.T) {
		if len(cols) != 3 {
			t.Fatalf("expected 3 columns, got %d", len(cols))
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

// TestFetchLogStreams_MaxResults verifies that the fetcher stops after a
// reasonable number of results instead of loading all streams from a log
// group with thousands of entries.
func TestFetchLogStreams_MaxResults(t *testing.T) {
	// Build a mock that returns 20 pages of 50 streams each (1000 total).
	// The fetcher should stop well before exhausting all pages.
	var outputs []*cloudwatchlogs.DescribeLogStreamsOutput
	for page := 0; page < 20; page++ {
		var streams []cwlogstypes.LogStream
		for i := 0; i < 50; i++ {
			streams = append(streams, cwlogstypes.LogStream{
				LogStreamName:      aws.String(fmt.Sprintf("stream-p%d-s%d", page, i)),
				LastEventTimestamp: aws.Int64(1711065600000),
			})
		}
		out := &cloudwatchlogs.DescribeLogStreamsOutput{
			LogStreams: streams,
		}
		if page < 19 {
			out.NextToken = aws.String(fmt.Sprintf("token-page-%d", page+1))
		}
		outputs = append(outputs, out)
	}

	mock := &mockCWLogsDescribeLogStreamsClient{outputs: outputs}
	result, err := awsclient.FetchLogStreams(context.Background(), mock, "/aws/lambda/huge-group", "",
)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

		resources := result.Resources

	// Should cap at a reasonable limit (e.g., 500) instead of loading all 1000
	if len(resources) > 500 {
		t.Errorf("expected <= 500 resources (capped), got %d — fetcher should limit pagination", len(resources))
	}

	// Should still return a meaningful number of results
	if len(resources) < 50 {
		t.Errorf("expected at least 50 resources, got %d", len(resources))
	}
}

// TestFetchLogStreams_ContinuationToken verifies that a non-empty
// continuation token is forwarded to the API as NextToken.
func TestFetchLogStreams_ContinuationToken(t *testing.T) {
	wrapper := &tokenCapturingLogStreamsMock{
		inner: &mockCWLogsDescribeLogStreamsClient{
			outputs: []*cloudwatchlogs.DescribeLogStreamsOutput{
				{
					LogStreams: []cwlogstypes.LogStream{
						{
							LogStreamName:      aws.String("stream-from-token"),
							LastEventTimestamp: aws.Int64(1711152000000),
						},
					},
				},
			},
		},
	}

	result, err := awsclient.FetchLogStreams(context.Background(), wrapper, "/aws/lambda/my-func", "my-continuation-token",
	)
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

// tokenCapturingLogStreamsMock wraps the log streams mock to capture NextToken.
type tokenCapturingLogStreamsMock struct {
	inner             *mockCWLogsDescribeLogStreamsClient
	capturedNextToken *string
}

func (m *tokenCapturingLogStreamsMock) DescribeLogStreams(ctx context.Context, params *cloudwatchlogs.DescribeLogStreamsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogStreamsOutput, error) {
	m.capturedNextToken = params.NextToken
	return m.inner.DescribeLogStreams(ctx, params, optFns...)
}
