package unit

// qa_enrich_rds_truncated_test.go — Tests that EnrichRDSDocDBMaintenance
// correctly reports Truncated=true when the DescribePendingMaintenanceActions
// response has a non-nil Marker (pagination continuation token).
//
// Updated for EnricherResult return type: result, err := EnrichRDSDocDBMaintenance(...).

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
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

	probeResources := []resource.Resource{{ID: "prod-db"}}
	result, err := awsclient.EnrichRDSDocDBMaintenance(context.Background(), clients, probeResources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Findings) != 1 {
		t.Errorf("len(Findings) = %d, want 1", len(result.Findings))
	}
	if result.Truncated {
		t.Error("Truncated = true, want false (no Marker)")
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

	probeResources := []resource.Resource{
		{ID: "prod-db-1"},
		{ID: "prod-db-2"},
	}
	result, err := awsclient.EnrichRDSDocDBMaintenance(context.Background(), clients, probeResources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Findings) != 2 {
		t.Errorf("len(Findings) = %d, want 2", len(result.Findings))
	}
	if !result.Truncated {
		t.Error("Truncated = false, want true (Marker is non-nil)")
	}
}

func TestEnrichRDSDocDBMaintenance_ZeroActionsNotTruncated(t *testing.T) {
	fake := &rdsMaintenanceFake{
		actions: nil,
		marker:  nil,
	}
	clients := &awsclient.ServiceClients{RDS: fake}

	result, err := awsclient.EnrichRDSDocDBMaintenance(context.Background(), clients, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Findings) != 0 {
		t.Errorf("len(Findings) = %d, want 0", len(result.Findings))
	}
	if result.Truncated {
		t.Error("Truncated = true, want false (empty response)")
	}
}

func TestEnrichRDSDocDBMaintenance_NilClients(t *testing.T) {
	clients := &awsclient.ServiceClients{RDS: nil}
	result, err := awsclient.EnrichRDSDocDBMaintenance(context.Background(), clients, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Findings) != 0 || result.Truncated {
		t.Errorf("nil RDS client: len(Findings)=%d Truncated=%v, want 0/false", len(result.Findings), result.Truncated)
	}
}
