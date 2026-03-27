package unit

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	sfntypes "github.com/aws/aws-sdk-go-v2/service/sfn/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// SFN Execution History fetcher tests (Level 2 child of SFN Executions)
// ---------------------------------------------------------------------------

// TestFetchSFNExecutionHistory_Basic verifies parsing of 3 events
// (ExecutionStarted, TaskScheduled, TaskSucceeded) with all key fields:
// Resource.ID, Name, Status, Fields keys, and RawStruct.
func TestFetchSFNExecutionHistory_Basic(t *testing.T) {
	ts1 := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)
	ts2 := time.Date(2024, 6, 15, 10, 0, 1, 0, time.UTC)
	ts3 := time.Date(2024, 6, 15, 10, 0, 5, 0, time.UTC)

	mock := &mockSFNGetExecutionHistoryClient{
		output: &sfn.GetExecutionHistoryOutput{
			Events: []sfntypes.HistoryEvent{
				{
					Id:        1,
					Timestamp: &ts1,
					Type:      sfntypes.HistoryEventTypeExecutionStarted,
					ExecutionStartedEventDetails: &sfntypes.ExecutionStartedEventDetails{
						Input: aws.String(`{"key":"value"}`),
					},
				},
				{
					Id:              2,
					PreviousEventId: 1,
					Timestamp:       &ts2,
					Type:            sfntypes.HistoryEventTypeTaskScheduled,
					TaskScheduledEventDetails: &sfntypes.TaskScheduledEventDetails{
						Resource:     aws.String("lambda:invoke"),
						ResourceType: aws.String("lambda"),
					},
				},
				{
					Id:              3,
					PreviousEventId: 2,
					Timestamp:       &ts3,
					Type:            sfntypes.HistoryEventTypeTaskSucceeded,
					TaskSucceededEventDetails: &sfntypes.TaskSucceededEventDetails{
						Resource:     aws.String("lambda:invoke"),
						ResourceType: aws.String("lambda"),
					},
				},
			},
		},
	}

	parentCtx := map[string]string{
		"execution_arn": "arn:aws:states:us-east-1:123456789012:execution:my-sm:exec-001",
	}

	resultSFN, err := awsclient.FetchSFNExecutionHistory(
		context.Background(),
		mock,
		parentCtx,
		"",
	)
	resources := resultSFN.Resources
	_ = resources
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 3 {
		t.Fatalf("expected 3 resources, got %d", len(resources))
	}

	t.Run("first_event_ID", func(t *testing.T) {
		if resources[0].ID != "1" {
			t.Errorf("ID: expected %q, got %q", "1", resources[0].ID)
		}
	})

	t.Run("first_event_Name_humanized", func(t *testing.T) {
		// ExecutionStarted -> "Execution Started"
		if resources[0].Name == "" {
			t.Error("Name should not be empty")
		}
	})

	t.Run("first_event_Status_active", func(t *testing.T) {
		// ExecutionStarted maps to "active"
		if resources[0].Status != "active" {
			t.Errorf("Status: expected %q, got %q", "active", resources[0].Status)
		}
	})

	t.Run("second_event_Status_pending", func(t *testing.T) {
		// TaskScheduled maps to "pending"
		if resources[1].Status != "pending" {
			t.Errorf("Status: expected %q, got %q", "pending", resources[1].Status)
		}
	})

	t.Run("third_event_Status_succeeded", func(t *testing.T) {
		// TaskSucceeded maps to "succeeded"
		if resources[2].Status != "succeeded" {
			t.Errorf("Status: expected %q, got %q", "succeeded", resources[2].Status)
		}
	})

	t.Run("Fields_timestamp", func(t *testing.T) {
		if resources[0].Fields["timestamp"] != "2024-06-15 10:00:00" {
			t.Errorf("Fields[timestamp]: expected %q, got %q",
				"2024-06-15 10:00:00", resources[0].Fields["timestamp"])
		}
	})

	t.Run("Fields_event_type", func(t *testing.T) {
		if resources[0].Fields["event_type"] != "ExecutionStarted" {
			t.Errorf("Fields[event_type]: expected %q, got %q",
				"ExecutionStarted", resources[0].Fields["event_type"])
		}
	})

	t.Run("Fields_event_type_short", func(t *testing.T) {
		if resources[0].Fields["event_type_short"] != "Execution Started" {
			t.Errorf("Fields[event_type_short]: expected %q, got %q",
				"Execution Started", resources[0].Fields["event_type_short"])
		}
	})

	t.Run("Fields_event_id", func(t *testing.T) {
		if resources[0].Fields["event_id"] != "1" {
			t.Errorf("Fields[event_id]: expected %q, got %q", "1", resources[0].Fields["event_id"])
		}
	})

	t.Run("Fields_previous_event_id", func(t *testing.T) {
		if resources[1].Fields["previous_event_id"] != "1" {
			t.Errorf("Fields[previous_event_id]: expected %q, got %q", "1", resources[1].Fields["previous_event_id"])
		}
	})

	t.Run("Fields_event_detail", func(t *testing.T) {
		// ExecutionStarted with Input should have detail
		if resources[0].Fields["event_detail"] == "" || resources[0].Fields["event_detail"] == "\u2014" {
			t.Error("Fields[event_detail] should contain input for ExecutionStarted")
		}
	})

	t.Run("RawStruct_is_HistoryEvent", func(t *testing.T) {
		if resources[0].RawStruct == nil {
			t.Fatal("RawStruct must not be nil")
		}
		raw, ok := resources[0].RawStruct.(sfntypes.HistoryEvent)
		if !ok {
			t.Fatalf("RawStruct should be sfntypes.HistoryEvent, got %T", resources[0].RawStruct)
		}
		if raw.Id != 1 {
			t.Errorf("RawStruct.Id: expected 1, got %d", raw.Id)
		}
	})

	t.Run("required_fields_present", func(t *testing.T) {
		requiredFields := []string{
			"timestamp", "event_type", "event_type_short",
			"state_name", "event_detail", "event_id", "previous_event_id",
		}
		for _, key := range requiredFields {
			if _, ok := resources[0].Fields[key]; !ok {
				t.Errorf("Fields missing key %q", key)
			}
		}
	})
}

// TestFetchSFNExecutionHistory_Empty verifies that an execution with no history
// events returns an empty slice with no error.
func TestFetchSFNExecutionHistory_Empty(t *testing.T) {
	mock := &mockSFNGetExecutionHistoryClient{
		output: &sfn.GetExecutionHistoryOutput{
			Events: []sfntypes.HistoryEvent{},
		},
	}

	parentCtx := map[string]string{
		"execution_arn": "arn:aws:states:us-east-1:123456789012:execution:sm:exec-empty",
	}

	resultSFN, err := awsclient.FetchSFNExecutionHistory(
		context.Background(),
		mock,
		parentCtx,
		"",
	)
	resources := resultSFN.Resources
	_ = resources
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}

// TestFetchSFNExecutionHistory_APIError verifies that API errors are propagated.
func TestFetchSFNExecutionHistory_APIError(t *testing.T) {
	mock := &mockSFNGetExecutionHistoryClient{
		err: fmt.Errorf("AWS API error: access denied"),
	}

	parentCtx := map[string]string{
		"execution_arn": "arn:aws:states:us-east-1:123456789012:execution:sm:exec-err",
	}

	resultSFN, err := awsclient.FetchSFNExecutionHistory(
		context.Background(),
		mock,
		parentCtx,
		"",
	)
	resources := resultSFN.Resources
	_ = resources
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "access denied") {
		t.Errorf("error should contain 'access denied', got %q", err.Error())
	}
	if resources != nil {
		t.Errorf("expected nil resources on error, got %d", len(resources))
	}
}

// TestFetchSFNExecutionHistory_Pagination verifies that paginated responses
// via NextToken are followed and all events collected across multiple pages.
func TestFetchSFNExecutionHistory_Pagination(t *testing.T) {
	ts := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)

	mock := &mockSFNGetExecutionHistoryClient{
		outputs: []*sfn.GetExecutionHistoryOutput{
			{
				NextToken: aws.String("page2-token"),
				Events: []sfntypes.HistoryEvent{
					{
						Id:        1,
						Timestamp: &ts,
						Type:      sfntypes.HistoryEventTypeExecutionStarted,
					},
					{
						Id:              2,
						PreviousEventId: 1,
						Timestamp:       &ts,
						Type:            sfntypes.HistoryEventTypeTaskStateEntered,
						StateEnteredEventDetails: &sfntypes.StateEnteredEventDetails{
							Name: aws.String("ProcessOrder"),
						},
					},
				},
			},
			{
				// No NextToken -- last page
				Events: []sfntypes.HistoryEvent{
					{
						Id:              3,
						PreviousEventId: 2,
						Timestamp:       &ts,
						Type:            sfntypes.HistoryEventTypeTaskSucceeded,
					},
				},
			},
		},
	}

	parentCtx := map[string]string{
		"execution_arn": "arn:aws:states:us-east-1:123456789012:execution:sm:exec-paginated",
	}

	resultSFN, err := awsclient.FetchSFNExecutionHistory(
		context.Background(),
		mock,
		parentCtx,
		"",
	)
	resources := resultSFN.Resources
	_ = resources
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 3 {
		t.Fatalf("expected 3 resources across 2 pages, got %d", len(resources))
	}

	if resources[0].ID != "1" {
		t.Errorf("first resource ID: expected %q, got %q", "1", resources[0].ID)
	}
	if resources[2].ID != "3" {
		t.Errorf("last resource ID: expected %q, got %q", "3", resources[2].ID)
	}
}

// TestFetchSFNExecutionHistory_MaxCap verifies that the fetcher caps results
// at 500 events even when the API returns more.
func TestFetchSFNExecutionHistory_MaxCap(t *testing.T) {
	ts := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)

	// Build a single page with 600 events (exceeds the 500 cap)
	events := make([]sfntypes.HistoryEvent, 600)
	for i := range events {
		events[i] = sfntypes.HistoryEvent{
			Id:        int64(i + 1),
			Timestamp: &ts,
			Type:      sfntypes.HistoryEventTypeTaskSucceeded,
		}
	}

	mock := &mockSFNGetExecutionHistoryClient{
		output: &sfn.GetExecutionHistoryOutput{
			Events: events,
		},
	}

	parentCtx := map[string]string{
		"execution_arn": "arn:aws:states:us-east-1:123456789012:execution:sm:exec-big",
	}

	resultSFN, err := awsclient.FetchSFNExecutionHistory(
		context.Background(),
		mock,
		parentCtx,
		"",
	)
	resources := resultSFN.Resources
	_ = resources
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 500 {
		t.Errorf("expected 500 resources (capped), got %d", len(resources))
	}
}

// ---------------------------------------------------------------------------
// Computed field tests: HumanizeEventType
// ---------------------------------------------------------------------------

// TestHumanizeEventType verifies CamelCase-to-spaced conversion.
func TestHumanizeEventType(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"TaskFailed", "TaskFailed", "Task Failed"},
		{"ExecutionStarted", "ExecutionStarted", "Execution Started"},
		{"LambdaFunctionScheduled", "LambdaFunctionScheduled", "Lambda Function Scheduled"},
		{"StateEntered", "StateEntered", "State Entered"},
		{"TaskStateEntered", "TaskStateEntered", "Task State Entered"},
		{"MapRunStarted", "MapRunStarted", "Map Run Started"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := awsclient.HumanizeEventType(tc.input)
			if got != tc.want {
				t.Errorf("HumanizeEventType(%q): expected %q, got %q",
					tc.input, tc.want, got)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Computed field tests: ClassifyEventStatus
// ---------------------------------------------------------------------------

// TestClassifyEventStatus verifies the mapping of event types to synthetic
// status strings used for row coloring.
func TestClassifyEventStatus(t *testing.T) {
	tests := []struct {
		name      string
		eventType string
		want      string
	}{
		// Active — ExecutionStarted must be checked BEFORE generic *Started
		{"ExecutionStarted_active", "ExecutionStarted", "active"},

		// Succeeded
		{"TaskSucceeded", "TaskSucceeded", "succeeded"},
		{"LambdaFunctionSucceeded", "LambdaFunctionSucceeded", "succeeded"},
		{"StateExited", "ChoiceStateExited", "succeeded"},
		{"ExecutionSucceeded", "ExecutionSucceeded", "succeeded"},

		// Failed
		{"TaskFailed", "TaskFailed", "failed"},
		{"LambdaFunctionTimedOut", "LambdaFunctionTimedOut", "failed"},
		{"ExecutionAborted", "ExecutionAborted", "failed"},
		{"ExecutionTimedOut", "ExecutionTimedOut", "failed"},

		// Pending
		{"TaskScheduled", "TaskScheduled", "pending"},
		{"TaskStarted", "TaskStarted", "pending"},
		{"StateEntered", "TaskStateEntered", "pending"},
		{"MapRunStarted", "MapRunStarted", "pending"},

		// Unknown -> active
		{"unknown_event", "SomethingUnknown", "active"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := awsclient.ClassifyEventStatus(tc.eventType)
			if got != tc.want {
				t.Errorf("ClassifyEventStatus(%q): expected %q, got %q",
					tc.eventType, tc.want, got)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Computed field tests: ExtractEventDetail
// ---------------------------------------------------------------------------

// TestExtractEventDetail_Failed verifies that TaskFailedEventDetails
// containing Error and Cause are included in the extracted detail string.
func TestExtractEventDetail_Failed(t *testing.T) {
	event := sfntypes.HistoryEvent{
		Id:   1,
		Type: sfntypes.HistoryEventTypeTaskFailed,
		TaskFailedEventDetails: &sfntypes.TaskFailedEventDetails{
			Resource:     aws.String("lambda:invoke"),
			ResourceType: aws.String("lambda"),
			Error:        aws.String("States.TaskFailed"),
			Cause:        aws.String("Lambda function returned error"),
		},
	}

	detail := awsclient.ExtractEventDetail(event)
	if !strings.Contains(detail, "States.TaskFailed") {
		t.Errorf("detail should contain Error, got %q", detail)
	}
	if !strings.Contains(detail, "Lambda function returned error") {
		t.Errorf("detail should contain Cause, got %q", detail)
	}
}

// TestExtractEventDetail_StateEntered verifies that StateEnteredEventDetails
// with Input are included in the extracted detail string.
func TestExtractEventDetail_StateEntered(t *testing.T) {
	event := sfntypes.HistoryEvent{
		Id:   1,
		Type: sfntypes.HistoryEventTypeTaskStateEntered,
		StateEnteredEventDetails: &sfntypes.StateEnteredEventDetails{
			Name:  aws.String("ProcessOrder"),
			Input: aws.String(`{"orderId":"12345"}`),
		},
	}

	detail := awsclient.ExtractEventDetail(event)
	if !strings.Contains(detail, "orderId") {
		t.Errorf("detail should contain input data, got %q", detail)
	}
}

// TestExtractEventDetail_Empty verifies that an event with no detail fields
// returns the em-dash placeholder.
func TestExtractEventDetail_Empty(t *testing.T) {
	event := sfntypes.HistoryEvent{
		Id:   1,
		Type: sfntypes.HistoryEventTypeTaskStarted,
		// No detail fields populated
	}

	detail := awsclient.ExtractEventDetail(event)
	if detail != "\u2014" {
		t.Errorf("expected em-dash placeholder, got %q", detail)
	}
}

// TestExtractEventDetail_NewlineStripped verifies that newlines in event
// detail strings are removed (design spec requirement).
func TestExtractEventDetail_NewlineStripped(t *testing.T) {
	event := sfntypes.HistoryEvent{
		Id:   1,
		Type: sfntypes.HistoryEventTypeTaskFailed,
		TaskFailedEventDetails: &sfntypes.TaskFailedEventDetails{
			Resource:     aws.String("lambda:invoke"),
			ResourceType: aws.String("lambda"),
			Error:        aws.String("Error\nwith\nnewlines"),
			Cause:        aws.String("Cause\nwith\nnewlines"),
		},
	}

	detail := awsclient.ExtractEventDetail(event)
	if strings.Contains(detail, "\n") {
		t.Errorf("detail should not contain newlines, got %q", detail)
	}
}

// ---------------------------------------------------------------------------
// State name tracking tests
// ---------------------------------------------------------------------------

// TestConvertHistoryEvent_StateName_FromStateEntered verifies that a
// StateEntered event extracts the state name from StateEnteredEventDetails.Name.
func TestConvertHistoryEvent_StateName_FromStateEntered(t *testing.T) {
	ts := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)
	event := sfntypes.HistoryEvent{
		Id:        1,
		Timestamp: &ts,
		Type:      sfntypes.HistoryEventTypeTaskStateEntered,
		StateEnteredEventDetails: &sfntypes.StateEnteredEventDetails{
			Name: aws.String("ProcessOrder"),
		},
	}

	var lastStateName string
	r := awsclient.ConvertHistoryEvent(event, &lastStateName)

	if r.Fields["state_name"] != "ProcessOrder" {
		t.Errorf("state_name: expected %q, got %q", "ProcessOrder", r.Fields["state_name"])
	}
	if lastStateName != "ProcessOrder" {
		t.Errorf("lastStateName should be updated to %q, got %q", "ProcessOrder", lastStateName)
	}
}

// TestConvertHistoryEvent_StateName_Inherited verifies that a Task event
// after a StateEntered event inherits the state name via the lastStateName pointer.
func TestConvertHistoryEvent_StateName_Inherited(t *testing.T) {
	ts := time.Date(2024, 6, 15, 10, 0, 1, 0, time.UTC)
	event := sfntypes.HistoryEvent{
		Id:              2,
		PreviousEventId: 1,
		Timestamp:       &ts,
		Type:            sfntypes.HistoryEventTypeTaskScheduled,
		TaskScheduledEventDetails: &sfntypes.TaskScheduledEventDetails{
			Resource:     aws.String("lambda:invoke"),
			ResourceType: aws.String("lambda"),
		},
	}

	lastStateName := "ProcessOrder"
	r := awsclient.ConvertHistoryEvent(event, &lastStateName)

	if r.Fields["state_name"] != "ProcessOrder" {
		t.Errorf("state_name: expected %q (inherited), got %q", "ProcessOrder", r.Fields["state_name"])
	}
}

// TestConvertHistoryEvent_StateName_ExecutionLevel verifies that an
// ExecutionStarted event (no state context) has state_name set to em-dash.
func TestConvertHistoryEvent_StateName_ExecutionLevel(t *testing.T) {
	ts := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)
	event := sfntypes.HistoryEvent{
		Id:        1,
		Timestamp: &ts,
		Type:      sfntypes.HistoryEventTypeExecutionStarted,
		ExecutionStartedEventDetails: &sfntypes.ExecutionStartedEventDetails{
			Input: aws.String(`{"key":"value"}`),
		},
	}

	var lastStateName string
	r := awsclient.ConvertHistoryEvent(event, &lastStateName)

	if r.Fields["state_name"] != "\u2014" {
		t.Errorf("state_name: expected em-dash for execution-level event, got %q", r.Fields["state_name"])
	}
}

// ---------------------------------------------------------------------------
// Registration tests
// ---------------------------------------------------------------------------

// TestSFNExecutionHistoryColumns verifies that SFNExecutionHistoryColumns
// returns the expected 4 columns with correct keys, titles, and widths.
func TestSFNExecutionHistoryColumns(t *testing.T) {
	cols := resource.SFNExecutionHistoryColumns()

	expectedKeys := []string{"timestamp", "event_type_short", "state_name", "event_detail"}

	t.Run("column_count", func(t *testing.T) {
		if len(cols) != 4 {
			t.Fatalf("expected 4 columns, got %d", len(cols))
		}
	})

	t.Run("column_keys", func(t *testing.T) {
		for i, wantKey := range expectedKeys {
			if cols[i].Key != wantKey {
				t.Errorf("col[%d].Key: expected %q, got %q", i, wantKey, cols[i].Key)
			}
		}
	})

	t.Run("column_titles", func(t *testing.T) {
		wantTitles := []string{"Timestamp", "Event Type", "State", "Detail"}
		for i, wantTitle := range wantTitles {
			if cols[i].Title != wantTitle {
				t.Errorf("col[%d].Title: expected %q, got %q", i, wantTitle, cols[i].Title)
			}
		}
	})

	t.Run("column_widths", func(t *testing.T) {
		wantWidths := []int{22, 24, 24, 40}
		for i, wantWidth := range wantWidths {
			if cols[i].Width != wantWidth {
				t.Errorf("col[%d].Width: expected %d, got %d", i, wantWidth, cols[i].Width)
			}
		}
	})

	t.Run("sortable_flags", func(t *testing.T) {
		// timestamp, event_type_short, state_name are sortable; event_detail is not
		wantSortable := []bool{true, true, true, false}
		for i, want := range wantSortable {
			if cols[i].Sortable != want {
				t.Errorf("col[%d].Sortable: expected %v, got %v", i, want, cols[i].Sortable)
			}
		}
	})
}

// TestSFNExecutionHistory_ChildTypeRegistered verifies that the child type
// is registered under the correct short name.
func TestSFNExecutionHistory_ChildTypeRegistered(t *testing.T) {
	td := resource.GetChildType("sfn_execution_history")
	if td == nil {
		t.Fatal("sfn_execution_history child resource type not registered")
	}
	if td.Name == "" {
		t.Error("child type Name should not be empty")
	}
	if td.ShortName != "sfn_execution_history" {
		t.Errorf("child type ShortName: expected %q, got %q",
			"sfn_execution_history", td.ShortName)
	}
}

// TestSFNExecutionHistory_PaginatedChildFetcherRegistered verifies that the paginated
// child fetcher is registered under the correct short name.
func TestSFNExecutionHistory_PaginatedChildFetcherRegistered(t *testing.T) {
	f := resource.GetPaginatedChildFetcher("sfn_execution_history")
	if f == nil {
		t.Fatal("sfn_execution_history paginated child fetcher not registered")
	}
}

// TestSFNExecutionHistory_ParentHasChildDef verifies that the parent
// sfn_executions child type has a child view definition for
// sfn_execution_history with key "enter".
func TestSFNExecutionHistory_ParentHasChildDef(t *testing.T) {
	td := resource.GetChildType("sfn_executions")
	if td == nil {
		t.Fatal("sfn_executions child type not found")
	}

	found := false
	for _, child := range td.Children {
		if child.ChildType == "sfn_execution_history" {
			found = true
			if child.Key != "enter" {
				t.Errorf("child Key: expected %q, got %q", "enter", child.Key)
			}
			if child.ContextKeys["execution_arn"] != "execution_arn" {
				t.Errorf("ContextKeys[execution_arn]: expected %q, got %q",
					"execution_arn", child.ContextKeys["execution_arn"])
			}
			if child.ContextKeys["execution_name"] != "Name" {
				t.Errorf("ContextKeys[execution_name]: expected %q, got %q",
					"Name", child.ContextKeys["execution_name"])
			}
			if child.DisplayNameKey != "execution_name" {
				t.Errorf("DisplayNameKey: expected %q, got %q",
					"execution_name", child.DisplayNameKey)
			}
			break
		}
	}
	if !found {
		t.Error("sfn_executions should have child view def for sfn_execution_history")
	}
}

// TestSFNExecutions_CopyField verifies that the sfn_executions child type
// has CopyField set to "execution_arn".
func TestSFNExecutions_CopyField(t *testing.T) {
	td := resource.GetChildType("sfn_executions")
	if td == nil {
		t.Fatal("sfn_executions child type not found")
	}
	if td.CopyField != "execution_arn" {
		t.Errorf("CopyField: expected %q, got %q", "execution_arn", td.CopyField)
	}
}

// TestSFNExecutionHistory_CopyField verifies that the sfn_execution_history
// child type has CopyField set to "event_detail".
func TestSFNExecutionHistory_CopyField(t *testing.T) {
	td := resource.GetChildType("sfn_execution_history")
	if td == nil {
		t.Fatal("sfn_execution_history child type not found")
	}
	if td.CopyField != "event_detail" {
		t.Errorf("CopyField: expected %q, got %q", "event_detail", td.CopyField)
	}
}

// ---------------------------------------------------------------------------
// Config defaults test
// ---------------------------------------------------------------------------

// TestConfigDefaultViewDef_SFNExecutionHistory verifies that the
// sfn_execution_history view definition has the expected list columns
// and non-empty detail paths.
func TestConfigDefaultViewDef_SFNExecutionHistory(t *testing.T) {
	vd := config.DefaultViewDef("sfn_execution_history")

	t.Run("list_columns", func(t *testing.T) {
		if len(vd.List) < 4 {
			t.Fatalf("expected at least 4 list columns for sfn_execution_history default, got %d", len(vd.List))
		}
	})

	t.Run("detail_paths", func(t *testing.T) {
		if len(vd.Detail) == 0 {
			t.Error("expected non-empty Detail paths for sfn_execution_history")
		}
		// Check for key detail fields
		detailStr := strings.Join(vd.Detail, ",")
		for _, expected := range []string{"Timestamp", "Type", "Id"} {
			if !strings.Contains(detailStr, expected) {
				t.Errorf("Detail should contain %q, got %v", expected, vd.Detail)
			}
		}
	})
}

// Ensure all imports are used.
var _ = aws.String
var _ = sfn.GetExecutionHistoryOutput{}
