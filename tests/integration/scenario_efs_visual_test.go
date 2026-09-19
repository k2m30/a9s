//go:build integration

package integration

// scenario_efs_visual_test.go checks the rendered TUI output (not fetcher
// return values) for efs against the universal UI rules and
// docs/resources/efs.md.
//
// The mount-target-down Wave-2 signal (any mount target LifeCycleState
// != "available") is Broken, so a Healthy row carrying it escalates to
// Broken and never renders a `!` glyph on green.

import (
	"strings"
	"testing"

	demofixtures "github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/resource"
)

const (
	efsWarnCreating         = demofixtures.WarnEFSCreatingID
	efsWarnUpdating         = demofixtures.WarnEFSUpdatingID
	efsWarnDeleting         = demofixtures.WarnEFSDeletingID
	efsBrokenError          = demofixtures.BrokenEFSErrorID
	efsBrokenNoMountTargets = demofixtures.BrokenEFSNoMountTargetsID
	efsWarnMulti            = demofixtures.WarnEFSMultiID
	efsWarnUpdatingMTDown   = demofixtures.WarnEFSUpdatingMTDownID
	efsHealthyMTDown        = demofixtures.HealthyEFSMTDownID

	efsMTDownPhrase            = "mount target down"
	efsMTDownDetailCapitalized = "Mount target down"

	// Menu badge count — rows whose Wave-1-only colour IsIssue, plus
	// Healthy rows carrying a Wave-2 `!`. Over the 12 efs fixtures:
	//   Wave-1 Broken (3):  fs-0brokenerror00001, fs-0brokennomt000001,
	//                       fs-0warnmulti0000001
	//   Wave-1 Warning (5): fs-0warncreating0001, fs-0warnupdating0001,
	//                       fs-0warndeleting0001, fs-0warnupdmtdown001,
	//                       fs-0unencrypted00001 (efs.unencrypted is Wave-1 `~`)
	//   Healthy + Wave-2 `!` (2): fs-0healthymtdown001 (mount-target-down)
	//                       and fs-0publicpolicy0001 (efs.public-policy)
	//   Not counted (2): prod-efs-app-data (clean) and fs-0nobackuppolicy01 —
	//                    its efs.no-backup-policy is Wave-2 `~`, which never
	//                    bumps the badge.
	// 3 + 5 + 2 = 10.
	efsExpectedIssueCount = 10
)

func TestScenario_EFSVisual(t *testing.T) {
	// A user-dir ~/.a9s/views overlay wins the merge, so config.Load points at
	// an empty tempdir and only the built-in defaults apply.
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())

	scenario := fullIntegrationNewDemoScenario(t)

	// Drive the real demo startup so Wave-2 enrichment runs end-to-end.
	runDemoStartup(t, scenario)

	scenario.ExpectMenuIssueCount("efs", efsExpectedIssueCount)

	scenario.OpenList("efs")

	for _, jargon := range []string{
		"CIS", " Flags", "Policy ", " Issues ",
		"NOBKP", "UNENC", " PUB ", "NOPROT",
	} {
		scenario.ExpectViewNotContains(jargon)
	}

	scenario.ExpectRowStatusBlank(demofixtures.ProdEFSID)

	scenario.ExpectRowStatusEquals(efsWarnCreating, "creating")
	scenario.ExpectRowStatusEquals(efsWarnUpdating, "updating")
	scenario.ExpectRowStatusEquals(efsWarnDeleting, "deleting")
	scenario.ExpectRowStatusEquals(efsBrokenError, "error")
	scenario.ExpectRowStatusEquals(efsBrokenNoMountTargets, "no mount targets")

	// warn-efs-multi: LifeCycleState=deleting + NumberOfMountTargets=0.
	// Broken "no mount targets" tops, hidden Warning "deleting".
	scenario.ExpectRowStatusEquals(efsWarnMulti, "no mount targets (+1)")

	// W1 Warning + W2 Broken stack. The Broken one tops: domain.TopFinding
	// selects by severity for both the Status phrase and the row colour, so a
	// row coloured red by efs.mount-target-down cannot read "updating".
	// warn-efs-updating-mt-down carries Wave-1 `~` updating and Wave-2 `!`
	// mount target down.
	scenario.ExpectRowStatusEquals(efsWarnUpdatingMTDown, "mount target down (+1)")

	// Wave-2 on Healthy escalates to Broken.
	scenario.ExpectRowStatusEquals(efsHealthyMTDown, efsMTDownPhrase)

	// Every fixture carrying a finding is non-green by the time the list
	// renders, so none carries a glyph.
	for _, id := range []string{
		efsWarnCreating, efsWarnUpdating, efsWarnDeleting,
		efsBrokenError, efsBrokenNoMountTargets,
		efsWarnMulti, efsWarnUpdatingMTDown, efsHealthyMTDown,
	} {
		scenario.ExpectRowNoGlyphPrefix(id)
	}

	scenario.ExpectRowNoGlyphPrefix(demofixtures.ProdEFSID)

	// `ct-events` is the windowed pivot and shows no count.
	root := selectEFSByID(t, scenario, demofixtures.ProdEFSID)
	scenario.OpenDetailResource("efs", root)
	scenario.ExpectNoAPIError()

	for _, displayName := range []string{
		"Security Groups",
		"Subnets",
		"Lambda Functions",
		"CloudWatch Alarms",
		"Backup Plans",
		"ECS Tasks",
		"Network Interfaces",
	} {
		scenario.ExpectRelatedRowCountAtLeast(displayName, 2)
	}
	// A file system has one VPC, one KMS key and one CFN stack.
	for _, displayName := range []string{
		"KMS Keys",
		"CloudFormation Stacks",
		"VPC",
	} {
		scenario.ExpectRelatedRowCountAtLeast(displayName, 1)
	}

	// The detail expands each phrase in Resource.Issues on its own line.
	scenario.Back()
	multi := selectEFSByID(t, scenario, efsWarnMulti)
	scenario.OpenDetailResource("efs", multi)
	scenario.ExpectNoAPIError()
	scenario.ExpectViewContains("No mount targets")
	scenario.ExpectViewContains("Deleting")

	scenario.Back()
	stack := selectEFSByID(t, scenario, efsWarnUpdatingMTDown)
	scenario.OpenDetailResource("efs", stack)
	scenario.ExpectNoAPIError()
	view := scenario.currentView()
	t.Log("\n" + view)

	scenario.ExpectViewContains(efsMTDownDetailCapitalized)
	scenario.ExpectViewContains("Updating")
	// Wave-2 Rows carry structured facts, not embedded in Summary.
	scenario.ExpectViewContains("Mount Target")
	scenario.ExpectViewContains("Degraded")
	scenario.ExpectViewContains("1/2")

	// The rendered frame must contain "Mount target down" on its own, not
	// something like "mount target down: fsmt-...-B in us-east-1b". The detail
	// renderer capitalizes the first letter so check both the raw phrase and
	// the capitalized form — a concatenation bug could leak either.
	for _, phrase := range []string{efsMTDownPhrase, efsMTDownDetailCapitalized} {
		if strings.Contains(view, phrase+": ") {
			t.Errorf("detail contains Summary concatenated with Row values (U11 violation): %q substring found", phrase+": ")
		}
	}

	scenario.AssertNoEnrichmentErrors()
}

// selectEFSByID looks up a concrete efs Resource from the demo clients so the
// scenario can call OpenDetailResource with a real resource value.
func selectEFSByID(t *testing.T, s *fullIntegrationScenario, id string) resource.Resource {
	t.Helper()
	return fullIntegrationMustFindResourceByID(t, s.clients, "efs", id)
}
