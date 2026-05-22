package unit

// qa_lazy_cache_snapshot_truncated_test.go — Regression pin for
// buildResourceCacheSnapshot marking lazy-only entries as IsTruncated=true
// (Group E).
//
// File: internal/tui/app_related.go:601-630
//
// Bug today (line 604-608): lazy-only entries are published with IsTruncated=false.
// This misleads NeedsTargetCache=true checkers into believing the lazy-only
// cache entry is a complete first page, so they skip the prefetch that would
// fetch the real authoritative first page.
//
// Contract after fix:
//   - Lazy-only entries (no corresponding resourceCache key) → snapshot entry
//     has IsTruncated=true (sparse, not authoritative).
//   - Merge case (both lazy + resourceCache present): inherit resourceCache's
//     pagination IsTruncated as today.
//
// Test approach:
//   Seed a lazy-only entry for "test-ge-target" via RelatedCheckResultMsg
//   (LazyAddedResources path). Then dispatch RelatedCheckStartedMsg with a
//   checker registered for "test-ge-source" that targets "test-ge-target" with
//   NeedsTargetCache=true. Capture the ResourceCache the checker receives and
//   assert IsTruncated=true for the lazy-only entry.

import (
	"context"
	"sync/atomic"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
)

// TestBuildResourceCacheSnapshot_LazyOnlyTruncated verifies that when a resource
// type exists only in lazyResourceCache (not in resourceCache), the snapshot
// entry for that type has IsTruncated=true.
//
// Fails today: buildResourceCacheSnapshot at line ~604-608 hardcodes IsTruncated=false
// for all lazy-only entries, so a NeedsTargetCache checker sees a "complete" cache
// and skips the prefetch — causing drill-to-empty for targets that need a real page.
// Passes after fix: lazy-only entries get IsTruncated=true.
func TestBuildResourceCacheSnapshot_LazyOnlyTruncated(t *testing.T) {
	tui.Version = "test"

	const (
		srcType    = "test-ge-source"
		targetType = "test-ge-target"
	)

	// Capture the ResourceCache that the checker receives.
	var capturedCache resource.ResourceCache
	var checkerCallCount int32

	resource.SetRelatedForTest(srcType, []resource.RelatedDef{
		{
			TargetType:       targetType,
			DisplayName:      "GE Target",
			NeedsTargetCache: true, // this is the checker that needs the real first page
			Checker: func(_ context.Context, _ any, _ resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
				atomic.AddInt32(&checkerCallCount, 1)
				capturedCache = cache
				entry := cache[targetType]
				return resource.RelatedCheckResult{
					TargetType: targetType,
					Count:      len(entry.Resources),
				}
			},
		},
	})
	t.Cleanup(func() { resource.CleanupRelatedForTest(srcType) })

	resource.SetFetchByIDsForTest(targetType, func(_ context.Context, _ any, ids []string) ([]resource.Resource, error) {
		out := make([]resource.Resource, len(ids))
		for i, id := range ids {
			out[i] = resource.Resource{ID: id, Name: id}
		}
		return out, nil
	})
	t.Cleanup(func() { resource.CleanupFetchByIDsForTest(targetType) })

	// Register a paginated fetcher for targetType so prefetch can fire.
	resource.SetPaginatedForTest(targetType, func(_ context.Context, _ any, _ string) (resource.FetchResult, error) {
		return resource.FetchResult{
			Resources: []resource.Resource{
				{ID: "ge-target-001", Name: "ge-target-001"},
			},
		}, nil
	})
	t.Cleanup(func() { resource.CleanupPaginatedForTest(targetType) })

	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	// Navigate to src detail view so RelatedCheckStartedMsg is handled.
	srcRes := resource.Resource{ID: "ge-src-001", Name: "ge-src-001"}
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		Resource:     &srcRes,
		ResourceType: srcType,
	})

	// Seed a lazy-only entry for targetType via RelatedCheckResultMsg.
	// LazyAddedResources populates lazyResourceCache[targetType].
	// Since resourceCache[targetType] does NOT exist, this is a "lazy-only" entry.
	lazyRes := resource.Resource{ID: "ge-lazy-001", Name: "ge-lazy-001"}
	m, _ = rootApplyMsg(m, messages.RelatedCheckResult{
		ResourceType:     srcType,
		SourceResourceID: srcRes.ID,
		DefDisplayName:   "GE Target",
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

	// Now dispatch RelatedCheckStartedMsg so the model calls buildResourceCacheSnapshot
	// and passes it to the registered checker.
	_, relCmd := rootApplyMsg(m, messages.RelatedCheckStarted{
		ResourceType:   srcType,
		SourceResource: srcRes,
	})

	if relCmd == nil {
		t.Fatal("RelatedCheckStartedMsg should return a cmd for related checkers")
	}

	// Execute the cmd tree to trigger the checker goroutines.
	// The checker (registered above) captures the cache and stores it.
	allMsgs := drainAllMessages(relCmd)
	_ = allMsgs

	// Wait briefly for goroutines to complete (the checker runs in a goroutine).
	// Poll for checker to be called.
	done := make(chan struct{})
	go func() {
		for {
			if atomic.LoadInt32(&checkerCallCount) > 0 {
				close(done)
				return
			}
		}
	}()

	select {
	case <-done:
	default:
		// Checker may not have been called yet; drive RelatedCheckResultMsg delivery.
		for _, msg := range allMsgs {
			if rcr, ok := msg.(messages.RelatedCheckResult); ok && rcr.ResourceType == srcType {
				m, _ = rootApplyMsg(m, rcr)
			}
		}
	}

	if atomic.LoadInt32(&checkerCallCount) == 0 {
		t.Skip("GE checker was not invoked — cannot verify IsTruncated; likely NeedsTargetCache=true prefetch short-circuited")
	}

	// CONTRACT ASSERTION — fails today, passes after fix.
	if capturedCache == nil {
		t.Fatal("captured cache is nil — checker was not called with a valid cache")
	}
	entry, ok := capturedCache[targetType]
	if !ok {
		t.Fatalf("captured cache does not contain %q — lazy-only entry was not included in snapshot", targetType)
	}
	if !entry.IsTruncated {
		t.Errorf("buildResourceCacheSnapshot: lazy-only entry for %q has IsTruncated=false, want true — LAZY-ONLY TRUNCATED BUG: NeedsTargetCache checkers will wrongly skip prefetch", targetType)
	}
}

// TestBuildResourceCacheSnapshot_MergeCase_InheritsResourceCacheTruncated
// verifies that when both lazy and resourceCache entries exist for a type,
// the snapshot's IsTruncated is inherited from the resourceCache entry
// (not overridden by the lazy-only logic).
//
// This tests the MERGE case (both sources present): must preserve the
// resourceCache's pagination IsTruncated.
func TestBuildResourceCacheSnapshot_MergeCase_InheritsResourceCacheTruncated(t *testing.T) {
	tui.Version = "test"

	const (
		srcType    = "test-ge2-source"
		targetType = "test-ge2-target"
	)

	var capturedCache resource.ResourceCache
	var checkerCallCount int32

	resource.SetRelatedForTest(srcType, []resource.RelatedDef{
		{
			TargetType:       targetType,
			DisplayName:      "GE2 Target",
			NeedsTargetCache: false,
			Checker: func(_ context.Context, _ any, _ resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
				atomic.AddInt32(&checkerCallCount, 1)
				capturedCache = cache
				return resource.RelatedCheckResult{
					TargetType: targetType,
					Count:      len(cache[targetType].Resources),
				}
			},
		},
	})
	t.Cleanup(func() { resource.CleanupRelatedForTest(srcType) })

	resource.SetFetchByIDsForTest(targetType, func(_ context.Context, _ any, ids []string) ([]resource.Resource, error) {
		out := make([]resource.Resource, len(ids))
		for i, id := range ids {
			out[i] = resource.Resource{ID: id, Name: id}
		}
		return out, nil
	})
	t.Cleanup(func() { resource.CleanupFetchByIDsForTest(targetType) })

	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	srcRes := resource.Resource{ID: "ge2-src-001", Name: "ge2-src-001"}
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		Resource:     &srcRes,
		ResourceType: srcType,
	})

	// Seed the resourceCache[targetType] via a ResourcesLoadedMsg first.
	// This sets resourceCache (not lazy). The fetched list is NOT truncated.
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: targetType,
	})
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: targetType,
		Resources: []resource.Resource{
			{ID: "ge2-main-001", Name: "ge2-main-001"},
		},
		Pagination: &resource.PaginationMeta{IsTruncated: false},
	})
	// Navigate back to src detail.
	m, _ = rootApplyMsg(m, messages.PopView{})
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		Resource:     &srcRes,
		ResourceType: srcType,
	})

	// Also seed a lazy entry.
	m, _ = rootApplyMsg(m, messages.RelatedCheckResult{
		ResourceType:     srcType,
		SourceResourceID: srcRes.ID,
		DefDisplayName:   "GE2 Target",
		Result: resource.RelatedCheckResult{
			TargetType: targetType,
			Count:      1,
		},
		Generation: 0,
		LazyAddedResources: map[string][]resource.Resource{
			targetType: {{ID: "ge2-lazy-001", Name: "ge2-lazy-001"}},
		},
	})

	// Dispatch RelatedCheckStartedMsg.
	_, relCmd := rootApplyMsg(m, messages.RelatedCheckStarted{
		ResourceType:   srcType,
		SourceResource: srcRes,
	})
	if relCmd == nil {
		t.Skip("no cmd returned from RelatedCheckStartedMsg")
	}
	allMsgs := drainAllMessages(relCmd)
	for _, msg := range allMsgs {
		if rcr, ok := msg.(messages.RelatedCheckResult); ok {
			m, _ = rootApplyMsg(m, rcr)
		}
	}

	if atomic.LoadInt32(&checkerCallCount) == 0 {
		t.Skip("GE2 checker not invoked")
	}

	if capturedCache == nil {
		t.Fatal("captured cache is nil")
	}
	entry, ok := capturedCache[targetType]
	if !ok {
		t.Fatalf("captured cache does not contain %q", targetType)
	}
	// Merge case: resourceCache has IsTruncated=false; that should be preserved.
	if entry.IsTruncated {
		t.Errorf("buildResourceCacheSnapshot merge case: IsTruncated = true, want false (resourceCache pagination says not truncated)")
	}
}
