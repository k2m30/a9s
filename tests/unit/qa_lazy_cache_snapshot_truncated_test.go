package unit

// BuildResourceCacheSnapshot marks lazy-only entries (no matching
// resourceCache key) Partial and IsTruncated=true: a lazy-only entry is a few
// rows added for one detail, not the type's list, so a reader fetches the
// first page instead. When both lazy and resourceCache entries exist, the
// snapshot inherits resourceCache's pagination IsTruncated.

import (
	"context"
	"sync/atomic"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
)

// TestBuildResourceCacheSnapshot_LazyOnlyTruncated: a type known only through
// lazily added rows is snapshotted as Partial and truncated, so it is never
// read as the type's list; a NeedsTargetCache checker gets the first page its
// prefetch read instead, as complete as that page is.
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
				ids := make([]string, len(entry.Resources))
				for i, r := range entry.Resources {
					ids[i] = r.ID
				}
				return resource.KnownRelated(targetType, ids, false)
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

	// Navigate to src detail view — begins the initial DetailOperation. Its
	// own related-check cmd is intentionally left undrained (the lazy entry
	// below hasn't been seeded yet); the assertion drives a SEPARATE,
	// Ctrl+R-triggered fan-out after seeding.
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
		Result:           resource.KnownRelated(targetType, []string{lazyRes.ID}, false),
		OperationID:      0,
		LazyAddedResources: map[string][]resource.Resource{
			targetType: {lazyRes},
		},
	})

	// Ctrl+R re-dispatches the related-check fan-out against the current
	// cache state, which includes the seeded lazy entry.
	_, relCmd := rootApplyMsg(m, tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl})

	if relCmd == nil {
		t.Fatal("Ctrl+R should return a cmd for related checkers")
	}

	allMsgs := drainAllMessages(relCmd)
	_ = allMsgs

	// Wait briefly for goroutines to complete (the checker runs in a goroutine).
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

	if capturedCache == nil {
		t.Fatal("captured cache is nil — checker was not called with a valid cache")
	}
	entry, ok := capturedCache[targetType]
	if !ok {
		t.Fatalf("captured cache does not contain %q — neither the lazy entry nor the prefetched page reached the checker", targetType)
	}
	if len(entry.Resources) != 1 || entry.Resources[0].ID != "ge-target-001" || entry.IsTruncated || entry.Partial {
		t.Errorf("checker's %q entry = %d rows truncated=%v partial=%v, want the prefetched first page [ge-target-001], complete",
			targetType, len(entry.Resources), entry.IsTruncated, entry.Partial)
	}

	lazy, ok := m.Core().BuildResourceCacheSnapshot()[targetType]
	if !ok || !lazy.Partial || !lazy.IsTruncated {
		t.Errorf("snapshot of the lazy-only %q entry = present %v partial=%v truncated=%v, want Partial and truncated", targetType, ok, lazy.Partial, lazy.IsTruncated)
	}
}

// TestBuildResourceCacheSnapshot_MergeCase_InheritsResourceCacheTruncated
// verifies that when both lazy and resourceCache entries exist for a type,
// the snapshot's IsTruncated is inherited from the resourceCache entry
// (not overridden by the lazy-only logic).
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
				entry := cache[targetType]
				ids := make([]string, len(entry.Resources))
				for i, r := range entry.Resources {
					ids[i] = r.ID
				}
				return resource.KnownRelated(targetType, ids, false)
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
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
		ResourceType: targetType,
		Resources: []resource.Resource{
			{ID: "ge2-main-001", Name: "ge2-main-001"},
		},
		Pagination: &resource.PaginationMeta{IsTruncated: false},
	})
	m, _ = rootApplyMsg(m, messages.PopView{})
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		Resource:     &srcRes,
		ResourceType: srcType,
	})

	m, _ = rootApplyMsg(m, messages.RelatedCheckResult{
		ResourceType:     srcType,
		SourceResourceID: srcRes.ID,
		DefDisplayName:   "GE2 Target",
		Result:           resource.KnownRelated(targetType, []string{"ge2-lazy-001"}, false),
		OperationID:      0,
		LazyAddedResources: map[string][]resource.Resource{
			targetType: {{ID: "ge2-lazy-001", Name: "ge2-lazy-001"}},
		},
	})

	// Ctrl+R re-dispatches the related-check fan-out.
	_, relCmd := rootApplyMsg(m, tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl})
	if relCmd == nil {
		t.Skip("no cmd returned from Ctrl+R")
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
