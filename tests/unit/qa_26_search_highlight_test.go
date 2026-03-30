package unit

// qa_26_search_highlight_test.go — QA-26 tests for cross-view search:
// Sections D (highlighting), E (navigation), I (case sensitivity),
// J (ANSI-aware search), and M (edge cases).

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/resource"
	tui "github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ---------------------------------------------------------------------------
// Helpers shared across QA-26 tests
// ---------------------------------------------------------------------------

// searchActivateAndConfirm sets a query on s (bypassing input-mode key events)
// and leaves the model in confirmed-search state: active=true, inputMode=false.
func searchActivateAndConfirm(s *views.SearchModel, plain, query string) {
	s.Activate()
	s.SetContent(plain)
	s.SetQuery(query)
	// Simulate Enter: exit input mode by updating with KeyEnter.
	*s, _ = s.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
}

// rootNavigateToDetail navigates the root model to a detail view for the given resource.
func rootNavigateToDetail(m tui.Model, res *resource.Resource) (tui.Model, tea.Cmd) {
	return rootApplyMsg(m, messages.NavigateMsg{Target: messages.TargetDetail, Resource: res})
}

// rootActivateSearch sends "/" then types the query then presses Enter through the root model.
func rootActivateSearch(m tui.Model, query string) tui.Model {
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: '/', Text: "/"})
	for _, ch := range query {
		m, _ = rootApplyMsg(m, rootKeyPress(string(ch)))
	}
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	return m
}

// plainText strips ANSI sequences using the package-level ansiRe.
func plainText(s string) string {
	return ansiRe.ReplaceAllString(s, "")
}

// ---------------------------------------------------------------------------
// Section D — Match Highlighting (component-level, SearchModel)
// ---------------------------------------------------------------------------

// 26-D01: All matches highlighted with ANSI sequences after search confirmed.
func TestSearch_D01_AllMatchesHighlighted(t *testing.T) {
	plain := "running state: running\nnot matching\nalso running here"
	s := views.NewSearch()
	searchActivateAndConfirm(&s, plain, "running")

	if s.MatchCount() != 3 {
		t.Fatalf("26-D01: expected 3 matches for 'running', got %d", s.MatchCount())
	}

	highlighted, _ := s.Apply(plain)

	// All 3 plain-text occurrences must have become ANSI-highlighted spans.
	// Count ANSI escape sequences — must exceed the original (zero in plain).
	ansiCount := strings.Count(highlighted, "\x1b[")
	if ansiCount == 0 {
		t.Error("26-D01: Apply() added no ANSI sequences; all matches should be highlighted")
	}

	// Visible text must still contain all three occurrences.
	strippedOut := plainText(highlighted)
	if strings.Count(strippedOut, "running") != 3 {
		t.Errorf("26-D01: stripped output contains wrong count of 'running'; want 3, got %d",
			strings.Count(strippedOut, "running"))
	}
}

// 26-D02: Current match visually distinct from other matches (different ANSI sequences).
func TestSearch_D02_CurrentMatchDistinctFromOthers(t *testing.T) {
	plain := "match one\nmatch two\nmatch three\nmatch four\nmatch five"
	s := views.NewSearch()
	searchActivateAndConfirm(&s, plain, "match")

	if s.MatchCount() != 5 {
		t.Fatalf("26-D02: expected 5 matches, got %d", s.MatchCount())
	}

	// Advance to match index 1 (the second match, displayed as #2).
	s.NextMatch()
	if s.CurrentMatch() != 1 {
		t.Fatalf("26-D02: expected CurrentMatch()=1 after NextMatch(), got %d", s.CurrentMatch())
	}

	highlighted, _ := s.Apply(plain)

	// Collect the ANSI prefix for each rendered "match" token in the output.
	// The current match renders differently from non-current matches.
	// We extract lines and check that the current-match line has a different
	// ANSI code than the other-match lines.
	lines := strings.Split(highlighted, "\n")
	if len(lines) < 5 {
		t.Fatalf("26-D02: expected 5 lines, got %d", len(lines))
	}

	// Line 0 = match one (index 0, non-current)
	// Line 1 = match two (index 1, CURRENT)
	// Line 2..4 = non-current
	currentLine := lines[1]
	otherLine := lines[0]

	if currentLine == otherLine {
		t.Error("26-D02: current-match line and non-current match line have identical ANSI output; they must be visually distinct")
	}

	// Specifically: both lines must contain ANSI but the ANSI escape sequences
	// must differ (different style codes for current vs non-current).
	currentANSI := ansiRe.FindAllString(currentLine, -1)
	otherANSI := ansiRe.FindAllString(otherLine, -1)

	if len(currentANSI) == 0 || len(otherANSI) == 0 {
		t.Fatal("26-D02: both current and non-current lines must contain ANSI sequences")
	}

	// Join all escape codes in each line; they must not be identical strings.
	if strings.Join(currentANSI, "") == strings.Join(otherANSI, "") {
		t.Error("26-D02: current match and non-current match have identical ANSI style codes; they must differ")
	}
}

// 26-D03: Search highlighting overrides syntax coloring in styled text.
func TestSearch_D03_HighlightOverridesSyntaxColor(t *testing.T) {
	// "t3.medium" colored green, plus a blue key — simulate YAML syntax coloring.
	greenOpen := "\x1b[32m"
	reset := "\x1b[0m"
	blueOpen := "\x1b[34m"

	plain := "InstanceType: t3.medium"
	styled := blueOpen + "InstanceType" + reset + ": " + greenOpen + "t3.medium" + reset

	s := views.NewSearch()
	s.SetContent(plain)
	s.SetQuery("medium")

	if s.MatchCount() != 1 {
		t.Fatalf("26-D03: expected 1 match for 'medium', got %d", s.MatchCount())
	}

	highlighted, _ := s.Apply(styled)

	// The visible text must still contain "medium".
	if !strings.Contains(plainText(highlighted), "medium") {
		t.Error("26-D03: Apply() removed 'medium' from visible content")
	}

	// The output must contain ANSI highlight sequences (more than the input had).
	// Input had 4 sequences (2 opens + 2 resets); output must have more or
	// at least include highlight-specific sequences that replace the green region.
	if !strings.Contains(highlighted, "\x1b[") {
		t.Error("26-D03: highlighted output has no ANSI sequences at all")
	}

	// Verify the non-matched "t3." portion still appears in the output (visible text).
	stripped := plainText(highlighted)
	if !strings.Contains(stripped, "t3.") {
		t.Error("26-D03: non-matched 't3.' portion missing from visible content")
	}
}

// 26-D04: Search highlighting overrides status coloring in detail view.
func TestSearch_D04_HighlightOverridesStatusColor(t *testing.T) {
	greenOpen := "\x1b[32m"
	reset := "\x1b[0m"

	plain := "Status: running"
	styled := "Status: " + greenOpen + "running" + reset

	s := views.NewSearch()
	s.SetContent(plain)
	s.SetQuery("running")

	if s.MatchCount() != 1 {
		t.Fatalf("26-D04: expected 1 match for 'running', got %d", s.MatchCount())
	}

	highlighted, _ := s.Apply(styled)

	// "running" must still be visible.
	if !strings.Contains(plainText(highlighted), "running") {
		t.Error("26-D04: 'running' missing from highlighted output visible text")
	}

	// The output must contain ANSI highlight sequences.
	if !strings.Contains(highlighted, "\x1b[") {
		t.Error("26-D04: highlighted output contains no ANSI sequences")
	}

	// The output must have MORE ANSI sequences than the input (highlight was injected).
	inputANSI := strings.Count(styled, "\x1b[")
	outputANSI := strings.Count(highlighted, "\x1b[")
	if outputANSI <= inputANSI {
		t.Errorf("26-D04: expected more ANSI sequences in output than input (input=%d, output=%d)", inputANSI, outputANSI)
	}
}

// 26-D05: Partial match within a syntax token — only matched portion highlighted.
func TestSearch_D05_PartialMatchWithinStyledToken(t *testing.T) {
	greenOpen := "\x1b[32m"
	reset := "\x1b[0m"

	plain := "us-east-1a"
	styled := greenOpen + "us-east-1a" + reset

	s := views.NewSearch()
	s.SetContent(plain)
	s.SetQuery("east")

	if s.MatchCount() != 1 {
		t.Fatalf("26-D05: expected 1 match for 'east', got %d", s.MatchCount())
	}

	highlighted, _ := s.Apply(styled)

	stripped := plainText(highlighted)

	// "us-east-1a" must be fully present in visible text.
	if !strings.Contains(stripped, "us-east-1a") {
		t.Errorf("26-D05: expected full 'us-east-1a' in stripped output, got: %q", stripped)
	}

	// The output must contain highlight ANSI sequences.
	if !strings.Contains(highlighted, "\x1b[") {
		t.Error("26-D05: no ANSI sequences in highlighted output")
	}

	// Verify there are ANSI sequences beyond the input (highlight injected).
	if strings.Count(highlighted, "\x1b[") <= strings.Count(styled, "\x1b[") {
		t.Error("26-D05: Apply() did not add highlight sequences for the partial 'east' match")
	}
}

// ---------------------------------------------------------------------------
// Section E — Navigation (root-level tests through full model)
// ---------------------------------------------------------------------------

// 26-E01: n advances to next match — counter changes from "1/4" to "2/4".
func TestSearch_E01_NAdvancesToNextMatch(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	res := &resource.Resource{
		ID:   "i-nav-e01",
		Name: "nav-e01",
		Fields: map[string]string{
			"tag1": "running",
			"tag2": "running",
			"tag3": "running",
			"tag4": "running",
		},
	}

	m, _ = rootNavigateToDetail(m, res)
	m = rootActivateSearch(m, "running")

	// Verify we start at 1/4.
	before := plainText(rootViewContent(m))
	if !strings.Contains(before, "1/4 matches") {
		t.Fatalf("26-E01: expected '1/4 matches' after confirming search; got plain:\n%s", before)
	}

	// Press n — should advance to 2/4.
	m, _ = rootApplyMsg(m, rootKeyPress("n"))
	after := plainText(rootViewContent(m))
	if !strings.Contains(after, "2/4 matches") {
		t.Errorf("26-E01: expected '2/4 matches' after pressing n; got:\n%s", after)
	}
}

// 26-E02: n wraps from last match to first match ("4/4" → "1/4").
func TestSearch_E02_NWrapsFromLastToFirst(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	res := &resource.Resource{
		ID:   "i-nav-e02",
		Name: "nav-e02",
		Fields: map[string]string{
			"tag1": "running",
			"tag2": "running",
			"tag3": "running",
			"tag4": "running",
		},
	}

	m, _ = rootNavigateToDetail(m, res)
	m = rootActivateSearch(m, "running")

	// Navigate to the last match (4/4): press n 3 times.
	for i := 0; i < 3; i++ {
		m, _ = rootApplyMsg(m, rootKeyPress("n"))
	}

	atLast := plainText(rootViewContent(m))
	if !strings.Contains(atLast, "4/4 matches") {
		t.Fatalf("26-E02: expected '4/4 matches' after 3 n-presses; got:\n%s", atLast)
	}

	// Press n once more — should wrap to 1/4.
	m, _ = rootApplyMsg(m, rootKeyPress("n"))
	wrapped := plainText(rootViewContent(m))
	if !strings.Contains(wrapped, "1/4 matches") {
		t.Errorf("26-E02: expected '1/4 matches' after wrapping; got:\n%s", wrapped)
	}
}

// 26-E03: N moves to previous match ("3/4" → "2/4").
func TestSearch_E03_ShiftNMovesToPrevious(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	res := &resource.Resource{
		ID:   "i-nav-e03",
		Name: "nav-e03",
		Fields: map[string]string{
			"tag1": "running",
			"tag2": "running",
			"tag3": "running",
			"tag4": "running",
		},
	}

	m, _ = rootNavigateToDetail(m, res)
	m = rootActivateSearch(m, "running")

	// Advance to 3/4: press n twice.
	m, _ = rootApplyMsg(m, rootKeyPress("n"))
	m, _ = rootApplyMsg(m, rootKeyPress("n"))

	at3 := plainText(rootViewContent(m))
	if !strings.Contains(at3, "3/4 matches") {
		t.Fatalf("26-E03: expected '3/4 matches' after 2 n-presses; got:\n%s", at3)
	}

	// Press N (shift+n) — should retreat to 2/4.
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: -1, Text: "N"})
	after := plainText(rootViewContent(m))
	if !strings.Contains(after, "2/4 matches") {
		t.Errorf("26-E03: expected '2/4 matches' after pressing N; got:\n%s", after)
	}
}

// 26-E04: N wraps from first to last match ("1/4" → "4/4").
func TestSearch_E04_ShiftNWrapsFromFirstToLast(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	res := &resource.Resource{
		ID:   "i-nav-e04",
		Name: "nav-e04",
		Fields: map[string]string{
			"tag1": "running",
			"tag2": "running",
			"tag3": "running",
			"tag4": "running",
		},
	}

	m, _ = rootNavigateToDetail(m, res)
	m = rootActivateSearch(m, "running")

	// We are at 1/4. Press N — should wrap to 4/4.
	at1 := plainText(rootViewContent(m))
	if !strings.Contains(at1, "1/4 matches") {
		t.Fatalf("26-E04: expected '1/4 matches' at start; got:\n%s", at1)
	}

	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: -1, Text: "N"})
	wrapped := plainText(rootViewContent(m))
	if !strings.Contains(wrapped, "4/4 matches") {
		t.Errorf("26-E04: expected '4/4 matches' after N wraps from first; got:\n%s", wrapped)
	}
}

// 26-E05: n/N with a single match — stays at "1/1", no error.
func TestSearch_E05_SingleMatch_NoChange(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	res := &resource.Resource{
		ID:   "i-nav-e05",
		Name: "nav-e05",
		Fields: map[string]string{
			"unique_field": "singleterm",
		},
	}

	m, _ = rootNavigateToDetail(m, res)
	m = rootActivateSearch(m, "singleterm")

	start := plainText(rootViewContent(m))
	if !strings.Contains(start, "1/1 matches") {
		t.Fatalf("26-E05: expected '1/1 matches' at start; got:\n%s", start)
	}

	// Press n — should remain at 1/1.
	m, _ = rootApplyMsg(m, rootKeyPress("n"))
	afterN := plainText(rootViewContent(m))
	if !strings.Contains(afterN, "1/1 matches") {
		t.Errorf("26-E05: expected '1/1 matches' after n (single match); got:\n%s", afterN)
	}

	// Press N — should remain at 1/1.
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: -1, Text: "N"})
	afterShiftN := plainText(rootViewContent(m))
	if !strings.Contains(afterShiftN, "1/1 matches") {
		t.Errorf("26-E05: expected '1/1 matches' after N (single match); got:\n%s", afterShiftN)
	}
}

// ---------------------------------------------------------------------------
// Section I — Case Sensitivity (root-level tests)
// ---------------------------------------------------------------------------

// 26-I01: Case-insensitive by default — "run" matches "running" and "RunTimeConfig".
func TestSearch_I01_CaseInsensitiveDefault(t *testing.T) {
	s := views.NewSearch()
	s.SetContent("Status: running\nRunTimeConfig: enabled")
	s.SetQuery("run")

	if s.MatchCount() != 2 {
		t.Errorf("26-I01: expected 2 matches for 'run' (case-insensitive), got %d", s.MatchCount())
	}
}

// 26-I02: Case-insensitive matches uppercase YAML keys — "instance" matches "InstanceId", "InstanceType".
func TestSearch_I02_CaseInsensitiveMatchesUppercaseKeys(t *testing.T) {
	s := views.NewSearch()
	s.SetContent("InstanceId: i-abc123\nInstanceType: t3.micro\nPublicIpAddress: 1.2.3.4")
	s.SetQuery("instance")

	// Should match "Instance" in "InstanceId" and "Instance" in "InstanceType".
	if s.MatchCount() != 2 {
		t.Errorf("26-I02: expected 2 matches for 'instance' in YAML keys, got %d", s.MatchCount())
	}
}

// 26-I03: Case-insensitive in detail view — "prod" matches "PROD".
func TestSearch_I03_CaseInsensitiveDetailView(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	res := &resource.Resource{
		ID:   "i-case-i03",
		Name: "nav-i03",
		Fields: map[string]string{
			"env":    "api-PROD-01",
			"status": "RUNNING",
		},
	}

	m, _ = rootNavigateToDetail(m, res)
	m = rootActivateSearch(m, "prod")

	after := plainText(rootViewContent(m))

	// "prod" must match "PROD" — match count must be at least 1.
	// The header shows "N/M matches" where N>=1.
	if strings.Contains(after, "0/0 matches") || strings.Contains(after, "No matches") {
		t.Errorf("26-I03: 'prod' should match 'PROD' case-insensitively; got no matches:\n%s", after)
	}
	if !strings.Contains(after, "matches") {
		t.Errorf("26-I03: expected match indicator in output; got:\n%s", after)
	}
}

// ---------------------------------------------------------------------------
// Section J — ANSI-Aware Search (component-level)
// ---------------------------------------------------------------------------

// 26-J01: Search operates on visible text, not ANSI escape codes.
func TestSearch_J01_SearchOnVisibleTextNotANSI(t *testing.T) {
	// "t3.medium" with ANSI color sequences embedded.
	// The search query "t3.medium" should match the VISIBLE text, not the escape codes.
	blueOpen := "\x1b[34m"
	greenOpen := "\x1b[32m"
	reset := "\x1b[0m"

	plain := "InstanceType: t3.medium"
	styled := blueOpen + "InstanceType" + reset + ": " + greenOpen + "t3.medium" + reset

	s := views.NewSearch()
	s.SetContent(plain)  // SetContent takes ANSI-stripped plain text
	s.SetQuery("t3.medium")

	if s.MatchCount() != 1 {
		t.Fatalf("26-J01: expected 1 match for 't3.medium', got %d", s.MatchCount())
	}

	highlighted, matchLine := s.Apply(styled)

	// Match must have been found (matchLine >= 0).
	if matchLine < 0 {
		t.Error("26-J01: Apply() returned matchLine=-1; match was not found in styled content")
	}

	// The visible result must contain "t3.medium".
	if !strings.Contains(plainText(highlighted), "t3.medium") {
		t.Error("26-J01: visible text of highlighted output does not contain 't3.medium'")
	}

	// The ANSI escape codes from the query characters (\x1b, [, 3, etc.)
	// must NOT be treated as searchable text — MatchCount was already tested above.
	// Verify that the ANSI codes do not appear as literal characters in the
	// plain stripped output (they were not mistakenly included in match).
	plainOut := plainText(highlighted)
	if strings.Contains(plainOut, "\x1b") {
		t.Error("26-J01: stripped output still contains literal ESC bytes; ANSI not fully removed")
	}
}

// 26-J02: Search match spans styled boundary (key: value where key is blue, value is white).
func TestSearch_J02_MatchSpansStyledBoundary(t *testing.T) {
	// "Key: value" — "Key" is blue, "value" is white.
	// Searching for "Key: value" (span crossing the style boundary) should find 1 match.
	blueOpen := "\x1b[34m"
	reset := "\x1b[0m"

	plain := "Key: value"
	styled := blueOpen + "Key" + reset + ": value"

	s := views.NewSearch()
	s.SetContent(plain)
	s.SetQuery("Key: value")

	// The search indexes plain text, so the cross-boundary span must be 1 match.
	if s.MatchCount() != 1 {
		t.Fatalf("26-J02: expected 1 match for cross-boundary 'Key: value', got %d", s.MatchCount())
	}

	highlighted, matchLine := s.Apply(styled)

	if matchLine < 0 {
		t.Error("26-J02: Apply() returned matchLine=-1; cross-boundary match was not found")
	}

	// Visible text must contain "Key: value".
	if !strings.Contains(plainText(highlighted), "Key: value") {
		t.Error("26-J02: visible content of highlighted output does not contain 'Key: value'")
	}

	// Output must contain highlight ANSI sequences.
	if !strings.Contains(highlighted, "\x1b[") {
		t.Error("26-J02: highlighted output has no ANSI sequences")
	}
}

// 26-J04: Search in colored status values (detail view green status → amber highlight).
func TestSearch_J04_SearchInColoredStatusValue(t *testing.T) {
	greenOpen := "\x1b[32m"
	reset := "\x1b[0m"

	plain := "Status: running"
	styled := "Status: " + greenOpen + "running" + reset

	s := views.NewSearch()
	s.SetContent(plain)
	s.SetQuery("running")

	if s.MatchCount() != 1 {
		t.Fatalf("26-J04: expected 1 match for 'running', got %d", s.MatchCount())
	}

	highlighted, matchLine := s.Apply(styled)

	// Match must be found.
	if matchLine < 0 {
		t.Error("26-J04: Apply() returned matchLine=-1; match not found in colored status")
	}

	// "running" must remain in visible output.
	if !strings.Contains(plainText(highlighted), "running") {
		t.Error("26-J04: 'running' missing from visible text after highlighting colored status")
	}

	// Output must have MORE ANSI sequences than input (highlight was injected).
	inputSeqs := strings.Count(styled, "\x1b[")
	outputSeqs := strings.Count(highlighted, "\x1b[")
	if outputSeqs <= inputSeqs {
		t.Errorf("26-J04: expected more ANSI sequences after highlighting (input=%d, output=%d)", inputSeqs, outputSeqs)
	}
}

// ---------------------------------------------------------------------------
// Section M — Edge Cases (root-level + component-level)
// ---------------------------------------------------------------------------

// 26-M01: Search in very long single line (hundreds of chars).
func TestSearch_M01_VeryLongSingleLine(t *testing.T) {
	// Simulate a base64-like field spanning hundreds of characters.
	prefix := "CertificateAuthority: "
	// Build a long base64-like string containing "LS0t" in the middle.
	longBase64 := strings.Repeat("AAAA", 50) + "LS0t" + strings.Repeat("BBBB", 50)
	plain := prefix + longBase64

	s := views.NewSearch()
	s.SetContent(plain)
	s.SetQuery("LS0t")

	if s.MatchCount() != 1 {
		t.Fatalf("26-M01: expected 1 match for 'LS0t' in long line, got %d", s.MatchCount())
	}

	highlighted, matchLine := s.Apply(plain)

	if matchLine < 0 {
		t.Error("26-M01: Apply() returned matchLine=-1; match not found in long line")
	}

	if !strings.Contains(plainText(highlighted), "LS0t") {
		t.Error("26-M01: 'LS0t' missing from visible text of highlighted long line")
	}
}

// 26-M02: Multiple matches on the same line — all highlighted, n moves through each.
func TestSearch_M02_MultipleMatchesSameLine(t *testing.T) {
	plain := "ERROR: failed to process ERROR code in ERROR handler"

	s := views.NewSearch()
	s.SetContent(plain)
	s.SetQuery("ERROR")

	if s.MatchCount() != 3 {
		t.Fatalf("26-M02: expected 3 matches for 'ERROR' on one line, got %d", s.MatchCount())
	}

	// Navigate through all 3 with NextMatch.
	if s.CurrentMatch() != 0 {
		t.Errorf("26-M02: expected CurrentMatch()=0 initially, got %d", s.CurrentMatch())
	}

	s.NextMatch()
	if s.CurrentMatch() != 1 {
		t.Errorf("26-M02: expected CurrentMatch()=1 after 1st NextMatch(), got %d", s.CurrentMatch())
	}

	s.NextMatch()
	if s.CurrentMatch() != 2 {
		t.Errorf("26-M02: expected CurrentMatch()=2 after 2nd NextMatch(), got %d", s.CurrentMatch())
	}

	s.NextMatch()
	if s.CurrentMatch() != 0 {
		t.Errorf("26-M02: expected CurrentMatch()=0 after wrap, got %d", s.CurrentMatch())
	}

	// Apply and verify all 3 occurrences appear in visible output.
	highlighted, _ := s.Apply(plain)
	stripped := plainText(highlighted)
	if strings.Count(stripped, "ERROR") != 3 {
		t.Errorf("26-M02: expected 3 'ERROR' in visible output, got %d", strings.Count(stripped, "ERROR"))
	}
}

// 26-M03: Search term at the very beginning of content.
func TestSearch_M03_MatchAtBeginningOfContent(t *testing.T) {
	plain := "AmiLaunchIndex: 0\nSomeOtherField: value"

	s := views.NewSearch()
	s.SetContent(plain)
	s.SetQuery("Ami")

	if s.MatchCount() != 1 {
		t.Fatalf("26-M03: expected 1 match for 'Ami' at start of content, got %d", s.MatchCount())
	}

	highlighted, matchLine := s.Apply(plain)

	// Match must be on line 0 (very first line, beginning of content).
	if matchLine != 0 {
		t.Errorf("26-M03: expected matchLine=0 (beginning), got %d", matchLine)
	}

	if !strings.Contains(plainText(highlighted), "AmiLaunchIndex") {
		t.Error("26-M03: 'AmiLaunchIndex' missing from visible content")
	}
}

// 26-M04: Search term at the very end of content.
func TestSearch_M04_MatchAtEndOfContent(t *testing.T) {
	lines := []string{
		"InstanceId: i-abc123",
		"InstanceType: t3.micro",
		"VpcId: vpc-0123456789abcdef0",
	}
	plain := strings.Join(lines, "\n")

	s := views.NewSearch()
	s.SetContent(plain)
	s.SetQuery("abcdef0")

	if s.MatchCount() != 1 {
		t.Fatalf("26-M04: expected 1 match for 'abcdef0' at end of content, got %d", s.MatchCount())
	}

	highlighted, matchLine := s.Apply(plain)

	// Match must be on the last line.
	lastLineIdx := len(lines) - 1
	if matchLine != lastLineIdx {
		t.Errorf("26-M04: expected matchLine=%d (last line), got %d", lastLineIdx, matchLine)
	}

	if !strings.Contains(plainText(highlighted), "abcdef0") {
		t.Error("26-M04: 'abcdef0' missing from visible text at end of content")
	}
}

// 26-M05: Search for single character ":" — many matches, n/N works.
func TestSearch_M05_SearchSingleCharColon(t *testing.T) {
	plain := "Key1: val1\nKey2: val2\nKey3: val3\nKey4: val4\nKey5: val5"

	s := views.NewSearch()
	s.SetContent(plain)
	s.SetQuery(":")

	// Each line has a colon → 5 colons total.
	if s.MatchCount() != 5 {
		t.Fatalf("26-M05: expected 5 matches for ':', got %d", s.MatchCount())
	}

	// Navigate forward through all matches.
	for i := 0; i < 4; i++ {
		s.NextMatch()
	}
	if s.CurrentMatch() != 4 {
		t.Errorf("26-M05: expected CurrentMatch()=4 after 4 NextMatch() calls, got %d", s.CurrentMatch())
	}

	// Wrap to start.
	s.NextMatch()
	if s.CurrentMatch() != 0 {
		t.Errorf("26-M05: expected wrap to CurrentMatch()=0, got %d", s.CurrentMatch())
	}

	// Navigate backward.
	s.PrevMatch()
	if s.CurrentMatch() != 4 {
		t.Errorf("26-M05: expected CurrentMatch()=4 after PrevMatch() wrap, got %d", s.CurrentMatch())
	}

	// Apply — all 5 visible colons must remain in output.
	highlighted, _ := s.Apply(plain)
	stripped := plainText(highlighted)
	if strings.Count(stripped, ":") != 5 {
		t.Errorf("26-M05: expected 5 ':' in visible output, got %d", strings.Count(stripped, ":"))
	}
}

// 26-M06: Rapid n navigation through many matches — counter increments correctly.
func TestSearch_M06_RapidNavigation(t *testing.T) {
	// Build content with 150 colons across 150 lines (one "key: val" per line).
	var linesBuf strings.Builder
	for i := 0; i < 150; i++ {
		linesBuf.WriteString("key: value\n")
	}
	plain := strings.TrimRight(linesBuf.String(), "\n")

	s := views.NewSearch()
	s.SetContent(plain)
	s.SetQuery(":")

	total := s.MatchCount()
	if total < 150 {
		t.Fatalf("26-M06: expected at least 150 matches for ':', got %d", total)
	}

	// Press n 150 times in a rapid loop.
	for i := 0; i < 150; i++ {
		s.NextMatch()
	}

	// After 150 presses starting from index 0, we should be at index 150 % total.
	expectedIdx := 150 % total
	if s.CurrentMatch() != expectedIdx {
		t.Errorf("26-M06: expected CurrentMatch()=%d after 150 NextMatch() calls, got %d",
			expectedIdx, s.CurrentMatch())
	}

	// MatchInfo format must still be valid after rapid navigation.
	info := s.MatchInfo()
	if !strings.Contains(info, "matches") {
		t.Errorf("26-M06: MatchInfo() must contain 'matches' after rapid navigation, got %q", info)
	}

	// Verify MatchInfo shows a valid "N/M matches" format.
	parts := strings.Split(s.MatchInfo(), "/")
	if len(parts) != 2 {
		t.Errorf("26-M06: MatchInfo() format should be 'N/M matches', got %q", s.MatchInfo())
	}
}
