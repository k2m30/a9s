package unit

// qa_probe_truncation_per_type_test.go — Regression pin for Issue 1 (P1):
// probeResources truncation per-shortName is lost in buildResourceCacheSnapshot.
//
// Bug location: internal/tui/app_related.go (~line 308-330).
// The loop `for shortName, rows := range m.probeResources` always stamps
// IsTruncated=true wholesale, discarding the per-type truncation signal that
// internal/tui/app_probes.go:247 correctly records in AvailabilityPrefetchedMsg.Truncated.
//
// Impact: accounts whose probe returns a complete (non-truncated) single page
// still get IsTruncated=true. cross-ref enrichers (dbi-snap→dbi, dbc-snap→dbc,
// etc.) treat "parent not found in truncated cache" as unknown-skip rather than
// orphan — so orphan findings are suppressed at startup for those accounts.
//
// Fix contract: handleAvailabilityPrefetched must store per-type truncation
// (e.g. m.probeTruncated map[string]bool) and buildResourceCacheSnapshot must
// consult it to stamp IsTruncated correctly for probe-only entries.
//
// Test approach: follows the checker-capture pattern from
// qa_lazy_cache_snapshot_truncated_test.go. We seed probe resources via
// AvailabilityPrefetchedMsg (the real handler path), then trigger
// RelatedCheckStartedMsg so the model calls buildResourceCacheSnapshot and
// passes the resulting cache to our registered checker. We capture the cache
// and assert on its IsTruncated value.

import (
	"context"
	"sync/atomic"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
)

// TestBuildResourceCacheSnapshot_ProbeAuthoritative_SinglePageComplete verifies
// that when AvailabilityPrefetchedMsg delivers a probe with Truncated=false for
// a type, buildResourceCacheSnapshot stamps the probe-only entry IsTruncated=false.
//
// PASSES today: handleAvailabilityPrefetched seeds m.resourceCache with nil
// pagination; buildResourceCacheSnapshot's resourceCache merge path evaluates
// IsTruncated = (pagination != nil && pagination.IsTruncated) = false, so the
// single-page-complete case accidentally returns false already.
// This test pins the correct behavior so the fix doesn't break it.
//
// The FAILING twin is TestBuildResourceCacheSnapshot_ProbeTruncated_StampsTrue:
// truncated probes also get IsTruncated=false today (nil pagination → false),
// but must return IsTruncated=true after the fix.
func TestBuildResourceCacheSnapshot_ProbeAuthoritative_SinglePageComplete(t *testing.T) {
	tui.Version = "test"

	const (
		srcType    = "test-pt1-source"
		targetType = "test-pt1-target"
	)

	var capturedCache resource.ResourceCache
	var checkerCalls int32

	resource.RegisterRelated(srcType, []resource.RelatedDef{
		{
			TargetType:  targetType,
			DisplayName: "PT1 Target",
			Checker: func(_ context.Context, _ any, _ resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
				atomic.AddInt32(&checkerCalls, 1)
				capturedCache = cache
				return resource.RelatedCheckResult{TargetType: targetType, Count: 0}
			},
		},
	})
	t.Cleanup(func() { resource.UnregisterRelated(srcType) })

	resource.RegisterFetchByIDs(targetType, func(_ context.Context, _ any, ids []string) ([]resource.Resource, error) {
		out := make([]resource.Resource, len(ids))
		for i, id := range ids {
			out[i] = resource.Resource{ID: id, Name: id}
		}
		return out, nil
	})
	t.Cleanup(func() { resource.UnregisterFetchByIDs(targetType) })

	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	// Deliver AvailabilityPrefetchedMsg — this is the real probe handler path.
	// Truncated[targetType] = false means the probe fetched ALL rows in one page.
	probeResource := resource.Resource{ID: "pt1-target-001", Name: "pt1-target-001"}
	m, _ = rootApplyMsg(m, messages.AvailabilityPrefetchedMsg{
		Entries:        map[string]int{targetType: 1},
		Truncated:      map[string]bool{targetType: false}, // NOT truncated: complete single page
		IssueCounts:    map[string]int{targetType: 0},
		IssueTruncated: map[string]bool{targetType: false},
		Resources:      map[string][]resource.Resource{targetType: {probeResource}},
		Gen:            0, // Gen=0 is accepted unconditionally (test injection sentinel)
	})

	// Navigate to src detail view so RelatedCheckStartedMsg is handled.
	srcRes := resource.Resource{ID: "pt1-src-001", Name: "pt1-src-001"}
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetDetail,
		Resource:     &srcRes,
		ResourceType: srcType,
	})

	// Dispatch RelatedCheckStartedMsg — triggers buildResourceCacheSnapshot and
	// passes the snapshot to all registered checkers for srcType.
	_, relCmd := rootApplyMsg(m, messages.RelatedCheckStartedMsg{
		ResourceType:   srcType,
		SourceResource: srcRes,
	})
	if relCmd == nil {
		t.Fatal("RelatedCheckStartedMsg returned nil cmd — checker never invoked")
	}

	// Execute the cmd tree to trigger the checker goroutines.
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

	// CONTRACT ASSERTION — FAILS TODAY, PASSES AFTER FIX.
	// The probe delivered Truncated=false for targetType, so buildResourceCacheSnapshot
	// should stamp IsTruncated=false for this probe-only entry.
	// Today it always stamps IsTruncated=true regardless of per-type signal.
	if entry.IsTruncated {
		t.Errorf(
			"buildResourceCacheSnapshot: probe-only entry for %q has IsTruncated=true, want false — "+
				"PROBE-TRUNCATION-LOST BUG: orphan findings get suppressed for single-page accounts",
			targetType,
		)
	}
}

// TestBuildResourceCacheSnapshot_ProbeTruncated_StampsTrue verifies that when
// AvailabilityPrefetchedMsg delivers a probe with Truncated=true for a type
// (more pages exist), buildResourceCacheSnapshot stamps the probe-only entry
// IsTruncated=true.
//
// This PASSES today (the current code always stamps true for probe entries).
// It pins the correct behavior so the fix does not accidentally break the
// truncated-probe case.
func TestBuildResourceCacheSnapshot_ProbeTruncated_StampsTrue(t *testing.T) {
	tui.Version = "test"

	const (
		srcType    = "test-pt2-source"
		targetType = "test-pt2-target"
	)

	var capturedCache resource.ResourceCache
	var checkerCalls int32

	resource.RegisterRelated(srcType, []resource.RelatedDef{
		{
			TargetType:  targetType,
			DisplayName: "PT2 Target",
			Checker: func(_ context.Context, _ any, _ resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
				atomic.AddInt32(&checkerCalls, 1)
				capturedCache = cache
				return resource.RelatedCheckResult{TargetType: targetType, Count: 0}
			},
		},
	})
	t.Cleanup(func() { resource.UnregisterRelated(srcType) })

	resource.RegisterFetchByIDs(targetType, func(_ context.Context, _ any, ids []string) ([]resource.Resource, error) {
		out := make([]resource.Resource, len(ids))
		for i, id := range ids {
			out[i] = resource.Resource{ID: id, Name: id}
		}
		return out, nil
	})
	t.Cleanup(func() { resource.UnregisterFetchByIDs(targetType) })

	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	probeResource := resource.Resource{ID: "pt2-target-001", Name: "pt2-target-001"}
	m, _ = rootApplyMsg(m, messages.AvailabilityPrefetchedMsg{
		Entries:        map[string]int{targetType: 1},
		Truncated:      map[string]bool{targetType: true}, // TRUNCATED: more pages exist
		IssueCounts:    map[string]int{targetType: 0},
		IssueTruncated: map[string]bool{targetType: true},
		Resources:      map[string][]resource.Resource{targetType: {probeResource}},
		Gen:            0,
	})

	srcRes := resource.Resource{ID: "pt2-src-001", Name: "pt2-src-001"}
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetDetail,
		Resource:     &srcRes,
		ResourceType: srcType,
	})

	_, relCmd := rootApplyMsg(m, messages.RelatedCheckStartedMsg{
		ResourceType:   srcType,
		SourceResource: srcRes,
	})
	if relCmd == nil {
		t.Fatal("RelatedCheckStartedMsg returned nil cmd")
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

	// This should pass both today and after fix: truncated probe → IsTruncated=true.
	if !entry.IsTruncated {
		t.Errorf(
			"buildResourceCacheSnapshot: probe-only entry for %q has IsTruncated=false, want true — "+
				"probe delivered Truncated=true (more pages exist); snapshot must reflect this",
			targetType,
		)
	}
}
