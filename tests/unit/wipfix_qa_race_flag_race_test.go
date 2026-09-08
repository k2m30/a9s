// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

//go:build race

package unit

// wipfixRaceDetector reports whether this binary was built with -race. See the
// !race twin for why the timing pins read it.
const wipfixRaceDetector = true
