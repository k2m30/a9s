package unit

// aws_dbi_issue_enrichment_test.go — Wave 2 enricher tests for dbi.
//
// AS-140 (Wave-2 enricher migration): FieldUpdates["status"] is no longer
// written by EnrichDBIMaintenance. The merged §4 status phrase is computed
// at render time by phraseFromFindings(r.Findings) in extractCellValue —
// wave-1 findings reach r.Findings via the fetcher, wave-2 findings via
// applyEnrichment.
//
// Tests drive aws.EnrichDBIMaintenance (the dbi-specific enricher) and assert:
//   - Findings keyed by Resource.ID (ARN suffix-matched).
//   - Severity "~" (informational, no S1 badge bump).
//   - Summary is the short S5 phrase "pending maintenance" — Action and
//     Description never appear in Summary (they belong in Rows per the
//     resource.EnrichmentFinding contract).
//   - FieldUpdates is nil or empty in every case (AS-140: no status overlay,
//     no "(+N)" suffix arithmetic on the enricher side).
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
	"github.com/k2m30/a9s/v3/internal/domain"
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

	result, err := awsclient.EnrichDBIMaintenance(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("EnrichDBIMaintenance error: %v", err)
	}

	finding, ok := result.Findings[fixtures.MaintDbiScheduledID]
	if !ok {
		t.Fatalf("expected finding for %q; Findings keys = %v", fixtures.MaintDbiScheduledID, findingKeys(result.Findings))
	}
	if finding.Severity != domain.SevWarn {
		t.Errorf("Severity = %v, want SevWarn", finding.Severity)
	}

	// Phrase is the short S5 phrase — concrete details (Action, Description)
	// must NOT appear here; they belong in AttentionDetail rows.
	if finding.Phrase != "pending maintenance" {
		t.Errorf("Phrase = %q, want %q", finding.Phrase, "pending maintenance")
	}
	if strings.Contains(finding.Phrase, "system-update") || strings.Contains(finding.Phrase, "New minor engine patch") {
		t.Errorf("Phrase must not embed Row content; got %q", finding.Phrase)
	}
	// The same facts must be present in AttentionDetail rows.
	wantRows := map[string]string{
		"Action":      "system-update",
		"Description": "New minor engine patch 16.2.3",
	}
	gotRows := map[string]string{}
	for _, r := range result.AttentionDetails[fixtures.MaintDbiScheduledID].Rows {
		gotRows[r.Label] = r.Value
	}
	for label, val := range wantRows {
		if gotRows[label] != val {
			t.Errorf("Rows[%q] = %q, want %q", label, gotRows[label], val)
		}
	}

	// AS-140: FieldUpdates must be nil/empty — the merged display phrase is
	// computed by phraseFromFindings(r.Findings) at render time.
	if updates, ok := result.FieldUpdates[fixtures.MaintDbiScheduledID]; ok && len(updates) != 0 {
		t.Errorf("AS-140: expected empty FieldUpdates for %q (status overlay removed); got %v", fixtures.MaintDbiScheduledID, updates)
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
		{ID: resourceID, Name: resourceID, Fields: map[string]string{"status": ""}},
	}

	result, err := awsclient.EnrichDBIMaintenance(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("EnrichDBIMaintenance error: %v", err)
	}
	finding, ok := result.Findings[resourceID]
	if !ok {
		t.Fatalf("expected finding for %q", resourceID)
	}
	if finding.Phrase != "pending maintenance" {
		t.Errorf("Phrase = %q, want %q", finding.Phrase, "pending maintenance")
	}
	var labels []string
	for _, r := range result.AttentionDetails[resourceID].Rows {
		labels = append(labels, r.Label)
	}
	for _, l := range labels {
		if l == "Description" {
			t.Errorf("Rows must omit Description when source field is nil; got labels=%v", labels)
		}
	}
}

// TestDBI_Enrich_NonHealthyStatus_NoFieldUpdates verifies AS-140: when the
// fetcher already set a non-empty Status (e.g. "publicly accessible"), the
// enricher still emits a Finding (for S5 visibility) but MUST NOT write
// FieldUpdates["status"]. The merged "publicly accessible (+1)" display is
// computed at render time by phraseFromFindings(r.Findings).
func TestDBI_Enrich_NonHealthyStatus_NoFieldUpdates(t *testing.T) {
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
			Fields: map[string]string{"status": "publicly accessible"},
		},
	}

	result, err := awsclient.EnrichDBIMaintenance(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("EnrichDBIMaintenance error: %v", err)
	}

	// Finding must be present (S5 visibility) — wave-1 stacking must NOT
	// suppress the wave-2 maintenance finding.
	if _, ok := result.Findings[resourceID]; !ok {
		t.Errorf("expected finding for %q on non-healthy row (S5 still needed)", resourceID)
	}

	// AS-140: FieldUpdates must be empty — no status overlay, no "(+1)" suffix
	// arithmetic on the enricher side.
	if updates, ok := result.FieldUpdates[resourceID]; ok && len(updates) != 0 {
		t.Errorf("AS-140: expected empty FieldUpdates for %q (status overlay removed); got %v", resourceID, updates)
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
		{ID: "other-instance", Fields: map[string]string{}},
	}

	result, err := awsclient.EnrichDBIMaintenance(context.Background(), clients, resources, nil)
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
	result, err := awsclient.EnrichDBIMaintenance(context.Background(), clients, nil, nil)
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
		{ID: "dbi-page1", Fields: map[string]string{"status": ""}},
		{ID: "dbi-page2", Fields: map[string]string{"status": ""}},
	}

	result, err := awsclient.EnrichDBIMaintenance(context.Background(), clients, resources, nil)
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

// TestDBI_Enrich_Wave1PlusWave2_NoFieldUpdates verifies AS-140: when a
// resource already has a Wave-1 warning status ("publicly accessible") and a
// Wave-2 maintenance finding stacks on top, the Finding is still emitted but
// FieldUpdates is left empty — the merged "publicly accessible (+1)" display
// is computed at render time by phraseFromFindings(r.Findings).
func TestDBI_Enrich_Wave1PlusWave2_NoFieldUpdates(t *testing.T) {
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
			Fields: map[string]string{"status": "publicly accessible"},
		},
	}

	result, err := awsclient.EnrichDBIMaintenance(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("EnrichDBIMaintenance error: %v", err)
	}

	// Finding must be present with SevWarn severity.
	finding, ok := result.Findings[resourceID]
	if !ok {
		t.Fatalf("expected finding for %q; Findings keys = %v", resourceID, findingKeys(result.Findings))
	}
	if finding.Severity != domain.SevWarn {
		t.Errorf("Severity = %v, want SevWarn", finding.Severity)
	}

	// AS-140: FieldUpdates must be empty.
	if updates, ok := result.FieldUpdates[resourceID]; ok && len(updates) != 0 {
		t.Errorf("AS-140: expected empty FieldUpdates for %q (status overlay removed); got %v", resourceID, updates)
	}

	// "~" severity must not bump the S1 badge.
	if result.IssueCount != 0 {
		t.Errorf("IssueCount = %d, want 0", result.IssueCount)
	}
}

// TestDBI_Enrich_Wave1PlusWave2_PostPR03eShape_NoFieldUpdates verifies AS-140
// on the post-PR-03e fetcher shape (Findings populated, Status empty,
// Fields["status"] carrying the merged §4 phrase). The Wave-2 maintenance
// finding is emitted to result.Findings, but FieldUpdates is empty — the
// render-layer phraseFromFindings(r.Findings) computes the merged
// "publicly accessible (+1)" display from the stacked findings.
//
// Pre-AS-140 this test pinned the suffix bump on FieldUpdates as an AS-132
// regression check; that concern is now fully delegated to the
// applyEnrichment / phraseFromFindings path.
func TestDBI_Enrich_Wave1PlusWave2_PostPR03eShape_NoFieldUpdates(t *testing.T) {
	const resourceID = fixtures.WarnDbiPublicMaintID
	const arn = fixtures.WarnDbiPublicMaintARN

	fake := &dbiMaintenanceFake{
		pages: [][]rdstypes.ResourcePendingMaintenanceActions{
			{
				{
					ResourceIdentifier: aws.String(arn),
					PendingMaintenanceActionDetails: []rdstypes.PendingMaintenanceAction{
						{Action: aws.String("os-upgrade"), Description: aws.String("Kernel security patch")},
					},
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{RDS: fake}

	// Post-PR-03e fetcher shape: Findings populated, Status empty, Fields["status"]
	// carrying the merged §4 phrase.
	resources := []resource.Resource{
		{
			ID:   resourceID,
			Name: resourceID,
			Findings: []domain.Finding{
				{Code: awsclient.CodeDBIPubliclyAccessible, Phrase: "publicly accessible", Severity: domain.SevWarn, Source: "wave1"},
			},
			Fields: map[string]string{"status": "publicly accessible"},
			// Status: "" — intentionally unset; mirrors the fetcher's post-PR-03e contract.
		},
	}

	result, err := awsclient.EnrichDBIMaintenance(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("EnrichDBIMaintenance error: %v", err)
	}

	// The wave-2 maintenance Finding must still be emitted.
	if _, ok := result.Findings[resourceID]; !ok {
		t.Errorf("expected finding for %q on post-PR-03e wave-1 shape", resourceID)
	}

	// AS-140: FieldUpdates must be empty.
	if updates, ok := result.FieldUpdates[resourceID]; ok && len(updates) != 0 {
		t.Errorf("AS-140: expected empty FieldUpdates for %q (status overlay removed); got %v", resourceID, updates)
	}
}

// TestDBI_Enrich_Wave1MultiPlusWave2_NoFieldUpdates verifies AS-140: when a
// resource already carries a Wave-1 multi-warning status, the enricher still
// emits its wave-2 Finding but does NOT touch FieldUpdates. The merged
// "(+N+1)" display is computed at render time from r.Findings.
func TestDBI_Enrich_Wave1MultiPlusWave2_NoFieldUpdates(t *testing.T) {
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
			Fields: map[string]string{"status": "no automated backups (+2)"},
		},
	}

	result, err := awsclient.EnrichDBIMaintenance(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("EnrichDBIMaintenance error: %v", err)
	}

	if _, ok := result.Findings[resourceID]; !ok {
		t.Errorf("expected finding for %q (wave-2 must emit even when wave-1 has +N)", resourceID)
	}
	if updates, ok := result.FieldUpdates[resourceID]; ok && len(updates) != 0 {
		t.Errorf("AS-140: expected empty FieldUpdates for %q (status overlay removed); got %v", resourceID, updates)
	}
}

// TestDBI_Enrich_HealthyPlusWave2_NoFieldUpdates_Regression verifies AS-140 for
// the healthy-resource case: a wave-2 finding is emitted (severity "~",
// Summary "pending maintenance") but FieldUpdates is empty. Pre-AS-140 the
// enricher would have written "maintenance scheduled" here.
func TestDBI_Enrich_HealthyPlusWave2_NoFieldUpdates_Regression(t *testing.T) {
	fix := fixtures.NewDBIFixtures()
	resources := buildDbiResources(t)

	fake := &dbiMaintenanceFake{
		pages: [][]rdstypes.ResourcePendingMaintenanceActions{fix.PendingMaintenanceActions},
	}
	clients := &awsclient.ServiceClients{RDS: fake}

	result, err := awsclient.EnrichDBIMaintenance(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("EnrichDBIMaintenance error: %v", err)
	}

	if _, ok := result.Findings[fixtures.MaintDbiScheduledID]; !ok {
		t.Errorf("expected finding for %q", fixtures.MaintDbiScheduledID)
	}
	if updates, ok := result.FieldUpdates[fixtures.MaintDbiScheduledID]; ok && len(updates) != 0 {
		t.Errorf("AS-140: expected empty FieldUpdates for %q (status overlay removed); got %v", fixtures.MaintDbiScheduledID, updates)
	}
}

// TestDBI_Enrich_VariousExistingStatuses_NoFieldUpdates verifies AS-140 for a
// matrix of pre-existing Wave-1 statuses: the enricher emits its Finding but
// never writes FieldUpdates regardless of the prior status value. Previously
// this test verified bump arithmetic on FieldUpdates; that responsibility
// moved to phraseFromFindings at render time.
func TestDBI_Enrich_VariousExistingStatuses_NoFieldUpdates(t *testing.T) {
	cases := []struct {
		name           string
		existingStatus string // Wave-1 status already on the resource
	}{
		{name: "no_existing_suffix", existingStatus: "publicly accessible"},
		{name: "existing_suffix_1", existingStatus: "publicly accessible (+1)"},
		{name: "existing_suffix_9", existingStatus: "publicly accessible (+9)"},
		{name: "unparsable_suffix", existingStatus: "foo (+bar)"},
		{name: "healthy_empty_status", existingStatus: ""},
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
					Fields: map[string]string{"status": tc.existingStatus},
				},
			}

			result, err := awsclient.EnrichDBIMaintenance(context.Background(), clients, resources, nil)
			if err != nil {
				t.Fatalf("EnrichDBIMaintenance error: %v", err)
			}

			// Finding must still be emitted regardless of pre-existing status.
			if _, ok := result.Findings[id]; !ok {
				t.Errorf("expected finding for %q", id)
			}

			// AS-140: FieldUpdates always empty — no status overlay.
			if updates, ok := result.FieldUpdates[id]; ok && len(updates) != 0 {
				t.Errorf("AS-140: expected empty FieldUpdates for %q (existingStatus=%q); got %v", id, tc.existingStatus, updates)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// internal helpers
// ---------------------------------------------------------------------------

func findingKeys(m map[string]domain.Finding) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
