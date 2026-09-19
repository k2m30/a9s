package unit

// The dbi maintenance enricher counts only the
// probed resources, not every pending-maintenance ARN in the account.

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

// rdsMaintenanceBugFake returns 4 pending maintenance actions (2 clusters + 2 instances)
// but only 2 of them match the probed "dbi" resources.
type rdsMaintenanceBugFake struct {
	awsclient.RDSAPI
}

func (f *rdsMaintenanceBugFake) DescribePendingMaintenanceActions(_ context.Context, _ *rds.DescribePendingMaintenanceActionsInput, _ ...func(*rds.Options)) (*rds.DescribePendingMaintenanceActionsOutput, error) {
	return &rds.DescribePendingMaintenanceActionsOutput{
		PendingMaintenanceActions: []rdstypes.ResourcePendingMaintenanceActions{
			// Two clusters — should NOT count for "dbi" resource type
			{ResourceIdentifier: aws.String("arn:aws:rds:eu-west-2:123456789012:cluster:docdb-cluster-dev")},
			{ResourceIdentifier: aws.String("arn:aws:rds:eu-west-2:123456789012:cluster:rds-eu-west-2-dev")},
			// Two instances — SHOULD count for "dbi" resource type
			{ResourceIdentifier: aws.String("arn:aws:rds:eu-west-2:123456789012:db:docdb-docdb-dev")},
			{ResourceIdentifier: aws.String("arn:aws:rds:eu-west-2:123456789012:db:rds-eu-west-2-dev-instance")},
		},
	}, nil
}

func TestEnrichDBIMaintenance_FindingsContainMatchingResources(t *testing.T) {
	fake := &rdsMaintenanceBugFake{}
	clients := &awsclient.ServiceClients{RDS: fake}

	probeResources := []resource.Resource{
		{ID: "docdb-docdb-dev", Name: "docdb-docdb-dev"},
		{ID: "rds-eu-west-2-dev-instance", Name: "rds-eu-west-2-dev-instance"},
	}

	result, err := awsclient.EnrichDBIMaintenance(context.Background(), clients, probeResources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// dbi enricher is account-wide: verify the probed instance IDs ARE
	// present in Findings (the key contract) and cluster ARNs are excluded.
	for _, r := range probeResources {
		if _, ok := result.Findings[r.ID]; !ok {
			t.Errorf("expected Findings to contain probed resource %q", r.ID)
		}
	}
	if len(result.Findings) != len(probeResources) {
		t.Errorf("len(Findings) = %d, want %d (cluster ARNs must be excluded by isInstanceARN)", len(result.Findings), len(probeResources))
	}
}

func TestEnrichDBIMaintenance_UnprobedResourcesDoNotAppear(t *testing.T) {
	fake := &rdsMaintenanceBugFake{}
	clients := &awsclient.ServiceClients{RDS: fake}

	probeResources := []resource.Resource{
		{ID: "unrelated-instance", Name: "unrelated-instance"},
	}

	result, err := awsclient.EnrichDBIMaintenance(context.Background(), clients, probeResources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := result.Findings["unrelated-instance"]; ok {
		t.Error("unrelated-instance must NOT appear in Findings — no matching maintenance action")
	}
	if len(result.Findings) != 0 {
		t.Errorf("len(Findings) = %d, want 0 (no probed resource matches any maintenance ARN)", len(result.Findings))
	}
}
