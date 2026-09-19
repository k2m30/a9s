package unit

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
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

// FetchByIDs receives the related IDs deduplicated in first-appearance order.
func TestLazyAdd_MissingFromCache_DedupsRepeatedIDsInChecker(t *testing.T) {
	const (
		srcType    = "test-lazy-dedup-source"
		targetType = "test-lazy-dedup-target"
	)

	var capturedIDs []string
	var capturedOnce atomic.Bool

	resource.SetRelatedForTest(srcType, []resource.RelatedDef{
		{
			TargetType:       targetType,
			DisplayName:      "Dedup Test Target",
			NeedsTargetCache: false,
			// KnownRelated dedupes ids itself, so Count is len(uniqueIDs).
			Checker: func(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
				return resource.KnownRelated(targetType, []string{"idA", "idA", "idB", "idB", "idA"}, false)
			},
		},
	})

	resource.SetFetchByIDsForTest(targetType, func(_ context.Context, _ any, ids []string) ([]resource.Resource, error) {
		if capturedOnce.CompareAndSwap(false, true) {
			cp := make([]string, len(ids))
			copy(cp, ids)
			capturedIDs = cp
		}
		var out []resource.Resource
		for _, id := range ids {
			out = append(out, resource.Resource{ID: id, Name: id})
		}
		return out, nil
	})

	t.Cleanup(func() {
		resource.CleanupRelatedForTest(srcType)
		resource.CleanupFetchByIDsForTest(targetType)
	})

	m := newBlessedModel(t, "testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	srcRes := resource.Resource{ID: "src-dedup-001"}

	_, batchCmd := rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		ResourceType: srcType,
		Resource:     &srcRes,
	})

	resultMsg, found := collectRelatedResult(t, batchCmd)
	if !found {
		t.Fatal("no RelatedCheckResultMsg received")
	}
	if resultMsg.Result.Count() != 2 {
		t.Errorf("Result.Count: got %d, want 2", resultMsg.Result.Count())
	}

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

// A FetchByIDs error still delivers the checker result with its Count, and
// with no lazy-added resources.
func TestLazyAdd_FetchByIDsErrorSwallowed_ChecksResultStillDelivered(t *testing.T) {
	const (
		srcType    = "test-lazy-error-source"
		targetType = "test-lazy-error-target"
	)

	resource.SetRelatedForTest(srcType, []resource.RelatedDef{
		{
			TargetType:       targetType,
			DisplayName:      "Error Swallow Test Target",
			NeedsTargetCache: false,
			Checker: func(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
				return resource.KnownRelated(targetType, []string{"id-not-in-cache"}, false)
			},
		},
	})

	resource.SetFetchByIDsForTest(targetType, func(_ context.Context, _ any, _ []string) ([]resource.Resource, error) {
		return nil, errors.New("simulated aws failure")
	})

	t.Cleanup(func() {
		resource.CleanupRelatedForTest(srcType)
		resource.CleanupFetchByIDsForTest(targetType)
	})

	m := newBlessedModel(t, "testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	srcRes := resource.Resource{ID: "src-error-001"}

	_, batchCmd := rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		ResourceType: srcType,
		Resource:     &srcRes,
	})

	resultMsg, found := collectRelatedResult(t, batchCmd)
	if !found {
		t.Fatal("no RelatedCheckResultMsg received — error may have leaked out instead of being swallowed")
	}

	if resultMsg.Result.Count() != 1 {
		t.Errorf("Result.Count: got %d, want 1 (checker result must survive FetchByIDs error)", resultMsg.Result.Count())
	}

	if resultMsg.LazyAddedResources != nil {
		t.Errorf("LazyAddedResources should be nil when FetchByIDs errors; got %v", resultMsg.LazyAddedResources)
	}

	if resultMsg.CachedPages != nil {
		t.Errorf("CachedPages should be nil (NeedsTargetCache=false); got %v", resultMsg.CachedPages)
	}

}
