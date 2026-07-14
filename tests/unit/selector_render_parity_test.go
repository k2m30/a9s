// selector_render_parity_test.go — live-path coverage for SelectorModel.RenderSelector
// across selector kinds (profile/region/theme) and a set of scenarios per kind.
//
// Originally a byte-parity gate comparing RenderSelector(body) against the legacy
// SelectorModel.View() built via NewProfile/NewRegion/NewTheme. Those constructors
// and View() are DEAD per specs/022-codebase-cleanup/wave3-map-text.md (selector.go:
// "LIVE: NewSelectorWithCtrl, NewTransientSelector, Update, SetSize, RenderSelector").
// Retargeted onto the live seam: NewTransientSelector(w, h) — RenderSelector reads
// only m.width/m.height from the model, everything else comes from app.SelectorBody
// — so no Update()-driven cursor walk is needed; each scenario passes the cursor
// position directly into the SelectorBody it renders. Assertions check that
// RenderSelector's own contract (selector.go) holds: every visible item renders,
// every filtered-out item does not, and the active item's "(current)" marker
// appears when it's in the visible window.
package unit_test

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/app"
	"github.com/k2m30/a9s/v3/internal/tui/views"
	"github.com/k2m30/a9s/v3/tests/unit/tuitest"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// bodyFromModel constructs the SelectorBody that buildSelectorBody(SelectorState)
// would produce for the same logical state — mirroring the filtering and
// cursor-clamping logic in selector.go.
//
// items is the full unfiltered list; filterText is the active filter;
// cursor is the cursor index into the FILTERED list; activeItem is the
// item that receives the "(current)" indicator; title is the frame title.
func bodyFromModel(items []string, filterText, activeItem, title string, cursor int) app.SelectorBody {
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

// assertSelectorRender renders body via the live NewTransientSelector(w, h) seam
// and checks selector.go's RenderSelector contract: every filtered-in item is
// present in the output, and — when a filter is active — every filtered-out item
// is absent.
func assertSelectorRender(t *testing.T, w, h int, body app.SelectorBody, kind, scenario string) string {
	t.Helper()
	m := views.NewTransientSelector(w, h)
	got := m.RenderSelector(body)
	for _, item := range body.Items {
		if !strings.Contains(got, item) {
			t.Errorf("kind=%s scenario=%s: RenderSelector output missing visible item %q\n---\n%s", kind, scenario, item, got)
		}
	}
	if body.Filter != "" {
		visible := map[string]bool{}
		for _, it := range body.Items {
			visible[it] = true
		}
		for _, it := range body.AllItems {
			if !visible[it] && strings.Contains(got, it) {
				t.Errorf("kind=%s scenario=%s: filter %q leaked excluded item %q into RenderSelector output\n---\n%s", kind, scenario, body.Filter, it, got)
			}
		}
	}
	return got
}

// ---------------------------------------------------------------------------
// Selector kind descriptors
// ---------------------------------------------------------------------------

type selectorKind struct {
	name       string
	items      []string
	activeItem string
	title      string
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
		{name: "profile", items: fakeProfiles, activeItem: "staging-account", title: "aws-profiles"},
		{name: "region", items: fakeRegions, activeItem: "eu-west-1", title: "aws-regions"},
		{name: "theme", items: fakeThemes, activeItem: "dracula", title: "themes"},
	}
}

// ---------------------------------------------------------------------------
// Top-level test
// ---------------------------------------------------------------------------

// TestSelectorRender_LiveSeam is the live-path coverage gate for RenderSelector.
// Each subtest builds a SelectorBody for a given scenario and asserts
// RenderSelector's rendering contract against it.
func TestSelectorRender_LiveSeam(t *testing.T) {
	tuitest.NoColor(t)

	for _, kind := range selectorKinds() {
		kind := kind
		t.Run(kind.name, func(t *testing.T) {
			runSelectorRenderScenarios(t, kind)
		})
	}
}

func runSelectorRenderScenarios(t *testing.T, kind selectorKind) {
	filterMap := map[string]string{
		"profile": "ac",  // matches *-account items
		"region":  "us",  // matches us-east-1 and us-west-2
		"theme":   "tok", // matches tokyo-night-*
	}
	filter := filterMap[kind.name]

	// S1: Default state — no filter, cursor at 0.
	t.Run("S1_Default", func(t *testing.T) {
		body := bodyFromModel(kind.items, "", kind.activeItem, kind.title, 0)
		assertSelectorRender(t, 80, 24, body, kind.name, "S1_Default")
	})

	// S2: Filter active narrowing the list.
	t.Run("S2_FilterActive", func(t *testing.T) {
		body := bodyFromModel(kind.items, filter, kind.activeItem, kind.title, 0)
		assertSelectorRender(t, 80, 24, body, kind.name, "S2_FilterActive")
	})

	// S3: Filter matching nothing.
	t.Run("S3_FilterNoMatch", func(t *testing.T) {
		body := bodyFromModel(kind.items, "zzznomatch", kind.activeItem, kind.title, 0)
		got := assertSelectorRender(t, 80, 24, body, kind.name, "S3_FilterNoMatch")
		if got != "No items available" {
			t.Errorf("kind=%s scenario=S3_FilterNoMatch: expected empty-state text, got %q", kind.name, got)
		}
	})

	// S4: Cursor on first item.
	t.Run("S4_CursorFirst", func(t *testing.T) {
		body := bodyFromModel(kind.items, "", kind.activeItem, kind.title, 0)
		assertSelectorRender(t, 80, 24, body, kind.name, "S4_CursorFirst")
	})

	// S5: Cursor on middle item.
	t.Run("S5_CursorMiddle", func(t *testing.T) {
		mid := len(kind.items) / 2
		body := bodyFromModel(kind.items, "", kind.activeItem, kind.title, mid)
		assertSelectorRender(t, 80, 24, body, kind.name, "S5_CursorMiddle")
	})

	// S6: Cursor on last item.
	t.Run("S6_CursorLast", func(t *testing.T) {
		last := len(kind.items) - 1
		body := bodyFromModel(kind.items, "", kind.activeItem, kind.title, last)
		assertSelectorRender(t, 80, 24, body, kind.name, "S6_CursorLast")
	})

	// S7: Active-item indicator — activeItem's row must carry "(current)".
	t.Run("S7_ActiveItemIndicator", func(t *testing.T) {
		body := bodyFromModel(kind.items, "", kind.activeItem, kind.title, 0)
		got := assertSelectorRender(t, 80, 24, body, kind.name, "S7_ActiveItemIndicator")
		if !strings.Contains(got, "(current)") {
			t.Errorf("kind=%s scenario=S7_ActiveItemIndicator: expected \"(current)\" marker for %q, got:\n%s", kind.name, kind.activeItem, got)
		}
	})

	// S7b: Active-item on a non-cursor row (cursor moved past the active item).
	t.Run("S7b_ActiveItemNonCursorRow", func(t *testing.T) {
		activeIdx := -1
		for i, item := range kind.items {
			if item == kind.activeItem {
				activeIdx = i
				break
			}
		}
		targetCursor := activeIdx + 1
		if targetCursor >= len(kind.items) {
			targetCursor = 0
		}
		body := bodyFromModel(kind.items, "", kind.activeItem, kind.title, targetCursor)
		got := assertSelectorRender(t, 80, 24, body, kind.name, "S7b_ActiveItemNonCursorRow")
		if !strings.Contains(got, "(current)") {
			t.Errorf("kind=%s scenario=S7b_ActiveItemNonCursorRow: expected \"(current)\" marker for %q, got:\n%s", kind.name, kind.activeItem, got)
		}
	})

	// S8: Narrow width (40) — forces label truncation by Lipgloss Width().
	t.Run("S8_NarrowWidth40", func(t *testing.T) {
		body := bodyFromModel(kind.items, "", kind.activeItem, kind.title, 0)
		assertSelectorRender(t, 40, 24, body, kind.name, "S8_NarrowWidth40")
	})

	// S9: Wide width (200).
	t.Run("S9_WideWidth200", func(t *testing.T) {
		body := bodyFromModel(kind.items, "", kind.activeItem, kind.title, 0)
		assertSelectorRender(t, 200, 24, body, kind.name, "S9_WideWidth200")
	})

	// S10: Filter active AND cursor on the last filtered result.
	t.Run("S10_FilterActiveCursorMid", func(t *testing.T) {
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
		last := len(filtered) - 1
		body := bodyFromModel(kind.items, filter, kind.activeItem, kind.title, last)
		assertSelectorRender(t, 80, 24, body, kind.name, "S10_FilterActiveCursorMid")
	})

	// S11: Small viewport (height=3) — cursor past the first visible window
	// forces RenderSelector's VisibleWindow scroll logic to kick in. Only the
	// scrolled-to window is rendered, so this does NOT use assertSelectorRender
	// (which expects every body.Items entry to be visible) — it checks the
	// window size and that the cursor's own item scrolled into view instead.
	t.Run("S11_SmallViewport", func(t *testing.T) {
		body := bodyFromModel(kind.items, "", kind.activeItem, kind.title, 3)
		m := views.NewTransientSelector(80, 3)
		got := m.RenderSelector(body)
		lines := strings.Split(got, "\n")
		if len(lines) != 3 {
			t.Errorf("kind=%s scenario=S11_SmallViewport: expected 3 visible rows (height=3), got %d:\n%s", kind.name, len(lines), got)
		}
		if !strings.Contains(got, kind.items[3]) {
			t.Errorf("kind=%s scenario=S11_SmallViewport: expected cursor item %q scrolled into view, got:\n%s", kind.name, kind.items[3], got)
		}
	})

	// S12: Empty items list — RenderSelector's documented empty-state text.
	t.Run("S12_EmptyItems", func(t *testing.T) {
		body := bodyFromModel([]string{}, "", "", kind.title, 0)
		m := views.NewTransientSelector(80, 24)
		got := m.RenderSelector(body)
		if got != "No items available" {
			t.Errorf("kind=%s scenario=S12_EmptyItems: expected empty-state text, got %q", kind.name, got)
		}
	})

	// S13: Single item list.
	t.Run("S13_SingleItem", func(t *testing.T) {
		singleItem := kind.items[0]
		body := bodyFromModel([]string{singleItem}, "", singleItem, kind.title, 0)
		got := assertSelectorRender(t, 80, 24, body, kind.name, "S13_SingleItem")
		if !strings.Contains(got, "(current)") {
			t.Errorf("kind=%s scenario=S13_SingleItem: expected \"(current)\" marker for the single active item, got:\n%s", kind.name, got)
		}
	})
}
