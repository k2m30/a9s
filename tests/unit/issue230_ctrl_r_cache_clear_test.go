package unit

// issue230_ctrl_r_cache_clear_test.go — Specification-driven regression tests for
// the dead Ctrl+R handler in detail.go:195-209 (issue #230).
//
// Business rules verified:
//  1. Ctrl+R from the detail view MUST clear the relatedCache entry for the current
//     resource (not just re-dispatch checkers on top of stale cached state).
//  2. Immediately after Ctrl+R, before any RelatedCheckResultMsg is processed,
//     the right column MUST NOT show the previously-cached count suffixes like "(2)".
//  3. Ctrl+R MUST produce at least one RelatedCheckStartedMsg in the cmd chain.
//
// All three tests use the actual Ctrl+R key press (via ctrlR() defined in
// bug_detail_refresh_resets_rightcol_test.go) — NOT a direct RelatedCheckStartedMsg
// injection — so they exercise the real handler path.
//
// Tests 2 and 3 are the key regression guards: if someone later re-routes Ctrl+R
// to the dead detail.go handler (which lacks cache clearing), Test 2 catches the
// stale-data regression and Test 3 catches the missing dispatch.

import (
	"fmt"
	"strings"
	"testing"
)

// TestContract_CtrlR_ClearsRelatedCache_ThenRechecks is the full integration test
// for issue #230. It verifies that Ctrl+R:
//
//	(a) clears the stale cached data so the right column shows loading state, and
//	(b) re-dispatches relatedcheckers (RelatedCheckStartedMsg appears in the chain).
//
// This test PASSES with current code (the LIVE path in app_handlers.go is correct).
// It is a regression guard to ensure the live path continues to work after the
// dead code in detail.go:195-209 is removed.
func TestContract_CtrlR_ClearsRelatedCache_ThenRechecks(t *testing.T) {
	m := setupEC2DetailWithResults(t)

	// Precondition: cached results are visible — "(2)" must appear.
	viewBefore := stripANSI(rootViewContent(m))
	if !strings.Contains(viewBefore, "(2)") {
		t.Fatalf("precondition failed: expected '(2)' in view before Ctrl+R to confirm "+
			"related counts are loaded and visible.\nView:\n%s", viewBefore)
	}

	// Send the actual Ctrl+R key — exercises the LIVE global handler path.
	m, refreshCmd := rootApplyMsg(m, ctrlR())

	// Drain exactly one level to process any immediate cmd (e.g., RelatedCheckStartedMsg).
	// We stop before feeding checker results back to keep the right column in loading state.
	var immediateMsg any
	if refreshCmd != nil {
		msg := refreshCmd()
		if msg != nil {
			immediateMsg = msg
			m, _ = rootApplyMsg(m, msg)
		}
	}

	// (a) Right column must be in loading state — stale "(2)" must be gone.
	viewAfter := stripANSI(rootViewContent(m))
	if strings.Contains(viewAfter, "(2)") {
		t.Fatalf("contract violated: after Ctrl+R the right column still shows stale '(2)' "+
			"— the relatedCache entry must be cleared before checkers re-run so stale data "+
			"is never shown during a refresh.\nView:\n%s", viewAfter)
	}

	// (b) RelatedCheckStartedMsg must have appeared in the chain.
	// It is either the immediate cmd result or needs another drain level.
	if immediateMsg == nil {
		t.Fatal("contract violated: Ctrl+R produced no cmd — " +
			"RelatedCheckStartedMsg must be dispatched to re-run checkers")
	}

	// Walk the cmd chain up to 5 levels to find RelatedCheckStartedMsg.
	found := false
	m2 := m
	cmd := refreshCmd
	for i := 0; i < 5 && cmd != nil; i++ {
		msg := cmd()
		if msg == nil {
			break
		}
		if _, ok := msg.(interface{ isRelatedCheckStarted() }); ok {
			found = true
			break
		}
		// Use fmt.Sprintf for type check without importing messages package.
		typeName := fmt.Sprintf("%T", msg)
		if typeName == "messages.RelatedCheckStartedMsg" {
			found = true
			break
		}
		m2, cmd = rootApplyMsg(m2, msg)
	}
	_ = m2

	if !found {
		// Re-drain from scratch with drainCmds for a cleaner check.
		m3 := setupEC2DetailWithResults(t)
		m3, refreshCmd3 := rootApplyMsg(m3, ctrlR())
		_, chainMsgs := drainCmds(t, m3, refreshCmd3, 10)
		for _, msg := range chainMsgs {
			if fmt.Sprintf("%T", msg) == "messages.RelatedCheckStartedMsg" {
				found = true
				break
			}
		}
	}

	if !found {
		t.Fatal("contract violated: Ctrl+R did not produce RelatedCheckStartedMsg " +
			"in the cmd chain — checkers must be re-dispatched after a refresh")
	}
}

// TestContract_CtrlR_FromDetail_ProducesRelatedCheckStarted pins the basic contract:
// pressing Ctrl+R while in a detail view MUST produce RelatedCheckStartedMsg
// somewhere in the cmd chain.
//
// This test PASSES with current code and serves as a simple regression pin.
func TestContract_CtrlR_FromDetail_ProducesRelatedCheckStarted(t *testing.T) {
	m := setupEC2DetailWithResults(t)

	// Send the actual Ctrl+R key press.
	m, refreshCmd := rootApplyMsg(m, ctrlR())

	// Drain the full cmd chain (up to 10 levels) and collect all messages.
	_, chainMsgs := drainCmds(t, m, refreshCmd, 10)

	found := false
	var msgTypes []string
	for _, msg := range chainMsgs {
		typeName := fmt.Sprintf("%T", msg)
		msgTypes = append(msgTypes, typeName)
		if typeName == "messages.RelatedCheckStartedMsg" {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("contract violated: Ctrl+R from detail view must produce "+
			"RelatedCheckStartedMsg in the cmd chain to re-run related checkers.\n"+
			"Messages produced: %v", msgTypes)
	}
}

// TestContract_CtrlR_RightColumnResets_BeforeRecheck is the key regression guard
// for the stale-data bug described in issue #230.
//
// If Ctrl+R is re-routed to the dead detail.go handler (which dispatches checkers
// but does NOT clear the relatedCache), this test catches it: the right column
// would still show old "(2)" counts instead of a loading state.
//
// Business rule: after Ctrl+R, before any new RelatedCheckResultMsg is delivered,
// the right column MUST NOT show the previously-loaded count values.
//
// This test PASSES with current code (the LIVE path calls ResetRightColumn via
// handleRefresh). It is the primary regression guard for issue #230.
func TestContract_CtrlR_RightColumnResets_BeforeRecheck(t *testing.T) {
	m := setupEC2DetailWithResults(t)

	// Precondition: right column shows loaded counts.
	viewBefore := stripANSI(rootViewContent(m))
	if !strings.Contains(viewBefore, "(2)") {
		t.Fatalf("precondition failed: expected '(2)' in right column before Ctrl+R.\n"+
			"View:\n%s", viewBefore)
	}

	// Press Ctrl+R — exercises the real key path.
	m, refreshCmd := rootApplyMsg(m, ctrlR())

	// Drain exactly ONE level of the cmd chain (to process RelatedCheckStartedMsg if
	// it is the immediate result). We intentionally do NOT feed RelatedCheckResultMsg
	// back in — that would populate the right column again and mask the bug.
	if refreshCmd != nil {
		msg := refreshCmd()
		if msg != nil {
			m, _ = rootApplyMsg(m, msg)
		}
	}

	// The right column must be in loading/empty state — no stale counts visible.
	viewAfter := stripANSI(rootViewContent(m))
	if strings.Contains(viewAfter, "(2)") {
		t.Fatalf("contract violated (issue #230 regression): after Ctrl+R the right column "+
			"still shows stale '(2)' count suffixes before any new checker results arrive.\n"+
			"This indicates the relatedCache was NOT cleared, so the right column was not "+
			"reset to loading state. The LIVE path in app_handlers.go must clear the cache "+
			"entry AND call ResetRightColumn before dispatching fresh checkers.\n"+
			"View:\n%s", viewAfter)
	}
}
