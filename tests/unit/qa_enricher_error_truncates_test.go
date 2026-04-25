package unit

// qa_enricher_error_truncates_test.go — Regression: per-resource API errors mark result as Truncated.
//
// Bug: When a per-resource API call fails, the enricher was returning an error
// (aborting the whole run) rather than continuing and marking Truncated=true.
// Fix: Per-resource API errors set truncated=true and continue processing
// remaining resources.
//
// Tests fail if the fix is reverted: Truncated would be false when an API error
// occurs, causing the badge to show a definitive count rather than "N+".

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	cbtypes "github.com/aws/aws-sdk-go-v2/service/codebuild/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

var errFakeAPI = errors.New("simulated API error")

// TestEnrichCodeBuildStatus_ListBuildsError_SetsTruncated verifies that a
// ListBuildsForProject error sets Truncated=true (not an abort).
func TestEnrichCodeBuildStatus_ListBuildsError_SetsTruncated(t *testing.T) {
	endTime := time.Date(2026, 4, 14, 12, 0, 0, 0, time.UTC)

	// Use the listErr field to simulate a failure for all ListBuildsForProject calls.
	fake := &codeBuildEnrichFake{
		listErr: errFakeAPI,
		projectBuilds: map[string]string{
			"proj-ok": "proj-ok:b1",
		},
		builds: map[string]cbtypes.Build{
			"proj-ok:b1": {
				Id:          aws.String("proj-ok:b1"),
				BuildStatus: cbtypes.StatusTypeSucceeded,
				EndTime:     &endTime,
			},
		},
	}
	clients := &awsclient.ServiceClients{CodeBuild: fake}
	resources := []resource.Resource{{ID: "proj-error"}, {ID: "proj-ok"}}

	result, err := awsclient.EnrichCodeBuildStatus(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error (per-resource errors must not propagate): %v", err)
	}
	if !result.Truncated {
		t.Error("Truncated must be true when a per-resource ListBuildsForProject call fails — was the per-resource error handling reverted?")
	}
}

// TestEnrichTargetGroupHealth_DescribeError_SetsTruncated verifies that a
// DescribeTargetHealth error sets Truncated=true and surfaces a composite error
// containing the enricher prefix and the failing resource ID.
func TestEnrichTargetGroupHealth_DescribeError_SetsTruncated(t *testing.T) {
	const tgName = "err-tg"
	const tgARN = "arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/err-tg/bbb"
	fake := &tgHealthFake{
		err: errFakeAPI, // all calls return error
	}
	clients := &awsclient.ServiceClients{ELBv2: fake}
	resources := []resource.Resource{
		{ID: tgName, Fields: map[string]string{"target_group_arn": tgARN}},
		{ID: "err2-tg", Fields: map[string]string{"target_group_arn": "arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/err2-tg/ccc"}},
	}

	result, err := awsclient.EnrichTargetGroupHealth(context.Background(), clients, resources, nil)
	if err == nil {
		t.Fatal("enricher must surface a composite error when DescribeTargetHealth fails")
	}
	if errStr := err.Error(); !strings.Contains(errStr, "tg-enrich:") {
		t.Errorf("composite error must contain \"tg-enrich:\", got: %q", errStr)
	}
	if errStr := err.Error(); !strings.Contains(errStr, tgName) {
		t.Errorf("composite error must contain the failing target group ID %q, got: %q", tgName, errStr)
	}
	if !result.Truncated {
		t.Error("Truncated must be true when DescribeTargetHealth fails — was the per-resource error handling reverted?")
	}
}

// TestEnrichCodePipelineStatus_GetStateError_SetsTruncated verifies that a
// GetPipelineState error sets Truncated=true and surfaces a composite error
// containing the enricher prefix.
func TestEnrichCodePipelineStatus_GetStateError_SetsTruncated(t *testing.T) {
	fake := &pipelineStateFake{
		err: errFakeAPI,
	}
	clients := &awsclient.ServiceClients{CodePipeline: fake}
	resources := []resource.Resource{
		{Name: "pipeline-that-errors"},
		{Name: "another-pipeline"},
	}

	result, err := awsclient.EnrichCodePipelineStatus(context.Background(), clients, resources, nil)
	if err == nil {
		t.Fatal("enricher must surface a composite error when GetPipelineState fails")
	}
	if errStr := err.Error(); !strings.Contains(errStr, "pipeline-enrich:") {
		t.Errorf("composite error must contain \"pipeline-enrich:\", got: %q", errStr)
	}
	if !result.Truncated {
		t.Error("Truncated must be true when GetPipelineState fails — was the per-resource error handling reverted?")
	}
}

// TestEnrichStepFunctionsStatus_ListExecutionsError_SetsTruncated verifies that a
// ListExecutions error sets Truncated=true and surfaces a composite error containing
// the enricher prefix and the failing state machine ARN.
func TestEnrichStepFunctionsStatus_ListExecutionsError_SetsTruncated(t *testing.T) {
	const smName = "err-sm"
	const smARN = "arn:aws:states:us-east-1:123456789012:stateMachine:err-sm"
	fake := &sfnEnrichFake{
		err: errFakeAPI,
	}
	clients := &awsclient.ServiceClients{SFN: fake}
	resources := []resource.Resource{
		{ID: smName, Fields: map[string]string{"arn": smARN}},
		{ID: "ok-sm", Fields: map[string]string{"arn": "arn:aws:states:us-east-1:123456789012:stateMachine:ok-sm"}},
	}

	result, err := awsclient.EnrichStepFunctionsStatus(context.Background(), clients, resources, nil)
	if err == nil {
		t.Fatal("enricher must surface a composite error when ListExecutions fails")
	}
	if errStr := err.Error(); !strings.Contains(errStr, "sfn-enrich:") {
		t.Errorf("composite error must contain \"sfn-enrich:\", got: %q", errStr)
	}
	if errStr := err.Error(); !strings.Contains(errStr, smName) {
		t.Errorf("composite error must contain the failing state machine ID %q, got: %q", smName, errStr)
	}
	if !result.Truncated {
		t.Error("Truncated must be true when ListExecutions fails — was the per-resource error handling reverted?")
	}
}

// TestEnrichGlueJobStatus_GetJobRunsError_SetsTruncated verifies that a
// GetJobRuns error sets Truncated=true.
func TestEnrichGlueJobStatus_GetJobRunsError_SetsTruncated(t *testing.T) {
	fake := &glueJobFake{
		err: errFakeAPI,
	}
	clients := &awsclient.ServiceClients{Glue: fake}
	resources := []resource.Resource{
		{ID: "arn:aws:glue:us-east-1:123456789012:job/err-job", Name: "err-job"},
		{ID: "arn:aws:glue:us-east-1:123456789012:job/ok-job", Name: "ok-job"},
	}

	result, err := awsclient.EnrichGlueJobStatus(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error (per-resource errors must not propagate): %v", err)
	}
	if !result.Truncated {
		t.Error("Truncated must be true when GetJobRuns fails — was the per-resource error handling reverted?")
	}
}
