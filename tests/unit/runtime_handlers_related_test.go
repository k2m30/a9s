// runtime_handlers_related_test.go — public-seam coverage for
// (*runtime.Core).HandleRelatedNavigate after the AS-150 migration moved the
// handler out of internal/tui into internal/runtime.
//
// Cases A–K mirror the Stage 2 scope on AS-201. The runtime seam is exactly
// what AS-150 exposed — these tests stand up *runtime.Core directly through
// runtime.New(session.New(), catalog.ResourceTypes) and assert the
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

	"github.com/k2m30/a9s/v3/internal/catalog"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime"
	"github.com/k2m30/a9s/v3/internal/session"
)

// newRuntimeCore returns a fresh *runtime.Core bound to a clean session and
// the static catalog. Test cases mutate the returned session's caches
// directly to seed each branch.
func newRuntimeCore(t *testing.T) (*runtime.Core, *session.Session) {
	t.Helper()
	s := session.New()
	c := runtime.New(s, catalog.ResourceTypes)
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
// depend on internal/aws being imported (which would pull init() side effects
// into tests/unit).
func TestHandleRelatedNavigate_ChildType_EnterChildView(t *testing.T) {
	const childShort = "test_child_type_handle_related_b"
	resource.RegisterChildType(resource.ResourceTypeDef{
		Name:      "Test Child",
		ShortName: childShort,
	})
	t.Cleanup(func() { resource.UnregisterChildType(childShort) })

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
	s.ResourceCache["ec2"] = &session.ResourceCacheEntry{
		Resources: []resource.Resource{{ID: "i-1"}},
	}

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
	s.ResourceCache["ec2"] = &session.ResourceCacheEntry{
		Resources: []resource.Resource{{ID: "i-1"}},
	}

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
	resource.RegisterFilteredPaginated("ec2",
		func(_ context.Context, _ any, _ map[string]string, _ string) (domain.FetchResult, error) {
			return domain.FetchResult{}, nil
		})
	t.Cleanup(func() { resource.UnregisterFilteredPaginated("ec2") })

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

// Case F — TargetID cache miss (no filtered fetcher, no cache entry) →
// FilteredList with FilterText==TargetID and a single KindFetchResources task.
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
		Key:   runtime.TaskKey{Kind: runtime.KindFetchResources, Scope: "ec2"},
		Cache: runtime.CacheNone,
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
	s.ResourceCache["ec2"] = &session.ResourceCacheEntry{
		Resources:  []resource.Resource{{ID: "i-1"}},
		Pagination: &domain.PaginationMeta{IsTruncated: true, NextToken: "next-tok-xyz"},
	}

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
	s.ResourceCache["ec2"] = &session.ResourceCacheEntry{
		Resources: []resource.Resource{{ID: "i-1"}, {ID: "i-2"}},
	}

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

// Case J — no IDs, no filter, no targetID → ResourceList with a single
// KindFetchResources task.
func TestHandleRelatedNavigate_NoIDsNoFilter_ResourceList(t *testing.T) {
	c, _ := newRuntimeCore(t)

	result, tasks := c.HandleRelatedNavigate(runtime.RelatedNavigateEvent{
		TargetType: "ec2",
	})

	if result.Kind != runtime.NavigationKindResourceList {
		t.Errorf("Kind = %v, want NavigationKindResourceList", result.Kind)
	}
	wantTasks := []runtime.TaskRequest{{
		Key:   runtime.TaskKey{Kind: runtime.KindFetchResources, Scope: "ec2"},
		Cache: runtime.CacheNone,
	}}
	if !reflect.DeepEqual(tasks, wantTasks) {
		t.Errorf("tasks = %+v, want %+v", tasks, wantTasks)
	}
}

// Case K — pure-lazy passthrough for Detail: TargetID hit lives only in
// LazyResourceCache; ResourceCache has no entry for the type.
//
// This proves relatedCacheSnapshot includes LazyResourceCache when resolving
// the TargetID Detail branch.
func TestHandleRelatedNavigate_PureLazyCacheHit_Detail(t *testing.T) {
	c, s := newRuntimeCore(t)
	s.LazyResourceCache["ec2"] = []resource.Resource{{ID: "i-1"}}

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
