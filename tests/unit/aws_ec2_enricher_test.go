package unit

// aws_ec2_enricher_test.go — Behavioral tests for EnrichEC2InstanceStatus.
//
// Contract assertions:
//   - DescribeInstanceStatus is called once (all instances, no resource iteration).
//   - InstanceStatus.Status="impaired" → 1 finding keyed by InstanceId, severity "!".
//   - SystemStatus.Status="impaired" → 1 finding keyed by InstanceId, severity "!".
//   - ScheduledEvent within next 5 days → 1 finding keyed by InstanceId, severity "~".
//   - All status checks ok, no events → 0 findings.
//   - clients.EC2 == nil → (EnricherResult{Findings: non-nil empty}, nil).
//   - API error → (EnricherResult{}, error propagated).

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ec2InstanceStatusFake implements EC2API for enrichment testing.
// It embeds the interface and overrides only DescribeInstanceStatus.
type ec2InstanceStatusFake struct {
	awsclient.EC2API
	statuses []ec2types.InstanceStatus
	err      error
}

func (f *ec2InstanceStatusFake) DescribeInstanceStatus(
	_ context.Context,
	_ *ec2.DescribeInstanceStatusInput,
	_ ...func(*ec2.Options),
) (*ec2.DescribeInstanceStatusOutput, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &ec2.DescribeInstanceStatusOutput{InstanceStatuses: f.statuses}, nil
}

// Compile-time check: ec2InstanceStatusFake satisfies EC2API.
var _ awsclient.EC2API = (*ec2InstanceStatusFake)(nil)

// daysFromNow returns a *time.Time n days in the future.
func daysFromNow(n int) *time.Time {
	t := time.Now().Add(time.Duration(n) * 24 * time.Hour)
	return &t
}

// TestEnrichEC2InstanceStatus_InstanceStatusImpairedProducesFindingSevBang verifies
// that an instance with InstanceStatus.Status="impaired" produces a finding with
// severity "!" keyed by the instance ID.
func TestEnrichEC2InstanceStatus_InstanceStatusImpairedProducesFindingSevBang(t *testing.T) {
	fake := &ec2InstanceStatusFake{
		statuses: []ec2types.InstanceStatus{
			{
				InstanceId: aws.String("i-0aaaa1111bbbbb222"),
				SystemStatus: &ec2types.InstanceStatusSummary{
					Status: ec2types.SummaryStatusOk,
				},
				InstanceStatus: &ec2types.InstanceStatusSummary{
					Status: ec2types.SummaryStatusImpaired,
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{EC2: fake}
	resources := []resource.Resource{{ID: "i-0aaaa1111bbbbb222", Fields: map[string]string{"state": "running"}}}

	result, err := awsclient.EnrichEC2InstanceStatus(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f, ok := result.Findings["i-0aaaa1111bbbbb222"]
	if !ok {
		t.Fatalf("expected finding keyed by instance ID %q", "i-0aaaa1111bbbbb222")
	}
	if f.Severity != "!" {
		t.Errorf("severity = %q, want %q", f.Severity, "!")
	}
	if result.IssueCount != 1 {
		t.Errorf("IssueCount = %d, want 1", result.IssueCount)
	}
}

// TestEnrichEC2InstanceStatus_SystemStatusImpairedProducesFindingSevBang verifies
// that an instance with SystemStatus.Status="impaired" produces a finding with
// severity "!" keyed by the instance ID.
func TestEnrichEC2InstanceStatus_SystemStatusImpairedProducesFindingSevBang(t *testing.T) {
	fake := &ec2InstanceStatusFake{
		statuses: []ec2types.InstanceStatus{
			{
				InstanceId: aws.String("i-0bbbb2222ccccc333"),
				SystemStatus: &ec2types.InstanceStatusSummary{
					Status: ec2types.SummaryStatusImpaired,
				},
				InstanceStatus: &ec2types.InstanceStatusSummary{
					Status: ec2types.SummaryStatusOk,
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{EC2: fake}
	resources := []resource.Resource{{ID: "i-0bbbb2222ccccc333", Fields: map[string]string{"state": "running"}}}

	result, err := awsclient.EnrichEC2InstanceStatus(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f, ok := result.Findings["i-0bbbb2222ccccc333"]
	if !ok {
		t.Fatalf("expected finding keyed by instance ID %q for impaired system status", "i-0bbbb2222ccccc333")
	}
	if f.Severity != "!" {
		t.Errorf("severity = %q, want %q", f.Severity, "!")
	}
}

// TestEnrichEC2InstanceStatus_ScheduledEventSoonProducesFindingSevTilde verifies
// that a running instance with ok status checks but a scheduled event within the
// next 5 days produces a finding with severity "~".
func TestEnrichEC2InstanceStatus_ScheduledEventSoonProducesFindingSevTilde(t *testing.T) {
	fake := &ec2InstanceStatusFake{
		statuses: []ec2types.InstanceStatus{
			{
				InstanceId: aws.String("i-0cccc3333ddddd444"),
				SystemStatus: &ec2types.InstanceStatusSummary{
					Status: ec2types.SummaryStatusOk,
				},
				InstanceStatus: &ec2types.InstanceStatusSummary{
					Status: ec2types.SummaryStatusOk,
				},
				Events: []ec2types.InstanceStatusEvent{
					{
						Code:            ec2types.EventCodeSystemReboot,
						Description:     aws.String("Scheduled reboot"),
						NotBefore:       daysFromNow(3),
					},
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{EC2: fake}
	resources := []resource.Resource{{ID: "i-0cccc3333ddddd444", Fields: map[string]string{"state": "running"}}}

	result, err := awsclient.EnrichEC2InstanceStatus(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f, ok := result.Findings["i-0cccc3333ddddd444"]
	if !ok {
		t.Fatalf("expected finding keyed by instance ID %q for imminent scheduled event", "i-0cccc3333ddddd444")
	}
	if f.Severity != "~" {
		t.Errorf("severity = %q, want %q", f.Severity, "~")
	}
}

// TestEnrichEC2InstanceStatus_HealthyInstanceProducesNoFinding verifies that a
// running instance with all status checks ok and no events produces zero findings.
func TestEnrichEC2InstanceStatus_HealthyInstanceProducesNoFinding(t *testing.T) {
	fake := &ec2InstanceStatusFake{
		statuses: []ec2types.InstanceStatus{
			{
				InstanceId: aws.String("i-0dddd4444eeeee555"),
				SystemStatus: &ec2types.InstanceStatusSummary{
					Status: ec2types.SummaryStatusOk,
				},
				InstanceStatus: &ec2types.InstanceStatusSummary{
					Status: ec2types.SummaryStatusOk,
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{EC2: fake}
	resources := []resource.Resource{{ID: "i-0dddd4444eeeee555", Fields: map[string]string{"state": "running"}}}

	result, err := awsclient.EnrichEC2InstanceStatus(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result.Findings["i-0dddd4444eeeee555"]; ok {
		t.Error("healthy instance must NOT appear in Findings")
	}
	if result.IssueCount != 0 {
		t.Errorf("IssueCount = %d, want 0", result.IssueCount)
	}
}

// TestEnrichEC2InstanceStatus_NilClientReturnsEmptyFindingsNoError verifies that
// when clients.EC2 is nil, the enricher returns a non-nil empty Findings map and
// no error.
func TestEnrichEC2InstanceStatus_NilClientReturnsEmptyFindingsNoError(t *testing.T) {
	clients := &awsclient.ServiceClients{EC2: nil}

	result, err := awsclient.EnrichEC2InstanceStatus(context.Background(), clients, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Findings == nil {
		t.Error("Findings must not be nil when EC2 client is nil")
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected empty Findings, got %d entries", len(result.Findings))
	}
}

// TestEnrichEC2InstanceStatus_APIErrorIsPropagated verifies that an API error from
// DescribeInstanceStatus is propagated as the enricher's return error.
func TestEnrichEC2InstanceStatus_APIErrorIsPropagated(t *testing.T) {
	apiErr := errors.New("ec2: describe instance status failed")
	fake := &ec2InstanceStatusFake{err: apiErr}
	clients := &awsclient.ServiceClients{EC2: fake}

	_, err := awsclient.EnrichEC2InstanceStatus(context.Background(), clients, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, apiErr) {
		t.Errorf("error = %v, want to wrap %v", err, apiErr)
	}
}
