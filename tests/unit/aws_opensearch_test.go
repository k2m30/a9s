package unit

// aws_opensearch_test.go — fetcher classification tests for the opensearch resource type.
//
// Tests drive FetchOpenSearchDomains with a fake two-API setup
// (ListDomainNames + DescribeDomains) and assert on Resource.Status,
// Resource.Issues, and Resource.Fields per impl-plan §1.1.

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/opensearch"
	ostypes "github.com/aws/aws-sdk-go-v2/service/opensearch/types"
	"github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	domainpkg "github.com/k2m30/a9s/v3/core/domain"
)

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

// mockOSListDomainNamesAPI returns the configured output when ListDomainNames
// is called.
type mockOSListDomainNamesAPI struct {
	output *opensearch.ListDomainNamesOutput
	err    error
}

func (m *mockOSListDomainNamesAPI) ListDomainNames(
	_ context.Context,
	_ *opensearch.ListDomainNamesInput,
	_ ...func(*opensearch.Options),
) (*opensearch.ListDomainNamesOutput, error) {
	return m.output, m.err
}

// mockOSDescribeDomainsAPI returns the configured output when DescribeDomains
// is called.
type mockOSDescribeDomainsAPI struct {
	output *opensearch.DescribeDomainsOutput
	err    error
}

func (m *mockOSDescribeDomainsAPI) DescribeDomains(
	_ context.Context,
	_ *opensearch.DescribeDomainsInput,
	_ ...func(*opensearch.Options),
) (*opensearch.DescribeDomainsOutput, error) {
	return m.output, m.err
}

// ---------------------------------------------------------------------------
// Common base domain constructor (minimal healthy domain)
// ---------------------------------------------------------------------------

func osTestBaseDomain(name string) ostypes.DomainStatus {
	return ostypes.DomainStatus{
		ARN:                    aws.String("arn:aws:es:us-east-1:123456789012:domain/" + name),
		DomainId:               aws.String("123456789012/" + name),
		DomainName:             aws.String(name),
		EngineVersion:          aws.String("OpenSearch_2.11"),
		Endpoint:               aws.String(name + ".us-east-1.es.amazonaws.com"),
		Created:                aws.Bool(true),
		Deleted:                aws.Bool(false),
		Processing:             aws.Bool(false),
		UpgradeProcessing:      aws.Bool(false),
		DomainProcessingStatus: ostypes.DomainProcessingStatusTypeActive,
		EncryptionAtRestOptions: &ostypes.EncryptionAtRestOptions{
			Enabled: aws.Bool(true),
		},
		DomainEndpointOptions: &ostypes.DomainEndpointOptions{
			EnforceHTTPS: aws.Bool(true),
		},
		ServiceSoftwareOptions: &ostypes.ServiceSoftwareOptions{
			UpdateAvailable: aws.Bool(false),
		},
		// Node-to-node encryption is read as off when the block is absent, so a
		// baseline that omitted it would give every test below a posture finding
		// it never meant to describe.
		NodeToNodeEncryptionOptions: &ostypes.NodeToNodeEncryptionOptions{
			Enabled: aws.Bool(true),
		},
	}
}

// ---------------------------------------------------------------------------
// T001 — healthy_happy_path: all signals clean
// ---------------------------------------------------------------------------

func TestOpenSearch_Fetch_HealthyHappyPath(t *testing.T) {
	domain := osTestBaseDomain("staging-analytics")

	listMock := &mockOSListDomainNamesAPI{
		output: &opensearch.ListDomainNamesOutput{
			DomainNames: []ostypes.DomainInfo{
				{DomainName: aws.String("staging-analytics")},
			},
		},
	}
	describeMock := &mockOSDescribeDomainsAPI{
		output: &opensearch.DescribeDomainsOutput{
			DomainStatusList: []ostypes.DomainStatus{domain},
		},
	}

	resources, err := awsclient.FetchOpenSearchDomains(context.Background(), listMock, describeMock)
	if err != nil {
		t.Fatalf("FetchOpenSearchDomains error: %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}
	r := resources[0]

	if r.Fields["status"] != "" {
		t.Errorf("Fields[\"status\"] = %q, want %q", r.Fields["status"], "")
	}
	if r.Fields["deleted"] != "false" {
		t.Errorf("Fields[\"deleted\"] = %q, want %q", r.Fields["deleted"], "false")
	}
	if r.Fields["processing"] != "false" {
		t.Errorf("Fields[\"processing\"] = %q, want %q", r.Fields["processing"], "false")
	}
	if r.Fields["upgrade_processing"] != "false" {
		t.Errorf("Fields[\"upgrade_processing\"] = %q, want %q", r.Fields["upgrade_processing"], "false")
	}
	if r.Fields["domain_processing_status"] != "Active" {
		t.Errorf("Fields[\"domain_processing_status\"] = %q, want %q", r.Fields["domain_processing_status"], "Active")
	}
	if r.Fields["service_software_update_available"] != "false" {
		t.Errorf("Fields[\"service_software_update_available\"] = %q, want %q", r.Fields["service_software_update_available"], "false")
	}
	if r.Fields["encryption_at_rest_enabled"] != "true" {
		t.Errorf("Fields[\"encryption_at_rest_enabled\"] = %q, want %q", r.Fields["encryption_at_rest_enabled"], "true")
	}
}

// ---------------------------------------------------------------------------
// T002 — deleted_dim: Deleted=true → deleting phrase
// ---------------------------------------------------------------------------

func TestOpenSearch_Fetch_DeletedDim(t *testing.T) {
	domain := osTestBaseDomain("obsolete-tenant-logs")
	domain.Deleted = aws.Bool(true)

	listMock := &mockOSListDomainNamesAPI{
		output: &opensearch.ListDomainNamesOutput{
			DomainNames: []ostypes.DomainInfo{{DomainName: aws.String("obsolete-tenant-logs")}},
		},
	}
	describeMock := &mockOSDescribeDomainsAPI{
		output: &opensearch.DescribeDomainsOutput{
			DomainStatusList: []ostypes.DomainStatus{domain},
		},
	}

	resources, err := awsclient.FetchOpenSearchDomains(context.Background(), listMock, describeMock)
	if err != nil {
		t.Fatalf("FetchOpenSearchDomains error: %v", err)
	}
	r := resources[0]

	const wantPhrase = "deleting: removal in progress"
	// Fetcher does not write Resource.Status — it is always "".
	if r.Fields["status"] != wantPhrase {
		t.Errorf("Fields[\"status\"] = %q, want %q", r.Fields["status"], wantPhrase)
	}
	if len(r.Findings) != 1 || r.Findings[0].Phrase != wantPhrase {
		t.Errorf("Findings = %v, want one finding with Phrase %q", r.Findings, wantPhrase)
	}
	if r.Fields["deleted"] != "true" {
		t.Errorf("Fields[\"deleted\"] = %q, want %q", r.Fields["deleted"], "true")
	}
}

// ---------------------------------------------------------------------------
// T003 — isolated_broken: DomainProcessingStatus="Isolated"
// ---------------------------------------------------------------------------

func TestOpenSearch_Fetch_IsolatedBroken(t *testing.T) {
	domain := osTestBaseDomain("legacy-search-isolated")
	domain.DomainProcessingStatus = ostypes.DomainProcessingStatusTypeIsolated

	listMock := &mockOSListDomainNamesAPI{
		output: &opensearch.ListDomainNamesOutput{
			DomainNames: []ostypes.DomainInfo{{DomainName: aws.String("legacy-search-isolated")}},
		},
	}
	describeMock := &mockOSDescribeDomainsAPI{
		output: &opensearch.DescribeDomainsOutput{
			DomainStatusList: []ostypes.DomainStatus{domain},
		},
	}

	resources, err := awsclient.FetchOpenSearchDomains(context.Background(), listMock, describeMock)
	if err != nil {
		t.Fatalf("FetchOpenSearchDomains error: %v", err)
	}
	r := resources[0]

	const wantPhrase = "isolated: quarantined by AWS"
	// Fetcher does not write Resource.Status — it is always "".
	if r.Fields["status"] != wantPhrase {
		t.Errorf("Fields[\"status\"] = %q, want %q", r.Fields["status"], wantPhrase)
	}
	if len(r.Findings) != 1 || r.Findings[0].Phrase != wantPhrase {
		t.Errorf("Findings = %v, want one finding with Phrase %q", r.Findings, wantPhrase)
	}
	if r.Fields["domain_processing_status"] != "Isolated" {
		t.Errorf("Fields[\"domain_processing_status\"] = %q, want %q", r.Fields["domain_processing_status"], "Isolated")
	}
}

// ---------------------------------------------------------------------------
// T004 — processing_warning: Processing=true
// ---------------------------------------------------------------------------

func TestOpenSearch_Fetch_ProcessingWarning(t *testing.T) {
	domain := osTestBaseDomain("acme-events")
	domain.Processing = aws.Bool(true)
	domain.DomainProcessingStatus = ostypes.DomainProcessingStatusTypeModifying

	listMock := &mockOSListDomainNamesAPI{
		output: &opensearch.ListDomainNamesOutput{
			DomainNames: []ostypes.DomainInfo{{DomainName: aws.String("acme-events")}},
		},
	}
	describeMock := &mockOSDescribeDomainsAPI{
		output: &opensearch.DescribeDomainsOutput{
			DomainStatusList: []ostypes.DomainStatus{domain},
		},
	}

	resources, err := awsclient.FetchOpenSearchDomains(context.Background(), listMock, describeMock)
	if err != nil {
		t.Fatalf("FetchOpenSearchDomains error: %v", err)
	}
	r := resources[0]

	const wantPhrase = "processing: config change in flight"
	// Fetcher does not write Resource.Status — it is always "".
	if r.Fields["status"] != wantPhrase {
		t.Errorf("Fields[\"status\"] = %q, want %q", r.Fields["status"], wantPhrase)
	}
	if len(r.Findings) != 1 || r.Findings[0].Phrase != wantPhrase {
		t.Errorf("Findings = %v, want one finding with Phrase %q", r.Findings, wantPhrase)
	}
	if r.Fields["processing"] != "true" {
		t.Errorf("Fields[\"processing\"] = %q, want %q", r.Fields["processing"], "true")
	}
}

// ---------------------------------------------------------------------------
// T005 — upgrade_processing_warning: UpgradeProcessing=true
// ---------------------------------------------------------------------------

func TestOpenSearch_Fetch_UpgradeProcessingWarning(t *testing.T) {
	domain := osTestBaseDomain("acme-search-alpha")
	domain.UpgradeProcessing = aws.Bool(true)
	domain.DomainProcessingStatus = ostypes.DomainProcessingStatusTypeUpgrading

	listMock := &mockOSListDomainNamesAPI{
		output: &opensearch.ListDomainNamesOutput{
			DomainNames: []ostypes.DomainInfo{{DomainName: aws.String("acme-search-alpha")}},
		},
	}
	describeMock := &mockOSDescribeDomainsAPI{
		output: &opensearch.DescribeDomainsOutput{
			DomainStatusList: []ostypes.DomainStatus{domain},
		},
	}

	resources, err := awsclient.FetchOpenSearchDomains(context.Background(), listMock, describeMock)
	if err != nil {
		t.Fatalf("FetchOpenSearchDomains error: %v", err)
	}
	r := resources[0]

	const wantPhrase = "processing: config change in flight"
	// Fetcher does not write Resource.Status — it is always "".
	if r.Fields["status"] != wantPhrase {
		t.Errorf("Fields[\"status\"] = %q, want %q", r.Fields["status"], wantPhrase)
	}
	if len(r.Findings) != 1 || r.Findings[0].Phrase != wantPhrase {
		t.Errorf("Findings = %v, want one finding with Phrase %q", r.Findings, wantPhrase)
	}
	if r.Fields["upgrade_processing"] != "true" {
		t.Errorf("Fields[\"upgrade_processing\"] = %q, want %q", r.Fields["upgrade_processing"], "true")
	}
}

// ---------------------------------------------------------------------------
// T006 — update_available_healthy_bang: UpdateAvailable=true, past AutomatedUpdateDate
// ---------------------------------------------------------------------------

// The update-forced and encryption-off signals were an enricher's until d1 moved
// them to wave 1; aws_opensearch_issue_enrichment_test.go covered them there and
// is deleted, because these fetcher tests already assert the same behaviour on
// the surface that now produces it.
func TestOpenSearch_Fetch_UpdateAvailableHealthyBang(t *testing.T) {
	domain := osTestBaseDomain("acme-product-search")
	// AutomatedUpdateDate in the past relative to the injected now below.
	domain.ServiceSoftwareOptions = &ostypes.ServiceSoftwareOptions{
		UpdateAvailable:     aws.Bool(true),
		AutomatedUpdateDate: aws.Time(time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC)),
		CurrentVersion:      aws.String("OpenSearch_2.11"),
		NewVersion:          aws.String("OpenSearch_2.13"),
	}

	listMock := &mockOSListDomainNamesAPI{
		output: &opensearch.ListDomainNamesOutput{
			DomainNames: []ostypes.DomainInfo{{DomainName: aws.String("acme-product-search")}},
		},
	}
	describeMock := &mockOSDescribeDomainsAPI{
		output: &opensearch.DescribeDomainsOutput{
			DomainStatusList: []ostypes.DomainStatus{domain},
		},
	}

	// Inject a fixed "now" — 2026-04-24 — so the "past AutomatedUpdateDate" branch
	// is exercised deterministically regardless of the real wall clock.
	fixedNow := time.Date(2026, 4, 24, 0, 0, 0, 0, time.UTC)
	resources, err := awsclient.FetchOpenSearchDomainsAt(context.Background(), listMock, describeMock, fixedNow)
	if err != nil {
		t.Fatalf("FetchOpenSearchDomains error: %v", err)
	}
	r := resources[0]

	// Fetcher does not write Resource.Status — it is always "".
	if r.Fields["status"] != "software update forced soon" {
		t.Errorf("Fields[\"status\"] = %q, want %q", r.Fields["status"], "software update forced soon")
	}
	// The phrase in the status column and the finding that produced it are one
	// object. An assertion that the column shows a signal the row carries no
	// finding for describes the split this batch removed.
	if len(r.Findings) != 1 || r.Findings[0].Code != "opensearch.update-forced" {
		t.Errorf("Findings = %+v, want exactly the wave-1 update-forced finding", r.Findings)
	}
	if r.Fields["service_software_update_available"] != "true" {
		t.Errorf("Fields[\"service_software_update_available\"] = %q, want %q", r.Fields["service_software_update_available"], "true")
	}
}

// ---------------------------------------------------------------------------
// T007 — update_available_future_date_silent: UpdateAvailable=true, future date
// ---------------------------------------------------------------------------

func TestOpenSearch_Fetch_UpdateAvailableFutureDateSilent(t *testing.T) {
	domain := osTestBaseDomain("acme-product-search-future")
	// AutomatedUpdateDate in the future.
	domain.ServiceSoftwareOptions = &ostypes.ServiceSoftwareOptions{
		UpdateAvailable:     aws.Bool(true),
		AutomatedUpdateDate: aws.Time(time.Now().Add(48 * time.Hour)),
		CurrentVersion:      aws.String("OpenSearch_2.11"),
		NewVersion:          aws.String("OpenSearch_2.13"),
	}

	listMock := &mockOSListDomainNamesAPI{
		output: &opensearch.ListDomainNamesOutput{
			DomainNames: []ostypes.DomainInfo{{DomainName: aws.String("acme-product-search-future")}},
		},
	}
	describeMock := &mockOSDescribeDomainsAPI{
		output: &opensearch.DescribeDomainsOutput{
			DomainStatusList: []ostypes.DomainStatus{domain},
		},
	}

	resources, err := awsclient.FetchOpenSearchDomains(context.Background(), listMock, describeMock)
	if err != nil {
		t.Fatalf("FetchOpenSearchDomains error: %v", err)
	}
	r := resources[0]

	if r.Fields["service_software_update_available"] != "false" {
		t.Errorf("Fields[\"service_software_update_available\"] = %q, want %q", r.Fields["service_software_update_available"], "false")
	}
}

// ---------------------------------------------------------------------------
// T008 — encryption_off_healthy_tilde: EncryptionAtRestOptions.Enabled=false
// ---------------------------------------------------------------------------

func TestOpenSearch_Fetch_EncryptionOffHealthyTilde(t *testing.T) {
	domain := osTestBaseDomain("legacy-analytics")
	domain.EncryptionAtRestOptions = &ostypes.EncryptionAtRestOptions{
		Enabled: aws.Bool(false),
	}

	listMock := &mockOSListDomainNamesAPI{
		output: &opensearch.ListDomainNamesOutput{
			DomainNames: []ostypes.DomainInfo{{DomainName: aws.String("legacy-analytics")}},
		},
	}
	describeMock := &mockOSDescribeDomainsAPI{
		output: &opensearch.DescribeDomainsOutput{
			DomainStatusList: []ostypes.DomainStatus{domain},
		},
	}

	resources, err := awsclient.FetchOpenSearchDomains(context.Background(), listMock, describeMock)
	if err != nil {
		t.Fatalf("FetchOpenSearchDomains error: %v", err)
	}
	r := resources[0]

	// Fetcher does not write Resource.Status — it is always "".
	if r.Fields["status"] != "encryption at rest off" {
		t.Errorf("Fields[\"status\"] = %q, want %q", r.Fields["status"], "encryption at rest off")
	}
	// Same reason as the update-forced row above: the status column reads from
	// the findings, so a displayed phrase without a finding is not a state the
	// fetcher can produce.
	if len(r.Findings) != 1 || r.Findings[0].Code != "opensearch.encryption-off" {
		t.Errorf("Findings = %+v, want exactly the wave-1 encryption-off finding", r.Findings)
	}
	if r.Fields["encryption_at_rest_enabled"] != "false" {
		t.Errorf("Fields[\"encryption_at_rest_enabled\"] = %q, want %q", r.Fields["encryption_at_rest_enabled"], "false")
	}
}

// ---------------------------------------------------------------------------
// T009 — multi_w2_update_plus_encryption_suffix (U7a/U7d analog — multi-W2 stacking)
// UpdateAvailable (past) AND EncryptionOff → Status suffix (+1)
// ---------------------------------------------------------------------------

func TestOpenSearch_Fetch_MultiW2UpdatePlusEncryptionSuffix(t *testing.T) {
	domain := osTestBaseDomain("acme-metrics")
	domain.ServiceSoftwareOptions = &ostypes.ServiceSoftwareOptions{
		UpdateAvailable:     aws.Bool(true),
		AutomatedUpdateDate: aws.Time(time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC)),
		CurrentVersion:      aws.String("OpenSearch_2.11"),
		NewVersion:          aws.String("OpenSearch_2.13"),
	}
	domain.EncryptionAtRestOptions = &ostypes.EncryptionAtRestOptions{
		Enabled: aws.Bool(false),
	}

	listMock := &mockOSListDomainNamesAPI{
		output: &opensearch.ListDomainNamesOutput{
			DomainNames: []ostypes.DomainInfo{{DomainName: aws.String("acme-metrics")}},
		},
	}
	describeMock := &mockOSDescribeDomainsAPI{
		output: &opensearch.DescribeDomainsOutput{
			DomainStatusList: []ostypes.DomainStatus{domain},
		},
	}

	resources, err := awsclient.FetchOpenSearchDomains(context.Background(), listMock, describeMock)
	if err != nil {
		t.Fatalf("FetchOpenSearchDomains error: %v", err)
	}
	r := resources[0]

	// Both signals read the DescribeDomains response the fetcher already holds
	// and make no AWS call, so they are wave-1 findings on the row rather than
	// enricher output. The status cell is built from those findings.
	if r.Fields["status"] != "software update forced soon (+1)" {
		t.Errorf("Fields[\"status\"] = %q, want %q", r.Fields["status"], "software update forced soon (+1)")
	}

	// Each independently-evaluated condition keeps its own Finding with its own
	// S5 sentence (owner contract #52) rather than one being demoted into a
	// supporting row of the other.
	byCode := map[domainpkg.FindingCode]domainpkg.Finding{}
	for _, f := range r.Findings {
		byCode[f.Code] = f
	}
	if len(r.Findings) != 2 {
		t.Fatalf("Findings = %+v, want the update-forced and encryption-off findings", r.Findings)
	}
	for _, want := range []struct {
		code     domainpkg.FindingCode
		phrase   string
		severity domainpkg.Severity
		detail   string
	}{
		{
			code:     "opensearch.update-forced",
			phrase:   "software update forced soon",
			severity: domainpkg.SevWarn,
			detail:   "AWS will apply this service software update itself once the scheduled date passes, taking whatever maintenance window it chooses. Apply it yourself before that date so the blue/green deployment lands at a time you picked.",
		},
		{
			code:     "opensearch.encryption-off",
			phrase:   "encryption at rest off",
			severity: domainpkg.SevWarn,
			detail:   "Data at rest is stored unencrypted. Enabling encryption at rest requires creating a new domain and migrating data — it cannot be turned on in place.",
		},
	} {
		got, ok := byCode[want.code]
		if !ok {
			t.Errorf("no finding with code %q; got %+v", want.code, r.Findings)
			continue
		}
		if got.Phrase != want.phrase {
			t.Errorf("%s: Phrase = %q, want %q", want.code, got.Phrase, want.phrase)
		}
		if got.Severity != want.severity {
			t.Errorf("%s: Severity = %v, want %v", want.code, got.Severity, want.severity)
		}
		if got.Detail != want.detail {
			t.Errorf("%s: Detail = %q, want its own S5 sentence %q", want.code, got.Detail, want.detail)
		}
	}
	for _, ad := range r.AttentionDetails {
		for _, row := range ad.Rows {
			if row.Label == "Additional" {
				t.Errorf("row {Label:%q Value:%q}: a generic \"Additional\" row means the second condition was demoted into a row instead of surfacing as its own finding",
					row.Label, row.Value)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// T010 — hardstate_plus_background_suffix (U7b analog — hard-state + background)
// Processing=true AND UpdateAvailable (past) → Status suffix (+1)
// ---------------------------------------------------------------------------

func TestOpenSearch_Fetch_HardStatePlusBackgroundSuffix(t *testing.T) {
	domain := osTestBaseDomain("acme-search-alpha-processing")
	domain.Processing = aws.Bool(true)
	domain.ServiceSoftwareOptions = &ostypes.ServiceSoftwareOptions{
		UpdateAvailable:     aws.Bool(true),
		AutomatedUpdateDate: aws.Time(time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC)),
		CurrentVersion:      aws.String("OpenSearch_2.11"),
		NewVersion:          aws.String("OpenSearch_2.13"),
	}

	listMock := &mockOSListDomainNamesAPI{
		output: &opensearch.ListDomainNamesOutput{
			DomainNames: []ostypes.DomainInfo{{DomainName: aws.String("acme-search-alpha-processing")}},
		},
	}
	describeMock := &mockOSDescribeDomainsAPI{
		output: &opensearch.DescribeDomainsOutput{
			DomainStatusList: []ostypes.DomainStatus{domain},
		},
	}

	resources, err := awsclient.FetchOpenSearchDomains(context.Background(), listMock, describeMock)
	if err != nil {
		t.Fatalf("FetchOpenSearchDomains error: %v", err)
	}
	r := resources[0]

	// Fetcher does not write Resource.Status — it is always "".
	// Display: processing (hard-state) + software update (background) → suffix (+1).
	if r.Fields["status"] != "processing: config change in flight (+1)" {
		t.Errorf("Fields[\"status\"] = %q, want %q", r.Fields["status"], "processing: config change in flight (+1)")
	}
	// Both signals are findings. The "(+1)" in the column counts the second one,
	// so a row that displayed the suffix while carrying one finding was counting
	// something it could not show.
	if len(r.Findings) != 2 {
		t.Errorf("Findings len = %d, want 2 (processing and update-forced); Findings = %+v", len(r.Findings), r.Findings)
	}
	if len(r.Findings) > 0 && r.Findings[0].Phrase != "processing: config change in flight" {
		t.Errorf("Findings[0].Phrase = %q, want %q", r.Findings[0].Phrase, "processing: config change in flight")
	}
}

// ---------------------------------------------------------------------------
// T011 — isolated_plus_encryption_off
// ---------------------------------------------------------------------------

func TestOpenSearch_Fetch_IsolatedPlusEncryptionOff(t *testing.T) {
	domain := osTestBaseDomain("legacy-search-isolated-enc-off")
	domain.DomainProcessingStatus = ostypes.DomainProcessingStatusTypeIsolated
	domain.EncryptionAtRestOptions = &ostypes.EncryptionAtRestOptions{
		Enabled: aws.Bool(false),
	}

	listMock := &mockOSListDomainNamesAPI{
		output: &opensearch.ListDomainNamesOutput{
			DomainNames: []ostypes.DomainInfo{{DomainName: aws.String("legacy-search-isolated-enc-off")}},
		},
	}
	describeMock := &mockOSDescribeDomainsAPI{
		output: &opensearch.DescribeDomainsOutput{
			DomainStatusList: []ostypes.DomainStatus{domain},
		},
	}

	resources, err := awsclient.FetchOpenSearchDomains(context.Background(), listMock, describeMock)
	if err != nil {
		t.Fatalf("FetchOpenSearchDomains error: %v", err)
	}
	r := resources[0]

	// Fetcher does not write Resource.Status — it is always "".
	// Display: isolated (hard-state) + encryption-off (background) → suffix (+1).
	if r.Fields["status"] != "isolated: quarantined by AWS (+1)" {
		t.Errorf("Fields[\"status\"] = %q, want %q", r.Fields["status"], "isolated: quarantined by AWS (+1)")
	}
	// Only the hard-state (isolated) is in Findings; EncryptionOff is enricher territory.
	// Same as the processing row: the second signal is a finding of its own, and
	// isolated leads because it is the worse of the two.
	if len(r.Findings) != 2 {
		t.Errorf("Findings len = %d, want 2 (isolated and encryption-off); Findings = %+v", len(r.Findings), r.Findings)
	}
	if len(r.Findings) > 0 && r.Findings[0].Phrase != "isolated: quarantined by AWS" {
		t.Errorf("Findings[0].Phrase = %q, want %q", r.Findings[0].Phrase, "isolated: quarantined by AWS")
	}
}

// ---------------------------------------------------------------------------
// T012 — deleted_plus_background_background_suppressed
// Deleted=true AND EncryptionOff AND UpdateAvailable (past) → Status suffix (+2)
// ---------------------------------------------------------------------------

func TestOpenSearch_Fetch_DeletedPlusBackgroundBackgroundSuppressed(t *testing.T) {
	domain := osTestBaseDomain("obsolete-tenant-logs-multi")
	domain.Deleted = aws.Bool(true)
	domain.EncryptionAtRestOptions = &ostypes.EncryptionAtRestOptions{
		Enabled: aws.Bool(false),
	}
	domain.ServiceSoftwareOptions = &ostypes.ServiceSoftwareOptions{
		UpdateAvailable:     aws.Bool(true),
		AutomatedUpdateDate: aws.Time(time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC)),
		CurrentVersion:      aws.String("OpenSearch_2.11"),
		NewVersion:          aws.String("OpenSearch_2.13"),
	}

	listMock := &mockOSListDomainNamesAPI{
		output: &opensearch.ListDomainNamesOutput{
			DomainNames: []ostypes.DomainInfo{{DomainName: aws.String("obsolete-tenant-logs-multi")}},
		},
	}
	describeMock := &mockOSDescribeDomainsAPI{
		output: &opensearch.DescribeDomainsOutput{
			DomainStatusList: []ostypes.DomainStatus{domain},
		},
	}

	resources, err := awsclient.FetchOpenSearchDomains(context.Background(), listMock, describeMock)
	if err != nil {
		t.Fatalf("FetchOpenSearchDomains error: %v", err)
	}
	r := resources[0]

	// Fetcher does not write Resource.Status — it is always "".
	// A domain being torn down returns the dim lifecycle finding and stops, so
	// the posture signals are never evaluated. There is no second finding to
	// count and the column carries the bare phrase; the old "(+2)" expected a
	// count of signals the row deliberately does not report.
	if r.Fields["status"] != "deleting: removal in progress" {
		t.Errorf("Fields[\"status\"] = %q, want %q", r.Fields["status"], "deleting: removal in progress")
	}
	if len(r.Findings) != 1 {
		t.Errorf("Findings len = %d, want 1 (only deleting); Findings = %+v", len(r.Findings), r.Findings)
	}
	if len(r.Findings) > 0 && r.Findings[0].Phrase != "deleting: removal in progress" {
		t.Errorf("Findings[0].Phrase = %q, want %q", r.Findings[0].Phrase, "deleting: removal in progress")
	}
}

// ---------------------------------------------------------------------------
// T013 — anti_cluster_health_red_is_oos (Wave 3 anti-test)
// No cluster_health finding should surface from the Wave 1/2 fetcher.
// ---------------------------------------------------------------------------

func TestOpenSearch_Fetch_Wave3ClusterHealthIsOutOfScope(t *testing.T) {
	domain := osTestBaseDomain("staging-analytics-cw")

	listMock := &mockOSListDomainNamesAPI{
		output: &opensearch.ListDomainNamesOutput{
			DomainNames: []ostypes.DomainInfo{{DomainName: aws.String("staging-analytics-cw")}},
		},
	}
	describeMock := &mockOSDescribeDomainsAPI{
		output: &opensearch.DescribeDomainsOutput{
			DomainStatusList: []ostypes.DomainStatus{domain},
		},
	}

	resources, err := awsclient.FetchOpenSearchDomains(context.Background(), listMock, describeMock)
	if err != nil {
		t.Fatalf("FetchOpenSearchDomains error: %v", err)
	}
	r := resources[0]

	// Wave 3 metric fields must NOT be populated by the fetcher.
	forbiddenKeys := []string{"cluster_health", "cluster_status_red", "cluster_status_yellow", "free_storage_space", "jvm_memory_pressure"}
	for _, key := range forbiddenKeys {
		if val, ok := r.Fields[key]; ok {
			t.Errorf("Fields[%q] = %q should not exist — Wave 3 CloudWatch metrics are out of scope", key, val)
		}
	}
	// Verify no cluster_health finding is raised.
	for _, f := range r.Findings {
		if f.Phrase == "cluster_health" || f.Phrase == "cluster health red" {
			t.Errorf("Findings contains %q — Wave 3 signals must not surface in fetcher", f.Phrase)
		}
	}
}

// ---------------------------------------------------------------------------
// E5 partial success — DescribeDomains ITSELF errors (e.g. es:DescribeDomains
// denied). ListDomainNames succeeded, so every domain it named is a real,
// listed domain; a batch-level DescribeDomains failure must not make them all
// vanish. FetchOpenSearchDomainsAt already has a per-name degraded-row pass
// for domains individually missing from a (successful) DescribeDomains
// response (opensearch.go's "described[name]" loop) — this pins the sibling
// case where the DescribeDomains call fails outright, which today short-
// circuits via an early `return nil, err` before that pass ever runs.
// ---------------------------------------------------------------------------

func TestOpenSearch_Fetch_DescribeDomainsBatchError_KeepsListedDomainsAsDegradedRows(t *testing.T) {
	listMock := &mockOSListDomainNamesAPI{
		output: &opensearch.ListDomainNamesOutput{
			DomainNames: []ostypes.DomainInfo{
				{DomainName: aws.String("prod-search-a")},
				{DomainName: aws.String("prod-search-b")},
			},
		},
	}
	describeMock := &mockOSDescribeDomainsAPI{
		// A real es:DescribeDomains denial is a typed AccessDenied API error;
		// the shared DegradedDetails classifier renders "details denied" for it
		// (a plain non-typed error would instead render "details unavailable").
		err: &smithy.GenericAPIError{Code: "AccessDeniedException", Message: "User is not authorized to perform: es:DescribeDomains"},
	}

	resources, err := awsclient.FetchOpenSearchDomains(context.Background(), listMock, describeMock)
	if err == nil {
		t.Fatal("expected a non-nil composite error when the DescribeDomains batch call itself fails, got nil")
	}

	if len(resources) != 2 {
		t.Fatalf("BUG: DescribeDomains batch error loses every listed domain instead of degrading each to a details-denied row — got %d resources, want 2 (opensearch.go's `if err != nil { return nil, fmt.Errorf(...) }` on the DescribeDomains call returns before the existing per-name degraded-row pass ever runs, so an es:DescribeDomains denial makes the whole account's OpenSearch list vanish instead of showing degraded rows)", len(resources))
	}

	wantIDs := map[string]bool{"prod-search-a": true, "prod-search-b": true}
	for _, r := range resources {
		if !wantIDs[r.ID] {
			t.Errorf("unexpected resource ID %q in degraded result, want one of %v", r.ID, []string{"prod-search-a", "prod-search-b"})
			continue
		}
		delete(wantIDs, r.ID)

		if r.Fields["status"] != "details denied" {
			t.Errorf("domain %q: Fields[\"status\"] = %q, want %q", r.ID, r.Fields["status"], "details denied")
		}
		if len(r.Findings) != 1 {
			t.Fatalf("domain %q: Findings = %+v, want exactly 1 details-denied finding", r.ID, r.Findings)
		}
		wantCode := awsclient.DetailsDeniedCode("opensearch")
		if r.Findings[0].Code != wantCode {
			t.Errorf("domain %q: Findings[0].Code = %q, want %q", r.ID, r.Findings[0].Code, wantCode)
		}
		if r.Findings[0].Severity != domainpkg.SevWarn {
			t.Errorf("domain %q: Findings[0].Severity = %v, want SevWarn", r.ID, r.Findings[0].Severity)
		}
	}
	if len(wantIDs) != 0 {
		t.Errorf("missing degraded rows for listed domains: %v", wantIDs)
	}
}
