// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// Package aws — degraded_resource.go
//
// The single truth source for the "details denied"/"details unavailable"
// degraded row: when a list call names a resource but the per-item describe
// fails (IAM denial, transient fault, nil body, absent from the response),
// the row is KEPT — name visible, cause phrase in the Status cell, the
// failure still aggregated into the composite fetch error. Dropping the row
// fakes an empty or shorter list: "you can't see it" must never render as
// "it isn't there": a readonly role can be allowed mwaa:ListEnvironments
// while denied airflow:GetEnvironment, and the environment would vanish.
//
// The two phrases are not interchangeable: an authorization denial
// (degradedAuthDenial — AccessDenied/AccessDeniedException, or EC2's
// UnauthorizedOperation) is a "details denied" row; everything else — nil
// error, throttling, cancellation, nil response body — is a "details
// unavailable" row. markDegraded is the one place that decides which, and
// binding the row is the one place that turns it into the type's finding.
//
// Every list-then-describe fetcher (N+1 or batch) MUST keep a row for every
// listed name through keepListed (or describeTally.one), building the degraded
// row with degradedRow or markDegraded only in the function it hands them,
// and declare both
// DetailsDeniedFindingDef and DetailsUnavailableFindingDef on its catalog
// literal, so the phrase, severity, and finding-code shape can never diverge
// per type.
package aws

import (
	"errors"
	"maps"
	"testing"

	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

const (
	detailsDeniedPhrase = "details denied"
	detailsDeniedDetail = "The per-item describe call for this row was denied, so a9s can show its name and nothing about its posture — the row is unjudged, not healthy. Grant the read-only describe permission for this type to the role you browse with, then refresh."

	detailsUnavailablePhrase = "details unavailable"
	detailsUnavailableDetail = "The per-item describe call for this row failed, so a9s can show its name and nothing about its posture — the row is unjudged, not healthy. Retry the refresh; if it persists, check the service's health and whether the call is being throttled."
)

// DetailsDeniedCode returns the per-type FindingCode for the degraded row
// ("<shortName>.warn.details_denied").
func DetailsDeniedCode(shortName string) domain.FindingCode {
	return domain.FindingCode(shortName + ".warn.details_denied")
}

// DetailsUnavailableCode returns the per-type FindingCode for the neutral
// degraded row ("<shortName>.warn.details_unavailable").
func DetailsUnavailableCode(shortName string) domain.FindingCode {
	return domain.FindingCode(shortName + ".warn.details_unavailable")
}

// DegradedFindingField is where DegradedDetails records the code of the
// finding it emitted, so a row rebuilt from Fields alone carries the finding's
// one identity. The status cell holds the same finding's rendered phrase, but
// a phrase is wording: reading it back would make every reword a silent
// retirement of the recovery, and catalog.ResolveColor reads the code for the
// colour of every type.
const DegradedFindingField = catalog.DegradedFindingField

// degradedAuthDenial reports whether err is an authorization denial, across
// the SDK's service-specific codes: AccessDenied/AccessDeniedException (most
// services) and UnauthorizedOperation (EC2 — lt's DescribeLaunchTemplateVersions
// is EC2-backed and never returns AccessDenied). Any other error — or a nil
// body (nil err) — is a non-auth failure and yields details_unavailable.
// Separate from accessDeniedErr (kms.go), which is KMS-only.
func degradedAuthDenial(err error) bool {
	// EC2 spells the same refusal UnauthorizedOperation, which is no class of
	// its own — it is a code this one site adds to the denial class.
	return IsAccessDenied(err) || ErrCodeIs(err, "UnauthorizedOperation")
}

// The two causes a fetcher records in DegradedFindingField before its row is
// bound. Each is the code of the type's degraded finding without the type's
// prefix, so a bound field, which holds the whole code, is never one of them.
const (
	degradedCauseDenied      = "warn.details_denied"
	degradedCauseUnavailable = "warn.details_unavailable"
)

// markDegraded records on r that the per-item describe behind it failed with
// err (nil for a row absent from the response). An authorization denial
// (degradedAuthDenial) is "details denied"; everything else — throttling,
// cancellation, a nil response body — is the neutral "details unavailable".
// "You can't see it" and "it didn't come back" are different facts and never
// share one phrase. Binding the row turns the cause into the finding its type
// declares for it.
func markDegraded(r resource.Resource, err error) resource.Resource {
	fields := make(map[string]string, len(r.Fields)+1)
	maps.Copy(fields, r.Fields)
	fields[DegradedFindingField] = degradedCauseUnavailable
	if degradedAuthDenial(err) {
		fields[DegradedFindingField] = degradedCauseDenied
	}
	r.Fields = fields
	return r
}

// degradedRow is the name-only row an N+1 fetcher emits when the per-item
// describe for id failed or came back empty.
//
// RawStruct stays nil on purpose. A synthetic struct carrying only the name
// is assertable, so every pivot that reads a field off the struct would find
// it empty and answer a confident zero — "this node group has no IAM role" —
// about a resource nobody was allowed to describe. Every pivot runs through
// DegradedRowAnswer, which answers any zero on such a row Unknown. Identity a
// pivot can legitimately use goes in Fields, which the disk cache preserves
// and RawStruct does not.
func degradedRow(id string, err error) resource.Resource {
	return markDegraded(resource.Resource{ID: id, Name: id, Fields: map[string]string{"name": id}}, err)
}

// resolveDegraded turns the cause markDegraded recorded on a row of shortName
// into that type's degraded finding, and records the finding's code in its
// place. A row already resolved, or never marked, is left as it is.
func resolveDegraded(shortName string, r *resource.Resource) {
	cause := r.Fields[DegradedFindingField]
	if cause != degradedCauseDenied && cause != degradedCauseUnavailable {
		return
	}
	code := domain.FindingCode(shortName + "." + cause)
	fields := maps.Clone(r.Fields)
	fields[DegradedFindingField] = string(code)
	r.Fields = fields
	if !hasFinding(r.Findings, code) {
		*r = withWave1(*r, wave1Finding(code))
	}
}

// DegradedDetails is degradedRow as a row of shortName carries it once bound,
// for a test that builds one directly.
func DegradedDetails(shortName, id string, err error) resource.Resource {
	if !testing.Testing() {
		panic("DegradedDetails called outside a test binary")
	}
	r := degradedRow(id, err)
	resolveDegraded(shortName, &r)
	return r
}

type listedAnswer struct {
	rows []resource.Resource
	gone map[string]bool
}

// keepListed is the one place a list-then-describe fetcher turns its listed
// names into rows. idOf maps a listed name to the ID of the row it lists.
// Every listed name keeps a row, in list order: the row the describe built
// for it, or degraded(name, err) when the describe failed (err) or left the
// name out (err nil), recorded as a failure of op under the row's ID. A name
// the service reports gone — in ans.gone, or through an IsNotFoundErr error —
// was deleted after the list call and is dropped without a failure. A name
// listed twice is one row. A described row no listed name maps to is kept
// after them, unless it names no ID at all: an answer that does not say what
// it describes is no answer for any listed name.
func keepListed(op string, names []string, idOf func(name string) string, ans listedAnswer, err error, degraded func(name string, err error) resource.Resource) ([]resource.Resource, []Failure) {
	described := make(map[string]resource.Resource, len(ans.rows))
	for _, r := range ans.rows {
		if r.ID != "" {
			described[r.ID] = r
		}
	}
	rows := make([]resource.Resource, 0, len(names))
	var failures []Failure
	seen := make(map[string]bool, len(names))
	for _, name := range names {
		id := idOf(name)
		if seen[id] {
			continue
		}
		seen[id] = true
		if r, ok := described[id]; ok {
			rows = append(rows, r)
			continue
		}
		if ans.gone[name] || IsNotFoundErr(err) || errors.Is(err, errNotInList) {
			continue
		}
		r := degraded(name, err)
		rows = append(rows, r)
		if err != nil {
			failures = append(failures, FailedCall(r.ID, err))
		} else {
			failures = append(failures, UnusableAnswer(r.ID, op+" returned no answer"))
		}
	}
	for _, r := range ans.rows {
		if r.ID != "" && !seen[r.ID] {
			seen[r.ID] = true
			rows = append(rows, r)
		}
	}
	return rows, failures
}

func listedNameIsID(name string) string { return name }

// describeTally is keepListed for a fetcher that describes one listed name
// per call. It counts the requests it sent next to the failures they
// recorded, so the aggregate's "N of M" is out of the calls made, never out
// of list entries that sent none.
type describeTally struct {
	failures []Failure
	sent     int
}

func (t *describeTally) one(op, name, id string, describe func() (*resource.Resource, error), degraded func(err error) resource.Resource) []resource.Resource {
	if name == "" {
		return nil
	}
	t.sent++
	row, err := describe()
	var ans listedAnswer
	if err == nil && row != nil {
		ans.rows = []resource.Resource{*row}
	}
	rows, failures := keepListed(op, []string{name}, func(string) string { return id }, ans, err, func(_ string, err error) resource.Resource {
		return degraded(err)
	})
	t.failures = append(t.failures, failures...)
	return rows
}

func (t *describeTally) err(label string) error {
	return AggregateFailures(label, t.failures, t.sent)
}

// errNotInList is what a describeTally describe returns for a name its answer
// places outside the list — a KMS key AWS manages, on the customer-key list.
// The name is dropped without a failure, like a gone one.
var errNotInList = errors.New("not in this list")

// describedRow is the eligibility of a wave-2 pass that sends again the
// describe the list fetch made for the row: a degraded row's finding already
// states that call failed, and a second answer would mark or report the same
// failure twice.
func describedRow(r resource.Resource) bool {
	return r.Fields[DegradedFindingField] == ""
}
