//go:build integration

package integration

// scenario_efs_visual_test.go — Phase 8 render-gate for the efs resource.
//
// Verifies the rendered TUI output (not fetcher return values) matches the
// universal UI rules and the §4 contract in docs/resources/efs.md.
//
// EFS has five Wave-1 signals (creating, updating, deleting, error,
// no mount targets) and one Wave-2 signal (any mount target LifeCycleState
// != "available") whose severity is `!` (Broken). Because the sole Wave-2
// signal is Broken, a Healthy row carrying a Wave-2 finding escalates to
// Broken (rule 5) — no `!` glyph renders on Healthy because no such row
// stays green. U3/U4 are therefore N/A for EFS; U7d is also N/A (no `~`).

import (
	"strings"
	"testing"

	demofixtures "github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/resource"
)

const (
	// Fixture IDs sourced from core/demo/fixtures/efs.go so renames or
	// fixture regeneration propagate to this test automatically.
	efsWarnCreating         = demofixtures.WarnEFSCreatingID
	efsWarnUpdating         = demofixtures.WarnEFSUpdatingID
	efsWarnDeleting         = demofixtures.WarnEFSDeletingID
	efsBrokenError          = demofixtures.BrokenEFSErrorID
	efsBrokenNoMountTargets = demofixtures.BrokenEFSNoMountTargetsID
	efsWarnMulti            = demofixtures.WarnEFSMultiID
	efsWarnUpdatingMTDown   = demofixtures.WarnEFSUpdatingMTDownID
	efsHealthyMTDown        = demofixtures.HealthyEFSMTDownID

	// Wave-2 Summary phrase (must match enricher output exactly).
	efsMTDownPhrase            = "mount target down"
	efsMTDownDetailCapitalized = "Mount target down"

	// Expected S1 badge count — rows whose Wave-1-only colour IsIssue, plus
	// Healthy rows carrying a Wave-2 `!`. Recount over the 12 efs fixtures:
	//   Wave-1 Broken (3):  fs-0brokenerror00001, fs-0brokennomt000001,
	//                       fs-0warnmulti0000001
	//   Wave-1 Warning (5): fs-0warncreating0001, fs-0warnupdating0001,
	//                       fs-0warndeleting0001, fs-0warnupdmtdown001,
	//                       plus fs-0unencrypted00001, which the databases
	//                       batch added — efs.unencrypted is Wave-1 `~`
	//   Healthy + Wave-2 `!` (2): fs-0healthymtdown001 (mount-target-down)
	//                       and fs-0publicpolicy0001 (efs.public-policy),
	//                       the latter from the databases batch
	//   Not counted (2): prod-efs-app-data (clean) and fs-0nobackuppolicy01 —
	//                    its efs.no-backup-policy is Wave-2 `~`, which never
	//                    bumps the badge. That is why the batch added three
	//                    efs findings but the badge moved by two.
	// 3 + 5 + 2 = 10.
	efsExpectedIssueCount = 10
)

func TestScenario_EFSVisual(t *testing.T) {
	// Isolate from user's ~/.a9s/views — the EFS defaults were updated in
	// phase 7 but the user's home yaml may still carry the pre-refactor
	// "State" (path: LifeCycleState) column. Pointing config to an empty
	// tempdir forces config.Load to fall back to the built-in defaults
	// which the test is asserting against.
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())

	scenario := fullIntegrationNewDemoScenario(t)

	// Drive the real demo startup so Wave-2 enrichment runs end-to-end.
	runDemoStartup(t, scenario)

	// ---------------------------------------------------------------
	// S1 menu badge — assert BEFORE OpenList while main menu is current.
	// ---------------------------------------------------------------
	scenario.ExpectMenuIssueCount("efs", efsExpectedIssueCount)

	scenario.OpenList("efs")

	// ---------------------------------------------------------------
	// Universal rule U10 — no jargon columns.
	// ---------------------------------------------------------------
	for _, jargon := range []string{
		"CIS", " Flags", "Policy ", " Issues ",
		"NOBKP", "UNENC", " PUB ", "NOPROT",
	} {
		scenario.ExpectViewNotContains(jargon)
	}

	// ---------------------------------------------------------------
	// U1 — Healthy row renders blank Status (graph-root fixture).
	// ---------------------------------------------------------------
	scenario.ExpectRowStatusBlank(demofixtures.ProdEFSID)

	// ---------------------------------------------------------------
	// U2 — Warning/Broken rows render the exact §4 "List text" phrase.
	// ---------------------------------------------------------------
	scenario.ExpectRowStatusEquals(efsWarnCreating, "creating")
	scenario.ExpectRowStatusEquals(efsWarnUpdating, "updating")
	scenario.ExpectRowStatusEquals(efsWarnDeleting, "deleting")
	scenario.ExpectRowStatusEquals(efsBrokenError, "error")
	scenario.ExpectRowStatusEquals(efsBrokenNoMountTargets, "no mount targets")

	// ---------------------------------------------------------------
	// U7a — multi-W1 `(+N-1)` suffix.
	// warn-efs-multi: LifeCycleState=deleting + NumberOfMountTargets=0.
	// Precedence: Broken "no mount targets" tops, hidden Warning "deleting".
	// ---------------------------------------------------------------
	scenario.ExpectRowStatusEquals(efsWarnMulti, "no mount targets (+1)")

	// ---------------------------------------------------------------
	// U7b — W1 Warning + W2 Broken stack. The Broken one tops: domain.TopFinding
	// selects by severity for both the Status phrase and the row colour, so a
	// row coloured red by efs.mount-target-down cannot read "updating".
	// warn-efs-updating-mt-down carries Wave-1 `~` updating and Wave-2 `!`
	// mount target down → "mount target down (+1)".
	// ---------------------------------------------------------------
	scenario.ExpectRowStatusEquals(efsWarnUpdatingMTDown, "mount target down (+1)")

	// ---------------------------------------------------------------
	// Wave-2 on Healthy escalates to Broken (no suffix, single finding).
	// ---------------------------------------------------------------
	scenario.ExpectRowStatusEquals(efsHealthyMTDown, efsMTDownPhrase)

	// ---------------------------------------------------------------
	// U5 — no `!` / `~` glyph on ANY non-green row. EFS has no `~` and
	// its only `!` source escalates the row to Broken, so every fixture
	// carrying a finding is non-green by the time the list renders.
	// ---------------------------------------------------------------
	for _, id := range []string{
		efsWarnCreating, efsWarnUpdating, efsWarnDeleting,
		efsBrokenError, efsBrokenNoMountTargets,
		efsWarnMulti, efsWarnUpdatingMTDown, efsHealthyMTDown,
	} {
		scenario.ExpectRowNoGlyphPrefix(id)
	}

	// Healthy graph root must also render no glyph.
	scenario.ExpectRowNoGlyphPrefix(demofixtures.ProdEFSID)

	// ---------------------------------------------------------------
	// U9 — Related panel on graph-root: every `count shown: yes` pivot
	// resolves to ≥ 1 AND at least 50% to ≥ 2. `ec2` intentionally 0
	// per spec §5; `ct-events` is the universal windowed pivot and
	// exempt (count shown: unknown).
	// ---------------------------------------------------------------
	root := selectEFSByID(t, scenario, demofixtures.ProdEFSID)
	scenario.OpenDetailResource("efs", root)
	scenario.ExpectNoAPIError()

	// Pivots expected with Count ≥ 2 (≥ 50% of the 10 `count shown: yes` pivots).
	for _, displayName := range []string{
		"Security Groups",    // 2
		"Subnets",            // 3
		"Lambda Functions",   // 2
		"CloudWatch Alarms",  // 2
		"Backup Plans",       // 2
		"ECS Tasks",          // 2
		"Network Interfaces", // 3
	} {
		scenario.ExpectRelatedRowCountAtLeast(displayName, 2)
	}
	// Pivots where Count = 1 is spec-accurate (VPC, KMS, CFN typically one-each).
	for _, displayName := range []string{
		"KMS Keys",
		"CloudFormation Stacks",
		"VPC",
	} {
		scenario.ExpectRelatedRowCountAtLeast(displayName, 1)
	}

	// ---------------------------------------------------------------
	// U7e + U7f render gate — multi-W1 fixture detail enumerates every
	// phrase in Resource.Issues. The `(+N)` suffix itself MUST NOT
	// appear in the detail — the detail expands each phrase on its own
	// line.
	// ---------------------------------------------------------------
	scenario.Back()
	multi := selectEFSByID(t, scenario, efsWarnMulti)
	scenario.OpenDetailResource("efs", multi)
	scenario.ExpectNoAPIError()
	scenario.ExpectViewContains("No mount targets")
	scenario.ExpectViewContains("Deleting")

	// ---------------------------------------------------------------
	// U7c — W1+W2 stacking fixture detail lists both the Wave-2 Summary
	// and the Wave-1 phrase. No finding silently disappears.
	// ---------------------------------------------------------------
	scenario.Back()
	stack := selectEFSByID(t, scenario, efsWarnUpdatingMTDown)
	scenario.OpenDetailResource("efs", stack)
	scenario.ExpectNoAPIError()
	view := scenario.currentView()
	t.Log("\n" + view) // Phase 8.4 user-visible sanity render (mandatory)

	scenario.ExpectViewContains(efsMTDownDetailCapitalized)
	scenario.ExpectViewContains("Updating") // Wave-1 phrase survives in Attention
	// Wave-2 Rows contract — structured facts, not embedded in Summary.
	scenario.ExpectViewContains("Mount Target") // Row label
	scenario.ExpectViewContains("Degraded")     // Row label for N/M counter
	scenario.ExpectViewContains("1/2")          // Row value — 1 of 2 MTs down

	// U11 regression pin — Summary must render as exact phrase, not a concatenation.
	// The rendered frame must contain "Mount target down" on its own, not
	// something like "mount target down: fsmt-...-B in us-east-1b". The detail
	// renderer capitalizes the first letter so check both the raw phrase and
	// the capitalized form — a concatenation bug could leak either.
	for _, phrase := range []string{efsMTDownPhrase, efsMTDownDetailCapitalized} {
		if strings.Contains(view, phrase+": ") {
			t.Errorf("detail contains Summary concatenated with Row values (U11 violation): %q substring found", phrase+": ")
		}
	}

	// U16 — no wave-2 enrichment errors leaked through during the scenario.
	scenario.AssertNoEnrichmentErrors()
}

// selectEFSByID looks up a concrete efs Resource from the demo clients so the
// scenario can call OpenDetailResource with a real resource value.
func selectEFSByID(t *testing.T, s *fullIntegrationScenario, id string) resource.Resource {
	t.Helper()
	return fullIntegrationMustFindResourceByID(t, s.clients, "efs", id)
}
