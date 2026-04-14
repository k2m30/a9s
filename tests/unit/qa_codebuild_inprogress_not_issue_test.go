package unit

// qa_codebuild_inprogress_not_issue_test.go — Regression: CodeBuild IN_PROGRESS
// builds must not produce findings.
//
// Bug: IN_PROGRESS builds were included in findings, causing false issues.
// Fix: IN_PROGRESS status is explicitly skipped (same as SUCCEEDED).
//
// Test fails if the fix is reverted: an IN_PROGRESS build would produce a finding.

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cbtypes "github.com/aws/aws-sdk-go-v2/service/codebuild/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// TestEnrichCodeBuildStatus_InProgressBuild_NotAnIssue verifies that an IN_PROGRESS
// build does not appear in Findings. IN_PROGRESS builds are active work — not failures.
func TestEnrichCodeBuildStatus_InProgressBuild_NotAnIssue(t *testing.T) {
	fake := &codeBuildEnrichFake{
		projectBuilds: map[string]string{
			"active-project": "active-project:build-in-progress",
		},
		builds: map[string]cbtypes.Build{
			"active-project:build-in-progress": {
				Id:          aws.String("active-project:build-in-progress"),
				BuildStatus: cbtypes.StatusTypeInProgress,
				// No EndTime — build is still running.
			},
		},
	}
	clients := &awsclient.ServiceClients{CodeBuild: fake}
	resources := []resource.Resource{{ID: "active-project"}}

	result, err := awsclient.EnrichCodeBuildStatus(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result.Findings["active-project"]; ok {
		t.Error("IN_PROGRESS build must NOT appear in Findings — was the IN_PROGRESS skip reverted?")
	}
	if result.IssueCount != 0 {
		t.Errorf("IssueCount = %d, want 0 for IN_PROGRESS build", result.IssueCount)
	}
}

// TestEnrichCodeBuildStatus_InProgressAndFailed_OnlyFailedIsIssue verifies that
// when a project has an IN_PROGRESS build (latest), it is not a finding, but a
// different project with a FAILED build still produces a finding. This pins
// that IN_PROGRESS skip is per-build, not per-enricher-run.
func TestEnrichCodeBuildStatus_InProgressAndFailed_OnlyFailedIsIssue(t *testing.T) {
	fake := &codeBuildEnrichFake{
		projectBuilds: map[string]string{
			"running-project": "running-project:build-running",
			"failed-project":  "failed-project:build-failed",
		},
		builds: map[string]cbtypes.Build{
			"running-project:build-running": {
				Id:          aws.String("running-project:build-running"),
				BuildStatus: cbtypes.StatusTypeInProgress,
			},
			"failed-project:build-failed": {
				Id:          aws.String("failed-project:build-failed"),
				BuildStatus: cbtypes.StatusTypeFailed,
			},
		},
	}
	clients := &awsclient.ServiceClients{CodeBuild: fake}
	resources := []resource.Resource{
		{ID: "running-project"},
		{ID: "failed-project"},
	}

	result, err := awsclient.EnrichCodeBuildStatus(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result.Findings["running-project"]; ok {
		t.Error("IN_PROGRESS build must NOT appear in Findings")
	}
	if _, ok := result.Findings["failed-project"]; !ok {
		t.Error("FAILED build must appear in Findings")
	}
	if result.IssueCount != 1 {
		t.Errorf("IssueCount = %d, want 1 (only the FAILED build)", result.IssueCount)
	}
}
