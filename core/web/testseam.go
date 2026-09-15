// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package web

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/runtime"
)

// SessionCoreForTest builds a session the way the server does and returns its
// runtime core.
func SessionCoreForTest(t *testing.T, profile, region, command string, demoMode, noCache bool) *runtime.Core {
	t.Helper()
	_, core := newSession(profile, region, command, demoMode, noCache, nil, "")
	return core
}
