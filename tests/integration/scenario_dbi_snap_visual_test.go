//go:build integration

package integration

// scenario_rds_snap_visual_test.go — Phase 8 render-gate for the dbi-snap resource.
//
// Verifies the rendered TUI output matches the universal UI rules and the
// per-resource §4 contract in docs/resources/dbi-snap.md. Wave 2 = None for
// this type, so glyph (`!`/`~`) and S5 EnrichmentFinding rules collapse to
// N/A. The two cross-ref Wave-1 signals (orphan, automated past retention)
// are emitted via the IssueEnricher's IssueAppends/FieldUpdates path.
// Authored by the a9s-implement-resource skill runner.

import (
	"strings"
	"testing"

	demofixtures "github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/resource"
)

func TestScenario_DBISnapVisual(t *testing.T) {
	scenario := fullIntegrationNewDemoScenario(t)
	runDemoStartup(t, scenario)

	// -----------------------------------------------------------------
	// S1 menu badge — assert BEFORE OpenList (the menu is the root view).
	// The count is the rows whose colour is an issue, and colour is the worst
	// severity among a row's findings across both waves.
	// Over the 12 dbi-snap fixtures:
	//   Wave-1 Broken (3):  prod-dbi-1-failed-snap, failed-with-unenc-snap,
	//                       legacy-mysql-snap-incompatible
	//   Wave-1 Warning (4): dev-feature-branch-snap (creating: 42%),
	//                       cross-region-copy-snap (copying),
	//                       unenc-pre-migration-snap,
	//                       multi-orphan-unenc-snap
	//   Wave-2 only (3):    orphan-deleted-db-snap (orphan),
	//                       rds:retention-test-2026-03-25 (past retention),
	//                       shared-with-all-dbi-snap (shared with all accounts)
	//   Not counted (2):    rds:prod-dbi-1-2026-04-15 and
	//                       awsbackup:job-deadbeef-snap are clean.
	// 3 + 4 + 3 = 10. The copying row is the one this batch added, as the
	// witness for a snapshot state a9s does not enumerate by name.
	scenario.ExpectMenuIssueCount("dbi-snap", 10)

	scenario.OpenList("dbi-snap")

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
	scenario.ExpectRowStatusBlank(demofixtures.ProdDBISnapID)
	scenario.ExpectRowStatusBlank(demofixtures.BackupCoveredDBISnapID)

	// Transitional Warning.
	scenario.ExpectRowStatusEquals(demofixtures.WarnDBISnapCreatingID, "creating: 42%")

	// Broken (severity wins; failed phrase is bare per spec §4 — no per-row failure-reason field on DBSnapshot).
	scenario.ExpectRowStatusEquals(demofixtures.BrokenDBISnapFailedID, "failed")
	scenario.ExpectRowStatusEquals(demofixtures.BrokenDBISnapIncompatibleID, "incompatible-restore")

	// U8 — Broken severity beats Warning: failed + Encrypted=false → "failed" alone (no suffix).
	scenario.ExpectRowStatusEquals(demofixtures.SeverityBrokenWarnDBISnapID, "failed")

	// Single-W1 Warnings.
	scenario.ExpectRowStatusEquals(demofixtures.WarnDBISnapUnencryptedID, "unencrypted")

	// Cross-ref Wave-1 (enricher).
	scenario.ExpectRowStatusEquals(demofixtures.WarnDBISnapOrphanID, "orphan: source DB deleted")
	scenario.ExpectRowStatusEquals(demofixtures.WarnDBISnapPastRetentionID, "automated, 23d past retention")

	// U7a — W1 unencrypted (fetcher, `~`) + W2 orphan (enricher, `!`) → top
	// phrase + (+1). The top is decided by severity, not by wave or by a
	// phrase ladder: domain.TopFinding takes the worst entry, so the `!`
	// orphan leads and the `~` unencrypted becomes the (+1).
	scenario.ExpectRowStatusEquals(demofixtures.MultiW1DBISnapID, "orphan: source DB deleted (+1)")

	// -----------------------------------------------------------------
	// Glyph rules.
	// -----------------------------------------------------------------
	// colorDBISnap (catalog_databases.go) resolves color via
	// colorFromAnyFinding first. dbiSnapOrphanCode and
	// dbiSnapPastRetentionCode are both declared Severity: SevBroken, so the
	// cross-ref enricher's findings now promote those rows straight to
	// Broken row color — they no longer stay Healthy-with-glyph the way they
	// did when the enricher emitted Findings without a severity color read.
	// Every row here resolves a non-Healthy color via a finding (or a plain
	// structural fallback for the truly healthy rows), so none carry a
	// glyph.
	for _, id := range []string{
		demofixtures.ProdDBISnapID,
		demofixtures.BackupCoveredDBISnapID,
		demofixtures.WarnDBISnapCreatingID,
		demofixtures.BrokenDBISnapFailedID,
		demofixtures.BrokenDBISnapIncompatibleID,
		demofixtures.SeverityBrokenWarnDBISnapID,
		demofixtures.WarnDBISnapUnencryptedID,
		demofixtures.MultiW1DBISnapID,
		demofixtures.WarnDBISnapOrphanID,
		demofixtures.WarnDBISnapPastRetentionID,
	} {
		scenario.ExpectRowNoGlyphPrefix(id)
	}

	// -----------------------------------------------------------------
	// Related panel — graph-root = ProdDBISnapID. Per impl-plan §9.3
	// dbi-snap has a structural exemption: dbi/kms are 1:1 by AWS data
	// model, dbc is always Count=0 (Aurora cluster snapshots live in
	// dbc-snap, not dbi-snap — real AWS rejects CreateDBSnapshot on
	// Aurora cluster members). The universal "≥50% Count ≥ 2" rule is
	// unsatisfiable; we assert ≥1 on the pivots that have a non-zero
	// case for dbi-snap and accept Count=0 on dbc.
	// -----------------------------------------------------------------
	root := selectDBISnapByID(t, scenario, demofixtures.ProdDBISnapID)
	scenario.OpenDetailResource("dbi-snap", root)
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

// TestScenario_DBISnapVisual_DetailSurfacesAllIssues asserts spec rule 7 for
// the detail view. Multi-warning fixtures must enumerate every Resource.Issues
// entry, not just the top phrase shown in the Status column.
func TestScenario_DBISnapVisual_DetailSurfacesAllIssues(t *testing.T) {
	scenario := fullIntegrationNewDemoScenario(t)
	runDemoStartup(t, scenario)
	scenario.OpenList("dbi-snap")

	type issueCase struct {
		id     string
		issues []string // nil = silence; Attention header must be absent
	}
	// Attention section capitalizes the first letter of each entry.
	cases := []issueCase{
		// Healthy baseline — Attention section must be absent.
		{demofixtures.ProdDBISnapID, nil},
		// Single Wave-1 phrases.
		{demofixtures.WarnDBISnapCreatingID, []string{"Creating: 42%"}},
		{demofixtures.BrokenDBISnapFailedID, []string{"Failed"}},
		{demofixtures.BrokenDBISnapIncompatibleID, []string{"Incompatible-restore"}},
		{demofixtures.WarnDBISnapUnencryptedID, []string{"Unencrypted"}},
		// U8 — Broken severity beats Warning. Encrypted=false signal is
		// suppressed by the fetcher when Status is a Broken end-state.
		{demofixtures.SeverityBrokenWarnDBISnapID, []string{"Failed"}},
		// Cross-ref Wave-1 (enricher) — single phrase.
		{demofixtures.WarnDBISnapOrphanID, []string{"Orphan: source DB deleted"}},
		{demofixtures.WarnDBISnapPastRetentionID, []string{"Automated, 23d past retention"}},
		// U7e — multi-W1 fixture: every entry must appear in the
		// Attention section. Order is severity-first (`!` tier wins) per
		// injectAttentionSection's stable sort, so the orphan finding (`!`
		// tier from the cross-ref enricher) precedes the fetcher's
		// "unencrypted" Wave-1 phrase (`~` tier from phraseTier).
		{demofixtures.MultiW1DBISnapID, []string{"Orphan: source DB deleted", "Unencrypted"}},
	}

	for _, tc := range cases {
		t.Run(tc.id, func(t *testing.T) {
			res := selectDBISnapByID(t, scenario, tc.id)
			scenario.OpenDetailResource("dbi-snap", res)
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

// TestScenario_DBISnapVisual_HealthyRowHasNoIssuesPhrases is a regression pin
// for "Healthy silence" in the detail view — no §4 phrase should leak onto
// a Healthy snapshot's detail screen.
func TestScenario_DBISnapVisual_HealthyRowHasNoIssuesPhrases(t *testing.T) {
	scenario := fullIntegrationNewDemoScenario(t)
	runDemoStartup(t, scenario)
	scenario.OpenList("dbi-snap")

	wave1Phrases := []string{
		"creating: ",
		"failed",
		"incompatible-",
		"unencrypted",
		"orphan: source DB deleted",
		"past retention",
	}

	for _, id := range []string{demofixtures.ProdDBISnapID} {
		t.Run(id, func(t *testing.T) {
			res := selectDBISnapByID(t, scenario, id)
			scenario.OpenDetailResource("dbi-snap", res)
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

// selectDBISnapByID looks up a concrete dbi-snap resource from the demo
// clients so the scenario can call OpenDetailResource with a real value.
func selectDBISnapByID(t *testing.T, s *fullIntegrationScenario, id string) resource.Resource {
	t.Helper()
	return fullIntegrationMustFindResourceByID(t, s.clients, "dbi-snap", id)
}
