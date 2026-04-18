package unit

import (
	"testing"

	tui "github.com/k2m30/a9s/v3/internal/tui"
)

// withTuiVersion sets tui.Version for the duration of t and restores the
// previous value via t.Cleanup. Safe for tests that exercise version-string
// rendering without leaking package-global state across tests.
func withTuiVersion(t *testing.T, v string) {
	t.Helper()
	prev := tui.Version
	tui.Version = v
	t.Cleanup(func() { tui.Version = prev })
}
