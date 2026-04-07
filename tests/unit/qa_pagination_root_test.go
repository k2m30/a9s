package unit

// qa_pagination_root_test.go — TDD tests for pagination bugs at the root model level.
//
// Bug 1: Paginated fetcher returns IsTruncated=true but frame title shows "(50)"
//        instead of "(50+)". These tests verify the root model correctly passes
//        PaginationMeta from ResourcesLoadedMsg down to the active resource list view.
//
// Bug 2: After loading resources, pressing Esc, and re-entering the same resource
//        type, the model makes new API calls and shows only the first page again.
//        The desired behavior is to preserve the previously loaded resources and
//        not issue any new fetch commands.
//
// Bug 3 (probe truncation): probeResourceAvailability calls GetFetcher, which
//        returns []Resource with no truncation info. For ct-events, which has a
//        paginated fetcher, the probe should call GetPaginatedFetcher and use
//        FetchResult.IsTruncated to set AvailabilityCheckedMsg.Truncated=true.
//        The downstream wiring (handler→menu→view) already works correctly, so
//        a fix to the probe alone is sufficient.
//
// Tests 1–2 exercise currently-working view-layer wiring at the root level and
// should PASS immediately.
//
// Tests 3–5 document the desired cache behavior that does not yet exist and are
// SKIPPED via t.Skip() so they do not block CI. Remove the t.Skip() once the
// resource cache is implemented.
//
// Tests 6–7 (TestQA_MainMenu_TruncatedAvailabilityShowsPlus and
// TestQA_MainMenu_NonTruncatedAvailabilityNoPlus) verify the downstream
// rendering path for Bug 3 at the MainMenuModel view level. They confirm the
// wiring from SetAvailability+SetTruncated through to View() already works for
// ct-events, so the only fix needed is in probeResourceAvailability itself.

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ---------------------------------------------------------------------------
// Helpers: ct-events test resources
// ---------------------------------------------------------------------------

// ctEventsResources returns n ct-events resources with sequential IDs.
// Uses the new _ct.* field schema (Status is verb-based, not ReadOnly).
// CreateBucket → "Create" prefix → write verb → "ct-write".
// _ct.actor is set to "usr-NNNN" so it renders in the ACTOR column and can be
// used as a unique per-resource assertion target.
func ctEventsResources(n int) []resource.Resource {
	resources := make([]resource.Resource, n)
	for i := range n {
		id := fmt.Sprintf("evt-%04d", i)
		actor := fmt.Sprintf("usr-%04d", i)
		resources[i] = resource.Resource{
			ID:     id,
			Name:   fmt.Sprintf("CreateBucket-%d", i),
			Status: "ct-write",
			Fields: map[string]string{
				"event_name":    fmt.Sprintf("CreateBucket-%d", i),
				"time":          "2026-03-28 14:30:15",
				"event_time":    "2026-03-28 14:30:15",
				"user":          "admin",
				"source":        "s3.amazonaws.com",
				"resource_type": "",
				"resource_name": "",
				"read_only":     "false",
				// New _ct.* fields required by the redesigned list columns.
				"_ct.verb":    "W",
				"_ct.actor":   actor,
				"_ct.origin":  "CLI",
				"_ct.target":  "(none)",
				"_ct.outcome": "OK",
			},
		}
	}
	return resources
}

// ctEventsResources2 returns n additional ct-events resources whose IDs start
// at offset, so they can be distinguished from the first page.
// DeleteObject → "Delete" prefix → destructive verb → "ct-write".
func ctEventsResources2(n, offset int) []resource.Resource {
	resources := make([]resource.Resource, n)
	for i := range n {
		idx := offset + i
		id := fmt.Sprintf("evt-%04d", idx)
		actor := fmt.Sprintf("usr-%04d", idx)
		resources[i] = resource.Resource{
			ID:     id,
			Name:   fmt.Sprintf("DeleteObject-%d", idx),
			Status: "ct-write",
			Fields: map[string]string{
				"event_name":    fmt.Sprintf("DeleteObject-%d", idx),
				"time":          "2026-03-28 14:30:15",
				"event_time":    "2026-03-28 14:30:15",
				"user":          "admin",
				"source":        "s3.amazonaws.com",
				"resource_type": "",
				"resource_name": "",
				"read_only":     "false",
				// New _ct.* fields required by the redesigned list columns.
				"_ct.verb":    "D",
				"_ct.actor":   actor,
				"_ct.origin":  "CLI",
				"_ct.target":  "(none)",
				"_ct.outcome": "OK",
			},
		}
	}
	return resources
}

// ec2TestResources returns n ec2-like resources.
func ec2TestResources(n int) []resource.Resource {
	resources := make([]resource.Resource, n)
	for i := range n {
		id := fmt.Sprintf("i-%05d", i)
		resources[i] = resource.Resource{
			ID:     id,
			Name:   fmt.Sprintf("web-server-%d", i),
			Status: "running",
			Fields: map[string]string{
				"instance_id":   id,
				"instance_type": "t3.micro",
				"state":         "running",
				"name":          fmt.Sprintf("web-server-%d", i),
			},
		}
	}
	return resources
}

// ---------------------------------------------------------------------------
// Test 1: Initial truncated load shows "(50+)" in root rendered view
// ---------------------------------------------------------------------------

// TestQA_PaginationRoot_InitialLoadShowsTruncated verifies that when a paginated
// resource type loads its first page with IsTruncated=true, the root model's
// rendered frame title contains "50+" (not just "50").
//
// This is Bug 1: the "(50+)" indicator was not being shown in practice.
func TestQA_PaginationRoot_InitialLoadShowsTruncated(t *testing.T) {
	tui.Version = "test"
	m := newRootSizedModel()

	// Navigate to ct-events (push the resource list view onto the stack)
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ct-events",
	})

	// Simulate the first page arriving with IsTruncated=true.
	// We bypass the actual fetch command and inject the message directly.
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
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

	plain := stripANSI(rootViewContent(m))

	// The "+" is the key indicator. The frame title must show "ct-events(50+)".
	if !strings.Contains(plain, "50+") {
		t.Errorf("expected frame title to contain '50+' for truncated first page, but got:\n%s", plain)
	}

	// Negative: must NOT show "(50)" without the "+". Because "ct-events(50+)"
	// contains the substring "ct-events(50", we check for the exact pattern
	// by asserting that the "+" is present immediately after "50" in the title.
	// The Contains("50+") assertion above is sufficient for this requirement.
}

// ---------------------------------------------------------------------------
// Test 2: Load more appends and updates the count to "(100)"
// ---------------------------------------------------------------------------

// TestQA_PaginationRoot_LoadMoreAppendsAndShowsUpdatedCount verifies that after
// pressing M and receiving a second page, the root model renders "(100)" in the
// frame title (no "+" because the last page was not truncated).
func TestQA_PaginationRoot_LoadMoreAppendsAndShowsUpdatedCount(t *testing.T) {
	tui.Version = "test"
	m := newRootSizedModel()

	// Navigate to ct-events
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ct-events",
	})

	// Load page 1: truncated
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "ct-events",
		Resources:    ctEventsResources(50),
		Pagination: &resource.PaginationMeta{
			IsTruncated: true,
			NextToken:   "page2",
			PageSize:    50,
			TotalHint:   -1,
		},
		Append: false,
	})

	// Press M to trigger load more
	m, cmd := rootApplyMsg(m, rootKeyPress("M"))

	// The resource list view must return a non-nil command when M is pressed on
	// a truncated list (it produces a LoadMoreMsg).
	if cmd == nil {
		t.Fatal("pressing M on a truncated list at root level should return a non-nil command")
	}

	// Load page 2: final page, not truncated
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
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

	plain := stripANSI(rootViewContent(m))

	// Total loaded = 100, no more pages → "(100)" without "+"
	if !strings.Contains(plain, "ct-events(100)") {
		t.Errorf("after loading two pages, expected frame title 'ct-events(100)', got:\n%s", plain)
	}

	// Must NOT show "100+" since the last page was not truncated
	if strings.Contains(plain, "100+") {
		t.Errorf("after loading final page, frame title must not contain '100+', got:\n%s", plain)
	}
}

// ---------------------------------------------------------------------------
// Test 3: Esc and re-enter preserves cached resources (EXPECTED TO FAIL)
// ---------------------------------------------------------------------------

// TestQA_PaginationRoot_EscAndReenter_PreservesCachedResources documents the
// desired behavior for Bug 2: after loading resources (including pressing M),
// pressing Esc to return to the main menu, and then re-entering the same
// resource type, the previously loaded 100 resources must be restored without
// issuing any new fetch commands.
//
// This test is SKIPPED until the resource cache is implemented.
// Remove the t.Skip() call once the cache feature is in place.
func TestQA_PaginationRoot_EscAndReenter_PreservesCachedResources(t *testing.T) {

	tui.Version = "test"
	m := newRootSizedModel()

	// Step 1: Navigate to ct-events
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ct-events",
	})

	// Step 2: Load page 1 (truncated)
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "ct-events",
		Resources:    ctEventsResources(50),
		Pagination: &resource.PaginationMeta{
			IsTruncated: true,
			NextToken:   "page2",
			PageSize:    50,
			TotalHint:   -1,
		},
		Append: false,
	})

	// Step 3: Press M to load more
	m, _ = rootApplyMsg(m, rootKeyPress("M"))

	// Step 4: Load page 2 (final)
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "ct-events",
		Resources:    ctEventsResources2(50, 50),
		Pagination: &resource.PaginationMeta{
			IsTruncated: false,
			PageSize:    50,
			TotalHint:   -1,
		},
		Append: true,
	})

	// Verify 100 resources are loaded before navigating away
	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "ct-events(100)") {
		t.Fatalf("precondition: expected 'ct-events(100)' before Esc, got:\n%s", plain)
	}

	// Step 5: Press Esc to return to main menu
	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEscape))

	// Step 6: Re-navigate to ct-events
	m, cmd := rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ct-events",
	})

	// KEY ASSERTION: no new fetch command should be issued when cache is present.
	// Currently this FAILS because the implementation always fetches on navigate.
	if cmd != nil {
		t.Errorf("re-entering ct-events after Esc should return nil cmd (cache hit), but got a non-nil command — this is Bug 2")
	}

	plain = stripANSI(rootViewContent(m))

	// Should still show 100 resources
	if !strings.Contains(plain, "ct-events(100)") {
		t.Errorf("after re-entering ct-events, expected 'ct-events(100)' (from cache), got:\n%s", plain)
	}

	// The first resource from page 1 must still be present.
	// We check for "usr-0000" which is the _ct.actor value rendered in the ACTOR column
	// for the first ctEventsResources() entry (the ID "evt-0000" is not rendered in any column).
	if !strings.Contains(plain, "usr-0000") {
		t.Errorf("after re-entering ct-events, first-page resource actor 'usr-0000' should be visible in ACTOR column")
	}
}

// ---------------------------------------------------------------------------
// Test 4: Re-entering cached list — M continues from last token (EXPECTED TO FAIL)
// ---------------------------------------------------------------------------

// TestQA_PaginationRoot_EscAndReenter_MKeyContinuesFromLastToken documents the
// desired behavior: after re-entering a cached resource list that was truncated,
// pressing M should continue from the saved continuation token, not start over.
//
// This test is SKIPPED until the resource cache is implemented.
func TestQA_PaginationRoot_EscAndReenter_MKeyContinuesFromLastToken(t *testing.T) {

	tui.Version = "test"
	m := newRootSizedModel()

	// Step 1: Navigate to ct-events, load one page, leave it truncated
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ct-events",
	})
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "ct-events",
		Resources:    ctEventsResources(50),
		Pagination: &resource.PaginationMeta{
			IsTruncated: true,
			NextToken:   "page2-continuation-token",
			PageSize:    50,
			TotalHint:   -1,
		},
		Append: false,
	})

	// Step 2: Press Esc to go back to main menu
	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEscape))

	// Step 3: Re-enter ct-events (should use cache — no fetch)
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ct-events",
	})

	// Step 4: Press M — should continue from "page2-continuation-token"
	_, cmd := rootApplyMsg(m, rootKeyPress("M"))

	if cmd == nil {
		t.Fatal("pressing M on a cached truncated list should return a non-nil command")
	}

	// Execute the command and verify it carries the correct continuation token
	msg := cmd()
	loadMore, ok := msg.(messages.LoadMoreMsg)
	if !ok {
		t.Fatalf("expected LoadMoreMsg from M key on cached truncated list, got %T", msg)
	}

	if loadMore.ContinuationToken != "page2-continuation-token" {
		t.Errorf("LoadMoreMsg should carry continuation token 'page2-continuation-token', got %q", loadMore.ContinuationToken)
	}
}

// ---------------------------------------------------------------------------
// Test 5: Independent cache per resource type (EXPECTED TO FAIL)
// ---------------------------------------------------------------------------

// TestQA_PaginationRoot_CachePerResourceType documents the desired behavior that
// different resource types have independent caches: navigating to ct-events,
// then ec2, then back to ct-events should restore the ct-events resources (not
// the ec2 resources), and vice versa.
//
// This test is SKIPPED until the resource cache is implemented.
func TestQA_PaginationRoot_CachePerResourceType(t *testing.T) {

	tui.Version = "test"
	m := newRootSizedModel()

	// Step 1: Navigate to ct-events, load 50 resources, Esc back
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ct-events",
	})
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
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

	// Step 2: Navigate to ec2, load 30 resources, Esc back
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
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

	// Step 3: Re-enter ct-events — must show 50 (not 30 or 0)
	m, ctCmd := rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ct-events",
	})

	if ctCmd != nil {
		t.Errorf("re-entering ct-events should not issue a fetch (cache hit), but returned a non-nil cmd")
	}

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "ct-events(50)") {
		t.Errorf("after re-entering ct-events, expected 'ct-events(50)', got:\n%s", plain)
	}
	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEscape))

	// Step 4: Re-enter ec2 — must show 30 (not 50 or 0)
	m, ec2Cmd := rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})

	if ec2Cmd != nil {
		t.Errorf("re-entering ec2 should not issue a fetch (cache hit), but returned a non-nil cmd")
	}

	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "ec2(30)") {
		t.Errorf("after re-entering ec2, expected 'ec2(30)', got:\n%s", plain)
	}
}

// ---------------------------------------------------------------------------
// Tests 6–7: MainMenuModel view-level wiring for probe truncation (Bug 3)
// ---------------------------------------------------------------------------

// TestQA_MainMenu_TruncatedAvailabilityShowsPlus verifies that when the main
// menu receives a truncated availability result for ct-events (the resource type
// affected by the probe bug), it renders "(50+)" — not "(50)".
//
// This test exercises the downstream half of Bug 3: the wiring from
// SetAvailability+SetTruncated through View() already works. The broken link
// is in probeResourceAvailability, which never sets Truncated=true even for
// paginated fetchers. Once that probe fix lands, the menu will automatically
// show "(50+)" via this path.
//
// This test should PASS immediately (the view wiring is correct).
func TestQA_MainMenu_TruncatedAvailabilityShowsPlus(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	styles.Reinit()
	t.Cleanup(func() { styles.Reinit() })

	m := views.NewMainMenu(keys.Default())
	m.SetSize(80, 200)

	// Simulate what handleAvailabilityChecked does when Truncated=true arrives.
	m.SetAvailability("ct-events", 50)
	m.SetTruncated("ct-events", true)

	plain := stripANSI(m.View())

	// The rendered line for ct-events must show "(50+)".
	if !strings.Contains(plain, "(50+)") {
		t.Errorf("expected main menu to contain '(50+)' for ct-events with Truncated=true and Count=50, got:\n%s", plain)
	}

	// Must NOT show a bare "(50)" — after removing "(50+)" occurrences, "(50)" must be absent.
	withoutPlus := strings.ReplaceAll(plain, "(50+)", "")
	if strings.Contains(withoutPlus, "(50)") {
		t.Errorf("expected no bare '(50)' when truncated, only '(50+)', got:\n%s", plain)
	}
}

// TestQA_MainMenu_NonTruncatedAvailabilityNoPlus verifies that when the main
// menu receives a non-truncated availability result for ct-events, it renders
// "(50)" — not "(50+)".
//
// This is the negative case for TestQA_MainMenu_TruncatedAvailabilityShowsPlus:
// it confirms the "+" is only added when Truncated=true, preventing false positives
// in the current probe output (which correctly returns Truncated=false for most
// resource types that use the non-paginated GetFetcher path).
//
// This test should PASS immediately.
func TestQA_MainMenu_NonTruncatedAvailabilityNoPlus(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	styles.Reinit()
	t.Cleanup(func() { styles.Reinit() })

	m := views.NewMainMenu(keys.Default())
	m.SetSize(80, 200)

	// Simulate what handleAvailabilityChecked does when Truncated=false arrives
	// (the current probe behavior for ct-events — this is the bug: Truncated is
	// always false because probeResourceAvailability never calls GetPaginatedFetcher).
	m.SetAvailability("ct-events", 50)
	m.SetTruncated("ct-events", false)

	plain := stripANSI(m.View())

	// Must show "(50)" without "+".
	if !strings.Contains(plain, "(50)") {
		t.Errorf("expected main menu to contain '(50)' for ct-events with Truncated=false and Count=50, got:\n%s", plain)
	}

	// Must NOT show "(50+)" — truncation is false.
	if strings.Contains(plain, "(50+)") {
		t.Errorf("expected no '(50+)' when Truncated=false for ct-events, got:\n%s", plain)
	}
}
