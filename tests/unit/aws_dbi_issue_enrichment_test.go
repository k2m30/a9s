package unit

// aws_dbi_issue_enrichment_test.go — Wave 2 enricher tests for dbi.
//
// Tests drive aws.EnrichDBIMaintenance (the dbi-specific enricher) and assert:
//   - Findings keyed by Resource.ID (ARN suffix-matched).
//   - Severity "~" (informational, no S1 badge bump).
//   - Summary is the short S5 phrase "pending maintenance" — Action and
//     Description never appear in Summary (they belong in Rows per the
//     resource.EnrichmentFinding contract).
//   - FieldUpdates["status"] == "maintenance scheduled" only on Healthy rows.
//   - Non-healthy rows get the finding but NOT the status field update.
//   - nil RDS client returns empty result gracefully.
//   - Multi-page response: all actions from both pages processed.

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// mock — satisfies awsclient.RDSAPI via embedding; overrides
// DescribePendingMaintenanceActions to return test-controlled data.
// Supports multi-page via the pages slice.
// ---------------------------------------------------------------------------

type dbiMaintenanceFake struct {
	awsclient.RDSAPI
	pages [][]rdstypes.ResourcePendingMaintenanceActions
	call  int
	err   error
}

func (f *dbiMaintenanceFake) DescribePendingMaintenanceActions(
	_ context.Context,
	_ *rds.DescribePendingMaintenanceActionsInput,
	_ ...func(*rds.Options),
) (*rds.DescribePendingMaintenanceActionsOutput, error) {
	if f.err != nil {
		return nil, f.err
	}
	if len(f.pages) == 0 {
		return &rds.DescribePendingMaintenanceActionsOutput{}, nil
	}
	idx := f.call
	if idx >= len(f.pages) {
		return &rds.DescribePendingMaintenanceActionsOutput{}, nil
	}
	f.call++
	var marker *string
	if f.call < len(f.pages) {
		marker = aws.String("next")
	}
	return &rds.DescribePendingMaintenanceActionsOutput{
		PendingMaintenanceActions: f.pages[idx],
		Marker:                    marker,
	}, nil
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// buildDbiResources converts all DBIFixtures.Instances into []resource.Resource
// by running them through FetchRDSInstancesPage (so Resource.Status is
// correctly derived by the fetcher, not hardcoded in tests).
func buildDbiResources(t *testing.T) []resource.Resource {
	t.Helper()
	fix := fixtures.NewDBIFixtures()
	mock := &mockRDSPageClient{instances: fix.Instances}
	result, err := awsclient.FetchRDSInstancesPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("buildDbiResources: FetchRDSInstancesPage error: %v", err)
	}
	return result.Resources
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestDBI_Enrich_MaintenancePending_HealthyRow verifies the full Wave 2 contract
// on the maint-dbi-scheduled fixture (spec §3.2 + §4 table row "Pending maintenance overdue").
func TestDBI_Enrich_MaintenancePending_HealthyRow(t *testing.T) {
	resources := buildDbiResources(t)
	fix := fixtures.NewDBIFixtures()

	fake := &dbiMaintenanceFake{
		pages: [][]rdstypes.ResourcePendingMaintenanceActions{fix.PendingMaintenanceActions},
	}
	clients := &awsclient.ServiceClients{RDS: fake}

	result, err := awsclient.EnrichDBIMaintenance(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("EnrichDBIMaintenance error: %v", err)
	}

	finding, ok := result.Findings[fixtures.MaintDbiScheduledID]
	if !ok {
		t.Fatalf("expected finding for %q; Findings keys = %v", fixtures.MaintDbiScheduledID, findingKeys(result.Findings))
	}
	if finding.Severity != "~" {
		t.Errorf("Severity = %q, want %q", finding.Severity, "~")
	}

	// Summary is the short S5 phrase — concrete details (Action, Description)
	// must NOT appear here; they belong in Rows. See the contract on
	// resource.EnrichmentFinding.
	if finding.Summary != "pending maintenance" {
		t.Errorf("Summary = %q, want %q", finding.Summary, "pending maintenance")
	}
	if strings.Contains(finding.Summary, "system-update") || strings.Contains(finding.Summary, "New minor engine patch") {
		t.Errorf("Summary must not embed Row content; got %q", finding.Summary)
	}
	// The same facts must be present in Rows.
	wantRows := map[string]string{
		"Action":      "system-update",
		"Description": "New minor engine patch 16.2.3",
	}
	gotRows := map[string]string{}
	for _, r := range finding.Rows {
		gotRows[r.Label] = r.Value
	}
	for label, val := range wantRows {
		if gotRows[label] != val {
			t.Errorf("Rows[%q] = %q, want %q", label, gotRows[label], val)
		}
	}

	if updates, ok := result.FieldUpdates[fixtures.MaintDbiScheduledID]; !ok {
		t.Errorf("FieldUpdates missing entry for %q", fixtures.MaintDbiScheduledID)
	} else if updates["status"] != "maintenance scheduled" {
		t.Errorf("FieldUpdates[%q][status] = %q, want %q", fixtures.MaintDbiScheduledID, updates["status"], "maintenance scheduled")
	}

	if result.IssueCount != 0 {
		t.Errorf("IssueCount = %d, want 0 (~ severity must not bump S1 badge)", result.IssueCount)
	}
}

// TestDBI_Enrich_MaintenancePending_NilDescription verifies that when the
// PendingMaintenanceAction.Description is nil, Summary stays the short phrase
// and the Description Row is simply omitted (rather than the Summary mutating
// to reflect missing details — see resource.EnrichmentFinding contract).
func TestDBI_Enrich_MaintenancePending_NilDescription(t *testing.T) {
	const resourceID = "inline-no-desc"
	const arn = "arn:aws:rds:us-east-1:123456789012:db:" + resourceID

	fake := &dbiMaintenanceFake{
		pages: [][]rdstypes.ResourcePendingMaintenanceActions{
			{
				{
					ResourceIdentifier: aws.String(arn),
					PendingMaintenanceActionDetails: []rdstypes.PendingMaintenanceAction{
						{Action: aws.String("os-upgrade"), Description: nil},
					},
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{RDS: fake}
	resources := []resource.Resource{
		{ID: resourceID, Name: resourceID, Status: "", Fields: map[string]string{"status": ""}},
	}

	result, err := awsclient.EnrichDBIMaintenance(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("EnrichDBIMaintenance error: %v", err)
	}
	finding, ok := result.Findings[resourceID]
	if !ok {
		t.Fatalf("expected finding for %q", resourceID)
	}
	if finding.Summary != "pending maintenance" {
		t.Errorf("Summary = %q, want %q", finding.Summary, "pending maintenance")
	}
	var labels []string
	for _, r := range finding.Rows {
		labels = append(labels, r.Label)
	}
	for _, l := range labels {
		if l == "Description" {
			t.Errorf("Rows must omit Description when source field is nil; got labels=%v", labels)
		}
	}
}

// TestDBI_Enrich_NonHealthyStatus_BumpsSuffix verifies that when the fetcher
// already set a non-empty Status (e.g. "publicly accessible"), the enricher
// adds a finding (for S5 visibility) AND bumps the FieldUpdates["status"] to
// "publicly accessible (+1)" — per spec §4 universal rule 7, Wave-2 findings
// bump the (+N) suffix so the operator sees there is more to open for.
func TestDBI_Enrich_NonHealthyStatus_BumpsSuffix(t *testing.T) {
	const resourceID = "inline-already-warning"
	const arn = "arn:aws:rds:us-east-1:123456789012:db:" + resourceID

	fake := &dbiMaintenanceFake{
		pages: [][]rdstypes.ResourcePendingMaintenanceActions{
			{
				{
					ResourceIdentifier: aws.String(arn),
					PendingMaintenanceActionDetails: []rdstypes.PendingMaintenanceAction{
						{Action: aws.String("os-upgrade"), Description: aws.String("OS patch")},
					},
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{RDS: fake}
	// Resource already has a Wave-1 warning status set.
	resources := []resource.Resource{
		{
			ID:     resourceID,
			Name:   resourceID,
			Status: "publicly accessible",
			Fields: map[string]string{"status": "publicly accessible"},
		},
	}

	result, err := awsclient.EnrichDBIMaintenance(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("EnrichDBIMaintenance error: %v", err)
	}

	// Finding must be present (S5 visibility).
	if _, ok := result.Findings[resourceID]; !ok {
		t.Errorf("expected finding for %q on non-healthy row (S5 still needed)", resourceID)
	}

	// FieldUpdates must bump the Wave-1 phrase with (+1) — NOT overwrite it with
	// "maintenance scheduled" and NOT leave it empty (universal rule 7).
	updates, ok := result.FieldUpdates[resourceID]
	if !ok {
		t.Fatalf("FieldUpdates missing entry for %q — Wave-2 must bump the suffix", resourceID)
	}
	want := "publicly accessible (+1)"
	if updates["status"] != want {
		t.Errorf("FieldUpdates[%q][status] = %q, want %q", resourceID, updates["status"], want)
	}
}

// TestDBI_Enrich_NoMatchNoFinding verifies that when the API returns ARNs
// for instances NOT in the resources slice, the result has no Findings.
func TestDBI_Enrich_NoMatchNoFinding(t *testing.T) {
	fake := &dbiMaintenanceFake{
		pages: [][]rdstypes.ResourcePendingMaintenanceActions{
			{
				{
					ResourceIdentifier: aws.String("arn:aws:rds:us-east-1:123456789012:db:not-in-resources"),
					PendingMaintenanceActionDetails: []rdstypes.PendingMaintenanceAction{
						{Action: aws.String("system-update")},
					},
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{RDS: fake}
	resources := []resource.Resource{
		{ID: "other-instance", Status: "", Fields: map[string]string{}},
	}

	result, err := awsclient.EnrichDBIMaintenance(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("EnrichDBIMaintenance error: %v", err)
	}
	if len(result.Findings) != 0 {
		t.Errorf("Findings must be empty when no ARN matches; got %v", result.Findings)
	}
}

// TestDBI_Enrich_NilRDSClient verifies that nil RDS client returns an empty
// result without error (degraded gracefully).
func TestDBI_Enrich_NilRDSClient(t *testing.T) {
	clients := &awsclient.ServiceClients{RDS: nil}
	result, err := awsclient.EnrichDBIMaintenance(context.Background(), clients, nil)
	if err != nil {
		t.Fatalf("EnrichDBIMaintenance error: %v", err)
	}
	if result.Findings == nil {
		t.Error("Findings must not be nil even when RDS client is nil")
	}
	if result.IssueCount != 0 {
		t.Errorf("IssueCount = %d, want 0", result.IssueCount)
	}
}

// TestDBI_Enrich_Pagination verifies that findings from both pages of a
// two-page DescribePendingMaintenanceActions response are processed.
func TestDBI_Enrich_Pagination(t *testing.T) {
	past := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	page1 := []rdstypes.ResourcePendingMaintenanceActions{
		{
			ResourceIdentifier: aws.String("arn:aws:rds:us-east-1:123456789012:db:dbi-page1"),
			PendingMaintenanceActionDetails: []rdstypes.PendingMaintenanceAction{
				{Action: aws.String("system-update"), AutoAppliedAfterDate: aws.Time(past)},
			},
		},
	}
	page2 := []rdstypes.ResourcePendingMaintenanceActions{
		{
			ResourceIdentifier: aws.String("arn:aws:rds:us-east-1:123456789012:db:dbi-page2"),
			PendingMaintenanceActionDetails: []rdstypes.PendingMaintenanceAction{
				{Action: aws.String("os-upgrade"), AutoAppliedAfterDate: aws.Time(past)},
			},
		},
	}

	fake := &dbiMaintenanceFake{pages: [][]rdstypes.ResourcePendingMaintenanceActions{page1, page2}}
	clients := &awsclient.ServiceClients{RDS: fake}
	resources := []resource.Resource{
		{ID: "dbi-page1", Status: "", Fields: map[string]string{"status": ""}},
		{ID: "dbi-page2", Status: "", Fields: map[string]string{"status": ""}},
	}

	result, err := awsclient.EnrichDBIMaintenance(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("EnrichDBIMaintenance error: %v", err)
	}
	if _, ok := result.Findings["dbi-page1"]; !ok {
		t.Error("expected finding for dbi-page1 (from page 1)")
	}
	if _, ok := result.Findings["dbi-page2"]; !ok {
		t.Error("expected finding for dbi-page2 (from page 2)")
	}
}

// ---------------------------------------------------------------------------
// Wave 1 + Wave 2 stacking — (+N) suffix (spec §4 universal rule 7)
// ---------------------------------------------------------------------------

// TestDBI_Enrich_Wave1PlusWave2_BumpsSuffix verifies that when a resource already
// has a Wave-1 warning status ("publicly accessible"), adding a Wave-2 maintenance
// finding bumps the status to "publicly accessible (+1)" — the operator sees there
// is more to open for in the S5 findings panel.
func TestDBI_Enrich_Wave1PlusWave2_BumpsSuffix(t *testing.T) {
	const resourceID = fixtures.WarnDbiPublicMaintID
	const arn = fixtures.WarnDbiPublicMaintARN

	fake := &dbiMaintenanceFake{
		pages: [][]rdstypes.ResourcePendingMaintenanceActions{
			{
				{
					ResourceIdentifier: aws.String(arn),
					PendingMaintenanceActionDetails: []rdstypes.PendingMaintenanceAction{
						{Action: aws.String("system-update"), Description: aws.String("OS patch")},
					},
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{RDS: fake}
	// Simulate the fetcher-produced status for this fixture (Wave 1 warning).
	resources := []resource.Resource{
		{
			ID:     resourceID,
			Name:   resourceID,
			Status: "publicly accessible",
			Fields: map[string]string{"status": "publicly accessible"},
		},
	}

	result, err := awsclient.EnrichDBIMaintenance(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("EnrichDBIMaintenance error: %v", err)
	}

	// Finding must be present with severity "~".
	finding, ok := result.Findings[resourceID]
	if !ok {
		t.Fatalf("expected finding for %q; Findings keys = %v", resourceID, findingKeys(result.Findings))
	}
	if finding.Severity != "~" {
		t.Errorf("Severity = %q, want %q", finding.Severity, "~")
	}

	// FieldUpdates must bump the existing Wave-1 phrase with (+1), NOT overwrite
	// with "maintenance scheduled" (Wave 1 phrase wins severity precedence).
	updates, ok := result.FieldUpdates[resourceID]
	if !ok {
		t.Fatalf("FieldUpdates missing entry for %q", resourceID)
	}
	want := "publicly accessible (+1)"
	if updates["status"] != want {
		t.Errorf("FieldUpdates[%q][status] = %q, want %q", resourceID, updates["status"], want)
	}

	// "~" severity must not bump the S1 badge.
	if result.IssueCount != 0 {
		t.Errorf("IssueCount = %d, want 0", result.IssueCount)
	}
}

// TestDBI_Enrich_Wave1MultiPlusWave2_BumpsExistingSuffix verifies that when a
// resource already carries a Wave-1 multi-warning status of "no automated backups (+2)"
// (3 stacked warnings), adding a Wave-2 maintenance finding increments the counter
// to "(+3)" — NOT producing "(+2) (+1)" or any double-suffix.
func TestDBI_Enrich_Wave1MultiPlusWave2_BumpsExistingSuffix(t *testing.T) {
	const resourceID = "inline-3warn-plus-maint"
	const arn = "arn:aws:rds:us-east-1:123456789012:db:" + resourceID

	fake := &dbiMaintenanceFake{
		pages: [][]rdstypes.ResourcePendingMaintenanceActions{
			{
				{
					ResourceIdentifier: aws.String(arn),
					PendingMaintenanceActionDetails: []rdstypes.PendingMaintenanceAction{
						{Action: aws.String("system-update"), Description: aws.String("Minor patch")},
					},
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{RDS: fake}
	// Simulate a 3-warning Wave-1 status already set by the fetcher.
	resources := []resource.Resource{
		{
			ID:     resourceID,
			Name:   resourceID,
			Status: "no automated backups (+2)",
			Fields: map[string]string{"status": "no automated backups (+2)"},
		},
	}

	result, err := awsclient.EnrichDBIMaintenance(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("EnrichDBIMaintenance error: %v", err)
	}

	updates, ok := result.FieldUpdates[resourceID]
	if !ok {
		t.Fatalf("FieldUpdates missing entry for %q", resourceID)
	}
	want := "no automated backups (+3)"
	if updates["status"] != want {
		t.Errorf("FieldUpdates[%q][status] = %q, want %q (must increment counter, not double-suffix)", resourceID, updates["status"], want)
	}
}

// TestDBI_Enrich_HealthyPlusWave2_NoSuffix_Regression is a regression pin: a
// healthy resource (empty Status) that gets a Wave-2 maintenance finding must
// receive "maintenance scheduled" — no suffix, since it is the sole finding.
func TestDBI_Enrich_HealthyPlusWave2_NoSuffix_Regression(t *testing.T) {
	fix := fixtures.NewDBIFixtures()
	resources := buildDbiResources(t)

	fake := &dbiMaintenanceFake{
		pages: [][]rdstypes.ResourcePendingMaintenanceActions{fix.PendingMaintenanceActions},
	}
	clients := &awsclient.ServiceClients{RDS: fake}

	result, err := awsclient.EnrichDBIMaintenance(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("EnrichDBIMaintenance error: %v", err)
	}

	updates, ok := result.FieldUpdates[fixtures.MaintDbiScheduledID]
	if !ok {
		t.Fatalf("FieldUpdates missing entry for %q", fixtures.MaintDbiScheduledID)
	}
	want := "maintenance scheduled"
	if updates["status"] != want {
		t.Errorf("FieldUpdates[%q][status] = %q, want %q (sole Wave-2 finding must not add suffix)", fixtures.MaintDbiScheduledID, updates["status"], want)
	}
}

// TestDBI_Enrich_BumpFindingSuffix_EdgeCases drives edge cases of the
// bumpFindingSuffix logic via EnrichDBIMaintenance (which calls bumpFindingSuffix
// internally when existing status is non-empty). This tests the helper indirectly
// to remain robust against helper-naming changes.
func TestDBI_Enrich_BumpFindingSuffix_EdgeCases(t *testing.T) {
	cases := []struct {
		name           string
		existingStatus string // Wave-1 status already on the resource
		wantStatus     string // expected FieldUpdates["status"] after Wave-2 enrichment
	}{
		{
			name:           "no_existing_suffix",
			existingStatus: "publicly accessible",
			wantStatus:     "publicly accessible (+1)",
		},
		{
			name:           "existing_suffix_1",
			existingStatus: "publicly accessible (+1)",
			wantStatus:     "publicly accessible (+2)",
		},
		{
			name:           "existing_suffix_9_becomes_10",
			existingStatus: "publicly accessible (+9)",
			wantStatus:     "publicly accessible (+10)",
		},
		{
			name:           "unparsable_suffix_appends_fresh",
			existingStatus: "foo (+bar)",
			wantStatus:     "foo (+bar) (+1)",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			id := "inline-bump-" + tc.name
			arn := "arn:aws:rds:us-east-1:123456789012:db:" + id

			fake := &dbiMaintenanceFake{
				pages: [][]rdstypes.ResourcePendingMaintenanceActions{
					{
						{
							ResourceIdentifier: aws.String(arn),
							PendingMaintenanceActionDetails: []rdstypes.PendingMaintenanceAction{
								{Action: aws.String("system-update")},
							},
						},
					},
				},
			}
			clients := &awsclient.ServiceClients{RDS: fake}
			resources := []resource.Resource{
				{
					ID:     id,
					Name:   id,
					Status: tc.existingStatus,
					Fields: map[string]string{"status": tc.existingStatus},
				},
			}

			result, err := awsclient.EnrichDBIMaintenance(context.Background(), clients, resources)
			if err != nil {
				t.Fatalf("EnrichDBIMaintenance error: %v", err)
			}

			updates, ok := result.FieldUpdates[id]
			if !ok {
				t.Fatalf("FieldUpdates missing entry for %q", id)
			}
			if updates["status"] != tc.wantStatus {
				t.Errorf("FieldUpdates[%q][status] = %q, want %q", id, updates["status"], tc.wantStatus)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// internal helpers
// ---------------------------------------------------------------------------

func findingKeys(m map[string]resource.EnrichmentFinding) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
