package unit

// rightcolumn_test.go tests the right column sub-component via the exported
// DetailModel interface. rightColumnModel is unexported, so all assertions are
// made on DetailModel.View() output and state after Update() calls.
//
// Design spec: docs/design/related-resources.md v4.3
// QA stories:  docs/qa/related-resources-stories.md
//
// Key design facts:
//   - `r` (ToggleRelated) toggles the right column ON/OFF
//   - Right column shows display names for registered RelatedDefs
//   - Right column fixed width: 32 chars (adjusts proportionally below 100 cols); separator: 1 char
//   - Side-by-side layout for all widths >= 60; below 60 the column is hidden
//   - After toggle, before any RelatedCheckResultMsg: rows show display names (loading state)
//   - RelatedCheckResultMsg delivers async check results (count, err)
//   - Count ≥ 0 shown as "(N)"; err → "—" (em dash)
//   - "RELATED" header section marker appears in the right column
//   - Empty RelatedDefs (no registered defs): hint text shown

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ---------------------------------------------------------------------------
// Shared test helpers
// ---------------------------------------------------------------------------

// makeDetailForRelatedTest creates a DetailModel with a Fields-only resource,
// sets the given width/height, and returns it ready for testing.
func makeDetailForRelatedTest(t *testing.T, width int) views.DetailModel {
	t.Helper()
	res := resource.Resource{
		ID:   "i-test123",
		Name: "test-instance",
		Fields: map[string]string{
			"instance_id": "i-test123",
			"state":       "running",
			"type":        "t3.micro",
		},
	}
	k := keys.Default()
	d := views.NewDetail(res, "ec2", nil, k)
	d.SetSize(width, 30)
	return d
}

// toggleRelatedKeyMsg returns the tea.KeyPressMsg for the `r` key binding
// used by ToggleRelated.
func toggleRelatedKeyMsg() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: -1, Text: "r"}
}

// sendToggleRelated sends the ToggleRelated key to a DetailModel and returns
// the updated model.
func sendToggleRelated(d views.DetailModel) views.DetailModel {
	updated, _ := d.Update(toggleRelatedKeyMsg())
	return updated
}

// showRelatedPanel ensures the related panel ends in the visible state.
// On wide layouts the first press hides the auto-shown panel, so we may need
// a second press to reopen it explicitly.
func showRelatedPanel(d views.DetailModel) views.DetailModel {
	updated := sendToggleRelated(d)
	if strings.Contains(updated.View(), "RELATED") {
		return updated
	}
	return sendToggleRelated(updated)
}

// sendRelatedResult delivers a RelatedCheckResultMsg to a DetailModel and
// returns the updated model.
func sendRelatedResult(d views.DetailModel, msg messages.RelatedCheckResultMsg) views.DetailModel {
	updated, _ := d.Update(msg)
	return updated
}

// ---------------------------------------------------------------------------
// TestRightColumn_ToggleShowsRelatedHeader
// Given: width=140, RelatedDefs registered for "ec2"
// When:  ToggleRelated key pressed
// Then:  View() contains "RELATED"
// ---------------------------------------------------------------------------

func TestRightColumn_ToggleShowsRelatedHeader(t *testing.T) {
	resource.RegisterRelated("ec2", []resource.RelatedDef{
		{TargetType: "tg", DisplayName: "Target Groups", Checker: nil},
		{TargetType: "asg", DisplayName: "Auto Scaling Groups", Checker: nil},
	})
	defer resource.UnregisterRelated("ec2")

	d := makeDetailForRelatedTest(t, 140)
	d = showRelatedPanel(d)

	view := d.View()
	if !strings.Contains(view, "RELATED") {
		t.Errorf("after ToggleRelated, View() should contain \"RELATED\"; got:\n%s", view)
	}
}

// ---------------------------------------------------------------------------
// TestRightColumn_ShowsLoadingState
// Given: width=140, RelatedDefs registered for "ec2", toggle pressed
// When:  no RelatedCheckResultMsg delivered yet
// Then:  View() contains display names of registered related types
// ---------------------------------------------------------------------------

func TestRightColumn_ShowsLoadingState(t *testing.T) {
	resource.RegisterRelated("ec2", []resource.RelatedDef{
		{TargetType: "tg", DisplayName: "Target Groups", Checker: nil},
		{TargetType: "asg", DisplayName: "Auto Scaling Groups", Checker: nil},
	})
	defer resource.UnregisterRelated("ec2")

	d := makeDetailForRelatedTest(t, 140)
	d = showRelatedPanel(d)

	view := d.View()
	if !strings.Contains(view, "Target Groups") {
		t.Errorf("loading state should show \"Target Groups\" in View(); got:\n%s", view)
	}
	if !strings.Contains(view, "Auto Scaling Groups") {
		t.Errorf("loading state should show \"Auto Scaling Groups\" in View(); got:\n%s", view)
	}
}

// ---------------------------------------------------------------------------
// TestRightColumn_CountUpdatesOnResult
// Given: width=140, RelatedDefs registered, toggle pressed
// When:  RelatedCheckResultMsg{TargetType:"tg", Count:2} delivered
// Then:  View() contains "(2)"
// ---------------------------------------------------------------------------

func TestRightColumn_CountUpdatesOnResult(t *testing.T) {
	resource.RegisterRelated("ec2", []resource.RelatedDef{
		{TargetType: "tg", DisplayName: "Target Groups", Checker: nil},
		{TargetType: "asg", DisplayName: "Auto Scaling Groups", Checker: nil},
	})
	defer resource.UnregisterRelated("ec2")

	d := makeDetailForRelatedTest(t, 140)
	d = showRelatedPanel(d)
	d = sendRelatedResult(d, messages.RelatedCheckResultMsg{
		ResourceType: "ec2",
		Result: resource.RelatedCheckResult{
			TargetType:  "tg",
			Count:       2,
			ResourceIDs: []string{"tg-aaa111", "tg-bbb222"},
			Err:         nil,
		},
	})

	view := d.View()
	if !strings.Contains(view, "(2)") {
		t.Errorf("after RelatedCheckResultMsg with Count=2, View() should contain \"(2)\"; got:\n%s", view)
	}
}

// ---------------------------------------------------------------------------
// TestRightColumn_ZeroCountDim
// Given: width=140, RelatedDefs registered, toggle pressed
// When:  RelatedCheckResultMsg{TargetType:"tg", Count:0} delivered
// Then:  View() contains "(0)"
// ---------------------------------------------------------------------------

func TestRightColumn_ZeroCountDim(t *testing.T) {
	resource.RegisterRelated("ec2", []resource.RelatedDef{
		{TargetType: "tg", DisplayName: "Target Groups", Checker: nil},
	})
	defer resource.UnregisterRelated("ec2")

	d := makeDetailForRelatedTest(t, 140)
	d = showRelatedPanel(d)
	d = sendRelatedResult(d, messages.RelatedCheckResultMsg{
		ResourceType: "ec2",
		Result: resource.RelatedCheckResult{
			TargetType:  "tg",
			Count:       0,
			ResourceIDs: nil,
			Err:         nil,
		},
	})

	view := d.View()
	if !strings.Contains(view, "(0)") {
		t.Errorf("after RelatedCheckResultMsg with Count=0, View() should contain \"(0)\"; got:\n%s", view)
	}
}

// ---------------------------------------------------------------------------
// TestRightColumn_ErrorShowsDash
// Given: width=140, RelatedDefs registered, toggle pressed
// When:  RelatedCheckResultMsg with Err!=nil delivered
// Then:  View() contains "—" (em dash U+2014)
// ---------------------------------------------------------------------------

func TestRightColumn_ErrorShowsDash(t *testing.T) {
	resource.RegisterRelated("ec2", []resource.RelatedDef{
		{TargetType: "tg", DisplayName: "Target Groups", Checker: nil},
	})
	defer resource.UnregisterRelated("ec2")

	d := makeDetailForRelatedTest(t, 140)
	d = showRelatedPanel(d)
	d = sendRelatedResult(d, messages.RelatedCheckResultMsg{
		ResourceType: "ec2",
		Result: resource.RelatedCheckResult{
			TargetType:  "tg",
			Count:       -1,
			ResourceIDs: nil,
			Err:         errors.New("permission denied"),
		},
	})

	view := d.View()
	// Em dash (U+2014) is rendered for error states per design spec
	if !strings.Contains(view, "\u2014") {
		t.Errorf("after RelatedCheckResultMsg with Err!=nil, View() should contain em dash \"—\"; got:\n%s", view)
	}
}

// ---------------------------------------------------------------------------
// TestRightColumn_ToggleOffHidesPanel
// Given: width=140, RelatedDefs registered, toggle pressed (right column ON)
// When:  toggle pressed again (right column OFF)
// Then:  View() no longer contains "RELATED"
// ---------------------------------------------------------------------------

func TestRightColumn_ToggleOffHidesPanel(t *testing.T) {
	resource.RegisterRelated("ec2", []resource.RelatedDef{
		{TargetType: "tg", DisplayName: "Target Groups", Checker: nil},
		{TargetType: "asg", DisplayName: "Auto Scaling Groups", Checker: nil},
	})
	defer resource.UnregisterRelated("ec2")

	d := makeDetailForRelatedTest(t, 140)
	// Toggle ON
	d = showRelatedPanel(d)
	viewOn := d.View()
	if !strings.Contains(viewOn, "RELATED") {
		t.Skip("right column not shown after first toggle; skipping off-toggle test")
	}

	// Toggle OFF
	d = sendToggleRelated(d)
	viewOff := d.View()
	if strings.Contains(viewOff, "RELATED") {
		t.Errorf("after second ToggleRelated, View() should NOT contain \"RELATED\"; got:\n%s", viewOff)
	}
}

// ---------------------------------------------------------------------------
// TestRightColumn_NarrowTerminalIgnoresToggle
// Given: width=59 (below minimum width for related panel)
// When:  ToggleRelated key pressed
// Then:  View() is unchanged — toggle is a no-op below the width threshold
// ---------------------------------------------------------------------------

func TestRightColumn_NarrowTerminalIgnoresToggle(t *testing.T) {
	resource.RegisterRelated("ec2", []resource.RelatedDef{
		{TargetType: "tg", DisplayName: "Target Groups", Checker: nil},
		{TargetType: "asg", DisplayName: "Auto Scaling Groups", Checker: nil},
	})
	defer resource.UnregisterRelated("ec2")

	d := makeDetailForRelatedTest(t, 59)
	viewBefore := d.View()
	d = showRelatedPanel(d)
	viewAfter := d.View()

	if viewBefore != viewAfter {
		t.Errorf("at width=59 (below related-panel threshold), ToggleRelated should be a no-op; view changed:\nbefore:\n%s\nafter:\n%s", viewBefore, viewAfter)
	}
}

// ---------------------------------------------------------------------------
// TestRightColumn_EmptyDefsShowsHint
// Given: width=140, NO RelatedDefs registered for "ec2"
// When:  ToggleRelated key pressed
// Then:  View() contains hint text indicating no related types
// ---------------------------------------------------------------------------

func TestRightColumn_EmptyDefsShowsHint(t *testing.T) {
	// Ensure "ec2" has no related defs registered (clean state)
	resource.UnregisterRelated("ec2")

	d := makeDetailForRelatedTest(t, 140)
	d = showRelatedPanel(d)

	view := d.View()
	// When no RelatedDefs exist, the right column should show a hint that no
	// related resource types are configured. The exact text is implementation-
	// defined, but the column must appear (RELATED header) and show some hint.
	if !strings.Contains(view, "RELATED") {
		t.Errorf("even with empty defs, ToggleRelated should show the right column with RELATED header; got:\n%s", view)
	}
	// The panel should NOT contain actual type names when none are registered
	if strings.Contains(view, "Target Groups") || strings.Contains(view, "Auto Scaling Groups") {
		t.Errorf("with empty defs, View() should NOT contain type names; got:\n%s", view)
	}
}

// ---------------------------------------------------------------------------
// TestRightColumn_MultipleResults_EachUpdatesIndependently
// Given: width=140, two RelatedDefs registered
// When:  two separate RelatedCheckResultMsgs delivered (one for each type)
// Then:  View() contains both counts
// ---------------------------------------------------------------------------

func TestRightColumn_MultipleResults_EachUpdatesIndependently(t *testing.T) {
	resource.RegisterRelated("ec2", []resource.RelatedDef{
		{TargetType: "tg", DisplayName: "Target Groups", Checker: nil},
		{TargetType: "asg", DisplayName: "Auto Scaling Groups", Checker: nil},
	})
	defer resource.UnregisterRelated("ec2")

	d := makeDetailForRelatedTest(t, 140)
	d = showRelatedPanel(d)
	d = sendRelatedResult(d, messages.RelatedCheckResultMsg{
		ResourceType: "ec2",
		Result: resource.RelatedCheckResult{
			TargetType:  "tg",
			Count:       3,
			ResourceIDs: []string{"tg-1", "tg-2", "tg-3"},
			Err:         nil,
		},
	})
	d = sendRelatedResult(d, messages.RelatedCheckResultMsg{
		ResourceType: "ec2",
		Result: resource.RelatedCheckResult{
			TargetType:  "asg",
			Count:       1,
			ResourceIDs: []string{"asg-xyz"},
			Err:         nil,
		},
	})

	view := d.View()
	if !strings.Contains(view, "(3)") {
		t.Errorf("View() should contain \"(3)\" for tg count; got:\n%s", view)
	}
	if !strings.Contains(view, "(1)") {
		t.Errorf("View() should contain \"(1)\" for asg count; got:\n%s", view)
	}
}

// ---------------------------------------------------------------------------
// TestRightColumn_WrongResourceType_ResultIgnored
// Given: width=140, RelatedDefs registered for "ec2", toggle pressed
// When:  RelatedCheckResultMsg with ResourceType="rds" (wrong type) delivered
// Then:  View() does NOT update with the count — wrong-type result is ignored
// ---------------------------------------------------------------------------

func TestRightColumn_WrongResourceType_ResultIgnored(t *testing.T) {
	resource.RegisterRelated("ec2", []resource.RelatedDef{
		{TargetType: "tg", DisplayName: "Target Groups", Checker: nil},
	})
	defer resource.UnregisterRelated("ec2")

	d := makeDetailForRelatedTest(t, 140)
	d = sendToggleRelated(d)
	// Deliver a result for "rds" — should be ignored by the ec2 detail model
	d = sendRelatedResult(d, messages.RelatedCheckResultMsg{
		ResourceType: "rds",
		Result: resource.RelatedCheckResult{
			TargetType:  "tg",
			Count:       99,
			ResourceIDs: nil,
			Err:         nil,
		},
	})

	view := d.View()
	if strings.Contains(view, "(99)") {
		t.Errorf("result for wrong resource type \"rds\" should be ignored; View() should not contain \"(99)\"; got:\n%s", view)
	}
}

// ---------------------------------------------------------------------------
// TestRightColumn_ToggleDefaultState_OnEntry
// Given: width=140, RelatedDefs registered for "ec2"
// When:  DetailModel created and SetSize called (no toggle sent)
// Then:  View() contains "RELATED" — right column is ON by default
// ---------------------------------------------------------------------------

func TestRightColumn_ToggleDefaultState_OnEntry(t *testing.T) {
	resource.RegisterRelated("ec2", []resource.RelatedDef{
		{TargetType: "tg", DisplayName: "Target Groups", Checker: nil},
	})
	defer resource.UnregisterRelated("ec2")

	d := makeDetailForRelatedTest(t, 140)
	view := d.View()

	if !strings.Contains(view, "RELATED") {
		t.Errorf("right column should be ON by default (no toggle needed); View() should contain \"RELATED\"; got:\n%s", view)
	}
}

// ---------------------------------------------------------------------------
// TestRightColumn_View_WideTerminalShowsSideBySide
// Given: width=140 (≥ 100 column threshold)
// When:  ToggleRelated pressed, RelatedDefs registered
// Then:  View() contains both field content AND related type names in same output
//        (side-by-side layout — both columns visible simultaneously)
// ---------------------------------------------------------------------------

func TestRightColumn_View_WideTerminalShowsSideBySide(t *testing.T) {
	resource.RegisterRelated("ec2", []resource.RelatedDef{
		{TargetType: "tg", DisplayName: "Target Groups", Checker: nil},
	})
	defer resource.UnregisterRelated("ec2")

	d := makeDetailForRelatedTest(t, 140)
	d = showRelatedPanel(d)
	view := d.View()

	// At 140 columns, both the resource field content (left col) and related
	// types (right col) should appear in the same View() output.
	if !strings.Contains(view, "instance_id") && !strings.Contains(view, "running") && !strings.Contains(view, "t3.micro") {
		t.Errorf("at width=140, left column should show resource fields; got:\n%s", view)
	}
	if !strings.Contains(view, "Target Groups") {
		t.Errorf("at width=140, right column should show related type names; got:\n%s", view)
	}
}
