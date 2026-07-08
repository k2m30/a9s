// detail_controller_scroll_follow_test.go — TDD red-phase pin for the
// detail-view scroll-follows-cursor bug: applyDetailActions (internal/app/
// detail_cursor.go) moves DetailState.FieldCursor on ActionMoveUp/MoveDown/
// MoveBottom but never reconciles DetailState.ScrollY, so the highlighted
// field can scroll off the bottom of the viewport. ActionMoveTop and
// PageUp/PageDown already mutate ScrollY; MoveUp/MoveDown/MoveBottom must
// reconcile it the same way, using the controller-stored DetailState.
// ViewportHeight (set once via Controller.SetDetailViewportHeight) as the
// renderer-supplied usable viewport height — the controller is the single
// owner of scroll reconciliation, so move actions no longer read Action.N
// per key-dispatch call site (the live path forgot to pass it — the bug)
// (mirrors the legacy syncViewportToCursor in
// internal/tui/views/detail_helpers.go:250-261).
package unit_test

import (
	"fmt"
	"testing"

	"github.com/k2m30/a9s/v3/internal/app"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// scrollProbeFieldCount is the number of plain (unregistered-type) fields
// seeded onto the probe resource — comfortably larger than
// scrollProbeHeight so cursor movement must scroll the viewport to keep the
// selection visible.
const scrollProbeFieldCount = 32

// scrollProbeHeight is the viewport height set once via
// Controller.SetDetailViewportHeight for these tests — mirrors the
// renderer-supplied usable field-list height for PageUp/PageDown.
const scrollProbeHeight = 10

// manyFieldsResource returns a resource with count plain Fields entries,
// keyed "field-00".."field-NN" so alphabetical sort (the unregistered-type
// flat-rendering path in projection.Generic — the same fallback exercised by
// detailParityMinimalResource/detailParityEmptyResource in
// detail_render_parity_test.go) yields a deterministic, section-free,
// spacer-free field-item list of length count: one FieldRow per entry, with
// no header/spacer skip logic for cursor movement to account for.
func manyFieldsResource(count int) resource.Resource {
	fields := make(map[string]string, count)
	for i := 0; i < count; i++ {
		key := fmt.Sprintf("field-%02d", i)
		fields[key] = fmt.Sprintf("value-%02d", i)
	}
	return resource.Resource{
		ID:     "scroll-probe-001",
		Name:   "scroll-probe",
		Fields: fields,
	}
}

// newScrollProbeController builds a Controller with a Detail screen pushed
// for an unregistered resource type carrying scrollProbeFieldCount plain
// fields, via the shared newDetailController harness (detail_render_parity_test.go).
func newScrollProbeController(t *testing.T) *app.Controller {
	t.Helper()
	return newDetailController(t, manyFieldsResource(scrollProbeFieldCount), "")
}

// assertScrollInvariant fails the test unless the cursor is within the
// visible viewport window [ScrollY, ScrollY+height) — the contract any
// scroll-follows-cursor reconciliation must uphold.
func assertScrollInvariant(t *testing.T, body *app.DetailBody, height int) {
	t.Helper()
	if body.FieldCursor < body.ScrollY || body.FieldCursor >= body.ScrollY+height {
		t.Fatalf("scroll invariant violated: ScrollY=%d FieldCursor=%d height=%d (want ScrollY <= FieldCursor < ScrollY+height)",
			body.ScrollY, body.FieldCursor, height)
	}
}

// TestDetailControllerScrollFollowsCursorDown pins the fix for
// ActionMoveDown: repeatedly moving the cursor down must keep it inside the
// visible viewport, and once the cursor has moved past the first page the
// viewport must actually have scrolled (ScrollY>0) — not just left the
// cursor invisibly off-screen.
func TestDetailControllerScrollFollowsCursorDown(t *testing.T) {
	c := newScrollProbeController(t)
	c.SetDetailViewportHeight(scrollProbeHeight)

	sawScroll := false
	for i := 0; i < 20; i++ {
		c.Apply(app.Action{Kind: app.ActionMoveDown})

		vs := c.Snapshot()
		if vs.Body.Detail == nil {
			t.Fatalf("iteration %d: Body.Detail is nil after ActionMoveDown", i)
		}
		body := vs.Body.Detail
		assertScrollInvariant(t, body, scrollProbeHeight)

		if body.FieldCursor >= scrollProbeHeight {
			if body.ScrollY <= 0 {
				t.Fatalf("iteration %d: FieldCursor=%d >= height=%d but ScrollY=%d — viewport did not follow the cursor",
					i, body.FieldCursor, scrollProbeHeight, body.ScrollY)
			}
			sawScroll = true
		}
	}
	if !sawScroll {
		t.Fatalf("test setup problem: FieldCursor never reached height=%d across 20 ActionMoveDown applications (field count=%d)",
			scrollProbeHeight, scrollProbeFieldCount)
	}
}

// TestDetailControllerMoveBottomScrolls pins the fix for ActionMoveBottom:
// jumping straight to the last field must also scroll the viewport so the
// selected (last) field is actually visible.
func TestDetailControllerMoveBottomScrolls(t *testing.T) {
	c := newScrollProbeController(t)
	c.SetDetailViewportHeight(scrollProbeHeight)

	c.Apply(app.Action{Kind: app.ActionMoveBottom})

	vs := c.Snapshot()
	if vs.Body.Detail == nil {
		t.Fatal("Body.Detail is nil after ActionMoveBottom")
	}
	body := vs.Body.Detail

	wantCursor := scrollProbeFieldCount - 1
	if body.FieldCursor != wantCursor {
		t.Fatalf("FieldCursor = %d, want %d (last field index)", body.FieldCursor, wantCursor)
	}
	assertScrollInvariant(t, body, scrollProbeHeight)
}

// TestDetailControllerMoveUpReturnsScrollToTop pins the fix for
// ActionMoveUp: walking the cursor back up from the bottom to the very
// first field must keep it inside the viewport at every step (not just at
// the final position — ScrollY sitting at 0 throughout, per the current
// bug, would otherwise satisfy a end-of-walk-only "ScrollY==0" assertion by
// accident) and must bring the viewport back to ScrollY==0 once the cursor
// reaches the top.
func TestDetailControllerMoveUpReturnsScrollToTop(t *testing.T) {
	c := newScrollProbeController(t)
	c.SetDetailViewportHeight(scrollProbeHeight)

	c.Apply(app.Action{Kind: app.ActionMoveBottom})
	pre := c.Snapshot().Body.Detail
	if pre == nil || pre.FieldCursor != scrollProbeFieldCount-1 {
		t.Fatalf("precondition: ActionMoveBottom did not land on the last field, got body=%+v", pre)
	}

	for guard := 0; ; guard++ {
		body := c.Snapshot().Body.Detail
		if body.FieldCursor == 0 {
			break
		}
		assertScrollInvariant(t, body, scrollProbeHeight)
		if guard > scrollProbeFieldCount+5 {
			t.Fatalf("FieldCursor never reached 0 after %d ActionMoveUp applications", guard)
		}
		c.Apply(app.Action{Kind: app.ActionMoveUp})
	}

	body := c.Snapshot().Body.Detail
	if body.ScrollY != 0 {
		t.Fatalf("ScrollY = %d, want 0 once FieldCursor is back at the top", body.ScrollY)
	}
}

// TestDetailControllerMoveDownNoHeightLeavesScrollUnchanged is a regression
// guard: when the controller was never told the viewport height (no
// SetDetailViewportHeight call, so DetailState.ViewportHeight stays at its
// zero value, e.g. a screen not yet rendered by any renderer), scroll
// reconciliation must be skipped entirely — ScrollY stays whatever it
// already was. This must stay green across the fix.
func TestDetailControllerMoveDownNoHeightLeavesScrollUnchanged(t *testing.T) {
	c := newScrollProbeController(t)

	c.Apply(app.Action{Kind: app.ActionMoveDown})

	body := c.Snapshot().Body.Detail
	if body == nil {
		t.Fatal("Body.Detail is nil after ActionMoveDown")
	}
	if body.ScrollY != 0 {
		t.Fatalf("ScrollY = %d, want 0 (ViewportHeight was never set via SetDetailViewportHeight, so it is <=0 and reconcile must no-op)", body.ScrollY)
	}
}
