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
	"crypto/tls"
	"errors"
	"fmt"
	"maps"
	"net"
	"slices"
	"strings"

	"github.com/aws/smithy-go"
)

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
		cause := groupCause(reason)
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

// deniedAction reads the action a role lacks out of an AWS authorization
// message. AWS models the action inside the message rather than as a field of
// its own, so this reads the message field; it never reads the SDK's
// formatted chain.
func deniedAction(message string) (string, bool) {
	const marker = "not authorized to perform: "
	_, action, ok := strings.Cut(message, marker)
	if !ok {
		return "", false
	}
	if j := strings.IndexAny(action, " ,;"); j >= 0 {
		action = action[:j]
	}
	action = strings.TrimRight(action, " .,;:")
	return action, action != ""
}

// groupCause is AggregateFailures' grouping key over reasons its callers have
// already rendered to text. It undoes the SDK's own Error() formatting: the
// operation preamble, the "api error" marker, and the per-call request id,
// host id and encoded authorization message, which differ on every resource
// and would otherwise put each failure in a group of its own.
//
// ponytail: a text reduction, because AggregateFailures' callers hand it
// "<id>: <err>" strings and the error's fields are gone by then. Upgrade path
// is an error-carrying collector, after which this reads CauseOf like every
// other surface and disappears.
func groupCause(reason string) string {
	if action, ok := deniedAction(reason); ok {
		return "not authorized to perform " + action
	}
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

// ClassRegionUnavailable is the class of a service the selected region does
// not offer: the endpoint host's DNS does not resolve. Named because two
// surfaces branch on it, and a literal in either would be a second decision
// that agrees with the class only by luck.
const ClassRegionUnavailable = "region-unavailable"

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
	// "No such host" for a service endpoint is the signature of a service the
	// region does not offer (live witness 2026-07-14: CodeArtifact in
	// eu-central-2). Any other resolver failure is the resolver's own problem
	// and must stay loud under its own class.
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		if dnsErr.IsNotFound {
			return ClassRegionUnavailable
		}
		return "dns"
	}
	// A certificate that does not verify is a proxy or a clock; a record
	// header the handshake could not read is a proxy answering in plaintext.
	// Both are checked before net.Error, which the wrapping *url.Error
	// satisfies for either.
	var certErr *tls.CertificateVerificationError
	var recErr tls.RecordHeaderError
	if errors.As(err, &certErr) || errors.As(err, &recErr) {
		return "tls"
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
	case "":
		// An empty class means no failure at all — the menu row shows its
		// alias and the operator is told nothing. A response with no modeled
		// code still failed.
		return "Unknown"
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
//
// tls, dns and transport are three classes because the operator does three
// different things: a certificate is a proxy or a clock, a resolver failure
// is resolution, and a refused or reset connection is the network path.
var errClassPhrasing = map[string]struct{ cause, row, sweepTitle string }{
	"timeout": {"timeout", "timeout", "sweep: timeout"},
	// No sweep title: an account cannot be region-unavailable as a whole, so
	// this class can never be every type's cause. The completeness gate knows.
	ClassRegionUnavailable: {"service not available in this region", "no service", ""},
	"tls":                  {"TLS handshake failed", "tls", "sweep: TLS failure"},
	"dns":                  {"DNS resolution failed", "dns", "sweep: DNS failure"},
	"transport":            {"transport failure", "transport", "sweep: transport failure"},
	"access-denied":        {"", "denied", "sweep: access denied"},
	"expired":              {"", "expired", "session expired"},
	"throttled":            {"", "throttled", "sweep: throttled"},
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

// CauseForClass returns the phrase a class supplies for itself, or empty when
// the error's own words say more than the class does.
func CauseForClass(class string) string {
	return errClassPhrasing[class].cause
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

// MessageOf returns an AWS error's own message field, or the error's words
// when no API error is in the chain. This is the one extraction of that
// field; a surface that wants the actionable AWS text asks here rather than
// %v-ing a chain whose wrapper prefixes consume the line.
func MessageOf(err error) string {
	if err == nil {
		return ""
	}
	if apiErr, ok := errors.AsType[smithy.APIError](err); ok {
		return apiErr.ErrorMessage()
	}
	return err.Error()
}

// CauseOf is what an operator can act on, read from the error's own fields.
// Every surface that renders a failure — a flash, a log line, a menu row —
// phrases it through here.
//
// The class speaks first where it says more than the error does. Otherwise the
// cause is the API error's code and message, both fields; the request id, the
// host id, and the service and operation preamble live in the SDK's formatted
// text and are never read, so a message whose own words spell one of them
// keeps them.
func CauseOf(err error) string {
	if err == nil {
		return ""
	}
	if cause := CauseForClass(ErrClass(err)); cause != "" {
		return cause
	}
	apiErr, ok := errors.AsType[smithy.APIError](err)
	if !ok {
		return err.Error()
	}
	message := strings.TrimSpace(apiErr.ErrorMessage())
	if action, ok := deniedAction(message); ok {
		return "not authorized to perform " + action
	}
	// AWS appends the encoded authorization message to the message field
	// itself. It is an opaque per-call token, not a cause, and it is long
	// enough to push the actionable text off the line.
	if i := strings.Index(message, "Encoded authorization failure message:"); i >= 0 {
		message = message[:i]
	}
	// A flash and a menu row are one line each; a multi-line message keeps
	// its first, which is where AWS puts the failure.
	message, _, _ = strings.Cut(message, "\n")
	message = strings.TrimRight(strings.TrimSpace(message), " ,.")
	switch {
	case message == "":
		return apiErr.ErrorCode()
	case apiErr.ErrorCode() == "":
		return message
	}
	return apiErr.ErrorCode() + ": " + message
}

// CauseInRegion is CauseOf with the region named when — and only when — the
// class is about the region. A caller that appended the region itself would be
// deciding a second time what the class already knows.
func CauseInRegion(err error, region string) string {
	cause := CauseOf(err)
	if region == "" || ErrClass(err) != ClassRegionUnavailable {
		return cause
	}
	return cause + " (" + region + ")"
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
