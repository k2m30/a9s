//go:build integration

package integration

// scenario_rds_snap_visual_test.go — Phase 8 render-gate for the rds-snap resource.
//
// Verifies the rendered TUI output matches the universal UI rules and the
// per-resource §4 contract in docs/resources/rds-snap.md. Wave 2 = None for
// this type, so glyph (`!`/`~`) and S5 EnrichmentFinding rules collapse to
// N/A. The two cross-ref Wave-1 signals (orphan, automated past retention)
// are emitted via the IssueEnricher's IssueAppends/FieldUpdates path.
// Authored by the a9s-implement-resource skill runner.

import (
	"strings"
	"testing"

	demofixtures "github.com/k2m30/a9s/v3/internal/demo/fixtures"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func TestScenario_RDSSnapVisual(t *testing.T) {
	scenario := fullIntegrationNewDemoScenario(t)
	runDemoStartup(t, scenario)

	// -----------------------------------------------------------------
	// S1 menu badge — assert BEFORE OpenList (menu is the root view).
	// N = count of distinct rds-snap instances whose final color is yellow/red:
	//
	//   Fetcher-direct (6):
	//     WarnRDSSnapCreatingID                        Warning  (creating: 42%)
	//     BrokenRDSSnapFailedID                        Broken   (failed)
	//     BrokenRDSSnapIncompatibleID                  Broken   (incompatible-restore)
	//     WarnRDSSnapUnencryptedID                     Warning  (unencrypted)
	//     MultiW1RDSSnapID                             Warning  (unencrypted (+1))
	//     SeverityBrokenWarnRDSSnapID                  Broken   (failed; W1 unenc suppressed)
	//
	//   Cross-ref enricher (2):
	//     WarnRDSSnapOrphanID                          Warning  (orphan: source DB deleted)
	//     WarnRDSSnapPastRetentionID                   Warning  (automated, 23d past retention)
	//
	// rds-snap has no Wave-2 `!` signals (Wave 2 = None per spec §3.2), so
	// every contributing instance is Wave-1 colored. Total: 8.
	scenario.ExpectMenuIssueCount("rds-snap", 8)

	scenario.OpenList("rds-snap")

	// -----------------------------------------------------------------
	// Universal column rules — no jargon columns.
	// -----------------------------------------------------------------
	for _, jargon := range []string{"CIS", "NOBKP", "UNENC", "NOPROT", "Flags", "Policy"} {
		scenario.ExpectViewNotContains(jargon)
	}
	// The Encrypted column was deleted per impl-plan §3.5 — guard against re-introduction.
	scenario.ExpectViewNotContains("Encrypted ")

	// -----------------------------------------------------------------
	// Wave 1 §4 phrases per fixture.
	// -----------------------------------------------------------------
	// Healthy: blank Status.
	scenario.ExpectRowStatusBlank(demofixtures.ProdRDSSnapID)
	scenario.ExpectRowStatusBlank(demofixtures.BackupCoveredRDSSnapID)

	// Transitional Warning.
	scenario.ExpectRowStatusEquals(demofixtures.WarnRDSSnapCreatingID, "creating: 42%")

	// Broken (severity wins; failed phrase is bare per spec §4 — no per-row failure-reason field on DBSnapshot).
	scenario.ExpectRowStatusEquals(demofixtures.BrokenRDSSnapFailedID, "failed")
	scenario.ExpectRowStatusEquals(demofixtures.BrokenRDSSnapIncompatibleID, "incompatible-restore")

	// U8 — Broken severity beats Warning: failed + Encrypted=false → "failed" alone (no suffix).
	scenario.ExpectRowStatusEquals(demofixtures.SeverityBrokenWarnRDSSnapID, "failed")

	// Single-W1 Warnings.
	scenario.ExpectRowStatusEquals(demofixtures.WarnRDSSnapUnencryptedID, "unencrypted")

	// Cross-ref Wave-1 (enricher).
	scenario.ExpectRowStatusEquals(demofixtures.WarnRDSSnapOrphanID, "orphan: source DB deleted")
	scenario.ExpectRowStatusEquals(demofixtures.WarnRDSSnapPastRetentionID, "automated, 23d past retention")

	// U7a — multi-W1: unencrypted (fetcher) + orphan (enricher) → top phrase + (+1).
	// Per §0.1 ladder: unencrypted < orphan: source DB deleted, so unencrypted is the top.
	scenario.ExpectRowStatusEquals(demofixtures.MultiW1RDSSnapID, "unencrypted (+1)")

	// -----------------------------------------------------------------
	// Glyph rules.
	// -----------------------------------------------------------------
	// rds-snap has no Wave-2 signals, so no fixture should ever carry a
	// `!` or `~` glyph regardless of color. Spot-check non-green rows AND
	// healthy rows — both should render glyph-free.
	for _, id := range []string{
		demofixtures.ProdRDSSnapID,
		demofixtures.BackupCoveredRDSSnapID,
		demofixtures.WarnRDSSnapCreatingID,
		demofixtures.BrokenRDSSnapFailedID,
		demofixtures.BrokenRDSSnapIncompatibleID,
		demofixtures.SeverityBrokenWarnRDSSnapID,
		demofixtures.WarnRDSSnapUnencryptedID,
		demofixtures.WarnRDSSnapOrphanID,
		demofixtures.WarnRDSSnapPastRetentionID,
		demofixtures.MultiW1RDSSnapID,
	} {
		scenario.ExpectRowNoGlyphPrefix(id)
	}

	// -----------------------------------------------------------------
	// Related panel — graph-root = ProdRDSSnapID. Per impl-plan §9.3
	// rds-snap has a structural exemption: dbi/kms are 1:1 by AWS data
	// model, dbc is always Count=0 (Aurora cluster snapshots live in
	// dbc-snap, not rds-snap — real AWS rejects CreateDBSnapshot on
	// Aurora cluster members). The universal "≥50% Count ≥ 2" rule is
	// unsatisfiable; we assert ≥1 on the pivots that have a non-zero
	// case for rds-snap and accept Count=0 on dbc.
	// -----------------------------------------------------------------
	root := selectRDSSnapByID(t, scenario, demofixtures.ProdRDSSnapID)
	scenario.OpenDetailResource("rds-snap", root)
	scenario.ExpectNoAPIError()
	for _, displayName := range []string{
		"DB Instances", "KMS Keys", "Backup Plans",
	} {
		scenario.ExpectRelatedRowCountAtLeast(displayName, 1)
	}

	scenario.Back()

	// -----------------------------------------------------------------
	// Paste the rendered list view once to the test log — Phase 8.4.
	// -----------------------------------------------------------------
	t.Log("\n" + scenario.currentView())
}

// TestScenario_RDSSnapVisual_DetailSurfacesAllIssues asserts spec rule 7 for
// the detail view. Multi-warning fixtures must enumerate every Resource.Issues
// entry, not just the top phrase shown in the Status column.
func TestScenario_RDSSnapVisual_DetailSurfacesAllIssues(t *testing.T) {
	scenario := fullIntegrationNewDemoScenario(t)
	runDemoStartup(t, scenario)
	scenario.OpenList("rds-snap")

	type issueCase struct {
		id     string
		issues []string // nil = silence; Attention header must be absent
	}
	// Attention section capitalizes the first letter of each entry.
	cases := []issueCase{
		// Healthy baseline — Attention section must be absent.
		{demofixtures.ProdRDSSnapID, nil},
		// Single Wave-1 phrases.
		{demofixtures.WarnRDSSnapCreatingID, []string{"Creating: 42%"}},
		{demofixtures.BrokenRDSSnapFailedID, []string{"Failed"}},
		{demofixtures.BrokenRDSSnapIncompatibleID, []string{"Incompatible-restore"}},
		{demofixtures.WarnRDSSnapUnencryptedID, []string{"Unencrypted"}},
		// U8 — Broken severity beats Warning. Encrypted=false signal is
		// suppressed by the fetcher when Status is a Broken end-state.
		{demofixtures.SeverityBrokenWarnRDSSnapID, []string{"Failed"}},
		// Cross-ref Wave-1 (enricher) — single phrase.
		{demofixtures.WarnRDSSnapOrphanID, []string{"Orphan: source DB deleted"}},
		{demofixtures.WarnRDSSnapPastRetentionID, []string{"Automated, 23d past retention"}},
		// U7e — multi-W1 fixture: every entry must appear in the
		// Attention section. Order is severity-first (`!` tier wins) per
		// injectAttentionSection's stable sort, so the orphan finding (`!`
		// tier from the cross-ref enricher) precedes the fetcher's
		// "unencrypted" Wave-1 phrase (`~` tier from phraseTier).
		{demofixtures.MultiW1RDSSnapID, []string{"Orphan: source DB deleted", "Unencrypted"}},
	}

	for _, tc := range cases {
		t.Run(tc.id, func(t *testing.T) {
			res := selectRDSSnapByID(t, scenario, tc.id)
			scenario.OpenDetailResource("rds-snap", res)
			scenario.ExpectNoAPIError()
			view := scenario.currentView()
			t.Log("\n" + view)

			if len(tc.issues) == 0 {
				expectNoAttentionSection(t, view)
			} else {
				expectAttentionSection(t, view, tc.issues)
			}
			scenario.Back()
		})
	}
}

// TestScenario_RDSSnapVisual_HealthyRowHasNoIssuesPhrases is a regression pin
// for "Healthy silence" in the detail view — no §4 phrase should leak onto
// a Healthy snapshot's detail screen.
func TestScenario_RDSSnapVisual_HealthyRowHasNoIssuesPhrases(t *testing.T) {
	scenario := fullIntegrationNewDemoScenario(t)
	runDemoStartup(t, scenario)
	scenario.OpenList("rds-snap")

	wave1Phrases := []string{
		"creating: ",
		"failed",
		"incompatible-",
		"unencrypted",
		"orphan: source DB deleted",
		"past retention",
	}

	for _, id := range []string{demofixtures.ProdRDSSnapID} {
		t.Run(id, func(t *testing.T) {
			res := selectRDSSnapByID(t, scenario, id)
			scenario.OpenDetailResource("rds-snap", res)
			scenario.ExpectNoAPIError()
			view := scenario.currentView()
			t.Log("\n" + view)

			expectNoAttentionSection(t, view)
			for _, phrase := range wave1Phrases {
				for _, line := range strings.Split(view, "\n") {
					if strings.Contains(line, phrase) {
						t.Errorf("Healthy row %q unexpectedly contains Wave-1 phrase %q in line: %q\nfull view:\n%s", id, phrase, line, view)
					}
				}
			}
			scenario.Back()
		})
	}
}

// selectRDSSnapByID looks up a concrete rds-snap resource from the demo
// clients so the scenario can call OpenDetailResource with a real value.
func selectRDSSnapByID(t *testing.T, s *fullIntegrationScenario, id string) resource.Resource {
	t.Helper()
	return fullIntegrationMustFindResourceByID(t, s.clients, "rds-snap", id)
}
