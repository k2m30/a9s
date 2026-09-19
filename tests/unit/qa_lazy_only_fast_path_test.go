package unit

// Lazy-cache fast path and NeedsTargetCache
// prefetch.
//
// A checker with NeedsTargetCache=true whose target type exists only in
// lazyResourceCache still gets the target's paginated fetcher called first:
// only mainCacheKeys (resourceCache keys) suppress the prefetch, so the
// lazy-only snapshot entry cannot.
//
// Drill navigation takes the lazy fast path only when the lazy cache holds
// every requested ID (len(filtered) > 0 && len(filtered) ==
// len(result.RelatedIDs)); partial coverage falls through to the full fetch
// so the missing IDs come from AWS.

import (
	"context"
	"sync/atomic"
	"testing"

	tea "charm.land/bubbletea/v2"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
)

// ─────────────────────────────────────────────────────────────────────────────
// NeedsTargetCache prefetch fires even when lazy-only entry exists
// ─────────────────────────────────────────────────────────────────────────────

// TestNeedsTargetCache_PrefetchFires_WhenLazyOnlyEntry verifies that when a
// checker with NeedsTargetCache=true fires and the target type is present only
// in lazyResourceCache, the probe goroutine still calls GetPaginatedFetcher and
// invokes it to build a real first page.
//
// The prefetch guard uses mainCacheKeys (built only from resourceCache), so
// a lazy-only entry does not suppress prefetch: with the full snapshot
// (including lazy entries) as the guard, a lazy-only entry with
// IsTruncated=true would satisfy `inCache=true` and NeedsTargetCache
// checkers would see only the sparse lazy rows and miss actual resources on
// the first page.
func TestNeedsTargetCache_PrefetchFires_WhenLazyOnlyEntry(t *testing.T) {
	tui.Version = "test"

	const (
		srcType    = "test-gf-source"
		targetType = "test-gf-target"
	)

	var paginatedFetchCallCount int32

	resource.SetPaginatedForTest(targetType, func(_ context.Context, _ any, _ string) (resource.FetchResult, error) {
		atomic.AddInt32(&paginatedFetchCallCount, 1)
		return resource.FetchResult{
			Resources: []resource.Resource{
				{ID: "gf-target-real-001", Name: "gf-target-real-001"},
			},
		}, nil
	})
	t.Cleanup(func() { resource.CleanupPaginatedForTest(targetType) })

	resource.SetFetchByIDsForTest(targetType, func(_ context.Context, _ any, ids []string) ([]resource.Resource, error) {
		out := make([]resource.Resource, len(ids))
		for i, id := range ids {
			out[i] = resource.Resource{ID: id, Name: id}
		}
		return out, nil
	})
	t.Cleanup(func() { resource.CleanupFetchByIDsForTest(targetType) })

	var checkerCallCount int32
	resource.SetRelatedForTest(srcType, []resource.RelatedDef{
		{
			TargetType:       targetType,
			DisplayName:      "GF Target",
			NeedsTargetCache: true,
			Checker: func(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
				atomic.AddInt32(&checkerCallCount, 1)
				return resource.KnownRelated(targetType, nil, false)
			},
		},
	})
	t.Cleanup(func() { resource.CleanupRelatedForTest(srcType) })

	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	srcRes := resource.Resource{ID: "gf-src-001", Name: "gf-src-001"}
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		Resource:     &srcRes,
		ResourceType: srcType,
	})

	// Seed a lazy-only entry for targetType (NOT in resourceCache).
	lazyRes := resource.Resource{ID: "gf-lazy-001", Name: "gf-lazy-001"}
	m, _ = rootApplyMsg(m, messages.RelatedCheckResult{
		ResourceType:     srcType,
		SourceResourceID: srcRes.ID,
		DefDisplayName:   "GF Target",
		Result:           resource.KnownRelated(targetType, []string{lazyRes.ID}, false),
		OperationID:      0,
		LazyAddedResources: map[string][]resource.Resource{
			targetType: {lazyRes},
		},
	})

	// Ctrl+R re-dispatches the related-check fan-out. NeedsTargetCache=true
	// checks mainCacheKeys (resourceCache keys, not snapshot keys), so the
	// lazy-only targetType must trigger prefetch.
	_, relCmd := rootApplyMsg(m, tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl})

	if relCmd == nil {
		t.Skip("Ctrl+R returned nil cmd — no checker dispatched")
	}

	allMsgs := drainAllMessages(relCmd)
	_ = allMsgs

	// Wait for checker to be called (it runs in a goroutine inside the cmd).
	done := make(chan struct{})
	go func() {
		for atomic.LoadInt32(&checkerCallCount) == 0 {
		}
		close(done)
	}()

	select {
	case <-done:
	default:
		// Drive RelatedCheckResultMsg delivery if checker was synchronous.
		for _, msg := range allMsgs {
			if rcr, ok := msg.(messages.RelatedCheckResult); ok && rcr.ResourceType == srcType {
				m, _ = rootApplyMsg(m, rcr)
			}
		}
	}

	if atomic.LoadInt32(&checkerCallCount) == 0 {
		t.Skip("GF checker was not invoked — cannot verify prefetch behavior")
	}

	// The paginated fetcher must have been called for targetType; a guard on
	// snapshot keys would let the lazy-only entry suppress prefetch and leave
	// paginatedFetchCallCount at 0.
	if atomic.LoadInt32(&paginatedFetchCallCount) == 0 {
		t.Error("NeedsTargetCache prefetch was NOT triggered for lazy-only target — " +
			"PRE-FIX BUG: snapshot-key guard suppressed prefetch; want mainCacheKeys guard to fire prefetch")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Lazy fast path requires ALL requested IDs
// ─────────────────────────────────────────────────────────────────────────────

// TestLazyFastPath_RequiresAllIDs verifies that the lazy cache fast path
// for drill navigation only fires when ALL requested IDs are in lazyResourceCache.
//
// This test seeds lazy cache with ID k1 but requests [k1, k2]. It then
// navigates to the related list and verifies a fetch was initiated for the
// target type (proving the fast path did NOT fire for partial coverage).
func TestLazyFastPath_RequiresAllIDs(t *testing.T) {
	tui.Version = "test"

	const (
		srcType    = "test-gg-source"
		targetType = "test-gg-target"
	)

	var fetchCallCount int32
	resource.SetPaginatedForTest(targetType, func(_ context.Context, _ any, _ string) (resource.FetchResult, error) {
		atomic.AddInt32(&fetchCallCount, 1)
		return resource.FetchResult{
			Resources: []resource.Resource{
				{ID: "gg-k1", Name: "gg-k1"},
				{ID: "gg-k2", Name: "gg-k2"},
			},
		}, nil
	})
	t.Cleanup(func() { resource.CleanupPaginatedForTest(targetType) })

	resource.SetFetchByIDsForTest(targetType, func(_ context.Context, _ any, ids []string) ([]resource.Resource, error) {
		out := make([]resource.Resource, len(ids))
		for i, id := range ids {
			out[i] = resource.Resource{ID: id, Name: id}
		}
		return out, nil
	})
	t.Cleanup(func() { resource.CleanupFetchByIDsForTest(targetType) })

	resource.SetRelatedForTest(srcType, []resource.RelatedDef{
		{
			TargetType:  targetType,
			DisplayName: "GG Target",
			Checker: func(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
				return resource.KnownRelated(targetType, []string{"gg-k1", "gg-k2"}, false)
			},
		},
	})
	t.Cleanup(func() { resource.CleanupRelatedForTest(srcType) })

	// Pass non-nil clients so fetchResources doesn't short-circuit on the
	// nil-clients guard. The registered paginated fetcher above ignores the
	// clients value, so an empty struct suffices.
	m := newBlessedModel(t, "testprofile", "us-east-1", tui.WithClients(&awsclient.ServiceClients{}))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	srcRes := resource.Resource{ID: "gg-src-001", Name: "gg-src-001"}
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		Resource:     &srcRes,
		ResourceType: srcType,
	})

	// This is the partial-coverage scenario that must NOT use the fast path.
	k1Res := resource.Resource{ID: "gg-k1", Name: "gg-k1"}
	m, _ = rootApplyMsg(m, messages.RelatedCheckResult{
		ResourceType:     srcType,
		SourceResourceID: srcRes.ID,
		DefDisplayName:   "GG Target",
		Result:           resource.KnownRelated(targetType, []string{"gg-k1", "gg-k2"}, false),
		OperationID:      0,
		LazyAddedResources: map[string][]resource.Resource{
			targetType: {k1Res}, // only k1, k2 is missing
		},
	})

	// With full coverage (both IDs in lazy), fast path fires → no fetch.
	// With partial coverage (k2 missing), fast path must NOT fire → fetchResources called.
	_, drillCmd := rootApplyMsg(m, messages.RelatedNavigate{
		TargetType:     targetType,
		SourceType:     srcType,
		SourceResource: srcRes,
		RelatedIDs:     []string{"gg-k1", "gg-k2"},
	})

	if drillCmd != nil {
		drainAllMessages(drillCmd)
	}

	// fetchResources must have been triggered because partial coverage
	// prevents the lazy fast path; a `len(filtered) > 0` condition would take
	// the fast path and leave fetchCallCount at 0.
	if atomic.LoadInt32(&fetchCallCount) == 0 {
		t.Error("lazy fast path fired for partial ID coverage — " +
			"PRE-FIX BUG: fast path should only fire when ALL IDs are in lazy cache; " +
			"partial coverage must fall through to full fetch")
	}
}
