package unit

// bug_detail_refresh_resets_rightcol_test.go — regression coverage for the
// Ctrl+R right-column reset on detail views: handleActionRefresh
// (core/app/actions_list.go) must clear the RelatedCache entry and reset the
// right column to loading state before the fresh checker results arrive, so
// stale counts are never visible during a reload.

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
func TestDetail_Refresh_ResetsRightColumn(t *testing.T) {
	// Set up an EC2 detail view with fully loaded related counts (each
	// Count=stubRelatedCount, chosen to never collide with a real demo count
	// for this instance — see stubRelatedCount's doc comment).
	m := setupEC2DetailWithResults(t)

	// Confirm counts are visible before refresh.
	viewBefore := stripANSI(rootViewContent(m))
	if !strings.Contains(viewBefore, "(7)") {
		t.Fatalf("precondition failed: expected '(7)' in view before Ctrl+R to confirm "+
			"related counts are loaded.\nView:\n%s", viewBefore)
	}

	// Send Ctrl+R — handleActionRefresh clears the RelatedCache entry and
	// begins a fresh DetailOperation.
	m, refreshCmd := rootApplyMsg(m, ctrlR())

	// Drain the immediate cmd (batch-aware — detail refresh for an enrichable type
	// now returns a tea.Batch of the related-check cmd + the enrich cmd) so every
	// immediate leaf message is processed by the root model. We stop after one level
	// — we do NOT want to feed checker results back in.
	m, _ = applyImmediateCmd(t, m, refreshCmd)

	viewAfter := stripANSI(rootViewContent(m))

	// The right column must be reset to loading state, so the stale "(7)" is
	// gone — including from any type whose checker resolves synchronously
	// within this single drain level (e.g. Security Groups, whose real count
	// for this instance is 2, never 7 — see stubRelatedCount).
	if strings.Contains(viewAfter, "(7)") {
		t.Fatalf("BUG: after Ctrl+R the right column still shows stale '(7)' count — "+
			"ResetRightColumn() must be called from handleRefresh to clear loaded counts "+
			"before the fresh checker results arrive.\nView:\n%s", viewAfter)
	}
}
