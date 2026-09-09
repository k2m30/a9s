// costs_review4_test.go — Cost Explorer TUI-layer pins: package unit (not
// unit_test), same reason as costs_review_findings_test.go's own file-level
// doc — these need the full TUI Model
// (newRootSizedModel/rootApplyMsg/assertStackInSync), unreachable from the
// headless-only package unit_test. Reuses
// reviewBaseServiceQuery/reviewFullMetricRecord (costs_review_findings_test.go,
// same package).
//
// P2 (internal/tui/app.go): `case messages.CostsLoaded` must dispatch the
// TaskRequest Handle returns from ApplyCostsLoaded (the N3 fallback
// re-fetch), or the fallback is dead in the terminal app.
//
// P4 (internal/tui/app_costs.go): dispatchCostsByIDTask pushes a placeholder
// rendererState; its wrapped cmd must pop it on the Flash (failure) branch
// as well as on messages.Navigate (success), or the placeholder is stranded
// on the TUI's own m.stack. Generic Flash handling does not reach it:
// handleFlash (app_flash.go) only calls m.core.HandleFlash (session-level
// runtime.Core), never app.Controller.Handle, so the Controller's
// popAutoOpenSinglePlaceholderOnNotFound is never reached from this path.
package unit

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/costs"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
)

// ===========================================================================
// P2 — the TUI's CostsLoaded handler must dispatch ApplyCostsLoaded's
// returned TaskRequest (the N3 granularity-fallback re-fetch), not discard
// it. matchesAwaited's own escape hatch (a zero-value ev.Query.Range skips
// the Range/Identity comparison) lets this pin the child frame's query
// SHAPE alone, without needing the TUI's real (uninjected time.Now())
// window boundaries.
// ===========================================================================

func TestCostsReview4_P2_TUI_CostsLoaded_DispatchesReturnedFallbackTask(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetCosts})
	assertStackInSync(t, m, "after navigating to costs")

	// The TUI navigates via time.Now() (no injected clock at this layer,
	// same as F3's own pattern) — the seeded record's period anchors to the
	// real current month so it lands inside the real window.
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	period := costs.Period{Start: start.Format("2006-01-02"), End: start.AddDate(0, 1, 0).Format("2006-01-02")}
	m, _ = rootApplyMsg(m, messages.CostsLoaded{
		Query:    reviewBaseServiceQuery(),
		Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{reviewFullMetricRecord(period, "Amazon EC2", 1200.0)}},
		Requests: 1,
	})

	// Drill one level (Enter -> ActionSelect): pushes a fresh, uncovered
	// USAGE_TYPE child frame — screen.Select's PushDrill sets
	// Fallback.Eligible since the selected SERVICE cell (1200.0) was
	// non-zero.
	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))
	assertStackInSync(t, m, "after drilling one level")

	// The child frame's own fetch genuinely returns zero records — the N3
	// fallback fires and ApplyCostsLoaded RETURNS a coarser-fetch
	// TaskRequest. childQuery matches the child frame's own SHAPE only
	// (Range left at its zero value — matchesAwaited's documented escape
	// hatch), so this needs no knowledge of the TUI's real-clock window
	// boundaries.
	childQuery := costs.Query{
		Granularity: costs.GranularityWeek.APIGranularity(),
		GroupBy:     []costs.Dimension{costs.DimensionUsageType},
		Filter:      costs.Filter{Equals: map[costs.Dimension][]string{costs.DimensionService: {"Amazon EC2"}}},
	}
	_, cmd := rootApplyMsg(m, messages.CostsLoaded{Query: childQuery, Grid: costs.GridResult{Fetched: true}})

	if cmd == nil {
		t.Error("m.Update(messages.CostsLoaded{Grid: costs.GridResult{Fetched: true}, ...}) returned a nil tea.Cmd even though the delivery triggered the N3 granularity fallback — internal/tui/app.go's CostsLoaded case discards Controller.Handle's returned TaskRequest entirely (`m.ctrl.Handle(msg); return m, nil`)")
	}
}

// ===========================================================================
// P4 — a by-ID resource-drill fetch that finds nothing must pop the
// placeholder list the TUI itself pushed (dispatchCostsByIDTask), not
// strand it above the costs screen.
// ===========================================================================

func TestCostsReview4_P4_TUI_ByIDFetchNotFound_PopsStrandedPlaceholder(t *testing.T) {
	tui.Version = "1.0.2"
	m := newBlessedModel(t, "testprofile-p4", "us-east-1", tui.WithClients(demo.NewServiceClients()), tui.WithNoCache(true))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 40})

	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetCosts})
	assertStackInSync(t, m, "after navigating to costs")

	now := time.Now()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	period := costs.Period{Start: start.Format("2006-01-02"), End: start.AddDate(0, 1, 0).Format("2006-01-02")}
	// resourceDrillAllowedService (costs.ResourceDrillAllowed's own hard
	// constant) — the ONLY SERVICE value CE's resource-level API supports;
	// every other service Refuses before ever reaching a RESOURCE_ID leaf.
	const ec2Service = "Amazon Elastic Compute Cloud - Compute"

	m, _ = rootApplyMsg(m, messages.CostsLoaded{
		Query:    reviewBaseServiceQuery(),
		Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{reviewFullMetricRecord(period, ec2Service, 900.0)}},
		Requests: 1,
	})
	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEnter)) // -> USAGE_TYPE child

	// A freshly-pushed drill child's Cursor is left at its zero value
	// (Row:0, Col:0) — applyCostsSelect's PushDrill case never sets it — so
	// the selected cell is the OLDEST week. Late in the month that week is
	// outside the 14-day resource-drill clamp, so the RESOURCE_ID drill would
	// be refused: plant the record on the newest week and scroll the cursor
	// there before drilling.
	usageWindow := costs.WindowWithin(period, costs.GranularityWeek, now)
	usagePeriod := usageWindow[len(usageWindow)-1]
	usageQuery := costs.Query{
		Granularity: costs.GranularityWeek.APIGranularity(),
		GroupBy:     []costs.Dimension{costs.DimensionUsageType},
		Filter:      costs.Filter{Equals: map[costs.Dimension][]string{costs.DimensionService: {ec2Service}}},
	}
	m, _ = rootApplyMsg(m, messages.CostsLoaded{
		Query:    usageQuery,
		Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{reviewFullMetricRecord(usagePeriod, "USE1-BoxUsage:m5.large", 450.0)}},
		Requests: 1,
	})
	m = m2MoveTUICursorToNewestColumn(m)                 // scroll to the newest week the record was planted at
	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEnter)) // -> RESOURCE_ID child

	// The NEWEST day in the window, not window[0] — costs.ClampResourceDrillWindow
	// (core/costs/drill.go) already trimmed the RESOURCE_ID frame's actual
	// Window down to the last ResourceDrillWindowRetentionDays, so window[0]'s
	// UNCLAMPED period can fall outside every column the frame actually
	// renders; window[len-1]'s End is always the clamp's own upper bound and
	// is therefore always a real column (see costs_lane_parity_test.go's
	// TestCostsLaneParity_A for the full trace).
	resourceWindow := costs.WindowWithin(usagePeriod, costs.GranularityDay, now)
	resourcePeriod := resourceWindow[len(resourceWindow)-1]
	resourceQuery := costs.Query{
		Granularity: costs.GranularityDay.APIGranularity(),
		GroupBy:     []costs.Dimension{costs.DimensionResourceID},
		Filter: costs.Filter{Equals: map[costs.Dimension][]string{
			costs.DimensionService:   {ec2Service},
			costs.DimensionUsageType: {"USE1-BoxUsage:m5.large"},
		}},
	}
	bogusID := "i-doesnotexist00000001"
	m, _ = rootApplyMsg(m, messages.CostsLoaded{
		Query:    resourceQuery,
		Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{reviewFullMetricRecord(resourcePeriod, bogusID, 450.0)}},
		Requests: 1,
	})
	assertStackInSync(t, m, "before the by-ID drill")
	m = m2MoveTUICursorToNewestColumn(m) // align the cursor with the newest-day cell the record above was planted at

	// Enter on the RESOURCE_ID leaf: applyCostsSelect pushes a placeholder
	// ScreenResourceList (both controller and TUI stacks) and returns
	// KindFetchByIDDetail. dispatchCostsByIDTask's own wrapped cmd fetches
	// the bogus ID against the REAL demo EC2 fixtures (safe — no client is
	// nil, unlike a raw pre-connect harness), which genuinely finds
	// nothing. S3: the ONE mechanism is the typed outcome itself — the
	// fetch wrapper's own failure IS messages.ByIDFetchFailed, not a
	// messages.Flash a separate case has to sniff for the pending ID.
	m, cmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("precondition: Enter on the RESOURCE_ID leaf returned a nil cmd — the by-ID fetch never dispatched")
	}
	msg := cmd()
	failed, ok := msg.(messages.ByIDFetchFailed)
	if !ok {
		t.Fatalf("precondition: by-ID fetch for a bogus instance ID = %#v (%T), want messages.ByIDFetchFailed", msg, msg)
	}

	m, _ = rootApplyMsg(m, failed)

	if plain := stripANSI(rootViewContent(m)); !strings.Contains(plain, "TOTAL") {
		t.Errorf("after the by-ID fetch's own messages.ByIDFetchFailed, the costs screen's own grid (\"TOTAL\" row) is not showing — the TUI is still stranded on the empty placeholder resource list dispatchCostsByIDTask pushed, never popped back to costs:\n%s", plain)
	}
}
