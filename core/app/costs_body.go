package app

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/k2m30/a9s/v3/core/costs"
	"github.com/k2m30/a9s/v3/core/costs/screen"
)

// apiCostPerCall is the CE API pricing used to compute the session cost
// estimate shown in the footer (FR-013: calls x $0.01).
const apiCostPerCall = 0.01

// costsDeltaNeutralThreshold is the |delta| fraction below which a cell's
// period-over-period change is rendered neutral rather than growth/drop
// (wireframe.md: "|delta| < threshold neutral").
const costsDeltaNeutralThreshold = 0.05

// costsDeltaStrongThreshold is the |delta| fraction at and above which a
// non-neutral cell is tagged "-strong" rather than "-soft" (wireframe.md:
// "growth red shades, drop green shades").
const costsDeltaStrongThreshold = 0.25

// buildCostsBody assembles a CostsBody from cs, pre-resolving every display
// string and color tag the renderer needs — RenderCosts consumes it
// verbatim, never recomputing (same contract as ListRow.Color). Grid
// aggregation is costs.BuildGrid's (via costsGridForFrame); the displayed
// row set, clamped cursor, and visible column window all come from
// screen.BuildViewModel (via costsViewModelForFrame, architecture.md Seam
// 8) — this function only formats what the ViewModel already resolved.
func buildCostsBody(cs *CostsState) *CostsBody {
	if cs.ErrorMsg != "" {
		body := &CostsBody{
			ErrorMsg:   cs.ErrorMsg,
			Metric:     string(cs.Metric),
			Loading:    cs.Loading,
			APICalls:   cs.APICalls,
			APICostUSD: formatAPICost(cs.APICalls),
		}
		// The frame title reads Pivot/Granularity even while erroring — an
		// error must not blank out what's being viewed.
		if len(cs.DrillStack) > 0 {
			top := cs.DrillStack[len(cs.DrillStack)-1]
			body.Pivot = string(top.RowDim)
			body.Granularity = string(top.Granularity)
			body.DataThrough = cs.Store.DataThrough(costsQueryForFrame(top, cs.Metric), cs.Now)
		}
		return body
	}

	top := cs.DrillStack[len(cs.DrillStack)-1]
	grid, vm := costsViewModelForFrame(cs, top)

	// D4/D5: vm.VisibleCols is already the renderer-reported-viewport
	// slice, anchored at top.ScrollX (reconciled against the cursor by
	// applyCostsMoveCol/reconcileCostsScrollToCursor) — ViewportCols<=0
	// (renderer hasn't reported a width yet, or every column already fits)
	// falls back to the unsliced full window inside BuildViewModel itself.
	columns := make([]CostColumn, len(vm.VisibleCols))
	for i, p := range vm.VisibleCols {
		// Open marks the current period; RenderCosts appends the "*" suffix
		// (wireframe.md) — Label stays the bare period text so a renderer
		// never needs to strip a baked-in marker back out.
		columns[i] = CostColumn{Label: periodLabel(p, top.Granularity), Open: !p.Closed(cs.Now)}
	}

	rows := make([]CostRow, len(vm.Rows))
	for ri, gr := range vm.Rows {
		cells := make([]CostCell, len(gr.Cells))
		for i, cv := range gr.Cells {
			cells[i] = buildCostCell(cv)
		}
		// costsGridForFrame already strips the vendor prefix off SERVICE
		// row labels — gr.Label is display-ready.
		rows[ri] = CostRow{Label: gr.Label, Cells: cells}
	}

	// grid.Totals is not part of ViewModel (Seam 8 names Rows/Cursor/
	// VisibleCols/Note only) — sliced to the identical visible window
	// costsVisibleColumnRange computes the same way BuildViewModel did
	// internally for VisibleCols.
	visibleStart, _ := costsVisibleColumnRange(cs, top, len(grid.Columns))
	totals := make([]CostCell, len(vm.VisibleCols))
	for i := range totals {
		totals[i] = buildCostCell(grid.Totals[visibleStart+i])
	}

	footerNote := costsFooterNote(vm)
	switch {
	case cs.DrillRefusedReason != "":
		// The most recent refused drill attempt's honest reason (FR-007)
		// takes priority over the cursor cell's own delta/anomaly note —
		// it is the more relevant, more recent user-facing feedback.
		footerNote = cs.DrillRefusedReason
	case cs.ResourceRowNote != "":
		footerNote = cs.ResourceRowNote
	case footerNote == "" && top.Fallback.Fired && len(vm.Rows) > 0:
		// N3: this frame's own fetch genuinely returned zero records at the
		// drilled granularity, so the granularity-fallback re-plan already
		// substituted the parent's own coarser granularity/period — Rows is
		// non-empty here precisely because that substitution succeeded;
		// the empty-grid case (vm.Note below) covers the fallback's own
		// re-fetch ALSO coming back empty.
		footerNote = fmt.Sprintf("showing %s data — no records at the drilled granularity for the selected period", top.Granularity)
	case footerNote == "" && vm.Note != "" && !cs.Loading && len(cs.DrillStack) > 1:
		// X11: a legitimately empty finer-grain drill — the fetch genuinely
		// completed (not Loading) with zero records at this granularity
		// (e.g. a charge billed only monthly, so its weekly/daily window
		// has nothing) — must explain itself, never render a silent empty
		// grid indistinguishable from "still loading" or "genuinely no
		// spend." vm.Note is the ViewModel's own (depth-agnostic) honesty
		// note (Seam 8, one source); root-level (DrillStack depth 1) stays
		// silent here: an empty pivot view is ordinary (a fresh account,
		// or every row filtered to zero), not evidence of a granularity
		// mismatch.
		footerNote = vm.Note
	case footerNote == "" && costsAnyTotalMixed(totals):
		// X8 (body-level acceptance): the domain layer already refuses a
		// bare cross-currency TOTAL (costs.SumCells/buildCostCell's Mixed
		// cells); this is the explanatory note the suppressed "—" cells
		// need, shown only as a fallback so it never masks a more
		// cursor-relevant delta/anomaly/refusal note.
		footerNote = "mixed currencies in this view — no combined TOTAL"
	}

	return &CostsBody{
		Pivot:       string(top.RowDim),
		Metric:      string(cs.Metric),
		Granularity: string(top.Granularity),
		Breadcrumb:  buildCostsBreadcrumb(cs.DrillStack),
		Columns:     columns,
		Rows:        rows,
		Totals:      totals,
		CursorRow:   vm.Cursor.Row,
		CursorCol:   vm.Cursor.Col,
		ScrollX:     visibleStart,
		Loading:     cs.Loading,
		FooterNote:  footerNote,
		APICalls:    cs.APICalls,
		APICostUSD:  formatAPICost(cs.APICalls),
		DataThrough: cs.Store.DataThrough(costsQueryForFrame(top, cs.Metric), cs.Now),
		Currency:    grid.Currency,
	}
}

// buildCostCell derives the pre-resolved display cell from one aggregated
// CellValue: comma-grouped one-decimal amount, delta bucket tag, and the
// anomaly/estimated/negative flags the renderer applies verbatim. A
// zero-valued cell is always "neutral" regardless of the underlying delta
// math — a $500 -> $0.0 transition is not a meaningful "drop" signal, it's
// spend simply ending.
func buildCostCell(cv costs.CellValue) CostCell {
	if cv.Mixed {
		// X8: a USD+EUR (or any other cross-currency) sum never renders as
		// a bare number — "—" is not a formatted zero, it is an explicit
		// suppression marker (buildCostsBody's footer note explains why).
		return CostCell{Amount: "—", DeltaTag: "neutral", Estimated: cv.Estimated, Mixed: true}
	}
	tag := deltaTag(cv.Delta)
	if cv.Amount.Value == 0 {
		tag = "neutral"
	}
	return CostCell{
		Amount:    fmtCostAmount(cv.Amount.Value),
		DeltaTag:  tag,
		Anomaly:   cv.Anomaly != nil,
		Estimated: cv.Estimated,
		Negative:  cv.Amount.Value < 0,
	}
}

// costsAnyTotalMixed reports whether any visible Totals cell is Mixed (X8).
func costsAnyTotalMixed(totals []CostCell) bool {
	for _, c := range totals {
		if c.Mixed {
			return true
		}
	}
	return false
}

// stripCostsServiceVendorPrefix removes AWS's "Amazon "/"AWS " vendor
// prefix from a SERVICE row label — the vendor name is implied by every row
// in the pivot, so it carries no information ("Elastic Compute Cloud -
// Compute", not "Amazon Elastic Compute Cloud - Compute").
func stripCostsServiceVendorPrefix(label string) string {
	if s, ok := strings.CutPrefix(label, "Amazon "); ok {
		return s
	}
	if s, ok := strings.CutPrefix(label, "AWS "); ok {
		return s
	}
	return label
}

// fmtCostAmount formats v as wireframe.md's grid amounts: comma-grouped
// thousands, exactly one decimal place, no currency symbol
// ("1,204.1", "2,971.3").
func fmtCostAmount(v float64) string {
	neg := v < 0
	if neg {
		v = -v
	}
	s := strconv.FormatFloat(v, 'f', 1, 64)
	if neg && s == "0.0" {
		// A small negative value (e.g. -0.04) rounds to "0.0" at one
		// decimal place — without this guard the sign is reapplied below,
		// producing the display "-0.0" for a magnitude that is displayed
		// as zero.
		neg = false
	}
	intPart, decPart, _ := strings.Cut(s, ".")
	var grouped strings.Builder
	n := len(intPart)
	for i, r := range intPart {
		if i > 0 && (n-i)%3 == 0 {
			grouped.WriteByte(',')
		}
		grouped.WriteRune(r)
	}
	out := grouped.String() + "." + decPart
	if neg {
		out = "-" + out
	}
	return out
}

// deltaTag buckets a period-over-period fraction into the five-way tag
// wireframe.md describes ("growth red shades, drop green shades, |delta| <
// threshold neutral"): "neutral" below costsDeltaNeutralThreshold, then
// "growth"/"drop" each split into "-soft" (below costsDeltaStrongThreshold)
// and "-strong" (at or above it) — or "" when there is no prior-column
// baseline (NaN — first column or missing prior data).
func deltaTag(delta float64) string {
	if math.IsNaN(delta) {
		return ""
	}
	mag := math.Abs(delta)
	if mag < costsDeltaNeutralThreshold {
		return "neutral"
	}
	intensity := "soft"
	if mag >= costsDeltaStrongThreshold {
		intensity = "strong"
	}
	if delta > 0 {
		return "growth-" + intensity
	}
	return "drop-" + intensity
}

// formatAPICost renders the session CE-call counter's estimated dollar cost
// (FR-013: calls x $0.01).
func formatAPICost(calls int) string {
	return fmt.Sprintf("$%.2f", float64(calls)*apiCostPerCall)
}

// periodLabel formats p for column headers / breadcrumbs at granularity g:
// "Jul'26" (month), "2026" (year), "Jan 1-4" (week, clipped to month
// boundary per wireframe.md), "Jan 5" (day).
func periodLabel(p costs.Period, g costs.Granularity) string {
	start, err := costs.ParseDate(p.Start)
	if err != nil {
		return p.Start
	}
	switch g {
	case costs.GranularityYear:
		return start.Format("2006")
	case costs.GranularityWeek:
		end, err := costs.ParseDate(p.End)
		if err != nil {
			return start.Format("Jan 2")
		}
		lastDay := end.AddDate(0, 0, -1)
		if start.Month() == lastDay.Month() {
			return fmt.Sprintf("%s %d–%d", start.Format("Jan"), start.Day(), lastDay.Day())
		}
		return fmt.Sprintf("%s – %s", start.Format("Jan 2"), lastDay.Format("Jan 2"))
	case costs.GranularityDay:
		return start.Format("Jan 2")
	default: // month
		return start.Format("Jan'06")
	}
}

// costsFrameTitle builds the Cost Explorer breadcrumb title (wireframe.md:
// "Costs: by service · invoice · monthly · Aug'25-Jul'26" at the root,
// "Costs: EC2 - Compute ▸ May'26 · by usage type · daily · invoice" when
// drilled) — the single source Snapshot() feeds into ViewState.FrameTitle,
// consumed verbatim by both the TUI and the web renderer.
func costsFrameTitle(body *CostsBody) string {
	if body == nil {
		return "Costs"
	}
	var b strings.Builder
	b.WriteString("Costs: ")
	if len(body.Breadcrumb) > 0 {
		b.WriteString(strings.Join(body.Breadcrumb, " ▸ "))
		b.WriteString(" · by ")
	} else {
		b.WriteString("by ")
	}
	b.WriteString(strings.ToLower(body.Pivot))
	b.WriteString(" · ")
	b.WriteString(body.Metric)
	b.WriteString(" · ")
	b.WriteString(body.Granularity)
	return b.String()
}

// buildCostsBreadcrumb describes the drill path: one segment per pushed
// frame naming the row value the user drilled into at that level (the
// dimension pinned entering frame i is stack[i-1].RowDim, keyed in
// stack[i].Filter).
func buildCostsBreadcrumb(stack []costs.DrillLevel) []string {
	if len(stack) <= 1 {
		return nil
	}
	out := make([]string, 0, len(stack)-1)
	for i := 1; i < len(stack); i++ {
		prev, cur := stack[i-1], stack[i]
		seg := string(prev.RowDim)
		if vals := cur.Filter.Equals[prev.RowDim]; len(vals) > 0 {
			// The pinned filter value may be a translated CE FILTERABLE
			// value (e.g. REGION's legitimately empty ""), never itself a
			// display string — DisplayKeyForFilterValue is the single
			// inverse every rendered segment reads through, so a drill
			// pinned on that empty value never renders as a blank
			// breadcrumb segment.
			seg = costs.DisplayKeyForFilterValue(prev.RowDim, vals[0])
			if prev.RowDim == costs.DimensionService {
				seg = stripCostsServiceVendorPrefix(seg)
			}
		}
		out = append(out, seg)
	}
	return out
}

// costsFooterNote is the footer's first slot: the cursor cell's anomaly
// root cause when flagged, else its period-over-period delta
// ("Δ vs prev: +12.4%"), else empty (no baseline). vm.Cursor already
// indexes vm.Rows[*].Cells validly (screen.BuildViewModel's clamp, Seam
// 8) — the only remaining guard is vm.Rows being empty (nothing to
// index), which the clamp intentionally leaves Cursor pointing at 0 for.
func costsFooterNote(vm screen.ViewModel) string {
	if len(vm.Rows) == 0 || vm.Cursor.Row >= len(vm.Rows) {
		return ""
	}
	cells := vm.Rows[vm.Cursor.Row].Cells
	if vm.Cursor.Col < 0 || vm.Cursor.Col >= len(cells) {
		return ""
	}
	cell := cells[vm.Cursor.Col]
	if cell.Anomaly != nil {
		return anomalyFooterNote(cell.Anomaly)
	}
	if math.IsNaN(cell.Delta) {
		return ""
	}
	return fmt.Sprintf("Δ vs prev: %+.1f%%", cell.Delta*100)
}

// anomalyFooterNote is the single place a costs.AnomalyMark becomes the
// footer's user-facing phrase: "⚠ anomaly: <service> +$<impact>/mo —
// <usage type>". Never renders mapAnomaly's raw "DIM=value" RootCause join
// (project rule forbids raw enum/dimension dumps in any rendered surface) —
// reads only the structured Dimension map.
func anomalyFooterNote(a *costs.AnomalyMark) string {
	service := stripCostsServiceVendorPrefix(anomalyDimensionValue(a, costs.DimensionService))
	usageType := anomalyDimensionValue(a, costs.DimensionUsageType)
	note := fmt.Sprintf("⚠ anomaly: %s +$%s/mo", service, fmtCostAmount(a.Impact.Value))
	if usageType != "" {
		note += " — " + usageType
	}
	return note
}

// anomalyDimensionValue reads dim from a.Dimension, "" when absent.
func anomalyDimensionValue(a *costs.AnomalyMark, dim costs.Dimension) string {
	return a.Dimension[dim]
}
