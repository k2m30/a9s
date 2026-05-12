package unit

// qa_26_search_core_test.go — Root-level integration tests for QA-26: Cross-View
// Search Component (Issue #89). All tests go through the root model via
// rootApplyMsg so that real key routing is exercised end-to-end.
//
// Sections implemented:
//   A — Activation (detail + YAML; A03-A05 skipped, views not yet implemented)
//   B — Typing a search query
//   C — Confirming search (Enter) and cancellation (Esc)
//   F — Match counter display
//   H — Empty / no-match states

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/resource"
	tui "github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
)

// ---------------------------------------------------------------------------
// Shared fixtures
// ---------------------------------------------------------------------------

// qa26Resource returns a resource with "running" in two fields (→ 2 matches),
// plus an availability zone to support multi-value testing.
func qa26Resource() *resource.Resource {
	return &resource.Resource{
		ID:   "i-test",
		Name: "test-instance",
		Fields: map[string]string{
			"state":             "running",
			"power_state":       "running",
			"instance_type":     "t3.medium",
			"availability_zone": "us-east-1a",
		},
	}
}

// qa26EmptyResource returns a resource with no content that could match
// "nonexistent-text".
func qa26EmptyResource() *resource.Resource {
	return &resource.Resource{
		ID:     "i-empty",
		Name:   "empty-resource",
		Fields: map[string]string{"key": "value"},
	}
}

// qa26SpecialCharsResource returns a resource whose fields contain colons and
// dashes to verify they are treated as literal search characters.
func qa26SpecialCharsResource() *resource.Resource {
	return &resource.Resource{
		ID:   "i-special",
		Name: "s3:GetObject-test",
		Fields: map[string]string{
			"arn":  "arn:aws:s3:::my-bucket",
			"zone": "us-east-1a",
		},
	}
}

// qa26NavigateDetail navigates the root model to the detail view for res.
func qa26NavigateDetail(m tui.Model, res *resource.Resource) tui.Model {
	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetDetail, Resource: res})
	return m
}

// qa26NavigateYAML navigates the root model to the YAML view for res.
func qa26NavigateYAML(m tui.Model, res *resource.Resource) tui.Model {
	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetYAML, Resource: res})
	return m
}

// qa26ActivateSearch presses "/" to enter search input mode.
func qa26ActivateSearch(m tui.Model) tui.Model {
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: '/', Text: "/"})
	return m
}

// qa26TypeQuery types each character of q into the search input one at a time.
func qa26TypeQuery(m tui.Model, q string) tui.Model {
	for _, ch := range q {
		m, _ = rootApplyMsg(m, rootKeyPress(string(ch)))
	}
	return m
}

// qa26PressEnter sends the Enter key.
func qa26PressEnter(m tui.Model) tui.Model {
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	return m
}

// qa26PressEsc sends the Escape key.
func qa26PressEsc(m tui.Model) tui.Model {
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	return m
}

// qa26PressBackspace sends a single Backspace key.
func qa26PressBackspace(m tui.Model) tui.Model {
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyBackspace})
	return m
}

// qa26PlainView returns the ANSI-stripped content of the current view.
func qa26PlainView(m tui.Model) string {
	return ansiRe.ReplaceAllString(rootViewContent(m), "")
}

// ---------------------------------------------------------------------------
// Section A — Activation
// ---------------------------------------------------------------------------

// 26-A01: "/" in detail view → header changes from "? for help" to "/"
func TestQA26_A01_SlashInDetailActivatesSearch(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()
	m = qa26NavigateDetail(m, qa26Resource())

	// Precondition: header shows "? for help" in normal mode.
	beforePlain := qa26PlainView(m)
	if !strings.Contains(beforePlain, "? for help") {
		t.Fatalf("precondition: expected '? for help' in header before search; got: %q", beforePlain)
	}

	// When: press "/".
	m = qa26ActivateSearch(m)

	afterPlain := qa26PlainView(m)

	// Then: "? for help" is gone and "/" is present in the header.
	if strings.Contains(afterPlain, "? for help") {
		t.Error("26-A01: header should not show '? for help' once search input is active")
	}
	if !strings.Contains(afterPlain, "/") {
		t.Errorf("26-A01: header should show '/' when search is active; got: %q", afterPlain)
	}
}

// 26-A02: "/" in YAML view → header changes from "? for help" to "/"
func TestQA26_A02_SlashInYAMLActivatesSearch(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()
	m = qa26NavigateYAML(m, qa26Resource())

	// Precondition: header shows "? for help".
	beforePlain := qa26PlainView(m)
	if !strings.Contains(beforePlain, "? for help") {
		t.Fatalf("precondition: expected '? for help' in header before search; got: %q", beforePlain)
	}

	// When: press "/".
	m = qa26ActivateSearch(m)

	afterPlain := qa26PlainView(m)

	// Then: "? for help" is replaced by "/".
	if strings.Contains(afterPlain, "? for help") {
		t.Error("26-A02: YAML header should not show '? for help' when search is active")
	}
	if !strings.Contains(afterPlain, "/") {
		t.Errorf("26-A02: YAML header should show '/' when search is active; got: %q", afterPlain)
	}
}

// ---------------------------------------------------------------------------
// Section B — Typing
// ---------------------------------------------------------------------------

// 26-B01: Characters typed appear in the header search input.
// Type "running" → header shows "/running".
func TestQA26_B01_TypedCharsAppearInHeader(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()
	m = qa26NavigateDetail(m, qa26Resource())
	m = qa26ActivateSearch(m)
	m = qa26TypeQuery(m, "running")

	plain := qa26PlainView(m)

	if !strings.Contains(plain, "/running") {
		t.Errorf("26-B01: header should show '/running' after typing; got: %q", plain)
	}
}

// 26-B01 (YAML variant): same behavior in the YAML view.
func TestQA26_B01_TypedCharsAppearInHeader_YAML(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()
	m = qa26NavigateYAML(m, qa26Resource())
	m = qa26ActivateSearch(m)
	m = qa26TypeQuery(m, "running")

	plain := qa26PlainView(m)

	if !strings.Contains(plain, "/running") {
		t.Errorf("26-B01 YAML: header should show '/running' after typing; got: %q", plain)
	}
}

// 26-B02: Backspace removes the last character.
// "/running" → Backspace → "/runnin"
// "/runnin" + 5× Backspace → "/"
func TestQA26_B02_BackspaceRemovesLastChar(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()
	m = qa26NavigateDetail(m, qa26Resource())
	m = qa26ActivateSearch(m)
	m = qa26TypeQuery(m, "running")

	// One Backspace: "running" → "runnin".
	m = qa26PressBackspace(m)
	plain := qa26PlainView(m)
	if !strings.Contains(plain, "/runnin") {
		t.Errorf("26-B02: after 1 Backspace header should show '/runnin'; got: %q", plain)
	}
	// Must not still show the full "/running".
	if strings.Contains(plain, "/running") {
		t.Error("26-B02: '/running' should be gone after Backspace removed the trailing 'g'")
	}

	// Six more Backspaces: "runnin" → "".
	for range 6 {
		m = qa26PressBackspace(m)
	}
	plain = qa26PlainView(m)
	// The header should show only "/" (empty query, not "/runnin" or "/running").
	if strings.Contains(plain, "/runnin") {
		t.Errorf("26-B02: after 7 Backspaces header should be empty '/'; still shows '/runnin': %q", plain)
	}
	if !strings.Contains(plain, "/") {
		t.Errorf("26-B02: header should still show '/' (search mode active but empty); got: %q", plain)
	}
}

// 26-B03: Matches highlight incrementally while typing (our impl highlights
// during typing, not only after Enter).
// After typing "running" the rendered output must contain ANSI escape sequences,
// confirming that highlights are produced.
func TestQA26_B03_HighlightsAppearedWhileTyping(t *testing.T) {
	tui.Version = "0.6.0"

	t.Run("DetailView", func(t *testing.T) {
		m := newRootSizedModel()
		m = qa26NavigateDetail(m, qa26Resource())
		m = qa26ActivateSearch(m)
		m = qa26TypeQuery(m, "running")

		raw := rootViewContent(m)
		if !strings.Contains(raw, "\x1b[") {
			t.Error("26-B03 detail: expected ANSI highlight sequences after typing 'running'")
		}
		// Visible text must still contain "running" unmodified.
		plain := ansiRe.ReplaceAllString(raw, "")
		if !strings.Contains(plain, "running") {
			t.Errorf("26-B03 detail: plain content missing 'running' after typing; got: %q", plain)
		}
	})

	t.Run("YAMLView", func(t *testing.T) {
		m := newRootSizedModel()
		m = qa26NavigateYAML(m, qa26Resource())
		m = qa26ActivateSearch(m)
		m = qa26TypeQuery(m, "running")

		raw := rootViewContent(m)
		if !strings.Contains(raw, "\x1b[") {
			t.Error("26-B03 YAML: expected ANSI highlight sequences after typing 'running'")
		}
		plain := ansiRe.ReplaceAllString(raw, "")
		if !strings.Contains(plain, "running") {
			t.Errorf("26-B03 YAML: plain content missing 'running' after typing; got: %q", plain)
		}
	})
}

// 26-B04: Special chars — colon in query is treated as a literal character.
// Type "s3:Get" → header shows "/s3:Get".
func TestQA26_B04_SpecialCharsColonTreatedLiteral(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()
	m = qa26NavigateDetail(m, qa26SpecialCharsResource())
	m = qa26ActivateSearch(m)
	m = qa26TypeQuery(m, "s3:Get")

	plain := qa26PlainView(m)

	if !strings.Contains(plain, "/s3:Get") {
		t.Errorf("26-B04: header should show '/s3:Get' with literal colon; got: %q", plain)
	}
	// Typing a colon must NOT have triggered command mode (":dbi" pattern).
	// Command mode would replace the search input with ":s3:Get".
	if strings.Contains(plain, ":s3:Get") {
		t.Error("26-B04: colon inside query must not activate command mode")
	}
}

// 26-B05: Dots, dashes, underscores in query — treated as literals.
// Type "us-east-1a" → header shows "/us-east-1a".
func TestQA26_B05_DotsAndDashesTreatedLiteral(t *testing.T) {
	tui.Version = "0.6.0"

	t.Run("DetailView", func(t *testing.T) {
		m := newRootSizedModel()
		m = qa26NavigateDetail(m, qa26SpecialCharsResource())
		m = qa26ActivateSearch(m)
		m = qa26TypeQuery(m, "us-east-1a")

		plain := qa26PlainView(m)
		if !strings.Contains(plain, "/us-east-1a") {
			t.Errorf("26-B05 detail: header should show '/us-east-1a'; got: %q", plain)
		}
	})

	t.Run("YAMLView", func(t *testing.T) {
		m := newRootSizedModel()
		m = qa26NavigateYAML(m, qa26SpecialCharsResource())
		m = qa26ActivateSearch(m)
		m = qa26TypeQuery(m, "us-east-1a")

		plain := qa26PlainView(m)
		if !strings.Contains(plain, "/us-east-1a") {
			t.Errorf("26-B05 YAML: header should show '/us-east-1a'; got: %q", plain)
		}
	})
}

// ---------------------------------------------------------------------------
// Section C — Confirming search (Enter) and cancellation (Esc)
// ---------------------------------------------------------------------------

// 26-C01: Enter confirms search → header shows match count "N/M matches",
// content has ANSI highlights.
func TestQA26_C01_EnterConfirmsSearch(t *testing.T) {
	tui.Version = "0.6.0"

	t.Run("DetailView", func(t *testing.T) {
		m := newRootSizedModel()
		m = qa26NavigateDetail(m, qa26Resource())
		m = qa26ActivateSearch(m)
		m = qa26TypeQuery(m, "running")
		m = qa26PressEnter(m)

		raw := rootViewContent(m)
		plain := ansiRe.ReplaceAllString(raw, "")

		// Header must show a match count indicator.
		if !strings.Contains(plain, "matches") {
			t.Errorf("26-C01 detail: header should contain 'matches' after confirming search; got: %q", plain)
		}
		// Confirmed search must produce ANSI highlights in the content.
		if !strings.Contains(raw, "\x1b[") {
			t.Error("26-C01 detail: expected ANSI highlight sequences after confirming search")
		}
	})

	t.Run("YAMLView", func(t *testing.T) {
		m := newRootSizedModel()
		m = qa26NavigateYAML(m, qa26Resource())
		m = qa26ActivateSearch(m)
		m = qa26TypeQuery(m, "running")
		m = qa26PressEnter(m)

		raw := rootViewContent(m)
		plain := ansiRe.ReplaceAllString(raw, "")

		if !strings.Contains(plain, "matches") {
			t.Errorf("26-C01 YAML: header should contain 'matches' after confirming search; got: %q", plain)
		}
		if !strings.Contains(raw, "\x1b[") {
			t.Error("26-C01 YAML: expected ANSI highlight sequences after confirming search")
		}
	})
}

// 26-C02: Enter on empty query → no search activated, header shows "? for help".
func TestQA26_C02_EnterOnEmptyQueryDoesNothing(t *testing.T) {
	tui.Version = "0.6.0"

	t.Run("DetailView", func(t *testing.T) {
		m := newRootSizedModel()
		m = qa26NavigateDetail(m, qa26Resource())
		m = qa26ActivateSearch(m)
		// Do NOT type anything — press Enter immediately on empty query.
		m = qa26PressEnter(m)

		plain := qa26PlainView(m)

		// Normal mode must be restored: header shows "? for help".
		if !strings.Contains(plain, "? for help") {
			t.Errorf("26-C02 detail: empty-query Enter should restore '? for help'; got: %q", plain)
		}
		// No match counter must appear.
		if strings.Contains(plain, "matches") {
			t.Error("26-C02 detail: empty-query Enter must not show a match count")
		}
	})

	t.Run("YAMLView", func(t *testing.T) {
		m := newRootSizedModel()
		m = qa26NavigateYAML(m, qa26Resource())
		m = qa26ActivateSearch(m)
		m = qa26PressEnter(m)

		plain := qa26PlainView(m)

		if !strings.Contains(plain, "? for help") {
			t.Errorf("26-C02 YAML: empty-query Enter should restore '? for help'; got: %q", plain)
		}
		if strings.Contains(plain, "matches") {
			t.Error("26-C02 YAML: empty-query Enter must not show a match count")
		}
	})
}

// 26-C03: Esc cancels input → header reverts to "? for help", no highlights.
func TestQA26_C03_EscCancelsInput(t *testing.T) {
	tui.Version = "0.6.0"

	t.Run("DetailView", func(t *testing.T) {
		m := newRootSizedModel()
		m = qa26NavigateDetail(m, qa26Resource())
		m = qa26ActivateSearch(m)
		m = qa26TypeQuery(m, "running")
		m = qa26PressEsc(m)

		plain := qa26PlainView(m)

		// Header must revert to normal mode.
		if !strings.Contains(plain, "? for help") {
			t.Errorf("26-C03 detail: Esc should restore '? for help'; got: %q", plain)
		}
		// No match counter must remain.
		if strings.Contains(plain, "matches") {
			t.Error("26-C03 detail: Esc should remove the match count indicator")
		}
		// No search input visible.
		if strings.Contains(plain, "/running") {
			t.Error("26-C03 detail: '/running' must be gone after Esc")
		}
	})

	t.Run("YAMLView", func(t *testing.T) {
		m := newRootSizedModel()
		m = qa26NavigateYAML(m, qa26Resource())
		m = qa26ActivateSearch(m)
		m = qa26TypeQuery(m, "running")
		m = qa26PressEsc(m)

		plain := qa26PlainView(m)

		if !strings.Contains(plain, "? for help") {
			t.Errorf("26-C03 YAML: Esc should restore '? for help'; got: %q", plain)
		}
		if strings.Contains(plain, "matches") {
			t.Error("26-C03 YAML: Esc should remove the match count indicator")
		}
		if strings.Contains(plain, "/running") {
			t.Error("26-C03 YAML: '/running' must be gone after Esc")
		}
	})
}

// ---------------------------------------------------------------------------
// Section F — Match counter display
// ---------------------------------------------------------------------------

// 26-F01: Match counter shows "1/N matches" on first confirm.
// qa26Resource has "running" in two fields → N = 2.
func TestQA26_F01_MatchCounterOnFirstConfirm(t *testing.T) {
	tui.Version = "0.6.0"

	t.Run("DetailView", func(t *testing.T) {
		m := newRootSizedModel()
		m = qa26NavigateDetail(m, qa26Resource())
		m = qa26ActivateSearch(m)
		m = qa26TypeQuery(m, "running")
		m = qa26PressEnter(m)

		plain := qa26PlainView(m)

		if !strings.Contains(plain, "1/2 matches") {
			t.Errorf("26-F01 detail: expected '1/2 matches' on first confirm; got: %q", plain)
		}
	})

	t.Run("YAMLView", func(t *testing.T) {
		m := newRootSizedModel()
		m = qa26NavigateYAML(m, qa26Resource())
		m = qa26ActivateSearch(m)
		m = qa26TypeQuery(m, "running")
		m = qa26PressEnter(m)

		plain := qa26PlainView(m)

		if !strings.Contains(plain, "1/2 matches") {
			t.Errorf("26-F01 YAML: expected '1/2 matches' on first confirm; got: %q", plain)
		}
	})
}

// 26-F02: Counter updates on "n" navigation.
// After confirming "running" (1/2 matches), press n → must show "2/2 matches".
func TestQA26_F02_CounterUpdatesOnNav(t *testing.T) {
	tui.Version = "0.6.0"

	t.Run("DetailView", func(t *testing.T) {
		m := newRootSizedModel()
		m = qa26NavigateDetail(m, qa26Resource())
		m = qa26ActivateSearch(m)
		m = qa26TypeQuery(m, "running")
		m = qa26PressEnter(m)

		// Confirm we're at 1/2.
		before := qa26PlainView(m)
		if !strings.Contains(before, "1/2 matches") {
			t.Fatalf("26-F02 detail precondition: expected '1/2 matches', got: %q", before)
		}

		// Press n.
		m, _ = rootApplyMsg(m, rootKeyPress("n"))
		after := qa26PlainView(m)
		if !strings.Contains(after, "2/2 matches") {
			t.Errorf("26-F02 detail: after 'n' expected '2/2 matches'; got: %q", after)
		}
	})

	t.Run("YAMLView", func(t *testing.T) {
		m := newRootSizedModel()
		m = qa26NavigateYAML(m, qa26Resource())
		m = qa26ActivateSearch(m)
		m = qa26TypeQuery(m, "running")
		m = qa26PressEnter(m)

		before := qa26PlainView(m)
		if !strings.Contains(before, "1/2 matches") {
			t.Fatalf("26-F02 YAML precondition: expected '1/2 matches', got: %q", before)
		}

		m, _ = rootApplyMsg(m, rootKeyPress("n"))
		after := qa26PlainView(m)
		if !strings.Contains(after, "2/2 matches") {
			t.Errorf("26-F02 YAML: after 'n' expected '2/2 matches'; got: %q", after)
		}
	})
}

// 26-F03: Zero matches → header shows "0/0 matches".
func TestQA26_F03_ZeroMatchesCounter(t *testing.T) {
	tui.Version = "0.6.0"

	t.Run("DetailView", func(t *testing.T) {
		m := newRootSizedModel()
		m = qa26NavigateDetail(m, qa26EmptyResource())
		m = qa26ActivateSearch(m)
		m = qa26TypeQuery(m, "nonexistent-term-xyz")
		m = qa26PressEnter(m)

		plain := qa26PlainView(m)
		if !strings.Contains(plain, "0/0 matches") {
			t.Errorf("26-F03 detail: expected '0/0 matches' for non-matching query; got: %q", plain)
		}
	})

	t.Run("YAMLView", func(t *testing.T) {
		m := newRootSizedModel()
		m = qa26NavigateYAML(m, qa26EmptyResource())
		m = qa26ActivateSearch(m)
		m = qa26TypeQuery(m, "nonexistent-term-xyz")
		m = qa26PressEnter(m)

		plain := qa26PlainView(m)
		if !strings.Contains(plain, "0/0 matches") {
			t.Errorf("26-F03 YAML: expected '0/0 matches' for non-matching query; got: %q", plain)
		}
	})
}

// 26-F04: Match counter appears in the header (our impl puts it in the header
// right side, not the bottom of the frame). After confirming "running" the
// counter must be on a line before the first frame border character ("┌").
func TestQA26_F04_CounterIsInHeader(t *testing.T) {
	tui.Version = "0.6.0"

	t.Run("DetailView", func(t *testing.T) {
		m := newRootSizedModel()
		m = qa26NavigateDetail(m, qa26Resource())
		m = qa26ActivateSearch(m)
		m = qa26TypeQuery(m, "running")
		m = qa26PressEnter(m)

		content := rootViewContent(m)
		lines := strings.Split(content, "\n")

		// Find the line containing the match indicator and the top frame border.
		matchLine := -1
		frameBorderLine := -1
		for i, line := range lines {
			plain := ansiRe.ReplaceAllString(line, "")
			if strings.Contains(plain, "1/2 matches") {
				matchLine = i
			}
			if strings.Contains(plain, "\u250c") { // ┌ top-left corner
				frameBorderLine = i
				break
			}
		}

		if matchLine == -1 {
			t.Fatalf("26-F04 detail: '1/2 matches' not found in rendered output")
		}
		if frameBorderLine == -1 {
			t.Fatalf("26-F04 detail: top frame border '┌' not found in rendered output")
		}
		if matchLine >= frameBorderLine {
			t.Errorf("26-F04 detail: match counter (line %d) should appear in the header (before frame border at line %d)", matchLine, frameBorderLine)
		}
	})

	t.Run("YAMLView", func(t *testing.T) {
		m := newRootSizedModel()
		m = qa26NavigateYAML(m, qa26Resource())
		m = qa26ActivateSearch(m)
		m = qa26TypeQuery(m, "running")
		m = qa26PressEnter(m)

		content := rootViewContent(m)
		lines := strings.Split(content, "\n")

		matchLine := -1
		frameBorderLine := -1
		for i, line := range lines {
			plain := ansiRe.ReplaceAllString(line, "")
			if strings.Contains(plain, "1/2 matches") {
				matchLine = i
			}
			if strings.Contains(plain, "\u250c") {
				frameBorderLine = i
				break
			}
		}

		if matchLine == -1 {
			t.Fatalf("26-F04 YAML: '1/2 matches' not found in rendered output")
		}
		if frameBorderLine == -1 {
			t.Fatalf("26-F04 YAML: top frame border '┌' not found in rendered output")
		}
		if matchLine >= frameBorderLine {
			t.Errorf("26-F04 YAML: match counter (line %d) should appear in the header (before frame border at line %d)", matchLine, frameBorderLine)
		}
	})
}

// ---------------------------------------------------------------------------
// Section H — Empty / no-match states
// ---------------------------------------------------------------------------

// 26-H01: Search in view with few fields for nonexistent text → "0/0 matches",
// no crash.
func TestQA26_H01_SearchFewFieldsNoMatch(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()
	m = qa26NavigateDetail(m, qa26EmptyResource())
	m = qa26ActivateSearch(m)
	m = qa26TypeQuery(m, "nonexistent-text")
	m = qa26PressEnter(m)

	plain := qa26PlainView(m)

	// Must show zero-match indicator.
	if !strings.Contains(plain, "0/0 matches") {
		t.Errorf("26-H01: expected '0/0 matches'; got: %q", plain)
	}
	// View must still be non-empty (no crash).
	if plain == "" {
		t.Fatal("26-H01: View() returned empty string (crash or blank)")
	}
}

// 26-H02: Search 100+ line YAML for nonexistent term → no highlights,
// "0/0 matches".
func TestQA26_H02_SearchLargeYAMLNoMatch(t *testing.T) {
	tui.Version = "0.6.0"

	// Build a resource with many fields to produce a multi-line YAML document.
	fields := map[string]string{
		"state":             "running",
		"instance_type":     "t3.large",
		"availability_zone": "us-east-1a",
		"vpc_id":            "vpc-0abc123def",
		"subnet_id":         "subnet-0abc123",
		"private_ip":        "10.0.1.42",
		"public_ip":         "54.0.0.1",
		"ami_id":            "ami-0abc1234567890abc",
		"key_name":          "my-key",
		"security_group":    "sg-0abc123",
		"platform":          "linux",
		"architecture":      "x86_64",
	}
	res := &resource.Resource{ID: "i-large-yaml", Name: "large-yaml-instance", Fields: fields}

	m := newRootSizedModel()
	m = qa26NavigateYAML(m, res)
	m = qa26ActivateSearch(m)
	m = qa26TypeQuery(m, "zzz-no-such-value-zzz")
	m = qa26PressEnter(m)

	raw := rootViewContent(m)
	plain := ansiRe.ReplaceAllString(raw, "")

	// Must show 0/0 matches.
	if !strings.Contains(plain, "0/0 matches") {
		t.Errorf("26-H02: expected '0/0 matches'; got: %q", plain)
	}
	// View must be non-empty.
	if plain == "" {
		t.Fatal("26-H02: View() returned empty string")
	}
}

// 26-H03: n/N pressed with zero matches → nothing happens, no crash.
func TestQA26_H03_NWithZeroMatchesNoOp(t *testing.T) {
	tui.Version = "0.6.0"

	t.Run("DetailView", func(t *testing.T) {
		m := newRootSizedModel()
		m = qa26NavigateDetail(m, qa26EmptyResource())
		m = qa26ActivateSearch(m)
		m = qa26TypeQuery(m, "nonexistent-text")
		m = qa26PressEnter(m)

		before := qa26PlainView(m)
		if !strings.Contains(before, "0/0 matches") {
			t.Fatalf("26-H03 detail precondition: expected '0/0 matches', got: %q", before)
		}

		// Press n — must not crash and counter must stay "0/0 matches".
		m, _ = rootApplyMsg(m, rootKeyPress("n"))
		afterN := qa26PlainView(m)
		if !strings.Contains(afterN, "0/0 matches") {
			t.Errorf("26-H03 detail: after 'n' with zero matches expected '0/0 matches'; got: %q", afterN)
		}

		// Press N — same.
		m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: -1, Text: "N"})
		afterShiftN := qa26PlainView(m)
		if !strings.Contains(afterShiftN, "0/0 matches") {
			t.Errorf("26-H03 detail: after 'N' with zero matches expected '0/0 matches'; got: %q", afterShiftN)
		}

		// View must still render.
		if afterShiftN == "" {
			t.Fatal("26-H03 detail: View() returned empty string after N with zero matches")
		}
	})

	t.Run("YAMLView", func(t *testing.T) {
		m := newRootSizedModel()
		m = qa26NavigateYAML(m, qa26EmptyResource())
		m = qa26ActivateSearch(m)
		m = qa26TypeQuery(m, "nonexistent-text")
		m = qa26PressEnter(m)

		before := qa26PlainView(m)
		if !strings.Contains(before, "0/0 matches") {
			t.Fatalf("26-H03 YAML precondition: expected '0/0 matches', got: %q", before)
		}

		m, _ = rootApplyMsg(m, rootKeyPress("n"))
		afterN := qa26PlainView(m)
		if !strings.Contains(afterN, "0/0 matches") {
			t.Errorf("26-H03 YAML: after 'n' with zero matches expected '0/0 matches'; got: %q", afterN)
		}

		m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: -1, Text: "N"})
		afterShiftN := qa26PlainView(m)
		if !strings.Contains(afterShiftN, "0/0 matches") {
			t.Errorf("26-H03 YAML: after 'N' with zero matches expected '0/0 matches'; got: %q", afterShiftN)
		}

		if afterShiftN == "" {
			t.Fatal("26-H03 YAML: View() returned empty string after N with zero matches")
		}
	})
}
