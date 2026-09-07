// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package app

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/session"
)

// TestEnsureListState_SeedsCTEventsDefaultSort verifies that a freshly-created
// ct-events list screen is seeded with the newest-first default sort by the
// controller (applyListDefaults via ensureListState), not by the per-keystroke
// view constructor. Guards the stack-lift move of the default-sort logic.
//
// The seeded value used to be "event_time", which is the Fields key that
// column sorts by and not a name any column answers to: the sort could
// therefore never be saved or restored, and the header arrow never appeared.
// The sort task made a column's name its own — "time" here — with the Fields
// key it compares carried on the column as its sort_key. Do not restore
// "event_time" as the seeded column.
func TestEnsureListState_SeedsCTEventsDefaultSort(t *testing.T) {
	c := New(runtime.New(session.New(), nil))
	c.ApplyIntents([]runtime.UIIntent{
		runtime.PushScreen{
			ID:      runtime.ScreenResourceList,
			Context: runtime.ScreenContext{ResourceType: "ct-events"},
		},
	})
	c.ensureListState()

	ls := c.topListState()
	if ls == nil {
		t.Fatal("topListState nil after ensureListState")
	}
	if ls.SortCol != "time" || ls.SortDir != "desc" {
		t.Errorf("ct-events default sort = %q/%q, want time/desc", ls.SortCol, ls.SortDir)
	}
}

// TestApplyListDefaults_NonCTEventsNoSeed verifies only ct-events gets a default
// sort; every other type starts unsorted so the renderer shows no sort glyph.
func TestApplyListDefaults_NonCTEventsNoSeed(t *testing.T) {
	ls := &ListState{Loading: true}
	applyListDefaults(ls, "ec2")
	if ls.SortCol != "" || ls.SortDir != "" {
		t.Errorf("ec2 got default sort %q/%q, want none", ls.SortCol, ls.SortDir)
	}
}
