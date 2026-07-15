// runtime_handlers_related_test.go — public-seam coverage for
// (*runtime.Core).HandleRelatedNavigate after the AS-150 migration moved the
// handler out of internal/tui into core/runtime.
//
// Cases A–K mirror the Stage 2 scope on AS-201. The runtime seam is exactly
// what AS-150 exposed — these tests stand up *runtime.Core directly through
// runtime.New(session.New(), catalog.All()) and assert the
// NavigationResult + []TaskRequest pair returned for each branch.
//
// HARD CONSTRAINT (per AS-203 acceptance): this file MUST NOT import
// charm.land/bubbletea/v2, lipgloss, or bubbles. The migration's whole point
// was decoupling the handler from Bubble Tea; bringing the framework back in
// here would defeat the test.
package unit

import (
	"context"
	"reflect"
	"testing"

	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/session"
)

// newRuntimeCore returns a fresh *runtime.Core bound to a clean session and
// the static catalog. Test cases mutate the returned session's caches
// directly to seed each branch.
func newRuntimeCore(t *testing.T) (*runtime.Core, *session.Session) {
	t.Helper()
	s := session.New()
	c := runtime.New(s, catalog.All())
	return c, s
}

// Case A — unknown target type → Flash with FlashIsError true, no tasks.
func TestHandleRelatedNavigate_UnknownType_Flash(t *testing.T) {
	c, _ := newRuntimeCore(t)

	result, tasks := c.HandleRelatedNavigate(runtime.RelatedNavigateEvent{
		TargetType: "definitely-not-a-real-type",
	})

	if result.Kind != runtime.NavigationKindFlash {
		t.Errorf("Kind = %v, want NavigationKindFlash", result.Kind)
	}
	if !result.FlashIsError {
		t.Error("FlashIsError = false, want true")
	}
	if len(tasks) != 0 {
		t.Errorf("len(tasks) = %d, want 0", len(tasks))
	}
}

// Case B — child type (e.g. "s3_objects") → EnterChildView, no tasks.
//
// Registers a transient child type for this test so the assertion does not
// depend on core/aws being imported (which would pull init() side effects
// into tests/unit).
func TestHandleRelatedNavigate_ChildType_EnterChildView(t *testing.T) {
	const childShort = "test_child_type_handle_related_b"
	resource.SetChildTypeForTest(resource.ResourceTypeDef{
		Name:      "Test Child",
		ShortName: childShort,
	})
	t.Cleanup(func() { resource.CleanupChildTypeForTest(childShort) })

	c, _ := newRuntimeCore(t)

	result, tasks := c.HandleRelatedNavigate(runtime.RelatedNavigateEvent{
		TargetType: childShort,
	})

	if result.Kind != runtime.NavigationKindEnterChildView {
		t.Errorf("Kind = %v, want NavigationKindEnterChildView", result.Kind)
	}
	if len(tasks) != 0 {
		t.Errorf("len(tasks) = %d, want 0", len(tasks))
	}
}

// Case C — top-level cache hit via TargetID → Detail, no tasks.
func TestHandleRelatedNavigate_TopLevelCacheHit_TargetID_Detail(t *testing.T) {
	c, s := newRuntimeCore(t)
	s.RowStore.Observe("ec2", []resource.Resource{{ID: "i-1"}}, nil, session.OriginFetch, false)

	result, tasks := c.HandleRelatedNavigate(runtime.RelatedNavigateEvent{
		TargetType: "ec2",
		TargetID:   "i-1",
	})

	if result.Kind != runtime.NavigationKindDetail {
		t.Errorf("Kind = %v, want NavigationKindDetail", result.Kind)
	}
	if result.TargetID != "i-1" {
		t.Errorf("TargetID = %q, want %q", result.TargetID, "i-1")
	}
	if len(tasks) != 0 {
		t.Errorf("len(tasks) = %d, want 0", len(tasks))
	}
}

// Case D — top-level cache hit via single RelatedIDs → Detail, no tasks.
func TestHandleRelatedNavigate_TopLevelCacheHit_SingleRelatedID_Detail(t *testing.T) {
	c, s := newRuntimeCore(t)
	s.RowStore.Observe("ec2", []resource.Resource{{ID: "i-1"}}, nil, session.OriginFetch, false)

	result, tasks := c.HandleRelatedNavigate(runtime.RelatedNavigateEvent{
		TargetType: "ec2",
		RelatedIDs: []string{"i-1"},
	})

	if result.Kind != runtime.NavigationKindDetail {
		t.Errorf("Kind = %v, want NavigationKindDetail", result.Kind)
	}
	if len(tasks) != 0 {
		t.Errorf("len(tasks) = %d, want 0", len(tasks))
	}
}

// Case E — FetchFilter + registered filtered fetcher → FilteredList with
// FetchFilter preserved and a single KindFetchFiltered task.
//
// Registers a no-op filtered paginated fetcher for the test type. t.Cleanup
// unregisters it so test order does not matter.
func TestHandleRelatedNavigate_FetchFilter_RegisteredFetcher_FilteredList(t *testing.T) {
	resource.SetFilteredPaginatedForTest("ec2",
		func(_ context.Context, _ any, _ map[string]string, _ string) (domain.FetchResult, error) {
			return domain.FetchResult{}, nil
		})
	t.Cleanup(func() { resource.CleanupFilteredPaginatedForTest("ec2") })

	c, _ := newRuntimeCore(t)

	filter := map[string]string{"vpc-id": "vpc-1"}
	result, tasks := c.HandleRelatedNavigate(runtime.RelatedNavigateEvent{
		TargetType:  "ec2",
		FetchFilter: filter,
	})

	if result.Kind != runtime.NavigationKindFilteredList {
		t.Errorf("Kind = %v, want NavigationKindFilteredList", result.Kind)
	}
	if !reflect.DeepEqual(result.FetchFilter, filter) {
		t.Errorf("FetchFilter = %v, want %v", result.FetchFilter, filter)
	}
	wantTasks := []runtime.TaskRequest{{
		Key:   runtime.TaskKey{Kind: runtime.KindFetchFiltered, Scope: "ec2"},
		Cache: runtime.CacheNone,
	}}
	if !reflect.DeepEqual(tasks, wantTasks) {
		t.Errorf("tasks = %+v, want %+v", tasks, wantTasks)
	}
}

// Case F — TargetID cache miss (no filtered fetcher, no cache entry) on a
// by-ID-capable type → FilteredList with FilterText==TargetID and a single
// KindFetchByIDDetail task.
//
// "ec2" now registers FetchByIDs (core/aws/catalog_compute.go — the
// costs resource-row navigation jump added it), so this case moved from the
// KindFetchResources else-branch to the KindFetchByIDDetail branch. The
// KindFetchResources-else-branch behavior itself is still pinned, just
// against "lambda" now — see
// TestHandleRelatedNavigate_NonByIDType_CacheMiss_EmitsFetchResources.
func TestHandleRelatedNavigate_TargetIDCacheMiss_FilteredList(t *testing.T) {
	c, _ := newRuntimeCore(t)

	result, tasks := c.HandleRelatedNavigate(runtime.RelatedNavigateEvent{
		TargetType: "ec2",
		TargetID:   "i-missing",
	})

	if result.Kind != runtime.NavigationKindFilteredList {
		t.Errorf("Kind = %v, want NavigationKindFilteredList", result.Kind)
	}
	if result.FilterText != "i-missing" {
		t.Errorf("FilterText = %q, want %q", result.FilterText, "i-missing")
	}
	if result.TargetID != "i-missing" {
		t.Errorf("TargetID = %q, want %q", result.TargetID, "i-missing")
	}
	wantTasks := []runtime.TaskRequest{{
		Key:     runtime.TaskKey{Kind: runtime.KindFetchByIDDetail, Scope: "ec2"},
		Cache:   runtime.CacheNone,
		Payload: runtime.FetchByIDDetailPayload{TargetType: "ec2", ID: "i-missing"},
	}}
	if !reflect.DeepEqual(tasks, wantTasks) {
		t.Errorf("tasks = %+v, want %+v", tasks, wantTasks)
	}
}

// Case G — multiple RelatedIDs cache miss, no further pages → FilteredList
// with RelatedIDs preserved and a single KindFetchResources task.
func TestHandleRelatedNavigate_MultipleRelatedIDs_CacheMiss_FetchResources(t *testing.T) {
	c, _ := newRuntimeCore(t)

	relatedIDs := []string{"i-1", "i-2"}
	result, tasks := c.HandleRelatedNavigate(runtime.RelatedNavigateEvent{
		TargetType: "ec2",
		RelatedIDs: relatedIDs,
	})

	if result.Kind != runtime.NavigationKindFilteredList {
		t.Errorf("Kind = %v, want NavigationKindFilteredList", result.Kind)
	}
	if !reflect.DeepEqual(result.RelatedIDs, relatedIDs) {
		t.Errorf("RelatedIDs = %v, want %v", result.RelatedIDs, relatedIDs)
	}
	wantTasks := []runtime.TaskRequest{{
		Key:   runtime.TaskKey{Kind: runtime.KindFetchResources, Scope: "ec2"},
		Cache: runtime.CacheNone,
	}}
	if !reflect.DeepEqual(tasks, wantTasks) {
		t.Errorf("tasks = %+v, want %+v", tasks, wantTasks)
	}
}

// Case H — multiple RelatedIDs, partial coverage + truncated cache →
// FilteredList with a single KindFetchMore task (continuation). AS-270:
// the continuation token rides on the TaskRequest as a FetchMorePayload.
func TestHandleRelatedNavigate_MultipleRelatedIDs_PartialCoverage_Truncated_FetchMore(t *testing.T) {
	c, s := newRuntimeCore(t)
	s.RowStore.Observe("ec2", []resource.Resource{{ID: "i-1"}}, &domain.PaginationMeta{IsTruncated: true, NextToken: "next-tok-xyz"}, session.OriginFetch, false)

	result, tasks := c.HandleRelatedNavigate(runtime.RelatedNavigateEvent{
		TargetType: "ec2",
		RelatedIDs: []string{"i-1", "i-2"},
	})

	if result.Kind != runtime.NavigationKindFilteredList {
		t.Errorf("Kind = %v, want NavigationKindFilteredList", result.Kind)
	}
	wantTasks := []runtime.TaskRequest{{
		Key:     runtime.TaskKey{Kind: runtime.KindFetchMore, Scope: "ec2"},
		Cache:   runtime.CacheNone,
		Payload: runtime.FetchMorePayload{ContinuationToken: "next-tok-xyz"},
	}}
	if !reflect.DeepEqual(tasks, wantTasks) {
		t.Errorf("tasks = %+v, want %+v", tasks, wantTasks)
	}
}

// Case I — multiple RelatedIDs fully covered by ResourceCache → FilteredList
// with no fetch task.
func TestHandleRelatedNavigate_MultipleRelatedIDs_FullyCached_NoFetch(t *testing.T) {
	c, s := newRuntimeCore(t)
	s.RowStore.Observe("ec2", []resource.Resource{{ID: "i-1"}, {ID: "i-2"}}, nil, session.OriginFetch, false)

	result, tasks := c.HandleRelatedNavigate(runtime.RelatedNavigateEvent{
		TargetType: "ec2",
		RelatedIDs: []string{"i-1", "i-2"},
	})

	if result.Kind != runtime.NavigationKindFilteredList {
		t.Errorf("Kind = %v, want NavigationKindFilteredList", result.Kind)
	}
	if len(tasks) != 0 {
		t.Errorf("len(tasks) = %d, want 0", len(tasks))
	}
}

// Case J — a truncated "(0+)" that found none yet (Truncated, no IDs) → a SCOPED
// FilteredList seeded empty, WITH a KindFetchResources task so the reapply-checker
// can populate and scope it. This is the identical path "(N+)" takes; the zero
// lower bound is never special-cased into a "goes to all" list, and never left
// without a fetch (which would strand the list empty). A NON-truncated no-scope
// event is the defensive ResourceList fallback instead — see the core/runtime
// package test TestHandleRelatedNavigate_ResourceList_EmitsFetchResources.
func TestHandleRelatedNavigate_TruncatedZero_ScopedListWithFetch(t *testing.T) {
	c, _ := newRuntimeCore(t)

	zeroPlus, zeroTasks := c.HandleRelatedNavigate(runtime.RelatedNavigateEvent{
		TargetType: "ec2",
		Truncated:  true,
	})
	nPlus, nTasks := c.HandleRelatedNavigate(runtime.RelatedNavigateEvent{
		TargetType: "ec2",
		RelatedIDs: []string{"i-1", "i-2"},
		Truncated:  true,
	})

	// (0+) and (N+) take the identical shape — Kind + task — differing only in
	// the seed IDs carried through as data.
	if zeroPlus.Kind != runtime.NavigationKindFilteredList || nPlus.Kind != runtime.NavigationKindFilteredList {
		t.Errorf("Kind (0+)=%v (N+)=%v, want both NavigationKindFilteredList", zeroPlus.Kind, nPlus.Kind)
	}
	fetchOnly := func(tasks []runtime.TaskRequest) bool {
		return len(tasks) == 1 && tasks[0].Key.Kind == runtime.KindFetchResources
	}
	if !fetchOnly(zeroTasks) {
		t.Errorf("(0+) tasks = %v, want one KindFetchResources task (populate + reapply)", zeroTasks)
	}
	if !fetchOnly(nTasks) {
		t.Errorf("(N+) tasks = %v, want one KindFetchResources task — identical to (0+)", nTasks)
	}
}

// Case K — pure-lazy passthrough for Detail: TargetID hit lives only in a
// Partial RowStore entry (ObservePartial); no full Observe ever landed.
//
// This proves relatedCacheSnapshot includes Partial RowStore entries when
// resolving the TargetID Detail branch.
func TestHandleRelatedNavigate_PureLazyCacheHit_Detail(t *testing.T) {
	c, s := newRuntimeCore(t)
	s.RowStore.ObservePartial("ec2", []resource.Resource{{ID: "i-1"}})

	result, tasks := c.HandleRelatedNavigate(runtime.RelatedNavigateEvent{
		TargetType: "ec2",
		TargetID:   "i-1",
	})

	if result.Kind != runtime.NavigationKindDetail {
		t.Errorf("Kind = %v, want NavigationKindDetail", result.Kind)
	}
	if len(tasks) != 0 {
		t.Errorf("len(tasks) = %d, want 0", len(tasks))
	}
}

// Case L — exact-ID drill to a by-ID-capable type, cache MISS → emits
// KindFetchByIDDetail instead of KindFetchResources.
//
// This is the key TDD case for the new by-ID dispatch path. The test
// registers a no-op FetchByIDs helper for a synthetic type, fires a
// RelatedNavigateEvent whose TargetID is absent from the session cache,
// and asserts that the runtime emits exactly one KindFetchByIDDetail task
// with the correct type-asserted FetchByIDDetailPayload.
func TestHandleRelatedNavigate_ByIDCapableType_CacheMiss_EmitsFetchByIDDetail(t *testing.T) {
	const targetType = "test-fetchbyid-type-l"
	resource.SetFetchByIDsForTest(targetType, func(_ context.Context, _ any, _ []string) ([]domain.Resource, error) {
		return nil, nil
	})
	t.Cleanup(func() { resource.CleanupFetchByIDsForTest(targetType) })

	resource.SetPaginatedForTest(targetType, func(_ context.Context, _ any, _ string) (domain.FetchResult, error) {
		return domain.FetchResult{}, nil
	})
	t.Cleanup(func() { resource.CleanupPaginatedForTest(targetType) })

	c, _ := newRuntimeCore(t)

	result, tasks := c.HandleRelatedNavigate(runtime.RelatedNavigateEvent{
		TargetType: targetType,
		TargetID:   "snap-0abc123",
	})

	if result.Kind != runtime.NavigationKindFilteredList {
		t.Errorf("Kind = %v, want NavigationKindFilteredList", result.Kind)
	}
	if len(tasks) != 1 {
		t.Fatalf("len(tasks) = %d, want 1", len(tasks))
	}
	if tasks[0].Key.Kind != runtime.KindFetchByIDDetail {
		t.Errorf("tasks[0].Key.Kind = %q, want %q", tasks[0].Key.Kind, runtime.KindFetchByIDDetail)
	}
	if tasks[0].Key.Scope != targetType {
		t.Errorf("tasks[0].Key.Scope = %q, want %q", tasks[0].Key.Scope, targetType)
	}
	payload, ok := tasks[0].Payload.(runtime.FetchByIDDetailPayload)
	if !ok {
		t.Fatalf("tasks[0].Payload type = %T, want runtime.FetchByIDDetailPayload", tasks[0].Payload)
	}
	wantPayload := runtime.FetchByIDDetailPayload{TargetType: targetType, ID: "snap-0abc123"}
	if payload != wantPayload {
		t.Errorf("Payload = %+v, want %+v", payload, wantPayload)
	}
}

// Case M — exact-ID drill to a type with NO FetchByIDs helper, cache MISS →
// regression guard that KindFetchResources is still emitted (not
// KindFetchByIDDetail).
//
// This pins the else-branch of the new conditional so a future refactor
// cannot accidentally route all cache-miss TargetID drills through the
// by-ID path.
func TestHandleRelatedNavigate_NonByIDType_CacheMiss_EmitsFetchResources(t *testing.T) {
	// "lambda" has no FetchByIDs helper registered in core/aws/
	// catalog_compute.go (unlike "ec2", "ebs-snap", "ami" — confirmed
	// directly against that file). If that ever changes this test will
	// catch the regression in the opposite direction. "ec2" itself moved to
	// the by-ID branch once it registered FetchByIDs — see
	// TestHandleRelatedNavigate_TargetIDCacheMiss_FilteredList.
	c, _ := newRuntimeCore(t)

	result, tasks := c.HandleRelatedNavigate(runtime.RelatedNavigateEvent{
		TargetType: "lambda",
		TargetID:   "arn:aws:lambda:us-east-1:123456789012:function:nofetchbyid",
	})

	if result.Kind != runtime.NavigationKindFilteredList {
		t.Errorf("Kind = %v, want NavigationKindFilteredList", result.Kind)
	}
	if len(tasks) != 1 {
		t.Fatalf("len(tasks) = %d, want 1", len(tasks))
	}
	if tasks[0].Key.Kind != runtime.KindFetchResources {
		t.Errorf("tasks[0].Key.Kind = %q, want %q (not KindFetchByIDDetail)", tasks[0].Key.Kind, runtime.KindFetchResources)
	}
	if tasks[0].Key.Scope != "lambda" {
		t.Errorf("tasks[0].Key.Scope = %q, want %q", tasks[0].Key.Scope, "lambda")
	}
	if _, isDetail := tasks[0].Payload.(runtime.FetchByIDDetailPayload); isDetail {
		t.Error("Payload is FetchByIDDetailPayload, want none — by-ID path must not fire for lambda (no FetchByIDs registered)")
	}
}

// Case N — exact-ID drill to a by-ID-capable type, cache HIT → NavigationKindDetail,
// NO fetch task at all.
//
// Safety property: the KindFetchByIDDetail path must only fire on cache miss.
// When the resource is already in the session cache the router must drill
// directly to the Detail view without any fetch task, regardless of whether
// the target type has a FetchByIDs helper.
func TestHandleRelatedNavigate_ByIDCapableType_CacheHit_Detail_NoFetchTask(t *testing.T) {
	const targetType = "test-fetchbyid-type-n"
	resource.SetFetchByIDsForTest(targetType, func(_ context.Context, _ any, _ []string) ([]domain.Resource, error) {
		return nil, nil
	})
	t.Cleanup(func() { resource.CleanupFetchByIDsForTest(targetType) })

	resource.SetPaginatedForTest(targetType, func(_ context.Context, _ any, _ string) (domain.FetchResult, error) {
		return domain.FetchResult{}, nil
	})
	t.Cleanup(func() { resource.CleanupPaginatedForTest(targetType) })

	c, s := newRuntimeCore(t)
	s.RowStore.Observe(targetType, []resource.Resource{{ID: "snap-cached"}}, nil, session.OriginFetch, false)

	result, tasks := c.HandleRelatedNavigate(runtime.RelatedNavigateEvent{
		TargetType: targetType,
		TargetID:   "snap-cached",
	})

	if result.Kind != runtime.NavigationKindDetail {
		t.Errorf("Kind = %v, want NavigationKindDetail", result.Kind)
	}
	if result.TargetID != "snap-cached" {
		t.Errorf("TargetID = %q, want %q", result.TargetID, "snap-cached")
	}
	if len(tasks) != 0 {
		t.Errorf("len(tasks) = %d, want 0 — cache hit must not emit any fetch task", len(tasks))
	}
}
