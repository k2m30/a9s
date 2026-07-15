package unit

// qa_enrich_rds_truncated_test.go — Tests that EnrichDBIMaintenance's
// account-wide DescribePendingMaintenanceActions pagination walk correctly
// terminates on Marker==nil/empty, and that hitting the EnrichmentCap page
// limit does NOT flip the aggregate Truncated flag — dbi only ever emits
// "~" (informational) findings, so a coverage gap in the account-wide walk
// must never lower-bound the issue badge (cf. issue_enrichment.go
// IssueEnricherResult.Truncated contract).
//
// Originally pinned against the dead EnrichRDSDocDBMaintenance (deleted:
// wired to no catalog Wave2 field), which set Truncated straight from the
// last page's Marker. EnrichDBIMaintenance (the live sibling per
// docs/resources/dbi.md §3.2) uses a different mechanism: it keeps
// paginating until Marker is nil/empty OR EnrichmentCap pages have been
// walked — the exact call count (== EnrichmentCap) is what proves the walk
// was actually cut off, since the aggregate Truncated flag itself stays
// false either way.

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

// rdsMaintenanceFake stubs DescribePendingMaintenanceActions and satisfies RDSAPI
// by embedding the real demo fake for all other methods.
type rdsMaintenanceFake struct {
	awsclient.RDSAPI // embed the interface to satisfy all other methods (will panic if called)
	actions          []rdstypes.ResourcePendingMaintenanceActions
	marker           *string
}

func (f *rdsMaintenanceFake) DescribePendingMaintenanceActions(_ context.Context, _ *rds.DescribePendingMaintenanceActionsInput, _ ...func(*rds.Options)) (*rds.DescribePendingMaintenanceActionsOutput, error) {
	return &rds.DescribePendingMaintenanceActionsOutput{
		PendingMaintenanceActions: f.actions,
		Marker:                    f.marker,
	}, nil
}

func TestEnrichDBIMaintenance_NotTruncated(t *testing.T) {
	fake := &rdsMaintenanceFake{
		actions: []rdstypes.ResourcePendingMaintenanceActions{
			{ResourceIdentifier: aws.String("arn:aws:rds:us-east-1:000000000000:db:prod-db")},
		},
		marker: nil, // no more pages
	}
	clients := &awsclient.ServiceClients{RDS: fake}

	probeResources := []resource.Resource{{ID: "prod-db"}}
	result, err := awsclient.EnrichDBIMaintenance(context.Background(), clients, probeResources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Findings) != 1 {
		t.Errorf("len(Findings) = %d, want 1", len(result.Findings))
	}
	if result.Truncated {
		t.Error("Truncated = true, want false (single page, no Marker)")
	}
}

// rdsUnboundedMaintenanceFake always hands back a non-nil Marker, so the
// pagination walk in EnrichDBIMaintenance never sees a natural stop and must
// be cut off by EnrichmentCap.
type rdsUnboundedMaintenanceFake struct {
	awsclient.RDSAPI
	calls int
}

func (f *rdsUnboundedMaintenanceFake) DescribePendingMaintenanceActions(_ context.Context, _ *rds.DescribePendingMaintenanceActionsInput, _ ...func(*rds.Options)) (*rds.DescribePendingMaintenanceActionsOutput, error) {
	f.calls++
	return &rds.DescribePendingMaintenanceActionsOutput{
		PendingMaintenanceActions: []rdstypes.ResourcePendingMaintenanceActions{
			{ResourceIdentifier: aws.String("arn:aws:rds:us-east-1:000000000000:db:prod-db")},
		},
		Marker: aws.String("next-page-token"), // never terminates on its own
	}, nil
}

// TestEnrichDBIMaintenance_PaginationCapDoesNotTruncateIssueBadge verifies
// that when the account-wide DescribePendingMaintenanceActions pagination
// walk never terminates on its own and must be cut off by EnrichmentCap, the
// aggregate Truncated flag stays false — dbi only ever emits "~"
// (informational) findings, so hitting the page cap never lower-bounds the
// issue badge. The exact call count (== EnrichmentCap) is the only signal
// that proves the walk was actually capped; dbi is account-wide, so there is
// no single resource row to mark via TruncatedIDs for this coverage gap.
func TestEnrichDBIMaintenance_PaginationCapDoesNotTruncateIssueBadge(t *testing.T) {
	fake := &rdsUnboundedMaintenanceFake{}
	clients := &awsclient.ServiceClients{RDS: fake}

	probeResources := []resource.Resource{{ID: "prod-db"}}
	result, err := awsclient.EnrichDBIMaintenance(context.Background(), clients, probeResources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Truncated {
		t.Error("Truncated = true, want false: dbi only emits \"~\" findings, so hitting EnrichmentCap on the account-wide pagination walk must not lower-bound the aggregate issue badge")
	}
	if fake.calls != awsclient.EnrichmentCap {
		t.Errorf("DescribePendingMaintenanceActions called %d times, want exactly EnrichmentCap=%d", fake.calls, awsclient.EnrichmentCap)
	}
}
