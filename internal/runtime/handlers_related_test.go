// handlers_related_test.go — unit tests for the runtime-side related-navigation
// dispatch (PR-05a-h4 / AS-150 NEEDS CHANGES finding #2).
//
// These tests live inside the runtime package so they can exercise unexported
// helpers (relatedFetchTasks, relatedCacheSnapshot) directly. The exported
// surface (ResolveRelatedNavigate, HandleRelatedNavigate) is also covered from
// tests/unit/, but driving the helpers from the same package is the cheapest
// way to pin the policy's edge cases.
package runtime

import (
	"testing"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/catalog"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/session"
)

// newTestSession constructs a Session with the maps populated so that handler
// paths writing into them don't panic.
func newTestSession() *session.Session {
	return session.New()
}

func TestRelatedCacheSnapshot_MergePrecedence(t *testing.T) {
	s := newTestSession()
	s.LazyResourceCache = map[string][]resource.Resource{
		"ec2": {{ID: "i-lazy"}, {ID: "i-shared"}},
	}
	s.ResourceCache = map[string]*session.ResourceCacheEntry{
		"ec2": {Resources: []resource.Resource{{ID: "i-cache", Name: "from-cache"}, {ID: "i-shared", Name: "from-cache"}}},
	}

	snap := relatedCacheSnapshot(s)

	rows, ok := snap["ec2"]
	if !ok {
		t.Fatalf("snap[ec2] missing")
	}
	// All three IDs must appear.
	got := make(map[string]string, len(rows))
	for _, r := range rows {
		got[r.ID] = r.Name
	}
	if got["i-lazy"] != "" {
		t.Errorf("i-lazy unexpectedly named %q", got["i-lazy"])
	}
	if _, ok := got["i-cache"]; !ok {
		t.Errorf("i-cache missing from snapshot")
	}
	// On collision, ResourceCache must win (Name set, not the lazy zero-value).
	if got["i-shared"] != "from-cache" {
		t.Errorf("i-shared name = %q, want %q (ResourceCache must win on collision)", got["i-shared"], "from-cache")
	}
}

func TestRelatedCacheSnapshot_LazyOnly(t *testing.T) {
	s := newTestSession()
	s.LazyResourceCache = map[string][]resource.Resource{
		"kms": {{ID: "alias/aws/managed"}},
	}

	snap := relatedCacheSnapshot(s)
	if len(snap["kms"]) != 1 || snap["kms"][0].ID != "alias/aws/managed" {
		t.Errorf("snap[kms] = %v, want [alias/aws/managed]", snap["kms"])
	}
}

func TestRelatedFetchTasks_FullCoverage_NoTask(t *testing.T) {
	s := newTestSession()
	s.ResourceCache = map[string]*session.ResourceCacheEntry{
		"ec2": {Resources: []resource.Resource{{ID: "i-1"}, {ID: "i-2"}}},
	}

	tasks := relatedFetchTasks(s, "ec2", []string{"i-1", "i-2"})
	if tasks != nil {
		t.Errorf("tasks = %v, want nil — full coverage should not request a fetch", tasks)
	}
}

func TestRelatedFetchTasks_LazyFullCoverage_NoTask(t *testing.T) {
	s := newTestSession()
	s.LazyResourceCache = map[string][]resource.Resource{
		"kms": {{ID: "alias/aws/managed-1"}, {ID: "alias/aws/managed-2"}},
	}

	tasks := relatedFetchTasks(s, "kms", []string{"alias/aws/managed-1", "alias/aws/managed-2"})
	if tasks != nil {
		t.Errorf("tasks = %v, want nil — lazy-cache full coverage should not request a fetch", tasks)
	}
}

func TestRelatedFetchTasks_PartialCoverage_TruncatedCache_FetchMore(t *testing.T) {
	s := newTestSession()
	s.ResourceCache = map[string]*session.ResourceCacheEntry{
		"ec2": {
			Resources:  []resource.Resource{{ID: "i-1"}},
			Pagination: &resource.PaginationMeta{IsTruncated: true, NextToken: "tok-2"},
		},
	}

	tasks := relatedFetchTasks(s, "ec2", []string{"i-1", "i-missing"})
	if len(tasks) != 1 {
		t.Fatalf("len(tasks) = %d, want 1", len(tasks))
	}
	if tasks[0].Key.Kind != KindFetchMore {
		t.Errorf("Kind = %q, want %q", tasks[0].Key.Kind, KindFetchMore)
	}
	if tasks[0].Key.Scope != "ec2" {
		t.Errorf("Scope = %q, want %q", tasks[0].Key.Scope, "ec2")
	}
}

func TestRelatedFetchTasks_FullMiss_FetchAll(t *testing.T) {
	s := newTestSession()

	tasks := relatedFetchTasks(s, "ec2", []string{"i-1", "i-2"})
	if len(tasks) != 1 {
		t.Fatalf("len(tasks) = %d, want 1", len(tasks))
	}
	if tasks[0].Key.Kind != KindFetchResources {
		t.Errorf("Kind = %q, want %q", tasks[0].Key.Kind, KindFetchResources)
	}
}

func TestRelatedFetchTasks_PartialCoverage_NotTruncated_FetchAll(t *testing.T) {
	// When the cache has a partial set and is NOT truncated, no further pages
	// can satisfy the missing IDs from this fetcher — fall back to a full fetch.
	s := newTestSession()
	s.ResourceCache = map[string]*session.ResourceCacheEntry{
		"ec2": {
			Resources:  []resource.Resource{{ID: "i-1"}},
			Pagination: &resource.PaginationMeta{IsTruncated: false},
		},
	}

	tasks := relatedFetchTasks(s, "ec2", []string{"i-1", "i-missing"})
	if len(tasks) != 1 {
		t.Fatalf("len(tasks) = %d, want 1", len(tasks))
	}
	if tasks[0].Key.Kind != KindFetchResources {
		t.Errorf("Kind = %q, want %q (not-truncated partial coverage must fall back to full fetch)", tasks[0].Key.Kind, KindFetchResources)
	}
}

func TestHandleRelatedNavigate_UnknownType_FlashOnly_NoTask(t *testing.T) {
	c := New(newTestSession(), catalog.ResourceTypes)

	result, tasks := c.HandleRelatedNavigate(RelatedNavigateEvent{TargetType: "nonexistent_xyz"})

	if result.Kind != NavigationKindFlash {
		t.Errorf("Kind = %v, want NavigationKindFlash", result.Kind)
	}
	if !result.FlashIsError {
		t.Errorf("FlashIsError = false, want true")
	}
	if tasks != nil {
		t.Errorf("tasks = %v, want nil — flash path must not emit a fetch task", tasks)
	}
}

func TestHandleRelatedNavigate_ChildType_NoTask(t *testing.T) {
	c := New(newTestSession(), catalog.ResourceTypes)

	result, tasks := c.HandleRelatedNavigate(RelatedNavigateEvent{
		TargetType: "ecr_images",
		RelatedIDs: []string{"my-repo|my-tag"},
	})

	if result.Kind != NavigationKindEnterChildView {
		t.Errorf("Kind = %v, want NavigationKindEnterChildView", result.Kind)
	}
	if tasks != nil {
		t.Errorf("tasks = %v, want nil — child-view navigation has no runtime fetch", tasks)
	}
}

func TestHandleRelatedNavigate_DetailCacheHit_NoTask(t *testing.T) {
	s := newTestSession()
	s.ResourceCache = map[string]*session.ResourceCacheEntry{
		"s3": {Resources: []resource.Resource{{ID: "prod-logs"}}},
	}
	c := New(s, catalog.ResourceTypes)

	result, tasks := c.HandleRelatedNavigate(RelatedNavigateEvent{
		TargetType: "s3",
		TargetID:   "prod-logs",
	})

	if result.Kind != NavigationKindDetail {
		t.Errorf("Kind = %v, want NavigationKindDetail", result.Kind)
	}
	if tasks != nil {
		t.Errorf("tasks = %v, want nil — cache-hit detail does not refetch", tasks)
	}
}

func TestHandleRelatedNavigate_FilteredList_FetchFilter_EmitsFiltered(t *testing.T) {
	c := New(newTestSession(), catalog.ResourceTypes)

	result, tasks := c.HandleRelatedNavigate(RelatedNavigateEvent{
		TargetType:  "ct-events",
		FetchFilter: map[string]string{"Username": "alice"},
	})

	if result.Kind != NavigationKindFilteredList {
		t.Fatalf("Kind = %v, want NavigationKindFilteredList", result.Kind)
	}
	if len(tasks) != 1 || tasks[0].Key.Kind != KindFetchFiltered {
		t.Errorf("tasks = %v, want one KindFetchFiltered task", tasks)
	}
}

func TestHandleRelatedNavigate_FilteredList_TargetIDMiss_EmitsFetchResources(t *testing.T) {
	c := New(newTestSession(), catalog.ResourceTypes)

	result, tasks := c.HandleRelatedNavigate(RelatedNavigateEvent{
		TargetType: "s3",
		TargetID:   "missing-bucket",
	})

	if result.Kind != NavigationKindFilteredList {
		t.Fatalf("Kind = %v, want NavigationKindFilteredList", result.Kind)
	}
	if result.FilterText != "missing-bucket" {
		t.Errorf("FilterText = %q, want %q", result.FilterText, "missing-bucket")
	}
	if len(tasks) != 1 || tasks[0].Key.Kind != KindFetchResources {
		t.Errorf("tasks = %v, want one KindFetchResources task", tasks)
	}
}

func TestHandleRelatedNavigate_ResourceList_EmitsFetchResources(t *testing.T) {
	c := New(newTestSession(), catalog.ResourceTypes)

	result, tasks := c.HandleRelatedNavigate(RelatedNavigateEvent{TargetType: "ec2"})

	if result.Kind != NavigationKindResourceList {
		t.Fatalf("Kind = %v, want NavigationKindResourceList", result.Kind)
	}
	if len(tasks) != 1 || tasks[0].Key.Kind != KindFetchResources {
		t.Errorf("tasks = %v, want one KindFetchResources task", tasks)
	}
	if tasks[0].Key.Scope != "ec2" {
		t.Errorf("Scope = %q, want %q", tasks[0].Key.Scope, "ec2")
	}
}

func TestHandleRelatedNavigate_RelatedIDs_FullCoverage_NoTask(t *testing.T) {
	s := newTestSession()
	s.ResourceCache = map[string]*session.ResourceCacheEntry{
		"ec2": {Resources: []resource.Resource{{ID: "i-1"}, {ID: "i-2"}, {ID: "i-3"}}},
	}
	c := New(s, catalog.ResourceTypes)

	result, tasks := c.HandleRelatedNavigate(RelatedNavigateEvent{
		TargetType: "ec2",
		RelatedIDs: []string{"i-1", "i-2"},
	})

	if result.Kind != NavigationKindFilteredList {
		t.Fatalf("Kind = %v, want NavigationKindFilteredList", result.Kind)
	}
	if tasks != nil {
		t.Errorf("tasks = %v, want nil — full-coverage filtered list has no fetch", tasks)
	}
}
