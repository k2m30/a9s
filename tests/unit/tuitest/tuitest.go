// Package tuitest provides shared test primitives for the a9s TUI test suite.
// It is imported by both the internal package unit and the external package
// unit_test so that harness logic lives in exactly one place.
//
// This is a normal (non-test) package so it can be imported across the package
// boundary without triggering import cycles. It must never import the test
// packages it supports.
package tuitest

import (
	"os"
	"regexp"
	"testing"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"

	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
)

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// Step sends msg through m.Update and returns the updated model and command.
// It is the canonical implementation of the rootApplyMsg / applyMsg pattern.
func Step(m tui.Model, msg tea.Msg) (tui.Model, tea.Cmd) {
	newM, cmd := m.Update(msg)
	return newM.(tui.Model), cmd
}

// StepModel sends msg through m.Update and returns the updated model,
// discarding the command. Use when the caller only needs the next state.
func StepModel(m tui.Model, msg tea.Msg) tui.Model {
	newM, _ := m.Update(msg)
	return newM.(tui.Model)
}

// Render returns the rendered content string from a root model's View().
func Render(m tui.Model) string {
	return m.View().Content
}

// Sized constructs a root model for the given AWS profile and region with an
// 80×40 terminal size already applied, so View() produces real output.
func Sized(profile, region string) tui.Model {
	m := tui.New(profile, region)
	m, _ = Step(m, tea.WindowSizeMsg{Width: 80, Height: 40})
	return m
}

// StripANSI removes ANSI escape sequences from s, returning plain text
// suitable for string assertions in tests.
func StripANSI(s string) string {
	return ansiRe.ReplaceAllString(s, "")
}

// Width returns the visible (ANSI-aware) display width of s.
func Width(s string) int {
	return lipgloss.Width(s)
}

// NoColor sets NO_COLOR=1 for the duration of t, reinitialises the style
// palette so all Render calls emit plain text, and restores the original
// environment on cleanup. Call at the top of any test that does string
// matching on rendered output.
func NoColor(t *testing.T) {
	t.Helper()
	t.Setenv("NO_COLOR", "1")
	styles.Reinit()
	t.Cleanup(func() {
		_ = os.Unsetenv("NO_COLOR")
		styles.Reinit()
	})
}
