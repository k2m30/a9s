package unit_test

import (
	"testing"
	"time"

	"github.com/k2m30/a9s/v3/core/costs"
)

func TestGranularity_APIGranularity(t *testing.T) {
	tests := []struct {
		g    costs.Granularity
		want string
	}{
		{costs.Granularity("year"), "MONTHLY"},
		{costs.Granularity("month"), "MONTHLY"},
		{costs.Granularity("week"), "DAILY"},
		{costs.Granularity("day"), "DAILY"},
	}
	for _, tt := range tests {
		t.Run(string(tt.g), func(t *testing.T) {
			if got := tt.g.APIGranularity(); got != tt.want {
				t.Errorf("Granularity(%q).APIGranularity() = %q, want %q", tt.g, got, tt.want)
			}
		})
	}
}

func TestPeriod_Closed(t *testing.T) {
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		p    costs.Period
		want bool
	}{
		{
			name: "full prior month is closed",
			p:    costs.Period{Start: "2026-06-01", End: "2026-07-01"},
			want: true,
		},
		{
			name: "boundary: period ending exactly on the 1st of the current month is closed",
			p:    costs.Period{Start: "2026-06-24", End: "2026-07-01"},
			want: true,
		},
		{
			name: "current open month is not closed",
			p:    costs.Period{Start: "2026-07-01", End: "2026-08-01"},
			want: false,
		},
		{
			name: "day just inside the current month is not closed",
			p:    costs.Period{Start: "2026-07-01", End: "2026-07-02"},
			want: false,
		},
		{
			name: "future month is not closed",
			p:    costs.Period{Start: "2026-08-01", End: "2026-09-01"},
			want: false,
		},
		{
			name: "distant closed year is closed",
			p:    costs.Period{Start: "2025-01-01", End: "2025-02-01"},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.p.Closed(now); got != tt.want {
				t.Errorf("Period{%q,%q}.Closed(%v) = %v, want %v", tt.p.Start, tt.p.End, now, got, tt.want)
			}
		})
	}
}

// TestQuery_Validate died with Query.Validate() itself (closure-wave
// deletion) — the two-GroupBy-dim-max constraint it pinned is no longer
// enforced at that seam.

func TestQuery_CacheKey_DeterministicAcrossMapIterationOrder(t *testing.T) {
	q := costs.Query{
		Granularity: "DAILY",
		GroupBy:     []costs.Dimension{costs.Dimension("SERVICE"), costs.Dimension("USAGE_TYPE")},
		Filter: costs.Filter{
			Equals: map[costs.Dimension][]string{
				costs.Dimension("SERVICE"):        {"Amazon Elastic Compute Cloud - Compute", "Amazon Simple Storage Service"},
				costs.Dimension("REGION"):         {"us-east-1", "eu-west-1"},
				costs.Dimension("LINKED_ACCOUNT"): {"123456789012"},
				costs.Dimension("USAGE_TYPE"):     {"BoxUsage:m5.large", "USE1-NatGateway-Bytes"},
			},
		},
		Range: costs.Period{Start: "2026-06-01", End: "2026-07-01"},
	}

	keys := make(map[string]struct{})
	for i := 0; i < 30; i++ {
		keys[q.CacheKey()] = struct{}{}
	}
	if len(keys) != 1 {
		t.Errorf("CacheKey() produced %d distinct values across 30 calls on the same Query (map iteration order must not leak into the key): %v", len(keys), keys)
	}
}

func TestQuery_CacheKey_VariesByShapeNotByRange(t *testing.T) {
	base := costs.Query{
		Granularity: "MONTHLY",
		GroupBy:     []costs.Dimension{costs.Dimension("SERVICE")},
		Filter: costs.Filter{
			Equals: map[costs.Dimension][]string{
				costs.Dimension("RECORD_TYPE"): {"Tax", "Credit", "Refund"},
			},
		},
		Range: costs.Period{Start: "2026-01-01", End: "2027-01-01"},
	}
	baseKey := base.CacheKey()

	granularityChanged := base
	granularityChanged.Granularity = "DAILY"
	if k := granularityChanged.CacheKey(); k == baseKey {
		t.Errorf("CacheKey() unchanged after Granularity changed: %q", k)
	}

	dimsChanged := base
	dimsChanged.GroupBy = []costs.Dimension{costs.Dimension("SERVICE"), costs.Dimension("REGION")}
	if k := dimsChanged.CacheKey(); k == baseKey {
		t.Errorf("CacheKey() unchanged after GroupBy changed: %q", k)
	}

	filterChanged := base
	filterChanged.Filter = costs.Filter{
		Equals: map[costs.Dimension][]string{
			costs.Dimension("RECORD_TYPE"): {"Tax"},
		},
	}
	if k := filterChanged.CacheKey(); k == baseKey {
		t.Errorf("CacheKey() unchanged after Filter changed: %q", k)
	}

	// Range only affects which periods a cached CacheKey covers, not the
	// key itself (data-model.md: "period range is stored per-record").
	rangeChanged := base
	rangeChanged.Range = costs.Period{Start: "2020-01-01", End: "2021-01-01"}
	if k := rangeChanged.CacheKey(); k != baseKey {
		t.Errorf("CacheKey() changed after Range-only change: got %q, want unchanged %q", k, baseKey)
	}
}
