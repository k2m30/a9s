package unit

// enrichment_glue_findings_test.go — Behavioral tests for EnrichGlueJobStatus.
//
// Contract assertions (enricher-contract.md):
//   - Returns EnricherResult.Findings keyed by job name (r.Name).
//   - Severity "!" for all findings.
//   - Summary "latest run FAILED" / "latest run ERROR" / "latest run TIMEOUT".
//   - IssueCount = len(Findings).
//   - Truncated = true when len(resources) > EnrichmentCap.
//   - Jobs with SUCCEEDED/RUNNING latest run must NOT appear in Findings.
//   - Empty resources → non-nil empty Findings map.

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/glue"
	gluetypes "github.com/aws/aws-sdk-go-v2/service/glue/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// glueJobFake implements GlueAPI subset for enrichment testing.
type glueJobFake struct {
	awsclient.GlueAPI
	// jobRuns maps job name → latest run state
	jobRuns map[string]gluetypes.JobRunState
	err     error
}

func (f *glueJobFake) GetJobRuns(
	_ context.Context,
	params *glue.GetJobRunsInput,
	_ ...func(*glue.Options),
) (*glue.GetJobRunsOutput, error) {
	if f.err != nil {
		return nil, f.err
	}
	name := aws.ToString(params.JobName)
	if state, ok := f.jobRuns[name]; ok {
		return &glue.GetJobRunsOutput{
			JobRuns: []gluetypes.JobRun{
				{JobName: &name, JobRunState: state},
			},
		}, nil
	}
	return &glue.GetJobRunsOutput{}, nil
}

// TestEnrichGlueJobStatus_FailedFindingKeyedByJobName verifies findings are
// keyed by job name (r.Name).
func TestEnrichGlueJobStatus_FailedFindingKeyedByJobName(t *testing.T) {
	fake := &glueJobFake{
		jobRuns: map[string]gluetypes.JobRunState{
			"my-glue-job": gluetypes.JobRunStateFailed,
		},
	}
	clients := &awsclient.ServiceClients{Glue: fake}
	resources := []resource.Resource{{Name: "my-glue-job"}}

	result, err := awsclient.EnrichGlueJobStatus(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result.Findings["my-glue-job"]; !ok {
		t.Errorf("expected finding keyed by job name %q", "my-glue-job")
	}
}

// TestEnrichGlueJobStatus_SummaryContainsFAILED verifies the summary for FAILED state.
func TestEnrichGlueJobStatus_SummaryContainsFAILED(t *testing.T) {
	fake := &glueJobFake{
		jobRuns: map[string]gluetypes.JobRunState{
			"fail-job": gluetypes.JobRunStateFailed,
		},
	}
	clients := &awsclient.ServiceClients{Glue: fake}
	resources := []resource.Resource{{Name: "fail-job"}}

	result, err := awsclient.EnrichGlueJobStatus(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	summary := result.Findings["fail-job"].Phrase
	if !strings.Contains(summary, "FAILED") {
		t.Errorf("summary %q must contain %q", summary, "FAILED")
	}
}

// TestEnrichGlueJobStatus_SummaryContainsERROR verifies the summary for ERROR state.
func TestEnrichGlueJobStatus_SummaryContainsERROR(t *testing.T) {
	fake := &glueJobFake{
		jobRuns: map[string]gluetypes.JobRunState{
			"err-job": gluetypes.JobRunStateError,
		},
	}
	clients := &awsclient.ServiceClients{Glue: fake}
	resources := []resource.Resource{{Name: "err-job"}}

	result, err := awsclient.EnrichGlueJobStatus(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	summary := result.Findings["err-job"].Phrase
	if !strings.Contains(summary, "ERROR") {
		t.Errorf("summary %q must contain %q", summary, "ERROR")
	}
}

// TestEnrichGlueJobStatus_SummaryContainsTIMEOUT verifies the summary for TIMEOUT state.
func TestEnrichGlueJobStatus_SummaryContainsTIMEOUT(t *testing.T) {
	fake := &glueJobFake{
		jobRuns: map[string]gluetypes.JobRunState{
			"timeout-job": gluetypes.JobRunStateTimeout,
		},
	}
	clients := &awsclient.ServiceClients{Glue: fake}
	resources := []resource.Resource{{Name: "timeout-job"}}

	result, err := awsclient.EnrichGlueJobStatus(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	summary := result.Findings["timeout-job"].Phrase
	if !strings.Contains(summary, "TIMEOUT") {
		t.Errorf("summary %q must contain %q", summary, "TIMEOUT")
	}
}

// TestEnrichGlueJobStatus_SucceededExcluded verifies SUCCEEDED jobs do not appear in Findings.
func TestEnrichGlueJobStatus_SucceededExcluded(t *testing.T) {
	fake := &glueJobFake{
		jobRuns: map[string]gluetypes.JobRunState{
			"ok-job": gluetypes.JobRunStateSucceeded,
		},
	}
	clients := &awsclient.ServiceClients{Glue: fake}
	resources := []resource.Resource{{Name: "ok-job"}}

	result, err := awsclient.EnrichGlueJobStatus(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result.Findings["ok-job"]; ok {
		t.Error("SUCCEEDED job must NOT appear in Findings")
	}
	if result.IssueCount != 0 {
		t.Errorf("IssueCount = %d, want 0", result.IssueCount)
	}
}

// TestEnrichGlueJobStatus_RunningExcluded verifies RUNNING jobs do not appear in Findings.
func TestEnrichGlueJobStatus_RunningExcluded(t *testing.T) {
	fake := &glueJobFake{
		jobRuns: map[string]gluetypes.JobRunState{
			"running-job": gluetypes.JobRunStateRunning,
		},
	}
	clients := &awsclient.ServiceClients{Glue: fake}
	resources := []resource.Resource{{Name: "running-job"}}

	result, err := awsclient.EnrichGlueJobStatus(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result.Findings["running-job"]; ok {
		t.Error("RUNNING job must NOT appear in Findings")
	}
	if result.IssueCount != 0 {
		t.Errorf("IssueCount = %d, want 0", result.IssueCount)
	}
}

// TestEnrichGlueJobStatus_IssueCountEqualsFindings verifies IssueCount = len(Findings).
func TestEnrichGlueJobStatus_IssueCountEqualsFindings(t *testing.T) {
	fake := &glueJobFake{
		jobRuns: map[string]gluetypes.JobRunState{
			"fail-a": gluetypes.JobRunStateFailed,
			"fail-b": gluetypes.JobRunStateError,
			"ok-c":   gluetypes.JobRunStateSucceeded,
		},
	}
	clients := &awsclient.ServiceClients{Glue: fake}
	resources := []resource.Resource{
		{Name: "fail-a"},
		{Name: "fail-b"},
		{Name: "ok-c"},
	}

	result, err := awsclient.EnrichGlueJobStatus(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IssueCount != 2 {
		t.Errorf("IssueCount = %d, want 2", result.IssueCount)
	}
	if result.IssueCount != len(result.Findings) {
		t.Errorf("IssueCount (%d) != len(Findings) (%d)", result.IssueCount, len(result.Findings))
	}
}

// TestEnrichGlueJobStatus_TruncatedWhenResourcesExceedCap verifies Truncated=true.
func TestEnrichGlueJobStatus_TruncatedWhenResourcesExceedCap(t *testing.T) {
	count := awsclient.EnrichmentCap + 1
	resources := make([]resource.Resource, count)
	jobRuns := make(map[string]gluetypes.JobRunState, count)
	for i := range count {
		name := fmt.Sprintf("glue-job-%03d", i)
		resources[i] = resource.Resource{Name: name}
		jobRuns[name] = gluetypes.JobRunStateSucceeded
	}
	fake := &glueJobFake{jobRuns: jobRuns}
	clients := &awsclient.ServiceClients{Glue: fake}

	result, err := awsclient.EnrichGlueJobStatus(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Truncated {
		t.Errorf("Truncated must be true when len(resources)=%d > EnrichmentCap=%d",
			count, awsclient.EnrichmentCap)
	}
}

// TestEnrichGlueJobStatus_EmptyResourcesReturnsEmptyFindings verifies nil/empty
// resources returns non-nil empty Findings.
func TestEnrichGlueJobStatus_EmptyResourcesReturnsEmptyFindings(t *testing.T) {
	fake := &glueJobFake{jobRuns: map[string]gluetypes.JobRunState{}}
	clients := &awsclient.ServiceClients{Glue: fake}

	result, err := awsclient.EnrichGlueJobStatus(context.Background(), clients, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Findings == nil {
		t.Error("Findings must not be nil on empty resources")
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected empty Findings, got %d entries", len(result.Findings))
	}
}
