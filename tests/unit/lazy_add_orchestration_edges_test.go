package unit

// lazy_add_orchestration_edges_test.go — pin tests for the lazy-add path in
// handleRelatedCheckStarted (internal/tui/app_related.go).
//
// Gap 2: missingFromCache dedup-within-input (the seen-map that strips repeated
//         IDs before calling FetchByIDs — app_related.go:394-401).
//
// Gap 3: FetchByIDs error swallowed — the RelatedCheckResultMsg is still
//         delivered with LazyAddedResources==nil and the checker's Count intact.

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
)

// collectRelatedResult runs batchCmd() and collects the first
// RelatedCheckResultMsg found — handles both single-cmd and tea.BatchMsg.
func collectRelatedResult(t *testing.T, batchCmd tea.Cmd) (messages.RelatedCheckResult, bool) {
	t.Helper()
	if batchCmd == nil {
		t.Fatal("batchCmd is nil — handleRelatedCheckStarted returned no command")
	}
	rawMsg := batchCmd()
	switch v := rawMsg.(type) {
	case messages.RelatedCheckResult:
		return v, true
	case tea.BatchMsg:
		for _, cmd := range v {
			if cmd == nil {
				continue
			}
			if r, ok := cmd().(messages.RelatedCheckResult); ok {
				return r, true
			}
		}
	}
	return messages.RelatedCheckResult{}, false
}

// ---------------------------------------------------------------------------
// Gap 2 — missingFromCache deduplicates repeated IDs before calling FetchByIDs
// ---------------------------------------------------------------------------

// TestLazyAdd_MissingFromCache_DedupsRepeatedIDsInChecker verifies that when a
// checker emits duplicate ResourceIDs (e.g. ["idA","idA","idB","idB","idA"]),
// the FetchByIDs call receives the deduplicated, first-appearance-ordered slice
// (["idA","idB"]) rather than the raw repeated list.
//
// This covers the seen-map branch in missingFromCache (app_related.go:394-401).
func TestLazyAdd_MissingFromCache_DedupsRepeatedIDsInChecker(t *testing.T) {
	const (
		srcType    = "test-lazy-dedup-source"
		targetType = "test-lazy-dedup-target"
	)

	// capturedIDs stores the exact ids slice FetchByIDs was called with.
	var capturedIDs []string
	var capturedOnce atomic.Bool

	resource.RegisterRelated(srcType, []resource.RelatedDef{
		{
			TargetType:       targetType,
			DisplayName:      "Dedup Test Target",
			NeedsTargetCache: false,
			Checker: func(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
				return resource.RelatedCheckResult{
					TargetType:  targetType,
					Count:       3,
					ResourceIDs: []string{"idA", "idA", "idB", "idB", "idA"},
				}
			},
		},
	})

	resource.RegisterFetchByIDs(targetType, func(_ context.Context, _ any, ids []string) ([]resource.Resource, error) {
		if capturedOnce.CompareAndSwap(false, true) {
			cp := make([]string, len(ids))
			copy(cp, ids)
			capturedIDs = cp
		}
		// Return a resource per id so lazyAdded is non-empty (triggers merge path).
		var out []resource.Resource
		for _, id := range ids {
			out = append(out, resource.Resource{ID: id, Name: id})
		}
		return out, nil
	})

	t.Cleanup(func() {
		resource.UnregisterRelated(srcType)
		resource.UnregisterFetchByIDs(targetType)
	})

	m := tui.New("testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	srcRes := resource.Resource{ID: "src-dedup-001"}

	_, batchCmd := rootApplyMsg(m, messages.RelatedCheckStarted{
		ResourceType:   srcType,
		SourceResource: srcRes,
	})

	resultMsg, found := collectRelatedResult(t, batchCmd)
	if !found {
		t.Fatal("no RelatedCheckResultMsg received")
	}
	// Checker's count must pass through unchanged.
	if resultMsg.Result.Count != 3 {
		t.Errorf("Result.Count: got %d, want 3", resultMsg.Result.Count)
	}

	// The dedup assertion — this is the core of Gap 2.
	if !capturedOnce.Load() {
		t.Fatal("FetchByIDs was not called — lazy-add path was not exercised")
	}
	wantIDs := []string{"idA", "idB"}
	if len(capturedIDs) != len(wantIDs) {
		t.Fatalf("FetchByIDs received %d IDs %v, want %d %v (duplicates not removed)",
			len(capturedIDs), capturedIDs, len(wantIDs), wantIDs)
	}
	for i, want := range wantIDs {
		if capturedIDs[i] != want {
			t.Errorf("FetchByIDs ids[%d]=%q, want %q", i, capturedIDs[i], want)
		}
	}
}

// ---------------------------------------------------------------------------
// Gap 3 — FetchByIDs error swallowed; checker result still delivered
// ---------------------------------------------------------------------------

// TestLazyAdd_FetchByIDsErrorSwallowed_ChecksResultStillDelivered verifies
// that when the registered FetchByIDs returns an error, the orchestrator:
//   - Does NOT propagate the error (no panic, no hang).
//   - Still delivers the RelatedCheckResultMsg with the checker's Count intact.
//   - Sets LazyAddedResources to nil (no partial data written).
//   - Leaves CachedPages nil (NeedsTargetCache=false avoids prefetch).
//
// Covers the `if extra, err := ff(ctx, m.clients, missing); err == nil` guard
// in handleRelatedCheckStarted (app_related.go:121).
func TestLazyAdd_FetchByIDsErrorSwallowed_ChecksResultStillDelivered(t *testing.T) {
	const (
		srcType    = "test-lazy-error-source"
		targetType = "test-lazy-error-target"
	)

	resource.RegisterRelated(srcType, []resource.RelatedDef{
		{
			TargetType:       targetType,
			DisplayName:      "Error Swallow Test Target",
			NeedsTargetCache: false,
			Checker: func(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
				return resource.RelatedCheckResult{
					TargetType:  targetType,
					Count:       1,
					ResourceIDs: []string{"id-not-in-cache"},
				}
			},
		},
	})

	resource.RegisterFetchByIDs(targetType, func(_ context.Context, _ any, _ []string) ([]resource.Resource, error) {
		return nil, errors.New("simulated aws failure")
	})

	t.Cleanup(func() {
		resource.UnregisterRelated(srcType)
		resource.UnregisterFetchByIDs(targetType)
	})

	m := tui.New("testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	srcRes := resource.Resource{ID: "src-error-001"}

	_, batchCmd := rootApplyMsg(m, messages.RelatedCheckStarted{
		ResourceType:   srcType,
		SourceResource: srcRes,
	})

	resultMsg, found := collectRelatedResult(t, batchCmd)
	if !found {
		t.Fatal("no RelatedCheckResultMsg received — error may have leaked out instead of being swallowed")
	}

	// Checker's original count must survive the FetchByIDs error.
	if resultMsg.Result.Count != 1 {
		t.Errorf("Result.Count: got %d, want 1 (checker result must survive FetchByIDs error)", resultMsg.Result.Count)
	}

	// No partial lazy data should appear.
	if resultMsg.LazyAddedResources != nil {
		t.Errorf("LazyAddedResources should be nil when FetchByIDs errors; got %v", resultMsg.LazyAddedResources)
	}

	// No prefetch happened (NeedsTargetCache=false).
	if resultMsg.CachedPages != nil {
		t.Errorf("CachedPages should be nil (NeedsTargetCache=false); got %v", resultMsg.CachedPages)
	}

}
