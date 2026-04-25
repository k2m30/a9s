//go:build integration

package integration

// scenario_backup_visual_test.go — Phase 8 render-gate for the backup resource.
//
// Verifies the rendered TUI output (not fetcher return values) matches the
// universal UI rules and the §4 contract in docs/resources/backup.md.
//
// backup has NO Wave-1 signals (§3.1) and two Wave-2 signals (§3.2):
//   • FAILED/EXPIRED/ABORTED jobs in last 24h → `!` Broken.
//   • PARTIAL jobs in last 24h  → `~` Warning.
// Rule-7 (+N) suffix arithmetic (U7a/U7b/U7e/U7f) is N/A — the suffix only
// activates when Wave-1 warnings coexist. U7d (! beats ~ on one row) is
// covered by the plan-broken-mixed fixture.

import (
	"testing"

	demofixtures "github.com/k2m30/a9s/v3/internal/demo/fixtures"
	"github.com/k2m30/a9s/v3/internal/resource"
)

const (
	// §4 Status column phrases (S4) — these are the canonical strings every
	// row's Status cell must contain verbatim for the given fixture.
	backupS4Broken1 = "1 job failed in last 24h"
	backupS4Broken2 = "2 jobs failed in last 24h"
	backupS4Partial = "partial: 1 of 3 resources skipped"

	// §4 detail text fragments (S5) — substrings that must appear in the
	// rendered detail view for the multi-finding fixtures.
	backupDetailBroken2Capitalize = "2 jobs failed in last 24h"
	backupDetailPartialCapitalize = "Partial: 1 of 3 resources skipped"

	// Plan names — ExpectRowNamePrefix asserts `"<glyph> <NAME>"` because the
	// glyph renders adjacent to the name column, and backup plan names
	// differ from plan IDs (name = "acme-prod-critical", id = UUID).
	backupNameProdCritical   = "acme-prod-critical"
	backupNameProdDatabase   = "acme-prod-database"
	backupNameStagingHourly  = "acme-staging-hourly"
	backupNameComplianceMix  = "acme-compliance-mixed"
	backupNameAppData        = "acme-app-data"
	backupNameHealthyDaily   = "acme-daily-backup"
	backupNameNeverRan       = "acme-newly-created"
	backupNameDevSporadic    = "acme-dev-sporadic"
)

func TestScenario_BackupVisual(t *testing.T) {
	scenario := fullIntegrationNewDemoScenario(t)

	// Drive the real demo startup so Wave 2 enrichment runs against the fake.
	runDemoStartup(t, scenario)

	// ---------------------------------------------------------------
	// S1 menu badge — assert BEFORE OpenList while the main menu is
	// still the current view.
	//
	// 4 plans carry `!` severity findings (plan-broken-1failed,
	// plan-broken-2failed, plan-broken-aborted, plan-broken-mixed) and
	// 1 plan carries a `~` severity finding (plan-warning-partial).
	// Per universal rule 4, only `!` severity bumps the badge, so the
	// expected count is 4. `unifiedIssueCount` filters by `!` severity.
	scenario.ExpectMenuIssueCount("backup", 4)

	scenario.OpenList("backup")

	// ---------------------------------------------------------------
	// Universal column rules — no jargon columns, in particular no
	// revival of the retired "Last Status" column (spec §4 collapses
	// every Wave-2 job-state phrase into the Status column).
	// ---------------------------------------------------------------
	for _, jargon := range []string{
		"Last Status", "CIS", " Flags", "Policy ", " Issues ",
		"NOBKP", "UNENC", " PUB ", "NOPROT",
	} {
		scenario.ExpectViewNotContains(jargon)
	}

	// ---------------------------------------------------------------
	// Healthy rows (S2 green, S4 blank, no glyph) — three fixtures:
	// plan-healthy-daily, plan-never-ran, plan-old-failure (its
	// only job is 48h out of window).
	// ---------------------------------------------------------------
	scenario.ExpectRowStatusBlank(demofixtures.HealthyDailyPlanID)
	scenario.ExpectRowStatusBlank(demofixtures.NeverRanPlanID)
	scenario.ExpectRowStatusBlank(demofixtures.DevSporadicPlanID)
	// Pass NAMEs for glyph-prefix checks — the harness asserts on the
	// literal substring `"<prefix><id>"`, and our glyph is rendered
	// next to the name column (plan name ≠ plan ID).
	scenario.ExpectRowNoGlyphPrefix(backupNameHealthyDaily)
	scenario.ExpectRowNoGlyphPrefix(backupNameNeverRan)
	scenario.ExpectRowNoGlyphPrefix(backupNameDevSporadic)

	// ---------------------------------------------------------------
	// §4 Status phrases — exact match.
	// ---------------------------------------------------------------
	scenario.ExpectRowStatusEquals(demofixtures.ProdCriticalPlanID, backupS4Broken1)
	scenario.ExpectRowStatusEquals(demofixtures.ProdDatabasePlanID, backupS4Broken2)
	scenario.ExpectRowStatusEquals(demofixtures.StagingHourlyPlanID, backupS4Broken1)
	scenario.ExpectRowStatusEquals(demofixtures.AppDataPlanID, backupS4Partial)
	// plan-broken-mixed: FAILED beats PARTIAL (U7d severity precedence).
	scenario.ExpectRowStatusEquals(demofixtures.ComplianceMixedPlanID, backupS4Broken1)

	// ---------------------------------------------------------------
	// Rule 3 — `!` / `~` glyphs. All backup findings land on Healthy
	// (green) rows because there are no Wave-1 signals; therefore
	// every Wave-2 finding gets a glyph.
	// ---------------------------------------------------------------
	for _, name := range []string{
		backupNameProdCritical,
		backupNameProdDatabase,
		backupNameStagingHourly,
		backupNameComplianceMix, // U7d: ! beats ~
	} {
		scenario.ExpectRowNamePrefix(name, "! ")
	}
	scenario.ExpectRowNamePrefix(backupNameAppData, "~ ")

	// ---------------------------------------------------------------
	// Related panel — graph-root is plan-broken-2failed (prod vault
	// with full kms + role + sns wiring). Every §2 pivot with
	// `count shown: yes` MUST render ≥1. ct-events is `count shown:
	// unknown` and exempt.
	// ---------------------------------------------------------------
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

	// ---------------------------------------------------------------
	// Rule 7 U7c — S5 Attention section for plan-broken-2failed must
	// render the Summary phrase and the per-job State rows so no
	// Wave-2 fact silently disappears.
	// ---------------------------------------------------------------
	view := scenario.currentView()
	t.Log("\n" + view) // 8.4 user-visible sanity render (mandatory)

	// Attention primary entry: glyph + capitalized phrase.
	scenario.ExpectViewContains(backupDetailBroken2Capitalize)
	// Per-job State rows — FAILED and EXPIRED both surface from Rows.
	scenario.ExpectViewContains("FAILED")
	scenario.ExpectViewContains("EXPIRED")
	// Most-recent timestamp row should also appear — asserted loosely as
	// a presence of the "Most recent" label (the concrete timestamp is
	// relative and not pinned here).
	scenario.ExpectViewContains("Most recent")

	// U11 regression guard — Summary (`backupS4Broken2`) must not contain
	// any concatenated Row value form. Asserted at unit-test level;
	// here we confirm the stable phrase renders without a trailing
	// "(KMSKeyNotAccessibleException: …)" leak or similar.
	scenario.ExpectViewNotContains(backupS4Broken2 + ":")

	// ---------------------------------------------------------------
	// U7d — plan-broken-mixed carries FAILED + PARTIAL. Detail view
	// must show the partial-jobs count in Rows so the `~` signal
	// does NOT silently disappear even though Status shows `!`.
	// ---------------------------------------------------------------
	scenario.Back()
	mixed := selectBackupByID(t, scenario, demofixtures.ComplianceMixedPlanID)
	scenario.OpenDetailResource("backup", mixed)
	scenario.ExpectNoAPIError()
	mixedView := scenario.currentView()
	t.Log("\n" + mixedView)

	scenario.ExpectViewContains("1 job failed in last 24h") // capitalized in Attention
	scenario.ExpectViewContains("Partial jobs")             // Row label preserves ~ info

	// ---------------------------------------------------------------
	// Warning fixture — plan-warning-partial. Detail view shows the
	// partial phrase and the supporting count rows.
	// ---------------------------------------------------------------
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
