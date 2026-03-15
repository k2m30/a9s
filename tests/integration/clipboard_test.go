//go:build integration

package integration

import (
	"os"
	"testing"

	"github.com/atotto/clipboard"
)

// skipIfNoClipboard skips the test if clipboard access is not available
// (e.g., running in SSH, headless CI, or container environments).
func skipIfNoClipboard(t *testing.T) {
	t.Helper()
	if os.Getenv("SSH_CLIENT") != "" || os.Getenv("SSH_TTY") != "" {
		t.Skip("clipboard not available in SSH session")
	}
	if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
		// On macOS, clipboard works without DISPLAY. On Linux, it requires X11/Wayland.
		if os.Getenv("TERM_PROGRAM") == "" && os.Getenv("__CFBundleIdentifier") == "" {
			// Not macOS terminal -- likely headless Linux
			if _, err := os.Stat("/usr/bin/xclip"); os.IsNotExist(err) {
				if _, err := os.Stat("/usr/bin/xsel"); os.IsNotExist(err) {
					if _, err := os.Stat("/usr/bin/wl-copy"); os.IsNotExist(err) {
						t.Skip("no clipboard utility available (xclip, xsel, or wl-copy)")
					}
				}
			}
		}
	}
	// Quick smoke test: try to read clipboard
	if _, err := clipboard.ReadAll(); err != nil {
		t.Skipf("clipboard not available: %v", err)
	}
}

// QA-180: Clipboard write and read back
func TestQA_180_ClipboardWriteAndReadBack(t *testing.T) {
	skipIfNoClipboard(t)

	testContent := "a9s-integration-test-clipboard-content-12345"

	err := clipboard.WriteAll(testContent)
	if err != nil {
		t.Fatalf("clipboard.WriteAll failed: %v", err)
	}

	readBack, err := clipboard.ReadAll()
	if err != nil {
		t.Fatalf("clipboard.ReadAll failed: %v", err)
	}

	if readBack != testContent {
		t.Errorf("clipboard round-trip failed: wrote %q, read %q", testContent, readBack)
	}

	t.Log("clipboard write/read round-trip succeeded")
}

// QA-180b: SSH clipboard failure
// This test verifies that clipboard operations fail gracefully in
// environments where the clipboard is not available.
func TestQA_180b_SSHClipboardFailure(t *testing.T) {
	// This test is only meaningful when clipboard is NOT available.
	// If clipboard IS available, we just verify the library doesn't panic
	// when called.
	err := clipboard.WriteAll("test")
	if err != nil {
		t.Logf("clipboard.WriteAll returned error (expected in headless): %v", err)
		// The key thing: no panic occurred
	} else {
		t.Log("clipboard available; write succeeded (not an SSH/headless environment)")
	}
}
