package unit

// bug_detail_refresh_resets_rightcol_test.go — Test revealing the Ctrl+R
// right-column reset bug on detail views.
//
// Bug: internal/tui/app_handlers.go:511-522 handles Ctrl+R on detail views:
// it deletes the cache entry and emits RelatedCheckStartedMsg, but never resets
// the right column's visible state. Stale counts remain visible during reload.
//
// The fix adds ResetRightColumn() to DetailModel and calls it from handleRefresh.
// This test FAILS with current code (stale counts persist) and passes after the fix
// (right column reverts to loading state).

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// ctrlR produces the Ctrl+R key press message used by the Refresh binding.
func ctrlR() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl}
}

// TestDetail_Refresh_ResetsRightColumn verifies that pressing Ctrl+R on a detail
// view with loaded related counts immediately resets the right column to loading
// state, so stale counts are not shown while the refresh is in progress.
//
// EXPECTED: after Ctrl+R the view does NOT show "(2)" count suffixes.
// ACTUAL (BUG): right column still shows "(2)" because ResetRightColumn() is
// never called — the stale counts remain until new results arrive.
//
// This test FAILS with current code.
func TestDetail_Refresh_ResetsRightColumn(t *testing.T) {
	// Set up an EC2 detail view with fully loaded related counts (each Count=2).
	m := setupEC2DetailWithResults(t)

	// Confirm counts are visible before refresh — "(2)" must appear.
	viewBefore := stripANSI(rootViewContent(m))
	if !strings.Contains(viewBefore, "(2)") {
		t.Fatalf("precondition failed: expected '(2)' in view before Ctrl+R to confirm "+
			"related counts are loaded.\nView:\n%s", viewBefore)
	}

	// Send Ctrl+R — the handler deletes the cache entry and emits RelatedCheckStartedMsg
	// but (BUG) does not reset the right column.
	m, refreshCmd := rootApplyMsg(m, ctrlR())

	// Drain the immediate cmd so RelatedCheckStartedMsg is processed by the root model.
	// We stop after one level — we do NOT want to feed checker results back in.
	if refreshCmd != nil {
		msg := refreshCmd()
		if msg != nil {
			m, _ = rootApplyMsg(m, msg)
		}
	}

	viewAfter := stripANSI(rootViewContent(m))

	// EXPECTED: the right column has been reset to loading state, so "(2)" is gone.
	// BUG: stale "(2)" count suffix is still present because ResetRightColumn() was
	// never called.
	if strings.Contains(viewAfter, "(2)") {
		t.Fatalf("BUG: after Ctrl+R the right column still shows stale '(2)' count — "+
			"ResetRightColumn() must be called from handleRefresh to clear loaded counts "+
			"before the fresh checker results arrive.\nView:\n%s", viewAfter)
	}
}
