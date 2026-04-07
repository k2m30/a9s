package views

import (
	"fmt"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/x/ansi"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
)

// matchPos records the position of a single query match in plain-text content.
type matchPos struct {
	line     int // 0-based line index in plain text
	startCol int // starting byte column in plain line
	endCol   int // ending byte column in plain line
}

// highlightEvent is used internally by highlightLine to track open/close positions.
type highlightEvent struct {
	visCol   int
	matchIdx int
	isOpen   bool
}

// SearchModel provides incremental text search over ANSI-styled content.
// It maintains a query, a list of match positions (computed against stripped
// plain text), and the currently-highlighted match index.
//
// Lifecycle:
//  1. Activate() — enters input mode; keystrokes are captured by Update().
//  2. SetContent(plain) — called with stripped content whenever the view re-renders.
//  3. Apply(styled) — inserts highlight ANSI at match positions; returns (result, matchLine).
//  4. Deactivate() — clears everything, returns to inactive state.
type SearchModel struct {
	query      string
	active     bool
	inputMode  bool // true while user is typing (before Enter/Esc)
	matches    []matchPos
	currentIdx int
	content    string // cached plain text (ANSI-stripped)
}

// highlight styles — created once at package init time.
var (
	searchCurrentStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#1a1b26")).
				Background(lipgloss.Color("#e0af68"))
	searchOtherStyle = lipgloss.NewStyle().
				Underline(true).
				Foreground(lipgloss.Color("#e0af68"))
)

// searchPasteMsg carries clipboard text pasted into a search query.
type searchPasteMsg string

// searchReadClipboard is a tea.Cmd that reads the system clipboard and
// returns a searchPasteMsg with the content, or nil on error.
func searchReadClipboard() tea.Msg {
	str, err := clipboard.ReadAll()
	if err != nil {
		return nil
	}
	return searchPasteMsg(str)
}

// NewSearch returns a zero-value SearchModel ready for use.
func NewSearch() SearchModel {
	return SearchModel{}
}

// SetContent stores the plain-text content and recomputes matches.
// The caller must pass ANSI-stripped text; Apply() receives the styled version.
func (s *SearchModel) SetContent(plain string) {
	s.content = plain
	s.recomputeMatches()
}

// SetQuery updates the search query and recomputes matches.
func (s *SearchModel) SetQuery(q string) {
	s.query = q
	s.recomputeMatches()
}

// Activate sets the model to active/input-mode, ready to receive keystrokes.
func (s *SearchModel) Activate() {
	s.active = true
	s.inputMode = true
}

// Deactivate clears all search state (query, matches, active, inputMode).
func (s *SearchModel) Deactivate() {
	s.query = ""
	s.active = false
	s.inputMode = false
	s.matches = nil
	s.currentIdx = 0
}

// NextMatch advances currentIdx, wrapping around to 0 after the last match.
// No-op when there are no matches.
func (s *SearchModel) NextMatch() {
	if len(s.matches) == 0 {
		return
	}
	s.currentIdx = (s.currentIdx + 1) % len(s.matches)
}

// PrevMatch decrements currentIdx, wrapping from 0 to the last match.
// No-op when there are no matches.
func (s *SearchModel) PrevMatch() {
	if len(s.matches) == 0 {
		return
	}
	s.currentIdx = (s.currentIdx - 1 + len(s.matches)) % len(s.matches)
}

// IsActive returns true when search is active (input or confirmed highlights).
func (s SearchModel) IsActive() bool { return s.active }

// IsInputMode returns true while the user is typing a search query.
func (s SearchModel) IsInputMode() bool { return s.inputMode }

// Query returns the current search query string.
func (s SearchModel) Query() string { return s.query }

// MatchCount returns the total number of matches found.
func (s SearchModel) MatchCount() int { return len(s.matches) }

// CurrentMatch returns the 0-based index of the currently-highlighted match.
func (s SearchModel) CurrentMatch() int { return s.currentIdx }

// MatchInfo returns a human-readable match summary: "N/M matches" (1-based N).
func (s SearchModel) MatchInfo() string {
	total := len(s.matches)
	if total == 0 {
		return "0/0 matches"
	}
	return fmt.Sprintf("%d/%d matches", s.currentIdx+1, total)
}

// Update handles key events in input mode.
//   - Printable chars (Key.Text != ""): appended to query.
//   - Backspace: removes last rune from query.
//   - Enter: exits input mode (keeps highlights).
//   - Esc: calls Deactivate().
func (s SearchModel) Update(msg tea.Msg) (SearchModel, tea.Cmd) {
	// Handle bracketed paste (Cmd+V on macOS via terminal bracketed-paste).
	if pm, ok := msg.(tea.PasteMsg); ok {
		s.query += pm.Content
		s.recomputeMatches()
		return s, nil
	}
	// Handle clipboard read result (ctrl+V async clipboard fetch).
	if pm, ok := msg.(searchPasteMsg); ok {
		s.query += string(pm)
		s.recomputeMatches()
		return s, nil
	}
	kp, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return s, nil
	}
	k := kp.Key()
	switch k.Code {
	case tea.KeyBackspace:
		runes := []rune(s.query)
		if len(runes) > 0 {
			s.query = string(runes[:len(runes)-1])
			s.recomputeMatches()
		}
	case tea.KeyEnter:
		if s.query == "" {
			s.Deactivate()
		} else {
			s.inputMode = false
		}
	case tea.KeyEscape:
		s.Deactivate()
	default:
		if k.Code == 'v' && k.Mod == tea.ModCtrl {
			// ctrl+V: async clipboard read; result arrives as searchPasteMsg.
			return s, searchReadClipboard
		}
		if k.Text != "" {
			s.query += k.Text
			s.recomputeMatches()
		}
	}
	return s, nil
}

// Apply inserts ANSI highlight sequences into styled content at match positions.
// It returns the highlighted content and the 0-based line number of the current
// match (-1 when there are no matches or no active query).
func (s SearchModel) Apply(styled string) (string, int) {
	if s.query == "" || len(s.matches) == 0 {
		return styled, -1
	}

	styledLines := strings.Split(styled, "\n")
	result := make([]string, len(styledLines))

	// Build a per-line index using searchLineEntry directly.
	matchesByLine := make(map[int][]searchLineEntry, len(s.matches))
	for i, m := range s.matches {
		matchesByLine[m.line] = append(matchesByLine[m.line], searchLineEntry{i, m})
	}

	currentMatchLine := s.matches[s.currentIdx].line

	for lineIdx, styledLine := range styledLines {
		lineMs, hasMatches := matchesByLine[lineIdx]
		if !hasMatches {
			result[lineIdx] = styledLine
			continue
		}
		result[lineIdx] = highlightLine(styledLine, lineMs, s.currentIdx)
	}

	return strings.Join(result, "\n"), currentMatchLine
}

// searchLineEntry carries a match index and its position for a single line.
type searchLineEntry struct {
	matchIdx int
	mp       matchPos
}

// highlightLine inserts ANSI highlight sequences into a single styled line.
// Positions are expressed in plain-text byte offsets.
func highlightLine(styledLine string, entries []searchLineEntry, currentIdx int) string {
	plainLine := ansi.Strip(styledLine)
	plainRunes := []rune(plainLine)

	// Build sorted event list (open/close per match).
	events := make([]highlightEvent, 0, len(entries)*2)
	for _, e := range entries {
		startRune := byteOffsetToRuneOffset(plainLine, e.mp.startCol)
		endRune := byteOffsetToRuneOffset(plainLine, e.mp.endCol)
		events = append(events,
			highlightEvent{startRune, e.matchIdx, true},
			highlightEvent{endRune, e.matchIdx, false},
		)
	}
	sortHighlightEvents(events)

	var out strings.Builder
	out.Grow(len(styledLine) + len(entries)*40)

	visPos := 0  // current visible rune column
	bytePos := 0 // current byte position in styledLine
	eIdx := 0    // next event to process

	for {
		// Fire events at the current visible position.
		for eIdx < len(events) && events[eIdx].visCol == visPos {
			ev := events[eIdx]
			if ev.isOpen {
				// Find the matching close to know the span length.
				endRune := visPos
				for k := eIdx + 1; k < len(events); k++ {
					if events[k].matchIdx == ev.matchIdx && !events[k].isOpen {
						endRune = events[k].visCol
						break
					}
				}
				if endRune > visPos && endRune <= len(plainRunes) {
					matchText := string(plainRunes[visPos:endRune])
					var rendered string
					if ev.matchIdx == currentIdx {
						rendered = searchCurrentStyle.Render(matchText)
					} else {
						rendered = searchOtherStyle.Render(matchText)
					}
					out.WriteString(rendered)
					// Skip past the visible characters in the styled bytes.
					bytePos = advanceStyledBytes(styledLine, bytePos, endRune-visPos)
					visPos = endRune
					// Advance eIdx past the close event for this match.
					eIdx = skipCloseEvent(events, eIdx)
					continue
				}
			}
			eIdx++
		}

		if bytePos >= len(styledLine) {
			break
		}

		// Emit next character from styled line.
		if styledLine[bytePos] == '\x1b' && bytePos+1 < len(styledLine) && styledLine[bytePos+1] == '[' {
			// ANSI CSI escape: copy verbatim, no visible advance.
			end := bytePos + 2
			for end < len(styledLine) {
				b := styledLine[end]
				end++
				if (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') {
					break
				}
			}
			out.WriteString(styledLine[bytePos:end])
			bytePos = end
		} else {
			r, size := decodeRune(styledLine[bytePos:])
			out.WriteRune(r)
			bytePos += size
			visPos++
		}
	}

	return out.String()
}

// byteOffsetToRuneOffset converts a byte offset in s to a rune count.
func byteOffsetToRuneOffset(s string, byteOff int) int {
	if byteOff >= len(s) {
		return len([]rune(s))
	}
	return len([]rune(s[:byteOff]))
}

// advanceStyledBytes advances bytePos in styledLine past n visible rune positions,
// skipping ANSI escapes transparently.
func advanceStyledBytes(styledLine string, bytePos, count int) int {
	consumed := 0
	pos := bytePos
	for consumed < count && pos < len(styledLine) {
		if styledLine[pos] == '\x1b' && pos+1 < len(styledLine) && styledLine[pos+1] == '[' {
			pos += 2
			for pos < len(styledLine) {
				b := styledLine[pos]
				pos++
				if (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') {
					break
				}
			}
		} else {
			_, size := decodeRune(styledLine[pos:])
			pos += size
			consumed++
		}
	}
	return pos
}

// skipCloseEvent returns the index after the close event for the match opened at events[openIdx].
func skipCloseEvent(events []highlightEvent, openIdx int) int {
	targetIdx := events[openIdx].matchIdx
	for i := openIdx + 1; i < len(events); i++ {
		if events[i].matchIdx == targetIdx && !events[i].isOpen {
			return i + 1
		}
	}
	return openIdx + 1
}

// sortHighlightEvents sorts events by visCol; at same col, closes before opens.
func sortHighlightEvents(events []highlightEvent) {
	// Insertion sort — event counts are tiny (single-digit per line typically).
	for i := 1; i < len(events); i++ {
		for j := i; j > 0; j-- {
			a, b := events[j-1], events[j]
			// a < b if a.visCol > b.visCol (swap), or same col and a is open but b is close.
			if a.visCol > b.visCol || (a.visCol == b.visCol && a.isOpen && !b.isOpen) {
				events[j-1], events[j] = events[j], events[j-1]
			} else {
				break
			}
		}
	}
}

// decodeRune decodes the first UTF-8 rune from s.
// Falls back to a single byte on invalid input.
func decodeRune(s string) (rune, int) {
	if len(s) == 0 {
		return 0, 0
	}
	b := s[0]
	if b < 0x80 {
		return rune(b), 1
	}
	switch {
	case b < 0xE0:
		if len(s) >= 2 {
			return rune(b&0x1F)<<6 | rune(s[1]&0x3F), 2
		}
	case b < 0xF0:
		if len(s) >= 3 {
			return rune(b&0x0F)<<12 | rune(s[1]&0x3F)<<6 | rune(s[2]&0x3F), 3
		}
	default:
		if len(s) >= 4 {
			return rune(b&0x07)<<18 | rune(s[1]&0x3F)<<12 | rune(s[2]&0x3F)<<6 | rune(s[3]&0x3F), 4
		}
	}
	return rune(b), 1
}

// recomputeMatches rebuilds the matches slice from the current query and content.
func (s *SearchModel) recomputeMatches() {
	s.matches = nil
	prevIdx := s.currentIdx
	if s.query == "" || s.content == "" {
		s.currentIdx = 0
		return
	}
	q := strings.ToLower(s.query)
	lines := strings.Split(s.content, "\n")
	for lineIdx, line := range lines {
		lower := strings.ToLower(line)
		start := 0
		for {
			idx := strings.Index(lower[start:], q)
			if idx < 0 {
				break
			}
			s.matches = append(s.matches, matchPos{
				line:     lineIdx,
				startCol: start + idx,
				endCol:   start + idx + len(q),
			})
			start += idx + len(q)
		}
	}
	// Preserve the match index across recomputation (e.g. after NextMatch
	// triggers refreshViewportContent which calls SetContent → recomputeMatches).
	switch {
	case len(s.matches) == 0:
		s.currentIdx = 0
	case prevIdx < len(s.matches):
		s.currentIdx = prevIdx
	default:
		s.currentIdx = len(s.matches) - 1
	}
}
