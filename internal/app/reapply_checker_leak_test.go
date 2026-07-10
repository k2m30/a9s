package app

import (
	"context"
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime"
	"github.com/k2m30/a9s/v3/internal/session"
)

// TestReapplyChecker_NotLeakedToNormalListAfterPop pins that a reapply checker
// registered while a truncated related list of type T is on top does not
// survive that list being popped: a later NORMAL list of type T must not
// inherit the stale checker and get a RelatedIDSet imposed on it (which would
// hide rows the normal list should show).
//
// Pre-fix failure: the checker was stored in a per-type controller map, never
// cleared when the related list was popped, so reapplyCheckerAgainst re-ran the
// old checker against the normal list's page and created a RelatedIDSet on it —
// e.g. after VPC -> Security Groups, opening the plain sg list filtered it to
// the old VPC's SGs.
func TestReapplyChecker_NotLeakedToNormalListAfterPop(t *testing.T) {
	c := New(runtime.New(session.New(), nil))

	// A related "sg" list on top, with a reapply checker that (like a truncated
	// VPC -> SG scan) narrows to a subset of the loaded rows.
	c.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{
		ID:      runtime.ScreenResourceList,
		Context: runtime.ScreenContext{ResourceType: "sg"},
	}})
	c.ensureListState()
	subset := func(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
		return resource.RelatedCheckResult{TargetType: "sg", ResourceIDs: []string{"sg-a"}}
	}
	c.PatchListReapplyChecker(subset, resource.Resource{ID: "vpc-1"})

	// Leave that related list (mimics Esc / pop).
	c.stack = c.stack[:len(c.stack)-1]

	// A fresh NORMAL list of the same type.
	c.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{
		ID:      runtime.ScreenResourceList,
		Context: runtime.ScreenContext{ResourceType: "sg"},
	}})
	c.ensureListState()
	ls := c.topListState()

	// A page lands for the normal list — the same call handle.go makes on
	// ResourcesLoaded.
	c.reapplyCheckerAgainst(ls, "sg", []resource.Resource{{ID: "sg-a"}, {ID: "sg-b"}})

	if ls.RelatedIDSet != nil {
		t.Fatalf("stale reapply checker leaked onto a normal sg list: RelatedIDSet=%v, want nil (a normal list must not be related-filtered)", ls.RelatedIDSet)
	}
}
