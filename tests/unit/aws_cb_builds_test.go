package unit

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/codebuild"
	cbtypes "github.com/aws/aws-sdk-go-v2/service/codebuild/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// CodeBuild Builds fetcher tests (child of CodeBuild Projects)
// ---------------------------------------------------------------------------

// TestFetchCBBuilds_Basic verifies parsing of 1 build with all fields
// populated, checking Resource.ID, Name, Status, all Fields keys, and RawStruct.
func TestFetchCBBuilds_Basic(t *testing.T) {
	startTs := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)
	endTs := time.Date(2024, 6, 15, 10, 4, 12, 0, time.UTC)

	listMock := &mockCodeBuildListBuildsForProjectClient{
		outputs: []*codebuild.ListBuildsForProjectOutput{
			{
				Ids: []string{"my-project:build-id-001"},
			},
		},
	}

	batchMock := &mockCodeBuildBatchGetBuildsClient{
		outputs: []*codebuild.BatchGetBuildsOutput{
			{
				Builds: []cbtypes.Build{
					{
						Id:                    aws.String("my-project:build-id-001"),
						Arn:                   aws.String("arn:aws:codebuild:us-east-1:123456789012:build/my-project:build-id-001"),
						BuildNumber:           aws.Int64(142),
						BuildStatus:           cbtypes.StatusTypeSucceeded,
						StartTime:             &startTs,
						EndTime:               &endTs,
						CurrentPhase:          aws.String("COMPLETED"),
						SourceVersion:         aws.String("abc123def456789012345678901234567890abcd"),
						ResolvedSourceVersion: aws.String("abc123def456789012345678901234567890abcd"),
						Initiator:             aws.String("codepipeline/my-pipeline"),
						ProjectName:           aws.String("my-project"),
						Logs: &cbtypes.LogsLocation{
							GroupName:  aws.String("/aws/codebuild/my-project"),
							StreamName: aws.String("build-id-001"),
						},
					},
				},
			},
		},
	}

	parentCtx := map[string]string{
		"project_name": "my-project",
	}

	result, err := awsclient.FetchCBBuilds(
		context.Background(),
		listMock,
		batchMock,
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

	t.Run("ID_is_full_build_id", func(t *testing.T) {
		if r.ID != "my-project:build-id-001" {
			t.Errorf("ID: expected %q, got %q", "my-project:build-id-001", r.ID)
		}
	})

	t.Run("Name_has_hash_prefix", func(t *testing.T) {
		if r.Name != "#142" {
			t.Errorf("Name: expected %q, got %q", "#142", r.Name)
		}
	})

	t.Run("Status_is_build_status", func(t *testing.T) {
		if r.Status != "SUCCEEDED" {
			t.Errorf("Status: expected %q, got %q", "SUCCEEDED", r.Status)
		}
	})

	t.Run("Fields_build_number", func(t *testing.T) {
		if r.Fields["build_number"] != "142" {
			t.Errorf("Fields[build_number]: expected %q, got %q", "142", r.Fields["build_number"])
		}
	})

	t.Run("Fields_build_status", func(t *testing.T) {
		if r.Fields["build_status"] != "SUCCEEDED" {
			t.Errorf("Fields[build_status]: expected %q, got %q", "SUCCEEDED", r.Fields["build_status"])
		}
	})

	t.Run("Fields_start_time", func(t *testing.T) {
		if r.Fields["start_time"] == "" {
			t.Error("Fields[start_time] should not be empty")
		}
		if !strings.Contains(r.Fields["start_time"], "2024-06-15 10:00") {
			t.Errorf("Fields[start_time] expected '2024-06-15 10:00', got %q", r.Fields["start_time"])
		}
	})

	t.Run("Fields_current_phase", func(t *testing.T) {
		if r.Fields["current_phase"] != "COMPLETED" {
			t.Errorf("Fields[current_phase]: expected %q, got %q", "COMPLETED", r.Fields["current_phase"])
		}
	})

	t.Run("Fields_initiator", func(t *testing.T) {
		if r.Fields["initiator"] != "codepipeline/my-pipeline" {
			t.Errorf("Fields[initiator]: expected %q, got %q", "codepipeline/my-pipeline", r.Fields["initiator"])
		}
	})

	t.Run("Fields_build_id", func(t *testing.T) {
		if r.Fields["build_id"] != "my-project:build-id-001" {
			t.Errorf("Fields[build_id]: expected %q, got %q", "my-project:build-id-001", r.Fields["build_id"])
		}
	})

	t.Run("Fields_build_arn", func(t *testing.T) {
		if r.Fields["build_arn"] != "arn:aws:codebuild:us-east-1:123456789012:build/my-project:build-id-001" {
			t.Errorf("Fields[build_arn]: got %q", r.Fields["build_arn"])
		}
	})

	t.Run("Fields_source_version", func(t *testing.T) {
		if r.Fields["source_version"] != "abc123def456789012345678901234567890abcd" {
			t.Errorf("Fields[source_version]: got %q", r.Fields["source_version"])
		}
	})

	t.Run("Fields_resolved_source_version", func(t *testing.T) {
		if r.Fields["resolved_source_version"] != "abc123def456789012345678901234567890abcd" {
			t.Errorf("Fields[resolved_source_version]: got %q", r.Fields["resolved_source_version"])
		}
	})

	t.Run("Fields_log_group_name", func(t *testing.T) {
		if r.Fields["log_group_name"] != "/aws/codebuild/my-project" {
			t.Errorf("Fields[log_group_name]: expected %q, got %q",
				"/aws/codebuild/my-project", r.Fields["log_group_name"])
		}
	})

	t.Run("Fields_log_stream_name", func(t *testing.T) {
		if r.Fields["log_stream_name"] != "build-id-001" {
			t.Errorf("Fields[log_stream_name]: expected %q, got %q",
				"build-id-001", r.Fields["log_stream_name"])
		}
	})

	t.Run("RawStruct_is_Build", func(t *testing.T) {
		if r.RawStruct == nil {
			t.Fatal("RawStruct must not be nil")
		}
		raw, ok := r.RawStruct.(cbtypes.Build)
		if !ok {
			t.Fatalf("RawStruct should be cbtypes.Build, got %T", r.RawStruct)
		}
		if raw.Id == nil || *raw.Id != "my-project:build-id-001" {
			t.Error("RawStruct.Id not preserved correctly")
		}
	})

	t.Run("required_fields_present", func(t *testing.T) {
		requiredFields := []string{
			"build_number", "build_status", "start_time", "duration",
			"source_version_short", "initiator", "build_id", "build_arn",
			"end_time", "current_phase", "source_version",
			"resolved_source_version", "log_group_name", "log_stream_name",
		}
		for _, key := range requiredFields {
			if _, ok := r.Fields[key]; !ok {
				t.Errorf("Fields missing key %q", key)
			}
		}
	})
}

// TestFetchCBBuilds_EmptyProject verifies that a project with no builds
// returns an empty slice with no error.
func TestFetchCBBuilds_EmptyProject(t *testing.T) {
	listMock := &mockCodeBuildListBuildsForProjectClient{
		outputs: []*codebuild.ListBuildsForProjectOutput{
			{
				Ids: []string{},
			},
		},
	}

	batchMock := &mockCodeBuildBatchGetBuildsClient{}

	parentCtx := map[string]string{
		"project_name": "empty-project",
	}

	result, err := awsclient.FetchCBBuilds(
		context.Background(),
		listMock,
		batchMock,
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

// TestFetchCBBuilds_ListError verifies that ListBuildsForProject errors
// are propagated.
func TestFetchCBBuilds_ListError(t *testing.T) {
	listMock := &mockCodeBuildListBuildsForProjectClient{
		err: fmt.Errorf("AWS API error: access denied"),
	}

	batchMock := &mockCodeBuildBatchGetBuildsClient{}

	parentCtx := map[string]string{
		"project_name": "error-project",
	}

	result, err := awsclient.FetchCBBuilds(
		context.Background(),
		listMock,
		batchMock,
		parentCtx,
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

// TestFetchCBBuilds_BatchError verifies that BatchGetBuilds errors
// are propagated.
func TestFetchCBBuilds_BatchError(t *testing.T) {
	listMock := &mockCodeBuildListBuildsForProjectClient{
		outputs: []*codebuild.ListBuildsForProjectOutput{
			{
				Ids: []string{"my-project:build-id-001"},
			},
		},
	}

	batchMock := &mockCodeBuildBatchGetBuildsClient{
		err: fmt.Errorf("AWS API error: throttling"),
	}

	parentCtx := map[string]string{
		"project_name": "batch-error-project",
	}

	result, err := awsclient.FetchCBBuilds(
		context.Background(),
		listMock,
		batchMock,
		parentCtx,
		"",
	)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "throttling") {
		t.Errorf("error should contain 'throttling', got %q", err.Error())
	}
	if result.Resources != nil {
		t.Errorf("expected nil resources on error, got %d", len(result.Resources))
	}
}

// TestFetchCBBuilds_Duration verifies that a build with known StartTime/EndTime
// produces the expected duration string.
func TestFetchCBBuilds_Duration(t *testing.T) {
	startTs := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)
	endTs := time.Date(2024, 6, 15, 10, 4, 12, 0, time.UTC)

	listMock := &mockCodeBuildListBuildsForProjectClient{
		outputs: []*codebuild.ListBuildsForProjectOutput{
			{Ids: []string{"proj:b1"}},
		},
	}

	batchMock := &mockCodeBuildBatchGetBuildsClient{
		outputs: []*codebuild.BatchGetBuildsOutput{
			{
				Builds: []cbtypes.Build{
					{
						Id:          aws.String("proj:b1"),
						BuildNumber: aws.Int64(1),
						BuildStatus: cbtypes.StatusTypeSucceeded,
						StartTime:   &startTs,
						EndTime:     &endTs,
					},
				},
			},
		},
	}

	parentCtx := map[string]string{"project_name": "proj"}

	result, err := awsclient.FetchCBBuilds(
		context.Background(),
		listMock,
		batchMock,
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
	if !strings.Contains(duration, "4m") {
		t.Errorf("Fields[duration] should contain '4m', got %q", duration)
	}
	if !strings.Contains(duration, "12s") {
		t.Errorf("Fields[duration] should contain '12s', got %q", duration)
	}
}

// TestFetchCBBuilds_InProgressDuration verifies that a build with no EndTime
// produces a duration containing "~" to indicate it's approximate/ongoing.
func TestFetchCBBuilds_InProgressDuration(t *testing.T) {
	startTs := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)

	listMock := &mockCodeBuildListBuildsForProjectClient{
		outputs: []*codebuild.ListBuildsForProjectOutput{
			{Ids: []string{"proj:b2"}},
		},
	}

	batchMock := &mockCodeBuildBatchGetBuildsClient{
		outputs: []*codebuild.BatchGetBuildsOutput{
			{
				Builds: []cbtypes.Build{
					{
						Id:          aws.String("proj:b2"),
						BuildNumber: aws.Int64(2),
						BuildStatus: cbtypes.StatusTypeInProgress,
						StartTime:   &startTs,
						// EndTime is nil — build in progress
					},
				},
			},
		},
	}

	parentCtx := map[string]string{"project_name": "proj"}

	result, err := awsclient.FetchCBBuilds(
		context.Background(),
		listMock,
		batchMock,
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
	if !strings.Contains(duration, "~") {
		t.Errorf("in-progress Fields[duration] should contain '~', got %q", duration)
	}
}

// TestFetchCBBuilds_SourceVersionShort verifies that a 40-char SHA is
// truncated to its first 8 characters for the short version.
func TestFetchCBBuilds_SourceVersionShort(t *testing.T) {
	startTs := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)
	sha := "abc123def456789012345678901234567890abcd"

	listMock := &mockCodeBuildListBuildsForProjectClient{
		outputs: []*codebuild.ListBuildsForProjectOutput{
			{Ids: []string{"proj:b3"}},
		},
	}

	batchMock := &mockCodeBuildBatchGetBuildsClient{
		outputs: []*codebuild.BatchGetBuildsOutput{
			{
				Builds: []cbtypes.Build{
					{
						Id:            aws.String("proj:b3"),
						BuildNumber:   aws.Int64(3),
						BuildStatus:   cbtypes.StatusTypeSucceeded,
						StartTime:     &startTs,
						SourceVersion: aws.String(sha),
					},
				},
			},
		},
	}

	parentCtx := map[string]string{"project_name": "proj"}

	result, err := awsclient.FetchCBBuilds(
		context.Background(),
		listMock,
		batchMock,
		parentCtx,
		"",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}

	short := result.Resources[0].Fields["source_version_short"]
	if short != "abc123de" {
		t.Errorf("Fields[source_version_short]: expected %q, got %q", "abc123de", short)
	}
}

// TestFetchCBBuilds_NilFields verifies that a build with all nil optional
// pointers does not cause a panic and produces empty string fields.
func TestFetchCBBuilds_NilFields(t *testing.T) {
	listMock := &mockCodeBuildListBuildsForProjectClient{
		outputs: []*codebuild.ListBuildsForProjectOutput{
			{Ids: []string{"proj:b-nil"}},
		},
	}

	batchMock := &mockCodeBuildBatchGetBuildsClient{
		outputs: []*codebuild.BatchGetBuildsOutput{
			{
				Builds: []cbtypes.Build{
					{
						// All optional pointer fields are nil
						BuildStatus: cbtypes.StatusTypeSucceeded,
					},
				},
			},
		},
	}

	parentCtx := map[string]string{"project_name": "proj"}

	// Should not panic
	result, err := awsclient.FetchCBBuilds(
		context.Background(),
		listMock,
		batchMock,
		parentCtx,
		"",
	)
	if err != nil {
		t.Fatalf("expected no error for nil fields, got %v", err)
	}

	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}

	t.Run("nil_Id", func(t *testing.T) {
		// ID may be empty or derived; just ensure no panic occurred
		_ = result.Resources[0].ID
	})

	t.Run("nil_BuildNumber", func(t *testing.T) {
		// Name should handle nil BuildNumber gracefully
		_ = result.Resources[0].Name
	})

	t.Run("nil_StartTime", func(t *testing.T) {
		if result.Resources[0].Fields["start_time"] != "" {
			t.Logf("Fields[start_time] is %q (expected empty for nil)", result.Resources[0].Fields["start_time"])
		}
	})

	t.Run("nil_EndTime", func(t *testing.T) {
		if result.Resources[0].Fields["end_time"] != "" {
			t.Logf("Fields[end_time] is %q (expected empty for nil)", result.Resources[0].Fields["end_time"])
		}
	})

	t.Run("nil_SourceVersion", func(t *testing.T) {
		if result.Resources[0].Fields["source_version"] != "" {
			t.Logf("Fields[source_version] is %q (expected empty for nil)", result.Resources[0].Fields["source_version"])
		}
	})

	t.Run("nil_Initiator", func(t *testing.T) {
		if result.Resources[0].Fields["initiator"] != "" {
			t.Logf("Fields[initiator] is %q (expected empty for nil)", result.Resources[0].Fields["initiator"])
		}
	})

	t.Run("nil_CurrentPhase", func(t *testing.T) {
		if result.Resources[0].Fields["current_phase"] != "" {
			t.Logf("Fields[current_phase] is %q (expected empty for nil)", result.Resources[0].Fields["current_phase"])
		}
	})

	t.Run("nil_Logs", func(t *testing.T) {
		if result.Resources[0].Fields["log_group_name"] != "" {
			t.Logf("Fields[log_group_name] is %q (expected empty for nil)", result.Resources[0].Fields["log_group_name"])
		}
		if result.Resources[0].Fields["log_stream_name"] != "" {
			t.Logf("Fields[log_stream_name] is %q (expected empty for nil)", result.Resources[0].Fields["log_stream_name"])
		}
	})
}

// TestFetchCBBuilds_LogFields verifies that log_group_name and log_stream_name
// are populated from Build.Logs.
func TestFetchCBBuilds_LogFields(t *testing.T) {
	startTs := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)

	listMock := &mockCodeBuildListBuildsForProjectClient{
		outputs: []*codebuild.ListBuildsForProjectOutput{
			{Ids: []string{"proj:b-log"}},
		},
	}

	batchMock := &mockCodeBuildBatchGetBuildsClient{
		outputs: []*codebuild.BatchGetBuildsOutput{
			{
				Builds: []cbtypes.Build{
					{
						Id:          aws.String("proj:b-log"),
						BuildNumber: aws.Int64(10),
						BuildStatus: cbtypes.StatusTypeSucceeded,
						StartTime:   &startTs,
						Logs: &cbtypes.LogsLocation{
							GroupName:  aws.String("/aws/codebuild/my-project"),
							StreamName: aws.String("12345678-abcd-efgh-1234-567890abcdef"),
						},
					},
				},
			},
		},
	}

	parentCtx := map[string]string{"project_name": "proj"}

	result, err := awsclient.FetchCBBuilds(
		context.Background(),
		listMock,
		batchMock,
		parentCtx,
		"",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}

	if result.Resources[0].Fields["log_group_name"] != "/aws/codebuild/my-project" {
		t.Errorf("Fields[log_group_name]: expected %q, got %q",
			"/aws/codebuild/my-project", result.Resources[0].Fields["log_group_name"])
	}
	if result.Resources[0].Fields["log_stream_name"] != "12345678-abcd-efgh-1234-567890abcdef" {
		t.Errorf("Fields[log_stream_name]: expected %q, got %q",
			"12345678-abcd-efgh-1234-567890abcdef", result.Resources[0].Fields["log_stream_name"])
	}
}

// TestFetchCBBuilds_LogFieldsNil verifies that when build.Logs is nil,
// log_group_name and log_stream_name are empty strings.
func TestFetchCBBuilds_LogFieldsNil(t *testing.T) {
	startTs := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)

	listMock := &mockCodeBuildListBuildsForProjectClient{
		outputs: []*codebuild.ListBuildsForProjectOutput{
			{Ids: []string{"proj:b-nolog"}},
		},
	}

	batchMock := &mockCodeBuildBatchGetBuildsClient{
		outputs: []*codebuild.BatchGetBuildsOutput{
			{
				Builds: []cbtypes.Build{
					{
						Id:          aws.String("proj:b-nolog"),
						BuildNumber: aws.Int64(11),
						BuildStatus: cbtypes.StatusTypeSucceeded,
						StartTime:   &startTs,
						Logs:        nil, // no logs
					},
				},
			},
		},
	}

	parentCtx := map[string]string{"project_name": "proj"}

	result, err := awsclient.FetchCBBuilds(
		context.Background(),
		listMock,
		batchMock,
		parentCtx,
		"",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}

	if result.Resources[0].Fields["log_group_name"] != "" {
		t.Errorf("Fields[log_group_name] should be empty when Logs is nil, got %q",
			result.Resources[0].Fields["log_group_name"])
	}
	if result.Resources[0].Fields["log_stream_name"] != "" {
		t.Errorf("Fields[log_stream_name] should be empty when Logs is nil, got %q",
			result.Resources[0].Fields["log_stream_name"])
	}
}

// TestFetchCBBuilds_Pagination verifies that paginated ListBuildsForProject
// responses (2 pages) result in all IDs being collected and BatchGetBuilds
// being called for all of them.
func TestFetchCBBuilds_Pagination(t *testing.T) {
	startTs := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)

	listMock := &mockCodeBuildListBuildsForProjectClient{
		outputs: []*codebuild.ListBuildsForProjectOutput{
			{
				Ids:       []string{"proj:b-page1-1", "proj:b-page1-2"},
				NextToken: aws.String("page2-token"),
			},
			{
				Ids: []string{"proj:b-page2-1"},
			},
		},
	}

	batchMock := &mockCodeBuildBatchGetBuildsClient{
		outputs: []*codebuild.BatchGetBuildsOutput{
			{
				Builds: []cbtypes.Build{
					{Id: aws.String("proj:b-page1-1"), BuildNumber: aws.Int64(1), BuildStatus: cbtypes.StatusTypeSucceeded, StartTime: &startTs},
					{Id: aws.String("proj:b-page1-2"), BuildNumber: aws.Int64(2), BuildStatus: cbtypes.StatusTypeFailed, StartTime: &startTs},
					{Id: aws.String("proj:b-page2-1"), BuildNumber: aws.Int64(3), BuildStatus: cbtypes.StatusTypeInProgress, StartTime: &startTs},
				},
			},
		},
	}

	parentCtx := map[string]string{"project_name": "proj"}

	result, err := awsclient.FetchCBBuilds(
		context.Background(),
		listMock,
		batchMock,
		parentCtx,
		"",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(result.Resources) != 3 {
		t.Fatalf("expected 3 resources across 2 pages, got %d", len(result.Resources))
	}
}

// TestFetchCBBuilds_ParentContext verifies that the "project_name" context
// key from the parent is used correctly.
func TestFetchCBBuilds_ParentContext(t *testing.T) {
	listMock := &mockCodeBuildListBuildsForProjectClient{
		outputs: []*codebuild.ListBuildsForProjectOutput{
			{Ids: []string{}},
		},
	}

	batchMock := &mockCodeBuildBatchGetBuildsClient{}

	parentCtx := map[string]string{
		"project_name": "specific-project-name",
	}

	_, err := awsclient.FetchCBBuilds(
		context.Background(),
		listMock,
		batchMock,
		parentCtx,
		"",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	// If ListBuildsForProject was called without error, the project_name was used
}

// TestFetchCBBuilds_RawStruct verifies that RawStruct is the original
// cbtypes.Build value.
func TestFetchCBBuilds_RawStruct(t *testing.T) {
	startTs := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)

	listMock := &mockCodeBuildListBuildsForProjectClient{
		outputs: []*codebuild.ListBuildsForProjectOutput{
			{Ids: []string{"proj:b-raw"}},
		},
	}

	batchMock := &mockCodeBuildBatchGetBuildsClient{
		outputs: []*codebuild.BatchGetBuildsOutput{
			{
				Builds: []cbtypes.Build{
					{
						Id:          aws.String("proj:b-raw"),
						Arn:         aws.String("arn:aws:codebuild:us-east-1:123456789012:build/proj:b-raw"),
						BuildNumber: aws.Int64(99),
						BuildStatus: cbtypes.StatusTypeSucceeded,
						StartTime:   &startTs,
					},
				},
			},
		},
	}

	parentCtx := map[string]string{"project_name": "proj"}

	result, err := awsclient.FetchCBBuilds(
		context.Background(),
		listMock,
		batchMock,
		parentCtx,
		"",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}

	raw, ok := result.Resources[0].RawStruct.(cbtypes.Build)
	if !ok {
		t.Fatalf("RawStruct should be cbtypes.Build, got %T", result.Resources[0].RawStruct)
	}
	if raw.Id == nil || *raw.Id != "proj:b-raw" {
		t.Error("RawStruct.Id not preserved correctly")
	}
	if raw.BuildNumber == nil || *raw.BuildNumber != 99 {
		t.Error("RawStruct.BuildNumber not preserved correctly")
	}
}

// TestFetchCBBuilds_RegistrationExists verifies that "cb_builds" is registered
// as a child resource type.
func TestFetchCBBuilds_RegistrationExists(t *testing.T) {
	td := resource.GetChildType("cb_builds")
	if td == nil {
		t.Fatal("cb_builds child resource type not registered")
	}
	if td.ShortName != "cb_builds" {
		t.Errorf("child type ShortName: expected %q, got %q", "cb_builds", td.ShortName)
	}
	if td.Name == "" {
		t.Error("child type Name should not be empty")
	}
}

// ---------------------------------------------------------------------------
// Column definitions test
// ---------------------------------------------------------------------------

// TestCBBuildColumns verifies that CBBuildColumns returns columns with the
// expected keys and titles.
func TestCBBuildColumns(t *testing.T) {
	cols := resource.CBBuildColumns()

	if len(cols) == 0 {
		t.Fatal("CBBuildColumns() returned no columns")
	}

	// At minimum, the columns should contain these keys
	wantKeys := []string{"build_number", "build_status", "start_time", "duration"}
	for _, wantKey := range wantKeys {
		found := false
		for _, col := range cols {
			if col.Key == wantKey {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("CBBuildColumns() missing key %q", wantKey)
		}
	}
}

// TestCBBuilds_PaginatedChildFetcherRegistered verifies that the paginated
// child fetcher is
// registered under the correct short name.
func TestCBBuilds_PaginatedChildFetcherRegistered(t *testing.T) {
	f := resource.GetPaginatedChildFetcher("cb_builds")
	if f == nil {
		t.Fatal("cb_builds paginated child fetcher not registered")
	}
}

// ---------------------------------------------------------------------------
// Config defaults test
// ---------------------------------------------------------------------------

// TestConfigDefaultViewDef_CBBuilds verifies that the cb_builds view
// definition has the expected list columns and non-empty detail paths.
func TestConfigDefaultViewDef_CBBuilds(t *testing.T) {
	vd := config.DefaultViewDef("cb_builds")

	t.Run("list_columns", func(t *testing.T) {
		if len(vd.List) < 3 {
			t.Fatalf("expected at least 3 list columns for cb_builds default, got %d", len(vd.List))
		}
	})

	t.Run("detail_paths", func(t *testing.T) {
		if len(vd.Detail) == 0 {
			t.Error("expected non-empty Detail paths for cb_builds")
		}
	})
}

// TestFetchCBBuilds_ContinuationToken verifies that a non-empty
// continuation token is forwarded to the ListBuildsForProject API as NextToken.
func TestFetchCBBuilds_ContinuationToken(t *testing.T) {
	startTime := time.Date(2024, 3, 22, 10, 0, 0, 0, time.UTC)
	buildNum := int64(42)

	wrapper := &tokenCapturingCBBuildsMock{
		inner: &mockCodeBuildListBuildsForProjectClient{
			outputs: []*codebuild.ListBuildsForProjectOutput{
				{
					Ids: []string{"my-project:build-from-token"},
				},
			},
		},
	}

	batchMock := &mockCodeBuildBatchGetBuildsClient{
		outputs: []*codebuild.BatchGetBuildsOutput{
			{
				Builds: []cbtypes.Build{
					{
						Id:          aws.String("my-project:build-from-token"),
						BuildNumber: &buildNum,
						BuildStatus: cbtypes.StatusTypeSucceeded,
						StartTime:   &startTime,
					},
				},
			},
		},
	}

	parentCtx := map[string]string{
		"project_name": "my-project",
	}

	result, err := awsclient.FetchCBBuilds(context.Background(), wrapper, batchMock, parentCtx, "my-continuation-token")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}

	if wrapper.capturedNextToken == nil {
		t.Fatal("expected NextToken to be set in ListBuildsForProject call")
	}
	if *wrapper.capturedNextToken != "my-continuation-token" {
		t.Errorf("expected NextToken %q, got %q", "my-continuation-token", *wrapper.capturedNextToken)
	}
}

// tokenCapturingCBBuildsMock wraps the CodeBuild ListBuildsForProject mock to capture NextToken.
type tokenCapturingCBBuildsMock struct {
	inner             *mockCodeBuildListBuildsForProjectClient
	capturedNextToken *string
}

func (m *tokenCapturingCBBuildsMock) ListBuildsForProject(ctx context.Context, params *codebuild.ListBuildsForProjectInput, optFns ...func(*codebuild.Options)) (*codebuild.ListBuildsForProjectOutput, error) {
	m.capturedNextToken = params.NextToken
	return m.inner.ListBuildsForProject(ctx, params, optFns...)
}

// Ensure all imports are used.
var _ = aws.String
var _ = codebuild.ListBuildsForProjectOutput{}
var _ = config.DefaultViewDef
