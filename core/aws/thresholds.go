// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import "time"

// Age cut-offs: how long a fact must have held before a9s reports it. They
// are a9s's policy and not AWS facts, so each number lives here once, and a
// phrase that states one takes it through its <N> slot.
const (
	ssmStaleAfterDays         = 365
	secretStaleAfterDays      = 365
	secretDormantAfterDays    = 180
	roleDormantAfterDays      = 90
	unusedCredentialAfterDays = 90
	// emptyGroupAfterDays: a group old enough that being empty is a state,
	// not a group still being set up.
	emptyGroupAfterDays = 30
	// logsStaleEmptyAfterDays: an empty log group old enough that nothing is
	// writing to it.
	logsStaleEmptyAfterDays = 90
	backupJobLookbackHours  = 24
	// ebsOrphanAfterDays: an "available" volume unattached this long is billed
	// hourly for no workload.
	ebsOrphanAfterDays = 7
	// ebsSnapAgedAfterDays: an automated snapshot this old has no retention
	// policy pruning it.
	ebsSnapAgedAfterDays         = 365
	dbcSnapManualUnusedAfterDays = 365
	// tgwPendingAcceptanceGraceHours: how long a cross-account attachment may
	// sit unaccepted before it reads as stuck rather than in progress.
	tgwPendingAcceptanceGraceHours = 24
)

func days(n int) time.Duration { return time.Duration(n) * 24 * time.Hour }
