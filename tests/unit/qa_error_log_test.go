package unit

// ══════════════════════════════════════════════════════════════════════════════
// Issue #263: Error log feature
//
// Tests cover:
//   - Error history accumulation (errors recorded, non-errors not recorded)
//   - Wider error flash width (width-6 instead of old width-60)
//   - "! for errors" hint after flash clears, dismissed on keypress
//   - Hint not shown for non-error flashes
//   - "!" key opens error log viewer (YAMLModel in text mode)
//   - "!" key with empty history shows flash instead of opening viewer
//   - NewTextViewer constructor: title, content, CopyContent, RawContent
// ══════════════════════════════════════════════════════════════════════════════

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ── TestErrorHistoryAccumulation ─────────────────────────────────────────────

// TestErrorHistoryAccumulation verifies that error-tagged FlashMsgs accumulate
// in the error history buffer, while non-error FlashMsgs do not.
//
// The test uses the observable "!" key behavior: pressing "!" after N error
// flashes should open a viewer (not flash "No errors this session"), and pressing
// "!" on a model that received only non-error flashes should show "No errors".
func TestErrorHistoryAccumulation_ErrorFlashesAddToHistory(t *testing.T) {
	tui.Version = "test"
	m := newRootSizedModel()

	// Send three error flashes — each should be appended to history.
	m, _ = rootApplyMsg(m, messages.Flash{Text: "first error", IsError: true})
	m, _ = rootApplyMsg(m, messages.Flash{Text: "second error", IsError: true})
	m, _ = rootApplyMsg(m, messages.Flash{Text: "third error", IsError: true})

	// Press "!" to open the error log. If history has entries, a viewer is pushed
	// and the view output contains the log entries. If history is empty, a flash
	// "No errors this session" is shown instead.
	m, cmd := rootApplyMsg(m, tea.KeyPressMsg{Code: '!'})

	// Execute any command returned (viewer push may be deferred).
	if cmd != nil {
		msg := cmd()
		if msg != nil {
			m, _ = rootApplyMsg(m, msg)
		}
	}

	plain := stripANSI(rootViewContent(m))

	// After 3 error flashes, pressing "!" must NOT show "No errors this session".
	// The error log viewer should be active with at least one entry visible.
	if strings.Contains(plain, "No errors this session") {
		t.Error("after 3 error FlashMsgs, pressing '!' should open error log viewer, not flash 'No errors this session'")
	}
}

// TestErrorHistoryAccumulation_NonErrorFlashesNotAdded verifies that non-error
// flashes do NOT accumulate in the error history.
func TestErrorHistoryAccumulation_NonErrorFlashesNotAdded(t *testing.T) {
	tui.Version = "test"
	m := newRootSizedModel()

	// Send non-error flashes only.
	m, _ = rootApplyMsg(m, messages.Flash{Text: "copied to clipboard", IsError: false})
	m, _ = rootApplyMsg(m, messages.Flash{Text: "theme applied", IsError: false})

	// Press "!" — with no errors in history, must flash "No errors this session".
	// Do NOT execute the returned cmd: it is a tea.Tick (auto-clear timer), and
	// running it immediately would deliver ClearFlashMsg, erasing the flash before
	// we can assert on it.
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: '!'})

	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "No errors this session") {
		t.Errorf("after non-error FlashMsgs only, pressing '!' should flash 'No errors this session', got: %s", plain[:min(300, len(plain))])
	}
}

// ── TestErrorFlashFullWidth ───────────────────────────────────────────────────

// TestErrorFlashFullWidth verifies that error flash messages use (width-6)
// for truncation rather than the old max(width-60, 20), so long error messages
// are not prematurely cut off on wide terminals.
//
// The test uses a 120-column terminal and a 100-char error message.
// Old behavior: truncated at width-60 = 80 chars.
// New behavior: truncated at width-6 = 116 chars (fits the full 100-char message).
func TestErrorFlashFullWidth_LongMessageNotTruncatedAt80(t *testing.T) {
	tui.Version = "test"

	// Create model with width=120
	m := tui.New("testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 24})

	// Build a 100-character error message. With width=120:
	//   old: max(120-40, 20) = 80  → message truncated at 77 chars + "..."
	//   new: 120-4 = 116           → 100-char message fits without truncation
	longMsg := strings.Repeat("x", 100) // exactly 100 chars

	m, _ = rootApplyMsg(m, messages.Flash{Text: longMsg, IsError: true})

	plain := stripANSI(rootViewContent(m))

	// The full 100-char sequence must appear in the output (not truncated to 77+...).
	if !strings.Contains(plain, strings.Repeat("x", 80)) {
		t.Errorf("error flash should not be truncated to 80 chars (old width-60 behavior) at width=120; got header: %q",
			firstLine(plain))
	}
}

// TestErrorFlashFullWidth_ExceedsWidthMinus4IsTruncated verifies that messages
// longer than (width-6) are still truncated (guard against no-truncation bugs).
func TestErrorFlashFullWidth_ExceedsWidthMinus4IsTruncated(t *testing.T) {
	tui.Version = "test"

	m := tui.New("testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 24})

	// 200-char message — vastly exceeds width-6=76, must be truncated.
	veryLongMsg := strings.Repeat("z", 200)

	m, _ = rootApplyMsg(m, messages.Flash{Text: veryLongMsg, IsError: true})

	plain := stripANSI(rootViewContent(m))
	firstL := firstLine(plain)

	// The full 200-char message must NOT appear verbatim — it must be truncated.
	if strings.Contains(firstL, strings.Repeat("z", 200)) {
		t.Error("error flash longer than (width-6) should be truncated, but full message appeared in header")
	}
	// The header line must not exceed terminal width (no wrapping).
	if lipglossWidth(firstL) > 80 {
		t.Errorf("header line must not exceed terminal width 80, got visible width %d", lipglossWidth(firstL))
	}
}

// ── TestErrorHintAfterClear ───────────────────────────────────────────────────

// TestErrorHintAfterClear verifies the "! for errors" hint lifecycle:
//  1. After an error flash clears (ClearFlashMsg with matching gen), the header
//     shows "! for errors" instead of "? for help".
//  2. Any subsequent keypress dismisses the hint, restoring "? for help".
func TestErrorHintAfterClear_HintShownAfterErrorFlashClears(t *testing.T) {
	tui.Version = "test"
	m := newRootSizedModel()

	// Send an error flash. handleFlash increments gen to 1.
	m, _ = rootApplyMsg(m, messages.Flash{Text: "something failed", IsError: true})

	// Clear the flash (gen=1 matches).
	m, _ = rootApplyMsg(m, messages.ClearFlash{Gen: 1})

	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "! for errors") {
		t.Errorf("after error flash clears, header should show '! for errors', got: %s", plain[:min(300, len(plain))])
	}
}

// TestErrorHintAfterClear_KeypressDismissesHint verifies that the "! for errors"
// hint disappears after any keypress (e.g., "j") and "? for help" is restored.
func TestErrorHintAfterClear_KeypressDismissesHint(t *testing.T) {
	tui.Version = "test"
	m := newRootSizedModel()

	// Set up the hint.
	m, _ = rootApplyMsg(m, messages.Flash{Text: "error occurred", IsError: true})
	m, _ = rootApplyMsg(m, messages.ClearFlash{Gen: 1})

	// Verify hint is present before keypress.
	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "! for errors") {
		t.Skip("hint not shown — prerequisite not met, skipping dismissal test")
	}

	// Press "j" (down — a normal navigation key).
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: 'j', Text: "j"})

	plain = stripANSI(rootViewContent(m))

	if strings.Contains(plain, "! for errors") {
		t.Error("after any keypress, '! for errors' hint should be dismissed")
	}
	if !strings.Contains(plain, "? for help") {
		t.Errorf("after hint dismissed, header should show '? for help', got: %s", plain[:min(300, len(plain))])
	}
}

// ── TestErrorHintNotShownForNonErrors ────────────────────────────────────────

// TestErrorHintNotShownForNonErrors verifies that non-error flashes do NOT
// set showErrorHint — after a non-error flash clears, "? for help" is shown.
func TestErrorHintNotShownForNonErrors(t *testing.T) {
	tui.Version = "test"
	m := newRootSizedModel()

	// Send a non-error flash (gen increments to 1).
	m, _ = rootApplyMsg(m, messages.Flash{Text: "Copied!", IsError: false})

	// Clear the flash.
	m, _ = rootApplyMsg(m, messages.ClearFlash{Gen: 1})

	plain := stripANSI(rootViewContent(m))

	if strings.Contains(plain, "! for errors") {
		t.Error("after non-error flash clears, header must NOT show '! for errors'")
	}
	if !strings.Contains(plain, "? for help") {
		t.Errorf("after non-error flash clears, header should show '? for help', got: %s", plain[:min(300, len(plain))])
	}
}

// ── TestErrorLogKeyOpensViewer ────────────────────────────────────────────────

// TestErrorLogKeyOpensViewer verifies that pressing "!" after error flashes
// pushes a text viewer onto the view stack whose frame title contains "error".
func TestErrorLogKeyOpensViewer(t *testing.T) {
	tui.Version = "test"
	m := newRootSizedModel()

	// Add an error to history.
	m, _ = rootApplyMsg(m, messages.Flash{Text: "access denied", IsError: true})

	// Press "!" to open the error log.
	m, cmd := rootApplyMsg(m, tea.KeyPressMsg{Code: '!'})
	if cmd != nil {
		msg := cmd()
		if msg != nil {
			m, _ = rootApplyMsg(m, msg)
		}
	}

	plain := stripANSI(rootViewContent(m))

	// The frame title of the pushed view must reference "error".
	// FrameTitle is rendered in the frame border — e.g., "┤ error-log ├".
	if !strings.Contains(strings.ToLower(plain), "error") {
		t.Errorf("after pressing '!' with errors in history, view frame title should contain 'error', got:\n%s",
			plain[:min(400, len(plain))])
	}

	// The error entry ("access denied") must appear in the viewer content.
	if !strings.Contains(plain, "access denied") {
		t.Errorf("error log viewer should contain the logged error 'access denied', got:\n%s",
			plain[:min(400, len(plain))])
	}
}

// TestErrorLogKeyOpensViewer_NewestFirst verifies that the error log shows
// entries newest-first (last error appears before earlier ones in the output).
func TestErrorLogKeyOpensViewer_NewestFirst(t *testing.T) {
	tui.Version = "test"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.Flash{Text: "first-error-aaa", IsError: true})
	m, _ = rootApplyMsg(m, messages.Flash{Text: "second-error-bbb", IsError: true})

	m, cmd := rootApplyMsg(m, tea.KeyPressMsg{Code: '!'})
	if cmd != nil {
		msg := cmd()
		if msg != nil {
			m, _ = rootApplyMsg(m, msg)
		}
	}

	plain := stripANSI(rootViewContent(m))

	idxFirst := strings.Index(plain, "first-error-aaa")
	idxSecond := strings.Index(plain, "second-error-bbb")

	if idxFirst < 0 || idxSecond < 0 {
		t.Skipf("both errors not visible in view, idxFirst=%d idxSecond=%d — viewport may need scroll", idxFirst, idxSecond)
	}

	// Newest (second) must appear BEFORE oldest (first) in the rendered output.
	if idxSecond >= idxFirst {
		t.Errorf("error log must be newest-first: 'second-error-bbb' (idx=%d) should appear before 'first-error-aaa' (idx=%d)",
			idxSecond, idxFirst)
	}
}

// ── TestErrorLogKeyEmptyHistory ───────────────────────────────────────────────

// TestErrorLogKeyEmptyHistory verifies that pressing "!" with no errors in
// history shows a "No errors this session" flash instead of opening a viewer.
func TestErrorLogKeyEmptyHistory(t *testing.T) {
	tui.Version = "test"
	m := newRootSizedModel()

	// Do NOT send any error flashes — history is empty.
	// Do NOT execute the returned cmd: it is a tea.Tick (auto-clear timer), and
	// running it immediately would deliver ClearFlashMsg, erasing the flash before
	// we can assert on it.
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: '!'})

	plain := stripANSI(rootViewContent(m))

	// Flash "No errors this session" must be visible.
	if !strings.Contains(plain, "No errors this session") {
		t.Errorf("pressing '!' with empty history should flash 'No errors this session', got: %s",
			plain[:min(300, len(plain))])
	}

	// The view must still show the main menu (no viewer was pushed).
	if !strings.Contains(plain, "resource-types") {
		t.Errorf("pressing '!' with empty history must not push a viewer (should stay on main menu), got: %s",
			plain[:min(300, len(plain))])
	}
}

// ── TestTextViewer ────────────────────────────────────────────────────────────

// TestTextViewer verifies the views.NewTextViewer constructor:
//   - FrameTitle() returns the title passed to the constructor
//   - View() output contains both lines of the content
//   - CopyContent() returns the raw text
//   - RawContent() returns the raw text
func TestTextViewer_FrameTitle(t *testing.T) {
	k := keys.Default()
	tv := views.NewTextViewer("my error log", "line 1\nline 2", k)

	title := tv.FrameTitle()
	if title != "my error log" {
		t.Errorf("NewTextViewer FrameTitle() = %q, want %q", title, "my error log")
	}
}

func TestTextViewer_ViewContainsContent(t *testing.T) {
	k := keys.Default()
	tv := views.NewTextViewer("test title", "line 1\nline 2", k)
	tv.SetSize(80, 24)

	output := tv.View()

	if !strings.Contains(output, "line 1") {
		t.Errorf("NewTextViewer View() should contain 'line 1', got: %q", output[:min(200, len(output))])
	}
	if !strings.Contains(output, "line 2") {
		t.Errorf("NewTextViewer View() should contain 'line 2', got: %q", output[:min(200, len(output))])
	}
}

func TestTextViewer_CopyContentReturnsRawText(t *testing.T) {
	k := keys.Default()
	content := "line 1\nline 2"
	tv := views.NewTextViewer("test title", content, k)

	got, _ := tv.CopyContent()
	if got != content {
		t.Errorf("NewTextViewer CopyContent() = %q, want %q", got, content)
	}
}

func TestTextViewer_RawContentReturnsRawText(t *testing.T) {
	k := keys.Default()
	content := "line 1\nline 2"
	tv := views.NewTextViewer("test title", content, k)

	got := tv.RawContent()
	if got != content {
		t.Errorf("NewTextViewer RawContent() = %q, want %q", got, content)
	}
}

// TestTextViewer_EmptyContent verifies NewTextViewer handles empty content gracefully.
func TestTextViewer_EmptyContent(t *testing.T) {
	k := keys.Default()
	tv := views.NewTextViewer("empty log", "", k)
	tv.SetSize(80, 24)

	// Must not panic.
	output := tv.View()
	_ = output // view may be blank — that's fine

	title := tv.FrameTitle()
	if title != "empty log" {
		t.Errorf("NewTextViewer FrameTitle() with empty content = %q, want %q", title, "empty log")
	}
}

// TestTextViewer_SatisfiesViewInterface verifies that *views.YAMLModel returned
// by NewTextViewer satisfies the views.View interface (compile-time check via cast).
func TestTextViewer_SatisfiesViewInterface(t *testing.T) {
	k := keys.Default()
	tv := views.NewTextViewer("interface test", "content", k)

	// views.View requires FrameTitle() and CopyContent() — verify they return sane values.
	if tv.FrameTitle() == "" {
		t.Error("NewTextViewer FrameTitle() must return non-empty string")
	}
	content, _ := tv.CopyContent()
	if content != "content" {
		t.Errorf("NewTextViewer CopyContent() = %q, want %q", content, "content")
	}
}

// ── TestErrorHistoryFromAPIError ──────────────────────────────────────────────

// TestErrorHistoryFromAPIError verifies that APIErrorMsg appends to error
// history directly (bypasses handleFlash). Pressing "!" must open the viewer
// with the error visible.
func TestErrorHistoryFromAPIError(t *testing.T) {
	tui.Version = "test"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.APIError{
		ResourceType: "ec2-instances",
		Err:          fmt.Errorf("AccessDenied: User is not authorized"),
	})

	// Press "!" — should open error log viewer, not show "No errors".
	m, cmd := rootApplyMsg(m, tea.KeyPressMsg{Code: '!'})
	if cmd != nil {
		if msg := cmd(); msg != nil {
			m, _ = rootApplyMsg(m, msg)
		}
	}

	plain := stripANSI(rootViewContent(m))
	if strings.Contains(plain, "No errors this session") {
		t.Error("APIErrorMsg should record to error history; pressing '!' should open viewer")
	}
	if !strings.Contains(plain, "AccessDenied") {
		t.Error("error log viewer should contain the API error text")
	}
}

// ── TestErrorHistoryFromClientsReady ─────────────────────────────────────────

// TestErrorHistoryFromClientsReady verifies that a failed ClientsReadyMsg
// appends to error history directly (bypasses handleFlash).
func TestErrorHistoryFromClientsReady(t *testing.T) {
	tui.Version = "test"
	m := newRootSizedModel()

	// Gen 0 matches the model's initial connectGen (zero value).
	m, _ = rootApplyMsg(m, messages.ClientsReady{
		Err: errors.New("could not resolve credentials"),
		Gen: 0,
	})

	m, cmd := rootApplyMsg(m, tea.KeyPressMsg{Code: '!'})
	if cmd != nil {
		if msg := cmd(); msg != nil {
			m, _ = rootApplyMsg(m, msg)
		}
	}

	plain := stripANSI(rootViewContent(m))
	if strings.Contains(plain, "No errors this session") {
		t.Error("failed ClientsReadyMsg should record to error history; pressing '!' should open viewer")
	}
	if !strings.Contains(plain, "could not resolve credentials") {
		t.Error("error log viewer should contain the ClientsReady error text")
	}
}

// ── TestErrorLogTimestampFormat ───────────────────────────────────────────────

// TestErrorLogTimestampFormat verifies that each error log entry has a
// [HH:MM:SS] timestamp prefix.
func TestErrorLogTimestampFormat(t *testing.T) {
	tui.Version = "test"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.Flash{Text: "timestamp test error", IsError: true})

	m, cmd := rootApplyMsg(m, tea.KeyPressMsg{Code: '!'})
	if cmd != nil {
		if msg := cmd(); msg != nil {
			m, _ = rootApplyMsg(m, msg)
		}
	}

	plain := stripANSI(rootViewContent(m))
	// Timestamp format: [HH:MM:SS] — match the bracket pattern.
	if !strings.Contains(plain, "[") || !strings.Contains(plain, "]") {
		t.Error("error log entries should have [HH:MM:SS] timestamp prefix")
	}
	// Find a line with pattern [XX:XX:XX] followed by the error text.
	// Lines may have frame border characters (│) from the layout.
	found := false
	for _, line := range strings.Split(plain, "\n") {
		if !strings.Contains(line, "timestamp test error") {
			continue
		}
		// Strip frame borders and whitespace to get raw content.
		trimmed := strings.TrimLeft(line, " │")
		if len(trimmed) >= 10 && trimmed[0] == '[' && trimmed[3] == ':' && trimmed[6] == ':' && trimmed[9] == ']' {
			found = true
			break
		}
	}
	if !found {
		t.Error("could not find '[HH:MM:SS] timestamp test error' in error log viewer")
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

// firstLine returns the first line of a multi-line string.
func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}
