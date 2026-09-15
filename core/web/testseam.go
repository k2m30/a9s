// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package web

import "testing"

// ArmedStartupCommandForTest builds a session the way the server does and
// returns the startup command armed on it.
func ArmedStartupCommandForTest(t *testing.T, profile, region, command string, demoMode, noCache bool) string {
	t.Helper()
	_, core := newSession(profile, region, command, demoMode, noCache, nil, "")
	return core.Session().Command
}
