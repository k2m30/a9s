// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package costs

// AnomalyMark is one CE cost anomaly overlaid on grid cells that fall
// within its Period and match its Dimension values.
type AnomalyMark struct {
	ID        string
	Score     float64
	Impact    Amount
	Period    Period
	RootCause string
	Dimension map[Dimension]string
}

// AnomalyResult is the outcome of one GetAnomalies attempt, distinguishing
// "the fetch was skipped" (Requested false — Marks/Err carry no meaning)
// from "the fetch ran": a genuinely empty Marks with Requested true and
// Err nil is an authoritative zero-anomalies result and clears a stale
// cached mark; Requested true with Err set is a failed attempt that must
// neither clear nor renew the cache, since nothing was actually learned.
// The single value every caller of (*Store).ApplyFetchResult constructs
// instead of passing a bare []AnomalyMark, whose nil-ness alone cannot
// distinguish "skipped" from "requested, found nothing".
type AnomalyResult struct {
	Requested bool
	Marks     []AnomalyMark
	Err       error
	// Truncated is true when the underlying GetAnomalies fetch was cut short
	// by a page cap with more pages still available (mirrors
	// GridResult.Truncated) — Marks is then a lower bound, not CE's own
	// authoritative complete list for the window, and must never be cached as
	// a confirmed 24h-TTL snapshot (see (*Store).ApplyFetchResult).
	Truncated bool
}

// GridResult is the outcome of one grid (GetCostAndUsage[WithResources])
// fetch attempt, distinguishing "the fetch was skipped" (Fetched false —
// Records/Err carry no meaning) from "the fetch ran": a genuinely empty
// Records with Fetched true is CE's own authoritative zero-group result,
// never to be mistaken for an unrelated anomaly-only (skipped-grid)
// delivery. Symmetric with AnomalyResult.
type GridResult struct {
	Fetched bool
	Records []Record
	Err     error
	// Truncated is true when the underlying CE grid fetch was cut short by a
	// pagination page cap with more pages still available — Records is then
	// a lower bound, not CE's own authoritative complete result, and callers
	// must render it as partial rather than as complete totals.
	Truncated bool
}
