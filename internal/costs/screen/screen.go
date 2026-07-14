// Package screen is the Cost Explorer screen's pure state machine
// (specs/021-cost-explorer/architecture.md): the "what should happen"
// decisions internal/app used to make inline, mixed in with session
// bookkeeping. Every function here is pure — inputs are values, outputs are
// typed outcomes; no session, no controller, no clocks except an injected
// now. Imports internal/costs only; internal/app imports this package, so
// this package must never import internal/app, internal/runtime, or
// internal/tui.
package screen

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/k2m30/a9s/v3/internal/costs"
)

// FetchPlan is what ensureCostsShapeFetched needs to know before dispatching
// a KindFetchCosts task. Grid and Anomalies are computed independently —
// cost coverage and anomaly freshness have different lifecycles and never
// gate each other (the doc's own named bug classes: a grid-missing shape
// must not re-request still-fresh anomalies, and a fully warm grid must not
// suppress a genuinely stale anomaly slot).
type FetchPlan struct {
	Grid      bool
	Anomalies bool
	Query     costs.Query
}

// PlanFetch derives FetchPlan from store's own coverage/freshness state at
// now — the single place both halves of "does anything need fetching" are
// decided, so a caller can never accidentally couple them.
func PlanFetch(store *costs.Store, q costs.Query, window []costs.Period, now time.Time) FetchPlan {
	_, missing := store.Lookup(q, window, now)
	_, anomaliesFresh := store.AnomaliesCoverage(window, now)
	return FetchPlan{
		Grid:      len(missing) > 0,
		Anomalies: !anomaliesFresh,
		Query:     q,
	}
}

// DrillPath owns the pinned dimensions accumulated by a drill chain, in the
// order they were pinned. NextDim is derived from what is already pinned —
// a dimension in the filter can never be offered again (X5): from the
// USAGE_TYPE pivot, drilling to SERVICE and pinning that too still advances
// to RESOURCE_ID, never a redundant USAGE_TYPE level.
type DrillPath struct {
	Pinned []costs.Dimension
}

// drillChain is the canonical dimension order every chain advances through,
// regardless of which dimension the chain started from — SERVICE first (if
// not yet pinned), then USAGE_TYPE, then RESOURCE_ID last.
var drillChain = []costs.Dimension{costs.DimensionService, costs.DimensionUsageType, costs.DimensionResourceID}

// Next returns the next unpinned dimension in drillChain, skipping any
// dimension p already has pinned. ok is false once every chain dimension
// (through RESOURCE_ID) is pinned — the bottom of the chain.
func (p DrillPath) Next() (costs.Dimension, bool) {
	pinned := make(map[costs.Dimension]bool, len(p.Pinned))
	for _, d := range p.Pinned {
		pinned[d] = true
	}
	for _, d := range drillChain {
		if !pinned[d] {
			return d, true
		}
	}
	return "", false
}

// finerGranularity walks the time-axis granularity chain one step finer —
// the drill's column granularity, orthogonal to DrillPath's dimension
// chain: year -> month -> week -> day, day staying at day.
func finerGranularity(g costs.Granularity) costs.Granularity {
	switch g {
	case costs.GranularityYear:
		return costs.GranularityMonth
	case costs.GranularityMonth:
		return costs.GranularityWeek
	default:
		return costs.GranularityDay
	}
}

// ScreenState is the screen-level input Select decides against: the current
// frame's pivot dimension and accumulated ancestry, its granularity, and
// whether its own shape is still in flight. ResourceTypeFor is the a9s
// catalog lookup (CostExplorerServiceName -> shortName), injected so this
// package never imports internal/resource.
type ScreenState struct {
	RowDim          costs.Dimension
	Path            DrillPath
	Filter          costs.Filter
	Granularity     costs.Granularity
	Loading         bool
	Now             time.Time
	ResourceTypeFor func(service string) (shortName string, ok bool)
}

// GridRowRef is the cursor's resolved row at Select time: Present is false
// when the cursor's row index does not resolve against the live (filtered)
// grid — a covered-but-empty shape, or a stale cursor the reducer has
// already clamped away.
type GridRowRef struct {
	Present bool
	Value   string
	// CellNonZero is whether the row's own cell at the selected column
	// (cell.Period) is non-zero — the granularity-fallback gate's own
	// "was the drilled parent cell actually non-zero" input, distinct from
	// the row's total across every column.
	CellNonZero bool
}

// PeriodRef is the cursor's selected column at Select time.
type PeriodRef struct {
	Period costs.Period
}

// SelectOutcome is the typed result of one Enter press — the caller applies
// it, never re-derives intent from grid state.
type SelectOutcome interface{ isSelectOutcome() }

// NoSelection is returned whenever there is nothing to descend into: the
// frame's own shape is still in flight (state.Loading — unconditional, even
// a resolvable row must not bypass it, X7), or the frame is fully covered
// but the cursor's row does not resolve against the live grid. The sole
// consumer (applyCostsSelect) always treated the two cases identically (a
// strict no-op), so they collapse into one outcome rather than two
// zero-field types with no distinct handling.
type NoSelection struct{}

func (NoSelection) isSelectOutcome() {}

// PushDrill is returned when the selected row descends one level: Value is
// the CE FILTERABLE value for the selected row (costs.FilterValueForKey) —
// legitimately empty for a REGION row keyed "NoRegion" (an unresolved row
// is NoSelection, never reaching PushDrill at all); Window is built via
// WindowWithin, always inside the selected cell's own period.
type PushDrill struct {
	Dim    costs.Dimension
	Value  string
	Window []costs.Period
	// Granularity is the pushed frame's own time-axis granularity — the
	// SAME finerGranularity(state.Granularity) step Select already
	// computed internally to build Window via WindowWithin, surfaced so
	// the caller never re-derives it (the "two homes" duplication this
	// field closes: internal/app used to keep its own finerGranularity
	// copy purely to set the pushed DrillLevel's Granularity field).
	Granularity costs.Granularity
	// ParentGranularity/SelectedPeriod/ParentCellNonZero carry the
	// granularity-fallback gate's inputs straight into the pushed frame's
	// own costs.FallbackGate — the parent drill step's own granularity and
	// selected period, and whether the selected cell was non-zero.
	ParentGranularity costs.Granularity
	SelectedPeriod    costs.Period
	ParentCellNonZero bool
}

func (PushDrill) isSelectOutcome() {}

// OpenResource is returned when the selected row is a leaf RESOURCE_ID row
// whose pinned SERVICE maps to a registered a9s resource type and whose
// window still falls within the resource-level retention horizon.
type OpenResource struct{ Locator ResourceLocator }

func (OpenResource) isSelectOutcome() {}

// Refuse is returned for an honest, footer-rendered refusal: an unmapped
// service, or a resource-leaf window entirely outside the 14-day
// resource-level retention horizon.
type Refuse struct{ Reason string }

func (Refuse) isSelectOutcome() {}

// ResourceLocator is the navigation context a resource-leaf OpenResource
// carries.
type ResourceLocator struct {
	Type string
	ID   string
}

// Select decides the outcome of one Enter press against state, row, cell.
// Loading gates unconditionally first (X7); an unresolved row is a strict
// no-op; a resolved row either descends (PushDrill, gated by the
// resource-drill window/service check when the NEXT dimension is
// RESOURCE_ID) or, when state.RowDim is already the chain's leaf
// (RESOURCE_ID), resolves to OpenResource or Refuse.
func Select(state ScreenState, row GridRowRef, cell PeriodRef) SelectOutcome {
	if state.Loading {
		return NoSelection{}
	}
	if !row.Present {
		return NoSelection{}
	}

	// A RESOURCE_ID row is always the chain's own leaf, regardless of what
	// ancestor dimensions the drill pinned to get here (SERVICE/USAGE_TYPE
	// alone govern DrillPath's own chain).
	if state.RowDim == costs.DimensionResourceID {
		return selectResourceLeaf(state, row, cell)
	}

	pinnedSoFar := append(append([]costs.Dimension{}, state.Path.Pinned...), state.RowDim)
	nextDim, ok := (DrillPath{Pinned: pinnedSoFar}).Next()
	if !ok {
		return selectResourceLeaf(state, row, cell)
	}

	// filterValue is the CE FILTERABLE value for the row being pinned —
	// identity for every dimension except REGION, where the grid's own
	// display key ("NoRegion") never matches as a filter value (only ""
	// does). Every downstream use of the pinned value (the RESOURCE_ID gate
	// probe below, and the pushed frame's own Value) reads through this one
	// translation, never row.Value directly.
	filterValue := costs.FilterValueForKey(state.RowDim, row.Value)

	nextGran := finerGranularity(state.Granularity)
	window := costs.WindowWithin(cell.Period, nextGran, state.Now)
	if nextDim == costs.DimensionResourceID {
		pinned := pinFilterValue(state.Filter, state.RowDim, filterValue)
		clamped := costs.ClampResourceDrillWindow(window, state.Now)
		if allowed, reason := costs.ResourceDrillAllowed(costs.DrillLevel{Window: clamped, Filter: pinned}, state.Now); !allowed {
			return Refuse{Reason: reason}
		}
		window = clamped
	}
	return PushDrill{
		Dim: nextDim, Value: filterValue, Window: window, Granularity: nextGran,
		ParentGranularity: state.Granularity, SelectedPeriod: cell.Period, ParentCellNonZero: row.CellNonZero,
	}
}

// selectResourceLeaf resolves Enter on a RESOURCE_ID row (state.RowDim is
// already the chain's leaf): the pinned SERVICE must map to a registered
// a9s resource type (Refuse otherwise), and the selected cell's own period
// must still fall within the 14-day resource-level retention horizon
// (Refuse otherwise) — only then does the row navigate to OpenResource.
func selectResourceLeaf(state ScreenState, row GridRowRef, cell PeriodRef) SelectOutcome {
	services := state.Filter.Equals[costs.DimensionService]
	if len(services) != 1 {
		return Refuse{Reason: "pin exactly one SERVICE to open a resource"}
	}
	service := services[0]
	if state.ResourceTypeFor == nil {
		return Refuse{Reason: fmt.Sprintf("no a9s resource type mapped for service %q", service)}
	}
	shortName, ok := state.ResourceTypeFor(service)
	if !ok {
		return Refuse{Reason: fmt.Sprintf("no a9s resource type mapped for service %q", service)}
	}

	clamped := costs.ClampResourceDrillWindow([]costs.Period{cell.Period}, state.Now)
	if allowed, reason := costs.ResourceDrillAllowed(costs.DrillLevel{Window: clamped, Filter: state.Filter}, state.Now); !allowed {
		return Refuse{Reason: reason}
	}

	return OpenResource{Locator: ResourceLocator{Type: shortName, ID: row.Value}}
}

// pinFilterValue returns a copy of f with dim pinned to value — the same
// "narrow the filter to the selected row" step every PushDrill takes,
// applied here only to build the RESOURCE_ID gate check's own Filter (the
// pushed frame's actual Filter is the caller's concern, this is a local,
// read-only probe).
func pinFilterValue(f costs.Filter, dim costs.Dimension, value string) costs.Filter {
	out := costs.Filter{
		Equals:    make(map[costs.Dimension][]string, len(f.Equals)+1),
		NotEquals: make(map[costs.Dimension][]string, len(f.NotEquals)),
	}
	for k, v := range f.Equals {
		out.Equals[k] = append([]string(nil), v...)
	}
	for k, v := range f.NotEquals {
		out.NotEquals[k] = append([]string(nil), v...)
	}
	out.Equals[dim] = []string{value}
	return out
}

// CursorPos is a (row, column) grid position. As BuildViewModel's INPUT it
// is the raw, possibly out-of-bounds cursor the reducer has accumulated
// (absolute column, unrelated to any viewport); as its OUTPUT (ViewModel.
// Cursor) it is always a valid index — Row into Rows, Col into VisibleCols
// (and into each Rows[i].Cells, which BuildViewModel slices to the same
// visible window) — unifying the old adapter's separate absolute cursorCol
// + viewport-relative relCursorCol into one post-clamp value.
type CursorPos struct {
	Row, Col int
}

// Viewport is the visible-column window: Cols<=0 means unsliced (show
// every column BuildViewModel receives); otherwise ScrollX is the starting
// column, clamped so the window never runs past the grid's own last
// column.
type Viewport struct {
	Cols    int
	ScrollX int
}

// ViewModel is the one clamped view the renderer and the Enter handler
// both read (architecture.md Seam 8: "the renderer and Enter handler share
// one clamped view-model") — Rows is the display-filtered, viewport-sliced
// row set; Cursor already indexes it validly; VisibleCols is the same
// column window; Note is the empty-finer-grain honesty explanation when
// Rows is empty.
type ViewModel struct {
	Rows        []costs.GridRow
	Cursor      CursorPos
	VisibleCols []costs.Period
	Note        string
}

// emptyFinerGrainNote is BuildViewModel's own honesty note for a
// zero-row result — the single source of the text (costs_codex_test.go's
// X11 controller-level pin stays as the full-stack acceptance test for the
// depth-gated ("not at root") refinement the adapter applies on top of
// this).
const emptyFinerGrainNote = "no records at this granularity for the selected period — this charge may be billed at a coarser granularity"

// costsAmountRoundsToZero reports whether v rounds to "0.0" under the
// grid's one-decimal-place display rounding.
func costsAmountRoundsToZero(v float64) bool {
	return math.Round(math.Abs(v)*10) == 0
}

// allCellsRoundToZero reports whether every cell in cells rounds to "0.0"
// — a row filtered on this basis has nothing to show in the VISIBLE
// window (a net-zero row from genuinely non-zero, offsetting cells is
// never dropped).
func allCellsRoundToZero(cells []costs.CellValue) bool {
	for _, c := range cells {
		if !costsAmountRoundsToZero(c.Amount.Value) {
			return false
		}
	}
	return true
}

// visibleRowTotal sums cells' own Amount.Value — the VISIBLE-window row
// total BuildViewModel's own re-sort ranks by, distinct from BuildGrid's
// full-window row total.
func visibleRowTotal(cells []costs.CellValue) float64 {
	var sum float64
	for _, c := range cells {
		sum += c.Amount.Value
	}
	return sum
}

// viewportSlice returns the [start, count) window into totalCols per vp —
// the whole range when vp.Cols<=0 or is not narrower than totalCols,
// otherwise a vp.Cols-wide window starting at vp.ScrollX (clamped so it
// never runs past totalCols).
func viewportSlice(totalCols int, vp Viewport) (start, count int) {
	start, count = 0, totalCols
	if vp.Cols > 0 && vp.Cols < totalCols {
		count = vp.Cols
		start = clampIndex(vp.ScrollX, 0, totalCols-count)
	}
	return start, count
}

// clampIndex clamps v into [lo, hi]; when hi < lo (an empty range) it
// returns lo — the "clamped floor" every zero-length Rows/VisibleCols
// case needs (a valid-looking index into a set that has nothing to index).
func clampIndex(v, lo, hi int) int {
	if hi < lo {
		return lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// BuildViewModel derives the one clamped, viewport-sliced view of g that
// both rendering and Enter-handling read (architecture.md Seam 8). Pure:
// g/rowDim/cursor/vp are the whole input; revision is carried only as a
// black-box memoization hint for callers that choose to cache around this
// call (this implementation does not cache internally — every call
// recomputes fresh from g, which is itself already the caller's own
// current, correct data).
//
// rowDim==DimensionLinkedAccount is exempt from the sub-cent display
// filter (ApplyRowAttrs already relabels those rows upstream; hiding a
// linked account because its VISIBLE window rounds to zero would make an
// account silently vanish from the account-level pivot, which the other
// pivots' display-filter reasoning does not apply to). Every kept row's
// Cells is sliced to the same visible window as VisibleCols, so
// Cursor.Col indexes both consistently.
func BuildViewModel(g costs.Grid, rowDim costs.Dimension, cursor CursorPos, vp Viewport, revision int) ViewModel {
	_ = revision
	visStart, visCount := viewportSlice(len(g.Columns), vp)
	visEnd := min(visStart+visCount, len(g.Columns))
	visibleCols := append([]costs.Period(nil), g.Columns[visStart:visEnd]...)

	out := make([]costs.GridRow, 0, len(g.Rows))
	for _, r := range g.Rows {
		cellEnd := min(visStart+visCount, len(r.Cells))
		cellStart := min(visStart, cellEnd)
		visible := r.Cells[cellStart:cellEnd]
		if rowDim != costs.DimensionLinkedAccount && allCellsRoundToZero(visible) {
			continue
		}
		out = append(out, costs.GridRow{
			Key:   r.Key,
			Label: r.Label,
			Cells: append([]costs.CellValue(nil), visible...),
		})
	}
	// data-model.md/spec.md: row sort is descending by row total over the
	// VISIBLE window, not BuildGrid's own full-window ordering — a row
	// whose spend lands entirely in a scrolled-out column must not
	// outrank one whose spend is entirely on-screen. Re-sort here, the one
	// place both rendering and Enter-handling read (Seam 8), rather than
	// each consumer re-deriving its own visible-window ranking.
	sort.SliceStable(out, func(i, j int) bool {
		ti, tj := math.Abs(visibleRowTotal(out[i].Cells)), math.Abs(visibleRowTotal(out[j].Cells))
		if math.IsNaN(ti) {
			return false
		}
		if math.IsNaN(tj) {
			return true
		}
		return ti > tj
	})

	note := ""
	if len(out) == 0 {
		note = emptyFinerGrainNote
	}

	return ViewModel{
		Rows: out,
		Cursor: CursorPos{
			Row: clampIndex(cursor.Row, 0, len(out)-1),
			Col: clampIndex(cursor.Col-visStart, 0, len(visibleCols)-1),
		},
		VisibleCols: visibleCols,
		Note:        note,
	}
}

// unblendedExcludedRecordTypes are the RECORD_TYPE values the unblended
// display metric's fetch excludes via Filter.NotEquals — data-model.md's
// display mapping: "unblended -> UnblendedCost (filter excludes Tax/
// Credit/Refund)". A distinct Query/CacheKey from the unfiltered invoice
// shape.
var unblendedExcludedRecordTypes = []string{"Tax", "Credit", "Refund"}

// QueryForFrame returns the CE query shape that must be cached/fetched to
// render drill's grid at metric: the base shape (RowDim group-by, drill's
// accumulated Filter, no RECORD_TYPE clause) for every display metric
// except unblended, which reads UnblendedCost from a DISTINCT,
// RECORD_TYPE-excluding fetch — its own Query/CacheKey, never mixed with
// the base fetch's records. Pure (costs.DrillLevel + costs.Metric ->
// costs.Query only) — the single source both the adapter's grid
// aggregation and its shape-miss detection call, so the two can never
// diverge on what "this frame's data" means.
func QueryForFrame(drill costs.DrillLevel, metric costs.Metric) costs.Query {
	filter := cloneFilter(drill.Filter)
	if metric == costs.MetricUnblended {
		filter.NotEquals[costs.DimensionRecordType] = unblendedExcludedRecordTypes
	}
	q := costs.Query{
		Granularity: drill.Granularity.APIGranularity(),
		GroupBy:     []costs.Dimension{drill.RowDim},
		Filter:      filter,
	}
	if len(drill.Window) > 0 {
		q.Range = costs.Period{Start: drill.Window[0].Start, End: drill.Window[len(drill.Window)-1].End}
	}
	return q
}

// cloneFilter returns a deep copy of f's Equals/NotEquals maps — QueryForFrame's
// own need to add an unblended-only NotEquals clause without mutating the
// caller's drill.Filter.
func cloneFilter(f costs.Filter) costs.Filter {
	out := costs.Filter{
		Equals:    make(map[costs.Dimension][]string, len(f.Equals)),
		NotEquals: make(map[costs.Dimension][]string, len(f.NotEquals)+1),
	}
	for k, v := range f.Equals {
		out.Equals[k] = append([]string(nil), v...)
	}
	for k, v := range f.NotEquals {
		out.NotEquals[k] = append([]string(nil), v...)
	}
	return out
}
