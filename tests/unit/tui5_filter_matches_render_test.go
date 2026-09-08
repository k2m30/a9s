// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// tui5_filter_matches_render_test.go — the text filter matches what the screen
// shows. A humanized column renders a readable cause, so that is what the
// operator types; the raw AWS constant behind it appears on no surface and
// must match nothing.
package unit_test

import (
	"context"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/resource"
)

// filterHits returns how many rows survive a text filter on a real demo list.
func filterHits(t *testing.T, shortName, query string) (int, *app.ListBody) {
	t.Helper()
	td := resource.FindResourceType(shortName)
	if td == nil {
		t.Fatalf("%s is not a registered resource type", shortName)
	}
	page, _ := td.Fetcher(context.Background(), demo.NewServiceClients(), "")
	if len(page.Resources) == 0 {
		t.Fatalf("%s has no demo fixtures", shortName)
	}

	c := openListController(t, shortName)
	c.ApplyResourcesLoaded(shortName, page.Resources, nil, false)
	c.Apply(app.Action{Kind: app.ActionSetFilter, Arg: query})
	body := c.Snapshot().Body.List
	if body == nil {
		t.Fatalf("%s: no list body", shortName)
	}
	return len(body.Rows), body
}

// TestFilterMatchesRenderedCell_HumanizedPathColumn pins both directions on
// the two columns the humanize flag reaches: the words on screen find the row,
// and the raw constant no surface shows finds nothing.
func TestFilterMatchesRenderedCell_HumanizedPathColumn(t *testing.T) {
	for _, tc := range []struct {
		shortName string
		rendered  string // what the cell says
		raw       string // the AWS constant behind it
	}{
		{"nat", "insufficient free addresses", "InsufficientFreeAddressesInSubnet"},
		{"ecs-task", "task failed to start", "TaskFailedToStart"},
	} {
		t.Run(tc.shortName, func(t *testing.T) {
			if n, _ := filterHits(t, tc.shortName, tc.rendered); n == 0 {
				t.Errorf("filter %q matched no row, but a cell renders exactly that", tc.rendered)
			}
			if n, _ := filterHits(t, tc.shortName, tc.raw); n != 0 {
				t.Errorf("filter %q matched %d row(s); no surface shows that constant", tc.raw, n)
			}
		})
	}
}
