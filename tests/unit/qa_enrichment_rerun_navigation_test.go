package unit

// The Ctrl+R enrichment rerun must
// fire even when the user navigates away before the fetch returns.
//
// The TypeGen tail that fires probeEnrichment after a Ctrl+R-wrapped
// ResourcesLoadedMsg runs unconditionally of the active view, whenever
// msg.TypeGen != 0 && msg.TypeGen == enrichmentTypeGen[T]; a tail nested
// inside an activeView().(*views.ResourceListModel) assertion is skipped
// once the user is back on the main menu, and findings stay cleared forever.

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
)

// ─────────────────────────────────────────────────────────────────────────────
// Ctrl+R + navigate away before fetch returns
// ─────────────────────────────────────────────────────────────────────────────

// TestListCtrlR_RerunDispatchedEvenAfterNavigatingAway verifies that when the
// user presses Ctrl+R on the EBS Volumes list and navigates back to the main
// menu before the fetch result arrives, the incoming
// ResourcesLoadedMsg{TypeGen=1} still dispatches probeEnrichment. ebs is
// registered in both EnricherRegistry and buildEnrichQueue's order list.
func TestListCtrlR_RerunDispatchedEvenAfterNavigatingAway(t *testing.T) {
	oldVersion := tui.Version
	tui.Version = "test"
	t.Cleanup(func() { tui.Version = oldVersion })
	m := newRootSizedModel()

	m = navigateToEBSList(m)

	// Ctrl+R bumps enrichmentTypeGen["ebs"] from 0 to 1.
	m, wrappedFetchCmd := rootApplyMsg(m, ctrlRKeyMsg())
	if wrappedFetchCmd == nil {
		t.Fatal("Ctrl+R on top-level EBS list must return a non-nil cmd (wrapped fetch)")
	}

	m, _ = rootApplyMsg(m, messages.PopView{})

	plain := stripANSI(rootViewContent(m))
	if !containsAny(plain, "resource-types", "EBS", "Volumes") {
		t.Logf("after PopViewMsg, view: %s", plain[:min(200, len(plain))])
	}

	// Delivered directly: executing the wrapped cmd fails on nil clients.
	loadedMsg := messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
		ResourceType: "ebs",
		Resources:    rerunEBSResources(),
		TypeGen:      1, // matches enrichmentTypeGen["ebs"]=1 set during Ctrl+R
	}

	// The active view is MainMenuModel here.
	m, probeCmd := rootApplyMsg(m, loadedMsg)

	if probeCmd == nil {
		t.Error("ResourcesLoadedMsg{TypeGen=1} must dispatch probeEnrichment even when " +
			"the user navigated away from the EBS list before the fetch returned. " +
			"Pre-fix: probeCmd is nil because the tail branch is nested inside the " +
			"active-ResourceListModel check which fails when active view is MainMenuModel.")
	}

	// With nil clients the enricher returns an error; only the dispatch matters.
	if probeCmd != nil {
		msg := probeCmd()
		switch msg.(type) {
		case messages.EnrichmentChecked:
		default:
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
