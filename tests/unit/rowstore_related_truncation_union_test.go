package unit_test

// rowstore_related_truncation_union_test.go — regression pin for
// core/runtime/rowstore_observe.go's resolveCachedPagePagination (#261
// boundary-sealing wave, item b): truncation is a UNION of
// entry.IsTruncated and entry.Pagination.IsTruncated, never a downgrade.
// Before the fix, a non-nil entry.Pagination won outright — an entry
// carrying IsTruncated=true alongside a Pagination whose own IsTruncated
// field was false silently lost the truncation signal. Driven through
// app.Controller.Handle(messages.RelatedCheckResult{CachedPages: ...}) —
// Core.HandleRelatedCheckResult's own CachedPages loop
// (core/runtime/handlers_resources.go) calls resolveCachedPagePagination to
// build the PatchResourceCache intent, the exact real path, not the
// unexported helper directly. (Core.HandleEvent's RowStore dual-write lane,
// observeRelatedCheckResultRows, shares the same helper but deliberately
// does not surface raw Pagination through BuildResourceCacheSnapshot —
// Core.ResourceCache, reading PatchResourceCache's own
// domain.ListViewCacheEntry, is this fix's one directly observable
// read-back.)

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

func TestResolveCachedPagePagination_TruncationUnion_ThroughRelatedCheckResultFold(t *testing.T) {
	const targetType = "test-truncation-union-target"

	cases := []struct {
		name           string
		entryTruncated bool
		pagination     *resource.PaginationMeta
		wantNilResult  bool
		wantTruncated  bool
		wantNextToken  string // only checked when pagination was non-nil
	}{
		{
			name:           "NilPagination_EntryNotTruncated_NoPaginationStored",
			entryTruncated: false,
			pagination:     nil,
			wantNilResult:  true,
		},
		{
			name:           "NilPagination_EntryTruncated_SynthesizesTruncatedTrue",
			entryTruncated: true,
			pagination:     nil,
			wantTruncated:  true,
		},
		{
			name:           "NonNilPaginationFalse_EntryTruncated_UnionYieldsTruncated",
			entryTruncated: true,
			pagination:     &resource.PaginationMeta{IsTruncated: false, NextToken: "tok-a"},
			wantTruncated:  true,
			wantNextToken:  "tok-a",
		},
		{
			name:           "NonNilPaginationTrue_EntryNotTruncated_UnionYieldsTruncated",
			entryTruncated: false,
			pagination:     &resource.PaginationMeta{IsTruncated: true, NextToken: "tok-b"},
			wantTruncated:  true,
			wantNextToken:  "tok-b",
		},
		{
			name:           "NonNilPaginationFalse_EntryNotTruncated_BothAgreeFalse",
			entryTruncated: false,
			pagination:     &resource.PaginationMeta{IsTruncated: false, NextToken: "tok-c"},
			wantTruncated:  false,
			wantNextToken:  "tok-c",
		},
	}

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, core := newTestControllerAndCore(t)
			// A distinct type name per subtest: HandleRelatedCheckResult's
			// CachedPages loop skips a type RowStore already has a Gen!=0
			// entry for (core/runtime/handlers_resources.go), so each
			// subtest needs its own never-before-seen target.
			tt := targetType + string(rune('a'+i))

			var originalTruncated bool
			if tc.pagination != nil {
				originalTruncated = tc.pagination.IsTruncated
			}

			c.Handle(messages.RelatedCheckResult{
				ResourceType:     "ec2",
				SourceResourceID: "i-truncation-union-1",
				DefDisplayName:   "Truncation Union Target",
				Result:           resource.KnownRelated(tt, []string{"r-1"}, false),
				CachedPages: map[string]resource.ResourceCacheEntry{
					tt: {
						Resources:   []resource.Resource{{ID: "r-1", Type: tt}},
						IsTruncated: tc.entryTruncated,
						Pagination:  tc.pagination,
					},
				},
			})

			// The caller's own *PaginationMeta must never be mutated in
			// place — resolveCachedPagePagination copies before flipping
			// IsTruncated, since the same pointer also reaches RowStore's
			// dual-write lane unchanged.
			if tc.pagination != nil && tc.pagination.IsTruncated != originalTruncated {
				t.Errorf("caller's original PaginationMeta.IsTruncated mutated in place: was %v, now %v", originalTruncated, tc.pagination.IsTruncated)
			}

			entry, ok := core.ResourceCache(tt)
			if !ok {
				t.Fatalf("Core.ResourceCache(%q) has no entry after the RelatedCheckResult fold", tt)
			}

			if tc.wantNilResult {
				if entry.Pagination != nil {
					t.Errorf("Pagination = %+v, want nil (no truncation signal at all)", entry.Pagination)
				}
				return
			}
			if entry.Pagination == nil {
				t.Fatalf("Pagination = nil, want non-nil with IsTruncated=%v", tc.wantTruncated)
			}
			if entry.Pagination.IsTruncated != tc.wantTruncated {
				t.Errorf("Pagination.IsTruncated = %v, want %v (union of entry.IsTruncated=%v and Pagination.IsTruncated=%v)",
					entry.Pagination.IsTruncated, tc.wantTruncated, tc.entryTruncated, originalTruncated)
			}
			if tc.wantNextToken != "" && entry.Pagination.NextToken != tc.wantNextToken {
				t.Errorf("Pagination.NextToken = %q, want %q preserved from the original Pagination (only IsTruncated should change)", entry.Pagination.NextToken, tc.wantNextToken)
			}
		})
	}
}
