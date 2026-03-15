package unit

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/internal/app"
	"github.com/k2m30/a9s/internal/resource"
)

// Table should render at full natural width. When the table is wider than
// the terminal, horizontal scrolling with left/right arrow keys pans the
// visible window. The cursor row stays visible.

func makeWideEC2() []resource.Resource {
	return []resource.Resource{
		{
			ID: "i-abc123", Name: "prod-web-server-us-east", Status: "running",
			Fields: map[string]string{
				"instance_id": "i-abc123",
				"name":        "prod-web-server-us-east",
				"state":       "running",
				"type":        "t3.medium",
				"private_ip":  "10.0.1.1",
				"public_ip":   "54.1.2.3",
				"launch_time": "2026-01-15T10:30:00Z",
			},
			DetailData: map[string]string{"Instance ID": "i-abc123"},
			RawJSON:    `{"id":"i-abc123"}`,
		},
	}
}

func TestHorizontalScroll_InitialViewShowsLeftColumns(t *testing.T) {
	state := app.NewAppState("", "")
	state.CurrentView = app.ResourceListView
	state.CurrentResourceType = "ec2"
	state.Width = 60 // narrower than full table
	state.Height = 24
	state.Resources = makeWideEC2()

	view := state.View()

	// First columns should be visible
	if !strings.Contains(view.Content, "i-abc123") {
		t.Error("Instance ID should be visible in initial scroll position")
	}
}

func TestHorizontalScroll_RightArrowPansRight(t *testing.T) {
	state := app.NewAppState("", "")
	state.CurrentView = app.ResourceListView
	state.CurrentResourceType = "ec2"
	state.Width = 60
	state.Height = 24
	state.Resources = makeWideEC2()

	// Press right arrow multiple times to scroll right
	for i := 0; i < 10; i++ {
		updated, _ := state.Update(tea.KeyPressMsg{Code: -1, Text: "l"})
		state = updated.(app.AppState)
	}

	view := state.View()

	// After scrolling right, later columns should become visible
	// and the leftmost column content may have scrolled off
	if state.HScrollOffset <= 0 {
		t.Error("HScrollOffset should increase after pressing right/l")
	}
	_ = view // no panic
}

func TestHorizontalScroll_LeftArrowPansLeft(t *testing.T) {
	state := app.NewAppState("", "")
	state.CurrentView = app.ResourceListView
	state.CurrentResourceType = "ec2"
	state.Width = 60
	state.Height = 24
	state.Resources = makeWideEC2()

	// Scroll right first
	for i := 0; i < 10; i++ {
		updated, _ := state.Update(tea.KeyPressMsg{Code: -1, Text: "l"})
		state = updated.(app.AppState)
	}
	offsetAfterRight := state.HScrollOffset

	// Scroll left
	for i := 0; i < 5; i++ {
		updated, _ := state.Update(tea.KeyPressMsg{Code: -1, Text: "h"})
		state = updated.(app.AppState)
	}

	if state.HScrollOffset >= offsetAfterRight {
		t.Error("HScrollOffset should decrease after pressing left/h")
	}
}

func TestHorizontalScroll_CannotScrollPastZero(t *testing.T) {
	state := app.NewAppState("", "")
	state.CurrentView = app.ResourceListView
	state.CurrentResourceType = "ec2"
	state.Width = 60
	state.Height = 24
	state.Resources = makeWideEC2()

	// Press left at offset 0 — should stay at 0
	updated, _ := state.Update(tea.KeyPressMsg{Code: -1, Text: "h"})
	state = updated.(app.AppState)

	if state.HScrollOffset != 0 {
		t.Errorf("HScrollOffset should not go below 0, got %d", state.HScrollOffset)
	}
}

func TestHorizontalScroll_ResetOnResourceSwitch(t *testing.T) {
	state := app.NewAppState("", "")
	state.CurrentView = app.ResourceListView
	state.CurrentResourceType = "ec2"
	state.Width = 60
	state.Height = 24
	state.Resources = makeWideEC2()

	// Scroll right
	for i := 0; i < 5; i++ {
		updated, _ := state.Update(tea.KeyPressMsg{Code: -1, Text: "l"})
		state = updated.(app.AppState)
	}

	// Switch to :rds
	state.CommandMode = true
	state.CommandText = "rds"
	updated, _ := state.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	state = updated.(app.AppState)

	if state.HScrollOffset != 0 {
		t.Errorf("HScrollOffset should reset on resource switch, got %d", state.HScrollOffset)
	}
}

func TestHorizontalScroll_WideTerminalNoScrollNeeded(t *testing.T) {
	state := app.NewAppState("", "")
	state.CurrentView = app.ResourceListView
	state.CurrentResourceType = "ec2"
	state.Width = 200 // wider than table
	state.Height = 24
	state.Resources = makeWideEC2()

	view := state.View()

	// All columns should be visible without scrolling
	if !strings.Contains(view.Content, "i-abc123") {
		t.Error("Instance ID should be visible")
	}
	if !strings.Contains(view.Content, "2026-01-15") {
		t.Error("Launch time should be visible at wide terminal")
	}
}
