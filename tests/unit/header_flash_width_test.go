// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// header_flash_width_test.go — a flash message is cut to the columns the
// header keeps for it.
//
// The header reserves that slot in terminal columns but cut the message to fit
// by counting runes. A message written in characters the terminal paints two
// cells wide passed the rune test at twice the width, and the header made room
// by dropping the profile and region — so the operator lost which account they
// were looking at in order to read "copied".
package unit

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui/text"
	"github.com/k2m30/a9s/v3/tests/unit/tuitest"
)

// headerLine renders the root model with msg flashing and returns the header.
func headerLine(t *testing.T, msg string) string {
	t.Helper()
	m := tuitest.Sized("testprofile", "us-east-1")
	m = tuitest.StepModel(m, messages.Flash{Text: msg})
	lines := strings.Split(tuitest.StripANSI(tuitest.Render(m)), "\n")
	if len(lines) == 0 {
		t.Fatal("the model rendered nothing")
	}
	return lines[0]
}

func TestHeaderFlash_WideMessageKeepsTheProfileAndRegion(t *testing.T) {
	tuitest.NoColor(t)

	// Same rune count, twice the columns. The header at 80 columns keeps 40
	// for the flash, so the wide one fits by rune count and not by width.
	const runes = 36
	ascii := strings.Repeat("x", runes)
	wide := strings.Repeat("名", runes)
	if w := text.Width(wide); w <= 40 || text.Width(ascii) > 40 {
		t.Fatalf("the probe messages are %d and %d columns; the wide one must exceed the header's 40 and the ascii one must not",
			w, text.Width(ascii))
	}

	const badge = "testprofile:us-east-1"
	if got := headerLine(t, ascii); !strings.Contains(got, badge) {
		t.Fatalf("the control failed: an ascii flash of %d columns already costs the header its profile and region:\n%q", runes, got)
	}
	if got := headerLine(t, wide); !strings.Contains(got, badge) {
		t.Errorf("a flash of %d columns pushed %q out of the header:\n%q", text.Width(wide), badge, got)
	}
}

// TestHeaderFlash_WideMessageIsCutToTheSlot pins the cut itself: whatever the
// header shows of the message occupies no more columns than the slot it was
// measured against.
func TestHeaderFlash_WideMessageIsCutToTheSlot(t *testing.T) {
	tuitest.NoColor(t)

	line := headerLine(t, strings.Repeat("名", 36))
	painted := strings.Count(line, "名") * 2
	if painted > 40 {
		t.Errorf("the header paints %d columns of the message into a 40-column slot:\n%q", painted, line)
	}
}
