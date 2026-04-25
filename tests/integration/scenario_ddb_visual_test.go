//go:build integration

package integration

// scenario_ddb_visual_test.go — Phase 8 render-gate for the ddb resource.
//
// Verifies the rendered TUI output (not fetcher return values) matches the
// universal UI rules and the §4 contract in docs/resources/ddb.md.
//
// ddb has:
//   - Zero Wave-1 signals (ListTables returns names only).
//   - Five Wave-2 signals: TableStatus {ACTIVE, CREATING, UPDATING, DELETING,
//     ARCHIVING, INACCESSIBLE_ENCRYPTION_CREDENTIALS, ARCHIVED} + PITR
//     disabled (~ severity).
//   - All Wave-2 `!`-severity count = 0 (PITR is ~ only), so the S1 badge
//     carries no `issues:N` annotation (N=0).
//
// Rule-7 coverage matrix (see docs/resources/ddb-impl-plan.md §4):
//   - U7a (multi Wave-1 suffix):       N/A — no Wave-1 signals.
//   - U7b (Wave-1 + Wave-2 suffix):    N/A — no Wave-1 signals.
//   - U7c (S5 every Wave-2 finding):   covered via `legacy-archived`
//                                      (ARCHIVED + PITR disabled stacks).
//   - U7d (`!` beats `~`):             N/A — spec has zero `!` signals.
//   - U7e (S5 every Wave-1 phrase):    N/A — no Wave-1 phrases to enumerate.
//   - U7f (fetcher populates Issues):  covered at unit-test level only.

import (
	"testing"

	demofixtures "github.com/k2m30/a9s/v3/internal/demo/fixtures"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// §4 phrases from docs/resources/ddb.md §4 — kept as consts so this file
// stays diff-able against the spec.
const (
	ddbPhraseCreating      = "creating"
	ddbPhraseUpdating      = "updating"
	ddbPhraseDeleting      = "deleting"
	ddbPhraseArchiving     = "archiving"
	ddbPhraseKMSLost       = "kms key inaccessible"
	ddbPhraseArchived      = "archived: kms key lost"
	ddbPhrasePITROff       = "PITR off"
	ddbPhraseArchivedPlus1 = "archived: kms key lost (+1)"
)

func TestScenario_DDBVisual(t *testing.T) {
	// Isolate from the developer's stale ~/.a9s/views/ddb.yaml. The in-repo
	// defaults already carry the new Status column (key:status, no PITR column),
	// but a stale user-dir overlay (path:TableStatus, separate PITR column) will
	// win the merge in config.GetViewDef and render the raw AWS enum. Point
	// config.Load at a fresh empty dir so only defaults apply.
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())

	scenario := fullIntegrationNewDemoScenario(t)

	// Drive the real demo startup so Wave-2 enrichment runs against the typed fake.
	runDemoStartup(t, scenario)

	// ---------------------------------------------------------------
	// S1 menu badge — assert BEFORE OpenList while the main menu is
	// still the current view.
	//
	// Expected: 6 = the `!`-severity Wave-2 fixtures only. Per universal
	// rule 4, `~` severity findings never bump the badge, so
	// `audit-pitr-off` (Healthy + `~`) does not contribute.
	// `unifiedIssueCount` filters by `!` severity.
	// ---------------------------------------------------------------
	scenario.ExpectMenuIssueCount("ddb", 6)

	scenario.OpenList("ddb")

	// ---------------------------------------------------------------
	// Universal column rules — no jargon columns. The old `PITR`
	// jargon column was deleted in phase 7; PITR posture now rides in
	// the Status column per §4.
	//
	// Note: `"PITR"` on its own is NOT in this deny-list because the
	// valid §4 phrase `"PITR off"` legitimately renders in the Status
	// column for `audit-pitr-off`. The deleted column was width-6 with
	// title `PITR` and values like `false`/`true`; the absence of the
	// column is asserted by the positive ExpectRowStatusEquals check
	// on `audit-pitr-off` further down — if the jargon column were
	// still in the view, the Status column would miss its §4 phrase.
	// ---------------------------------------------------------------
	for _, jargon := range []string{
		"CIS", " Flags", "Policy ", " Issues ",
		"NOBKP", "UNENC", " PUB ", "NOPROT",
	} {
		scenario.ExpectViewNotContains(jargon)
	}

	// ---------------------------------------------------------------
	// S4 status column — §4 phrases, verbatim.
	// Healthy rows render blank; the graph-root `orders-prod` is the
	// baseline Healthy fixture.
	// ---------------------------------------------------------------
	scenario.ExpectRowStatusBlank(demofixtures.OrdersProdID)

	scenario.ExpectRowStatusEquals(demofixtures.SessionsCreatingID, ddbPhraseCreating)
	scenario.ExpectRowStatusEquals(demofixtures.SessionsUpdatingID, ddbPhraseUpdating)
	scenario.ExpectRowStatusEquals(demofixtures.AnalyticsDeletingID, ddbPhraseDeleting)
	scenario.ExpectRowStatusEquals(demofixtures.LegacyArchivingID, ddbPhraseArchiving)
	scenario.ExpectRowStatusEquals(demofixtures.LegacyKMSLostID, ddbPhraseKMSLost)

	// `audit-pitr-off` — Healthy + Wave-2 `~` finding. Status phrase
	// is "PITR off" (enricher-set on a Healthy row, no suffix).
	scenario.ExpectRowStatusEquals(demofixtures.AuditPITROffID, ddbPhrasePITROff)

	// `legacy-archived` — ARCHIVED + PITR disabled → multi-W2 stacking
	// expressed by the Wave-2 enricher bumping the pre-existing
	// Wave-2-phrase Status via `resource.BumpFindingSuffix`. (ddb's
	// "Wave-1" bucket is effectively the fetcher-driven TableStatus
	// mapping; the enricher treats it as the baseline.)
	scenario.ExpectRowStatusEquals(demofixtures.LegacyArchivedID, ddbPhraseArchivedPlus1)

	// ---------------------------------------------------------------
	// Rule 3 — `~` glyph prefixes a Healthy (green) row with a `~`
	// finding. `audit-pitr-off` is the only such fixture.
	// ---------------------------------------------------------------
	scenario.ExpectRowNamePrefix(demofixtures.AuditPITROffID, "~ ")

	// ---------------------------------------------------------------
	// Rule 3 — every NON-green row renders WITHOUT a glyph regardless
	// of finding presence. Color is the signal. Includes the multi-W2
	// `legacy-archived` row (red) which carries a Wave-2 `~` finding
	// but must not glyph.
	// ---------------------------------------------------------------
	for _, id := range []string{
		demofixtures.SessionsCreatingID,
		demofixtures.SessionsUpdatingID,
		demofixtures.AnalyticsDeletingID,
		demofixtures.LegacyArchivingID,
		demofixtures.LegacyKMSLostID,
		demofixtures.LegacyArchivedID,
	} {
		scenario.ExpectRowNoGlyphPrefix(id)
	}

	// The healthy baseline also must NOT carry a glyph.
	scenario.ExpectRowNoGlyphPrefix(demofixtures.OrdersProdID)

	// ---------------------------------------------------------------
	// Related panel — graph-root fixture shows non-zero counts for
	// EVERY pivot whose §2 contract is `count shown: yes`. Spec:
	// docs/resources/ddb.md §2. User guidance 2026-04-23: "related
	// resources MUST work. if they don't it's a bug. simple".
	// ---------------------------------------------------------------
	root := selectDDBByID(t, scenario, demofixtures.OrdersProdID)
	scenario.OpenDetailResource("ddb", root)
	scenario.ExpectNoAPIError()

	for _, displayName := range []string{
		"CloudWatch Alarms",
		"Backup Plans",
		"Kinesis Streams",
		"KMS Key",
		"Lambda Functions",
		"Log Groups",
		"VPC Endpoints",
	} {
		scenario.ExpectRelatedRowCountAtLeast(displayName, 1)
	}

	// ---------------------------------------------------------------
	// Rule 7 U7c — S5 Attention section on `legacy-archived` shows
	// BOTH the fetcher-side Wave-2 phrase (`archived: kms key lost`)
	// AND the Wave-2 PITR finding (`PITR off`). No finding silently
	// disappears when the Status cell shows the worst-severity phrase.
	// ---------------------------------------------------------------
	scenario.Back()
	stacked := selectDDBByID(t, scenario, demofixtures.LegacyArchivedID)
	scenario.OpenDetailResource("ddb", stacked)
	scenario.ExpectNoAPIError()
	view := scenario.currentView()
	t.Log("\n" + view) // 8.4 user-visible sanity render (mandatory)

	// Attention entry for the Wave-2-from-fetcher phrase, capitalized
	// at render time by injectAttentionSection → capitalizeFirst.
	scenario.ExpectViewContains("Archived: kms key lost")
	// Attention entry for the Wave-2 PITR finding.
	scenario.ExpectViewContains("PITR off")

	// U11 regression guard — the PITR enricher's Summary must be the
	// stable short phrase. If Summary leaked any Row text, we'd see
	// embedded colons or parens after "PITR off"; assert that shape
	// does not appear.
	scenario.ExpectViewNotContains("PITR off:")
	scenario.ExpectViewNotContains("PITR off (")

	// ---------------------------------------------------------------
	// Rule 7 U7c cross-check — Healthy + ~ fixture also surfaces the
	// Wave-2 finding in the detail Attention section.
	// ---------------------------------------------------------------
	scenario.Back()
	pitr := selectDDBByID(t, scenario, demofixtures.AuditPITROffID)
	scenario.OpenDetailResource("ddb", pitr)
	scenario.ExpectNoAPIError()
	scenario.ExpectViewContains("PITR off")
}

// selectDDBByID looks up a concrete ddb resource from the demo clients so the
// scenario can call OpenDetailResource with a real resource value.
func selectDDBByID(t *testing.T, s *fullIntegrationScenario, id string) resource.Resource {
	t.Helper()
	return fullIntegrationMustFindResourceByID(t, s.clients, "ddb", id)
}
