//go:build integration

package integration

// scenario_transfer_visual_test.go — Phase 8 render-gate for the transfer
// resource. Verifies the rendered TUI output (not fetcher return values)
// matches the universal UI rules and the §4 contract in
// docs/resources/transfer.md.
//
// transfer uses the in-fetcher N+1 pattern (ListServers + DescribeServer per
// id), so all findings land during demo startup with no separate enricher.
// Every issue-severity finding is color-bearing — no glyph-on-green exists:
//   - OFFLINE/STARTING/STOPPING/STOP_FAILED → Warning
//   - START_FAILED → Broken
//   - legacy security policy / no activity logging → Warning (Describe-borne)
//   - DescribeServer denied → Warning rich degraded row (list fields KEPT)
// The State enum has no deleted/terminal value, so no Dim bucket exists.

import (
	"testing"

	demofixtures "github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/resource"
)

// §4 phrases pinned locally — any drift in the fetcher surfaces here instead
// of in unit tests that could be rewritten without noticing.
const (
	transferPhraseOffline       = "offline: not accepting transfers"
	transferPhraseStarting      = "starting"
	transferPhraseStopping      = "stopping"
	transferPhraseStartFailed   = "start failed"
	transferPhraseStopFailed    = "stop failed"
	transferPhraseLegacyPolicy  = "legacy security policy"
	transferPhraseNoLogging     = "no activity logging"
	transferPhraseDetailsDenied = "details denied"

	// Rule-7 rolled-up form: OFFLINE + legacy policy + no logging.
	transferPhraseMultiP2 = "offline: not accepting transfers (+2)"

	// Child-row phrase (agreements child view).
	transferPhraseAgreementInactive = "inactive: partner traffic rejected"

	// Detail-view Attention entries (capitalizeFirst applied at render).
	transferDetailOffline       = "Offline: not accepting transfers"
	transferDetailLegacyPolicy  = "Legacy security policy"
	transferDetailNoLogging     = "No activity logging"
	transferDetailDetailsDenied = "Details denied"
)

func TestScenario_TransferVisual(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	scenario := fullIntegrationNewDemoScenario(t)
	runDemoStartup(t, scenario)

	// S1 menu badge — issue-COLORED rows: offline, starting, stopping,
	// stop-failed, legacy-policy, no-logging, multi, details-denied (8
	// Warning) + start-failed (1 Broken) = 9.
	scenario.ExpectMenuIssueCount("transfer", 9)

	scenario.OpenList("transfer")

	// Universal column rules — no jargon columns.
	for _, jargon := range []string{
		"CIS", " Flags", " Issues ", "NOBKP", "UNENC", "NOPROT", "PUB ",
	} {
		scenario.ExpectViewNotContains(jargon)
	}

	// Healthy rows: blank Status.
	for _, id := range []string{
		demofixtures.ProdAS2GatewayID,
		demofixtures.SftpUsersProdID,
		demofixtures.SftpLambdaAuthID,
	} {
		scenario.ExpectRowStatusBlank(id)
	}

	// §4 phrases per state bucket.
	scenario.ExpectRowStatusEquals(demofixtures.WarnTransferOfflineID, transferPhraseOffline)
	scenario.ExpectRowStatusEquals(demofixtures.WarnTransferStartingID, transferPhraseStarting)
	scenario.ExpectRowStatusEquals(demofixtures.WarnTransferStoppingID, transferPhraseStopping)
	scenario.ExpectRowStatusEquals(demofixtures.BrokenTransferStartFailedID, transferPhraseStartFailed)
	scenario.ExpectRowStatusEquals(demofixtures.WarnTransferStopFailedID, transferPhraseStopFailed)

	// Describe-borne config findings (yellow rows, color is the signal).
	scenario.ExpectRowStatusEquals(demofixtures.WarnTransferLegacyPolicyID, transferPhraseLegacyPolicy)
	scenario.ExpectRowStatusEquals(demofixtures.WarnTransferNoLoggingID, transferPhraseNoLogging)

	// Listed-but-denied server: RICH degraded row — list fields kept (State
	// ONLINE ⇒ no state finding), details-denied is the only phrase.
	scenario.ExpectRowStatusEquals(demofixtures.WarnTransferDetailsDeniedID, transferPhraseDetailsDenied)

	// Rule 7 — three findings stack: state phrase wins, (+2) suffix.
	scenario.ExpectRowStatusEquals(demofixtures.WarnTransferMultiID, transferPhraseMultiP2)

	// Glyph rules: NO glyph anywhere for transfer — every finding is
	// color-bearing, so no row stays green while carrying one.
	for _, id := range []string{
		demofixtures.ProdAS2GatewayID,
		demofixtures.SftpUsersProdID,
		demofixtures.SftpLambdaAuthID,
		demofixtures.WarnTransferOfflineID,
		demofixtures.WarnTransferStartingID,
		demofixtures.WarnTransferStoppingID,
		demofixtures.BrokenTransferStartFailedID,
		demofixtures.WarnTransferStopFailedID,
		demofixtures.WarnTransferLegacyPolicyID,
		demofixtures.WarnTransferNoLoggingID,
		demofixtures.WarnTransferMultiID,
		demofixtures.WarnTransferDetailsDeniedID,
	} {
		scenario.ExpectRowNoGlyphPrefix(id)
	}

	// Related panel — every §2 pivot with `count shown: yes` ≥ 1 on the
	// graph root: role 1, vpc 1, subnet 3, vpce 1, logs 2, acm 1.
	root := selectTransferByID(t, scenario, demofixtures.ProdAS2GatewayID)
	scenario.OpenDetailResource("transfer", root)
	scenario.ExpectNoAPIError()

	for displayName, atLeast := range map[string]int{
		"IAM Roles":        1,
		"VPC":              1,
		"Subnets":          3,
		"VPC Endpoints":    1,
		"Log Groups":       2,
		"ACM Certificates": 1,
		"Elastic IPs":      3,
	} {
		scenario.ExpectRelatedRowCountAtLeast(displayName, atLeast)
	}

	scenario.Back()

	// Conditional pivot: the AWS_LAMBDA-IdP fixture resolves its authorizer.
	lambdaAuth := selectTransferByID(t, scenario, demofixtures.SftpLambdaAuthID)
	scenario.OpenDetailResource("transfer", lambdaAuth)
	scenario.ExpectNoAPIError()
	scenario.ExpectRelatedRowCountAtLeast("Lambda Functions", 1)

	scenario.Back()

	// Rule 7 U7c/U7e — the multi fixture's detail enumerates ALL three
	// findings as their own capitalized Attention entries.
	multi := selectTransferByID(t, scenario, demofixtures.WarnTransferMultiID)
	scenario.OpenDetailResource("transfer", multi)
	scenario.ExpectNoAPIError()

	// 8.4 user-visible sanity render (mandatory).
	t.Log("\n" + scenario.currentView())

	scenario.ExpectViewContains(transferDetailOffline)
	scenario.ExpectViewContains(transferDetailLegacyPolicy)
	scenario.ExpectViewContains(transferDetailNoLogging)

	scenario.Back()

	// Rich degraded row in detail: the details-denied entry surfaces with
	// transfer's own §4 sentence (not the generic name-only wording), and
	// the list-borne facts stay visible.
	denied := selectTransferByID(t, scenario, demofixtures.WarnTransferDetailsDeniedID)
	scenario.OpenDetailResource("transfer", denied)
	scenario.ExpectNoAPIError()
	scenario.ExpectViewContains(transferDetailDetailsDenied)
	scenario.ExpectViewContains("only the listed fields are visible")

	scenario.Back()

	// Agreements child view (`e` on the graph root): both agreements render;
	// the INACTIVE one carries its §2.1 phrase.
	scenario.ApplyFilter(demofixtures.ProdAS2GatewayID)
	scenario.Press("e")
	scenario.ExpectViewContains(demofixtures.AgreementProdPartnerID)
	scenario.ExpectViewContains(demofixtures.AgreementOldPartnerID)
	scenario.ExpectViewContains(transferPhraseAgreementInactive)
	t.Log("\n" + scenario.currentView())

	scenario.AssertNoEnrichmentErrors()
}

// TestScenario_TransferVisual_HealthySilence — spec §4 "Healthy silence": the
// showroom row renders no Attention section and no finding phrase, and the
// wave-3 anti-tests hold (UserCount 0 on the AS2 graph root produces no
// finding; LoggingRole-nil-with-structured-logs never fires no-logging).
func TestScenario_TransferVisual_HealthySilence(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	scenario := fullIntegrationNewDemoScenario(t)
	runDemoStartup(t, scenario)
	scenario.OpenList("transfer")

	res := selectTransferByID(t, scenario, demofixtures.ProdAS2GatewayID)
	scenario.OpenDetailResource("transfer", res)
	scenario.ExpectNoAPIError()
	view := scenario.currentView()
	t.Log("\n" + view)

	expectNoAttentionSection(t, view)
	for _, phrase := range []string{
		transferPhraseLegacyPolicy, transferPhraseNoLogging,
		transferPhraseDetailsDenied, "FilesIn", "FilesOut",
	} {
		scenario.ExpectViewNotContains(phrase)
	}
}

// selectTransferByID looks up a concrete transfer resource from the demo
// clients so the scenario can call OpenDetailResource with a real resource.
func selectTransferByID(t *testing.T, s *fullIntegrationScenario, id string) resource.Resource {
	t.Helper()
	return fullIntegrationMustFindResourceByID(t, s.clients, "transfer", id)
}
