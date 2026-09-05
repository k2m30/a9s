// costs_fixture_anchor_test.go — the demo costs dataset has exactly one
// clock, and every test that reads that dataset must read the same one.
//
// core/demo/fixtures/costs.go anchors its 13-month synthetic window to a
// month resolved from the wall clock, so a test that hard-codes a month
// label ("Jan'26"), a window bound, or the planted anomaly's coordinate is
// a SECOND source of truth for "which months exist". The two agree only in
// the month the literal was written in and silently disagree every month
// after — the drift that took three costs tests red without a single line
// of production code changing.
//
// The helpers below are the only sanctioned way for a test to name a month
// of that dataset: they derive it from the fixture's own exported anchor.
// The table test at the bottom pins the property that makes the derivation
// safe — the generator's whole month window, and the planted anomaly inside
// it, move together with the anchor, at any anchor.

package unit

import (
	"testing"
	"time"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/costs"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
)

// costsFixtureMonthTime parses a fixture month string ("YYYY-MM-01") into
// the UTC instant of its first day.
func costsFixtureMonthTime(t *testing.T, month string) time.Time {
	t.Helper()
	parsed, err := costs.ParseDate(month)
	if err != nil {
		t.Fatalf("costs.ParseDate(%q): %v — the demo costs fixture no longer exposes months as YYYY-MM-01", month, err)
	}
	return parsed
}

// costsFixtureAnchorNow is a wall-clock instant inside the demo costs
// fixture's own anchor month (its newest, still-open month). A costs test
// that drives the demo transport must pass THIS as its `now`, never a date
// literal: the trailing month window a controller builds from it is then
// guaranteed to end at the anchor and therefore to contain every month the
// fixture planted a story in.
func costsFixtureAnchorNow(t *testing.T) time.Time {
	t.Helper()
	return costsFixtureNowInMonth(t, fixtures.CostsAnchorMonth)
}

// costsFixtureNowInMonth returns an instant safely inside the given fixture
// month — mid-month, so it is never the first or last day and never lands
// outside the month on any calendar.
func costsFixtureNowInMonth(t *testing.T, month string) time.Time {
	t.Helper()
	return costsFixtureMonthTime(t, month).AddDate(0, 0, 14)
}

// TestCostsFixtureAnchorNow_WindowAlwaysCoversTheGrowthStory pins the one
// property every converted costs test leans on: a MONTH window built from
// the fixture's anchor instant always ends at the anchor month AND contains
// the planted growth/anomaly month. That is what makes "find the column
// labelled after CostsGrowthMonth" a safe assertion instead of a literal.
// Checked across a year rollover and a short month, because the helper
// picks a day inside the anchor month and a day arithmetic that slipped
// into a neighbouring month would silently shift the whole window by one.
func TestCostsFixtureAnchorNow_WindowAlwaysCoversTheGrowthStory(t *testing.T) {
	anchors := []time.Time{
		time.Date(2026, time.January, 15, 12, 0, 0, 0, time.UTC),
		time.Date(2027, time.February, 15, 12, 0, 0, 0, time.UTC), // pinned + 13 months
		time.Date(2026, time.December, 31, 23, 0, 0, 0, time.UTC), // year rollover
		time.Date(2028, time.February, 29, 6, 0, 0, 0, time.UTC),  // leap-day, short month
	}

	for _, anchor := range anchors {
		t.Run(anchor.Format("2006-01-02"), func(t *testing.T) {
			anchorMonth := fixtures.CostsAnchorMonthAt(anchor)
			growthMonth := fixtures.CostsGrowthMonthAt(anchor)

			now := costsFixtureNowInMonth(t, anchorMonth)
			if got := fixtures.CostsAnchorMonthAt(now); got != anchorMonth {
				t.Fatalf("the anchor instant fell outside its own month: anchor month %q, but the instant resolves to %q", anchorMonth, got)
			}

			window := costs.BuildWindow(costs.GranularityMonth, now)
			if len(window) == 0 {
				t.Fatal("BuildWindow(MONTH) returned no periods for the anchor instant")
			}
			if got := window[len(window)-1].Start; got != anchorMonth {
				t.Errorf("newest window period starts %q, want the anchor month %q", got, anchorMonth)
			}
			var covered bool
			for _, p := range window {
				if p.Start == growthMonth {
					covered = true
					break
				}
			}
			if !covered {
				t.Errorf("the fixture's growth/anomaly month %q is not in the window built from its own anchor %q: %v — a test looking for that column would find nothing", growthMonth, anchorMonth, window)
			}
		})
	}
}

// costsFixtureMonthLabel renders a fixture month through the SAME formatter
// the grid's own column headers go through, so a test can find that column
// by label without spelling one out and without restating the format
// string — a restated format is the second truth source one layer down.
func costsFixtureMonthLabel(t *testing.T, month string) string {
	t.Helper()
	start := costsFixtureMonthTime(t, month)
	return app.PeriodLabel(costs.Period{
		Start: costs.FormatDate(start),
		End:   costs.FormatDate(start.AddDate(0, 1, 0)),
	}, costs.GranularityMonth)
}

// TestCostsFixtures_AnchorDrivesTheWholeDataset pins that the synthetic
// costs dataset is a pure function of its anchor: the 13-month window ends
// at the anchor month, the planted growth story sits at a FIXED OFFSET
// inside that window (index 6), and the single planted anomaly names that
// same month and the same service/usage type the growth ramp was built
// from. Run at a pinned anchor and at anchor+13 months — the second anchor
// shares no month with the first, so any month that stayed put between the
// two runs (a constant baked into the generator instead of derived from the
// anchor) shows up as a mismatch here rather than as a costs test that
// mysteriously goes red next quarter.
//
// The window length (13) and the growth offset (6) are spelled out rather
// than read from fixtures.CostsWindowMonths / CostsGrowthMonthIndex on
// purpose: asserting a value against the constant that produced it proves
// only that the constant equals itself. These are the numbers the dataset
// is specified to have, so a change to either constant must fail here and
// be re-decided, not silently followed.
func TestCostsFixtures_AnchorDrivesTheWholeDataset(t *testing.T) {
	pinned := time.Date(2026, time.January, 15, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name            string
		now             time.Time
		wantAnchorMonth string
		wantGrowthMonth string
	}{
		{
			name:            "pinned anchor",
			now:             pinned,
			wantAnchorMonth: "2026-01-01",
			wantGrowthMonth: "2025-07-01",
		},
		{
			// Thirteen months on: a full window later, so the two runs
			// share not one month. Nothing about the dataset may be frozen.
			name:            "anchor plus 13 months",
			now:             pinned.AddDate(0, 13, 0),
			wantAnchorMonth: "2027-02-01",
			wantGrowthMonth: "2026-08-01",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := fixtures.NewCostsFixturesAt(tc.now)

			if len(f.Months) != 13 {
				t.Fatalf("Months length got %d want 13 — the dataset is a 13-month trailing window", len(f.Months))
			}
			if got := f.Months[12]; got != tc.wantAnchorMonth {
				t.Errorf("newest month got %q want %q — the window must END at the anchor's own month", got, tc.wantAnchorMonth)
			}
			for i := 1; i < len(f.Months); i++ {
				prev := costsFixtureMonthTime(t, f.Months[i-1])
				want := costs.FormatDate(prev.AddDate(0, 1, 0))
				if f.Months[i] != want {
					t.Fatalf("Months[%d] got %q want %q — the window must be 13 CONSECUTIVE months", i, f.Months[i], want)
				}
			}
			if got := f.Months[6]; got != tc.wantGrowthMonth {
				t.Errorf("growth-story month (Months[6]) got %q want %q", got, tc.wantGrowthMonth)
			}

			if f.Anomaly.Month != tc.wantGrowthMonth {
				t.Errorf("Anomaly.Month got %q want %q — the planted anomaly must sit on the growth story's own month, or the anomaly overlay marks a cell the ramp never happened in", f.Anomaly.Month, tc.wantGrowthMonth)
			}
			if f.Anomaly.Service != fixtures.CostsGrowthService {
				t.Errorf("Anomaly.Service got %q want %q", f.Anomaly.Service, fixtures.CostsGrowthService)
			}
			if f.Anomaly.UsageType != fixtures.CostsGrowthUsageType {
				t.Errorf("Anomaly.UsageType got %q want %q", f.Anomaly.UsageType, fixtures.CostsGrowthUsageType)
			}
			if f.Anomaly.Impact <= 0 {
				t.Errorf("Anomaly.Impact got %v want a positive step — the story is a cost INCREASE at the growth month", f.Anomaly.Impact)
			}

			// Every fact must land in the declared window. A row carrying a
			// month outside Months is a column the grid can never render.
			inWindow := make(map[string]bool, len(f.Months))
			for _, m := range f.Months {
				inWindow[m] = true
			}
			if len(f.Rows) == 0 {
				t.Fatal("Rows is empty — the generator produced no facts at this anchor")
			}
			for _, r := range f.Rows {
				if !inWindow[r.Month] {
					t.Fatalf("row %+v carries month %q, which is outside the dataset's own 13-month window %v", r, r.Month, f.Months)
				}
			}

			// The growth story must actually be a step UP at the growth
			// month, not merely labelled as one: the anomaly's Impact is
			// derived from that step, so a flat ramp would publish an
			// anomaly nothing in the grid supports.
			growth := costsFixtureSumFor(f, tc.wantGrowthMonth, fixtures.CostsGrowthService, fixtures.CostsGrowthUsageType)
			prevMonth := costs.FormatDate(costsFixtureMonthTime(t, tc.wantGrowthMonth).AddDate(0, -1, 0))
			before := costsFixtureSumFor(f, prevMonth, fixtures.CostsGrowthService, fixtures.CostsGrowthUsageType)
			if growth <= before {
				t.Errorf("growth story at %s got %v, month before (%s) got %v — want a step UP", tc.wantGrowthMonth, growth, prevMonth, before)
			}
		})
	}
}

// costsFixtureSumFor totals every account's share of one (month, service,
// usage type) fact — the dataset splits each amount across two synthetic
// linked accounts, so a single row never carries the whole figure.
func costsFixtureSumFor(f *fixtures.CostsFixtures, month, service, usageType string) float64 {
	var total float64
	for _, r := range f.Rows {
		if r.Month == month && r.Service == service && r.UsageType == usageType {
			total += r.Amount
		}
	}
	return total
}
