// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// tui5_smoke_readonly_poll_test.go — the live read-only smoke gave the sweep
// counter a 30-second window measured from tmux session start, which is before
// the app has connected to AWS or painted anything. On a slow connect the whole
// window went to the connect and the smoke failed on a healthy account. The
// window belongs after the menu has painted.
package unit_test

import (
	"os"
	"strings"
	"testing"
)

// TestSmokeReadonlyWaitsForTheMenuBeforeTimingTheSweep reads the script and
// asserts the sweep-counter poll is preceded by a poll for the menu having
// painted. Read from the source because the script talks to a live account and
// cannot run here.
func TestSmokeReadonlyWaitsForTheMenuBeforeTimingTheSweep(t *testing.T) {
	data, err := os.ReadFile("../../scripts/smoke-readonly.sh")
	if err != nil {
		t.Fatalf("reading smoke-readonly.sh: %v", err)
	}
	body := string(data)

	menuPainted := strings.Index(body, "resource-types(")
	if menuPainted < 0 {
		menuPainted = strings.Index(body, `resource-types\(`)
	}
	sweepWindow := strings.Index(body, "SAW_VERIFYING=0")
	if sweepWindow < 0 {
		t.Fatal("no sweep-counter window found in smoke-readonly.sh")
	}
	if menuPainted < 0 || menuPainted > sweepWindow {
		t.Errorf("the sweep-counter window opens before anything waits for the menu to "+
			"paint, so a slow connect spends the window connecting (menu-painted wait at "+
			"offset %d, window opens at %d)", menuPainted, sweepWindow)
	}
}
