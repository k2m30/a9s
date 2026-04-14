package unit

// enrichment_sfn_findings_test.go — Behavioral tests for EnrichStepFunctionsStatus.
//
// Contract assertions (enricher-contract.md):
//   - Returns EnricherResult.Findings keyed by state machine ARN (r.ID).
//   - Severity "!" for all findings.
//   - Summary: "latest execution FAILED" / "latest execution TIMED_OUT" / "latest execution ABORTED".
//   - IssueCount = len(Findings).
//   - Truncated = true when len(resources) > EnrichmentCap.
//   - State machines with SUCCEEDED/RUNNING latest execution must NOT appear in Findings.
//   - Empty resources → non-nil empty Findings map.

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	sfntypes "github.com/aws/aws-sdk-go-v2/service/sfn/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// sfnEnrichFake implements SFNAPI subset for enrichment testing.
type sfnEnrichFake struct {
	awsclient.SFNAPI
	// executions maps state machine ARN → latest execution status
	executions map[string]sfntypes.ExecutionStatus
	err        error
}

func (f *sfnEnrichFake) ListExecutions(
	_ context.Context,
	params *sfn.ListExecutionsInput,
	_ ...func(*sfn.Options),
) (*sfn.ListExecutionsOutput, error) {
	if f.err != nil {
		return nil, f.err
	}
	arn := aws.ToString(params.StateMachineArn)
	if status, ok := f.executions[arn]; ok {
		return &sfn.ListExecutionsOutput{
			Executions: []sfntypes.ExecutionListItem{
				{Status: status, StateMachineArn: &arn},
			},
		}, nil
	}
	return &sfn.ListExecutionsOutput{}, nil
}

// TestEnrichStepFunctionsStatus_FailedFindingKeyedByARN verifies findings are
// keyed by state machine ARN (r.ID).
func TestEnrichStepFunctionsStatus_FailedFindingKeyedByARN(t *testing.T) {
	smARN := "arn:aws:states:us-east-1:123456789012:stateMachine:my-sm"
	fake := &sfnEnrichFake{
		executions: map[string]sfntypes.ExecutionStatus{
			smARN: sfntypes.ExecutionStatusFailed,
		},
	}
	clients := &awsclient.ServiceClients{SFN: fake}
	resources := []resource.Resource{{ID: smARN}}

	result, err := awsclient.EnrichStepFunctionsStatus(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result.Findings[smARN]; !ok {
		t.Errorf("expected finding keyed by state machine ARN %q", smARN)
	}
}

// TestEnrichStepFunctionsStatus_SummaryContainsFAILED verifies the summary for FAILED status.
func TestEnrichStepFunctionsStatus_SummaryContainsFAILED(t *testing.T) {
	smARN := "arn:aws:states:us-east-1:123456789012:stateMachine:sum-sm"
	fake := &sfnEnrichFake{executions: map[string]sfntypes.ExecutionStatus{smARN: sfntypes.ExecutionStatusFailed}}
	clients := &awsclient.ServiceClients{SFN: fake}
	resources := []resource.Resource{{ID: smARN}}

	result, err := awsclient.EnrichStepFunctionsStatus(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	summary := result.Findings[smARN].Summary
	if !strings.Contains(summary, "FAILED") {
		t.Errorf("summary %q must contain %q", summary, "FAILED")
	}
}

// TestEnrichStepFunctionsStatus_SummaryTimedOut verifies the summary for TIMED_OUT.
func TestEnrichStepFunctionsStatus_SummaryTimedOut(t *testing.T) {
	smARN := "arn:aws:states:us-east-1:123456789012:stateMachine:to-sm"
	fake := &sfnEnrichFake{executions: map[string]sfntypes.ExecutionStatus{smARN: sfntypes.ExecutionStatusTimedOut}}
	clients := &awsclient.ServiceClients{SFN: fake}
	resources := []resource.Resource{{ID: smARN}}

	result, err := awsclient.EnrichStepFunctionsStatus(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	summary := result.Findings[smARN].Summary
	// Status token from SDK is "TIMED_OUT" — just verify it contains the key distinguisher
	if !strings.Contains(summary, "TIMED_OUT") {
		t.Errorf("summary %q must contain %q", summary, "TIMED_OUT")
	}
}

// TestEnrichStepFunctionsStatus_SummaryAborted verifies the summary for ABORTED.
func TestEnrichStepFunctionsStatus_SummaryAborted(t *testing.T) {
	smARN := "arn:aws:states:us-east-1:123456789012:stateMachine:ab-sm"
	fake := &sfnEnrichFake{executions: map[string]sfntypes.ExecutionStatus{smARN: sfntypes.ExecutionStatusAborted}}
	clients := &awsclient.ServiceClients{SFN: fake}
	resources := []resource.Resource{{ID: smARN}}

	result, err := awsclient.EnrichStepFunctionsStatus(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	summary := result.Findings[smARN].Summary
	if !strings.Contains(summary, "ABORTED") {
		t.Errorf("summary %q must contain %q", summary, "ABORTED")
	}
}

// TestEnrichStepFunctionsStatus_SucceededExcluded verifies SUCCEEDED state machines
// do not appear in Findings.
func TestEnrichStepFunctionsStatus_SucceededExcluded(t *testing.T) {
	smARN := "arn:aws:states:us-east-1:123456789012:stateMachine:ok-sm"
	fake := &sfnEnrichFake{executions: map[string]sfntypes.ExecutionStatus{smARN: sfntypes.ExecutionStatusSucceeded}}
	clients := &awsclient.ServiceClients{SFN: fake}
	resources := []resource.Resource{{ID: smARN}}

	result, err := awsclient.EnrichStepFunctionsStatus(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result.Findings[smARN]; ok {
		t.Error("SUCCEEDED state machine must NOT appear in Findings")
	}
	if result.IssueCount != 0 {
		t.Errorf("IssueCount = %d, want 0", result.IssueCount)
	}
}

// TestEnrichStepFunctionsStatus_TruncatedWhenResourcesExceedCap verifies Truncated.
func TestEnrichStepFunctionsStatus_TruncatedWhenResourcesExceedCap(t *testing.T) {
	count := awsclient.EnrichmentCap + 1
	resources := make([]resource.Resource, count)
	executions := make(map[string]sfntypes.ExecutionStatus, count)
	for i := range count {
		arn := fmt.Sprintf("arn:aws:states:us-east-1:123456789012:stateMachine:sm-%03d", i)
		resources[i] = resource.Resource{ID: arn}
		executions[arn] = sfntypes.ExecutionStatusSucceeded
	}
	fake := &sfnEnrichFake{executions: executions}
	clients := &awsclient.ServiceClients{SFN: fake}

	result, err := awsclient.EnrichStepFunctionsStatus(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Truncated {
		t.Errorf("Truncated must be true when len(resources)=%d > EnrichmentCap=%d",
			count, awsclient.EnrichmentCap)
	}
}

// TestEnrichStepFunctionsStatus_EmptyResourcesReturnsEmptyFindings verifies nil
// resources returns non-nil empty Findings.
func TestEnrichStepFunctionsStatus_EmptyResourcesReturnsEmptyFindings(t *testing.T) {
	fake := &sfnEnrichFake{executions: map[string]sfntypes.ExecutionStatus{}}
	clients := &awsclient.ServiceClients{SFN: fake}

	result, err := awsclient.EnrichStepFunctionsStatus(context.Background(), clients, nil)
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
