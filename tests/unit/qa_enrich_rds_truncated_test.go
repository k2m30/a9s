package unit

// qa_enrich_rds_truncated_test.go — Tests that EnrichRDSDocDBMaintenance
// correctly reports truncated=true when the DescribePendingMaintenanceActions
// response has a non-nil Marker (pagination continuation token).

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
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
		Marker:                   f.marker,
	}, nil
}

func TestEnrichRDSDocDBMaintenance_NotTruncated(t *testing.T) {
	fake := &rdsMaintenanceFake{
		actions: []rdstypes.ResourcePendingMaintenanceActions{
			{ResourceIdentifier: aws.String("arn:aws:rds:us-east-1:000000000000:db:prod-db")},
		},
		marker: nil, // no more pages
	}
	clients := &awsclient.ServiceClients{RDS: fake}

	count, truncated, err := awsclient.EnrichRDSDocDBMaintenance(context.Background(), clients, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 1 {
		t.Errorf("count = %d, want 1", count)
	}
	if truncated {
		t.Error("truncated = true, want false (no Marker)")
	}
}

func TestEnrichRDSDocDBMaintenance_Truncated(t *testing.T) {
	fake := &rdsMaintenanceFake{
		actions: []rdstypes.ResourcePendingMaintenanceActions{
			{ResourceIdentifier: aws.String("arn:aws:rds:us-east-1:000000000000:db:prod-db-1")},
			{ResourceIdentifier: aws.String("arn:aws:rds:us-east-1:000000000000:db:prod-db-2")},
		},
		marker: aws.String("next-page-token"), // more pages exist
	}
	clients := &awsclient.ServiceClients{RDS: fake}

	count, truncated, err := awsclient.EnrichRDSDocDBMaintenance(context.Background(), clients, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}
	if !truncated {
		t.Error("truncated = false, want true (Marker is non-nil)")
	}
}

func TestEnrichRDSDocDBMaintenance_ZeroActionsNotTruncated(t *testing.T) {
	fake := &rdsMaintenanceFake{
		actions: nil,
		marker:  nil,
	}
	clients := &awsclient.ServiceClients{RDS: fake}

	count, truncated, err := awsclient.EnrichRDSDocDBMaintenance(context.Background(), clients, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 0 {
		t.Errorf("count = %d, want 0", count)
	}
	if truncated {
		t.Error("truncated = true, want false (empty response)")
	}
}

func TestEnrichRDSDocDBMaintenance_NilClients(t *testing.T) {
	clients := &awsclient.ServiceClients{RDS: nil}
	count, truncated, err := awsclient.EnrichRDSDocDBMaintenance(context.Background(), clients, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 0 || truncated {
		t.Errorf("nil RDS client: count=%d truncated=%v, want 0/false", count, truncated)
	}
}
