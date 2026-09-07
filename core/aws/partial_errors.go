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
// Reads the code through the one classifier, so both a typed SDK error (e.g.
// route53 types.NoSuchHostedZone) and the generic API error shape are covered
// by the same match — classification is code-based, never a message substring scan.
// "NotFound" also covers S3's empty-body-404 synthesis from HTTP status
// text, which carries no modeled error shape of its own.
func IsNotFoundErr(err error) bool {
	return ErrCodeIs(err,
		"NoSuchBucket", "NotFound", "NoSuchHostedZone", "ResourceNotFoundException", "InvalidInstanceID.NotFound",
		// RDS and DocumentDB spell the same race with their own codes; a
		// snapshot deleted between the list call and a per-snapshot
		// describe answers one of these.
		"DBSnapshotNotFound", "DBClusterSnapshotNotFoundFault")
}

// aggregateFailuresCap is the maximum number of distinct causes
// AggregateFailures names before summarizing the rest. Failures are grouped by
// cause, so this bounds the line by how many DIFFERENT things went wrong, not
// by how many resources they happened to.
const aggregateFailuresCap = 5

// Failure is one failed per-item call, recorded from the error's own fields:
// which item refused, the class every surface branches on, and the cause it
// phrases. It is the only shape a failure travels in — a caller that rendered
// its own "<id>: <err>" would hand the aggregate a string it could only reduce
// back to a cause by re-reading words the SDK wrote.
type Failure struct {
	ID    string
	Class string
	Cause string
}

// FailedCall records a per-item call that failed with err.
func FailedCall(id string, err error) Failure {
	return FailedCallInRegion(id, err, "")
}

// FailedCallInRegion is FailedCall for a call whose failure may be about the
// region itself; the region is named only when the class says so, exactly as
// CauseInRegion decides for every other surface.
func FailedCallInRegion(id string, err error, region string) Failure {
	cause := CauseInRegion(err, region)
	if cause == "" {
		// A failure with nothing to say still has to say something: an empty
		// cause would read as a failure with no reason at all.
		cause = "no reason given"
	}
	// One aggregate can cover several calls — a related walk resolves a name
	// and then reads what it points at — and the id alone does not say which
	// of them refused. The SDK names the operation in the error's own fields.
	// A denied call's cause already names the action, so it is not decorated
	// twice.
	if opErr, ok := errors.AsType[*smithy.OperationError](err); ok &&
		opErr.OperationName != "" && !strings.Contains(cause, opErr.OperationName) {
		cause = opErr.OperationName + ": " + cause
	}
	return Failure{ID: id, Class: ErrClass(err), Cause: cause}
}

// UnusableAnswer records an item the service answered for without the field
// the caller needs — a nil struct, a missing default version, a page that says
// it is truncated and names no marker. There is no error to classify, so a9s
// states the cause itself.
func UnusableAnswer(id, cause string) Failure {
	return Failure{ID: id, Class: classUnknown, Cause: cause}
}

// classErr carries the recorded class of a partial-batch failure out through
// the composite error. The composite's own words are a9s's sentence and hold
// no AWS error to read, so without this every aggregated denial reclassifies
// as Unknown on the surfaces downstream of the fetcher.
type classErr struct {
	error
	class string
}

// classCarrier is what ErrClass looks for before it inspects the chain: an
// error that already knows its class because a recorder read it off the
// original.
type classCarrier interface {
	error
	errClass() string
}

func (e classErr) errClass() string { return e.class }

// isPhrased reports whether err is a composite AggregateFailures has already
// phrased. Its words are the failure line — the class it carries is for a
// surface that branches on it, never for one that reads it.
func isPhrased(err error) bool {
	_, ok := errors.AsType[classCarrier](err)
	return ok
}

// AggregateFailures builds the canonical composite error for a partial-batch
// operation. opName is the operation label (e.g. "kms FetchByIDs",
// "policy FetchByIDs", "ecs-task ListTargets"). failures are the records the
// iteration collected through FailedCall/UnusableAnswer. total is the total
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
// grouped by their recorded cause and each cause is stated once, with how many
// resources it covered and one example:
//
//	"<op> failed for 46 of 46 IDs: not authorized to perform ec2:DescribeSnapshotAttribute (e.g. snap-0abc)"
//	"<op> failed for 3 of 9 IDs: timeout (2, e.g. snap-1); no metadata (1, e.g. snap-3)"
//
// Above aggregateFailuresCap distinct causes the rest are summarized as
// "; and N more causes". Grouping is per cause rather than per class so two
// actions the same role lacks stay two lines: one denial must never speak for
// another. The class travels with the composite when every failure shares one,
// which is what lets a fetcher's denied batch still read as access-denied.
func AggregateFailures(opName string, failures []Failure, total int) error {
	if len(failures) == 0 {
		return nil
	}

	// Ordered by id here rather than by every caller: an aggregate built from
	// a parallel walk would otherwise name whichever failure its goroutine
	// finished first as the example, and read differently on every run.
	failures = slices.SortedFunc(slices.Values(failures), func(a, b Failure) int {
		if c := strings.Compare(a.ID, b.ID); c != 0 {
			return c
		}
		return strings.Compare(a.Cause, b.Cause)
	})

	type group struct {
		cause   string
		example string
		n       int
	}
	var groups []*group
	byCause := map[string]*group{}
	class := failures[0].Class
	for _, f := range failures {
		g, ok := byCause[f.Cause]
		if !ok {
			g = &group{cause: f.Cause, example: f.ID}
			byCause[f.Cause] = g
			groups = append(groups, g)
		}
		g.n++
		if f.Class != class {
			class = ""
		}
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

	err := fmt.Errorf("%s failed for %d of %d IDs: %s%s",
		opName, len(failures), total, strings.Join(parts, "; "), suffix)
	if class == "" {
		return err
	}
	return classErr{error: err, class: class}
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

// classUnknown is the class of an error carrying no AWS error code at all: a
// response the SDK could not bind, or a failure that never reached a service
// and is not one of the transport classes named below.
const classUnknown = "Unknown"

// ClassifyAWSError inspects an error for a smithy.APIError and returns the
// error code, message, and whether the operation is retryable. This is the one
// read of the SDK's error type in a9s: every other site asks ErrClass for the
// class, ErrCodeIs for a code it names itself, or MessageOf for the message.
func ClassifyAWSError(err error) (code string, message string, retryable bool) {
	if err == nil {
		return "", "", false
	}

	apiErr, ok := errors.AsType[smithy.APIError](err)
	if !ok {
		return classUnknown, err.Error(), false
	}

	code = apiErr.ErrorCode()
	message = apiErr.ErrorMessage()

	switch code {
	case "Throttling", "ThrottlingException", "TooManyRequestsException", "RequestLimitExceeded", "SlowDown":
		retryable = true
	default:
		retryable = false
	}

	return code, message, retryable
}

// ErrCodeIs reports whether err carries one of the named AWS error codes.
//
// The codes a site names itself are the ones AWS models as errors but an
// operator reads as answers: "this table has no resource policy", "this launch
// template is gone", "this bucket lives in another region". They are not
// classes — a9s says nothing about them beyond "this one is not a failure" —
// so they are asked for by code, through the one classifier, rather than by
// each site unwrapping the SDK's error type for itself.
func ErrCodeIs(err error, codes ...string) bool {
	code, _, _ := ClassifyAWSError(err)
	if code == "" || code == classUnknown {
		return false
	}
	return slices.Contains(codes, code)
}

// The classes several surfaces branch on are named: a literal at a branch
// would be a second decision that agrees with the class only by luck.
//
// ClassRegionUnavailable is a service the selected region does not offer —
// the endpoint host's DNS does not resolve.
const (
	ClassRegionUnavailable = "region-unavailable"
	ClassAccessDenied      = "access-denied"
	ClassThrottled         = "throttled"
)

// IsAccessDenied reports whether the role was refused the call. It is the one
// reading of that condition: the several codes AWS spells it with live in
// ErrClass's table, not at the sites that act on it.
func IsAccessDenied(err error) bool {
	return ErrClass(err) == ClassAccessDenied
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
	if c, ok := errors.AsType[classCarrier](err); ok {
		return c.errClass()
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
		return ClassThrottled
	case "AccessDenied", "AccessDeniedException":
		return ClassAccessDenied
	case "ExpiredToken", "ExpiredTokenException", "RequestExpired":
		return "expired"
	case "":
		// An empty class means no failure at all — the menu row shows its
		// alias and the operator is told nothing. A response with no modeled
		// code still failed.
		return classUnknown
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
	ClassAccessDenied:      {"", "denied", "sweep: access denied"},
	"expired":              {"", "expired", "session expired"},
	ClassThrottled:         {"", "throttled", "sweep: throttled"},
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
	if isPhrased(err) {
		return err.Error()
	}
	if cause := CauseForClass(ErrClass(err)); cause != "" {
		return cause
	}
	apiErr, ok := errors.AsType[smithy.APIError](err)
	if !ok {
		// An error the SDK could not model still arrives wrapped in the
		// operation error, whose own words are the service and operation the
		// caller has already named. Reading the error it carries drops that
		// preamble by structure rather than by trimming text.
		if opErr, wrapped := errors.AsType[*smithy.OperationError](err); wrapped && opErr.Unwrap() != nil {
			return opErr.Unwrap().Error()
		}
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
	// A composite's records were each recorded in a region and already say so;
	// naming it again around the whole sentence would name it twice.
	if isPhrased(err) {
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
