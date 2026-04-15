package unit

// aws_tgw_enricher_test.go — Behavioral tests for EnrichTGWAttachments.
//
// Contract assertions:
//   - DescribeTransitGatewayAttachments is called once per TGW resource (filtered by TGW ID).
//   - All attachments State=available → 0 findings.
//   - Any attachment State=failed → finding for that TGW, severity "!".
//   - Any attachment State=modifying → finding for that TGW, severity "~".
//   - clients.EC2 == nil → (EnricherResult{Findings: non-nil empty}, nil).
//   - API error for a resource → 0 findings for that resource, Truncated=true, no error returned.

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// tgwAttachmentFake implements EC2API for TGW attachment enrichment testing.
// It embeds the interface and overrides only DescribeTransitGatewayAttachments.
// The results map is keyed by transit-gateway-id filter value so the fake can
// serve different responses per TGW resource.
type tgwAttachmentFake struct {
	awsclient.EC2API
	// results maps TGW ID → attachment list. Used when errByTGW has no entry.
	results map[string][]ec2types.TransitGatewayAttachment
	// errByTGW maps TGW ID → error; overrides results when set.
	errByTGW map[string]error
}

func (f *tgwAttachmentFake) DescribeTransitGatewayAttachments(
	_ context.Context,
	in *ec2.DescribeTransitGatewayAttachmentsInput,
	_ ...func(*ec2.Options),
) (*ec2.DescribeTransitGatewayAttachmentsOutput, error) {
	// Extract the transit-gateway-id filter value to look up the per-TGW response.
	tgwID := ""
	if in != nil {
		for _, f := range in.Filters {
			if f.Name != nil && *f.Name == "transit-gateway-id" && len(f.Values) > 0 {
				tgwID = f.Values[0]
				break
			}
		}
	}
	if f.errByTGW != nil {
		if err, ok := f.errByTGW[tgwID]; ok {
			return nil, err
		}
	}
	attachments := f.results[tgwID]
	return &ec2.DescribeTransitGatewayAttachmentsOutput{TransitGatewayAttachments: attachments}, nil
}

// Compile-time check: tgwAttachmentFake satisfies EC2API.
var _ awsclient.EC2API = (*tgwAttachmentFake)(nil)

// tgwResources returns a slice of TGW Resource stubs with the given IDs.
func tgwResources(ids ...string) []resource.Resource {
	res := make([]resource.Resource, 0, len(ids))
	for _, id := range ids {
		res = append(res, resource.Resource{
			ID:   id,
			Name: "tgw-" + id,
			Fields: map[string]string{
				"tgw_id": id,
				"state":  "available",
			},
		})
	}
	return res
}

// tgwAttachment builds a TransitGatewayAttachment with the given TGW ID and state.
func tgwAttachment(tgwID, attachID string, state ec2types.TransitGatewayAttachmentState) ec2types.TransitGatewayAttachment {
	return ec2types.TransitGatewayAttachment{
		TransitGatewayId:           aws.String(tgwID),
		TransitGatewayAttachmentId: aws.String(attachID),
		State:                      state,
	}
}

// TestEnrichTGWAttachments_AllAvailableProducesNoFindings verifies that when all
// attachments for both TGWs are in the "available" state, no findings are produced.
func TestEnrichTGWAttachments_AllAvailableProducesNoFindings(t *testing.T) {
	fake := &tgwAttachmentFake{
		results: map[string][]ec2types.TransitGatewayAttachment{
			"tgw-00000001": {tgwAttachment("tgw-00000001", "tgw-attach-a001", ec2types.TransitGatewayAttachmentStateAvailable)},
			"tgw-00000002": {tgwAttachment("tgw-00000002", "tgw-attach-a002", ec2types.TransitGatewayAttachmentStateAvailable)},
		},
	}
	clients := &awsclient.ServiceClients{EC2: fake}
	resources := tgwResources("tgw-00000001", "tgw-00000002")

	result, err := awsclient.EnrichTGWAttachments(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Findings == nil {
		t.Fatal("Findings must not be nil")
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings, got %d: %v", len(result.Findings), result.Findings)
	}
	if result.IssueCount != 0 {
		t.Errorf("IssueCount = %d, want 0", result.IssueCount)
	}
}

// TestEnrichTGWAttachments_FailedAttachmentProducesFindingSevBang verifies that
// when TGW-1 has an attachment in "failed" state, a finding with severity "!" is
// produced for TGW-1, and TGW-2 (all available) produces no finding.
func TestEnrichTGWAttachments_FailedAttachmentProducesFindingSevBang(t *testing.T) {
	fake := &tgwAttachmentFake{
		results: map[string][]ec2types.TransitGatewayAttachment{
			"tgw-00000001": {tgwAttachment("tgw-00000001", "tgw-attach-b001", ec2types.TransitGatewayAttachmentStateFailed)},
			"tgw-00000002": {tgwAttachment("tgw-00000002", "tgw-attach-b002", ec2types.TransitGatewayAttachmentStateAvailable)},
		},
	}
	clients := &awsclient.ServiceClients{EC2: fake}
	resources := tgwResources("tgw-00000001", "tgw-00000002")

	result, err := awsclient.EnrichTGWAttachments(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f, ok := result.Findings["tgw-00000001"]
	if !ok {
		t.Fatalf("expected finding keyed by %q", "tgw-00000001")
	}
	if f.Severity != "!" {
		t.Errorf("severity = %q, want %q", f.Severity, "!")
	}
	if _, ok := result.Findings["tgw-00000002"]; ok {
		t.Error("tgw-00000002 must NOT appear in Findings — all its attachments are available")
	}
	if result.IssueCount != 1 {
		t.Errorf("IssueCount = %d, want 1", result.IssueCount)
	}
}

// TestEnrichTGWAttachments_ModifyingAttachmentProducesFindingSevTilde verifies that
// when TGW-1 has an attachment in "modifying" state, a finding with severity "~" is
// produced (informational, not breaking).
func TestEnrichTGWAttachments_ModifyingAttachmentProducesFindingSevTilde(t *testing.T) {
	fake := &tgwAttachmentFake{
		results: map[string][]ec2types.TransitGatewayAttachment{
			"tgw-00000001": {tgwAttachment("tgw-00000001", "tgw-attach-c001", ec2types.TransitGatewayAttachmentStateModifying)},
			"tgw-00000002": {tgwAttachment("tgw-00000002", "tgw-attach-c002", ec2types.TransitGatewayAttachmentStateAvailable)},
		},
	}
	clients := &awsclient.ServiceClients{EC2: fake}
	resources := tgwResources("tgw-00000001", "tgw-00000002")

	result, err := awsclient.EnrichTGWAttachments(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f, ok := result.Findings["tgw-00000001"]
	if !ok {
		t.Fatalf("expected finding keyed by %q for modifying attachment", "tgw-00000001")
	}
	if f.Severity != "~" {
		t.Errorf("severity = %q, want %q", f.Severity, "~")
	}
	if result.IssueCount != 1 {
		t.Errorf("IssueCount = %d, want 1", result.IssueCount)
	}
}

// TestEnrichTGWAttachments_NilClientReturnsEmptyFindingsNoError verifies that when
// clients.EC2 is nil the enricher returns a non-nil empty Findings map and no error.
func TestEnrichTGWAttachments_NilClientReturnsEmptyFindingsNoError(t *testing.T) {
	clients := &awsclient.ServiceClients{EC2: nil}

	result, err := awsclient.EnrichTGWAttachments(context.Background(), clients, tgwResources("tgw-00000001", "tgw-00000002"))
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

// TestEnrichTGWAttachments_APIErrorSetsTruncatedNoError verifies that when the
// API call for TGW-1 returns an error, the enricher sets Truncated=true, produces
// 0 findings, and does not propagate the error.
func TestEnrichTGWAttachments_APIErrorSetsTruncatedNoError(t *testing.T) {
	apiErr := errors.New("ec2: DescribeTransitGatewayAttachments throttled")
	fake := &tgwAttachmentFake{
		errByTGW: map[string]error{
			"tgw-00000001": apiErr,
		},
		results: map[string][]ec2types.TransitGatewayAttachment{
			"tgw-00000002": {tgwAttachment("tgw-00000002", "tgw-attach-d001", ec2types.TransitGatewayAttachmentStateAvailable)},
		},
	}
	clients := &awsclient.ServiceClients{EC2: fake}
	resources := tgwResources("tgw-00000001", "tgw-00000002")

	result, err := awsclient.EnrichTGWAttachments(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings on API error, got %d", len(result.Findings))
	}
	if !result.Truncated {
		t.Error("Truncated must be true when an API call fails")
	}
}
