package unit

import (
	"testing"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime"
)

func TestResolveRelatedNavigate(t *testing.T) {
	type tc struct {
		name  string
		ev    runtime.RelatedNavigateEvent
		cache map[string][]resource.Resource
		want  runtime.NavigationResult
	}

	cases := []tc{
		{
			name:  "unknown target type → NavigationKindFlash error",
			ev:    runtime.RelatedNavigateEvent{TargetType: "nonexistent_type_xyz"},
			cache: nil,
			want: runtime.NavigationResult{
				Kind:         runtime.NavigationKindFlash,
				FlashIsError: true,
			},
		},
		{
			name: "FetchFilter set → NavigationKindFilteredList",
			ev: runtime.RelatedNavigateEvent{
				TargetType:  "ct-events",
				FetchFilter: map[string]string{"AccessKeyId": "AKIATEST"},
			},
			cache: map[string][]resource.Resource{},
			want: runtime.NavigationResult{
				Kind:        runtime.NavigationKindFilteredList,
				TargetType:  "ct-events",
				FetchFilter: map[string]string{"AccessKeyId": "AKIATEST"},
			},
		},
		{
			name: "TargetID set + cache hit → NavigationKindDetail",
			ev:   runtime.RelatedNavigateEvent{TargetType: "s3", TargetID: "prod-logs"},
			cache: map[string][]resource.Resource{
				"s3": {{ID: "prod-logs", Name: "prod-logs"}},
			},
			want: runtime.NavigationResult{
				Kind:       runtime.NavigationKindDetail,
				TargetType: "s3",
				TargetID:   "prod-logs",
			},
		},
		{
			name: "TargetID set + cache miss → NavigationKindFilteredList with FilterText",
			ev:   runtime.RelatedNavigateEvent{TargetType: "s3", TargetID: "missing-bucket"},
			cache: map[string][]resource.Resource{
				"s3": {{ID: "other-bucket"}},
			},
			want: runtime.NavigationResult{
				Kind:       runtime.NavigationKindFilteredList,
				TargetType: "s3",
				TargetID:   "missing-bucket",
				FilterText: "missing-bucket",
			},
		},
		{
			name: "single RelatedIDs + cache hit → NavigationKindDetail",
			ev:   runtime.RelatedNavigateEvent{TargetType: "s3", RelatedIDs: []string{"prod-logs"}},
			cache: map[string][]resource.Resource{
				"s3": {{ID: "prod-logs"}},
			},
			want: runtime.NavigationResult{
				Kind:       runtime.NavigationKindDetail,
				TargetType: "s3",
				RelatedIDs: []string{"prod-logs"},
			},
		},
		{
			name: "multi RelatedIDs → NavigationKindFilteredList",
			ev:   runtime.RelatedNavigateEvent{TargetType: "ec2", RelatedIDs: []string{"i-1", "i-2"}},
			cache: map[string][]resource.Resource{
				"ec2": {{ID: "i-1"}, {ID: "i-2"}, {ID: "i-3"}},
			},
			want: runtime.NavigationResult{
				Kind:       runtime.NavigationKindFilteredList,
				TargetType: "ec2",
				RelatedIDs: []string{"i-1", "i-2"},
			},
		},
		{
			name:  "no IDs no filter → NavigationKindResourceList",
			ev:    runtime.RelatedNavigateEvent{TargetType: "ec2"},
			cache: map[string][]resource.Resource{},
			want: runtime.NavigationResult{
				Kind:       runtime.NavigationKindResourceList,
				TargetType: "ec2",
			},
		},
		{
			name:  "child type s3_objects → NavigationKindEnterChildView",
			ev:    runtime.RelatedNavigateEvent{TargetType: "s3_objects", RelatedIDs: []string{"prod-logs|app.log"}},
			cache: map[string][]resource.Resource{},
			want: runtime.NavigationResult{
				Kind:       runtime.NavigationKindEnterChildView,
				TargetType: "s3_objects",
				RelatedIDs: []string{"prod-logs|app.log"},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := runtime.ResolveRelatedNavigate(tc.ev, tc.cache)

			if got.Kind != tc.want.Kind {
				t.Errorf("Kind = %v, want %v", got.Kind, tc.want.Kind)
			}

			if tc.want.TargetType != "" && got.TargetType != tc.want.TargetType {
				t.Errorf("TargetType = %q, want %q", got.TargetType, tc.want.TargetType)
			}

			switch tc.want.Kind {
			case runtime.NavigationKindFlash:
				if !got.FlashIsError {
					t.Errorf("FlashIsError = false, want true")
				}
				t.Logf("FlashMessage = %q", got.FlashMessage)

			case runtime.NavigationKindFilteredList:
				if tc.want.TargetID != "" && got.TargetID != tc.want.TargetID {
					t.Errorf("TargetID = %q, want %q", got.TargetID, tc.want.TargetID)
				}
				if tc.want.FilterText != "" && got.FilterText != tc.want.FilterText {
					t.Errorf("FilterText = %q, want %q", got.FilterText, tc.want.FilterText)
				}
				if len(tc.want.FetchFilter) > 0 {
					for k, v := range tc.want.FetchFilter {
						if got.FetchFilter[k] != v {
							t.Errorf("FetchFilter[%q] = %q, want %q", k, got.FetchFilter[k], v)
						}
					}
				}
				if len(tc.want.RelatedIDs) > 0 {
					if len(got.RelatedIDs) != len(tc.want.RelatedIDs) {
						t.Errorf("len(RelatedIDs) = %d, want %d", len(got.RelatedIDs), len(tc.want.RelatedIDs))
					}
				}

			case runtime.NavigationKindDetail:
				if tc.want.TargetID != "" && got.TargetID != tc.want.TargetID {
					t.Errorf("TargetID = %q, want %q", got.TargetID, tc.want.TargetID)
				}
				if len(tc.want.RelatedIDs) == 1 && len(got.RelatedIDs) == 1 && got.RelatedIDs[0] != tc.want.RelatedIDs[0] {
					t.Errorf("RelatedIDs[0] = %q, want %q", got.RelatedIDs[0], tc.want.RelatedIDs[0])
				}

			case runtime.NavigationKindEnterChildView:
				if len(tc.want.RelatedIDs) > 0 && len(got.RelatedIDs) == 0 {
					t.Errorf("RelatedIDs empty, want %v", tc.want.RelatedIDs)
				}

			case runtime.NavigationKindResourceList:
				// TargetType already asserted above.
			}
		})
	}
}

// TestResolveRelatedNavigate_FetchFilterRequiresRegisteredFetcher verifies that
// a FetchFilter on a type with no registered FilteredPaginatedFetcher (e.g. "vpc")
// is silently ignored and the resolver falls through to the standard RelatedIDs
// path instead of returning NavigationKindFilteredList.
//
// Regression for: "no filtered fetcher registered for: X" runtime panic caused
// by a checker that sets FetchFilter on an unsupported target type.
func TestResolveRelatedNavigate_FetchFilterRequiresRegisteredFetcher(t *testing.T) {
	// "vpc" does not have a FilteredPaginatedFetcher registered — only "ct-events" does.
	ev := runtime.RelatedNavigateEvent{
		TargetType:  "vpc",
		FetchFilter: map[string]string{"some-key": "some-value"},
		RelatedIDs:  []string{"vpc-aaaa"},
	}
	cache := map[string][]resource.Resource{
		"vpc": {{ID: "vpc-aaaa"}},
	}

	got := runtime.ResolveRelatedNavigate(ev, cache)

	if got.Kind == runtime.NavigationKindFilteredList {
		t.Errorf("Kind = NavigationKindFilteredList, want NOT NavigationKindFilteredList: FetchFilter without registered fetcher must not take the filtered path")
	}
	if got.Kind != runtime.NavigationKindDetail {
		t.Errorf("Kind = %v, want NavigationKindDetail: single RelatedID cache hit should navigate to detail", got.Kind)
	}
	if got.TargetID != "vpc-aaaa" && (len(got.RelatedIDs) == 0 || got.RelatedIDs[0] != "vpc-aaaa") {
		t.Errorf("expected resolved ID = %q, got TargetID=%q RelatedIDs=%v", "vpc-aaaa", got.TargetID, got.RelatedIDs)
	}
}

// TestResolveRelatedNavigate_FetchFilterHonoredForCtEvents verifies that a
// FetchFilter on "ct-events" — the only type with a registered
// FilteredPaginatedFetcher — correctly takes the NavigationKindFilteredList path
// and preserves the filter map.
func TestResolveRelatedNavigate_FetchFilterHonoredForCtEvents(t *testing.T) {
	ev := runtime.RelatedNavigateEvent{
		TargetType:  "ct-events",
		FetchFilter: map[string]string{"Username": "alice"},
	}

	got := runtime.ResolveRelatedNavigate(ev, nil)

	if got.Kind != runtime.NavigationKindFilteredList {
		t.Errorf("Kind = %v, want NavigationKindFilteredList: ct-events has a registered FilteredPaginatedFetcher", got.Kind)
	}
	if got.TargetType != "ct-events" {
		t.Errorf("TargetType = %q, want %q", got.TargetType, "ct-events")
	}
	if got.FetchFilter["Username"] != "alice" {
		t.Errorf("FetchFilter[%q] = %q, want %q", "Username", got.FetchFilter["Username"], "alice")
	}
}

// TestResolveRelatedNavigate_TargetIDCacheHitWinsOverFetchFilter pins the
// precedence rule from issue #278: when an exact target is already known
// (TargetID is set and the resource is in the cache), the resolver must drill
// in directly rather than run a filtered fetch, even if FetchFilter is also
// present and the target type has a registered FilteredPaginatedFetcher.
func TestResolveRelatedNavigate_TargetIDCacheHitWinsOverFetchFilter(t *testing.T) {
	ev := runtime.RelatedNavigateEvent{
		TargetType:  "ct-events",
		TargetID:    "event-xyz",
		FetchFilter: map[string]string{"Username": "alice"},
	}
	cache := map[string][]resource.Resource{
		"ct-events": {{ID: "event-xyz", Name: "event-xyz"}},
	}

	got := runtime.ResolveRelatedNavigate(ev, cache)

	if got.Kind != runtime.NavigationKindDetail {
		t.Fatalf("Kind = %v, want NavigationKindDetail: exact TargetID cache hit must win over FetchFilter", got.Kind)
	}
	if got.TargetID != "event-xyz" {
		t.Errorf("TargetID = %q, want %q", got.TargetID, "event-xyz")
	}
}

// TestResolveRelatedNavigate_SingleRelatedIDCacheHitWinsOverFetchFilter pins
// the same precedence rule for the single-RelatedIDs path.
func TestResolveRelatedNavigate_SingleRelatedIDCacheHitWinsOverFetchFilter(t *testing.T) {
	ev := runtime.RelatedNavigateEvent{
		TargetType:  "ct-events",
		RelatedIDs:  []string{"event-xyz"},
		FetchFilter: map[string]string{"Username": "alice"},
	}
	cache := map[string][]resource.Resource{
		"ct-events": {{ID: "event-xyz", Name: "event-xyz"}},
	}

	got := runtime.ResolveRelatedNavigate(ev, cache)

	if got.Kind != runtime.NavigationKindDetail {
		t.Fatalf("Kind = %v, want NavigationKindDetail: single RelatedID cache hit must win over FetchFilter", got.Kind)
	}
	if len(got.RelatedIDs) != 1 || got.RelatedIDs[0] != "event-xyz" {
		t.Errorf("RelatedIDs = %v, want [event-xyz]", got.RelatedIDs)
	}
}

// TestResolveRelatedNavigate_TargetIDCacheMiss_FallsBackToFetchFilter verifies
// the complement: when the exact target is NOT in cache, the resolver falls
// back to the FetchFilter path so the live filtered fetch still runs.
func TestResolveRelatedNavigate_TargetIDCacheMiss_FallsBackToFetchFilter(t *testing.T) {
	ev := runtime.RelatedNavigateEvent{
		TargetType:  "ct-events",
		TargetID:    "event-missing",
		FetchFilter: map[string]string{"Username": "alice"},
	}
	cache := map[string][]resource.Resource{
		"ct-events": {{ID: "event-xyz"}},
	}

	got := runtime.ResolveRelatedNavigate(ev, cache)

	if got.Kind != runtime.NavigationKindFilteredList {
		t.Fatalf("Kind = %v, want NavigationKindFilteredList: TargetID cache miss + FetchFilter with registered fetcher must fall back to filtered fetch", got.Kind)
	}
	if got.FetchFilter["Username"] != "alice" {
		t.Errorf("FetchFilter[Username] = %q, want alice — filter must be preserved on fallback", got.FetchFilter["Username"])
	}
}
