// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// Package aws — testing helper for retry configuration.
package aws

import "time"

// SetRetryMaxBackoffForTest caps the wait between two attempts of every
// client CreateServiceClients builds afterwards, and returns a function that
// restores the previous cap. The attempt budget and the retry decision stay
// as production has them; only the wall-clock wait of a retry shrinks. The cap
// is process-wide and read when clients are built, so a test that sets it must
// not run in parallel with a test that builds clients.
func SetRetryMaxBackoffForTest(d time.Duration) (restore func()) {
	prev := retryMaxBackoffForTest.Swap(int64(d))
	return func() { retryMaxBackoffForTest.Store(prev) }
}
