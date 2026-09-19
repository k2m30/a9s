//go:build integration

package integration

// scenario_lt_visual_test.go checks the rendered TUI output (not fetcher
// return values) for lt against the universal UI rules and
// docs/resources/lt.md.
//
// The lt list API carries no health data — IMDSv1
// and explicit-off EBS encryption are describe-borne fetcher findings; the
// deprecated-AMI warning is the cache-scan enricher (zero SDK calls). Every
// issue-severity finding is color-bearing; no glyph-on-green and no
// Broken/Dim buckets exist for this type (all findings SevWarn).

import (
	"testing"

	demofixtures "github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/resource"
)

const (
	ltPhraseIMDSv1        = "IMDSv1 allowed"
	ltPhraseUnencrypted   = "EBS encryption disabled"
	ltPhraseDeprecatedAMI = "deprecated AMI"
	ltPhraseDetailsDenied = "details denied"

	// imdsv1 + unencrypted on one template.
	ltPhraseMultiP1 = "IMDSv1 allowed (+1)"

	// Detail-view Attention entries: the first letter is capitalized for display.
	ltDetailIMDSv1        = "IMDSv1 allowed"
	ltDetailUnencrypted   = "EBS encryption disabled"
	ltDetailDeprecatedAMI = "Deprecated AMI"

	// The list renders LaunchTemplateName (the Name column), not the
	// lt-0… id — row lookups go by these fixture names.
	ltNameProdWeb       = "prod-web-lt"
	ltNameEKSNode       = "eks-node-lt"
	ltNameSSMAmi        = "ssm-ami-lt"
	ltNameIMDSv1        = "warn-lt-imdsv1"
	ltNameIMDSv1Default = "warn-lt-imdsv1-def"
	ltNameUnencrypted   = "warn-lt-unencrypted"
	ltNameMulti         = "warn-lt-multi"
	ltNameDenied        = "warn-lt-denied"
	ltNameDeprecatedAMI = "warn-lt-deprecated-ami"
)

func TestScenario_LTVisual(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	scenario := fullIntegrationNewDemoScenario(t)
	runDemoStartup(t, scenario)

	// Menu badge — rows whose Wave-1-only colour IsIssue, plus Healthy
	// rows carrying a Wave-2 `!`. Over the 10 lt fixtures:
	//   Wave-1 Warning (5): lt-0warnimdsv11111a, lt-0warnimdsvdef111a,
	//                       lt-0warnunencrypt1a, lt-0warnmulti111111a,
	//                       lt-0warndenied11111a (degraded name-only row)
	//   Healthy + Wave-2 `!` (1): lt-0warnuserdata111a (lt.user-data-secret)
	//   Not counted (4): the three clean templates, and lt-0warndeprecated1a
	//                    whose deprecated-AMI check is Wave-2 `~` and never
	//                    bumps the badge
	// 5 + 1 = 6.
	scenario.ExpectMenuIssueCount("lt", 7)

	scenario.OpenList("lt")

	for _, jargon := range []string{
		"CIS", " Flags", " Issues ", "NOBKP", "UNENC", "NOPROT", "PUB ",
	} {
		scenario.ExpectViewNotContains(jargon)
	}

	// Healthy rows: blank Status — including the ssm-alias template (no
	// pivot, no finding) and the well-configured graph roots.
	for _, name := range []string{ltNameProdWeb, ltNameEKSNode, ltNameSSMAmi} {
		scenario.ExpectRowStatusBlank(name)
	}

	scenario.ExpectRowStatusEquals(ltNameIMDSv1, ltPhraseIMDSv1)
	// MetadataOptions nil defaults to optional — same finding.
	scenario.ExpectRowStatusEquals(ltNameIMDSv1Default, ltPhraseIMDSv1)
	scenario.ExpectRowStatusEquals(ltNameUnencrypted, ltPhraseUnencrypted)
	// The enricher-borne background check colours the row Warning; the `~`
	// class only keeps it out of the menu badge.
	scenario.ExpectRowStatusEquals(ltNameDeprecatedAMI, ltPhraseDeprecatedAMI)

	// Listed-but-denied template: rich degraded row — list fields kept,
	// details-denied is the only phrase.
	scenario.ExpectRowStatusEquals(ltNameDenied, ltPhraseDetailsDenied)

	scenario.ExpectRowStatusEquals(ltNameMulti, ltPhraseMultiP1)

	// Glyph rules: every finding is color-bearing — no row wears a glyph
	// (the color is the signal); healthy rows are also glyph-free.
	for _, name := range []string{
		ltNameProdWeb,
		ltNameEKSNode,
		ltNameSSMAmi,
		ltNameIMDSv1,
		ltNameIMDSv1Default,
		ltNameUnencrypted,
		ltNameDeprecatedAMI,
		ltNameMulti,
		ltNameDenied,
	} {
		scenario.ExpectRowNoGlyphPrefix(name)
	}

	root := selectLTByID(t, scenario, demofixtures.ProdWebLTID)
	scenario.OpenDetailResource("lt", root)
	scenario.ExpectNoAPIError()

	for displayName, atLeast := range map[string]int{
		"AMI":                 1,
		"Auto Scaling Groups": 2,
		"EC2 Instances":       2,
		"KMS Key":             1,
		"Security Groups":     2,
	} {
		scenario.ExpectRelatedRowCountAtLeast(displayName, atLeast)
	}

	scenario.Back()

	// The EKS node template resolves the network-interface-path pivots.
	eksRoot := selectLTByID(t, scenario, demofixtures.EKSNodeLTID)
	scenario.OpenDetailResource("lt", eksRoot)
	scenario.ExpectNoAPIError()
	scenario.ExpectRelatedRowCountAtLeast("EKS Node Groups", 1)
	scenario.ExpectRelatedRowCountAtLeast("Subnets", 1)
	scenario.ExpectRelatedRowCountAtLeast("Security Groups", 1)

	scenario.Back()

	// The multi fixture's detail lists both findings as their own Attention
	// entries.
	multi := selectLTByID(t, scenario, demofixtures.WarnLTMultiID)
	scenario.OpenDetailResource("lt", multi)
	scenario.ExpectNoAPIError()

	t.Log("\n" + scenario.currentView())

	scenario.ExpectViewContains(ltDetailIMDSv1)
	scenario.ExpectViewContains(ltDetailUnencrypted)

	scenario.Back()

	// The enricher-borne finding names the AMI in the detail view.
	deprecated := selectLTByID(t, scenario, demofixtures.WarnLTDeprecatedAMIID)
	scenario.OpenDetailResource("lt", deprecated)
	scenario.ExpectNoAPIError()
	scenario.ExpectViewContains(ltDetailDeprecatedAMI)

	scenario.AssertNoEnrichmentErrors()
}

// TestScenario_LTVisual_HealthySilence — the showroom root renders no
// Attention section and no finding phrase; default==latest is display-only
// and reference counts belong to the related panel, not a column.
func TestScenario_LTVisual_HealthySilence(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	scenario := fullIntegrationNewDemoScenario(t)
	runDemoStartup(t, scenario)
	scenario.OpenList("lt")

	res := selectLTByID(t, scenario, demofixtures.ProdWebLTID)
	scenario.OpenDetailResource("lt", res)
	scenario.ExpectNoAPIError()
	view := scenario.currentView()
	t.Log("\n" + view)

	expectNoAttentionSection(t, view)
	for _, phrase := range []string{
		ltPhraseIMDSv1, ltPhraseUnencrypted, ltPhraseDeprecatedAMI,
		ltPhraseDetailsDenied,
	} {
		scenario.ExpectViewNotContains(phrase)
	}
}

// selectLTByID looks up a concrete lt resource from the demo clients so the
// scenario can call OpenDetailResource with a real resource.
func selectLTByID(t *testing.T, s *fullIntegrationScenario, id string) resource.Resource {
	t.Helper()
	return fullIntegrationMustFindResourceByID(t, s.clients, "lt", id)
}
