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

// Cursor returns the current cursor position.
func (s *ScrollState) Cursor() int {
	return s.cursor
}

// Total returns the total item count.
func (s *ScrollState) Total() int {
	return s.total
}

// Up decrements the cursor, flooring at 0.
func (s *ScrollState) Up() {
	if s.cursor > 0 {
		s.cursor--
	}
}

// Down increments the cursor, capped at total-1.
func (s *ScrollState) Down() {
	if s.cursor < s.total-1 {
		s.cursor++
	}
}

// Top sets cursor to 0.
func (s *ScrollState) Top() {
	s.cursor = 0
}

// Bottom sets cursor to max(0, total-1).
func (s *ScrollState) Bottom() {
	s.cursor = max(0, s.total-1)
}

// PageUp moves cursor up by pageSize, flooring at 0.
func (s *ScrollState) PageUp(pageSize int) {
	s.cursor -= pageSize
	if s.cursor < 0 {
		s.cursor = 0
	}
}

// PageDown moves cursor down by pageSize, capped at total-1.
func (s *ScrollState) PageDown(pageSize int) {
	s.cursor += pageSize
	if s.total > 0 && s.cursor >= s.total {
		s.cursor = s.total - 1
	}
	if s.total == 0 {
		s.cursor = 0
	}
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

// SetTotal updates the total count and clamps the cursor.
func (s *ScrollState) SetTotal(n int) {
	if n <= 0 {
		s.total = 0
		s.cursor = 0
		return
	}
	s.total = n
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
	start := s.cursor - half
	if start < 0 {
		start = 0
	}
	end := start + viewHeight
	if end > total {
		end = total
		start = end - viewHeight
		if start < 0 {
			start = 0
		}
	}
	return start, end
}
