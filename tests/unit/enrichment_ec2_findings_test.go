package unit

// enrichment_ec2_findings_test.go — Behavioral tests for EnrichEC2StatusChecks.
//
// Contract assertions (enricher-contract.md):
//   - Returns EnricherResult.Findings keyed by instance ID (not ARN).
//   - Severity "!" for all findings.
//   - Summary: "system status impaired" when system status is impaired.
//   - Summary: "instance status impaired" when only instance status is impaired.
//   - IssueCount = len(Findings).
//   - Truncated = true when NextToken is non-nil.
//   - Empty result → non-nil empty Findings map.

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ec2StatusFake satisfies the EC2API subset needed by EnrichEC2StatusChecks.
// It embeds EC2API to panic on any method not under test.
type ec2StatusFake struct {
	awsclient.EC2API
	statusOutput *ec2.DescribeInstanceStatusOutput
	statusErr    error
}

func (f *ec2StatusFake) DescribeInstanceStatus(
	_ context.Context,
	_ *ec2.DescribeInstanceStatusInput,
	_ ...func(*ec2.Options),
) (*ec2.DescribeInstanceStatusOutput, error) {
	if f.statusErr != nil {
		return nil, f.statusErr
	}
	if f.statusOutput != nil {
		return f.statusOutput, nil
	}
	return &ec2.DescribeInstanceStatusOutput{}, nil
}

// TestEnrichEC2StatusChecks_SystemImpaired verifies system-impaired findings
// are keyed by instance ID with summary "system status impaired".
func TestEnrichEC2StatusChecks_SystemImpaired_FindingsKeyedByInstanceID(t *testing.T) {
	out := &ec2.DescribeInstanceStatusOutput{
		InstanceStatuses: []ec2types.InstanceStatus{
			{
				InstanceId: aws.String("i-0abc123"),
				SystemStatus: &ec2types.InstanceStatusSummary{
					Status: ec2types.SummaryStatusImpaired,
				},
				InstanceStatus: &ec2types.InstanceStatusSummary{
					Status: ec2types.SummaryStatusOk,
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{EC2: &ec2StatusFake{statusOutput: out}}

	result, err := awsclient.EnrichEC2StatusChecks(context.Background(), clients, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f, ok := result.Findings["i-0abc123"]
	if !ok {
		t.Fatalf("expected finding keyed by instance ID %q", "i-0abc123")
	}
	if f.Severity != "!" {
		t.Errorf("severity = %q, want %q", f.Severity, "!")
	}
	if f.Summary != "system status impaired" {
		t.Errorf("summary = %q, want %q", f.Summary, "system status impaired")
	}
}

// TestEnrichEC2StatusChecks_InstanceImpaired verifies instance-only-impaired findings
// carry summary "instance status impaired".
func TestEnrichEC2StatusChecks_InstanceImpaired_SummaryDistinct(t *testing.T) {
	out := &ec2.DescribeInstanceStatusOutput{
		InstanceStatuses: []ec2types.InstanceStatus{
			{
				InstanceId: aws.String("i-0def456"),
				SystemStatus: &ec2types.InstanceStatusSummary{
					Status: ec2types.SummaryStatusOk,
				},
				InstanceStatus: &ec2types.InstanceStatusSummary{
					Status: ec2types.SummaryStatusImpaired,
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{EC2: &ec2StatusFake{statusOutput: out}}

	result, err := awsclient.EnrichEC2StatusChecks(context.Background(), clients, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f, ok := result.Findings["i-0def456"]
	if !ok {
		t.Fatal("expected finding for i-0def456")
	}
	if f.Summary != "instance status impaired" {
		t.Errorf("summary = %q, want %q", f.Summary, "instance status impaired")
	}
}

// TestEnrichEC2StatusChecks_BothImpairedPrefersSystem verifies that when both
// system and instance status are impaired, the summary is "system status impaired".
func TestEnrichEC2StatusChecks_BothImpairedPrefersSystemSummary(t *testing.T) {
	out := &ec2.DescribeInstanceStatusOutput{
		InstanceStatuses: []ec2types.InstanceStatus{
			{
				InstanceId: aws.String("i-0ghi789"),
				SystemStatus: &ec2types.InstanceStatusSummary{
					Status: ec2types.SummaryStatusImpaired,
				},
				InstanceStatus: &ec2types.InstanceStatusSummary{
					Status: ec2types.SummaryStatusImpaired,
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{EC2: &ec2StatusFake{statusOutput: out}}

	result, err := awsclient.EnrichEC2StatusChecks(context.Background(), clients, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Findings["i-0ghi789"].Summary != "system status impaired" {
		t.Errorf("both impaired: summary = %q, want %q", result.Findings["i-0ghi789"].Summary, "system status impaired")
	}
}

// TestEnrichEC2StatusChecks_IssueCountEqualsFindings verifies IssueCount = len(Findings).
func TestEnrichEC2StatusChecks_IssueCountEqualsNumberOfImpairedInstances(t *testing.T) {
	out := &ec2.DescribeInstanceStatusOutput{
		InstanceStatuses: []ec2types.InstanceStatus{
			{
				InstanceId:   aws.String("i-aaa"),
				SystemStatus: &ec2types.InstanceStatusSummary{Status: ec2types.SummaryStatusImpaired},
				InstanceStatus: &ec2types.InstanceStatusSummary{
					Status: ec2types.SummaryStatusOk,
				},
			},
			{
				InstanceId:   aws.String("i-bbb"),
				SystemStatus: &ec2types.InstanceStatusSummary{Status: ec2types.SummaryStatusOk},
				InstanceStatus: &ec2types.InstanceStatusSummary{
					Status: ec2types.SummaryStatusImpaired,
				},
			},
			{
				// healthy — should NOT appear in findings
				InstanceId:   aws.String("i-ccc"),
				SystemStatus: &ec2types.InstanceStatusSummary{Status: ec2types.SummaryStatusOk},
				InstanceStatus: &ec2types.InstanceStatusSummary{
					Status: ec2types.SummaryStatusOk,
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{EC2: &ec2StatusFake{statusOutput: out}}

	result, err := awsclient.EnrichEC2StatusChecks(context.Background(), clients, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IssueCount != 2 {
		t.Errorf("IssueCount = %d, want 2", result.IssueCount)
	}
	if len(result.Findings) != 2 {
		t.Errorf("len(Findings) = %d, want 2", len(result.Findings))
	}
	if result.IssueCount != len(result.Findings) {
		t.Errorf("IssueCount (%d) != len(Findings) (%d)", result.IssueCount, len(result.Findings))
	}
	if _, ok := result.Findings["i-ccc"]; ok {
		t.Error("healthy instance i-ccc must not appear in Findings")
	}
}

// TestEnrichEC2StatusChecks_TruncatedWhenNextTokenPresent verifies Truncated=true
// when the API response includes a NextToken.
func TestEnrichEC2StatusChecks_TruncatedWhenNextTokenPresent(t *testing.T) {
	out := &ec2.DescribeInstanceStatusOutput{
		InstanceStatuses: []ec2types.InstanceStatus{
			{
				InstanceId:   aws.String("i-truncated"),
				SystemStatus: &ec2types.InstanceStatusSummary{Status: ec2types.SummaryStatusImpaired},
				InstanceStatus: &ec2types.InstanceStatusSummary{
					Status: ec2types.SummaryStatusOk,
				},
			},
		},
		NextToken: aws.String("more-results"),
	}
	clients := &awsclient.ServiceClients{EC2: &ec2StatusFake{statusOutput: out}}

	result, err := awsclient.EnrichEC2StatusChecks(context.Background(), clients, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Truncated {
		t.Error("Truncated must be true when NextToken is non-nil")
	}
}

// TestEnrichEC2StatusChecks_EmptyReturnsNonNilMap verifies empty result has non-nil Findings.
func TestEnrichEC2StatusChecks_EmptyReturnsNonNilMap(t *testing.T) {
	out := &ec2.DescribeInstanceStatusOutput{}
	clients := &awsclient.ServiceClients{EC2: &ec2StatusFake{statusOutput: out}}

	result, err := awsclient.EnrichEC2StatusChecks(context.Background(), clients, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Findings == nil {
		t.Error("Findings must not be nil on empty result")
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected empty Findings, got %d entries", len(result.Findings))
	}
	if result.IssueCount != 0 {
		t.Errorf("IssueCount = %d, want 0", result.IssueCount)
	}
}

// TestEnrichEC2StatusChecks_ResourcesIgnored verifies the enricher ignores the
// resources slice (account-wide API — no per-resource filtering).
func TestEnrichEC2StatusChecks_ResourcesIgnored(t *testing.T) {
	out := &ec2.DescribeInstanceStatusOutput{
		InstanceStatuses: []ec2types.InstanceStatus{
			{
				InstanceId:   aws.String("i-account-wide"),
				SystemStatus: &ec2types.InstanceStatusSummary{Status: ec2types.SummaryStatusImpaired},
				InstanceStatus: &ec2types.InstanceStatusSummary{
					Status: ec2types.SummaryStatusOk,
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{EC2: &ec2StatusFake{statusOutput: out}}
	// Pass a different resource slice — should still find i-account-wide
	resources := []resource.Resource{{ID: "i-something-else"}}

	result, err := awsclient.EnrichEC2StatusChecks(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result.Findings["i-account-wide"]; !ok {
		t.Error("account-wide enricher must include findings for IDs not in input slice")
	}
}
