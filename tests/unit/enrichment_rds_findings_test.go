package unit

// enrichment_rds_findings_test.go — Behavioral tests for EnrichRDSDocDBMaintenance.
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

	result, err := awsclient.EnrichRDSDocDBMaintenance(context.Background(), clients, resources)
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

	result, err := awsclient.EnrichRDSDocDBMaintenance(context.Background(), clients, resources)
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

	result, err := awsclient.EnrichRDSDocDBMaintenance(context.Background(), clients, resources)
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

	result, err := awsclient.EnrichRDSDocDBMaintenance(context.Background(), clients, resources)
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

	result, err := awsclient.EnrichRDSDocDBMaintenance(context.Background(), clients, nil)
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

	result, err := awsclient.EnrichRDSDocDBMaintenance(context.Background(), clients, resources)
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
	result, err := awsclient.EnrichRDSDocDBMaintenance(context.Background(), clients, nil)
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
