// SPDX-License-Identifier: GPL-3.0-or-later

// core/cache's skip-report sites route through the opt-in core/logging
// facility rather than log.Printf, which writes to stderr while the TUI owns
// the terminal. LoadDirIn's skip sites call
// logging.L().Warn("cache skip", "file", <path>, "reason", <error text>).
package unit_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/cache"
	"github.com/k2m30/a9s/v3/core/logging"
)

// resetLoggingToDisabled is registered first (t.Cleanup runs LIFO, so it
// fires last) in every test here that enables the facility. Packages unit and
// unit_test share one test binary and one process-wide core/logging
// singleton, so a test that leaves Enabled()==true would redirect later
// log.L().Warn calls into its deleted t.TempDir() path.
func resetLoggingToDisabled(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		_, _ = logging.Setup("")
	})
}

// writeCorruptTypeFile creates <root>/<profile>--<region>/<shortName>.yaml
// containing YAML that fails to unmarshal into cache.TypeFile, at the layout
// cache.DirIn computes.
func writeCorruptTypeFile(t *testing.T, root, profile, region, shortName string) string {
	t.Helper()
	pairDir := filepath.Join(root, cache.SanitizePathElem(profile)+"--"+cache.SanitizePathElem(region))
	if err := os.MkdirAll(pairDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", pairDir, err)
	}
	path := filepath.Join(pairDir, shortName+".yaml")
	if err := os.WriteFile(path, []byte("not: [valid: yaml"), 0o600); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
	return path
}

func TestLoggingFacility_Setup_TempPath_CacheSkipRoutesToLogFile(t *testing.T) {
	resetLoggingToDisabled(t)

	logPath := filepath.Join(t.TempDir(), "a9s.log")
	closeFn, err := logging.Setup(logPath)
	if err != nil {
		t.Fatalf("Setup(%q) returned error: %v", logPath, err)
	}
	defer func() { _ = closeFn() }()

	if !logging.Enabled() {
		t.Fatal("Enabled() = false after Setup with a valid temp path, want true")
	}
	if logging.L() == nil {
		t.Fatal("L() returned nil after a successful Setup")
	}

	root := t.TempDir()
	corruptPath := writeCorruptTypeFile(t, root, "profile-a", "us-east-1", "ec2")

	cache.LoadDirIn(root, "profile-a", "us-east-1")

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", logPath, err)
	}
	content := string(data)
	if !strings.Contains(content, "cache skip") {
		t.Errorf("log file missing a \"cache skip\" line, got:\n%s", content)
	}
	if !strings.Contains(content, filepath.Base(corruptPath)) {
		t.Errorf("log file's cache-skip line does not name the corrupt file (%s), got:\n%s", filepath.Base(corruptPath), content)
	}
}

// TestLoggingFacility_SetupEmptyPath_DisablesAndWritesNoFile pins the
// no-file half of the disabled contract directly: a candidate log path that
// would receive lines were the facility enabled must stay absent after
// Setup("") and a cache skip through the real load path.
func TestLoggingFacility_SetupEmptyPath_DisablesAndWritesNoFile(t *testing.T) {
	resetLoggingToDisabled(t)

	logDir := t.TempDir()
	candidateLogPath := filepath.Join(logDir, "a9s.log")

	closeFn, err := logging.Setup("")
	if err != nil {
		t.Fatalf("Setup(\"\") returned error: %v", err)
	}
	defer func() { _ = closeFn() }()

	if logging.Enabled() {
		t.Fatal("Enabled() = true after Setup(\"\"), want false")
	}

	root := t.TempDir()
	writeCorruptTypeFile(t, root, "profile-b", "us-west-2", "s3")

	cache.LoadDirIn(root, "profile-b", "us-west-2")

	if _, statErr := os.Stat(candidateLogPath); !os.IsNotExist(statErr) {
		t.Errorf("Setup(\"\") must write no file anywhere: %q exists (stat err=%v)", candidateLogPath, statErr)
	}
}

// TestLoggingFacility_SetupEmptyPath_StopsWritesToThePreviouslyEnabledPath
// pins "unset produces no file at the previously-used path": a path that WAS
// receiving lines while enabled must receive no NEW bytes once Setup("")
// disables the facility, even though the file itself (from the earlier
// enabled write) still exists on disk.
func TestLoggingFacility_SetupEmptyPath_StopsWritesToThePreviouslyEnabledPath(t *testing.T) {
	resetLoggingToDisabled(t)

	logPath := filepath.Join(t.TempDir(), "a9s.log")
	closeFn, err := logging.Setup(logPath)
	if err != nil {
		t.Fatalf("Setup(%q) returned error: %v", logPath, err)
	}

	root := t.TempDir()
	writeCorruptTypeFile(t, root, "profile-c", "eu-west-1", "vpc")
	cache.LoadDirIn(root, "profile-c", "eu-west-1")
	_ = closeFn()

	before, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) after the enabled load: %v", logPath, err)
	}

	disableFn, err := logging.Setup("")
	if err != nil {
		t.Fatalf("Setup(\"\") returned error: %v", err)
	}
	defer func() { _ = disableFn() }()

	root2 := t.TempDir()
	writeCorruptTypeFile(t, root2, "profile-c", "eu-west-1", "sg")
	cache.LoadDirIn(root2, "profile-c", "eu-west-1")

	after, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) after the disabled load: %v", logPath, err)
	}
	if string(after) != string(before) {
		t.Errorf("a cache skip while the facility is disabled must add nothing to the previously-used log file; before=%dB after=%dB\nbefore:\n%s\nafter:\n%s", len(before), len(after), before, after)
	}
}

func TestLoggingFacility_SetupUnopenablePath_ReturnsErrorAndStaysDisabled(t *testing.T) {
	resetLoggingToDisabled(t)

	// Prime an enabled state first so this test can observe Setup actually
	// flipping back to disabled on failure, not merely inheriting disabled
	// state left over from a sibling test.
	primePath := filepath.Join(t.TempDir(), "a9s.log")
	if primeClose, primeErr := logging.Setup(primePath); primeErr != nil {
		t.Fatalf("priming Setup(%q) returned error: %v", primePath, primeErr)
	} else {
		defer func() { _ = primeClose() }()
	}
	if !logging.Enabled() {
		t.Fatal("priming Setup call did not enable the facility")
	}

	// os.OpenFile never creates missing parent directories, so a path under
	// a nonexistent parent is unopenable without any real permission
	// manipulation — deterministic and sandbox-safe.
	unopenable := filepath.Join(t.TempDir(), "does-not-exist", "nested", "a9s.log")
	closeFn, err := logging.Setup(unopenable)
	if err == nil {
		if closeFn != nil {
			_ = closeFn()
		}
		t.Fatalf("Setup(%q) returned nil error for an unopenable path (missing parent dir)", unopenable)
	}
	if logging.Enabled() {
		t.Error("Enabled() = true after a failed Setup call, want false (state must stay disabled)")
	}
}
