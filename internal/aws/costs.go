// costs.go — Cost Explorer fetchers: SDK <-> internal/costs domain mapping,
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

	"github.com/k2m30/a9s/v3/internal/costs"
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
// unmodified so the caller can render/log the raw failure. Code-matching
// delegates to ClassifyAWSError (internal/aws/errors.go), the shared
// taxonomy — it recognizes both the bare "AccessDenied" code some AWS API
// paths return and the "...Exception"-suffixed form, where this file's own
// switch previously matched only the latter.
func classifyCostsError(err error) error {
	if err == nil {
		return nil
	}
	code, _, _ := ClassifyAWSError(err)
	switch code {
	case "AccessDenied", "AccessDeniedException":
		return fmt.Errorf("%w: %w", ErrCostsAccessDenied, err)
	case "DataUnavailableException":
		return fmt.Errorf("%w: %w", ErrCostsDataUnavailable, err)
	case "Throttling", "ThrottlingException", "TooManyRequestsException", "RequestLimitExceeded":
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
	for {
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

		if page.NextPageToken == nil {
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

// FetchDimensionValues paginates GetDimensionValues to exhaustion, mapping
// dimension value -> its "description" attribute (e.g. linked-account id ->
// account name).
func FetchDimensionValues(ctx context.Context, api CostsGetDimensionValuesAPI, dim costs.Dimension, window costs.Period) (map[string]string, error) {
	attrs := make(map[string]string)
	input := &costexplorer.GetDimensionValuesInput{
		Dimension:  cetypes.Dimension(dim),
		TimePeriod: &cetypes.DateInterval{Start: aws.String(window.Start), End: aws.String(window.End)},
	}

	for {
		out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*costexplorer.GetDimensionValuesOutput, error) {
			return api.GetDimensionValues(ctx, input)
		})
		if err != nil {
			return nil, classifyCostsError(err)
		}
		mapAttrs(attrs, out.DimensionValues)
		if out.NextPageToken == nil {
			break
		}
		input.NextPageToken = out.NextPageToken
	}
	return attrs, nil
}

// FetchCostAnomalies paginates GetAnomalies to exhaustion, mapping each
// anomaly's first root cause into a preformatted RootCause string and a
// Dimension map for per-cell matching (FR-014).
func FetchCostAnomalies(ctx context.Context, api CostsGetAnomaliesAPI, window costs.Period) ([]costs.AnomalyMark, error) {
	marks, _, err := FetchCostAnomaliesCounted(ctx, api, window)
	return marks, err
}

// FetchCostAnomaliesCounted is FetchCostAnomalies plus the number of
// GetAnomalies pages actually requested — CostsLoaded.Requests must fold
// this in alongside the main cost-and-usage fetch's own count, since the
// anomaly overlay is a separate billed CE call riding alongside it.
func FetchCostAnomaliesCounted(ctx context.Context, api CostsGetAnomaliesAPI, window costs.Period) ([]costs.AnomalyMark, int, error) {
	var marks []costs.AnomalyMark
	requests := 0
	input := &costexplorer.GetAnomaliesInput{
		DateInterval: &cetypes.AnomalyDateInterval{StartDate: aws.String(window.Start), EndDate: aws.String(window.End)},
	}

	for {
		out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*costexplorer.GetAnomaliesOutput, error) {
			return api.GetAnomalies(ctx, input)
		})
		requests++
		if err != nil {
			return nil, requests, classifyCostsError(err)
		}
		for _, a := range out.Anomalies {
			marks = append(marks, mapAnomaly(a))
		}
		if out.NextPageToken == nil {
			break
		}
		input.NextPageToken = out.NextPageToken
	}
	return marks, requests, nil
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
