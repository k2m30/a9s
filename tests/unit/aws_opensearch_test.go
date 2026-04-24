package unit

// aws_opensearch_test.go — fetcher classification tests for the opensearch resource type.
//
// Tests drive FetchOpenSearchDomains with a fake two-API setup
// (ListDomainNames + DescribeDomains) and assert on Resource.Status,
// Resource.Issues, and Resource.Fields per impl-plan §1.1.

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/opensearch"
	ostypes "github.com/aws/aws-sdk-go-v2/service/opensearch/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
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
// Helper — fetchOneDomain runs FetchOpenSearchDomains for a single DomainStatus
// and returns the single resource, failing the test if count != 1.
// ---------------------------------------------------------------------------

func fetchOneDomain(t *testing.T, domain ostypes.DomainStatus) (*struct{ resources []interface{} }, interface{}) {
	t.Helper()

	domainName := ""
	if domain.DomainName != nil {
		domainName = *domain.DomainName
	}

	listMock := &mockOSListDomainNamesAPI{
		output: &opensearch.ListDomainNamesOutput{
			DomainNames: []ostypes.DomainInfo{
				{DomainName: aws.String(domainName)},
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
	return nil, resources[0]
}

// fetchOneDomainResult returns the resource from FetchOpenSearchDomains for a
// single DomainStatus. This is the main helper used by tests.
func fetchOneDomainResult(t *testing.T, domain ostypes.DomainStatus) interface{ /* resource.Resource */ } {
	t.Helper()
	_, r := fetchOneDomain(t, domain)
	return r
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

	if r.Status != "" {
		t.Errorf("Status = %q, want %q (Healthy silence — rule 1)", r.Status, "")
	}
	if r.Fields["status"] != "" {
		t.Errorf("Fields[\"status\"] = %q, want %q", r.Fields["status"], "")
	}
	if len(r.Issues) != 0 {
		t.Errorf("Issues = %v, want nil (no Wave 1, no hard-state)", r.Issues)
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
	if r.Status != wantPhrase {
		t.Errorf("Status = %q, want %q", r.Status, wantPhrase)
	}
	if !reflect.DeepEqual(r.Issues, []string{wantPhrase}) {
		t.Errorf("Issues = %v, want [%q] (U7f deep-equals)", r.Issues, wantPhrase)
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
	if r.Status != wantPhrase {
		t.Errorf("Status = %q, want %q", r.Status, wantPhrase)
	}
	if !reflect.DeepEqual(r.Issues, []string{wantPhrase}) {
		t.Errorf("Issues = %v, want [%q] (U7f deep-equals)", r.Issues, wantPhrase)
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
	if r.Status != wantPhrase {
		t.Errorf("Status = %q, want %q", r.Status, wantPhrase)
	}
	if !reflect.DeepEqual(r.Issues, []string{wantPhrase}) {
		t.Errorf("Issues = %v, want [%q] (U7f deep-equals)", r.Issues, wantPhrase)
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
	if r.Status != wantPhrase {
		t.Errorf("Status = %q, want %q", r.Status, wantPhrase)
	}
	if !reflect.DeepEqual(r.Issues, []string{wantPhrase}) {
		t.Errorf("Issues = %v, want [%q] (U7f deep-equals)", r.Issues, wantPhrase)
	}
	if r.Fields["upgrade_processing"] != "true" {
		t.Errorf("Fields[\"upgrade_processing\"] = %q, want %q", r.Fields["upgrade_processing"], "true")
	}
}

// ---------------------------------------------------------------------------
// T006 — update_available_healthy_bang: UpdateAvailable=true, past AutomatedUpdateDate
// ---------------------------------------------------------------------------

func TestOpenSearch_Fetch_UpdateAvailableHealthyBang(t *testing.T) {
	domain := osTestBaseDomain("acme-product-search")
	// AutomatedUpdateDate in the past (2026-04-20 < 2026-04-24 today per spec).
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

	resources, err := awsclient.FetchOpenSearchDomains(context.Background(), listMock, describeMock)
	if err != nil {
		t.Fatalf("FetchOpenSearchDomains error: %v", err)
	}
	r := resources[0]

	if r.Status != "software update forced soon" {
		t.Errorf("Status = %q, want %q", r.Status, "software update forced soon")
	}
	if len(r.Issues) != 0 {
		t.Errorf("Issues = %v, want nil (background-check → Finding, not Issues)", r.Issues)
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

	if r.Status != "" {
		t.Errorf("Status = %q, want %q (not yet forced — silent)", r.Status, "")
	}
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

	if r.Status != "encryption at rest off" {
		t.Errorf("Status = %q, want %q", r.Status, "encryption at rest off")
	}
	if len(r.Issues) != 0 {
		t.Errorf("Issues = %v, want nil (background signal → Finding, not Issues)", r.Issues)
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

	if r.Status != "software update forced soon (+1)" {
		t.Errorf("Status = %q, want %q (! beats ~; hidden = 1)", r.Status, "software update forced soon (+1)")
	}
	if len(r.Issues) != 0 {
		t.Errorf("Issues = %v, want nil (no hard-state; findings go in EnrichmentFinding)", r.Issues)
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

	if r.Status != "processing: config change in flight (+1)" {
		t.Errorf("Status = %q, want %q", r.Status, "processing: config change in flight (+1)")
	}
	if !reflect.DeepEqual(r.Issues, []string{"processing: config change in flight"}) {
		t.Errorf("Issues = %v, want [\"processing: config change in flight\"] (hard-state in Issues only)", r.Issues)
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

	if r.Status != "isolated: quarantined by AWS (+1)" {
		t.Errorf("Status = %q, want %q", r.Status, "isolated: quarantined by AWS (+1)")
	}
	if !reflect.DeepEqual(r.Issues, []string{"isolated: quarantined by AWS"}) {
		t.Errorf("Issues = %v, want [\"isolated: quarantined by AWS\"]", r.Issues)
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

	if r.Status != "deleting: removal in progress (+2)" {
		t.Errorf("Status = %q, want %q", r.Status, "deleting: removal in progress (+2)")
	}
	if !reflect.DeepEqual(r.Issues, []string{"deleting: removal in progress"}) {
		t.Errorf("Issues = %v, want [\"deleting: removal in progress\"]", r.Issues)
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
	// Verify no cluster_health issue is raised.
	for _, issue := range r.Issues {
		if issue == "cluster_health" || issue == "cluster health red" {
			t.Errorf("Issues contains %q — Wave 3 signals must not surface in fetcher", issue)
		}
	}
}
