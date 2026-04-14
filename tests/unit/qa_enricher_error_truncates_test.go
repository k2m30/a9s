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

	result, err := awsclient.EnrichCodeBuildStatus(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error (per-resource errors must not propagate): %v", err)
	}
	if !result.Truncated {
		t.Error("Truncated must be true when a per-resource ListBuildsForProject call fails — was the per-resource error handling reverted?")
	}
}

// TestEnrichTargetGroupHealth_DescribeError_SetsTruncated verifies that a
// DescribeTargetHealth error sets Truncated=true.
func TestEnrichTargetGroupHealth_DescribeError_SetsTruncated(t *testing.T) {
	fake := &tgHealthFake{
		err: errFakeAPI, // all calls return error
	}
	clients := &awsclient.ServiceClients{ELBv2: fake}
	resources := []resource.Resource{
		{ID: "arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/err-tg/bbb"},
		{ID: "arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/err2-tg/ccc"},
	}

	result, err := awsclient.EnrichTargetGroupHealth(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error (per-resource errors must not propagate): %v", err)
	}
	if !result.Truncated {
		t.Error("Truncated must be true when DescribeTargetHealth fails — was the per-resource error handling reverted?")
	}
}

// TestEnrichCodePipelineStatus_GetStateError_SetsTruncated verifies that a
// GetPipelineState error sets Truncated=true.
func TestEnrichCodePipelineStatus_GetStateError_SetsTruncated(t *testing.T) {
	fake := &pipelineStateFake{
		err: errFakeAPI,
	}
	clients := &awsclient.ServiceClients{CodePipeline: fake}
	resources := []resource.Resource{
		{Name: "pipeline-that-errors"},
		{Name: "another-pipeline"},
	}

	result, err := awsclient.EnrichCodePipelineStatus(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error (per-resource errors must not propagate): %v", err)
	}
	if !result.Truncated {
		t.Error("Truncated must be true when GetPipelineState fails — was the per-resource error handling reverted?")
	}
}

// TestEnrichDynamoDBStatus_DescribeTableError_SetsTruncated verifies that a
// DescribeTable error sets Truncated=true.
func TestEnrichDynamoDBStatus_DescribeTableError_SetsTruncated(t *testing.T) {
	fake := &ddbStatusFake{
		err: errFakeAPI,
	}
	clients := &awsclient.ServiceClients{DynamoDB: fake}
	resources := []resource.Resource{
		{ID: "arn:aws:dynamodb:us-east-1:123456789012:table/err-table", Name: "err-table"},
		{ID: "arn:aws:dynamodb:us-east-1:123456789012:table/ok-table", Name: "ok-table"},
	}

	result, err := awsclient.EnrichDynamoDBStatus(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error (per-resource errors must not propagate): %v", err)
	}
	if !result.Truncated {
		t.Error("Truncated must be true when DescribeTable fails — was the per-resource error handling reverted?")
	}
}

// TestEnrichStepFunctionsStatus_ListExecutionsError_SetsTruncated verifies that a
// ListExecutions error sets Truncated=true.
func TestEnrichStepFunctionsStatus_ListExecutionsError_SetsTruncated(t *testing.T) {
	fake := &sfnEnrichFake{
		err: errFakeAPI,
	}
	clients := &awsclient.ServiceClients{SFN: fake}
	resources := []resource.Resource{
		{ID: "arn:aws:states:us-east-1:123456789012:stateMachine:err-sm"},
		{ID: "arn:aws:states:us-east-1:123456789012:stateMachine:ok-sm"},
	}

	result, err := awsclient.EnrichStepFunctionsStatus(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error (per-resource errors must not propagate): %v", err)
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

	result, err := awsclient.EnrichGlueJobStatus(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error (per-resource errors must not propagate): %v", err)
	}
	if !result.Truncated {
		t.Error("Truncated must be true when GetJobRuns fails — was the per-resource error handling reverted?")
	}
}
