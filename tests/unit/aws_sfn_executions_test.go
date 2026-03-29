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
// SFN Executions fetcher tests (child of Step Functions)
// ---------------------------------------------------------------------------

// TestFetchSFNExecutions_Basic verifies parsing of 1 execution with all fields
// populated, checking Resource.ID, Name, Status, all Fields keys, and RawStruct.
func TestFetchSFNExecutions_Basic(t *testing.T) {
	startTs := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)
	stopTs := time.Date(2024, 6, 15, 10, 2, 47, 0, time.UTC)
	redriveTs := time.Date(2024, 6, 15, 11, 0, 0, 0, time.UTC)
	itemCount := int32(42)
	redriveCount := int32(1)

	mock := &mockSFNListExecutionsClient{
		output: &sfn.ListExecutionsOutput{
			Executions: []sfntypes.ExecutionListItem{
				{
					ExecutionArn:           aws.String("arn:aws:states:us-east-1:123456789012:execution:my-state-machine:exec-001"),
					Name:                   aws.String("exec-001"),
					StartDate:              &startTs,
					StopDate:               &stopTs,
					StateMachineArn:        aws.String("arn:aws:states:us-east-1:123456789012:stateMachine:my-state-machine"),
					Status:                 sfntypes.ExecutionStatusSucceeded,
					ItemCount:              &itemCount,
					MapRunArn:              aws.String("arn:aws:states:us-east-1:123456789012:mapRun:my-state-machine/exec-001:map-run-id"),
					RedriveCount:           &redriveCount,
					RedriveDate:            &redriveTs,
					StateMachineAliasArn:   aws.String("arn:aws:states:us-east-1:123456789012:stateMachine:my-state-machine:prod"),
					StateMachineVersionArn: aws.String("arn:aws:states:us-east-1:123456789012:stateMachine:my-state-machine:1"),
				},
			},
		},
	}

	parentCtx := map[string]string{
		"state_machine_arn": "arn:aws:states:us-east-1:123456789012:stateMachine:my-state-machine",
	}

	result, err := awsclient.FetchSFNExecutions(
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

	t.Run("ID_is_execution_name", func(t *testing.T) {
		if r.ID != "exec-001" {
			t.Errorf("ID: expected %q, got %q", "exec-001", r.ID)
		}
	})

	t.Run("Name_is_execution_name", func(t *testing.T) {
		if r.Name != "exec-001" {
			t.Errorf("Name: expected %q, got %q", "exec-001", r.Name)
		}
	})

	t.Run("Status_is_uppercase", func(t *testing.T) {
		if r.Status != "SUCCEEDED" {
			t.Errorf("Status: expected %q, got %q", "SUCCEEDED", r.Status)
		}
	})

	t.Run("Fields_execution_arn", func(t *testing.T) {
		if r.Fields["execution_arn"] != "arn:aws:states:us-east-1:123456789012:execution:my-state-machine:exec-001" {
			t.Errorf("Fields[execution_arn]: expected full ARN, got %q", r.Fields["execution_arn"])
		}
	})

	t.Run("Fields_name", func(t *testing.T) {
		if r.Fields["name"] != "exec-001" {
			t.Errorf("Fields[name]: expected %q, got %q", "exec-001", r.Fields["name"])
		}
	})

	t.Run("Fields_status", func(t *testing.T) {
		if r.Fields["status"] != "SUCCEEDED" {
			t.Errorf("Fields[status]: expected %q, got %q", "SUCCEEDED", r.Fields["status"])
		}
	})

	t.Run("Fields_start_date", func(t *testing.T) {
		if r.Fields["start_date"] == "" {
			t.Error("Fields[start_date] should not be empty")
		}
		if !strings.Contains(r.Fields["start_date"], "2024-06-15 10:00") {
			t.Errorf("Fields[start_date] expected '2024-06-15 10:00', got %q", r.Fields["start_date"])
		}
	})

	t.Run("Fields_stop_date", func(t *testing.T) {
		if r.Fields["stop_date"] == "" {
			t.Error("Fields[stop_date] should not be empty")
		}
		if !strings.Contains(r.Fields["stop_date"], "2024-06-15 10:02") {
			t.Errorf("Fields[stop_date] expected '2024-06-15 10:02', got %q", r.Fields["stop_date"])
		}
	})

	t.Run("Fields_duration", func(t *testing.T) {
		if r.Fields["duration"] == "" {
			t.Error("Fields[duration] should not be empty")
		}
		if !strings.Contains(r.Fields["duration"], "2m") {
			t.Errorf("Fields[duration] should contain '2m', got %q", r.Fields["duration"])
		}
	})

	t.Run("Fields_state_machine_arn", func(t *testing.T) {
		if r.Fields["state_machine_arn"] != "arn:aws:states:us-east-1:123456789012:stateMachine:my-state-machine" {
			t.Errorf("Fields[state_machine_arn]: got %q", r.Fields["state_machine_arn"])
		}
	})

	t.Run("Fields_state_machine_alias_arn", func(t *testing.T) {
		if r.Fields["state_machine_alias_arn"] == "" {
			t.Error("Fields[state_machine_alias_arn] should not be empty")
		}
	})

	t.Run("Fields_state_machine_version_arn", func(t *testing.T) {
		if r.Fields["state_machine_version_arn"] == "" {
			t.Error("Fields[state_machine_version_arn] should not be empty")
		}
	})

	t.Run("Fields_map_run_arn", func(t *testing.T) {
		if r.Fields["map_run_arn"] == "" {
			t.Error("Fields[map_run_arn] should not be empty")
		}
	})

	t.Run("Fields_item_count", func(t *testing.T) {
		if r.Fields["item_count"] != "42" {
			t.Errorf("Fields[item_count]: expected %q, got %q", "42", r.Fields["item_count"])
		}
	})

	t.Run("Fields_redrive_count", func(t *testing.T) {
		if r.Fields["redrive_count"] != "1" {
			t.Errorf("Fields[redrive_count]: expected %q, got %q", "1", r.Fields["redrive_count"])
		}
	})

	t.Run("Fields_redrive_date", func(t *testing.T) {
		if r.Fields["redrive_date"] == "" {
			t.Error("Fields[redrive_date] should not be empty")
		}
	})

	t.Run("RawStruct_is_ExecutionListItem", func(t *testing.T) {
		if r.RawStruct == nil {
			t.Fatal("RawStruct must not be nil")
		}
		raw, ok := r.RawStruct.(sfntypes.ExecutionListItem)
		if !ok {
			t.Fatalf("RawStruct should be sfntypes.ExecutionListItem, got %T", r.RawStruct)
		}
		if raw.ExecutionArn == nil || *raw.ExecutionArn != "arn:aws:states:us-east-1:123456789012:execution:my-state-machine:exec-001" {
			t.Error("RawStruct.ExecutionArn not preserved correctly")
		}
	})

	// Verify all expected fields are present
	t.Run("required_fields_present", func(t *testing.T) {
		requiredFields := []string{
			"execution_arn", "name", "status", "start_date", "stop_date",
			"duration", "state_machine_arn", "state_machine_alias_arn",
			"state_machine_version_arn", "map_run_arn", "item_count",
			"redrive_count", "redrive_date",
		}
		for _, key := range requiredFields {
			if _, ok := r.Fields[key]; !ok {
				t.Errorf("Fields missing key %q", key)
			}
		}
	})
}

// TestFetchSFNExecutions_Empty verifies that a state machine with no executions
// returns an empty slice with no error.
func TestFetchSFNExecutions_Empty(t *testing.T) {
	mock := &mockSFNListExecutionsClient{
		output: &sfn.ListExecutionsOutput{
			Executions: []sfntypes.ExecutionListItem{},
		},
	}

	parentCtx := map[string]string{
		"state_machine_arn": "arn:aws:states:us-east-1:123456789012:stateMachine:empty-sm",
	}

	result, err := awsclient.FetchSFNExecutions(
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

// TestFetchSFNExecutions_APIError verifies that API errors are propagated.
func TestFetchSFNExecutions_APIError(t *testing.T) {
	mock := &mockSFNListExecutionsClient{
		err: fmt.Errorf("AWS API error: access denied"),
	}

	parentCtx := map[string]string{
		"state_machine_arn": "arn:aws:states:us-east-1:123456789012:stateMachine:err-sm",
	}

	result, err := awsclient.FetchSFNExecutions(
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

// TestFetchSFNExecutions_NilFields verifies that nil optional fields
// (StopDate, MapRunArn, StateMachineAliasArn, StateMachineVersionArn,
// ItemCount, RedriveCount, RedriveDate) do not cause a panic.
func TestFetchSFNExecutions_NilFields(t *testing.T) {
	startTs := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)

	mock := &mockSFNListExecutionsClient{
		output: &sfn.ListExecutionsOutput{
			Executions: []sfntypes.ExecutionListItem{
				{
					ExecutionArn:    aws.String("arn:aws:states:us-east-1:123456789012:execution:sm:exec-nil"),
					Name:            aws.String("exec-nil"),
					StartDate:       &startTs,
					StateMachineArn: aws.String("arn:aws:states:us-east-1:123456789012:stateMachine:sm"),
					Status:          sfntypes.ExecutionStatusRunning,
					// All optional fields are nil:
					// StopDate, MapRunArn, StateMachineAliasArn,
					// StateMachineVersionArn, ItemCount, RedriveCount, RedriveDate
				},
			},
		},
	}

	parentCtx := map[string]string{
		"state_machine_arn": "arn:aws:states:us-east-1:123456789012:stateMachine:sm",
	}

	// Should not panic
	result, err := awsclient.FetchSFNExecutions(
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

	t.Run("nil_StopDate", func(t *testing.T) {
		r := result.Resources[0]
		if r.Fields["stop_date"] != "" {
			t.Logf("Fields[stop_date] is %q (expected empty for nil)", r.Fields["stop_date"])
		}
	})

	t.Run("nil_MapRunArn", func(t *testing.T) {
		r := result.Resources[0]
		if r.Fields["map_run_arn"] != "" {
			t.Logf("Fields[map_run_arn] is %q (expected empty for nil)", r.Fields["map_run_arn"])
		}
	})

	t.Run("nil_StateMachineAliasArn", func(t *testing.T) {
		r := result.Resources[0]
		if r.Fields["state_machine_alias_arn"] != "" {
			t.Logf("Fields[state_machine_alias_arn] is %q (expected empty for nil)", r.Fields["state_machine_alias_arn"])
		}
	})

	t.Run("nil_StateMachineVersionArn", func(t *testing.T) {
		r := result.Resources[0]
		if r.Fields["state_machine_version_arn"] != "" {
			t.Logf("Fields[state_machine_version_arn] is %q (expected empty for nil)", r.Fields["state_machine_version_arn"])
		}
	})

	t.Run("nil_ItemCount", func(t *testing.T) {
		r := result.Resources[0]
		if r.Fields["item_count"] != "" {
			t.Logf("Fields[item_count] is %q (expected empty for nil)", r.Fields["item_count"])
		}
	})

	t.Run("nil_RedriveCount", func(t *testing.T) {
		r := result.Resources[0]
		if r.Fields["redrive_count"] != "" {
			t.Logf("Fields[redrive_count] is %q (expected empty for nil)", r.Fields["redrive_count"])
		}
	})

	t.Run("nil_RedriveDate", func(t *testing.T) {
		r := result.Resources[0]
		if r.Fields["redrive_date"] != "" {
			t.Logf("Fields[redrive_date] is %q (expected empty for nil)", r.Fields["redrive_date"])
		}
	})

	t.Run("status_populated", func(t *testing.T) {
		r := result.Resources[0]
		if r.Status != "RUNNING" {
			t.Errorf("Status: expected %q, got %q", "RUNNING", r.Status)
		}
	})
}

// TestFetchSFNExecutions_Pagination verifies that paginated responses via
// NextToken are followed and all executions collected across multiple pages.
// TestFetchSFNExecutions_Pagination verifies the single-page pagination contract:
// one API call is made per invocation, resources from that page are returned,
// and IsTruncated/NextToken reflect whether more pages exist. A second call
// with the continuation token verifies the token is forwarded and the final
// page sets IsTruncated=false.
func TestFetchSFNExecutions_Pagination(t *testing.T) {
	startTs := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)
	stopTs := time.Date(2024, 6, 15, 10, 5, 0, 0, time.UTC)

	parentCtx := map[string]string{
		"state_machine_arn": "arn:aws:states:us-east-1:123456789012:stateMachine:sm",
	}

	// Page 1: 3 executions with NextToken indicating more pages exist.
	page1Mock := &mockSFNListExecutionsClient{
		outputs: []*sfn.ListExecutionsOutput{
			{
				NextToken: aws.String("page2-token"),
				Executions: []sfntypes.ExecutionListItem{
					{
						ExecutionArn:    aws.String("arn:aws:states:us-east-1:123456789012:execution:sm:exec-p1-1"),
						Name:            aws.String("exec-p1-1"),
						StartDate:       &startTs,
						StopDate:        &stopTs,
						StateMachineArn: aws.String("arn:aws:states:us-east-1:123456789012:stateMachine:sm"),
						Status:          sfntypes.ExecutionStatusSucceeded,
					},
					{
						ExecutionArn:    aws.String("arn:aws:states:us-east-1:123456789012:execution:sm:exec-p1-2"),
						Name:            aws.String("exec-p1-2"),
						StartDate:       &startTs,
						StateMachineArn: aws.String("arn:aws:states:us-east-1:123456789012:stateMachine:sm"),
						Status:          sfntypes.ExecutionStatusRunning,
					},
					{
						ExecutionArn:    aws.String("arn:aws:states:us-east-1:123456789012:execution:sm:exec-p1-3"),
						Name:            aws.String("exec-p1-3"),
						StartDate:       &startTs,
						StopDate:        &stopTs,
						StateMachineArn: aws.String("arn:aws:states:us-east-1:123456789012:stateMachine:sm"),
						Status:          sfntypes.ExecutionStatusFailed,
					},
				},
			},
		},
	}

	// First call: no continuation token — fetches page 1.
	result1, err := awsclient.FetchSFNExecutions(context.Background(), page1Mock, parentCtx, "")
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

	t.Run("page1_execution_ids", func(t *testing.T) {
		expectedIDs := []string{"exec-p1-1", "exec-p1-2", "exec-p1-3"}
		for i, expectedID := range expectedIDs {
			if result1.Resources[i].ID != expectedID {
				t.Errorf("resources[%d].ID: expected %q, got %q", i, expectedID, result1.Resources[i].ID)
			}
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
		requiredFields := []string{"execution_arn", "name", "status", "start_date"}
		for i, r := range result1.Resources {
			for _, key := range requiredFields {
				if _, ok := r.Fields[key]; !ok {
					t.Errorf("page 1: resource[%d].Fields missing key %q", i, key)
				}
			}
		}
	})

	// Page 2: 2 executions with no NextToken — last page.
	page2Mock := &mockSFNListExecutionsClient{
		outputs: []*sfn.ListExecutionsOutput{
			{
				// No NextToken — last page
				Executions: []sfntypes.ExecutionListItem{
					{
						ExecutionArn:    aws.String("arn:aws:states:us-east-1:123456789012:execution:sm:exec-p2-1"),
						Name:            aws.String("exec-p2-1"),
						StartDate:       &startTs,
						StopDate:        &stopTs,
						StateMachineArn: aws.String("arn:aws:states:us-east-1:123456789012:stateMachine:sm"),
						Status:          sfntypes.ExecutionStatusAborted,
					},
					{
						ExecutionArn:    aws.String("arn:aws:states:us-east-1:123456789012:execution:sm:exec-p2-2"),
						Name:            aws.String("exec-p2-2"),
						StartDate:       &startTs,
						StopDate:        &stopTs,
						StateMachineArn: aws.String("arn:aws:states:us-east-1:123456789012:stateMachine:sm"),
						Status:          sfntypes.ExecutionStatusTimedOut,
					},
				},
			},
		},
	}

	// Second call: pass continuation token from page 1 to fetch page 2.
	result2, err := awsclient.FetchSFNExecutions(context.Background(), page2Mock, parentCtx, result1.Pagination.NextToken)
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

	t.Run("page2_execution_ids", func(t *testing.T) {
		expectedIDs := []string{"exec-p2-1", "exec-p2-2"}
		for i, expectedID := range expectedIDs {
			if result2.Resources[i].ID != expectedID {
				t.Errorf("page 2: resources[%d].ID: expected %q, got %q", i, expectedID, result2.Resources[i].ID)
			}
		}
	})
}

// TestFetchSFNExecutions_MaxCap verifies that a single API page of 50
// executions is returned as-is with correct IsTruncated=true metadata when the
// API indicates more pages exist. The 200-item cap no longer applies — each
// call returns one page and the caller drives pagination via continuation tokens.
func TestFetchSFNExecutions_MaxCap(t *testing.T) {
	startTs := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)
	stopTs := time.Date(2024, 6, 15, 12, 5, 0, 0, time.UTC)

	// Build one page of 50 executions with a NextToken indicating more pages exist.
	var executions []sfntypes.ExecutionListItem
	for i := 0; i < 50; i++ {
		executions = append(executions, sfntypes.ExecutionListItem{
			ExecutionArn:    aws.String(fmt.Sprintf("arn:aws:states:us-east-1:123456789012:execution:sm:exec-p0-%d", i)),
			Name:            aws.String(fmt.Sprintf("exec-p0-%d", i)),
			StartDate:       &startTs,
			StopDate:        &stopTs,
			StateMachineArn: aws.String("arn:aws:states:us-east-1:123456789012:stateMachine:sm"),
			Status:          sfntypes.ExecutionStatusSucceeded,
		})
	}

	mock := &mockSFNListExecutionsClient{
		outputs: []*sfn.ListExecutionsOutput{
			{
				Executions: executions,
				NextToken:  aws.String("token-page-1"),
			},
		},
	}

	parentCtx := map[string]string{
		"state_machine_arn": "arn:aws:states:us-east-1:123456789012:stateMachine:sm",
	}

	result, err := awsclient.FetchSFNExecutions(context.Background(), mock, parentCtx, "")
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

	t.Run("first_execution_correct", func(t *testing.T) {
		if result.Resources[0].ID != "exec-p0-0" {
			t.Errorf("first resource ID: expected %q, got %q", "exec-p0-0", result.Resources[0].ID)
		}
	})

	t.Run("last_execution_correct", func(t *testing.T) {
		if result.Resources[49].ID != "exec-p0-49" {
			t.Errorf("last resource ID: expected %q, got %q", "exec-p0-49", result.Resources[49].ID)
		}
	})
}

// TestFetchSFNExecutions_DurationComputed verifies that a SUCCEEDED execution
// with known StartDate/StopDate produces a correctly formatted duration string.
func TestFetchSFNExecutions_DurationComputed(t *testing.T) {
	startTs := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)
	stopTs := time.Date(2024, 6, 15, 10, 2, 47, 0, time.UTC) // 2m 47s

	mock := &mockSFNListExecutionsClient{
		output: &sfn.ListExecutionsOutput{
			Executions: []sfntypes.ExecutionListItem{
				{
					ExecutionArn:    aws.String("arn:aws:states:us-east-1:123456789012:execution:sm:dur-exec"),
					Name:            aws.String("dur-exec"),
					StartDate:       &startTs,
					StopDate:        &stopTs,
					StateMachineArn: aws.String("arn:aws:states:us-east-1:123456789012:stateMachine:sm"),
					Status:          sfntypes.ExecutionStatusSucceeded,
				},
			},
		},
	}

	parentCtx := map[string]string{
		"state_machine_arn": "arn:aws:states:us-east-1:123456789012:stateMachine:sm",
	}

	result, err := awsclient.FetchSFNExecutions(
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

	duration := result.Resources[0].Fields["duration"]

	t.Run("contains_minutes_and_seconds", func(t *testing.T) {
		if !strings.Contains(duration, "2m") {
			t.Errorf("duration should contain '2m', got %q", duration)
		}
		if !strings.Contains(duration, "47s") {
			t.Errorf("duration should contain '47s', got %q", duration)
		}
	})

	t.Run("no_approximate_prefix", func(t *testing.T) {
		if strings.HasPrefix(duration, "~") {
			t.Errorf("completed execution duration should not have '~' prefix, got %q", duration)
		}
	})
}

// TestFetchSFNExecutions_DurationRunning verifies that a RUNNING execution
// with nil StopDate produces a duration string starting with "~".
func TestFetchSFNExecutions_DurationRunning(t *testing.T) {
	startTs := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)

	mock := &mockSFNListExecutionsClient{
		output: &sfn.ListExecutionsOutput{
			Executions: []sfntypes.ExecutionListItem{
				{
					ExecutionArn:    aws.String("arn:aws:states:us-east-1:123456789012:execution:sm:running-exec"),
					Name:            aws.String("running-exec"),
					StartDate:       &startTs,
					StateMachineArn: aws.String("arn:aws:states:us-east-1:123456789012:stateMachine:sm"),
					Status:          sfntypes.ExecutionStatusRunning,
					// StopDate is nil for running executions
				},
			},
		},
	}

	parentCtx := map[string]string{
		"state_machine_arn": "arn:aws:states:us-east-1:123456789012:stateMachine:sm",
	}

	result, err := awsclient.FetchSFNExecutions(
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

	duration := result.Resources[0].Fields["duration"]

	t.Run("starts_with_tilde", func(t *testing.T) {
		if !strings.HasPrefix(duration, "~") {
			t.Errorf("running execution duration should start with '~', got %q", duration)
		}
	})
}

// TestFetchSFNExecutions_StatusPreserved verifies that the Status field
// preserves the uppercase SFN status values.
func TestFetchSFNExecutions_StatusPreserved(t *testing.T) {
	startTs := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)
	stopTs := time.Date(2024, 6, 15, 10, 5, 0, 0, time.UTC)

	testCases := []struct {
		status   sfntypes.ExecutionStatus
		expected string
	}{
		{sfntypes.ExecutionStatusSucceeded, "SUCCEEDED"},
		{sfntypes.ExecutionStatusFailed, "FAILED"},
		{sfntypes.ExecutionStatusRunning, "RUNNING"},
		{sfntypes.ExecutionStatusTimedOut, "TIMED_OUT"},
		{sfntypes.ExecutionStatusAborted, "ABORTED"},
		{sfntypes.ExecutionStatusPendingRedrive, "PENDING_REDRIVE"},
	}

	for _, tc := range testCases {
		t.Run(tc.expected, func(t *testing.T) {
			mock := &mockSFNListExecutionsClient{
				output: &sfn.ListExecutionsOutput{
					Executions: []sfntypes.ExecutionListItem{
						{
							ExecutionArn:    aws.String("arn:aws:states:us-east-1:123456789012:execution:sm:status-exec"),
							Name:            aws.String("status-exec"),
							StartDate:       &startTs,
							StopDate:        &stopTs,
							StateMachineArn: aws.String("arn:aws:states:us-east-1:123456789012:stateMachine:sm"),
							Status:          tc.status,
						},
					},
				},
			}

			parentCtx := map[string]string{
				"state_machine_arn": "arn:aws:states:us-east-1:123456789012:stateMachine:sm",
			}

			result, err := awsclient.FetchSFNExecutions(
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

			if result.Resources[0].Status != tc.expected {
				t.Errorf("Status: expected %q, got %q", tc.expected, result.Resources[0].Status)
			}
			if result.Resources[0].Fields["status"] != tc.expected {
				t.Errorf("Fields[status]: expected %q, got %q", tc.expected, result.Resources[0].Fields["status"])
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Registration tests
// ---------------------------------------------------------------------------

// TestSFNExecutionColumns verifies that SFNExecutionColumns returns the expected
// columns with correct keys, titles, and widths.
func TestSFNExecutionColumns(t *testing.T) {
	cols := resource.SFNExecutionColumns()

	expectedKeys := []string{"name", "status", "start_date", "stop_date", "duration"}

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
		expectedWidths := []int{36, 12, 22, 22, 12}
		for i, expected := range expectedWidths {
			if cols[i].Width != expected {
				t.Errorf("column[%d] (%s).Width: expected %d, got %d", i, cols[i].Key, expected, cols[i].Width)
			}
		}
	})

	t.Run("sortable_columns", func(t *testing.T) {
		for i, col := range cols {
			if !col.Sortable {
				t.Errorf("column[%d] (%s) should be sortable", i, col.Key)
			}
		}
	})
}

// TestSFNExecutions_ChildTypeRegistered verifies that the child type is
// registered under the correct short name.
func TestSFNExecutions_ChildTypeRegistered(t *testing.T) {
	td := resource.GetChildType("sfn_executions")
	if td == nil {
		t.Fatal("sfn_executions child resource type not registered")
	}
	if td.Name == "" {
		t.Error("child type Name should not be empty")
	}
	if td.ShortName != "sfn_executions" {
		t.Errorf("child type ShortName: expected %q, got %q", "sfn_executions", td.ShortName)
	}
}

// TestSFNExecutions_PaginatedChildFetcherRegistered verifies that the paginated
// child fetcher is
// registered under the correct short name.
func TestSFNExecutions_PaginatedChildFetcherRegistered(t *testing.T) {
	f := resource.GetPaginatedChildFetcher("sfn_executions")
	if f == nil {
		t.Fatal("sfn_executions paginated child fetcher not registered")
	}
}

// TestSFNExecutions_ParentHasChildDef verifies that the parent sfn resource
// type has a child view definition for sfn_executions with key "enter".
func TestSFNExecutions_ParentHasChildDef(t *testing.T) {
	rt := resource.FindResourceType("sfn")
	if rt == nil {
		t.Fatal("sfn resource type not found")
	}

	found := false
	for _, child := range rt.Children {
		if child.ChildType == "sfn_executions" {
			found = true
			if child.Key != "enter" {
				t.Errorf("expected key %q, got %q", "enter", child.Key)
			}
			if child.ContextKeys["state_machine_arn"] == "" {
				t.Error("ContextKeys should include 'state_machine_arn'")
			}
			if child.ContextKeys["state_machine_name"] == "" {
				t.Error("ContextKeys should include 'state_machine_name'")
			}
			if child.DisplayNameKey != "state_machine_name" {
				t.Errorf("DisplayNameKey: expected %q, got %q", "state_machine_name", child.DisplayNameKey)
			}
		}
	}
	if !found {
		t.Error("sfn Children should contain sfn_executions child view def")
	}
}

// TestSFNExecutions_DrillCondition_BlocksExpress verifies that the DrillCondition
// on the SFN child def blocks Express state machines and allows Standard ones.
func TestSFNExecutions_DrillCondition_BlocksExpress(t *testing.T) {
	rt := resource.FindResourceType("sfn")
	if rt == nil {
		t.Fatal("sfn resource type not found")
	}

	var childDef *resource.ChildViewDef
	for i := range rt.Children {
		if rt.Children[i].ChildType == "sfn_executions" {
			childDef = &rt.Children[i]
			break
		}
	}
	if childDef == nil {
		t.Fatal("sfn_executions child view def not found")
	}

	if childDef.DrillCondition == nil {
		t.Fatal("DrillCondition should not be nil for sfn_executions")
	}

	t.Run("blocks_EXPRESS", func(t *testing.T) {
		expressResource := resource.Resource{
			ID:   "express-sm",
			Name: "express-sm",
			Fields: map[string]string{
				"type": "EXPRESS",
			},
		}
		if childDef.DrillCondition(expressResource) {
			t.Error("DrillCondition should return false for EXPRESS state machines")
		}
	})

	t.Run("allows_STANDARD", func(t *testing.T) {
		standardResource := resource.Resource{
			ID:   "standard-sm",
			Name: "standard-sm",
			Fields: map[string]string{
				"type": "STANDARD",
			},
		}
		if !childDef.DrillCondition(standardResource) {
			t.Error("DrillCondition should return true for STANDARD state machines")
		}
	})

	t.Run("allows_empty_type", func(t *testing.T) {
		emptyResource := resource.Resource{
			ID:     "unknown-sm",
			Name:   "unknown-sm",
			Fields: map[string]string{},
		}
		if !childDef.DrillCondition(emptyResource) {
			t.Error("DrillCondition should return true when type field is absent")
		}
	})
}

// TestSFNExecutions_DrillBlockMessage verifies that the SFN child def has a
// non-empty DrillBlockMessage containing "Express".
func TestSFNExecutions_DrillBlockMessage(t *testing.T) {
	rt := resource.FindResourceType("sfn")
	if rt == nil {
		t.Fatal("sfn resource type not found")
	}

	var childDef *resource.ChildViewDef
	for i := range rt.Children {
		if rt.Children[i].ChildType == "sfn_executions" {
			childDef = &rt.Children[i]
			break
		}
	}
	if childDef == nil {
		t.Fatal("sfn_executions child view def not found")
	}

	if childDef.DrillBlockMessage == "" {
		t.Error("DrillBlockMessage should not be empty for sfn_executions")
	}
	if !strings.Contains(childDef.DrillBlockMessage, "Express") {
		t.Errorf("DrillBlockMessage should contain 'Express', got %q", childDef.DrillBlockMessage)
	}
}

// ---------------------------------------------------------------------------
// formatHumanDuration tests
// ---------------------------------------------------------------------------

// TestFormatHumanDuration verifies the duration formatting helper for various
// durations: seconds only, minutes+seconds, hours+minutes, days+hours.
func TestFormatHumanDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{"zero", 0, "0s"},
		{"seconds_only", 45 * time.Second, "45s"},
		{"one_minute", 60 * time.Second, "1m 0s"},
		{"minutes_and_seconds", 2*time.Minute + 47*time.Second, "2m 47s"},
		{"one_hour", time.Hour, "1h 0m"},
		{"hours_and_minutes", 2*time.Hour + 30*time.Minute, "2h 30m"},
		{"one_day", 24 * time.Hour, "1d 0h"},
		{"days_and_hours", 3*24*time.Hour + 12*time.Hour, "3d 12h"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := awsclient.FormatHumanDuration(tc.duration)
			if got != tc.want {
				t.Errorf("FormatHumanDuration(%v): expected %q, got %q", tc.duration, tc.want, got)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Config defaults test
// ---------------------------------------------------------------------------

// TestConfigDefaultViewDef_SFNExecutions verifies that the sfn_executions
// view definition has the expected list columns and detail paths.
func TestConfigDefaultViewDef_SFNExecutions(t *testing.T) {
	// This import is used indirectly through the config package
	vd := config.DefaultViewDef("sfn_executions")

	t.Run("list_columns", func(t *testing.T) {
		if len(vd.List) < 5 {
			t.Fatalf("expected at least 5 list columns for sfn_executions default, got %d", len(vd.List))
		}
	})

	t.Run("detail_paths", func(t *testing.T) {
		if len(vd.Detail) == 0 {
			t.Error("expected non-empty Detail paths for sfn_executions")
		}
		// Check for key detail fields
		detailStr := strings.Join(vd.Detail, ",")
		for _, expected := range []string{"ExecutionArn", "Name", "Status", "StartDate", "StopDate"} {
			if !strings.Contains(detailStr, expected) {
				t.Errorf("Detail should contain %q, got %v", expected, vd.Detail)
			}
		}
	})
}

// TestFetchSFNExecutions_ContinuationToken verifies that a non-empty
// continuation token is forwarded to the API as NextToken.
func TestFetchSFNExecutions_ContinuationToken(t *testing.T) {
	startTs := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)

	wrapper := &tokenCapturingSFNExecutionsMock{
		inner: &mockSFNListExecutionsClient{
			output: &sfn.ListExecutionsOutput{
				Executions: []sfntypes.ExecutionListItem{
					{
						ExecutionArn:    aws.String("arn:aws:states:us-east-1:123456789012:execution:my-sm:exec-from-token"),
						Name:            aws.String("exec-from-token"),
						StartDate:       &startTs,
						StateMachineArn: aws.String("arn:aws:states:us-east-1:123456789012:stateMachine:my-sm"),
						Status:          sfntypes.ExecutionStatusSucceeded,
					},
				},
			},
		},
	}

	parentCtx := map[string]string{
		"state_machine_arn": "arn:aws:states:us-east-1:123456789012:stateMachine:my-sm",
	}

	result, err := awsclient.FetchSFNExecutions(context.Background(), wrapper, parentCtx, "my-continuation-token")
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

// tokenCapturingSFNExecutionsMock wraps the SFN ListExecutions mock to capture NextToken.
type tokenCapturingSFNExecutionsMock struct {
	inner             *mockSFNListExecutionsClient
	capturedNextToken *string
}

func (m *tokenCapturingSFNExecutionsMock) ListExecutions(ctx context.Context, params *sfn.ListExecutionsInput, optFns ...func(*sfn.Options)) (*sfn.ListExecutionsOutput, error) {
	m.capturedNextToken = params.NextToken
	return m.inner.ListExecutions(ctx, params, optFns...)
}
