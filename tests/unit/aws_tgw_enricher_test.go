package unit

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

type tgwAttachmentFake struct {
	awsclient.EC2API
	results  map[string][]ec2types.TransitGatewayAttachment
	errByTGW map[string]error
}

func (f *tgwAttachmentFake) DescribeTransitGatewayAttachments(
	_ context.Context,
	in *ec2.DescribeTransitGatewayAttachmentsInput,
	_ ...func(*ec2.Options),
) (*ec2.DescribeTransitGatewayAttachmentsOutput, error) {
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

var _ awsclient.EC2API = (*tgwAttachmentFake)(nil)

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

func tgwAttachment(tgwID, attachID string, state ec2types.TransitGatewayAttachmentState) ec2types.TransitGatewayAttachment {
	return ec2types.TransitGatewayAttachment{
		TransitGatewayId:           aws.String(tgwID),
		TransitGatewayAttachmentId: aws.String(attachID),
		State:                      state,
	}
}

func TestEnrichTGWAttachments_AllAvailableProducesNoFindings(t *testing.T) {
	fake := &tgwAttachmentFake{
		results: map[string][]ec2types.TransitGatewayAttachment{
			"tgw-00000001": {tgwAttachment("tgw-00000001", "tgw-attach-a001", ec2types.TransitGatewayAttachmentStateAvailable)},
			"tgw-00000002": {tgwAttachment("tgw-00000002", "tgw-attach-a002", ec2types.TransitGatewayAttachmentStateAvailable)},
		},
	}
	clients := &awsclient.ServiceClients{EC2: fake}
	resources := tgwResources("tgw-00000001", "tgw-00000002")

	result, err := awsclient.EnrichTGWAttachments(context.Background(), clients, resources, nil)
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

func TestEnrichTGWAttachments_FailedAttachmentProducesFindingSevBang(t *testing.T) {
	fake := &tgwAttachmentFake{
		results: map[string][]ec2types.TransitGatewayAttachment{
			"tgw-00000001": {tgwAttachment("tgw-00000001", "tgw-attach-b001", ec2types.TransitGatewayAttachmentStateFailed)},
			"tgw-00000002": {tgwAttachment("tgw-00000002", "tgw-attach-b002", ec2types.TransitGatewayAttachmentStateAvailable)},
		},
	}
	clients := &awsclient.ServiceClients{EC2: fake}
	resources := tgwResources("tgw-00000001", "tgw-00000002")

	result, err := awsclient.EnrichTGWAttachments(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fs, ok := result.Findings["tgw-00000001"]
	if !ok {
		t.Fatalf("expected finding keyed by %q", "tgw-00000001")
	}
	f := fs[0]
	if f.Severity != domain.SevBroken {
		t.Errorf("severity = %v, want %v", f.Severity, "!")
	}
	if _, ok := result.Findings["tgw-00000002"]; ok {
		t.Error("tgw-00000002 must NOT appear in Findings — all its attachments are available")
	}
}

func TestEnrichTGWAttachments_ModifyingAttachmentProducesFindingSevTilde(t *testing.T) {
	fake := &tgwAttachmentFake{
		results: map[string][]ec2types.TransitGatewayAttachment{
			"tgw-00000001": {tgwAttachment("tgw-00000001", "tgw-attach-c001", ec2types.TransitGatewayAttachmentStateModifying)},
			"tgw-00000002": {tgwAttachment("tgw-00000002", "tgw-attach-c002", ec2types.TransitGatewayAttachmentStateAvailable)},
		},
	}
	clients := &awsclient.ServiceClients{EC2: fake}
	resources := tgwResources("tgw-00000001", "tgw-00000002")

	result, err := awsclient.EnrichTGWAttachments(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fs, ok := result.Findings["tgw-00000001"]
	if !ok {
		t.Fatalf("expected finding keyed by %q for modifying attachment", "tgw-00000001")
	}
	f := fs[0]
	if f.Severity != domain.SevWarn {
		t.Errorf("severity = %v, want %v", f.Severity, "~")
	}
	if len(result.Findings) != 1 {
		t.Errorf("len(Findings) = %d, want 1 (only tgw-00000001, the available one must not appear)", len(result.Findings))
	}
}

func TestEnrichTGWAttachments_NilClientReturnsEmptyFindingsNoError(t *testing.T) {
	clients := &awsclient.ServiceClients{EC2: nil}

	result, err := awsclient.EnrichTGWAttachments(context.Background(), clients, tgwResources("tgw-00000001", "tgw-00000002"), nil)
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

func TestEnrichTGWAttachments_APIErrorSetsTruncatedAndSurfacesError(t *testing.T) {
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

	result, err := awsclient.EnrichTGWAttachments(context.Background(), clients, resources, nil)
	if err == nil {
		t.Fatal("enricher must surface a composite error when an API call fails")
	}
	// The aggregate names the call, not the type: the type comes from the
	// registry key at the surface, and a type in the label would render it
	// twice ("enrich tgw: tgw: DescribeTransitGatewayAttachments ...").
	if errStr := err.Error(); !strings.Contains(errStr, "DescribeTransitGatewayAttachments") {
		t.Errorf("composite error must name the call, %q, got: %q", "DescribeTransitGatewayAttachments", errStr)
	}
	if errStr := err.Error(); !strings.Contains(errStr, "tgw-00000001") {
		t.Errorf("composite error must contain the failing TGW ID \"tgw-00000001\", got: %q", errStr)
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings on API error, got %d", len(result.Findings))
	}
	if !result.Truncated {
		t.Error("Truncated must be true when an API call fails")
	}
}
