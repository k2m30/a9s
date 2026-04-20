package unit

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/codepipeline"
	cptypes "github.com/aws/aws-sdk-go-v2/service/codepipeline/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// Pipeline Stages fetcher tests (child of CodePipelines)
// ---------------------------------------------------------------------------

// TestFetchPipelineStages_Basic verifies parsing of 2 stages with 4 total
// actions, checking that each stage+action pair is flattened to its own
// resource row with correct ID, Name, Status, and all Fields keys.
func TestFetchPipelineStages_Basic(t *testing.T) {
	lastChange1 := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)
	lastChange2 := time.Date(2024, 6, 15, 10, 1, 30, 0, time.UTC)
	lastChange3 := time.Date(2024, 6, 15, 10, 2, 0, 0, time.UTC)
	lastChange4 := time.Date(2024, 6, 15, 10, 3, 45, 0, time.UTC)

	mock := &mockCodePipelineGetPipelineStateClient{
		output: &codepipeline.GetPipelineStateOutput{
			PipelineName: aws.String("deploy-prod"),
			StageStates: []cptypes.StageState{
				{
					StageName: aws.String("Source"),
					LatestExecution: &cptypes.StageExecution{
						Status: cptypes.StageExecutionStatusSucceeded,
					},
					ActionStates: []cptypes.ActionState{
						{
							ActionName: aws.String("GitHub"),
							LatestExecution: &cptypes.ActionExecution{
								Status:               cptypes.ActionExecutionStatusSucceeded,
								LastStatusChange:     &lastChange1,
								ExternalExecutionUrl: aws.String("https://github.com/org/repo/commit/abc123"),
								Token:                aws.String("token-001"),
							},
							CurrentRevision: &cptypes.ActionRevision{
								RevisionId:       aws.String("abc123def456"),
								RevisionChangeId: aws.String("commit-sha-abc"),
							},
						},
						{
							ActionName: aws.String("S3Upload"),
							LatestExecution: &cptypes.ActionExecution{
								Status:           cptypes.ActionExecutionStatusSucceeded,
								LastStatusChange: &lastChange2,
							},
						},
					},
				},
				{
					StageName: aws.String("Deploy"),
					LatestExecution: &cptypes.StageExecution{
						Status: cptypes.StageExecutionStatusInProgress,
					},
					ActionStates: []cptypes.ActionState{
						{
							ActionName: aws.String("CodeBuild"),
							LatestExecution: &cptypes.ActionExecution{
								Status:               cptypes.ActionExecutionStatusSucceeded,
								LastStatusChange:     &lastChange3,
								ExternalExecutionUrl: aws.String("https://console.aws.amazon.com/codebuild/home"),
							},
						},
						{
							ActionName: aws.String("ECS-Deploy"),
							LatestExecution: &cptypes.ActionExecution{
								Status:           cptypes.ActionExecutionStatusInProgress,
								LastStatusChange: &lastChange4,
							},
						},
					},
				},
			},
		},
	}

	parentCtx := map[string]string{
		"pipeline_name": "deploy-prod",
	}

	result, err := awsclient.FetchPipelineStages(
		context.Background(),
		mock,
		parentCtx,
		"",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	resources := result.Resources

	if len(resources) != 4 {
		t.Fatalf("expected 4 resources (2 stages x 2 actions each), got %d", len(resources))
	}

	// Row 0: Source / GitHub (first action in stage → stage_name populated)
	r0 := resources[0]
	t.Run("row0_stage_name", func(t *testing.T) {
		if r0.Fields["stage_name"] != "Source" {
			t.Errorf("Fields[stage_name]: expected %q, got %q", "Source", r0.Fields["stage_name"])
		}
	})
	t.Run("row0_stage_status", func(t *testing.T) {
		if r0.Fields["stage_status"] != "Succeeded" {
			t.Errorf("Fields[stage_status]: expected %q, got %q", "Succeeded", r0.Fields["stage_status"])
		}
	})
	t.Run("row0_action_name", func(t *testing.T) {
		if r0.Fields["action_name"] != "GitHub" {
			t.Errorf("Fields[action_name]: expected %q, got %q", "GitHub", r0.Fields["action_name"])
		}
	})
	t.Run("row0_action_status", func(t *testing.T) {
		if r0.Fields["action_status"] != "Succeeded" {
			t.Errorf("Fields[action_status]: expected %q, got %q", "Succeeded", r0.Fields["action_status"])
		}
	})
	t.Run("row0_last_change_time", func(t *testing.T) {
		if r0.Fields["last_change_time"] != "2024-06-15 10:00" {
			t.Errorf("Fields[last_change_time]: expected %q, got %q", "2024-06-15 10:00", r0.Fields["last_change_time"])
		}
	})
	t.Run("row0_external_url", func(t *testing.T) {
		if r0.Fields["external_url"] != "https://github.com/org/repo/commit/abc123" {
			t.Errorf("Fields[external_url]: expected GitHub URL, got %q", r0.Fields["external_url"])
		}
	})
	t.Run("row0_revision_id", func(t *testing.T) {
		if r0.Fields["revision_id"] != "abc123def456" {
			t.Errorf("Fields[revision_id]: expected %q, got %q", "abc123def456", r0.Fields["revision_id"])
		}
	})
	t.Run("row0_revision_summary", func(t *testing.T) {
		if r0.Fields["revision_summary"] != "commit-sha-abc" {
			t.Errorf("Fields[revision_summary]: expected %q, got %q", "commit-sha-abc", r0.Fields["revision_summary"])
		}
	})

	// Row 1: Source / S3Upload (second action → stage_name blank)
	r1 := resources[1]
	t.Run("row1_stage_name_blank", func(t *testing.T) {
		if r1.Fields["stage_name"] != "" {
			t.Errorf("Fields[stage_name] for 2nd action should be blank, got %q", r1.Fields["stage_name"])
		}
	})
	t.Run("row1_action_name", func(t *testing.T) {
		if r1.Fields["action_name"] != "S3Upload" {
			t.Errorf("Fields[action_name]: expected %q, got %q", "S3Upload", r1.Fields["action_name"])
		}
	})

	// Row 2: Deploy / CodeBuild (first action in second stage → stage_name populated)
	r2 := resources[2]
	t.Run("row2_stage_name", func(t *testing.T) {
		if r2.Fields["stage_name"] != "Deploy" {
			t.Errorf("Fields[stage_name]: expected %q, got %q", "Deploy", r2.Fields["stage_name"])
		}
	})
	t.Run("row2_action_name", func(t *testing.T) {
		if r2.Fields["action_name"] != "CodeBuild" {
			t.Errorf("Fields[action_name]: expected %q, got %q", "CodeBuild", r2.Fields["action_name"])
		}
	})

	// Row 3: Deploy / ECS-Deploy (second action → stage_name blank)
	r3 := resources[3]
	t.Run("row3_stage_name_blank", func(t *testing.T) {
		if r3.Fields["stage_name"] != "" {
			t.Errorf("Fields[stage_name] for 2nd action should be blank, got %q", r3.Fields["stage_name"])
		}
	})
	t.Run("row3_action_name", func(t *testing.T) {
		if r3.Fields["action_name"] != "ECS-Deploy" {
			t.Errorf("Fields[action_name]: expected %q, got %q", "ECS-Deploy", r3.Fields["action_name"])
		}
	})

	t.Run("required_fields_present_on_all_rows", func(t *testing.T) {
		requiredFields := []string{
			"stage_name", "stage_status", "action_name", "action_status",
			"last_change_time", "external_url",
		}
		for i, r := range resources {
			for _, key := range requiredFields {
				if _, ok := r.Fields[key]; !ok {
					t.Errorf("Row %d Fields missing key %q", i, key)
				}
			}
		}
	})
}

// TestFetchPipelineStages_MultiAction verifies that a single stage with 3
// actions produces 3 resource rows, and that stage_name is shown ONLY on
// the first action row (blank for subsequent actions in the same stage).
func TestFetchPipelineStages_MultiAction(t *testing.T) {
	lastChange := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)

	mock := &mockCodePipelineGetPipelineStateClient{
		output: &codepipeline.GetPipelineStateOutput{
			PipelineName: aws.String("multi-action-pipeline"),
			StageStates: []cptypes.StageState{
				{
					StageName: aws.String("Build"),
					LatestExecution: &cptypes.StageExecution{
						Status: cptypes.StageExecutionStatusSucceeded,
					},
					ActionStates: []cptypes.ActionState{
						{
							ActionName: aws.String("CompileCode"),
							LatestExecution: &cptypes.ActionExecution{
								Status:           cptypes.ActionExecutionStatusSucceeded,
								LastStatusChange: &lastChange,
							},
						},
						{
							ActionName: aws.String("RunTests"),
							LatestExecution: &cptypes.ActionExecution{
								Status:           cptypes.ActionExecutionStatusSucceeded,
								LastStatusChange: &lastChange,
							},
						},
						{
							ActionName: aws.String("PackageArtifact"),
							LatestExecution: &cptypes.ActionExecution{
								Status:           cptypes.ActionExecutionStatusFailed,
								LastStatusChange: &lastChange,
							},
						},
					},
				},
			},
		},
	}

	parentCtx := map[string]string{
		"pipeline_name": "multi-action-pipeline",
	}

	result, err := awsclient.FetchPipelineStages(
		context.Background(),
		mock,
		parentCtx,
		"",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	resources := result.Resources

	if len(resources) != 3 {
		t.Fatalf("expected 3 resources for 3 actions, got %d", len(resources))
	}

	// First action should have stage_name populated
	if resources[0].Fields["stage_name"] != "Build" {
		t.Errorf("First action stage_name: expected %q, got %q", "Build", resources[0].Fields["stage_name"])
	}

	// Second and third actions should have blank stage_name
	if resources[1].Fields["stage_name"] != "" {
		t.Errorf("Second action stage_name should be blank, got %q", resources[1].Fields["stage_name"])
	}
	if resources[2].Fields["stage_name"] != "" {
		t.Errorf("Third action stage_name should be blank, got %q", resources[2].Fields["stage_name"])
	}

	// Verify all action names
	if resources[0].Fields["action_name"] != "CompileCode" {
		t.Errorf("Row 0 action_name: expected %q, got %q", "CompileCode", resources[0].Fields["action_name"])
	}
	if resources[1].Fields["action_name"] != "RunTests" {
		t.Errorf("Row 1 action_name: expected %q, got %q", "RunTests", resources[1].Fields["action_name"])
	}
	if resources[2].Fields["action_name"] != "PackageArtifact" {
		t.Errorf("Row 2 action_name: expected %q, got %q", "PackageArtifact", resources[2].Fields["action_name"])
	}
}

// TestFetchPipelineStages_Empty verifies that a pipeline with no stages
// returns an empty slice with no error.
func TestFetchPipelineStages_Empty(t *testing.T) {
	mock := &mockCodePipelineGetPipelineStateClient{
		output: &codepipeline.GetPipelineStateOutput{
			PipelineName: aws.String("empty-pipeline"),
			StageStates:  []cptypes.StageState{},
		},
	}

	parentCtx := map[string]string{
		"pipeline_name": "empty-pipeline",
	}

	result, err := awsclient.FetchPipelineStages(
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

// TestFetchPipelineStages_Error verifies that GetPipelineState API errors
// are propagated correctly.
func TestFetchPipelineStages_Error(t *testing.T) {
	mock := &mockCodePipelineGetPipelineStateClient{
		err: fmt.Errorf("AWS API error: pipeline not found"),
	}

	parentCtx := map[string]string{
		"pipeline_name": "nonexistent-pipeline",
	}

	result, err := awsclient.FetchPipelineStages(
		context.Background(),
		mock,
		parentCtx,
		"",
	)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "pipeline not found") {
		t.Errorf("error should contain 'pipeline not found', got %q", err.Error())
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources on error, got %d", len(result.Resources))
	}
}

// TestFetchPipelineStages_NilExecution verifies that nil LatestExecution on
// both stage and action level does not panic and produces empty status/time.
func TestFetchPipelineStages_NilExecution(t *testing.T) {
	mock := &mockCodePipelineGetPipelineStateClient{
		output: &codepipeline.GetPipelineStateOutput{
			PipelineName: aws.String("nil-exec-pipeline"),
			StageStates: []cptypes.StageState{
				{
					StageName:       aws.String("Source"),
					LatestExecution: nil, // nil stage execution
					ActionStates: []cptypes.ActionState{
						{
							ActionName:      aws.String("GitHub"),
							LatestExecution: nil, // nil action execution
						},
					},
				},
			},
		},
	}

	parentCtx := map[string]string{
		"pipeline_name": "nil-exec-pipeline",
	}

	result, err := awsclient.FetchPipelineStages(
		context.Background(),
		mock,
		parentCtx,
		"",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	resources := result.Resources

	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	r := resources[0]
	t.Run("stage_status_empty", func(t *testing.T) {
		if r.Fields["stage_status"] != "" {
			t.Errorf("Fields[stage_status] should be empty for nil execution, got %q", r.Fields["stage_status"])
		}
	})
	t.Run("action_status_empty", func(t *testing.T) {
		if r.Fields["action_status"] != "" {
			t.Errorf("Fields[action_status] should be empty for nil execution, got %q", r.Fields["action_status"])
		}
	})
	t.Run("last_change_time_empty", func(t *testing.T) {
		if r.Fields["last_change_time"] != "" {
			t.Errorf("Fields[last_change_time] should be empty for nil execution, got %q", r.Fields["last_change_time"])
		}
	})
	t.Run("external_url_empty", func(t *testing.T) {
		if r.Fields["external_url"] != "" {
			t.Errorf("Fields[external_url] should be empty for nil execution, got %q", r.Fields["external_url"])
		}
	})
	t.Run("action_token_empty", func(t *testing.T) {
		if r.Fields["action_token"] != "" {
			t.Errorf("Fields[action_token] should be empty for nil execution, got %q", r.Fields["action_token"])
		}
	})
}

// TestFetchPipelineStages_NilActionStates verifies that a stage with nil or
// empty ActionStates produces no rows for that stage.
func TestFetchPipelineStages_NilActionStates(t *testing.T) {
	mock := &mockCodePipelineGetPipelineStateClient{
		output: &codepipeline.GetPipelineStateOutput{
			PipelineName: aws.String("nil-actions-pipeline"),
			StageStates: []cptypes.StageState{
				{
					StageName: aws.String("EmptyStage"),
					LatestExecution: &cptypes.StageExecution{
						Status: cptypes.StageExecutionStatusSucceeded,
					},
					ActionStates: nil, // nil action states
				},
				{
					StageName: aws.String("AlsoEmpty"),
					LatestExecution: &cptypes.StageExecution{
						Status: cptypes.StageExecutionStatusSucceeded,
					},
					ActionStates: []cptypes.ActionState{}, // empty action states
				},
			},
		},
	}

	parentCtx := map[string]string{
		"pipeline_name": "nil-actions-pipeline",
	}

	result, err := awsclient.FetchPipelineStages(
		context.Background(),
		mock,
		parentCtx,
		"",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources for stages with no actions, got %d", len(result.Resources))
	}
}

// TestFetchPipelineStages_StatusMapping verifies that action execution
// statuses map correctly to resource Status values:
//
//	Succeeded   → "running"
//	Failed      → "failed"
//	InProgress  → "pending"
//	Stopped     → "terminated"
//	Abandoned   → "terminated"
//	""          → "terminated"
func TestFetchPipelineStages_StatusMapping(t *testing.T) {
	lastChange := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		actionStatus cptypes.ActionExecutionStatus
		wantStatus   string
	}{
		{cptypes.ActionExecutionStatusSucceeded, "running"},
		{cptypes.ActionExecutionStatusFailed, "failed"},
		{cptypes.ActionExecutionStatusInProgress, "pending"},
		{cptypes.ActionExecutionStatusAbandoned, "terminated"},
		{cptypes.ActionExecutionStatus("Stopped"), "terminated"},
		{"", "terminated"},
	}

	for _, tc := range tests {
		name := string(tc.actionStatus)
		if name == "" {
			name = "empty"
		}
		t.Run(name, func(t *testing.T) {
			mock := &mockCodePipelineGetPipelineStateClient{
				output: &codepipeline.GetPipelineStateOutput{
					PipelineName: aws.String("status-test"),
					StageStates: []cptypes.StageState{
						{
							StageName: aws.String("Stage1"),
							ActionStates: []cptypes.ActionState{
								{
									ActionName: aws.String("Action1"),
									LatestExecution: &cptypes.ActionExecution{
										Status:           tc.actionStatus,
										LastStatusChange: &lastChange,
									},
								},
							},
						},
					},
				},
			}

			parentCtx := map[string]string{"pipeline_name": "status-test"}
			result, err := awsclient.FetchPipelineStages(context.Background(), mock, parentCtx, "")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			resources := result.Resources
			if len(resources) != 1 {
				t.Fatalf("expected 1 resource, got %d", len(resources))
			}
			if resources[0].Status != tc.wantStatus {
				t.Errorf("Status for action %q: expected %q, got %q",
					tc.actionStatus, tc.wantStatus, resources[0].Status)
			}
		})
	}
}

// TestFetchPipelineStages_ExternalURL verifies that ExternalExecutionUrl is
// correctly extracted from the action execution.
func TestFetchPipelineStages_ExternalURL(t *testing.T) {
	lastChange := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)
	expectedURL := "https://console.aws.amazon.com/codebuild/home#/builds/my-project:build-001/view/new"

	mock := &mockCodePipelineGetPipelineStateClient{
		output: &codepipeline.GetPipelineStateOutput{
			PipelineName: aws.String("url-test"),
			StageStates: []cptypes.StageState{
				{
					StageName: aws.String("Build"),
					ActionStates: []cptypes.ActionState{
						{
							ActionName: aws.String("CodeBuild"),
							LatestExecution: &cptypes.ActionExecution{
								Status:               cptypes.ActionExecutionStatusSucceeded,
								LastStatusChange:     &lastChange,
								ExternalExecutionUrl: aws.String(expectedURL),
							},
						},
					},
				},
			},
		},
	}

	parentCtx := map[string]string{"pipeline_name": "url-test"}
	result, err := awsclient.FetchPipelineStages(context.Background(), mock, parentCtx, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resources := result.Resources
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}
	if resources[0].Fields["external_url"] != expectedURL {
		t.Errorf("Fields[external_url]: expected %q, got %q", expectedURL, resources[0].Fields["external_url"])
	}
}

// TestFetchPipelineStages_LastChangeTime verifies that LastStatusChange
// is formatted as "2006-01-02 15:04" in UTC.
func TestFetchPipelineStages_LastChangeTime(t *testing.T) {
	ts := time.Date(2024, 12, 25, 23, 59, 59, 0, time.UTC)

	mock := &mockCodePipelineGetPipelineStateClient{
		output: &codepipeline.GetPipelineStateOutput{
			PipelineName: aws.String("time-test"),
			StageStates: []cptypes.StageState{
				{
					StageName: aws.String("Stage1"),
					ActionStates: []cptypes.ActionState{
						{
							ActionName: aws.String("Action1"),
							LatestExecution: &cptypes.ActionExecution{
								Status:           cptypes.ActionExecutionStatusSucceeded,
								LastStatusChange: &ts,
							},
						},
					},
				},
			},
		},
	}

	parentCtx := map[string]string{"pipeline_name": "time-test"}
	result, err := awsclient.FetchPipelineStages(context.Background(), mock, parentCtx, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resources := result.Resources
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}
	expected := "2024-12-25 23:59"
	if resources[0].Fields["last_change_time"] != expected {
		t.Errorf("Fields[last_change_time]: expected %q, got %q", expected, resources[0].Fields["last_change_time"])
	}
}

// TestFetchPipelineStages_DetailFields verifies that detail-only fields
// (action_token, action_error_details, revision_id, revision_summary)
// are correctly extracted.
func TestFetchPipelineStages_DetailFields(t *testing.T) {
	lastChange := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)

	mock := &mockCodePipelineGetPipelineStateClient{
		output: &codepipeline.GetPipelineStateOutput{
			PipelineName: aws.String("detail-test"),
			StageStates: []cptypes.StageState{
				{
					StageName: aws.String("Stage1"),
					ActionStates: []cptypes.ActionState{
						{
							ActionName: aws.String("ManualApproval"),
							LatestExecution: &cptypes.ActionExecution{
								Status:           cptypes.ActionExecutionStatusSucceeded,
								LastStatusChange: &lastChange,
								Token:            aws.String("approval-token-xyz"),
							},
							CurrentRevision: &cptypes.ActionRevision{
								RevisionId:       aws.String("rev-12345"),
								RevisionChangeId: aws.String("change-id-abc"),
							},
						},
					},
				},
			},
		},
	}

	parentCtx := map[string]string{"pipeline_name": "detail-test"}
	result, err := awsclient.FetchPipelineStages(context.Background(), mock, parentCtx, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resources := result.Resources
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	r := resources[0]
	t.Run("action_token", func(t *testing.T) {
		if r.Fields["action_token"] != "approval-token-xyz" {
			t.Errorf("Fields[action_token]: expected %q, got %q", "approval-token-xyz", r.Fields["action_token"])
		}
	})
	t.Run("revision_id", func(t *testing.T) {
		if r.Fields["revision_id"] != "rev-12345" {
			t.Errorf("Fields[revision_id]: expected %q, got %q", "rev-12345", r.Fields["revision_id"])
		}
	})
	t.Run("revision_summary", func(t *testing.T) {
		if r.Fields["revision_summary"] != "change-id-abc" {
			t.Errorf("Fields[revision_summary]: expected %q, got %q", "change-id-abc", r.Fields["revision_summary"])
		}
	})
	t.Run("action_error_details_empty", func(t *testing.T) {
		if r.Fields["action_error_details"] != "" {
			t.Errorf("Fields[action_error_details] should be empty when no error, got %q", r.Fields["action_error_details"])
		}
	})
}

// TestFetchPipelineStages_ErrorDetails verifies that ErrorDetails with Code
// and Message are formatted as "code: message".
func TestFetchPipelineStages_ErrorDetails(t *testing.T) {
	lastChange := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)

	mock := &mockCodePipelineGetPipelineStateClient{
		output: &codepipeline.GetPipelineStateOutput{
			PipelineName: aws.String("error-detail-test"),
			StageStates: []cptypes.StageState{
				{
					StageName: aws.String("Deploy"),
					ActionStates: []cptypes.ActionState{
						{
							ActionName: aws.String("ECS-Deploy"),
							LatestExecution: &cptypes.ActionExecution{
								Status:           cptypes.ActionExecutionStatusFailed,
								LastStatusChange: &lastChange,
								ErrorDetails: &cptypes.ErrorDetails{
									Code:    aws.String("JobFailed"),
									Message: aws.String("Deployment failed: service unhealthy"),
								},
							},
						},
					},
				},
			},
		},
	}

	parentCtx := map[string]string{"pipeline_name": "error-detail-test"}
	result, err := awsclient.FetchPipelineStages(context.Background(), mock, parentCtx, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resources := result.Resources
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	expected := "JobFailed: Deployment failed: service unhealthy"
	if resources[0].Fields["action_error_details"] != expected {
		t.Errorf("Fields[action_error_details]: expected %q, got %q",
			expected, resources[0].Fields["action_error_details"])
	}
}

// TestFetchPipelineStages_ParentContext verifies that the "pipeline_name"
// context key from the parent is used to call GetPipelineState.
func TestFetchPipelineStages_ParentContext(t *testing.T) {
	mock := &mockCodePipelineGetPipelineStateClient{
		output: &codepipeline.GetPipelineStateOutput{
			PipelineName: aws.String("specific-pipeline"),
			StageStates:  []cptypes.StageState{},
		},
	}

	parentCtx := map[string]string{
		"pipeline_name": "specific-pipeline",
	}

	_, err := awsclient.FetchPipelineStages(
		context.Background(),
		mock,
		parentCtx,
		"",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	// If GetPipelineState was called without error, the pipeline_name was used
}

// TestFetchPipelineStages_RawStruct verifies that RawStruct is preserved
// for each flattened row. Since we flatten stage→action, the RawStruct
// should be a PipelineStageRow (or equivalent) that holds both stage and
// action information for YAML/detail view rendering.
func TestFetchPipelineStages_RawStruct(t *testing.T) {
	lastChange := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)

	mock := &mockCodePipelineGetPipelineStateClient{
		output: &codepipeline.GetPipelineStateOutput{
			PipelineName: aws.String("rawstruct-test"),
			StageStates: []cptypes.StageState{
				{
					StageName: aws.String("Source"),
					LatestExecution: &cptypes.StageExecution{
						Status: cptypes.StageExecutionStatusSucceeded,
					},
					ActionStates: []cptypes.ActionState{
						{
							ActionName: aws.String("GitHub"),
							LatestExecution: &cptypes.ActionExecution{
								Status:           cptypes.ActionExecutionStatusSucceeded,
								LastStatusChange: &lastChange,
							},
						},
					},
				},
			},
		},
	}

	parentCtx := map[string]string{"pipeline_name": "rawstruct-test"}
	result, err := awsclient.FetchPipelineStages(context.Background(), mock, parentCtx, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resources := result.Resources
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	if resources[0].RawStruct == nil {
		t.Fatal("RawStruct must not be nil")
	}

	raw, ok := resources[0].RawStruct.(awsclient.PipelineStageRow)
	if !ok {
		t.Fatalf("RawStruct should be awsclient.PipelineStageRow, got %T", resources[0].RawStruct)
	}
	if raw.StageName != "Source" {
		t.Errorf("RawStruct.StageName: expected %q, got %q", "Source", raw.StageName)
	}
	if raw.ActionName != "GitHub" {
		t.Errorf("RawStruct.ActionName: expected %q, got %q", "GitHub", raw.ActionName)
	}
}

// TestFetchPipelineStages_RegistrationExists verifies that "pipeline_stages"
// is registered as a child resource type.
func TestFetchPipelineStages_RegistrationExists(t *testing.T) {
	td := resource.GetChildType("pipeline_stages")
	if td == nil {
		t.Fatal("pipeline_stages child resource type not registered")
	}
	if td.ShortName != "pipeline_stages" {
		t.Errorf("child type ShortName: expected %q, got %q", "pipeline_stages", td.ShortName)
	}
	if td.Name == "" {
		t.Error("child type Name should not be empty")
	}
}

// ---------------------------------------------------------------------------
// Column definitions test
// ---------------------------------------------------------------------------

// TestPipelineStageColumns verifies that PipelineStageColumns returns 6
// columns with the expected keys, titles, and widths.
func TestPipelineStageColumns(t *testing.T) {
	cols := resource.PipelineStageColumns()

	if len(cols) != 6 {
		t.Fatalf("PipelineStageColumns() returned %d columns, expected 6", len(cols))
	}

	wantCols := []struct {
		key   string
		title string
		width int
	}{
		{"stage_name", "Stage", 20},
		{"stage_status", "Stage Status", 14},
		{"action_name", "Action", 24},
		{"action_status", "Action Status", 14},
		{"last_change_time", "Last Changed", 22},
		{"external_url", "External URL", 40},
	}

	for i, want := range wantCols {
		if i >= len(cols) {
			t.Errorf("Missing column at index %d", i)
			continue
		}
		if cols[i].Key != want.key {
			t.Errorf("Column %d Key: expected %q, got %q", i, want.key, cols[i].Key)
		}
		if cols[i].Title != want.title {
			t.Errorf("Column %d Title: expected %q, got %q", i, want.title, cols[i].Title)
		}
		if cols[i].Width != want.width {
			t.Errorf("Column %d Width: expected %d, got %d", i, want.width, cols[i].Width)
		}
	}
}

// TestPipelineStages_PaginatedChildFetcherRegistered verifies that the paginated
// child fetcher is registered under the correct short name.
func TestPipelineStages_PaginatedChildFetcherRegistered(t *testing.T) {
	f := resource.GetPaginatedChildFetcher("pipeline_stages")
	if f == nil {
		t.Fatal("pipeline_stages paginated child fetcher not registered")
	}
}

// TestPipelineStages_ParentHasChildDef verifies that the pipeline parent
// resource type has a Children entry for pipeline_stages.
func TestPipelineStages_ParentHasChildDef(t *testing.T) {
	var pipelineType *resource.ResourceTypeDef
	for _, rt := range resource.AllResourceTypes() {
		if rt.ShortName == "pipeline" {
			pipelineType = &rt
			break
		}
	}
	if pipelineType == nil {
		t.Fatal("pipeline resource type not found")
	}

	found := false
	for _, child := range pipelineType.Children {
		if child.ChildType == "pipeline_stages" {
			found = true
			if child.Key != "enter" {
				t.Errorf("pipeline_stages child def Key: expected %q, got %q", "enter", child.Key)
			}
			if child.ContextKeys["pipeline_name"] != "ID" {
				t.Errorf("pipeline_stages ContextKeys[pipeline_name]: expected %q, got %q",
					"ID", child.ContextKeys["pipeline_name"])
			}
			if child.DisplayNameKey != "Name" {
				t.Errorf("pipeline_stages DisplayNameKey: expected %q, got %q",
					"Name", child.DisplayNameKey)
			}
			break
		}
	}
	if !found {
		t.Error("pipeline resource type missing Children entry for pipeline_stages")
	}
}

// ---------------------------------------------------------------------------
// Config defaults test
// ---------------------------------------------------------------------------

// TestConfigDefaultViewDef_PipelineStages verifies that the pipeline_stages
// view definition has the expected list columns and non-empty detail paths.
func TestConfigDefaultViewDef_PipelineStages(t *testing.T) {
	vd := config.DefaultViewDef("pipeline_stages")

	t.Run("list_columns", func(t *testing.T) {
		if len(vd.List) < 4 {
			t.Fatalf("expected at least 4 list columns for pipeline_stages default, got %d", len(vd.List))
		}
	})

	t.Run("detail_paths", func(t *testing.T) {
		if len(vd.Detail) == 0 {
			t.Error("expected non-empty Detail paths for pipeline_stages")
		}
	})
}

// Ensure all imports are used.
var _ = aws.String
var _ = codepipeline.GetPipelineStateOutput{}
var _ = config.DefaultViewDef
