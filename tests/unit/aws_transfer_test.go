package unit

// aws_transfer_test.go — fetcher tests for FetchTransferServersPage plus the
// Agreements child fetcher (FetchTransferAgreements) and its detail-view
// cert/profile enrichment (docs/resources/transfer.md §3/§4,
// docs/resources/transfer-impl-plan.md §0/§1).
//
// Transfer is an in-fetcher N+1 (ListServers + DescribeServer per id, the
// mwaa/eks pattern) — every §3.2 signal is emitted fetcher-side with
// Source "wave1". Unlike mwaa, a details-denied row stays RICH: it is built
// from the ListedServer fields (state finding included) with the
// transfer.warn.details_denied finding appended, never degraded to a
// name-only row. These tests exercise the fetcher and the agreements child
// fetcher directly against the shared demo fixtures plus inline adversarial
// fakes.

import (
	"context"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/transfer"
	transfertypes "github.com/aws/aws-sdk-go-v2/service/transfer/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo/fakes"
	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

// fetchTransferDemoPage fetches the shared demo fixture page. The demo set
// includes the details-denied witness, so the fetch is a designed E5 partial
// success (rows + composite error naming only that fixture); any OTHER error
// fails the test.
func fetchTransferDemoPage(t *testing.T) resource.FetchResult {
	t.Helper()
	clients := &awsclient.ServiceClients{Transfer: fakes.NewTransfer()}
	result, err := awsclient.FetchTransferServersPage(context.Background(), clients, "")
	if err != nil && !strings.Contains(err.Error(), fixtures.WarnTransferDetailsDeniedID) {
		t.Fatalf("expected only the details-denied composite error, got %v", err)
	}
	return result
}

// mustFindTransferResource returns the resource with the given ID from a
// slice of fetched resources, failing the test if absent.
func mustFindTransferResource(t *testing.T, resources []resource.Resource, id string) resource.Resource {
	t.Helper()
	for _, r := range resources {
		if r.ID == id {
			return r
		}
	}
	t.Fatalf("resource %q not found in fetch result", id)
	return resource.Resource{}
}

// transferAsDescribedServer asserts RawStruct is a DescribedServer (healthy
// row contract, docs/resources/transfer-impl-plan.md §0), accepting either
// the pointer or value form.
func transferAsDescribedServer(t *testing.T, raw any) *transfertypes.DescribedServer {
	t.Helper()
	switch v := raw.(type) {
	case *transfertypes.DescribedServer:
		return v
	case transfertypes.DescribedServer:
		return &v
	default:
		t.Fatalf("RawStruct = %T, want transfertypes.DescribedServer (healthy row)", raw)
		return nil
	}
}

// transferAsListedServer asserts RawStruct is a ListedServer (degraded
// details-denied row contract), accepting either the pointer or value form.
func transferAsListedServer(t *testing.T, raw any) *transfertypes.ListedServer {
	t.Helper()
	switch v := raw.(type) {
	case *transfertypes.ListedServer:
		return v
	case transfertypes.ListedServer:
		return &v
	default:
		t.Fatalf("RawStruct = %T, want transfertypes.ListedServer (degraded/details-denied row)", raw)
		return nil
	}
}

// transferUnimplementedAPI supplies no-op implementations of the four
// Transfer operations not exercised by a given fetcher-only fake, so each
// local stub below only overrides the one or two methods under test while
// still satisfying awsclient.TransferAPI.
type transferUnimplementedAPI struct{}

func (transferUnimplementedAPI) ListAgreements(
	_ context.Context, _ *transfer.ListAgreementsInput, _ ...func(*transfer.Options),
) (*transfer.ListAgreementsOutput, error) {
	return nil, fmt.Errorf("ListAgreements should not be called in this test")
}

func (transferUnimplementedAPI) DescribeAgreement(
	_ context.Context, _ *transfer.DescribeAgreementInput, _ ...func(*transfer.Options),
) (*transfer.DescribeAgreementOutput, error) {
	return nil, fmt.Errorf("DescribeAgreement should not be called in this test")
}

func (transferUnimplementedAPI) DescribeProfile(
	_ context.Context, _ *transfer.DescribeProfileInput, _ ...func(*transfer.Options),
) (*transfer.DescribeProfileOutput, error) {
	return nil, fmt.Errorf("DescribeProfile should not be called in this test")
}

func (transferUnimplementedAPI) DescribeCertificate(
	_ context.Context, _ *transfer.DescribeCertificateInput, _ ...func(*transfer.Options),
) (*transfer.DescribeCertificateOutput, error) {
	return nil, fmt.Errorf("DescribeCertificate should not be called in this test")
}

// ---------------------------------------------------------------------------
// online_silence
// ---------------------------------------------------------------------------

func TestFetchTransferServersPage_OnlineSilence(t *testing.T) {
	result := fetchTransferDemoPage(t)
	r := mustFindTransferResource(t, result.Resources, fixtures.ProdAS2GatewayID)

	if len(r.Findings) != 0 {
		t.Errorf("Findings: expected 0 for healthy ONLINE server, got %d: %+v", len(r.Findings), r.Findings)
	}
	if r.Fields["status"] != "" {
		t.Errorf(`Fields["status"] = %q, want "" (S4 blank on a healthy row)`, r.Fields["status"])
	}
	described := transferAsDescribedServer(t, r.RawStruct)
	if aws.ToString(described.ServerId) != fixtures.ProdAS2GatewayID {
		t.Errorf("RawStruct.ServerId = %q, want %q", aws.ToString(described.ServerId), fixtures.ProdAS2GatewayID)
	}
}

// ---------------------------------------------------------------------------
// state_phrases — exact §4 phrase + severity + Detail per state; no raw enum
// leaks into Phrase or Fields["status"].
// ---------------------------------------------------------------------------

func TestFetchTransferServersPage_StatePhrases(t *testing.T) {
	result := fetchTransferDemoPage(t)

	cases := []struct {
		id       string
		phrase   string
		detail   string
		severity domain.Severity
		rawEnum  string
	}{
		{
			fixtures.WarnTransferOfflineID, "offline: not accepting transfers",
			"Server is offline; partners cannot connect until it is started.",
			domain.SevWarn, "OFFLINE",
		},
		{
			fixtures.WarnTransferStartingID, "starting",
			"Server is starting; not yet fully able to respond.",
			domain.SevWarn, "STARTING",
		},
		{
			fixtures.WarnTransferStoppingID, "stopping",
			"Server is stopping; transfers are draining.",
			domain.SevWarn, "STOPPING",
		},
		{
			fixtures.BrokenTransferStartFailedID, "start failed",
			"Server failed to come online; partner transfers are down.",
			domain.SevBroken, "START_FAILED",
		},
		{
			fixtures.WarnTransferStopFailedID, "stop failed",
			"Stop failed; the server may still be serving transfers.",
			domain.SevWarn, "STOP_FAILED",
		},
	}

	for _, tc := range cases {
		t.Run(tc.id, func(t *testing.T) {
			r := mustFindTransferResource(t, result.Resources, tc.id)
			if len(r.Findings) == 0 {
				t.Fatalf("Findings: expected at least 1 finding, got 0")
			}
			f := r.Findings[0]
			if f.Phrase != tc.phrase {
				t.Errorf("Findings[0].Phrase = %q, want %q", f.Phrase, tc.phrase)
			}
			if f.Severity != tc.severity {
				t.Errorf("Findings[0].Severity = %v, want %v", f.Severity, tc.severity)
			}
			if f.Detail != tc.detail {
				t.Errorf("Findings[0].Detail = %q, want %q", f.Detail, tc.detail)
			}
			if strings.Contains(f.Phrase, tc.rawEnum) {
				t.Errorf("Phrase %q must not contain the raw AWS enum %q", f.Phrase, tc.rawEnum)
			}
			if strings.Contains(r.Fields["status"], tc.rawEnum) {
				t.Errorf(`Fields["status"] = %q must not contain the raw AWS enum %q`, r.Fields["status"], tc.rawEnum)
			}
			if r.Fields["status"] != tc.phrase {
				t.Errorf(`Fields["status"] = %q, want %q (S4 mirrors the state finding phrase)`, r.Fields["status"], tc.phrase)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// legacy_policy — finding present, Detail names the policy, Phrase does NOT
// (Phrase/Row anti-duplication, transfer-impl-plan.md §1 item 11).
// ---------------------------------------------------------------------------

func TestFetchTransferServersPage_LegacyPolicyFinding(t *testing.T) {
	result := fetchTransferDemoPage(t)
	r := mustFindTransferResource(t, result.Resources, fixtures.WarnTransferLegacyPolicyID)

	var finding *domain.Finding
	for i := range r.Findings {
		if r.Findings[i].Phrase == "legacy security policy" {
			finding = &r.Findings[i]
			break
		}
	}
	if finding == nil {
		t.Fatalf("no %q finding in Findings: %+v", "legacy security policy", r.Findings)
	}
	if finding.Severity != domain.SevWarn {
		t.Errorf("Severity = %v, want SevWarn", finding.Severity)
	}
	const policyName = "TransferSecurityPolicy-2018-11"
	wantDetail := "Security policy " + policyName + " allows weak ciphers / old TLS; move to a current policy."
	if finding.Detail != wantDetail {
		t.Errorf("Detail = %q, want %q", finding.Detail, wantDetail)
	}
	if strings.Contains(finding.Phrase, policyName) {
		t.Errorf("Phrase %q must not contain the policy name %q — it belongs in Detail only", finding.Phrase, policyName)
	}
}

// ---------------------------------------------------------------------------
// no_logging — LoggingRole nil AND StructuredLogDestinations empty.
// ---------------------------------------------------------------------------

func TestFetchTransferServersPage_NoLoggingFinding(t *testing.T) {
	result := fetchTransferDemoPage(t)
	r := mustFindTransferResource(t, result.Resources, fixtures.WarnTransferNoLoggingID)

	var finding *domain.Finding
	for i := range r.Findings {
		if r.Findings[i].Phrase == "no activity logging" {
			finding = &r.Findings[i]
			break
		}
	}
	if finding == nil {
		t.Fatalf("no %q finding in Findings: %+v", "no activity logging", r.Findings)
	}
	if finding.Severity != domain.SevWarn {
		t.Errorf("Severity = %v, want SevWarn", finding.Severity)
	}
	const wantDetail = "Neither a logging role nor structured log destinations are configured."
	if finding.Detail != wantDetail {
		t.Errorf("Detail = %q, want %q", finding.Detail, wantDetail)
	}
}

// ---------------------------------------------------------------------------
// multi_stack — OFFLINE + legacy policy + no logging: findings ordered per
// §4 precedence (state, then legacy-policy, then no-logging).
// ---------------------------------------------------------------------------

func TestFetchTransferServersPage_MultiStackOrdered(t *testing.T) {
	result := fetchTransferDemoPage(t)
	r := mustFindTransferResource(t, result.Resources, fixtures.WarnTransferMultiID)

	got := make([]string, len(r.Findings))
	for i, f := range r.Findings {
		got[i] = f.Phrase
	}
	want := []string{"offline: not accepting transfers", "legacy security policy", "no activity logging"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ordered Findings phrases = %v, want %v", got, want)
	}
}

// ---------------------------------------------------------------------------
// details_denied_rich — a denied DescribeServer keeps the row, built from
// ListedServer fields (State ONLINE → no state finding), with the
// transfer.warn.details_denied finding appended; composite error names the id.
// ---------------------------------------------------------------------------

func TestFetchTransferServersPage_DetailsDeniedRich(t *testing.T) {
	clients := &awsclient.ServiceClients{Transfer: fakes.NewTransfer()}
	result, err := awsclient.FetchTransferServersPage(context.Background(), clients, "")
	if err == nil {
		t.Fatal("expected a composite error naming the details-denied server, got nil")
	}
	if !strings.Contains(err.Error(), fixtures.WarnTransferDetailsDeniedID) {
		t.Errorf("composite error must name %q, got: %q", fixtures.WarnTransferDetailsDeniedID, err.Error())
	}

	r := mustFindTransferResource(t, result.Resources, fixtures.WarnTransferDetailsDeniedID)

	if len(r.Findings) != 1 {
		t.Fatalf("Findings: expected exactly 1 (details-denied only — State ONLINE has no state finding), got %d: %+v",
			len(r.Findings), r.Findings)
	}
	f := r.Findings[0]
	if f.Code != "transfer.warn.details_denied" {
		t.Errorf("Findings[0].Code = %q, want %q", f.Code, "transfer.warn.details_denied")
	}
	if f.Phrase != "details denied" {
		t.Errorf("Findings[0].Phrase = %q, want %q", f.Phrase, "details denied")
	}
	if f.Severity != domain.SevWarn {
		t.Errorf("Findings[0].Severity = %v, want SevWarn", f.Severity)
	}
	const wantDetail = "Access to server details was denied; only the listed fields are visible."
	if f.Detail != wantDetail {
		t.Errorf("Findings[0].Detail = %q, want %q (transfer-specific — rows are rich, not name-only)", f.Detail, wantDetail)
	}
	if r.Fields["status"] != "details denied" {
		t.Errorf(`Fields["status"] = %q, want %q`, r.Fields["status"], "details denied")
	}

	listed := transferAsListedServer(t, r.RawStruct)
	if aws.ToString(listed.ServerId) != fixtures.WarnTransferDetailsDeniedID {
		t.Errorf("RawStruct.ServerId = %q, want %q", aws.ToString(listed.ServerId), fixtures.WarnTransferDetailsDeniedID)
	}
	if listed.State != transfertypes.StateOnline {
		t.Errorf("RawStruct.State = %v, want ONLINE (the row must keep the list-derived state, not lose it)", listed.State)
	}
}

// ---------------------------------------------------------------------------
// list_denied_is_error — AccessDenied on ListServers must never render as an
// empty successful result.
// ---------------------------------------------------------------------------

type transferListErrorFake struct {
	transferUnimplementedAPI
	err error
}

func (f *transferListErrorFake) ListServers(
	_ context.Context, _ *transfer.ListServersInput, _ ...func(*transfer.Options),
) (*transfer.ListServersOutput, error) {
	return nil, f.err
}

func (f *transferListErrorFake) DescribeServer(
	_ context.Context, _ *transfer.DescribeServerInput, _ ...func(*transfer.Options),
) (*transfer.DescribeServerOutput, error) {
	return nil, fmt.Errorf("DescribeServer should not be called when ListServers fails")
}

var _ awsclient.TransferAPI = (*transferListErrorFake)(nil)

func TestFetchTransferServersPage_ListDeniedIsError(t *testing.T) {
	fake := &transferListErrorFake{
		err: &transfertypes.AccessDeniedException{Message: aws.String("User is not authorized to perform transfer:ListServers")},
	}
	clients := &awsclient.ServiceClients{Transfer: fake}

	result, err := awsclient.FetchTransferServersPage(context.Background(), clients, "")
	if err == nil {
		t.Fatal("FetchTransferServersPage must return a non-nil error when ListServers is denied — never an empty successful result")
	}
	if !strings.Contains(err.Error(), "AccessDenied") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "AccessDenied")
	}
	if len(result.Resources) != 0 {
		t.Errorf("Resources: expected 0 on error, got %d", len(result.Resources))
	}
}

// ---------------------------------------------------------------------------
// partial_describe — U12/E5: 5 listed, 2 describes fail → 5 rows (2
// rich-degraded) + composite error naming both, in "N of M" form.
// ---------------------------------------------------------------------------

type transferPartialDescribeFake struct {
	transferUnimplementedAPI
	listed    []transfertypes.ListedServer
	servers   map[string]transfertypes.DescribedServer
	errByName map[string]error
}

func (f *transferPartialDescribeFake) ListServers(
	_ context.Context, _ *transfer.ListServersInput, _ ...func(*transfer.Options),
) (*transfer.ListServersOutput, error) {
	return &transfer.ListServersOutput{Servers: f.listed}, nil
}

func (f *transferPartialDescribeFake) DescribeServer(
	_ context.Context, input *transfer.DescribeServerInput, _ ...func(*transfer.Options),
) (*transfer.DescribeServerOutput, error) {
	id := aws.ToString(input.ServerId)
	if err, ok := f.errByName[id]; ok {
		return nil, err
	}
	s, ok := f.servers[id]
	if !ok {
		return nil, fmt.Errorf("server %q not found", id)
	}
	return &transfer.DescribeServerOutput{Server: &s}, nil
}

var _ awsclient.TransferAPI = (*transferPartialDescribeFake)(nil)

func transferPartialListedServer(id string) transfertypes.ListedServer {
	return transfertypes.ListedServer{
		ServerId:             aws.String(id),
		Arn:                  aws.String("arn:aws:transfer:us-east-1:123456789012:server/" + id),
		State:                transfertypes.StateOnline,
		Domain:               transfertypes.DomainS3,
		EndpointType:         transfertypes.EndpointTypePublic,
		IdentityProviderType: transfertypes.IdentityProviderTypeServiceManaged,
		UserCount:            aws.Int32(0),
	}
}

func transferPartialDescribedServer(id string) transfertypes.DescribedServer {
	return transfertypes.DescribedServer{
		ServerId:             aws.String(id),
		Arn:                  aws.String("arn:aws:transfer:us-east-1:123456789012:server/" + id),
		State:                transfertypes.StateOnline,
		Domain:               transfertypes.DomainS3,
		EndpointType:         transfertypes.EndpointTypePublic,
		IdentityProviderType: transfertypes.IdentityProviderTypeServiceManaged,
		LoggingRole:          aws.String("arn:aws:iam::123456789012:role/transfer-test-logging"),
		SecurityPolicyName:   aws.String("TransferSecurityPolicy-2024-01"),
		UserCount:            aws.Int32(0),
	}
}

func TestFetchTransferServersPage_PartialDescribe(t *testing.T) {
	ids := []string{"server-a", "server-b", "server-c", "server-denied", "server-missing"}
	listed := make([]transfertypes.ListedServer, len(ids))
	for i, id := range ids {
		listed[i] = transferPartialListedServer(id)
	}
	fake := &transferPartialDescribeFake{
		listed: listed,
		servers: map[string]transfertypes.DescribedServer{
			"server-a": transferPartialDescribedServer("server-a"),
			"server-b": transferPartialDescribedServer("server-b"),
			"server-c": transferPartialDescribedServer("server-c"),
		},
		errByName: map[string]error{
			"server-denied":  &transfertypes.AccessDeniedException{Message: aws.String("not authorized")},
			"server-missing": &transfertypes.ResourceNotFoundException{Message: aws.String("Server server-missing not found")},
		},
	}
	clients := &awsclient.ServiceClients{Transfer: fake}

	result, err := awsclient.FetchTransferServersPage(context.Background(), clients, "")
	if err == nil {
		t.Fatal("FetchTransferServersPage must return a composite error when DescribeServer fails for some servers")
	}
	if len(result.Resources) != 5 {
		t.Fatalf("got %d resources, want 5 — a denied/missing DescribeServer must never make a listed server vanish", len(result.Resources))
	}

	degraded := map[string]bool{"server-denied": true, "server-missing": true}
	for _, r := range result.Resources {
		if degraded[r.ID] {
			if len(r.Findings) != 1 || r.Findings[0].Phrase != "details denied" {
				t.Errorf("degraded row %q must carry exactly the %q finding, got %+v", r.ID, "details denied", r.Findings)
			}
			continue
		}
		if len(r.Findings) != 0 {
			t.Errorf("healthy row %q: expected 0 findings, got %+v", r.ID, r.Findings)
		}
	}

	errStr := err.Error()
	for _, want := range []string{"server-denied", "server-missing", "AccessDenied", "ResourceNotFound", "2 of 5"} {
		if !strings.Contains(errStr, want) {
			t.Errorf("composite error must contain %q, got: %q", want, errStr)
		}
	}
}

// ---------------------------------------------------------------------------
// wave3_anti — no Wave-3 metric/history strings anywhere; UserCount==0 and a
// lone nil LoggingRole (structured logs still present) never become findings.
// ---------------------------------------------------------------------------

type transferSingleServerFake struct {
	transferUnimplementedAPI
	listed    transfertypes.ListedServer
	described transfertypes.DescribedServer
}

func (f *transferSingleServerFake) ListServers(
	_ context.Context, _ *transfer.ListServersInput, _ ...func(*transfer.Options),
) (*transfer.ListServersOutput, error) {
	return &transfer.ListServersOutput{Servers: []transfertypes.ListedServer{f.listed}}, nil
}

func (f *transferSingleServerFake) DescribeServer(
	_ context.Context, _ *transfer.DescribeServerInput, _ ...func(*transfer.Options),
) (*transfer.DescribeServerOutput, error) {
	d := f.described
	return &transfer.DescribeServerOutput{Server: &d}, nil
}

var _ awsclient.TransferAPI = (*transferSingleServerFake)(nil)

func TestFetchTransferServersPage_WaveThreeAntiTests(t *testing.T) {
	result := fetchTransferDemoPage(t)

	forbidden := []string{"FilesIn", "FilesOut", "BytesIn", "BytesOut", "InboundMessage", "OutboundMessage", "ListExecutions"}
	for _, r := range result.Resources {
		for _, f := range r.Findings {
			for _, s := range forbidden {
				if strings.Contains(f.Phrase, s) || strings.Contains(f.Detail, s) {
					t.Errorf("resource %q: Finding %+v must never surface Wave-3 metric/history name %q", r.ID, f, s)
				}
			}
		}
	}

	// prod-as2-gateway has UserCount 0 — legitimately zero for an AS2/external-IdP
	// server (spec §3.1) — must never raise a finding.
	gateway := mustFindTransferResource(t, result.Resources, fixtures.ProdAS2GatewayID)
	if len(gateway.Findings) != 0 {
		t.Errorf("UserCount==0 must not raise a finding, got Findings: %+v", gateway.Findings)
	}

	// LoggingRole nil ALONE (StructuredLogDestinations still populated) must
	// not raise the no-logging finding — only the AND-condition (both empty)
	// does (spec §3.2).
	id := "wave3-anti-logging-role-nil"
	listed := transferPartialListedServer(id)
	described := transferPartialDescribedServer(id)
	described.LoggingRole = nil
	described.StructuredLogDestinations = []string{"arn:aws:logs:us-east-1:123456789012:log-group:/aws/transfer/wave3-anti:*"}

	fake := &transferSingleServerFake{listed: listed, described: described}
	clients := &awsclient.ServiceClients{Transfer: fake}
	single, err := awsclient.FetchTransferServersPage(context.Background(), clients, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	r := mustFindTransferResource(t, single.Resources, id)
	if len(r.Findings) != 0 {
		t.Errorf("LoggingRole nil alone (structured logs present) must not raise a finding, got Findings: %+v", r.Findings)
	}
}

// ---------------------------------------------------------------------------
// E7 sanity — Resource.ID is ServerId; Fields["arn"] holds the full ARN.
// ---------------------------------------------------------------------------

func TestFetchTransferServersPage_ResourceIDAndArnMapping(t *testing.T) {
	result := fetchTransferDemoPage(t)
	r := mustFindTransferResource(t, result.Resources, fixtures.ProdAS2GatewayID)

	if r.ID != fixtures.ProdAS2GatewayID {
		t.Errorf("ID = %q, want ServerId %q", r.ID, fixtures.ProdAS2GatewayID)
	}
	wantArn := "arn:aws:transfer:us-east-1:123456789012:server/" + fixtures.ProdAS2GatewayID
	if r.Fields["arn"] != wantArn {
		t.Errorf(`Fields["arn"] = %q, want %q`, r.Fields["arn"], wantArn)
	}
}

// ---------------------------------------------------------------------------
// Agreements child fetcher — INACTIVE agreement carries the "inactive:
// partner traffic rejected" finding; ACTIVE carries none.
// ---------------------------------------------------------------------------

func fetchTransferAgreementsForServer(t *testing.T, serverID string) []resource.Resource {
	t.Helper()
	result, err := awsclient.FetchTransferAgreements(context.Background(), fakes.NewTransfer(), serverID, "")
	if err != nil {
		t.Fatalf("FetchTransferAgreements(%s) returned error: %v", serverID, err)
	}
	return result.Resources
}

func TestFetchTransferAgreements_InactiveFinding(t *testing.T) {
	agreements := fetchTransferAgreementsForServer(t, fixtures.ProdAS2GatewayID)

	inactive := mustFindTransferResource(t, agreements, fixtures.AgreementOldPartnerID)
	var finding *domain.Finding
	for i := range inactive.Findings {
		if inactive.Findings[i].Phrase == "inactive: partner traffic rejected" {
			finding = &inactive.Findings[i]
			break
		}
	}
	if finding == nil {
		t.Fatalf("no %q finding on INACTIVE agreement, got Findings: %+v", "inactive: partner traffic rejected", inactive.Findings)
	}
	if finding.Severity != domain.SevWarn {
		t.Errorf("Severity = %v, want SevWarn", finding.Severity)
	}

	active := mustFindTransferResource(t, agreements, fixtures.AgreementProdPartnerID)
	if len(active.Findings) != 0 {
		t.Errorf("ACTIVE agreement: expected 0 findings, got %+v", active.Findings)
	}
}

// ---------------------------------------------------------------------------
// Agreement detail — inline profile/cert resolution via DetailEnrich:
// expired cert → Broken "expired"; <30d cert → Warning "expires in <N>d".
// ---------------------------------------------------------------------------

// transferAgreementsChildShortName discovers the registered Agreements
// child-type ShortName from the "transfer" catalog entry's own Children
// list, rather than guessing a literal — Agreements is documented as the
// only server-scoped child (docs/resources/transfer.md §2.1).
func transferAgreementsChildShortName(t *testing.T) string {
	t.Helper()
	def := resource.FindResourceType("transfer")
	if def == nil {
		t.Fatal(`resource.FindResourceType("transfer") returned nil — transfer catalog registration missing`)
	}
	if len(def.Children) == 0 {
		t.Fatal(`transfer catalog entry has no Children — expected the Agreements child view (docs/resources/transfer.md §2.1)`)
	}
	return def.Children[0].ChildType
}

func TestTransferAgreementDetailEnrich_CertExpiry(t *testing.T) {
	childShortName := transferAgreementsChildShortName(t)
	enrich := resource.GetDetailEnricher(childShortName)
	if enrich == nil {
		t.Fatalf("no DetailEnricher registered for transfer agreements child type %q", childShortName)
	}

	agreements := fetchTransferAgreementsForServer(t, fixtures.ProdAS2GatewayID)
	agreement := mustFindTransferResource(t, agreements, fixtures.AgreementProdPartnerID)

	clients := &awsclient.ServiceClients{Transfer: fakes.NewTransfer()}
	enriched, err := enrich(context.Background(), clients, agreement)
	if err != nil {
		t.Fatalf("DetailEnrich returned error: %v", err)
	}

	expiringRe := regexp.MustCompile(`expires in \d+d`)
	var expiredFinding, expiringFinding *domain.Finding
	for i := range enriched.Findings {
		f := &enriched.Findings[i]
		switch {
		case f.Phrase == "expired":
			expiredFinding = f
		case expiringRe.MatchString(f.Phrase):
			expiringFinding = f
		}
	}

	if expiredFinding == nil {
		t.Fatalf("no %q finding for cert %q, got Findings: %+v", "expired", fixtures.CertExpiredID, enriched.Findings)
	}
	if expiredFinding.Severity != domain.SevBroken {
		t.Errorf("expired-cert Severity = %v, want SevBroken", expiredFinding.Severity)
	}

	if expiringFinding == nil {
		t.Fatalf("no %q finding for cert %q, got Findings: %+v", `expires in \d+d`, fixtures.CertExpiringID, enriched.Findings)
	}
	if expiringFinding.Severity != domain.SevWarn {
		t.Errorf("expiring-cert Severity = %v, want SevWarn", expiringFinding.Severity)
	}
}
