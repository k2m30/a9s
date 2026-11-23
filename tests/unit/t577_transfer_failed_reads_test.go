package unit

import (
	"context"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/transfer"
	transfertypes "github.com/aws/aws-sdk-go-v2/service/transfer/types"
	"github.com/aws/smithy-go"

	"github.com/k2m30/a9s/v3/core/app"
	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo/fakes"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// t577Denied is the error the SDK hands back for an IAM denial: an
// OperationError naming the operation around an AccessDeniedException.
func t577Denied(service, op, action string) error {
	return &smithy.OperationError{
		ServiceID:     service,
		OperationName: op,
		Err: &smithy.GenericAPIError{
			Code:    "AccessDeniedException",
			Message: "User: arn:aws:sts::123456789012:assumed-role/example-readonly/session is not authorized to perform: " + action,
		},
	}
}

type t577TransferFake struct {
	awsclient.TransferAPI
	denyDescribeAgreement bool
	failCertificate       string
}

func (f *t577TransferFake) DescribeAgreement(ctx context.Context, in *transfer.DescribeAgreementInput, opts ...func(*transfer.Options)) (*transfer.DescribeAgreementOutput, error) {
	if f.denyDescribeAgreement {
		return nil, t577Denied("Transfer", "DescribeAgreement", "transfer:DescribeAgreement")
	}
	return f.TransferAPI.DescribeAgreement(ctx, in, opts...)
}

func (f *t577TransferFake) DescribeCertificate(ctx context.Context, in *transfer.DescribeCertificateInput, opts ...func(*transfer.Options)) (*transfer.DescribeCertificateOutput, error) {
	if aws.ToString(in.CertificateId) == f.failCertificate {
		return nil, t577Denied("Transfer", "DescribeCertificate", "transfer:DescribeCertificate")
	}
	return f.TransferAPI.DescribeCertificate(ctx, in, opts...)
}

func t577HasFinding(fs []domain.Finding, code domain.FindingCode) bool {
	for _, f := range fs {
		if f.Code == code {
			return true
		}
	}
	return false
}

// A role allowed transfer:ListAgreements but denied transfer:DescribeAgreement
// still sees every agreement ListAgreements named: the ListedAgreement carries
// the id, description, Status and both profile ids, so the row is built from
// it, marked as not inspected, and an INACTIVE agreement keeps its warning —
// Status came from the list call, not the denied describe.
func TestT577_TransferAgreements_DescribeDenied_RowsBuiltFromListedAgreement(t *testing.T) {
	child := transferAgreementsChildShortName(t)
	fake := &t577TransferFake{TransferAPI: fakes.NewTransfer(), denyDescribeAgreement: true}

	listed, err := fakes.NewTransfer().ListAgreements(context.Background(), &transfer.ListAgreementsInput{ServerId: aws.String(fixtures.ProdAS2GatewayID)})
	if err != nil {
		t.Fatalf("precondition: demo ListAgreements: %v", err)
	}
	if len(listed.Agreements) != 2 {
		t.Fatalf("precondition: demo server %s lists %d agreements, want 2", fixtures.ProdAS2GatewayID, len(listed.Agreements))
	}

	result, err := awsclient.FetchTransferAgreements(context.Background(), fake, fixtures.ProdAS2GatewayID, "")

	if err == nil {
		t.Fatal("FetchTransferAgreements returned nil error with every DescribeAgreement denied; the failure must be reported")
	}
	if !strings.Contains(err.Error(), "DescribeAgreement") || !strings.Contains(err.Error(), "2 of 2") {
		t.Errorf("error = %q, want it to name DescribeAgreement failing for 2 of 2", err)
	}
	if len(result.Resources) != len(listed.Agreements) {
		t.Fatalf("got %d agreement rows, want %d — a denied describe must not hide an agreement ListAgreements named", len(result.Resources), len(listed.Agreements))
	}

	deniedCode := awsclient.DetailsDeniedCode(child)
	inactiveCode := domain.FindingCode("transfer.warn.agreement_inactive")
	for _, la := range listed.Agreements {
		id := aws.ToString(la.AgreementId)
		row := mustFindTransferResource(t, result.Resources, id)

		if got, want := row.Fields["agreement_id"], id; got != want {
			t.Errorf("%s: agreement_id = %q, want %q", id, got, want)
		}
		if got, want := row.Fields["description"], aws.ToString(la.Description); got != want {
			t.Errorf("%s: description = %q, want the ListedAgreement's %q", id, got, want)
		}
		if got, want := row.Fields["local_profile"], aws.ToString(la.LocalProfileId); got != want {
			t.Errorf("%s: local_profile = %q, want the ListedAgreement's %q", id, got, want)
		}
		if got, want := row.Fields["partner_profile"], aws.ToString(la.PartnerProfileId); got != want {
			t.Errorf("%s: partner_profile = %q, want the ListedAgreement's %q", id, got, want)
		}
		if got, want := row.Fields["server_id"], fixtures.ProdAS2GatewayID; got != want {
			t.Errorf("%s: server_id = %q, want %q", id, got, want)
		}

		var denied *domain.Finding
		for i := range row.Findings {
			if row.Findings[i].Code == deniedCode {
				denied = &row.Findings[i]
			}
		}
		if denied == nil {
			t.Errorf("%s: findings %+v carry no %s — the row must read as not inspected, not healthy", id, row.Findings, deniedCode)
		} else if denied.Phrase != "details denied" {
			t.Errorf("%s: %s phrase = %q, want %q", id, deniedCode, denied.Phrase, "details denied")
		}

		wantInactive := la.Status == transfertypes.AgreementStatusTypeInactive
		if got := t577HasFinding(row.Findings, inactiveCode); got != wantInactive {
			t.Errorf("%s (Status %s): has %s = %v, want %v", id, la.Status, inactiveCode, got, wantInactive)
		}
		if got, want := row.Fields["status"], domain.StatusPhrase(row.Findings); got != want {
			t.Errorf("%s: status cell = %q, want the phrase of its findings %q", id, got, want)
		}
	}
}

func TestT577_TransferAgreements_DescribeAnswered_NoNotInspectedMark(t *testing.T) {
	child := transferAgreementsChildShortName(t)
	fake := &t577TransferFake{TransferAPI: fakes.NewTransfer()}

	result, err := awsclient.FetchTransferAgreements(context.Background(), fake, fixtures.ProdAS2GatewayID, "")
	if err != nil {
		t.Fatalf("FetchTransferAgreements: %v", err)
	}
	for _, r := range result.Resources {
		if t577HasFinding(r.Findings, awsclient.DetailsDeniedCode(child)) || t577HasFinding(r.Findings, awsclient.DetailsUnavailableCode(child)) {
			t.Errorf("%s: carries a degraded finding %+v with DescribeAgreement answering", r.ID, r.Findings)
		}
	}
}

// t577PartialAgreementEnrichment runs the real agreement detail enricher with
// one certificate lookup refused: the local profile's expiring certificate
// cannot be read, the partner profile's expired one can.
func t577PartialAgreementEnrichment(t *testing.T) (resource.Resource, resource.Resource, error) {
	t.Helper()
	child := transferAgreementsChildShortName(t)
	enrich := resource.GetDetailEnricher(child)
	if enrich == nil {
		t.Fatalf("no DetailEnricher registered for %q", child)
	}
	agreement := mustFindTransferResource(t, fetchTransferAgreementsForServer(t, fixtures.ProdAS2GatewayID), fixtures.AgreementProdPartnerID)

	fake := &t577TransferFake{TransferAPI: fakes.NewTransfer(), failCertificate: fixtures.CertExpiringID}
	enriched, err := enrich(context.Background(), &awsclient.DetailEnrichmentCtx{Clients: &awsclient.ServiceClients{Transfer: fake}}, agreement)
	if err == nil {
		t.Fatal("precondition: enricher returned nil error with DescribeCertificate refused")
	}
	if !t577HasFinding(enriched.Findings, "transfer.broken.cert_expired") {
		t.Fatalf("precondition: enricher's partial result lacks the resolved certificate's expired finding: %+v", enriched.Findings)
	}
	if enriched.Fields["local_profile"] != "ACME-LOCAL" || enriched.Fields["partner_profile"] != "PARTNER-CO" {
		t.Fatalf("precondition: enricher's partial result did not resolve both profiles: %+v", enriched.Fields)
	}
	return agreement, enriched, err
}

func t577AttentionValues(fields []app.FieldRow) string {
	var vals []string
	for _, f := range fields {
		if f.Path == "Attention" {
			vals = append(vals, f.Value)
		}
	}
	return strings.Join(vals, " | ")
}

// A detail enrichment that resolved part of the agreement and failed on one
// certificate lookup carries both facts: what it resolved is shown, and the
// lookup that did not run is reported. Discarding the partial result would
// hide an expired certificate a9s did read behind the one it could not.
func TestT577_TransferAgreementDetail_PartialEnrichment_AppliedAndReported_Web(t *testing.T) {
	child := transferAgreementsChildShortName(t)
	agreement, enriched, enrichErr := t577PartialAgreementEnrichment(t)

	ctrl := newTestController(t)
	ctrl.ApplyIntents([]runtime.UIIntent{
		runtime.PushScreen{
			ID:      runtime.ScreenDetail,
			Context: runtime.ScreenContext{ResourceType: child, ResourceID: agreement.ID},
		},
	})
	ctrl.EnsureDetailState(agreement, child)

	before := ctrl.Snapshot()
	if before.Body.Detail == nil {
		t.Fatal("precondition: no detail body for the open agreement")
	}
	if got := t577AttentionValues(before.Body.Detail.Fields); strings.Contains(got, "expired") {
		t.Fatalf("precondition: the unenriched agreement already shows %q", got)
	}

	ctrl.Handle(messages.EnrichDetailResult{
		ResourceType: child,
		ResourceID:   agreement.ID,
		EnrichedRes:  enriched,
		Err:          enrichErr,
	})

	snap := ctrl.Snapshot()
	if snap.Body.Detail == nil {
		t.Fatal("no detail body after the enrichment result")
	}
	if got := t577AttentionValues(snap.Body.Detail.Fields); !strings.Contains(got, "expired") {
		t.Errorf("Attention = %q, want the resolved certificate's Broken %q", got, "expired")
	}
	var local, partner string
	for _, f := range snap.Body.Detail.Fields {
		switch f.Path {
		case "local_profile":
			local = f.Value
		case "partner_profile":
			partner = f.Value
		}
	}
	if local != "ACME-LOCAL" || partner != "PARTNER-CO" {
		t.Errorf("profiles = (%q, %q), want the resolved As2Ids (%q, %q)", local, partner, "ACME-LOCAL", "PARTNER-CO")
	}
	if !snap.Header.Flash.IsError || !strings.Contains(snap.Header.Flash.Text, "enrich "+child) {
		t.Errorf("flash = %+v, want an error flash naming %q for the certificate lookup that failed", snap.Header.Flash, "enrich "+child)
	}
}

func TestT577_TransferAgreementDetail_PartialEnrichment_AppliedAndReported_TUI(t *testing.T) {
	child := transferAgreementsChildShortName(t)
	agreement, enriched, enrichErr := t577PartialAgreementEnrichment(t)

	m := newEnrichApp(t)
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		ResourceType: child,
		Resource:     &agreement,
	})
	before := stripANSI(rootViewContent(m))
	if strings.Contains(before, "ACME-LOCAL") || strings.Contains(before, "expired") {
		t.Fatalf("precondition: the unenriched agreement detail already shows enrichment output:\n%s", before)
	}

	m, _ = rootApplyMsg(m, messages.EnrichDetailResult{
		ResourceType: child,
		ResourceID:   agreement.ID,
		EnrichedRes:  enriched,
		Err:          enrichErr,
	})

	view := stripANSI(rootViewContent(m))
	var missing []string
	for _, want := range []string{"expired", "ACME-LOCAL", "PARTNER-CO", "enrich " + child} {
		if !strings.Contains(view, want) {
			missing = append(missing, want)
		}
	}
	if len(missing) > 0 {
		t.Errorf("TUI view after the partial enrichment lacks %q:\n%s", missing, view)
	}
}
