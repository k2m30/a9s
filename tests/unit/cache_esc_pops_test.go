package unit

// cache_esc_pops_test.go — Regression test for the cache-poisoning bug where
// related-navigation lists (EscPops=true, ParentContext=nil) overwrite the
// top-level resource cache when they receive a ResourcesLoadedMsg.
//
// Bug location: internal/tui/app.go
//   Line 246: `if rl.ParentContext() == nil {`             → needs `&& !rl.EscPops()`
//   Line 454: `if rl.ParentContext() != nil { return }`   → needs `|| rl.EscPops()`
//
// Related-navigation lists call SetEscPops(true) but leave ParentContext=nil.
// The cache-write guards only check ParentContext, so related lists slip through
// and overwrite the full top-level cache with their filtered subset.
//
// This test FAILS with current code (guard missing) and must pass after fix.

import (
	"context"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
)

// TestCachePoison_RelatedNavigate_DoesNotOverwriteTopLevelCache verifies that
// a related-navigation resource list (EscPops=true, ParentContext=nil) does NOT
// overwrite the top-level resource cache when it receives a ResourcesLoadedMsg.
//
// Scenario:
//  1. Load full EC2 list (all demo EC2 instances).
//  2. Enter detail view of the first EC2 instance.
//  3. Send RelatedNavigateMsg (no IDs → fallback path) → pushes related EC2 list
//     with EscPops=true, ParentContext=nil.
//  4. Load a PARTIAL EC2 list (1 resource) into the related view to simulate
//     a poisoning write to the cache.
//  5. Press Escape → pops the related list (EscPops path).
//  6. Press Escape → pops the detail view → back to top-level EC2 list.
//  7. Press Escape → pops the EC2 list → back to main menu.
//  8. Navigate to EC2 list again via NavigateMsg → restores from cache.
//  9. Assert: the restored EC2 list shows the FULL count, not the poisoned count of 1.
//
// With the bug present (lines 246/454 missing EscPops guard), step 4 overwrites
// cache["ec2"] with 1 resource, so step 8 restores a list of 1 instead of 5.
func TestCachePoison_RelatedNavigate_DoesNotOverwriteTopLevelCache(t *testing.T) {
	// ── Step 1: Set up model and load full EC2 list ──────────────────────────

	m := tui.New("demo", "us-east-1", tui.WithDemo(true))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	clients := demo.NewServiceClients()
	ec2Res, err := awsclient.FetchEC2Instances(context.Background(), clients.EC2)
	if err != nil || len(ec2Res) < 2 {
		t.Fatalf("demo ec2 fixtures need at least 2 resources for cache-poisoning test: err=%v len=%d", err, len(ec2Res))
	}
	fullCount := len(ec2Res)

	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "ec2",
		Resources:    ec2Res,
	})

	// ── Step 2: Enter detail view of the first EC2 instance ─────────────────

	m, enterCmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))
	// Drain the cmd chain so the detail view is fully pushed and
	// RelatedCheckStartedMsg (if any) is consumed.
	m, _ = drainCmds(t, m, enterCmd, 5)

	// ── Step 3: Send RelatedNavigateMsg — fallback (no IDs) path ────────────
	// The fallback calls newRelatedList which sets EscPops=true, ParentContext=nil.
	// This simulates a user pressing a related-navigation key (e.g., from the
	// right column) without a specific target ID.

	navMsg := messages.RelatedNavigateMsg{
		TargetType: "ec2",
		SourceResource: resource.Resource{
			ID:   "vpc-demo-001",
			Name: "demo-vpc",
		},
		SourceType: "vpc",
		// No TargetID, no RelatedIDs → fallback path in handleRelatedNavigate.
	}
	m, _ = rootApplyMsg(m, navMsg)

	// ── Step 4: Load a PARTIAL EC2 list into the related view ───────────────
	// This is the poisoning step: only 1 resource, but the active view is the
	// related list (EscPops=true, ParentContext=nil).
	// With the bug, line 246 sees ParentContext()==nil and overwrites cache["ec2"].

	partialList := ec2Res[0:1] // only the first EC2 instance
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "ec2",
		Resources:    partialList,
	})

	// ── Step 5: Press Escape → pops the related list (EscPops path) ─────────

	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEscape))

	// ── Step 6: Press Escape → pops the detail view ─────────────────────────

	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEscape))

	// ── Step 7: Press Escape → pops the top-level EC2 list → main menu ──────
	// This ensures the EC2 list is no longer the active view, so navigating
	// back will pull from cache (not from in-memory list state).

	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEscape))

	// ── Step 8: Navigate back to EC2 list → restores from cache ─────────────

	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})

	// ── Step 9: Assert the EC2 list shows the FULL count ────────────────────

	view := stripANSI(rootViewContent(m))

	// The frame title includes the resource count prefix, e.g. "ec2(5" or "ec2(5/N issues)".
	// With the bug, it would show "ec2(1" (the poisoned count).
	expectedCountStr := fmt.Sprintf("(%d", fullCount)
	poisonedCountStr := fmt.Sprintf("(%d", len(partialList))

	// The poisoned count prefix must not appear unless it also matches the full count prefix
	// (they could coincide when fullCount == len(partialList), but that never happens here).
	if strings.Contains(view, poisonedCountStr) && !strings.Contains(view, expectedCountStr) {
		t.Fatalf(
			"BUG: cache was poisoned by related-navigation list — EC2 list shows count %s "+
				"instead of full count %s.\n"+
				"Fix: add `&& !rl.EscPops()` to the guard at app.go:246, "+
				"and `|| rl.EscPops()` to the guard at app.go:454.\nView:\n%s",
			poisonedCountStr, expectedCountStr, view,
		)
	}

	if !strings.Contains(view, expectedCountStr) {
		t.Fatalf(
			"EC2 list after cache restore must show full count %s but got unexpected view.\n"+
				"full count=%d, poisoned count=%d\nView:\n%s",
			expectedCountStr, fullCount, len(partialList), view,
		)
	}
}
