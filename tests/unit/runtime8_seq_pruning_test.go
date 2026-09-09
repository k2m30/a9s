// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// runtime8_seq_pruning_test.go — a screen's sequence dies with the screen.
//
// The list ordering counter is keyed by the screen instance that dispatched,
// which is what lets a drill order its own refreshes. A screen instance is
// never reused, so its entry answers nothing once the screen is popped, and an
// operator who opens and closes drills all afternoon leaves one entry behind
// per drill.
package unit_test

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
)

// openSeqDrill pushes a drill of the given type, dispatches one refresh from
// it so it draws a sequence, and returns the screen instance it drew for.
func openSeqDrill(t *testing.T, c *app.Controller) domain.Gen {
	t.Helper()
	c.PushChildListScreen("ec2")
	c.PatchListEscPops(true)
	c.ApplyResourcesLoaded("ec2", []resource.Resource{guardRow("i-0ddd444444444444d", "drill")}, nil, false)
	screen := c.GetListInstance()
	if screen == 0 {
		t.Fatal("the pushed drill has no screen instance, so it can draw no sequence")
	}
	_, tasks := c.Apply(app.Action{Kind: app.ActionRefresh})
	if got := guardFetchTask(t, tasks).ScreenID; got != screen {
		t.Fatalf("the drill's refresh names screen %d, want %d", got, screen)
	}
	return screen
}

// TestListFetchSeq_APoppedScreensSequenceIsForgotten pins the pruning. A
// popped screen's key answers nothing: no request of its can still be in
// flight that any surface would accept, because the screen the answer would
// land on is gone.
func TestListFetchSeq_APoppedScreensSequenceIsForgotten(t *testing.T) {
	c, core := newTestControllerAndCore(t)
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	c.ApplyResourcesLoaded("ec2", []resource.Resource{guardRow("i-0aaa111111111111a", "parent")}, nil, false)

	screen := openSeqDrill(t, c)
	if core.LatestListFetchSeq(screen) == 0 {
		t.Fatal("the open drill drew no sequence, so this pin has nothing to see pruned")
	}

	c.ApplyIntents([]runtime.UIIntent{runtime.PopScreen{}})

	if got := core.LatestListFetchSeq(screen); got != 0 {
		t.Errorf("the popped drill's screen %d still holds sequence %d — its entry is kept for the rest "+
			"of the session, and one is added for every drill the operator ever opens", screen, got)
	}
}

// TestListFetchSeq_OpeningAndClosingDrillsLeavesNothingBehind is the same
// property over a session's worth of drilling: every screen opened and closed
// is forgotten, and the one still open is not.
func TestListFetchSeq_OpeningAndClosingDrillsLeavesNothingBehind(t *testing.T) {
	c, core := newTestControllerAndCore(t)
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	c.ApplyResourcesLoaded("ec2", []resource.Resource{guardRow("i-0aaa111111111111a", "parent")}, nil, false)

	var closed []domain.Gen
	for range 20 {
		screen := openSeqDrill(t, c)
		closed = append(closed, screen)
		c.ApplyIntents([]runtime.UIIntent{runtime.PopScreen{}})
	}

	var kept []domain.Gen
	for _, screen := range closed {
		if core.LatestListFetchSeq(screen) != 0 {
			kept = append(kept, screen)
		}
	}
	if len(kept) > 0 {
		t.Errorf("%d of 20 closed drills still hold a sequence (%v) — the map grows by one entry per drill "+
			"for the life of the session", len(kept), kept)
	}

	// The drill still on screen keeps its own, or the fix has pruned a live
	// screen's ordering out from under it.
	live := openSeqDrill(t, c)
	if core.LatestListFetchSeq(live) == 0 {
		t.Errorf("the drill still open (screen %d) lost its sequence — pruning must follow the pop, not "+
			"every pop", live)
	}
}
