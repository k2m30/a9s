//go:build integration

// scenario_console_open_test.go — demo-mode smoke coverage for the
// "o = open in AWS console / O = copy console URL" feature (spec:
// console-url-spec.md). Uses the scripted scenario harness
// (SCENARIO_HARNESS.md) rather than a PTY/subprocess launch of ./a9s
// --demo: the harness drives tui.Model.Update() directly and executes the
// returned tea.Cmd values, which is sufficient to prove (a) the demo-mode
// flash renders on "o" and no real browser process is ever spawned, and (b)
// "O" produces a copy-confirmation flash and (where the environment
// supports reading the system clipboard) that the copied value really is
// an https console URL.
//
// Uses the shared fullIntegrationNewDemoScenario constructor directly — it
// now sets tui.WithIsDemo(true) (see scripted_scenario_helpers_test.go), so
// this file no longer needs its own model-construction workaround.
package integration

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestConsoleOpen_Demo_KeyFlashesDisabledAndNeverExecsBrowser points $BROWSER
// at a sentinel script that would create a marker file if ever executed,
// then presses "o" on a loaded demo resource list. Demo mode must short-
// circuit to the disabled-link flash before ever reaching openBrowserCmd, so
// the marker must never appear — this is a regression guard against that
// short-circuit ever being accidentally removed or reordered. The marker
// check runs before the flash-text assertion so a wording drift in the
// flash copy can never mask a real (and far more serious) no-exec
// regression.
func TestConsoleOpen_Demo_KeyFlashesDisabledAndNeverExecsBrowser(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sentinel script uses a POSIX shebang; not portable to this harness on windows")
	}

	dir := t.TempDir()
	marker := filepath.Join(dir, "opened.marker")
	sentinel := filepath.Join(dir, "fake-browser.sh")
	script := "#!/bin/sh\ntouch \"" + marker + "\"\n"
	if err := os.WriteFile(sentinel, []byte(script), 0o755); err != nil { //nolint:gosec // test-owned temp script, execute bit required
		t.Fatalf("failed to write sentinel browser script: %v", err)
	}
	t.Setenv("BROWSER", sentinel)

	scenario := fullIntegrationNewDemoScenario(t)
	scenario.OpenList("ec2")

	// Not scenario.Press("o"): Press calls applyAndDrain, which recursively
	// executes every returned tea.Cmd — including the flash's real
	// auto-clear timer (handleFlash's ApplyIntents-driven ClearFlash tick).
	// Draining that timer for real (tests/integration's TestMain does not
	// shrink flash durations the way tests/unit's does) sleeps out the
	// flash's real lifetime and clears it before any assertion could see
	// it, racing a false "no flash" failure regardless of wording. A single
	// non-draining applyMsg captures the flash text (set synchronously
	// during Update(), before its cmd is ever invoked) with no race, then
	// the returned cmd is drained explicitly afterward — purely to prove,
	// for real, that doing so never touches $BROWSER.
	cmd := scenario.applyMsg(fullIntegrationScenarioKeyPress(t, "o"))

	scenario.ExpectNoAPIError()
	scenario.ExpectViewContains("demo mode — console link disabled (O still copies)")

	scenario.drainCmd(cmd)
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("sentinel marker file exists — 'o' in demo mode must never exec $BROWSER")
	} else if !os.IsNotExist(err) {
		t.Fatalf("unexpected error checking sentinel marker file: %v", err)
	}
}

// TestConsoleOpen_Demo_UppercaseOCopiesAnHTTPSConsoleURL presses "O" on a
// loaded demo resource list and asserts the value that reaches the pasteboard
// is an https console URL. The read-back goes through readClipboardAfter so a
// sibling test's copy cannot be mistaken for this one's.
func TestConsoleOpen_Demo_UppercaseOCopiesAnHTTPSConsoleURL(t *testing.T) {
	scenario := fullIntegrationNewDemoScenario(t)
	scenario.OpenList("ec2")

	got := readClipboardAfter(t, func() {
		scenario.Press("O")

		if scenario.lastFlash == nil {
			t.Fatal("pressing 'O' should produce a flash confirming the console-URL copy")
		}
		scenario.ExpectNoAPIError()
	})
	if !strings.HasPrefix(got, "https://") || !strings.Contains(got, "console.aws.amazon.com") {
		t.Errorf("clipboard content after 'O' = %q, want an https://...console.aws.amazon.com/... URL", got)
	}
}
