// tui_selector_test.go — live SelectorModel.Update() coverage: cursor
// movement (Up/Down/Top/Bottom/PageUp/PageDown) and boundary clamping.
//
// NewSelector/NewProfile/NewRegion/NewTheme, View, FrameTitle, Title,
// SetFilter, GetFilter, CopyContent, GetHelpContext, and Init are DEAD per
// specs/022-codebase-cleanup/wave3-map-text.md (selector.go: "LIVE:
// NewSelectorWithCtrl, NewTransientSelector, Update, SetSize, RenderSelector;
// DEAD: NewSelector, NewProfile, NewRegion, NewTheme, Init, View, FrameTitle,
// CopyContent, GetHelpContext, Title, SetFilter, GetFilter"). The live
// equivalents for what those dead methods pinned:
//   - rendering shape (view-shows-current-marker/all-items/empty, filtered
//     items): selector_render_parity_test.go's TestSelectorRender_LiveSeam,
//     built on NewTransientSelector + app.SelectorBody.
//   - the real '/' filter mode end-to-end: qa_filtering_test.go's
//     TestQA_Filter_11_15/11_16_*SelectorFilterWorks.
//   - copy is a no-op on a selector screen: text_ports_test.go's
//     TestPort_SelectorCopy_IsNoOp.
//
// Update() itself IS live (app_stack.go's rsKindSelector case calls
// NewSelectorWithCtrl(m.ctrl, ...).Update(msg)), and it drives real
// app.Controller cursor actions (ActionMoveUp/Down/Top/Bottom/PageUp/
// PageDown) that nothing else in this suite exercises through a selector
// screen — kept here via NewSelectorWithCtrl on a throwaway Controller
// (mirroring exactly what the now-dead NewSelector did internally, minus
// the dead wrapper itself).
package unit

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

func selectorKeyPress(char string) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: -1, Text: char}
}

func selectorSpecialKey(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code}
}

// newLiveSelector builds a SelectorModel via the live NewSelectorWithCtrl
// seam, backed by a throwaway Controller seeded exactly like app_stack.go's
// pushSelectorScreen seeds the real one.
func newLiveSelector(items []string, activeItem, title string, onSelect func(string) tea.Msg, k keys.Map) views.SelectorModel {
	c := app.New(runtime.Bootstrap("", "", nil))
	c.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenProfileSelector}})
	c.EnsureSelectorState(items, activeItem, title)
	m := views.NewSelectorWithCtrl(c, onSelect, k)
	m.SetSize(80, 20)
	return m
}

// ── Navigation: Up/Down ─────────────────────────────────────────────────────

func TestSelector_DownMovesSelection(t *testing.T) {
	k := keys.Default()
	items := []string{"item-1", "item-2", "item-3"}
	var selected string
	m := newLiveSelector(items, "item-1", "test", func(s string) tea.Msg {
		selected = s
		return messages.ProfileSelected{Profile: s}
	}, k)

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
	m := newLiveSelector(items, "item-1", "test", func(s string) tea.Msg {
		selected = s
		return messages.ProfileSelected{Profile: s}
	}, k)

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

// ── Navigation: Top/Bottom (g/G) ────────────────────────────────────────────

func TestSelector_GGoesToTop(t *testing.T) {
	k := keys.Default()
	items := []string{"item-1", "item-2", "item-3"}
	var selected string
	m := newLiveSelector(items, "item-1", "test", func(s string) tea.Msg {
		selected = s
		return messages.ProfileSelected{Profile: s}
	}, k)

	m, _ = m.Update(selectorKeyPress("j"))
	m, _ = m.Update(selectorKeyPress("j"))
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
	m := newLiveSelector(items, "item-1", "test", func(s string) tea.Msg {
		selected = s
		return messages.ProfileSelected{Profile: s}
	}, k)

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
	m := newLiveSelector(items, "", "test", func(s string) tea.Msg {
		selected = s
		return messages.ProfileSelected{Profile: s}
	}, k)
	m.SetSize(80, 10) // small height

	m, _ = m.Update(selectorSpecialKey(tea.KeyPgDown))
	m, cmd := m.Update(selectorSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("Enter should produce a command")
	}
	cmd()
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
	m := newLiveSelector(items, "", "test", func(s string) tea.Msg {
		selected = s
		return messages.ProfileSelected{Profile: s}
	}, k)
	m.SetSize(80, 10)

	m, _ = m.Update(selectorKeyPress("G"))
	m, _ = m.Update(selectorSpecialKey(tea.KeyPgUp))
	m, cmd := m.Update(selectorSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("Enter should produce a command")
	}
	cmd()
	if selected == items[len(items)-1] {
		t.Error("after G then PageUp, cursor should not be at bottom")
	}
}

// ── Cursor boundaries ───────────────────────────────────────────────────────

func TestSelector_CursorStopsAtTop(t *testing.T) {
	k := keys.Default()
	items := []string{"a", "b", "c"}
	var selected string
	m := newLiveSelector(items, "", "test", func(s string) tea.Msg {
		selected = s
		return messages.ProfileSelected{Profile: s}
	}, k)

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
	m := newLiveSelector(items, "", "test", func(s string) tea.Msg {
		selected = s
		return messages.ProfileSelected{Profile: s}
	}, k)

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

// ── Unhandled keys ──────────────────────────────────────────────────────────

func TestSelector_UnhandledKeyReturnsNilCmd(t *testing.T) {
	k := keys.Default()
	m := newLiveSelector([]string{"a"}, "a", "test", func(s string) tea.Msg { return nil }, k)
	_, cmd := m.Update(selectorKeyPress("x"))
	if cmd != nil {
		t.Error("unhandled key 'x' should return nil cmd")
	}
}

func TestSelector_NonKeyMsgPassthrough(t *testing.T) {
	k := keys.Default()
	items := []string{"a", "b"}
	var selected string
	m := newLiveSelector(items, "a", "test", func(s string) tea.Msg {
		selected = s
		return messages.ProfileSelected{Profile: s}
	}, k)
	_, cmd := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	if cmd != nil {
		t.Error("WindowSizeMsg should return nil cmd")
	}
	// Model must still function normally after a non-key message.
	m, cmd = m.Update(selectorSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("Enter should still produce a command after a non-key msg")
	}
	cmd()
	if selected != "a" {
		t.Errorf("selection unaffected by non-key msg, expected 'a', got %s", selected)
	}
}
