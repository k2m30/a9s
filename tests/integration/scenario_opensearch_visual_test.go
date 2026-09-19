//go:build integration

package integration

// scenario_opensearch_visual_test.go checks the rendered TUI output (not
// fetcher return values) for opensearch against the universal UI rules and
// docs/resources/opensearch.md.
//
// Every opensearch signal is Wave 1. DescribeDomains is the fetcher's own call
// and DomainStatus carries all of them, so there is no second pass and no
// signal that shows a glyph without colouring its own row:
//   - Deleted           → Dim,     "deleting: removal in progress"
//   - Isolated          → Broken,  "isolated: quarantined by AWS"
//   - PolicyPublic      → Broken,  "reachable outside a VPC"
//   - Processing        → Warning, "processing: config change in flight"
//   - UpdateForcedSoon  → Warning, "software update forced soon"
//   - EncryptionOff     → Warning, "encryption at rest off"
//   - HTTPSNotEnforced  → Warning, "HTTPS not enforced"
//   - NodeToNodeOff     → Warning, "node-to-node encryption off"

import (
	"testing"

	demofixtures "github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/resource"
)

const (
	openSearchPhraseDeleting   = "deleting: removal in progress"
	openSearchPhraseIsolated   = "isolated: quarantined by AWS"
	openSearchPhraseProcessing = "processing: config change in flight"
	openSearchPhraseUpdate     = "software update forced soon"
	openSearchPhraseEncryption = "encryption at rest off"
	// acme-search-alpha carries processing and update-forced, acme-metrics
	// carries update-forced and encryption-off. All four are Wave-1 warnings,
	// so the phrase and the row colour resolve through one selection and the
	// tie falls to the order the predicate emits them in: the lifecycle state
	// leads on the first row, the update on the second.
	openSearchPhraseProcessingP1  = "processing: config change in flight (+1)"
	openSearchPhraseUpdateP1      = "software update forced soon (+1)"
	openSearchDetailPhraseUpdate  = "Software update forced soon"
	openSearchDetailPhraseEncOff  = "Encryption at rest off"
	openSearchDetailPhraseProcess = "Processing: config change in flight"
)

func TestScenario_OpenSearchVisual(t *testing.T) {
	// A user-dir ~/.a9s/ overlay wins the merge, so config points at an empty
	// dir and defaults_databases.go applies.
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	scenario := fullIntegrationNewDemoScenario(t)
	runDemoStartup(t, scenario)

	// Menu badge — the count of rows whose colour is an issue. Colour is
	// the worst severity among a row's findings, so a Warn finding counts
	// wherever it came from and a Dim one never does.
	// Over the 13 opensearch fixtures:
	//   Broken (2):  legacy-search-isolated (isolated),
	//                acme-public-search (reachable outside a VPC)
	//   Warning (8): acme-events and acme-search-alpha (processing),
	//                acme-product-search and acme-metrics (update forced),
	//                legacy-analytics (encryption at rest off),
	//                acme-http-search (HTTPS not enforced),
	//                acme-plaintext-nodes (node-to-node encryption off),
	//                warn-os-details-unavailable (degraded name-only row —
	//                absent from the DescribeDomains response, a non-auth
	//                degradation rather than an IAM denial)
	//   Not counted (3): staging-analytics and acme-logs are clean, and
	//                obsolete-tenant-logs is Dim.
	// 2 + 8 = 10. The encryption and transport rows are Wave 1 findings, so
	// they colour their own rows and count here.
	scenario.ExpectMenuIssueCount("opensearch", 10)

	scenario.OpenList("opensearch")

	for _, jargon := range []string{
		"CIS", " Flags", " Issues ", "NOBKP", "UNENC", "NOPROT", "PUB",
	} {
		scenario.ExpectViewNotContains(jargon)
	}
	// The word "Processing" appears in phrases like "processing: config change
	// in flight", so the check is scoped to the header form.
	scenario.ExpectViewNotContains("Processing    ")

	for _, id := range []string{
		demofixtures.HealthyBaselineDomain,
		demofixtures.GraphRootDomain,
	} {
		scenario.ExpectRowStatusBlank(id)
	}

	scenario.ExpectRowStatusEquals(demofixtures.DeletingDomain, openSearchPhraseDeleting)
	scenario.ExpectRowStatusEquals(demofixtures.IsolatedDomain, openSearchPhraseIsolated)
	scenario.ExpectRowStatusEquals(demofixtures.ProcessingDomain, openSearchPhraseProcessing)
	scenario.ExpectRowStatusEquals(demofixtures.UpdateAvailableDomain, openSearchPhraseUpdate)
	scenario.ExpectRowStatusEquals(demofixtures.EncryptionOffDomain, openSearchPhraseEncryption)

	// Processing + UpdateAvailable → hard-state wins, suffix +1.
	scenario.ExpectRowStatusEquals(demofixtures.ProcessingPlusUpdateDomain, openSearchPhraseProcessingP1)
	// The multi-background row carries update-forced and encryption-off, both
	// Wave-1 warnings, so the update leads and the suffix counts the second.
	scenario.ExpectRowStatusEquals(demofixtures.MultiBackgroundDomain, openSearchPhraseUpdateP1)

	// colorOpenSearch (catalog_databases.go) resolves color via
	// colorFromAnyFinding first, so every finding colours its own row and the
	// row color carries the signal instead of a glyph.
	for _, id := range []string{
		demofixtures.DeletingDomain,
		demofixtures.IsolatedDomain,
		demofixtures.ProcessingDomain,
		demofixtures.ProcessingPlusUpdateDomain,
		demofixtures.UpdateAvailableDomain,
		demofixtures.EncryptionOffDomain,
		demofixtures.MultiBackgroundDomain,
	} {
		scenario.ExpectRowNoGlyphPrefix(id)
	}
	for _, id := range []string{
		demofixtures.HealthyBaselineDomain,
		demofixtures.GraphRootDomain,
	} {
		scenario.ExpectRowNoGlyphPrefix(id)
	}

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

	// The list Status rolls the multi-background findings up as
	// "software update forced soon (+1)"; the detail lists each as its own
	// Attention entry.
	multi := selectOpenSearchByID(t, scenario, demofixtures.MultiBackgroundDomain)
	scenario.OpenDetailResource("opensearch", multi)
	scenario.ExpectNoAPIError()

	view := scenario.currentView()
	t.Log("\n" + view)

	scenario.ExpectViewContains(openSearchDetailPhraseUpdate)
	scenario.ExpectViewContains(openSearchDetailPhraseEncOff)

	scenario.Back()

	// Hard-state phrases are carried in Resource.Issues, so the detail view
	// lists them alongside the other findings.
	processing := selectOpenSearchByID(t, scenario, demofixtures.ProcessingPlusUpdateDomain)
	scenario.OpenDetailResource("opensearch", processing)
	scenario.ExpectNoAPIError()

	scenario.ExpectViewContains(openSearchDetailPhraseProcess)
	scenario.ExpectViewContains(openSearchDetailPhraseUpdate)
}

// TestScenario_OpenSearchVisual_HealthyRowsHaveNoAttentionSection asserts
// Healthy rows render with no Attention section and no signal phrase in their
// detail view.
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
