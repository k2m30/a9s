package unit

import (
	"testing"

	_ "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
)

// TestResolveRelatedNavigate_TruncatedSingleCachedID_StaysScanList pins that a
// truncated ("(1+)") reverse-scan pivot with exactly ONE found ID resolves to
// the scan list (NavigationKindFilteredList + Truncated) — never a single
// detail — even though that one found id is already cached and even when the
// caller also populated TargetID.
//
// Pre-fix failure: the single-RelatedID cache-hit branch (and the TargetID
// cache-hit / cache-miss branches) ran before the truncated branch, so a "(1+)"
// pivot opened one detail / a by-ID list and never emitted the population fetch
// + reapply needed to discover the remaining matches on later pages.
func TestResolveRelatedNavigate_TruncatedSingleCachedID_StaysScanList(t *testing.T) {
	cache := map[string][]resource.Resource{"ec2": {{ID: "i-1"}}}

	cases := []struct {
		name string
		ev   runtime.RelatedNavigateEvent
	}{
		{
			name: "RelatedIDs only",
			ev:   runtime.RelatedNavigateEvent{TargetType: "ec2", RelatedIDs: []string{"i-1"}, Truncated: true},
		},
		{
			name: "TargetID also set",
			ev:   runtime.RelatedNavigateEvent{TargetType: "ec2", RelatedIDs: []string{"i-1"}, TargetID: "i-1", Truncated: true},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := runtime.ResolveRelatedNavigate(tc.ev, cache)
			if got.Kind != runtime.NavigationKindFilteredList {
				t.Fatalf("truncated single-ID pivot must resolve to a scan list; got Kind=%v, want NavigationKindFilteredList", got.Kind)
			}
			if !got.Truncated {
				t.Fatalf("Truncated must survive resolution so the population fetch + reapply run; got Truncated=false")
			}
			if got.TargetID != "" {
				t.Fatalf("a truncated scan must not carry a single-target detail id; got TargetID=%q", got.TargetID)
			}
		})
	}
}
