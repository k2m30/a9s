package app

import (
	"strings"

	"github.com/k2m30/a9s/v3/internal/runtime"
)

// selectorPageSize is the default cursor jump for PageUp/PageDown on a
// selector screen when the renderer does not supply a viewport size via Action.N.
const selectorPageSize = 10

// selectorPageSizeFor returns the page size for a PageUp/PageDown action:
// the renderer-supplied viewport page size (Action.N) when given, else the default.
func selectorPageSizeFor(a Action) int {
	if a.N > 0 {
		return a.N
	}
	return selectorPageSize
}

// topSelectorState returns the SelectorState of the top-of-stack screen when
// the top screen is a selector (profile, region, or theme), nil otherwise.
func (c *Controller) topSelectorState() *SelectorState {
	if len(c.stack) == 0 {
		return nil
	}
	top := c.stack[len(c.stack)-1]
	if top.ID != runtime.ScreenProfileSelector &&
		top.ID != runtime.ScreenRegion &&
		top.ID != runtime.ScreenTheme {
		return nil
	}
	return c.stack[len(c.stack)-1].State.Selector
}

// EnsureSelectorState is the exported surface that TUI builders call
// immediately after pushing a selector screen (ScreenProfileSelector,
// ScreenRegion, ScreenTheme) so that Snapshot().Body.Selector is
// non-nil from the first render. Delegates to ensureSelectorState.
func (c *Controller) EnsureSelectorState(items []string, activeItem, title string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ensureSelectorState(items, activeItem, title)
}

// ensureSelectorState ensures the top selector screen has an initialised
// SelectorState. Called lazily from applyNavResult after a selector PushScreen
// so action handlers never dereference a nil SelectorState.
func (c *Controller) ensureSelectorState(items []string, activeItem, title string) {
	if len(c.stack) == 0 {
		return
	}
	top := &c.stack[len(c.stack)-1]
	if top.ID != runtime.ScreenProfileSelector &&
		top.ID != runtime.ScreenRegion &&
		top.ID != runtime.ScreenTheme {
		return
	}
	if top.State.Selector == nil {
		top.State.Selector = &SelectorState{
			Items:      items,
			ActiveItem: activeItem,
			Title:      title,
		}
	}
}

// selectorVisibleItems applies the filter from ss to ss.Items and returns the
// visible subset, mirroring selector.go applyFilter semantics exactly.
func selectorVisibleItems(ss *SelectorState) []string {
	if ss.Filter == "" {
		return ss.Items
	}
	q := strings.ToLower(ss.Filter)
	result := make([]string, 0, len(ss.Items))
	for _, item := range ss.Items {
		if strings.Contains(strings.ToLower(item), q) {
			result = append(result, item)
		}
	}
	return result
}

// buildSelectorBody constructs a SelectorBody from SelectorState, mirroring
// the data that selector.go View() and FrameTitle() consume.
func buildSelectorBody(ss *SelectorState) *SelectorBody {
	visible := selectorVisibleItems(ss)

	cursor := ss.Cursor
	if len(visible) > 0 && cursor >= len(visible) {
		cursor = len(visible) - 1
	}
	if cursor < 0 {
		cursor = 0
	}

	return &SelectorBody{
		Items:      visible,
		Selected:   cursor,
		AllItems:   ss.Items,
		Filter:     ss.Filter,
		ActiveItem: ss.ActiveItem,
		Title:      ss.Title,
	}
}

// selectorFrameTitle mirrors selector.go FrameTitle(), producing e.g.
// "aws-profiles(6)" or "aws-regions(3/17)".
func selectorFrameTitle(ss *SelectorState) string {
	total := len(ss.Items)
	visible := selectorVisibleItems(ss)
	filtered := len(visible)
	if ss.Filter != "" && filtered != total {
		return ss.Title + "(" + itoa(filtered) + "/" + itoa(total) + ")"
	}
	return ss.Title + "(" + itoa(total) + ")"
}

// SelectorFrameTitle returns the frame-border title for the top selector
// screen. Returns an empty string when the top screen is not a selector.
func (c *Controller) SelectorFrameTitle() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ss := c.topSelectorState()
	if ss == nil {
		return ""
	}
	return selectorFrameTitle(ss)
}

// SelectorSelected returns the item at the current cursor position in the top
// selector screen, plus a bool indicating whether the selector is active. Used
// by SelectorModel.Update to read the selected item on Enter without storing
// it in the renderer.
func (c *Controller) SelectorSelected() (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ss := c.topSelectorState()
	if ss == nil {
		return "", false
	}
	body := buildSelectorBody(ss)
	if len(body.Items) == 0 || body.Selected >= len(body.Items) {
		return "", false
	}
	return body.Items[body.Selected], true
}

// SelectorTitle returns the title of the top selector screen (e.g.
// "aws-profiles", "aws-regions", "themes"). Returns empty string when the top
// screen is not a selector.
func (c *Controller) SelectorTitle() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ss := c.topSelectorState()
	if ss == nil {
		return ""
	}
	return ss.Title
}

// SelectorFilter returns the current filter text of the top selector screen.
func (c *Controller) SelectorFilter() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ss := c.topSelectorState()
	if ss == nil {
		return ""
	}
	return ss.Filter
}
