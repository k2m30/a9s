package unit

// issue237_239_240_241_related_fixes_test.go — regression tests for related-view
// infrastructure fixes from code review of branch 006-related-views-infra.
//
// Business rules verified:
//
//   #237 — Cold-miss write-back must preserve NextToken so pagination past the
//           probe's first page works when the user opens the resource from the main menu.
//
//   #239 — RelatedCheckResultMsg from a previous check batch (wrong generation)
//           must be silently discarded; only results from the current generation
//           should update the right column. Prevents stale counts after Ctrl+R,
//           profile switch, and region switch.
//
//   #240 — Field-only related checkers (NeedsTargetCache=false) must NOT trigger
//           a cold-cache prefetch of the target type. Only checkers that actually
//           read the target cache should pay the API cost of a cold fetch.
//
//   #241 — At most maxConcurrentProbes (4) related checkers should run concurrently
//           for a single detail view, matching the architecture spec.

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
)

// ---------------------------------------------------------------------------
// #237 — NextToken preserved in cold-miss write-back
// ---------------------------------------------------------------------------

// TestIssue237_ColdMissWriteBack_PreservesNextToken verifies that when a related
// checker cold-fetches a target type, the pagination token from the AWS response is
// preserved end-to-end so that users can advance past the probe's first page.
//
// Given: a target type whose paginated fetcher returns a first page with NextToken
// When:  a related checker cold-misses and triggers a prefetch (NeedsTargetCache=true)
// Then:  the RelatedCheckResultMsg.CachedPages entry carries the full Pagination
//
//	(including NextToken), not a synthetic PaginationMeta with an empty token
func TestIssue237_ColdMissWriteBack_PreservesNextToken(t *testing.T) {
	const (
		srcType    = "_t237_src"
		targetType = "_t237_target"
		wantToken  = "next-page-token-abc123"
	)

	// Register a paginated fetcher that returns a truncated first page with a token.
	resource.RegisterPaginated(targetType, func(_ context.Context, _ any, _ string) (resource.FetchResult, error) {
		return resource.FetchResult{
			Resources: []resource.Resource{{ID: "r1"}},
			Pagination: &resource.PaginationMeta{
				IsTruncated: true,
				NextToken:   wantToken,
			},
		}, nil
	})

	// Register a related def that reads the target from cache (NeedsTargetCache=true),
	// ensuring the cold-miss prefetch path is exercised.
	resource.RegisterRelated(srcType, []resource.RelatedDef{
		{
			TargetType:       targetType,
			DisplayName:      "Test Target",
			NeedsTargetCache: true,
			Checker: func(_ context.Context, _ any, _ resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
				entry, ok := cache[targetType]
				if !ok {
					return resource.RelatedCheckResult{Count: -1}
				}
				return resource.RelatedCheckResult{Count: len(entry.Resources)}
			},
		},
	})

	t.Cleanup(func() {
		resource.UnregisterRelated(srcType)
		resource.UnregisterPaginated(targetType)
	})

	m := tui.New("testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	srcRes := resource.Resource{ID: "src-237-instance"}

	// Dispatch: returns a batch of checker cmds.
	_, batchCmd := rootApplyMsg(m, messages.RelatedCheckStarted{
		ResourceType:   srcType,
		SourceResource: srcRes,
	})
	if batchCmd == nil {
		t.Fatal("handleRelatedCheckStarted returned nil — expected checker batch")
	}

	// Execute the batch. With a single def, Bubble Tea may return the result
	// directly rather than wrapping it in tea.BatchMsg — handle both cases.
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

	// The checker cold-missed, so CachedPages must carry the full Pagination.
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

// ---------------------------------------------------------------------------
// #239 — Stale related-check results discarded after Ctrl+R
// ---------------------------------------------------------------------------

// TestIssue239_StaleGenerationResult_IsDiscarded verifies that a RelatedCheckResultMsg
// carrying a non-zero generation that doesn't match the current relatedGen is silently
// dropped and does not update the right column.
//
// Business rule: the right column must not show counts from a previous check batch
// after the user has triggered a refresh (Ctrl+R).
//
// Mechanism: relatedGen starts at 1. The initial batch stamps gen=1. Ctrl+R
// increments relatedGen to 2. Late results from the initial batch (gen=1) must
// be discarded — they no longer match relatedGen=2.
//
// Observable proxy: if a gen=1 stale result is applied after relatedGen=2,
// the right column shows the stale count; if discarded, no count appears.
func TestIssue239_StaleGenerationResult_IsDiscarded(t *testing.T) {
	const viewedResourceID = "i-0a1b2c3d4e5f60001"

	// Set up EC2 detail view using demo fixtures so the right column is rendered.
	m := setupEC2DetailWithResults(t)

	// Verify precondition: the view shows related counts after setup.
	viewBefore := stripANSI(rootViewContent(m))
	if !strings.Contains(viewBefore, "(2)") {
		t.Fatalf("precondition: expected '(2)' in view before test; got:\n%s", viewBefore)
	}

	// Trigger Ctrl+R: increments relatedGen from 1 to 2 and clears the relatedCache
	// entry so the right column returns to loading state.
	m, _ = rootApplyMsg(m, ctrlR())

	viewAfterRefresh := stripANSI(rootViewContent(m))
	if strings.Contains(viewAfterRefresh, "(2)") {
		t.Fatalf("precondition: after Ctrl+R stale '(2)' still visible — relatedCache not cleared:\n%s", viewAfterRefresh)
	}

	// Inject a result with gen=1 (the initial batch's generation, now stale since
	// relatedGen=2). This simulates a late arrival from the previous batch.
	m, _ = rootApplyMsg(m, messages.RelatedCheckResult{
		ResourceType:     "ec2",
		SourceResourceID: viewedResourceID,
		Generation:       1, // initial batch generation — stale after Ctrl+R (relatedGen=2)
		Result: resource.RelatedCheckResult{
			TargetType: "tg",
			Count:      99, // distinctive count — must NOT appear
		},
	})

	viewAfterStale := stripANSI(rootViewContent(m))
	if strings.Contains(viewAfterStale, "99") {
		t.Errorf("stale gen=1 result applied after relatedGen=2 — it must be discarded.\n"+
			"Fix: the initial relatedGen must be >0 so gen=0 (unset) is always stale "+
			"and gen=1 results are correctly rejected after the first Ctrl+R.\n"+
			"View:\n%s", viewAfterStale)
	}
}

// TestIssue239_CurrentGenerationResult_IsAccepted verifies that results stamped with
// the current relatedGen ARE applied to the right column.
//
// After Ctrl+R, relatedGen becomes 2. A result with gen=2 must be accepted;
// a result with gen=0 (test injection sentinel) must also be accepted.
func TestIssue239_CurrentGenerationResult_IsAccepted(t *testing.T) {
	const viewedResourceID = "i-0a1b2c3d4e5f60001"

	m := setupEC2DetailWithResults(t)
	m, _ = rootApplyMsg(m, ctrlR()) // relatedGen: 1 → 2

	// gen=2 matches relatedGen=2 → accepted.
	m, _ = rootApplyMsg(m, messages.RelatedCheckResult{
		ResourceType:     "ec2",
		SourceResourceID: viewedResourceID,
		Generation:       2, // current generation after one Ctrl+R
		Result: resource.RelatedCheckResult{
			TargetType: "tg",
			Count:      7,
		},
	})

	view := stripANSI(rootViewContent(m))
	if !strings.Contains(view, "(7)") {
		t.Errorf("gen=2 result should be accepted when relatedGen=2; right column should show '(7)'.\nView:\n%s", view)
	}
}

// ---------------------------------------------------------------------------
// #240 — Field-only checkers skip cold-cache prefetch
// ---------------------------------------------------------------------------

// TestIssue240_FieldOnlyChecker_NoPrefetch verifies that a checker with
// NeedsTargetCache=false does NOT trigger a cold-cache API call, even when the
// target type is absent from the resource cache.
//
// Given: a related def with NeedsTargetCache=false and a checker that ignores the cache
// When:  RelatedCheckStartedMsg is dispatched with no existing cache
// Then:  RelatedCheckResultMsg.CachedPages is nil — no AWS API call was made
func TestIssue240_FieldOnlyChecker_NoPrefetch(t *testing.T) {
	const (
		srcType    = "_t240_src_field"
		targetType = "_t240_target_field"
	)

	fetchCalled := atomic.Bool{}

	resource.RegisterPaginated(targetType, func(_ context.Context, _ any, _ string) (resource.FetchResult, error) {
		fetchCalled.Store(true) // must NOT be called for field-only checker
		return resource.FetchResult{Resources: []resource.Resource{{ID: "should-not-fetch"}}}, nil
	})

	resource.RegisterRelated(srcType, []resource.RelatedDef{
		{
			TargetType:       targetType,
			DisplayName:      "Field-Only Checker",
			NeedsTargetCache: false, // field-only: derives result from source, not target cache
			Checker: func(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
				// Derive result purely from source resource fields — no cache reads.
				if res.Fields["has_target"] == "true" {
					return resource.RelatedCheckResult{Count: 1, ResourceIDs: []string{"derived-id"}}
				}
				return resource.RelatedCheckResult{Count: 0}
			},
		},
	})

	t.Cleanup(func() {
		resource.UnregisterRelated(srcType)
		resource.UnregisterPaginated(targetType)
	})

	m := tui.New("testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	srcRes := resource.Resource{
		ID:     "src-240",
		Fields: map[string]string{"has_target": "true"},
	}

	_, batchCmd := rootApplyMsg(m, messages.RelatedCheckStarted{
		ResourceType:   srcType,
		SourceResource: srcRes,
	})
	if batchCmd == nil {
		t.Fatal("handleRelatedCheckStarted returned nil")
	}

	// Collect all RelatedCheckResultMsg values (handle single-def and multi-def batches).
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

// TestIssue240_CacheDependentChecker_DoesPrefetch verifies the positive case:
// a checker with NeedsTargetCache=true DOES trigger the cold-cache prefetch.
//
// Given: a related def with NeedsTargetCache=true
// When:  the target type is absent from the resource cache
// Then:  the paginated fetcher is called and CachedPages is non-nil
func TestIssue240_CacheDependentChecker_DoesPrefetch(t *testing.T) {
	const (
		srcType    = "_t240_src_cache"
		targetType = "_t240_target_cache"
	)

	fetchCalled := atomic.Bool{}

	resource.RegisterPaginated(targetType, func(_ context.Context, _ any, _ string) (resource.FetchResult, error) {
		fetchCalled.Store(true)
		return resource.FetchResult{
			Resources:  []resource.Resource{{ID: "t1"}},
			Pagination: &resource.PaginationMeta{IsTruncated: false},
		}, nil
	})

	resource.RegisterRelated(srcType, []resource.RelatedDef{
		{
			TargetType:       targetType,
			DisplayName:      "Cache-Dependent Checker",
			NeedsTargetCache: true,
			Checker: func(_ context.Context, _ any, _ resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
				entry, ok := cache[targetType]
				if !ok {
					return resource.RelatedCheckResult{Count: -1}
				}
				return resource.RelatedCheckResult{Count: len(entry.Resources)}
			},
		},
	})

	t.Cleanup(func() {
		resource.UnregisterRelated(srcType)
		resource.UnregisterPaginated(targetType)
	})

	m := tui.New("testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	srcRes := resource.Resource{ID: "src-240-cache"}

	_, batchCmd := rootApplyMsg(m, messages.RelatedCheckStarted{
		ResourceType:   srcType,
		SourceResource: srcRes,
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

// ---------------------------------------------------------------------------
// #241 — Concurrency cap: at most 4 checkers run simultaneously
// ---------------------------------------------------------------------------

// TestIssue241_ConcurrentProbesCappedAt4 verifies that when more than 4 related
// checkers are registered, at most 4 run simultaneously.
//
// Business rule (architecture spec): max 4 concurrent probes per detail view
// to avoid saturating AWS API rate limits.
//
// Method: register 8 checkers, each of which blocks on a gate channel until released.
// Count the maximum number observed running simultaneously using an atomic counter.
// The max concurrent count must be <= 4.
func TestIssue241_ConcurrentProbesCappedAt4(t *testing.T) {
	const (
		srcType     = "_t241_src"
		numCheckers = 8 // more than maxConcurrentProbes (4)
		maxAllowed  = 4
	)

	// Register 8 target types.
	for i := range numCheckers {
		targetType := "_t241_target_" + string(rune('a'+i))
		idx := i
		resource.RegisterPaginated(targetType, func(_ context.Context, _ any, _ string) (resource.FetchResult, error) {
			return resource.FetchResult{
				Resources:  []resource.Resource{{ID: "r" + string(rune('a'+idx))}},
				Pagination: &resource.PaginationMeta{IsTruncated: false},
			}, nil
		})
		t.Cleanup(func() { resource.UnregisterPaginated(targetType) })
	}

	var (
		concurrentNow int64 // currently running checkers
		maxSeen       int64 // maximum observed simultaneously
		mu            sync.Mutex
	)

	// gate blocks each checker until all are known to have started (or been gated).
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

				// Block until the test releases the gate or until timeout.
				select {
				case <-gate:
				case <-time.After(2 * time.Second):
				}

				atomic.AddInt64(&concurrentNow, -1)
				return resource.RelatedCheckResult{Count: 1}
			},
		}
	}

	resource.RegisterRelated(srcType, defs)
	t.Cleanup(func() { resource.UnregisterRelated(srcType) })

	m := tui.New("testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	srcRes := resource.Resource{ID: "src-241"}

	_, batchCmd := rootApplyMsg(m, messages.RelatedCheckStarted{
		ResourceType:   srcType,
		SourceResource: srcRes,
	})
	if batchCmd == nil {
		t.Fatal("handleRelatedCheckStarted returned nil")
	}

	rawMsg := batchCmd()
	batchMsg, ok := rawMsg.(tea.BatchMsg)
	if !ok {
		t.Fatalf("expected tea.BatchMsg, got %T", rawMsg)
	}

	// Launch all checker cmds concurrently, as tea.Batch would.
	var wg sync.WaitGroup
	for _, cmd := range batchMsg {
		if cmd == nil {
			continue
		}
		wg.Go(func() {
			cmd()
		})
	}

	// Allow checkers to start and stabilize, then release the gate.
	// 10ms is sufficient for goroutines to reach the select statement.
	time.Sleep(10 * time.Millisecond)
	close(gate)
	wg.Wait()

	if maxSeen > maxAllowed {
		t.Errorf("concurrency cap violated: %d checkers ran simultaneously (max allowed: %d); "+
			"fix: add semaphore with cap=%d in handleRelatedCheckStarted", maxSeen, maxAllowed, maxAllowed)
	}
}
