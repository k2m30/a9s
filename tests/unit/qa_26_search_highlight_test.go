package unit

// Cross-view search: highlighting, navigation, case sensitivity, ANSI-aware
// matching and edge cases.

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	tui "github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// searchActivateAndConfirm sets a query on s (bypassing input-mode key events)
// and leaves the model in confirmed-search state: active=true, inputMode=false.
func searchActivateAndConfirm(s *views.SearchModel, plain, query string) {
	s.Activate()
	s.SetContent(plain)
	s.SetQuery(query)
	*s, _ = s.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
}

// rootNavigateToDetail navigates the root model to a detail view for the given resource.
func rootNavigateToDetail(m tui.Model, res *resource.Resource) (tui.Model, tea.Cmd) {
	return rootApplyMsg(m, messages.Navigate{Target: messages.TargetDetail, Resource: res})
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

// All matches are highlighted with ANSI sequences once the search is confirmed.
func TestSearch_D01_AllMatchesHighlighted(t *testing.T) {
	plain := "running state: running\nnot matching\nalso running here"
	s := views.SearchModel{}
	searchActivateAndConfirm(&s, plain, "running")

	if s.MatchCount() != 3 {
		t.Fatalf("26-D01: expected 3 matches for 'running', got %d", s.MatchCount())
	}

	highlighted, _ := s.Apply(plain)

	ansiCount := strings.Count(highlighted, "\x1b[")
	if ansiCount == 0 {
		t.Error("26-D01: Apply() added no ANSI sequences; all matches should be highlighted")
	}

	strippedOut := plainText(highlighted)
	if strings.Count(strippedOut, "running") != 3 {
		t.Errorf("26-D01: stripped output contains wrong count of 'running'; want 3, got %d",
			strings.Count(strippedOut, "running"))
	}
}

// The current match renders with different ANSI sequences from the others.
func TestSearch_D02_CurrentMatchDistinctFromOthers(t *testing.T) {
	plain := "match one\nmatch two\nmatch three\nmatch four\nmatch five"
	s := views.SearchModel{}
	searchActivateAndConfirm(&s, plain, "match")

	if s.MatchCount() != 5 {
		t.Fatalf("26-D02: expected 5 matches, got %d", s.MatchCount())
	}

	s.NextMatch()
	if s.CurrentMatch() != 1 {
		t.Fatalf("26-D02: expected CurrentMatch()=1 after NextMatch(), got %d", s.CurrentMatch())
	}

	highlighted, _ := s.Apply(plain)

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

	currentANSI := ansiRe.FindAllString(currentLine, -1)
	otherANSI := ansiRe.FindAllString(otherLine, -1)

	if len(currentANSI) == 0 || len(otherANSI) == 0 {
		t.Fatal("26-D02: both current and non-current lines must contain ANSI sequences")
	}

	if strings.Join(currentANSI, "") == strings.Join(otherANSI, "") {
		t.Error("26-D02: current match and non-current match have identical ANSI style codes; they must differ")
	}
}

// Search highlighting overrides syntax coloring in styled text.
func TestSearch_D03_HighlightOverridesSyntaxColor(t *testing.T) {
	greenOpen := "\x1b[32m"
	reset := "\x1b[0m"
	blueOpen := "\x1b[34m"

	plain := "InstanceType: t3.medium"
	styled := blueOpen + "InstanceType" + reset + ": " + greenOpen + "t3.medium" + reset

	s := views.SearchModel{}
	s.SetContent(plain)
	s.SetQuery("medium")

	if s.MatchCount() != 1 {
		t.Fatalf("26-D03: expected 1 match for 'medium', got %d", s.MatchCount())
	}

	highlighted, _ := s.Apply(styled)

	if !strings.Contains(plainText(highlighted), "medium") {
		t.Error("26-D03: Apply() removed 'medium' from visible content")
	}

	if !strings.Contains(highlighted, "\x1b[") {
		t.Error("26-D03: highlighted output has no ANSI sequences at all")
	}

	stripped := plainText(highlighted)
	if !strings.Contains(stripped, "t3.") {
		t.Error("26-D03: non-matched 't3.' portion missing from visible content")
	}
}

// Search highlighting overrides status coloring in the detail view.
func TestSearch_D04_HighlightOverridesStatusColor(t *testing.T) {
	greenOpen := "\x1b[32m"
	reset := "\x1b[0m"

	plain := "Status: running"
	styled := "Status: " + greenOpen + "running" + reset

	s := views.SearchModel{}
	s.SetContent(plain)
	s.SetQuery("running")

	if s.MatchCount() != 1 {
		t.Fatalf("26-D04: expected 1 match for 'running', got %d", s.MatchCount())
	}

	highlighted, _ := s.Apply(styled)

	if !strings.Contains(plainText(highlighted), "running") {
		t.Error("26-D04: 'running' missing from highlighted output visible text")
	}

	if !strings.Contains(highlighted, "\x1b[") {
		t.Error("26-D04: highlighted output contains no ANSI sequences")
	}

	inputANSI := strings.Count(styled, "\x1b[")
	outputANSI := strings.Count(highlighted, "\x1b[")
	if outputANSI <= inputANSI {
		t.Errorf("26-D04: expected more ANSI sequences in output than input (input=%d, output=%d)", inputANSI, outputANSI)
	}
}

// A partial match within a styled token highlights only the matched portion.
func TestSearch_D05_PartialMatchWithinStyledToken(t *testing.T) {
	greenOpen := "\x1b[32m"
	reset := "\x1b[0m"

	plain := "us-east-1a"
	styled := greenOpen + "us-east-1a" + reset

	s := views.SearchModel{}
	s.SetContent(plain)
	s.SetQuery("east")

	if s.MatchCount() != 1 {
		t.Fatalf("26-D05: expected 1 match for 'east', got %d", s.MatchCount())
	}

	highlighted, _ := s.Apply(styled)

	stripped := plainText(highlighted)

	if !strings.Contains(stripped, "us-east-1a") {
		t.Errorf("26-D05: expected full 'us-east-1a' in stripped output, got: %q", stripped)
	}

	if !strings.Contains(highlighted, "\x1b[") {
		t.Error("26-D05: no ANSI sequences in highlighted output")
	}

	if strings.Count(highlighted, "\x1b[") <= strings.Count(styled, "\x1b[") {
		t.Error("26-D05: Apply() did not add highlight sequences for the partial 'east' match")
	}
}

// n advances to the next match: "1/4" → "2/4".
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

	before := plainText(rootViewContent(m))
	if !strings.Contains(before, "1/4 matches") {
		t.Fatalf("26-E01: expected '1/4 matches' after confirming search; got plain:\n%s", before)
	}

	m, _ = rootApplyMsg(m, rootKeyPress("n"))
	after := plainText(rootViewContent(m))
	if !strings.Contains(after, "2/4 matches") {
		t.Errorf("26-E01: expected '2/4 matches' after pressing n; got:\n%s", after)
	}
}

// n wraps from the last match to the first: "4/4" → "1/4".
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

	for range 3 {
		m, _ = rootApplyMsg(m, rootKeyPress("n"))
	}

	atLast := plainText(rootViewContent(m))
	if !strings.Contains(atLast, "4/4 matches") {
		t.Fatalf("26-E02: expected '4/4 matches' after 3 n-presses; got:\n%s", atLast)
	}

	m, _ = rootApplyMsg(m, rootKeyPress("n"))
	wrapped := plainText(rootViewContent(m))
	if !strings.Contains(wrapped, "1/4 matches") {
		t.Errorf("26-E02: expected '1/4 matches' after wrapping; got:\n%s", wrapped)
	}
}

// N moves to the previous match: "3/4" → "2/4".
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

	m, _ = rootApplyMsg(m, rootKeyPress("n"))
	m, _ = rootApplyMsg(m, rootKeyPress("n"))

	at3 := plainText(rootViewContent(m))
	if !strings.Contains(at3, "3/4 matches") {
		t.Fatalf("26-E03: expected '3/4 matches' after 2 n-presses; got:\n%s", at3)
	}

	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: -1, Text: "N"})
	after := plainText(rootViewContent(m))
	if !strings.Contains(after, "2/4 matches") {
		t.Errorf("26-E03: expected '2/4 matches' after pressing N; got:\n%s", after)
	}
}

// N wraps from the first match to the last: "1/4" → "4/4".
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

// n/N with a single match stay at "1/1".
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

	m, _ = rootApplyMsg(m, rootKeyPress("n"))
	afterN := plainText(rootViewContent(m))
	if !strings.Contains(afterN, "1/1 matches") {
		t.Errorf("26-E05: expected '1/1 matches' after n (single match); got:\n%s", afterN)
	}

	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: -1, Text: "N"})
	afterShiftN := plainText(rootViewContent(m))
	if !strings.Contains(afterShiftN, "1/1 matches") {
		t.Errorf("26-E05: expected '1/1 matches' after N (single match); got:\n%s", afterShiftN)
	}
}

// Search is case-insensitive by default: "run" matches "running" and
// "RunTimeConfig".
func TestSearch_I01_CaseInsensitiveDefault(t *testing.T) {
	s := views.SearchModel{}
	s.SetContent("Status: running\nRunTimeConfig: enabled")
	s.SetQuery("run")

	if s.MatchCount() != 2 {
		t.Errorf("26-I01: expected 2 matches for 'run' (case-insensitive), got %d", s.MatchCount())
	}
}

// Case-insensitive search matches uppercase YAML keys: "instance" matches
// "InstanceId" and "InstanceType".
func TestSearch_I02_CaseInsensitiveMatchesUppercaseKeys(t *testing.T) {
	s := views.SearchModel{}
	s.SetContent("InstanceId: i-abc123\nInstanceType: t3.micro\nPublicIpAddress: 1.2.3.4")
	s.SetQuery("instance")

	if s.MatchCount() != 2 {
		t.Errorf("26-I02: expected 2 matches for 'instance' in YAML keys, got %d", s.MatchCount())
	}
}

// Case-insensitive search in the detail view: "prod" matches "PROD".
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

	if strings.Contains(after, "0/0 matches") || strings.Contains(after, "No matches") {
		t.Errorf("26-I03: 'prod' should match 'PROD' case-insensitively; got no matches:\n%s", after)
	}
	if !strings.Contains(after, "matches") {
		t.Errorf("26-I03: expected match indicator in output; got:\n%s", after)
	}
}

// Search operates on visible text, not ANSI escape codes.
func TestSearch_J01_SearchOnVisibleTextNotANSI(t *testing.T) {
	blueOpen := "\x1b[34m"
	greenOpen := "\x1b[32m"
	reset := "\x1b[0m"

	plain := "InstanceType: t3.medium"
	styled := blueOpen + "InstanceType" + reset + ": " + greenOpen + "t3.medium" + reset

	s := views.SearchModel{}
	s.SetContent(plain) // SetContent takes ANSI-stripped plain text
	s.SetQuery("t3.medium")

	if s.MatchCount() != 1 {
		t.Fatalf("26-J01: expected 1 match for 't3.medium', got %d", s.MatchCount())
	}

	highlighted, matchLine := s.Apply(styled)

	if matchLine < 0 {
		t.Error("26-J01: Apply() returned matchLine=-1; match was not found in styled content")
	}

	if !strings.Contains(plainText(highlighted), "t3.medium") {
		t.Error("26-J01: visible text of highlighted output does not contain 't3.medium'")
	}

	plainOut := plainText(highlighted)
	if strings.Contains(plainOut, "\x1b") {
		t.Error("26-J01: stripped output still contains literal ESC bytes; ANSI not fully removed")
	}
}

// A match can span a style boundary: "Key: value" with a blue key and a white
// value.
func TestSearch_J02_MatchSpansStyledBoundary(t *testing.T) {
	blueOpen := "\x1b[34m"
	reset := "\x1b[0m"

	plain := "Key: value"
	styled := blueOpen + "Key" + reset + ": value"

	s := views.SearchModel{}
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

	if !strings.Contains(plainText(highlighted), "Key: value") {
		t.Error("26-J02: visible content of highlighted output does not contain 'Key: value'")
	}

	if !strings.Contains(highlighted, "\x1b[") {
		t.Error("26-J02: highlighted output has no ANSI sequences")
	}
}

// Search finds a colored status value in the detail view.
func TestSearch_J04_SearchInColoredStatusValue(t *testing.T) {
	greenOpen := "\x1b[32m"
	reset := "\x1b[0m"

	plain := "Status: running"
	styled := "Status: " + greenOpen + "running" + reset

	s := views.SearchModel{}
	s.SetContent(plain)
	s.SetQuery("running")

	if s.MatchCount() != 1 {
		t.Fatalf("26-J04: expected 1 match for 'running', got %d", s.MatchCount())
	}

	highlighted, matchLine := s.Apply(styled)

	if matchLine < 0 {
		t.Error("26-J04: Apply() returned matchLine=-1; match not found in colored status")
	}

	if !strings.Contains(plainText(highlighted), "running") {
		t.Error("26-J04: 'running' missing from visible text after highlighting colored status")
	}

	inputSeqs := strings.Count(styled, "\x1b[")
	outputSeqs := strings.Count(highlighted, "\x1b[")
	if outputSeqs <= inputSeqs {
		t.Errorf("26-J04: expected more ANSI sequences after highlighting (input=%d, output=%d)", inputSeqs, outputSeqs)
	}
}

// Search in a single line hundreds of characters long.
func TestSearch_M01_VeryLongSingleLine(t *testing.T) {
	prefix := "CertificateAuthority: "
	longBase64 := strings.Repeat("AAAA", 50) + "LS0t" + strings.Repeat("BBBB", 50)
	plain := prefix + longBase64

	s := views.SearchModel{}
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

// Multiple matches on one line are all highlighted, and n moves through each.
func TestSearch_M02_MultipleMatchesSameLine(t *testing.T) {
	plain := "ERROR: failed to process ERROR code in ERROR handler"

	s := views.SearchModel{}
	s.SetContent(plain)
	s.SetQuery("ERROR")

	if s.MatchCount() != 3 {
		t.Fatalf("26-M02: expected 3 matches for 'ERROR' on one line, got %d", s.MatchCount())
	}

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

	highlighted, _ := s.Apply(plain)
	stripped := plainText(highlighted)
	if strings.Count(stripped, "ERROR") != 3 {
		t.Errorf("26-M02: expected 3 'ERROR' in visible output, got %d", strings.Count(stripped, "ERROR"))
	}
}

// A search term at the very beginning of the content.
func TestSearch_M03_MatchAtBeginningOfContent(t *testing.T) {
	plain := "AmiLaunchIndex: 0\nSomeOtherField: value"

	s := views.SearchModel{}
	s.SetContent(plain)
	s.SetQuery("Ami")

	if s.MatchCount() != 1 {
		t.Fatalf("26-M03: expected 1 match for 'Ami' at start of content, got %d", s.MatchCount())
	}

	highlighted, matchLine := s.Apply(plain)

	if matchLine != 0 {
		t.Errorf("26-M03: expected matchLine=0 (beginning), got %d", matchLine)
	}

	if !strings.Contains(plainText(highlighted), "AmiLaunchIndex") {
		t.Error("26-M03: 'AmiLaunchIndex' missing from visible content")
	}
}

// A search term at the very end of the content.
func TestSearch_M04_MatchAtEndOfContent(t *testing.T) {
	lines := []string{
		"InstanceId: i-abc123",
		"InstanceType: t3.micro",
		"VpcId: vpc-0123456789abcdef0",
	}
	plain := strings.Join(lines, "\n")

	s := views.SearchModel{}
	s.SetContent(plain)
	s.SetQuery("abcdef0")

	if s.MatchCount() != 1 {
		t.Fatalf("26-M04: expected 1 match for 'abcdef0' at end of content, got %d", s.MatchCount())
	}

	highlighted, matchLine := s.Apply(plain)

	lastLineIdx := len(lines) - 1
	if matchLine != lastLineIdx {
		t.Errorf("26-M04: expected matchLine=%d (last line), got %d", lastLineIdx, matchLine)
	}

	if !strings.Contains(plainText(highlighted), "abcdef0") {
		t.Error("26-M04: 'abcdef0' missing from visible text at end of content")
	}
}

// Searching for ":" gives many matches, and n/N work across them.
func TestSearch_M05_SearchSingleCharColon(t *testing.T) {
	plain := "Key1: val1\nKey2: val2\nKey3: val3\nKey4: val4\nKey5: val5"

	s := views.SearchModel{}
	s.SetContent(plain)
	s.SetQuery(":")

	if s.MatchCount() != 5 {
		t.Fatalf("26-M05: expected 5 matches for ':', got %d", s.MatchCount())
	}

	for range 4 {
		s.NextMatch()
	}
	if s.CurrentMatch() != 4 {
		t.Errorf("26-M05: expected CurrentMatch()=4 after 4 NextMatch() calls, got %d", s.CurrentMatch())
	}

	s.NextMatch()
	if s.CurrentMatch() != 0 {
		t.Errorf("26-M05: expected wrap to CurrentMatch()=0, got %d", s.CurrentMatch())
	}

	s.PrevMatch()
	if s.CurrentMatch() != 4 {
		t.Errorf("26-M05: expected CurrentMatch()=4 after PrevMatch() wrap, got %d", s.CurrentMatch())
	}

	highlighted, _ := s.Apply(plain)
	stripped := plainText(highlighted)
	if strings.Count(stripped, ":") != 5 {
		t.Errorf("26-M05: expected 5 ':' in visible output, got %d", strings.Count(stripped, ":"))
	}
}

// Rapid n navigation through many matches keeps the counter correct.
func TestSearch_M06_RapidNavigation(t *testing.T) {
	var linesBuf strings.Builder
	for range 150 {
		linesBuf.WriteString("key: value\n")
	}
	plain := strings.TrimRight(linesBuf.String(), "\n")

	s := views.SearchModel{}
	s.SetContent(plain)
	s.SetQuery(":")

	total := s.MatchCount()
	if total < 150 {
		t.Fatalf("26-M06: expected at least 150 matches for ':', got %d", total)
	}

	for range 150 {
		s.NextMatch()
	}

	expectedIdx := 150 % total
	if s.CurrentMatch() != expectedIdx {
		t.Errorf("26-M06: expected CurrentMatch()=%d after 150 NextMatch() calls, got %d",
			expectedIdx, s.CurrentMatch())
	}

	info := s.MatchInfo()
	if !strings.Contains(info, "matches") {
		t.Errorf("26-M06: MatchInfo() must contain 'matches' after rapid navigation, got %q", info)
	}

	parts := strings.Split(s.MatchInfo(), "/")
	if len(parts) != 2 {
		t.Errorf("26-M06: MatchInfo() format should be 'N/M matches', got %q", s.MatchInfo())
	}
}
