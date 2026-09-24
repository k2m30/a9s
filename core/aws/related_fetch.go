// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// related_fetch.go reads the target lists related checkers count over, and
// holds relatedAnswer, the one rule that decides a related row's state.
package aws

import (
	"cmp"
	"context"
	"errors"
	"slices"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// DefaultPageSize is the number of resources fetched per paginated API call.
// It aliases resource.DefaultPageSize, the single source of truth.
const DefaultPageSize = resource.DefaultPageSize

// FetchRelatedTarget returns the target list for a pivot that matches on
// what a row's details say. It checks the ResourceCache first, then falls back
// to the registered paginated fetcher for the first page only.
//
// Returns (resources, isTruncated, error):
//   - cache hit: returns cached resources, no AWS call.
//   - cache entry that is not the list (CachedList): a miss.
//   - cache miss + registered fetcher: fetches the first page.
//   - cache miss + no fetcher: returns nil, false, nil (graceful no-op).
//
// isTruncated is set when the list is a subset (relatedRowsByID) and when a
// row's details could not be read: that row may be the one that matches.
//
// A nil list means nothing was read: callers MUST answer UnknownRelated, since
// a count from a list that was never fetched is a guess. A list that WAS read
// but came back truncated is a different answer — whatever matched in it is
// real, and zero matches is a real zero so far, so callers carry isTruncated
// through and the row renders "(N+)".
func FetchRelatedTarget(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, subset, err := relatedRowsByID(ctx, clients, cache, target)
	return resources, subset || anyDegraded(resources), err
}

// relatedRowsByID is FetchRelatedTarget for a pivot that matches a row by its
// ID or Name, which a row whose details could not be read still carries: it
// is truncated only when the list is a subset of the list AWS holds.
func relatedRowsByID(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	if entry, ok := CachedList(cache, target); ok && !elsewhere(clients) {
		// A present entry is a complete answer even when it holds nothing:
		// the executor seeds it straight from a fetcher that found zero
		// resources, and nil there would be indistinguishable from "no entry".
		resources := entry.Resources
		if resources == nil {
			resources = []resource.Resource{}
		}
		return resources, entry.IsTruncated, nil
	}
	pf := resource.GetPaginatedFetcher(target)
	if pf == nil {
		return nil, false, nil
	}
	result, err := FirstPage(ctx, clients, target)
	if err != nil && len(result.Resources) == 0 {
		return nil, false, err
	}
	resources := result.Resources
	if resources == nil {
		resources = []resource.Resource{}
	}
	return resources, FetchIsPartial(result, err), nil
}

// CachedList returns target's cache entry when it holds the type's list. An
// entry of rows restored from disk (FieldsOnly: no SDK struct, possibly
// stale) or added one by one for a detail (Partial) is not the list: every
// reader treats it as absent.
func CachedList(cache resource.ResourceCache, target string) (resource.ResourceCacheEntry, bool) {
	entry, ok := cache[target]
	return entry, ok && !entry.FieldsOnly && !entry.Partial
}

// cachedRelatedList reads target's list from the cache alone, for a pivot
// whose contract forbids a call: ok is false when the session holds no list
// of target, and truncated says the list held is a subset.
func cachedRelatedList(cache resource.ResourceCache, target string) (rows []resource.Resource, truncated, ok bool) {
	entry, ok := CachedList(cache, target)
	return entry.Resources, entry.IsTruncated, ok
}

// typedRow pairs a cached Resource's ID with its RawStruct already asserted
// to T, so callers of cachedTypedRows never re-assert.
type typedRow[T any] struct {
	ID  string
	Raw T
}

// cachedTypedRows reads the shortName entry directly from cache — it never
// fetches. Tri-state contract: no list (CachedList) →
// (nil, false, false) = unknown; a list none of whose rows asserts to T →
// (nil, false, false) = unknown; a typed list → the asserting rows only. A row that carries no T — a row of
// another type, a row whose details could not be read — may be the one that
// matches, so dropping it makes the rows a subset and truncated true.
func cachedTypedRows[T any](cache resource.ResourceCache, shortName string) (rows []typedRow[T], truncated bool, ok bool) {
	list, truncated, present := cachedRelatedList(cache, shortName)
	if !present {
		return nil, false, false
	}
	if len(list) == 0 {
		return nil, truncated, true
	}
	dropped := false
	for _, r := range list {
		raw, asserted := assertStruct[T](r.RawStruct)
		if !asserted {
			dropped = true
			continue
		}
		rows = append(rows, typedRow[T]{ID: r.ID, Raw: raw})
	}
	if len(rows) == 0 {
		return nil, false, false
	}
	return rows, truncated || dropped, true
}

// FetchIsPartial reports whether a fetcher's answer is a subset of the list
// it was asked for: a page that ended with a continuation token, a page that
// failed, or an item the fetcher dropped. An item whose row the answer still
// holds lost its details, not its place in the list — the row says so itself
// (anyDegraded), for the pivots that read details.
func FetchIsPartial(result resource.FetchResult, err error) bool {
	return (result.Pagination != nil && result.Pagination.IsTruncated) || !RowsHeld(err, result.Resources)
}

// StoredPagination is the pagination a fetch result is stored under: its own,
// or, when FetchIsPartial finds the rows a subset its cursor does not report,
// a lower bound no cursor can resume.
func StoredPagination(result resource.FetchResult, err error) *resource.PaginationMeta {
	p := result.Pagination
	if (p != nil && p.IsTruncated) || !FetchIsPartial(result, err) {
		return p
	}
	meta := resource.PaginationMeta{TotalHint: -1}
	if p != nil {
		meta = *p
	}
	meta.IsTruncated, meta.LowerBoundOnly = true, true
	return &meta
}

// RowsHeld reports whether err, returned beside rows, is nothing but per-item
// failures on rows that rows still holds. Any other error may have cost a row.
func RowsHeld(err error, rows []resource.Resource) bool {
	switch e := err.(type) {
	case nil:
		return true
	case failuresErr:
		return !slices.ContainsFunc(e.failures, func(f Failure) bool {
			return f.Page > 0 || !slices.ContainsFunc(rows, func(r resource.Resource) bool { return r.ID == f.ID })
		})
	case interface{ Unwrap() []error }:
		for _, inner := range e.Unwrap() {
			if !RowsHeld(inner, rows) {
				return false
			}
		}
		return true
	}
	return false
}

// anyDegraded reports whether list holds a row whose details could not be
// read: such a row may be the one that matches, so a scan over list is a
// lower bound.
func anyDegraded(list []resource.Resource) bool {
	return slices.ContainsFunc(list, func(r resource.Resource) bool { return r.Fields[DegradedFindingField] != "" })
}

// relatedRead is what a checker read of the places its relation can live in.
// relatedAnswer is the one rule that turns it into the row's state: checkers
// report, they never build a count, a lower bound or a proven zero themselves.
type relatedRead struct {
	ids []string
	// partial: a place was read only in part — a target list that is a
	// subset, a walk stopped at its cap, a call that failed beside ones that
	// answered — so more rows may exist.
	partial bool
	// unread: a place the relation can live in could not be read at all.
	unread bool
	// atMostOne: the relation holds at most one target row, so one found is
	// the whole answer however much else was left unread.
	atMostOne bool
	// failure is why a place went unread; it reaches the flash, not the row.
	failure error
	// failed: a place's one call failed, rather than a scan over many rows
	// each refusing its own read: with nothing found the row is that error.
	failed bool
	// noClient: a place went unread because the session has no client for
	// it, so no call was made; what the other places found is not the
	// answer a connected session gives, and the row reads unknown.
	noClient bool
	// heuristic: the ids are candidates matched by a shared property, not
	// rows AWS names as related (heuristicResult).
	heuristic bool
}

// readOf is what answer r says was read, so a checker whose relation lives
// in several places folds each place's answer into one relatedAnswer.
func readOf(r resource.RelatedCheckResult) relatedRead {
	if r.State() == domain.RelatedResolved {
		return relatedRead{ids: r.ResourceIDs(), partial: r.Truncated(), failure: r.Failure(), heuristic: r.Coverage() == resource.CoverageHeuristic}
	}
	return relatedRead{unread: true, failure: cmp.Or(r.Err(), r.Failure()), failed: r.State() == domain.RelatedError}
}

// unreadBy is the read of a place a failed read left unread: err reaches the
// flash, except a missing client, which made no call.
func unreadBy(err error) relatedRead {
	if errors.Is(err, errClientMissing) {
		return relatedRead{unread: true, noClient: true}
	}
	return relatedRead{unread: true, failure: err, failed: err != nil}
}

// pagedRead is what a PageAll walk says beside the items it returned:
// partial when it stopped at its cap, unread with the failure when a page
// failed, so the items of the pages before it stay a lower bound.
func pagedRead(complete bool, err error) relatedRead {
	if err != nil {
		return unreadBy(err)
	}
	return relatedRead{partial: !complete}
}

// joinReads is the read of a relation that lives in several places: every id
// any place found, partial or unread when any place was.
func joinReads(reads ...relatedRead) relatedRead {
	var out relatedRead
	var failures []error
	for _, r := range reads {
		out.ids = append(out.ids, r.ids...)
		out.partial = out.partial || r.partial
		out.unread = out.unread || r.unread
		out.failed = out.failed || r.failed
		out.noClient = out.noClient || r.noClient
		out.heuristic = out.heuristic || r.heuristic
		if r.failure != nil {
			failures = append(failures, r.failure)
		}
	}
	out.failure = errors.Join(failures...)
	return out
}

// relatedAnswer is the rule: exact when every place was read over a whole
// list; a lower bound when something was read only in part, or a place could
// not be read beside rows found elsewhere; unknown when a place could not be
// read and nothing was found, that place's error when its one call failed; a
// proven 0 only when everything was read and held nothing.
func relatedAnswer(target string, r relatedRead) resource.RelatedCheckResult {
	res := relatedState(target, r)
	if r.failure == nil {
		return res
	}
	if r.failed && res.State() == domain.RelatedUnknown {
		return ReadFailed(target, r.failure)
	}
	return res.WithFailure(r.failure)
}

func relatedState(target string, r relatedRead) resource.RelatedCheckResult {
	var ids []string
	for _, id := range r.ids {
		if id != "" {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	ids = slices.Compact(ids)
	switch {
	case r.atMostOne && len(ids) == 1:
		return resource.KnownRelated(target, ids, false)
	case r.unread && (len(ids) == 0 || r.noClient):
		return resource.UnknownRelated(target)
	case r.heuristic && (len(ids) > 0 || r.partial):
		h := resource.HeuristicRelated(target, ids)
		if r.unread || r.partial {
			return h.PartialScan()
		}
		return h
	case r.unread || r.partial:
		return resource.KnownRelated(target, ids, true)
	case len(ids) == 0:
		return resource.ProvenZero(target, "every place the relation lives")
	}
	return resource.KnownRelated(target, ids, false)
}

// rowReads tallies a reverse scan's per-row reads: the rows read, and the
// rows that could not be, with the errors that say why.
type rowReads struct {
	read, unread int
	failed       []failedRead
}

// failedRead is one row's read that failed, and why.
type failedRead struct {
	id  string
	err error
}

// missed records a row that could not be read without a call being made.
func (s *rowReads) missed() { s.unread++ }

// fail records a row whose read failed with err. A missing client made no
// call, so it is not a failure to report.
func (s *rowReads) fail(id string, err error) {
	s.unread++
	if !errors.Is(err, errClientMissing) {
		s.failed = append(s.failed, failedRead{id, err})
	}
}

// answer reports the scan: the ids found over a list that is a subset when
// truncated. A row not read makes the count a lower bound, no row read makes
// it unknown, and the failures under op reach the flash.
func (s rowReads) answer(target, op string, ids []string, truncated bool) resource.RelatedCheckResult {
	failures := make([]Failure, len(s.failed))
	for i, f := range s.failed {
		failures[i] = FailedCall(f.id, f.err)
	}
	return relatedAnswer(target, relatedRead{
		ids:     ids,
		partial: truncated || s.unread > 0,
		unread:  s.read == 0 && s.unread > 0,
		failure: AggregateFailures(op, failures, s.read+s.unread),
	})
}

// relatedResultTrunc is the answer of a checker that searched: ids found, and
// whether the search stopped short.
func relatedResultTrunc(target string, ids []string, truncated bool) resource.RelatedCheckResult {
	return relatedAnswer(target, relatedRead{ids: ids, partial: truncated})
}

// foundNone is the answer of a checker that read place, the one place its
// relation lives, and found it empty.
func foundNone(target, place string) resource.RelatedCheckResult {
	_ = place
	return relatedAnswer(target, relatedRead{})
}

// keyMissing is the answer of a checker whose source row lacks key, the id,
// ARN or name it would ask AWS with: nothing was read, so the answer is
// unknown, never a proven 0.
func keyMissing(target, key string) resource.RelatedCheckResult {
	_ = key
	return NotRead(target)
}

// NotRead is the answer of a checker that could not read where its relation
// lives and found nothing: unknown.
func NotRead(target string) resource.RelatedCheckResult {
	return relatedAnswer(target, relatedRead{unread: true})
}

// ReadFailed is the answer of a checker whose read failed before it found
// anything: the row carries err. A read that fails after rows were found
// reports them partial through relatedAnswer instead.
func ReadFailed(target string, err error) resource.RelatedCheckResult {
	return resource.ErrorRelated(target, err)
}

// alsoPartial is r once a second place the checker read turned out to be read
// only in part: a count becomes a lower bound, anything else is unchanged.
func alsoPartial(r resource.RelatedCheckResult, partial bool) resource.RelatedCheckResult {
	if !partial || r.State() != domain.RelatedResolved {
		return r
	}
	return r.PartialScan()
}

// alsoRead is r once a second place the checker read is folded in: a partial
// place makes a count a lower bound, an unread one a lower bound or, with
// nothing found, unknown or its error, and its failure reaches the flash. The
// place is read where r was, so r's Region stays the answer's.
func alsoRead(r resource.RelatedCheckResult, place relatedRead) resource.RelatedCheckResult {
	if !place.partial && !place.unread && place.failure == nil {
		return r
	}
	return relatedAnswer(r.TargetType(), joinReads(readOf(r), place)).WithRegion(r.Region())
}

// relatedFanOutCap bounds the calls a checker makes one per target row, for
// a relation AWS offers no reverse read of (docs/related-resources.md rule 7).
const relatedFanOutCap = EnrichmentCap

// fanOut is the part of rows a checker reads one call each for, and whether
// the cap left rows unread: a walk the cap stopped is a lower bound.
func fanOut[T any](rows []T) ([]T, bool) {
	if len(rows) > relatedFanOutCap {
		return rows[:relatedFanOutCap], true
	}
	return rows, false
}

// heuristicResult is the answer of a pivot that matches by a property every
// related resource must share with the source but unrelated ones may share
// too: the matches are candidates, and a complete scan that found none is a
// proven zero.
func heuristicResult(target string, ids []string, truncated bool) resource.RelatedCheckResult {
	if !truncated && len(ids) == 0 {
		return relatedAnswer(target, relatedRead{})
	}
	r := resource.HeuristicRelated(target, ids)
	if truncated {
		return r.PartialScan()
	}
	return r
}

// candidatesResult is heuristicResult for a property a related resource may
// lack: its matches are candidates, and a complete scan that found none has
// not read where the relation lives, so it is no answer.
func candidatesResult(target string, ids []string, truncated bool) resource.RelatedCheckResult {
	if len(ids) == 0 && !truncated {
		return relatedAnswer(target, relatedRead{unread: true})
	}
	return heuristicResult(target, ids, truncated)
}

// unreadZero is what a checker owes when it found nothing and could not read
// its source row. A disk-cache replay carries no RawStruct, so a zero there is
// a claim about a row nobody looked at — the checker had no filter to scan
// with, and "none" and "we have not looked" are not the same answer.
//
// Everything else passes through untouched: anything the checker did find, an
// error, an already-unknown, and a truncated zero — a truncated result means a
// real list was read with whatever filter the row's Fields could supply, and
// that lower bound is honest whether or not the RawStruct was there.
func unreadZero(res resource.Resource, r resource.RelatedCheckResult) resource.RelatedCheckResult {
	if res.RawStruct != nil || r.State() != domain.RelatedResolved || r.Count() != 0 || r.Truncated() {
		return r
	}
	return NotRead(r.TargetType()).WithRegion(r.Region())
}

// unreadZeroScanned is unreadZero for a checker that did read the target list.
// A zero over a population of none is proven whatever the source row could or
// could not say — no row existed for it to match — so only a zero over rows
// that were really there is a claim the source row has to back.
func unreadZeroScanned(res resource.Resource, scanned int, r resource.RelatedCheckResult) resource.RelatedCheckResult {
	if scanned == 0 {
		return r
	}
	return unreadZero(res, r)
}

// TargetClientMissing is resource.ClientMissing for the target type.
func TargetClientMissing(clients any, target string) error {
	return resource.ClientMissing(resource.TypeDef(target), clients)
}
