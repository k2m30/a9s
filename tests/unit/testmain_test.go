package unit

import (
	"os"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func TestMain(m *testing.M) {
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
	// TEST_SKIP_INSTALL=1 lets sub-process tests exercise the
	// panic-before-SetTypes path without triggering a bootstrap here.
	if os.Getenv("TEST_SKIP_INSTALL") != "1" {
		awsclient.Install()
		resource.WireProjection()
	}
	code := m.Run()
	if cleanupDir != "" {
		os.RemoveAll(cleanupDir) //nolint:errcheck // best-effort cleanup, process is exiting regardless
	}
	os.Exit(code)
}
