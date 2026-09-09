package unit

// issue230_ctrl_r_cache_clear_test.go — Ctrl+R from a detail view must clear
// the RelatedCache entry for the current resource (not just re-dispatch
// checkers on top of stale cached state) and reset the right column to
// loading state before any fresh checker result arrives, so stale counts are
// never visible during a refresh. All three tests use the actual Ctrl+R key
// press (via ctrlR(), defined in bug_detail_refresh_resets_rightcol_test.go)
// so they exercise the real handler path (core/app/actions_list.go's
// handleActionRefresh).

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// TestContract_CtrlR_ClearsRelatedCache_ThenRechecks is the full integration
// test. It verifies that Ctrl+R:
//
//	(a) clears the stale cached data so the right column shows loading state, and
//	(b) re-dispatches related checkers (a RelatedCheckResult appears in the chain).
func TestContract_CtrlR_ClearsRelatedCache_ThenRechecks(t *testing.T) {
	m := setupEC2DetailWithResults(t)

	// Precondition: cached results are visible.
	viewBefore := stripANSI(rootViewContent(m))
	if !strings.Contains(viewBefore, "(7)") {
		t.Fatalf("precondition failed: expected '(7)' in view before Ctrl+R to confirm "+
			"related counts are loaded and visible.\nView:\n%s", viewBefore)
	}

	// Send the actual Ctrl+R key — exercises the LIVE global handler path.
	m, refreshCmd := rootApplyMsg(m, ctrlR())

	// Drain exactly one level (batch-aware — detail refresh for an enrichable
	// type now returns a tea.Batch of the related-check cmd + the enrich cmd)
	// to process any immediate leaf messages (e.g., a RelatedCheckResult).
	// We stop before feeding checker results back to keep the right column in loading state.
	var immediateMsgs []tea.Msg
	m, immediateMsgs = applyImmediateCmd(t, m, refreshCmd)

	// (a) Right column must be in loading state — the stale "(7)" must be
	// gone, including from any type whose checker resolves synchronously
	// within this single drain level (e.g. Security Groups, whose real count
	// for this instance is 2, never 7 — see stubRelatedCount).
	viewAfter := stripANSI(rootViewContent(m))
	if strings.Contains(viewAfter, "(7)") {
		t.Fatalf("contract violated: after Ctrl+R the right column still shows stale '(7)' "+
			"— the relatedCache entry must be cleared before checkers re-run so stale data "+
			"is never shown during a refresh.\nView:\n%s", viewAfter)
	}

	// (b) A RelatedCheckResult must have appeared in the chain.
	// It is either the immediate cmd result or needs another drain level.
	if len(immediateMsgs) == 0 {
		t.Fatal("contract violated: Ctrl+R produced no cmd — " +
			"a RelatedCheckResult must be dispatched to re-run checkers")
	}

	// Scan the already-collected immediateMsgs (from applyImmediateCmd above)
	// for RelatedCheckResult — do NOT re-execute refreshCmd a second time
	// via drainCmds. refreshCmd is a tea.Cmd closure; the real Bubble Tea
	// runtime invokes it exactly once per dispatch, and re-invoking it here
	// would simulate something the runtime never does.
	found := false
	for _, msg := range immediateMsgs {
		if _, ok := msg.(messages.RelatedCheckResult); ok {
			found = true
			break
		}
	}

	if !found {
		t.Fatal("contract violated: Ctrl+R did not produce a RelatedCheckResult " +
			"in the cmd chain — checkers must be re-dispatched after a refresh")
	}
}

// TestContract_CtrlR_FromDetail_ProducesRelatedCheckStarted pins the basic contract:
// pressing Ctrl+R while in a detail view MUST produce a RelatedCheckResult
// somewhere in the cmd chain.
func TestContract_CtrlR_FromDetail_ProducesRelatedCheckStarted(t *testing.T) {
	m := setupEC2DetailWithResults(t)

	// Send the actual Ctrl+R key press.
	m, refreshCmd := rootApplyMsg(m, ctrlR())

	// Drain the full cmd chain (up to 10 levels) and collect all messages.
	_, chainMsgs := drainCmds(t, m, refreshCmd, 10)

	found := false
	var msgTypes []string
	for _, msg := range chainMsgs {
		msgTypes = append(msgTypes, fmt.Sprintf("%T", msg))
		if _, ok := msg.(messages.RelatedCheckResult); ok {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("contract violated: Ctrl+R from detail view must produce "+
			"a RelatedCheckResult in the cmd chain to re-run related checkers.\n"+
			"Messages produced: %v", msgTypes)
	}
}

// TestContract_CtrlR_RightColumnResets_BeforeRecheck is the key regression
// guard against stale data: after Ctrl+R, before any new RelatedCheckResult
// is delivered, the right column MUST NOT show the previously-loaded count
// values.
func TestContract_CtrlR_RightColumnResets_BeforeRecheck(t *testing.T) {
	m := setupEC2DetailWithResults(t)

	// Precondition: right column shows loaded counts.
	viewBefore := stripANSI(rootViewContent(m))
	if !strings.Contains(viewBefore, "(7)") {
		t.Fatalf("precondition failed: expected '(7)' in right column before Ctrl+R.\n"+
			"View:\n%s", viewBefore)
	}

	// Press Ctrl+R — exercises the real key path.
	m, refreshCmd := rootApplyMsg(m, ctrlR())

	// Drain exactly ONE level of the cmd chain (batch-aware — detail refresh for
	// an enrichable type now returns a tea.Batch of the related-check cmd + the
	// enrich cmd) to process any immediate leaf messages, including a
	// RelatedCheckResult if it is among them. We intentionally do NOT feed
	// further RelatedCheckResults back in — that would populate the right
	// column again and mask the bug.
	m, _ = applyImmediateCmd(t, m, refreshCmd)

	// The right column must be in loading/empty state — no stale counts
	// visible, including from any type whose checker resolves synchronously
	// within this single drain level (e.g. Security Groups, whose real count
	// for this instance is 2, never 7 — see stubRelatedCount).
	viewAfter := stripANSI(rootViewContent(m))
	if strings.Contains(viewAfter, "(7)") {
		t.Fatalf("contract violated (issue #230 regression): after Ctrl+R the right column "+
			"still shows stale '(7)' count suffixes before any new checker results arrive.\n"+
			"This indicates the RelatedCache was NOT cleared, so the right column was not "+
			"reset to loading state. handleActionRefresh must clear the cache entry AND "+
			"reset the right column before dispatching fresh checkers.\n"+
			"View:\n%s", viewAfter)
	}
}
