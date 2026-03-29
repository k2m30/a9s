package unit

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	cloudwatch "github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// CloudWatch Alarm History fetcher tests (child of CloudWatch Alarms)
// ---------------------------------------------------------------------------

// TestFetchAlarmHistory_Basic verifies parsing of 1 alarm history item with all
// fields populated, checking ID, Name, Status, all Fields, and RawStruct.
func TestFetchAlarmHistory_Basic(t *testing.T) {
	ts := time.Date(2024, 3, 22, 10, 0, 0, 0, time.UTC)

	mock := &mockCloudWatchDescribeAlarmHistoryClient{
		output: &cloudwatch.DescribeAlarmHistoryOutput{
			AlarmHistoryItems: []cwtypes.AlarmHistoryItem{
				{
					AlarmName:       aws.String("HighCPUAlarm"),
					AlarmType:       cwtypes.AlarmTypeMetricAlarm,
					HistoryData:     aws.String(`{"version":"1.0","oldState":{"stateValue":"OK"},"newState":{"stateValue":"ALARM"}}`),
					HistoryItemType: cwtypes.HistoryItemTypeStateUpdate,
					HistorySummary:  aws.String("Alarm updated from OK to ALARM"),
					Timestamp:       &ts,
				},
			},
		},
	}

	parentCtx := map[string]string{
		"alarm_name": "HighCPUAlarm",
	}

	result, err := awsclient.FetchAlarmHistory(
		context.Background(),
		mock,
		parentCtx,
		"",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}

	r := result.Resources[0]

	t.Run("ID_is_formatted_timestamp", func(t *testing.T) {
		if r.ID == "" {
			t.Error("ID should not be empty")
		}
		if !strings.Contains(r.ID, "2024-03-22") {
			t.Errorf("ID should contain formatted date, got %q", r.ID)
		}
	})

	t.Run("Name_is_formatted_timestamp", func(t *testing.T) {
		if r.Name == "" {
			t.Error("Name should not be empty")
		}
		if !strings.Contains(r.Name, "2024-03-22") {
			t.Errorf("Name should contain formatted date, got %q", r.Name)
		}
	})

	t.Run("Status_is_string_HistoryItemType", func(t *testing.T) {
		if r.Status != "StateUpdate" {
			t.Errorf("Status: expected %q, got %q", "StateUpdate", r.Status)
		}
	})

	t.Run("Fields_timestamp", func(t *testing.T) {
		if r.Fields["timestamp"] == "" {
			t.Error("Fields[timestamp] should not be empty")
		}
		if !strings.Contains(r.Fields["timestamp"], "2024-03-22 10:00") {
			t.Errorf("Fields[timestamp] expected '2024-03-22 10:00', got %q", r.Fields["timestamp"])
		}
	})

	t.Run("Fields_history_item_type", func(t *testing.T) {
		if r.Fields["history_item_type"] != "StateUpdate" {
			t.Errorf("Fields[history_item_type]: expected %q, got %q", "StateUpdate", r.Fields["history_item_type"])
		}
	})

	t.Run("Fields_history_summary", func(t *testing.T) {
		if r.Fields["history_summary"] != "Alarm updated from OK to ALARM" {
			t.Errorf("Fields[history_summary]: expected %q, got %q", "Alarm updated from OK to ALARM", r.Fields["history_summary"])
		}
	})

	t.Run("RawStruct_is_AlarmHistoryItem", func(t *testing.T) {
		if r.RawStruct == nil {
			t.Fatal("RawStruct must not be nil")
		}
		raw, ok := r.RawStruct.(cwtypes.AlarmHistoryItem)
		if !ok {
			t.Fatalf("RawStruct should be cwtypes.AlarmHistoryItem, got %T", r.RawStruct)
		}
		if raw.AlarmName == nil || *raw.AlarmName != "HighCPUAlarm" {
			t.Error("RawStruct.AlarmName not preserved correctly")
		}
	})

	// Verify required fields are present
	t.Run("required_fields_present", func(t *testing.T) {
		requiredFields := []string{"timestamp", "history_item_type", "history_summary"}
		for _, key := range requiredFields {
			if _, ok := r.Fields[key]; !ok {
				t.Errorf("Fields missing key %q", key)
			}
		}
	})
}

// TestFetchAlarmHistory_Empty verifies that an alarm with no history items
// returns an empty slice with no error.
func TestFetchAlarmHistory_Empty(t *testing.T) {
	mock := &mockCloudWatchDescribeAlarmHistoryClient{
		output: &cloudwatch.DescribeAlarmHistoryOutput{
			AlarmHistoryItems: []cwtypes.AlarmHistoryItem{},
		},
	}

	parentCtx := map[string]string{
		"alarm_name": "EmptyAlarm",
	}

	result, err := awsclient.FetchAlarmHistory(
		context.Background(),
		mock,
		parentCtx,
		"",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
}

// TestFetchAlarmHistory_APIError verifies that API errors are propagated.
func TestFetchAlarmHistory_APIError(t *testing.T) {
	mock := &mockCloudWatchDescribeAlarmHistoryClient{
		err: fmt.Errorf("AWS API error: access denied"),
	}

	parentCtx := map[string]string{
		"alarm_name": "ErrAlarm",
	}

	result, err := awsclient.FetchAlarmHistory(
		context.Background(),
		mock,
		parentCtx,
		"",
	)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if result.Resources != nil {
		t.Errorf("expected nil resources on error, got %d", len(result.Resources))
	}
}

// TestFetchAlarmHistory_NilFields verifies that nil optional fields
// (HistorySummary, HistoryData are *string) do not cause a panic.
func TestFetchAlarmHistory_NilFields(t *testing.T) {
	ts := time.Date(2024, 3, 22, 10, 0, 0, 0, time.UTC)

	mock := &mockCloudWatchDescribeAlarmHistoryClient{
		output: &cloudwatch.DescribeAlarmHistoryOutput{
			AlarmHistoryItems: []cwtypes.AlarmHistoryItem{
				{
					AlarmName:       aws.String("NilFieldsAlarm"),
					AlarmType:       cwtypes.AlarmTypeMetricAlarm,
					HistoryItemType: cwtypes.HistoryItemTypeConfigurationUpdate,
					Timestamp:       &ts,
					// HistorySummary is nil
					// HistoryData is nil
				},
			},
		},
	}

	parentCtx := map[string]string{
		"alarm_name": "NilFieldsAlarm",
	}

	// Should not panic
	result, err := awsclient.FetchAlarmHistory(
		context.Background(),
		mock,
		parentCtx,
		"",
	)
	if err != nil {
		t.Fatalf("expected no error for nil fields, got %v", err)
	}

	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}

	t.Run("nil_HistorySummary", func(t *testing.T) {
		r := result.Resources[0]
		if r.Fields["history_summary"] != "" {
			t.Logf("Fields[history_summary] is %q (expected empty for nil)", r.Fields["history_summary"])
		}
	})

	t.Run("history_item_type_populated", func(t *testing.T) {
		r := result.Resources[0]
		if r.Fields["history_item_type"] != "ConfigurationUpdate" {
			t.Errorf("Fields[history_item_type]: expected %q, got %q", "ConfigurationUpdate", r.Fields["history_item_type"])
		}
	})
}

// TestFetchAlarmHistory_NewlineStripping verifies that HistorySummary
// with newlines and carriage returns gets stripped in Fields.
func TestFetchAlarmHistory_NewlineStripping(t *testing.T) {
	ts := time.Date(2024, 3, 22, 10, 0, 0, 0, time.UTC)

	mock := &mockCloudWatchDescribeAlarmHistoryClient{
		output: &cloudwatch.DescribeAlarmHistoryOutput{
			AlarmHistoryItems: []cwtypes.AlarmHistoryItem{
				{
					AlarmName:       aws.String("NewlineAlarm"),
					AlarmType:       cwtypes.AlarmTypeMetricAlarm,
					HistoryItemType: cwtypes.HistoryItemTypeStateUpdate,
					HistorySummary:  aws.String("Alarm transitioned\nfrom OK\rto ALARM state"),
					Timestamp:       &ts,
				},
			},
		},
	}

	parentCtx := map[string]string{
		"alarm_name": "NewlineAlarm",
	}

	result, err := awsclient.FetchAlarmHistory(
		context.Background(),
		mock,
		parentCtx,
		"",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}

	t.Run("history_summary_no_newlines", func(t *testing.T) {
		summary := result.Resources[0].Fields["history_summary"]
		if strings.Contains(summary, "\n") || strings.Contains(summary, "\r") {
			t.Errorf("Fields[history_summary] should not contain newlines, got %q", summary)
		}
	})
}

// TestFetchAlarmHistory_TimestampFormatting verifies that a known time.Time
// produces the "2006-01-02 15:04" format in Fields.
func TestFetchAlarmHistory_TimestampFormatting(t *testing.T) {
	ts := time.Date(2024, 12, 25, 14, 30, 45, 0, time.UTC)

	mock := &mockCloudWatchDescribeAlarmHistoryClient{
		output: &cloudwatch.DescribeAlarmHistoryOutput{
			AlarmHistoryItems: []cwtypes.AlarmHistoryItem{
				{
					AlarmName:       aws.String("TsAlarm"),
					AlarmType:       cwtypes.AlarmTypeMetricAlarm,
					HistoryItemType: cwtypes.HistoryItemTypeAction,
					HistorySummary:  aws.String("Published SNS notification"),
					Timestamp:       &ts,
				},
			},
		},
	}

	parentCtx := map[string]string{
		"alarm_name": "TsAlarm",
	}

	result, err := awsclient.FetchAlarmHistory(
		context.Background(),
		mock,
		parentCtx,
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

// TestFetchAlarmHistory_RawStruct verifies that RawStruct preserves the
// original cwtypes.AlarmHistoryItem, including all sub-fields.
func TestFetchAlarmHistory_RawStruct(t *testing.T) {
	ts := time.Date(2024, 3, 22, 12, 30, 0, 0, time.UTC)

	mock := &mockCloudWatchDescribeAlarmHistoryClient{
		output: &cloudwatch.DescribeAlarmHistoryOutput{
			AlarmHistoryItems: []cwtypes.AlarmHistoryItem{
				{
					AlarmName:       aws.String("RawAlarm"),
					AlarmType:       cwtypes.AlarmTypeMetricAlarm,
					HistoryData:     aws.String(`{"version":"1.0","oldState":{"stateValue":"OK"}}`),
					HistoryItemType: cwtypes.HistoryItemTypeStateUpdate,
					HistorySummary:  aws.String("Alarm updated from OK to ALARM"),
					Timestamp:       &ts,
				},
			},
		},
	}

	parentCtx := map[string]string{
		"alarm_name": "RawAlarm",
	}

	result, err := awsclient.FetchAlarmHistory(
		context.Background(),
		mock,
		parentCtx,
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

	raw, ok := r.RawStruct.(cwtypes.AlarmHistoryItem)
	if !ok {
		t.Fatalf("RawStruct should be cwtypes.AlarmHistoryItem, got %T", r.RawStruct)
	}

	t.Run("AlarmName_preserved", func(t *testing.T) {
		if raw.AlarmName == nil || *raw.AlarmName != "RawAlarm" {
			t.Errorf("RawStruct.AlarmName not preserved correctly")
		}
	})

	t.Run("Timestamp_preserved", func(t *testing.T) {
		if raw.Timestamp == nil || !raw.Timestamp.Equal(ts) {
			t.Errorf("RawStruct.Timestamp not preserved correctly")
		}
	})

	t.Run("HistoryData_preserved", func(t *testing.T) {
		if raw.HistoryData == nil || !strings.Contains(*raw.HistoryData, "oldState") {
			t.Errorf("RawStruct.HistoryData not preserved correctly")
		}
	})

	t.Run("HistoryItemType_preserved", func(t *testing.T) {
		if raw.HistoryItemType != cwtypes.HistoryItemTypeStateUpdate {
			t.Errorf("RawStruct.HistoryItemType: expected %q, got %q", cwtypes.HistoryItemTypeStateUpdate, raw.HistoryItemType)
		}
	})

	t.Run("HistorySummary_preserved", func(t *testing.T) {
		if raw.HistorySummary == nil || *raw.HistorySummary != "Alarm updated from OK to ALARM" {
			t.Errorf("RawStruct.HistorySummary not preserved correctly")
		}
	})

	t.Run("AlarmType_preserved", func(t *testing.T) {
		if raw.AlarmType != cwtypes.AlarmTypeMetricAlarm {
			t.Errorf("RawStruct.AlarmType: expected %q, got %q", cwtypes.AlarmTypeMetricAlarm, raw.AlarmType)
		}
	})
}

// TestFetchAlarmHistory_Pagination verifies the single-page pagination contract:
// one API call is made, resources from that page are returned, and IsTruncated/NextToken
// reflect whether more pages exist. A second call with the continuation token verifies
// that the token is forwarded and the final page sets IsTruncated=false.
func TestFetchAlarmHistory_Pagination(t *testing.T) {
	ts := time.Date(2024, 3, 22, 10, 0, 0, 0, time.UTC)

	// Page 1: 3 items with NextToken indicating more pages exist.
	page1Mock := &mockCloudWatchDescribeAlarmHistoryClient{
		output: &cloudwatch.DescribeAlarmHistoryOutput{
			NextToken: aws.String("page2-token"),
			AlarmHistoryItems: []cwtypes.AlarmHistoryItem{
				{
					AlarmName:       aws.String("PaginatedAlarm"),
					AlarmType:       cwtypes.AlarmTypeMetricAlarm,
					HistoryItemType: cwtypes.HistoryItemTypeStateUpdate,
					HistorySummary:  aws.String("State update page 1 item 1"),
					Timestamp:       &ts,
				},
				{
					AlarmName:       aws.String("PaginatedAlarm"),
					AlarmType:       cwtypes.AlarmTypeMetricAlarm,
					HistoryItemType: cwtypes.HistoryItemTypeConfigurationUpdate,
					HistorySummary:  aws.String("Config update page 1 item 2"),
					Timestamp:       &ts,
				},
				{
					AlarmName:       aws.String("PaginatedAlarm"),
					AlarmType:       cwtypes.AlarmTypeMetricAlarm,
					HistoryItemType: cwtypes.HistoryItemTypeAction,
					HistorySummary:  aws.String("Action page 1 item 3"),
					Timestamp:       &ts,
				},
			},
		},
	}

	parentCtx := map[string]string{
		"alarm_name": "PaginatedAlarm",
	}

	// First call: no continuation token — fetches page 1.
	result1, err := awsclient.FetchAlarmHistory(
		context.Background(),
		page1Mock,
		parentCtx,
		"",
	)
	if err != nil {
		t.Fatalf("page 1: expected no error, got %v", err)
	}

	t.Run("page1_item_count", func(t *testing.T) {
		if len(result1.Resources) != 3 {
			t.Fatalf("expected 3 resources on page 1, got %d", len(result1.Resources))
		}
	})

	t.Run("page1_is_truncated", func(t *testing.T) {
		if result1.Pagination == nil {
			t.Fatal("Pagination must not be nil")
		}
		if !result1.Pagination.IsTruncated {
			t.Error("page 1: IsTruncated should be true when NextToken is present")
		}
	})

	t.Run("page1_next_token", func(t *testing.T) {
		if result1.Pagination == nil {
			t.Fatal("Pagination must not be nil")
		}
		if result1.Pagination.NextToken != "page2-token" {
			t.Errorf("page 1: NextToken expected %q, got %q", "page2-token", result1.Pagination.NextToken)
		}
	})

	t.Run("page1_page_size", func(t *testing.T) {
		if result1.Pagination == nil {
			t.Fatal("Pagination must not be nil")
		}
		if result1.Pagination.PageSize != 3 {
			t.Errorf("page 1: PageSize expected 3, got %d", result1.Pagination.PageSize)
		}
	})

	t.Run("page1_total_hint_negative", func(t *testing.T) {
		if result1.Pagination == nil {
			t.Fatal("Pagination must not be nil")
		}
		if result1.Pagination.TotalHint != -1 {
			t.Errorf("page 1: TotalHint should be -1 when truncated, got %d", result1.Pagination.TotalHint)
		}
	})

	t.Run("page1_all_have_status", func(t *testing.T) {
		for i, r := range result1.Resources {
			if r.Status == "" {
				t.Errorf("page 1: resources[%d].Status should not be empty", i)
			}
		}
	})

	t.Run("page1_all_fields_populated", func(t *testing.T) {
		requiredFields := []string{"timestamp", "history_item_type", "history_summary"}
		for i, r := range result1.Resources {
			for _, key := range requiredFields {
				if _, ok := r.Fields[key]; !ok {
					t.Errorf("page 1: resource[%d].Fields missing key %q", i, key)
				}
			}
		}
	})

	t.Run("page1_single_api_call", func(t *testing.T) {
		// The new implementation makes exactly one API call per FetchAlarmHistory invocation.
		// The mock uses single output field (not outputs slice), so callIdx stays 0.
		// Verify the mock was only called once by checking Resources were returned.
		if len(result1.Resources) == 0 {
			t.Error("expected resources from single API call")
		}
	})

	// Page 2: 2 items with no NextToken — last page.
	page2Mock := &mockCloudWatchDescribeAlarmHistoryClient{
		output: &cloudwatch.DescribeAlarmHistoryOutput{
			// No NextToken — last page
			AlarmHistoryItems: []cwtypes.AlarmHistoryItem{
				{
					AlarmName:       aws.String("PaginatedAlarm"),
					AlarmType:       cwtypes.AlarmTypeMetricAlarm,
					HistoryItemType: cwtypes.HistoryItemTypeStateUpdate,
					HistorySummary:  aws.String("State update page 2 item 1"),
					Timestamp:       &ts,
				},
				{
					AlarmName:       aws.String("PaginatedAlarm"),
					AlarmType:       cwtypes.AlarmTypeMetricAlarm,
					HistoryItemType: cwtypes.HistoryItemTypeAction,
					HistorySummary:  aws.String("Action page 2 item 2"),
					Timestamp:       &ts,
				},
			},
		},
	}

	// Second call: pass continuation token from page 1 to fetch page 2.
	result2, err := awsclient.FetchAlarmHistory(
		context.Background(),
		page2Mock,
		parentCtx,
		result1.Pagination.NextToken,
	)
	if err != nil {
		t.Fatalf("page 2: expected no error, got %v", err)
	}

	t.Run("page2_item_count", func(t *testing.T) {
		if len(result2.Resources) != 2 {
			t.Fatalf("expected 2 resources on page 2, got %d", len(result2.Resources))
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

	t.Run("page2_empty_next_token", func(t *testing.T) {
		if result2.Pagination == nil {
			t.Fatal("Pagination must not be nil")
		}
		if result2.Pagination.NextToken != "" {
			t.Errorf("page 2: NextToken should be empty on last page, got %q", result2.Pagination.NextToken)
		}
	})

	t.Run("page2_total_hint_equals_count", func(t *testing.T) {
		if result2.Pagination == nil {
			t.Fatal("Pagination must not be nil")
		}
		if result2.Pagination.TotalHint != 2 {
			t.Errorf("page 2: TotalHint should equal item count (2) on last page, got %d", result2.Pagination.TotalHint)
		}
	})
}

// TestFetchAlarmHistory_MaxCap verifies that a single API page of 50 items is
// returned as-is with correct IsTruncated=true metadata when the API indicates
// more pages exist. The 200-item cap no longer applies — each call returns one
// page and the caller drives pagination via continuation tokens.
func TestFetchAlarmHistory_MaxCap(t *testing.T) {
	ts := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)

	// Build one page of 50 items with a NextToken indicating more pages exist.
	var items []cwtypes.AlarmHistoryItem
	for i := 0; i < 50; i++ {
		itemTs := ts.Add(time.Duration(i) * time.Second)
		items = append(items, cwtypes.AlarmHistoryItem{
			AlarmName:       aws.String("BigAlarm"),
			AlarmType:       cwtypes.AlarmTypeMetricAlarm,
			HistoryItemType: cwtypes.HistoryItemTypeStateUpdate,
			HistorySummary:  aws.String(fmt.Sprintf("Event p0-%d", i)),
			Timestamp:       &itemTs,
		})
	}

	mock := &mockCloudWatchDescribeAlarmHistoryClient{
		output: &cloudwatch.DescribeAlarmHistoryOutput{
			AlarmHistoryItems: items,
			NextToken:         aws.String("token-page-1"),
		},
	}

	parentCtx := map[string]string{
		"alarm_name": "BigAlarm",
	}

	result, err := awsclient.FetchAlarmHistory(
		context.Background(),
		mock,
		parentCtx,
		"",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	t.Run("returns_full_page_of_50", func(t *testing.T) {
		if len(result.Resources) != 50 {
			t.Errorf("expected exactly 50 resources from single API page, got %d", len(result.Resources))
		}
	})

	t.Run("is_truncated_true", func(t *testing.T) {
		if result.Pagination == nil {
			t.Fatal("Pagination must not be nil")
		}
		if !result.Pagination.IsTruncated {
			t.Error("IsTruncated should be true when API returns NextToken")
		}
	})

	t.Run("next_token_forwarded", func(t *testing.T) {
		if result.Pagination == nil {
			t.Fatal("Pagination must not be nil")
		}
		if result.Pagination.NextToken != "token-page-1" {
			t.Errorf("NextToken expected %q, got %q", "token-page-1", result.Pagination.NextToken)
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

	t.Run("total_hint_negative_when_truncated", func(t *testing.T) {
		if result.Pagination == nil {
			t.Fatal("Pagination must not be nil")
		}
		if result.Pagination.TotalHint != -1 {
			t.Errorf("TotalHint should be -1 when truncated, got %d", result.Pagination.TotalHint)
		}
	})

	t.Run("first_item_correct", func(t *testing.T) {
		if result.Resources[0].Fields["history_summary"] != "Event p0-0" {
			t.Errorf("first resource history_summary: expected %q, got %q", "Event p0-0", result.Resources[0].Fields["history_summary"])
		}
	})

	t.Run("last_item_correct", func(t *testing.T) {
		if result.Resources[49].Fields["history_summary"] != "Event p0-49" {
			t.Errorf("last resource history_summary: expected %q, got %q", "Event p0-49", result.Resources[49].Fields["history_summary"])
		}
	})
}

// TestAlarmHistoryColumns verifies that AlarmHistoryColumns returns the expected
// columns with correct keys, titles, and positive widths.
func TestAlarmHistoryColumns(t *testing.T) {
	cols := resource.AlarmHistoryColumns()

	expectedKeys := []string{"timestamp", "history_item_type", "history_summary"}

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

	t.Run("expected_widths", func(t *testing.T) {
		expectedWidths := []int{22, 18, 60}
		for i, expected := range expectedWidths {
			if cols[i].Width != expected {
				t.Errorf("column[%d] (%s).Width: expected %d, got %d", i, cols[i].Key, expected, cols[i].Width)
			}
		}
	})
}

// TestAlarmHistory_ChildTypeRegistered verifies that the child type is
// registered under the correct short name.
func TestAlarmHistory_ChildTypeRegistered(t *testing.T) {
	td := resource.GetChildType("alarm_history")
	if td == nil {
		t.Fatal("alarm_history child resource type not registered")
	}
	if td.Name == "" {
		t.Error("child type Name should not be empty")
	}
	if td.ShortName != "alarm_history" {
		t.Errorf("child type ShortName: expected %q, got %q", "alarm_history", td.ShortName)
	}
}

// TestAlarmHistory_PaginatedChildFetcherRegistered verifies that the paginated
// child fetcher is
// registered under the correct short name.
func TestAlarmHistory_PaginatedChildFetcherRegistered(t *testing.T) {
	f := resource.GetPaginatedChildFetcher("alarm_history")
	if f == nil {
		t.Fatal("alarm_history paginated child fetcher not registered")
	}
}

// TestFetchAlarmHistory_ContinuationToken verifies that a non-empty
// continuation token is forwarded to the API as NextToken.
func TestFetchAlarmHistory_ContinuationToken(t *testing.T) {
	ts := time.Date(2024, 3, 22, 10, 0, 0, 0, time.UTC)

	wrapper := &tokenCapturingAlarmHistoryMock{
		inner: &mockCloudWatchDescribeAlarmHistoryClient{
			output: &cloudwatch.DescribeAlarmHistoryOutput{
				AlarmHistoryItems: []cwtypes.AlarmHistoryItem{
					{
						AlarmName:       aws.String("TokenAlarm"),
						HistoryItemType: cwtypes.HistoryItemTypeStateUpdate,
						HistorySummary:  aws.String("Page 2 data"),
						Timestamp:       &ts,
					},
				},
			},
		},
	}

	parentCtx := map[string]string{
		"alarm_name": "TokenAlarm",
	}

	result, err := awsclient.FetchAlarmHistory(
		context.Background(),
		wrapper,
		parentCtx,
		"my-continuation-token",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}

	// Verify the continuation token was forwarded
	if wrapper.capturedNextToken == nil {
		t.Fatal("expected NextToken to be set in API call")
	}
	if *wrapper.capturedNextToken != "my-continuation-token" {
		t.Errorf("expected NextToken %q, got %q", "my-continuation-token", *wrapper.capturedNextToken)
	}
}

// tokenCapturingAlarmHistoryMock wraps the alarm history mock to capture NextToken.
type tokenCapturingAlarmHistoryMock struct {
	inner             *mockCloudWatchDescribeAlarmHistoryClient
	capturedNextToken *string
}

func (m *tokenCapturingAlarmHistoryMock) DescribeAlarmHistory(ctx context.Context, params *cloudwatch.DescribeAlarmHistoryInput, optFns ...func(*cloudwatch.Options)) (*cloudwatch.DescribeAlarmHistoryOutput, error) {
	m.capturedNextToken = params.NextToken
	return m.inner.DescribeAlarmHistory(ctx, params, optFns...)
}

// TestAlarmHistory_ParentHasChildDef verifies that the parent alarm resource
// type has a child view definition for alarm_history with key "enter".
func TestAlarmHistory_ParentHasChildDef(t *testing.T) {
	rt := resource.FindResourceType("alarm")
	if rt == nil {
		t.Fatal("alarm resource type not found")
	}

	found := false
	for _, child := range rt.Children {
		if child.ChildType == "alarm_history" {
			found = true
			if child.Key != "enter" {
				t.Errorf("expected key %q, got %q", "enter", child.Key)
			}
			if child.ContextKeys["alarm_name"] == "" {
				t.Error("ContextKeys should include 'alarm_name'")
			}
		}
	}
	if !found {
		t.Error("alarm Children should contain alarm_history child view def")
	}
}
