// The TUI's messages.CostsLoaded case must dispatch the TaskRequest
// ApplyCostsLoaded returns (the granularity-fallback re-fetch), or the
// fallback never runs in the terminal app.
//
// dispatchCostsByIDTask pushes a placeholder rendererState that must be popped
// on a failed fetch as well as on messages.Navigate, or it is stranded on the
// TUI's m.stack.
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

// matchesAwaited skips the Range/Identity comparison for a zero-value
// ev.Query.Range, so the child frame's query shape alone matches it without
// the TUI's real-clock window boundaries.

func TestCostsReview4_P2_TUI_CostsLoaded_DispatchesReturnedFallbackTask(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetCosts})
	assertStackInSync(t, m, "after navigating to costs")

	// The TUI navigates via time.Now(), so the seeded record's period anchors to
	// the real current month.
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

	// The child frame's fetch returns zero records, so ApplyCostsLoaded returns a
	// coarser-fetch TaskRequest; childQuery leaves Range at its zero value.
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

	// The record is planted on the newest week and the cursor scrolled there, so
	// the RESOURCE_ID drill starts inside the 14-day resource-drill clamp.
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

	// Enter on the RESOURCE_ID leaf pushes a placeholder ScreenResourceList on
	// both stacks and returns KindFetchByIDDetail; the wrapped cmd fetches the
	// bogus ID against the demo EC2 fixtures, finds nothing, and resolves as
	// messages.ByIDFetchFailed.
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
