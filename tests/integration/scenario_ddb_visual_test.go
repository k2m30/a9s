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
// Rule-7 coverage matrix (see docs/historical/resources-impl-plans/ddb-impl-plan.md §4):
//   - U7a (multi Wave-1 suffix):       N/A — no Wave-1 signals.
//   - U7b (Wave-1 + Wave-2 suffix):    N/A — no Wave-1 signals.
//   - U7c (S5 every Wave-2 finding):   covered via `legacy-archived`
//                                      (ARCHIVED + PITR disabled stacks).
//   - U7d (`!` beats `~`):             N/A — spec has zero `!` signals.
//   - U7e (S5 every Wave-1 phrase):    N/A — no Wave-1 phrases to enumerate.
//   - U7f (fetcher populates Issues):  covered at unit-test level only.

import (
	"testing"

	demofixtures "github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/resource"
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
	ddbPhrasePITROff       = "point-in-time recovery disabled"
	ddbPhraseArchivedPlus1 = "archived: kms key lost (+1)"

	// ddbDetailPITROffCapitalize is the Attention-section rendering of
	// ddbPhrasePITROff — injectAttentionSection capitalizes the first
	// letter of every finding phrase (capitalizeFirst), so the Status
	// column's lowercase phrase and the detail view's capitalized phrase
	// are two different literal strings for the same finding.
	ddbDetailPITROffCapitalize = "Point-in-time recovery disabled"
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
	// Expected: 7 = the `!`-severity Wave-2 fixtures plus the
	// listed-but-denied witness (warn-ddb-details-denied), whose degraded
	// name-only row is Warning-colored and therefore issue-counted. Per
	// universal rule 4, `~` severity findings never bump the badge, so
	// `audit-pitr-off` (Healthy + `~`) does not contribute.
	// ---------------------------------------------------------------
	scenario.ExpectMenuIssueCount("ddb", 7)

	scenario.OpenList("ddb")

	// ---------------------------------------------------------------
	// Universal column rules — no jargon columns. The old `PITR`
	// jargon column was deleted in phase 7; PITR posture now rides in
	// the Status column per §4.
	//
	// Note: `"PITR"` on its own is NOT in this deny-list because the
	// operator-phrase Status value for `audit-pitr-off` is
	// "point-in-time recovery disabled". The deleted column was
	// width-6 with title `PITR` and values like `false`/`true`; the
	// absence of the column is asserted by the positive
	// ExpectRowStatusEquals check on `audit-pitr-off` further down —
	// if the jargon column were still in the view, the Status column
	// would miss its §4 phrase.
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
	// is "point-in-time recovery disabled" (enricher-set on a Healthy
	// row, no suffix) — the operator-phrase architecture humanizes the
	// raw PITR flag instead of rendering the old "PITR off" jargon.
	scenario.ExpectRowStatusEquals(demofixtures.AuditPITROffID, ddbPhrasePITROff)

	// `legacy-archived` — ARCHIVED + PITR disabled → multi-W2 stacking
	// expressed by the Wave-2 enricher bumping the pre-existing
	// Wave-2-phrase Status via `resource.BumpFindingSuffix`. (ddb's
	// "Wave-1" bucket is effectively the fetcher-driven TableStatus
	// mapping; the enricher treats it as the baseline.)
	scenario.ExpectRowStatusEquals(demofixtures.LegacyArchivedID, ddbPhraseArchivedPlus1)

	// ---------------------------------------------------------------
	// Rule 3 — every row renders WITHOUT a glyph. colorDDB
	// (catalog_databases.go) resolves color via colorFromAnyFinding first,
	// and ddbCodePITROff is declared Severity: SevWarn, so `audit-pitr-off`
	// now renders Warning row color directly instead of staying
	// Healthy-with-`~`-glyph — the glyph is retired once the row itself
	// carries the finding's color.
	// ---------------------------------------------------------------
	for _, id := range []string{
		demofixtures.SessionsCreatingID,
		demofixtures.SessionsUpdatingID,
		demofixtures.AnalyticsDeletingID,
		demofixtures.LegacyArchivingID,
		demofixtures.LegacyKMSLostID,
		demofixtures.LegacyArchivedID,
		demofixtures.AuditPITROffID,
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
	// AND the Wave-2 PITR finding (`point-in-time recovery disabled`).
	// No finding silently disappears when the Status cell shows the
	// worst-severity phrase.
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
	// Attention entry for the Wave-2 PITR finding — the operator phrase,
	// capitalized at render time, never the retired "PITR off" jargon.
	scenario.ExpectViewContains(ddbDetailPITROffCapitalize)
	scenario.ExpectViewNotContains("PITR off")

	// U11 regression guard — the PITR enricher's Summary must be the
	// stable short phrase. If Summary leaked any Row text, we'd see
	// embedded colons or parens after the phrase; assert that shape
	// does not appear.
	scenario.ExpectViewNotContains(ddbDetailPITROffCapitalize + ":")
	scenario.ExpectViewNotContains(ddbDetailPITROffCapitalize + " (")

	// ---------------------------------------------------------------
	// Rule 7 U7c cross-check — Healthy + ~ fixture also surfaces the
	// Wave-2 finding in the detail Attention section.
	// ---------------------------------------------------------------
	scenario.Back()
	pitr := selectDDBByID(t, scenario, demofixtures.AuditPITROffID)
	scenario.OpenDetailResource("ddb", pitr)
	scenario.ExpectNoAPIError()
	scenario.ExpectViewContains(ddbDetailPITROffCapitalize)
}

// selectDDBByID looks up a concrete ddb resource from the demo clients so the
// scenario can call OpenDetailResource with a real resource value.
func selectDDBByID(t *testing.T, s *fullIntegrationScenario, id string) resource.Resource {
	t.Helper()
	return fullIntegrationMustFindResourceByID(t, s.clients, "ddb", id)
}
