// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// Package costs is the pure domain layer for the Cost Explorer feature:
// query/record/period types, cache-key derivation, grid aggregation, and
// drill-chain rules consumed by core/aws (SDK mapping) and core/app
// (view-state assembly). No AWS SDK or TUI imports — see
// specs/021-cost-explorer/data-model.md for the full contract.
package costs

import (
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"
)

// Granularity of the visible time axis. YEAR and WEEK are client-side
// aggregations — the CE API serves only DAILY and MONTHLY (HOURLY unused).
type Granularity string

// Recognized Granularity values.
const (
	GranularityYear  Granularity = "year"
	GranularityMonth Granularity = "month"
	GranularityWeek  Granularity = "week"
	GranularityDay   Granularity = "day"
)

// APIGranularity returns the CE granularity that backs g: DAILY for
// week/day, MONTHLY for month/year. Single source for the aggregation rule
// (FR-005).
func (g Granularity) APIGranularity() string {
	switch g {
	case GranularityWeek, GranularityDay:
		return "DAILY"
	default:
		return "MONTHLY"
	}
}

// Metric keys stored metric values on a Record — the five display modes
// cycled by 'b'.
type Metric string

// Recognized Metric values.
const (
	MetricInvoice      Metric = "invoice"
	MetricUnblended    Metric = "unblended"
	MetricAmortized    Metric = "amortized"
	MetricNetAmortized Metric = "net-amortized"
	MetricBlended      Metric = "blended"
)

// Dimension is a CE group-by/filter dimension used by pivots and drills.
type Dimension string

// Recognized Dimension values.
const (
	DimensionService       Dimension = "SERVICE"
	DimensionRegion        Dimension = "REGION"
	DimensionLinkedAccount Dimension = "LINKED_ACCOUNT"
	DimensionUsageType     Dimension = "USAGE_TYPE"
	DimensionPurchaseType  Dimension = "PURCHASE_TYPE"
	DimensionRecordType    Dimension = "RECORD_TYPE"
	DimensionResourceID    Dimension = "RESOURCE_ID"
)

// regionlessDisplayKey is the GROUPING key CE reports for a region-less
// charge (e.g. Tax) under the REGION dimension.
const regionlessDisplayKey = "NoRegion"

// FilterValueForKey translates a grid row's display key — as CE reports it
// in a GroupBy result — into the value that must be used to FILTER on dim.
// Identity for every dimension except REGION: CE groups region-less charges
// under the literal grouping key "NoRegion" but that string never matches
// as a FILTER value — only the empty string does (live-verified: filtering
// REGION=["NoRegion"] returns $0.00, REGION=[""] returns the real amount).
// DisplayKeyForFilterValue is this function's exact inverse.
func FilterValueForKey(dim Dimension, displayKey string) string {
	if dim == DimensionRegion && displayKey == regionlessDisplayKey {
		return ""
	}
	return displayKey
}

// DisplayKeyForFilterValue is FilterValueForKey's inverse — the
// human-readable form to render (breadcrumb, frame title) for a pinned
// filter value, so a drill pinned on REGION's legitimately empty filter
// value never renders as a blank segment.
func DisplayKeyForFilterValue(dim Dimension, value string) string {
	if dim == DimensionRegion && value == "" {
		return regionlessDisplayKey
	}
	return value
}

// Period is one time bucket [Start, End) in YYYY-MM-DD (CE convention).
type Period struct {
	Start string `yaml:"start"`
	End   string `yaml:"end"`
}

// utcDate truncates now to its own UTC calendar date at midnight — now.UTC()
// FIRST, so the Y/M/D extracted are the UTC calendar's, never now's own zone
// (a caller in UTC+13 at local midnight is still on the PREVIOUS UTC day).
// The single truncation point every UTC-calendar-day decision in this
// package shares, so none can silently reconstruct a zone-local date instead.
func utcDate(now time.Time) time.Time {
	u := now.UTC()
	return time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC)
}

// Closed reports whether p ended on or before the first day of now's month
// — closed periods are immutable in CE and cached forever. now is injected;
// this package never calls time.Now() itself.
func (p Period) Closed(now time.Time) bool {
	end, err := ParseDate(p.End)
	if err != nil {
		return false
	}
	nowUTC := utcDate(now)
	firstOfMonth := time.Date(nowUTC.Year(), nowUTC.Month(), 1, 0, 0, 0, 0, time.UTC)
	return !end.After(firstOfMonth)
}

// dateLayout is the date-only format every period boundary ("YYYY-MM-DD") is
// stored and parsed in — the single layout string ParseDate/FormatDate
// share, so no caller can drift onto a different layout for the same data.
const dateLayout = "2006-01-02"

// ParseDate parses s as a9s's period-boundary date format. The single point
// every date-only Period.Start/End string in this package (and
// core/app's costs consumers) is parsed through, so a layout change
// only ever touches dateLayout.
func ParseDate(s string) (time.Time, error) {
	return time.Parse(dateLayout, s)
}

// FormatDate formats t as a9s's period-boundary date format — ParseDate's
// exact inverse.
func FormatDate(t time.Time) string {
	return t.Format(dateLayout)
}

// Amount is one metric value as CE returns it. Amounts stay
// strings-parsed-to-float64 at the edge; currency travels with the value
// (no conversion is ever performed).
type Amount struct {
	Value float64 `yaml:"v"`
	Unit  string  `yaml:"u"`
}

// Record is the atomic cached fact: one group-key tuple x one period x all
// metrics. Fetchers produce these; everything else (grids, drills) derives
// from them.
type Record struct {
	Period    Period            `yaml:"p"`
	Keys      []string          `yaml:"k"`
	Metrics   map[Metric]Amount `yaml:"m"`
	Estimated bool              `yaml:"est,omitempty"`
}

// Filter is a normalized subset of the CE Expression tree sufficient for
// this feature: AND of dimension equals-any-of clauses, plus AND of
// dimension not-equals-any-of clauses (mapped to CE Not{Dimensions{...}}
// by core/aws.buildFilterExpression).
type Filter struct {
	Equals    map[Dimension][]string `yaml:"eq,omitempty"`
	NotEquals map[Dimension][]string `yaml:"neq,omitempty"`
}

// Query is the canonical fetch shape. CacheKey() is derived only from
// fields that change the result set: granularity, group-bys, filter — the
// period range is stored per-record, so one Query shape may be satisfied by
// many cached periods.
type Query struct {
	Granularity string
	GroupBy     []Dimension
	Filter      Filter
	Range       Period
}

// CacheKey returns the deterministic key identifying the result *shape* of
// q: granularity, group-by dimensions, and filter clauses (both Equals and
// NotEquals), all sorted so map iteration order and GroupBy/filter
// construction order never leak into the key. Range is deliberately
// excluded — it only determines which periods a given CacheKey's cache
// entries cover, not the shape itself.
func (q Query) CacheKey() string {
	dims := make([]string, len(q.GroupBy))
	for i, d := range q.GroupBy {
		dims[i] = string(d)
	}
	sort.Strings(dims)

	return fmt.Sprintf("%s|%s|eq:%s|neq:%s",
		q.Granularity, strings.Join(dims, ","),
		strings.Join(filterClauses(q.Filter.Equals), ";"),
		strings.Join(filterClauses(q.Filter.NotEquals), ";"),
	)
}

// Identity returns the deterministic identity of ONE fetch request: its
// result shape (CacheKey) plus the exact period range requested. THE single
// source both ensureCostsShapeFetched's TaskKey.Scope (core/app/
// costs_state.go — distinct fetch shapes/ranges must get distinct TaskKeys,
// or the web transport's in-flight dedup silently drops the second one) and
// ApplyCostsLoaded's awaited-result matching consume, rather than each
// re-deriving its own CacheKey+Range comparison.
func (q Query) Identity() string {
	return fmt.Sprintf("%s|range:%s-%s", q.CacheKey(), q.Range.Start, q.Range.End)
}

// filterClauses renders m's dimension clauses in a stable, sorted form,
// shared by CacheKey's Equals and NotEquals treatment.
func filterClauses(m map[Dimension][]string) []string {
	dims := make([]Dimension, 0, len(m))
	for d := range m {
		dims = append(dims, d)
	}
	slices.Sort(dims)

	clauses := make([]string, 0, len(dims))
	for _, d := range dims {
		vals := append([]string(nil), m[d]...)
		sort.Strings(vals)
		clauses = append(clauses, fmt.Sprintf("%s=%s", d, strings.Join(vals, ",")))
	}
	return clauses
}
