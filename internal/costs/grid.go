package costs

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// Grid is what the controller snapshots into CostsBody.
type Grid struct {
	RowDim   Dimension
	Rows     []GridRow
	Columns  []Period
	Totals   []CellValue
	Currency string
}

// GridRow is one dimension-value's series of cells across Grid.Columns.
type GridRow struct {
	Key   string
	Label string
	Cells []CellValue
}

// CellValue is one (row x column) rendered amount.
type CellValue struct {
	Amount    Amount
	Delta     float64
	Estimated bool
	Anomaly   *AnomalyMark
	// Mixed is true only on a Grid.Totals cell whose contributing rows
	// spanned more than one currency this column — Amount is meaningless
	// (SumCells refused to produce a bare cross-currency sum); renderers
	// must suppress the numeric amount and explain instead, never fall
	// back to displaying a naive total.
	Mixed bool
}

// Money is one dimension-tagged amount whose unit must survive until
// aggregation is proven legal — the type SumCells consumes and produces, so
// a cross-currency sum can never leave the domain layer as a bare float64.
type Money struct {
	Value float64
	Unit  string
}

// TotalOutcome is the result of summing a set of Money values. Total is
// valid only when Mixed is false: every summed cell shared one currency.
// Mixed true means the cells spanned more than one currency — Total carries
// no meaningful value, and renderers must show per-cell amounts with an
// explanatory note instead of a combined TOTAL.
type TotalOutcome struct {
	Total Money
	Mixed bool
}

// SumCells sums cells' values when every cell with a non-empty Unit shares
// the SAME unit; a second, different unit reports Mixed instead of quietly
// summing across currencies. A cell with an empty Unit (no data contributed
// this column) never itself triggers Mixed — its Value (always zero for an
// empty cell) still folds into the running sum, but it carries no currency
// opinion of its own.
func SumCells(cells []Money) TotalOutcome {
	var sum float64
	unit := ""
	mixed := false
	for _, c := range cells {
		sum += c.Value
		if c.Unit == "" {
			continue
		}
		if unit == "" {
			unit = c.Unit
		} else if unit != c.Unit {
			mixed = true
		}
	}
	if mixed {
		return TotalOutcome{Mixed: true}
	}
	return TotalOutcome{Total: Money{Value: sum, Unit: unit}}
}

// cellAgg accumulates raw metric contributions for one (row, column) cell
// before Delta/Amount are derived.
type cellAgg struct {
	sum       float64
	unit      string
	estimated bool
	has       bool
}

// BuildGrid aggregates recs into the requested window's columns (summing
// daily records into ISO weeks and monthly records into years as needed —
// one canonical data source per FR-005, no independently fetched
// aggregates), computes per-row period-over-period deltas, sorts rows desc
// by absolute row total, and drops rows whose every cell is zero or has no
// data — a net-zero row with offsetting non-zero cells stays.
func BuildGrid(recs []Record, metric Metric, window []Period) Grid {
	colRanges := make([][2]time.Time, len(window))
	for i, w := range window {
		colRanges[i] = [2]time.Time{parseDateOrZero(w.Start), parseDateOrZero(w.End)}
	}

	rowOrder := make([]string, 0)
	rowCells := make(map[string][]cellAgg)
	unitSet := make(map[string]struct{})

	for _, rec := range recs {
		amt, ok := rec.Metrics[metric]
		if !ok {
			continue
		}
		colIdx := columnFor(rec.Period, colRanges)
		if colIdx == -1 {
			continue
		}
		key := strings.Join(rec.Keys, "/")
		cells, exists := rowCells[key]
		if !exists {
			cells = make([]cellAgg, len(window))
			rowOrder = append(rowOrder, key)
		}
		c := cells[colIdx]
		c.sum += amt.Value
		c.has = true
		if amt.Unit != "" {
			c.unit = amt.Unit
			unitSet[amt.Unit] = struct{}{}
		}
		if rec.Estimated {
			c.estimated = true
		}
		cells[colIdx] = c
		rowCells[key] = cells
	}

	type rowAgg struct {
		key   string
		cells []cellAgg
		total float64
	}
	rows := make([]rowAgg, 0, len(rowOrder))
	for _, key := range rowOrder {
		cells := rowCells[key]
		total := 0.0
		for _, c := range cells {
			total += c.sum
		}
		rows = append(rows, rowAgg{key: key, cells: cells, total: total})
	}

	// Sort desc by ABSOLUTE total so a large negative (credit) row ranks by
	// its true magnitude. A NaN total (malformed metric) can't be
	// meaningfully ranked — pin it last rather than let it poison the
	// comparator (NaN < x and x < NaN are both false, which breaks sort's
	// ordering invariant and can scramble unrelated rows).
	sort.SliceStable(rows, func(i, j int) bool {
		ai, aj := math.Abs(rows[i].total), math.Abs(rows[j].total)
		if math.IsNaN(ai) {
			return false
		}
		if math.IsNaN(aj) {
			return true
		}
		return ai > aj
	})

	kept := make([]GridRow, 0, len(rows))
	for _, r := range rows {
		if allCellsZeroOrEmpty(r.cells) {
			continue
		}
		kept = append(kept, GridRow{Key: r.key, Label: r.key, Cells: buildCellValues(r.cells)})
	}

	totalsCells := make([]cellAgg, len(window))
	totalsMoney := make([][]Money, len(window))
	for _, r := range rows {
		for i, c := range r.cells {
			totalsCells[i] = mergeCellAgg(totalsCells[i], c)
			if c.has {
				totalsMoney[i] = append(totalsMoney[i], Money{Value: c.sum, Unit: c.unit})
			}
		}
	}

	currency := ""
	if len(unitSet) == 1 {
		for u := range unitSet {
			currency = u
		}
	}

	return Grid{
		Rows:     kept,
		Columns:  append([]Period(nil), window...),
		Totals:   buildTotalsCellValues(totalsCells, totalsMoney),
		Currency: currency,
	}
}

// allCellsZeroOrEmpty reports whether every cell is either zero-valued or
// carries no data at all. A row filtered on this basis has nothing to show
// in ANY column — distinct from a row whose SUM happens to net to zero from
// genuinely non-zero cells (e.g. +100 then -100), which is never dropped.
func allCellsZeroOrEmpty(cells []cellAgg) bool {
	for _, c := range cells {
		if c.has && c.sum != 0 {
			return false
		}
	}
	return true
}

// ApplyRowAttrs relabels g's rows using attrs (id -> display name, e.g.
// Store.Attrs' linked-account id -> account name): a row whose Key has an
// attrs entry becomes "name (id)"; a row with no entry keeps its raw Key
// unchanged.
func ApplyRowAttrs(g Grid, attrs map[string]string) Grid {
	for i, row := range g.Rows {
		if name, ok := attrs[row.Key]; ok {
			g.Rows[i].Label = fmt.Sprintf("%s (%s)", name, row.Key)
		}
	}
	return g
}

// ApplyAnomalies marks every grid cell whose row matches an AnomalyMark's
// g.RowDim dimension value and whose column CONTAINS the mark's Period.Start
// with that mark (FR-014). Matching keys off Start containment rather than
// full-Period containment: a month-long anomaly drilled into week/day
// columns no longer fits inside any single finer column, but its Start day
// still does, and that is the column the overlay belongs on.
func ApplyAnomalies(g Grid, marks []AnomalyMark) Grid {
	if len(marks) == 0 {
		return g
	}
	colRanges := make([][2]time.Time, len(g.Columns))
	for i, w := range g.Columns {
		colRanges[i] = [2]time.Time{parseDateOrZero(w.Start), parseDateOrZero(w.End)}
	}
	// Each mark's column is a function of the mark and the grid's columns
	// alone — never the row — so it is resolved once per mark here rather
	// than recomputed on every (row, mark) pair the loop below visits. A
	// mark whose own Period.Start fails to parse is skipped outright (-1)
	// rather than silently searching for a zero-Time column match.
	markCols := make([]int, len(marks))
	for i, mark := range marks {
		t, err := ParseDate(mark.Period.Start)
		if err != nil {
			markCols[i] = -1
			continue
		}
		markCols[i] = columnContaining(t, colRanges)
	}
	for ri := range g.Rows {
		row := &g.Rows[ri]
		for mi, mark := range marks {
			val, has := mark.Dimension[g.RowDim]
			if !has || val != row.Key {
				continue
			}
			colIdx := markCols[mi]
			if colIdx == -1 || colIdx >= len(row.Cells) {
				continue
			}
			mk := mark
			row.Cells[colIdx].Anomaly = &mk
		}
	}
	return g
}

func mergeCellAgg(a, b cellAgg) cellAgg {
	a.sum += b.sum
	if b.has {
		a.has = true
	}
	if b.unit != "" {
		a.unit = b.unit
	}
	if b.estimated {
		a.estimated = true
	}
	return a
}

// buildCellValues derives Amount/Delta/Estimated for one row's cells.
// Delta is NaN for the first column (no previous column exists) and
// whenever the previous column has no data or a zero value (percentage
// change from zero is undefined, not infinite). The denominator is
// |prevVal|, not prevVal: delta's sign must reflect the effect on SPEND, not
// a naive signed ratio — for a negative (credit/refund) row, -100 -> -50 is
// the credit shrinking (spend growth, positive delta) while -50 -> -100 is
// the credit growing (a drop, negative delta); dividing by the signed
// prevVal would invert both.
func buildCellValues(cells []cellAgg) []CellValue {
	out := make([]CellValue, len(cells))
	var prevVal float64
	prevHas := false
	for i, c := range cells {
		delta := math.NaN()
		if i > 0 && prevHas && prevVal != 0 {
			delta = (c.sum - prevVal) / math.Abs(prevVal)
		}
		out[i] = CellValue{Amount: Amount{Value: c.sum, Unit: c.unit}, Delta: delta, Estimated: c.estimated}
		prevVal = c.sum
		prevHas = c.has
	}
	return out
}

// buildTotalsCellValues derives the Totals row's cells through SumCells —
// BuildGrid's own Seam 5 wiring: a column whose contributing rows spanned
// more than one currency reports Mixed instead of a naive numeric sum
// (X8). Delta is computed against the previous NON-MIXED column's total
// only — a Mixed column breaks the period-over-period baseline for the
// column after it, exactly as a missing-data column already does.
func buildTotalsCellValues(cells []cellAgg, money [][]Money) []CellValue {
	out := make([]CellValue, len(cells))
	var prevVal float64
	prevHas := false
	for i, c := range cells {
		outcome := SumCells(money[i])
		if outcome.Mixed {
			out[i] = CellValue{Delta: math.NaN(), Estimated: c.estimated, Mixed: true}
			prevHas = false
			continue
		}
		delta := math.NaN()
		if i > 0 && prevHas && prevVal != 0 {
			delta = (outcome.Total.Value - prevVal) / math.Abs(prevVal)
		}
		out[i] = CellValue{
			Amount:    Amount{Value: outcome.Total.Value, Unit: outcome.Total.Unit},
			Delta:     delta,
			Estimated: c.estimated,
		}
		prevVal = outcome.Total.Value
		prevHas = true
	}
	return out
}

// columnFor returns the index of the window column that fully contains p,
// or -1 when p does not fall within any column — including when either of
// p's own boundaries fails to parse, so a malformed record is skipped
// outright rather than silently matched (or not) via a zero-Time stand-in.
func columnFor(p Period, ranges [][2]time.Time) int {
	start, errS := ParseDate(p.Start)
	end, errE := ParseDate(p.End)
	if errS != nil || errE != nil {
		return -1
	}
	for i, r := range ranges {
		if !start.Before(r[0]) && !end.After(r[1]) {
			return i
		}
	}
	return -1
}

// columnContaining returns the index of the window column whose [Start, End)
// range contains t, or -1 when no column does.
func columnContaining(t time.Time, ranges [][2]time.Time) int {
	for i, r := range ranges {
		if !t.Before(r[0]) && t.Before(r[1]) {
			return i
		}
	}
	return -1
}

// parseDateOrZero parses s via ParseDate, returning the zero Time on a
// parse failure. Used only to build column boundaries from data this
// package generates itself (a Grid's own Columns, a drill's own Window) —
// never on externally-sourced record/mark data, which is skipped outright
// on a parse failure instead (columnFor, ApplyAnomalies' per-mark
// resolution) rather than falling back to an unmatchable zero-Time range.
func parseDateOrZero(s string) time.Time {
	t, _ := ParseDate(s)
	return t
}
