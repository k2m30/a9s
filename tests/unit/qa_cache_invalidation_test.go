package unit

// qa_cache_invalidation_test.go — TDD tests for resource cache invalidation.
//
// These tests verify that the resource cache is cleared on profile/region
// switch, selectively cleared on refresh, and correctly updated when
// additional pages are loaded after re-entering a cached list.
//
// All tests are EXPECTED TO FAIL until the resource cache feature is
// implemented in the root model (issue #111).

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
)

// ---------------------------------------------------------------------------
// TestQA_CacheInvalidation_ProfileSwitchClearsAll
// ---------------------------------------------------------------------------

// TestQA_CacheInvalidation_ProfileSwitchClearsAll verifies that sending a
// ProfileSelectedMsg clears the entire resource cache synchronously.
// After the profile switch, navigating to a previously-loaded resource type
// must issue a fresh fetch (non-nil cmd) rather than serving the cached state.
func TestQA_CacheInvalidation_ProfileSwitchClearsAll(t *testing.T) {
	tui.Version = "test"
	m := newRootSizedModel()

	// Step 1: navigate to ct-events and load data so the cache is populated.
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ct-events",
	})
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "ct-events",
		Resources:    ctEventsResources(50),
		Pagination: &resource.PaginationMeta{
			IsTruncated: false,
			PageSize:    50,
			TotalHint:   -1,
		},
		Append: false,
	})

	// Confirm the data is present before Esc.
	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "ct-events(50") {
		t.Fatalf("precondition: expected 'ct-events(50...)' before profile switch, got:\n%s", plain)
	}

	// Step 2: press Esc back to main menu.
	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEscape))

	// Step 3: switch profile — this must clear the cache.
	m, _ = rootApplyMsg(m, messages.ProfileSelected{Profile: "other-profile"})

	// Step 4: navigate to ct-events again.
	// KEY ASSERTION: a fresh fetch must be issued because the cache was cleared.
	_, cmd := rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ct-events",
	})

	if cmd == nil {
		t.Errorf("after ProfileSelectedMsg, navigating to ct-events should issue a fetch (cache cleared), but cmd was nil")
	}
}

// ---------------------------------------------------------------------------
// TestQA_CacheInvalidation_RegionSwitchClearsAll
// ---------------------------------------------------------------------------

// TestQA_CacheInvalidation_RegionSwitchClearsAll verifies that sending a
// RegionSelectedMsg clears the entire resource cache synchronously.
// After the region switch, navigating to a previously-loaded resource type
// must issue a fresh fetch (non-nil cmd) rather than serving the cached state.
func TestQA_CacheInvalidation_RegionSwitchClearsAll(t *testing.T) {
	tui.Version = "test"
	m := newRootSizedModel()

	// Step 1: navigate to ct-events and load data.
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ct-events",
	})
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "ct-events",
		Resources:    ctEventsResources(50),
		Pagination: &resource.PaginationMeta{
			IsTruncated: false,
			PageSize:    50,
			TotalHint:   -1,
		},
		Append: false,
	})

	// Confirm data is present.
	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "ct-events(50") {
		t.Fatalf("precondition: expected 'ct-events(50...)' before region switch, got:\n%s", plain)
	}

	// Step 2: press Esc back to main menu.
	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEscape))

	// Step 3: switch region — this must clear the cache.
	m, _ = rootApplyMsg(m, messages.RegionSelected{Region: "eu-west-1"})

	// Step 4: navigate to ct-events again.
	// KEY ASSERTION: a fresh fetch must be issued because the cache was cleared.
	_, cmd := rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ct-events",
	})

	if cmd == nil {
		t.Errorf("after RegionSelectedMsg, navigating to ct-events should issue a fetch (cache cleared), but cmd was nil")
	}
}

// ---------------------------------------------------------------------------
// TestQA_CacheInvalidation_RefreshClearsCurrentTypeOnly
// ---------------------------------------------------------------------------

// TestQA_CacheInvalidation_RefreshClearsCurrentTypeOnly verifies that pressing
// Ctrl+R while viewing a resource list clears only that resource type's cache
// slot and issues a fresh fetch for it, while other resource types' caches
// remain intact.
func TestQA_CacheInvalidation_RefreshClearsCurrentTypeOnly(t *testing.T) {
	tui.Version = "test"
	m := newRootSizedModel()

	// Step 1: Load ct-events (50 items), press Esc.
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ct-events",
	})
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "ct-events",
		Resources:    ctEventsResources(50),
		Pagination: &resource.PaginationMeta{
			IsTruncated: false,
			PageSize:    50,
			TotalHint:   -1,
		},
		Append: false,
	})
	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEscape))

	// Step 2: Load ec2 (30 items), press Esc.
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "ec2",
		Resources:    ec2TestResources(30),
		Pagination: &resource.PaginationMeta{
			IsTruncated: false,
			PageSize:    30,
			TotalHint:   -1,
		},
		Append: false,
	})
	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEscape))

	// Step 3: Re-enter ct-events — must be a cache hit (cmd == nil).
	m, ctCmd := rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ct-events",
	})
	if ctCmd != nil {
		t.Fatalf("precondition: re-entering ct-events should be a cache hit (cmd == nil), but got a non-nil cmd — cache not yet implemented")
	}

	// Step 4: Press Ctrl+R (code 0x12) to refresh ct-events.
	// This must clear ct-events from the cache and issue a fresh fetch.
	_, refreshCmd := rootApplyMsg(m, rootSpecialKey(0x12))
	if refreshCmd == nil {
		t.Errorf("Ctrl+R on ct-events should issue a fresh fetch (non-nil cmd), but got nil")
	}

	// Step 5: Press Esc back to main menu, then re-enter ec2.
	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEscape))
	_, ec2Cmd := rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})

	// KEY ASSERTION: ec2 cache must still be intact — no fresh fetch.
	if ec2Cmd != nil {
		t.Errorf("after refreshing ct-events only, ec2 should still be a cache hit (cmd == nil), but got non-nil cmd")
	}
}

// ---------------------------------------------------------------------------
// TestQA_CacheInvalidation_CacheUpdatesOnAdditionalPage
// ---------------------------------------------------------------------------

// TestQA_CacheInvalidation_CacheUpdatesOnAdditionalPage verifies that after
// loading page 2 via the M key, pressing Esc and re-entering the same resource
// type shows the combined 100 resources from both pages (the cache must be
// updated when additional pages arrive, not just on the initial load).
func TestQA_CacheInvalidation_CacheUpdatesOnAdditionalPage(t *testing.T) {
	tui.Version = "test"
	m := newRootSizedModel()

	// Step 1: Navigate to ct-events, load page 1 (50 items, truncated).
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ct-events",
	})
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "ct-events",
		Resources:    ctEventsResources(50),
		Pagination: &resource.PaginationMeta{
			IsTruncated: true,
			NextToken:   "page2-token",
			PageSize:    50,
			TotalHint:   -1,
		},
		Append: false,
	})

	// Confirm truncated indicator is shown.
	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "50+") {
		t.Fatalf("precondition: expected '50+' after page 1, got:\n%s", plain)
	}

	// Step 2: Press Esc — cache stores 50 items, truncated.
	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEscape))

	// Step 3: Re-enter ct-events — must be a cache hit (cmd == nil) with 50 items.
	m, ctCmd1 := rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ct-events",
	})
	if ctCmd1 != nil {
		t.Fatalf("precondition: re-entering ct-events (page 1) should be a cache hit (cmd == nil), but got non-nil cmd — cache not yet implemented")
	}
	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "50+") {
		t.Fatalf("precondition: expected '50+' after cache hit, got:\n%s", plain)
	}

	// Step 4: Press M to load more, then deliver page 2 (50 more items, final).
	m, _ = rootApplyMsg(m, rootKeyPress("M"))
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "ct-events",
		Resources:    ctEventsResources2(50, 50),
		Pagination: &resource.PaginationMeta{
			IsTruncated: false,
			NextToken:   "",
			PageSize:    50,
			TotalHint:   -1,
		},
		Append: true,
	})

	// Confirm 100 items are shown.
	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "ct-events(100") {
		t.Fatalf("precondition: expected 'ct-events(100...)' after page 2, got:\n%s", plain)
	}

	// Step 5: Press Esc — cache must now store 100 combined items.
	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEscape))

	// Step 6: Re-enter ct-events — cache hit must show 100 items.
	m, ctCmd2 := rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ct-events",
	})

	// KEY ASSERTION: still a cache hit (no fresh fetch).
	if ctCmd2 != nil {
		t.Errorf("re-entering ct-events after loading page 2 should still be a cache hit (cmd == nil), but got non-nil cmd")
	}

	plain = stripANSI(rootViewContent(m))

	// KEY ASSERTION: must show 100 combined items, not just 50.
	if !strings.Contains(plain, "ct-events(100") {
		t.Errorf("after re-entering ct-events, expected 'ct-events(100...)' (both pages cached), got:\n%s", plain)
	}

	// Verify first-page items are still present after the cache update.
	// We check for "usr-0000" which is the _ct.actor value rendered in the ACTOR column
	// for the first ctEventsResources() entry (the ID "evt-0000" is not rendered in any column).
	if !strings.Contains(plain, "usr-0000") {
		t.Errorf("after cache hit with 2 pages, first-page resource actor 'usr-0000' should be visible in ACTOR column")
	}
}
