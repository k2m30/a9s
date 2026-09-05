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
	"errors"
	"fmt"
	"net"
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
	case "NoSuchBucket", "NotFound", "NoSuchHostedZone", "ResourceNotFoundException", "InvalidInstanceID.NotFound":
		return true
	default:
		return false
	}
}

// aggregateFailuresCap is the maximum number of per-ID failures
// AggregateFailures enumerates verbatim before summarizing the remainder —
// a wide partial-failure set (e.g. an IAM sweep failing on 36 of 49 groups)
// must not grow one flash/log line without bound.
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
// without an extra conditional. Composite shape, at or below the cap:
//
//	"<op> failed for N of M IDs: <f1>; <f2>; ..."
//
// Above aggregateFailuresCap, only the first cap failures are named
// verbatim and the rest are summarized:
//
//	"<op> failed for N of M IDs: <f1>; <f2>; <f3>; <f4>; <f5>; and N-5 more"
func AggregateFailures(opName string, failures []string, total int) error {
	if len(failures) == 0 {
		return nil
	}
	shown := failures
	suffix := ""
	if len(failures) > aggregateFailuresCap {
		shown = failures[:aggregateFailuresCap]
		suffix = fmt.Sprintf("; and %d more", len(failures)-aggregateFailuresCap)
	}
	return fmt.Errorf("%s failed for %d of %d IDs: %s%s",
		opName, len(failures), total, strings.Join(shown, "; "), suffix)
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
