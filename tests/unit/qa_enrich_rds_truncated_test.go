package unit

// EnrichDBIMaintenance's account-wide
// DescribePendingMaintenanceActions walk stops on Marker==nil/empty or after
// EnrichmentCap pages. The call
// count (== EnrichmentCap) is what proves the walk was cut off.

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

// rdsMaintenanceFake stubs DescribePendingMaintenanceActions.
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
		marker: nil,
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
	// No backup list is loaded, so the backup-coverage check marks the row;
	// the single-page walk itself left nothing at its cap.
	if got := result.TruncatedIDs["prod-db"]; got == awsclient.CheckCap {
		t.Errorf("prod-db marked %q after a single-page walk", got)
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
		Marker: aws.String("next-page-token"),
	}, nil
}

// TestEnrichDBIMaintenance_PaginationCapDoesNotTruncateIssueBadge verifies
// that the account-wide DescribePendingMaintenanceActions pagination walk,
// which never terminates on its own, is cut off by EnrichmentCap: the exact
// call count (== EnrichmentCap) proves the walk was capped.
func TestEnrichDBIMaintenance_PaginationCapDoesNotTruncateIssueBadge(t *testing.T) {
	fake := &rdsUnboundedMaintenanceFake{}
	clients := &awsclient.ServiceClients{RDS: fake}

	probeResources := []resource.Resource{{ID: "prod-db"}}
	_, err := awsclient.EnrichDBIMaintenance(context.Background(), clients, probeResources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.calls != awsclient.EnrichmentCap {
		t.Errorf("DescribePendingMaintenanceActions called %d times, want exactly EnrichmentCap=%d", fake.calls, awsclient.EnrichmentCap)
	}
}
