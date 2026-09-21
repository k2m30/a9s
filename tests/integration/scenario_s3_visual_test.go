//go:build integration

package integration

// scenario_s3_visual_test.go checks the rendered TUI output (not fetcher
// return values) for s3 against the universal UI rules and
// docs/resources/s3.md.
//
// s3 has no Wave-1 signals. colorS3 (catalog_databases.go) resolves color
// via colorFromAnyFinding, so a public-access-block finding renders its
// bucket row Warn directly and no s3 row carries a glyph.

import (
	"testing"

	demofixtures "github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/resource"
)

const (
	// Bucket IDs for the 4 PAB-finding fixtures. The no-PAB bucket is exported
	// because it also carries the access-control-list route into s3.public.
	s3NoPABBucketID  = demofixtures.S3BucketPublicByACL
	s3PartialPABID   = "a9s-demo-partial-pab"
	s3MultiFailPABID = "a9s-demo-multifail-pab"
	s3NilCfgPABID    = "a9s-demo-nilcfg"
	// The badge counts rows whose Wave-1-only colour IsIssue, plus
	// Healthy rows carrying a Wave-2 `!`. s3 has no Wave-1 signals at all, so
	// only the second clause can fire. Two of the 42 bucket fixtures carry a
	// `!`, one for each route into s3.public:
	// acme-public-datasets, whose bucket policy status is public, and
	// a9s-demo-nopab, whose access control list grants AllUsers read with no
	// public access block to disregard it.
	// Every other s3 finding — the three remaining PAB fixtures and the
	// versioning, MFA-delete, access-logging, lifecycle and object-lock
	// fixtures — is Wave-2 `~` and never bumps the badge.
	s3ExpectedIssueBkt = 2

	// Wave-2 Rows row labels/values emitted by EnrichS3Posture.
	s3Row_BlockPublicAcls    = "BlockPublicAcls"
	s3Row_BlockPublicPolicy  = "BlockPublicPolicy"
	s3Row_AccountLevelLabel  = "Account-level PAB"
	s3Row_AccountLevelValue  = "may still apply"
	s3Row_NoPABStatusValue   = "no public access block configuration"
	s3S4Phrase               = "public access block incomplete"
	s3DetailPhraseCapitalize = "Public access block incomplete"

	// s3ACLPublicStatus is the no-PAB bucket's Status cell. That bucket's
	// access control list also routes it into s3.public, so it carries two
	// findings: the broken-tier
	// phrase takes the cell and "(+1)" stands for the warn-tier public
	// access block finding beside it. The other three PAB fixtures carry
	// one finding each and still render s3S4Phrase alone.
	s3ACLPublicStatus = "publicly accessible (+1)"
)

func TestScenario_S3Visual(t *testing.T) {
	scenario := fullIntegrationNewDemoScenario(t)

	// Drive the real demo startup so Wave 2 enrichment runs against the fake.
	runDemoStartup(t, scenario)

	// The menu badge is read while the main menu is still the current view.
	scenario.ExpectMenuIssueCount("s3", s3ExpectedIssueBkt)

	scenario.OpenList("s3")

	for _, jargon := range []string{
		"Public Access", "CIS", " Flags", "Policy ", " Issues ",
		"NOBKP", "UNENC", " PUB ", "NOPROT",
	} {
		scenario.ExpectViewNotContains(jargon)
	}

	scenario.ExpectRowStatusBlank(demofixtures.HealthyBucketName)

	// The no-PAB bucket leads with its broken-tier finding and counts the PAB
	// one in its suffix.
	scenario.ExpectRowStatusEquals(s3NoPABBucketID, s3ACLPublicStatus)
	scenario.ExpectRowStatusEquals(s3PartialPABID, s3S4Phrase)
	scenario.ExpectRowStatusEquals(s3MultiFailPABID, s3S4Phrase)
	scenario.ExpectRowStatusEquals(s3NilCfgPABID, s3S4Phrase)

	for _, id := range []string{s3NoPABBucketID, s3PartialPABID, s3MultiFailPABID, s3NilCfgPABID} {
		scenario.ExpectRowNoGlyphPrefix(id)
	}

	scenario.ExpectRowNoGlyphPrefix(demofixtures.HealthyBucketName)

	// A registered pivot that always returns 0 is a defect.
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
		"Backup Plans",
		"EventBridge Rules",
		"Route 53",
		"IAM Roles",
	} {
		scenario.ExpectRelatedRowCountAtLeast(displayName, 1)
	}

	// IAM Users and WAF are not s3 pivots.
	scenario.ExpectViewNotContains("IAM Users")
	scenario.ExpectViewNotContains("WAF")

	// An operator investigating an issue bucket needs something to drill
	// into, so its related panel resolves non-zero counts too.
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

	// The multi-false-pab fixture has BlockPublicAcls=false AND
	// BlockPublicPolicy=false; both rows render in the detail view.
	scenario.Back()
	multiFail := selectS3ByID(t, scenario, s3MultiFailPABID)
	scenario.OpenDetailResource("s3", multiFail)
	scenario.ExpectNoAPIError()
	view := scenario.currentView()
	t.Log("\n" + view)

	scenario.ExpectViewContains(s3DetailPhraseCapitalize)
	scenario.ExpectViewContains(s3Row_BlockPublicAcls)
	scenario.ExpectViewContains(s3Row_BlockPublicPolicy)
	scenario.ExpectViewContains(s3Row_AccountLevelLabel)
	scenario.ExpectViewContains(s3Row_AccountLevelValue)

	// The Summary carries no Row value.
	scenario.ExpectViewNotContains("public access block incomplete: ")
	scenario.ExpectViewNotContains(s3S4Phrase + " (")

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
