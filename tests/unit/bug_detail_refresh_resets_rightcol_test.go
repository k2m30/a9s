package unit

// Ctrl+R on a detail view resets the right column to loading before fresh
// checker results arrive, so stale counts are never visible during a reload.

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func ctrlR() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl}
}

func TestDetail_Refresh_ResetsRightColumn(t *testing.T) {
	m := setupEC2DetailWithResults(t)

	viewBefore := stripANSI(rootViewContent(m))
	if !strings.Contains(viewBefore, "(7)") {
		t.Fatalf("precondition failed: expected '(7)' in view before Ctrl+R to confirm "+
			"related counts are loaded.\nView:\n%s", viewBefore)
	}

	m, refreshCmd := rootApplyMsg(m, ctrlR())

	// One drain level only: feeding checker results back in would repopulate
	// the counts.
	m, _ = applyImmediateCmd(t, m, refreshCmd)

	viewAfter := stripANSI(rootViewContent(m))

	// A checker that resolves within one drain level (Security Groups) reports
	// its real count, never 7, so any "(7)" is stale.
	if strings.Contains(viewAfter, "(7)") {
		t.Fatalf("BUG: after Ctrl+R the right column still shows stale '(7)' count — "+
			"ResetRightColumn() must be called from handleRefresh to clear loaded counts "+
			"before the fresh checker results arrive.\nView:\n%s", viewAfter)
	}
}
