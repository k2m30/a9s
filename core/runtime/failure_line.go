// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package runtime

// failure_line.go — one voice for a failed call. Every sentence a failure is
// announced with is built here, so a partial result, a region gap and a
// connect failure cannot read as three different kinds of event.

import (
	"strings"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
)

// failureLine is the sentence a failed call gets: what it was about, then the
// cause read from the error's class and fields, with the region named when the
// class is about the region.
func failureLine(subject string, err error, region string) string {
	// An enricher that runs two passes joins their aggregates, and errors.Join
	// writes a newline between them. This is one line — a flash, a status bar,
	// a log entry — so the passes are separated the way causes are.
	cause := strings.ReplaceAll(awsclient.CauseInRegion(err, region), "\n", "; ")
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

// softFailure reports whether a failed per-type call belongs in the `!` error
// log rather than in a blocking banner: rows already on screen say what the
// list is honestly showing, and a region gap is a fact about the account the
// operator cannot act on here.
func softFailure(err error, hasRows bool) bool {
	return hasRows || awsclient.ErrClass(err) == awsclient.ClassRegionUnavailable
}
