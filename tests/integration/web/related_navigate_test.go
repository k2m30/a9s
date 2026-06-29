//go:build integration

package webintegration

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/app"
)

// TestWebRelatedNavigate_SingleTarget_SeedsDetailFromCache guards the
// related-navigate regression where navigating into a single-target related row
// (NavigationKindDetail cache-hit path) produced an empty detail placeholder:
// FrameTitle fell back to the literal screen-ID string "detail" and Fields was
// empty, because the old code relied on a by-id fetcher that most resource
// types don't register.
//
// The fix (applyRelatedNavResult NavigationKindDetail case) seeds the detail
// synchronously from the already-cached resource via Core.RelatedCachedResource.
//
// Sequence:
//  1. Navigate to the ec2 list → first row is "web-prod-01".
//  2. ActionOpenDetail → detail for web-prod-01; its first related row is
//     "Target Groups" (TargetType "tg"), which has count=1 (acme-web-tg).
//  3. ActionToggleFocus → focus the related panel.
//  4. ActionSelect → navigates into the focused single-target row.
//
// Assertions:
//   - Body.Kind == "detail" (not list / menu / text).
//   - FrameTitle == "acme-web-tg" — the cached target group was seeded, not an
//     empty placeholder (which would produce FrameTitle == "detail" or "").
//   - len(Body.Detail.Fields) > 0 — the detail has real field rows.
func TestWebRelatedNavigate_SingleTarget_SeedsDetailFromCache(t *testing.T) {
	c, cleanup := startServer(t)
	defer cleanup()

	// Step 1: navigate to the EC2 list.
	c.action(t, app.ActionCommand, "ec2")
	vs := c.state(t)
	if vs.Body.Kind != app.BodyKindList {
		t.Fatalf("step 1 — expected ec2 list, got Body.Kind=%q", vs.Body.Kind)
	}
	if vs.Body.List == nil || len(vs.Body.List.Rows) == 0 {
		t.Fatal("step 1 — ec2 list has no rows; demo fixtures must be loaded")
	}

	// Step 2: open detail for the selected row (index 0 = web-prod-01).
	c.action(t, app.ActionOpenDetail, "")
	vs = c.state(t)
	if vs.Body.Kind != app.BodyKindDetail {
		t.Fatalf("step 2 — expected detail after open-detail, got Body.Kind=%q", vs.Body.Kind)
	}
	if vs.Body.Detail == nil {
		t.Fatal("step 2 — Body.Detail is nil after open-detail")
	}
	if !vs.Body.Detail.RelatedVisible {
		t.Skip("step 2 — ec2 detail: RelatedVisible=false — related panel not available; cannot guard regression")
	}

	// The headless runRelatedCheckers must have populated the related rows via
	// DrainSync so the panel is navigable. Skip rather than fail if the panel
	// is still loading (would indicate a separate runRelatedCheckers regression).
	var targetGroupsRow *app.RelatedBlock
	for i := range vs.Body.Detail.Related {
		row := &vs.Body.Detail.Related[i]
		if row.Loading || row.Err {
			continue
		}
		if row.TargetType == "tg" {
			targetGroupsRow = row
			break
		}
	}
	if targetGroupsRow == nil {
		t.Skip("step 2 — Target Groups related row not found or still loading — " +
			"runRelatedCheckers regression would be a separate bug; skipping this guard")
	}
	if targetGroupsRow.Count != 1 {
		t.Skipf("step 2 — Target Groups count=%d, want 1 — "+
			"demo fixture for web-prod-01 must have exactly one target group (acme-web-tg); "+
			"skipping single-target guard", targetGroupsRow.Count)
	}

	// Move the related cursor to the Target Groups row (index 0 is the first
	// related def; it should already be the cursor default, but move up to be safe).
	// RelatedCursor starts at 0. Find the index of the tg row in the Related slice.
	tgIdx := -1
	for i, row := range vs.Body.Detail.Related {
		if row.TargetType == "tg" {
			tgIdx = i
			break
		}
	}
	// Move cursor down to the tg row (from default position 0).
	for i := 0; i < tgIdx; i++ {
		c.action(t, app.ActionMoveDown, "")
	}

	// Step 3: toggle focus → move focus to the related panel.
	c.action(t, app.ActionToggleFocus, "")
	vs = c.state(t)
	if vs.Body.Detail == nil || !vs.Body.Detail.RelatedFocused {
		t.Fatal("step 3 — related panel not focused after toggle-focus; cannot test navigate")
	}

	// Step 4: select the focused row → navigate into the single-target TG detail.
	c.action(t, app.ActionSelect, "")
	vs = c.state(t)

	// --- Regression assertions ---

	// The stack must have navigated to a detail screen (not stayed on the same
	// detail with related focused, which is the broken pre-fix state).
	if vs.Body.Kind != app.BodyKindDetail {
		t.Fatalf("related-navigate single-target: Body.Kind=%q, want %q — "+
			"NavigationKindDetail cache-hit path must push a detail screen",
			vs.Body.Kind, app.BodyKindDetail)
	}
	if vs.Body.Detail == nil {
		t.Fatal("related-navigate single-target: Body.Detail is nil — detail screen must be populated")
	}

	// FrameTitle must be the target group name, NOT the fallback "detail" or "".
	// Pre-fix: applyRelatedNavResult for NavigationKindDetail called ensureDetailState
	// without seeding from cache, so Resource.Name and Resource.ID were both empty,
	// and detailFrameTitleLocked() returned "". The TUI renderer then substituted
	// the ScreenID string "detail" as the frame title.
	name := vs.FrameTitle
	if name == "" || name == "detail" {
		t.Fatalf("related-navigate to single-target landed on empty placeholder "+
			"(FrameTitle=%q, fields=%d) — want the cached target group detail \"acme-web-tg\"",
			name, len(vs.Body.Detail.Fields))
	}
	if name != "acme-web-tg" {
		t.Fatalf("related-navigate to single-target: FrameTitle=%q, want \"acme-web-tg\" — "+
			"the cached target group detail was not seeded correctly",
			name)
	}

	// The detail must have non-empty Fields — the projector must have run on the
	// cached resource, not on a zero-value placeholder.
	n := len(vs.Body.Detail.Fields)
	if n == 0 {
		t.Fatalf("related-navigate to single-target landed on empty placeholder "+
			"(name=%q, fields=%d) — want the cached target group detail",
			name, n)
	}
}
