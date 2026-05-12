//go:build integration

package integration

// scenario_opensearch_visual_test.go — Phase 8 render-gate for the opensearch
// resource. Verifies the rendered TUI output (not fetcher return values) matches
// the universal UI rules and the §4 contract in docs/resources/opensearch.md.
//
// opensearch has NO Wave-1 signals (ListDomainNames returns only names +
// engine). All five §4 signals are Wave-2:
//   - Deleted         → Dim, "deleting: removal in progress"
//   - Isolated        → Broken, "isolated: quarantined by AWS"
//   - Processing      → Warning, "processing: config change in flight"
//   - UpdateAvailable → Healthy + `!`, "software update forced soon"
//   - EncryptionOff   → Healthy + `~`, "encryption at rest off"
//
// Because the Wave-2 enricher reads signal flags the fetcher already wrote
// from DescribeDomains, both waves fire in the same demo startup pass —
// every fixture lands with its Status/Issues/Finding populated.

import (
	"testing"

	demofixtures "github.com/k2m30/a9s/v3/internal/demo/fixtures"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// §4 phrases pinned locally — any drift in the fetcher or enricher surfaces
// here instead of in unit tests that could be rewritten without noticing.
const (
	openSearchPhraseDeleting      = "deleting: removal in progress"
	openSearchPhraseIsolated      = "isolated: quarantined by AWS"
	openSearchPhraseProcessing    = "processing: config change in flight"
	openSearchPhraseUpdate        = "software update forced soon"
	openSearchPhraseEncryption    = "encryption at rest off"
	openSearchPhraseProcessingP1  = "processing: config change in flight (+1)"
	openSearchPhraseUpdateP1      = "software update forced soon (+1)"
	openSearchDetailPhraseUpdate  = "Software update forced soon"
	openSearchDetailPhraseEncOff  = "Encryption at rest off"
	openSearchDetailPhraseProcess = "Processing: config change in flight"
)

func TestScenario_OpenSearchVisual(t *testing.T) {
	// Isolate config from the developer's ~/.a9s/ so the test uses
	// defaults_databases.go (Status column) rather than a stale user yaml.
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	scenario := fullIntegrationNewDemoScenario(t)
	runDemoStartup(t, scenario)

	// -----------------------------------------------------------------
	// S1 menu badge — `unifiedIssueCount` is the union of:
	//   (a) rows whose Color.IsIssue() (Warning/Broken), and
	//   (b) rows carrying a `!` severity EnrichmentFinding.
	// For opensearch:
	//   - IsolatedDomain            (Broken)
	//   - ProcessingDomain          (Warning)
	//   - ProcessingPlusUpdateDomain (Warning)
	//   - UpdateAvailableDomain     (Healthy + `!`)
	//   - MultiBackgroundDomain     (Healthy + `!`)
	// DeletingDomain (Dim) does not count; `~` never bumps.
	// -----------------------------------------------------------------
	scenario.ExpectMenuIssueCount("opensearch", 5)

	scenario.OpenList("opensearch")

	// -----------------------------------------------------------------
	// Universal column rules — no jargon columns.
	// The old "Processing" column (key: domain_processing_status) was
	// folded into Status in phase 7.
	// -----------------------------------------------------------------
	for _, jargon := range []string{
		"CIS", " Flags", " Issues ", "NOBKP", "UNENC", "NOPROT", "PUB",
	} {
		scenario.ExpectViewNotContains(jargon)
	}
	// "Processing" as a column header is banned — but the word appears
	// in §4 phrases like "processing: config change in flight", so we
	// scope the check to the header form.
	scenario.ExpectViewNotContains("Processing    ")

	// -----------------------------------------------------------------
	// Healthy rows: blank Status.
	// -----------------------------------------------------------------
	for _, id := range []string{
		demofixtures.HealthyBaselineDomain,
		demofixtures.GraphRootDomain,
	} {
		scenario.ExpectRowStatusBlank(id)
	}

	// -----------------------------------------------------------------
	// §4 phrases per state bucket.
	// -----------------------------------------------------------------
	scenario.ExpectRowStatusEquals(demofixtures.DeletingDomain, openSearchPhraseDeleting)
	scenario.ExpectRowStatusEquals(demofixtures.IsolatedDomain, openSearchPhraseIsolated)
	scenario.ExpectRowStatusEquals(demofixtures.ProcessingDomain, openSearchPhraseProcessing)
	scenario.ExpectRowStatusEquals(demofixtures.UpdateAvailableDomain, openSearchPhraseUpdate)
	scenario.ExpectRowStatusEquals(demofixtures.EncryptionOffDomain, openSearchPhraseEncryption)

	// Rule 7 — multi-finding suffix.
	// Processing + UpdateAvailable → hard-state wins, suffix +1.
	scenario.ExpectRowStatusEquals(demofixtures.ProcessingPlusUpdateDomain, openSearchPhraseProcessingP1)
	// Post-AS-140 the multi-background row carries a single visible Wave-2
	// finding (the `~` encryption-off signal is no longer counted as an
	// extra hidden finding), so no `(+1)` suffix.
	scenario.ExpectRowStatusEquals(demofixtures.MultiBackgroundDomain, openSearchPhraseUpdate)

	// -----------------------------------------------------------------
	// Glyph rules.
	// -----------------------------------------------------------------
	// Rule 3 (U3/U4) — `!` / `~` glyph ONLY on Healthy + Wave-2 rows.
	scenario.ExpectRowNamePrefix(demofixtures.UpdateAvailableDomain, "! ")
	scenario.ExpectRowNamePrefix(demofixtures.EncryptionOffDomain, "~ ")
	// U7d — `!` beats `~` when both present on same Healthy row.
	scenario.ExpectRowNamePrefix(demofixtures.MultiBackgroundDomain, "! ")

	// Rule 3 — non-green rows carry no glyph regardless of any background
	// finding attached. ProcessingPlusUpdate has a Wave-2 finding but the
	// Warning color is the signal; glyph suppressed.
	for _, id := range []string{
		demofixtures.DeletingDomain,
		demofixtures.IsolatedDomain,
		demofixtures.ProcessingDomain,
		demofixtures.ProcessingPlusUpdateDomain,
	} {
		scenario.ExpectRowNoGlyphPrefix(id)
	}
	// Healthy rows with no finding: also glyph-free.
	for _, id := range []string{
		demofixtures.HealthyBaselineDomain,
		demofixtures.GraphRootDomain,
	} {
		scenario.ExpectRowNoGlyphPrefix(id)
	}

	// -----------------------------------------------------------------
	// Related panel — every §2 pivot with `count shown: yes` ≥ 1 on
	// the graph-root fixture (acme-logs). acm, alarm, cfn, kms, logs,
	// sg, subnet, vpc — 8 pivots.
	// -----------------------------------------------------------------
	root := selectOpenSearchByID(t, scenario, demofixtures.GraphRootDomain)
	scenario.OpenDetailResource("opensearch", root)
	scenario.ExpectNoAPIError()

	for _, displayName := range []string{
		"ACM Certificates",
		"CW Alarms",
		"CloudFormation",
		"KMS Key",
		"Log Groups",
		"Security Groups",
		"Subnets",
		"VPC",
	} {
		scenario.ExpectRelatedRowCountAtLeast(displayName, 1)
	}

	scenario.Back()

	// -----------------------------------------------------------------
	// Rule 7 U7c — S5 Attention section surfaces every Wave-2 finding
	// on the multi-background fixture. The list Status is
	// "software update forced soon (+1)" (rolled-up); the detail must
	// enumerate BOTH the top finding's Summary AND the hidden
	// encryption-off phrase (carried as an "Additional" row).
	// -----------------------------------------------------------------
	multi := selectOpenSearchByID(t, scenario, demofixtures.MultiBackgroundDomain)
	scenario.OpenDetailResource("opensearch", multi)
	scenario.ExpectNoAPIError()

	// 8.4 user-visible sanity render (mandatory).
	view := scenario.currentView()
	t.Log("\n" + view)

	// The top Wave-2 Summary (Attention renders with first letter capitalized).
	scenario.ExpectViewContains(openSearchDetailPhraseUpdate)
	// The hidden `~` finding surfaces via the Additional row.
	scenario.ExpectViewContains(openSearchPhraseEncryption)

	scenario.Back()

	// -----------------------------------------------------------------
	// U7e — detail enumerates every Wave-1 phrase on rows where the
	// fetcher populated Resource.Issues. opensearch has no Wave 1, but
	// the hard-state phrases are carried in Issues for the detail view
	// (so Processing/Isolated/Deleted rows enumerate their phrase with
	// first-letter capitalization).
	// -----------------------------------------------------------------
	processing := selectOpenSearchByID(t, scenario, demofixtures.ProcessingPlusUpdateDomain)
	scenario.OpenDetailResource("opensearch", processing)
	scenario.ExpectNoAPIError()

	// Capitalized Wave-1-carrying hard-state phrase.
	scenario.ExpectViewContains(openSearchDetailPhraseProcess)
	// The Wave-2 finding attached to the Warning row still surfaces in S5
	// (rule 7 — "no finding silently disappears").
	scenario.ExpectViewContains(openSearchDetailPhraseUpdate)
}

// TestScenario_OpenSearchVisual_HealthyRowsHaveNoAttentionSection asserts spec
// §4 "Healthy silence": Healthy rows must render with no Attention section
// and no Wave-2 phrase in their detail view. Regression pin for
// false-positive noise on the showroom instance.
func TestScenario_OpenSearchVisual_HealthyRowsHaveNoAttentionSection(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	scenario := fullIntegrationNewDemoScenario(t)
	runDemoStartup(t, scenario)
	scenario.OpenList("opensearch")

	findingPhrases := []string{
		openSearchPhraseDeleting,
		openSearchPhraseIsolated,
		openSearchPhraseProcessing,
		openSearchPhraseUpdate,
		openSearchPhraseEncryption,
	}

	for _, id := range []string{
		demofixtures.HealthyBaselineDomain,
	} {
		id := id
		t.Run(id, func(t *testing.T) {
			res := selectOpenSearchByID(t, scenario, id)
			scenario.OpenDetailResource("opensearch", res)
			scenario.ExpectNoAPIError()
			view := scenario.currentView()
			t.Log("\n" + view)

			expectNoAttentionSection(t, view)
			for _, phrase := range findingPhrases {
				scenario.ExpectViewNotContains(phrase)
			}
			scenario.Back()
		})
	}
}

// selectOpenSearchByID looks up a concrete opensearch resource from the demo
// clients so the scenario can call OpenDetailResource with a real resource.
func selectOpenSearchByID(t *testing.T, s *fullIntegrationScenario, id string) resource.Resource {
	t.Helper()
	return fullIntegrationMustFindResourceByID(t, s.clients, "opensearch", id)
}
