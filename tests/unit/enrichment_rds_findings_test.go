package unit

// enrichment_rds_findings_test.go — Behavioral tests for EnrichRDSDocDBMaintenance
// plus the AS-140 stacked wave-1+wave-2 case for EnrichDBIMaintenance.
//
// Contract assertions (enricher-contract.md):
//   - Returns EnricherResult.Findings keyed by Resource.ID (ARN-suffix match).
//   - Severity "~" for all findings (informational, excluded from menu badge).
//   - Summary format: "pending maintenance: <actions>".
//   - IssueCount always 0 (severity "~" rule).
//   - Findings may contain entries for resources NOT in the input slice (account-wide).
//   - Empty result → non-nil empty Findings map.
//   - Truncated = true when Marker is non-nil.

import (
	"context"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// enrichRDSFake satisfies awsclient.RDSAPI by embedding the interface
// and overriding only the method under test.
type enrichRDSFake struct {
	awsclient.RDSAPI
	actions []rdstypes.ResourcePendingMaintenanceActions
	marker  *string
	err     error
}

func (f *enrichRDSFake) DescribePendingMaintenanceActions(
	_ context.Context,
	_ *rds.DescribePendingMaintenanceActionsInput,
	_ ...func(*rds.Options),
) (*rds.DescribePendingMaintenanceActionsOutput, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &rds.DescribePendingMaintenanceActionsOutput{
		PendingMaintenanceActions: f.actions,
		Marker:                    f.marker,
	}, nil
}

// TestEnrichRDSDocDBMaintenance_FindingsKeyedByResourceID verifies that the findings
// map is keyed by the ARN suffix — i.e. Resource.ID form — not the full ARN.
func TestEnrichRDSDocDBMaintenance_FindingsKeyedByResourceID(t *testing.T) {
	fake := &enrichRDSFake{
		actions: []rdstypes.ResourcePendingMaintenanceActions{
			{
				ResourceIdentifier: aws.String("arn:aws:rds:eu-west-2:123456789012:db:prod-db"),
				PendingMaintenanceActionDetails: []rdstypes.PendingMaintenanceAction{
					{Action: aws.String("system-update")},
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{RDS: fake}
	resources := []resource.Resource{{ID: "prod-db"}}

	result, err := awsclient.EnrichRDSDocDBMaintenance(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result.Findings["prod-db"]; !ok {
		t.Errorf("expected finding keyed by resource ID %q; got findings map: %v", "prod-db", result.Findings)
	}
	if _, ok := result.Findings["arn:aws:rds:eu-west-2:123456789012:db:prod-db"]; ok {
		t.Error("finding must NOT be keyed by full ARN")
	}
}

// TestEnrichRDSDocDBMaintenance_SeverityTilde verifies findings carry severity "~".
func TestEnrichRDSDocDBMaintenance_SeverityTilde(t *testing.T) {
	fake := &enrichRDSFake{
		actions: []rdstypes.ResourcePendingMaintenanceActions{
			{
				ResourceIdentifier: aws.String("arn:aws:rds:us-east-1:000000000000:db:my-db"),
				PendingMaintenanceActionDetails: []rdstypes.PendingMaintenanceAction{
					{Action: aws.String("os-upgrade")},
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{RDS: fake}
	resources := []resource.Resource{{ID: "my-db"}}

	result, err := awsclient.EnrichRDSDocDBMaintenance(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f, ok := result.Findings["my-db"]
	if !ok {
		t.Fatal("expected finding for my-db")
	}
	if f.Severity != "~" {
		t.Errorf("severity = %q, want %q", f.Severity, "~")
	}
}

// TestEnrichRDSDocDBMaintenance_SummaryFormat verifies the summary matches
// "pending maintenance: <action>" contract.
func TestEnrichRDSDocDBMaintenance_SummaryFormat(t *testing.T) {
	fake := &enrichRDSFake{
		actions: []rdstypes.ResourcePendingMaintenanceActions{
			{
				ResourceIdentifier: aws.String("arn:aws:rds:us-east-1:000000000000:db:my-db"),
				PendingMaintenanceActionDetails: []rdstypes.PendingMaintenanceAction{
					{Action: aws.String("system-update")},
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{RDS: fake}
	resources := []resource.Resource{{ID: "my-db"}}

	result, err := awsclient.EnrichRDSDocDBMaintenance(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f := result.Findings["my-db"]
	if !strings.HasPrefix(f.Summary, "pending maintenance") {
		t.Errorf("summary %q does not start with %q", f.Summary, "pending maintenance")
	}
	if !strings.Contains(f.Summary, "system-update") {
		t.Errorf("summary %q does not contain action name %q", f.Summary, "system-update")
	}
}

// TestEnrichRDSDocDBMaintenance_OffPageFindingsAreSkipped verifies that
// findings for resources NOT in the input slice are dropped. Emitting them
// would inflate unifiedIssueCount above the visible row count — e.g. when
// the enricher is dispatched for dbc (clusters), instance ARNs would otherwise
// produce findings keyed by instance IDs that don't correspond to any
// cluster row, surfacing as "DB Clusters (2) issues:4".
func TestEnrichRDSDocDBMaintenance_OffPageFindingsAreSkipped(t *testing.T) {
	fake := &enrichRDSFake{
		actions: []rdstypes.ResourcePendingMaintenanceActions{
			{
				ResourceIdentifier: aws.String("arn:aws:rds:us-east-1:000000000000:db:off-page-db"),
				PendingMaintenanceActionDetails: []rdstypes.PendingMaintenanceAction{
					{Action: aws.String("system-update")},
				},
			},
			{
				ResourceIdentifier: aws.String("arn:aws:rds:us-east-1:000000000000:db:on-page-db"),
				PendingMaintenanceActionDetails: []rdstypes.PendingMaintenanceAction{
					{Action: aws.String("system-update")},
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{RDS: fake}
	resources := []resource.Resource{{ID: "on-page-db"}}

	result, err := awsclient.EnrichRDSDocDBMaintenance(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result.Findings["off-page-db"]; ok {
		t.Error("off-page-db must NOT appear in Findings — orphan finding inflates badge above visible rows")
	}
	if _, ok := result.Findings["on-page-db"]; !ok {
		t.Error("expected finding for on-page-db (matches an input resource ID)")
	}
	if len(result.Findings) > len(resources) {
		t.Errorf("invariant violated: len(Findings)=%d > len(resources)=%d", len(result.Findings), len(resources))
	}
}

// TestEnrichRDSDocDBMaintenance_EmptyReturnsNonNilMap verifies the empty case returns
// a non-nil empty Findings map (MUST NOT return nil — banner logic depends on len(Findings)).
func TestEnrichRDSDocDBMaintenance_EmptyReturnsNonNilMap(t *testing.T) {
	fake := &enrichRDSFake{actions: nil, marker: nil}
	clients := &awsclient.ServiceClients{RDS: fake}

	result, err := awsclient.EnrichRDSDocDBMaintenance(context.Background(), clients, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Findings == nil {
		t.Error("Findings must not be nil on empty result — use make(map[string]EnrichmentFinding)")
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected empty Findings, got %d entries", len(result.Findings))
	}
	if result.IssueCount != 0 {
		t.Errorf("IssueCount = %d, want 0", result.IssueCount)
	}
}

// TestEnrichRDSDocDBMaintenance_TruncatedWhenMarkerPresent verifies Truncated=true
// when the API response has a non-nil Marker.
func TestEnrichRDSDocDBMaintenance_TruncatedWhenMarkerPresent(t *testing.T) {
	fake := &enrichRDSFake{
		actions: []rdstypes.ResourcePendingMaintenanceActions{
			{
				ResourceIdentifier: aws.String("arn:aws:rds:us-east-1:000000000000:db:db-1"),
				PendingMaintenanceActionDetails: []rdstypes.PendingMaintenanceAction{
					{Action: aws.String("system-update")},
				},
			},
		},
		marker: aws.String("next-page"),
	}
	clients := &awsclient.ServiceClients{RDS: fake}
	resources := []resource.Resource{{ID: "db-1"}}

	result, err := awsclient.EnrichRDSDocDBMaintenance(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Truncated {
		t.Error("Truncated must be true when API response Marker is non-nil")
	}
}

// TestEnrichRDSDocDBMaintenance_NilRDSClientReturnsEmptyFindings verifies nil client
// returns non-nil empty Findings (not an error — degraded gracefully).
func TestEnrichRDSDocDBMaintenance_NilRDSClientReturnsEmptyFindings(t *testing.T) {
	clients := &awsclient.ServiceClients{RDS: nil}
	result, err := awsclient.EnrichRDSDocDBMaintenance(context.Background(), clients, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Findings == nil {
		t.Error("Findings must not be nil even when RDS client is nil")
	}
	if result.IssueCount != 0 {
		t.Errorf("IssueCount = %d, want 0", result.IssueCount)
	}
}

// dbiStackedFake satisfies awsclient.RDSAPI for the AS-140 stacked-finding test.
type dbiStackedFake struct {
	awsclient.RDSAPI
	actions []rdstypes.ResourcePendingMaintenanceActions
}

func (f *dbiStackedFake) DescribePendingMaintenanceActions(
	_ context.Context,
	_ *rds.DescribePendingMaintenanceActionsInput,
	_ ...func(*rds.Options),
) (*rds.DescribePendingMaintenanceActionsOutput, error) {
	return &rds.DescribePendingMaintenanceActionsOutput{
		PendingMaintenanceActions: f.actions,
	}, nil
}

// TestEnrichDBI_Wave1StoppedPlusWave2_StackedFindings_AS140 pins AS-140 for the
// stacked wave-1+wave-2 case: a resource that already carries a wave-1 finding
// "stopped" (from the fetcher) plus a wave-2 maintenance finding emitted by
// EnrichDBIMaintenance.
//
// After both layers have contributed, the operator should see TWO findings on
// the resource:
//   - The wave-1 "stopped" finding stays in input resource.Findings (untouched
//     by the enricher).
//   - The wave-2 "pending maintenance" finding lands in result.Findings[id]
//     and is grafted onto resource.Findings later by applyEnrichment.
//
// AS-140 contract for THIS enricher run:
//   - result.Findings[id] is populated with the wave-2 Finding (1 entry).
//   - result.FieldUpdates is empty (or its [id] sub-map is empty/missing) —
//     no status overlay, no "(+1)" suffix arithmetic from the enricher side.
//   - The wave-1 Finding on input resource.Findings remains in place; combined
//     with the new wave-2 Finding, the unified r.Findings carries 2 entries
//     once applyEnrichment runs upstream. The render-layer phraseFromFindings
//     consumes both to produce "stopped (+1)".
func TestEnrichDBI_Wave1StoppedPlusWave2_StackedFindings_AS140(t *testing.T) {
	const resourceID = "stacked-stopped-plus-maint"
	const arn = "arn:aws:rds:us-east-1:123456789012:db:" + resourceID

	fake := &dbiStackedFake{
		actions: []rdstypes.ResourcePendingMaintenanceActions{
			{
				ResourceIdentifier: aws.String(arn),
				PendingMaintenanceActionDetails: []rdstypes.PendingMaintenanceAction{
					{Action: aws.String("system-update"), Description: aws.String("Engine patch")},
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{RDS: fake}

	// Input resource already carries a wave-1 "stopped" finding from the fetcher.
	// Post-PR-03e fetcher contract: Findings populated, Fields["status"] carries
	// the §4 phrase, Resource.Status intentionally empty.
	wave1Finding := domain.Finding{
		Code:     "dbi.broken.stopped",
		Phrase:   "stopped",
		Severity: domain.SevBroken,
		Source:   "wave1",
	}
	resources := []resource.Resource{
		{
			ID:       resourceID,
			Name:     resourceID,
			Findings: []domain.Finding{wave1Finding},
			Fields:   map[string]string{"status": "stopped"},
		},
	}

	result, err := awsclient.EnrichDBIMaintenance(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("EnrichDBIMaintenance error: %v", err)
	}

	// The enricher must emit exactly one Finding for this resource (the wave-2
	// maintenance signal). The wave-1 "stopped" finding belongs on the input
	// resource and stays out of result.Findings.
	wave2, ok := result.Findings[resourceID]
	if !ok {
		t.Fatalf("expected wave-2 Finding for %q; result.Findings keys = %v", resourceID, findingKeys(result.Findings))
	}
	if wave2.Summary != "pending maintenance" {
		t.Errorf("wave-2 Summary = %q, want %q", wave2.Summary, "pending maintenance")
	}
	if wave2.Severity != "~" {
		t.Errorf("wave-2 Severity = %q, want %q", wave2.Severity, "~")
	}

	// AS-140: result.FieldUpdates must be empty — the merged "stopped (+1)"
	// display is computed at render time by phraseFromFindings(r.Findings).
	if updates, hasUpdates := result.FieldUpdates[resourceID]; hasUpdates && len(updates) != 0 {
		t.Errorf("AS-140: expected empty FieldUpdates for %q (status overlay removed); got %v", resourceID, updates)
	}

	// The wave-1 finding stays untouched on the input resource so the unified
	// r.Findings (after applyEnrichment) carries 2 entries — one wave-1, one
	// wave-2. We verify the input resource's wave-1 finding survived to
	// confirm the enricher did not mutate the input.
	if len(resources[0].Findings) != 1 || resources[0].Findings[0].Phrase != "stopped" {
		t.Errorf("input wave-1 Finding must remain intact (count=1, phrase=%q); got Findings=%+v",
			"stopped", resources[0].Findings)
	}

	// Combined understanding: 1 wave-1 on resource.Findings + 1 wave-2 in
	// result.Findings[id] = 2 entries the unified r.Findings will carry after
	// applyEnrichment. Pin that count explicitly so a future regression that
	// drops the wave-2 emit is caught here.
	gotWave2Count := 0
	if _, ok := result.Findings[resourceID]; ok {
		gotWave2Count = 1
	}
	totalEntries := len(resources[0].Findings) + gotWave2Count
	if totalEntries != 2 {
		t.Errorf("wave-1 + wave-2 stacked Findings count = %d, want 2 (wave-1 on input + wave-2 in result)", totalEntries)
	}
}
