package unit

// enrich_lane_log_entry_test.go — a failed enrichment is on the `!` log
// screen, once.
//
// This is a lock, not a fix. The entry is there today: the enrich lane's
// FlashIntent is re-emitted as messages.Flash by the plural applyIntents path
// (internal/tui/app_dispatch.go:56), and Core.HandleFlash appends the history
// entry for an error flash (core/runtime/handlers.go:127). A handler that also
// appended the line itself would write it twice, which is what this pins
// against — the failure mode is a second entry, not a missing one.

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/runtime/messages"
	tui "github.com/k2m30/a9s/v3/internal/tui"
)

// logScreenLines drives one event through the real model, delivers the flash
// the way the program loop would, opens the error log with "!" and returns its
// lines that mention want.
func logScreenLines(t *testing.T, msg tea.Msg, want string) []string {
	t.Helper()
	m2, cmd := newTestModel().Update(msg)
	m := m2.(tui.Model)
	if f, ok := walkForFlash(cmd); ok {
		m3, _ := m.Update(f)
		m = m3.(tui.Model)
	}
	opened, _ := m.Update(tea.KeyPressMsg{Code: '!', Text: "!"})

	var lines []string
	for _, l := range strings.Split(opened.(tui.Model).View().Content, "\n") {
		// The flash sits in the header; the log entries are timestamped.
		if strings.Contains(l, want) && strings.Contains(l, "] ") {
			lines = append(lines, l)
		}
	}
	return lines
}

// TestEnrichLane_FailureIsOnTheLogScreenOnce pins what the operator looks for:
// a refused enrichment is on the error-log screen, and exactly once.
func TestEnrichLane_FailureIsOnTheLogScreenOnce(t *testing.T) {
	lines := logScreenLines(t,
		messages.EnrichmentChecked{ResourceType: "ddb", Err: errors.New("connection reset")},
		"enrich ddb: connection reset")
	if len(lines) != 1 {
		t.Errorf("the error log holds %d entries for one refused enrichment, want 1:\n%s",
			len(lines), strings.Join(lines, "\n"))
	}
}

// TestAPIErrorLane_FailureIsOnTheLogScreenOnce is the same pin for the lane
// that reaches the log the other way: its handler appends the entry itself
// because its intents take the direct-mutate path
// (internal/tui/app_flash.go:49), which does not re-emit the flash.
func TestAPIErrorLane_FailureIsOnTheLogScreenOnce(t *testing.T) {
	lines := logScreenLines(t,
		messages.APIError{ResourceType: "ec2", Err: errors.New("connection reset")},
		"connection reset")
	if len(lines) != 1 {
		t.Errorf("the error log holds %d entries for one API error, want 1:\n%s",
			len(lines), strings.Join(lines, "\n"))
	}
}
