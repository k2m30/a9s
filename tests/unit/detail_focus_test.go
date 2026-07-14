package unit_test

// detail_focus_test.go — tests for Tab-key focus switching in the detail view
// and right-column Enter/Esc behaviour when the right column is focused (T019).
//
// Design spec: docs/design/related-resources.md v4.3
// QA stories:  docs/qa/related-resources-stories.md
//
// Key facts about Tab/focus sequencing:
//   - At width >= 100 with registered defs, the right column is auto-shown on
//     SetSize (rightColAutoShown=true, rightColVisible=false).
//   - Tab only toggles focus when rightColVisible=true.
//   - The first press of r (ToggleRelated) at width >= 100 transitions the column
//     from auto-shown → rightColVisible=true (still visible, no hide).
//   - Only after this transition does Tab toggle focus on/off.
//   - At width < 100 the right column is never shown; Tab is always a no-op.
//   - Esc while right column is focused: unfocuses (does NOT pop the view stack).
//   - Enter while right column is focused: emits RelatedNavigateMsg for selected row.

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

// makeDetailForFocusTest creates a DetailModel for "ec2" with a fields-only
// resource at the given width. It does NOT register related defs — the caller
// is responsible for that (and for cleanup via defer).
func makeDetailForFocusTest(t *testing.T, width int) views.DetailModel {
	t.Helper()
	res := resource.Resource{
		ID:   "i-focus-test",
		Name: "focus-test-instance",
		Fields: map[string]string{
			"instance_id": "i-focus-test",
			"state":       "running",
		},
	}
	k := keys.Default()
	d := views.NewDetail(res, "ec2", nil, k)
	d.SetSize(width, 30)
	return d
}

// tabKeyMsg returns the tea.KeyPressMsg for the Tab key.
func tabKeyMsg() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: tea.KeyTab}
}

// makeExplicitlyVisible transitions a DetailModel's right column from auto-shown
// (rightColAutoShown=true, rightColVisible=false) to explicitly visible
// (rightColVisible=true). This is required before Tab can toggle focus.
// With the current UX, the first press of r hides the auto-shown column and the
// second press re-opens it explicitly.
// Precondition: width >= 100, related defs registered, SetSize called.
func makeExplicitlyVisible(d views.DetailModel) views.DetailModel {
	updated, _ := d.Update(tea.KeyPressMsg{Code: -1, Text: "r"})
	updated, _ = updated.Update(tea.KeyPressMsg{Code: -1, Text: "r"})
	return updated
}

// focusRightColumn brings the right column to focused state by pressing r
// (explicit-visible transition) then Tab (focus). This is the canonical
// two-step sequence to reach "right column focused".
func focusRightColumn(d views.DetailModel) views.DetailModel {
	d = makeExplicitlyVisible(d)
	updated, _ := d.Update(tabKeyMsg())
	return updated
}

// ---------------------------------------------------------------------------
// TestDetail_TabSwitchesFocus
// Given: width=140, RelatedDefs registered for "ec2", r pressed (explicit-visible
//
//	transition), then Tab pressed
//
// When:  Tab pressed with rightColVisible=true
// Then:  View() output changes — focused row gets highlight applied
// ---------------------------------------------------------------------------
