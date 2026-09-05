package unit

// prowler_w3_tgw_test.go — tgw.auto-accept-attachments.
//
// A transit gateway set to auto-accept shared attachments joins any VPC an
// RAM share points at it without review, so the option is a posture signal
// independent of the gateway's lifecycle state. A gateway already being torn
// down is not something an operator can act on, so it stays clean.

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

const (
	w3CodeTGWAutoAccept = domain.FindingCode("tgw.auto-accept-attachments")
	w3CodeTGWPending    = domain.FindingCode("tgw.state.pending")
	w3CodeTGWDeleting   = domain.FindingCode("tgw.state.deleting")

	w3PhraseTGWAutoAccept = "auto-accepts shared attachments"
)

type w3TGWAPI struct{ gateways []ec2types.TransitGateway }

func (f w3TGWAPI) DescribeTransitGateways(_ context.Context, _ *ec2.DescribeTransitGatewaysInput, _ ...func(*ec2.Options)) (*ec2.DescribeTransitGatewaysOutput, error) {
	return &ec2.DescribeTransitGatewaysOutput{TransitGateways: f.gateways}, nil
}

func w3FetchTGWs(t *testing.T, gws ...ec2types.TransitGateway) []resource.Resource {
	t.Helper()
	res, err := awsclient.FetchTransitGatewaysPage(context.Background(), w3TGWAPI{gateways: gws}, "")
	if err != nil {
		t.Fatalf("FetchTransitGatewaysPage: %v", err)
	}
	return res.Resources
}

// w3TGW builds a realistic DescribeTransitGateways row. Passing opts nil
// models the response AWS returns while a gateway is still being created.
func w3TGW(id, state string, opts *ec2types.TransitGatewayOptions) ec2types.TransitGateway {
	return ec2types.TransitGateway{
		TransitGatewayId: aws.String(id),
		TransitGatewayArn: aws.String(
			"arn:aws:ec2:us-east-1:123456789012:transit-gateway/" + id),
		State:       ec2types.TransitGatewayState(state),
		OwnerId:     aws.String("123456789012"),
		Description: aws.String("acme shared backbone"),
		Options:     opts,
		Tags:        []ec2types.Tag{{Key: aws.String("Name"), Value: aws.String("acme-backbone")}},
	}
}

// w3TGWOptions mirrors the option block AWS returns, with only the
// auto-accept setting varying.
func w3TGWOptions(autoAccept ec2types.AutoAcceptSharedAttachmentsValue) *ec2types.TransitGatewayOptions {
	return &ec2types.TransitGatewayOptions{
		AmazonSideAsn:                aws.Int64(64512),
		AutoAcceptSharedAttachments:  autoAccept,
		DefaultRouteTableAssociation: ec2types.DefaultRouteTableAssociationValueEnable,
		DefaultRouteTablePropagation: ec2types.DefaultRouteTablePropagationValueEnable,
		DnsSupport:                   ec2types.DnsSupportValueEnable,
	}
}

func TestW3TGWAutoAccept_Flagged(t *testing.T) {
	rows := w3FetchTGWs(t, w3TGW("tgw-0aa11bb22cc33dd44", "available",
		w3TGWOptions(ec2types.AutoAcceptSharedAttachmentsValueEnable)))

	f, ok := w3FindingByCode(rows[0].Findings, w3CodeTGWAutoAccept)
	if !ok {
		t.Fatalf("AutoAcceptSharedAttachments=enable produced no %s; findings=%+v", w3CodeTGWAutoAccept, rows[0].Findings)
	}
	if f.Phrase != w3PhraseTGWAutoAccept {
		t.Errorf("Phrase = %q, want %q", f.Phrase, w3PhraseTGWAutoAccept)
	}
	if f.Severity != domain.SevWarn {
		t.Errorf("Severity = %v, want SevWarn", f.Severity)
	}
	if f.Source != "wave1" {
		t.Errorf("Source = %q, want %q", f.Source, "wave1")
	}
	if f.Detail == "" {
		t.Error("Detail is empty; every finding carries an operator sentence")
	}
	w3AssertRows(t, rows[0].AttentionDetails[w3CodeTGWAutoAccept].Rows, [][2]string{
		{"Auto-accept shared attachments", "enabled"},
	})
}

func TestW3TGWAutoAccept_HealthyAndUnknown(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts *ec2types.TransitGatewayOptions
	}{
		{"explicitly disabled", w3TGWOptions(ec2types.AutoAcceptSharedAttachmentsValueDisable)},
		{"options absent", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows := w3FetchTGWs(t, w3TGW("tgw-0aa11bb22cc33dd44", "available", tc.opts))
			if f, ok := w3FindingByCode(rows[0].Findings, w3CodeTGWAutoAccept); ok {
				t.Errorf("got %s (%q), want no finding", f.Code, f.Phrase)
			}
		})
	}
}

// TestW3TGWAutoAccept_IndependentOfState pins that a transitional gateway
// still reports its posture — the two conditions are evaluated separately.
func TestW3TGWAutoAccept_IndependentOfState(t *testing.T) {
	rows := w3FetchTGWs(t, w3TGW("tgw-0aa11bb22cc33dd44", "pending",
		w3TGWOptions(ec2types.AutoAcceptSharedAttachmentsValueEnable)))

	if _, ok := w3FindingByCode(rows[0].Findings, w3CodeTGWPending); !ok {
		t.Errorf("missing %s; findings=%+v", w3CodeTGWPending, rows[0].Findings)
	}
	if _, ok := w3FindingByCode(rows[0].Findings, w3CodeTGWAutoAccept); !ok {
		t.Errorf("missing %s; findings=%+v", w3CodeTGWAutoAccept, rows[0].Findings)
	}
	if len(rows[0].Findings) != 2 {
		t.Errorf("got %d findings %+v, want exactly 2", len(rows[0].Findings), rows[0].Findings)
	}
}

// TestW3TGWAutoAccept_DeletingEmitsNoPostureFinding pins the disposal rule: a
// gateway on its way out has no configuration left to fix, so surfacing a
// posture warning on it is pure noise.
func TestW3TGWAutoAccept_DeletingEmitsNoPostureFinding(t *testing.T) {
	rows := w3FetchTGWs(t, w3TGW("tgw-0aa11bb22cc33dd44", "deleting",
		w3TGWOptions(ec2types.AutoAcceptSharedAttachmentsValueEnable)))

	if _, ok := w3FindingByCode(rows[0].Findings, w3CodeTGWAutoAccept); ok {
		t.Errorf("deleting gateway carries %s; findings=%+v", w3CodeTGWAutoAccept, rows[0].Findings)
	}
	if _, ok := w3FindingByCode(rows[0].Findings, w3CodeTGWDeleting); !ok {
		t.Errorf("missing %s; findings=%+v", w3CodeTGWDeleting, rows[0].Findings)
	}
}
