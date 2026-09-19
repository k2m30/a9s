package unit_test

// Resource.IsRelatedActionable,
// the single source of truth consumed by isActionableRow (internal/tui/views/rightcolumn.go).
//
// Only a PROVEN zero is a dead end:
//   resource.IsRelatedActionable treats a RESOLVED count==0 as ACTIONABLE when
//   it is TRUNCATED (a "0+" lower bound from a truncated target scan — more
//   may exist on later pages, so the user can drill in). Only a proven exact
//   zero (truncated==false) is a non-actionable dead end. RelatedDeferred
//   pivots (server-side FetchFilter navigation) remain actionable regardless.
//
// Cursor-skip and Tab-focus-entry mechanics are covered live by
// app_related_cursor_skip_test.go and app_related_focus_entry_test.go; the
// "(0+)"/"(0)" render contract by related_unknown_badge_test.go and
// tui_related_dim_parity_test.go.
//
// Related-panel contract: docs/related-resources.md

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

func TestIsRelatedActionable_Table(t *testing.T) {
	cases := []struct {
		name           string
		state          domain.RelatedRowState
		count          int
		truncated      bool
		wantActionable bool
	}{
		{"ProvenZero_NotActionable", domain.RelatedResolved, 0, false, false},
		{"TruncZero_IsActionable", domain.RelatedResolved, 0, true, true},
		{"UnknownCount_WithFetchFilter_Actionable", domain.RelatedDeferred, 0, false, true},
		{"UnknownCount_NoFilter_Actionable", domain.RelatedUnknown, 0, false, true},
		{"PositiveCount_NoFilter_Actionable", domain.RelatedResolved, 3, false, true},
		{"PositiveCount_Truncated_Actionable", domain.RelatedResolved, 3, true, true},
		{"Loading_BlocksRegardlessOfCount", domain.RelatedLoading, 5, false, false},
		{"Error_BlocksRegardlessOfCount", domain.RelatedError, 5, false, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := resource.IsRelatedActionable(tc.state, tc.count, tc.truncated)
			if got != tc.wantActionable {
				t.Errorf("IsRelatedActionable(state=%v, count=%d, truncated=%v) = %v, want %v",
					tc.state, tc.count, tc.truncated, got, tc.wantActionable)
			}
		})
	}
}
