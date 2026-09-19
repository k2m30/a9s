//go:build integration

package integration

// scenario_mwaa_visual_test.go checks the rendered TUI output (not fetcher
// return values) for mwaa against the universal UI rules and
// docs/resources/mwaa.md.
//
// ListEnvironments returns only names. Every signal comes from the
// in-fetcher GetEnvironment pass (the eks pattern), so all findings land
// during demo startup with no separate enricher. Every issue-severity
// finding is color-bearing, so no row stays green while carrying one:
//   - CREATING/CREATING_SNAPSHOT/PENDING/UPDATING/ROLLING_BACK/MAINTENANCE → Warning
//   - CREATE_FAILED/UPDATE_FAILED/UNAVAILABLE → Broken
//   - DELETING/DELETED → Dim
//   - LastUpdate.Status==FAILED on AVAILABLE → Warning "last update failed"
//   - WebserverAccessMode public on AVAILABLE → Warning "webserver public"

import (
	"testing"

	demofixtures "github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/resource"
)

const (
	mwaaPhraseCreating      = "creating"
	mwaaPhraseSnapshotting  = "creating snapshot"
	mwaaPhrasePending       = "pending: awaiting VPC endpoints"
	mwaaPhraseUpdating      = "updating"
	mwaaPhraseRollback      = "rolling back: update failed"
	mwaaPhraseMaintenance   = "maintenance in progress"
	mwaaPhraseCreateFailed  = "create failed"
	mwaaPhraseUpdateFailed  = "update failed: rolled back"
	mwaaPhraseUnavailable   = "unavailable: not stable"
	mwaaPhraseDeleting      = "deleting"
	mwaaPhraseDeleted       = "deleted"
	mwaaPhraseStaleUpdate   = "last update failed"
	mwaaPhrasePublic        = "webserver public"
	mwaaPhraseDetailsDenied = "details denied"

	mwaaPhraseRollbackP1    = "rolling back: update failed (+1)"
	mwaaPhraseStaleUpdateP1 = "last update failed (+1)"

	// Detail-view Attention entries: the first letter is capitalized for display.
	mwaaDetailStaleUpdate = "Last update failed"
	mwaaDetailPublic      = "Webserver public"
	mwaaDetailRollback    = "Rolling back: update failed"
)

func TestScenario_MWAAVisual(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	scenario := fullIntegrationNewDemoScenario(t)
	runDemoStartup(t, scenario)

	// Menu badge — unifiedIssueCount counts issue-COLORED rows
	// (Warning + Broken). For mwaa: 6 Warning-state + 3 Broken + 3
	// background-warning + 1 details-denied = 13. Dim rows do not bump.
	scenario.ExpectMenuIssueCount("mwaa", 14)

	scenario.OpenList("mwaa")

	for _, jargon := range []string{
		"CIS", " Flags", " Issues ", "NOBKP", "UNENC", "NOPROT", "PUB ",
	} {
		scenario.ExpectViewNotContains(jargon)
	}

	for _, id := range []string{
		demofixtures.ProdAirflowEtlID,
		demofixtures.ProdAirflowReportingID,
	} {
		scenario.ExpectRowStatusBlank(id)
	}

	scenario.ExpectRowStatusEquals(demofixtures.WarnAirflowCreatingID, mwaaPhraseCreating)
	scenario.ExpectRowStatusEquals(demofixtures.WarnAirflowSnapshottingID, mwaaPhraseSnapshotting)
	scenario.ExpectRowStatusEquals(demofixtures.WarnAirflowPendingID, mwaaPhrasePending)
	scenario.ExpectRowStatusEquals(demofixtures.WarnAirflowUpdatingID, mwaaPhraseUpdating)
	scenario.ExpectRowStatusEquals(demofixtures.WarnAirflowMaintenanceID, mwaaPhraseMaintenance)
	scenario.ExpectRowStatusEquals(demofixtures.BrokenAirflowCreateFailedID, mwaaPhraseCreateFailed)
	scenario.ExpectRowStatusEquals(demofixtures.BrokenAirflowUnavailableID, mwaaPhraseUnavailable)
	scenario.ExpectRowStatusEquals(demofixtures.DimAirflowDeletingID, mwaaPhraseDeleting)
	scenario.ExpectRowStatusEquals(demofixtures.DimAirflowDeletedID, mwaaPhraseDeleted)

	scenario.ExpectRowStatusEquals(demofixtures.WarnAirflowStaleUpdateID, mwaaPhraseStaleUpdate)
	scenario.ExpectRowStatusEquals(demofixtures.WarnAirflowPublicID, mwaaPhrasePublic)
	// Listed-but-denied environment: name-only degraded row, never dropped
	// (a real IAM shape: List allowed, GetEnvironment denied).
	scenario.ExpectRowStatusEquals(demofixtures.WarnAirflowDetailsDeniedID, mwaaPhraseDetailsDenied)

	// ROLLING_BACK state + failed-update finding on the same row.
	scenario.ExpectRowStatusEquals(demofixtures.WarnAirflowRollbackID, mwaaPhraseRollbackP1)
	// UPDATE_FAILED state + failed-update finding.
	scenario.ExpectRowStatusEquals(demofixtures.BrokenAirflowUpdateFailedID, mwaaPhraseUpdateFailed+" (+1)")
	scenario.ExpectRowStatusEquals(demofixtures.WarnAirflowMultiID, mwaaPhraseStaleUpdateP1)

	for _, id := range []string{
		demofixtures.WarnAirflowStaleUpdateID,
		demofixtures.WarnAirflowPublicID,
		demofixtures.WarnAirflowMultiID,
		demofixtures.DimAirflowDeletedID,
		demofixtures.WarnAirflowCreatingID,
		demofixtures.WarnAirflowSnapshottingID,
		demofixtures.WarnAirflowPendingID,
		demofixtures.WarnAirflowUpdatingID,
		demofixtures.WarnAirflowRollbackID,
		demofixtures.WarnAirflowMaintenanceID,
		demofixtures.BrokenAirflowCreateFailedID,
		demofixtures.BrokenAirflowUpdateFailedID,
		demofixtures.BrokenAirflowUnavailableID,
		demofixtures.DimAirflowDeletingID,
		// The details-denied degraded row is color-bearing (Warning).
		demofixtures.WarnAirflowDetailsDeniedID,
	} {
		scenario.ExpectRowNoGlyphPrefix(id)
	}
	scenario.ExpectRowNoGlyphPrefix(demofixtures.ProdAirflowEtlID)
	scenario.ExpectRowNoGlyphPrefix(demofixtures.ProdAirflowReportingID)

	root := selectMWAAByID(t, scenario, demofixtures.ProdAirflowEtlID)
	scenario.OpenDetailResource("mwaa", root)
	scenario.ExpectNoAPIError()

	for displayName, atLeast := range map[string]int{
		"CW Alarms":       2,
		"KMS Key":         1,
		"Log Groups":      5,
		"IAM Roles":       1,
		"S3 Buckets":      1,
		"Security Groups": 2,
		"Subnets":         2,
	} {
		scenario.ExpectRelatedRowCountAtLeast(displayName, atLeast)
	}

	scenario.Back()

	// The failed-update Attention entry carries the ErrorMessage row.
	multi := selectMWAAByID(t, scenario, demofixtures.WarnAirflowMultiID)
	scenario.OpenDetailResource("mwaa", multi)
	scenario.ExpectNoAPIError()

	t.Log("\n" + scenario.currentView())

	scenario.ExpectViewContains(mwaaDetailStaleUpdate)
	scenario.ExpectViewContains(mwaaDetailPublic)
	scenario.ExpectViewContains("requirements.txt install failed")

	scenario.Back()

	// A finding on a non-green row still surfaces in the detail view.
	rollback := selectMWAAByID(t, scenario, demofixtures.WarnAirflowRollbackID)
	scenario.OpenDetailResource("mwaa", rollback)
	scenario.ExpectNoAPIError()
	scenario.ExpectViewContains(mwaaDetailRollback)
	scenario.ExpectViewContains(mwaaDetailStaleUpdate)

	scenario.AssertNoEnrichmentErrors()
}

// TestScenario_MWAAVisual_HealthySilence — the showroom row renders no
// Attention section and no finding phrase; disabled WebserverLogs on
// prod-airflow-reporting produces no finding and AirflowVersion is a plain
// fact.
func TestScenario_MWAAVisual_HealthySilence(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	scenario := fullIntegrationNewDemoScenario(t)
	runDemoStartup(t, scenario)
	scenario.OpenList("mwaa")

	res := selectMWAAByID(t, scenario, demofixtures.ProdAirflowReportingID)
	scenario.OpenDetailResource("mwaa", res)
	scenario.ExpectNoAPIError()
	view := scenario.currentView()
	t.Log("\n" + view)

	expectNoAttentionSection(t, view)
	for _, phrase := range []string{
		mwaaPhraseStaleUpdate, mwaaPhrasePublic, mwaaPhraseRollback,
		"SchedulerHeartbeat", "ImportErrors",
	} {
		scenario.ExpectViewNotContains(phrase)
	}
}

// selectMWAAByID looks up a concrete mwaa resource from the demo clients so
// the scenario can call OpenDetailResource with a real resource.
func selectMWAAByID(t *testing.T, s *fullIntegrationScenario, id string) resource.Resource {
	t.Helper()
	return fullIntegrationMustFindResourceByID(t, s.clients, "mwaa", id)
}
