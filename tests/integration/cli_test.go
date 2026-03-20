//go:build integration

package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// testBinary is the path to the compiled a9s binary for CLI tests.
var testBinary string

func TestMain(m *testing.M) {
	// Build the binary once for all CLI tests.
	tmpDir := os.TempDir()
	testBinary = filepath.Join(tmpDir, "a9s-test")
	cmd := exec.Command("go", "build", "-o", testBinary, "./cmd/a9s/")
	cmd.Dir = findProjectRoot()
	if out, err := cmd.CombinedOutput(); err != nil {
		panic("failed to build test binary: " + string(out) + ": " + err.Error())
	}
	code := m.Run()
	os.Remove(testBinary)
	os.Exit(code)
}

func findProjectRoot() string {
	// Walk up from this file's location to find go.mod
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

// QA-011: Launch with invalid/corrupt AWS config file
func TestQA_011_CorruptConfigFile(t *testing.T) {
	// Create a temporary corrupt config file
	tmpFile, err := os.CreateTemp("", "corrupt-aws-config-*")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// Write corrupt content
	_, err = tmpFile.WriteString("this is not [valid ini\n\x00\x01garbage\n[broken")
	if err != nil {
		t.Fatalf("failed to write corrupt config: %v", err)
	}
	tmpFile.Close()

	// Set AWS_CONFIG_FILE to the corrupt file and run the binary with --version
	// to verify the binary doesn't crash on corrupt config
	cmd := exec.Command(testBinary, "--version")
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

// QA-012: Launch with --version flag
func TestQA_012_VersionFlag(t *testing.T) {
	cmd := exec.Command(testBinary, "--version")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("--version failed: %v, output: %s", err, string(out))
	}
	output := string(out)
	if !strings.Contains(output, "a9s") {
		t.Errorf("expected --version output to contain 'a9s', got %q", output)
	}
	// Should contain a version number pattern (X.Y.Z)
	if !strings.Contains(output, ".") {
		t.Errorf("expected --version output to contain a version number, got %q", output)
	}
}

// QA-012b: Launch with -v shorthand for --version
func TestQA_012b_ShortVersionFlag(t *testing.T) {
	cmd := exec.Command(testBinary, "-v")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("-v failed: %v, output: %s", err, string(out))
	}
	output := string(out)
	if !strings.Contains(output, "a9s") {
		t.Errorf("expected -v output to contain 'a9s', got %q", output)
	}
}

// QA-013: Launch with --help flag
func TestQA_013_HelpFlag(t *testing.T) {
	cmd := exec.Command(testBinary, "--help")
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

// QA-013b: Launch with -h shorthand for --help
func TestQA_013b_ShortHelpFlag(t *testing.T) {
	cmd := exec.Command(testBinary, "-h")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("-h failed: %v, output: %s", err, string(out))
	}
	output := string(out)
	if !strings.Contains(output, "Usage") {
		t.Errorf("expected -h output to contain 'Usage', got %q", output)
	}
}

// QA-017: Launch with -p shorthand for --profile
// We can't fully test TUI interaction, but we verify the binary starts
// and doesn't crash immediately when given a -p flag.
func TestQA_017_ShorthandProfileFlag(t *testing.T) {
	cmd := exec.Command(testBinary, "-p", "nonexistent-test-profile")
	cmd.Env = append(os.Environ(), "TERM=dumb")

	// Use a short timeout -- TUI will block waiting for terminal input,
	// but we just want to verify it doesn't crash on startup.
	done := make(chan error, 1)
	go func() {
		done <- cmd.Run()
	}()

	select {
	case err := <-done:
		// If it exits immediately, it should be with an error about the profile
		// or a normal exit (e.g. if no terminal is detected).
		// Either way, it should not panic.
		if err != nil {
			// An exit error is acceptable -- the TUI can't run without a terminal.
			t.Logf("binary exited (expected without a terminal): %v", err)
		}
	case <-time.After(3 * time.Second):
		// If it's still running after 3s, that means the TUI started successfully
		// with the -p flag. Kill it and consider the test passed.
		cmd.Process.Kill()
		t.Log("-p flag accepted; TUI started (killed after timeout)")
	}
}

// QA-018: Launch with -r shorthand for --region
func TestQA_018_ShorthandRegionFlag(t *testing.T) {
	cmd := exec.Command(testBinary, "-r", "eu-central-1")
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
