// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// blessed_construction_test.go — the one constructor tests build a controller
// or a root model through.
//
// A test that builds a real app.Controller (directly, or transitively via
// tui.New) and then drives a ResourcesLoaded/EnrichmentChecked/
// AvailabilityChecked event through it can queue an async
// availability-cache save. If that goroutine outlives the test's t.TempDir()
// cleanup it writes into a directory os.RemoveAll is concurrently tearing
// down — and because the cache root is read live at write time, the leaked
// write lands wherever A9S_CONFIG_FOLDER points by the time the scheduler
// runs it, i.e. inside some other test's directory.
//
// Pairing the close with t.Cleanup here, at the one construction point, is
// what makes the ordering correct everywhere: t.Cleanup is LIFO, the caller
// resolves its temp directory before it constructs, so the close registered
// here always runs before that directory is removed. A call site that builds
// its own is one that can get the order wrong.
package unit

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/internal/tui"
)

// newBlessedController builds an app.Controller over core and closes it when
// the test ends.
func newBlessedController(t testing.TB, core *runtime.Core) *app.Controller {
	t.Helper()
	c := app.New(core)
	t.Cleanup(c.Close)
	return c
}

// newBlessedModel builds a root tui.Model and releases what it owns when the
// test ends: the app context its fetches ride on, and the controller whose
// cache writer would otherwise outlive the test.
func newBlessedModel(t testing.TB, profile, region string, opts ...tui.Option) tui.Model {
	t.Helper()
	m := tui.New(profile, region, opts...)
	t.Cleanup(func() {
		m.Cancel()
		m.CloseController()
	})
	return m
}
