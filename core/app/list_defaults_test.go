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
// view constructor.
//
// The seeded column is "time", a name the column answers to; the Fields key
// it compares is carried on the column as its sort_key. A Fields key such as
// "event_time" names no column, so a sort seeded with it could never be
// saved or restored and the header arrow would never appear.
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
