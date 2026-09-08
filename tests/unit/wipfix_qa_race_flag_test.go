// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

//go:build !race

package unit

// wipfixRaceDetector reports whether this binary was built with -race. The
// detector instruments every memory access, so a timing pin needs its own
// budget under it — measured the same way as the ordinary one, never guessed
// from the ordinary one.
const wipfixRaceDetector = false
