// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package unit_test

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/web"
)

// A demo session answers --no-cache the way the terminal does
// (cmd/a9s/main.go passes the flag through for demo too): with the flag off
// the cache stays on, so the demo exercises the cache-load-then-sweep chain
// the installed app runs for a real operator.
func TestWebSession_DemoHonoursNoCacheFlag(t *testing.T) {
	for name, noCache := range map[string]bool{"cached": false, "no-cache": true} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
			core := web.SessionCoreForTest(t, demo.DemoProfile, demo.DemoRegion, "", true, noCache)
			if got := core.NoCache(); got != noCache {
				t.Errorf("a demo web session started with noCache=%v has NoCache() = %v, want %v", noCache, got, noCache)
			}
		})
	}
}
