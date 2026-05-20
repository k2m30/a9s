package unit

// messaging_child_view_test.go — AS-726 PR-04i — verify the four messaging
// child views are registered through the new catalog child registry, with
// intact column/fetcher metadata.
//
// Per spec §4 the child-view init()s (currently in sns_sub_by_topic.go,
// eb_rule_targets.go, sfn_executions.go, sfn_execution_history.go) are
// migrated from `resource.RegisterChildType` + `resource.RegisterPaginatedChild`
// to `catalog.RegisterChildView`. `catalog.FindChild` is the new authoritative
// accessor for messaging children.
//
// The CopyField expectations come from the spec's inline-after blocks in §4:
//
//   sns_subscriptions     → CopyField "endpoint"
//   eb_rule_targets       → CopyField "target_arn"
//   sfn_executions        → CopyField "execution_arn"
//   sfn_execution_history → CopyField "event_detail"

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/catalog"
)

func TestMessagingChildViewsViaCatalog(t *testing.T) {
	cases := []struct {
		shortName string
		copyField string
	}{
		{"sns_subscriptions", "endpoint"},
		{"eb_rule_targets", "target_arn"},
		{"sfn_executions", "execution_arn"},
		{"sfn_execution_history", "event_detail"},
	}

	for _, tc := range cases {
		t.Run(tc.shortName, func(t *testing.T) {
			c := catalog.FindChild(tc.shortName)
			if c == nil {
				t.Fatalf("catalog.FindChild(%q) returned nil — messaging child view not registered via catalog", tc.shortName)
			}
			if c.ChildFetcher == nil {
				t.Errorf("%s: ChildFetcher nil on catalog child row", tc.shortName)
			}
			if len(c.Columns) == 0 {
				t.Errorf("%s: Columns empty on catalog child row", tc.shortName)
			}
			if c.CopyField != tc.copyField {
				t.Errorf("%s: CopyField = %q, want %q (per spec §4 inline-after block)",
					tc.shortName, c.CopyField, tc.copyField)
			}
		})
	}
}
