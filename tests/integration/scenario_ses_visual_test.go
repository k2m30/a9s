//go:build integration

package integration

// scenario_ses_visual_test.go checks the rendered TUI output (not fetcher
// return values) for ses against the universal UI rules and
// docs/resources/ses.md.
//
// ses has 5 Wave-1 signals (PENDING / FAILED / TEMPORARY_FAILURE /
// NOT_STARTED / SendingEnabled==false) and 3 Wave-2 signals (PROBATION,
// SHUTDOWN, quota > 80%). The demo account is HEALTHY because an
// account-level SHUTDOWN would clobber every identity row, so Wave-2
// rendering is covered by aws_ses_issue_enrichment_test.go instead.

import (
	"testing"

	demofixtures "github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/resource"
)

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

	// Menu badge — counts distinct identities whose row color is
	// Warning / Broken (Wave-1) OR carry a Wave-2 finding. Demo default
	// is HEALTHY account + low quota, so no Wave-2 contribution; the 6
	// Wave-1 fixtures (PENDING, FAILED, TEMP_FAILURE, NOT_STARTED,
	// sending-disabled, multi) each contribute 1 → issues:6.
	scenario.ExpectMenuIssueCount("ses", 6)

	scenario.OpenList("ses")

	// Verification and sending state share the single Status column.
	for _, jargon := range []string{
		"Verification", "Sending", "CIS", " Flags", " Issues ",
		"NOBKP", "UNENC", "NOPROT",
	} {
		scenario.ExpectViewNotContains(jargon)
	}

	scenario.ExpectRowStatusBlank(sesHealthyDomainID)
	scenario.ExpectRowStatusBlank(sesHealthyEmailID)

	scenario.ExpectRowStatusEquals(sesPendingID, sesPhrasePending)
	scenario.ExpectRowStatusEquals(sesFailedID, sesPhraseFailed)
	scenario.ExpectRowStatusEquals(sesTempFailureID, sesPhraseTempFailure)
	scenario.ExpectRowStatusEquals(sesNotStartedID, sesPhraseNotStarted)
	scenario.ExpectRowStatusEquals(sesSuppressedID, sesPhraseSendingDisabled)

	scenario.ExpectRowStatusEquals(sesMultiW1ID, sesPhraseMultiW1)

	// The demo account is HEALTHY and under quota, so no Wave-2 finding fires
	// and Healthy rows carry no glyph.
	for _, id := range []string{
		sesHealthyDomainID,
		sesHealthyEmailID,
	} {
		scenario.ExpectRowNoGlyphPrefix(id)
	}

	// Non-green rows carry no glyph regardless of finding.
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

	root := selectSESByID(t, scenario, sesHealthyDomainID)
	scenario.OpenDetailResource("ses", root)
	scenario.ExpectNoAPIError()

	// SES event destinations ship to Firehose, not Kinesis Data Streams.
	for _, displayName := range []string{
		"Route 53 (DNS)",
		"Lambda Functions",
		"S3 Buckets",
		"SNS Topics",
	} {
		scenario.ExpectRelatedRowCountAtLeast(displayName, 1)
	}

	// The fixture has several rules on the "default" bus; the exact count is
	// incidental.
	scenario.ExpectRelatedRowCountAtLeast("EventBridge Rules", 1)

	// The list shows "verification failed (+1)"; the detail lists both
	// phrases.
	scenario.Back()
	multi := selectSESByID(t, scenario, sesMultiW1ID)
	scenario.OpenDetailResource("ses", multi)
	scenario.ExpectNoAPIError()

	view := scenario.currentView()
	t.Log("\n" + view)

	scenario.ExpectViewContains(sesDetailPhraseFailed)
	scenario.ExpectViewContains(sesDetailPhraseSDisabled)
}

// TestScenario_SESVisual_HealthyRowsHaveNoAttentionSection asserts Healthy
// identity rows render with no Attention section and no Wave-1 phrase in
// their detail view.
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

var _ = demofixtures.SESGraphRootIdentity
