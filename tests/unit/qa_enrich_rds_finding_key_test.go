package unit

// EnrichDBIMaintenance emits findings only for probed dbi instance IDs.
// Cluster ARNs (dbc) never become findings: they would inflate the banner
// count, and the detail view's Background Check section looks findings up
// by resource.ID, which never matches a cluster ARN suffix.

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

// rdsFindingKeyFake returns 4 pending maintenance actions:
// 2 matching probed instance IDs, 2 for clusters not in probeResources.
type rdsFindingKeyFake struct {
	awsclient.RDSAPI
}

func (f *rdsFindingKeyFake) DescribePendingMaintenanceActions(
	_ context.Context,
	_ *rds.DescribePendingMaintenanceActionsInput,
	_ ...func(*rds.Options),
) (*rds.DescribePendingMaintenanceActionsOutput, error) {
	return &rds.DescribePendingMaintenanceActionsOutput{
		PendingMaintenanceActions: []rdstypes.ResourcePendingMaintenanceActions{
			{
				ResourceIdentifier: aws.String("arn:aws:rds:eu-west-2:123456789012:db:rds-instance-a"),
				PendingMaintenanceActionDetails: []rdstypes.PendingMaintenanceAction{
					{Action: aws.String("system-update")},
				},
			},
			{
				ResourceIdentifier: aws.String("arn:aws:rds:eu-west-2:123456789012:db:rds-instance-b"),
				PendingMaintenanceActionDetails: []rdstypes.PendingMaintenanceAction{
					{Action: aws.String("os-upgrade")},
				},
			},
			{
				ResourceIdentifier: aws.String("arn:aws:rds:eu-west-2:123456789012:cluster:docdb-cluster-dev"),
				PendingMaintenanceActionDetails: []rdstypes.PendingMaintenanceAction{
					{Action: aws.String("system-update")},
				},
			},
			{
				ResourceIdentifier: aws.String("arn:aws:rds:eu-west-2:123456789012:cluster:rds-cluster-prod"),
				PendingMaintenanceActionDetails: []rdstypes.PendingMaintenanceAction{
					{Action: aws.String("engine-version-upgrade")},
				},
			},
		},
	}, nil
}

// TestEnrichDBIMaintenance_OnlyEmitsForProbedResources verifies that the
// enricher only creates findings for resources whose ID matches a probed instance.
// Cluster ARNs must not produce findings via arnSuffix fallback.
func TestEnrichDBIMaintenance_OnlyEmitsForProbedResources(t *testing.T) {
	fake := &rdsFindingKeyFake{}
	clients := &awsclient.ServiceClients{RDS: fake}
	probeResources := []resource.Resource{
		{ID: "rds-instance-a"},
		{ID: "rds-instance-b"},
	}

	result, err := awsclient.EnrichDBIMaintenance(context.Background(), clients, probeResources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Findings) != 2 {
		t.Errorf("len(Findings) = %d, want 2 (only probed instances, not cluster ARN suffixes)", len(result.Findings))
	}
	for _, r := range probeResources {
		if _, ok := result.Findings[r.ID]; !ok {
			t.Errorf("expected finding for probed resource %q", r.ID)
		}
	}
	if _, ok := result.Findings["docdb-cluster-dev"]; ok {
		t.Error("finding for unprobed cluster ARN suffix must NOT appear in Findings")
	}
	if _, ok := result.Findings["rds-cluster-prod"]; ok {
		t.Error("finding for unprobed cluster ARN suffix must NOT appear in Findings")
	}
	if len(result.Findings) != 2 {
		t.Errorf("len(Findings) = %d, want 2 (one finding per probed resource)", len(result.Findings))
	}
}
