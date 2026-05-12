package unit

// aws_dbc_issue_enrichment_test.go — Wave 2 enricher tests for dbc.
//
// AS-140 (Wave-2 enricher migration): FieldUpdates["status"] is no longer
// written by EnrichDBCMaintenance. The merged §4 status phrase is computed
// at render time by phraseFromFindings(r.Findings) in extractCellValue.
//
// Tests drive aws.EnrichDBCMaintenance (the dbc-specific enricher) and assert:
//   - Findings keyed by Resource.ID (ARN suffix-matched, "cluster" segment only).
//   - Severity "!" (S1-badge-bumping — different from dbi's "~").
//   - IssueCount increments for each overdue finding.
//   - Summary is "maintenance overdue" (short phrase; rows carry concrete detail).
//   - FieldUpdates is nil or empty in every case (AS-140: no status overlay,
//     no "(+N)" suffix arithmetic on the enricher side).
//   - Future-dated actions (not yet overdue) are NOT emitted.
//   - nil DocDB client returns empty result gracefully.
//   - Instance ARNs ("db:" prefix) are filtered out — only "cluster:" matches.
//   - Multi-page response: both pages are accumulated.

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/docdb"
	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// fake — implements DocDBAPI; all methods return stubs except
// DescribePendingMaintenanceActions, which is under test control.
// ---------------------------------------------------------------------------

type dbcMaintenanceFake struct {
	pages [][]docdbtypes.ResourcePendingMaintenanceActions
	call  int
	err   error
}

func (f *dbcMaintenanceFake) DescribeDBClusters(
	_ context.Context,
	_ *docdb.DescribeDBClustersInput,
	_ ...func(*docdb.Options),
) (*docdb.DescribeDBClustersOutput, error) {
	return &docdb.DescribeDBClustersOutput{}, nil
}

func (f *dbcMaintenanceFake) DescribeDBClusterSnapshots(
	_ context.Context,
	_ *docdb.DescribeDBClusterSnapshotsInput,
	_ ...func(*docdb.Options),
) (*docdb.DescribeDBClusterSnapshotsOutput, error) {
	return &docdb.DescribeDBClusterSnapshotsOutput{}, nil
}

func (f *dbcMaintenanceFake) DescribeDBSubnetGroups(
	_ context.Context,
	_ *docdb.DescribeDBSubnetGroupsInput,
	_ ...func(*docdb.Options),
) (*docdb.DescribeDBSubnetGroupsOutput, error) {
	return &docdb.DescribeDBSubnetGroupsOutput{}, nil
}

func (f *dbcMaintenanceFake) DescribePendingMaintenanceActions(
	_ context.Context,
	_ *docdb.DescribePendingMaintenanceActionsInput,
	_ ...func(*docdb.Options),
) (*docdb.DescribePendingMaintenanceActionsOutput, error) {
	if f.err != nil {
		return nil, f.err
	}
	if len(f.pages) == 0 {
		return &docdb.DescribePendingMaintenanceActionsOutput{}, nil
	}
	idx := f.call
	if idx >= len(f.pages) {
		return &docdb.DescribePendingMaintenanceActionsOutput{}, nil
	}
	f.call++
	var marker *string
	if f.call < len(f.pages) {
		marker = aws.String("next")
	}
	return &docdb.DescribePendingMaintenanceActionsOutput{
		PendingMaintenanceActions: f.pages[idx],
		Marker:                    marker,
	}, nil
}

// Compile-time check: dbcMaintenanceFake satisfies DocDBAPI.
var _ awsclient.DocDBAPI = (*dbcMaintenanceFake)(nil)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// buildDBCResources converts all DBCFixtures.DBClusters into []resource.Resource
// by running them through FetchDocDBClustersPage so Resource.Status/Fields are
// correctly derived by the fetcher.
func buildDBCResources(t *testing.T) []resource.Resource {
	t.Helper()
	fix := fixtures.NewDBCFixtures()
	mock := singlePageDocDB(fix.DBClusters)
	result, err := awsclient.FetchDocDBClustersPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("buildDBCResources: FetchDocDBClustersPage error: %v", err)
	}
	return result.Resources
}

// pastDate returns a time in the past used as AutoAppliedAfterDate.
func pastDate() *time.Time {
	t := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	return &t
}

// futureDate returns a time far in the future.
func futureDate() *time.Time {
	t := time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)
	return &t
}

// dbcFindingKeys returns all keys in the findings map (for error reporting).
func dbcFindingKeys(m map[string]resource.EnrichmentFinding) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestDBC_Enrich_MaintenanceOverdue_HealthyRow verifies the full Wave 2
// contract for the MaintDbcOverdueID fixture (healthy + overdue maintenance).
func TestDBC_Enrich_MaintenanceOverdue_HealthyRow(t *testing.T) {
	resources := buildDBCResources(t)
	fix := fixtures.NewDBCFixtures()

	fake := &dbcMaintenanceFake{
		pages: [][]docdbtypes.ResourcePendingMaintenanceActions{fix.PendingMaintenanceActions},
	}
	clients := &awsclient.ServiceClients{DocDB: fake}

	result, err := awsclient.EnrichDBCMaintenance(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("EnrichDBCMaintenance error: %v", err)
	}

	finding, ok := result.Findings[fixtures.MaintDbcOverdueID]
	if !ok {
		t.Fatalf("expected finding for %q; Findings keys = %v", fixtures.MaintDbcOverdueID, dbcFindingKeys(result.Findings))
	}

	// Severity "!" — DBC maintenance is S1-badge-bumping (unlike DBI's "~").
	if finding.Severity != "!" {
		t.Errorf("Severity = %q, want %q", finding.Severity, "!")
	}

	// Summary is the short S5 phrase.
	if finding.Summary != "maintenance overdue" {
		t.Errorf("Summary = %q, want %q", finding.Summary, "maintenance overdue")
	}

	// "!" findings increment the issue count.
	if result.IssueCount < 1 {
		t.Errorf("IssueCount = %d, want ≥1 (! severity bumps badge)", result.IssueCount)
	}

	// AS-140: FieldUpdates must be nil/empty — the merged display phrase is
	// computed by phraseFromFindings(r.Findings) at render time.
	if updates, ok := result.FieldUpdates[fixtures.MaintDbcOverdueID]; ok && len(updates) != 0 {
		t.Errorf("AS-140: expected empty FieldUpdates for %q (status overlay removed); got %v", fixtures.MaintDbcOverdueID, updates)
	}
}

// TestDBC_Enrich_FutureDate_NoFinding verifies that when AutoAppliedAfterDate
// is in the future, the enricher does NOT emit a finding (not yet overdue).
func TestDBC_Enrich_FutureDate_NoFinding(t *testing.T) {
	const clusterID = "future-dbc"
	const arn = "arn:aws:rds:us-east-1:123456789012:cluster:" + clusterID

	fake := &dbcMaintenanceFake{
		pages: [][]docdbtypes.ResourcePendingMaintenanceActions{
			{
				{
					ResourceIdentifier: aws.String(arn),
					PendingMaintenanceActionDetails: []docdbtypes.PendingMaintenanceAction{
						{Action: aws.String("os-upgrade"), AutoAppliedAfterDate: futureDate()},
					},
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{DocDB: fake}
	resources := []resource.Resource{
		{ID: clusterID, Status: "", Fields: map[string]string{"status": ""}},
	}

	result, err := awsclient.EnrichDBCMaintenance(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("EnrichDBCMaintenance error: %v", err)
	}
	if _, ok := result.Findings[clusterID]; ok {
		t.Errorf("expected no finding for future-dated action; got one for %q", clusterID)
	}
	if result.IssueCount != 0 {
		t.Errorf("IssueCount = %d, want 0 (future-dated action not overdue)", result.IssueCount)
	}
}

// TestDBC_Enrich_InstanceARN_Filtered verifies that maintenance ARNs whose
// resource-type segment is "db" (RDS instances) are filtered out — only
// "cluster:" ARNs match the dbc enricher.
func TestDBC_Enrich_InstanceARN_Filtered(t *testing.T) {
	const clusterID = "actual-cluster"
	const clusterARN = "arn:aws:rds:us-east-1:123456789012:cluster:" + clusterID
	const instanceARN = "arn:aws:rds:us-east-1:123456789012:db:some-instance"

	fake := &dbcMaintenanceFake{
		pages: [][]docdbtypes.ResourcePendingMaintenanceActions{
			{
				// Instance ARN — must be filtered.
				{
					ResourceIdentifier: aws.String(instanceARN),
					PendingMaintenanceActionDetails: []docdbtypes.PendingMaintenanceAction{
						{Action: aws.String("os-upgrade"), AutoAppliedAfterDate: pastDate()},
					},
				},
				// Cluster ARN — must match.
				{
					ResourceIdentifier: aws.String(clusterARN),
					PendingMaintenanceActionDetails: []docdbtypes.PendingMaintenanceAction{
						{Action: aws.String("system-update"), AutoAppliedAfterDate: pastDate()},
					},
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{DocDB: fake}
	resources := []resource.Resource{
		{ID: clusterID, Status: "", Fields: map[string]string{"status": ""}},
		// The instance is NOT in the dbc resource list.
	}

	result, err := awsclient.EnrichDBCMaintenance(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("EnrichDBCMaintenance error: %v", err)
	}
	// Cluster ARN → finding.
	if _, ok := result.Findings[clusterID]; !ok {
		t.Errorf("expected finding for cluster %q; Findings = %v", clusterID, dbcFindingKeys(result.Findings))
	}
	// Instance ARN → no extra finding.
	if len(result.Findings) != 1 {
		t.Errorf("expected exactly 1 finding (cluster only), got %d: %v", len(result.Findings), dbcFindingKeys(result.Findings))
	}
}

// TestDBC_Enrich_NilDocDBClient verifies graceful degradation when the DocDB
// client is nil — returns an empty result without error.
func TestDBC_Enrich_NilDocDBClient(t *testing.T) {
	clients := &awsclient.ServiceClients{DocDB: nil}
	result, err := awsclient.EnrichDBCMaintenance(context.Background(), clients, nil, nil)
	if err != nil {
		t.Fatalf("EnrichDBCMaintenance error: %v", err)
	}
	if result.Findings == nil {
		t.Error("Findings must not be nil even when DocDB client is nil")
	}
	if result.IssueCount != 0 {
		t.Errorf("IssueCount = %d, want 0", result.IssueCount)
	}
}

// TestDBC_Enrich_Wave1PlusWave2_NoFieldUpdates verifies AS-140: when the
// fetcher already populated Status (Wave 1 warning), the Wave-2 maintenance
// Finding is still emitted with severity "!" but FieldUpdates is left empty.
// The merged "no automated backups (+1)" display is computed at render time
// by phraseFromFindings(r.Findings).
func TestDBC_Enrich_Wave1PlusWave2_NoFieldUpdates(t *testing.T) {
	const clusterID = fixtures.WarnDbcNoBkpMaintID
	const arn = fixtures.WarnDbcNoBkpMaintARN

	fake := &dbcMaintenanceFake{
		pages: [][]docdbtypes.ResourcePendingMaintenanceActions{
			{
				{
					ResourceIdentifier: aws.String(arn),
					PendingMaintenanceActionDetails: []docdbtypes.PendingMaintenanceAction{
						{Action: aws.String("system-update"), AutoAppliedAfterDate: pastDate()},
					},
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{DocDB: fake}
	// Simulate Wave-1 status set by the fetcher for the no-backup cluster.
	resources := []resource.Resource{
		{
			ID:     clusterID,
			Name:   clusterID,
			Status: "no automated backups",
			Fields: map[string]string{"status": "no automated backups"},
		},
	}

	result, err := awsclient.EnrichDBCMaintenance(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("EnrichDBCMaintenance error: %v", err)
	}

	// Finding present with "!" severity.
	finding, ok := result.Findings[clusterID]
	if !ok {
		t.Fatalf("expected finding for %q; Findings = %v", clusterID, dbcFindingKeys(result.Findings))
	}
	if finding.Severity != "!" {
		t.Errorf("Severity = %q, want %q", finding.Severity, "!")
	}

	// AS-140: FieldUpdates must be empty.
	if updates, ok := result.FieldUpdates[clusterID]; ok && len(updates) != 0 {
		t.Errorf("AS-140: expected empty FieldUpdates for %q (status overlay removed); got %v", clusterID, updates)
	}
}

// TestDBC_Enrich_NoMatchNoFinding verifies that maintenance actions for ARNs
// not present in the resources slice produce no findings.
func TestDBC_Enrich_NoMatchNoFinding(t *testing.T) {
	fake := &dbcMaintenanceFake{
		pages: [][]docdbtypes.ResourcePendingMaintenanceActions{
			{
				{
					ResourceIdentifier: aws.String("arn:aws:rds:us-east-1:123456789012:cluster:not-in-resources"),
					PendingMaintenanceActionDetails: []docdbtypes.PendingMaintenanceAction{
						{Action: aws.String("os-upgrade"), AutoAppliedAfterDate: pastDate()},
					},
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{DocDB: fake}
	resources := []resource.Resource{
		{ID: "other-cluster", Status: "", Fields: map[string]string{}},
	}

	result, err := awsclient.EnrichDBCMaintenance(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("EnrichDBCMaintenance error: %v", err)
	}
	if len(result.Findings) != 0 {
		t.Errorf("Findings must be empty when no ARN matches; got %v", result.Findings)
	}
}

// TestDBC_Enrich_Pagination verifies that findings from both pages of a
// two-page DescribePendingMaintenanceActions response are processed.
func TestDBC_Enrich_Pagination(t *testing.T) {
	const cluster1 = "dbc-page1"
	const cluster2 = "dbc-page2"
	const arn1 = "arn:aws:rds:us-east-1:123456789012:cluster:" + cluster1
	const arn2 = "arn:aws:rds:us-east-1:123456789012:cluster:" + cluster2

	page1 := []docdbtypes.ResourcePendingMaintenanceActions{
		{
			ResourceIdentifier: aws.String(arn1),
			PendingMaintenanceActionDetails: []docdbtypes.PendingMaintenanceAction{
				{Action: aws.String("os-upgrade"), AutoAppliedAfterDate: pastDate()},
			},
		},
	}
	page2 := []docdbtypes.ResourcePendingMaintenanceActions{
		{
			ResourceIdentifier: aws.String(arn2),
			PendingMaintenanceActionDetails: []docdbtypes.PendingMaintenanceAction{
				{Action: aws.String("system-update"), AutoAppliedAfterDate: pastDate()},
			},
		},
	}

	fake := &dbcMaintenanceFake{pages: [][]docdbtypes.ResourcePendingMaintenanceActions{page1, page2}}
	clients := &awsclient.ServiceClients{DocDB: fake}
	resources := []resource.Resource{
		{ID: cluster1, Status: "", Fields: map[string]string{"status": ""}},
		{ID: cluster2, Status: "", Fields: map[string]string{"status": ""}},
	}

	result, err := awsclient.EnrichDBCMaintenance(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("EnrichDBCMaintenance error: %v", err)
	}
	if _, ok := result.Findings[cluster1]; !ok {
		t.Error("expected finding for dbc-page1 (from page 1)")
	}
	if _, ok := result.Findings[cluster2]; !ok {
		t.Error("expected finding for dbc-page2 (from page 2)")
	}
	if result.IssueCount != 2 {
		t.Errorf("IssueCount = %d, want 2 (two overdue clusters)", result.IssueCount)
	}
}

// TestDBC_Enrich_FindingRows verifies that Action, Apply Method, Earliest Target,
// and Description are emitted as rows under the finding.
func TestDBC_Enrich_FindingRows(t *testing.T) {
	const clusterID = "rows-cluster"
	const arn = "arn:aws:rds:us-east-1:123456789012:cluster:" + clusterID
	overdueTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	fake := &dbcMaintenanceFake{
		pages: [][]docdbtypes.ResourcePendingMaintenanceActions{
			{
				{
					ResourceIdentifier: aws.String(arn),
					PendingMaintenanceActionDetails: []docdbtypes.PendingMaintenanceAction{
						{
							Action:               aws.String("os-upgrade"),
							Description:          aws.String("Security patch"),
							OptInStatus:          aws.String("opt-in-not-required"),
							AutoAppliedAfterDate: &overdueTime,
						},
					},
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{DocDB: fake}
	resources := []resource.Resource{
		{ID: clusterID, Status: "", Fields: map[string]string{"status": ""}},
	}

	result, err := awsclient.EnrichDBCMaintenance(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("EnrichDBCMaintenance error: %v", err)
	}
	finding, ok := result.Findings[clusterID]
	if !ok {
		t.Fatalf("expected finding for %q", clusterID)
	}

	gotRows := map[string]string{}
	for _, row := range finding.Rows {
		gotRows[row.Label] = row.Value
	}

	if gotRows["Action"] != "os-upgrade" {
		t.Errorf("Rows[Action] = %q, want %q", gotRows["Action"], "os-upgrade")
	}
	if gotRows["Description"] != "Security patch" {
		t.Errorf("Rows[Description] = %q, want %q", gotRows["Description"], "Security patch")
	}
	if gotRows["Apply Method"] != "opt-in-not-required" {
		t.Errorf("Rows[Apply Method] = %q, want %q", gotRows["Apply Method"], "opt-in-not-required")
	}
	if gotRows["Earliest Target"] == "" {
		t.Errorf("Rows[Earliest Target] must not be empty for overdue action")
	}
}

// TestDBC_Enrich_ForcedApplyDate_Overdue verifies that ForcedApplyDate in the
// past also triggers the overdue finding (not just AutoAppliedAfterDate).
func TestDBC_Enrich_ForcedApplyDate_Overdue(t *testing.T) {
	const clusterID = "forced-overdue-cluster"
	const arn = "arn:aws:rds:us-east-1:123456789012:cluster:" + clusterID
	past := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	fake := &dbcMaintenanceFake{
		pages: [][]docdbtypes.ResourcePendingMaintenanceActions{
			{
				{
					ResourceIdentifier: aws.String(arn),
					PendingMaintenanceActionDetails: []docdbtypes.PendingMaintenanceAction{
						{
							Action:          aws.String("system-update"),
							ForcedApplyDate: &past,
							// AutoAppliedAfterDate is nil — ForcedApplyDate must still trigger.
						},
					},
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{DocDB: fake}
	resources := []resource.Resource{
		{ID: clusterID, Status: "", Fields: map[string]string{"status": ""}},
	}

	result, err := awsclient.EnrichDBCMaintenance(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("EnrichDBCMaintenance error: %v", err)
	}
	if _, ok := result.Findings[clusterID]; !ok {
		t.Errorf("expected finding for ForcedApplyDate-overdue cluster %q; Findings = %v", clusterID, dbcFindingKeys(result.Findings))
	}
}
