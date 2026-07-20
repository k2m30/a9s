package unit_test

// rightcolumn_test.go tests the right column panel via the live
// Controller + NewTransientDetail.RenderDetail(body) seam (022-codebase-cleanup
// wave 3, DetailModel cluster — views.NewDetail/.Update()/.View() are
// production-dead; RenderDetail is the only reachable render entry point,
// see internal/tui/renderer.go).
//
// Design spec: docs/design/related-resources.md v4.3
// QA stories:  docs/qa/related-resources-stories.md
//
// Key design facts:
//   - `r` (ActionToggleRelated) toggles the right column ON/OFF
//   - Right column shows display names for registered RelatedDefs
//   - Side-by-side layout for all widths >= 60; below 60 the column is hidden
//   - After toggle, before any related result: rows show display names (loading state)
//   - Count >= 0 shown as "(N)"; err -> "—" (em dash)
//   - "RELATED" header section marker appears in the right column
//   - Empty RelatedDefs (no registered defs): hint text shown

import (
	"errors"
	"strings"
	"testing"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ---------------------------------------------------------------------------
// Shared test helpers
// ---------------------------------------------------------------------------

// rightColEC2Resource returns the Fields-only ec2 resource shared by every
// test below.
func rightColEC2Resource() resource.Resource {
	return resource.Resource{
		ID:   "i-test123",
		Name: "test-instance",
		Fields: map[string]string{
			"instance_id": "i-test123",
			"state":       "running",
			"type":        "t3.micro",
		},
	}
}

// newRightColController builds a detail controller for rightColEC2Resource
// and primes ds.RelatedRows/RelatedVisible via InitDetailRelatedRows —
// mirroring what real navigation does (core/app/navigate.go calls
// InitDetailRelatedRows right after EnsureDetailState). Without this priming,
// newDetailController alone leaves ds.RelatedRows empty, so buildDetailBody
// falls back to synthesizing loading blocks straight from the registered
// defs (detail_body.go's "len(ds.RelatedRows) == 0" branch) while
// ds.RelatedVisible itself stays false — a real but narrower fallback path
// that never reflects the toggle semantics a real user hits, since on real
// entry ds.RelatedVisible is already true by the time any render happens.
// Calling InitDetailRelatedRows here closes that gap so ActionToggleRelated
// exercises the same starting condition production does.
func newRightColController(t *testing.T, resourceType string) *app.Controller {
	t.Helper()
	c := newDetailController(t, rightColEC2Resource(), resourceType)
	c.InitDetailRelatedRows(resourceType)
	return c
}

// renderRightCol renders c's current detail state at width w, height h via
// the live NewTransientDetail+RenderDetail seam, ANSI-stripped: every caller
// in this file does textual (Contains/equality) assertions, not raw-style
// comparisons, so the normalized form is what they should actually compare.
func renderRightCol(t *testing.T, c *app.Controller, w, h int) string {
	t.Helper()
	body := c.Snapshot().Body.Detail
	if body == nil {
		t.Fatal("Body.Detail is nil")
	}
	vp := viewport.New(viewport.WithWidth(w), viewport.WithHeight(h))
	m := views.NewTransientDetail(w, h, vp)
	return stripAnsi(m.RenderDetail(*body))
}

// showRightColPanel establishes a definite "related panel visible" state for
// callers that need one as setup. It normalizes from whatever the CURRENT
// render shows rather than assuming a fixed starting state: registered
// RelatedDefs auto-show the panel on entry (ActionToggleRelated is then the
// FIRST flip, from visible to hidden — core/app/detail_cursor.go's
// ActionToggleRelated is a plain `ds.RelatedVisible = !ds.RelatedVisible`),
// while TestRightColumn_EmptyDefsShowsHint deliberately has no defs
// registered and starts hidden (first flip goes hidden -> visible). Reading
// the render before touching the toggle, and only toggling-then-asserting
// the OPPOSITE state, asserts the real inversion regardless of which side it
// starts on — a no-op or otherwise broken ActionToggleRelated fails at the
// very first toggle it's exercised through, instead of silently landing on
// "RELATED is present" by coincidence of toggle count.
func showRightColPanel(t *testing.T, c *app.Controller, w, h int) string {
	t.Helper()

	initial := renderRightCol(t, c, w, h)
	if strings.Contains(initial, "RELATED") {
		c.Apply(app.Action{Kind: app.ActionToggleRelated})
		hidden := renderRightCol(t, c, w, h)
		if strings.Contains(hidden, "RELATED") {
			t.Fatalf("ActionToggleRelated on an initially-visible related panel should hide it; got:\n%s", hidden)
		}
	}

	c.Apply(app.Action{Kind: app.ActionToggleRelated})
	shown := renderRightCol(t, c, w, h)
	if !strings.Contains(shown, "RELATED") {
		t.Fatalf("ActionToggleRelated should show the related panel; got:\n%s", shown)
	}
	return shown
}

// deliverRightColResult applies a RelatedCheckResult for "ec2" to c via the
// live ApplyDetailRelatedResultForResource seam (mirrors what
// DetailModel.Update's controller-backed path does for messages.RelatedCheckResult).
func deliverRightColResult(c *app.Controller, displayName, targetType string, count int, err error) {
	errMsg := ""
	state := domain.RelatedResolved
	if err != nil {
		errMsg = err.Error()
		state = domain.RelatedError
	}
	c.ApplyDetailRelatedResultForResource("ec2", "i-test123", displayName, targetType, state, count, false, errMsg, false, nil, nil)
}

// ---------------------------------------------------------------------------
// TestRightColumn_ToggleShowsRelatedHeader
// ---------------------------------------------------------------------------

func TestRightColumn_ToggleShowsRelatedHeader(t *testing.T) {
	replaceEC2Related(t, []resource.RelatedDef{
		{TargetType: "tg", DisplayName: "Target Groups", Checker: noopChecker},
		{TargetType: "asg", DisplayName: "Auto Scaling Groups", Checker: noopChecker},
	})

	c := newRightColController(t, "ec2")
	view := showRightColPanel(t, c, 140, 30)
	if !strings.Contains(view, "RELATED") {
		t.Errorf("after ActionToggleRelated, render should contain \"RELATED\"; got:\n%s", view)
	}
}

// ---------------------------------------------------------------------------
// TestRightColumn_ShowsLoadingState
// ---------------------------------------------------------------------------

func TestRightColumn_ShowsLoadingState(t *testing.T) {
	replaceEC2Related(t, []resource.RelatedDef{
		{TargetType: "tg", DisplayName: "Target Groups", Checker: noopChecker},
		{TargetType: "asg", DisplayName: "Auto Scaling Groups", Checker: noopChecker},
	})

	c := newRightColController(t, "ec2")
	view := showRightColPanel(t, c, 140, 30)
	if !strings.Contains(view, "Target Groups") {
		t.Errorf("loading state should show \"Target Groups\"; got:\n%s", view)
	}
	if !strings.Contains(view, "Auto Scaling Groups") {
		t.Errorf("loading state should show \"Auto Scaling Groups\"; got:\n%s", view)
	}
}

// ---------------------------------------------------------------------------
// TestRightColumn_CountUpdatesOnResult
// ---------------------------------------------------------------------------

func TestRightColumn_CountUpdatesOnResult(t *testing.T) {
	replaceEC2Related(t, []resource.RelatedDef{
		{TargetType: "tg", DisplayName: "Target Groups", Checker: noopChecker},
		{TargetType: "asg", DisplayName: "Auto Scaling Groups", Checker: noopChecker},
	})

	c := newRightColController(t, "ec2")
	showRightColPanel(t, c, 140, 30)
	deliverRightColResult(c, "Target Groups", "tg", 2, nil)

	view := renderRightCol(t, c, 140, 30)
	if !strings.Contains(view, "(2)") {
		t.Errorf("after Count=2 result, render should contain \"(2)\"; got:\n%s", view)
	}
}

// ---------------------------------------------------------------------------
// TestRightColumn_ZeroCountDim
// ---------------------------------------------------------------------------

func TestRightColumn_ZeroCountDim(t *testing.T) {
	replaceEC2Related(t, []resource.RelatedDef{
		{TargetType: "tg", DisplayName: "Target Groups", Checker: noopChecker},
	})

	c := newRightColController(t, "ec2")
	showRightColPanel(t, c, 140, 30)
	deliverRightColResult(c, "Target Groups", "tg", 0, nil)

	view := renderRightCol(t, c, 140, 30)
	if !strings.Contains(view, "(0)") {
		t.Errorf("after Count=0 result, render should contain \"(0)\"; got:\n%s", view)
	}
}

// ---------------------------------------------------------------------------
// TestRightColumn_ErrorShowsDash
// ---------------------------------------------------------------------------

func TestRightColumn_ErrorShowsDash(t *testing.T) {
	replaceEC2Related(t, []resource.RelatedDef{
		{TargetType: "tg", DisplayName: "Target Groups", Checker: noopChecker},
	})

	c := newRightColController(t, "ec2")
	showRightColPanel(t, c, 140, 30)
	deliverRightColResult(c, "Target Groups", "tg", 0, errors.New("permission denied"))

	view := renderRightCol(t, c, 140, 30)
	if !strings.Contains(view, "—") {
		t.Errorf("after an error result, render should contain em dash \"—\"; got:\n%s", view)
	}
}

// ---------------------------------------------------------------------------
// TestRightColumn_ToggleOffHidesPanel
// ---------------------------------------------------------------------------

func TestRightColumn_ToggleOffHidesPanel(t *testing.T) {
	replaceEC2Related(t, []resource.RelatedDef{
		{TargetType: "tg", DisplayName: "Target Groups", Checker: noopChecker},
		{TargetType: "asg", DisplayName: "Auto Scaling Groups", Checker: noopChecker},
	})

	c := newRightColController(t, "ec2")
	viewOn := showRightColPanel(t, c, 140, 30)
	if !strings.Contains(viewOn, "RELATED") {
		t.Fatalf("precondition: right column should be shown after showRightColPanel; got:\n%s", viewOn)
	}

	c.Apply(app.Action{Kind: app.ActionToggleRelated})
	viewOff := renderRightCol(t, c, 140, 30)
	if strings.Contains(viewOff, "RELATED") {
		t.Errorf("after second ActionToggleRelated, render should NOT contain \"RELATED\"; got:\n%s", viewOff)
	}
}

// ---------------------------------------------------------------------------
// TestRightColumn_NarrowTerminalIgnoresToggle
// ---------------------------------------------------------------------------

// TestRightColumn_NarrowTerminalIgnoresToggle drives the real full-TUI 'r'
// key (via the demo root model), not a bare Controller.Apply(ActionToggleRelated) —
// the headless Apply call has no terminal-width context at all, so comparing
// two Apply-then-render results at width=59 only proves the panel stays
// hidden at that width (RenderDetail's own width gate), never that the 'r'
// keypress itself was ignored. handleToggleRelated
// (internal/tui/runtime_adapter_navigate.go) has its own width guard
// (rs.width < layout.MinInnerContentWidth returns before touching
// rightColVisible) — proven here by toggling narrow, resizing back wide, and
// requiring the panel to be in exactly the state it was in before the narrow
// toggle attempt.
func TestRightColumn_NarrowTerminalIgnoresToggle(t *testing.T) {
	oldDefs := append([]resource.RelatedDef(nil), resource.GetRelated("ec2")...)
	t.Cleanup(func() { resource.SetRelatedForTest("ec2", oldDefs) })
	resource.SetRelatedForTest("ec2", []resource.RelatedDef{
		{TargetType: "tg", DisplayName: "Target Groups", Checker: resource.NoopCheckerForTest},
		{TargetType: "asg", DisplayName: "Auto Scaling Groups", Checker: resource.NoopCheckerForTest},
	})

	m := newPreviewDemoModel(t, 120, 30)
	ec2Res := previewEC2Resource()
	m, _ = previewApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		ResourceType: "ec2",
		Resource:     &ec2Res,
	})

	wideBefore := previewView(m)
	if !strings.Contains(wideBefore, "RELATED") {
		t.Fatalf("precondition: RELATED panel should be visible at width=120; got:\n%s", wideBefore)
	}

	// 59 < layout.MinTerminalWidth(60); inner width 57 < MinInnerContentWidth(58).
	m, _ = previewApplyMsg(m, tea.WindowSizeMsg{Width: 59, Height: 30})
	m, _ = previewApplyMsg(m, tea.KeyPressMsg{Code: -1, Text: "r"})

	m, _ = previewApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 30})
	wideAfter := previewView(m)

	if wideAfter != wideBefore {
		t.Errorf("'r' at a narrow width should be ignored; after resizing back to width=120 the panel state changed:\nbefore:\n%s\nafter:\n%s", wideBefore, wideAfter)
	}
}

// ---------------------------------------------------------------------------
// TestRightColumn_EmptyDefsShowsHint
// ---------------------------------------------------------------------------

func TestRightColumn_EmptyDefsShowsHint(t *testing.T) {
	unregisterEC2Related(t)

	c := newRightColController(t, "ec2")
	view := showRightColPanel(t, c, 140, 30)

	if !strings.Contains(view, "RELATED") {
		t.Errorf("even with empty defs, ActionToggleRelated should show the right column with RELATED header; got:\n%s", view)
	}
	if strings.Contains(view, "Target Groups") || strings.Contains(view, "Auto Scaling Groups") {
		t.Errorf("with empty defs, render should NOT contain type names; got:\n%s", view)
	}
	if !strings.Contains(view, "No related types registered") {
		t.Errorf("with empty defs, render should show the empty-state hint 'No related types registered'; got:\n%s", view)
	}
}

// ---------------------------------------------------------------------------
// TestRightColumn_MultipleResults_EachUpdatesIndependently
// ---------------------------------------------------------------------------

func TestRightColumn_MultipleResults_EachUpdatesIndependently(t *testing.T) {
	replaceEC2Related(t, []resource.RelatedDef{
		{TargetType: "tg", DisplayName: "Target Groups", Checker: noopChecker},
		{TargetType: "asg", DisplayName: "Auto Scaling Groups", Checker: noopChecker},
	})

	c := newRightColController(t, "ec2")
	showRightColPanel(t, c, 140, 30)
	deliverRightColResult(c, "Target Groups", "tg", 3, nil)
	deliverRightColResult(c, "Auto Scaling Groups", "asg", 1, nil)

	view := renderRightCol(t, c, 140, 30)
	tgLine := findLineContaining(view, "Target Groups")
	if !strings.Contains(tgLine, "(3)") {
		t.Errorf("Target Groups row should show \"(3)\"; got line:\n%s\nfull view:\n%s", tgLine, view)
	}
	asgLine := findLineContaining(view, "Auto Scaling Groups")
	if !strings.Contains(asgLine, "(1)") {
		t.Errorf("Auto Scaling Groups row should show \"(1)\"; got line:\n%s\nfull view:\n%s", asgLine, view)
	}
}

// ---------------------------------------------------------------------------
// TestRightColumn_ToggleDefaultState_OnEntry
// ---------------------------------------------------------------------------

func TestRightColumn_ToggleDefaultState_OnEntry(t *testing.T) {
	replaceEC2Related(t, []resource.RelatedDef{
		{TargetType: "tg", DisplayName: "Target Groups", Checker: noopChecker},
	})

	c := newRightColController(t, "ec2")
	view := renderRightCol(t, c, 140, 30)

	if !strings.Contains(view, "RELATED") {
		t.Errorf("right column should be ON by default (no toggle needed); render should contain \"RELATED\"; got:\n%s", view)
	}
}

// ---------------------------------------------------------------------------
// TestRightColumn_View_WideTerminalShowsSideBySide
// ---------------------------------------------------------------------------

func TestRightColumn_View_WideTerminalShowsSideBySide(t *testing.T) {
	replaceEC2Related(t, []resource.RelatedDef{
		{TargetType: "tg", DisplayName: "Target Groups", Checker: noopChecker},
	})

	c := newRightColController(t, "ec2")
	view := showRightColPanel(t, c, 140, 30)

	if !strings.Contains(view, "instance_id") && !strings.Contains(view, "running") && !strings.Contains(view, "t3.micro") {
		t.Errorf("at width=140, left column should show resource fields; got:\n%s", view)
	}
	if !strings.Contains(view, "Target Groups") {
		t.Errorf("at width=140, right column should show related type names; got:\n%s", view)
	}
}
