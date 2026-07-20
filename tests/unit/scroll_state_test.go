// scroll_state_test.go — live ScrollState coverage.
//
// Up/Down/Top/Bottom/PageUp/PageDown/SetTotal/Clamp are DEAD per
// specs/022-codebase-cleanup/wave3-map-text.md (scroll.go: "LIVE:
// NewScrollState, SetCursor, VisibleWindow (via RenderSelector);
// DEAD: Cursor, Total, Up, Down, Top, Bottom, PageUp, PageDown, SetTotal,
// Clamp") — cursor movement is controller-owned (app.SelectorState /
// app.DetailState), not driven through these ScrollState methods anymore.
// VisibleWindow's centered-window cases moved to text_ports_test.go
// (TestWave3Port_ScrollState_VisibleWindow_*) ahead of this trim, so they
// survive here without duplication.
package unit

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ---------------------------------------------------------------------------
// ScrollState: basic construction and getters
// ---------------------------------------------------------------------------

func TestScrollState_NewHasZeroCursor(t *testing.T) {
	s := views.NewScrollState(10)
	if s.CursorForTest() != 0 {
		t.Errorf("new ScrollState cursor should be 0, got %d", s.CursorForTest())
	}
}

func TestScrollState_NewWithTotal(t *testing.T) {
	s := views.NewScrollState(5)
	if s.TotalForTest() != 5 {
		t.Errorf("expected total 5, got %d", s.TotalForTest())
	}
}

func TestScrollState_NewWithZeroTotal(t *testing.T) {
	s := views.NewScrollState(0)
	if s.CursorForTest() != 0 {
		t.Errorf("cursor should be 0 with zero total, got %d", s.CursorForTest())
	}
}

// ---------------------------------------------------------------------------
// SetCursor (with clamping) — LIVE, used by RenderSelector via NewScrollState.
// ---------------------------------------------------------------------------

func TestScrollState_SetCursor_Normal(t *testing.T) {
	s := views.NewScrollState(10)
	s.SetCursor(5)
	if s.CursorForTest() != 5 {
		t.Errorf("SetCursor(5) should set cursor to 5, got %d", s.CursorForTest())
	}
}

func TestScrollState_SetCursor_ClampsAboveTotal(t *testing.T) {
	s := views.NewScrollState(5)
	s.SetCursor(10)
	if s.CursorForTest() != 4 {
		t.Errorf("SetCursor(10) with total=5 should clamp to 4, got %d", s.CursorForTest())
	}
}

func TestScrollState_SetCursor_ClampsNegative(t *testing.T) {
	s := views.NewScrollState(5)
	s.SetCursor(-3)
	if s.CursorForTest() != 0 {
		t.Errorf("SetCursor(-3) should clamp to 0, got %d", s.CursorForTest())
	}
}

func TestScrollState_SetCursor_ZeroTotal(t *testing.T) {
	s := views.NewScrollState(0)
	s.SetCursor(5)
	if s.CursorForTest() != 0 {
		t.Errorf("SetCursor(5) with total=0 should stay at 0, got %d", s.CursorForTest())
	}
}
