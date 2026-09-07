// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package runtime

// failure_line.go — one voice for a failed call. Every error-history entry is
// built here, so a partial result, a region gap and a connect failure cannot
// read as three different kinds of event.

import (
	"time"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
)

// failureLine is the sentence a failed call gets: what it was about, then the
// cause read from the error's class and fields, with the region named when the
// class is about the region.
func failureLine(subject string, err error, region string) string {
	cause := awsclient.CauseInRegion(err, region)
	if subject == "" {
		return cause
	}
	return subject + ": " + cause
}

// appendErrorHistory is the only construction of an error-history entry. A
// caller that built the intent itself would be free to stamp a different time
// or phrase the same failure a second way.
func appendErrorHistory(message string) AppendErrorHistoryIntent {
	return AppendErrorHistoryIntent{Time: time.Now(), Message: message}
}

// softFailure reports whether a failed per-type call belongs in the `!` error
// log rather than in a blocking banner: rows already on screen say what the
// list is honestly showing, and a region gap is a fact about the account the
// operator cannot act on here.
func softFailure(err error, hasRows bool) bool {
	return hasRows || awsclient.ErrClass(err) == awsclient.ClassRegionUnavailable
}
