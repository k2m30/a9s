package unit

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/internal/tui/keys"
	"github.com/k2m30/a9s/internal/tui/messages"
	"github.com/k2m30/a9s/internal/tui/views"
)

// ═══════════════════════════════════════════════════════════════════════════
// SelectorModel tests — unified profile/region selector
// ═══════════════════════════════════════════════════════════════════════════

func selectorKeyPress(char string) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: -1, Text: char}
}

func selectorSpecialKey(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code}
}

// ── NewProfile constructor wrapper ──────────────────────────────────────────

func TestSelector_NewProfileReturnsSelector(t *testing.T) {
	k := keys.Default()
	m := views.NewProfile([]string{"default", "staging"}, "default", k)
	// Should be a SelectorModel
	if m.Title() != "aws-profiles" {
		t.Errorf("NewProfile Title() = %q, want %q", m.Title(), "aws-profiles")
	}
}

func TestSelector_NewProfileEnterReturnsProfileSelectedMsg(t *testing.T) {
	k := keys.Default()
	m := views.NewProfile([]string{"default", "staging"}, "default", k)
	m.SetSize(80, 20)

	_, cmd := m.Update(selectorSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("Enter should produce a command")
	}
	msg := cmd()
	psm, ok := msg.(messages.ProfileSelectedMsg)
	if !ok {
		t.Fatalf("expected ProfileSelectedMsg, got %T", msg)
	}
	if psm.Profile != "default" {
		t.Errorf("expected 'default', got %s", psm.Profile)
	}
}

// ── NewRegion constructor wrapper ───────────────────────────────────────────

func TestSelector_NewRegionReturnsSelector(t *testing.T) {
	k := keys.Default()
	m := views.NewRegion([]string{"us-east-1", "eu-west-1"}, "us-east-1", k)
	if m.Title() != "aws-regions" {
		t.Errorf("NewRegion Title() = %q, want %q", m.Title(), "aws-regions")
	}
}

func TestSelector_NewRegionEnterReturnsRegionSelectedMsg(t *testing.T) {
	k := keys.Default()
	m := views.NewRegion([]string{"us-east-1", "eu-west-1"}, "us-east-1", k)
	m.SetSize(80, 20)

	_, cmd := m.Update(selectorSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("Enter should produce a command")
	}
	msg := cmd()
	rsm, ok := msg.(messages.RegionSelectedMsg)
	if !ok {
		t.Fatalf("expected RegionSelectedMsg, got %T", msg)
	}
	if rsm.Region != "us-east-1" {
		t.Errorf("expected 'us-east-1', got %s", rsm.Region)
	}
}

// ── Title() method ──────────────────────────────────────────────────────────

func TestSelector_Title(t *testing.T) {
	k := keys.Default()
	m := views.NewSelector([]string{"a", "b"}, "a", "custom-title", func(s string) tea.Msg {
		return nil
	}, k)
	if m.Title() != "custom-title" {
		t.Errorf("Title() = %q, want %q", m.Title(), "custom-title")
	}
}

// ── Navigation: Up/Down ─────────────────────────────────────────────────────

func TestSelector_DownMovesSelection(t *testing.T) {
	k := keys.Default()
	items := []string{"item-1", "item-2", "item-3"}
	var selected string
	m := views.NewSelector(items, "item-1", "test", func(s string) tea.Msg {
		selected = s
		return messages.ProfileSelectedMsg{Profile: s}
	}, k)
	m.SetSize(80, 20)

	// Move down to item-2
	m, _ = m.Update(selectorKeyPress("j"))
	m, cmd := m.Update(selectorSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("Enter should produce a command")
	}
	cmd()
	if selected != "item-2" {
		t.Errorf("after j, expected item-2, got %s", selected)
	}
}

func TestSelector_UpMovesSelection(t *testing.T) {
	k := keys.Default()
	items := []string{"item-1", "item-2", "item-3"}
	var selected string
	m := views.NewSelector(items, "item-1", "test", func(s string) tea.Msg {
		selected = s
		return messages.ProfileSelectedMsg{Profile: s}
	}, k)
	m.SetSize(80, 20)

	// Move down twice, then up once
	m, _ = m.Update(selectorKeyPress("j"))
	m, _ = m.Update(selectorKeyPress("j"))
	m, _ = m.Update(selectorKeyPress("k"))
	m, cmd := m.Update(selectorSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("Enter should produce a command")
	}
	cmd()
	if selected != "item-2" {
		t.Errorf("after j j k, expected item-2, got %s", selected)
	}
}

// ── Navigation: Top/Bottom (g/G) — NEW in SelectorModel ────────────────────

func TestSelector_GGoesToTop(t *testing.T) {
	k := keys.Default()
	items := []string{"item-1", "item-2", "item-3"}
	var selected string
	m := views.NewSelector(items, "item-1", "test", func(s string) tea.Msg {
		selected = s
		return messages.ProfileSelectedMsg{Profile: s}
	}, k)
	m.SetSize(80, 20)

	// Move down to item-3
	m, _ = m.Update(selectorKeyPress("j"))
	m, _ = m.Update(selectorKeyPress("j"))
	// Now press g to go to top
	m, _ = m.Update(selectorKeyPress("g"))
	m, cmd := m.Update(selectorSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("Enter should produce a command")
	}
	cmd()
	if selected != "item-1" {
		t.Errorf("after j j g, expected item-1 (top), got %s", selected)
	}
}

func TestSelector_ShiftGGoesToBottom(t *testing.T) {
	k := keys.Default()
	items := []string{"item-1", "item-2", "item-3"}
	var selected string
	m := views.NewSelector(items, "item-1", "test", func(s string) tea.Msg {
		selected = s
		return messages.ProfileSelectedMsg{Profile: s}
	}, k)
	m.SetSize(80, 20)

	// Press G to go to bottom
	m, _ = m.Update(selectorKeyPress("G"))
	m, cmd := m.Update(selectorSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("Enter should produce a command")
	}
	cmd()
	if selected != "item-3" {
		t.Errorf("after G, expected item-3 (bottom), got %s", selected)
	}
}

// ── Navigation: PageUp/PageDown ─────────────────────────────────────────────

func TestSelector_PageDownMovesCursor(t *testing.T) {
	k := keys.Default()
	items := make([]string, 30)
	for i := range items {
		items[i] = "item-" + string(rune('a'+i%26))
	}
	var selected string
	m := views.NewSelector(items, "", "test", func(s string) tea.Msg {
		selected = s
		return messages.ProfileSelectedMsg{Profile: s}
	}, k)
	m.SetSize(80, 10) // small height

	m, _ = m.Update(selectorSpecialKey(tea.KeyPgDown))
	m, cmd := m.Update(selectorSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("Enter should produce a command")
	}
	cmd()
	// After page down, cursor should have moved past 0
	if selected == items[0] {
		t.Error("after PageDown, cursor should have moved past first item")
	}
}

func TestSelector_PageUpMovesCursor(t *testing.T) {
	k := keys.Default()
	items := make([]string, 30)
	for i := range items {
		items[i] = "item-" + string(rune('a'+i%26))
	}
	var selected string
	m := views.NewSelector(items, "", "test", func(s string) tea.Msg {
		selected = s
		return messages.ProfileSelectedMsg{Profile: s}
	}, k)
	m.SetSize(80, 10)

	// Go to bottom, then page up
	m, _ = m.Update(selectorKeyPress("G"))
	m, _ = m.Update(selectorSpecialKey(tea.KeyPgUp))
	m, cmd := m.Update(selectorSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("Enter should produce a command")
	}
	cmd()
	// Should not be at bottom after page up
	if selected == items[len(items)-1] {
		t.Error("after G then PageUp, cursor should not be at bottom")
	}
}

// ── Cursor boundaries ───────────────────────────────────────────────────────

func TestSelector_CursorStopsAtTop(t *testing.T) {
	k := keys.Default()
	items := []string{"a", "b", "c"}
	var selected string
	m := views.NewSelector(items, "", "test", func(s string) tea.Msg {
		selected = s
		return messages.ProfileSelectedMsg{Profile: s}
	}, k)
	m.SetSize(80, 20)

	m, _ = m.Update(selectorKeyPress("k"))
	m, _ = m.Update(selectorKeyPress("k"))
	m, cmd := m.Update(selectorSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("Enter should produce a command")
	}
	cmd()
	if selected != "a" {
		t.Errorf("cursor should stop at top, expected 'a', got %s", selected)
	}
}

func TestSelector_CursorStopsAtBottom(t *testing.T) {
	k := keys.Default()
	items := []string{"a", "b", "c"}
	var selected string
	m := views.NewSelector(items, "", "test", func(s string) tea.Msg {
		selected = s
		return messages.ProfileSelectedMsg{Profile: s}
	}, k)
	m.SetSize(80, 20)

	m, _ = m.Update(selectorKeyPress("j"))
	m, _ = m.Update(selectorKeyPress("j"))
	m, _ = m.Update(selectorKeyPress("j"))
	m, _ = m.Update(selectorKeyPress("j"))
	m, cmd := m.Update(selectorSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("Enter should produce a command")
	}
	cmd()
	if selected != "c" {
		t.Errorf("cursor should stop at bottom, expected 'c', got %s", selected)
	}
}

// ── FrameTitle ──────────────────────────────────────────────────────────────

func TestSelector_FrameTitleShowsCount(t *testing.T) {
	k := keys.Default()
	m := views.NewProfile([]string{"a", "b", "c"}, "a", k)
	if m.FrameTitle() != "aws-profiles(3)" {
		t.Errorf("FrameTitle() = %q, want %q", m.FrameTitle(), "aws-profiles(3)")
	}
}

func TestSelector_FrameTitleShowsFilteredCount(t *testing.T) {
	k := keys.Default()
	m := views.NewProfile([]string{"alpha", "beta", "gamma"}, "alpha", k)
	m.SetFilter("al")
	title := m.FrameTitle()
	if title != "aws-profiles(1/3)" {
		t.Errorf("FrameTitle() = %q, want %q", title, "aws-profiles(1/3)")
	}
}

func TestSelector_RegionFrameTitle(t *testing.T) {
	k := keys.Default()
	m := views.NewRegion([]string{"us-east-1", "eu-west-1"}, "us-east-1", k)
	if m.FrameTitle() != "aws-regions(2)" {
		t.Errorf("FrameTitle() = %q, want %q", m.FrameTitle(), "aws-regions(2)")
	}
}

// ── View rendering ──────────────────────────────────────────────────────────

func TestSelector_ViewShowsCurrentMarker(t *testing.T) {
	k := keys.Default()
	m := views.NewProfile([]string{"default", "staging", "prod"}, "staging", k)
	m.SetSize(80, 20)
	view := m.View()
	plain := stripANSI(view)
	if !strings.Contains(plain, "(current)") {
		t.Error("view should show (current) marker for active item")
	}
	// The (current) marker should be on the staging line
	lines := strings.Split(plain, "\n")
	found := false
	for _, line := range lines {
		if strings.Contains(line, "staging") && strings.Contains(line, "(current)") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("staging line should have (current) marker, got: %s", plain)
	}
}

func TestSelector_ViewShowsAllItems(t *testing.T) {
	k := keys.Default()
	items := []string{"item-1", "item-2", "item-3"}
	m := views.NewSelector(items, "", "test", func(s string) tea.Msg { return nil }, k)
	m.SetSize(80, 20)
	view := m.View()
	for _, item := range items {
		if !strings.Contains(view, item) {
			t.Errorf("view should contain %q", item)
		}
	}
}

func TestSelector_ViewEmptyItems(t *testing.T) {
	k := keys.Default()
	m := views.NewProfile([]string{}, "", k)
	m.SetSize(80, 20)
	view := m.View()
	if !strings.Contains(view, "No items available") {
		t.Errorf("empty selector should show 'No items available', got: %s", view)
	}
}

// ── Filter ──────────────────────────────────────────────────────────────────

func TestSelector_SetFilterFiltersItems(t *testing.T) {
	k := keys.Default()
	m := views.NewProfile([]string{"alpha", "beta", "gamma"}, "alpha", k)
	m.SetSize(80, 20)
	m.SetFilter("be")
	view := m.View()
	plain := stripANSI(view)
	if !strings.Contains(plain, "beta") {
		t.Error("filtered view should contain 'beta'")
	}
	if strings.Contains(plain, "alpha") {
		t.Error("filtered view should NOT contain 'alpha'")
	}
	if strings.Contains(plain, "gamma") {
		t.Error("filtered view should NOT contain 'gamma'")
	}
}

func TestSelector_SetFilterResetsCursor(t *testing.T) {
	k := keys.Default()
	items := []string{"alpha", "beta", "gamma"}
	var selected string
	m := views.NewSelector(items, "", "test", func(s string) tea.Msg {
		selected = s
		return messages.ProfileSelectedMsg{Profile: s}
	}, k)
	m.SetSize(80, 20)

	// Move cursor down
	m, _ = m.Update(selectorKeyPress("j"))
	// Apply filter
	m.SetFilter("ga")
	// Enter should select filtered item at cursor 0
	m, cmd := m.Update(selectorSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("Enter should produce a command")
	}
	cmd()
	if selected != "gamma" {
		t.Errorf("after filter 'ga', Enter should select 'gamma', got %s", selected)
	}
}

func TestSelector_GetFilter(t *testing.T) {
	k := keys.Default()
	m := views.NewProfile([]string{"a", "b"}, "a", k)
	m.SetFilter("test-filter")
	if m.GetFilter() != "test-filter" {
		t.Errorf("GetFilter() = %q, want %q", m.GetFilter(), "test-filter")
	}
}

func TestSelector_ClearFilter(t *testing.T) {
	k := keys.Default()
	m := views.NewProfile([]string{"alpha", "beta"}, "alpha", k)
	m.SetSize(80, 20)
	m.SetFilter("be")
	m.SetFilter("")
	view := m.View()
	plain := stripANSI(view)
	if !strings.Contains(plain, "alpha") {
		t.Error("after clearing filter, view should contain 'alpha'")
	}
	if !strings.Contains(plain, "beta") {
		t.Error("after clearing filter, view should contain 'beta'")
	}
}

// ── CopyContent ─────────────────────────────────────────────────────────────

func TestSelector_CopyContentReturnsEmpty(t *testing.T) {
	k := keys.Default()
	m := views.NewProfile([]string{"a"}, "a", k)
	content, label := m.CopyContent()
	if content != "" || label != "" {
		t.Errorf("CopyContent() should return empty, got %q, %q", content, label)
	}
}

// ── GetHelpContext ───────────────────────────────────────────────────────────

func TestSelector_GetHelpContextReturnsSelector(t *testing.T) {
	k := keys.Default()
	m := views.NewProfile([]string{"a"}, "a", k)
	if m.GetHelpContext() != views.HelpFromSelector {
		t.Errorf("GetHelpContext() should return HelpFromSelector, got %v", m.GetHelpContext())
	}
}

// ── Init ────────────────────────────────────────────────────────────────────

func TestSelector_InitReturnsNilCmd(t *testing.T) {
	k := keys.Default()
	m := views.NewProfile([]string{"a"}, "a", k)
	m2, cmd := m.Init()
	if cmd != nil {
		t.Error("Init() should return nil cmd")
	}
	if m2.Title() != "aws-profiles" {
		t.Error("Init() should return same model")
	}
}

// ── Unhandled keys ──────────────────────────────────────────────────────────

func TestSelector_UnhandledKeyReturnsNilCmd(t *testing.T) {
	k := keys.Default()
	m := views.NewProfile([]string{"a"}, "a", k)
	_, cmd := m.Update(selectorKeyPress("x"))
	if cmd != nil {
		t.Error("unhandled key 'x' should return nil cmd")
	}
}

func TestSelector_NonKeyMsgPassthrough(t *testing.T) {
	k := keys.Default()
	m := views.NewProfile([]string{"a", "b"}, "a", k)
	m2, cmd := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	if cmd != nil {
		t.Error("WindowSizeMsg should return nil cmd")
	}
	if m2.FrameTitle() != "aws-profiles(2)" {
		t.Errorf("model should be unchanged after non-key msg, got %q", m2.FrameTitle())
	}
}

// ── Compile-time interface checks ───────────────────────────────────────────

var _ views.View = (*views.SelectorModel)(nil)
var _ views.Filterable = (*views.SelectorModel)(nil)
