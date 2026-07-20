// SPDX-License-Identifier: GPL-3.0-or-later

package views

// ScrollState manages cursor position and scroll window calculation
// for list-style views. It encapsulates the duplicated cursor/scroll
// logic found in ResourceListModel and MainMenuModel.
type ScrollState struct {
	cursor int
	total  int
}

// NewScrollState creates a ScrollState with the given total item count.
func NewScrollState(total int) ScrollState {
	return ScrollState{total: total}
}

// CursorForTest returns the current cursor position. Test-only: no
// production caller — production reads the cursor value it set itself
// (e.g. body.Selected) rather than reading it back off ScrollState, which is
// used in production only as a stateless VisibleWindow calculator.
func (s *ScrollState) CursorForTest() int {
	return s.cursor
}

// TotalForTest returns the total item count. Test-only: no production
// caller — see CursorForTest's doc comment.
func (s *ScrollState) TotalForTest() int {
	return s.total
}

// SetCursor sets the cursor to n, clamping to [0, total-1].
func (s *ScrollState) SetCursor(n int) {
	if s.total == 0 {
		s.cursor = 0
		return
	}
	s.cursor = n
	s.Clamp()
}

// Clamp ensures the cursor is within [0, total-1].
func (s *ScrollState) Clamp() {
	if s.total <= 0 {
		s.cursor = 0
		return
	}
	if s.cursor < 0 {
		s.cursor = 0
	}
	if s.cursor >= s.total {
		s.cursor = s.total - 1
	}
}

// VisibleWindow calculates the start and end row indices using
// a centered-cursor algorithm. Returns (start, end) where start
// is inclusive and end is exclusive.
func (s *ScrollState) VisibleWindow(viewHeight int) (int, int) {
	total := s.total
	if total <= viewHeight {
		return 0, total
	}

	half := viewHeight / 2
	start := max(s.cursor-half, 0)
	end := start + viewHeight
	if end > total {
		end = total
		start = max(end-viewHeight, 0)
	}
	return start, end
}
