// handlers_related_test.go — same-package internal tests for the
// unexported helpers in handlers_related.go.
//
// The public seam (*runtime.Core).HandleRelatedNavigate is exercised from
// tests/unit/runtime_handlers_related_test.go (Cases A–K). This file pins the
// helper-level precedence and cache-coverage rules the public seam cannot
// observe directly (e.g. dedup of LazyResourceCache + ResourceCache, the
// truncated-vs-missing-page-vs-no-entry fan-out, and the FetchFilter detection
// step inside resolveRelatedNavigate).
//
// HARD CONSTRAINT (per AS-203 acceptance): this file MUST NOT import
// charm.land/bubbletea/v2, lipgloss, or bubbles.
package runtime

import (
	"context"
	"reflect"
	"testing"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/session"
)

// ─── relatedFetchTasks — Cases a–f ─────────────────────────────────────────

// Case a — all IDs covered by ResourceCache only → no fetch.
func TestRelatedFetchTasks_AllCachedViaResourceCache_Nil(t *testing.T) {
	s := session.New()
	s.ResourceCache["ec2"] = &session.ResourceCacheEntry{
		Resources: []resource.Resource{{ID: "i-1"}, {ID: "i-2"}},
	}

	got := relatedFetchTasks(s, "ec2", []string{"i-1", "i-2"})
	if got != nil {
		t.Errorf("got %+v, want nil", got)
	}
}

// Case b — all IDs covered by LazyResourceCache only → no fetch.
func TestRelatedFetchTasks_AllCachedViaLazyCache_Nil(t *testing.T) {
	s := session.New()
	s.LazyResourceCache["ec2"] = []resource.Resource{{ID: "i-1"}, {ID: "i-2"}}

	got := relatedFetchTasks(s, "ec2", []string{"i-1", "i-2"})
	if got != nil {
		t.Errorf("got %+v, want nil", got)
	}
}

// Case c — coverage split across both caches, fully covered → no fetch.
// Proves dedup logic considers both maps when computing "missing".
func TestRelatedFetchTasks_MixedFullCoverage_Nil(t *testing.T) {
	s := session.New()
	s.ResourceCache["ec2"] = &session.ResourceCacheEntry{
		Resources: []resource.Resource{{ID: "i-1"}},
	}
	s.LazyResourceCache["ec2"] = []resource.Resource{{ID: "i-2"}}

	got := relatedFetchTasks(s, "ec2", []string{"i-1", "i-2"})
	if got != nil {
		t.Errorf("got %+v, want nil", got)
	}
}

// Case d — miss with ResourceCache.Pagination.IsTruncated==true → KindFetchMore.
func TestRelatedFetchTasks_MissTruncated_FetchMore(t *testing.T) {
	s := session.New()
	s.ResourceCache["ec2"] = &session.ResourceCacheEntry{
		Resources:  []resource.Resource{{ID: "i-1"}},
		Pagination: &domain.PaginationMeta{IsTruncated: true},
	}

	got := relatedFetchTasks(s, "ec2", []string{"i-1", "i-2"})
	want := []TaskRequest{{
		Key:   TaskKey{Kind: KindFetchMore, Scope: "ec2"},
		Cache: CacheNone,
	}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

// Case e — miss with ResourceCache present but Pagination==nil → KindFetchResources.
func TestRelatedFetchTasks_MissPaginationNil_FetchResources(t *testing.T) {
	s := session.New()
	s.ResourceCache["ec2"] = &session.ResourceCacheEntry{
		Resources: []resource.Resource{{ID: "i-1"}},
	}

	got := relatedFetchTasks(s, "ec2", []string{"i-1", "i-2"})
	want := []TaskRequest{{
		Key:   TaskKey{Kind: KindFetchResources, Scope: "ec2"},
		Cache: CacheNone,
	}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

// Case f — miss with no ResourceCache entry at all (LazyResourceCache only
// covers some, not all) → KindFetchResources.
func TestRelatedFetchTasks_MissNoResourceCache_LazyOnly_FetchResources(t *testing.T) {
	s := session.New()
	s.LazyResourceCache["ec2"] = []resource.Resource{{ID: "i-1"}}

	got := relatedFetchTasks(s, "ec2", []string{"i-1", "i-2"})
	want := []TaskRequest{{
		Key:   TaskKey{Kind: KindFetchResources, Scope: "ec2"},
		Cache: CacheNone,
	}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

// ─── relatedCacheSnapshot — Cases a–d ──────────────────────────────────────

// snapResourceIDs is a tiny accessor that returns the IDs visible in the snapshot
// for the given shortName, preserving order.
func snapResourceIDs(snap map[string][]resource.Resource, shortName string) []string {
	rs := snap[shortName]
	ids := make([]string, 0, len(rs))
	for _, r := range rs {
		ids = append(ids, r.ID)
	}
	return ids
}

// Case a — ResourceCache only → snapshot mirrors ResourceCache.
func TestRelatedCacheSnapshot_ResourceCacheOnly(t *testing.T) {
	s := session.New()
	s.ResourceCache["ec2"] = &session.ResourceCacheEntry{
		Resources: []resource.Resource{{ID: "i-1"}, {ID: "i-2"}},
	}

	snap := relatedCacheSnapshot(s)
	got := snapResourceIDs(snap, "ec2")
	want := []string{"i-1", "i-2"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ec2 IDs = %v, want %v", got, want)
	}
}

// Case b — LazyResourceCache only → snapshot mirrors LazyResourceCache.
func TestRelatedCacheSnapshot_LazyCacheOnly(t *testing.T) {
	s := session.New()
	s.LazyResourceCache["ec2"] = []resource.Resource{{ID: "i-1"}, {ID: "i-2"}}

	snap := relatedCacheSnapshot(s)
	got := snapResourceIDs(snap, "ec2")
	want := []string{"i-1", "i-2"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ec2 IDs = %v, want %v", got, want)
	}
}

// Case c — Both, no ID collision → snapshot is the union (ResourceCache rows
// first, then any LazyResourceCache rows not already present).
func TestRelatedCacheSnapshot_BothNoCollision_Union(t *testing.T) {
	s := session.New()
	s.ResourceCache["ec2"] = &session.ResourceCacheEntry{
		Resources: []resource.Resource{{ID: "i-1"}},
	}
	s.LazyResourceCache["ec2"] = []resource.Resource{{ID: "i-2"}}

	snap := relatedCacheSnapshot(s)

	got := map[string]struct{}{}
	for _, r := range snap["ec2"] {
		got[r.ID] = struct{}{}
	}
	for _, id := range []string{"i-1", "i-2"} {
		if _, ok := got[id]; !ok {
			t.Errorf("missing %q in snapshot: have %v", id, snap["ec2"])
		}
	}
	if len(snap["ec2"]) != 2 {
		t.Errorf("len(snapshot) = %d, want 2", len(snap["ec2"]))
	}
}

// Case d — Both, ID collision on "i-1" → snapshot contains exactly one entry
// for "i-1", and that entry is the ResourceCache version. We distinguish the
// two source rows by their Name field.
func TestRelatedCacheSnapshot_BothCollision_ResourceCacheWins(t *testing.T) {
	s := session.New()
	s.ResourceCache["ec2"] = &session.ResourceCacheEntry{
		Resources: []resource.Resource{{ID: "i-1", Name: "from-resource-cache"}},
	}
	s.LazyResourceCache["ec2"] = []resource.Resource{{ID: "i-1", Name: "from-lazy-cache"}}

	snap := relatedCacheSnapshot(s)

	if len(snap["ec2"]) != 1 {
		t.Fatalf("len(snapshot) = %d, want 1 (dedup expected)", len(snap["ec2"]))
	}
	if got := snap["ec2"][0].Name; got != "from-resource-cache" {
		t.Errorf("Name = %q, want %q (ResourceCache must win on collision)",
			got, "from-resource-cache")
	}
}

// ─── resolveRelatedNavigate — Cases a–h ────────────────────────────────────

// Case a — unknown type → Flash with FlashIsError true.
func TestResolveRelatedNavigate_Unknown_Flash(t *testing.T) {
	got := resolveRelatedNavigate(
		RelatedNavigateEvent{TargetType: "definitely-not-a-real-type"},
		nil,
	)
	if got.Kind != NavigationKindFlash {
		t.Errorf("Kind = %v, want NavigationKindFlash", got.Kind)
	}
	if !got.FlashIsError {
		t.Error("FlashIsError = false, want true")
	}
}

// Case b — child type → EnterChildView, RelatedIDs preserved.
func TestResolveRelatedNavigate_ChildType_EnterChildView(t *testing.T) {
	const childShort = "test_child_resolve_b"
	resource.RegisterChildType(resource.ResourceTypeDef{
		Name:      "Test Child",
		ShortName: childShort,
	})
	t.Cleanup(func() { resource.UnregisterChildType(childShort) })

	relatedIDs := []string{"x-1", "x-2"}
	got := resolveRelatedNavigate(
		RelatedNavigateEvent{TargetType: childShort, RelatedIDs: relatedIDs},
		nil,
	)
	if got.Kind != NavigationKindEnterChildView {
		t.Errorf("Kind = %v, want NavigationKindEnterChildView", got.Kind)
	}
	if !reflect.DeepEqual(got.RelatedIDs, relatedIDs) {
		t.Errorf("RelatedIDs = %v, want %v", got.RelatedIDs, relatedIDs)
	}
}

// Case c — TargetID present + in cache → Detail.
func TestResolveRelatedNavigate_TargetIDHit_Detail(t *testing.T) {
	cache := map[string][]resource.Resource{
		"ec2": {{ID: "i-1"}},
	}
	got := resolveRelatedNavigate(
		RelatedNavigateEvent{TargetType: "ec2", TargetID: "i-1"},
		cache,
	)
	if got.Kind != NavigationKindDetail {
		t.Errorf("Kind = %v, want NavigationKindDetail", got.Kind)
	}
	if got.TargetID != "i-1" {
		t.Errorf("TargetID = %q, want %q", got.TargetID, "i-1")
	}
}

// Case d — single RelatedIDs hit → Detail.
func TestResolveRelatedNavigate_SingleRelatedIDHit_Detail(t *testing.T) {
	cache := map[string][]resource.Resource{
		"ec2": {{ID: "i-1"}},
	}
	got := resolveRelatedNavigate(
		RelatedNavigateEvent{TargetType: "ec2", RelatedIDs: []string{"i-1"}},
		cache,
	)
	if got.Kind != NavigationKindDetail {
		t.Errorf("Kind = %v, want NavigationKindDetail", got.Kind)
	}
}

// Case e — FetchFilter + registered filtered fetcher → FilteredList with
// FetchFilter set.
func TestResolveRelatedNavigate_FetchFilter_RegisteredFetcher_FilteredList(t *testing.T) {
	resource.RegisterFilteredPaginated("ec2",
		func(_ context.Context, _ any, _ map[string]string, _ string) (domain.FetchResult, error) {
			return domain.FetchResult{}, nil
		})
	t.Cleanup(func() { resource.UnregisterFilteredPaginated("ec2") })

	filter := map[string]string{"vpc-id": "vpc-1"}
	got := resolveRelatedNavigate(
		RelatedNavigateEvent{TargetType: "ec2", FetchFilter: filter},
		nil,
	)
	if got.Kind != NavigationKindFilteredList {
		t.Errorf("Kind = %v, want NavigationKindFilteredList", got.Kind)
	}
	if !reflect.DeepEqual(got.FetchFilter, filter) {
		t.Errorf("FetchFilter = %v, want %v", got.FetchFilter, filter)
	}
}

// Case f — TargetID present + miss → FilteredList with FilterText==TargetID.
func TestResolveRelatedNavigate_TargetIDMiss_FilteredList(t *testing.T) {
	got := resolveRelatedNavigate(
		RelatedNavigateEvent{TargetType: "ec2", TargetID: "i-missing"},
		nil,
	)
	if got.Kind != NavigationKindFilteredList {
		t.Errorf("Kind = %v, want NavigationKindFilteredList", got.Kind)
	}
	if got.FilterText != "i-missing" {
		t.Errorf("FilterText = %q, want %q", got.FilterText, "i-missing")
	}
	if got.TargetID != "i-missing" {
		t.Errorf("TargetID = %q, want %q", got.TargetID, "i-missing")
	}
}

// Case g — multiple RelatedIDs, no FetchFilter → FilteredList with RelatedIDs.
func TestResolveRelatedNavigate_MultipleRelatedIDs_FilteredList(t *testing.T) {
	relatedIDs := []string{"i-1", "i-2"}
	got := resolveRelatedNavigate(
		RelatedNavigateEvent{TargetType: "ec2", RelatedIDs: relatedIDs},
		nil,
	)
	if got.Kind != NavigationKindFilteredList {
		t.Errorf("Kind = %v, want NavigationKindFilteredList", got.Kind)
	}
	if !reflect.DeepEqual(got.RelatedIDs, relatedIDs) {
		t.Errorf("RelatedIDs = %v, want %v", got.RelatedIDs, relatedIDs)
	}
}

// Case h — default → ResourceList.
func TestResolveRelatedNavigate_Default_ResourceList(t *testing.T) {
	got := resolveRelatedNavigate(
		RelatedNavigateEvent{TargetType: "ec2"},
		nil,
	)
	if got.Kind != NavigationKindResourceList {
		t.Errorf("Kind = %v, want NavigationKindResourceList", got.Kind)
	}
}
