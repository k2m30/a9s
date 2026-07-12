package costs

import "time"

// defaultYearColumns and defaultMonthColumns are the trailing column counts
// BuildWindow uses at year/month granularity — a fixed-count window ending
// at anchor's own bucket (the open period), matching wireframe.md's 12-month
// default view (Aug'25-Jul'26).
const (
	defaultYearColumns  = 5
	defaultMonthColumns = 12
)

// HistoryHorizonMonths bounds how far back Cost Explorer's default cost
// entitlement retains data: the open month plus 12 closed (CE rejects a
// Range.Start beyond it with ValidationException "You haven't enabled
// historical data beyond 14 months."). The single source for that horizon —
// both BuildWindow's own Range.Start clamp (year's fixed 5-column window
// would otherwise request years far older than the entitlement) and
// month-granularity scroll-to-load's extend-older cap use this constant,
// never a locally duplicated 13.
const HistoryHorizonMonths = 13

// BuildWindow returns the column boundaries at granularity g anchored on
// anchor. Year/month use a fixed-count trailing window ending at anchor's
// own bucket, clamped so its earliest period never starts before
// HistoryHorizonMonths' entitlement horizon (clampWindowStartToHorizon).
// Week/day are bounded drill-zoom windows — every finer bucket inside
// anchor's enclosing month (week) or enclosing ISO week (day) — per
// wireframe.md's zoom example ("weeks of that month"); always well within
// the horizon, so they are never clamped. Pure: anchor is always injected,
// this package never calls time.Now().
func BuildWindow(g Granularity, anchor time.Time) []Period {
	return TrailingWindow(g, anchor, anchor)
}

// TrailingWindow returns the column boundaries at granularity g, a
// fixed-count trailing window ENDING at anchor's own bucket (year/month) or
// bounded to anchor's enclosing month/week (week/day) — identical bucket
// generation to BuildWindow — but clamps the result (history horizon, and
// the open bucket's own End ceiling) against now, not anchor: a zoom-out
// re-anchored on an older cursor period must still clamp against the REAL
// now, never against the (possibly older) anchor it re-centers on. Pure:
// anchor and now are always injected, this package never calls
// time.Now().
func TrailingWindow(g Granularity, anchor, now time.Time) []Period {
	anchor = anchor.UTC()
	nowUTC := now.UTC()
	switch g {
	case GranularityYear:
		return clampWindowStartToHorizon(yearWindow(anchor, defaultYearColumns, nowUTC), nowUTC)
	case GranularityWeek:
		return weekWindowsInMonth(anchor)
	case GranularityDay:
		return dayWindowsInWeek(anchor)
	default: // GranularityMonth and any unrecognized value
		return clampWindowStartToHorizon(monthWindow(anchor, defaultMonthColumns), nowUTC)
	}
}

// WindowWithin returns the finer-granularity g periods tiling sel — the
// generic "drill into this cell" window constructor (year->month,
// month->week, week->day), replacing the ambiguous BuildWindow(newGran,
// selectedPeriod.Start) callers used to reach for: BuildWindow's anchor
// doubles as both the window's end AND its shape's own natural span, which
// is correct only when g is TrailingAnchored (year/month) and sel IS the
// canonical trailing window — never when sel is an arbitrary drilled-into
// period (e.g. a past year cell), where BuildWindow(Month, sel.Start)
// silently returns a trailing window ending at sel's own January instead of
// that year's own Jan-Dec. WindowWithin always tiles strictly within sel's
// own [Start, End) bounds, clamped to the CE history horizon and to now's
// own first-of-next-month ceiling. Pure: sel and now are always injected.
func WindowWithin(sel Period, g Granularity, now time.Time) []Period {
	switch g {
	case GranularityMonth:
		return monthsWithinPeriod(sel, now)
	case GranularityWeek:
		return weeksWithinPeriod(sel)
	case GranularityDay:
		return daysWithinPeriod(sel)
	default:
		return nil
	}
}

// monthsWithinPeriod tiles sel (any period, not necessarily calendar-year
// aligned) into calendar months lying inside it, dropping months that have
// not happened yet (past now's own month) and clamping the earliest month
// to the CE history horizon — the year->month drill case (X4a), and any
// other month-tiling of an arbitrary parent period.
func monthsWithinPeriod(sel Period, now time.Time) []Period {
	start, errS := ParseDate(sel.Start)
	end, errE := ParseDate(sel.End)
	if errS != nil || errE != nil {
		return nil
	}
	nowUTC := now.UTC()
	if cutoff := firstOfNextMonth(nowUTC); cutoff.Before(end) {
		end = cutoff
	}
	var out []Period
	for m := start; m.Before(end); m = m.AddDate(0, 1, 0) {
		out = append(out, Period{Start: FormatDate(m), End: FormatDate(m.AddDate(0, 1, 0))})
	}
	return clampWindowStartToHorizon(out, nowUTC)
}

// weeksWithinPeriod tiles sel's own calendar month into ISO weeks, clipped
// to the month boundary — sel is expected to be a month-shaped period (the
// month->week drill), so its own Start already identifies the month.
func weeksWithinPeriod(sel Period) []Period {
	start, err := ParseDate(sel.Start)
	if err != nil {
		return nil
	}
	return weekWindowsInMonth(start)
}

// daysWithinPeriod tiles sel's own ISO week into days, clipped to the
// enclosing month — sel is expected to be a week-shaped period (the
// week->day drill), so its own Start already identifies the week. A sel
// that is ALREADY day-shaped (End is exactly Start+1 day, e.g. a
// RESOURCE_ID frame's own single-day cell) tiles to exactly itself instead
// — tiling its enclosing week would silently widen a one-day selection to
// seven.
func daysWithinPeriod(sel Period) []Period {
	start, errS := ParseDate(sel.Start)
	end, errE := ParseDate(sel.End)
	if errS != nil || errE != nil {
		return nil
	}
	if end.Equal(start.AddDate(0, 0, 1)) {
		return []Period{sel}
	}
	return dayWindowsInWeek(start)
}

// clampWindowStartToHorizon drops every period in window that ends at or
// before the HistoryHorizonMonths cutoff (anchor's own month-start, minus
// HistoryHorizonMonths-1 months — no overlap with the entitlement at all),
// and clips the earliest surviving period's Start forward to the cutoff
// when it straddles it, so the caller still gets the full contiguous span
// CE actually covers rather than a hard refusal.
func clampWindowStartToHorizon(window []Period, anchor time.Time) []Period {
	cutoff := time.Date(anchor.Year(), anchor.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -(HistoryHorizonMonths - 1), 0)
	out := make([]Period, 0, len(window))
	for _, p := range window {
		end, err := ParseDate(p.End)
		if err != nil || !end.After(cutoff) {
			continue
		}
		start, err := ParseDate(p.Start)
		if err == nil && start.Before(cutoff) {
			p.Start = FormatDate(cutoff)
		}
		out = append(out, p)
	}
	return out
}

// TrailingAnchored reports whether g's window is always the canonical
// trailing-window-ending-at-now (year, month — the two granularities
// FR-002's default view is defined at) rather than a window bounded to one
// specific parent bucket (week, day — reached only by zooming into a
// specific month/week). Zoom callers use this to decide whether a
// granularity change re-anchors on "now" or on the cursor's current
// period: re-anchoring month/year on the cursor (rather than now) is what
// lets a zoom-out walk the window before any period the session has ever
// fetched.
func (g Granularity) TrailingAnchored() bool {
	return g == GranularityYear || g == GranularityMonth
}

// yearWindow builds n trailing calendar-year periods ending at anchor's own
// year. The last (open, current) year period's calendar-year End would run
// through next December — CE rejects a Range.End past the first day of the
// month after capAt's own month, so the open year is capped there too,
// exactly like the open month's own bucket boundary. capAt is anchor for
// BuildWindow's own (unchanged) semantics, or the real now for
// TrailingWindow — the window's shape is always anchor's, only the open
// bucket's own ceiling is capAt's.
func yearWindow(anchor time.Time, n int, capAt time.Time) []Period {
	endYear := anchor.Year()
	out := make([]Period, n)
	for i := range n {
		y := endYear - n + 1 + i
		start := time.Date(y, 1, 1, 0, 0, 0, 0, time.UTC)
		end := time.Date(y+1, 1, 1, 0, 0, 0, 0, time.UTC)
		out[i] = Period{Start: FormatDate(start), End: FormatDate(end)}
	}
	if cutoff := firstOfNextMonth(capAt); cutoff.Before(parseDateOrZero(out[n-1].End)) {
		out[n-1].End = FormatDate(cutoff)
	}
	return out
}

// firstOfNextMonth returns the first day of the month after t's own month —
// CE's hard Range.End ceiling ("end date past the beginning of next month
// is rejected"), the single clamp point every open-bucket boundary in this
// file is measured against.
func firstOfNextMonth(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, 1, 0)
}

func monthWindow(anchor time.Time, n int) []Period {
	endMonth := time.Date(anchor.Year(), anchor.Month(), 1, 0, 0, 0, 0, time.UTC)
	out := make([]Period, n)
	for i := range n {
		start := endMonth.AddDate(0, -(n - 1 - i), 0)
		end := start.AddDate(0, 1, 0)
		out[i] = Period{Start: FormatDate(start), End: FormatDate(end)}
	}
	return out
}

// weekWindowsInMonth returns one Period per ISO week (Mon-Sun) touching the
// calendar month containing anchor, with the first/last week clipped to the
// month boundary (matching wireframe.md's "Jan 1-4" / "Jan 26-31" example).
func weekWindowsInMonth(anchor time.Time) []Period {
	monthStart := time.Date(anchor.Year(), anchor.Month(), 1, 0, 0, 0, 0, time.UTC)
	monthEnd := monthStart.AddDate(0, 1, 0)

	var out []Period
	cur := mondayOnOrBefore(monthStart)
	for cur.Before(monthEnd) {
		weekEnd := cur.AddDate(0, 0, 7)
		s, e := cur, weekEnd
		if s.Before(monthStart) {
			s = monthStart
		}
		if e.After(monthEnd) {
			e = monthEnd
		}
		out = append(out, Period{Start: FormatDate(s), End: FormatDate(e)})
		cur = weekEnd
	}
	return out
}

// dayWindowsInWeek returns one Period per day in the ISO week (Mon-Sun)
// containing anchor, clipped to the calendar month containing anchor —
// mirroring weekWindowsInMonth's own clip, so a week whose Monday falls in
// the prior month (e.g. drilling April 1, a Wednesday, whose ISO week
// starts March 30) never spills before the month it was drilled into, and a
// week whose Sunday falls in the next month never spills past it (the same
// boundary CE's Range.End ceiling enforces at anchor's own month).
func dayWindowsInWeek(anchor time.Time) []Period {
	monthStart := time.Date(anchor.Year(), anchor.Month(), 1, 0, 0, 0, 0, time.UTC)
	monthEnd := monthStart.AddDate(0, 1, 0)
	weekStart := mondayOnOrBefore(time.Date(anchor.Year(), anchor.Month(), anchor.Day(), 0, 0, 0, 0, time.UTC))

	var out []Period
	for i := range 7 {
		s := weekStart.AddDate(0, 0, i)
		e := s.AddDate(0, 0, 1)
		if s.Before(monthStart) || e.After(monthEnd) {
			continue
		}
		out = append(out, Period{Start: FormatDate(s), End: FormatDate(e)})
	}
	return out
}

// NativeCoveragePeriods tiles [window[0].Start, window[last].End) into the
// native fetch-granularity buckets (DAILY: one per day, MONTHLY: one per
// month) apiGranularity's Query actually returns records under — the exact
// buckets Store.Merge keys records by. Callers stamp MergeCoverage against
// these, never against display-level buckets (week/year): an empty
// exact-match entry at a coarser bucket's own key would shadow the real
// native records Store.Lookup's lookupContained would otherwise aggregate.
func NativeCoveragePeriods(apiGranularity string, window []Period) []Period {
	if len(window) == 0 {
		return nil
	}
	start, err := ParseDate(window[0].Start)
	if err != nil {
		return nil
	}
	end, err := ParseDate(window[len(window)-1].End)
	if err != nil {
		return nil
	}

	var out []Period
	if apiGranularity == "DAILY" {
		for d := start; d.Before(end); d = d.AddDate(0, 0, 1) {
			out = append(out, Period{Start: FormatDate(d), End: FormatDate(d.AddDate(0, 0, 1))})
		}
		return out
	}
	for m := start; m.Before(end); m = m.AddDate(0, 1, 0) {
		out = append(out, Period{Start: FormatDate(m), End: FormatDate(m.AddDate(0, 1, 0))})
	}
	return out
}

func mondayOnOrBefore(t time.Time) time.Time {
	// time.Weekday: Sunday=0..Saturday=6; ISO offset from Monday=0..Sunday=6.
	offset := (int(t.Weekday()) + 6) % 7
	return t.AddDate(0, 0, -offset)
}
