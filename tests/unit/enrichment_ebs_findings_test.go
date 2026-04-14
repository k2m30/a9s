package unit

// enrichment_ebs_findings_test.go — Behavioral tests for EnrichEBSVolumeStatus.
//
// Contract assertions (enricher-contract.md):
//   - Returns EnricherResult.Findings keyed by volume ID.
//   - Severity "!" for all findings.
//   - Summary: "volume I/O degraded".
//   - IssueCount = len(Findings).
//   - Volumes with status "ok" must NOT appear in Findings.
//   - Truncated = true when NextToken is non-nil.
//   - Empty result → non-nil empty Findings map.

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
)

// ebsStatusFake satisfies the EC2API subset needed by EnrichEBSVolumeStatus.
type ebsStatusFake struct {
	awsclient.EC2API
	volumeOutput *ec2.DescribeVolumeStatusOutput
	volumeErr    error
}

func (f *ebsStatusFake) DescribeVolumeStatus(
	_ context.Context,
	_ *ec2.DescribeVolumeStatusInput,
	_ ...func(*ec2.Options),
) (*ec2.DescribeVolumeStatusOutput, error) {
	if f.volumeErr != nil {
		return nil, f.volumeErr
	}
	if f.volumeOutput != nil {
		return f.volumeOutput, nil
	}
	return &ec2.DescribeVolumeStatusOutput{}, nil
}

// EnrichEC2StatusChecks also calls DescribeInstanceStatus via EC2API; we need to stub that
// to avoid panics when ebsStatusFake is used as EC2API. Add a no-op.
func (f *ebsStatusFake) DescribeInstanceStatus(
	_ context.Context,
	_ *ec2.DescribeInstanceStatusInput,
	_ ...func(*ec2.Options),
) (*ec2.DescribeInstanceStatusOutput, error) {
	return &ec2.DescribeInstanceStatusOutput{}, nil
}

// TestEnrichEBSVolumeStatus_FindingsKeyedByVolumeID verifies findings are keyed by volume ID.
func TestEnrichEBSVolumeStatus_FindingsKeyedByVolumeID(t *testing.T) {
	out := &ec2.DescribeVolumeStatusOutput{
		VolumeStatuses: []ec2types.VolumeStatusItem{
			{
				VolumeId: aws.String("vol-0abc123"),
				VolumeStatus: &ec2types.VolumeStatusInfo{
					Status: "impaired",
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{EC2: &ebsStatusFake{volumeOutput: out}}

	result, err := awsclient.EnrichEBSVolumeStatus(context.Background(), clients, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result.Findings["vol-0abc123"]; !ok {
		t.Errorf("expected finding keyed by volume ID %q", "vol-0abc123")
	}
}

// TestEnrichEBSVolumeStatus_SummaryVolumeIODegraded verifies the exact summary string.
func TestEnrichEBSVolumeStatus_SummaryVolumeIODegraded(t *testing.T) {
	out := &ec2.DescribeVolumeStatusOutput{
		VolumeStatuses: []ec2types.VolumeStatusItem{
			{
				VolumeId:     aws.String("vol-sum"),
				VolumeStatus: &ec2types.VolumeStatusInfo{Status: "impaired"},
			},
		},
	}
	clients := &awsclient.ServiceClients{EC2: &ebsStatusFake{volumeOutput: out}}

	result, err := awsclient.EnrichEBSVolumeStatus(context.Background(), clients, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f := result.Findings["vol-sum"]
	if f.Summary != "volume I/O degraded" {
		t.Errorf("summary = %q, want %q", f.Summary, "volume I/O degraded")
	}
}

// TestEnrichEBSVolumeStatus_OkVolumesExcluded verifies volumes with "ok" status
// do not appear in Findings.
func TestEnrichEBSVolumeStatus_OkVolumesExcluded(t *testing.T) {
	out := &ec2.DescribeVolumeStatusOutput{
		VolumeStatuses: []ec2types.VolumeStatusItem{
			{
				VolumeId:     aws.String("vol-ok"),
				VolumeStatus: &ec2types.VolumeStatusInfo{Status: "ok"},
			},
			{
				VolumeId:     aws.String("vol-bad"),
				VolumeStatus: &ec2types.VolumeStatusInfo{Status: "impaired"},
			},
		},
	}
	clients := &awsclient.ServiceClients{EC2: &ebsStatusFake{volumeOutput: out}}

	result, err := awsclient.EnrichEBSVolumeStatus(context.Background(), clients, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result.Findings["vol-ok"]; ok {
		t.Error("volume with status 'ok' must NOT appear in Findings")
	}
	if _, ok := result.Findings["vol-bad"]; !ok {
		t.Error("impaired volume must appear in Findings")
	}
}

// TestEnrichEBSVolumeStatus_IssueCountEqualsFindings verifies IssueCount = len(Findings).
func TestEnrichEBSVolumeStatus_IssueCountEqualsImpairedVolumeCount(t *testing.T) {
	out := &ec2.DescribeVolumeStatusOutput{
		VolumeStatuses: []ec2types.VolumeStatusItem{
			{VolumeId: aws.String("vol-a"), VolumeStatus: &ec2types.VolumeStatusInfo{Status: "impaired"}},
			{VolumeId: aws.String("vol-b"), VolumeStatus: &ec2types.VolumeStatusInfo{Status: "impaired"}},
			{VolumeId: aws.String("vol-c"), VolumeStatus: &ec2types.VolumeStatusInfo{Status: "ok"}},
		},
	}
	clients := &awsclient.ServiceClients{EC2: &ebsStatusFake{volumeOutput: out}}

	result, err := awsclient.EnrichEBSVolumeStatus(context.Background(), clients, nil)
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

// TestEnrichEBSVolumeStatus_TruncatedWhenNextTokenPresent verifies Truncated=true
// when the API response includes a NextToken.
func TestEnrichEBSVolumeStatus_TruncatedWhenNextTokenPresent(t *testing.T) {
	out := &ec2.DescribeVolumeStatusOutput{
		VolumeStatuses: []ec2types.VolumeStatusItem{
			{VolumeId: aws.String("vol-t"), VolumeStatus: &ec2types.VolumeStatusInfo{Status: "impaired"}},
		},
		NextToken: aws.String("more-pages"),
	}
	clients := &awsclient.ServiceClients{EC2: &ebsStatusFake{volumeOutput: out}}

	result, err := awsclient.EnrichEBSVolumeStatus(context.Background(), clients, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Truncated {
		t.Error("Truncated must be true when NextToken is non-nil")
	}
}

// TestEnrichEBSVolumeStatus_EmptyReturnsNonNilMap verifies empty result has non-nil Findings.
func TestEnrichEBSVolumeStatus_EmptyReturnsNonNilMap(t *testing.T) {
	out := &ec2.DescribeVolumeStatusOutput{}
	clients := &awsclient.ServiceClients{EC2: &ebsStatusFake{volumeOutput: out}}

	result, err := awsclient.EnrichEBSVolumeStatus(context.Background(), clients, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Findings == nil {
		t.Error("Findings must not be nil on empty result")
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected empty Findings, got %d entries", len(result.Findings))
	}
}
