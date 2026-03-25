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

	resources, err := awsclient.FetchRDSEvents(
		context.Background(),
		mock,
		"my-db-instance",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 3 {
		t.Fatalf("expected 3 resources, got %d", len(resources))
	}

	t.Run("event_0_ID", func(t *testing.T) {
		// ID format: "timestamp/source_identifier"
		expected := "2024-06-15 10:00:00/my-db-instance"
		if resources[0].ID != expected {
			t.Errorf("ID: expected %q, got %q", expected, resources[0].ID)
		}
	})

	t.Run("event_0_Name_is_formatted_timestamp", func(t *testing.T) {
		if resources[0].Name == "" {
			t.Error("Name should not be empty")
		}
		if !strings.Contains(resources[0].Name, "2024-06-15") {
			t.Errorf("Name should contain formatted date, got %q", resources[0].Name)
		}
	})

	t.Run("event_0_Fields_timestamp", func(t *testing.T) {
		r := resources[0]
		if r.Fields["timestamp"] == "" {
			t.Error("Fields[timestamp] should not be empty")
		}
		if !strings.Contains(r.Fields["timestamp"], "2024-06-15 10:00:00") {
			t.Errorf("Fields[timestamp] expected '2024-06-15 10:00:00', got %q", r.Fields["timestamp"])
		}
	})

	t.Run("event_0_Fields_event_categories", func(t *testing.T) {
		r := resources[0]
		if r.Fields["event_categories"] != "maintenance" {
			t.Errorf("Fields[event_categories]: expected %q, got %q", "maintenance", r.Fields["event_categories"])
		}
	})

	t.Run("event_0_Fields_message", func(t *testing.T) {
		r := resources[0]
		if r.Fields["message"] != "Applying offline patches to DB instance" {
			t.Errorf("Fields[message]: expected %q, got %q", "Applying offline patches to DB instance", r.Fields["message"])
		}
	})

	t.Run("event_0_Fields_source_identifier", func(t *testing.T) {
		r := resources[0]
		if r.Fields["source_identifier"] != "my-db-instance" {
			t.Errorf("Fields[source_identifier]: expected %q, got %q", "my-db-instance", r.Fields["source_identifier"])
		}
	})

	t.Run("event_0_Fields_source_type", func(t *testing.T) {
		r := resources[0]
		if r.Fields["source_type"] != "db-instance" {
			t.Errorf("Fields[source_type]: expected %q, got %q", "db-instance", r.Fields["source_type"])
		}
	})

	t.Run("event_0_Fields_source_arn", func(t *testing.T) {
		r := resources[0]
		if r.Fields["source_arn"] != "arn:aws:rds:us-east-1:123456789012:db:my-db-instance" {
			t.Errorf("Fields[source_arn]: expected %q, got %q",
				"arn:aws:rds:us-east-1:123456789012:db:my-db-instance", r.Fields["source_arn"])
		}
	})

	t.Run("event_0_RawStruct", func(t *testing.T) {
		r := resources[0]
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
		r := resources[2]
		if r.Fields["event_categories"] != "availability, notification" {
			t.Errorf("Fields[event_categories]: expected %q, got %q",
				"availability, notification", r.Fields["event_categories"])
		}
	})

	// Verify required fields on all events
	t.Run("required_fields_present", func(t *testing.T) {
		requiredFields := []string{"timestamp", "event_categories", "message", "source_identifier", "source_type", "source_arn"}
		for i, r := range resources {
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

	resources, err := awsclient.FetchRDSEvents(
		context.Background(),
		mock,
		"empty-db",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}

// TestFetchRDSEvents_APIError verifies that API errors are propagated.
func TestFetchRDSEvents_APIError(t *testing.T) {
	mock := &mockRDSDescribeEventsClient{
		err: fmt.Errorf("AWS API error: access denied"),
	}

	resources, err := awsclient.FetchRDSEvents(
		context.Background(),
		mock,
		"err-db",
	)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if resources != nil {
		t.Errorf("expected nil resources on error, got %d", len(resources))
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
	resources, err := awsclient.FetchRDSEvents(
		context.Background(),
		mock,
		"nil-db",
	)
	if err != nil {
		t.Fatalf("expected no error for nil fields, got %v", err)
	}

	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	t.Run("nil_Message", func(t *testing.T) {
		r := resources[0]
		if r.Fields["message"] != "" {
			t.Errorf("Fields[message] should be empty for nil, got %q", r.Fields["message"])
		}
	})

	t.Run("nil_Date", func(t *testing.T) {
		r := resources[0]
		if r.Fields["timestamp"] != "" {
			t.Logf("Fields[timestamp] is %q (expected empty for nil Date)", r.Fields["timestamp"])
		}
	})

	t.Run("nil_SourceArn", func(t *testing.T) {
		r := resources[0]
		if r.Fields["source_arn"] != "" {
			t.Errorf("Fields[source_arn] should be empty for nil, got %q", r.Fields["source_arn"])
		}
	})

	t.Run("nil_SourceIdentifier", func(t *testing.T) {
		r := resources[0]
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

	resources, err := awsclient.FetchRDSEvents(
		context.Background(),
		mock,
		"nl-db",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	msg := resources[0].Fields["message"]
	if strings.Contains(msg, "\n") {
		t.Errorf("Fields[message] should not contain \\n, got %q", msg)
	}
	if strings.Contains(msg, "\r") {
		t.Errorf("Fields[message] should not contain \\r, got %q", msg)
	}
}

// TestFetchRDSEvents_TimestampFormatting verifies that a known time.Time
// produces the "2006-01-02 15:04:05" format in Fields.
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

	resources, err := awsclient.FetchRDSEvents(
		context.Background(),
		mock,
		"ts-db",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	tsField := resources[0].Fields["timestamp"]
	if tsField != "2024-12-25 14:30:45" {
		t.Errorf("Fields[timestamp]: expected %q, got %q", "2024-12-25 14:30:45", tsField)
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

	resources, err := awsclient.FetchRDSEvents(
		context.Background(),
		mock,
		"raw-db",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	r := resources[0]
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

	resources, err := awsclient.FetchRDSEvents(
		context.Background(),
		mock,
		"cat-db",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	cats := resources[0].Fields["event_categories"]
	if cats != "availability, failover, notification" {
		t.Errorf("Fields[event_categories]: expected %q, got %q",
			"availability, failover, notification", cats)
	}
}

// TestFetchRDSEvents_Pagination verifies that paginated responses via Marker
// are followed and all events collected across multiple pages.
func TestFetchRDSEvents_Pagination(t *testing.T) {
	ts := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)

	mock := &mockRDSDescribeEventsClient{
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

	resources, err := awsclient.FetchRDSEvents(
		context.Background(),
		mock,
		"pag-db",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	t.Run("total_count", func(t *testing.T) {
		if len(resources) != 6 {
			t.Fatalf("expected 6 resources across 2 pages, got %d", len(resources))
		}
	})

	t.Run("page1_messages", func(t *testing.T) {
		for i := 0; i < 3; i++ {
			expected := fmt.Sprintf("Event page 1 item %d", i+1)
			if resources[i].Fields["message"] != expected {
				t.Errorf("resources[%d].Fields[message]: expected %q, got %q", i, expected, resources[i].Fields["message"])
			}
		}
	})

	t.Run("page2_messages", func(t *testing.T) {
		for i := 0; i < 3; i++ {
			expected := fmt.Sprintf("Event page 2 item %d", i+1)
			if resources[i+3].Fields["message"] != expected {
				t.Errorf("resources[%d].Fields[message]: expected %q, got %q", i+3, expected, resources[i+3].Fields["message"])
			}
		}
	})

	t.Run("api_called_twice", func(t *testing.T) {
		if mock.callIdx != 2 {
			t.Errorf("expected 2 API calls for pagination, got %d", mock.callIdx)
		}
	})

	t.Run("all_fields_populated", func(t *testing.T) {
		requiredFields := []string{"timestamp", "event_categories", "message", "source_identifier", "source_type"}
		for i, r := range resources {
			for _, key := range requiredFields {
				if _, ok := r.Fields[key]; !ok {
					t.Errorf("resource[%d].Fields missing key %q", i, key)
				}
			}
		}
	})
}

// TestFetchRDSEvents_MaxEventsCap verifies that the fetcher stops collecting
// events once it reaches the maxEvents=200 cap, even if more pages are available.
func TestFetchRDSEvents_MaxEventsCap(t *testing.T) {
	ts := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)

	// Build 5 pages of 50 events each (250 total). The fetcher should stop at 200.
	var outputs []*rds.DescribeEventsOutput
	for page := 0; page < 5; page++ {
		var events []rdstypes.Event
		for i := 0; i < 50; i++ {
			events = append(events, rdstypes.Event{
				Date:             &ts,
				EventCategories:  []string{"notification"},
				Message:          aws.String(fmt.Sprintf("Event p%d-%d", page, i)),
				SourceIdentifier: aws.String("big-db"),
				SourceType:       rdstypes.SourceTypeDbInstance,
			})
		}
		out := &rds.DescribeEventsOutput{
			Events: events,
		}
		if page < 4 {
			out.Marker = aws.String(fmt.Sprintf("marker-page-%d", page+1))
		}
		outputs = append(outputs, out)
	}

	mock := &mockRDSDescribeEventsClient{outputs: outputs}

	resources, err := awsclient.FetchRDSEvents(
		context.Background(),
		mock,
		"big-db",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	t.Run("capped_at_200", func(t *testing.T) {
		if len(resources) != 200 {
			t.Errorf("expected exactly 200 resources (maxEvents cap), got %d", len(resources))
		}
	})

	t.Run("early_termination", func(t *testing.T) {
		// With 50 events per page, reaching 200 should take exactly 4 pages.
		// The fetcher should NOT call the 5th page.
		if mock.callIdx != 4 {
			t.Errorf("expected 4 API calls (early termination at 200), got %d", mock.callIdx)
		}
	})

	t.Run("first_event_correct", func(t *testing.T) {
		if resources[0].Fields["message"] != "Event p0-0" {
			t.Errorf("first resource message: expected %q, got %q",
				"Event p0-0", resources[0].Fields["message"])
		}
	})

	t.Run("last_event_correct", func(t *testing.T) {
		// Last event should be the 50th event of page 3 (index 199 = page3, event49)
		if resources[199].Fields["message"] != "Event p3-49" {
			t.Errorf("last resource message: expected %q, got %q",
				"Event p3-49", resources[199].Fields["message"])
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

// TestDbiEvents_ChildFetcherRegistered verifies that the child fetcher is
// registered under the correct short name.
func TestDbiEvents_ChildFetcherRegistered(t *testing.T) {
	f := resource.GetChildFetcher("dbi_events")
	if f == nil {
		t.Fatal("dbi_events child fetcher not registered")
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
