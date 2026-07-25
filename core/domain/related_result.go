// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package domain

import "sort"

// KnownRelated returns a proven RelatedCheckResult: the checker's lookup
// completed and ids is either the exhaustive match set (truncated == false)
// or the best-effort subset found so far, with more possibly unseen
// (truncated == true). Duplicate and empty IDs are dropped and the result is
// sorted for deterministic rendering.
//
// truncated == true covers two distinct callers uniformly, both rendering
// "(N+)": a scan over a cache page that was itself truncated, and a
// partial-success union where some independent calls succeeded (contributing
// ids) and at least one failed (so the true count may be higher). In the
// latter case the per-call error is deliberately not attached anywhere on the
// result — there is no constructor that accepts both ids and an error — so
// the row stays actionable ("N+") instead of collapsing to a dead end via
// EffectiveState. An empty ids with truncated == true ("(0+)") is the honest
// "scanned one page/call, found none yet, more may exist" — distinct from a
// proven zero (truncated == false), which is a dead end.
func KnownRelated(targetType string, ids []string, truncated bool) RelatedCheckResult {
	set := make(map[string]struct{}, len(ids))
	uniq := make([]string, 0, len(ids))
	for _, id := range ids {
		if id == "" {
			continue
		}
		if _, ok := set[id]; ok {
			continue
		}
		set[id] = struct{}{}
		uniq = append(uniq, id)
	}
	sort.Strings(uniq)
	return RelatedCheckResult{
		targetType:  targetType,
		count:       len(uniq),
		resourceIDs: uniq,
		truncated:   truncated,
	}
}

// UnknownRelated returns a RelatedCheckResult representing "the checker
// could not determine the count because a prerequisite lookup failed". The
// most common case is a two-hop checker (snapshot → source DB instance →
// cluster) where the SOURCE was not found in a truncated intermediate cache,
// so the hop to the TARGET was never attempted. Renders as the fourth visible
// state — a blank, navigable row (no count, drill in) — never "(?)".
func UnknownRelated(targetType string) RelatedCheckResult {
	return RelatedCheckResult{targetType: targetType, state: RelatedUnknown}
}

// ErrorRelated returns a RelatedCheckResult representing "the checker (or a
// prerequisite AWS call) returned an error". Renders blank (no count — "(?)"
// is forbidden) AND dimmed: unlike UnknownRelated it is a dead end, not
// navigable, because drilling into data that never resolved is misleading.
// The failure is surfaced separately through a Flash{IsError:true} + the "!"
// error log (Golden Contract rule 6); the user retries with Ctrl+R.
func ErrorRelated(targetType string, err error) RelatedCheckResult {
	return RelatedCheckResult{targetType: targetType, state: RelatedError, err: err}
}

// DeferredRelated returns a RelatedCheckResult representing "the count is not
// resolved locally; Enter should drill in via a server-side FetchFilter fetch
// instead". Renders with a blank count badge and is always actionable.
func DeferredRelated(targetType string, filter map[string]string) RelatedCheckResult {
	return RelatedCheckResult{targetType: targetType, state: RelatedDeferred, fetchFilter: filter}
}
