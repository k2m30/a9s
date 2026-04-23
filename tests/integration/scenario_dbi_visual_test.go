//go:build integration

package integration

// scenario_dbi_visual_test.go — Phase 8 render-gate for the dbi resource.
//
// Verifies the rendered TUI output (not fetcher return values) matches the
// universal UI rules and the per-resource §4 contract in docs/resources/dbi.md.
// Authored by the a9s-implement-resource skill runner (not QA), because these
// assertions guard rendering pipeline drift independent of unit-test coverage.
//
// Demo mode runs Wave 2 enrichment against fixture data (the !m.isDemo guard
// was removed 2026-04-22 — the skip was wrong; typed fakes implement the
// enricher APIs). Every assertion below therefore exercises the real Update
// loop end-to-end, no injection required.

import (
	"strings"
	"testing"

	demofixtures "github.com/k2m30/a9s/v3/internal/demo/fixtures"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
)

func TestScenario_DBIVisual(t *testing.T) {
	scenario := fullIntegrationNewDemoScenario(t)

	// Drive the real demo startup: Init → ClientsReadyMsg → demoPrefetchCounts
	// → AvailabilityPrefetchedMsg → startEnrichment → EnrichmentCheckedMsg.
	// The scripted-scenario constructor skips Init, so enrichment never fires
	// without this step. Running the full chain here means every assertion
	// below exercises the SAME code path an `./a9s --demo` user sees.
	runDemoStartup(t, scenario)

	scenario.OpenList("dbi")

	// -----------------------------------------------------------------
	// Universal column rules — no jargon columns anywhere in the frame.
	// -----------------------------------------------------------------
	for _, jargon := range []string{"CIS", " Flags", "NOBKP", "UNENC", "NOPROT", "cis_flags"} {
		scenario.ExpectViewNotContains(jargon)
	}

	// -----------------------------------------------------------------
	// Wave 1 §4 phrases per fixture.
	// -----------------------------------------------------------------
	// Healthy rows: blank Status.
	scenario.ExpectRowStatusBlank(demofixtures.ProdDbiID)
	scenario.ExpectRowStatusBlank(demofixtures.ProdDbiAuroraID)

	// Transitional Warnings.
	scenario.ExpectRowStatusEquals(demofixtures.StagingDbiModifyingID, "modifying: DBInstanceClass")
	scenario.ExpectRowStatusEquals(demofixtures.StagingDbiRebootingID, "rebooting")

	// Broken.
	scenario.ExpectRowStatusEquals(demofixtures.BrokenDbiStorageFullID, "storage-full")
	scenario.ExpectRowStatusEquals(demofixtures.BrokenDbiEncryptionLockedID, "encryption key unavailable")

	// Config Warnings (single-phrase).
	scenario.ExpectRowStatusEquals(demofixtures.WarnDbiNoBackupsID, "no automated backups")
	scenario.ExpectRowStatusEquals(demofixtures.WarnDbiPublicID, "publicly accessible")
	scenario.ExpectRowStatusEquals(demofixtures.WarnDbiUnencryptedID, "unencrypted storage")
	scenario.ExpectRowStatusEquals(demofixtures.WarnDbiUnprotectedID, "deletion protection off")

	// Rule 7 — multi-W1: 3 warnings → top + (+2).
	scenario.ExpectRowStatusEquals(demofixtures.WarnDbiMultiID, "no automated backups (+2)")

	// Rule 7 — W1 + W2 stack: Warning phrase + (+1) for the hidden Wave-2 finding.
	scenario.ExpectRowStatusEquals(demofixtures.WarnDbiPublicMaintID, "publicly accessible (+1)")

	// Rule 3 — Wave 2 on Healthy row: S4 = "maintenance scheduled".
	scenario.ExpectRowStatusEquals(demofixtures.MaintDbiScheduledID, "maintenance scheduled")

	// -----------------------------------------------------------------
	// Glyph rules.
	// -----------------------------------------------------------------
	// Rule 3 — `~` glyph on Healthy + ~ finding.
	scenario.ExpectRowNamePrefix(demofixtures.MaintDbiScheduledID, "~ ")

	// Rule 3 — non-green rows must NOT carry a glyph regardless of finding.
	// `warn-dbi-public-maint` has a Wave-2 finding but is Warning-colored.
	for _, id := range []string{
		demofixtures.StagingDbiModifyingID,
		demofixtures.StagingDbiRebootingID,
		demofixtures.BrokenDbiStorageFullID,
		demofixtures.BrokenDbiEncryptionLockedID,
		demofixtures.WarnDbiNoBackupsID,
		demofixtures.WarnDbiPublicID,
		demofixtures.WarnDbiUnencryptedID,
		demofixtures.WarnDbiUnprotectedID,
		demofixtures.WarnDbiMultiID,
		demofixtures.WarnDbiPublicMaintID,
		// Plain Healthy rows with no finding also have no glyph.
		demofixtures.ProdDbiID,
		demofixtures.ProdDbiAuroraID,
	} {
		scenario.ExpectRowNoGlyphPrefix(id)
	}

	// -----------------------------------------------------------------
	// Related panel — every §2 pivot with `count shown: yes` ≥ 1 for the
	// designated graph-root fixture (`prod-dbi-1`).
	// -----------------------------------------------------------------
	prod := selectDBIByID(t, scenario, demofixtures.ProdDbiID)
	scenario.OpenDetailResource("dbi", prod)
	scenario.ExpectNoAPIError()
	for _, displayName := range []string{
		"Security Groups", "KMS Key", "Subnets", "CloudWatch Alarms",
		"RDS Snapshots", "Log Groups", "VPC", "Secrets Manager",
		"IAM Roles", "Network Interfaces",
	} {
		scenario.ExpectRelatedRowCountAtLeast(displayName, 1)
	}

	// Aurora member → every §2 `count shown: yes` pivot ≥ 1.
	// prod-dbi-aurora-1 is the "all pivots non-zero" graph-root — it covers
	// every registered dbi pivot on a single fixture. ct-events is exempt
	// per §5 (count shown: unknown for windowed LookupEvents).
	scenario.Back()
	aurora := selectDBIByID(t, scenario, demofixtures.ProdDbiAuroraID)
	scenario.OpenDetailResource("dbi", aurora)
	scenario.ExpectNoAPIError()
	for _, displayName := range []string{
		"Security Groups", "KMS Key", "Subnets", "CloudWatch Alarms",
		"RDS Snapshots", "Log Groups", "VPC", "Secrets Manager",
		"IAM Roles", "Network Interfaces", "RDS Clusters",
	} {
		scenario.ExpectRelatedRowCountAtLeast(displayName, 1)
	}

	// -----------------------------------------------------------------
	// Rule 7 U7c — S5 shows every finding even on a row whose S4 is Wave 1.
	// -----------------------------------------------------------------
	scenario.Back()
	publicMaint := selectDBIByID(t, scenario, demofixtures.WarnDbiPublicMaintID)
	scenario.OpenDetailResource("dbi", publicMaint)
	scenario.ExpectNoAPIError()
	scenario.ExpectViewContains("os-upgrade")            // Wave 2 Action row
	scenario.ExpectViewContains("Kernel security patch") // Wave 2 Description row
}

// attentionSectionHeaderLine returns the index of the "Attention (N)"
// section header line within the ANSI-stripped rendered frame, or -1 if
// absent.
//
// The detail view renders the header inside a box: "│ Attention (3)  │ RELATED │".
// We split on "│" and check that the first content cell, trimmed, begins
// with "Attention " (the header includes a count — "Attention (3)" etc).
func attentionSectionHeaderLine(lines []string) int {
	for i, l := range lines {
		parts := strings.Split(l, "│")
		if len(parts) < 2 {
			continue
		}
		cell := strings.TrimSpace(parts[1])
		if cell == "Attention" || strings.HasPrefix(cell, "Attention (") {
			return i
		}
	}
	return -1
}

// expectAttentionSection asserts that the rendered view contains an
// "Attention (N)" section header and that every expected phrase appears
// after that header, in the given order (§4 precedence). Fails with the
// full rendered frame on any violation so that regressions in the renderer
// are immediately visible in test output.
func expectAttentionSection(t *testing.T, view string, phrases []string) {
	t.Helper()
	lines := strings.Split(view, "\n")
	hdr := attentionSectionHeaderLine(lines)
	if hdr < 0 {
		t.Fatalf("Attention section header not found. view:\n%s", view)
	}
	prev := hdr
	for _, phrase := range phrases {
		found := -1
		for i := hdr + 1; i < len(lines); i++ {
			if strings.Contains(lines[i], phrase) {
				found = i
				break
			}
		}
		if found < 0 {
			t.Fatalf("phrase %q not found after Attention header (header at line %d). view:\n%s", phrase, hdr, view)
		}
		if found <= prev {
			t.Fatalf("phrase %q at line %d appears before previous phrase at line %d (§4 precedence violated). view:\n%s", phrase, found, prev, view)
		}
		prev = found
	}
}

// expectNoAttentionSection asserts that the rendered view does NOT contain
// an "Attention" section header — i.e. the row has no active signals at all
// (spec §4 "Healthy silence" AND no Wave 2 finding).
func expectNoAttentionSection(t *testing.T, view string) {
	t.Helper()
	lines := strings.Split(view, "\n")
	if hdr := attentionSectionHeaderLine(lines); hdr >= 0 {
		t.Fatalf("Attention section header unexpectedly present (line %d). view:\n%s", hdr, view)
	}
}

// TestScenario_DBIVisual_DetailSurfacesAllIssues asserts spec rule 7 ("every finding
// individually visible across S2–S5") for the detail view (S5). The list view
// already renders warning phrases correctly; this test guards against S5 regressions.
//
// Assertions are tighter than plain ExpectViewContains: phrases must appear AFTER
// the "Attention (N)" section header and in §4 precedence order.
func TestScenario_DBIVisual_DetailSurfacesAllIssues(t *testing.T) {
	scenario := fullIntegrationNewDemoScenario(t)
	runDemoStartup(t, scenario)
	scenario.OpenList("dbi")

	type issueCase struct {
		id     string
		issues []string // nil or empty = truly silent; Attention header must be absent
	}
	// The Attention section capitalizes the first letter of each entry for
	// presentation; the underlying data (Resource.Issues, finding Summary) is
	// unchanged. Expected phrases below reflect the rendered form.
	cases := []issueCase{
		// Healthy rows with no Wave 2 finding — Attention section must be absent.
		{demofixtures.ProdDbiID, nil},
		{demofixtures.ProdDbiAuroraID, nil},
		// Transitional (Wave-1 single phrase).
		{demofixtures.StagingDbiModifyingID, []string{"Modifying: DBInstanceClass"}},
		{demofixtures.StagingDbiRebootingID, []string{"Rebooting"}},
		// Broken (Wave-1 single phrase).
		{demofixtures.BrokenDbiStorageFullID, []string{"Storage-full"}},
		{demofixtures.BrokenDbiEncryptionLockedID, []string{"Encryption key unavailable"}},
		// Single Config Warnings.
		{demofixtures.WarnDbiNoBackupsID, []string{"No automated backups"}},
		{demofixtures.WarnDbiPublicID, []string{"Publicly accessible"}},
		{demofixtures.WarnDbiUnencryptedID, []string{"Unencrypted storage"}},
		{demofixtures.WarnDbiUnprotectedID, []string{"Deletion protection off"}},
		// Multi Config Warnings — first entry capitalized, rest stay lowercase (only
		// the first rune of each entry line is capitalized; these are separate entries).
		{demofixtures.WarnDbiMultiID, []string{"No automated backups", "Publicly accessible", "Unencrypted storage"}},
		// Wave-1 warning + Wave-2 maintenance — both must appear under Attention.
		{demofixtures.WarnDbiPublicMaintID, []string{"Publicly accessible", "os-upgrade"}},
		// Wave-2 only on Healthy row — Attention section present, Wave-2 Summary visible.
		{demofixtures.MaintDbiScheduledID, []string{"system-update"}},
		// Legacy fixture: all 4 Wave-1 warnings.
		{"db-public-no-encryption", []string{"No automated backups", "Publicly accessible", "Unencrypted storage", "Deletion protection off"}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.id, func(t *testing.T) {
			res := selectDBIByID(t, scenario, tc.id)
			scenario.OpenDetailResource("dbi", res)
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

// TestScenario_DBIVisual_HealthyRowsHaveNoIssuesPhrases is a dedicated regression
// pin for "Healthy silence" (spec §4 rule): Healthy rows must not render any
// Wave-1 config-warning phrase in the detail view. This is separate from
// TestScenario_DBIVisual_DetailSurfacesAllIssues so a failure is immediately
// identifiable as a "false positive" (noise on Healthy row) vs a missing phrase.
func TestScenario_DBIVisual_HealthyRowsHaveNoIssuesPhrases(t *testing.T) {
	scenario := fullIntegrationNewDemoScenario(t)
	runDemoStartup(t, scenario)
	scenario.OpenList("dbi")

	wave1Phrases := []string{
		"no automated backups",
		"publicly accessible",
		"unencrypted storage",
		"deletion protection off",
	}

	for _, id := range []string{demofixtures.ProdDbiID, demofixtures.ProdDbiAuroraID} {
		id := id
		t.Run(id, func(t *testing.T) {
			res := selectDBIByID(t, scenario, id)
			scenario.OpenDetailResource("dbi", res)
			scenario.ExpectNoAPIError()
			view := scenario.currentView()
			t.Log("\n" + view)

			expectNoAttentionSection(t, view)
			for _, phrase := range wave1Phrases {
				for _, line := range strings.Split(view, "\n") {
					// The phrase must not appear in any line after any Attention-like header.
					// Since there must be no Attention header (checked above), any occurrence
					// would be a spurious embedding — flag it.
					if strings.Contains(line, phrase) {
						t.Errorf("Healthy row %q unexpectedly contains Wave-1 phrase %q in line: %q\nfull view:\n%s", id, phrase, line, view)
					}
				}
			}
			scenario.Back()
		})
	}
}

// selectDBIByID looks up a concrete dbi resource from the demo clients so the
// scenario can call OpenDetailResource with a real resource value.
func selectDBIByID(t *testing.T, s *fullIntegrationScenario, id string) resource.Resource {
	t.Helper()
	return fullIntegrationMustFindResourceByID(t, s.clients, "dbi", id)
}

// runDemoStartup drives the Init → ClientsReadyMsg → AvailabilityPrefetchedMsg
// chain so Wave 2 enrichment runs against the demo fixtures. The scripted
// scenario constructor only applies a synthetic ClientsReadyMsg without
// draining follow-up commands, which bypasses the enrichment dispatch. This
// helper restores the full production path so render-gate assertions match
// what an `./a9s --demo` user actually sees on screen.
func runDemoStartup(t *testing.T, s *fullIntegrationScenario) {
	t.Helper()
	// Init returns a one-shot command that yields ClientsReadyMsg. Drain it:
	// the handler then produces AvailabilityPrefetchedMsg, which in turn
	// dispatches Wave 2 enrichment. applyAndDrain walks the full chain.
	initCmd := s.model.Init()
	for _, msg := range fullIntegrationCollectCmdMessages(initCmd) {
		// Skip the ClientsReadyMsg we've already synthesized via the scenario
		// constructor — re-applying it would reset state. Only process the
		// messages.AvailabilityPrefetchedMsg family when it arrives naturally
		// from the demoPrefetchCounts command chain.
		if _, ok := msg.(messages.ClientsReadyMsg); ok {
			s.applyAndDrain(msg)
			continue
		}
		s.applyMsg(msg)
	}
}
