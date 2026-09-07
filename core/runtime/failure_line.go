// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package runtime

// failure_line.go — one voice for a failed call. Every error-history entry is
// built here, so a partial result, a region gap and a connect failure cannot
// read as three different kinds of event.

import (
	"strings"
	"time"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
)

// failureLine is the sentence a failed call gets: what it was about, then the
// cause read from the error's class and fields, with the region named when the
// class is about the region.
func failureLine(subject string, err error, region string) string {
	cause := awsclient.CauseInRegion(err, region)
	// Three callers build the subject as a prefix plus a resource type, and
	// the type can be empty: an unregistered fetch, a detail enrichment whose
	// event carries none, a related result with no target.
	if subject = strings.TrimSpace(subject); subject == "" {
		return cause
	}
	// A partial-batch cause is labelled with the resource type by the fetcher
	// that built it, and the subject the handler passes ends with that same
	// type: "availability ec2: ec2: DescribeInstances failed for ...". The
	// qualifier is what the subject adds; the type is already said.
	if qualifier, typ, ok := strings.Cut(subject, " "); ok && strings.HasPrefix(cause, typ+":") {
		return qualifier + " " + cause
	}
	if strings.HasPrefix(cause, subject+":") {
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
