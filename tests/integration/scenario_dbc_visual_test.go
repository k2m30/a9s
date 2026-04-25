//go:build integration

package integration

// scenario_dbc_visual_test.go — Phase 8 render-gate for the dbc resource.
//
// Verifies the rendered TUI output (not fetcher return values) matches the
// universal UI rules and the per-resource §4 contract in docs/resources/dbc.md.
// Authored by the a9s-implement-resource skill runner.

import (
	"strings"
	"testing"

	demofixtures "github.com/k2m30/a9s/v3/internal/demo/fixtures"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func TestScenario_DBCVisual(t *testing.T) {
	scenario := fullIntegrationNewDemoScenario(t)
	runDemoStartup(t, scenario)

	// -----------------------------------------------------------------
	// S1 menu badge — assert BEFORE OpenList (menu is the root view).
	// -----------------------------------------------------------------
	// N = count of DBC instances with at least one attention signal (Wave-1
	// non-green + Wave-2 `!` on Healthy rows). Per the 12 non-aurora dbc
	// fixtures: 10 non-green (4 broken + 1 transitional + 3 single-warn + 1
	// multi-warn + 1 warn-plus-maint) + 1 healthy-overdue = 11.
	scenario.ExpectMenuIssueCount("dbc", 11)

	scenario.OpenList("dbc")

	// -----------------------------------------------------------------
	// Universal column rules — no jargon columns anywhere in the frame.
	// -----------------------------------------------------------------
	for _, jargon := range []string{"CIS", "NOBKP", "UNENC", "NOPROT", "cis_flags"} {
		scenario.ExpectViewNotContains(jargon)
	}
	// The "Writer" column was deleted too — make sure no column header of that
	// literal name sneaks back in via a future defaults_databases.go edit.
	scenario.ExpectViewNotContains("Writer ")

	// -----------------------------------------------------------------
	// Wave 1 §4 phrases per fixture.
	// -----------------------------------------------------------------
	// Healthy row: blank Status.
	scenario.ExpectRowStatusBlank(demofixtures.ProdDbcID)

	// Transitional Warning.
	scenario.ExpectRowStatusEquals("warn-dbc-modifying", "modifying: in progress")

	// Broken.
	scenario.ExpectRowStatusEquals("broken-dbc-failed", "failed: cluster operation")
	scenario.ExpectRowStatusEquals("broken-dbc-enc-unreachable", "encryption key unreachable")
	scenario.ExpectRowStatusEquals("broken-dbc-incompat-params", "parameter group incompatible")
	scenario.ExpectRowStatusEquals("broken-dbc-no-writer", "no writer: reads only")

	// Config Warnings (single-phrase).
	scenario.ExpectRowStatusEquals("warn-dbc-no-prot", "delete-protection off")
	scenario.ExpectRowStatusEquals("warn-dbc-unenc", "not encrypted at rest")
	scenario.ExpectRowStatusEquals("warn-dbc-no-bkp", "no automated backups")

	// Rule 7 U7a — multi-W1: 3 warnings → top + (+2). §4 precedence = delete-protection first.
	scenario.ExpectRowStatusEquals("warn-dbc-multi", "delete-protection off (+2)")

	// Rule 7 U7b — W1 + W2 stack: Warning phrase + (+1) for the hidden Wave-2 finding.
	scenario.ExpectRowStatusEquals(demofixtures.WarnDbcNoBkpMaintID, "no automated backups (+1)")

	// Rule 3 — Wave 2 `!` on Healthy row: S4 = "maintenance overdue", row stays Healthy-green.
	scenario.ExpectRowStatusEquals(demofixtures.MaintDbcOverdueID, "maintenance overdue")

	// -----------------------------------------------------------------
	// Glyph rules.
	// -----------------------------------------------------------------
	// Rule 3 — `!` glyph on Healthy + `!` finding (dbc's only Wave-2 severity is `!`).
	scenario.ExpectRowNamePrefix(demofixtures.MaintDbcOverdueID, "! ")

	// Rule 3 — non-green rows must NOT carry a glyph regardless of finding.
	// `warn-dbc-no-bkp-plus-maint` has a Wave-2 finding but is Warning-colored.
	for _, id := range []string{
		"warn-dbc-modifying",
		"broken-dbc-failed",
		"broken-dbc-enc-unreachable",
		"broken-dbc-incompat-params",
		"broken-dbc-no-writer",
		"warn-dbc-no-prot",
		"warn-dbc-unenc",
		"warn-dbc-no-bkp",
		"warn-dbc-multi",
		demofixtures.WarnDbcNoBkpMaintID,
		// Plain Healthy rows with no finding also have no glyph.
		demofixtures.ProdDbcID,
	} {
		scenario.ExpectRowNoGlyphPrefix(id)
	}

	// -----------------------------------------------------------------
	// Related panel — every §2 `count shown: yes` pivot returns ≥ 1 for the
	// graph-root fixture (`acme-docdb-prod`). The "RDS Instances" pivot is
	// registered universally for dbc but only meaningful for Aurora clusters
	// (DocumentDB clusters never have RDS instance members per AWS), so it's
	// asserted separately below on the Aurora fixture.
	// -----------------------------------------------------------------
	prod := selectDBCByID(t, scenario, demofixtures.ProdDbcID)
	scenario.OpenDetailResource("dbc", prod)
	scenario.ExpectNoAPIError()
	for _, displayName := range []string{
		"Security Groups", "CloudWatch Alarms", "Log Groups", "KMS Key",
		"Secrets Manager", "DB Cluster Snapshots", "Subnets", "VPC",
	} {
		scenario.ExpectRelatedRowCountAtLeast(displayName, 1)
	}

	// -----------------------------------------------------------------
	// Aurora cluster — "all pivots non-zero" graph-root for dbc. Every
	// registered §2 pivot resolves on a single fixture here, including
	// "RDS Instances" (which DocDB graph-roots can't cover because
	// DocumentDB clusters don't have RDS instance members).
	// -----------------------------------------------------------------
	scenario.Back()
	aurora := selectDBCByID(t, scenario, "prod-aurora-cluster")
	scenario.OpenDetailResource("dbc", aurora)
	scenario.ExpectNoAPIError()
	for _, displayName := range []string{
		"Security Groups", "CloudWatch Alarms", "Log Groups", "KMS Key",
		"Secrets Manager", "RDS Instances", "DB Cluster Snapshots",
		"Subnets", "VPC",
	} {
		scenario.ExpectRelatedRowCountAtLeast(displayName, 1)
	}

	// -----------------------------------------------------------------
	// Rule 7 U7c — S5 shows the Wave-2 finding details even on a row whose
	// Status is a Wave-1 phrase.
	// -----------------------------------------------------------------
	scenario.Back()
	noBkpMaint := selectDBCByID(t, scenario, demofixtures.WarnDbcNoBkpMaintID)
	scenario.OpenDetailResource("dbc", noBkpMaint)
	scenario.ExpectNoAPIError()
	// The Wave-2 Action row ("system-update") and Description
	// ("Cluster parameter upgrade") come from buildDBCPendingMaintenance().
	scenario.ExpectViewContains("system-update")
	scenario.ExpectViewContains("Cluster parameter upgrade")

	// Paste the rendered detail once to the test log — Phase 8.4 visual-sanity.
	t.Log("\n" + scenario.currentView())
}

// TestScenario_DBCVisual_DetailSurfacesAllIssues asserts spec rule 7 for the
// detail view. Multi-warning fixtures must enumerate every Resource.Issues
// entry, not just the top phrase shown in the Status column.
func TestScenario_DBCVisual_DetailSurfacesAllIssues(t *testing.T) {
	scenario := fullIntegrationNewDemoScenario(t)
	runDemoStartup(t, scenario)
	scenario.OpenList("dbc")

	type issueCase struct {
		id     string
		issues []string // nil = silence; Attention header must be absent
	}
	// Attention section capitalizes the first letter of each entry.
	cases := []issueCase{
		// Healthy baseline — Attention section must be absent.
		{demofixtures.ProdDbcID, nil},
		// Transitional + single-warning (Wave-1 single phrase).
		{"warn-dbc-modifying", []string{"Modifying: in progress"}},
		// Broken (Wave-1 single phrase).
		{"broken-dbc-failed", []string{"Failed: cluster operation"}},
		{"broken-dbc-enc-unreachable", []string{"Encryption key unreachable"}},
		{"broken-dbc-incompat-params", []string{"Parameter group incompatible"}},
		{"broken-dbc-no-writer", []string{"No writer: reads only"}},
		// Single Config Warnings.
		{"warn-dbc-no-prot", []string{"Delete-protection off"}},
		{"warn-dbc-unenc", []string{"Not encrypted at rest"}},
		{"warn-dbc-no-bkp", []string{"No automated backups"}},
		// U7e — multi Config Warnings: every entry of Resource.Issues must appear
		// in detail (capitalized), in §4 precedence order.
		{"warn-dbc-multi", []string{"Delete-protection off", "Not encrypted at rest", "No automated backups"}},
		// U7c — Wave-1 warning + Wave-2 maintenance — both must appear under Attention.
		// Wave-2 severity "!" sorts BEFORE the Wave-1 "~" entry, so its rows
		// (Action: system-update) precede the Wave-1 phrase "No automated backups".
		{demofixtures.WarnDbcNoBkpMaintID, []string{"system-update", "No automated backups"}},
		// Wave-2 only on Healthy row — Attention section present with Wave-2 Summary.
		{demofixtures.MaintDbcOverdueID, []string{"os-upgrade"}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.id, func(t *testing.T) {
			res := selectDBCByID(t, scenario, tc.id)
			scenario.OpenDetailResource("dbc", res)
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

// TestScenario_DBCVisual_AttentionGlyphSurvivesColorCap pins that the `!`
// glyph is NOT weakened by the color-cap rule introduced alongside this test.
//
// Background: Healthy rows with a Wave-2 `!` finding render the list row
// green and the detail Attention entry with `!` glyph + Warning (yellow)
// color — the glyph keeps its severity signal, but the color matches the
// row's S2 bucket so the detail view doesn't contradict the list. The color
// itself is unit-tested directly (see capTierToRowBucket in
// internal/tui/views/attention_color_cap_test.go); the scenario harness
// strips ANSI so we only assert the glyph-survival half here.
func TestScenario_DBCVisual_AttentionGlyphSurvivesColorCap(t *testing.T) {
	scenario := fullIntegrationNewDemoScenario(t)
	runDemoStartup(t, scenario)
	scenario.OpenList("dbc")

	res := selectDBCByID(t, scenario, demofixtures.MaintDbcOverdueID)
	scenario.OpenDetailResource("dbc", res)
	scenario.ExpectNoAPIError()

	view := scenario.currentView()
	t.Log("\n" + view)

	var entryLine string
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "Maintenance overdue") {
			entryLine = line
			break
		}
	}
	if entryLine == "" {
		t.Fatalf("Attention entry line containing %q not found. view:\n%s", "Maintenance overdue", view)
	}
	if !strings.Contains(entryLine, "! Maintenance overdue") {
		t.Errorf("entry line missing expected glyph prefix `! `: %q\n\nThe color-cap rule must NOT weaken the glyph — only the render color.", entryLine)
	}
}

// TestScenario_DBCVisual_HealthyRowHasNoIssuesPhrases is a dedicated regression
// pin for "Healthy silence" in the detail view.
func TestScenario_DBCVisual_HealthyRowHasNoIssuesPhrases(t *testing.T) {
	scenario := fullIntegrationNewDemoScenario(t)
	runDemoStartup(t, scenario)
	scenario.OpenList("dbc")

	wave1Phrases := []string{
		"no automated backups",
		"not encrypted at rest",
		"delete-protection off",
		"no writer: reads only",
		"encryption key unreachable",
		"parameter group incompatible",
		"failed: cluster operation",
	}

	for _, id := range []string{demofixtures.ProdDbcID} {
		id := id
		t.Run(id, func(t *testing.T) {
			res := selectDBCByID(t, scenario, id)
			scenario.OpenDetailResource("dbc", res)
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

// selectDBCByID looks up a concrete dbc resource from the demo clients so the
// scenario can call OpenDetailResource with a real resource value.
func selectDBCByID(t *testing.T, s *fullIntegrationScenario, id string) resource.Resource {
	t.Helper()
	return fullIntegrationMustFindResourceByID(t, s.clients, "dbc", id)
}
