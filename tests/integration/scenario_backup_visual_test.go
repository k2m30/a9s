//go:build integration

package integration

// scenario_backup_visual_test.go checks the rendered TUI output for backup
// against the universal UI rules and docs/resources/backup.md.
//
// backup has two Wave-2 signals:
//   • FAILED/EXPIRED/ABORTED jobs in last 24h → Broken row color.
//   • PARTIAL jobs in last 24h  → Warning row color.
// colorBackup (catalog_backup.go) resolves color via colorFromAnyFinding, so
// every plan with a finding renders its Broken/Warning row color directly;
// the `!`/`~` name-glyph is reserved for findings that land on an
// otherwise-Healthy row.

import (
	"testing"

	demofixtures "github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/resource"
)

const (
	backupS4Broken1 = "1 job failed in last 24h"
	backupS4Broken2 = "2 jobs failed in last 24h"
	backupS4Partial = "partial: 1 of 3 resources skipped"

	backupDetailBroken2Capitalize = "2 jobs failed in last 24h"
	backupDetailPartialCapitalize = "Partial: 1 of 3 resources skipped"

	// Glyph-prefix assertions key on `"<glyph> <NAME>"` because the glyph
	// renders adjacent to the name column, and backup plan names differ from
	// plan IDs.
	backupNameProdCritical  = "acme-prod-critical"
	backupNameProdDatabase  = "acme-prod-database"
	backupNameStagingHourly = "acme-staging-hourly"
	backupNameComplianceMix = "acme-compliance-mixed"
	backupNameAppData       = "acme-app-data"
	backupNameHealthyDaily  = "acme-daily-backup"
	backupNameNeverRan      = "acme-newly-created"
	backupNameDevSporadic   = "acme-dev-sporadic"
)

func TestScenario_BackupVisual(t *testing.T) {
	scenario := fullIntegrationNewDemoScenario(t)

	// Drive the real demo startup so Wave 2 enrichment runs against the fake.
	runDemoStartup(t, scenario)

	// The menu badge is read while the main menu is still the current view.
	// Only `!` severity bumps the badge, so the `~` finding on
	// plan-warning-partial does not count.
	scenario.ExpectMenuIssueCount("backup", 4)

	scenario.OpenList("backup")

	// Every Wave-2 job-state phrase renders in the Status column.
	for _, jargon := range []string{
		"Last Status", "CIS", " Flags", "Policy ", " Issues ",
		"NOBKP", "UNENC", " PUB ", "NOPROT",
	} {
		scenario.ExpectViewNotContains(jargon)
	}

	// DevSporadicPlanID's only job is 48h old, outside the 24h window.
	scenario.ExpectRowStatusBlank(demofixtures.HealthyDailyPlanID)
	scenario.ExpectRowStatusBlank(demofixtures.NeverRanPlanID)
	scenario.ExpectRowStatusBlank(demofixtures.DevSporadicPlanID)
	scenario.ExpectRowNoGlyphPrefix(backupNameHealthyDaily)
	scenario.ExpectRowNoGlyphPrefix(backupNameNeverRan)
	scenario.ExpectRowNoGlyphPrefix(backupNameDevSporadic)

	scenario.ExpectRowStatusEquals(demofixtures.ProdCriticalPlanID, backupS4Broken1)
	scenario.ExpectRowStatusEquals(demofixtures.ProdDatabasePlanID, backupS4Broken2)
	scenario.ExpectRowStatusEquals(demofixtures.StagingHourlyPlanID, backupS4Broken1)
	scenario.ExpectRowStatusEquals(demofixtures.AppDataPlanID, backupS4Partial)
	// FAILED beats PARTIAL on one row.
	scenario.ExpectRowStatusEquals(demofixtures.ComplianceMixedPlanID, backupS4Broken1)

	// The row color carries the severity signal, so no backup row renders a
	// name glyph.
	for _, name := range []string{
		backupNameProdCritical,
		backupNameProdDatabase,
		backupNameStagingHourly,
		backupNameComplianceMix,
		backupNameAppData,
		backupNameHealthyDaily,
		backupNameNeverRan,
		backupNameDevSporadic,
	} {
		scenario.ExpectRowNoGlyphPrefix(name)
	}

	// ct-events shows no count, so only counted pivots are asserted.
	root := selectBackupByID(t, scenario, demofixtures.ProdDatabasePlanID)
	scenario.OpenDetailResource("backup", root)
	scenario.ExpectNoAPIError()

	for _, displayName := range []string{
		"IAM Roles",
		"KMS Keys",
		"SNS Topics",
	} {
		scenario.ExpectRelatedRowCountAtLeast(displayName, 1)
	}

	view := scenario.currentView()
	t.Log("\n" + view)

	scenario.ExpectViewContains(backupDetailBroken2Capitalize)
	// Per-job State rows humanize the raw AWS enum before it reaches the
	// rendered surface.
	scenario.ExpectViewContains("State: failed")
	scenario.ExpectViewContains("State: expired")
	scenario.ExpectViewNotContains("FAILED")
	scenario.ExpectViewNotContains("EXPIRED")
	// The timestamp is relative, so only the label is asserted.
	scenario.ExpectViewContains("Most recent")

	// The summary phrase carries no concatenated row value such as a trailing
	// "(KMSKeyNotAccessibleException: …)".
	scenario.ExpectViewNotContains(backupS4Broken2 + ":")

	// plan-broken-mixed carries FAILED + PARTIAL; the partial-jobs row keeps
	// the `~` signal visible while Status shows `!`.
	scenario.Back()
	mixed := selectBackupByID(t, scenario, demofixtures.ComplianceMixedPlanID)
	scenario.OpenDetailResource("backup", mixed)
	scenario.ExpectNoAPIError()
	mixedView := scenario.currentView()
	t.Log("\n" + mixedView)

	scenario.ExpectViewContains("1 job failed in last 24h")
	scenario.ExpectViewContains("Partial jobs")

	scenario.Back()
	partial := selectBackupByID(t, scenario, demofixtures.AppDataPlanID)
	scenario.OpenDetailResource("backup", partial)
	scenario.ExpectNoAPIError()
	scenario.ExpectViewContains(backupDetailPartialCapitalize)
	scenario.ExpectViewContains("Partial jobs")
	scenario.ExpectViewContains("Total jobs")
}

// selectBackupByID looks up a concrete backup resource from the demo clients so
// the scenario can call OpenDetailResource with a real resource value.
func selectBackupByID(t *testing.T, s *fullIntegrationScenario, id string) resource.Resource {
	t.Helper()
	return fullIntegrationMustFindResourceByID(t, s.clients, "backup", id)
}
