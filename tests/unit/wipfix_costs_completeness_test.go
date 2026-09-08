package unit_test

// wipfix_costs_completeness_test.go — rows 1 and 7: a partial Cost Explorer
// answer must stay visibly partial.
//
// Row 1: a grid fetch cut by the CE pagination cap merges its partial records
// into the store and stamps the whole window covered, while the "partial data"
// warning lives only on the active drill frame. Leave the screen and the
// warning is gone; the periods that had closed by then are cached as
// authoritative and a refresh cannot repair them. Completeness has to travel
// with the data into the store and into its coverage decision.
//
// Row 7: a page-capped GetAnomalies result is dropped outright by
// ApplyFetchResult, so confirmed anomalies vanish from the grid with nothing
// said. A visibly incomplete overlay is worth keeping — separately from the
// authoritative snapshot the cache serves.

import (
	"strings"
	"testing"
	"time"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/costs"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// wipfixCostsProfile is the profile newTestController runs under; a costs
// store seeded for it is the one EnsureCostsState will load from disk.
const wipfixCostsProfile = "demo"

// wipfixSeededCostsController seeds the on-disk cost store via seed, then
// opens the costs screen over it — exactly the shape of re-entering the
// screen in a later session, where nothing but the cache remembers what the
// last fetch was allowed to see. The controller is built first because it is
// what points the cache root at this test's own temp dir.
func wipfixSeededCostsController(t *testing.T, now time.Time, seed func(*costs.Store)) *app.Controller {
	t.Helper()
	c := newTestController(t)
	store := costs.LoadStore(wipfixCostsProfile)
	seed(store)
	if err := store.Save(); err != nil {
		t.Fatalf("seeding on-disk store: %v", err)
	}
	c.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenCosts}})
	c.EnsureCostsState(now)
	return c
}

// TestCostsStore_TruncatedGridStillWarnsAfterReopen pins row 1's first half:
// the warning about partial dollars survives leaving and re-entering the
// screen, because the store knows the window it is serving was cut short.
func TestCostsStore_TruncatedGridStillWarnsAfterReopen(t *testing.T) {
	window := costs.BuildWindow(costs.GranularityMonth, fixedCostsNow)
	c := wipfixSeededCostsController(t, fixedCostsNow, func(store *costs.Store) {
		var recs []costs.Record
		for _, p := range window {
			recs = append(recs, fullMetricRecord(p, "Amazon EC2", 100))
		}
		store.ApplyFetchResult(costs.FetchResult{
			Query:     baseServiceQuery(),
			Records:   recs,
			Coverage:  window,
			Truncated: true,
		}, fixedCostsNow)
	})
	note := c.Snapshot().Body.Costs.FooterNote
	if !strings.Contains(note, "partial") {
		t.Errorf("footer note after reopening on a page-capped window = %q, want a partial-data warning — the totals on screen are a lower bound and the store is the only thing that still knows it", note)
	}
}

// TestCostsStore_TruncatedPeriodIsNeverImmutable pins row 1's second half: a
// bucket whose fetch was cut short must stay refetchable, so a later complete
// fetch replaces it. Without it, a period that had already closed when the
// capped fetch landed is cached as authoritative forever.
func TestCostsStore_TruncatedPeriodIsNeverImmutable(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	q := baseServiceQuery()
	// A period that closed long before either fetch: settlementLag has
	// elapsed, so a COMPLETE fetch of it would be immutable.
	closed := costs.Period{Start: "2026-01-01", End: "2026-02-01"}
	capped := fixedCostsNow.Add(-48 * time.Hour)

	store := costs.LoadStore("wipfix-truncated-immutable")
	store.ApplyFetchResult(costs.FetchResult{
		Query:     q,
		Records:   []costs.Record{fullMetricRecord(closed, "Amazon EC2", 100)},
		Coverage:  []costs.Period{closed},
		Truncated: true,
	}, capped)

	// The repair: the same window, fetched whole.
	store.ApplyFetchResult(costs.FetchResult{
		Query:    q,
		Records:  []costs.Record{fullMetricRecord(closed, "Amazon EC2", 900)},
		Coverage: []costs.Period{closed},
	}, fixedCostsNow)

	recs, missing := store.Lookup(q, []costs.Period{closed}, fixedCostsNow)
	if len(missing) != 0 {
		t.Fatalf("period reported missing after the repair fetch: %+v", missing)
	}
	var got float64
	for _, r := range recs {
		if len(r.Keys) == 1 && r.Keys[0] == "Amazon EC2" {
			got = r.Metrics[costs.MetricInvoice].Value
		}
	}
	if got != 900 {
		t.Errorf("invoice after the repair fetch = %v, want 900 — a bucket written from a page-capped fetch must never be promoted to immutable, or the refresh has nothing it can fix", got)
	}
	if store.Partial(q, []costs.Period{closed}, fixedCostsNow) {
		t.Errorf("the window is still reported partial after a complete fetch replaced it")
	}
}

// TestCostsStore_PartialAnomalyOverlayStillRenders pins row 7: a page-capped
// GetAnomalies result keeps its marks on the grid, with the warning that says
// they are a lower bound — and leaves the authoritative cached snapshot alone.
func TestCostsStore_PartialAnomalyOverlayStillRenders(t *testing.T) {
	const service = "Amazon Elastic Compute Cloud - Compute"
	c := newCostsController(t, fixedCostsNow)
	q := baseServiceQuery()
	root := topDrill(t, c)
	curPeriod := root.Window[len(root.Window)-1]

	c.Handle(messages.CostsLoaded{
		Query: q,
		Grid:  costs.GridResult{Fetched: true, Records: []costs.Record{monthRecord(fixedCostsNow, service, 1200.0)}},
		Anomalies: []costs.AnomalyMark{{
			ID:        "anomaly-1",
			Impact:    costs.Amount{Value: 500, Unit: "USD"},
			Period:    curPeriod,
			Dimension: map[costs.Dimension]string{costs.DimensionService: service},
		}},
		AnomaliesTruncated: true,
		Window:             root.Window,
		Requests:           1,
	})

	body := c.Snapshot().Body.Costs
	if body == nil {
		t.Fatal("no costs body")
	}
	marked := false
	for _, row := range body.Rows {
		for _, cell := range row.Cells {
			if cell.Anomaly {
				marked = true
			}
		}
	}
	if !marked {
		t.Errorf("no grid cell carries the anomaly overlay — a confirmed anomaly must not vanish just because the walk that found it was cut short (rows=%d)", len(body.Rows))
	}
	if !strings.Contains(body.FooterNote, "partial") {
		t.Errorf("footer note = %q, want a note saying the anomaly overlay is a lower bound — an overlay that is knowingly incomplete has to say so", body.FooterNote)
	}
}

// TestCostsStore_PartialAnomalyOverlayLeavesAuthoritativeCacheAlone is row 7's
// other half: keeping the incomplete overlay must not promote it to the
// snapshot the cache serves as CE's own complete list for the window.
func TestCostsStore_PartialAnomalyOverlayLeavesAuthoritativeCacheAlone(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	q := baseServiceQuery()
	window := costs.BuildWindow(costs.GranularityMonth, fixedCostsNow)
	newest := window[len(window)-1]
	mark := costs.AnomalyMark{
		ID:        "anomaly-1",
		Impact:    costs.Amount{Value: 420, Unit: "USD"},
		Period:    newest,
		Dimension: map[costs.Dimension]string{costs.DimensionService: "Amazon EC2"},
	}

	store := costs.LoadStore("wipfix-partial-anomaly-cache")
	store.ApplyFetchResult(costs.FetchResult{
		Query:     q,
		Records:   []costs.Record{fullMetricRecord(newest, "Amazon EC2", 100)},
		Coverage:  window,
		Anomalies: costs.AnomalyResult{Requested: true, Marks: []costs.AnomalyMark{mark}, Truncated: true},
	}, fixedCostsNow)

	if _, ok := store.Anomalies(fixedCostsNow); ok {
		t.Errorf("the authoritative anomaly cache accepted a page-capped result — a lower bound must never masquerade as CE's complete list for the window")
	}
	marks, partial := store.AnomalyOverlay(window, fixedCostsNow)
	if !partial || len(marks) != 1 {
		t.Errorf("AnomalyOverlay = (%d marks, partial=%v), want (1, true) — the marks a capped walk did find are still real", len(marks), partial)
	}

	// A later complete fetch is authoritative and retires the partial set.
	store.ApplyFetchResult(costs.FetchResult{
		Query:     q,
		Records:   []costs.Record{fullMetricRecord(newest, "Amazon EC2", 100)},
		Coverage:  window,
		Anomalies: costs.AnomalyResult{Requested: true, Marks: []costs.AnomalyMark{}},
	}, fixedCostsNow)
	if marks, partial := store.AnomalyOverlay(window, fixedCostsNow); partial || len(marks) != 0 {
		t.Errorf("AnomalyOverlay after an authoritative zero-anomaly fetch = (%d marks, partial=%v), want (0, false)", len(marks), partial)
	}
}
