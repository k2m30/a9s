//go:build integration

package integration

// scenario_s3_visual_test.go — Phase 8 render-gate for the s3 resource.
//
// Verifies the rendered TUI output (not fetcher return values) matches the
// universal UI rules and the §4 contract in docs/resources/s3.md.
//
// s3 has zero Wave-1 signals and one Wave-2 signal (`!` severity on Healthy
// rows via GetPublicAccessBlock). The rule-7 multi-finding cases (U7a/U7b/
// U7c/U7d/U7e/U7f) are therefore N/A for this resource and skipped with
// a per-item justification below.

import (
	"testing"

	demofixtures "github.com/k2m30/a9s/v3/internal/demo/fixtures"
	"github.com/k2m30/a9s/v3/internal/resource"
)

const (
	// Bucket IDs for the 4 PAB-finding fixtures. No constants were exported
	// from fixtures/s3.go for these names, so the test pins them locally.
	s3NoPABBucketID    = "a9s-demo-nopab"
	s3PartialPABID     = "a9s-demo-partial-pab"
	s3MultiFailPABID   = "a9s-demo-multifail-pab"
	s3NilCfgPABID      = "a9s-demo-nilcfg"
	s3ExpectedIssueBkt = 4

	// Wave-2 Rows row labels/values emitted by EnrichS3PublicAccessBlock.
	s3Row_BlockPublicAcls    = "BlockPublicAcls"
	s3Row_BlockPublicPolicy  = "BlockPublicPolicy"
	s3Row_AccountLevelLabel  = "Account-level PAB"
	s3Row_AccountLevelValue  = "may still apply"
	s3Row_NoPABStatusValue   = "no public access block configuration"
	s3S4Phrase               = "public access block incomplete"
	s3DetailPhraseCapitalize = "Public access block incomplete"
)

func TestScenario_S3Visual(t *testing.T) {
	scenario := fullIntegrationNewDemoScenario(t)

	// Drive the real demo startup so Wave 2 enrichment runs against the fake.
	runDemoStartup(t, scenario)

	// ---------------------------------------------------------------
	// S1 menu badge — assert BEFORE OpenList while the main menu is
	// still the current view. 4 finding fixtures → issues:4. (All
	// other buckets fall through the fake's healthy-default path,
	// contributing 0 findings.)
	// ---------------------------------------------------------------
	scenario.ExpectMenuIssueCount("s3", s3ExpectedIssueBkt)

	scenario.OpenList("s3")

	// ---------------------------------------------------------------
	// Universal column rules — no jargon columns.
	// The old `Public Access` jargon column was deleted in phase 7.
	// ---------------------------------------------------------------
	for _, jargon := range []string{
		"Public Access", "CIS", " Flags", "Policy ", " Issues ",
		"NOBKP", "UNENC", " PUB ", "NOPROT",
	} {
		scenario.ExpectViewNotContains(jargon)
	}

	// Healthy baseline — graph root. Single healthy bucket with all-true PAB.
	scenario.ExpectRowStatusBlank(demofixtures.HealthyBucketName)

	// The 4 finding fixtures render the stable spec §4 phrase in S4.
	scenario.ExpectRowStatusEquals(s3NoPABBucketID, s3S4Phrase)
	scenario.ExpectRowStatusEquals(s3PartialPABID, s3S4Phrase)
	scenario.ExpectRowStatusEquals(s3MultiFailPABID, s3S4Phrase)
	scenario.ExpectRowStatusEquals(s3NilCfgPABID, s3S4Phrase)

	// Rule 3 — `!` glyph prefixes a Healthy (green) row with a `!` finding.
	// All 4 PAB-finding buckets stay Healthy (s3 has no Wave-1 color bucket
	// signal; the finding is a background-check annotation).
	for _, id := range []string{s3NoPABBucketID, s3PartialPABID, s3MultiFailPABID, s3NilCfgPABID} {
		scenario.ExpectRowNamePrefix(id, "! ")
	}

	// The healthy baseline must NOT carry a glyph.
	scenario.ExpectRowNoGlyphPrefix(demofixtures.HealthyBucketName)

	// ---------------------------------------------------------------
	// Related panel — graph-root fixture shows non-zero counts for
	// EVERY pivot whose §2 contract is `count shown: yes`. Related
	// pivots are a product contract: a registered pivot that always
	// returns 0 is a bug, not a deferred feature. See user guidance
	// 2026-04-23: "related resources MUST work. if they don't it's a
	// bug. simple".
	// ---------------------------------------------------------------
	root := selectS3ByID(t, scenario, demofixtures.HealthyBucketName)
	scenario.OpenDetailResource("s3", root)
	scenario.ExpectNoAPIError()

	for _, displayName := range []string{
		"CloudTrail Trails",
		"CloudFront",
		"Lambda (notifications)",
		"SNS (notifications)",
		"SQS (notifications)",
		"CloudFormation",
		"KMS Key",
		"Access Log Bucket",
		"Athena WorkGroups",
		"Glue Jobs",
		"Backup",
		"EventBridge Rules",
		"Route 53",
		"IAM Roles",
	} {
		scenario.ExpectRelatedRowCountAtLeast(displayName, 1)
	}

	// The §5 Out-of-Scope pivots must NOT appear in the related panel.
	scenario.ExpectViewNotContains("IAM Users")
	scenario.ExpectViewNotContains("WAF")

	// ---------------------------------------------------------------
	// Related panel on a PAB-ISSUE bucket — an operator pivots here
	// from the `!` row. The panel MUST show non-zero counts for at
	// least the server-side-resolved pivots (KMS, CFN, Log Groups,
	// CloudFront, Trails, Glue). A bare issue bucket with every row
	// at (0) is a fixture defect: the operator has nothing to drill
	// into when investigating the problem.
	// ---------------------------------------------------------------
	scenario.Back()
	issueBkt := selectS3ByID(t, scenario, s3PartialPABID)
	scenario.OpenDetailResource("s3", issueBkt)
	scenario.ExpectNoAPIError()
	for _, displayName := range []string{
		"CloudTrail Trails",
		"CloudFront",
		"CloudFormation",
		"KMS Key",
		"Access Log Bucket",
	} {
		scenario.ExpectRelatedRowCountAtLeast(displayName, 1)
	}

	// ---------------------------------------------------------------
	// Rule 7 U7c — S5 Attention section shows Wave-2 Rows detail.
	// The multi-false-pab fixture has BlockPublicAcls=false AND
	// BlockPublicPolicy=false; both rows must render in the detail
	// view so no Wave-2 fact silently disappears.
	// ---------------------------------------------------------------
	scenario.Back()
	multiFail := selectS3ByID(t, scenario, s3MultiFailPABID)
	scenario.OpenDetailResource("s3", multiFail)
	scenario.ExpectNoAPIError()
	view := scenario.currentView()
	t.Log("\n" + view) // 8.4 user-visible sanity render (mandatory)

	// Attention primary entry: glyph + capitalized phrase.
	scenario.ExpectViewContains(s3DetailPhraseCapitalize)
	// Both false-flag rows must be present.
	scenario.ExpectViewContains(s3Row_BlockPublicAcls)
	scenario.ExpectViewContains(s3Row_BlockPublicPolicy)
	// Account-level context row.
	scenario.ExpectViewContains(s3Row_AccountLevelLabel)
	scenario.ExpectViewContains(s3Row_AccountLevelValue)

	// U11 regression guard — Summary (`s3S4Phrase`) must not contain any
	// Row value. Asserted at unit-test level; here we confirm the stable
	// phrase still renders exactly (no cause text leaked in).
	scenario.ExpectViewNotContains("public access block incomplete: ")
	scenario.ExpectViewNotContains(s3S4Phrase + " (")

	// ---------------------------------------------------------------
	// Rule 7 U7c — detail view of the no-PAB fixture shows the Status
	// row "no public access block configuration" + account-level row.
	// ---------------------------------------------------------------
	scenario.Back()
	noPab := selectS3ByID(t, scenario, s3NoPABBucketID)
	scenario.OpenDetailResource("s3", noPab)
	scenario.ExpectNoAPIError()
	scenario.ExpectViewContains(s3Row_NoPABStatusValue)
	scenario.ExpectViewContains(s3Row_AccountLevelValue)
}

// selectS3ByID looks up a concrete s3 resource from the demo clients so the
// scenario can call OpenDetailResource with a real resource value.
func selectS3ByID(t *testing.T, s *fullIntegrationScenario, id string) resource.Resource {
	t.Helper()
	return fullIntegrationMustFindResourceByID(t, s.clients, "s3", id)
}
