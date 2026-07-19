package unit

// qa_enrichment_review_fixes_test.go — regression test for a review finding
// landed on top of feature 018-enrichment-visibility: Ctrl+R on a top-level
// list must NOT clear the active ResourceListModel's findings immediately —
// they stay applied (stale-until-replaced) until the rerun's fresh
// EnrichmentChecked result actually lands, avoiding a user-visible flicker
// across the full AWS round-trip. See the corrected test below for both
// halves: no blank window before the fresh result lands, and real removal
// once it genuinely omits the finding.
//
// A second review finding this file used to pin (resolveIdentityColumn
// running on the full pre-hscroll column list, so the marker doesn't jump to
// a different semantic column when scrolled) is now covered by
// wave3_list_ports_test.go's TestWave3MarkerColParity_
// EnrichmentFindingsWithHScroll_AllResourceTypes, which pins the live
// RenderList seam (this file's ResourceListModel.View() harness is dead
// code).

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
)

// -----------------------------------------------------------------------------
// Ctrl+R keeps findings visible until the fresh result lands, rather than
// blanking them at keypress time
// -----------------------------------------------------------------------------

// TestCtrlR_RetainsActiveListFindingsUntilFreshEnrichment pins the corrected
// contract: pressing Ctrl+R on a top-level list must NOT blank the active
// ResourceListModel's findings immediately, before the wrapped fetch even
// returns. Eagerly blanking findings at keypress time produced a real,
// user-visible flicker for the full AWS round-trip between the keypress and
// the rerun's EnrichmentChecked arrival — findings must be stale-until-
// replaced, not blank-until-replaced (see qa_glyph_continuity_test.go's
// TestRerunStart_KeepsVisibleFindingsUntilReplaced for a second pin of the
// same contract, asserting BOTH halves: no blank window before the fresh
// result lands, and real removal once the fresh result genuinely omits the
// finding).
//
// Uses the test helpers defined in qa_enrichment_rerun_overlap_test.go:
// newRootSizedModel, rootApplyMsg, navigateToEC2List, ctrlRKeyMsg.
// isVisibleUnderCtrlZ toggles the ctrl+z attention filter on m, checks
// whether needle is visible, then toggles it back off (returning the
// original model so callers can keep making assertions without ctrl+z
// bleeding into later checks). Since the color-findings-conformance wave,
// colorEC2 is colorFromAnyFinding-only (core/aws/catalog_compute.go) —
// once a SevBroken/SevWarn Finding is applied, resolveListDecoratorFull's
// "! "/"~ " glyph prefix branch is skipped entirely (it only fires when
// ResolveColor()==ColorHealthy; see core/app/list_columns.go and
// .claude/agent-memory/a9s-coder/project_color_findings_conformance_glyph_interplay.md).
// The renderer-agnostic, stronger check for "is this row an applied issue"
// is ctrl+z survival, not the literal glyph text.
func isVisibleUnderCtrlZ(m tui.Model, needle string) (tui.Model, bool) {
	m, _ = rootApplyMsg(m, ctrlZ())
	visible := strings.Contains(stripANSI(m.View().Content), needle)
	m, _ = rootApplyMsg(m, ctrlZ())
	return m, visible
}

func TestCtrlR_RetainsActiveListFindingsUntilFreshEnrichment(t *testing.T) {
	tui.Version = "test"
	m := newRootSizedModel()
	m = navigateToEC2List(m)

	// Load real EC2 resources into the list so there's something to mark.
	resources := rerunEC2Resources()
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "ec2",
		Resources:    resources,
	})

	// Deliver a successful EnrichmentCheckedMsg to populate findings on the
	// active list via the handler's live-update path.
	m, _ = rootApplyMsg(m, enrichmentCheckedWithFindings(0, 0))

	// Sanity: the impaired instance survives the ctrl+z attention filter
	// (it is now an applied issue row).
	var visible bool
	m, visible = isVisibleUnderCtrlZ(m, "web-server-1")
	if !visible {
		t.Fatal("pre-condition failed: expected web-server-1 to survive ctrl+z (finding applied) before Ctrl+R")
	}

	// Dispatch Ctrl+R via the real key path. "ec2" has a registered Wave-2
	// issue enricher (EnrichEC2InstanceStatus, core/aws/catalog_compute.go),
	// so handleRefresh's rsKindList branch calls
	// m.core.BumpEnrichmentTypeGen("ec2") — EnrichmentTypeGen["ec2"] goes
	// from 0 (set by the enrichmentCheckedWithFindings(0,0) probe above) to
	// 1. The real rerun's eventual EnrichmentChecked carries that same
	// bumped value (refreshActiveListWithEnrichmentRerun stamps tok onto the
	// wrapped ResourcesLoaded, and handlers_resources.go's TypeGen-match
	// branch is what fires the next probeEnrichment with it) — see
	// core/runtime/handlers_availability.go's per-type generation guard
	// (`msg.TypeGen != 0 && msg.TypeGen != EnrichmentTypeGen[rt]`).
	m, _ = rootApplyMsg(m, ctrlRKeyMsg())

	// Assertion (corrected): the finding must still be applied immediately
	// after Ctrl+R — no fetch/enrichment response has landed yet, so the old
	// finding is still the best-known truth (stale-until-replaced).
	m, visible = isVisibleUnderCtrlZ(m, "web-server-1")
	if !visible {
		t.Error("Ctrl+R must NOT blank the active list's findings before the fresh result lands; web-server-1 no longer survives ctrl+z immediately after keypress")
	}

	// Once a fresh EnrichmentCheckedMsg lands with the generation the real
	// rerun actually produced (TypeGen=1, matching the bump above) and
	// genuinely omits the finding (resource recovered), the finding MUST be
	// removed. TypeGen=0 here would be a false positive: the per-type
	// generation guard treats TypeGen=0 as "not a tracked rerun" and accepts
	// it unconditionally regardless of the real EnrichmentTypeGen value, so
	// it would prove nothing about generation-matched replacement.
	recovered := messages.EnrichmentChecked{
		ResourceType: "ec2",
		Truncated:    false,
		Findings:     map[string][]domain.Finding{},
		Gen:          0,
		TypeGen:      1,
	}
	m, _ = rootApplyMsg(m, recovered)

	_, visible = isVisibleUnderCtrlZ(m, "web-server-1")
	if visible {
		t.Error("a finding absent from the fresh, generation-matched enrichment result must be removed once that result lands; web-server-1 still survives ctrl+z")
	}
}
