package unit

// aws_vpc_enricher_test.go — Behavioral tests for EnrichVPCFlowLogs.
//
// Contract assertions:
//   - DescribeFlowLogs is called once per VPC resource (filtered by resource-id).
//   - All VPCs have at least one FlowLog with FlowLogStatus=ACTIVE → 0 findings.
//   - VPC has FlowLogs[] empty → finding for that VPC, severity "~".
//   - VPC has FlowLogs only with non-ACTIVE status (e.g. "PENDING") → finding for that VPC, severity "~".
//   - clients.EC2 == nil → (EnricherResult{Findings: non-nil empty}, nil).
//   - API error for VPC-1, ok for VPC-2 → 0 findings for VPC-1, findings for VPC-2 per its state, Truncated=true.

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2svc "github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// vpcFlowLogFake implements EC2API for VPC flow-log enrichment testing.
// It embeds the interface and overrides only DescribeFlowLogs.
// The results map is keyed by the "resource-id" filter value so the fake can
// serve different responses per VPC resource.
type vpcFlowLogFake struct {
	awsclient.EC2API
	// results maps VPC ID → flow log list. Used when errByVPC has no entry.
	results map[string][]ec2types.FlowLog
	// errByVPC maps VPC ID → error; overrides results when set.
	errByVPC map[string]error
}

func (f *vpcFlowLogFake) DescribeFlowLogs(
	_ context.Context,
	in *ec2svc.DescribeFlowLogsInput,
	_ ...func(*ec2svc.Options),
) (*ec2svc.DescribeFlowLogsOutput, error) {
	vpcID := ""
	if in != nil {
		for _, fl := range in.Filter {
			if fl.Name != nil && *fl.Name == "resource-id" && len(fl.Values) > 0 {
				vpcID = fl.Values[0]
				break
			}
		}
	}
	if f.errByVPC != nil {
		if err, ok := f.errByVPC[vpcID]; ok {
			return nil, err
		}
	}
	logs := f.results[vpcID]
	return &ec2svc.DescribeFlowLogsOutput{FlowLogs: logs}, nil
}

// Compile-time check: vpcFlowLogFake satisfies EC2API.
var _ awsclient.EC2API = (*vpcFlowLogFake)(nil)

// vpcResources returns a slice of VPC Resource stubs with the given IDs.
func vpcResources(ids ...string) []resource.Resource {
	res := make([]resource.Resource, 0, len(ids))
	for _, id := range ids {
		res = append(res, resource.Resource{
			ID:   id,
			Name: "vpc-" + id,
			Fields: map[string]string{
				"vpc_id": id,
				"state":  "available",
			},
		})
	}
	return res
}

// activeFlowLog returns a FlowLog stub with FlowLogStatus=ACTIVE for the given VPC.
func activeFlowLog(vpcID, flowLogID string) ec2types.FlowLog {
	return ec2types.FlowLog{
		FlowLogId:     aws.String(flowLogID),
		FlowLogStatus: aws.String("ACTIVE"),
		ResourceId:    aws.String(vpcID),
	}
}

// pendingFlowLog returns a FlowLog stub with FlowLogStatus=PENDING for the given VPC.
func pendingFlowLog(vpcID, flowLogID string) ec2types.FlowLog {
	return ec2types.FlowLog{
		FlowLogId:     aws.String(flowLogID),
		FlowLogStatus: aws.String("PENDING"),
		ResourceId:    aws.String(vpcID),
	}
}

// TestEnrichVPCFlowLogs_BothActiveProducesNoFindings verifies that when both VPCs
// have an ACTIVE flow log, no findings are produced.
func TestEnrichVPCFlowLogs_BothActiveProducesNoFindings(t *testing.T) {
	fake := &vpcFlowLogFake{
		results: map[string][]ec2types.FlowLog{
			"vpc-00000001": {activeFlowLog("vpc-00000001", "fl-a001")},
			"vpc-00000002": {activeFlowLog("vpc-00000002", "fl-a002")},
		},
	}
	clients := &awsclient.ServiceClients{EC2: fake}
	resources := vpcResources("vpc-00000001", "vpc-00000002")

	result, err := awsclient.EnrichVPCFlowLogs(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Findings == nil {
		t.Fatal("Findings must not be nil")
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings, got %d: %v", len(result.Findings), result.Findings)
	}
}

// TestEnrichVPCFlowLogs_NoLogsProducesFindingSevTilde verifies that when both VPCs
// have empty FlowLogs[], a finding with severity "~" is produced for each.
func TestEnrichVPCFlowLogs_NoLogsProducesFindingSevTilde(t *testing.T) {
	fake := &vpcFlowLogFake{
		results: map[string][]ec2types.FlowLog{
			"vpc-00000001": {},
			"vpc-00000002": {},
		},
	}
	clients := &awsclient.ServiceClients{EC2: fake}
	resources := vpcResources("vpc-00000001", "vpc-00000002")

	result, err := awsclient.EnrichVPCFlowLogs(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Findings) != 2 {
		t.Errorf("expected 2 findings, got %d", len(result.Findings))
	}
	for _, id := range []string{"vpc-00000001", "vpc-00000002"} {
		f, ok := result.Findings[id]
		if !ok {
			t.Errorf("expected finding for %q", id)
			continue
		}
		if f.Severity != domain.SevWarn {
			t.Errorf("%s: severity = %v, want SevWarn", id, f.Severity)
		}
	}
}

// TestEnrichVPCFlowLogs_InactiveOnlyProducesFindingForAffectedVPC verifies that when
// VPC-1 has only a PENDING (non-ACTIVE) flow log, a finding with severity "~" is
// produced for VPC-1. VPC-2 with an ACTIVE flow log produces no finding.
func TestEnrichVPCFlowLogs_InactiveOnlyProducesFindingForAffectedVPC(t *testing.T) {
	fake := &vpcFlowLogFake{
		results: map[string][]ec2types.FlowLog{
			"vpc-00000001": {pendingFlowLog("vpc-00000001", "fl-b001")},
			"vpc-00000002": {activeFlowLog("vpc-00000002", "fl-b002")},
		},
	}
	clients := &awsclient.ServiceClients{EC2: fake}
	resources := vpcResources("vpc-00000001", "vpc-00000002")

	result, err := awsclient.EnrichVPCFlowLogs(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f, ok := result.Findings["vpc-00000001"]
	if !ok {
		t.Fatalf("expected finding keyed by %q", "vpc-00000001")
	}
	if f.Severity != domain.SevWarn {
		t.Errorf("severity = %v, want %v", f.Severity, "~")
	}
	if _, ok := result.Findings["vpc-00000002"]; ok {
		t.Error("vpc-00000002 must NOT appear in Findings — it has an ACTIVE flow log")
	}
	if result.IssueCount != 0 {
		t.Errorf("IssueCount = %d, want 0 (~ findings do not contribute to badge)", result.IssueCount)
	}
	if len(result.Findings) != 1 {
		t.Errorf("len(Findings) = %d, want 1 (finding is produced even when IssueCount is 0)", len(result.Findings))
	}
}

// TestEnrichVPCFlowLogs_NilClientReturnsEmptyFindingsNoError verifies that when
// clients.EC2 is nil the enricher returns a non-nil empty Findings map and no error.
func TestEnrichVPCFlowLogs_NilClientReturnsEmptyFindingsNoError(t *testing.T) {
	clients := &awsclient.ServiceClients{EC2: nil}

	result, err := awsclient.EnrichVPCFlowLogs(context.Background(), clients, vpcResources("vpc-00000001", "vpc-00000002"), nil)
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

// TestEnrichVPCFlowLogs_APIErrorSetsTruncatedFindsOtherVPC verifies that when the
// API call for VPC-1 returns an error, the enricher sets Truncated=true and still
// produces a finding for VPC-2 (which has no active flow log).
func TestEnrichVPCFlowLogs_APIErrorSetsTruncatedFindsOtherVPC(t *testing.T) {
	apiErr := errors.New("ec2: DescribeFlowLogs throttled")
	fake := &vpcFlowLogFake{
		errByVPC: map[string]error{
			"vpc-00000001": apiErr,
		},
		results: map[string][]ec2types.FlowLog{
			"vpc-00000002": {},
		},
	}
	clients := &awsclient.ServiceClients{EC2: fake}
	resources := vpcResources("vpc-00000001", "vpc-00000002")

	result, err := awsclient.EnrichVPCFlowLogs(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Truncated {
		t.Error("Truncated must be true when an API call fails")
	}
	if _, ok := result.Findings["vpc-00000001"]; ok {
		t.Error("vpc-00000001 must NOT appear in Findings on API error")
	}
	f, ok := result.Findings["vpc-00000002"]
	if !ok {
		t.Fatalf("expected finding for vpc-00000002 (no active flow logs)")
	}
	if f.Severity != domain.SevWarn {
		t.Errorf("vpc-00000002 severity = %v, want %v", f.Severity, "~")
	}
}
