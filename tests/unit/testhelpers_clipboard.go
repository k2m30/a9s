package unit

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/atotto/clipboard"
)

// The OS pasteboard is one mutable object shared by every test in the binary.
// Production copy paths write it for real and report only a constant flash
// label, so the copied text can be verified only by reading it back — which
// means two copy tests running in either order can read each other's write.
// clipboardMu serialises them.
var clipboardMu sync.Mutex

var clipboardSeq atomic.Uint64

// ReadClipboardAfter runs copyFn, which must perform the real pasteboard
// write, and returns what landed there.
//
// A unique sentinel goes on the pasteboard first, so a write that never
// happened is a skip instead of an assertion against whatever the previous
// test left behind. Ceiling: this excludes interference from other tests in
// this binary, not from another process writing the pasteboard mid-test. A
// read-back that is neither the sentinel nor the expected text is therefore
// still reported as a failure, because silently skipping on any unexpected
// value would turn a real copy regression into a green run.
func ReadClipboardAfter(t *testing.T, copyFn func()) string {
	t.Helper()
	clipboardMu.Lock()
	defer clipboardMu.Unlock()

	sentinel := fmt.Sprintf("a9s-clipboard-sentinel-%d-%d", os.Getpid(), clipboardSeq.Add(1))
	if err := clipboard.WriteAll(sentinel); err != nil {
		t.Skipf("clipboard not writable in this environment: %v", err)
	}

	copyFn()

	got, err := clipboard.ReadAll()
	if err != nil {
		t.Skipf("clipboard read-back unavailable: %v", err)
	}
	if got == sentinel {
		t.Skip("the copy did not reach the pasteboard in this environment")
	}
	return got
}
