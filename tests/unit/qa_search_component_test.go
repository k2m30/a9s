package unit

// SearchModel (internal/tui/views/search.go):
// match discovery, highlighting, navigation and match info.

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// TestSearch_SetContentAndQuery_FindsMatches verifies that after setting plain
// text content and a query, MatchCount() reflects the correct number of hits.
func TestSearch_SetContentAndQuery_FindsMatches(t *testing.T) {
	s := views.SearchModel{}
	s.SetContent("line one\nline two\nline one again")
	s.SetQuery("one")
	if s.MatchCount() != 2 {
		t.Errorf("expected 2 matches for 'one', got %d", s.MatchCount())
	}
}

// TestSearch_Apply_ReturnsHighlightedContent verifies that Apply() inserts
// additional ANSI escape sequences around matched text and returns the line
// number (0-based) of the current match.
func TestSearch_Apply_ReturnsHighlightedContent(t *testing.T) {
	plain := "line one\nline two\nline one again"
	styled := plain // no existing ANSI in this simple case
	s := views.SearchModel{}
	s.SetContent(plain)
	s.SetQuery("one")

	highlighted, matchLine := s.Apply(styled)

	originalAnsiCount := strings.Count(styled, "\x1b[")
	highlightedAnsiCount := strings.Count(highlighted, "\x1b[")
	if highlightedAnsiCount <= originalAnsiCount {
		t.Errorf("Apply() did not add ANSI highlight sequences; before=%d after=%d",
			originalAnsiCount, highlightedAnsiCount)
	}

	lineCount := strings.Count(plain, "\n") + 1
	if matchLine < 0 || matchLine >= lineCount {
		t.Errorf("Apply() returned out-of-range matchLine=%d (content has %d lines)",
			matchLine, lineCount)
	}

	plain2 := ansiRe.ReplaceAllString(highlighted, "")
	if !strings.Contains(plain2, "line one") {
		t.Errorf("Apply() stripped visible content; got: %q", plain2)
	}
}

// TestSearch_ANSIContent_MatchesVisibleTextOnly verifies that SetContent takes
// plain text for indexing while Apply operates on a separately ANSI-styled
// version of the same content. Highlighting must not corrupt existing sequences.
func TestSearch_ANSIContent_MatchesVisibleTextOnly(t *testing.T) {
	plain := "hello world"
	styled := "\x1b[31mhello\x1b[0m world"

	s := views.SearchModel{}
	s.SetContent(plain)
	s.SetQuery("hello")

	if s.MatchCount() != 1 {
		t.Errorf("expected 1 match, got %d", s.MatchCount())
	}

	highlighted, _ := s.Apply(styled)

	if !strings.Contains(highlighted, "\x1b[0m") {
		t.Errorf("Apply() removed existing ANSI reset sequence; result: %q", highlighted)
	}

	plain2 := ansiRe.ReplaceAllString(highlighted, "")
	if !strings.Contains(plain2, "hello world") {
		t.Errorf("Apply() corrupted visible content; got: %q", plain2)
	}
}

// TestSearch_NextMatch_CyclesThroughMatches verifies that calling NextMatch()
// advances CurrentMatch() and wraps around after the last match.
func TestSearch_NextMatch_CyclesThroughMatches(t *testing.T) {
	s := views.SearchModel{}
	s.SetContent("test alpha\ntest beta\ntest gamma")
	s.SetQuery("test")

	if s.MatchCount() != 3 {
		t.Fatalf("expected 3 matches, got %d", s.MatchCount())
	}

	if s.CurrentMatch() != 0 {
		t.Errorf("expected initial CurrentMatch()=0, got %d", s.CurrentMatch())
	}

	s.NextMatch()
	if s.CurrentMatch() != 1 {
		t.Errorf("after 1st NextMatch(): expected 1, got %d", s.CurrentMatch())
	}

	s.NextMatch()
	if s.CurrentMatch() != 2 {
		t.Errorf("after 2nd NextMatch(): expected 2, got %d", s.CurrentMatch())
	}

	s.NextMatch()
	if s.CurrentMatch() != 0 {
		t.Errorf("after wrap NextMatch(): expected 0, got %d", s.CurrentMatch())
	}
}

// TestSearch_PrevMatch_CyclesBackward verifies that PrevMatch() decrements
// CurrentMatch() and wraps from 0 to the last match.
func TestSearch_PrevMatch_CyclesBackward(t *testing.T) {
	s := views.SearchModel{}
	s.SetContent("test alpha\ntest beta\ntest gamma")
	s.SetQuery("test")

	if s.MatchCount() != 3 {
		t.Fatalf("expected 3 matches, got %d", s.MatchCount())
	}

	s.PrevMatch()
	if s.CurrentMatch() != 2 {
		t.Errorf("after PrevMatch() from 0: expected 2, got %d", s.CurrentMatch())
	}

	s.PrevMatch()
	if s.CurrentMatch() != 1 {
		t.Errorf("after 2nd PrevMatch(): expected 1, got %d", s.CurrentMatch())
	}

	s.PrevMatch()
	if s.CurrentMatch() != 0 {
		t.Errorf("after 3rd PrevMatch(): expected 0, got %d", s.CurrentMatch())
	}
}

// TestSearch_ZeroMatches_MatchInfoShowsZero verifies that when no matches exist,
// MatchCount() == 0, MatchInfo() == "0/0 matches", and NextMatch()/PrevMatch()
// are no-ops (CurrentMatch() stays 0).
func TestSearch_ZeroMatches_MatchInfoShowsZero(t *testing.T) {
	s := views.SearchModel{}
	s.SetContent("hello")
	s.SetQuery("xyz")

	if s.MatchCount() != 0 {
		t.Errorf("expected 0 matches, got %d", s.MatchCount())
	}
	if s.MatchInfo() != "0/0 matches" {
		t.Errorf("expected MatchInfo()='0/0 matches', got %q", s.MatchInfo())
	}

	s.NextMatch()
	if s.CurrentMatch() != 0 {
		t.Errorf("NextMatch() on zero matches changed CurrentMatch() to %d", s.CurrentMatch())
	}
	s.PrevMatch()
	if s.CurrentMatch() != 0 {
		t.Errorf("PrevMatch() on zero matches changed CurrentMatch() to %d", s.CurrentMatch())
	}
}

// TestSearch_EmptyQuery_NoMatches verifies that an empty query results in zero
// matches and Apply() returns the styled content unchanged.
func TestSearch_EmptyQuery_NoMatches(t *testing.T) {
	plain := "hello world"
	s := views.SearchModel{}
	s.SetContent(plain)
	s.SetQuery("")

	if s.MatchCount() != 0 {
		t.Errorf("empty query: expected 0 matches, got %d", s.MatchCount())
	}

	highlighted, _ := s.Apply(plain)
	if highlighted != plain {
		t.Errorf("empty query: Apply() should return content unchanged; got %q", highlighted)
	}
}

// TestSearch_CaseInsensitive verifies that matching is case-insensitive.
func TestSearch_CaseInsensitive(t *testing.T) {
	s := views.SearchModel{}
	s.SetContent("Hello HELLO hello")
	s.SetQuery("hello")

	if s.MatchCount() != 3 {
		t.Errorf("case-insensitive: expected 3 matches for 'hello', got %d", s.MatchCount())
	}
}

// TestSearch_Activate_Deactivate verifies IsActive(), IsInputMode(), and that
// Deactivate() clears the query.
func TestSearch_Activate_Deactivate(t *testing.T) {
	s := views.SearchModel{}

	if s.IsActive() {
		t.Error("expected IsActive()=false before Activate()")
	}

	s.Activate()
	if !s.IsActive() {
		t.Error("expected IsActive()=true after Activate()")
	}
	if !s.IsInputMode() {
		t.Error("expected IsInputMode()=true immediately after Activate()")
	}

	s.SetQuery("something")
	s.Deactivate()

	if s.IsActive() {
		t.Error("expected IsActive()=false after Deactivate()")
	}
	if s.Query() != "" {
		t.Errorf("expected Query()='' after Deactivate(), got %q", s.Query())
	}
}

// TestSearch_MatchInfo_Format verifies the MatchInfo() format is "N/M matches"
// where N is the 1-based current match number and M is the total count.
func TestSearch_MatchInfo_Format(t *testing.T) {
	s := views.SearchModel{}
	s.SetContent("foo alpha\nfoo beta\nfoo gamma\nfoo delta\nfoo epsilon")
	s.SetQuery("foo")

	if s.MatchCount() != 5 {
		t.Fatalf("expected 5 matches, got %d", s.MatchCount())
	}

	s.NextMatch() // index 1
	s.NextMatch() // index 2

	want := "3/5 matches"
	if s.MatchInfo() != want {
		t.Errorf("MatchInfo(): expected %q, got %q", want, s.MatchInfo())
	}
}
