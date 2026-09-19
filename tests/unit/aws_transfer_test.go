package unit

// Transfer is an in-fetcher N+1 (ListServers + DescribeServer per id); every
// signal is emitted fetcher-side with Source "wave1". A row whose
// DescribeServer is denied stays rich: it is built from the ListedServer fields
// with the transfer.warn.details_denied finding appended.

import (
	"context"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/transfer"
	transfertypes "github.com/aws/aws-sdk-go-v2/service/transfer/types"

	"github.com/k2m30/a9s/v3/core/app"
	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/config"
	"github.com/k2m30/a9s/v3/core/demo/fakes"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/semantics/projection"
	"github.com/k2m30/a9s/v3/core/session"
)

// The demo set includes a server whose DescribeServer is denied, so the fetch
// returns rows plus a composite error naming only that server; any other error
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
			"The server is not accepting connections, so partners fail to connect rather than seeing an error they can act on. Start it if that was not deliberate, and tell the partners if it is going to stay down.",
			domain.SevWarn, "OFFLINE",
		},
		{
			fixtures.WarnTransferStartingID, "starting",
			"The server is coming up and is not answering yet, so a partner connecting now is refused. Wait for it to come online before rerunning a failed transfer.",
			domain.SevWarn, "STARTING",
		},
		{
			fixtures.WarnTransferStoppingID, "stopping",
			"The server is draining and will stop accepting connections shortly, so in-flight transfers are finishing and new ones are not. Confirm nothing is scheduled into the window before it goes down.",
			domain.SevWarn, "STOPPING",
		},
		{
			fixtures.BrokenTransferStartFailedID, "start failed",
			"The server could not come online, so every partner connecting to it is being refused and any transfer schedule behind it has stopped. Check the endpoint's VPC and address configuration and its identity provider, then start it again.",
			domain.SevBroken, "START_FAILED",
		},
		{
			fixtures.WarnTransferStopFailedID, "stop failed",
			"The stop did not take, so the server may still be accepting connections while its state says otherwise, and anything relying on it being down is wrong. Try the stop again and confirm the state before assuming it is off.",
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
	// Detail is the static sentence FindingDef declares for
	// transferCodeLegacyPolicy; the policy name stays in the resource's own fields.
	const wantDetail = "The server's security policy still allows weak ciphers and old TLS versions, so a client can be steered onto a breakable connection. Move the server to a current security policy."
	if finding.Detail != wantDetail {
		t.Errorf("Detail = %q, want %q", finding.Detail, wantDetail)
	}
	if strings.Contains(finding.Phrase, policyName) {
		t.Errorf("Phrase %q must not contain the policy name %q — it belongs in the resource's own fields only", finding.Phrase, policyName)
	}
	if strings.Contains(finding.Detail, policyName) {
		t.Errorf("Detail %q must not contain the policy name %q — one sentence per code, never keyed by state", finding.Detail, policyName)
	}
}

// No logging means LoggingRole nil and StructuredLogDestinations empty.

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
	const wantDetail = "The server records nothing about who connected or what moved, so a disputed or missing transfer cannot be reconstructed afterwards. Attach a logging role or configure a structured log destination."
	if finding.Detail != wantDetail {
		t.Errorf("Detail = %q, want %q", finding.Detail, wantDetail)
	}
}

// Findings are ordered state, then legacy policy, then no logging.

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
	const wantDetail = "Reading this server was denied, so its endpoint, logging and identity-provider settings are unjudged rather than clean. Grant the role you browse with permission to describe the server, then refresh."
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

func TestFetchTransferServersPage_ListDeniedIsError(t *testing.T) {
	fake := &fakeTransferServers{
		ListErr:     &transfertypes.AccessDeniedException{Message: aws.String("User is not authorized to perform transfer:ListServers")},
		DescribeErr: fmt.Errorf("DescribeServer should not be called when ListServers fails"),
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
	fake := &fakeTransferServers{
		Listed: listed,
		Servers: map[string]transfertypes.DescribedServer{
			"server-a": transferPartialDescribedServer("server-a"),
			"server-b": transferPartialDescribedServer("server-b"),
			"server-c": transferPartialDescribedServer("server-c"),
		},
		ErrByName: map[string]error{
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

	// server-denied → AccessDenied (auth) → "details denied";
	// server-missing → ResourceNotFound (non-auth) → the neutral "details
	// unavailable". A not-found server must never read as an IAM denial.
	wantPhrase := map[string]string{"server-denied": "details denied", "server-missing": "details unavailable"}
	for _, r := range result.Resources {
		if phrase, ok := wantPhrase[r.ID]; ok {
			if len(r.Findings) != 1 || r.Findings[0].Phrase != phrase {
				t.Errorf("degraded row %q must carry exactly the %q finding, got %+v", r.ID, phrase, r.Findings)
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

// UserCount 0 and a lone nil LoggingRole (structured logs still present) are
// never findings.

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

	// UserCount 0 is legitimate for an AS2/external-IdP server.
	gateway := mustFindTransferResource(t, result.Resources, fixtures.ProdAS2GatewayID)
	if len(gateway.Findings) != 0 {
		t.Errorf("UserCount==0 must not raise a finding, got Findings: %+v", gateway.Findings)
	}

	id := "wave3-anti-logging-role-nil"
	listed := transferPartialListedServer(id)
	described := transferPartialDescribedServer(id)
	described.LoggingRole = nil
	described.StructuredLogDestinations = []string{"arn:aws:logs:us-east-1:123456789012:log-group:/aws/transfer/wave3-anti:*"}

	fake := &fakeTransferServers{Listed: []transfertypes.ListedServer{listed}, Described: &described}
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

// transferAgreementsChildShortName reads the Agreements child ShortName from
// the transfer catalog entry's Children; Agreements is the only server-scoped
// child.
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
	enriched, err := enrich(context.Background(), &awsclient.DetailEnrichmentCtx{Clients: clients}, agreement)
	if err != nil {
		t.Fatalf("DetailEnrich returned error: %v", err)
	}

	expiringRe := regexp.MustCompile(`^expires in (\d+)d$`)
	var expiredFinding, expiringFinding *domain.Finding
	var expiringDays string
	for i := range enriched.Findings {
		f := &enriched.Findings[i]
		switch {
		case f.Phrase == "expired":
			expiredFinding = f
		default:
			if m := expiringRe.FindStringSubmatch(f.Phrase); m != nil {
				expiringFinding = f
				expiringDays = m[1]
			}
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
	d, err := strconv.Atoi(expiringDays)
	if err != nil {
		t.Fatalf("expiring-cert Phrase %q: day count %q did not parse: %v", expiringFinding.Phrase, expiringDays, err)
	}
	if d <= 0 || d >= 30 {
		t.Errorf("expiring-cert Phrase %q: day count %d out of the <30d warning window's own bound (0 < d < 30)", expiringFinding.Phrase, d)
	}
}

// The enricher writes Fields["local_profile"] and Fields["partner_profile"],
// and the Detail declares those keys, so the resolved As2Id, not the bare
// profile id, reaches the rendered detail.
func TestTransferAgreementDetailEnrich_As2IdVisibleInRenderedDetail(t *testing.T) {
	childShortName := transferAgreementsChildShortName(t)
	enrich := resource.GetDetailEnricher(childShortName)
	if enrich == nil {
		t.Fatalf("no DetailEnricher registered for transfer agreements child type %q", childShortName)
	}

	agreements := fetchTransferAgreementsForServer(t, fixtures.ProdAS2GatewayID)
	agreement := mustFindTransferResource(t, agreements, fixtures.AgreementProdPartnerID)

	clients := &awsclient.ServiceClients{Transfer: fakes.NewTransfer()}
	enriched, err := enrich(context.Background(), &awsclient.DetailEnrichmentCtx{Clients: clients}, agreement)
	if err != nil {
		t.Fatalf("DetailEnrich returned error: %v", err)
	}

	// projection.buildItems sets r.Type to the view's resource type when the
	// resource carries none.
	enriched.Type = childShortName

	sections := projection.GenericWithConfig(config.DefaultConfig())(enriched)
	if len(sections) == 0 {
		t.Fatal("projector returned zero sections for the enriched agreement resource")
	}

	rendered := make(map[string]string, 8)
	for _, sec := range sections {
		for _, item := range sec.Items {
			if item.Path != "" {
				rendered[item.Path] = item.Value
			}
		}
	}

	if got, want := rendered["local_profile"], "ACME-LOCAL"; got != want {
		t.Errorf("rendered local_profile = %q, want resolved As2Id %q — enriched.Fields[\"local_profile\"]=%q IS set correctly by enrichTransferAgreement but never reaches the detail render path (see test doc comment)", got, want, enriched.Fields["local_profile"])
	}
	if got, want := rendered["partner_profile"], "PARTNER-CO"; got != want {
		t.Errorf("rendered partner_profile = %q, want resolved As2Id %q — the same render path as local_profile (enriched.Fields[\"partner_profile\"]=%q)", got, want, enriched.Fields["partner_profile"])
	}
}

// newTestController builds a hermetic *app.Controller, closing it before its
// t.TempDir is removed. TestControllerConstructionDisciplineGate allows a
// direct app.New only inside such blessed helpers; the unit_test package has
// its own copy.
func newTestController(t *testing.T) *app.Controller {
	t.Helper()
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	s := session.New()
	s.Profile = "demo"
	s.Region = "us-east-1"
	core := runtime.New(s, nil)
	c := newBlessedController(t, core)
	t.Cleanup(c.Close)
	return c
}

// An ACTIVE agreement opens with no findings; enrichment then adds an
// "expired" (Broken) and an "expires in <N>d" (Warning) cert finding, both
// with Source "wave1". Both reach the open detail's Attention block, Broken
// first, through the same ApplyDetailEnrichmentForResource call
// handleEnrichDetailResult makes.
func TestTransferAgreementDetailEnrich_BothCertFindingsReachOpenDetailAttention(t *testing.T) {
	childShortName := transferAgreementsChildShortName(t)
	enrich := resource.GetDetailEnricher(childShortName)
	if enrich == nil {
		t.Fatalf("no DetailEnricher registered for transfer agreements child type %q", childShortName)
	}

	agreements := fetchTransferAgreementsForServer(t, fixtures.ProdAS2GatewayID)
	agreement := mustFindTransferResource(t, agreements, fixtures.AgreementProdPartnerID)
	if len(agreement.Findings) != 0 {
		t.Fatalf("precondition: ACTIVE agreement should start with 0 findings, got %+v", agreement.Findings)
	}

	clients := &awsclient.ServiceClients{Transfer: fakes.NewTransfer()}
	enriched, err := enrich(context.Background(), &awsclient.DetailEnrichmentCtx{Clients: clients}, agreement)
	if err != nil {
		t.Fatalf("DetailEnrich returned error: %v", err)
	}
	if len(enriched.Findings) != 2 {
		t.Fatalf("precondition: enriched agreement should carry 2 cert findings (Broken expired + Warn expiring), got %+v", enriched.Findings)
	}

	ctrl := newTestController(t)

	ctrl.ApplyIntents([]runtime.UIIntent{
		runtime.PushScreen{
			ID:      runtime.ScreenDetail,
			Context: runtime.ScreenContext{ResourceType: childShortName, ResourceID: agreement.ID},
		},
	})
	ctrl.EnsureDetailState(agreement, childShortName)

	// primaryWave2Finding yields (nil, nil) for this resource;
	// handleEnrichDetailResult passes that through.
	ctrl.ApplyDetailEnrichmentForResource(childShortName, agreement.ID, enriched, nil, nil)

	snap := ctrl.Snapshot()
	if snap.Body.Detail == nil {
		t.Fatal("Body.Detail is nil after ApplyDetailEnrichmentForResource")
	}

	var attentionText []string
	for _, f := range snap.Body.Detail.Fields {
		if f.Path == "Attention" {
			attentionText = append(attentionText, f.Value)
		}
	}
	joined := strings.ToLower(strings.Join(attentionText, " | "))

	idxExpired := strings.Index(joined, "expired")
	idxExpiring := strings.Index(joined, "expires in")

	if idxExpired == -1 {
		t.Errorf("BUG: open agreement detail's Attention block is missing the Broken %q finding for cert %q (got rows: %v) — see test doc comment for root cause", "expired", fixtures.CertExpiredID, attentionText)
	}
	if idxExpiring == -1 {
		t.Errorf("BUG: open agreement detail's Attention block is missing the Warning %q finding for cert %q (got rows: %v) — same root cause as the expired-cert failure above", "expires in <N>d", fixtures.CertExpiringID, attentionText)
	}
	if idxExpired != -1 && idxExpiring != -1 && idxExpired > idxExpiring {
		t.Errorf("Attention block must list the Broken finding before the Warning finding; got rows: %v", attentionText)
	}
}
