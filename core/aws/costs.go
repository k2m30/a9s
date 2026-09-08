// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// costs.go — Cost Explorer fetchers: SDK <-> core/costs domain mapping,
// pagination, and error classification. Read-only (FR-016): only Get*
// operations are ever called.
package aws

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer"
	cetypes "github.com/aws/aws-sdk-go-v2/service/costexplorer/types"

	"github.com/k2m30/a9s/v3/core/costs"
)

// ceMetrics are the four underlying CE metrics requested on every call.
var ceMetrics = []string{"UnblendedCost", "BlendedCost", "AmortizedCost", "NetAmortizedCost"}

// CostFetchResult carries mapped records/attrs plus a request counter for
// the $-per-session footer (FR-013): the counter is data returned by the
// fetcher, not a side effect the caller must separately track.
type CostFetchResult struct {
	Records      []costs.Record
	Attrs        map[string]string
	RequestCount int
	// Truncated is true when drainCostUsagePages stopped at costsGridPageCap
	// with more pages still available — Records is then a lower bound, never
	// a discovered-complete zero-group result, and must render as partial
	// rather than as authoritative totals (FR-017: partial dollars rendered
	// as complete dollars is a correctness defect).
	Truncated bool
}

// Typed sentinel errors surfaced as explicit view states (FR-017): an
// empty grid must never masquerade as zero spend.
var (
	ErrCostsAccessDenied    = errors.New("cost explorer: access denied")
	ErrCostsDataUnavailable = errors.New("cost explorer: data not available for this account")
	ErrCostsThrottled       = errors.New("cost explorer: request throttled")

	// ErrCostsResourceDrillMissingServiceFilter guards
	// GetCostAndUsageWithResources: CE rejects a resource-level call that
	// doesn't isolate exactly one SERVICE. The drill query always pins one
	// before reaching RESOURCE_ID (costs.ResourceDrillAllowed), so this
	// surfaces as an explicit ErrorMsg (FR-017) rather than a raw CE 400 if
	// that gate is ever bypassed.
	ErrCostsResourceDrillMissingServiceFilter = errors.New("cost explorer: resource-level fetch requires exactly one SERVICE filter")
)

// classifyCostsError wraps err with the matching typed sentinel when it
// carries a recognized CE error code; anything else passes through
// unmodified so the caller can render/log the raw failure. Which codes are a
// denial and which a throttle is the class table's decision, not this file's.
func classifyCostsError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case IsAccessDenied(err):
		return fmt.Errorf("%w: %w", ErrCostsAccessDenied, err)
	case ErrCodeIs(err, "DataUnavailableException"):
		return fmt.Errorf("%w: %w", ErrCostsDataUnavailable, err)
	case ErrClass(err) == ClassThrottled:
		return fmt.Errorf("%w: %w", ErrCostsThrottled, err)
	}
	return err
}

// invoiceMetricKey returns the costs.Metric key that UnblendedCost should
// be parsed into: "unblended" only for the unblended display shape's own
// NotEquals[RECORD_TYPE] Tax/Credit/Refund exclusion (data-model.md's
// display mapping) — a distinct Query/CacheKey scoped to a specific set of
// record types. An Equals[RECORD_TYPE] clause is a RECORD_TYPE pivot/drill
// in invoice mode (digit 6 then Enter), not the unblended shape, and stays
// keyed under "invoice" so cs.Metric=="invoice" finds it.
func invoiceMetricKey(f costs.Filter) costs.Metric {
	if _, has := f.NotEquals[costs.DimensionRecordType]; has {
		return costs.MetricUnblended
	}
	return costs.MetricInvoice
}

func buildGroupBy(dims []costs.Dimension) []cetypes.GroupDefinition {
	if len(dims) == 0 {
		return nil
	}
	out := make([]cetypes.GroupDefinition, len(dims))
	for i, d := range dims {
		out[i] = cetypes.GroupDefinition{Type: cetypes.GroupDefinitionTypeDimension, Key: aws.String(string(d))}
	}
	return out
}

func buildFilterExpression(f costs.Filter) *cetypes.Expression {
	exprs := dimensionExpressions(f.Equals)
	for _, d := range sortedFilterDims(f.NotEquals) {
		exprs = append(exprs, cetypes.Expression{
			Not: &cetypes.Expression{
				Dimensions: &cetypes.DimensionValues{Key: cetypes.Dimension(d), Values: f.NotEquals[d]},
			},
		})
	}
	switch len(exprs) {
	case 0:
		return nil
	case 1:
		return &exprs[0]
	default:
		return &cetypes.Expression{And: exprs}
	}
}

func sortedFilterDims(m map[costs.Dimension][]string) []costs.Dimension {
	dims := make([]costs.Dimension, 0, len(m))
	for d := range m {
		dims = append(dims, d)
	}
	slices.Sort(dims)
	return dims
}

func dimensionExpressions(m map[costs.Dimension][]string) []cetypes.Expression {
	dims := sortedFilterDims(m)
	exprs := make([]cetypes.Expression, 0, len(dims))
	for _, d := range dims {
		exprs = append(exprs, cetypes.Expression{
			Dimensions: &cetypes.DimensionValues{Key: cetypes.Dimension(d), Values: m[d]},
		})
	}
	return exprs
}

// mapCEGroup parses one CE Group's metrics into a costs.Record, mapping
// UnblendedCost into invoiceKey and the remaining three metrics into their
// fixed keys. Any other metric CE returns (e.g. a still-requested-but-
// unmapped one) is silently skipped, not an error.
func mapCEGroup(g cetypes.Group, period costs.Period, estimated bool, invoiceKey costs.Metric) (costs.Record, error) {
	metrics := make(map[costs.Metric]costs.Amount, len(g.Metrics))
	for sdkKey, mv := range g.Metrics {
		var key costs.Metric
		switch sdkKey {
		case "UnblendedCost":
			key = invoiceKey
		case "BlendedCost":
			key = costs.MetricBlended
		case "AmortizedCost":
			key = costs.MetricAmortized
		case "NetAmortizedCost":
			key = costs.MetricNetAmortized
		default:
			continue
		}
		val, err := strconv.ParseFloat(aws.ToString(mv.Amount), 64)
		if err != nil {
			return costs.Record{}, fmt.Errorf("costs: parsing %s amount %q: %w", sdkKey, aws.ToString(mv.Amount), err)
		}
		metrics[key] = costs.Amount{Value: val, Unit: aws.ToString(mv.Unit)}
	}
	return costs.Record{
		Period:    period,
		Keys:      append([]string(nil), g.Keys...),
		Metrics:   metrics,
		Estimated: estimated,
	}, nil
}

func mapAttrs(dst map[string]string, attrs []cetypes.DimensionValuesWithAttributes) {
	for _, a := range attrs {
		dst[aws.ToString(a.Value)] = a.Attributes["description"]
	}
}

// costsGridPageCap bounds drainCostUsagePages to at most this many
// GetCostAndUsage[WithResources] requests per fetch. GetCostAndUsage has no
// SDK-generated paginator to inherit a stop condition from, every request is
// a billed $0.01 CE call (RequestCount), and KindFetchCosts runs with no
// wall-clock backstop of its own (executor.go forwards the caller's ctx
// unwrapped) — so a page cap is the only thing standing between a
// misbehaving response and an unbounded bill.
const costsGridPageCap = 50

// costsAnomalyPageCap bounds FetchCostAnomalies the same way, at a smaller
// ceiling: GetAnomalies' own generated paginator already treats a
// nil-or-empty NextPageToken as terminal (the contract this file's hand-rolled
// loop now matches), so this cap is a pure belt against an account with an
// unusually large anomaly backlog, not a correctness fix on its own. Firing
// it sets AnomaliesFetchResult.Truncated rather than silently capping the
// mark set — the same "lower bound, not a discovered-complete result"
// distinction CostFetchResult.Truncated already makes for the grid fetch.
const costsAnomalyPageCap = 20

// costUsagePage is the per-page shape GetCostAndUsage and
// GetCostAndUsageWithResources responses both carry — drainCostUsagePages
// drains either one via a caller-supplied fetch-one-page closure, so the
// two fetchers share one pagination/mapping loop.
type costUsagePage struct {
	ResultsByTime            []cetypes.ResultByTime
	DimensionValueAttributes []cetypes.DimensionValuesWithAttributes
	NextPageToken            *string
}

// drainCostUsagePages pages fetchPage to exhaustion, mapping every page's
// groups into Records and every page's DimensionValueAttributes into
// Attrs. Every request — including a failed one — counts toward
// RequestCount (FR-013).
func drainCostUsagePages(invoiceKey costs.Metric, fetchPage func(nextToken *string) (costUsagePage, error)) (CostFetchResult, error) {
	result := CostFetchResult{Attrs: make(map[string]string)}
	var token *string
	for pages := 0; ; pages++ {
		page, err := fetchPage(token)
		result.RequestCount++
		if err != nil {
			return result, classifyCostsError(err)
		}

		mapAttrs(result.Attrs, page.DimensionValueAttributes)
		for _, rbt := range page.ResultsByTime {
			period := costs.Period{Start: aws.ToString(rbt.TimePeriod.Start), End: aws.ToString(rbt.TimePeriod.End)}
			for _, g := range rbt.Groups {
				rec, err := mapCEGroup(g, period, rbt.Estimated, invoiceKey)
				if err != nil {
					return result, err
				}
				result.Records = append(result.Records, rec)
			}
		}

		// Matches the stop condition every AWS-generated paginator in this
		// SDK uses (e.g. kms.ListAliasesPaginator.HasMorePages): a non-nil
		// but empty NextPageToken is exhaustion, not "one more page" — this
		// API has no generated paginator of its own to inherit that contract
		// from, so it is stated explicitly here.
		if page.NextPageToken == nil || *page.NextPageToken == "" {
			return result, nil
		}
		if pages+1 >= costsGridPageCap {
			result.Truncated = true
			return result, nil
		}
		token = page.NextPageToken
	}
}

// FetchCostAndUsage paginates GetCostAndUsage to exhaustion.
func FetchCostAndUsage(ctx context.Context, api CostsGetCostAndUsageAPI, q costs.Query) (CostFetchResult, error) {
	input := &costexplorer.GetCostAndUsageInput{
		Granularity: cetypes.Granularity(q.Granularity),
		Metrics:     ceMetrics,
		TimePeriod:  &cetypes.DateInterval{Start: aws.String(q.Range.Start), End: aws.String(q.Range.End)},
		GroupBy:     buildGroupBy(q.GroupBy),
		Filter:      buildFilterExpression(q.Filter),
	}
	return drainCostUsagePages(invoiceMetricKey(q.Filter), func(token *string) (costUsagePage, error) {
		input.NextPageToken = token
		out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*costexplorer.GetCostAndUsageOutput, error) {
			return api.GetCostAndUsage(ctx, input)
		})
		if err != nil {
			return costUsagePage{}, err
		}
		return costUsagePage{out.ResultsByTime, out.DimensionValueAttributes, out.NextPageToken}, nil
	})
}

// FetchCostAndUsageWithResources paginates GetCostAndUsageWithResources to
// exhaustion — the resource-level drill (FR-007), gated by
// costs.ResourceDrillAllowed before this is ever called.
func FetchCostAndUsageWithResources(ctx context.Context, api CostsGetCostAndUsageWithResourcesAPI, q costs.Query) (CostFetchResult, error) {
	input := &costexplorer.GetCostAndUsageWithResourcesInput{
		Granularity: cetypes.Granularity(q.Granularity),
		Metrics:     ceMetrics,
		TimePeriod:  &cetypes.DateInterval{Start: aws.String(q.Range.Start), End: aws.String(q.Range.End)},
		GroupBy:     buildGroupBy(q.GroupBy),
		Filter:      buildFilterExpression(q.Filter),
	}
	return drainCostUsagePages(invoiceMetricKey(q.Filter), func(token *string) (costUsagePage, error) {
		input.NextPageToken = token
		out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*costexplorer.GetCostAndUsageWithResourcesOutput, error) {
			return api.GetCostAndUsageWithResources(ctx, input)
		})
		if err != nil {
			return costUsagePage{}, err
		}
		return costUsagePage{out.ResultsByTime, out.DimensionValueAttributes, out.NextPageToken}, nil
	})
}

// AnomaliesFetchResult is the outcome of one FetchCostAnomalies call,
// mirroring CostFetchResult.Truncated for the anomaly overlay: Truncated is
// true when costsAnomalyPageCap fired before GetAnomalies' own
// NextPageToken exhaustion, meaning Marks is a lower bound — anomalies past
// the cap were never requested — rather than CE's own authoritative
// complete list for the window.
type AnomaliesFetchResult struct {
	Marks     []costs.AnomalyMark
	Requests  int
	Truncated bool
}

// FetchCostAnomalies paginates GetAnomalies up to costsAnomalyPageCap,
// mapping each anomaly's first root cause into a preformatted RootCause
// string and a Dimension map for per-cell matching (FR-014). Requests is
// the number of GetAnomalies pages actually requested — CostsLoaded.Requests
// must fold this in alongside the main cost-and-usage fetch's own count,
// since the anomaly overlay is a separate billed CE call riding alongside
// it.
func FetchCostAnomalies(ctx context.Context, api CostsGetAnomaliesAPI, window costs.Period) (AnomaliesFetchResult, error) {
	var result AnomaliesFetchResult
	input := &costexplorer.GetAnomaliesInput{
		DateInterval: &cetypes.AnomalyDateInterval{StartDate: aws.String(window.Start), EndDate: aws.String(window.End)},
	}

	for pages := 0; ; pages++ {
		out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*costexplorer.GetAnomaliesOutput, error) {
			return api.GetAnomalies(ctx, input)
		})
		result.Requests++
		if err != nil {
			return result, classifyCostsError(err)
		}
		for _, a := range out.Anomalies {
			result.Marks = append(result.Marks, mapAnomaly(a))
		}
		// Same nil-or-empty stop condition GetAnomaliesPaginator.HasMorePages
		// itself uses — this hand-rolled loop now matches its own API's
		// generated paginator instead of diverging from it.
		if out.NextPageToken == nil || *out.NextPageToken == "" {
			return result, nil
		}
		if pages+1 >= costsAnomalyPageCap {
			result.Truncated = true
			return result, nil
		}
		input.NextPageToken = out.NextPageToken
	}
}

// FetchCostAnomaliesCounted is FetchCostAnomalies with its result unpacked
// into the (marks, requests, err) tuple this package's existing callers
// expect: an error on any page discards whatever marks earlier pages had
// already collected, the same all-or-nothing behavior FetchCostAnomalies
// itself has on error. It drops AnomaliesFetchResult.Truncated, so a caller
// reached through this entry point cannot tell a page-capped fetch apart
// from a genuinely complete one — FetchCostAnomalies is the entry point
// that carries that signal through to CostsLoaded.
func FetchCostAnomaliesCounted(ctx context.Context, api CostsGetAnomaliesAPI, window costs.Period) ([]costs.AnomalyMark, int, error) {
	result, err := FetchCostAnomalies(ctx, api, window)
	if err != nil {
		return nil, result.Requests, err
	}
	return result.Marks, result.Requests, nil
}

// rootCauseDim pairs a RootCause struct field with its costs.Dimension key
// and preformatted label, in the fixed order the RootCause string renders.
type rootCauseDim struct {
	dim   costs.Dimension
	value *string
}

// anomalyDateOnly normalizes a GetAnomalies date string to date-only
// ("2006-01-02"): live CE sends full RFC3339 timestamps
// ("2026-06-15T00:00:00Z"), the demo fixture sends bare dates already —
// grid column matching is exact costs.Period string equality against
// date-only periods, so an un-normalized RFC3339 value would never match
// any column.
func anomalyDateOnly(s string) string {
	date, _, _ := strings.Cut(s, "T")
	return date
}

func mapAnomaly(a cetypes.Anomaly) costs.AnomalyMark {
	dim := make(map[costs.Dimension]string)
	var parts []string

	if len(a.RootCauses) > 0 {
		rc := a.RootCauses[0]
		fields := []rootCauseDim{
			{costs.DimensionService, rc.Service},
			{costs.DimensionRegion, rc.Region},
			{costs.DimensionLinkedAccount, rc.LinkedAccount},
			{costs.DimensionUsageType, rc.UsageType},
		}
		for _, f := range fields {
			if f.value == nil {
				continue
			}
			dim[f.dim] = *f.value
			parts = append(parts, fmt.Sprintf("%s=%s", f.dim, *f.value))
		}
	}

	var score float64
	if a.AnomalyScore != nil {
		score = a.AnomalyScore.CurrentScore
	}
	var impact costs.Amount
	if a.Impact != nil {
		impact = costs.Amount{Value: a.Impact.MaxImpact, Unit: "USD"}
	}

	return costs.AnomalyMark{
		ID:     aws.ToString(a.AnomalyId),
		Score:  score,
		Impact: impact,
		Period: costs.Period{
			Start: anomalyDateOnly(aws.ToString(a.AnomalyStartDate)),
			End:   anomalyDateOnly(aws.ToString(a.AnomalyEndDate)),
		},
		RootCause: strings.Join(parts, ", "),
		Dimension: dim,
	}
}
