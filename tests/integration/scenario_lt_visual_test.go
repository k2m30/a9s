//go:build integration

package integration

// scenario_lt_visual_test.go — Phase 8 render-gate for the lt resource.
// Verifies the rendered TUI output (not fetcher return values) matches the
// universal UI rules and the §4 contract in docs/resources/lt.md.
//
// lt has NO Wave-1 signals (the list API carries no health data) — IMDSv1
// and explicit-off EBS encryption are describe-borne fetcher findings; the
// deprecated-AMI warning is the cache-scan enricher (zero SDK calls). Every
// issue-severity finding is color-bearing; no glyph-on-green and no
// Broken/Dim buckets exist for this type (all findings SevWarn).

import (
	"testing"

	demofixtures "github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/resource"
)

// §4 phrases pinned locally — any drift in the fetcher/enricher surfaces
// here instead of in unit tests that could be rewritten without noticing.
const (
	ltPhraseIMDSv1        = "IMDSv1 allowed"
	ltPhraseUnencrypted   = "EBS encryption disabled"
	ltPhraseDeprecatedAMI = "deprecated AMI"
	ltPhraseDetailsDenied = "details denied"

	// Rule-7 rolled-up form: imdsv1 + unencrypted on one template.
	ltPhraseMultiP1 = "IMDSv1 allowed (+1)"

	// Detail-view Attention entries (capitalizeFirst applied at render).
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

	// S1 menu badge — rows whose Wave-1-only colour IsIssue, plus Healthy
	// rows carrying a Wave-2 `!`. Recount over the 10 lt fixtures:
	//   Wave-1 Warning (5): lt-0warnimdsv11111a, lt-0warnimdsvdef111a,
	//                       lt-0warnunencrypt1a, lt-0warnmulti111111a,
	//                       lt-0warndenied11111a (degraded name-only row)
	//   Healthy + Wave-2 `!` (1): lt-0warnuserdata111a, which the compute
	//                       batch added — lt.user-data-secret is `!`
	//   Not counted (4): the three clean templates, and lt-0warndeprecated1a
	//                    whose deprecated-AMI check is Wave-2 `~` and
	//                    deliberately never bumps the badge
	// 5 + 1 = 6.
	scenario.ExpectMenuIssueCount("lt", 7)

	scenario.OpenList("lt")

	// Universal column rules — no jargon columns.
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

	// §4 phrases.
	scenario.ExpectRowStatusEquals(ltNameIMDSv1, ltPhraseIMDSv1)
	// The trap witness: MetadataOptions nil defaults to optional — same finding.
	scenario.ExpectRowStatusEquals(ltNameIMDSv1Default, ltPhraseIMDSv1)
	scenario.ExpectRowStatusEquals(ltNameUnencrypted, ltPhraseUnencrypted)
	// The enricher-borne background check: Warning-colored row + phrase
	// (owner ruling 2026-07-15 — color derives from findings uniformly;
	// the `~` class only keeps it out of the S1 badge).
	scenario.ExpectRowStatusEquals(ltNameDeprecatedAMI, ltPhraseDeprecatedAMI)

	// Listed-but-denied template: rich degraded row — list fields kept,
	// details-denied is the only phrase.
	scenario.ExpectRowStatusEquals(ltNameDenied, ltPhraseDetailsDenied)

	// Rule 7 — two findings stack: first-in-precedence phrase + (+1).
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

	// Related panel — prod-web-lt graph root: ami 1, asg 2, ec2 2, kms 1,
	// sg 2 (≥2 on 3 of 5 countable pivots — the 60% that beats the 50% gate).
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

	// Second graph root — the NI-path witnesses: ng 1, subnet 1, sg 1.
	eksRoot := selectLTByID(t, scenario, demofixtures.EKSNodeLTID)
	scenario.OpenDetailResource("lt", eksRoot)
	scenario.ExpectNoAPIError()
	scenario.ExpectRelatedRowCountAtLeast("EKS Node Groups", 1)
	scenario.ExpectRelatedRowCountAtLeast("Subnets", 1)
	scenario.ExpectRelatedRowCountAtLeast("Security Groups", 1)

	scenario.Back()

	// Rule 7 U7c/U7e — the multi fixture's detail enumerates BOTH findings
	// as their own capitalized Attention entries.
	multi := selectLTByID(t, scenario, demofixtures.WarnLTMultiID)
	scenario.OpenDetailResource("lt", multi)
	scenario.ExpectNoAPIError()

	// 8.4 user-visible sanity render (mandatory).
	t.Log("\n" + scenario.currentView())

	scenario.ExpectViewContains(ltDetailIMDSv1)
	scenario.ExpectViewContains(ltDetailUnencrypted)

	scenario.Back()

	// The enricher-borne finding surfaces in S5 with the AMI named.
	deprecated := selectLTByID(t, scenario, demofixtures.WarnLTDeprecatedAMIID)
	scenario.OpenDetailResource("lt", deprecated)
	scenario.ExpectNoAPIError()
	scenario.ExpectViewContains(ltDetailDeprecatedAMI)

	scenario.AssertNoEnrichmentErrors()
}

// TestScenario_LTVisual_HealthySilence — spec §4 "Healthy silence": the
// showroom root renders no Attention section and no finding phrase, and the
// wave-3 anti-tests hold (default==latest is display-only; reference counts
// are the related panel's job, not a column).
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
