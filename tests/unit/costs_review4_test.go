// costs_review4_test.go — Cost Explorer external review round 3, TUI-layer
// half (P2, P4): package unit (not unit_test), same reason as
// costs_review_findings_test.go's own file-level doc — these findings need
// the full TUI Model (newRootSizedModel/rootApplyMsg/assertStackInSync),
// unreachable from the headless-only package unit_test. Reuses
// reviewBaseServiceQuery/reviewFullMetricRecord (costs_review_findings_test.go,
// same package).
//
// P2 (internal/tui/app.go:343-345): confirmed in source — `case
// messages.CostsLoaded: m.ctrl.Handle(msg); return m, nil` discards BOTH of
// Handle's return values. Handle (internal/app/handle.go:153) DOES capture
// and return ApplyCostsLoaded's TaskRequest now (the N3 fallback re-fetch);
// the TUI just throws it away, so the fallback is dead in the terminal app.
//
// P4 (internal/tui/app_costs.go:84-102): confirmed in source —
// dispatchCostsByIDTask pushes a placeholder rendererState (line 88) but its
// wrapped cmd only pops it on the messages.Navigate (success) branch; a
// Flash (failure) falls through `return msg` unchanged, leaving the
// placeholder stranded on the TUI's own m.stack. Verified this is NOT
// reachable via any generic Flash handling: handleFlash (app_flash.go) only
// calls m.core.HandleFlash (session-level runtime.Core), never
// app.Controller.Handle — X10's own popAutoOpenSinglePlaceholderOnNotFound
// mechanism lives in Controller.Handle and is simply never reached from
// this TUI path at all.
package unit

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/costs"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
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
	m := tui.New("testprofile-p4", "us-east-1", tui.WithClients(demo.NewServiceClients()), tui.WithNoCache(true))
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
	// (Row:0, Col:0) — applyCostsSelect's PushDrill case never sets it —
	// unlike the root/TrailingAnchored frames' own FR-002 "open at today"
	// placement. The selected CELL is therefore window[0], not the period
	// containing "now".
	usageWindow := costs.WindowWithin(period, costs.GranularityWeek, now)
	usagePeriod := usageWindow[0]
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
	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEnter)) // -> RESOURCE_ID child

	resourceWindow := costs.WindowWithin(usagePeriod, costs.GranularityDay, now)
	resourcePeriod := resourceWindow[0]
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
