// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package app

import (
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/costs"
	"github.com/k2m30/a9s/v3/core/costs/screen"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// CostsState is the controller-owned mutable state for the costs screen.
// One instance per drill level lives on the DrillStack; the top frame is
// live. Store holds every cached Query result merged in this session (and
// loaded from disk at Ensure time); Now is the clock injected once at
// EnsureCostsState and reused by every later action/render — the costs
// package's own "now is injected, never time.Now() internally" convention
// extended to its controller-side consumer.
type CostsState struct {
	DrillStack []costs.DrillLevel `json:"drill_stack"`
	Metric     costs.Metric       `json:"metric"`
	Loading    bool               `json:"loading,omitempty"`
	ErrorMsg   string             `json:"error_msg,omitempty"`
	APICalls   int                `json:"api_calls"`

	// AwaitedIdentity is the Query.Identity() of the fetch most recently
	// dispatched by ensureCostsShapeFetched and not yet resolved by a
	// matching ApplyCostsLoaded delivery ("" when nothing is in flight for
	// the current top frame). It is the state-derived signal
	// applyCostsSelect gates Enter on: a rowKey=="" cursor is ambiguous on
	// its own (it can mean either "still loading" or "every row
	// display-filtered"), but AwaitedIdentity resolves it precisely,
	// independent of what applyCostsSelect's own inputs can see.
	AwaitedIdentity string `json:"-"`

	// DrillRefusedReason is the honest FR-007 explanation for the most
	// recent refused resource drill (outside the 14-day retention window
	// even after clamping) — set by applyCostsSelect, surfaced verbatim by
	// buildCostsBody's footer, and cleared at the start of every Enter
	// press so it never outlives the attempt that produced it.
	DrillRefusedReason string `json:"drill_refused_reason,omitempty"`

	// ResourceRowNote is the observable result of Enter on a RESOURCE_ID
	// row (the bottom of the drill chain, FR-008): the selected resource ID
	// for a service with a mapped a9s detail view, an honest "unsupported"
	// message otherwise. Set by applyCostsSelect, surfaced verbatim by
	// buildCostsBody's footer (same seam as DrillRefusedReason), cleared at
	// the start of every Enter press.
	ResourceRowNote string `json:"resource_row_note,omitempty"`

	// ViewportCols is the renderer-supplied visible time-column count, set
	// via SetCostsViewportCols before each snapshot — named and shaped
	// after DetailState.ViewportHeight/SetDetailViewportHeight. Zero means
	// "unknown/unset": buildCostsBody shows every column, unsliced.
	ViewportCols int `json:"viewport_cols,omitempty"`

	Store *costs.Store `json:"-"`
	Now   time.Time    `json:"-"`
}

// metricCycle is the 'b' cycle order (data-model.md's display mapping list).
// net-unblended is stored-only and never appears here.
var metricCycle = []costs.Metric{
	costs.MetricInvoice,
	costs.MetricUnblended,
	costs.MetricAmortized,
	costs.MetricNetAmortized,
	costs.MetricBlended,
}

// granularityChain is the zoom order (year outermost, day finest).
var granularityChain = []costs.Granularity{
	costs.GranularityYear,
	costs.GranularityMonth,
	costs.GranularityWeek,
	costs.GranularityDay,
}

// topCostsState returns the CostsState of the top-of-stack screen when the
// top screen is ScreenCosts, nil otherwise. Callers must hold c.mu. Action
// handling (pivot/zoom/select/scroll) uses this: those require costs to be
// the ACTIVE screen, since key handling itself would go to an overlay on
// top if one were pushed.
func (c *Controller) topCostsState() *CostsState {
	if len(c.stack) == 0 {
		return nil
	}
	top := c.stack[len(c.stack)-1]
	if top.ID != runtime.ScreenCosts {
		return nil
	}
	return top.State.Costs
}

// costsStateBeneathOverlay returns the CostsState of the nearest ScreenCosts
// entry anywhere on the stack, searching from the top down. A CostsLoaded
// fetch is dispatched while costs is the active screen, but its result can
// land after the user has pushed Help/Identity on top of it — that delivery
// must still merge into the costs screen beneath rather than being dropped
// because the top-of-stack screen is no longer ScreenCosts. Callers must
// hold c.mu.
func (c *Controller) costsStateBeneathOverlay() *CostsState {
	for i := len(c.stack) - 1; i >= 0; i-- {
		if c.stack[i].ID == runtime.ScreenCosts {
			return c.stack[i].State.Costs
		}
	}
	return nil
}

// costsOpenAtNewestDrillLevel builds a DrillLevel positioned per FR-002
// "open at today": cursor on the newest (rightmost, current/open) column of
// window. ScrollX is left at zero here — reconcileCostsScrollToCursor (run
// separately once ViewportCols is known) is what actually pins the newest
// column to the visible right edge; this helper only owns the cursor
// placement every reset-to-default call site (initial seed, digit-0 pivot,
// trailing-anchored zoom-out) shares.
func costsOpenAtNewestDrillLevel(rowDim costs.Dimension, filter costs.Filter, gran costs.Granularity, window []costs.Period) costs.DrillLevel {
	dl := costs.DrillLevel{RowDim: rowDim, Filter: filter, Granularity: gran, Window: window}
	dl.Cursor.Col = len(window) - 1
	return dl
}

// costsCurrentCol returns the index of the newest drill-window column whose
// period has already started at or before now — the "current" column. Week
// and day child windows (unlike months, which monthsWithinPeriod
// future-clamps) tile the whole parent period, so a mid-month drill's raw
// last column can be an empty future week/day; the default child cursor must
// land on today's column, not that. Falls back to col 0 only for a window
// entirely in the future, which a normal drill never produces.
func costsCurrentCol(window []costs.Period, now time.Time) int {
	nowUTC := now.UTC()
	col := 0
	for i, p := range window {
		start, err := costs.ParseDate(p.Start)
		if err != nil || start.After(nowUTC) {
			continue
		}
		col = i
	}
	return col
}

// EnsureCostsState seeds the top-of-stack costs screen with its root drill
// frame (service pivot, monthly, invoice, trailing 12 months ending at
// now's own month) and loads the on-disk per-profile cost cache. Set-once —
// mirrors ensureTextState/ensureSelectorState: a second call on an
// already-seeded screen is a no-op.
func (c *Controller) EnsureCostsState(now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ensureCostsState(now)
}

func (c *Controller) ensureCostsState(now time.Time) {
	if len(c.stack) == 0 {
		return
	}
	top := &c.stack[len(c.stack)-1]
	if top.ID != runtime.ScreenCosts {
		return
	}
	if top.State.Costs != nil {
		return
	}
	// FR-002 "open at today": the cursor starts on the newest (current,
	// open) period, not column 0 — costsOpenAtNewestDrillLevel is the single
	// helper every reset-to-default path (initial seed, digit-0 pivot,
	// trailing-anchored zoom-out) shares, so they can never drift apart on
	// what "today" means.
	window := costs.BuildWindow(costs.GranularityMonth, now)
	// NoCache (demo, or an explicit --no-cache session) mirrors every other
	// on-disk cache in the codebase (session.NoCache's own
	// EnsureCacheStore-gated family): a memory-only Store, never reading a
	// real ~/.a9s/cache/<profile>--costs.yaml file and never writing one
	// (ApplyCostsLoaded's own dirty-store flush is gated the same way).
	var store *costs.Store
	if c.core.NoCache() {
		store = costs.NewMemoryStore(c.core.Profile())
	} else {
		store = costs.LoadStore(c.core.Profile())
	}
	// X9: LoadStore's own Recovered() (a corrupt/alien-version cache file
	// set aside to .bak) must always flash — a silent data loss otherwise.
	if store.Recovered() {
		c.applyIntents([]runtime.UIIntent{runtime.FlashIntent{
			Text: "cost cache was corrupt — set aside, recovered with a fresh cache",
		}})
	}
	top.State.Costs = &CostsState{
		DrillStack: []costs.DrillLevel{costsOpenAtNewestDrillLevel(
			costs.DimensionService, costs.Filter{}, costs.GranularityMonth, window,
		)},
		Metric: costs.MetricInvoice,
		Store:  store,
		Now:    now,
	}
	c.lastCostsNow = now
}

// GetCostsDrillStack returns a copy of the nearest ScreenCosts entry's drill
// stack (searching top-down past any overlay — costsStateBeneathOverlay,
// the same read used by ApplyCostsLoaded), root frame first. A read
// accessor, not an action-dispatch gate: the costs domain's own DrillStack
// is unaffected by a transient overlay sitting on top (Help/Identity, or a
// by-ID placeholder list awaiting its single-row delivery), so callers
// inspecting drill depth must still see it. Empty when no ScreenCosts entry
// exists anywhere on the stack, or CostsState has not been seeded yet.
func (c *Controller) GetCostsDrillStack() []costs.DrillLevel {
	c.mu.RLock()
	defer c.mu.RUnlock()
	cs := c.costsStateBeneathOverlay()
	if cs == nil {
		return nil
	}
	return append([]costs.DrillLevel(nil), cs.DrillStack...)
}

// SetCostsViewportCols sets ViewportCols on the top costs screen's
// CostsState to the renderer-supplied visible time-column count and
// reconciles ScrollX against the current cursor — the same reconciliation
// applyCostsMoveCol performs on a scroll action, run here too so the FIRST
// render after ViewportCols becomes known (before the user has ever
// scrolled) already shows the cursor's column, e.g. the newest period at
// the right edge on initial open (FR-002). No-op when the top screen is
// not ScreenCosts.
func (c *Controller) SetCostsViewportCols(n int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	cs := c.topCostsState()
	if cs == nil {
		return
	}
	cs.ViewportCols = n
	if len(cs.DrillStack) == 0 {
		return
	}
	top := &cs.DrillStack[len(cs.DrillStack)-1]
	reconcileCostsScrollToCursor(top, n)
}

// EnsureCostsFetch checks the top costs screen's current shape against the
// Store and returns the KindFetchCosts TaskRequest when the cache is cold
// (nil on a warm cache) — the sole fetch decision point for opening the
// Cost Explorer screen (SC-002: HandleNavigate/NavigateKindPushCosts must
// never fetch unconditionally). Call after EnsureCostsState has seeded the
// screen.
func (c *Controller) EnsureCostsFetch() []runtime.TaskRequest {
	c.mu.Lock()
	defer c.mu.Unlock()
	cs := c.topCostsState()
	if cs == nil {
		return nil
	}
	return c.stampDispatchSnapshotLocked(costsTaskSlice(c.ensureCostsShapeFetched(cs)))
}

// costsRefreshTasks re-dispatches the active/beneath costs screen's own top
// frame — the P5 counterpart to activeListRefreshTasks (C10's retry idiom)
// for a costs screen opened before AWS finished connecting: the pre-connect
// fetch failed with executor.go's "no client configured" error (or was
// never dispatched at all), and ClientsReady landing must recover it
// without user action. Clears a stale pre-connect ErrorMsg unconditionally
// — ensureCostsShapeFetched itself only clears it on a fresh dispatch, and a
// warm shape (nothing to re-fetch) must not keep showing it either. Callers
// must hold c.mu.
func (c *Controller) costsRefreshTasks() []runtime.TaskRequest {
	cs := c.costsStateBeneathOverlay()
	if cs == nil {
		return nil
	}
	cs.ErrorMsg = ""
	return costsTaskSlice(c.ensureCostsShapeFetched(cs))
}

// ForceRefreshCosts implements Ctrl+R on the costs screen (FR-012):
// expires the active shape's cached open period so ensureCostsShapeFetched
// treats it as missing regardless of freshness, then dispatches the
// resulting fetch — closed periods are untouched, refetched only if they
// were already genuinely absent. Passes top.Window straight through:
// ExpireOpenPeriod translates a display window into the NATIVE
// fetch-granularity buckets (e.g. daily for a week display) it actually
// keys entries by internally — the single place that translation happens,
// so no caller (this one included) can forget it.
func (c *Controller) ForceRefreshCosts() []runtime.TaskRequest {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.stampDispatchSnapshotLocked(c.forceRefreshCostsLocked())
}

// forceRefreshCostsLocked is ForceRefreshCosts' body, factored out so
// handleActionRefresh (already running under c.mu, held once by Apply) can
// reach it without a re-entrant Lock. Callers must hold c.mu.
func (c *Controller) forceRefreshCostsLocked() []runtime.TaskRequest {
	cs := c.topCostsState()
	if cs == nil || len(cs.DrillStack) == 0 {
		return nil
	}
	top := cs.DrillStack[len(cs.DrillStack)-1]
	q := costsQueryForFrame(top, cs.Metric)
	cs.Store.ExpireOpenPeriod(q, top.Window, cs.Now)
	return costsTaskSlice(c.ensureCostsShapeFetched(cs))
}

// costsQueryForFrame returns the CE query shape that must be cached/fetched
// to render drill's grid at metric — screen.QueryForFrame is the pure
// implementation (moved there: it touches only costs.DrillLevel/
// costs.Metric, nothing session-scoped); this one-line wrapper is the
// single call site every costs_state.go/costs_body.go consumer keeps using.
func costsQueryForFrame(drill costs.DrillLevel, metric costs.Metric) costs.Query {
	return screen.QueryForFrame(drill, metric)
}

// costsGridForFrame builds the raw aggregated Grid for one drill frame
// against cs's accumulated Store — costs.BuildGrid plus the overlays that
// must happen BEFORE any display-level post-processing: anomaly marks
// (FR-014); for a LINKED_ACCOUNT pivot, the store's cached id->name attrs;
// for a SERVICE pivot, AWS's vendor prefix stripped off every row label
// (the single seam every SERVICE-labelled surface — grid rows, the
// breadcrumb segment naming a SERVICE-pinned frame, the anomaly footer —
// reads through, so none of them can independently drift on when/how the
// strip applies). The display-level post-processing itself (the zero-row
// filter, cursor clamp, viewport slice, empty-finer-grain note) is
// screen.BuildViewModel's now (architecture.md Seam 8) — see
// costsViewModelForFrame, the single source cursor movement, drilling, and
// render all read.
func costsGridForFrame(cs *CostsState, drill costs.DrillLevel) costs.Grid {
	q := costsQueryForFrame(drill, cs.Metric)
	recs, _ := cs.Store.Lookup(q, drill.Window, cs.Now)
	grid := costs.BuildGrid(recs, cs.Metric, drill.Window)
	// BuildGrid has no RowDim parameter (deliberately, to keep its
	// signature stable) — the caller stamps it before any post-processing
	// step (ApplyAnomalies, BuildViewModel's own zero-row filter) that
	// keys off it.
	grid.RowDim = drill.RowDim
	if marks, ok := cs.Store.Anomalies(cs.Now); ok {
		grid = costs.ApplyAnomalies(grid, marks)
	}
	if drill.RowDim == costs.DimensionService {
		for i := range grid.Rows {
			grid.Rows[i].Label = stripCostsServiceVendorPrefix(grid.Rows[i].Label)
		}
	}
	if drill.RowDim == costs.DimensionLinkedAccount {
		grid = costs.ApplyRowAttrs(grid, cs.Store.Attrs())
	}
	return grid
}

// costsViewModelForFrame builds drill's raw Grid (costsGridForFrame) then
// screen.BuildViewModel's clamped view of it — the single source cursor
// movement (applyCostsMoveRow/MoveCol), drilling (applyCostsSelect), and
// render (buildCostsBody) all read, so none of them can independently
// drift on the displayed row set, the clamped cursor, or the visible
// column window (architecture.md Seam 8: "the renderer and Enter handler
// share one clamped view-model"). No adapter-level caching: both
// costs.BuildGrid and screen.BuildViewModel recompute fresh every call —
// cs.Store.Revision() is threaded through only as BuildViewModel's own
// black-box memoization hint, never an adapter-side cache key anymore.
func costsViewModelForFrame(cs *CostsState, drill costs.DrillLevel) (costs.Grid, screen.ViewModel) {
	grid := costsGridForFrame(cs, drill)
	vm := screen.BuildViewModel(grid, drill.RowDim,
		screen.CursorPos{Row: drill.Cursor.Row, Col: drill.Cursor.Col},
		screen.Viewport{Cols: cs.ViewportCols, ScrollX: drill.ScrollX},
		cs.Store.Revision(),
	)
	return grid, vm
}

// costsVisibleColumnRange returns the [start, count) slice of totalCols
// currently in view, given cs.ViewportCols and drill.ScrollX — screen.
// BuildViewModel computes the identical window internally for Rows/
// VisibleCols; this is the small remainder buildCostsBody still needs on
// its own to slice grid.Totals, which BuildViewModel's pure signature does
// not carry (Seam 8 names Rows/Cursor/VisibleCols/Note only).
func costsVisibleColumnRange(cs *CostsState, drill costs.DrillLevel, totalCols int) (start, count int) {
	start, count = 0, totalCols
	if cs.ViewportCols > 0 && cs.ViewportCols < totalCols {
		count = cs.ViewportCols
		start = clampInt(drill.ScrollX, 0, totalCols-count)
	}
	return start, count
}

// ensureCostsShapeFetched checks the store for the query shape backing cs's
// top drill frame at cs.Metric via screen.PlanFetch — the single place
// grid coverage and anomaly freshness are decided, independently of each
// other (architecture.md Seam 1). A grid shape-miss sets Loading and
// dispatches a task covering the whole window (closed periods merge
// idempotently, so re-requesting already-cached ones alongside a genuine
// gap is harmless); a grid-warm shape with a stale/absent anomaly slot
// (X3) still dispatches an anomalies-only task, SkipGrid'd so it never
// re-bills for cost data the store already has, and never touches
// Loading/AwaitedIdentity since nothing the grid render depends on is
// missing. Returns nil (and clears Loading) only when both are fresh.
func (c *Controller) ensureCostsShapeFetched(cs *CostsState) *runtime.TaskRequest {
	top := cs.DrillStack[len(cs.DrillStack)-1]
	q := costsQueryForFrame(top, cs.Metric)
	plan := screen.PlanFetch(cs.Store, q, top.Window, cs.Now)

	if !plan.Grid {
		// The requested shape is fully covered — clear a stale ErrorMsg left
		// by a DIFFERENT shape's earlier failed fetch, so switching to an
		// already-warm shape renders its grid instead of masking it behind
		// the old error. AwaitedIdentity also clears: nothing is in flight
		// for the current top frame's shape.
		cs.Loading = false
		cs.ErrorMsg = ""
		cs.AwaitedIdentity = ""
		if !plan.Anomalies {
			return nil
		}
		return &runtime.TaskRequest{
			Key:     runtime.TaskKey{Kind: runtime.KindFetchCosts, Scope: q.Identity() + "|anomalies-only"},
			Cache:   runtime.CacheNone,
			Payload: runtime.FetchCostsPayload{Query: q, Window: top.Window, SkipGrid: true},
		}
	}
	cs.Loading = true
	cs.ErrorMsg = ""
	// Scope is q.Identity() — the shape+range identity, not a constant
	// "costs" string — so a shape switch mid-flight (e.g. SERVICE -> REGION
	// before SERVICE's fetch completes) gets its own distinct TaskKey rather
	// than colliding with the still-in-flight request and being silently
	// dropped by the web transport's in-flight dedup. AwaitedIdentity mirrors
	// the same scope — the state applyCostsSelect checks to tell "genuinely
	// in flight" apart from "every row display-filtered."
	cs.AwaitedIdentity = q.Identity()
	return &runtime.TaskRequest{
		Key:     runtime.TaskKey{Kind: runtime.KindFetchCosts, Scope: q.Identity()},
		Cache:   runtime.CacheNone,
		Payload: runtime.FetchCostsPayload{Query: q, Window: top.Window, SkipAnomalies: !plan.Anomalies},
	}
}

// reconcileCostsScrollToCursor keeps top.Cursor.Col within the visible
// window [ScrollX, ScrollX+viewportCols) by adjusting top.ScrollX —
// mirrors reconcileDetailScrollToCursor (detail_cursor.go). No-op when
// viewportCols is unset (<=0, renderer hasn't reported a width yet).
func reconcileCostsScrollToCursor(top *costs.DrillLevel, viewportCols int) {
	if viewportCols <= 0 {
		return
	}
	if top.Cursor.Col < top.ScrollX {
		top.ScrollX = top.Cursor.Col
	} else if top.Cursor.Col >= top.ScrollX+viewportCols {
		top.ScrollX = top.Cursor.Col - viewportCols + 1
	}
	maxScroll := max(len(top.Window)-viewportCols, 0)
	top.ScrollX = clampInt(top.ScrollX, 0, maxScroll)
}

// applyCostsMoveRow clamps the top drill frame's cursor row to the live
// grid's row bounds after a ±1 move.
func (c *Controller) applyCostsMoveRow(cs *CostsState, delta int) {
	top := &cs.DrillStack[len(cs.DrillStack)-1]
	_, vm := costsViewModelForFrame(cs, *top)
	top.Cursor.Row = clampInt(top.Cursor.Row+delta, 0, len(vm.Rows)-1)
}

// applyCostsMoveCol clamps the top drill frame's cursor column to the
// window's bounds after a ±1 move (time-axis horizontal scroll, FR-004),
// then reconciles ScrollX so the cursor stays within the renderer's
// visible viewport: scrolling right past the last visible column
// advances the window toward the newest period; scrolling left past the
// first (oldest) loaded column at month granularity extends the window
// with one older month (scroll-to-load) rather than clamping dead, up to
// costs.HistoryHorizonMonths (the same entitlement horizon
// costs.BuildWindow's own Range.Start clamp enforces — single source) —
// returns the resulting KindFetchCosts TaskRequest when the extension
// leaves the new shape uncached, nil otherwise (including every ordinary
// in-window move).
func (c *Controller) applyCostsMoveCol(cs *CostsState, delta int) *runtime.TaskRequest {
	top := &cs.DrillStack[len(cs.DrillStack)-1]
	if delta < 0 && top.Cursor.Col == 0 && top.Granularity == costs.GranularityMonth && len(top.Window) < costs.HistoryHorizonMonths {
		if task := c.extendCostsWindowOlder(cs, top); task != nil {
			return task
		}
	}
	top.Cursor.Col = clampInt(top.Cursor.Col+delta, 0, len(top.Window)-1)
	reconcileCostsScrollToCursor(top, cs.ViewportCols)
	// The zero-row display filter reads only the VISIBLE cells (Seam 8),
	// so a horizontal scroll can change which rows are filtered even
	// though the row set itself never moved — re-clamp Cursor.Row against
	// the ViewModel built at the new scroll position, the same single
	// source applyCostsSelect and buildCostsBody read.
	_, vm := costsViewModelForFrame(cs, *top)
	top.Cursor.Row = clampInt(top.Cursor.Row, 0, len(vm.Rows)-1)
	return nil
}

// extendCostsWindowOlder prepends one older month to top.Window, lands the
// cursor on it, and dispatches the fetch for the extended range — nil when
// the oldest column's Start does not parse (should not happen; BuildWindow
// always produces "YYYY-MM-01").
func (c *Controller) extendCostsWindowOlder(cs *CostsState, top *costs.DrillLevel) *runtime.TaskRequest {
	oldestStart, err := costs.ParseDate(top.Window[0].Start)
	if err != nil {
		return nil
	}
	newStart := oldestStart.AddDate(0, -1, 0)
	top.Window = append([]costs.Period{{Start: costs.FormatDate(newStart), End: top.Window[0].Start}}, top.Window...)
	top.Cursor.Col = 0
	top.ScrollX = 0
	reconcileCostsScrollToCursor(top, cs.ViewportCols)
	return c.ensureCostsShapeFetched(cs)
}

// applyCostZoom walks the granularity chain in/out, mutating the top frame
// in place (never pushes a drill frame — FR-005). Boundary steps (in at
// day, out at year) are no-ops. Week/day are bounded, drill-style windows
// anchored on the cursor's current period (zooming IN on a specific month
// must show that month's weeks). Month/year are always the canonical
// trailing window ending at cs.Now (Granularity.TrailingAnchored) — never
// re-anchored on the cursor — so zooming back out can never walk the
// window before any period this session has ever fetched: the
// restored window is byte-identical to the one EnsureCostsState/digit-0
// already build, which by construction never precedes the earliest column
// the app has ever shown.
//
// Returns the KindFetchCosts TaskRequest when the new granularity's query
// shape is not yet cached (FR-017 shape-miss), nil otherwise.
func (c *Controller) applyCostZoom(cs *CostsState, in bool) *runtime.TaskRequest {
	top := &cs.DrillStack[len(cs.DrillStack)-1]
	idx := granChainIndex(top.Granularity)
	if in {
		if idx >= len(granularityChain)-1 {
			return nil
		}
		idx++
	} else {
		if idx <= 0 {
			return nil
		}
		idx--
	}
	newGran := granularityChain[idx]

	var newWindow []costs.Period
	if newGran.TrailingAnchored() {
		newWindow = costs.BuildWindow(newGran, cs.Now)
	} else {
		anchor := cs.Now
		if top.Cursor.Col >= 0 && top.Cursor.Col < len(top.Window) {
			if t, err := costs.ParseDate(top.Window[top.Cursor.Col].Start); err == nil {
				anchor = t
			}
		}
		newWindow = costs.BuildWindow(newGran, anchor)
	}

	// A RESOURCE_ID top frame's zoom-out is a boundary no-op, same shape as
	// the idx<=0 check above — see resourceDrillWindowExceedsRetention's doc
	// comment below for why. Nothing is mutated on this path.
	if !in && top.RowDim == costs.DimensionResourceID && resourceDrillWindowExceedsRetention(newWindow) {
		return nil
	}

	top.Granularity = newGran
	top.Window = newWindow
	// FR-002 "open at today": zooming back out to a trailing-anchored
	// (month/year) window re-lands on the newest period, matching the
	// default view's own cursor placement — not on column 0, which would
	// silently jump the cursor to the oldest month every time. A zoom-IN
	// (or an out-zoom that stays bounded, week/day) keeps landing at the
	// start of the newly bounded window, since that IS the period being
	// drilled into.
	if !in && newGran.TrailingAnchored() {
		top.Cursor.Col = len(newWindow) - 1
	} else {
		top.Cursor.Col = 0
	}
	top.ScrollX = 0
	reconcileCostsScrollToCursor(top, cs.ViewportCols)
	return c.ensureCostsShapeFetched(cs)
}

// resourceDrillWindowExceedsRetention reports whether window's overall span
// (its first period's Start to its last period's End) is wider than the CE
// resource-level retention bound. costs.ClampResourceDrillWindow only drops
// periods whose Start predates the cutoff — it cannot by itself flag a
// window like a RESOURCE_ID zoom-out's full-month Week tiling, whose Start
// can legitimately sit exactly at the cutoff while its End still runs many
// days past it (into days with no retained cost data, or into the future).
func resourceDrillWindowExceedsRetention(window []costs.Period) bool {
	if len(window) == 0 {
		return false
	}
	start, errS := costs.ParseDate(window[0].Start)
	end, errE := costs.ParseDate(window[len(window)-1].End)
	if errS != nil || errE != nil {
		return false
	}
	return end.Sub(start).Hours()/24 > costs.ResourceDrillWindowRetentionDays
}

func granChainIndex(g costs.Granularity) int {
	if i := slices.Index(granularityChain, g); i >= 0 {
		return i
	}
	return 1 // month is the default
}

// applyCostMetricCycle advances the display metric to the next entry in
// metricCycle, wrapping back to invoice, and ensures the resulting query
// shape (unblended is distinct; the other four share the base shape) is
// fetched — see ensureCostsShapeFetched.
func (c *Controller) applyCostMetricCycle(cs *CostsState) *runtime.TaskRequest {
	idx := max(slices.Index(metricCycle, cs.Metric), 0)
	cs.Metric = metricCycle[(idx+1)%len(metricCycle)]
	// X6: clamp the cursor row against the NEW metric's filtered ViewModel
	// right here, at mutation time — a metric switch can shrink the
	// visible row set (a row's display metric may have no value under the
	// new metric), and a stale Cursor.Row left for applyCostsSelect to
	// discover later resolves to an empty row and reads as "nothing to
	// select" instead of the clamped (last visible) row the user actually
	// sees highlighted.
	top := &cs.DrillStack[len(cs.DrillStack)-1]
	_, vm := costsViewModelForFrame(cs, *top)
	top.Cursor.Row = clampInt(top.Cursor.Row, 0, len(vm.Rows)-1)
	return c.ensureCostsShapeFetched(cs)
}

// applyCostPivot handles digit keys 0-9. 1-6 mutate the top frame's RowDim
// in place (keeping any accumulated Filter/Window — a re-pivot of the
// current scope, not a reset) and reset its cursor to the top-left cell,
// per US2 acceptance #1 ("same columns... cursor reset to top-left").
// 0 resets to the default view: root frame, service pivot, invoice metric,
// every drill popped (data-model.md's pivot-preset table). 7-9 are reserved
// no-ops.
//
// Returns the KindFetchCosts TaskRequest when the new pivot's GroupBy shape
// is not yet cached (FR-017 shape-miss), nil otherwise (including the 7-9
// no-op case).
func (c *Controller) applyCostPivot(cs *CostsState, n int) *runtime.TaskRequest {
	switch n {
	case 0:
		window := costs.BuildWindow(costs.GranularityMonth, cs.Now)
		cs.DrillStack = []costs.DrillLevel{costsOpenAtNewestDrillLevel(
			costs.DimensionService, costs.Filter{}, costs.GranularityMonth, window,
		)}
		cs.Metric = costs.MetricInvoice
		reconcileCostsScrollToCursor(&cs.DrillStack[0], cs.ViewportCols)
		return c.ensureCostsShapeFetched(cs)
	case 1, 2, 3, 4, 5, 6:
		// X12: only Row resets (a new pivot dimension's rows are unrelated
		// to the old ones); Col is PRESERVED, merely clamped to the
		// window's own bounds (unchanged by a pivot) — resetting it to 0
		// used to yank the view to the oldest column and poison the next
		// drill's anchor.
		top := &cs.DrillStack[len(cs.DrillStack)-1]
		top.RowDim = pivotDigitDims[n]
		top.Cursor.Row = 0
		top.Cursor.Col = clampInt(top.Cursor.Col, 0, len(top.Window)-1)
		reconcileCostsScrollToCursor(top, cs.ViewportCols)
		return c.ensureCostsShapeFetched(cs)
	default:
		// 7-9 reserved — no-op.
		return nil
	}
}

// pivotDigitDims is the digit-key-to-dimension table for applyCostPivot's
// 1-6 cases, index 0 unused (digit 0 resets to the default view instead of
// pivoting).
var pivotDigitDims = [7]costs.Dimension{
	1: costs.DimensionService,
	2: costs.DimensionRegion,
	3: costs.DimensionLinkedAccount,
	4: costs.DimensionUsageType,
	5: costs.DimensionPurchaseType,
	6: costs.DimensionRecordType,
}

// applyCostsBack pops one drill frame, restoring the parent frame's cursor,
// scroll, and RowDim exactly (they were never mutated by the child drill).
// Returns false when already at the root frame — the caller falls through
// to the generic screen-pop (Esc leaves the Cost Explorer entirely).
//
// CostsState-wide fields (Loading/AwaitedIdentity/ErrorMsg/
// DrillRefusedReason/ResourceRowNote) describe the just-popped CHILD frame's
// own in-flight/failed fetch, never the parent's — the parent's own data is
// already available (Select never drills from a Loading frame), so it must
// never render blocked behind state a frame it no longer owns left behind.
// Cleared unconditionally on every actual pop; a caller that wants to set
// one of these fresh (e.g. ApplyCostsLoaded's own classified-refusal
// auto-pop) must do so AFTER calling this, not before.
func (c *Controller) applyCostsBack(cs *CostsState) bool {
	if len(cs.DrillStack) <= 1 {
		return false
	}
	cs.DrillStack = cs.DrillStack[:len(cs.DrillStack)-1]
	cs.Loading = false
	cs.AwaitedIdentity = ""
	cs.ErrorMsg = ""
	cs.DrillRefusedReason = ""
	cs.ResourceRowNote = ""
	return true
}

// costsResourceRowTargetType returns the a9s resource short name whose
// catalog entry's CostExplorerServiceName matches the RESOURCE_ID frame's
// pinned SERVICE dimension value, or "" when no registered type maps to
// that service — the catalog (resource.AllResourceTypes) is the single
// source of the SERVICE-to-type mapping, so a type can never drift between
// its own catalog entry and what costs navigation believes it maps to.
func costsResourceRowTargetType(filter costs.Filter) string {
	vals := filter.Equals[costs.DimensionService]
	if len(vals) != 1 {
		return ""
	}
	svc := vals[0]
	for _, def := range resource.AllResourceTypes() {
		if def.CostExplorerServiceName == svc {
			return def.ShortName
		}
	}
	return ""
}

// applyCostsSelect drills into the cursor's (row, period) cell: narrows the
// filter to the row's dimension value, pivots to the next dimension in the
// chain (account/region -> service -> usage type -> resource, FR-006), and
// re-renders columns one granularity finer within the selected period. At
// the bottom of the chain (RESOURCE_ID) it navigates to the mapped a9s
// resource detail view, or sets ResourceRowNote for a service with none
// (FR-008); it also no-ops when the resource-level drill's window/filter
// gate (FR-007) is not satisfied. The decision itself is
// screen.Select's — this is a thin switch over its typed outcome
// (architecture.md Seam 3): Loading gates unconditionally (X7), a stale
// cursor beyond the live grid's row count is a strict no-op
// (screen.NoSelection), never a blind empty-value pin.
//
// Returns the KindFetchCosts TaskRequest when the new child frame's query
// shape is not yet cached (FR-017 shape-miss), nil otherwise (including
// every no-op path).
func (c *Controller) applyCostsSelect(cs *CostsState) *runtime.TaskRequest {
	// Cleared on every attempt so a stale note/refusal reason never outlives
	// the Enter press that produced it — the cases below set one fresh when
	// applicable.
	cs.DrillRefusedReason = ""
	cs.ResourceRowNote = ""

	cur := &cs.DrillStack[len(cs.DrillStack)-1]
	if cur.Cursor.Col < 0 || cur.Cursor.Col >= len(cur.Window) {
		return nil
	}

	// Seam 8: the ViewModel's own Cursor.Row is always a valid index into
	// Rows when Rows is non-empty (screen.BuildViewModel's clamp) — reading
	// through it here, rather than re-checking cur.Cursor.Row's raw bounds,
	// means a stale cursor left over from ANY prior mutation resolves to
	// the clamped (last visible) row instead of screen.NoSelection, not just
	// the specific mutation points that call costsViewModelForFrame of
	// their own accord (X6's principle applied uniformly at this seam).
	_, vm := costsViewModelForFrame(cs, *cur)
	row := screen.GridRowRef{}
	if len(vm.Rows) > 0 {
		cells := vm.Rows[vm.Cursor.Row].Cells
		nonZero := vm.Cursor.Col >= 0 && vm.Cursor.Col < len(cells) && cells[vm.Cursor.Col].Amount.Value != 0
		row = screen.GridRowRef{Present: true, Value: vm.Rows[vm.Cursor.Row].Key, CellNonZero: nonZero}
	}
	cell := screen.PeriodRef{Period: cur.Window[cur.Cursor.Col]}

	q := costsQueryForFrame(*cur, cs.Metric)
	// Loading is state-derived, not a fresh coverage recompute: AwaitedIdentity
	// is set the moment ensureCostsShapeFetched dispatches this exact frame's
	// query and cleared the moment a MATCHING CostsLoaded delivery lands
	// (ApplyCostsLoaded), regardless of whether every window period ends up
	// individually re-verified as covered — the same lenient "the fetch this
	// frame is waiting on has resolved" signal every other action already
	// relies on. X7's fix is that this check is no longer skipped for a
	// frame the user just pushed: PushDrill below leaves AwaitedIdentity set
	// exactly as ensureCostsShapeFetched left it, so a fast second Enter on a
	// still-loading fresh frame reads Loading true here too.
	state := screen.ScreenState{
		RowDim:          cur.RowDim,
		Path:            screen.DrillPath{Pinned: costsAncestorRowDims(cs.DrillStack)},
		Filter:          cur.Filter,
		Granularity:     cur.Granularity,
		Loading:         cs.AwaitedIdentity != "" && q.Identity() == cs.AwaitedIdentity,
		Now:             cs.Now,
		ResourceTypeFor: costsResourceTypeForService,
	}

	switch out := screen.Select(state, row, cell).(type) {
	case screen.NoSelection:
		return nil
	case screen.Refuse:
		cs.DrillRefusedReason = out.Reason
		return nil
	case screen.OpenResource:
		// The SAME by-ID placeholder constructor the related panel's own
		// single-target drill uses (pushByIDPlaceholderList, list_state.go):
		// pushes a placeholder ScreenResourceList flagged AutoOpenSingle so
		// autoOpenSingleDetail (handle.go) replaces it with the real detail
		// once the KindFetchByIDDetail delivery lands — reachable from every
		// caller of Controller.Handle (web/headless), not only a
		// screen-specific adapter special case. out.Locator.Type always has a
		// FetchByIDs helper registered (costsResourceTypeForService's own
		// gate below), so the constructor always applies the by-ID flags.
		c.pushByIDPlaceholderList(out.Locator.Type, out.Locator.ID)
		return &runtime.TaskRequest{
			Key:     runtime.TaskKey{Kind: runtime.KindFetchByIDDetail, Scope: out.Locator.Type},
			Cache:   runtime.CacheNone,
			Payload: runtime.FetchByIDDetailPayload{TargetType: out.Locator.Type, ID: out.Locator.ID},
		}
	case screen.PushDrill:
		pinned := cloneCostsFilter(cur.Filter)
		pinned.Equals[cur.RowDim] = []string{out.Value}
		cs.DrillStack = append(cs.DrillStack, costs.DrillLevel{
			RowDim: out.Dim,
			Filter: pinned,
			// out.Granularity is the SAME finerGranularity(cur.Granularity)
			// step Select already computed internally to build out.Window
			// via WindowWithin — consuming it here (rather than an
			// app-layer finerGranularity copy re-deriving it) means Window
			// and Granularity can never describe two different chains.
			Granularity: out.Granularity,
			Window:      out.Window,
			// N3: the granularity-fallback gate's inputs, carried straight
			// through from Select's own PushDrill outcome — Eligible only
			// when the parent's selected cell was non-zero, so an
			// ordinarily-empty drill never fires it.
			Fallback: costs.FallbackGate{
				Eligible:          out.ParentCellNonZero,
				ParentGranularity: out.ParentGranularity,
				SelectedPeriod:    out.SelectedPeriod,
			},
		})
		// Open the child on the current column — the newest whose period has
		// already begun (FR-002 "open at today"), like the root/pivot frames.
		// The col-0 (oldest) default parks on a cell outside the RESOURCE_ID
		// 14-day retention window late in the month; the raw last column is
		// wrong too, because week/day child windows — unlike months, which
		// monthsWithinPeriod future-clamps — tile the whole parent period, so
		// mid-month their last column is an empty FUTURE week/day. reconcile
		// pins the chosen column to the visible edge.
		top := &cs.DrillStack[len(cs.DrillStack)-1]
		top.Cursor.Col = costsCurrentCol(top.Window, cs.Now)
		reconcileCostsScrollToCursor(top, cs.ViewportCols)
		// X7: AwaitedIdentity is left exactly as ensureCostsShapeFetched set
		// it for the new child frame (set when genuinely a shape-miss, ""
		// when the child's shape happens to already be cached) — no longer
		// force-cleared to permit blind chaining; a fast second Enter on
		// this still-loading frame reads Loading true via the same check
		// above.
		return c.ensureCostsShapeFetched(cs)
	default:
		return nil
	}
}

// costsAncestorRowDims returns the RowDim of every frame BELOW the top of
// stack — the screen.DrillPath.Pinned ancestry Select needs: each pushed
// frame's own RowDim becomes "pinned" the moment a child frame descends
// past it.
func costsAncestorRowDims(stack []costs.DrillLevel) []costs.Dimension {
	if len(stack) <= 1 {
		return nil
	}
	out := make([]costs.Dimension, len(stack)-1)
	for i := 0; i < len(stack)-1; i++ {
		out[i] = stack[i].RowDim
	}
	return out
}

// costsResourceTypeForService is the screen.ScreenState.ResourceTypeFor
// callback: the a9s catalog mapping (costsResourceRowTargetType) AND
// FetchByIDs registration folded into one check, so a service resolving
// to a shortName with no FetchByIDs registered refuses exactly like an
// unmapped service — screen package cannot import core/resource, so
// this closure is injected instead.
func costsResourceTypeForService(service string) (string, bool) {
	shortName := costsResourceRowTargetType(costs.Filter{Equals: map[costs.Dimension][]string{
		costs.DimensionService: {service},
	}})
	if shortName == "" || resource.GetFetchByIDs(shortName) == nil {
		return "", false
	}
	return shortName, true
}

func cloneCostsFilter(f costs.Filter) costs.Filter {
	out := costs.Filter{
		Equals:    make(map[costs.Dimension][]string, len(f.Equals)+1),
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

// isResourceDrillQuery reports whether q is the RESOURCE_ID-shaped query a
// resource-level drill dispatches (GetCostAndUsageWithResources) — the only
// shape a classified refusal error is downgraded from a blocking ErrorMsg to
// an FR-007 FooterNote refusal for.
func isResourceDrillQuery(q costs.Query) bool {
	return len(q.GroupBy) == 1 && q.GroupBy[0] == costs.DimensionResourceID
}

// isClassifiedResourceDrillRefusal reports whether err is one of the
// specific classified CE sentinels a resource-level drill can legitimately
// hit as an honest, expected outcome rather than a fetch failure:
// ErrCostsAccessDenied (the real-account shape — CE's resource-level opt-in
// refusal arrives as AccessDeniedException, "Resource-level data
// granularity is an opt-in only feature...") and ErrCostsDataUnavailable.
func isClassifiedResourceDrillRefusal(err error) bool {
	return errors.Is(err, awsclient.ErrCostsAccessDenied) || errors.Is(err, awsclient.ErrCostsDataUnavailable)
}

// costsResourceDrillRefusalNote builds the FooterNote text for a classified
// resource-level opt-in refusal (FR-007-shaped: an honest, human-readable
// explanation, not a silent no-op). Extracts the underlying API error
// MESSAGE via errors.As(err, *smithy.APIError) — the same clean
// extraction core/aws/errors.go's ClassifyAWSError already does —
// rather than %v-ing the full chain, whose "operation error ..."/"https
// response error ..." wrapper prefixes would otherwise consume the footer
// line and truncate the actionable AWS text away before it is ever seen.
// Falls back to err.Error() only when no APIError is in the chain, so a
// non-AWS failure still gets an honest (if unclassified) message.
func costsResourceDrillRefusalNote(err error) string {
	msg := err.Error()
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		msg = apiErr.ErrorMessage()
	}
	return fmt.Sprintf("resource-level cost data unavailable for this account: %s", msg)
}

// ApplyCostsLoaded merges one Cost Explorer fetch result into the costs
// screen's Store + session counters (FR-013: APICalls increments by
// Requests even on error, and Store.Merge/MergeCoverage/MergeAttrs run
// unconditionally on success, regardless of match — a delivery is never
// silently dropped, even one landing while Help/Identity is stacked above
// the costs screen, or after the costs screen has been popped off the
// stack entirely — see the cs==nil branch below). Loading/ErrorMsg,
// however, resolve ONLY when ev.Query matches the CURRENTLY awaited
// top-frame query by BOTH CacheKey and Range: the top frame can move on to
// a different shape or a different period range at the SAME shape (e.g.
// zooming from May to June at week granularity — identical CacheKey,
// different Range) before an earlier fetch's result arrives, and that
// now-stale-from-the-controller's-perspective delivery must not clear
// Loading or install an error for what the user is actually waiting on —
// only the result that matches both does. On a matching error it
// sets ErrorMsg so buildCostsBody renders the explicit error state
// (FR-017) instead of an empty grid; on a matching success it clears any
// prior ErrorMsg. DataThrough is not tracked here at all — buildCostsBody
// derives it fresh from cs.Store's own cached records for the current
// shape (costs.Store.DataThrough), so a warm disk cache re-entered with
// zero fetches this session still shows a correct value.
// ApplyCostsLoaded's return is the granularity-fallback re-fetch (N3) it
// may itself dispatch when this delivery lands with zero records for a
// frame whose selected parent cell was non-zero — nil on every other path.
// Handle appends it to the tasks it returns, mirroring every other
// controller action.
func (c *Controller) ApplyCostsLoaded(ev messages.CostsLoaded) *runtime.TaskRequest {
	cs := c.costsStateBeneathOverlay()
	if cs == nil {
		// The costs screen was fully popped before this delivery arrived —
		// costsStateBeneathOverlay finds no ScreenCosts entry anywhere on
		// the stack, so there is no CostsState left to mutate. The fetch was
		// still genuinely billed and its data real (FR-013/FR-017: billed CE
		// data is never silently discarded) — merge it into a fresh
		// disk-backed Store for this profile instead, mirroring every other
		// on-disk cache's NoCache gate (R4): under NoCache there is no
		// screen state left to apply to AND no disk cache to touch, so this
		// is a pure no-op rather than a stray read+write. now falls back to
		// lastCostsNow (the clock this session's costs screen was last
		// seeded with) rather than a fresh read, so this merge's FetchedAt
		// stays consistent with whatever clock the rest of the session used —
		// significant for a fixed injected-clock test, harmless in production.
		// The zero-value fallback goes through the package seam like every
		// other clock read here.
		if ev.Err != nil || c.core.NoCache() {
			return nil
		}
		store := costs.LoadStore(c.core.Profile())
		now := c.lastCostsNow
		if now.IsZero() {
			now = Now()
		}
		store.ApplyFetchResult(costs.FetchResult{
			Query:     ev.Query,
			Records:   ev.Grid.Records,
			Attrs:     ev.Attrs,
			Anomalies: costsAnomalyResultFromEvent(ev),
			Requests:  ev.Requests,
		}, now)
		if len(ev.Window) > 0 && ev.Grid.Fetched {
			store.MergeCoverage(ev.Query, ev.Window, now)
		}
		c.costsDirtyStore = store
		return nil
	}
	cs.APICalls += ev.Requests

	matchesAwaited := false
	if len(cs.DrillStack) > 0 {
		top := cs.DrillStack[len(cs.DrillStack)-1]
		awaited := costsQueryForFrame(top, cs.Metric)
		// A zero-value ev.Query.Range means the delivery carries no range
		// scope at all — every real fetch dispatch stamps one via
		// costsQueryForFrame (whenever its drill.Window is non-empty), so
		// this only ever matches on shape alone for a range-agnostic
		// delivery, never masking a genuine different-range mismatch.
		// Otherwise Query.Identity() (CacheKey+Range) is THE match — the
		// same identity ensureCostsShapeFetched stamps onto the TaskKey that
		// requested this very delivery.
		matchesAwaited = awaited.CacheKey() == ev.Query.CacheKey() &&
			(ev.Query.Range == (costs.Period{}) || awaited.Identity() == ev.Query.Identity())
	}

	if ev.Err != nil {
		if matchesAwaited {
			cs.Loading = false
			cs.AwaitedIdentity = ""
			if isResourceDrillQuery(ev.Query) && isClassifiedResourceDrillRefusal(ev.Err) {
				// A classified resource-level opt-in refusal (CE requires
				// explicit per-account enrollment for
				// GetCostAndUsageWithResources) is an honest, expected
				// outcome of attempting the drill — not a fetch failure that
				// should blank the whole grid behind the blocking ErrorMsg
				// body. It surfaces as the same FR-007 FooterNote seam
				// DrillRefusedReason already feeds, and the just-pushed
				// RESOURCE_ID frame is popped back to its parent so the
				// screen never shows an empty child frame with nothing to
				// display. Every other error keeps the blocking ErrorMsg. Pop
				// FIRST, set the reason AFTER — applyCostsBack itself clears
				// DrillRefusedReason (P3: a popped child frame's state must
				// never leak onto its parent), so setting the note before the
				// pop would have it wiped by the very call that surfaces it.
				c.applyCostsBack(cs)
				cs.DrillRefusedReason = costsResourceDrillRefusalNote(ev.Err)
			} else {
				cs.ErrorMsg = ev.Err.Error()
			}
		}
		return nil
	}
	if matchesAwaited {
		cs.Loading = false
		cs.ErrorMsg = ""
		cs.AwaitedIdentity = ""
	}

	cs.Store.ApplyFetchResult(costs.FetchResult{
		Query:     ev.Query,
		Records:   ev.Grid.Records,
		Attrs:     ev.Attrs,
		Anomalies: costsAnomalyResultFromEvent(ev),
		Requests:  ev.Requests,
	}, cs.Now)
	if len(ev.Window) > 0 && ev.Grid.Fetched {
		cs.Store.MergeCoverage(ev.Query, ev.Window, cs.Now)
	}

	// Persist beyond this screen's lifetime — otherwise a fetch merged into
	// the in-memory Store is lost the moment the costs screen is left and
	// re-entered (LoadStore reads only what Save last wrote). Marked dirty
	// here rather than saved synchronously — Handle flushes it once c.mu is
	// released, so the disk write never blocks a concurrent action against
	// the controller. A write failure is surfaced as a flash, never a panic
	// and never silently dropped data. R4: skipped entirely under NoCache —
	// a memory-only Store (ensureCostsState's own NoCache branch) must
	// never reach a disk write.
	if !c.core.NoCache() {
		c.costsDirtyStore = cs.Store
	}

	return c.applyCostsGranularityFallback(cs, ev, matchesAwaited)
}

// applyCostsGranularityFallback is N3's re-plan trigger: when this delivery
// matches what the top frame was waiting on, genuinely carried zero
// records, and the top frame's own FallbackGate is eligible (the parent
// drill step's selected cell was non-zero) and hasn't already fired once,
// the frame is re-planned in place at its own parent granularity/period —
// mutating Granularity/Window/Cursor/ScrollX exactly like a zoom, never
// pushing a new frame — and the resulting coarser fetch is dispatched.
// Fired is set unconditionally the moment this fires, so a second empty
// delivery for the same frame can never chain a further re-plan.
func (c *Controller) applyCostsGranularityFallback(cs *CostsState, ev messages.CostsLoaded, matchesAwaited bool) *runtime.TaskRequest {
	if !matchesAwaited || !ev.Grid.Fetched || len(ev.Grid.Records) != 0 || len(cs.DrillStack) == 0 {
		return nil
	}
	top := &cs.DrillStack[len(cs.DrillStack)-1]
	if !top.Fallback.Eligible || top.Fallback.Fired {
		return nil
	}
	// The re-planned window starts from the parent step's own selected
	// period — for a RESOURCE_ID-shaped frame (top.RowDim), that period must
	// be re-clamped against the 14-day resource-drill retention cutoff
	// exactly as the original PushDrill clamped it (screen.go's own
	// ClampResourceDrillWindow call): the parent cell being non-zero
	// (Fallback.Eligible) says nothing about whether it still falls inside
	// the cutoff, since a single-period clamp is all-or-nothing while the
	// finer window this frame originally drilled from could still partially
	// survive.
	window := []costs.Period{top.Fallback.SelectedPeriod}
	if top.RowDim == costs.DimensionResourceID {
		window = costs.ClampResourceDrillWindow(window, cs.Now)
	}
	top.Fallback.Fired = true
	if len(window) == 0 {
		return nil
	}
	top.Granularity = top.Fallback.ParentGranularity
	top.Window = window
	top.Cursor.Col = 0
	top.ScrollX = 0
	return c.ensureCostsShapeFetched(cs)
}

// costsAnomalyResultFromEvent derives the AnomalyResult
// (*costs.Store).ApplyFetchResult needs from ev.Anomalies alone: a non-nil
// slice (even empty) is an authoritative fetch result — the executor always
// forces a non-nil slice on a genuine GetAnomalies success, empty or not
// (executor.go) — so Requested is true; nil covers everything that must
// preserve the cached marks/TTL untouched (a SkipAnomalies delivery, or one
// whose anomaly fetch errored), which need no separate skip/error flag on
// the wire message since both want the identical "preserve" outcome.
func costsAnomalyResultFromEvent(ev messages.CostsLoaded) costs.AnomalyResult {
	return costs.AnomalyResult{Requested: ev.Anomalies != nil, Marks: ev.Anomalies}
}

func clampInt(v, lo, hi int) int {
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
