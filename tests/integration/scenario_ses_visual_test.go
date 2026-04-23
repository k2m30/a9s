//go:build integration

package integration

// scenario_ses_visual_test.go — Phase 8 render-gate for the ses resource.
//
// Verifies the rendered TUI output (not fetcher return values) matches the
// universal UI rules and the §4 contract in docs/resources/ses.md.
//
// ses has 5 Wave-1 signals (PENDING / FAILED / TEMPORARY_FAILURE /
// NOT_STARTED / SendingEnabled==false) and 3 Wave-2 signals (PROBATION,
// SHUTDOWN, quota > 80%). The demo fixture's GetAccountDefault is HEALTHY
// by design — an always-SHUTDOWN demo fixture would clobber every identity
// row and destroy Wave-1 readability in the showroom. Wave-2 rendering
// (U3 / U4 / U7b S4-bump / U6 menu badge) is therefore covered at the
// unit-test level (aws_ses_issue_enrichment_test.go — 25 tests asserting
// the exact FieldUpdates map and Summary/Rows shape) rather than in this
// render gate. See Phase 9 report rationale.

import (
	"testing"

	demofixtures "github.com/k2m30/a9s/v3/internal/demo/fixtures"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// SES list column + §4 phrase + fixture-ID constants pinned locally from
// docs/resources/ses.md §4 "List text".
const (
	sesHealthyDomainID       = "acme-corp.com"
	sesHealthyEmailID        = "noreply@acme-corp.com"
	sesPendingID             = "alerts@acme-corp.com"
	sesFailedID              = "ses-failed.acme-corp.com"
	sesTempFailureID         = "temp.acme-corp.com"
	sesNotStartedID          = "notstarted.acme-corp.com"
	sesSuppressedID          = "suppressed@acme-corp.com"
	sesMultiW1ID             = "broken.acme-corp.com"
	sesPhrasePending         = "pending verification"
	sesPhraseFailed          = "verification failed"
	sesPhraseTempFailure     = "verify: temp failure"
	sesPhraseNotStarted      = "verification not started"
	sesPhraseSendingDisabled = "sending disabled"
	sesPhraseMultiW1         = "verification failed (+1)"
	sesDetailPhraseFailed    = "Verification failed"
	sesDetailPhraseSDisabled = "Sending disabled"
)

func TestScenario_SESVisual(t *testing.T) {
	scenario := fullIntegrationNewDemoScenario(t)

	// Drive the full demo startup so Wave 2 enrichment runs (even though
	// the default fixture returns HEALTHY, the chain must complete so
	// FieldUpdates are merged end-to-end).
	runDemoStartup(t, scenario)

	// -----------------------------------------------------------------
	// S1 menu badge — counts distinct identities whose row color is
	// Warning / Broken (Wave-1) OR carry a Wave-2 finding. Demo default
	// is HEALTHY account + low quota, so no Wave-2 contribution; the 6
	// Wave-1 fixtures (PENDING, FAILED, TEMP_FAILURE, NOT_STARTED,
	// sending-disabled, multi) each contribute 1 → issues:6.
	// -----------------------------------------------------------------
	scenario.ExpectMenuIssueCount("ses", 6)

	scenario.OpenList("ses")

	// -----------------------------------------------------------------
	// Universal column rules — no jargon columns. The old "Verification"
	// and "Sending" jargon columns were folded into a single Status
	// column in phase 7. Only identity + type + status remain.
	// -----------------------------------------------------------------
	for _, jargon := range []string{
		"Verification", "Sending", "CIS", " Flags", " Issues ",
		"NOBKP", "UNENC", "NOPROT",
	} {
		scenario.ExpectViewNotContains(jargon)
	}

	// -----------------------------------------------------------------
	// Wave 1 §4 phrases per fixture.
	// -----------------------------------------------------------------
	// Healthy rows: blank Status.
	scenario.ExpectRowStatusBlank(sesHealthyDomainID)
	scenario.ExpectRowStatusBlank(sesHealthyEmailID)

	// Single-signal Wave 1.
	scenario.ExpectRowStatusEquals(sesPendingID, sesPhrasePending)
	scenario.ExpectRowStatusEquals(sesFailedID, sesPhraseFailed)
	scenario.ExpectRowStatusEquals(sesTempFailureID, sesPhraseTempFailure)
	scenario.ExpectRowStatusEquals(sesNotStartedID, sesPhraseNotStarted)
	scenario.ExpectRowStatusEquals(sesSuppressedID, sesPhraseSendingDisabled)

	// Rule 7 U7a — multi-W1: top phrase + (+N-1) suffix.
	scenario.ExpectRowStatusEquals(sesMultiW1ID, sesPhraseMultiW1)

	// -----------------------------------------------------------------
	// Glyph rules.
	// -----------------------------------------------------------------
	// Rule 3 — Healthy rows with no Wave-2 finding must NOT carry a glyph
	// (demo default is HEALTHY account + under-quota — no finding fires).
	for _, id := range []string{
		sesHealthyDomainID,
		sesHealthyEmailID,
	} {
		scenario.ExpectRowNoGlyphPrefix(id)
	}

	// Rule 3 — non-green rows must NOT carry a glyph regardless of finding.
	for _, id := range []string{
		sesPendingID,
		sesFailedID,
		sesTempFailureID,
		sesNotStartedID,
		sesSuppressedID,
		sesMultiW1ID,
	} {
		scenario.ExpectRowNoGlyphPrefix(id)
	}

	// -----------------------------------------------------------------
	// Related panel — every §2 pivot with `count shown: yes` ≥ 1 for the
	// designated graph-root fixture (`acme-corp.com`). See user guidance
	// 2026-04-23: "related resources MUST work. if they don't it's a bug".
	// -----------------------------------------------------------------
	root := selectSESByID(t, scenario, sesHealthyDomainID)
	scenario.OpenDetailResource("ses", root)
	scenario.ExpectNoAPIError()

	for _, displayName := range []string{
		"Route 53 (DNS)",
		"EventBridge Rules",
		"Kinesis Streams",
		"Lambda Functions",
		"S3 Buckets",
		"SNS Topics",
	} {
		scenario.ExpectRelatedRowCountAtLeast(displayName, 1)
	}

	// -----------------------------------------------------------------
	// Rule 7 U7c / U7e — S5 Attention section surfaces every Wave-1
	// phrase on the multi-W1 fixture. The list shows "verification
	// failed (+1)" (rolled-up); the detail enumerates both phrases.
	// -----------------------------------------------------------------
	scenario.Back()
	multi := selectSESByID(t, scenario, sesMultiW1ID)
	scenario.OpenDetailResource("ses", multi)
	scenario.ExpectNoAPIError()

	// 8.4 user-visible sanity render (mandatory).
	view := scenario.currentView()
	t.Log("\n" + view)

	// Both §4 phrases from Resource.Issues must appear in the rendered
	// Attention section with first letter capitalized (unified renderer
	// applies capitalizeFirst at display time).
	scenario.ExpectViewContains(sesDetailPhraseFailed)
	scenario.ExpectViewContains(sesDetailPhraseSDisabled)
}

// TestScenario_SESVisual_HealthyRowsHaveNoAttentionSection asserts spec §4
// "Healthy silence": Healthy identity rows must render with no Attention
// section and no Wave-1 phrase in their detail view. Dedicated regression
// pin so a failure is immediately identifiable as false-positive noise.
func TestScenario_SESVisual_HealthyRowsHaveNoAttentionSection(t *testing.T) {
	scenario := fullIntegrationNewDemoScenario(t)
	runDemoStartup(t, scenario)
	scenario.OpenList("ses")

	wave1Phrases := []string{
		sesPhrasePending,
		sesPhraseFailed,
		sesPhraseTempFailure,
		sesPhraseNotStarted,
		sesPhraseSendingDisabled,
	}

	for _, id := range []string{sesHealthyDomainID, sesHealthyEmailID} {
		id := id
		t.Run(id, func(t *testing.T) {
			res := selectSESByID(t, scenario, id)
			scenario.OpenDetailResource("ses", res)
			scenario.ExpectNoAPIError()
			view := scenario.currentView()
			t.Log("\n" + view)

			expectNoAttentionSection(t, view)
			for _, phrase := range wave1Phrases {
				scenario.ExpectViewNotContains(phrase)
			}
			scenario.Back()
		})
	}
}

// selectSESByID looks up a concrete ses resource from the demo clients so the
// scenario can call OpenDetailResource with a real resource value.
func selectSESByID(t *testing.T, s *fullIntegrationScenario, id string) resource.Resource {
	t.Helper()
	return fullIntegrationMustFindResourceByID(t, s.clients, "ses", id)
}

// The demo fixture exports are referenced through a side-effect of the
// import — if future refactors drop the import we want the test to still
// compile. This assignment is a no-op at runtime.
var _ = demofixtures.SESGraphRootIdentity
