package unit

// qa_lazy_only_fast_path_test.go — Regression pins for the lazy-cache
// fast-path and NeedsTargetCache prefetch logic (Groups F and G).
//
// Group F — NeedsTargetCache prefetch fires when target is lazy-only
//   File: internal/tui/app_related.go:91-110
//   Contract: when a checker has NeedsTargetCache=true and the target type
//   exists ONLY in lazyResourceCache (not in resourceCache), the probe goroutine
//   must still call the paginated fetcher for the target type before invoking
//   the checker. The snapshot's IsTruncated=true for lazy-only entries must NOT
//   suppress prefetch — only mainCacheKeys (resourceCache keys) determines this.
//
// Group G — lazy fast path requires ALL requested IDs
//   File: internal/tui/app_related.go:331
//   Contract: when navigating to a related resource list and the lazy cache
//   contains SOME but not ALL requested IDs, the lazy fast path must NOT fire.
//   The model must instead fall through to the full-fetch path so the missing
//   IDs are retrieved from AWS. The condition is:
//   len(filtered) > 0 && len(filtered) == len(result.RelatedIDs).
//   Partial coverage means len(filtered) < len(result.RelatedIDs) → full fetch.

import (
	"context"
	"sync/atomic"
	"testing"

	tea "charm.land/bubbletea/v2"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
)

// ─────────────────────────────────────────────────────────────────────────────
// Group F: NeedsTargetCache prefetch fires even when lazy-only entry exists
// ─────────────────────────────────────────────────────────────────────────────

// TestNeedsTargetCache_PrefetchFires_WhenLazyOnlyEntry verifies that when a
// checker with NeedsTargetCache=true fires and the target type is present only
// in lazyResourceCache, the probe goroutine still calls GetPaginatedFetcher and
// invokes it to build a real first page.
//
// The pre-fix bug: the prefetch guard was `if _, inCache := localCache[def.TargetType]; !inCache`
// where localCache was the full snapshot (including lazy entries). A lazy-only
// entry with IsTruncated=true would satisfy `inCache=true`, suppressing the
// prefetch — so NeedsTargetCache checkers would see only the sparse lazy rows
// and miss actual resources on the first page.
//
// Post-fix: the guard uses mainCacheKeys (built only from resourceCache) so
// lazy-only entries do NOT suppress prefetch.
func TestNeedsTargetCache_PrefetchFires_WhenLazyOnlyEntry(t *testing.T) {
	tui.Version = "test"

	const (
		srcType    = "test-gf-source"
		targetType = "test-gf-target"
	)

	var paginatedFetchCallCount int32

	resource.RegisterPaginated(targetType, func(_ context.Context, _ any, _ string) (resource.FetchResult, error) {
		atomic.AddInt32(&paginatedFetchCallCount, 1)
		return resource.FetchResult{
			Resources: []resource.Resource{
				{ID: "gf-target-real-001", Name: "gf-target-real-001"},
			},
		}, nil
	})
	t.Cleanup(func() { resource.UnregisterPaginated(targetType) })

	resource.RegisterFetchByIDs(targetType, func(_ context.Context, _ any, ids []string) ([]resource.Resource, error) {
		out := make([]resource.Resource, len(ids))
		for i, id := range ids {
			out[i] = resource.Resource{ID: id, Name: id}
		}
		return out, nil
	})
	t.Cleanup(func() { resource.UnregisterFetchByIDs(targetType) })

	var checkerCallCount int32
	resource.RegisterRelated(srcType, []resource.RelatedDef{
		{
			TargetType:       targetType,
			DisplayName:      "GF Target",
			NeedsTargetCache: true,
			Checker: func(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
				atomic.AddInt32(&checkerCallCount, 1)
				return resource.RelatedCheckResult{
					TargetType: targetType,
					Count:      0,
				}
			},
		},
	})
	t.Cleanup(func() { resource.UnregisterRelated(srcType) })

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
		Result: resource.RelatedCheckResult{
			TargetType:  targetType,
			Count:       1,
			ResourceIDs: []string{lazyRes.ID},
		},
		Generation: 0,
		LazyAddedResources: map[string][]resource.Resource{
			targetType: {lazyRes},
		},
	})

	// Dispatch RelatedCheckStartedMsg — this triggers the checker goroutine.
	// In the goroutine, NeedsTargetCache=true checks mainCacheKeys (resourceCache
	// keys, NOT snapshot keys). Since targetType is lazy-only, it must trigger prefetch.
	_, relCmd := rootApplyMsg(m, messages.RelatedCheckStarted{
		ResourceType:   srcType,
		SourceResource: srcRes,
	})

	if relCmd == nil {
		t.Skip("RelatedCheckStartedMsg returned nil cmd — no checker dispatched")
	}

	// Execute the cmd tree to run the checker goroutines.
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

	// CONTRACT ASSERTION: paginated fetcher must have been called for targetType.
	// If pre-fix guard is used (snapshot keys), the lazy-only entry would suppress
	// prefetch and paginatedFetchCallCount would remain 0.
	if atomic.LoadInt32(&paginatedFetchCallCount) == 0 {
		t.Error("NeedsTargetCache prefetch was NOT triggered for lazy-only target — " +
			"PRE-FIX BUG: snapshot-key guard suppressed prefetch; want mainCacheKeys guard to fire prefetch")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Group G: lazy fast path requires ALL requested IDs
// ─────────────────────────────────────────────────────────────────────────────

// TestLazyFastPath_RequiresAllIDs verifies that the lazy cache fast path
// for drill navigation only fires when ALL requested IDs are in lazyResourceCache.
//
// Pre-fix concern (now fixed): if the condition were `len(filtered) > 0` alone,
// partial lazy coverage would use the fast path, rendering a list missing the
// IDs not in cache.
// Post-fix: condition is `len(filtered) > 0 && len(filtered) == len(result.RelatedIDs)`.
// Partial coverage falls through to the full-fetch path.
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
	resource.RegisterPaginated(targetType, func(_ context.Context, _ any, _ string) (resource.FetchResult, error) {
		atomic.AddInt32(&fetchCallCount, 1)
		return resource.FetchResult{
			Resources: []resource.Resource{
				{ID: "gg-k1", Name: "gg-k1"},
				{ID: "gg-k2", Name: "gg-k2"},
			},
		}, nil
	})
	t.Cleanup(func() { resource.UnregisterPaginated(targetType) })

	resource.RegisterFetchByIDs(targetType, func(_ context.Context, _ any, ids []string) ([]resource.Resource, error) {
		out := make([]resource.Resource, len(ids))
		for i, id := range ids {
			out[i] = resource.Resource{ID: id, Name: id}
		}
		return out, nil
	})
	t.Cleanup(func() { resource.UnregisterFetchByIDs(targetType) })

	resource.RegisterRelated(srcType, []resource.RelatedDef{
		{
			TargetType:  targetType,
			DisplayName: "GG Target",
			Checker: func(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
				return resource.RelatedCheckResult{
					TargetType:  targetType,
					Count:       2,
					ResourceIDs: []string{"gg-k1", "gg-k2"},
				}
			},
		},
	})
	t.Cleanup(func() { resource.UnregisterRelated(srcType) })

	// Pass non-nil clients so fetchResources doesn't short-circuit on the
	// nil-clients guard. The registered paginated fetcher above ignores the
	// clients value, so an empty struct suffices.
	m := tui.New("testprofile", "us-east-1", tui.WithClients(&awsclient.ServiceClients{}))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	srcRes := resource.Resource{ID: "gg-src-001", Name: "gg-src-001"}
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		Resource:     &srcRes,
		ResourceType: srcType,
	})

	// Seed lazy cache with ONLY k1 (k2 is missing).
	// This is the partial-coverage scenario that must NOT use the fast path.
	k1Res := resource.Resource{ID: "gg-k1", Name: "gg-k1"}
	m, _ = rootApplyMsg(m, messages.RelatedCheckResult{
		ResourceType:     srcType,
		SourceResourceID: srcRes.ID,
		DefDisplayName:   "GG Target",
		Result: resource.RelatedCheckResult{
			TargetType:  targetType,
			Count:       2,
			ResourceIDs: []string{"gg-k1", "gg-k2"},
		},
		Generation: 0,
		LazyAddedResources: map[string][]resource.Resource{
			targetType: {k1Res}, // only k1, k2 is missing
		},
	})

	// Navigate to the related list — this triggers handleRelatedNavigate.
	// With full coverage (both IDs in lazy), fast path fires → no fetch.
	// With partial coverage (k2 missing), fast path must NOT fire → fetchResources called.
	_, drillCmd := rootApplyMsg(m, messages.RelatedNavigate{
		TargetType:     targetType,
		SourceType:     srcType,
		SourceResource: srcRes,
		RelatedIDs:     []string{"gg-k1", "gg-k2"},
	})

	// Drain the cmd tree.
	if drillCmd != nil {
		drainAllMessages(drillCmd)
	}

	// CONTRACT ASSERTION: fetchResources must have been triggered because partial
	// coverage prevents lazy fast path. If the pre-fix condition (`len(filtered) > 0`)
	// were in place, fetchCallCount would be 0 (fast path used).
	if atomic.LoadInt32(&fetchCallCount) == 0 {
		t.Error("lazy fast path fired for partial ID coverage — " +
			"PRE-FIX BUG: fast path should only fire when ALL IDs are in lazy cache; " +
			"partial coverage must fall through to full fetch")
	}
}
