// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package unit_test

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/web"
)

// The startup command is armed on every live web session, with or without
// the cache.
func TestWebSession_StartupCommandIsArmedWithoutCache(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	for name, noCache := range map[string]bool{"cached": false, "no-cache": true} {
		t.Run(name, func(t *testing.T) {
			if got := web.SessionCoreForTest(t, "example-readonly", "us-east-1", "ec2", false, noCache).Session().Command; got != "ec2" {
				t.Errorf("a live %s session armed %q as its startup command, want ec2", name, got)
			}
		})
	}
}
