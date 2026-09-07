// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// Package aws — partial_errors.go
//
// AggregateFailures and AggregateMissing implement the canonical composite
// error format for partial-batch AWS operations. Every fetcher, checker, and
// enricher that iterates and describes per-item MUST use one of these helpers
// to surface per-item failures while preserving partial success (silent-skip
// ban, error-handling rules E2, E3, E5).
//
// Why this exists: before this helper, each FetchByIDs function inlined its
// own composite-error format. Any divergence in format caused test assertions
// to break when the wording drifted. Centralising here guarantees a single
// format that tests can pin on.
package aws

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net"
	"slices"
	"strings"

	"github.com/aws/smithy-go"
)

// IsEndpointNotFound reports whether err is a DNS resolution failure for a
// service endpoint host — the signature of a service that is not offered in
// the selected region (e.g. CodeArtifact in eu-central-2: dial tcp lookup
// codeartifact.eu-central-2.amazonaws.com: no such host). Callers use this to
// render "service not available in region <r>" instead of raw transport
// jargon, and to log-only rather than banner. Deliberately narrow: only
// *net.DNSError with IsNotFound qualifies — offline networks and flaky DNS
// fail differently and must stay loud.
func IsEndpointNotFound(err error) bool {
	var dnsErr *net.DNSError
	return errors.As(err, &dnsErr) && dnsErr.IsNotFound
}

// IsNotFoundErr reports whether err is the canonical "resource no longer
// exists" response for a per-ID describe call. This is the classification
// contract every fetcher, checker, and enricher in this package must use for
// that case: a resource that existed at list time but was deleted before the
// per-ID describe call landed is an operational race, not a failure —
// callers mark the ID truncated (data incomplete), emit no finding, and keep
// it out of the failure aggregate. It is never surfaced as a hard error or a
// false-positive finding (e.g. "no encryption configured" on a bucket that
// no longer exists).
//
// Unwraps via errors.As to smithy.APIError, so both a typed SDK error (e.g.
// route53 types.NoSuchHostedZone, which implements smithy.APIError) and the
// generic smithy.GenericAPIError shape are covered by the same ErrorCode
// match — classification is code-based, never a message substring scan.
// "NotFound" also covers S3's empty-body-404 synthesis from HTTP status
// text, which carries no modeled error shape of its own.
func IsNotFoundErr(err error) bool {
	var apiErr smithy.APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	switch apiErr.ErrorCode() {
	case "NoSuchBucket", "NotFound", "NoSuchHostedZone", "ResourceNotFoundException", "InvalidInstanceID.NotFound",
		// RDS and DocumentDB spell the same race with their own codes; a
		// snapshot deleted between the list call and a per-snapshot
		// describe answers one of these.
		"DBSnapshotNotFound", "DBClusterSnapshotNotFoundFault":
		return true
	default:
		return false
	}
}

// aggregateFailuresCap is the maximum number of distinct causes
// AggregateFailures names before summarizing the rest. Failures are grouped by
// cause, so this bounds the line by how many DIFFERENT things went wrong, not
// by how many resources they happened to.
const aggregateFailuresCap = 5

// AggregateFailures builds the canonical composite error for a partial-batch
// operation. opName is the operation label (e.g. "kms FetchByIDs",
// "policy FetchByIDs", "ecs-task ListTargets"). failures is a slice of
// "<id>: <reason>" strings collected during iteration. total is the total
// number of items attempted (failures + successes).
//
// Returns nil when len(failures) == 0 so callers can write:
//
//	return resources, AggregateFailures(op, failures, total)
//
// without an extra conditional.
//
// This is the one place a partial-batch failure is phrased. A role denied one
// action fails on every resource of the type at once, so the failures are
// grouped by cause and each cause is stated once, with how many resources it
// covered and one example:
//
//	"<op> failed for 46 of 46 IDs: not authorized to perform ec2:DescribeSnapshotAttribute (e.g. snap-0abc)"
//	"<op> failed for 3 of 9 IDs: context deadline exceeded (2, e.g. snap-1); no metadata (1, e.g. snap-3)"
//
// Above aggregateFailuresCap distinct causes the rest are summarized as
// "; and N more causes".
func AggregateFailures(opName string, failures []string, total int) error {
	if len(failures) == 0 {
		return nil
	}

	type group struct {
		cause   string
		example string
		n       int
	}
	var groups []*group
	byCause := map[string]*group{}
	for _, f := range failures {
		// AggregateFailures' documented input shape is "<id>: <reason>"; an
		// entry with no separator is all reason and contributes no example.
		id, reason, ok := strings.Cut(f, ": ")
		if !ok {
			id, reason = "", f
		}
		cause := causeOf(reason)
		g, ok := byCause[cause]
		if !ok {
			g = &group{cause: cause, example: id}
			byCause[cause] = g
			groups = append(groups, g)
		}
		g.n++
	}

	suffix := ""
	if len(groups) > aggregateFailuresCap {
		suffix = fmt.Sprintf("; and %d more causes", len(groups)-aggregateFailuresCap)
		groups = groups[:aggregateFailuresCap]
	}

	parts := make([]string, 0, len(groups))
	for _, g := range groups {
		part := g.cause
		switch {
		case len(byCause) == 1 && g.example != "":
			part += fmt.Sprintf(" (e.g. %s)", g.example)
		case g.example != "":
			part += fmt.Sprintf(" (%d, e.g. %s)", g.n, g.example)
		case len(byCause) > 1:
			part += fmt.Sprintf(" (%d)", g.n)
		}
		parts = append(parts, part)
	}

	return fmt.Errorf("%s failed for %d of %d IDs: %s%s",
		opName, len(failures), total, strings.Join(parts, "; "), suffix)
}

// causeOf reduces one failure reason to what an operator can act on. An AWS
// authorization failure becomes the action the role lacks; anything else keeps
// its own words with the per-call transport noise removed — a request id, a
// host id and an encoded authorization message differ on every resource, so
// carrying them would defeat the grouping and fill the log with tokens nobody
// can use.
func causeOf(reason string) string {
	const notAuthorized = "not authorized to perform: "
	if i := strings.Index(reason, notAuthorized); i >= 0 {
		action := reason[i+len(notAuthorized):]
		if j := strings.IndexAny(action, " ,;"); j >= 0 {
			action = action[:j]
		}
		return "not authorized to perform " + action
	}
	// The SDK's "operation error <Service>: <Op>, " preamble goes for every
	// error class, not only for the API errors whose "api error " marker used
	// to hide it — a timeout and a transport failure carry the same preamble,
	// and the operation is already named by the caller's op label. The head
	// must look like "<Service>: <Op>" so prose that merely mentions an
	// operation error keeps its words.
	const opMarker = "operation error "
	if i := strings.Index(reason, opMarker); i >= 0 {
		head, rest, ok := strings.Cut(reason[i+len(opMarker):], ", ")
		if ok && strings.Contains(head, ": ") {
			reason = rest
		}
	}
	stripped := reason
	if i := strings.Index(stripped, "api error "); i >= 0 {
		stripped = stripped[i+len("api error "):]
	}
	for _, noise := range []string{"Encoded authorization failure message:", "RequestID:", "HostID:", "\n"} {
		if i := strings.Index(stripped, noise); i >= 0 {
			stripped = stripped[:i]
		}
	}
	stripped = strings.TrimRight(strings.TrimSpace(stripped), " ,.")
	if stripped != "" {
		return stripped
	}
	// A reason that was nothing but per-call noise still has to say something:
	// an empty cause would read as a failure with no reason at all.
	if reason = strings.TrimSpace(reason); reason != "" {
		return reason
	}
	return "no reason given"
}

// ErrClass maps an error to the short class word every surface that phrases a
// failure reads — the menu row's cause mark, ScanStatus.Err, the account-wide
// title, and CauseOf below. context.DeadlineExceeded is checked before
// ClassifyAWSError because a context error is never a smithy.APIError and
// would otherwise land in the "Unknown" bucket.
func ErrClass(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	// A call that never reached a service has no AWS error code to classify;
	// its own text is a URL and a socket address, which name the endpoint but
	// not the failure.
	var netErr net.Error
	if errors.As(err, &netErr) {
		return "transport"
	}
	code, _, _ := ClassifyAWSError(err)
	switch code {
	case "Throttling", "ThrottlingException", "TooManyRequestsException", "RequestLimitExceeded":
		return "throttled"
	case "AccessDenied", "AccessDeniedException":
		return "access-denied"
	case "ExpiredToken", "ExpiredTokenException", "RequestExpired":
		return "expired"
	default:
		return code
	}
}

// errClassPhrasing is the vocabulary a9s owns for the failure classes it names
// itself: the cause the failure renders as, the word a menu row shows in its
// alias column, and the phrase the title uses when every type in the sweep
// failed this way. One table, so a class cannot read as itself on one surface
// and as a generic error on another.
//
// An empty cause means the error's own words say more than the class does — a
// denied call names the action the role lacks, an API error carries its
// message — and CauseOf keeps them.
var errClassPhrasing = map[string]struct{ cause, row, sweepTitle string }{
	"timeout":       {"timeout", "timeout", "sweep: timeout"},
	"transport":     {"transport failure", "transport", "sweep: transport failure"},
	"access-denied": {"", "denied", "sweep: access denied"},
	"expired":       {"", "expired", "session expired"},
	"throttled":     {"", "throttled", "sweep: throttled"},
}

// unmodeledWord is what a row shows for a class a9s does not name itself: a
// raw AWS error code, which the operator can act on no differently than on any
// other failure.
const unmodeledWord = "error"

// RowWord returns the word a menu row shows for an error class. Empty for no
// failure at all — a row with nothing to explain shows its alias instead.
func RowWord(class string) string {
	if class == "" {
		return ""
	}
	if p, ok := errClassPhrasing[class]; ok {
		return p.row
	}
	return unmodeledWord
}

// SweepTitleForWord returns the account-wide phrasing for a row word: what the
// title says when EVERY probe in the sweep failed the same way, in place of one
// mark per row.
func SweepTitleForWord(word string) string {
	if word == unmodeledWord {
		return "sweep: error"
	}
	for _, p := range errClassPhrasing {
		if p.row == word {
			return p.sweepTitle
		}
	}
	return ""
}

// NamedErrClasses returns every class a9s names itself, sorted. A class outside
// this set is a raw AWS error code passed through by ErrClass.
func NamedErrClasses() []string {
	return slices.Sorted(maps.Keys(errClassPhrasing))
}

// CauseOf is the error-level entry to the same reduction AggregateFailures
// applies per failure: what an operator can act on, with the per-call
// transport noise removed. Every surface that renders a failure — a flash, a
// log line, an aggregated batch error — phrases it through here.
func CauseOf(err error) string {
	if err == nil {
		return ""
	}
	if p, ok := errClassPhrasing[ErrClass(err)]; ok && p.cause != "" {
		return p.cause
	}
	return causeOf(err.Error())
}

// AggregateMissing is the narrower variant used when the operation is a
// single batch call (like DescribeImages) and the failure signal is
// "requested IDs that didn't appear in the response" rather than per-call
// errors. Shape:
//
//	"<op>: N of M IDs not found in response: <id1>, <id2>, ..."
func AggregateMissing(opName string, missingIDs []string, total int) error {
	if len(missingIDs) == 0 {
		return nil
	}
	return fmt.Errorf("%s: %d of %d IDs not found in response: %s",
		opName, len(missingIDs), total, strings.Join(missingIDs, ", "))
}
