//go:build integration

package integration

import (
	"regexp"
	"testing"

	"github.com/k2m30/a9s/v3/internal/app"
)

// ansiEscapePattern matches ANSI escape sequences (CSI sequences).
var ansiEscapePattern = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// QA-178: NO_COLOR environment variable disables colors
func TestQA_178_NoColorEnvVar(t *testing.T) {
	// Set NO_COLOR before initializing styles. The lipgloss/termenv library
	// respects this environment variable and should suppress ANSI codes.
	t.Setenv("NO_COLOR", "1")
	t.Setenv("TERM", "dumb")

	// Re-init styles with NO_COLOR set
	app.InitStyles()

	state := app.NewAppState("", "")
	state.Width = 80
	state.Height = 24

	view := state.View()
	content := view.Content

	if content == "" {
		t.Fatal("expected non-empty view content")
	}

	// Check for ANSI escape sequences in the output.
	// With NO_COLOR set, there should be no color escape sequences.
	matches := ansiEscapePattern.FindAllString(content, -1)
	if len(matches) > 0 {
		// Filter out non-color escapes (e.g. cursor movement may still be present).
		// Color codes use SGR format: ESC[...m
		colorPattern := regexp.MustCompile(`\x1b\[[0-9;]*m`)
		colorMatches := colorPattern.FindAllString(content, -1)
		if len(colorMatches) > 0 {
			t.Errorf("with NO_COLOR=1, view should not contain ANSI color codes, found %d: %v",
				len(colorMatches), colorMatches[:min(5, len(colorMatches))])
		}
	}

	t.Log("NO_COLOR respected: no ANSI color codes detected in output")
}

// QA-179: Non-256-color terminal
func TestQA_179_Non256ColorTerminal(t *testing.T) {
	// Simulate a basic terminal that only supports 8 colors.
	t.Setenv("TERM", "dumb")
	t.Setenv("COLORTERM", "")

	app.InitStyles()

	state := app.NewAppState("", "")
	state.Width = 80
	state.Height = 24

	// Should render without panic
	view := state.View()
	if view.Content == "" {
		t.Error("expected non-empty view content with dumb terminal")
	}
	t.Log("dumb terminal rendering succeeded without panic")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
