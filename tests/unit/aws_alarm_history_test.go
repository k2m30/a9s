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
		if !strings.Contains(r.Fields["timestamp"], "2024-03-22 10:00:00") {
			t.Errorf("Fields[timestamp] expected '2024-03-22 10:00:00', got %q", r.Fields["timestamp"])
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
// produces the "2006-01-02 15:04:05" format in Fields.
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
	if tsField != "2024-12-25 14:30:45" {
		t.Errorf("Fields[timestamp]: expected %q, got %q", "2024-12-25 14:30:45", tsField)
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

// TestFetchAlarmHistory_Pagination verifies that paginated responses via
// NextToken are followed and all history items collected across multiple pages.
func TestFetchAlarmHistory_Pagination(t *testing.T) {
	ts := time.Date(2024, 3, 22, 10, 0, 0, 0, time.UTC)

	mock := &mockCloudWatchDescribeAlarmHistoryClient{
		outputs: []*cloudwatch.DescribeAlarmHistoryOutput{
			{
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
			{
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
		},
	}

	parentCtx := map[string]string{
		"alarm_name": "PaginatedAlarm",
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

	t.Run("total_count", func(t *testing.T) {
		if len(result.Resources) != 5 {
			t.Fatalf("expected 5 resources across 2 pages, got %d", len(result.Resources))
		}
	})

	t.Run("all_have_status", func(t *testing.T) {
		for i, r := range result.Resources {
			if r.Status == "" {
				t.Errorf("resources[%d].Status should not be empty", i)
			}
		}
	})

	t.Run("api_called_twice", func(t *testing.T) {
		if mock.callIdx != 2 {
			t.Errorf("expected 2 API calls for pagination, got %d", mock.callIdx)
		}
	})

	t.Run("all_fields_populated", func(t *testing.T) {
		requiredFields := []string{"timestamp", "history_item_type", "history_summary"}
		for i, r := range result.Resources {
			for _, key := range requiredFields {
				if _, ok := r.Fields[key]; !ok {
					t.Errorf("resource[%d].Fields missing key %q", i, key)
				}
			}
		}
	})
}

// TestFetchAlarmHistory_MaxCap verifies that the fetcher stops
// collecting history items once it reaches the 200 cap.
func TestFetchAlarmHistory_MaxCap(t *testing.T) {
	ts := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)

	// Build 5 pages of 50 items each (250 total). The fetcher should stop at 200.
	var outputs []*cloudwatch.DescribeAlarmHistoryOutput
	for page := 0; page < 5; page++ {
		var items []cwtypes.AlarmHistoryItem
		for i := 0; i < 50; i++ {
			itemTs := ts.Add(time.Duration(page*50+i) * time.Second)
			items = append(items, cwtypes.AlarmHistoryItem{
				AlarmName:       aws.String("BigAlarm"),
				AlarmType:       cwtypes.AlarmTypeMetricAlarm,
				HistoryItemType: cwtypes.HistoryItemTypeStateUpdate,
				HistorySummary:  aws.String(fmt.Sprintf("Event p%d-%d", page, i)),
				Timestamp:       &itemTs,
			})
		}
		out := &cloudwatch.DescribeAlarmHistoryOutput{
			AlarmHistoryItems: items,
		}
		if page < 4 {
			out.NextToken = aws.String(fmt.Sprintf("token-page-%d", page+1))
		}
		outputs = append(outputs, out)
	}

	mock := &mockCloudWatchDescribeAlarmHistoryClient{outputs: outputs}

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

	t.Run("capped_at_200", func(t *testing.T) {
		if len(result.Resources) != 200 {
			t.Errorf("expected exactly 200 resources (max cap), got %d", len(result.Resources))
		}
	})

	t.Run("early_termination", func(t *testing.T) {
		// With 50 items per page, reaching 200 should take exactly 4 pages.
		// The fetcher should NOT call the 5th page.
		if mock.callIdx != 4 {
			t.Errorf("expected 4 API calls (early termination at 200), got %d", mock.callIdx)
		}
	})

	t.Run("first_item_correct", func(t *testing.T) {
		if result.Resources[0].Fields["history_summary"] != "Event p0-0" {
			t.Errorf("first resource history_summary: expected %q, got %q", "Event p0-0", result.Resources[0].Fields["history_summary"])
		}
	})

	t.Run("last_item_correct", func(t *testing.T) {
		// Last item should be the 50th item of page 3 (index 199 = page3, item49)
		if result.Resources[199].Fields["history_summary"] != "Event p3-49" {
			t.Errorf("last resource history_summary: expected %q, got %q", "Event p3-49", result.Resources[199].Fields["history_summary"])
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
