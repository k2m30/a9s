package unit

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/tui"
)

// ReadClipboardAfter runs copyFn, which must perform the copy the test is
// asserting on, and returns the text the app handed to the clipboard.
//
// The copy is captured at the app's write seam rather than read back off the
// OS pasteboard. The pasteboard is one mutable object shared by every process
// on the machine, so a read-back asserts on whoever wrote last — another test
// in this binary, a test in a parallel worktree, or the human at the keyboard.
// Capturing means the assertion sees this copy and no other, and it holds on a
// headless machine with no pasteboard at all.
//
// A copy that never reached the seam fails: the empty string is not any
// expected content, and the caller's own assertion reports it.
func ReadClipboardAfter(t *testing.T, copyFn func()) string {
	t.Helper()

	var got string
	restore := tui.SetClipboardWriteForTest(func(s string) error {
		got = s
		return nil
	})
	defer restore()

	copyFn()

	return got
}
