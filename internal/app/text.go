package app

import (
	"strings"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime"
)

// textPageSize is the default scroll jump for PageUp/PageDown on a text screen
// when the renderer does not supply a viewport size via Action.N.
const textPageSize = 10

// textPageSizeFor returns the page size for a PageUp/PageDown action:
// the renderer-supplied viewport height (Action.N) when given, else the default.
func textPageSizeFor(a Action) int {
	if a.N > 0 {
		return a.N
	}
	return textPageSize
}

// isTextScreen reports whether id is one of the text-viewer screen IDs
// (YAML, JSON, or error-log).
func isTextScreen(id runtime.ScreenID) bool {
	return id == runtime.ScreenYAML || id == runtime.ScreenJSON || id == runtime.ScreenErrorLog
}

// topTextState returns the TextState of the top-of-stack screen when the top
// screen is a text viewer (ScreenYAML, ScreenJSON, or ScreenErrorLog), nil otherwise.
func (c *Controller) topTextState() *TextState {
	if len(c.stack) == 0 {
		return nil
	}
	top := c.stack[len(c.stack)-1]
	if !isTextScreen(top.ID) {
		return nil
	}
	return c.stack[len(c.stack)-1].State.Text
}

// EnsureTextState is the exported surface that TUI builders call immediately
// after pushing a YAML or JSON screen so that Snapshot().Body.Text is non-nil
// from the first render. Delegates to ensureTextState.
func (c *Controller) EnsureTextState(lines []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ensureTextState(lines)
}

// ensureTextState initialises the top text screen's TextState. It is a
// set-once operation: if TextState is already non-nil the call is a no-op.
// Callers must hold c.mu (write).
func (c *Controller) ensureTextState(lines []string) {
	if len(c.stack) == 0 {
		return
	}
	top := &c.stack[len(c.stack)-1]
	if !isTextScreen(top.ID) {
		return
	}
	if top.State.Text == nil {
		top.State.Text = &TextState{
			Lines: lines,
		}
	}
}

// UpdateTextLines replaces the Lines in the top text screen's TextState with
// new content. Unlike EnsureTextState this is NOT set-once: it is called when
// enrichment arrives after the screen was pushed so that the re-rendered
// syntax-colored content replaces the pre-enrichment snapshot. Other TextState
// fields (search, wrap, scrollY) are preserved.
func (c *Controller) UpdateTextLines(lines []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	ts := c.topTextState()
	if ts == nil {
		return
	}
	ts.Lines = lines
}

// GetTextScreenContext returns the ScreenID and ScreenContext of the top text
// screen (YAML, JSON, or error-log). Used by the TUI adapter to determine
// whether the active text view is YAML or JSON before regenerating enriched
// content lines. Returns ("", empty context) when the top screen is not a
// text screen.
func (c *Controller) GetTextScreenContext() (runtime.ScreenID, runtime.ScreenContext) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if len(c.stack) == 0 {
		return "", runtime.ScreenContext{}
	}
	top := c.stack[len(c.stack)-1]
	if !isTextScreen(top.ID) {
		return "", runtime.ScreenContext{}
	}
	return top.ID, top.Ctx
}

// buildTextSearchMatches scans lines for all case-insensitive occurrences of
// query and returns a SearchMatch slice matching the SearchModel.recomputeMatches
// semantics used by the YAML/JSON views.
func buildTextSearchMatches(lines []string, query string) []SearchMatch {
	if query == "" {
		return nil
	}
	q := strings.ToLower(query)
	var matches []SearchMatch
	for lineIdx, line := range lines {
		lower := strings.ToLower(line)
		start := 0
		for {
			idx := strings.Index(lower[start:], q)
			if idx < 0 {
				break
			}
			matches = append(matches, SearchMatch{
				Line:     lineIdx,
				ColStart: start + idx,
				ColEnd:   start + idx + len(q),
			})
			start += idx + len(q)
		}
	}
	return matches
}

// buildTextBody constructs a TextBody from TextState, mirroring the data that
// YAMLModel.View() / JSONModel.View() consume via their viewport content.
func buildTextBody(ts *TextState) *TextBody {
	matches := buildTextSearchMatches(ts.Lines, ts.Search)

	// Clamp SearchCursor to valid range.
	cursor := ts.SearchCursor
	if len(matches) == 0 {
		cursor = 0
	} else if cursor >= len(matches) {
		cursor = len(matches) - 1
	}
	if cursor < 0 {
		cursor = 0
	}

	return &TextBody{
		Lines:         ts.Lines,
		SearchMatches: matches,
		Wrap:          ts.Wrap,
		ScrollY:       ts.ScrollY,
		SearchCursor:  cursor,
		Search:        ts.Search,
	}
}

// TextFrameTitle returns the frame-border title for the top text screen.
// Returns an empty string when the top screen is not a text screen.
// The title is the raw ScreenID string ("yaml" or "json"); callers that need
// a resource-qualified title (e.g. "i-0abc123 yaml") should use the TUI
// view's FrameTitle() which has access to the resource.Resource.
func (c *Controller) TextFrameTitle() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ts := c.topTextState()
	if ts == nil {
		return ""
	}
	top := c.stack[len(c.stack)-1]
	return string(top.ID)
}

// ErrorHistoryLines formats the controller's session error history as
// display lines, newest-first, "[HH:MM:SS] message" per entry — matching the
// format the TUI's error-log viewer (! key) has always shown. Used to seed
// ScreenErrorLog's TextState via EnsureTextState so the error-log screen
// renders from Snapshot().Body.Text like every other ctrl-backed text screen,
// with the controller's errorHistory as the single source of truth (goal-4
// wave 4a — see AppendErrorHistoryIntent in intents.go for how entries land).
func (c *Controller) ErrorHistoryLines() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if len(c.errorHistory) == 0 {
		return nil
	}
	lines := make([]string, len(c.errorHistory))
	for i := 0; i < len(c.errorHistory); i++ {
		e := c.errorHistory[len(c.errorHistory)-1-i]
		lines[i] = "[" + e.t.Format("15:04:05") + "] " + e.message
	}
	return lines
}

// HasErrorHistory reports whether the controller has recorded any session
// errors. Used by the TUI's '!' key handler to short-circuit with a "No
// errors this session" flash instead of pushing an empty error-log screen.
func (c *Controller) HasErrorHistory() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.errorHistory) > 0
}

// GetTextResource returns the resource for the top text screen (YAML/JSON)
// by resolving it from the resource cache using the screen's ScreenContext.
// Returns the zero-value Resource when the top screen is not a text screen
// or when the resource cannot be resolved from the cache.
func (c *Controller) GetTextResource() resource.Resource {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if len(c.stack) == 0 {
		return resource.Resource{}
	}
	top := c.stack[len(c.stack)-1]
	if !isTextScreen(top.ID) {
		return resource.Resource{}
	}
	if top.Ctx.ResourceType == "" || top.Ctx.ResourceID == "" {
		return resource.Resource{}
	}
	if r, ok := c.findCachedResourceByID(top.Ctx.ResourceType, top.Ctx.ResourceID); ok {
		return r
	}
	return resource.Resource{}
}
