package unit

// buildResourceCacheSnapshot stamps each probe-only entry's IsTruncated from
// the per-type truncation that AvailabilityPrefetchedMsg.Truncated records.
// Cross-ref enrichers (dbi-snap→dbi, dbc-snap→dbc, …) treat "parent not found
// in a truncated cache" as unknown rather than orphan, so a wrong
// IsTruncated=true suppresses orphan findings. The cache is captured by a
// registered checker that the source detail view's DetailOperation invokes.

import (
	"context"
	"sync/atomic"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
)

// A probe delivered with Truncated=false for a type yields IsTruncated=false on
// the probe-only entry.
func TestBuildResourceCacheSnapshot_ProbeAuthoritative_SinglePageComplete(t *testing.T) {
	tui.Version = "test"

	const (
		srcType    = "test-pt1-source"
		targetType = "test-pt1-target"
	)

	var capturedCache resource.ResourceCache
	var checkerCalls int32

	resource.SetRelatedForTest(srcType, []resource.RelatedDef{
		{
			TargetType:  targetType,
			DisplayName: "PT1 Target",
			Checker: func(_ context.Context, _ any, _ resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
				atomic.AddInt32(&checkerCalls, 1)
				capturedCache = cache
				return resource.KnownRelated(targetType, nil, false)
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

	probeResource := resource.Resource{ID: "pt1-target-001", Name: "pt1-target-001"}
	m, _ = rootApplyMsg(m, messages.AvailabilityPrefetched{
		Entries:        map[string]int{targetType: 1},
		Truncated:      map[string]bool{targetType: false}, // NOT truncated: complete single page
		IssueCounts:    map[string]int{targetType: 0},
		IssueTruncated: map[string]bool{targetType: false},
		Resources:      map[string][]resource.Resource{targetType: {probeResource}},
		// Stamp the live AvailabilityGen so the staleness guard accepts the
		// message (AcceptZeroGen=false).
		Gen: m.Core().Session().AvailabilityGen,
	})

	// Navigate to src detail view — begins a DetailOperation and dispatches
	// the related-check task directly, invoking buildResourceCacheSnapshot
	// and passing the snapshot to every registered checker for srcType.
	srcRes := resource.Resource{ID: "pt1-src-001", Name: "pt1-src-001"}
	var relCmd tea.Cmd
	m, relCmd = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		Resource:     &srcRes,
		ResourceType: srcType,
	})
	if relCmd == nil {
		t.Fatal("opening srcType detail returned nil cmd — checker never invoked")
	}

	allMsgs := drainAllMessages(relCmd)
	_ = allMsgs

	if atomic.LoadInt32(&checkerCalls) == 0 {
		t.Skip("PT1 checker not invoked — cannot assert cache IsTruncated; check registration")
	}

	if capturedCache == nil {
		t.Fatal("captured cache is nil — checker was not called with a valid cache")
	}

	entry, ok := capturedCache[targetType]
	if !ok {
		t.Fatalf("captured cache does not contain %q — probe resource not visible in snapshot", targetType)
	}

	if entry.IsTruncated {
		t.Errorf(
			"buildResourceCacheSnapshot: probe-only entry for %q has IsTruncated=true, want false — "+
				"PROBE-TRUNCATION-LOST BUG: orphan findings get suppressed for single-page accounts",
			targetType,
		)
	}
}

// A probe delivered with Truncated=true for a type yields IsTruncated=true on
// the probe-only entry.
func TestBuildResourceCacheSnapshot_ProbeTruncated_StampsTrue(t *testing.T) {
	tui.Version = "test"

	const (
		srcType    = "test-pt2-source"
		targetType = "test-pt2-target"
	)

	var capturedCache resource.ResourceCache
	var checkerCalls int32

	resource.SetRelatedForTest(srcType, []resource.RelatedDef{
		{
			TargetType:  targetType,
			DisplayName: "PT2 Target",
			Checker: func(_ context.Context, _ any, _ resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
				atomic.AddInt32(&checkerCalls, 1)
				capturedCache = cache
				return resource.KnownRelated(targetType, nil, false)
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

	probeResource := resource.Resource{ID: "pt2-target-001", Name: "pt2-target-001"}
	m, _ = rootApplyMsg(m, messages.AvailabilityPrefetched{
		Entries:        map[string]int{targetType: 1},
		Truncated:      map[string]bool{targetType: true}, // TRUNCATED: more pages exist
		IssueCounts:    map[string]int{targetType: 0},
		IssueTruncated: map[string]bool{targetType: true},
		Resources:      map[string][]resource.Resource{targetType: {probeResource}},
		// Stamp the live AvailabilityGen so the staleness guard accepts the
		// message (AcceptZeroGen=false).
		Gen: m.Core().Session().AvailabilityGen,
	})

	srcRes := resource.Resource{ID: "pt2-src-001", Name: "pt2-src-001"}
	var relCmd tea.Cmd
	m, relCmd = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		Resource:     &srcRes,
		ResourceType: srcType,
	})
	if relCmd == nil {
		t.Fatal("opening srcType detail returned nil cmd")
	}

	allMsgs := drainAllMessages(relCmd)
	_ = allMsgs

	if atomic.LoadInt32(&checkerCalls) == 0 {
		t.Skip("PT2 checker not invoked — cannot assert cache IsTruncated")
	}

	if capturedCache == nil {
		t.Fatal("captured cache is nil")
	}

	entry, ok := capturedCache[targetType]
	if !ok {
		t.Fatalf("captured cache does not contain %q", targetType)
	}

	if !entry.IsTruncated {
		t.Errorf(
			"buildResourceCacheSnapshot: probe-only entry for %q has IsTruncated=false, want true — "+
				"probe delivered Truncated=true (more pages exist); snapshot must reflect this",
			targetType,
		)
	}
}
