// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package runtime

import "time"

// SetFlashDurationsForTest overrides the flash auto-clear windows so tests that
// drain the full tea.Cmd chain do not block on the real 2 s / 5 s tea.Tick
// timers. Production never calls this; it exists for the tests/unit TestMain to
// shrink the windows once for the whole binary. Not concurrency-safe by design:
// call it before any test runs (TestMain), never mid-suite.
func SetFlashDurationsForTest(status, apiError time.Duration) {
	flashDuration = status
	apiErrorFlashDuration = apiError
}
