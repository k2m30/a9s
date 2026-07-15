// costs_handlers.go — demo transport handlers for AWS Cost Explorer
// (service prefix "ce", awsjson1.1, X-Amz-Target
// "AWSInsightsIndexService.<Operation>"). Cost Explorer has no typed-fake
// client path (internal/aws.CostsAPI is shaped around SDK response types,
// not a mockable Go interface with per-field fixture data), so it is served
// as raw JSON like STS — built from internal/demo/fixtures/costs.go,
// filtered/grouped per the request's Granularity, GroupBy, and Filter so
// the growth-story drill (SERVICE=EC2 - Other x USAGE_TYPE) narrows
// correctly.
package demo

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/k2m30/a9s/v3/core/costs"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
)

// registerCostExplorerHandlers registers the four CE operations the Cost
// Explorer feature calls (internal/aws/costs.go): GetCostAndUsage,
// GetCostAndUsageWithResources, GetAnomalies, GetDimensionValues.
func registerCostExplorerHandlers(t *Transport) {
	fx := fixtures.NewCostsFixtures()
	t.Handle("ce", "GetCostAndUsage", func(req *http.Request) (*http.Response, error) {
		return costsGetCostAndUsageResponse(fx, req)
	})
	t.Handle("ce", "GetCostAndUsageWithResources", costsGetCostAndUsageWithResourcesResponse)
	t.Handle("ce", "GetAnomalies", func(_ *http.Request) (*http.Response, error) {
		return costsGetAnomaliesResponse(fx)
	})
	t.Handle("ce", "GetDimensionValues", func(req *http.Request) (*http.Response, error) {
		return costsGetDimensionValuesResponse(fx, req)
	})
}

// ceDimensionFilter mirrors the CE Expression tree shape this feature ever
// sends (internal/aws/costs.go's buildFilterExpression): a single
// Dimensions clause, an And-list of Dimensions/Not clauses, or a Not
// wrapping one Dimensions clause (the unblended metric's RECORD_TYPE
// exclusion).
type ceDimensionFilter struct {
	Dimensions *struct {
		Key    string   `json:"Key"`
		Values []string `json:"Values"`
	} `json:"Dimensions"`
	And []ceDimensionFilter `json:"And"`
	Not *ceDimensionFilter  `json:"Not"`
}

// flatten splits f into its equals and not-equals dimension clauses,
// mirroring costs.Filter's Equals/NotEquals split.
func (f *ceDimensionFilter) flatten() (eq, notEq map[string][]string) {
	eq, notEq = map[string][]string{}, map[string][]string{}
	f.collect(eq, notEq, false)
	return eq, notEq
}

func (f *ceDimensionFilter) collect(eq, notEq map[string][]string, negate bool) {
	if f == nil {
		return
	}
	dst := eq
	if negate {
		dst = notEq
	}
	if f.Dimensions != nil {
		dst[f.Dimensions.Key] = f.Dimensions.Values
	}
	for i := range f.And {
		f.And[i].collect(eq, notEq, negate)
	}
	f.Not.collect(eq, notEq, !negate)
}

type ceGetCostAndUsageInput struct {
	Granularity string `json:"Granularity"`
	GroupBy     []struct {
		Type string `json:"Type"`
		Key  string `json:"Key"`
	} `json:"GroupBy"`
	Filter     *ceDimensionFilter `json:"Filter"`
	TimePeriod struct {
		Start string `json:"Start"`
		End   string `json:"End"`
	} `json:"TimePeriod"`
}

type ceGetDimensionValuesInput struct {
	Dimension  string `json:"Dimension"`
	TimePeriod struct {
		Start string `json:"Start"`
		End   string `json:"End"`
	} `json:"TimePeriod"`
}

// costsRowDimValue resolves row's value for one CE group-by/filter
// dimension. REGION/PURCHASE_TYPE are uniform across the demo dataset
// (single-region fixture) — grouping by them still returns one non-empty
// row rather than an error. LINKED_ACCOUNT reads the row's own split-account
// tag (fixtures.splitRowsAcrossAccounts), so grouping by it returns both
// synthetic accounts realistically instead of one always-same row.
func costsRowDimValue(row fixtures.CostsRow, dim string) string {
	switch dim {
	case "SERVICE":
		return row.Service
	case "USAGE_TYPE":
		return row.UsageType
	case "RECORD_TYPE":
		return row.RecordType
	case "REGION":
		return fixtures.CostsDemoRegion
	case "LINKED_ACCOUNT":
		return row.Account
	case "PURCHASE_TYPE":
		return fixtures.CostsPurchaseOnDemand
	}
	return ""
}

func costsRowMatchesFilter(row fixtures.CostsRow, eq, notEq map[string][]string) bool {
	for dim, values := range eq {
		if len(values) == 0 {
			continue
		}
		if !slices.Contains(values, costsRowDimValue(row, dim)) {
			return false
		}
	}
	for dim, values := range notEq {
		if len(values) == 0 {
			continue
		}
		if slices.Contains(values, costsRowDimValue(row, dim)) {
			return false
		}
	}
	return true
}

// ceGroupCell accumulates one (month, group-key-tuple)'s summed amount.
type ceGroupCell struct {
	vals   []string
	amount float64
}

func costsGetCostAndUsageResponse(fx *fixtures.CostsFixtures, req *http.Request) (*http.Response, error) {
	var in ceGetCostAndUsageInput
	if err := readJSONBody(req, &in); err != nil {
		return errorResponse(400, "SerializationException", err.Error()), nil
	}

	groupDims := make([]string, len(in.GroupBy))
	for i, gb := range in.GroupBy {
		groupDims[i] = gb.Key
	}
	eqFilter, notEqFilter := in.Filter.flatten()

	var resultsByTime []map[string]any
	if in.Granularity == "DAILY" {
		resultsByTime = costsDailyResultsByTime(fx, groupDims, eqFilter, notEqFilter, in.TimePeriod.Start, in.TimePeriod.End)
	} else {
		resultsByTime = costsMonthlyResultsByTime(fx, groupDims, eqFilter, notEqFilter, in.TimePeriod.Start, in.TimePeriod.End)
	}

	out := map[string]any{
		"ResultsByTime":            resultsByTime,
		"DimensionValueAttributes": costsDimensionValueAttributes(groupDims),
		"NextPageToken":            nil,
	}
	return jsonOKResponse(out)
}

// costsGetCostAndUsageWithResourcesResponse serves the resource-level drill
// (FR-007/FR-008): one bucket per day in the requested range, one group per
// fixtures.CostsResourceRow registered under every SERVICE the request's
// filter pins — a service with no registered rows returns an empty (but
// valid) ResultsByTime, not an error.
func costsGetCostAndUsageWithResourcesResponse(req *http.Request) (*http.Response, error) {
	var in ceGetCostAndUsageInput
	if err := readJSONBody(req, &in); err != nil {
		return errorResponse(400, "SerializationException", err.Error()), nil
	}
	eqFilter, _ := in.Filter.flatten()

	var rows []fixtures.CostsResourceRow
	for _, svc := range eqFilter["SERVICE"] {
		rows = append(rows, fixtures.CostsResourceRowsByService[svc]...)
	}

	start, startErr := costs.ParseDate(in.TimePeriod.Start)
	end, endErr := costs.ParseDate(in.TimePeriod.End)

	var resultsByTime []map[string]any
	if startErr == nil && endErr == nil {
		for d := start; d.Before(end); d = d.AddDate(0, 0, 1) {
			groupList := make([]map[string]any, 0, len(rows))
			for _, row := range rows {
				groupList = append(groupList, map[string]any{
					"Keys":    []string{row.ResourceID},
					"Metrics": costsMetricValues(row.DailyAmount),
				})
			}
			dayStart := costs.FormatDate(d)
			dayEnd := costs.FormatDate(d.AddDate(0, 0, 1))
			resultsByTime = append(resultsByTime, map[string]any{
				"TimePeriod": map[string]string{"Start": dayStart, "End": dayEnd},
				"Total":      map[string]any{},
				"Groups":     groupList,
				"Estimated":  dayStart[:7]+"-01" == fixtures.CostsAnchorMonth,
			})
		}
	}

	out := map[string]any{
		"ResultsByTime":            resultsByTime,
		"DimensionValueAttributes": nil,
		"NextPageToken":            nil,
	}
	return jsonOKResponse(out)
}

// costsEmitResultsByTime converts byBucket (bucket key -> group key -> cell)
// into CE's ResultsByTime shape, sorted by bucket then group key.
// bucketPeriod resolves a bucket key into its (Start, End) TimePeriod — the
// only difference between the monthly ("YYYY-MM-01" bucket keys, End is the
// next month) and daily ("Start/End" bucket keys, already a full period)
// aggregations, shared by costsMonthlyResultsByTime and
// costsDailyResultsByTime.
func costsEmitResultsByTime(byBucket map[string]map[string]*ceGroupCell, bucketPeriod func(bucketKey string) (start, end string)) []map[string]any {
	bucketKeys := make([]string, 0, len(byBucket))
	for k := range byBucket {
		bucketKeys = append(bucketKeys, k)
	}
	sort.Strings(bucketKeys)

	resultsByTime := make([]map[string]any, 0, len(bucketKeys))
	for _, bucketKey := range bucketKeys {
		groups := byBucket[bucketKey]
		keys := make([]string, 0, len(groups))
		for k := range groups {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		groupList := make([]map[string]any, 0, len(keys))
		for _, k := range keys {
			cell := groups[k]
			groupList = append(groupList, map[string]any{
				"Keys":    cell.vals,
				"Metrics": costsMetricValues(cell.amount),
			})
		}

		start, end := bucketPeriod(bucketKey)
		resultsByTime = append(resultsByTime, map[string]any{
			"TimePeriod": map[string]string{"Start": start, "End": end},
			"Total":      map[string]any{},
			"Groups":     groupList,
			"Estimated":  start[:7]+"-01" == fixtures.CostsAnchorMonth,
		})
	}
	return resultsByTime
}

// costsMonthlyResultsByTime is the MONTHLY-granularity aggregation: one
// bucket per fixture row.Month, filtered against [start, end) at month
// precision.
func costsMonthlyResultsByTime(fx *fixtures.CostsFixtures, groupDims []string, eq, notEq map[string][]string, start, end string) []map[string]any {
	byMonth := map[string]map[string]*ceGroupCell{}
	for _, row := range fx.Rows {
		if !costsRowMatchesFilter(row, eq, notEq) {
			continue
		}
		if start != "" && row.Month < start {
			continue
		}
		if end != "" && row.Month >= end {
			continue
		}
		vals := make([]string, len(groupDims))
		for i, d := range groupDims {
			vals[i] = costsRowDimValue(row, d)
		}
		key := strings.Join(vals, "\x1f")
		month := byMonth[row.Month]
		if month == nil {
			month = map[string]*ceGroupCell{}
			byMonth[row.Month] = month
		}
		cell := month[key]
		if cell == nil {
			cell = &ceGroupCell{vals: vals}
			month[key] = cell
		}
		cell.amount += row.Amount
	}

	return costsEmitResultsByTime(byMonth, func(month string) (string, string) {
		return month, costsNextMonth(month)
	})
}

// costsDailyResultsByTime is the DAILY-granularity aggregation (FR-018's
// week/day zoom): every fixture row's monthly Amount is deterministically
// split across that row's calendar days (costsSplitAmountAcrossDays — equal
// share with the exact remainder folded into the last day, so a service's
// daily amounts always sum back to its monthly fixture amount, no
// randomness), then filtered against [start, end) at DAY precision — unlike
// costsMonthlyResultsByTime's row.Month string-compare, this correctly
// serves a request whose range starts and ends mid-month. The monthly
// fixture rows stay the single source of truth: no parallel daily dataset.
func costsDailyResultsByTime(fx *fixtures.CostsFixtures, groupDims []string, eq, notEq map[string][]string, start, end string) []map[string]any {
	byDay := map[string]map[string]*ceGroupCell{}
	for _, row := range fx.Rows {
		if !costsRowMatchesFilter(row, eq, notEq) {
			continue
		}
		monthStart, err := costs.ParseDate(row.Month)
		if err != nil {
			continue
		}
		days := daysInMonth(monthStart)
		shares := costsSplitAmountAcrossDays(row.Amount, days)

		vals := make([]string, len(groupDims))
		for i, d := range groupDims {
			vals[i] = costsRowDimValue(row, d)
		}
		key := strings.Join(vals, "\x1f")

		for i := range days {
			dayStart := monthStart.AddDate(0, 0, i)
			dayStartStr := costs.FormatDate(dayStart)
			if start != "" && dayStartStr < start {
				continue
			}
			if end != "" && dayStartStr >= end {
				continue
			}
			dayEndStr := costs.FormatDate(dayStart.AddDate(0, 0, 1))
			dayKey := dayStartStr + "/" + dayEndStr

			dayCells := byDay[dayKey]
			if dayCells == nil {
				dayCells = map[string]*ceGroupCell{}
				byDay[dayKey] = dayCells
			}
			cell := dayCells[key]
			if cell == nil {
				cell = &ceGroupCell{vals: vals}
				dayCells[key] = cell
			}
			cell.amount += shares[i]
		}
	}

	return costsEmitResultsByTime(byDay, func(dayKey string) (string, string) {
		s, e, _ := strings.Cut(dayKey, "/")
		return s, e
	})
}

// daysInMonth returns the number of calendar days in monthStart's month
// (monthStart is always the 1st, per the fixture's "YYYY-MM-01" convention).
func daysInMonth(monthStart time.Time) int {
	return int(monthStart.AddDate(0, 1, 0).Sub(monthStart).Hours() / 24)
}

// costsSplitAmountAcrossDays deterministically spreads amount across days
// day-buckets: an equal share per day, with the exact remainder folded into
// the last day so the shares sum EXACTLY back to amount (within float
// arithmetic — no rounding, no randomness).
func costsSplitAmountAcrossDays(amount float64, days int) []float64 {
	if days <= 0 {
		return nil
	}
	share := amount / float64(days)
	out := make([]float64, days)
	var sum float64
	for i := range days - 1 {
		out[i] = share
		sum += share
	}
	out[days-1] = amount - sum
	return out
}

// costsMetricValues maps one aggregated amount onto all four CE metrics the
// fetcher always requests (internal/aws/costs.go's ceMetrics) — the demo
// dataset has no separate blended/amortized modeling, so every metric
// mirrors the invoice (UnblendedCost) amount.
func costsMetricValues(amount float64) map[string]map[string]string {
	amt := strconv.FormatFloat(amount, 'f', 4, 64)
	v := map[string]string{"Amount": amt, "Unit": "USD"}
	return map[string]map[string]string{
		"UnblendedCost":    v,
		"BlendedCost":      v,
		"AmortizedCost":    v,
		"NetAmortizedCost": v,
	}
}

// costsDimensionValueAttributes returns the LINKED_ACCOUNT id->name
// attributes the demo dataset carries when grouped by account — both
// synthetic accounts (fixtures.CostsDemoAccountID/CostsDemoAccountIDAlt),
// not just one, so a LINKED_ACCOUNT-pivoted grid displays real account
// names for every row it renders.
func costsDimensionValueAttributes(groupDims []string) []map[string]any {
	if !slices.Contains(groupDims, "LINKED_ACCOUNT") {
		return nil
	}
	return []map[string]any{
		{
			"Value":      fixtures.CostsDemoAccountID,
			"Attributes": map[string]string{"description": fixtures.CostsDemoAccountName},
		},
		{
			"Value":      fixtures.CostsDemoAccountIDAlt,
			"Attributes": map[string]string{"description": fixtures.CostsDemoAccountNameAlt},
		},
	}
}

func costsGetAnomaliesResponse(fx *fixtures.CostsFixtures) (*http.Response, error) {
	a := fx.Anomaly
	end := costsNextMonth(a.Month)
	anomaly := map[string]any{
		"AnomalyId":        a.ID,
		"AnomalyStartDate": a.Month,
		"AnomalyEndDate":   end,
		"AnomalyScore":     map[string]any{"CurrentScore": a.Score, "MaxScore": a.Score},
		"Impact": map[string]any{
			"MaxImpact":             a.Impact,
			"TotalImpact":           a.Impact,
			"TotalImpactPercentage": 42.0,
		},
		"MonitorArn": "arn:aws:ce::" + fixtures.CostsDemoAccountID + ":anomalymonitor/demo-cost-monitor",
		"RootCauses": []map[string]any{{
			"Service":   a.Service,
			"Region":    fixtures.CostsDemoRegion,
			"UsageType": a.UsageType,
		}},
	}
	out := map[string]any{
		"Anomalies":     []map[string]any{anomaly},
		"NextPageToken": nil,
	}
	return jsonOKResponse(out)
}

func costsGetDimensionValuesResponse(fx *fixtures.CostsFixtures, req *http.Request) (*http.Response, error) {
	var in ceGetDimensionValuesInput
	if err := readJSONBody(req, &in); err != nil {
		return errorResponse(400, "SerializationException", err.Error()), nil
	}

	seen := map[string]struct{}{}
	var values []map[string]any
	for _, row := range fx.Rows {
		v := costsRowDimValue(row, in.Dimension)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		entry := map[string]any{"Value": v}
		if in.Dimension == "LINKED_ACCOUNT" {
			entry["Attributes"] = map[string]string{"description": fixtures.CostsAccountName(v)}
		}
		values = append(values, entry)
	}
	sort.Slice(values, func(i, j int) bool {
		return values[i]["Value"].(string) < values[j]["Value"].(string)
	})

	out := map[string]any{
		"DimensionValues": values,
		"ReturnSize":      len(values),
		"TotalSize":       len(values),
		"NextPageToken":   nil,
	}
	return jsonOKResponse(out)
}

// costsNextMonth returns the first day of the month after month
// ("YYYY-MM-01"), matching CE's exclusive-end period convention.
func costsNextMonth(month string) string {
	t, err := costs.ParseDate(month)
	if err != nil {
		return month
	}
	return costs.FormatDate(t.AddDate(0, 1, 0))
}

func readJSONBody(req *http.Request, v any) error {
	if req.Body == nil || req.Body == http.NoBody {
		return nil
	}
	data, err := io.ReadAll(req.Body)
	if err != nil {
		return err
	}
	req.Body = io.NopCloser(bytes.NewReader(data))
	if len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, v)
}

func jsonOKResponse(v any) (*http.Response, error) {
	body, err := json.Marshal(v)
	if err != nil {
		return errorResponse(500, "InternalError", err.Error()), nil
	}
	return &http.Response{
		StatusCode: 200,
		Status:     "200 OK",
		Header:     http.Header{"Content-Type": []string{"application/x-amz-json-1.1"}},
		Body:       io.NopCloser(bytes.NewReader(body)),
	}, nil
}
