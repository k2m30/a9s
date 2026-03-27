package unit

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/glue"
	gluetypes "github.com/aws/aws-sdk-go-v2/service/glue/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// Glue Job Runs fetcher tests (child of Glue Jobs)
// ---------------------------------------------------------------------------

// TestFetchGlueJobRuns_Basic verifies parsing of 1 SUCCEEDED run with all
// fields populated, checking Resource.ID, Name, Status, all Fields keys,
// and RawStruct.
func TestFetchGlueJobRuns_Basic(t *testing.T) {
	startTs := time.Date(2024, 8, 10, 14, 30, 0, 0, time.UTC)
	dpuSec := 45000.0

	mock := &mockGlueGetJobRunsClient{
		output: &glue.GetJobRunsOutput{
			JobRuns: []gluetypes.JobRun{
				{
					Id:            aws.String("jr_abc12345-6789-0abc-def0-123456789012"),
					JobName:       aws.String("etl-daily-load"),
					JobRunState:   gluetypes.JobRunStateSucceeded,
					StartedOn:     &startTs,
					ExecutionTime: 2843,
					ErrorMessage:  aws.String(""),
					DPUSeconds:    &dpuSec,
				},
			},
		},
	}

	result, err := awsclient.FetchGlueJobRuns(
		context.Background(),
		mock,
		"etl-daily-load",
			"",
)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}

	r := result.Resources[0]

	t.Run("ID_is_full_run_id", func(t *testing.T) {
		if r.ID != "jr_abc12345-6789-0abc-def0-123456789012" {
			t.Errorf("ID: expected %q, got %q", "jr_abc12345-6789-0abc-def0-123456789012", r.ID)
		}
	})

	t.Run("Name_is_started_on_timestamp", func(t *testing.T) {
		if r.Name != "2024-08-10 14:30" {
			t.Errorf("Name: expected %q, got %q", "2024-08-10 14:30", r.Name)
		}
	})

	t.Run("Status_is_job_run_state", func(t *testing.T) {
		if r.Status != "SUCCEEDED" {
			t.Errorf("Status: expected %q, got %q", "SUCCEEDED", r.Status)
		}
	})

	t.Run("Fields_run_id_short", func(t *testing.T) {
		if r.Fields["run_id_short"] != "jr_abc12" {
			t.Errorf("Fields[run_id_short]: expected %q, got %q", "jr_abc12", r.Fields["run_id_short"])
		}
	})

	t.Run("Fields_job_run_state", func(t *testing.T) {
		if r.Fields["job_run_state"] != "SUCCEEDED" {
			t.Errorf("Fields[job_run_state]: expected %q, got %q", "SUCCEEDED", r.Fields["job_run_state"])
		}
	})

	t.Run("Fields_started_on", func(t *testing.T) {
		if r.Fields["started_on"] != "2024-08-10 14:30" {
			t.Errorf("Fields[started_on]: expected %q, got %q", "2024-08-10 14:30", r.Fields["started_on"])
		}
	})

	t.Run("Fields_execution_time_human", func(t *testing.T) {
		if r.Fields["execution_time_human"] != "47m 23s" {
			t.Errorf("Fields[execution_time_human]: expected %q, got %q", "47m 23s", r.Fields["execution_time_human"])
		}
	})

	t.Run("Fields_error_message", func(t *testing.T) {
		if r.Fields["error_message"] != "" {
			t.Errorf("Fields[error_message]: expected empty, got %q", r.Fields["error_message"])
		}
	})

	t.Run("Fields_dpu_hours", func(t *testing.T) {
		if r.Fields["dpu_hours"] != "12.5" {
			t.Errorf("Fields[dpu_hours]: expected %q, got %q", "12.5", r.Fields["dpu_hours"])
		}
	})

	t.Run("Fields_run_id", func(t *testing.T) {
		if r.Fields["run_id"] != "jr_abc12345-6789-0abc-def0-123456789012" {
			t.Errorf("Fields[run_id]: expected %q, got %q", "jr_abc12345-6789-0abc-def0-123456789012", r.Fields["run_id"])
		}
	})

	t.Run("Fields_job_name", func(t *testing.T) {
		if r.Fields["job_name"] != "etl-daily-load" {
			t.Errorf("Fields[job_name]: expected %q, got %q", "etl-daily-load", r.Fields["job_name"])
		}
	})

	t.Run("RawStruct_is_JobRun", func(t *testing.T) {
		if r.RawStruct == nil {
			t.Fatal("RawStruct must not be nil")
		}
		raw, ok := r.RawStruct.(gluetypes.JobRun)
		if !ok {
			t.Fatalf("RawStruct should be gluetypes.JobRun, got %T", r.RawStruct)
		}
		if raw.Id == nil || *raw.Id != "jr_abc12345-6789-0abc-def0-123456789012" {
			t.Error("RawStruct.Id not preserved correctly")
		}
	})

	t.Run("required_fields_present", func(t *testing.T) {
		requiredFields := []string{
			"run_id_short", "job_run_state", "started_on",
			"execution_time_human", "error_message", "dpu_hours",
			"run_id", "job_name",
		}
		for _, key := range requiredFields {
			if _, ok := r.Fields[key]; !ok {
				t.Errorf("Fields missing key %q", key)
			}
		}
	})
}

// TestFetchGlueJobRuns_Empty verifies that a job with no runs returns an
// empty slice with no error.
func TestFetchGlueJobRuns_Empty(t *testing.T) {
	mock := &mockGlueGetJobRunsClient{
		output: &glue.GetJobRunsOutput{
			JobRuns: []gluetypes.JobRun{},
		},
	}

	result, err := awsclient.FetchGlueJobRuns(
		context.Background(),
		mock,
		"empty-job",
			"",
)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
}

// TestFetchGlueJobRuns_APIError verifies that API errors are propagated.
func TestFetchGlueJobRuns_APIError(t *testing.T) {
	mock := &mockGlueGetJobRunsClient{
		err: fmt.Errorf("AWS API error: access denied"),
	}

	result, err := awsclient.FetchGlueJobRuns(
		context.Background(),
		mock,
		"error-job",
			"",
)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "access denied") {
		t.Errorf("error should contain 'access denied', got %q", err.Error())
	}
	if result.Resources != nil {
		t.Errorf("expected nil resources on error, got %d", len(result.Resources))
	}
}

// TestFetchGlueJobRuns_NilOptionalFields verifies that nil ErrorMessage,
// nil DPUSeconds, nil StartedOn, nil Id, nil JobName do not cause a panic.
func TestFetchGlueJobRuns_NilOptionalFields(t *testing.T) {
	mock := &mockGlueGetJobRunsClient{
		output: &glue.GetJobRunsOutput{
			JobRuns: []gluetypes.JobRun{
				{
					// All optional pointer fields are nil
					JobRunState: gluetypes.JobRunStateRunning,
				},
			},
		},
	}

	// Should not panic
	result, err := awsclient.FetchGlueJobRuns(
		context.Background(),
		mock,
		"nil-fields-job",
			"",
)
	if err != nil {
		t.Fatalf("expected no error for nil fields, got %v", err)
	}

	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}

	t.Run("nil_Id", func(t *testing.T) {
		// ID may be empty; just ensure no panic occurred
		_ = result.Resources[0].ID
	})

	t.Run("nil_StartedOn", func(t *testing.T) {
		if result.Resources[0].Fields["started_on"] != "" {
			t.Logf("Fields[started_on] is %q (expected empty for nil)", result.Resources[0].Fields["started_on"])
		}
	})

	t.Run("nil_ErrorMessage", func(t *testing.T) {
		if result.Resources[0].Fields["error_message"] != "" {
			t.Logf("Fields[error_message] is %q (expected empty for nil)", result.Resources[0].Fields["error_message"])
		}
	})

	t.Run("nil_DPUSeconds", func(t *testing.T) {
		if result.Resources[0].Fields["dpu_hours"] != "" {
			t.Logf("Fields[dpu_hours] is %q (expected empty for nil)", result.Resources[0].Fields["dpu_hours"])
		}
	})

	t.Run("nil_JobName", func(t *testing.T) {
		if result.Resources[0].Fields["job_name"] != "" {
			t.Logf("Fields[job_name] is %q (expected empty for nil)", result.Resources[0].Fields["job_name"])
		}
	})

	t.Run("status_populated", func(t *testing.T) {
		if result.Resources[0].Status != "RUNNING" {
			t.Errorf("Status: expected %q, got %q", "RUNNING", result.Resources[0].Status)
		}
	})
}

// TestFetchGlueJobRuns_ComputedRunIDShort verifies run_id_short truncation:
// - ID with 36+ chars -> first 8 chars
// - ID with 5 chars -> unchanged
func TestFetchGlueJobRuns_ComputedRunIDShort(t *testing.T) {
	startTs := time.Date(2024, 8, 10, 14, 30, 0, 0, time.UTC)

	mock := &mockGlueGetJobRunsClient{
		output: &glue.GetJobRunsOutput{
			JobRuns: []gluetypes.JobRun{
				{
					Id:          aws.String("jr_abc12345-6789-0abc-def0-123456789012"),
					JobRunState: gluetypes.JobRunStateSucceeded,
					StartedOn:   &startTs,
				},
				{
					Id:          aws.String("short"),
					JobRunState: gluetypes.JobRunStateFailed,
					StartedOn:   &startTs,
				},
			},
		},
	}

	result, err := awsclient.FetchGlueJobRuns(
		context.Background(),
		mock,
		"truncation-job",
			"",
)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(result.Resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(result.Resources))
	}

	t.Run("long_id_truncated_to_8", func(t *testing.T) {
		if result.Resources[0].Fields["run_id_short"] != "jr_abc12" {
			t.Errorf("Fields[run_id_short]: expected %q, got %q", "jr_abc12", result.Resources[0].Fields["run_id_short"])
		}
	})

	t.Run("short_id_unchanged", func(t *testing.T) {
		if result.Resources[1].Fields["run_id_short"] != "short" {
			t.Errorf("Fields[run_id_short]: expected %q, got %q", "short", result.Resources[1].Fields["run_id_short"])
		}
	})
}

// TestFetchGlueJobRuns_ExecutionTimeHuman verifies human-readable formatting:
// - 2843s -> "47m 23s"
// - 7200s -> "2h 0m"
// - 0s    -> "" (not completed)
func TestFetchGlueJobRuns_ExecutionTimeHuman(t *testing.T) {
	startTs := time.Date(2024, 8, 10, 14, 30, 0, 0, time.UTC)

	mock := &mockGlueGetJobRunsClient{
		output: &glue.GetJobRunsOutput{
			JobRuns: []gluetypes.JobRun{
				{
					Id:            aws.String("run-2843s"),
					JobRunState:   gluetypes.JobRunStateSucceeded,
					StartedOn:     &startTs,
					ExecutionTime: 2843,
				},
				{
					Id:            aws.String("run-7200s"),
					JobRunState:   gluetypes.JobRunStateSucceeded,
					StartedOn:     &startTs,
					ExecutionTime: 7200,
				},
				{
					Id:            aws.String("run-0s"),
					JobRunState:   gluetypes.JobRunStateRunning,
					StartedOn:     &startTs,
					ExecutionTime: 0,
				},
			},
		},
	}

	result, err := awsclient.FetchGlueJobRuns(
		context.Background(),
		mock,
		"duration-job",
			"",
)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(result.Resources) != 3 {
		t.Fatalf("expected 3 resources, got %d", len(result.Resources))
	}

	t.Run("2843s_is_47m_23s", func(t *testing.T) {
		dur := result.Resources[0].Fields["execution_time_human"]
		if dur != "47m 23s" {
			t.Errorf("Fields[execution_time_human]: expected %q, got %q", "47m 23s", dur)
		}
	})

	t.Run("7200s_is_2h_0m", func(t *testing.T) {
		dur := result.Resources[1].Fields["execution_time_human"]
		if dur != "2h 0m" {
			t.Errorf("Fields[execution_time_human]: expected %q, got %q", "2h 0m", dur)
		}
	})

	t.Run("0s_is_empty", func(t *testing.T) {
		dur := result.Resources[2].Fields["execution_time_human"]
		if dur != "" {
			t.Errorf("Fields[execution_time_human]: expected empty for 0s, got %q", dur)
		}
	})
}

// TestFetchGlueJobRuns_DPUHours verifies DPU hours calculation:
// - 45000.0 -> "12.5"
// - 0.0     -> ""
// - nil     -> ""
func TestFetchGlueJobRuns_DPUHours(t *testing.T) {
	startTs := time.Date(2024, 8, 10, 14, 30, 0, 0, time.UTC)
	dpuNonZero := 45000.0
	dpuZero := 0.0

	mock := &mockGlueGetJobRunsClient{
		output: &glue.GetJobRunsOutput{
			JobRuns: []gluetypes.JobRun{
				{
					Id:          aws.String("dpu-45000"),
					JobRunState: gluetypes.JobRunStateSucceeded,
					StartedOn:   &startTs,
					DPUSeconds:  &dpuNonZero,
				},
				{
					Id:          aws.String("dpu-zero"),
					JobRunState: gluetypes.JobRunStateSucceeded,
					StartedOn:   &startTs,
					DPUSeconds:  &dpuZero,
				},
				{
					Id:          aws.String("dpu-nil"),
					JobRunState: gluetypes.JobRunStateSucceeded,
					StartedOn:   &startTs,
					DPUSeconds:  nil,
				},
			},
		},
	}

	result, err := awsclient.FetchGlueJobRuns(
		context.Background(),
		mock,
		"dpu-job",
			"",
)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(result.Resources) != 3 {
		t.Fatalf("expected 3 resources, got %d", len(result.Resources))
	}

	t.Run("45000_dpu_seconds_is_12.5_hours", func(t *testing.T) {
		if result.Resources[0].Fields["dpu_hours"] != "12.5" {
			t.Errorf("Fields[dpu_hours]: expected %q, got %q", "12.5", result.Resources[0].Fields["dpu_hours"])
		}
	})

	t.Run("zero_dpu_seconds_is_empty", func(t *testing.T) {
		if result.Resources[1].Fields["dpu_hours"] != "" {
			t.Errorf("Fields[dpu_hours]: expected empty for 0.0, got %q", result.Resources[1].Fields["dpu_hours"])
		}
	})

	t.Run("nil_dpu_seconds_is_empty", func(t *testing.T) {
		if result.Resources[2].Fields["dpu_hours"] != "" {
			t.Errorf("Fields[dpu_hours]: expected empty for nil, got %q", result.Resources[2].Fields["dpu_hours"])
		}
	})
}

// TestFetchGlueJobRuns_ErrorMessageNewlineStripping verifies that \n and \r
// in error messages are replaced with spaces.
func TestFetchGlueJobRuns_ErrorMessageNewlineStripping(t *testing.T) {
	startTs := time.Date(2024, 8, 10, 14, 30, 0, 0, time.UTC)

	mock := &mockGlueGetJobRunsClient{
		output: &glue.GetJobRunsOutput{
			JobRuns: []gluetypes.JobRun{
				{
					Id:           aws.String("err-run"),
					JobRunState:  gluetypes.JobRunStateFailed,
					StartedOn:    &startTs,
					ErrorMessage: aws.String("Line 1\nLine 2\rLine 3\r\nLine 4"),
				},
			},
		},
	}

	result, err := awsclient.FetchGlueJobRuns(
		context.Background(),
		mock,
		"error-job",
			"",
)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}

	errMsg := result.Resources[0].Fields["error_message"]
	if strings.Contains(errMsg, "\n") {
		t.Errorf("error_message should not contain newlines, got %q", errMsg)
	}
	if strings.Contains(errMsg, "\r") {
		t.Errorf("error_message should not contain carriage returns, got %q", errMsg)
	}
	if !strings.Contains(errMsg, "Line 1") || !strings.Contains(errMsg, "Line 4") {
		t.Errorf("error_message should preserve text content, got %q", errMsg)
	}
}

// TestFetchGlueJobRuns_Pagination verifies that paginated responses via
// NextToken are followed and all job runs collected across multiple pages.
func TestFetchGlueJobRuns_Pagination(t *testing.T) {
	startTs := time.Date(2024, 8, 10, 14, 30, 0, 0, time.UTC)

	mock := &mockGlueGetJobRunsClient{
		outputs: []*glue.GetJobRunsOutput{
			{
				NextToken: aws.String("page2-token"),
				JobRuns: []gluetypes.JobRun{
					{
						Id:          aws.String("run-p1-1"),
						JobRunState: gluetypes.JobRunStateSucceeded,
						StartedOn:   &startTs,
					},
					{
						Id:          aws.String("run-p1-2"),
						JobRunState: gluetypes.JobRunStateFailed,
						StartedOn:   &startTs,
					},
				},
			},
			{
				// No NextToken -- last page
				JobRuns: []gluetypes.JobRun{
					{
						Id:          aws.String("run-p2-1"),
						JobRunState: gluetypes.JobRunStateRunning,
						StartedOn:   &startTs,
					},
				},
			},
		},
	}

	result, err := awsclient.FetchGlueJobRuns(
		context.Background(),
		mock,
		"paginated-job",
			"",
)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	t.Run("total_count", func(t *testing.T) {
		if len(result.Resources) != 3 {
			t.Fatalf("expected 3 resources across 2 pages, got %d", len(result.Resources))
		}
	})

	t.Run("page1_runs", func(t *testing.T) {
		expectedIDs := []string{"run-p1-1", "run-p1-2"}
		for i, expectedID := range expectedIDs {
			if result.Resources[i].ID != expectedID {
				t.Errorf("resources[%d].ID: expected %q, got %q", i, expectedID, result.Resources[i].ID)
			}
		}
	})

	t.Run("page2_runs", func(t *testing.T) {
		if result.Resources[2].ID != "run-p2-1" {
			t.Errorf("resources[2].ID: expected %q, got %q", "run-p2-1", result.Resources[2].ID)
		}
	})

	t.Run("api_called_twice", func(t *testing.T) {
		if mock.callIdx != 2 {
			t.Errorf("expected 2 API calls for pagination, got %d", mock.callIdx)
		}
	})

	t.Run("all_have_status", func(t *testing.T) {
		for i, r := range result.Resources {
			if r.Status == "" {
				t.Errorf("resources[%d].Status should not be empty", i)
			}
		}
	})
}

// TestFetchGlueJobRuns_MaxRunsCap verifies that the fetcher stops collecting
// runs once it reaches the maxRuns=200 cap.
func TestFetchGlueJobRuns_MaxRunsCap(t *testing.T) {
	startTs := time.Date(2024, 8, 10, 14, 30, 0, 0, time.UTC)

	// Build 5 pages of 50 runs each (250 total). The fetcher should stop at 200.
	var outputs []*glue.GetJobRunsOutput
	for page := 0; page < 5; page++ {
		var runs []gluetypes.JobRun
		for i := 0; i < 50; i++ {
			runs = append(runs, gluetypes.JobRun{
				Id:          aws.String(fmt.Sprintf("run-p%d-%d", page, i)),
				JobRunState: gluetypes.JobRunStateSucceeded,
				StartedOn:   &startTs,
			})
		}
		out := &glue.GetJobRunsOutput{
			JobRuns: runs,
		}
		if page < 4 {
			out.NextToken = aws.String(fmt.Sprintf("token-page-%d", page+1))
		}
		outputs = append(outputs, out)
	}

	mock := &mockGlueGetJobRunsClient{outputs: outputs}

	result, err := awsclient.FetchGlueJobRuns(
		context.Background(),
		mock,
		"capped-job",
			"",
)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	t.Run("capped_at_200", func(t *testing.T) {
		if len(result.Resources) != 200 {
			t.Errorf("expected exactly 200 resources (maxRuns cap), got %d", len(result.Resources))
		}
	})

	t.Run("early_termination", func(t *testing.T) {
		// With 50 runs per page, reaching 200 should take exactly 4 pages.
		// The fetcher should NOT call the 5th page.
		if mock.callIdx != 4 {
			t.Errorf("expected 4 API calls (early termination at 200), got %d", mock.callIdx)
		}
	})

	t.Run("first_run_correct", func(t *testing.T) {
		if result.Resources[0].ID != "run-p0-0" {
			t.Errorf("first resource ID: expected %q, got %q", "run-p0-0", result.Resources[0].ID)
		}
	})

	t.Run("last_run_correct", func(t *testing.T) {
		// Last run should be the 50th of page 3 (index 199 = page3, item49)
		if result.Resources[199].ID != "run-p3-49" {
			t.Errorf("last resource ID: expected %q, got %q", "run-p3-49", result.Resources[199].ID)
		}
	})
}

// TestFetchGlueJobRuns_RawStruct verifies that RawStruct is the original
// gluetypes.JobRun value.
func TestFetchGlueJobRuns_RawStruct(t *testing.T) {
	startTs := time.Date(2024, 8, 10, 14, 30, 0, 0, time.UTC)

	mock := &mockGlueGetJobRunsClient{
		output: &glue.GetJobRunsOutput{
			JobRuns: []gluetypes.JobRun{
				{
					Id:          aws.String("raw-run-123"),
					JobName:     aws.String("raw-job"),
					JobRunState: gluetypes.JobRunStateSucceeded,
					StartedOn:   &startTs,
				},
			},
		},
	}

	result, err := awsclient.FetchGlueJobRuns(
		context.Background(),
		mock,
		"raw-job",
			"",
)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}

	raw, ok := result.Resources[0].RawStruct.(gluetypes.JobRun)
	if !ok {
		t.Fatalf("RawStruct should be gluetypes.JobRun, got %T", result.Resources[0].RawStruct)
	}
	if raw.Id == nil || *raw.Id != "raw-run-123" {
		t.Error("RawStruct.Id not preserved correctly")
	}
	if raw.JobName == nil || *raw.JobName != "raw-job" {
		t.Error("RawStruct.JobName not preserved correctly")
	}
}

// ---------------------------------------------------------------------------
// Column definitions test
// ---------------------------------------------------------------------------

// TestGlueRunColumns verifies that GlueRunColumns returns the expected 6
// columns with correct keys, widths, titles, and sortability.
func TestGlueRunColumns(t *testing.T) {
	cols := resource.GlueRunColumns()

	expectedKeys := []string{
		"run_id_short", "job_run_state", "started_on",
		"execution_time_human", "error_message", "dpu_hours",
	}

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

	t.Run("expected_widths", func(t *testing.T) {
		expectedWidths := []int{12, 12, 22, 14, 44, 10}
		for i, expected := range expectedWidths {
			if cols[i].Width != expected {
				t.Errorf("column[%d] (%s).Width: expected %d, got %d", i, cols[i].Key, expected, cols[i].Width)
			}
		}
	})

	t.Run("expected_titles", func(t *testing.T) {
		expectedTitles := []string{"Run ID", "State", "Started", "Execution Time", "Error Message", "DPU Hours"}
		for i, expected := range expectedTitles {
			if cols[i].Title != expected {
				t.Errorf("column[%d] (%s).Title: expected %q, got %q", i, cols[i].Key, expected, cols[i].Title)
			}
		}
	})

	t.Run("error_message_not_sortable", func(t *testing.T) {
		for _, col := range cols {
			if col.Key == "error_message" {
				if col.Sortable {
					t.Error("error_message column should not be sortable")
				}
				return
			}
		}
		t.Error("error_message column not found")
	})

	t.Run("other_columns_sortable", func(t *testing.T) {
		for _, col := range cols {
			if col.Key != "error_message" && !col.Sortable {
				t.Errorf("column %q should be sortable", col.Key)
			}
		}
	})
}

// ---------------------------------------------------------------------------
// Registration tests
// ---------------------------------------------------------------------------

// TestGlueRuns_ChildTypeRegistered verifies that "glue_runs" is registered
// as a child resource type.
func TestGlueRuns_ChildTypeRegistered(t *testing.T) {
	td := resource.GetChildType("glue_runs")
	if td == nil {
		t.Fatal("glue_runs child resource type not registered")
	}
	if td.Name == "" {
		t.Error("child type Name should not be empty")
	}
	if td.ShortName != "glue_runs" {
		t.Errorf("child type ShortName: expected %q, got %q", "glue_runs", td.ShortName)
	}
}

// TestGlueRuns_PaginatedChildFetcherRegistered verifies that the paginated
// child fetcher is
// registered under the correct short name.
func TestGlueRuns_PaginatedChildFetcherRegistered(t *testing.T) {
	f := resource.GetPaginatedChildFetcher("glue_runs")
	if f == nil {
		t.Fatal("glue_runs paginated child fetcher not registered")
	}
}

// TestGlueRuns_ParentHasChildDef verifies that the parent glue resource
// type has a child view definition for glue_runs with key "enter" and
// correct ContextKeys.
func TestGlueRuns_ParentHasChildDef(t *testing.T) {
	rt := resource.FindResourceType("glue")
	if rt == nil {
		t.Fatal("glue resource type not found")
	}

	found := false
	for _, child := range rt.Children {
		if child.ChildType == "glue_runs" {
			found = true
			if child.Key != "enter" {
				t.Errorf("expected key %q, got %q", "enter", child.Key)
			}
			if child.ContextKeys["job_name"] != "ID" {
				t.Errorf("ContextKeys[job_name]: expected %q, got %q", "ID", child.ContextKeys["job_name"])
			}
			if child.DisplayNameKey != "job_name" {
				t.Errorf("DisplayNameKey: expected %q, got %q", "job_name", child.DisplayNameKey)
			}
		}
	}
	if !found {
		t.Error("glue Children should contain glue_runs child view def")
	}
}

// TestGlueRuns_CopyField verifies that the glue_runs child type has
// CopyField set to "error_message".
func TestGlueRuns_CopyField(t *testing.T) {
	td := resource.GetChildType("glue_runs")
	if td == nil {
		t.Fatal("glue_runs child type not found")
	}
	if td.CopyField != "error_message" {
		t.Errorf("CopyField: expected %q, got %q", "error_message", td.CopyField)
	}
}

// ---------------------------------------------------------------------------
// Config defaults test
// ---------------------------------------------------------------------------

// TestConfigDefaultViewDef_GlueRuns verifies that the glue_runs view
// definition has the expected list columns and non-empty detail paths.
func TestConfigDefaultViewDef_GlueRuns(t *testing.T) {
	vd := config.DefaultViewDef("glue_runs")

	t.Run("list_columns", func(t *testing.T) {
		if len(vd.List) < 3 {
			t.Fatalf("expected at least 3 list columns for glue_runs default, got %d", len(vd.List))
		}
	})

	t.Run("detail_paths", func(t *testing.T) {
		if len(vd.Detail) == 0 {
			t.Error("expected non-empty Detail paths for glue_runs")
		}
	})
}

// TestFetchGlueJobRuns_ContinuationToken verifies that a non-empty
// continuation token is forwarded to the API as NextToken.
func TestFetchGlueJobRuns_ContinuationToken(t *testing.T) {
	startTs := time.Date(2024, 8, 10, 14, 30, 0, 0, time.UTC)

	wrapper := &tokenCapturingGlueRunsMock{
		inner: &mockGlueGetJobRunsClient{
			output: &glue.GetJobRunsOutput{
				JobRuns: []gluetypes.JobRun{
					{
						Id:          aws.String("jr_from_token"),
						JobName:     aws.String("my-job"),
						JobRunState: gluetypes.JobRunStateSucceeded,
						StartedOn:   &startTs,
					},
				},
			},
		},
	}

	result, err := awsclient.FetchGlueJobRuns(context.Background(), wrapper, "my-job", "my-continuation-token")
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

// tokenCapturingGlueRunsMock wraps the glue runs mock to capture NextToken.
type tokenCapturingGlueRunsMock struct {
	inner             *mockGlueGetJobRunsClient
	capturedNextToken *string
}

func (m *tokenCapturingGlueRunsMock) GetJobRuns(ctx context.Context, params *glue.GetJobRunsInput, optFns ...func(*glue.Options)) (*glue.GetJobRunsOutput, error) {
	m.capturedNextToken = params.NextToken
	return m.inner.GetJobRuns(ctx, params, optFns...)
}

// Ensure all imports are used.
var _ = aws.String
var _ = glue.GetJobRunsOutput{}
var _ = config.DefaultViewDef
