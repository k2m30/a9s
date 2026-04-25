package unit

// aws_opensearch_issue_enrichment_test.go — Wave-2 enricher tests for the
// opensearch resource type.
//
// Tests drive aws.EnrichOpenSearchDomains (the opensearch-specific Wave 2
// enricher) and assert on IssueEnricherResult per impl-plan §1.3.
//
// Critical contract assertions (U11):
//   - Summary == "<short phrase>" (e.g. "software update forced soon")
//   - !strings.Contains(Summary, row.Value) for every row
//   - Enricher must NOT overwrite Status on resources with a hard-state Status

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	ostypes "github.com/aws/aws-sdk-go-v2/service/opensearch/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// buildOSResource builds a minimal resource.Resource from an ostypes.DomainStatus.
// The Fields map is populated with the values the fetcher would set.
func buildOSResource(domain ostypes.DomainStatus, statusOverride string) resource.Resource {
	name := ""
	if domain.DomainName != nil {
		name = *domain.DomainName
	}

	updateAvailable := "false"
	if domain.ServiceSoftwareOptions != nil &&
		domain.ServiceSoftwareOptions.UpdateAvailable != nil &&
		*domain.ServiceSoftwareOptions.UpdateAvailable {
		// Only "true" if past the AutomatedUpdateDate.
		if domain.ServiceSoftwareOptions.AutomatedUpdateDate != nil &&
			domain.ServiceSoftwareOptions.AutomatedUpdateDate.Before(time.Now()) {
			updateAvailable = "true"
		}
	}

	encryptionEnabled := "true"
	if domain.EncryptionAtRestOptions != nil &&
		domain.EncryptionAtRestOptions.Enabled != nil &&
		!*domain.EncryptionAtRestOptions.Enabled {
		encryptionEnabled = "false"
	}

	fields := map[string]string{
		"status":                            statusOverride,
		"service_software_update_available": updateAvailable,
		"encryption_at_rest_enabled":        encryptionEnabled,
	}

	if domain.ServiceSoftwareOptions != nil {
		if domain.ServiceSoftwareOptions.CurrentVersion != nil {
			fields["current_version"] = *domain.ServiceSoftwareOptions.CurrentVersion
		}
		if domain.ServiceSoftwareOptions.AutomatedUpdateDate != nil {
			fields["automated_update_date"] = domain.ServiceSoftwareOptions.AutomatedUpdateDate.Format("2006-01-02")
		}
	}

	return resource.Resource{
		ID:        name,
		Name:      name,
		Status:    statusOverride,
		Fields:    fields,
		RawStruct: domain,
	}
}

// u11SummaryRowCheck asserts the U11 contract: Summary must not contain any
// row value. Fails the test if violated.
func u11SummaryRowCheck(t *testing.T, finding resource.EnrichmentFinding) {
	t.Helper()
	for _, row := range finding.Rows {
		if strings.Contains(finding.Summary, row.Value) {
			t.Errorf("U11 violation: Summary %q contains row.Value %q — facts must live in Rows only", finding.Summary, row.Value)
		}
	}
}

// ---------------------------------------------------------------------------
// Test 1 — enricher_healthy_with_update_available_emits_bang_finding
// ---------------------------------------------------------------------------

func TestOpenSearch_Enrich_UpdateAvailable_EmitsBangFinding(t *testing.T) {
	fix := fixtures.NewOpenSearchFixtures()

	// Find the UpdateAvailableDomain fixture.
	var updateDomain ostypes.DomainStatus
	for _, d := range fix.Domains {
		if d.DomainName != nil && *d.DomainName == fixtures.UpdateAvailableDomain {
			updateDomain = d
			break
		}
	}
	if updateDomain.DomainName == nil {
		t.Fatalf("UpdateAvailableDomain fixture not found")
	}

	resources := []resource.Resource{
		buildOSResource(updateDomain, "software update forced soon"),
	}

	result, err := awsclient.EnrichOpenSearchDomains(context.Background(), nil, resources, nil)
	if err != nil {
		t.Fatalf("EnrichOpenSearchDomains error: %v", err)
	}

	id := fixtures.UpdateAvailableDomain
	finding, ok := result.Findings[id]
	if !ok {
		t.Fatalf("no Finding for resource %q", id)
	}

	if finding.Severity != "!" {
		t.Errorf("Severity = %q, want %q", finding.Severity, "!")
	}
	if finding.Summary != "software update forced soon" {
		t.Errorf("Summary = %q, want %q", finding.Summary, "software update forced soon")
	}

	// U11 — Summary must not contain any row value.
	u11SummaryRowCheck(t, finding)

	if result.IssueCount != 1 {
		t.Errorf("IssueCount = %d, want 1 (! bumps menu badge)", result.IssueCount)
	}

	// Verify rows contain "Automated Update" and "Current Version".
	hasAutomatedUpdate := false
	hasCurrentVersion := false
	for _, row := range finding.Rows {
		if row.Label == "Automated Update" {
			hasAutomatedUpdate = true
		}
		if row.Label == "Current Version" {
			hasCurrentVersion = true
		}
	}
	if !hasAutomatedUpdate {
		t.Errorf("Rows missing {Label:\"Automated Update\", ...}; got: %v", finding.Rows)
	}
	if !hasCurrentVersion {
		t.Errorf("Rows missing {Label:\"Current Version\", ...}; got: %v", finding.Rows)
	}
}

// ---------------------------------------------------------------------------
// Test 2 — enricher_healthy_with_encryption_off_emits_tilde_finding
// ---------------------------------------------------------------------------

func TestOpenSearch_Enrich_EncryptionOff_EmitsTildeFinding(t *testing.T) {
	fix := fixtures.NewOpenSearchFixtures()

	var encOffDomain ostypes.DomainStatus
	for _, d := range fix.Domains {
		if d.DomainName != nil && *d.DomainName == fixtures.EncryptionOffDomain {
			encOffDomain = d
			break
		}
	}
	if encOffDomain.DomainName == nil {
		t.Fatalf("EncryptionOffDomain fixture not found")
	}

	resources := []resource.Resource{
		buildOSResource(encOffDomain, "encryption at rest off"),
	}

	result, err := awsclient.EnrichOpenSearchDomains(context.Background(), nil, resources, nil)
	if err != nil {
		t.Fatalf("EnrichOpenSearchDomains error: %v", err)
	}

	id := fixtures.EncryptionOffDomain
	finding, ok := result.Findings[id]
	if !ok {
		t.Fatalf("no Finding for resource %q", id)
	}

	if finding.Severity != "~" {
		t.Errorf("Severity = %q, want %q", finding.Severity, "~")
	}
	if finding.Summary != "encryption at rest off" {
		t.Errorf("Summary = %q, want %q", finding.Summary, "encryption at rest off")
	}

	// U11 — Summary must not contain any row value.
	u11SummaryRowCheck(t, finding)

	if result.IssueCount != 0 {
		t.Errorf("IssueCount = %d, want 0 (~ never bumps badge)", result.IssueCount)
	}
}

// ---------------------------------------------------------------------------
// Test 3 — enricher_multi_background_top_wins_hidden_surfaces_as_row
// UpdateAvailable (!) + EncryptionOff (~) → ! wins, ~ surfaces as Additional row
// ---------------------------------------------------------------------------

func TestOpenSearch_Enrich_MultiBackground_TopWinsHiddenSurfacesAsRow(t *testing.T) {
	fix := fixtures.NewOpenSearchFixtures()

	var multiDomain ostypes.DomainStatus
	for _, d := range fix.Domains {
		if d.DomainName != nil && *d.DomainName == fixtures.MultiBackgroundDomain {
			multiDomain = d
			break
		}
	}
	if multiDomain.DomainName == nil {
		t.Fatalf("MultiBackgroundDomain fixture not found")
	}

	resources := []resource.Resource{
		buildOSResource(multiDomain, "software update forced soon (+1)"),
	}

	result, err := awsclient.EnrichOpenSearchDomains(context.Background(), nil, resources, nil)
	if err != nil {
		t.Fatalf("EnrichOpenSearchDomains error: %v", err)
	}

	id := fixtures.MultiBackgroundDomain
	finding, ok := result.Findings[id]
	if !ok {
		t.Fatalf("no Finding for resource %q", id)
	}

	// ! beats ~
	if finding.Severity != "!" {
		t.Errorf("Severity = %q, want %q (! beats ~)", finding.Severity, "!")
	}
	if finding.Summary != "software update forced soon" {
		t.Errorf("Summary = %q, want %q", finding.Summary, "software update forced soon")
	}

	// Hidden ~ surfaces as Additional row.
	hasAdditional := false
	for _, row := range finding.Rows {
		if row.Label == "Additional" && row.Value == "encryption at rest off" {
			hasAdditional = true
		}
	}
	if !hasAdditional {
		t.Errorf("Rows missing {Label:\"Additional\", Value:\"encryption at rest off\"}; got: %v", finding.Rows)
	}

	// U11 — Summary must not contain any row value.
	u11SummaryRowCheck(t, finding)

	if result.IssueCount != 1 {
		t.Errorf("IssueCount = %d, want 1 (multi still counts as 1 instance)", result.IssueCount)
	}
}

// ---------------------------------------------------------------------------
// Test 4 — enricher_hardstate_plus_background_no_field_update
// Fetcher is authoritative for Status — enricher must NOT overwrite Status
// ---------------------------------------------------------------------------

func TestOpenSearch_Enrich_HardStatePlusBackground_NoFieldUpdate(t *testing.T) {
	fix := fixtures.NewOpenSearchFixtures()

	var procPlusUpdateDomain ostypes.DomainStatus
	for _, d := range fix.Domains {
		if d.DomainName != nil && *d.DomainName == fixtures.ProcessingPlusUpdateDomain {
			procPlusUpdateDomain = d
			break
		}
	}
	if procPlusUpdateDomain.DomainName == nil {
		t.Fatalf("ProcessingPlusUpdateDomain fixture not found")
	}

	// Simulate fetcher having already set the hard-state Status with stacked suffix.
	resources := []resource.Resource{
		buildOSResource(procPlusUpdateDomain, "processing: config change in flight (+1)"),
	}

	result, err := awsclient.EnrichOpenSearchDomains(context.Background(), nil, resources, nil)
	if err != nil {
		t.Fatalf("EnrichOpenSearchDomains error: %v", err)
	}

	id := fixtures.ProcessingPlusUpdateDomain
	finding, ok := result.Findings[id]
	if !ok {
		t.Fatalf("no Finding for resource %q", id)
	}

	if finding.Severity != "!" {
		t.Errorf("Severity = %q, want %q", finding.Severity, "!")
	}
	if finding.Summary != "software update forced soon" {
		t.Errorf("Summary = %q, want %q", finding.Summary, "software update forced soon")
	}

	// Enricher must NOT overwrite Status (fetcher is authoritative for opensearch).
	if result.FieldUpdates != nil {
		if updates, ok := result.FieldUpdates[id]; ok && updates != nil {
			if _, hasStatus := updates["status"]; hasStatus {
				t.Errorf("FieldUpdates[%q][\"status\"] is set — enricher must not overwrite Status for opensearch", id)
			}
		}
	}

	if result.IssueCount != 1 {
		t.Errorf("IssueCount = %d, want 1", result.IssueCount)
	}
}

// ---------------------------------------------------------------------------
// Test 5 — enricher_no_signals_no_finding
// Healthy resource with no flags → no Finding
// ---------------------------------------------------------------------------

func TestOpenSearch_Enrich_NoSignals_NoFinding(t *testing.T) {
	fix := fixtures.NewOpenSearchFixtures()

	var healthyDomain ostypes.DomainStatus
	for _, d := range fix.Domains {
		if d.DomainName != nil && *d.DomainName == fixtures.HealthyBaselineDomain {
			healthyDomain = d
			break
		}
	}
	if healthyDomain.DomainName == nil {
		t.Fatalf("HealthyBaselineDomain fixture not found")
	}

	resources := []resource.Resource{
		buildOSResource(healthyDomain, ""),
	}

	result, err := awsclient.EnrichOpenSearchDomains(context.Background(), nil, resources, nil)
	if err != nil {
		t.Fatalf("EnrichOpenSearchDomains error: %v", err)
	}

	if len(result.Findings) != 0 {
		t.Errorf("len(Findings) = %d, want 0 (healthy domain has no findings)", len(result.Findings))
	}
	if result.IssueCount != 0 {
		t.Errorf("IssueCount = %d, want 0", result.IssueCount)
	}
}

// ---------------------------------------------------------------------------
// Test 6 — enricher_nil_clients_no_panic
// nil ServiceClients must return zero-value result without panicking
// ---------------------------------------------------------------------------

func TestOpenSearch_Enrich_NilClients_NoPanic(t *testing.T) {
	// Build a resource that would trigger findings if the enricher ran normally.
	resources := []resource.Resource{
		{
			ID:   "test-domain",
			Name: "test-domain",
			Fields: map[string]string{
				"service_software_update_available": "true",
				"encryption_at_rest_enabled":        "false",
			},
			RawStruct: ostypes.DomainStatus{
				DomainName: aws.String("test-domain"),
				ServiceSoftwareOptions: &ostypes.ServiceSoftwareOptions{
					UpdateAvailable:     aws.Bool(true),
					AutomatedUpdateDate: aws.Time(time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC)),
				},
				EncryptionAtRestOptions: &ostypes.EncryptionAtRestOptions{
					Enabled: aws.Bool(false),
				},
			},
		},
	}

	// Must not panic even with nil ServiceClients.
	var clients *awsclient.ServiceClients
	result, err := awsclient.EnrichOpenSearchDomains(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("EnrichOpenSearchDomains with nil clients returned error: %v", err)
	}
	// Contract: Findings must always be non-nil so downstream callers can iterate
	// without a nil-check. An empty map is fine; nil is a structural bug.
	if result.Findings == nil {
		t.Fatalf("EnrichOpenSearchDomains with nil clients returned Findings == nil; want non-nil (possibly empty) map")
	}
}
