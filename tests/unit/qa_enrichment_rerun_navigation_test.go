package unit

// qa_enrichment_rerun_navigation_test.go — RED test for Bug 2: Ctrl+R rerun skipped
// when user navigates away before the fetch returns.
//
// Bug: The tail branch in app.go (lines 452-482) that fires probeEnrichment after a
// Ctrl+R-wrapped ResourcesLoadedMsg is nested inside:
//
//	if rl, ok := updatedModel.activeView().(*views.ResourceListModel); ok {
//	    if rl.ParentContext() == nil && !rl.EscPops() {
//	        // ... tail branch here
//	    }
//	}
//
// If the user navigates away (e.g., back to main menu) before the fetch returns,
// activeView() is no longer a *ResourceListModel, the outer type-assert fails,
// and the entire block — including the TypeGen tail — is skipped. The enrichment
// rerun is never dispatched and findings stay cleared forever.
//
// Demanded behavior (post-fix): the TypeGen tail must run unconditionally of the
// active view, whenever msg.TypeGen != 0 && msg.TypeGen == enrichmentTypeGen[T].
//
// Test T067:
//   T067 — Ctrl+R + navigate away: probeEnrichment must still fire when wrapped
//           fetch result arrives after the user has left the EC2 list.

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
)

// ─────────────────────────────────────────────────────────────────────────────
// T067 — Ctrl+R + navigate away before fetch returns
// ─────────────────────────────────────────────────────────────────────────────

// TestListCtrlR_RerunDispatchedEvenAfterNavigatingAway verifies FR-014 edge case:
// when the user presses Ctrl+R on EC2 list and then navigates back to the main menu
// BEFORE the fetch result arrives, the incoming ResourcesLoadedMsg{TypeGen=1} must
// still dispatch probeEnrichment.
//
// Pre-fix: The tail branch is inside the active-ResourceListModel check, so when
// the active view is MainMenuModel (after navigating away), the branch is never
// reached. The returned cmd is nil — probeEnrichment is never dispatched.
// Findings stay cleared and are never refreshed by the rerun.
//
// Post-fix: The tail branch runs regardless of active view whenever the TypeGen
// token matches. The returned cmd is non-nil (probeEnrichment was dispatched).
func TestListCtrlR_RerunDispatchedEvenAfterNavigatingAway(t *testing.T) {
	tui.Version = "test"
	m := newRootSizedModel()

	// Step 1: Navigate to EC2 list.
	m = navigateToEC2List(m)

	// Step 2: Press Ctrl+R — bumps enrichmentTypeGen["ec2"] from 0 to 1.
	// We capture the wrapped fetch cmd but do NOT execute it yet.
	m, wrappedFetchCmd := rootApplyMsg(m, ctrlRKeyMsg())
	if wrappedFetchCmd == nil {
		t.Fatal("Ctrl+R on top-level EC2 list must return a non-nil cmd (wrapped fetch)")
	}

	// Step 3: Navigate back to the main menu BEFORE the fetch returns.
	// This simulates the user pressing Esc (or any key that pops the EC2 list).
	m, _ = rootApplyMsg(m, messages.PopViewMsg{})

	// Confirm we're back at the main menu (sanity check).
	plain := stripANSI(rootViewContent(m))
	if !containsAny(plain, "resource-types", "EC2") {
		t.Logf("after PopViewMsg, view: %s", plain[:min(200, len(plain))])
		// Not fatal — continue with the test regardless.
	}

	// Step 4: The wrapped fetch cmd now returns a ResourcesLoadedMsg with TypeGen=1.
	// We simulate this by delivering the message directly rather than executing the
	// cmd (which would fail due to nil clients). We use TypeGen=1 matching the gen
	// that was bumped at Ctrl+R time.
	loadedMsg := messages.ResourcesLoadedMsg{
		ResourceType: "ec2",
		Resources:    rerunEC2Resources(),
		TypeGen:      1, // matches enrichmentTypeGen["ec2"]=1 set during Ctrl+R
	}

	// Step 5: Deliver the ResourcesLoadedMsg to the model.
	// Active view at this point is MainMenuModel (not ResourceListModel).
	m, probeCmd := rootApplyMsg(m, loadedMsg)

	// ASSERTION: probeCmd must be non-nil — probeEnrichment must have been dispatched
	// even though the active view is now MainMenuModel.
	//
	// Pre-fix: probeCmd is nil because the tail branch is only reached when
	// activeView() is a *ResourceListModel. Since the user navigated away,
	// activeView() is MainMenuModel, the type-assert fails, and the tail never runs.
	//
	// Post-fix: the tail branch is moved outside the active-view check and fires
	// unconditionally when msg.TypeGen != 0 && msg.TypeGen == enrichmentTypeGen[T].
	if probeCmd == nil {
		t.Error("ResourcesLoadedMsg{TypeGen=1} must dispatch probeEnrichment even when " +
			"the user navigated away from the EC2 list before the fetch returned. " +
			"Pre-fix: probeCmd is nil because the tail branch is nested inside the " +
			"active-ResourceListModel check which fails when active view is MainMenuModel.")
	}

	// Step 6: If probeCmd is non-nil (post-fix), execute it and verify it does not
	// panic. With nil clients the enricher will return an error, which is acceptable
	// — we only need to confirm the cmd was dispatched (the branch ran).
	if probeCmd != nil {
		msg := probeCmd()
		switch msg.(type) {
		case messages.EnrichmentCheckedMsg:
			// Expected: probe fired, returned EnrichmentCheckedMsg (likely with Err != nil
			// due to nil clients, but the dispatch itself occurred).
		default:
			// BatchMsg or other — also acceptable; the dispatch ran.
		}
	}

	_ = m
}

// containsAny returns true if s contains any of the provided substrings.
func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if len(s) > 0 && len(sub) > 0 {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}
