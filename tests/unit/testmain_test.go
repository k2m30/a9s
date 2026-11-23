package unit

import (
	"fmt"
	"os"
	"testing"
	"time"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/config"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
)

func TestMain(m *testing.M) {
	// Shrink the flash auto-clear windows: tests that drain the full tea.Cmd
	// chain would otherwise block on the real 2 s / 5 s tea.Tick timers. The
	// flash still fires and still clears — only the wall-clock shrinks — so no
	// assertion changes, only ~30-40s of suite time is reclaimed.
	runtime.SetFlashDurationsForTest(time.Millisecond, time.Millisecond)
	// Same idea for the SDK retryer's backoff: the attempt budget and the
	// retry decision stay, only the wait a throttle/server-error test would
	// sit through shrinks.
	awsclient.SetRetryMaxBackoffForTest(time.Millisecond)
	// styles holds package-level vars rebuilt by styles.ReinitForTest(), which reads
	// NO_COLOR at call time. The invoking shell's NO_COLOR must not change
	// which SGR assertions pass in this binary, so the baseline is normalized
	// to "colors on" before any test runs.
	if _, noColorSet := os.LookupEnv("NO_COLOR"); noColorSet {
		os.Unsetenv("NO_COLOR") //nolint:errcheck // best-effort hermetic default, not test-critical
		styles.ReinitForTest()
	}
	// Package-wide hermetic default: any test constructor in this binary
	// (package unit AND package unit_test share this one TestMain) that
	// builds a tui.Model/session without its own t.Setenv("A9S_CONFIG_FOLDER",
	// t.TempDir()) call would otherwise read/write the developer's REAL
	// ~/.a9s cache and config. A per-test A9S_CONFIG_FOLDER override (via t.Setenv)
	// still wins for that test's duration.
	var cleanupDir string
	if os.Getenv("A9S_CONFIG_FOLDER") == "" {
		if dir, err := os.MkdirTemp("", "a9s-testmain-config-*"); err == nil {
			os.Setenv("A9S_CONFIG_FOLDER", dir) //nolint:errcheck // best-effort hermetic default, not test-critical
			cleanupDir = dir
		}
	}
	// Package-wide hermetic default, same reasoning as A9S_CONFIG_FOLDER
	// above: any test in this binary that drives a copy key and drains the
	// returned cmd would otherwise overwrite whatever the developer had on
	// the pasteboard, and would race every other test in every parallel
	// worktree that does the same. Tests that assert on the copied text
	// install their own capture over this one (ReadClipboardAfter).
	tui.SetClipboardWriteForTest(func(string) error { return nil })

	// TEST_SKIP_INSTALL=1 lets sub-process tests exercise the
	// panic-before-SetTypes path without triggering a bootstrap here.
	if os.Getenv("TEST_SKIP_INSTALL") != "1" {
		awsclient.Install()
		resource.WireProjection()
		// The default views are built once, from the child types registered at
		// the first call. Built here, before any test registers one of its own,
		// a test's child type cannot become a default view for every later test.
		config.SharedDefaultConfig()
	}
	fixturesChanged := fixtures.MutationCheckForTest()
	code := m.Run()
	// Every demo fixture consumer gets its own copy; a shared set or exported
	// fixture variable that differs after the run was written through a path
	// that bypasses the copy, and every test after the writer read it.
	if changed := fixturesChanged(); len(changed) > 0 {
		fmt.Fprintf(os.Stderr, "demo fixtures changed during the run: %v\n", changed)
		code = 1
	}
	// The per-test check sees only goroutines labelled by a blessed
	// constructor; this one sees a controller built any other way.
	if leaked := CheckNoProductionGoroutines(); leaked != "" {
		fmt.Fprintf(os.Stderr, "a9s goroutines still running after the last test:\n%s\n", leaked)
		code = 1
	}
	if cleanupDir != "" {
		os.RemoveAll(cleanupDir) //nolint:errcheck // best-effort cleanup, process is exiting regardless
	}
	os.Exit(code)
}
