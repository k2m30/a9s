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

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
)

// listInstanceReporter is what a root model has to answer to for a
// hand-built page message to be stamped for the screen the test is driving.
// tui.Model does not implement it yet: the accessor is one line over
// app.Controller.GetListInstance, and until it lands StampPage below leaves
// model-driven messages alone. Replace this assertion with a direct call the
// day it does.
type listInstanceReporter interface {
	ListInstanceForTest() domain.Gen
}

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// StampPage fills in the screen identity a page message would have carried
// had a real dispatch produced it. A test that hand-builds
// messages.ResourcesLoaded is standing in for the fetch command, and the
// command stamps the instance of the screen that issued it; without it the
// message names no screen and can only be routed by resource type, which is
// a guess as soon as two lists of one type are stacked.
//
// The sequence is deliberately left at zero. It orders one screen's requests
// against each other, and a hand-built page has no sibling request to be
// ordered against — a zero sequence is never superseded, which is what a
// single delivery means. A test that IS about ordering sets its own.
//
// Messages that are not pages, and pages that already name a screen, pass
// through untouched: a test that means to deliver a page naming nobody
// (or one naming a screen it has since popped) still can.
func StampPage(screen domain.Gen, msg tea.Msg) tea.Msg {
	page, ok := msg.(messages.ResourcesLoaded)
	if !ok || page.ScreenID != 0 || screen == 0 {
		return msg
	}
	page.ScreenID = screen
	return page
}

// Step sends msg through m.Update and returns the updated model and command.
// It is the canonical implementation of the rootApplyMsg / applyMsg pattern,
// and the one place a model-driven test's hand-built page is stamped.
func Step(m tui.Model, msg tea.Msg) (tui.Model, tea.Cmd) {
	newM, cmd := m.Update(stampFor(m, msg))
	return newM.(tui.Model), cmd
}

// StepModel sends msg through m.Update and returns the updated model,
// discarding the command. Use when the caller only needs the next state.
func StepModel(m tui.Model, msg tea.Msg) tui.Model {
	newM, _ := m.Update(stampFor(m, msg))
	return newM.(tui.Model)
}

// stampFor is StampPage for the screen m is currently showing.
func stampFor(m tui.Model, msg tea.Msg) tea.Msg {
	r, ok := any(m).(listInstanceReporter)
	if !ok {
		return msg
	}
	return StampPage(r.ListInstanceForTest(), msg)
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
	styles.ReinitForTest()
	t.Cleanup(func() {
		_ = os.Unsetenv("NO_COLOR")
		styles.ReinitForTest()
	})
}

// ForceColor unsets NO_COLOR for the duration of t, reinitialises the style
// palette so all Render calls emit ANSI escape sequences, and restores the
// original environment on cleanup. Call at the top of any test that does
// string matching on colored rendered output.
func ForceColor(t *testing.T) {
	t.Helper()
	original, wasSet := os.LookupEnv("NO_COLOR")
	os.Unsetenv("NO_COLOR") //nolint:errcheck
	styles.ReinitForTest()
	t.Cleanup(func() {
		if wasSet {
			os.Setenv("NO_COLOR", original) //nolint:errcheck
		} else {
			_ = os.Unsetenv("NO_COLOR")
		}
		styles.ReinitForTest()
	})
}
