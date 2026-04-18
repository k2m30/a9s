package unit

// qa_enrich_rds_finding_key_test.go — regression tests for Bug B2:
// EnrichRDSDocDBMaintenance emits findings for cluster ARNs that do not match
// any probed instance ID, via the arnSuffix fallback. This bloats the banner
// count and prevents the detail view Background Check section from appearing
// (detail lookup uses resource.ID, which never matches a cluster ARN suffix).

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
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
			// 2 instance ARNs — IDs match probeResources
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
			// 2 cluster ARNs — NOT in probeResources, should not produce findings
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

// TestEnrichRDSDocDBMaintenance_OnlyEmitsForProbedResources verifies that the
// enricher only creates findings for resources whose ID matches a probed instance.
// Cluster ARNs not in probeResources must not produce findings via arnSuffix fallback.
// FAILS today: current code uses arnSuffix fallback → len(findings)==4 not 2.
func TestEnrichRDSDocDBMaintenance_OnlyEmitsForProbedResources(t *testing.T) {
	fake := &rdsFindingKeyFake{}
	clients := &awsclient.ServiceClients{RDS: fake}
	probeResources := []resource.Resource{
		{ID: "rds-instance-a"},
		{ID: "rds-instance-b"},
	}

	result, err := awsclient.EnrichRDSDocDBMaintenance(context.Background(), clients, probeResources)
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
	if result.IssueCount != 0 {
		t.Errorf("IssueCount = %d, want 0 (~ findings do not count toward badge)", result.IssueCount)
	}
	if len(result.Findings) != 2 {
		t.Errorf("len(Findings) = %d, want 2 (one finding per probed resource)", len(result.Findings))
	}
}

