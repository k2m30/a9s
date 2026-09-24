//go:build integration

package integration

// scenario_ddb_visual_test.go checks the rendered TUI output (not fetcher
// return values) for ddb against the universal UI rules and
// docs/resources/ddb.md. ListTables returns names only, so TableStatus and
// PITR come from Wave 2.

import (
	"testing"

	demofixtures "github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/resource"
)

const (
	ddbPhraseCreating      = "creating"
	ddbPhraseUpdating      = "updating"
	ddbPhraseDeleting      = "deleting"
	ddbPhraseArchiving     = "archiving"
	ddbPhraseKMSLost       = "kms key inaccessible"
	ddbPhraseArchived      = "archived: kms key lost"
	ddbPhrasePITROff       = "point-in-time recovery disabled"
	ddbPhraseArchivedPlus1 = "archived: kms key lost (+1)"

	// ddbDetailPITROffCapitalize is the Attention-section rendering of
	// ddbPhrasePITROff — the Attention block capitalizes the first
	// letter of every finding phrase, so the Status
	// column's lowercase phrase and the detail view's capitalized phrase
	// are two different literal strings for the same finding.
	ddbDetailPITROffCapitalize = "Point-in-time recovery disabled"
)

func TestScenario_DDBVisual(t *testing.T) {
	// A user-dir ~/.a9s/views/ddb.yaml overlay wins the merge in
	// config.GetViewDef, so config.Load points at a fresh empty dir and only
	// defaults apply.
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())

	scenario := fullIntegrationNewDemoScenario(t)

	// Drive the real demo startup so Wave-2 enrichment runs against the typed fake.
	runDemoStartup(t, scenario)

	// The menu badge is read while the main menu is still the current view.
	// It counts rows whose Wave-1-only colour IsIssue, plus Healthy rows
	// carrying a Wave-2 `!`; a Healthy row with only a Wave-2 `~` never bumps it.
	scenario.ExpectMenuIssueCount("ddb", 10)

	scenario.OpenList("ddb")

	// PITR posture rides in the Status column.
	for _, jargon := range []string{
		"CIS", " Flags", "Policy ", " Issues ",
		"NOBKP", "UNENC", " PUB ", "NOPROT",
	} {
		scenario.ExpectViewNotContains(jargon)
	}

	scenario.ExpectRowStatusBlank(demofixtures.OrdersProdID)

	scenario.ExpectRowStatusEquals(demofixtures.SessionsCreatingID, ddbPhraseCreating)
	scenario.ExpectRowStatusEquals(demofixtures.SessionsUpdatingID, ddbPhraseUpdating)
	scenario.ExpectRowStatusEquals(demofixtures.AnalyticsDeletingID, ddbPhraseDeleting)
	scenario.ExpectRowStatusEquals(demofixtures.LegacyArchivingID, ddbPhraseArchiving)
	scenario.ExpectRowStatusEquals(demofixtures.LegacyKMSLostID, ddbPhraseKMSLost)

	// The enricher sets this phrase on a Healthy row, with no suffix.
	scenario.ExpectRowStatusEquals(demofixtures.AuditPITROffID, ddbPhrasePITROff)

	// ARCHIVED + PITR disabled → multi-W2 stacking
	// expressed by the Wave-2 enricher bumping the pre-existing
	// Wave-2-phrase Status via `resource.BumpFindingSuffix`. (ddb's
	// "Wave-1" bucket is effectively the fetcher-driven TableStatus
	// mapping; the enricher treats it as the baseline.)
	scenario.ExpectRowStatusEquals(demofixtures.LegacyArchivedID, ddbPhraseArchivedPlus1)

	// colorDDB (catalog_databases.go) resolves color via colorFromAnyFinding
	// first, and ddbCodePITROff is declared Severity: SevWarn, so
	// `audit-pitr-off` renders Warning row color and carries no glyph.
	for _, id := range []string{
		demofixtures.SessionsCreatingID,
		demofixtures.SessionsUpdatingID,
		demofixtures.AnalyticsDeletingID,
		demofixtures.LegacyArchivingID,
		demofixtures.LegacyKMSLostID,
		demofixtures.LegacyArchivedID,
		demofixtures.AuditPITROffID,
	} {
		scenario.ExpectRowNoGlyphPrefix(id)
	}

	scenario.ExpectRowNoGlyphPrefix(demofixtures.OrdersProdID)

	root := selectDDBByID(t, scenario, demofixtures.OrdersProdID)
	scenario.OpenDetailResource("ddb", root)
	scenario.ExpectNoAPIError()

	for _, displayName := range []string{
		"CloudWatch Alarms",
		"Backup Plans",
		"Kinesis Streams",
		"KMS Key",
		"Lambda Functions",
		"VPC Endpoints",
	} {
		scenario.ExpectRelatedRowCountAtLeast(displayName, 1)
	}

	// Every finding stays visible in the Attention section while the Status
	// cell shows only the worst-severity phrase.
	scenario.Back()
	stacked := selectDDBByID(t, scenario, demofixtures.LegacyArchivedID)
	scenario.OpenDetailResource("ddb", stacked)
	scenario.ExpectNoAPIError()
	view := scenario.currentView()
	t.Log("\n" + view)

	scenario.ExpectViewContains("Archived: kms key lost")
	scenario.ExpectViewContains(ddbDetailPITROffCapitalize)
	scenario.ExpectViewNotContains("PITR off")

	// The PITR enricher's Summary is the short phrase; row text leaking into
	// it shows as a colon or paren after the phrase.
	scenario.ExpectViewNotContains(ddbDetailPITROffCapitalize + ":")
	scenario.ExpectViewNotContains(ddbDetailPITROffCapitalize + " (")

	scenario.Back()
	pitr := selectDDBByID(t, scenario, demofixtures.AuditPITROffID)
	scenario.OpenDetailResource("ddb", pitr)
	scenario.ExpectNoAPIError()
	scenario.ExpectViewContains(ddbDetailPITROffCapitalize)
}

// selectDDBByID looks up a concrete ddb resource from the demo clients so the
// scenario can call OpenDetailResource with a real resource value.
func selectDDBByID(t *testing.T, s *fullIntegrationScenario, id string) resource.Resource {
	t.Helper()
	return fullIntegrationMustFindResourceByID(t, s.clients, "ddb", id)
}
