//go:build integration

package integration

// scenario_dbc_visual_test.go checks the rendered TUI output for dbc against
// the universal UI rules and docs/resources/dbc.md.

import (
	"strings"
	"testing"

	demofixtures "github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/resource"
)

func TestScenario_DBCVisual(t *testing.T) {
	scenario := fullIntegrationNewDemoScenario(t)
	runDemoStartup(t, scenario)

	// The menu badge is read while the menu is the root view.
	// N = rows whose Wave-1-only colour IsIssue, plus Healthy rows carrying a
	// Wave-2 `!`. Over the 23 dbc fixtures:
	//   Wave-1 Broken (9):   broken-dbc-{failed,no-writer,incompat-params,
	//                        enc-unreachable,encryption-recoverable,stopped,
	//                        cloning-failed,migration-failed,upgrade-failed}
	//   Wave-1 Warning (11): warn-dbc-{modifying,no-bkp,no-prot,unenc,multi,
	//                        no-bkp-plus-maint,single-az,minor-upgrade-off,
	//                        iam-auth-off,default-master-user,
	//                        unrecognised-status}
	//   Healthy + Wave-2 `!` (1): healthy-dbc-maint-overdue
	//   Not counted (2): acme-docdb-prod, prod-aurora-cluster — Healthy, no finding
	// 9 + 11 + 1 = 21.
	scenario.ExpectMenuIssueCount("dbc", 21)

	scenario.OpenList("dbc")

	for _, jargon := range []string{"CIS", "NOBKP", "UNENC", "NOPROT", "cis_flags"} {
		scenario.ExpectViewNotContains(jargon)
	}
	scenario.ExpectViewNotContains("Writer ")

	scenario.ExpectRowStatusBlank(demofixtures.ProdDbcID)

	scenario.ExpectRowStatusEquals("warn-dbc-modifying", "modifying: in progress")

	scenario.ExpectRowStatusEquals("broken-dbc-failed", "failed: cluster operation")
	scenario.ExpectRowStatusEquals("broken-dbc-enc-unreachable", "encryption key unreachable")
	scenario.ExpectRowStatusEquals("broken-dbc-incompat-params", "parameter group incompatible")
	scenario.ExpectRowStatusEquals("broken-dbc-no-writer", "no writer: reads only")

	scenario.ExpectRowStatusEquals("warn-dbc-no-prot", "delete-protection off")
	scenario.ExpectRowStatusEquals("warn-dbc-unenc", "not encrypted at rest")
	scenario.ExpectRowStatusEquals("warn-dbc-no-bkp", "no automated backups")

	// Among Wave-1 warnings, delete-protection takes precedence.
	scenario.ExpectRowStatusEquals("warn-dbc-multi", "delete-protection off (+2)")

	// The cell leads with the WORST finding, not the Wave-1 one:
	// domain.TopFinding is the single selection the Status
	// phrase and the row colour share, "so a red row can never read as a
	// warning". Here Wave-2 dbc.maintenance-overdue is `!` and Wave-1
	// no_automated_backups is `~`, so maintenance overdue tops and the
	// backups finding becomes the (+1).
	scenario.ExpectRowStatusEquals(demofixtures.WarnDbcNoBkpMaintID, "maintenance overdue (+1)")

	// dbcCodeMaintenanceOverdue is Severity: SevBroken (catalog_databases.go),
	// so colorFromAnyFinding resolves the row color to Broken directly.
	scenario.ExpectRowStatusEquals(demofixtures.MaintDbcOverdueID, "maintenance overdue")

	// Non-green rows carry no glyph regardless of finding.
	// Every dbc finding here resolves a non-Healthy row colour via
	// colorFromAnyFinding, so none of these rows carry a glyph.
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
		demofixtures.MaintDbcOverdueID,
		demofixtures.ProdDbcID,
	} {
		scenario.ExpectRowNoGlyphPrefix(id)
	}

	// The "RDS Instances" pivot is registered for every dbc but only meaningful
	// for Aurora clusters (DocumentDB clusters never have RDS instance members),
	// so it is asserted below on the Aurora fixture.
	prod := selectDBCByID(t, scenario, demofixtures.ProdDbcID)
	scenario.OpenDetailResource("dbc", prod)
	scenario.ExpectNoAPIError()
	for _, displayName := range []string{
		"Security Groups", "CloudWatch Alarms", "Log Groups", "KMS Key",
		"Secrets Manager", "DB Cluster Snapshots", "Subnets", "VPC",
	} {
		scenario.ExpectRelatedRowCountAtLeast(displayName, 1)
	}

	// The Aurora cluster resolves every registered pivot on a single fixture.
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

	// The detail view shows the Wave-2 finding details even on a row whose
	// Status is a Wave-1 phrase.
	scenario.Back()
	noBkpMaint := selectDBCByID(t, scenario, demofixtures.WarnDbcNoBkpMaintID)
	scenario.OpenDetailResource("dbc", noBkpMaint)
	scenario.ExpectNoAPIError()
	// The Wave-2 Action row ("system-update") and Description
	// ("Cluster parameter upgrade") come from buildDBCPendingMaintenance().
	scenario.ExpectViewContains("system-update")
	scenario.ExpectViewContains("Cluster parameter upgrade")

	t.Log("\n" + scenario.currentView())
}

// TestScenario_DBCVisual_DetailSurfacesAllIssues asserts that multi-warning
// fixtures enumerate every Resource.Issues
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
		{demofixtures.ProdDbcID, nil},
		{"warn-dbc-modifying", []string{"Modifying: in progress"}},
		{"broken-dbc-failed", []string{"Failed: cluster operation"}},
		{"broken-dbc-enc-unreachable", []string{"Encryption key unreachable"}},
		{"broken-dbc-incompat-params", []string{"Parameter group incompatible"}},
		{"broken-dbc-no-writer", []string{"No writer: reads only"}},
		{"warn-dbc-no-prot", []string{"Delete-protection off"}},
		{"warn-dbc-unenc", []string{"Not encrypted at rest"}},
		{"warn-dbc-no-bkp", []string{"No automated backups"}},
		// Multi Config Warnings appear in precedence order.
		{"warn-dbc-multi", []string{"Delete-protection off", "Not encrypted at rest", "No automated backups"}},
		// Wave-2 severity "!" sorts BEFORE the Wave-1 "~" entry, so its rows
		// (Action: system-update) precede the Wave-1 phrase "No automated backups".
		{demofixtures.WarnDbcNoBkpMaintID, []string{"system-update", "No automated backups"}},
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
// glyph survives the color-cap rule.
//
// Healthy rows with a Wave-2 `!` finding render the list row
// green and the detail Attention entry with `!` glyph + Warning (yellow)
// color — the glyph keeps its severity signal, but the color matches the
// row's colour bucket so the detail view doesn't contradict the list. The color
// itself is unit-tested directly (see the Attention colour-cap tests in
// tests/unit/detail_ports_test.go); the scenario harness
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

// TestScenario_DBCVisual_HealthyRowHasNoIssuesPhrases pins "Healthy silence"
// in the detail view.
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
