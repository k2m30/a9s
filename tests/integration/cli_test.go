//go:build integration

package integration

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
)

var testBinary string

func TestMain(m *testing.M) {
	// Install the AWS catalog before any in-process integration test calls
	// into catalog.Find / catalog.All (transitively via tui.New, etc.).
	aws.Install()
	resource.WireProjection()

	// Package-wide hermetic default: a scenario that presses a copy key
	// would otherwise overwrite whatever the developer had on the
	// pasteboard, and race every other binary doing the same. Tests that
	// assert on the copied text install their own capture over this one.
	tui.SetClipboardWriteForTest(func(string) error { return nil })

	// Build the binary once for all CLI tests. Stamp main.version via
	// ldflags so TestQA_012_VersionFlag's X.Y.Z assertion holds — without
	// ldflags, buildinfo.ResolveVersion falls back to "dev" and `--version`
	// prints "a9s dev" with no "." in it.
	//
	// The binary lives in a directory of this run's own, never at a fixed
	// name under $TMPDIR: several worktrees run this suite at once, and a
	// shared path means one run's TestMain deletes the binary another run is
	// still executing.
	buildDir, err := os.MkdirTemp("", "a9s-cli-test-*")
	if err != nil {
		panic("failed to create the test binary directory: " + err.Error())
	}
	testBinary = filepath.Join(buildDir, "a9s-test")
	cmd := exec.CommandContext(context.Background(), "go", "build", "-ldflags", "-X main.version=test-0.0.0", "-o", testBinary, "./cmd/a9s/")
	cmd.Dir = findProjectRoot()
	if out, err := cmd.CombinedOutput(); err != nil {
		panic("failed to build test binary: " + string(out) + ": " + err.Error())
	}
	code := m.Run()
	os.RemoveAll(buildDir) //nolint:errcheck // best-effort cleanup, the process is exiting
	os.Exit(code)
}

func findProjectRoot() string {
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "."
}

func TestQA_011_CorruptConfigFile(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "corrupt-aws-config-*")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString("this is not [valid ini\n\x00\x01garbage\n[broken")
	if err != nil {
		t.Fatalf("failed to write corrupt config: %v", err)
	}
	tmpFile.Close()

	cmd := exec.CommandContext(t.Context(), testBinary, "--version")
	cmd.Env = append(os.Environ(),
		"AWS_CONFIG_FILE="+tmpFile.Name(),
		"AWS_SHARED_CREDENTIALS_FILE=/nonexistent",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("binary crashed with corrupt config: %v, output: %s", err, string(out))
	}
	if !strings.Contains(string(out), "a9s") {
		t.Errorf("expected output to contain 'a9s', got %q", string(out))
	}
}

func TestQA_012_VersionFlag(t *testing.T) {
	cmd := exec.CommandContext(t.Context(), testBinary, "--version")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("--version failed: %v, output: %s", err, string(out))
	}
	output := string(out)
	if !strings.Contains(output, "a9s") {
		t.Errorf("expected --version output to contain 'a9s', got %q", output)
	}
	if !strings.Contains(output, ".") {
		t.Errorf("expected --version output to contain a version number, got %q", output)
	}
}

func TestQA_012b_ShortVersionFlag(t *testing.T) {
	cmd := exec.CommandContext(t.Context(), testBinary, "-v")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("-v failed: %v, output: %s", err, string(out))
	}
	output := string(out)
	if !strings.Contains(output, "a9s") {
		t.Errorf("expected -v output to contain 'a9s', got %q", output)
	}
}

func TestQA_013_HelpFlag(t *testing.T) {
	cmd := exec.CommandContext(t.Context(), testBinary, "--help")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("--help failed: %v, output: %s", err, string(out))
	}
	output := string(out)
	if !strings.Contains(output, "Usage") {
		t.Errorf("expected --help output to contain 'Usage', got %q", output)
	}
	if !strings.Contains(output, "--profile") {
		t.Errorf("expected --help output to contain '--profile', got %q", output)
	}
	if !strings.Contains(output, "--region") {
		t.Errorf("expected --help output to contain '--region', got %q", output)
	}
}

func TestQA_013b_ShortHelpFlag(t *testing.T) {
	cmd := exec.CommandContext(t.Context(), testBinary, "-h")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("-h failed: %v, output: %s", err, string(out))
	}
	output := string(out)
	if !strings.Contains(output, "Usage") {
		t.Errorf("expected -h output to contain 'Usage', got %q", output)
	}
}

func TestQA_017_ShorthandProfileFlag(t *testing.T) {
	cmd := exec.CommandContext(t.Context(), testBinary, "-p", "nonexistent-test-profile")
	cmd.Env = append(os.Environ(), "TERM=dumb")

	// The TUI blocks on terminal input, so a process still alive after the
	// timeout started without crashing.
	done := make(chan error, 1)
	go func() {
		done <- cmd.Run()
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Logf("binary exited (expected without a terminal): %v", err)
		}
	case <-time.After(3 * time.Second):
		cmd.Process.Kill()
		t.Log("-p flag accepted; TUI started (killed after timeout)")
	}
}

func TestQA_018_ShorthandRegionFlag(t *testing.T) {
	cmd := exec.CommandContext(t.Context(), testBinary, "-r", "eu-central-1")
	cmd.Env = append(os.Environ(), "TERM=dumb")

	done := make(chan error, 1)
	go func() {
		done <- cmd.Run()
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Logf("binary exited (expected without a terminal): %v", err)
		}
	case <-time.After(3 * time.Second):
		cmd.Process.Kill()
		t.Log("-r flag accepted; TUI started (killed after timeout)")
	}
}
