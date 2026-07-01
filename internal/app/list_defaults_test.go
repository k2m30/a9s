package app

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/runtime"
	"github.com/k2m30/a9s/v3/internal/session"
)

// TestEnsureListState_SeedsCTEventsDefaultSort verifies that a freshly-created
// ct-events list screen is seeded with the event_time DESC default sort by the
// controller (applyListDefaults via ensureListState), not by the per-keystroke
// view constructor. Guards the stack-lift move of the default-sort logic.
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
	if ls.SortCol != "event_time" || ls.SortDir != "desc" {
		t.Errorf("ct-events default sort = %q/%q, want event_time/desc", ls.SortCol, ls.SortDir)
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
