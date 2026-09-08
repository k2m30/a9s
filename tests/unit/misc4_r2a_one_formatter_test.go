// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package unit_test

// misc4_r2a_one_formatter_test.go — misc4 round 2, item (a).
//
// A list cell is filled from one of two lanes: the Fields map the fetcher
// wrote, or the RawStruct path fieldpath renders. The two rendered the same
// value differently — a bool was "Yes" down one lane and "true" down the
// other, a timestamp was "2026-04-20 00:00" or "2026-04-20T00:00:00Z" — so
// which one an operator saw depended on whether the row came from a live
// struct or a warm cache, and on a column-count comparison in the cascade.
//
// Both lanes now return through one formatter, so the lane a value took
// changes where it came from and never what it says.

import (
	"testing"
	"time"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/resource"
)

type misc4Raw struct {
	Flag    bool
	Stamp   time.Time
	Version string
}

// misc4BothLanes renders one fact down each lane and returns (fields, raw).
func misc4BothLanes(t *testing.T, key, path, fieldValue string, raw any) (string, string) {
	t.Helper()
	fromFields := app.ExtractCellValue(
		app.ColumnDef{Key: key, Title: "Cell", Width: 20},
		nil,
		resource.Resource{ID: "r1", Fields: map[string]string{key: fieldValue}},
	)
	fromRaw := app.ExtractCellValue(
		app.ColumnDef{Path: path, Title: "Cell", Width: 20},
		nil,
		resource.Resource{ID: "r1", RawStruct: raw},
	)
	return fromFields, fromRaw
}

// TestBothLanesRenderABoolTheSameWay pins the bool word.
func TestBothLanesRenderABoolTheSameWay(t *testing.T) {
	for _, tc := range []struct {
		field string
		raw   bool
		want  string
	}{
		{"true", true, "Yes"},
		{"false", false, "No"},
	} {
		fields, rawLane := misc4BothLanes(t, "flag", "Flag", tc.field, misc4Raw{Flag: tc.raw})
		if fields != rawLane {
			t.Errorf("bool %v: Fields lane %q, RawStruct lane %q", tc.raw, fields, rawLane)
		}
		if fields != tc.want {
			t.Errorf("bool %v rendered %q, want %q", tc.raw, fields, tc.want)
		}
	}
}

// TestBothLanesRenderATimestampTheSameWay pins the timestamp shape, for the
// two spellings fetchers actually write into Fields.
func TestBothLanesRenderATimestampTheSameWay(t *testing.T) {
	stamp := time.Date(2026, 4, 20, 9, 30, 0, 0, time.UTC)
	for _, field := range []string{"2026-04-20T09:30:00Z", "2026-04-20 09:30"} {
		fields, rawLane := misc4BothLanes(t, "stamp", "Stamp", field, misc4Raw{Stamp: stamp})
		if fields != rawLane {
			t.Errorf("timestamp %q: Fields lane %q, RawStruct lane %q", field, fields, rawLane)
		}
		if fields != "2026-04-20 09:30" {
			t.Errorf("timestamp %q rendered %q, want \"2026-04-20 09:30\"", field, fields)
		}
	}
	// A date with no time of day is still a date; the shape says so.
	fields, _ := misc4BothLanes(t, "stamp", "Stamp", "2026-04-20", misc4Raw{Stamp: stamp})
	if fields != "2026-04-20 00:00" {
		t.Errorf("date-only rendered %q, want \"2026-04-20 00:00\"", fields)
	}
}

// TestTheFormatterLeavesNumbersAlone pins the deliberate limit: a decimal is
// an engine version as often as it is a number, and 1.10 is not 1.1.
func TestTheFormatterLeavesNumbersAlone(t *testing.T) {
	for _, v := range []string{"1.10", "5.00", "8.0.35", "0.0.0.0/0"} {
		got := app.ExtractCellValue(
			app.ColumnDef{Key: "version", Title: "Cell", Width: 20},
			nil,
			resource.Resource{ID: "r1", Fields: map[string]string{"version": v}},
		)
		if got != v {
			t.Errorf("%q was reformatted to %q; numbers are left alone", v, got)
		}
	}
}
