package unit

// qa_bug_issue_count_test.go — Tests for three bugs found in live testing:
// 1. Main menu FrameTitle under ctrl+z should show filtered/total count
// 2. RDS enricher must count only resources that match probed resources, not all maintenance ARNs
// 3. (Consequence of #2 — correct enricher count means correct badge)

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ---------------------------------------------------------------------------
// Bug 1: Main menu FrameTitle should show filtered/total when ctrl+z active
// ---------------------------------------------------------------------------

func TestMainMenuFrameTitle_CtrlZShowsFilteredCount(t *testing.T) {
	m := views.NewMainMenu(keys.Default())
	total := len(resource.AllResourceTypes())

	// Mark ec2 and rds as having issues, everything else as zero
	m.SetIssues("ec2", 1, false)
	m.SetIssues("dbi", 2, false)
	for _, rt := range resource.AllResourceTypes() {
		if rt.ShortName != "ec2" && rt.ShortName != "dbi" {
			m.SetIssues(rt.ShortName, 0, false)
		}
	}

	// Enable ctrl+z
	m.Toggle()
	// Re-trigger applyFilter
	m.SetIssues("ec2", 1, false)

	title := m.FrameTitle()

	// Should show something like "resource-types(2/66) [!]"
	// The 2 is the number of visible (issue) types, 66 is total
	expectedFiltered := fmt.Sprintf("2/%d", total)
	if !strings.Contains(title, expectedFiltered) {
		t.Errorf("FrameTitle() = %q, want to contain %q (filtered/total)", title, expectedFiltered)
	}
	if !strings.Contains(title, "[!]") {
		t.Errorf("FrameTitle() = %q, want '[!]'", title)
	}
}

// ---------------------------------------------------------------------------
// Bug 2: RDS enricher counts all maintenance ARNs, not matching resources
// ---------------------------------------------------------------------------

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

func TestEnrichRDSDocDBMaintenance_CountsOnlyMatchingResources(t *testing.T) {
	fake := &rdsMaintenanceBugFake{}
	clients := &awsclient.ServiceClients{RDS: fake}

	// The probed resources for "dbi" are the 2 DB instances
	probeResources := []resource.Resource{
		{ID: "docdb-docdb-dev", Name: "docdb-docdb-dev"},
		{ID: "rds-eu-west-2-dev-instance", Name: "rds-eu-west-2-dev-instance"},
	}

	count, _, err := awsclient.EnrichRDSDocDBMaintenance(context.Background(), clients, probeResources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should be 2 (only the instances that match our probed resources), NOT 4
	if count != 2 {
		t.Errorf("EnrichRDSDocDBMaintenance count = %d, want 2 (only matching probed resources, not all 4 ARNs)", count)
	}
}

func TestEnrichRDSDocDBMaintenance_NoMatchingResources(t *testing.T) {
	fake := &rdsMaintenanceBugFake{}
	clients := &awsclient.ServiceClients{RDS: fake}

	// Probed resources don't match any maintenance ARNs
	probeResources := []resource.Resource{
		{ID: "unrelated-instance", Name: "unrelated-instance"},
	}

	count, _, err := awsclient.EnrichRDSDocDBMaintenance(context.Background(), clients, probeResources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if count != 0 {
		t.Errorf("count = %d, want 0 (no matching resources)", count)
	}
}

func TestEnrichRDSDocDBMaintenance_IssueCountNeverExceedsResourceCount(t *testing.T) {
	fake := &rdsMaintenanceBugFake{}
	clients := &awsclient.ServiceClients{RDS: fake}

	probeResources := []resource.Resource{
		{ID: "docdb-docdb-dev", Name: "docdb-docdb-dev"},
		{ID: "rds-eu-west-2-dev-instance", Name: "rds-eu-west-2-dev-instance"},
	}

	count, _, err := awsclient.EnrichRDSDocDBMaintenance(context.Background(), clients, probeResources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if count > len(probeResources) {
		t.Errorf("issue count %d exceeds resource count %d — enricher must not count more issues than resources", count, len(probeResources))
	}
}
