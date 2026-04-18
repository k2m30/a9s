package unit

import (
	"testing"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
)

func TestResolveRelatedNavigate(t *testing.T) {
	type tc struct {
		name  string
		msg   messages.RelatedNavigateMsg
		cache map[string][]resource.Resource
		want  tui.NavigationResult
	}

	cases := []tc{
		{
			name:  "unknown target type → KindFlash error",
			msg:   messages.RelatedNavigateMsg{TargetType: "nonexistent_type_xyz"},
			cache: nil,
			want: tui.NavigationResult{
				Kind:         tui.KindFlash,
				FlashIsError: true,
			},
		},
		{
			name: "FetchFilter set → KindFilteredList",
			msg: messages.RelatedNavigateMsg{
				TargetType:  "ct-events",
				FetchFilter: map[string]string{"AccessKeyId": "AKIATEST"},
			},
			cache: map[string][]resource.Resource{},
			want: tui.NavigationResult{
				Kind:        tui.KindFilteredList,
				TargetType:  "ct-events",
				FetchFilter: map[string]string{"AccessKeyId": "AKIATEST"},
			},
		},
		{
			name: "TargetID set + cache hit → KindDetail",
			msg:  messages.RelatedNavigateMsg{TargetType: "s3", TargetID: "prod-logs"},
			cache: map[string][]resource.Resource{
				"s3": {{ID: "prod-logs", Name: "prod-logs"}},
			},
			want: tui.NavigationResult{
				Kind:       tui.KindDetail,
				TargetType: "s3",
				TargetID:   "prod-logs",
			},
		},
		{
			name: "TargetID set + cache miss → KindFilteredList with FilterText",
			msg:  messages.RelatedNavigateMsg{TargetType: "s3", TargetID: "missing-bucket"},
			cache: map[string][]resource.Resource{
				"s3": {{ID: "other-bucket"}},
			},
			want: tui.NavigationResult{
				Kind:       tui.KindFilteredList,
				TargetType: "s3",
				TargetID:   "missing-bucket",
				FilterText: "missing-bucket",
			},
		},
		{
			name: "single RelatedIDs + cache hit → KindDetail",
			msg:  messages.RelatedNavigateMsg{TargetType: "s3", RelatedIDs: []string{"prod-logs"}},
			cache: map[string][]resource.Resource{
				"s3": {{ID: "prod-logs"}},
			},
			want: tui.NavigationResult{
				Kind:       tui.KindDetail,
				TargetType: "s3",
				RelatedIDs: []string{"prod-logs"},
			},
		},
		{
			name: "multi RelatedIDs → KindFilteredList",
			msg:  messages.RelatedNavigateMsg{TargetType: "ec2", RelatedIDs: []string{"i-1", "i-2"}},
			cache: map[string][]resource.Resource{
				"ec2": {{ID: "i-1"}, {ID: "i-2"}, {ID: "i-3"}},
			},
			want: tui.NavigationResult{
				Kind:       tui.KindFilteredList,
				TargetType: "ec2",
				RelatedIDs: []string{"i-1", "i-2"},
			},
		},
		{
			name:  "no IDs no filter → KindResourceList",
			msg:   messages.RelatedNavigateMsg{TargetType: "ec2"},
			cache: map[string][]resource.Resource{},
			want: tui.NavigationResult{
				Kind:       tui.KindResourceList,
				TargetType: "ec2",
			},
		},
		{
			name:  "child type s3_objects → KindEnterChildView",
			msg:   messages.RelatedNavigateMsg{TargetType: "s3_objects", RelatedIDs: []string{"prod-logs|app.log"}},
			cache: map[string][]resource.Resource{},
			want: tui.NavigationResult{
				Kind:       tui.KindEnterChildView,
				TargetType: "s3_objects",
				RelatedIDs: []string{"prod-logs|app.log"},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tui.ResolveRelatedNavigate(tc.msg, tc.cache)

			if got.Kind != tc.want.Kind {
				t.Errorf("Kind = %v, want %v", got.Kind, tc.want.Kind)
			}

			if tc.want.TargetType != "" && got.TargetType != tc.want.TargetType {
				t.Errorf("TargetType = %q, want %q", got.TargetType, tc.want.TargetType)
			}

			switch tc.want.Kind {
			case tui.KindFlash:
				if !got.FlashIsError {
					t.Errorf("FlashIsError = false, want true")
				}
				t.Logf("FlashMessage = %q", got.FlashMessage)

			case tui.KindFilteredList:
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

			case tui.KindDetail:
				if tc.want.TargetID != "" && got.TargetID != tc.want.TargetID {
					t.Errorf("TargetID = %q, want %q", got.TargetID, tc.want.TargetID)
				}
				if len(tc.want.RelatedIDs) == 1 && len(got.RelatedIDs) == 1 && got.RelatedIDs[0] != tc.want.RelatedIDs[0] {
					t.Errorf("RelatedIDs[0] = %q, want %q", got.RelatedIDs[0], tc.want.RelatedIDs[0])
				}

			case tui.KindEnterChildView:
				if len(tc.want.RelatedIDs) > 0 && len(got.RelatedIDs) == 0 {
					t.Errorf("RelatedIDs empty, want %v", tc.want.RelatedIDs)
				}

			case tui.KindResourceList:
				// TargetType already asserted above.
			}
		})
	}
}

// TestResolveRelatedNavigate_FetchFilterRequiresRegisteredFetcher verifies that
// a FetchFilter on a type with no registered FilteredPaginatedFetcher (e.g. "vpc")
// is silently ignored and the resolver falls through to the standard RelatedIDs
// path instead of returning KindFilteredList.
//
// Regression for: "no filtered fetcher registered for: X" runtime panic caused
// by a checker that sets FetchFilter on an unsupported target type.
func TestResolveRelatedNavigate_FetchFilterRequiresRegisteredFetcher(t *testing.T) {
	// "vpc" does not have a FilteredPaginatedFetcher registered — only "ct-events" does.
	msg := messages.RelatedNavigateMsg{
		TargetType:  "vpc",
		FetchFilter: map[string]string{"some-key": "some-value"},
		RelatedIDs:  []string{"vpc-aaaa"},
	}
	cache := map[string][]resource.Resource{
		"vpc": {{ID: "vpc-aaaa"}},
	}

	got := tui.ResolveRelatedNavigate(msg, cache)

	// The guard must strip the filter path when no fetcher is registered.
	if got.Kind == tui.KindFilteredList {
		t.Errorf("Kind = KindFilteredList, want NOT KindFilteredList: FetchFilter without registered fetcher must not take the filtered path")
	}
	// With 1 RelatedID and a cache hit the resolver should land on KindDetail.
	if got.Kind != tui.KindDetail {
		t.Errorf("Kind = %v, want KindDetail: single RelatedID cache hit should navigate to detail", got.Kind)
	}
	if got.TargetID != "vpc-aaaa" && (len(got.RelatedIDs) == 0 || got.RelatedIDs[0] != "vpc-aaaa") {
		t.Errorf("expected resolved ID = %q, got TargetID=%q RelatedIDs=%v", "vpc-aaaa", got.TargetID, got.RelatedIDs)
	}
}

// TestResolveRelatedNavigate_FetchFilterHonoredForCtEvents verifies that a
// FetchFilter on "ct-events" — the only type with a registered
// FilteredPaginatedFetcher — correctly takes the KindFilteredList path and
// preserves the filter map.
func TestResolveRelatedNavigate_FetchFilterHonoredForCtEvents(t *testing.T) {
	msg := messages.RelatedNavigateMsg{
		TargetType:  "ct-events",
		FetchFilter: map[string]string{"Username": "alice"},
	}

	got := tui.ResolveRelatedNavigate(msg, nil)

	if got.Kind != tui.KindFilteredList {
		t.Errorf("Kind = %v, want KindFilteredList: ct-events has a registered FilteredPaginatedFetcher", got.Kind)
	}
	if got.TargetType != "ct-events" {
		t.Errorf("TargetType = %q, want %q", got.TargetType, "ct-events")
	}
	if got.FetchFilter["Username"] != "alice" {
		t.Errorf("FetchFilter[%q] = %q, want %q", "Username", got.FetchFilter["Username"], "alice")
	}
}
