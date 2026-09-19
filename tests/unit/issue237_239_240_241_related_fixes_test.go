package unit

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// A cold-miss prefetch carries the full Pagination, NextToken included, in
// RelatedCheckResult.CachedPages, so the operator can page past the probe's
// first page.

// distinctiveIDs returns n distinct, non-empty IDs — used to construct a
// resource.KnownRelated result whose Count is a specific test-chosen marker
// value (e.g. "99" or "7") so an assertion can check for that exact number
// appearing (or not appearing) in the rendered view.
func distinctiveIDs(n int) []string {
	ids := make([]string, n)
	for i := range ids {
		ids[i] = fmt.Sprintf("distinctive-id-%d", i)
	}
	return ids
}

func TestIssue237_ColdMissWriteBack_PreservesNextToken(t *testing.T) {
	const (
		srcType    = "_t237_src"
		targetType = "_t237_target"
		wantToken  = "next-page-token-abc123"
	)

	resource.SetPaginatedForTest(targetType, func(_ context.Context, _ any, _ string) (resource.FetchResult, error) {
		return resource.FetchResult{
			Resources: []resource.Resource{{ID: "r1"}},
			Pagination: &resource.PaginationMeta{
				IsTruncated: true,
				NextToken:   wantToken,
			},
		}, nil
	})

	resource.SetRelatedForTest(srcType, []resource.RelatedDef{
		{
			TargetType:       targetType,
			DisplayName:      "Test Target",
			NeedsTargetCache: true,
			Checker: func(_ context.Context, _ any, _ resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
				entry, ok := cache[targetType]
				if !ok {
					return resource.UnknownRelated(targetType)
				}
				ids := make([]string, len(entry.Resources))
				for i, r := range entry.Resources {
					ids[i] = r.ID
				}
				return resource.KnownRelated(targetType, ids, false)
			},
		},
	})

	t.Cleanup(func() {
		resource.CleanupRelatedForTest(srcType)
		resource.CleanupPaginatedForTest(targetType)
	})

	m := newBlessedModel(t, "testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	srcRes := resource.Resource{ID: "src-237-instance"}

	_, batchCmd := rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		ResourceType: srcType,
		Resource:     &srcRes,
	})
	if batchCmd == nil {
		t.Fatal("handleRelatedCheckStarted returned nil — expected checker batch")
	}

	// With a single def, Bubble Tea may return the result directly rather than
	// wrapping it in tea.BatchMsg.
	rawMsg := batchCmd()
	var resultMsg messages.RelatedCheckResult
	found := false
	switch v := rawMsg.(type) {
	case messages.RelatedCheckResult:
		resultMsg = v
		found = true
	case tea.BatchMsg:
		for _, cmd := range v {
			if cmd == nil {
				continue
			}
			if r, ok2 := cmd().(messages.RelatedCheckResult); ok2 {
				resultMsg = r
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("no RelatedCheckResultMsg received; got %T", rawMsg)
	}

	if resultMsg.CachedPages == nil {
		t.Fatal("CachedPages is nil — cold-miss prefetch did not fire (check NeedsTargetCache=true)")
	}
	entry, ok := resultMsg.CachedPages[targetType]
	if !ok {
		t.Fatalf("CachedPages missing entry for %q", targetType)
	}
	if entry.Pagination == nil {
		t.Fatal("CachedPages entry has nil Pagination — NextToken is lost (fix: store fr.Pagination in ResourceCacheEntry)")
	}
	if !entry.Pagination.IsTruncated {
		t.Error("Pagination.IsTruncated should be true")
	}
	if entry.Pagination.NextToken != wantToken {
		t.Errorf("NextToken: got %q, want %q — pagination will reset to page 1 instead of continuing", entry.Pagination.NextToken, wantToken)
	}
}

// A late result from the detail operation before Ctrl+R is dropped: Ctrl+R
// begins a fresh DetailOperation (BeginDetailOperation bumps
// session.DetailOpGen), so the right column never shows counts from the
// previous check batch.
func TestIssue239_StaleGenerationResult_IsDiscarded(t *testing.T) {
	const viewedResourceID = "i-0a1b2c3d4e5f60001"

	m := setupEC2DetailWithResults(t)

	staleOp := m.Core().ActiveDetailOp()

	viewBefore := stripANSI(rootViewContent(m))
	if !strings.Contains(viewBefore, "(7)") {
		t.Fatalf("precondition: expected '(7)' in view before test; got:\n%s", viewBefore)
	}

	m, _ = rootApplyMsg(m, ctrlR())

	viewAfterRefresh := stripANSI(rootViewContent(m))
	if strings.Contains(viewAfterRefresh, "(7)") {
		t.Fatalf("precondition: after Ctrl+R stale '(7)' still visible — relatedCache not cleared:\n%s", viewAfterRefresh)
	}

	m, _ = rootApplyMsg(m, messages.RelatedCheckResult{
		ResourceType:     "ec2",
		SourceResourceID: viewedResourceID,
		OperationID:      staleOp,
		Result:           resource.KnownRelated("tg", distinctiveIDs(99), false), // distinctive count — must NOT appear
	})

	viewAfterStale := stripANSI(rootViewContent(m))
	if strings.Contains(viewAfterStale, "99") {
		t.Errorf("stale-operation result applied after Ctrl+R began a new operation — it must be discarded.\n"+
			"View:\n%s", viewAfterStale)
	}
}

func TestIssue239_CurrentGenerationResult_IsAccepted(t *testing.T) {
	const viewedResourceID = "i-0a1b2c3d4e5f60001"

	m := setupEC2DetailWithResults(t)
	m, _ = rootApplyMsg(m, ctrlR()) // begins a fresh DetailOperation

	currentOp := m.Core().ActiveDetailOp()

	m, _ = rootApplyMsg(m, messages.RelatedCheckResult{
		ResourceType:     "ec2",
		SourceResourceID: viewedResourceID,
		OperationID:      currentOp,
		Result:           resource.KnownRelated("tg", distinctiveIDs(7), false),
	})

	view := stripANSI(rootViewContent(m))
	if !strings.Contains(view, "(7)") {
		t.Errorf("current-operation result should be accepted; right column should show '(7)'.\nView:\n%s", view)
	}
}

// A checker with NeedsTargetCache=false triggers no cold-cache fetch of its
// target type: only checkers that read the target cache pay that API cost.
func TestIssue240_FieldOnlyChecker_NoPrefetch(t *testing.T) {
	const (
		srcType    = "_t240_src_field"
		targetType = "_t240_target_field"
	)

	fetchCalled := atomic.Bool{}

	resource.SetPaginatedForTest(targetType, func(_ context.Context, _ any, _ string) (resource.FetchResult, error) {
		fetchCalled.Store(true) // must NOT be called for field-only checker
		return resource.FetchResult{Resources: []resource.Resource{{ID: "should-not-fetch"}}}, nil
	})

	resource.SetRelatedForTest(srcType, []resource.RelatedDef{
		{
			TargetType:       targetType,
			DisplayName:      "Field-Only Checker",
			NeedsTargetCache: false, // field-only: derives result from source, not target cache
			Checker: func(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
				if res.Fields["has_target"] == "true" {
					return resource.KnownRelated(targetType, []string{"derived-id"}, false)
				}
				return resource.KnownRelated(targetType, nil, false)
			},
		},
	})

	t.Cleanup(func() {
		resource.CleanupRelatedForTest(srcType)
		resource.CleanupPaginatedForTest(targetType)
	})

	m := newBlessedModel(t, "testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	srcRes := resource.Resource{
		ID:     "src-240",
		Fields: map[string]string{"has_target": "true"},
	}

	_, batchCmd := rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		ResourceType: srcType,
		Resource:     &srcRes,
	})
	if batchCmd == nil {
		t.Fatal("handleRelatedCheckStarted returned nil")
	}

	var results []messages.RelatedCheckResult
	rawMsg := batchCmd()
	switch v := rawMsg.(type) {
	case messages.RelatedCheckResult:
		results = append(results, v)
	case tea.BatchMsg:
		for _, cmd := range v {
			if cmd == nil {
				continue
			}
			if r, ok2 := cmd().(messages.RelatedCheckResult); ok2 {
				results = append(results, r)
			}
		}
	}

	for _, r := range results {
		if r.CachedPages != nil {
			t.Errorf("field-only checker (NeedsTargetCache=false) must NOT populate CachedPages; "+
				"got: %v — this means an unnecessary AWS API call was made", r.CachedPages)
		}
	}

	if fetchCalled.Load() {
		t.Error("paginated fetcher was called for a NeedsTargetCache=false checker — " +
			"field-only checkers must not trigger cold-cache prefetches")
	}
}

func TestIssue240_CacheDependentChecker_DoesPrefetch(t *testing.T) {
	const (
		srcType    = "_t240_src_cache"
		targetType = "_t240_target_cache"
	)

	fetchCalled := atomic.Bool{}

	resource.SetPaginatedForTest(targetType, func(_ context.Context, _ any, _ string) (resource.FetchResult, error) {
		fetchCalled.Store(true)
		return resource.FetchResult{
			Resources:  []resource.Resource{{ID: "t1"}},
			Pagination: &resource.PaginationMeta{IsTruncated: false},
		}, nil
	})

	resource.SetRelatedForTest(srcType, []resource.RelatedDef{
		{
			TargetType:       targetType,
			DisplayName:      "Cache-Dependent Checker",
			NeedsTargetCache: true,
			Checker: func(_ context.Context, _ any, _ resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
				entry, ok := cache[targetType]
				if !ok {
					return resource.UnknownRelated(targetType)
				}
				ids := make([]string, len(entry.Resources))
				for i, r := range entry.Resources {
					ids[i] = r.ID
				}
				return resource.KnownRelated(targetType, ids, false)
			},
		},
	})

	t.Cleanup(func() {
		resource.CleanupRelatedForTest(srcType)
		resource.CleanupPaginatedForTest(targetType)
	})

	m := newBlessedModel(t, "testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	srcRes := resource.Resource{ID: "src-240-cache"}

	_, batchCmd := rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		ResourceType: srcType,
		Resource:     &srcRes,
	})
	if batchCmd == nil {
		t.Fatal("handleRelatedCheckStarted returned nil")
	}

	var resultMsg messages.RelatedCheckResult
	rawMsg240 := batchCmd()
	switch v := rawMsg240.(type) {
	case messages.RelatedCheckResult:
		resultMsg = v
	case tea.BatchMsg:
		for _, cmd := range v {
			if cmd == nil {
				continue
			}
			if r, ok2 := cmd().(messages.RelatedCheckResult); ok2 {
				resultMsg = r
			}
		}
	default:
		t.Fatalf("unexpected msg type %T", rawMsg240)
	}

	if !fetchCalled.Load() {
		t.Error("paginated fetcher was NOT called for a NeedsTargetCache=true checker — cold-miss prefetch must fire")
	}
	if resultMsg.CachedPages == nil {
		t.Error("CachedPages should be non-nil for a NeedsTargetCache=true checker on cold cache")
	}
}

// At most 4 related checkers run concurrently for one detail view, to avoid
// saturating AWS API rate limits.
func TestIssue241_ConcurrentProbesCappedAt4(t *testing.T) {
	const (
		srcType     = "_t241_src"
		numCheckers = 8 // more than maxConcurrentProbes (4)
		maxAllowed  = 4
	)

	for i := range numCheckers {
		targetType := "_t241_target_" + string(rune('a'+i))
		idx := i
		resource.SetPaginatedForTest(targetType, func(_ context.Context, _ any, _ string) (resource.FetchResult, error) {
			return resource.FetchResult{
				Resources:  []resource.Resource{{ID: "r" + string(rune('a'+idx))}},
				Pagination: &resource.PaginationMeta{IsTruncated: false},
			}, nil
		})
		t.Cleanup(func() { resource.CleanupPaginatedForTest(targetType) })
	}

	var (
		concurrentNow int64 // currently running checkers
		maxSeen       int64 // maximum observed simultaneously
		mu            sync.Mutex
	)

	gate := make(chan struct{})

	defs := make([]resource.RelatedDef, numCheckers)
	for i := range numCheckers {
		targetType := "_t241_target_" + string(rune('a'+i))
		defs[i] = resource.RelatedDef{
			TargetType:       targetType,
			DisplayName:      "Checker " + string(rune('a'+i)),
			NeedsTargetCache: false, // field-only so no prefetch; isolates concurrency test
			Checker: func(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
				n := atomic.AddInt64(&concurrentNow, 1)
				mu.Lock()
				if n > maxSeen {
					maxSeen = n
				}
				mu.Unlock()

				select {
				case <-gate:
				case <-time.After(2 * time.Second):
				}

				atomic.AddInt64(&concurrentNow, -1)
				return resource.KnownRelated(targetType, []string{targetType + "-related"}, false)
			},
		}
	}

	resource.SetRelatedForTest(srcType, defs)
	t.Cleanup(func() { resource.CleanupRelatedForTest(srcType) })

	m := newBlessedModel(t, "testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	srcRes := resource.Resource{ID: "src-241"}

	_, batchCmd := rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		ResourceType: srcType,
		Resource:     &srcRes,
	})
	if batchCmd == nil {
		t.Fatal("handleRelatedCheckStarted returned nil")
	}

	rawMsg := batchCmd()
	batchMsg, ok := rawMsg.(tea.BatchMsg)
	if !ok {
		t.Fatalf("expected tea.BatchMsg, got %T", rawMsg)
	}

	var wg sync.WaitGroup
	for _, cmd := range batchMsg {
		if cmd == nil {
			continue
		}
		wg.Go(func() {
			cmd()
		})
	}

	// 10ms lets the goroutines reach the select before the gate is released.
	time.Sleep(10 * time.Millisecond)
	close(gate)
	wg.Wait()

	if maxSeen > maxAllowed {
		t.Errorf("concurrency cap violated: %d checkers ran simultaneously (max allowed: %d); "+
			"fix: add semaphore with cap=%d in handleRelatedCheckStarted", maxSeen, maxAllowed, maxAllowed)
	}
}
