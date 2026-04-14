package unit

// enrichment_pipeline_findings_test.go — Behavioral tests for EnrichCodePipelineStatus.
//
// Contract assertions (enricher-contract.md):
//   - Returns EnricherResult.Findings keyed by pipeline name (r.Name).
//   - Severity "!" for all findings.
//   - Summary format: "stage <Name> failed".
//   - IssueCount = len(Findings).
//   - Truncated = true when len(resources) > EnrichmentCap.
//   - Pipelines with no failed stages must NOT appear in Findings.
//   - Empty resources slice → non-nil empty Findings map.

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/codepipeline"
	cptypes "github.com/aws/aws-sdk-go-v2/service/codepipeline/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// pipelineStateFake implements CodePipelineAPI subset for enrichment testing.
type pipelineStateFake struct {
	awsclient.CodePipelineAPI
	// states maps pipeline name → GetPipelineStateOutput
	states map[string]*codepipeline.GetPipelineStateOutput
	err    error
}

func (f *pipelineStateFake) GetPipelineState(
	_ context.Context,
	params *codepipeline.GetPipelineStateInput,
	_ ...func(*codepipeline.Options),
) (*codepipeline.GetPipelineStateOutput, error) {
	if f.err != nil {
		return nil, f.err
	}
	name := aws.ToString(params.Name)
	if out, ok := f.states[name]; ok {
		return out, nil
	}
	return &codepipeline.GetPipelineStateOutput{}, nil
}

func stageState(stageName string, status cptypes.StageExecutionStatus) cptypes.StageState {
	return cptypes.StageState{
		StageName: aws.String(stageName),
		LatestExecution: &cptypes.StageExecution{
			Status: status,
		},
	}
}

// TestEnrichCodePipelineStatus_FailedStageKeyedByPipelineName verifies findings
// are keyed by pipeline name (r.Name).
func TestEnrichCodePipelineStatus_FailedStageKeyedByPipelineName(t *testing.T) {
	fake := &pipelineStateFake{
		states: map[string]*codepipeline.GetPipelineStateOutput{
			"my-pipeline": {
				StageStates: []cptypes.StageState{
					stageState("Deploy", "Failed"),
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{CodePipeline: fake}
	resources := []resource.Resource{{ID: "pipe-id", Name: "my-pipeline"}}

	result, err := awsclient.EnrichCodePipelineStatus(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result.Findings["my-pipeline"]; !ok {
		t.Errorf("expected finding keyed by pipeline name %q", "my-pipeline")
	}
}

// TestEnrichCodePipelineStatus_SeverityBang verifies severity "!".
func TestEnrichCodePipelineStatus_SeverityBang(t *testing.T) {
	fake := &pipelineStateFake{
		states: map[string]*codepipeline.GetPipelineStateOutput{
			"pipeline-sev": {StageStates: []cptypes.StageState{stageState("Build", "Failed")}},
		},
	}
	clients := &awsclient.ServiceClients{CodePipeline: fake}
	resources := []resource.Resource{{Name: "pipeline-sev"}}

	result, err := awsclient.EnrichCodePipelineStatus(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f := result.Findings["pipeline-sev"]
	if f.Severity != "!" {
		t.Errorf("severity = %q, want %q", f.Severity, "!")
	}
}

// TestEnrichCodePipelineStatus_SummaryContainsStageName verifies "stage <Name> failed" format.
func TestEnrichCodePipelineStatus_SummaryContainsStageName(t *testing.T) {
	fake := &pipelineStateFake{
		states: map[string]*codepipeline.GetPipelineStateOutput{
			"summary-pipeline": {
				StageStates: []cptypes.StageState{
					stageState("Integration-Test", "Failed"),
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{CodePipeline: fake}
	resources := []resource.Resource{{Name: "summary-pipeline"}}

	result, err := awsclient.EnrichCodePipelineStatus(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	summary := result.Findings["summary-pipeline"].Summary
	if !strings.Contains(summary, "Integration-Test") {
		t.Errorf("summary %q must contain stage name %q", summary, "Integration-Test")
	}
	if !strings.Contains(summary, "failed") {
		t.Errorf("summary %q must contain %q", summary, "failed")
	}
}

// TestEnrichCodePipelineStatus_NoFailedStageExcluded verifies pipelines with
// no failed stages do not appear in Findings.
func TestEnrichCodePipelineStatus_NoFailedStageExcluded(t *testing.T) {
	fake := &pipelineStateFake{
		states: map[string]*codepipeline.GetPipelineStateOutput{
			"ok-pipeline": {
				StageStates: []cptypes.StageState{
					stageState("Source", cptypes.StageExecutionStatusSucceeded),
					stageState("Build", cptypes.StageExecutionStatusSucceeded),
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{CodePipeline: fake}
	resources := []resource.Resource{{Name: "ok-pipeline"}}

	result, err := awsclient.EnrichCodePipelineStatus(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result.Findings["ok-pipeline"]; ok {
		t.Error("pipeline with no failed stages must NOT appear in Findings")
	}
	if result.IssueCount != 0 {
		t.Errorf("IssueCount = %d, want 0", result.IssueCount)
	}
}

// TestEnrichCodePipelineStatus_TruncatedWhenResourcesExceedCap verifies Truncated.
func TestEnrichCodePipelineStatus_TruncatedWhenResourcesExceedCap(t *testing.T) {
	count := awsclient.EnrichmentCap + 1
	resources := make([]resource.Resource, count)
	states := make(map[string]*codepipeline.GetPipelineStateOutput, count)
	for i := range count {
		name := fmt.Sprintf("pipeline-%03d", i)
		resources[i] = resource.Resource{Name: name}
		states[name] = &codepipeline.GetPipelineStateOutput{
			StageStates: []cptypes.StageState{stageState("Deploy", cptypes.StageExecutionStatusSucceeded)},
		}
	}
	fake := &pipelineStateFake{states: states}
	clients := &awsclient.ServiceClients{CodePipeline: fake}

	result, err := awsclient.EnrichCodePipelineStatus(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Truncated {
		t.Errorf("Truncated must be true when len(resources)=%d > EnrichmentCap=%d",
			count, awsclient.EnrichmentCap)
	}
}

// TestEnrichCodePipelineStatus_EmptyResourcesReturnsEmptyFindings verifies empty
// resources returns non-nil empty Findings.
func TestEnrichCodePipelineStatus_EmptyResourcesReturnsEmptyFindings(t *testing.T) {
	fake := &pipelineStateFake{states: map[string]*codepipeline.GetPipelineStateOutput{}}
	clients := &awsclient.ServiceClients{CodePipeline: fake}

	result, err := awsclient.EnrichCodePipelineStatus(context.Background(), clients, nil)
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
