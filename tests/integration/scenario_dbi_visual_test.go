//go:build integration

package integration

// scenario_dbi_visual_test.go checks the rendered TUI output (not fetcher
// return values) for dbi against the universal UI rules and
// docs/resources/dbi.md.
//
// Demo mode runs Wave 2 enrichment against fixture data (typed fakes
// implement the enricher APIs), so every assertion below exercises the real
// Update loop end-to-end.

import (
	"strings"
	"testing"

	demofixtures "github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
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

	for _, jargon := range []string{"CIS", " Flags", "NOBKP", "UNENC", "NOPROT", "cis_flags"} {
		scenario.ExpectViewNotContains(jargon)
	}

	scenario.ExpectRowStatusBlank(demofixtures.ProdDbiID)
	scenario.ExpectRowStatusBlank(demofixtures.ProdDbiAuroraID)

	scenario.ExpectRowStatusEquals(demofixtures.StagingDbiModifyingID, "modifying: DBInstanceClass")
	scenario.ExpectRowStatusEquals(demofixtures.StagingDbiRebootingID, "rebooting")

	scenario.ExpectRowStatusEquals(demofixtures.BrokenDbiStorageFullID, "storage-full")
	scenario.ExpectRowStatusEquals(demofixtures.BrokenDbiEncryptionLockedID, "encryption key unavailable")

	scenario.ExpectRowStatusEquals(demofixtures.WarnDbiNoBackupsID, "no automated backups")
	scenario.ExpectRowStatusEquals(demofixtures.WarnDbiPublicID, "public endpoint")
	scenario.ExpectRowStatusEquals(demofixtures.WarnDbiUnencryptedID, "unencrypted storage")
	scenario.ExpectRowStatusEquals(demofixtures.WarnDbiUnprotectedID, "deletion protection off")

	scenario.ExpectRowStatusEquals(demofixtures.WarnDbiMultiID, "no automated backups (+2)")

	// The (+1) counts the hidden Wave-2 finding.
	scenario.ExpectRowStatusEquals(demofixtures.WarnDbiPublicMaintID, "public endpoint (+1)")

	// The finding's Severity: SevWarn also promotes the row itself to
	// Warning color via colorFromAnyFinding.
	scenario.ExpectRowStatusEquals(demofixtures.MaintDbiScheduledID, "maintenance scheduled")

	// colorDBI (catalog_databases.go) resolves color via colorFromAnyFinding
	// first. dbiCodePendingMaintenance is declared Severity: SevWarn, so
	// MaintDbiScheduledID renders Warning row color directly and carries no
	// glyph.
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
		demofixtures.MaintDbiScheduledID,
		demofixtures.ProdDbiID,
		demofixtures.ProdDbiAuroraID,
	} {
		scenario.ExpectRowNoGlyphPrefix(id)
	}

	prod := selectDBIByID(t, scenario, demofixtures.ProdDbiID)
	scenario.OpenDetailResource("dbi", prod)
	scenario.ExpectNoAPIError()
	for _, displayName := range []string{
		"Security Groups", "KMS Key", "Subnets", "CloudWatch Alarms",
		"DB Instance Snapshots", "Log Groups", "VPC", "Secrets Manager",
		"IAM Roles", "Network Interfaces",
	} {
		scenario.ExpectRelatedRowCountAtLeast(displayName, 1)
	}

	// Aurora cluster instances take no dbi-snap (DescribeDBSnapshots rejects
	// on Aurora cluster members — Aurora cluster snapshots live in dbc-snap),
	// so "DB Instance Snapshots" is asserted on ProdDbiID above. ct-events
	// shows no count for windowed LookupEvents.
	scenario.Back()
	aurora := selectDBIByID(t, scenario, demofixtures.ProdDbiAuroraID)
	scenario.OpenDetailResource("dbi", aurora)
	scenario.ExpectNoAPIError()
	for _, displayName := range []string{
		"Security Groups", "KMS Key", "Subnets", "CloudWatch Alarms",
		"Log Groups", "VPC", "Secrets Manager",
		"IAM Roles", "Network Interfaces", "RDS Clusters",
	} {
		scenario.ExpectRelatedRowCountAtLeast(displayName, 1)
	}

	// The detail view shows every finding even on a row whose Status is Wave 1.
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
// after that header, in the given order. Fails with the full rendered frame
// on any violation.
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
// an "Attention" section header — i.e. the row has no active signals at all.
func expectNoAttentionSection(t *testing.T, view string) {
	t.Helper()
	lines := strings.Split(view, "\n")
	if hdr := attentionSectionHeaderLine(lines); hdr >= 0 {
		t.Fatalf("Attention section header unexpectedly present (line %d). view:\n%s", hdr, view)
	}
}

// TestScenario_DBIVisual_DetailSurfacesAllIssues asserts every finding is
// individually visible in the detail view.
//
// Assertions are tighter than plain ExpectViewContains: phrases must appear AFTER
// the "Attention (N)" section header and in precedence order.
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
		{demofixtures.ProdDbiID, nil},
		{demofixtures.ProdDbiAuroraID, nil},
		{demofixtures.StagingDbiModifyingID, []string{"Modifying: DBInstanceClass"}},
		{demofixtures.StagingDbiRebootingID, []string{"Rebooting"}},
		{demofixtures.BrokenDbiStorageFullID, []string{"Storage-full"}},
		{demofixtures.BrokenDbiEncryptionLockedID, []string{"Encryption key unavailable"}},
		{demofixtures.WarnDbiNoBackupsID, []string{"No automated backups"}},
		{demofixtures.WarnDbiPublicID, []string{"Public endpoint"}},
		{demofixtures.WarnDbiUnencryptedID, []string{"Unencrypted storage"}},
		{demofixtures.WarnDbiUnprotectedID, []string{"Deletion protection off"}},
		// Each entry is a separate line, so each one's first rune is capitalized.
		{demofixtures.WarnDbiMultiID, []string{"No automated backups", "Public endpoint", "Unencrypted storage"}},
		{demofixtures.WarnDbiPublicMaintID, []string{"Public endpoint", "os-upgrade"}},
		{demofixtures.MaintDbiScheduledID, []string{"system-update"}},
		// The bulk pool sets DeletionProtection, so only warn-dbi-unprotected
		// carries the deletion-protection finding.
		{"db-public-no-encryption", []string{"No automated backups", "Public endpoint", "Unencrypted storage"}},
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

// TestScenario_DBIVisual_HealthyRowsHaveNoIssuesPhrases pins "Healthy
// silence": Healthy rows render no Wave-1 config-warning phrase in the
// detail view. This is separate from
// TestScenario_DBIVisual_DetailSurfacesAllIssues so a failure is immediately
// identifiable as a "false positive" (noise on Healthy row) vs a missing phrase.
func TestScenario_DBIVisual_HealthyRowsHaveNoIssuesPhrases(t *testing.T) {
	scenario := fullIntegrationNewDemoScenario(t)
	runDemoStartup(t, scenario)
	scenario.OpenList("dbi")

	wave1Phrases := []string{
		"no automated backups",
		"public endpoint",
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
// helper runs the full production path so assertions match what an
// `./a9s --demo` user sees on screen.
func runDemoStartup(t *testing.T, s *fullIntegrationScenario) {
	t.Helper()
	// Init returns a one-shot command that yields ClientsReadyMsg. Drain it:
	// the handler then produces AvailabilityPrefetchedMsg, which in turn
	// dispatches Wave 2 enrichment. applyAndDrain walks the full chain.
	initCmd := s.model.Init()
	for _, msg := range fullIntegrationCollectCmdMessages(initCmd) {
		if _, ok := msg.(messages.ClientsReady); ok {
			s.applyAndDrain(msg)
			continue
		}
		s.applyMsg(msg)
	}
}
