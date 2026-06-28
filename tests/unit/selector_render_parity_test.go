// selector_render_parity_test.go — byte-parity gate for the selector flip.
//
// Asserts that SelectorModel.RenderSelector(body) produces output byte-identical
// to the legacy SelectorModel.View() for the same logical state — across the
// three selector kinds (profile, region, theme) and a set of scenarios per kind.
//
// Strategy:
//   - Legacy side: build a SelectorModel via NewProfile/NewRegion/NewTheme,
//     SetSize, drive cursor via Update(KeyPressMsg) and SetFilter — then call
//     m.View() to get the oracle string.
//   - Controller side: construct a SelectorBody that exactly mirrors the
//     model's filtered/cursor state (matching what buildSelectorBody would
//     produce for an equivalent SelectorState). Call m.RenderSelector(body)
//     on the SAME sized model.
//   - Assert got == legacy EXACTLY (byte-parity). Any difference is a bug in
//     RenderSelector and must be reported, not suppressed.
//
// Note on the controller stack: applyIntents(PushScreen) does not yet
// initialize SelectorState (that wiring lands in a later PR-C slice), so
// the controller Body.Selector path is not exercised here. The parity test
// directly compares View() with RenderSelector(body) on a shared model
// instance — which is the contract that gates the selector flip.
package unit_test

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/app"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// selectorParityKeyPress builds a KeyPressMsg for a single character.
func selectorParityKeyPress(char string) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: -1, Text: char}
}

// assertSelectorParity calls m.View() and m.RenderSelector(body) on the same
// model and fails with a line-by-line diff if the two strings differ.
// kind and scenario are used only for error context.
func assertSelectorParity(t *testing.T, m *views.SelectorModel, body app.SelectorBody, kind, scenario string) {
	t.Helper()
	legacy := m.View()
	got := m.RenderSelector(body)
	if got == legacy {
		return
	}
	legacyLines := strings.Split(legacy, "\n")
	gotLines := strings.Split(got, "\n")
	maxLines := len(legacyLines)
	if len(gotLines) > maxLines {
		maxLines = len(gotLines)
	}
	var diff strings.Builder
	diff.WriteString(fmt.Sprintf(
		"kind=%s scenario=%s — RenderSelector differs from View():\n  legacy lines=%d  RenderSelector lines=%d\n",
		kind, scenario, len(legacyLines), len(gotLines),
	))
	for i := 0; i < maxLines; i++ {
		legLine, gotLine := "", ""
		if i < len(legacyLines) {
			legLine = legacyLines[i]
		}
		if i < len(gotLines) {
			gotLine = gotLines[i]
		}
		if legLine != gotLine {
			diff.WriteString(fmt.Sprintf(
				"  line %d:\n    legacy:         %q\n    RenderSelector: %q\n",
				i+1, legLine, gotLine,
			))
		}
	}
	t.Errorf("byte-parity FAILED:\n%s", diff.String())
}

// bodyFromModel constructs the SelectorBody that should be byte-identical to
// what buildSelectorBody(SelectorState) would produce for the same logical
// state — mirroring the filtering and cursor-clamping logic in selector.go.
//
// items is the full unfiltered list; filterText is the active filter;
// cursor is the cursor index into the FILTERED list; activeItem is the
// item that receives the "(current)" indicator; title is the frame title.
func bodyFromModel(items []string, filterText, activeItem, title string, cursor int) app.SelectorBody {
	// Apply filter exactly as applyFilter does.
	var filtered []string
	if filterText == "" {
		filtered = items
	} else {
		q := strings.ToLower(filterText)
		for _, item := range items {
			if strings.Contains(strings.ToLower(item), q) {
				filtered = append(filtered, item)
			}
		}
	}
	// Clamp cursor exactly as buildSelectorBody does.
	if len(filtered) > 0 && cursor >= len(filtered) {
		cursor = len(filtered) - 1
	}
	if cursor < 0 {
		cursor = 0
	}
	return app.SelectorBody{
		Items:      filtered,
		Selected:   cursor,
		AllItems:   items,
		Filter:     filterText,
		ActiveItem: activeItem,
		Title:      title,
	}
}

// ---------------------------------------------------------------------------
// Selector kind descriptors
// ---------------------------------------------------------------------------

type selectorKind struct {
	name       string
	items      []string
	activeItem string
	title      string
	newModel   func(items []string, active string, k keys.Map) views.SelectorModel
}

// fakeProfiles/Regions/Themes are realistic but entirely synthetic —
// no real account data, profile names, or AWS resource IDs.
var fakeProfiles = []string{
	"dev-account",
	"staging-account",
	"prod-account",
	"ops-account",
	"sandbox-account",
}

var fakeRegions = []string{
	"us-east-1",
	"us-west-2",
	"eu-west-1",
	"eu-central-1",
	"ap-southeast-1",
	"ap-northeast-1",
}

var fakeThemes = []string{
	"tokyo-night-dark",
	"tokyo-night-light",
	"dracula",
	"catppuccin-mocha",
	"nord",
}

func selectorKinds() []selectorKind {
	return []selectorKind{
		{
			name:       "profile",
			items:      fakeProfiles,
			activeItem: "staging-account",
			title:      "aws-profiles",
			newModel:   views.NewProfile,
		},
		{
			name:       "region",
			items:      fakeRegions,
			activeItem: "eu-west-1",
			title:      "aws-regions",
			newModel:   views.NewRegion,
		},
		{
			name:       "theme",
			items:      fakeThemes,
			activeItem: "dracula",
			title:      "themes",
			newModel:   views.NewTheme,
		},
	}
}

// ---------------------------------------------------------------------------
// Top-level parity test
// ---------------------------------------------------------------------------

// TestSelectorRenderParity is the byte-parity gate for RenderSelector.
// Each subtest builds the same logical state on the legacy (View) and
// controller (RenderSelector) sides and asserts identical output.
func TestSelectorRenderParity(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	styles.Reinit()
	t.Cleanup(styles.Reinit)

	for _, kind := range selectorKinds() {
		kind := kind
		t.Run(kind.name, func(t *testing.T) {
			runSelectorParityScenarios(t, kind)
		})
	}
}

func runSelectorParityScenarios(t *testing.T, kind selectorKind) {
	k := keys.Default()

	// -------------------------------------------------------------------------
	// S1: Default state — no filter, cursor at 0.
	// -------------------------------------------------------------------------
	t.Run("S1_Default", func(t *testing.T) {
		m := kind.newModel(kind.items, kind.activeItem, k)
		m.SetSize(80, 24)

		body := bodyFromModel(kind.items, "", kind.activeItem, kind.title, 0)
		assertSelectorParity(t, &m, body, kind.name, "S1_Default")
	})

	// -------------------------------------------------------------------------
	// S2: Filter active narrowing the list.
	// -------------------------------------------------------------------------
	t.Run("S2_FilterActive", func(t *testing.T) {
		m := kind.newModel(kind.items, kind.activeItem, k)
		m.SetSize(80, 24)

		// Use a prefix that matches at least one item in every kind:
		// profiles → "dev", regions → "us", themes → "tokyo"
		filterMap := map[string]string{
			"profile": "ac",  // matches *-account items
			"region":  "us",  // matches us-east-1 and us-west-2
			"theme":   "tok", // matches tokyo-night-*
		}
		filter := filterMap[kind.name]

		m.SetFilter(filter)
		// cursor stays at 0 after SetFilter (SetFilter resets cursor).
		body := bodyFromModel(kind.items, filter, kind.activeItem, kind.title, 0)
		assertSelectorParity(t, &m, body, kind.name, "S2_FilterActive")
	})

	// -------------------------------------------------------------------------
	// S3: Filter matching nothing.
	// -------------------------------------------------------------------------
	t.Run("S3_FilterNoMatch", func(t *testing.T) {
		m := kind.newModel(kind.items, kind.activeItem, k)
		m.SetSize(80, 24)

		m.SetFilter("zzznomatch")
		body := bodyFromModel(kind.items, "zzznomatch", kind.activeItem, kind.title, 0)
		assertSelectorParity(t, &m, body, kind.name, "S3_FilterNoMatch")
	})

	// -------------------------------------------------------------------------
	// S4: Cursor on first item (explicit — same as default but explicit move).
	// -------------------------------------------------------------------------
	t.Run("S4_CursorFirst", func(t *testing.T) {
		m := kind.newModel(kind.items, kind.activeItem, k)
		m.SetSize(80, 24)

		// Move down once then back to top via 'g'.
		m, _ = m.Update(selectorParityKeyPress("j"))
		m, _ = m.Update(selectorParityKeyPress("g"))
		// cursor is now at 0.
		body := bodyFromModel(kind.items, "", kind.activeItem, kind.title, 0)
		assertSelectorParity(t, &m, body, kind.name, "S4_CursorFirst")
	})

	// -------------------------------------------------------------------------
	// S5: Cursor on middle item.
	// -------------------------------------------------------------------------
	t.Run("S5_CursorMiddle", func(t *testing.T) {
		m := kind.newModel(kind.items, kind.activeItem, k)
		m.SetSize(80, 24)

		mid := len(kind.items) / 2
		for i := 0; i < mid; i++ {
			m, _ = m.Update(selectorParityKeyPress("j"))
		}
		body := bodyFromModel(kind.items, "", kind.activeItem, kind.title, mid)
		assertSelectorParity(t, &m, body, kind.name, "S5_CursorMiddle")
	})

	// -------------------------------------------------------------------------
	// S6: Cursor on last item.
	// -------------------------------------------------------------------------
	t.Run("S6_CursorLast", func(t *testing.T) {
		m := kind.newModel(kind.items, kind.activeItem, k)
		m.SetSize(80, 24)

		last := len(kind.items) - 1
		// Press 'G' to jump to bottom.
		m, _ = m.Update(selectorParityKeyPress("G"))
		body := bodyFromModel(kind.items, "", kind.activeItem, kind.title, last)
		assertSelectorParity(t, &m, body, kind.name, "S6_CursorLast")
	})

	// -------------------------------------------------------------------------
	// S7: Active-item indicator — ensure "(current)" renders identically.
	// The activeItem is in the list; cursor is on a different row.
	// -------------------------------------------------------------------------
	t.Run("S7_ActiveItemIndicator", func(t *testing.T) {
		m := kind.newModel(kind.items, kind.activeItem, k)
		m.SetSize(80, 24)

		// Keep cursor at 0 — if activeItem is at 0 the indicator is on the
		// selected row; if not it's on a non-selected row. Both paths matter.
		body := bodyFromModel(kind.items, "", kind.activeItem, kind.title, 0)
		assertSelectorParity(t, &m, body, kind.name, "S7_ActiveItemIndicator")
	})

	// -------------------------------------------------------------------------
	// S7b: Active-item on non-cursor row (cursor moved past active item).
	// -------------------------------------------------------------------------
	t.Run("S7b_ActiveItemNonCursorRow", func(t *testing.T) {
		m := kind.newModel(kind.items, kind.activeItem, k)
		m.SetSize(80, 24)

		// Find the active item index and move cursor past it.
		activeIdx := -1
		for i, item := range kind.items {
			if item == kind.activeItem {
				activeIdx = i
				break
			}
		}
		// Move cursor to a row after the active item (wrap to 0 if at end).
		targetCursor := activeIdx + 1
		if targetCursor >= len(kind.items) {
			targetCursor = 0
		}
		// Move from 0 to targetCursor by pressing j.
		for i := 0; i < targetCursor; i++ {
			m, _ = m.Update(selectorParityKeyPress("j"))
		}
		body := bodyFromModel(kind.items, "", kind.activeItem, kind.title, targetCursor)
		assertSelectorParity(t, &m, body, kind.name, "S7b_ActiveItemNonCursorRow")
	})

	// -------------------------------------------------------------------------
	// S8: Narrow width (40) — forces label truncation by Lipgloss Width().
	// -------------------------------------------------------------------------
	t.Run("S8_NarrowWidth40", func(t *testing.T) {
		m := kind.newModel(kind.items, kind.activeItem, k)
		m.SetSize(40, 24)

		body := bodyFromModel(kind.items, "", kind.activeItem, kind.title, 0)
		assertSelectorParity(t, &m, body, kind.name, "S8_NarrowWidth40")
	})

	// -------------------------------------------------------------------------
	// S9: Wide width (200).
	// -------------------------------------------------------------------------
	t.Run("S9_WideWidth200", func(t *testing.T) {
		m := kind.newModel(kind.items, kind.activeItem, k)
		m.SetSize(200, 24)

		body := bodyFromModel(kind.items, "", kind.activeItem, kind.title, 0)
		assertSelectorParity(t, &m, body, kind.name, "S9_WideWidth200")
	})

	// -------------------------------------------------------------------------
	// S10: Filter active AND cursor in the middle of filtered results.
	// -------------------------------------------------------------------------
	t.Run("S10_FilterActiveCursorMid", func(t *testing.T) {
		// Use a filter that yields at least 2 items.
		filterMap := map[string]string{
			"profile": "ac",
			"region":  "us",
			"theme":   "tok",
		}
		filter := filterMap[kind.name]

		m := kind.newModel(kind.items, kind.activeItem, k)
		m.SetSize(80, 24)
		m.SetFilter(filter)

		// Count filtered items to find middle.
		var filtered []string
		q := strings.ToLower(filter)
		for _, item := range kind.items {
			if strings.Contains(strings.ToLower(item), q) {
				filtered = append(filtered, item)
			}
		}
		if len(filtered) < 2 {
			t.Skipf("filter %q yields < 2 items for %s, skipping cursor-mid scenario", filter, kind.name)
		}

		// Move cursor to last filtered item.
		last := len(filtered) - 1
		for i := 0; i < last; i++ {
			m, _ = m.Update(selectorParityKeyPress("j"))
		}
		body := bodyFromModel(kind.items, filter, kind.activeItem, kind.title, last)
		assertSelectorParity(t, &m, body, kind.name, "S10_FilterActiveCursorMid")
	})

	// -------------------------------------------------------------------------
	// S11: Small viewport (height=3) — scroll window smaller than list.
	// -------------------------------------------------------------------------
	t.Run("S11_SmallViewport", func(t *testing.T) {
		m := kind.newModel(kind.items, kind.activeItem, k)
		m.SetSize(80, 3)

		// Move cursor past the first visible window.
		for i := 0; i < 3; i++ {
			m, _ = m.Update(selectorParityKeyPress("j"))
		}
		body := bodyFromModel(kind.items, "", kind.activeItem, kind.title, 3)
		assertSelectorParity(t, &m, body, kind.name, "S11_SmallViewport")
	})

	// -------------------------------------------------------------------------
	// S12: Empty items list.
	// -------------------------------------------------------------------------
	t.Run("S12_EmptyItems", func(t *testing.T) {
		m := kind.newModel([]string{}, "", k)
		m.SetSize(80, 24)

		body := bodyFromModel([]string{}, "", "", kind.title, 0)
		assertSelectorParity(t, &m, body, kind.name, "S12_EmptyItems")
	})

	// -------------------------------------------------------------------------
	// S13: Single item list.
	// -------------------------------------------------------------------------
	t.Run("S13_SingleItem", func(t *testing.T) {
		singleItem := kind.items[0]
		m := kind.newModel([]string{singleItem}, singleItem, k)
		m.SetSize(80, 24)

		body := bodyFromModel([]string{singleItem}, "", singleItem, kind.title, 0)
		assertSelectorParity(t, &m, body, kind.name, "S13_SingleItem")
	})
}
