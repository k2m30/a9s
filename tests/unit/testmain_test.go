package unit

import (
	"os"
	"testing"
	"time"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
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
	// Same idea for AWS retry backoff: keep MaxAttempts so retry LOGIC is still
	// exercised, but shrink the 500ms BaseDelay that throttle/server-error tests
	// (e.g. TestFetchKMSKeysPage_*) were paying for real.
	awsclient.SetRetryConfigForTest(&awsclient.RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   time.Millisecond,
		MaxDelay:    10 * time.Millisecond,
		Jitter:      false,
	})
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
	// ~/.a9s cache and config — e.g. newRootSizedModel (tuitest.Sized ->
	// tui.New), newEC2ListModel/newEC2DetailModel/newEC2YAMLModel
	// (tui.New directly). A live ./a9s pilot run mutating that real
	// directory mid-suite previously made env-dependent assertions
	// (loading-state text, flash-message content, detail-view rows) flap
	// depending on what was left on disk. A per-test A9S_CONFIG_FOLDER
	// override (via t.Setenv) still wins for that test's duration and is
	// unaffected by this default — this only closes the gap for
	// constructors that never redirected it at all.
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
	}
	code := m.Run()
	// newRootSizedModel (tui_root_test.go) closes the PREVIOUS call's
	// controller on every new call, but the very LAST model it ever built in
	// this run has no later call to trigger that drain — flush it here,
	// before cleanupDir (which its own auto-isolated directory may still be,
	// or may itself be a still-live A9S_CONFIG_FOLDER target) is removed.
	if prevRootModel != nil {
		prevRootModel.CloseController()
		prevRootModel = nil
	}
	if cleanupDir != "" {
		os.RemoveAll(cleanupDir) //nolint:errcheck // best-effort cleanup, process is exiting regardless
	}
	os.Exit(code)
}
