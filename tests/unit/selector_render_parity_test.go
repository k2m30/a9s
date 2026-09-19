// SelectorModel.RenderSelector across
// selector kinds (profile/region/theme). RenderSelector reads only
// m.width/m.height from the model; everything else comes from
// app.SelectorBody, so each scenario passes the cursor position directly into
// the SelectorBody it renders.
package unit_test

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/internal/tui/views"
	"github.com/k2m30/a9s/v3/tests/unit/tuitest"
)

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

// TestSelectorRender_LiveSeam asserts RenderSelector's rendering contract
// for each selector kind and scenario.
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

	t.Run("S1_Default", func(t *testing.T) {
		body := bodyFromModel(kind.items, "", kind.activeItem, kind.title, 0)
		assertSelectorRender(t, 80, 24, body, kind.name, "S1_Default")
	})

	t.Run("S2_FilterActive", func(t *testing.T) {
		body := bodyFromModel(kind.items, filter, kind.activeItem, kind.title, 0)
		assertSelectorRender(t, 80, 24, body, kind.name, "S2_FilterActive")
	})

	t.Run("S3_FilterNoMatch", func(t *testing.T) {
		body := bodyFromModel(kind.items, "zzznomatch", kind.activeItem, kind.title, 0)
		got := assertSelectorRender(t, 80, 24, body, kind.name, "S3_FilterNoMatch")
		if got != "No items available" {
			t.Errorf("kind=%s scenario=S3_FilterNoMatch: expected empty-state text, got %q", kind.name, got)
		}
	})

	t.Run("S4_CursorFirst", func(t *testing.T) {
		body := bodyFromModel(kind.items, "", kind.activeItem, kind.title, 0)
		assertSelectorRender(t, 80, 24, body, kind.name, "S4_CursorFirst")
	})

	t.Run("S5_CursorMiddle", func(t *testing.T) {
		mid := len(kind.items) / 2
		body := bodyFromModel(kind.items, "", kind.activeItem, kind.title, mid)
		assertSelectorRender(t, 80, 24, body, kind.name, "S5_CursorMiddle")
	})

	t.Run("S6_CursorLast", func(t *testing.T) {
		last := len(kind.items) - 1
		body := bodyFromModel(kind.items, "", kind.activeItem, kind.title, last)
		assertSelectorRender(t, 80, 24, body, kind.name, "S6_CursorLast")
	})

	t.Run("S7_ActiveItemIndicator", func(t *testing.T) {
		body := bodyFromModel(kind.items, "", kind.activeItem, kind.title, 0)
		got := assertSelectorRender(t, 80, 24, body, kind.name, "S7_ActiveItemIndicator")
		if !strings.Contains(got, "(current)") {
			t.Errorf("kind=%s scenario=S7_ActiveItemIndicator: expected \"(current)\" marker for %q, got:\n%s", kind.name, kind.activeItem, got)
		}
	})

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

	// Width 40 forces label truncation by Lipgloss Width().
	t.Run("S8_NarrowWidth40", func(t *testing.T) {
		body := bodyFromModel(kind.items, "", kind.activeItem, kind.title, 0)
		assertSelectorRender(t, 40, 24, body, kind.name, "S8_NarrowWidth40")
	})

	t.Run("S9_WideWidth200", func(t *testing.T) {
		body := bodyFromModel(kind.items, "", kind.activeItem, kind.title, 0)
		assertSelectorRender(t, 200, 24, body, kind.name, "S9_WideWidth200")
	})

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

	// Height 3 with the cursor past the first visible window scrolls
	// RenderSelector's VisibleWindow, so only the scrolled-to window is rendered:
	// this checks the window size and that the cursor's own item scrolled into
	// view.
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

	t.Run("S12_EmptyItems", func(t *testing.T) {
		body := bodyFromModel([]string{}, "", "", kind.title, 0)
		m := views.NewTransientSelector(80, 24)
		got := m.RenderSelector(body)
		if got != "No items available" {
			t.Errorf("kind=%s scenario=S12_EmptyItems: expected empty-state text, got %q", kind.name, got)
		}
	})

	t.Run("S13_SingleItem", func(t *testing.T) {
		singleItem := kind.items[0]
		body := bodyFromModel([]string{singleItem}, "", singleItem, kind.title, 0)
		got := assertSelectorRender(t, 80, 24, body, kind.name, "S13_SingleItem")
		if !strings.Contains(got, "(current)") {
			t.Errorf("kind=%s scenario=S13_SingleItem: expected \"(current)\" marker for the single active item, got:\n%s", kind.name, got)
		}
	})
}
