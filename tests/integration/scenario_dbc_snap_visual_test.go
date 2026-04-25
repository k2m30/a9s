//go:build integration

package integration

// scenario_dbc_snap_visual_test.go — Phase-8 render-gate for dbc-snap.
//
// Mirrors scenario_dbi_snap_visual_test.go's coverage of the cross-ref Wave-1
// signals (orphan + past-retention) emitted by the SnapshotCrossRef helper
// instantiated for dbc-snap. dbc-snap covers BOTH DocumentDB and Aurora cluster
// snapshots — they share DescribeDBClusterSnapshots — so the test fixtures
// include one of each.

import (
	"testing"

	demofixtures "github.com/k2m30/a9s/v3/internal/demo/fixtures"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// TestScenario_DBCSnapVisual_CrossRefSignals pins the orphan and past-retention
// phrases emitted by the dbc-snap cross-ref enricher to the rendered Status
// column. This is the regression pin against silent enricher regressions.
func TestScenario_DBCSnapVisual_CrossRefSignals(t *testing.T) {
	scenario := fullIntegrationNewDemoScenario(t)
	runDemoStartup(t, scenario)

	scenario.OpenList("dbc-snap")

	// Universal column rules — no jargon columns.
	for _, jargon := range []string{"CIS", "NOBKP", "UNENC", "NOPROT", "Flags", "Policy"} {
		scenario.ExpectViewNotContains(jargon)
	}

	// Cross-ref Wave-1: orphan — parent cluster missing from dbc cache.
	scenario.ExpectRowStatusEquals(demofixtures.WarnDBCSnapOrphanID, "orphan: source cluster deleted")

	// Cross-ref Wave-1: past-retention — automated, age=30d, parent retention=7.
	// Days-over is now-relative, so assert the phrase appears anywhere in the
	// rendered list view instead of pinning an exact day count.
	scenario.ExpectViewContains("past retention")

	// Healthy fixtures (Aurora + DocDB cluster snapshots whose parent is
	// in the dbc cache and within retention) should render blank Status.
	scenario.ExpectRowStatusBlank(demofixtures.ProdDBCSnapAuroraID)

	t.Log("\n" + scenario.currentView())
}

// TestScenario_DBCSnapVisual_DetailSurfacesAttention asserts the Attention
// section renders the cross-ref finding in the dbc-snap detail view (S5).
func TestScenario_DBCSnapVisual_DetailSurfacesAttention(t *testing.T) {
	scenario := fullIntegrationNewDemoScenario(t)
	runDemoStartup(t, scenario)
	scenario.OpenList("dbc-snap")

	cases := []struct {
		id      string
		summary string
		row     string
	}{
		{
			id:      demofixtures.WarnDBCSnapOrphanID,
			summary: "Orphan: source cluster deleted",
			row:     "deleted-legacy-cluster",
		},
		{
			id:      demofixtures.WarnDBCSnapPastRetentionID,
			summary: "past retention",
			row:     demofixtures.ProdDbcID,
		},
	}

	for _, tc := range cases {
		t.Run(tc.id, func(t *testing.T) {
			res := selectDBCSnapByID(t, scenario, tc.id)
			scenario.OpenDetailResource("dbc-snap", res)
			scenario.ExpectNoAPIError()
			view := scenario.currentView()
			t.Log("\n" + view)

			scenario.ExpectViewContains(tc.summary)
			scenario.ExpectViewContains(tc.row)
			scenario.Back()
		})
	}
}

// selectDBCSnapByID looks up a concrete dbc-snap resource for OpenDetailResource.
func selectDBCSnapByID(t *testing.T, s *fullIntegrationScenario, id string) resource.Resource {
	t.Helper()
	return fullIntegrationMustFindResourceByID(t, s.clients, "dbc-snap", id)
}

// TestScenario_DBCSnapVisual_FailedPlusManualOldStacks pins the multi-signal
// visual rendering for WarnDBCSnapFailedAndManualOldID:
//
//   Status="failed" + SnapshotType="manual" + age=400d + parent="deleted-legacy-cluster"
//   → two Wave-1 signals: failed (Broken) + orphan (Warning); manual-age is suppressed
//   → rendered status column: "failed (+1)"
//
// Broken (failed) wins precedence; the manual-age Warning is suppressed;
// the orphan cross-ref still stacks → 'failed (+1)'.
//
// Also asserts that WarnDBCSnapIncompatibleRestoreID renders
// "incompatible-restore" (single Broken phrase, no multi-suffix).
//
// These tests FAIL until the coder ships:
//   - ComputeDBCSnapStatusAndIssues in internal/aws/dbc_snap.go
//   - WarnDBCSnapFailedAndManualOldID fixture in internal/demo/fixtures/dbc.go
//   - WarnDBCSnapIncompatibleRestoreID fixture in internal/demo/fixtures/dbc.go
func TestScenario_DBCSnapVisual_FailedPlusManualOldStacks(t *testing.T) {
	scenario := fullIntegrationNewDemoScenario(t)
	runDemoStartup(t, scenario)

	scenario.OpenList("dbc-snap")

	// Two signals: failed (Broken) + orphan (Warning); manual-age suppressed by
	// Broken precedence → top="failed", suffix=(+1).
	view := scenario.currentView()
	failedMultiID := demofixtures.WarnDBCSnapFailedAndManualOldID

	scenario.ExpectRowStatusEquals(failedMultiID, "failed (+1)")

	// WarnDBCSnapIncompatibleRestoreID: single Broken phrase, no suffix.
	scenario.ExpectRowStatusEquals(demofixtures.WarnDBCSnapIncompatibleRestoreID, "incompatible-restore")

	_ = view // used for logging below

	// -----------------------------------------------------------------
	// Detail view: Attention section must carry the Broken phrase and the
	// orphan cross-ref finding. The manual-age Warning is suppressed under
	// Broken precedence and must NOT appear.
	// -----------------------------------------------------------------
	res := selectDBCSnapByID(t, scenario, failedMultiID)
	scenario.OpenDetailResource("dbc-snap", res)
	scenario.ExpectNoAPIError()
	detailView := scenario.currentView()
	t.Log("\n" + detailView)

	// "Failed" (Broken) and the orphan cross-ref appear; "manual, unused" does not.
	expectAttentionSection(t, detailView, []string{
		"Failed",                        // Broken phrase (capitalized by injectAttentionSection)
		"Orphan: source cluster deleted", // cross-ref finding Summary (capitalized by injectAttentionSection)
		"deleted-legacy-cluster",        // orphan parent identifier in the Source Cluster row
	})

	scenario.Back()
	t.Log("\n" + scenario.currentView())
}

