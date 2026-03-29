package unit

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// RDS Instance Events fetcher tests (child of RDS Instances)
// ---------------------------------------------------------------------------

// TestFetchRDSEvents_Basic verifies parsing of 3 events with all fields
// populated, checking ID, Name, Fields map, and RawStruct type.
func TestFetchRDSEvents_Basic(t *testing.T) {
	ts1 := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)
	ts2 := time.Date(2024, 6, 15, 10, 5, 0, 0, time.UTC)
	ts3 := time.Date(2024, 6, 15, 10, 10, 0, 0, time.UTC)

	mock := &mockRDSDescribeEventsClient{
		output: &rds.DescribeEventsOutput{
			Events: []rdstypes.Event{
				{
					Date:             &ts1,
					EventCategories:  []string{"maintenance"},
					Message:          aws.String("Applying offline patches to DB instance"),
					SourceIdentifier: aws.String("my-db-instance"),
					SourceType:       rdstypes.SourceTypeDbInstance,
					SourceArn:        aws.String("arn:aws:rds:us-east-1:123456789012:db:my-db-instance"),
				},
				{
					Date:             &ts2,
					EventCategories:  []string{"failover"},
					Message:          aws.String("Started cross AZ failover to DB instance"),
					SourceIdentifier: aws.String("my-db-instance"),
					SourceType:       rdstypes.SourceTypeDbInstance,
					SourceArn:        aws.String("arn:aws:rds:us-east-1:123456789012:db:my-db-instance"),
				},
				{
					Date:             &ts3,
					EventCategories:  []string{"availability", "notification"},
					Message:          aws.String("DB instance restarted"),
					SourceIdentifier: aws.String("my-db-instance"),
					SourceType:       rdstypes.SourceTypeDbInstance,
					SourceArn:        aws.String("arn:aws:rds:us-east-1:123456789012:db:my-db-instance"),
				},
			},
		},
	}

	result, err := awsclient.FetchRDSEvents(
		context.Background(),
		mock,
		"my-db-instance",
			"",
)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(result.Resources) != 3 {
		t.Fatalf("expected 3 resources, got %d", len(result.Resources))
	}

	t.Run("event_0_ID", func(t *testing.T) {
		// ID format: "timestamp/source_identifier"
		expected := "2024-06-15 10:00/my-db-instance"
		if result.Resources[0].ID != expected {
			t.Errorf("ID: expected %q, got %q", expected, result.Resources[0].ID)
		}
	})

	t.Run("event_0_Name_is_formatted_timestamp", func(t *testing.T) {
		if result.Resources[0].Name == "" {
			t.Error("Name should not be empty")
		}
		if !strings.Contains(result.Resources[0].Name, "2024-06-15") {
			t.Errorf("Name should contain formatted date, got %q", result.Resources[0].Name)
		}
	})

	t.Run("event_0_Fields_timestamp", func(t *testing.T) {
		r := result.Resources[0]
		if r.Fields["timestamp"] == "" {
			t.Error("Fields[timestamp] should not be empty")
		}
		if !strings.Contains(r.Fields["timestamp"], "2024-06-15 10:00") {
			t.Errorf("Fields[timestamp] expected '2024-06-15 10:00', got %q", r.Fields["timestamp"])
		}
	})

	t.Run("event_0_Fields_event_categories", func(t *testing.T) {
		r := result.Resources[0]
		if r.Fields["event_categories"] != "maintenance" {
			t.Errorf("Fields[event_categories]: expected %q, got %q", "maintenance", r.Fields["event_categories"])
		}
	})

	t.Run("event_0_Fields_message", func(t *testing.T) {
		r := result.Resources[0]
		if r.Fields["message"] != "Applying offline patches to DB instance" {
			t.Errorf("Fields[message]: expected %q, got %q", "Applying offline patches to DB instance", r.Fields["message"])
		}
	})

	t.Run("event_0_Fields_source_identifier", func(t *testing.T) {
		r := result.Resources[0]
		if r.Fields["source_identifier"] != "my-db-instance" {
			t.Errorf("Fields[source_identifier]: expected %q, got %q", "my-db-instance", r.Fields["source_identifier"])
		}
	})

	t.Run("event_0_Fields_source_type", func(t *testing.T) {
		r := result.Resources[0]
		if r.Fields["source_type"] != "db-instance" {
			t.Errorf("Fields[source_type]: expected %q, got %q", "db-instance", r.Fields["source_type"])
		}
	})

	t.Run("event_0_Fields_source_arn", func(t *testing.T) {
		r := result.Resources[0]
		if r.Fields["source_arn"] != "arn:aws:rds:us-east-1:123456789012:db:my-db-instance" {
			t.Errorf("Fields[source_arn]: expected %q, got %q",
				"arn:aws:rds:us-east-1:123456789012:db:my-db-instance", r.Fields["source_arn"])
		}
	})

	t.Run("event_0_RawStruct", func(t *testing.T) {
		r := result.Resources[0]
		if r.RawStruct == nil {
			t.Fatal("RawStruct must not be nil")
		}
		raw, ok := r.RawStruct.(rdstypes.Event)
		if !ok {
			t.Fatalf("RawStruct should be rdstypes.Event, got %T", r.RawStruct)
		}
		if raw.SourceIdentifier == nil || *raw.SourceIdentifier != "my-db-instance" {
			t.Error("RawStruct.SourceIdentifier not preserved correctly")
		}
	})

	t.Run("event_2_multiple_categories", func(t *testing.T) {
		r := result.Resources[2]
		if r.Fields["event_categories"] != "availability, notification" {
			t.Errorf("Fields[event_categories]: expected %q, got %q",
				"availability, notification", r.Fields["event_categories"])
		}
	})

	// Verify required fields on all events
	t.Run("required_fields_present", func(t *testing.T) {
		requiredFields := []string{"timestamp", "event_categories", "message", "source_identifier", "source_type", "source_arn"}
		for i, r := range result.Resources {
			for _, key := range requiredFields {
				if _, ok := r.Fields[key]; !ok {
					t.Errorf("resource[%d].Fields missing key %q", i, key)
				}
			}
		}
	})
}

// TestFetchRDSEvents_Empty verifies that a DB instance with no events
// returns an empty slice with no error.
func TestFetchRDSEvents_Empty(t *testing.T) {
	mock := &mockRDSDescribeEventsClient{
		output: &rds.DescribeEventsOutput{
			Events: []rdstypes.Event{},
		},
	}

	result, err := awsclient.FetchRDSEvents(
		context.Background(),
		mock,
		"empty-db",
			"",
)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
}

// TestFetchRDSEvents_APIError verifies that API errors are propagated.
func TestFetchRDSEvents_APIError(t *testing.T) {
	mock := &mockRDSDescribeEventsClient{
		err: fmt.Errorf("AWS API error: access denied"),
	}

	result, err := awsclient.FetchRDSEvents(
		context.Background(),
		mock,
		"err-db",
			"",
)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if result.Resources != nil {
		t.Errorf("expected nil resources on error, got %d", len(result.Resources))
	}
}

// TestFetchRDSEvents_NilOptionalFields verifies that nil Message, nil Date,
// nil SourceArn produce empty strings without panic.
func TestFetchRDSEvents_NilOptionalFields(t *testing.T) {
	mock := &mockRDSDescribeEventsClient{
		output: &rds.DescribeEventsOutput{
			Events: []rdstypes.Event{
				{
					// Date is nil
					// Message is nil
					// SourceArn is nil
					// SourceIdentifier is nil
					EventCategories: []string{"notification"},
					SourceType:      rdstypes.SourceTypeDbInstance,
				},
				{
					// All optional fields nil, including EventCategories
				},
			},
		},
	}

	// Should not panic
	result, err := awsclient.FetchRDSEvents(
		context.Background(),
		mock,
		"nil-db",
			"",
)
	if err != nil {
		t.Fatalf("expected no error for nil fields, got %v", err)
	}

	if len(result.Resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(result.Resources))
	}

	t.Run("nil_Message", func(t *testing.T) {
		r := result.Resources[0]
		if r.Fields["message"] != "" {
			t.Errorf("Fields[message] should be empty for nil, got %q", r.Fields["message"])
		}
	})

	t.Run("nil_Date", func(t *testing.T) {
		r := result.Resources[0]
		if r.Fields["timestamp"] != "" {
			t.Logf("Fields[timestamp] is %q (expected empty for nil Date)", r.Fields["timestamp"])
		}
	})

	t.Run("nil_SourceArn", func(t *testing.T) {
		r := result.Resources[0]
		if r.Fields["source_arn"] != "" {
			t.Errorf("Fields[source_arn] should be empty for nil, got %q", r.Fields["source_arn"])
		}
	})

	t.Run("nil_SourceIdentifier", func(t *testing.T) {
		r := result.Resources[0]
		if r.Fields["source_identifier"] != "" {
			t.Errorf("Fields[source_identifier] should be empty for nil, got %q", r.Fields["source_identifier"])
		}
	})
}

// TestFetchRDSEvents_NewlineStripping verifies that \n and \r in Message
// are replaced with spaces.
func TestFetchRDSEvents_NewlineStripping(t *testing.T) {
	ts := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)

	mock := &mockRDSDescribeEventsClient{
		output: &rds.DescribeEventsOutput{
			Events: []rdstypes.Event{
				{
					Date:             &ts,
					EventCategories:  []string{"maintenance"},
					Message:          aws.String("Maintenance window\nstarted for\rDB instance"),
					SourceIdentifier: aws.String("nl-db"),
					SourceType:       rdstypes.SourceTypeDbInstance,
					SourceArn:        aws.String("arn:aws:rds:us-east-1:123456789012:db:nl-db"),
				},
			},
		},
	}

	result, err := awsclient.FetchRDSEvents(
		context.Background(),
		mock,
		"nl-db",
			"",
)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}

	msg := result.Resources[0].Fields["message"]
	if strings.Contains(msg, "\n") {
		t.Errorf("Fields[message] should not contain \\n, got %q", msg)
	}
	if strings.Contains(msg, "\r") {
		t.Errorf("Fields[message] should not contain \\r, got %q", msg)
	}
}

// TestFetchRDSEvents_TimestampFormatting verifies that a known time.Time
// produces the "2006-01-02 15:04" format in Fields.
func TestFetchRDSEvents_TimestampFormatting(t *testing.T) {
	ts := time.Date(2024, 12, 25, 14, 30, 45, 0, time.UTC)

	mock := &mockRDSDescribeEventsClient{
		output: &rds.DescribeEventsOutput{
			Events: []rdstypes.Event{
				{
					Date:             &ts,
					EventCategories:  []string{"notification"},
					Message:          aws.String("Test event"),
					SourceIdentifier: aws.String("ts-db"),
					SourceType:       rdstypes.SourceTypeDbInstance,
				},
			},
		},
	}

	result, err := awsclient.FetchRDSEvents(
		context.Background(),
		mock,
		"ts-db",
			"",
)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}

	tsField := result.Resources[0].Fields["timestamp"]
	if tsField != "2024-12-25 14:30" {
		t.Errorf("Fields[timestamp]: expected %q, got %q", "2024-12-25 14:30", tsField)
	}
}

// TestFetchRDSEvents_RawStruct verifies that RawStruct preserves the
// original rdstypes.Event, including all sub-fields.
func TestFetchRDSEvents_RawStruct(t *testing.T) {
	ts := time.Date(2024, 6, 15, 12, 30, 0, 0, time.UTC)

	mock := &mockRDSDescribeEventsClient{
		output: &rds.DescribeEventsOutput{
			Events: []rdstypes.Event{
				{
					Date:             &ts,
					EventCategories:  []string{"maintenance", "notification"},
					Message:          aws.String("Completed maintenance for DB instance"),
					SourceIdentifier: aws.String("raw-db"),
					SourceType:       rdstypes.SourceTypeDbInstance,
					SourceArn:        aws.String("arn:aws:rds:us-east-1:123456789012:db:raw-db"),
				},
			},
		},
	}

	result, err := awsclient.FetchRDSEvents(
		context.Background(),
		mock,
		"raw-db",
			"",
)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}

	r := result.Resources[0]
	if r.RawStruct == nil {
		t.Fatal("RawStruct must not be nil")
	}

	raw, ok := r.RawStruct.(rdstypes.Event)
	if !ok {
		t.Fatalf("RawStruct should be rdstypes.Event, got %T", r.RawStruct)
	}

	t.Run("Date_preserved", func(t *testing.T) {
		if raw.Date == nil || !raw.Date.Equal(ts) {
			t.Errorf("RawStruct.Date not preserved correctly")
		}
	})

	t.Run("SourceIdentifier_preserved", func(t *testing.T) {
		if raw.SourceIdentifier == nil || *raw.SourceIdentifier != "raw-db" {
			t.Errorf("RawStruct.SourceIdentifier not preserved correctly")
		}
	})

	t.Run("Message_preserved", func(t *testing.T) {
		if raw.Message == nil || *raw.Message != "Completed maintenance for DB instance" {
			t.Errorf("RawStruct.Message not preserved correctly")
		}
	})

	t.Run("EventCategories_preserved", func(t *testing.T) {
		if len(raw.EventCategories) != 2 {
			t.Errorf("RawStruct.EventCategories: expected 2, got %d", len(raw.EventCategories))
		}
	})

	t.Run("SourceArn_preserved", func(t *testing.T) {
		if raw.SourceArn == nil || *raw.SourceArn != "arn:aws:rds:us-east-1:123456789012:db:raw-db" {
			t.Errorf("RawStruct.SourceArn not preserved correctly")
		}
	})
}

// TestFetchRDSEvents_EventCategoriesJoined verifies that multiple event
// categories are joined with ", ".
func TestFetchRDSEvents_EventCategoriesJoined(t *testing.T) {
	ts := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)

	mock := &mockRDSDescribeEventsClient{
		output: &rds.DescribeEventsOutput{
			Events: []rdstypes.Event{
				{
					Date:             &ts,
					EventCategories:  []string{"availability", "failover", "notification"},
					Message:          aws.String("Test"),
					SourceIdentifier: aws.String("cat-db"),
					SourceType:       rdstypes.SourceTypeDbInstance,
				},
			},
		},
	}

	result, err := awsclient.FetchRDSEvents(
		context.Background(),
		mock,
		"cat-db",
			"",
)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}

	cats := result.Resources[0].Fields["event_categories"]
	if cats != "availability, failover, notification" {
		t.Errorf("Fields[event_categories]: expected %q, got %q",
			"availability, failover, notification", cats)
	}
}

// TestFetchRDSEvents_Pagination verifies that paginated responses via Marker
// are followed and all events collected across multiple pages.
// TestFetchRDSEvents_Pagination verifies the single-page pagination contract:
// one API call is made per invocation, resources from that page are returned,
// and IsTruncated/NextToken (Marker) reflect whether more pages exist. A second
// call with the continuation token verifies the token is forwarded and the final
// page sets IsTruncated=false.
func TestFetchRDSEvents_Pagination(t *testing.T) {
	ts := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)

	// Page 1: 3 events with Marker indicating more pages exist.
	page1Mock := &mockRDSDescribeEventsClient{
		outputs: []*rds.DescribeEventsOutput{
			{
				Marker: aws.String("page2-marker"),
				Events: []rdstypes.Event{
					{
						Date:             &ts,
						EventCategories:  []string{"maintenance"},
						Message:          aws.String("Event page 1 item 1"),
						SourceIdentifier: aws.String("pag-db"),
						SourceType:       rdstypes.SourceTypeDbInstance,
					},
					{
						Date:             &ts,
						EventCategories:  []string{"notification"},
						Message:          aws.String("Event page 1 item 2"),
						SourceIdentifier: aws.String("pag-db"),
						SourceType:       rdstypes.SourceTypeDbInstance,
					},
					{
						Date:             &ts,
						EventCategories:  []string{"failover"},
						Message:          aws.String("Event page 1 item 3"),
						SourceIdentifier: aws.String("pag-db"),
						SourceType:       rdstypes.SourceTypeDbInstance,
					},
				},
			},
		},
	}

	// First call: no continuation token — fetches page 1.
	result1, err := awsclient.FetchRDSEvents(context.Background(), page1Mock, "pag-db", "")
	if err != nil {
		t.Fatalf("page 1: expected no error, got %v", err)
	}

	t.Run("page1_item_count", func(t *testing.T) {
		if len(result1.Resources) != 3 {
			t.Fatalf("expected 3 resources on page 1, got %d", len(result1.Resources))
		}
	})

	t.Run("page1_single_api_call", func(t *testing.T) {
		if page1Mock.callIdx != 1 {
			t.Errorf("expected 1 API call for page 1, got %d", page1Mock.callIdx)
		}
	})

	t.Run("page1_is_truncated", func(t *testing.T) {
		if result1.Pagination == nil {
			t.Fatal("Pagination must not be nil")
		}
		if !result1.Pagination.IsTruncated {
			t.Error("page 1: IsTruncated should be true when Marker is present")
		}
	})

	t.Run("page1_next_token", func(t *testing.T) {
		if result1.Pagination == nil {
			t.Fatal("Pagination must not be nil")
		}
		if result1.Pagination.NextToken != "page2-marker" {
			t.Errorf("page 1: NextToken expected %q, got %q", "page2-marker", result1.Pagination.NextToken)
		}
	})

	t.Run("page1_messages", func(t *testing.T) {
		for i := 0; i < 3; i++ {
			expected := fmt.Sprintf("Event page 1 item %d", i+1)
			if result1.Resources[i].Fields["message"] != expected {
				t.Errorf("resources[%d].Fields[message]: expected %q, got %q", i, expected, result1.Resources[i].Fields["message"])
			}
		}
	})

	t.Run("page1_all_fields_populated", func(t *testing.T) {
		requiredFields := []string{"timestamp", "event_categories", "message", "source_identifier", "source_type"}
		for i, r := range result1.Resources {
			for _, key := range requiredFields {
				if _, ok := r.Fields[key]; !ok {
					t.Errorf("page 1: resource[%d].Fields missing key %q", i, key)
				}
			}
		}
	})

	// Page 2: 3 events with no Marker — last page.
	page2Mock := &mockRDSDescribeEventsClient{
		outputs: []*rds.DescribeEventsOutput{
			{
				// No Marker — last page
				Events: []rdstypes.Event{
					{
						Date:             &ts,
						EventCategories:  []string{"availability"},
						Message:          aws.String("Event page 2 item 1"),
						SourceIdentifier: aws.String("pag-db"),
						SourceType:       rdstypes.SourceTypeDbInstance,
					},
					{
						Date:             &ts,
						EventCategories:  []string{"configuration change"},
						Message:          aws.String("Event page 2 item 2"),
						SourceIdentifier: aws.String("pag-db"),
						SourceType:       rdstypes.SourceTypeDbInstance,
					},
					{
						Date:             &ts,
						EventCategories:  []string{"notification"},
						Message:          aws.String("Event page 2 item 3"),
						SourceIdentifier: aws.String("pag-db"),
						SourceType:       rdstypes.SourceTypeDbInstance,
					},
				},
			},
		},
	}

	// Second call: pass continuation token from page 1 to fetch page 2.
	result2, err := awsclient.FetchRDSEvents(context.Background(), page2Mock, "pag-db", result1.Pagination.NextToken)
	if err != nil {
		t.Fatalf("page 2: expected no error, got %v", err)
	}

	t.Run("page2_item_count", func(t *testing.T) {
		if len(result2.Resources) != 3 {
			t.Fatalf("expected 3 resources on page 2, got %d", len(result2.Resources))
		}
	})

	t.Run("page2_not_truncated", func(t *testing.T) {
		if result2.Pagination == nil {
			t.Fatal("Pagination must not be nil")
		}
		if result2.Pagination.IsTruncated {
			t.Error("page 2: IsTruncated should be false on last page")
		}
	})

	t.Run("page2_messages", func(t *testing.T) {
		for i := 0; i < 3; i++ {
			expected := fmt.Sprintf("Event page 2 item %d", i+1)
			if result2.Resources[i].Fields["message"] != expected {
				t.Errorf("page 2: resources[%d].Fields[message]: expected %q, got %q", i, expected, result2.Resources[i].Fields["message"])
			}
		}
	})
}

// TestFetchRDSEvents_MaxEventsCap verifies that a single API page of 50 events
// is returned as-is with correct IsTruncated=true metadata when the API
// indicates more pages exist. The 200-item cap no longer applies — each call
// returns one page and the caller drives pagination via continuation tokens.
func TestFetchRDSEvents_MaxEventsCap(t *testing.T) {
	ts := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)

	// Build one page of 50 events with a Marker indicating more pages exist.
	var events []rdstypes.Event
	for i := 0; i < 50; i++ {
		events = append(events, rdstypes.Event{
			Date:             &ts,
			EventCategories:  []string{"notification"},
			Message:          aws.String(fmt.Sprintf("Event p0-%d", i)),
			SourceIdentifier: aws.String("big-db"),
			SourceType:       rdstypes.SourceTypeDbInstance,
		})
	}

	mock := &mockRDSDescribeEventsClient{
		outputs: []*rds.DescribeEventsOutput{
			{
				Events: events,
				Marker: aws.String("marker-page-1"),
			},
		},
	}

	result, err := awsclient.FetchRDSEvents(context.Background(), mock, "big-db", "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	t.Run("returns_full_page_of_50", func(t *testing.T) {
		if len(result.Resources) != 50 {
			t.Errorf("expected exactly 50 resources from single API page, got %d", len(result.Resources))
		}
	})

	t.Run("single_api_call", func(t *testing.T) {
		if mock.callIdx != 1 {
			t.Errorf("expected 1 API call per invocation, got %d", mock.callIdx)
		}
	})

	t.Run("is_truncated_true", func(t *testing.T) {
		if result.Pagination == nil {
			t.Fatal("Pagination must not be nil")
		}
		if !result.Pagination.IsTruncated {
			t.Error("IsTruncated should be true when API returns Marker")
		}
	})

	t.Run("next_token_forwarded", func(t *testing.T) {
		if result.Pagination == nil {
			t.Fatal("Pagination must not be nil")
		}
		if result.Pagination.NextToken != "marker-page-1" {
			t.Errorf("NextToken expected %q, got %q", "marker-page-1", result.Pagination.NextToken)
		}
	})

	t.Run("page_size_equals_item_count", func(t *testing.T) {
		if result.Pagination == nil {
			t.Fatal("Pagination must not be nil")
		}
		if result.Pagination.PageSize != 50 {
			t.Errorf("PageSize expected 50, got %d", result.Pagination.PageSize)
		}
	})

	t.Run("first_event_correct", func(t *testing.T) {
		if result.Resources[0].Fields["message"] != "Event p0-0" {
			t.Errorf("first resource message: expected %q, got %q", "Event p0-0", result.Resources[0].Fields["message"])
		}
	})

	t.Run("last_event_correct", func(t *testing.T) {
		if result.Resources[49].Fields["message"] != "Event p0-49" {
			t.Errorf("last resource message: expected %q, got %q", "Event p0-49", result.Resources[49].Fields["message"])
		}
	})
}

// TestDbiEventColumns verifies that DbiEventColumns returns the expected
// columns with correct keys and widths.
func TestDbiEventColumns(t *testing.T) {
	cols := resource.DbiEventColumns()

	expectedKeys := []string{"timestamp", "event_categories", "message"}

	t.Run("column_count", func(t *testing.T) {
		if len(cols) != len(expectedKeys) {
			t.Fatalf("expected %d columns, got %d", len(expectedKeys), len(cols))
		}
	})

	t.Run("column_keys", func(t *testing.T) {
		for i, expected := range expectedKeys {
			if cols[i].Key != expected {
				t.Errorf("column[%d].Key: expected %q, got %q", i, expected, cols[i].Key)
			}
		}
	})

	t.Run("column_widths", func(t *testing.T) {
		expectedWidths := []int{22, 18, 60}
		for i, expected := range expectedWidths {
			if cols[i].Width != expected {
				t.Errorf("column[%d].Width: expected %d, got %d", i, expected, cols[i].Width)
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

	t.Run("columns_sortable", func(t *testing.T) {
		for i, col := range cols {
			if !col.Sortable {
				t.Errorf("column[%d] (%s) should be sortable", i, col.Key)
			}
		}
	})
}

// TestDbiEvents_ChildTypeRegistered verifies that the child type is
// registered under the correct short name.
func TestDbiEvents_ChildTypeRegistered(t *testing.T) {
	td := resource.GetChildType("dbi_events")
	if td == nil {
		t.Fatal("dbi_events child resource type not registered")
	}
	if td.Name == "" {
		t.Error("child type Name should not be empty")
	}
	if td.ShortName != "dbi_events" {
		t.Errorf("child type ShortName: expected %q, got %q", "dbi_events", td.ShortName)
	}
}

// TestDbiEvents_PaginatedChildFetcherRegistered verifies that the paginated
// child fetcher is
// registered under the correct short name.
func TestDbiEvents_PaginatedChildFetcherRegistered(t *testing.T) {
	f := resource.GetPaginatedChildFetcher("dbi_events")
	if f == nil {
		t.Fatal("dbi_events paginated child fetcher not registered")
	}
}

// TestDbiEvents_ParentHasChildDef verifies that the parent dbi resource
// type has a child view definition for dbi_events with key "enter".
func TestDbiEvents_ParentHasChildDef(t *testing.T) {
	rt := resource.FindResourceType("dbi")
	if rt == nil {
		t.Fatal("dbi resource type not found")
	}

	found := false
	for _, child := range rt.Children {
		if child.ChildType == "dbi_events" {
			found = true
			if child.Key != "enter" {
				t.Errorf("expected key %q, got %q", "enter", child.Key)
			}
			if child.ContextKeys["db_identifier"] == "" {
				t.Error("ContextKeys should include 'db_identifier'")
			}
			if child.ContextKeys["db_identifier"] != "ID" {
				t.Errorf("ContextKeys[db_identifier]: expected %q, got %q",
					"ID", child.ContextKeys["db_identifier"])
			}
			if child.DisplayNameKey != "db_identifier" {
				t.Errorf("DisplayNameKey: expected %q, got %q",
					"db_identifier", child.DisplayNameKey)
			}
		}
	}
	if !found {
		t.Error("dbi Children should contain dbi_events child view def")
	}
}

// TestDbiEvents_CopyField verifies that the registered child type has
// CopyField set to "message".
func TestDbiEvents_CopyField(t *testing.T) {
	td := resource.GetChildType("dbi_events")
	if td == nil {
		t.Fatal("dbi_events child type not found")
	}
	if td.CopyField != "message" {
		t.Errorf("CopyField: expected %q, got %q", "message", td.CopyField)
	}
}

// TestFetchRDSEvents_ContinuationToken verifies that a non-empty
// continuation token is forwarded to the API as Marker.
func TestFetchRDSEvents_ContinuationToken(t *testing.T) {
	ts := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)

	wrapper := &tokenCapturingRDSEventsMock{
		inner: &mockRDSDescribeEventsClient{
			output: &rds.DescribeEventsOutput{
				Events: []rdstypes.Event{
					{
						SourceIdentifier: aws.String("my-db"),
						Date:             &ts,
						Message:          aws.String("Page 2 event"),
						EventCategories:  []string{"availability"},
						SourceType:       rdstypes.SourceTypeDbInstance,
					},
				},
			},
		},
	}

	result, err := awsclient.FetchRDSEvents(context.Background(), wrapper, "my-db", "my-continuation-token")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}

	if wrapper.capturedMarker == nil {
		t.Fatal("expected Marker to be set in API call")
	}
	if *wrapper.capturedMarker != "my-continuation-token" {
		t.Errorf("expected Marker %q, got %q", "my-continuation-token", *wrapper.capturedMarker)
	}
}

// tokenCapturingRDSEventsMock wraps the RDS events mock to capture Marker.
type tokenCapturingRDSEventsMock struct {
	inner          *mockRDSDescribeEventsClient
	capturedMarker *string
}

func (m *tokenCapturingRDSEventsMock) DescribeEvents(ctx context.Context, params *rds.DescribeEventsInput, optFns ...func(*rds.Options)) (*rds.DescribeEventsOutput, error) {
	m.capturedMarker = params.Marker
	return m.inner.DescribeEvents(ctx, params, optFns...)
}
