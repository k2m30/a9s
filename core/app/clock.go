// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package app

import "time"

// Now is this package's read of the wall clock. Production calls it wherever
// it needs the current instant; SetNowForTest pins it so a scenario that
// seeds a date-dependent screen (the costs window) renders the same output on
// any day. The costs code itself keeps taking an injected now — this is only
// the seam at the lane entrance where that now is first produced.
func Now() time.Time { return nowFn() }

var nowFn = time.Now

// SetNowForTest replaces the clock Now reads and returns a function that
// restores the previous one. Not concurrency-safe by design, same as the
// package's other test seams: set it before the scenario runs, restore after.
func SetNowForTest(fn func() time.Time) func() {
	prev := nowFn
	nowFn = fn
	return func() { nowFn = prev }
}
