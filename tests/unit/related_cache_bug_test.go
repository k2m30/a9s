package unit

// related_cache_bug_test.go — Tests revealing the related-check cache bug.
//
// Bug: When a user re-enters the same EC2 detail view after pressing Esc,
// the root model creates a fresh DetailModel (line 406 app_handlers.go),
// which always returns NeedsRelatedCheck()==true, causing all related checkers
// to re-dispatch from scratch instead of using cached results from the first visit.
//
// Tests 1 and 2 FAIL with current code (they reveal the bug).
// Tests 3, 4, and 5 PASS with current code (they verify correct existing behavior).

import (
	"context"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/demo/fakes"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
)

// drainCmds executes the cmd chain to completion (up to maxDepth) and returns
// the model after all produced messages have been processed.
// Returns the final model plus the list of all messages that were produced.
//
// Batch-aware (mirrors the extractEnrichmentChecked idiom in
// qa_enrich_pipeline_dispatch_test.go): detail-open/refresh for an enrichable
// type now returns a tea.Batch (related-check cmd + enrich cmd), and the real
// Bubble Tea runtime executes every batch member and delivers each leaf
// message to Update independently — a walker that treated tea.BatchMsg as an
// opaque leaf would simulate that unfaithfully. Cmds are processed via a
// queue rather than a single linear chain so a batch's members (and whatever
// cmds they each go on to produce) are all drained.
//
// maxDepth bounds APPLIED messages only (the pre-batching meaning every
// caller's budget was tuned to) — a nil cmd, a nil msg, or unwrapping a
// tea.BatchMsg into its members is free and does not consume the budget.
// Only a leaf message that is actually applied to the model via rootApplyMsg
// counts toward maxDepth.
func drainCmds(t *testing.T, m tui.Model, cmd tea.Cmd, maxDepth int) (tui.Model, []tea.Msg) {
	t.Helper()
	var allMsgs []tea.Msg
	pending := []tea.Cmd{cmd}
	for applied := 0; applied < maxDepth && len(pending) > 0; {
		cur := pending[0]
		pending = pending[1:]
		if cur == nil {
			continue
		}
		msg := cur()
		if msg == nil {
			continue
		}
		if batch, ok := msg.(tea.BatchMsg); ok {
			pending = append(pending, batch...)
			continue
		}
		allMsgs = append(allMsgs, msg)
		applied++
		var next tea.Cmd
		m, next = rootApplyMsg(m, msg)
		if next != nil {
			pending = append(pending, next)
		}
	}
	return m, allMsgs
}

// extractLeafMsgs executes cmd (recursing through nested tea.BatchMsg to any
// depth) and returns every leaf tea.Msg produced. Mirrors extractEnrichmentChecked
// (qa_enrich_pipeline_dispatch_test.go) but is msg-type-agnostic; unlike
// drainCmds it does not follow cmds returned by applying those leaf messages
// to a model — it only flattens the one cmd (and any nested batch) passed in.
func extractLeafMsgs(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if msg == nil {
		return nil
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		var out []tea.Msg
		for _, sub := range batch {
			out = append(out, extractLeafMsgs(sub)...)
		}
		return out
	}
	return []tea.Msg{msg}
}

// applyImmediateCmd executes cmd once (batch-aware via extractLeafMsgs) and
// applies every leaf message it produces to m, in order. It does NOT follow
// any cmd returned by applying those leaf messages — this is a single
// "immediate" step, not a full drain (see drainCmds for that) — used by
// callers that intentionally stop before feeding async checker results back
// in, so intermediate (e.g. loading) state can be asserted.
func applyImmediateCmd(t *testing.T, m tui.Model, cmd tea.Cmd) (tui.Model, []tea.Msg) {
	t.Helper()
	leaves := extractLeafMsgs(cmd)
	for _, msg := range leaves {
		m, _ = rootApplyMsg(m, msg)
	}
	return m, leaves
}

// setupEC2DetailWithResults is a shared helper that:
//  1. Creates a demo root model at 120×36.
//  2. Navigates to the EC2 resource list.
//  3. Presses Enter on the first EC2 instance (detail view).
//  4. Feeds RelatedCheckResultMsg for every registered EC2 related type (Count=2).
//  5. Returns the model ready for the Esc+re-enter phase.
func setupEC2DetailWithResults(t *testing.T) tui.Model {
	t.Helper()

	m := tui.New("demo", "us-east-1",
		tui.WithClients(demo.NewServiceClients()),
		tui.WithIsDemo(true),
		tui.WithNoCache(true),
		tui.WithProfileForTest(demo.DemoProfile),
		tui.WithRegionForTest(demo.DemoRegion))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	// Navigate to EC2 list.
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})

	ec2Client := fakes.NewEC2()
	ec2Res, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchEC2InstancesPage(context.Background(), ec2Client, token)
	})
	if err != nil || len(ec2Res) == 0 {
		t.Fatalf("demo ec2 fixtures missing (err=%v, len=%d)", err, len(ec2Res))
	}

	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "ec2",
		Resources:    ec2Res,
	})

	// First Enter → the cmd chain is:
	//   Enter key → ResourceListModel returns NavigateMsg cmd
	//   NavigateMsg → root creates DetailModel → returns RelatedCheckStartedMsg cmd
	//   RelatedCheckStartedMsg → root dispatches async checkers
	m, firstCmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))
	m, firstMsgs := drainCmds(t, m, firstCmd, 5)

	// Verify RelatedCheckStartedMsg was produced somewhere in the chain.
	foundRelatedCheck := false
	for _, msg := range firstMsgs {
		if _, ok := msg.(messages.RelatedCheckStarted); ok {
			foundRelatedCheck = true
			break
		}
	}
	if !foundRelatedCheck {
		types := make([]string, len(firstMsgs))
		for i, msg := range firstMsgs {
			types[i] = fmt.Sprintf("%T", msg)
		}
		t.Fatalf("first detail entry must produce RelatedCheckStartedMsg in cmd chain; got: %v", types)
	}

	// Feed results for all registered EC2 related types.
	for _, def := range resource.GetRelated("ec2") {
		m, _ = rootApplyMsg(m, messages.RelatedCheckResult{
			ResourceType: "ec2",
			Result: resource.RelatedCheckResult{
				TargetType:  def.TargetType,
				Count:       2,
				ResourceIDs: []string{"related-id-1", "related-id-2"},
			},
		})
	}

	return m
}

// TestBug_RelatedCheckResults_NotCachedOnReentry verifies that re-entering
// the same EC2 instance's detail view does NOT re-dispatch RelatedCheckStartedMsg.
//
// EXPECTED: cached results are reused; no second RelatedCheckStartedMsg.
// ACTUAL (BUG): fresh DetailModel is created, NeedsRelatedCheck()==true always,
// so RelatedCheckStartedMsg is emitted again.
//
// This test FAILS with current code.
func TestBug_RelatedCheckResults_NotCachedOnReentry(t *testing.T) {
	m := setupEC2DetailWithResults(t)

	// Esc → back to EC2 list.
	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEscape))

	// Re-enter the SAME EC2 instance (cursor is still on first item).
	// Drain the full cmd chain to see all messages produced.
	m, secondCmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))
	_, secondMsgs := drainCmds(t, m, secondCmd, 5)

	// EXPECTED: no RelatedCheckStartedMsg on re-entry (results are cached).
	// BUG: the root model always creates a fresh DetailModel, so it always re-emits.
	for _, msg := range secondMsgs {
		if _, ok := msg.(messages.RelatedCheckStarted); ok {
			t.Fatal("BUG: re-entering the same EC2 detail view should NOT re-dispatch " +
				"RelatedCheckStartedMsg — related check results must be cached from the first visit")
		}
	}
}

// TestBug_RelatedCheckResults_RightColShowsCachedCounts verifies that after
// re-entering the same EC2 instance's detail, the View() output immediately
// shows the cached related counts (e.g., "(2)") rather than a loading state.
//
// EXPECTED: right column shows "Target Groups (2)" (or similar) immediately.
// ACTUAL (BUG): right column shows loading state because all checkers re-run.
//
// This test FAILS with current code.
func TestBug_RelatedCheckResults_RightColShowsCachedCounts(t *testing.T) {
	m := setupEC2DetailWithResults(t)

	// Esc → back to EC2 list.
	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEscape))

	// Re-enter the SAME EC2 instance.
	// Use applyRootAndCmd to advance through the Navigate chain into detail.
	m = applyRootAndCmd(t, m, rootSpecialKey(tea.KeyEnter))

	view := stripANSI(rootViewContent(m))

	// EXPECTED: at least one related type shows its cached count of 2.
	// BUG: no "(2)" appears because all checkers were re-dispatched and results are pending.
	if !strings.Contains(view, "(2)") {
		t.Fatalf("BUG: re-entering detail should show cached related counts immediately; "+
			"expected '(2)' in view output.\nView:\n%s", view)
	}
}

// TestBug_RelatedCheckCache_DifferentResource_ShouldRecheck verifies that
// entering a DIFFERENT EC2 instance's detail view DOES trigger fresh checks.
// This is a cache-miss scenario and must always produce RelatedCheckStartedMsg.
//
// This test PASSES with current code (correct existing behavior).
func TestBug_RelatedCheckCache_DifferentResource_ShouldRecheck(t *testing.T) {
	m := setupEC2DetailWithResults(t)

	// Esc → back to EC2 list.
	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEscape))

	// Move cursor to the SECOND EC2 instance.
	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyDown))

	// Enter the second EC2 instance → drain cmd chain → must include RelatedCheckStartedMsg.
	m, secondCmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))
	_, secondMsgs := drainCmds(t, m, secondCmd, 5)

	foundRelatedCheck := false
	for _, msg := range secondMsgs {
		if _, ok := msg.(messages.RelatedCheckStarted); ok {
			foundRelatedCheck = true
			break
		}
	}
	if !foundRelatedCheck {
		t.Fatal("entering a DIFFERENT EC2 instance must dispatch RelatedCheckStartedMsg (cache miss)")
	}
}

// TestBug_RelatedCheckCache_InvalidatedOnProfileSwitch verifies that a profile
// switch invalidates any cached related-check results so that the next detail
// entry re-checks from scratch.
//
// Since there is no relatedCheckCache yet, this test PASSES with current code
// (no cache to invalidate). It documents the required invalidation contract for
// when the cache is implemented.
func TestBug_RelatedCheckCache_InvalidatedOnProfileSwitch(t *testing.T) {
	m := setupEC2DetailWithResults(t)

	// Esc → back to EC2 list.
	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEscape))

	// Simulate a profile switch. In demo mode, ProfileSelectedMsg is a no-op
	// (returns immediately). But we feed it to exercise the cache invalidation path.
	m, _ = rootApplyMsg(m, messages.ProfileSelected{Profile: "other-profile"})

	// Re-enter the first EC2 instance — drain cmd chain.
	m, secondCmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))
	_, secondMsgs := drainCmds(t, m, secondCmd, 5)

	// In demo mode, ProfileSelectedMsg returns (m, nil) so the EC2 list is still
	// active and Enter works normally. Verify RelatedCheckStartedMsg is produced.
	foundRelatedCheck := false
	for _, msg := range secondMsgs {
		if _, ok := msg.(messages.RelatedCheckStarted); ok {
			foundRelatedCheck = true
			break
		}
	}
	if len(secondMsgs) > 0 && !foundRelatedCheck {
		// After a profile switch, any cached related results are for the old profile
		// and must be invalidated.
		types := make([]string, len(secondMsgs))
		for i, msg := range secondMsgs {
			types[i] = fmt.Sprintf("%T", msg)
		}
		t.Fatalf("after profile switch, re-entering detail must dispatch "+
			"RelatedCheckStartedMsg (cache invalidated); got: %v", types)
	}
}
