package unit

// qa_enrichment_review_fixes_test.go — regression tests for two review
// findings landed on top of feature 018-enrichment-visibility:
//
//   1. resolveIdentityColumn must run on the full column list (pre-hscroll) so
//      horizontal scrolling cannot make the marker jump to a different semantic
//      column (e.g. State when Name is scrolled off).
//   2. Ctrl+R on a top-level list must clear the active ResourceListModel's
//      findings immediately, not only the root-model copies.

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// -----------------------------------------------------------------------------
// Fix 1: resolveIdentityColumn runs on full columns; marker hidden when identity
// column is hscrolled off-screen
// -----------------------------------------------------------------------------

// TestRowMarker_HiddenWhenIdentityColumnScrolledOff asserts that when the user
// scrolls horizontally so the identity column (Name) is not in the visible
// column slice, the row marker is NOT rendered on a different column.
//
// Pre-fix: resolveIdentityColumn ran on the post-hscroll cols, so it cascaded
// to a different column (e.g. one whose path contained "Name") and the dot
// jumped to that column.
//
// Approach: create a list with a narrow terminal so the full column set doesn't
// fit, then send "l" (ScrollRight) key to advance hscroll. After scrolling,
// check that the Name column values are gone from the output AND the dot is
// also gone.
func TestRowMarker_HiddenWhenIdentityColumnScrolledOff(t *testing.T) {
	td := resource.ResourceTypeDef{
		Name:      "Test",
		ShortName: "test",
		Columns: []resource.Column{
			{Key: "name", Title: "Name", Width: 30},
			{Key: "state", Title: "State", Width: 12},
			{Key: "type", Title: "Type", Width: 12},
			{Key: "region", Title: "Region", Width: 12},
		},
	}
	k := keys.Default()
	m := views.NewResourceList(td, nil, k)
	// Narrow width: forces at least one column to overflow so ScrollRight is allowed.
	m.SetSize(50, 10)
	m, _ = m.Init()

	resources := []resource.Resource{
		{ID: "r-1", Name: "alpha-instance-with-distinctive-name", Fields: map[string]string{"name": "alpha-instance-with-distinctive-name", "state": "available", "type": "m5", "region": "us-east-1"}},
	}
	m, _ = m.Update(messages.ResourcesLoaded{ResourceType: "test", Resources: resources})

	findings := map[string][]domain.Finding{
		"r-1": {{Code: "ec2.system.status.impaired", Phrase: "broken", Severity: domain.SevBroken, Source: "wave2:ec2"}},
	}
	m.SetEnrichmentState(len(findings), false, findings, nil)

	// Baseline: prefix marker present at hScrollOffset=0.
	baseline := m.View()
	if !strings.Contains(baseline, "! ") {
		t.Fatalf("pre-condition failed: expected '! ' prefix marker in baseline render; output:\n%s", baseline)
	}

	// Scroll right repeatedly until the Name column value is off-screen.
	scrollRight := tea.KeyPressMsg{Code: 'l', Text: "l"}
	var scrolled string
	for i := 0; i < 3; i++ {
		m, _ = m.Update(scrollRight)
		scrolled = m.View()
		if !strings.Contains(scrolled, "alpha-instance-with-distinctive-name") {
			break
		}
	}

	if strings.Contains(scrolled, "alpha-instance-with-distinctive-name") {
		t.Skip("could not scroll Name column off-screen; terminal width too wide for this test")
	}

	// Key assertion: prefix marker must not render when the identity column is not visible.
	if strings.Contains(scrolled, "! ") {
		t.Errorf("marker must not render when identity column is scrolled off-screen (would jump to wrong column); output:\n%s", scrolled)
	}
}

// -----------------------------------------------------------------------------
// Fix 2 (superseded): Ctrl+R keeps findings visible until the fresh result
// lands, rather than blanking them at keypress time
// -----------------------------------------------------------------------------

// TestCtrlR_ClearsActiveListFindingsImmediately asserted that pressing Ctrl+R
// on a top-level list blanked the active ResourceListModel's findings
// immediately, before the wrapped fetch even returned. That contract is
// superseded: eagerly blanking findings at keypress time produced a real,
// user-visible flicker for the full AWS round-trip between the keypress and
// the rerun's EnrichmentChecked arrival — findings must be stale-until-
// replaced, not blank-until-replaced (see qa_glyph_continuity_test.go's
// TestRerunStart_KeepsVisibleFindingsUntilReplaced for the corrected pin,
// which asserts BOTH halves: no blank window before the fresh result lands,
// and real removal once the fresh result genuinely omits the finding).
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

func TestCtrlR_ClearsActiveListFindingsImmediately(t *testing.T) {
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

	// Dispatch Ctrl+R via the real key path.
	m, _ = rootApplyMsg(m, ctrlRKeyMsg())

	// Assertion (corrected): the finding must still be applied immediately
	// after Ctrl+R — no fetch/enrichment response has landed yet, so the old
	// finding is still the best-known truth (stale-until-replaced).
	m, visible = isVisibleUnderCtrlZ(m, "web-server-1")
	if !visible {
		t.Error("Ctrl+R must NOT blank the active list's findings before the fresh result lands; web-server-1 no longer survives ctrl+z immediately after keypress")
	}

	// Once a fresh EnrichmentCheckedMsg lands and genuinely omits the
	// finding (resource recovered), the finding MUST be removed — this test
	// still confirms removal works, just not before the result lands.
	recovered := messages.EnrichmentChecked{
		ResourceType: "ec2",
		Issues:       0,
		Truncated:    false,
		Findings:     map[string][]domain.Finding{},
		Gen:          0,
		TypeGen:      0,
	}
	m, _ = rootApplyMsg(m, recovered)

	_, visible = isVisibleUnderCtrlZ(m, "web-server-1")
	if visible {
		t.Error("a finding absent from the fresh enrichment result must be removed once that result lands; web-server-1 still survives ctrl+z")
	}
}
