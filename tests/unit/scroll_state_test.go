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
	if s.Cursor() != 0 {
		t.Errorf("new ScrollState cursor should be 0, got %d", s.Cursor())
	}
}

func TestScrollState_NewWithTotal(t *testing.T) {
	s := views.NewScrollState(5)
	if s.Total() != 5 {
		t.Errorf("expected total 5, got %d", s.Total())
	}
}

func TestScrollState_NewWithZeroTotal(t *testing.T) {
	s := views.NewScrollState(0)
	if s.Cursor() != 0 {
		t.Errorf("cursor should be 0 with zero total, got %d", s.Cursor())
	}
}

// ---------------------------------------------------------------------------
// Up
// ---------------------------------------------------------------------------

func TestScrollState_Up_DecrementsCursor(t *testing.T) {
	s := views.NewScrollState(5)
	s.SetCursor(3)
	s.Up()
	if s.Cursor() != 2 {
		t.Errorf("after Up from 3, expected 2, got %d", s.Cursor())
	}
}

func TestScrollState_Up_FloorsAtZero(t *testing.T) {
	s := views.NewScrollState(5)
	s.Up()
	if s.Cursor() != 0 {
		t.Errorf("Up at 0 should stay at 0, got %d", s.Cursor())
	}
}

func TestScrollState_Up_FromOne(t *testing.T) {
	s := views.NewScrollState(5)
	s.SetCursor(1)
	s.Up()
	if s.Cursor() != 0 {
		t.Errorf("Up from 1 should be 0, got %d", s.Cursor())
	}
}

// ---------------------------------------------------------------------------
// Down
// ---------------------------------------------------------------------------

func TestScrollState_Down_IncrementsCursor(t *testing.T) {
	s := views.NewScrollState(5)
	s.Down()
	if s.Cursor() != 1 {
		t.Errorf("after Down from 0, expected 1, got %d", s.Cursor())
	}
}

func TestScrollState_Down_CappedAtTotalMinusOne(t *testing.T) {
	s := views.NewScrollState(5)
	s.SetCursor(4) // last item
	s.Down()
	if s.Cursor() != 4 {
		t.Errorf("Down at last item should stay at 4, got %d", s.Cursor())
	}
}

func TestScrollState_Down_ZeroTotal(t *testing.T) {
	s := views.NewScrollState(0)
	s.Down()
	if s.Cursor() != 0 {
		t.Errorf("Down with zero total should stay at 0, got %d", s.Cursor())
	}
}

// ---------------------------------------------------------------------------
// Top
// ---------------------------------------------------------------------------

func TestScrollState_Top_SetsCursorToZero(t *testing.T) {
	s := views.NewScrollState(10)
	s.SetCursor(7)
	s.Top()
	if s.Cursor() != 0 {
		t.Errorf("Top should set cursor to 0, got %d", s.Cursor())
	}
}

func TestScrollState_Top_AlreadyAtZero(t *testing.T) {
	s := views.NewScrollState(5)
	s.Top()
	if s.Cursor() != 0 {
		t.Errorf("Top at 0 should stay 0, got %d", s.Cursor())
	}
}

// ---------------------------------------------------------------------------
// Bottom
// ---------------------------------------------------------------------------

func TestScrollState_Bottom_SetsCursorToLastItem(t *testing.T) {
	s := views.NewScrollState(10)
	s.Bottom()
	if s.Cursor() != 9 {
		t.Errorf("Bottom with total=10 should set cursor to 9, got %d", s.Cursor())
	}
}

func TestScrollState_Bottom_ZeroTotal(t *testing.T) {
	s := views.NewScrollState(0)
	s.Bottom()
	if s.Cursor() != 0 {
		t.Errorf("Bottom with total=0 should stay at 0, got %d", s.Cursor())
	}
}

func TestScrollState_Bottom_SingleItem(t *testing.T) {
	s := views.NewScrollState(1)
	s.Bottom()
	if s.Cursor() != 0 {
		t.Errorf("Bottom with total=1 should set cursor to 0, got %d", s.Cursor())
	}
}

// ---------------------------------------------------------------------------
// PageUp
// ---------------------------------------------------------------------------

func TestScrollState_PageUp_MovesByPageSize(t *testing.T) {
	s := views.NewScrollState(20)
	s.SetCursor(10)
	s.PageUp(5)
	if s.Cursor() != 5 {
		t.Errorf("PageUp(5) from 10 should give 5, got %d", s.Cursor())
	}
}

func TestScrollState_PageUp_FloorsAtZero(t *testing.T) {
	s := views.NewScrollState(20)
	s.SetCursor(3)
	s.PageUp(10)
	if s.Cursor() != 0 {
		t.Errorf("PageUp(10) from 3 should floor at 0, got %d", s.Cursor())
	}
}

func TestScrollState_PageUp_AlreadyAtZero(t *testing.T) {
	s := views.NewScrollState(10)
	s.PageUp(5)
	if s.Cursor() != 0 {
		t.Errorf("PageUp at 0 should stay at 0, got %d", s.Cursor())
	}
}

// ---------------------------------------------------------------------------
// PageDown
// ---------------------------------------------------------------------------

func TestScrollState_PageDown_MovesByPageSize(t *testing.T) {
	s := views.NewScrollState(20)
	s.PageDown(5)
	if s.Cursor() != 5 {
		t.Errorf("PageDown(5) from 0 should give 5, got %d", s.Cursor())
	}
}

func TestScrollState_PageDown_CappedAtTotalMinusOne(t *testing.T) {
	s := views.NewScrollState(10)
	s.SetCursor(7)
	s.PageDown(10)
	if s.Cursor() != 9 {
		t.Errorf("PageDown(10) from 7 with total=10 should cap at 9, got %d", s.Cursor())
	}
}

func TestScrollState_PageDown_AlreadyAtEnd(t *testing.T) {
	s := views.NewScrollState(10)
	s.SetCursor(9)
	s.PageDown(5)
	if s.Cursor() != 9 {
		t.Errorf("PageDown at end should stay at 9, got %d", s.Cursor())
	}
}

func TestScrollState_PageDown_ZeroTotal(t *testing.T) {
	s := views.NewScrollState(0)
	s.PageDown(5)
	if s.Cursor() != 0 {
		t.Errorf("PageDown with total=0 should stay at 0, got %d", s.Cursor())
	}
}

// ---------------------------------------------------------------------------
// SetCursor (with clamping)
// ---------------------------------------------------------------------------

func TestScrollState_SetCursor_Normal(t *testing.T) {
	s := views.NewScrollState(10)
	s.SetCursor(5)
	if s.Cursor() != 5 {
		t.Errorf("SetCursor(5) should set cursor to 5, got %d", s.Cursor())
	}
}

func TestScrollState_SetCursor_ClampsAboveTotal(t *testing.T) {
	s := views.NewScrollState(5)
	s.SetCursor(10)
	if s.Cursor() != 4 {
		t.Errorf("SetCursor(10) with total=5 should clamp to 4, got %d", s.Cursor())
	}
}

func TestScrollState_SetCursor_ClampsNegative(t *testing.T) {
	s := views.NewScrollState(5)
	s.SetCursor(-3)
	if s.Cursor() != 0 {
		t.Errorf("SetCursor(-3) should clamp to 0, got %d", s.Cursor())
	}
}

func TestScrollState_SetCursor_ZeroTotal(t *testing.T) {
	s := views.NewScrollState(0)
	s.SetCursor(5)
	if s.Cursor() != 0 {
		t.Errorf("SetCursor(5) with total=0 should stay at 0, got %d", s.Cursor())
	}
}

// ---------------------------------------------------------------------------
// SetTotal (with cursor clamping)
// ---------------------------------------------------------------------------

func TestScrollState_SetTotal_IncreasesTotal(t *testing.T) {
	s := views.NewScrollState(5)
	s.SetCursor(3)
	s.SetTotal(10)
	if s.Total() != 10 {
		t.Errorf("expected total 10, got %d", s.Total())
	}
	if s.Cursor() != 3 {
		t.Errorf("cursor should remain at 3 when total increases, got %d", s.Cursor())
	}
}

func TestScrollState_SetTotal_DecreasesClampsCursor(t *testing.T) {
	s := views.NewScrollState(10)
	s.SetCursor(8)
	s.SetTotal(5)
	if s.Total() != 5 {
		t.Errorf("expected total 5, got %d", s.Total())
	}
	if s.Cursor() != 4 {
		t.Errorf("cursor should clamp to 4 when total decreases to 5, got %d", s.Cursor())
	}
}

func TestScrollState_SetTotal_ToZero(t *testing.T) {
	s := views.NewScrollState(10)
	s.SetCursor(5)
	s.SetTotal(0)
	if s.Cursor() != 0 {
		t.Errorf("cursor should be 0 when total set to 0, got %d", s.Cursor())
	}
}

// ---------------------------------------------------------------------------
// Clamp
// ---------------------------------------------------------------------------

func TestScrollState_Clamp_AlreadyValid(t *testing.T) {
	s := views.NewScrollState(5)
	s.SetCursor(3)
	s.Clamp()
	if s.Cursor() != 3 {
		t.Errorf("Clamp should not change valid cursor, got %d", s.Cursor())
	}
}

// ---------------------------------------------------------------------------
// VisibleWindow: centered-cursor algorithm
// ---------------------------------------------------------------------------

func TestScrollState_VisibleWindow_AllFit(t *testing.T) {
	s := views.NewScrollState(5)
	s.SetCursor(2)
	start, end := s.VisibleWindow(10)
	if start != 0 || end != 5 {
		t.Errorf("when all items fit, expected (0,5), got (%d,%d)", start, end)
	}
}

func TestScrollState_VisibleWindow_CursorCentered(t *testing.T) {
	s := views.NewScrollState(20)
	s.SetCursor(10)
	start, end := s.VisibleWindow(5)
	// half = 2, start = 10-2 = 8, end = 8+5 = 13
	if start != 8 || end != 13 {
		t.Errorf("expected cursor=10 centered in (8,13), got (%d,%d)", start, end)
	}
}

func TestScrollState_VisibleWindow_CursorNearTop(t *testing.T) {
	s := views.NewScrollState(20)
	s.SetCursor(1)
	start, end := s.VisibleWindow(5)
	// half = 2, start = 1-2 = -1 -> clamped to 0, end = 0+5 = 5
	if start != 0 || end != 5 {
		t.Errorf("expected (0,5) when cursor near top, got (%d,%d)", start, end)
	}
}

func TestScrollState_VisibleWindow_CursorNearBottom(t *testing.T) {
	s := views.NewScrollState(20)
	s.SetCursor(19)
	start, end := s.VisibleWindow(5)
	// half = 2, start = 19-2 = 17, end = 17+5 = 22 > 20 -> end=20, start=20-5=15
	if start != 15 || end != 20 {
		t.Errorf("expected (15,20) when cursor near bottom, got (%d,%d)", start, end)
	}
}

func TestScrollState_VisibleWindow_ZeroTotal(t *testing.T) {
	s := views.NewScrollState(0)
	start, end := s.VisibleWindow(5)
	if start != 0 || end != 0 {
		t.Errorf("expected (0,0) with zero total, got (%d,%d)", start, end)
	}
}

func TestScrollState_VisibleWindow_ViewHeightOne(t *testing.T) {
	s := views.NewScrollState(10)
	s.SetCursor(5)
	start, end := s.VisibleWindow(1)
	if end-start != 1 {
		t.Errorf("with viewHeight=1, should show exactly 1 row, got %d", end-start)
	}
	if s.Cursor() < start || s.Cursor() >= end {
		t.Errorf("cursor %d should be within [%d,%d)", s.Cursor(), start, end)
	}
}

func TestScrollState_VisibleWindow_ExactFit(t *testing.T) {
	s := views.NewScrollState(5)
	s.SetCursor(2)
	start, end := s.VisibleWindow(5)
	if start != 0 || end != 5 {
		t.Errorf("when total == viewHeight, expected (0,5), got (%d,%d)", start, end)
	}
}

// ---------------------------------------------------------------------------
// Integration: sequential operations
// ---------------------------------------------------------------------------

func TestScrollState_SequentialMoves(t *testing.T) {
	s := views.NewScrollState(5)
	// Down, Down, Down, Up => cursor should be at 2
	s.Down()
	s.Down()
	s.Down()
	s.Up()
	if s.Cursor() != 2 {
		t.Errorf("after D,D,D,U expected cursor=2, got %d", s.Cursor())
	}
}

func TestScrollState_TopBottomSequence(t *testing.T) {
	s := views.NewScrollState(10)
	s.Bottom()
	if s.Cursor() != 9 {
		t.Errorf("after Bottom, expected 9, got %d", s.Cursor())
	}
	s.Top()
	if s.Cursor() != 0 {
		t.Errorf("after Top, expected 0, got %d", s.Cursor())
	}
	s.Bottom()
	if s.Cursor() != 9 {
		t.Errorf("after second Bottom, expected 9, got %d", s.Cursor())
	}
}

func TestScrollState_PageUpDown_RoundTrip(t *testing.T) {
	s := views.NewScrollState(100)
	s.PageDown(10)
	s.PageDown(10)
	s.PageUp(10)
	if s.Cursor() != 10 {
		t.Errorf("PgDown(10)*2 then PgUp(10) should be 10, got %d", s.Cursor())
	}
}
