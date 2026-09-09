// costs_review5_test.go — S3: the by-ID fetch failure must travel as its
// own typed outcome (messages.ByIDFetchFailed), and that must be the ONLY
// mechanism that can pop dispatchCostsByIDTask's placeholder — no parallel
// messages.Flash text-sniff (strings.Contains(msg.Text, id)) surviving
// alongside it. A dual path is exactly as unsound as the original
// GetListAutoOpenSingle() heuristic it was meant to replace: any string
// that happens to contain the pending ID (a coincidence, a copy-pasted
// error, an unrelated resource sharing a substring) would still mis-pop.
// package unit (not unit_test): needs the full TUI Model, same reason as
// costs_review4_test.go.
package unit

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/costs"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
)

// costsReview5BogusInstanceID is the fixed target every test in this file
// drills to — shared so a test can craft a Flash that names it without
// re-deriving it from the helper's return value.
const costsReview5BogusInstanceID = "i-doesnotexist00000001"

// costsReview5DrillToStrandedByIDPlaceholder replays costs_review4_test.go's
// P4 setup verbatim through the Enter on the RESOURCE_ID leaf that pushes
// the placeholder and dispatches the by-ID fetch — returning the model and
// the (non-nil) dispatched cmd, still unresolved, so callers can inject
// whatever arrives before/instead of the fetch's own result.
func costsReview5DrillToStrandedByIDPlaceholder(t *testing.T, profile string) (tui.Model, tea.Cmd) {
	t.Helper()
	tui.Version = "1.0.2"
	m := newBlessedModel(t, profile, "us-east-1", tui.WithClients(demo.NewServiceClients()), tui.WithNoCache(true))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 40})

	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetCosts})
	assertStackInSync(t, m, "after navigating to costs")

	now := time.Now()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	period := costs.Period{Start: start.Format("2006-01-02"), End: start.AddDate(0, 1, 0).Format("2006-01-02")}
	const ec2Service = "Amazon Elastic Compute Cloud - Compute"

	m, _ = rootApplyMsg(m, messages.CostsLoaded{
		Query:    reviewBaseServiceQuery(),
		Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{reviewFullMetricRecord(period, ec2Service, 900.0)}},
		Requests: 1,
	})
	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEnter)) // -> USAGE_TYPE child

	// Newest week, not usageWindow[0]: the CostsLoaded record below is
	// planted on exactly one week, and Enter only dispatches a by-ID fetch
	// from a cursor sitting on that non-empty cell — every other week in the
	// window is still zero-valued, so drilling from wherever the cursor
	// happens to be left would dispatch nothing. Plant the record on the
	// newest week and scroll the cursor there before drilling.
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
	bogusID := costsReview5BogusInstanceID
	m, _ = rootApplyMsg(m, messages.CostsLoaded{
		Query:    resourceQuery,
		Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{reviewFullMetricRecord(resourcePeriod, bogusID, 450.0)}},
		Requests: 1,
	})
	assertStackInSync(t, m, "before the by-ID drill")
	m = m2MoveTUICursorToNewestColumn(m) // align the cursor with the newest-day cell the record above was planted at

	m, cmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("precondition: Enter on the RESOURCE_ID leaf returned a nil cmd — the by-ID fetch never dispatched")
	}
	return m, cmd
}

// ===========================================================================
// S3a — an error Flash whose TEXT happens to mention the pending instance
// ID must NOT pop the placeholder — the inverse of the old behavior. This is
// deliberately the HARDEST case for a lingering text-sniff fallback (unlike
// an unrelated flash with no mention of the ID at all): only an exact typed
// messages.ByIDFetchFailed match may pop it, never a string coincidence.
// The placeholder's own fetch (cmd) must itself resolve as
// messages.ByIDFetchFailed directly — not a messages.Flash a separate case
// then has to sniff for the ID — proving there is exactly ONE mechanism.
// ===========================================================================

func TestCostsReview5_S3a_ErrorFlashMentioningPendingID_MustNotPop(t *testing.T) {
	m, cmd := costsReview5DrillToStrandedByIDPlaceholder(t, "testprofile-s3a")

	// A Flash that would satisfy any "does msg.Text mention the pending ID"
	// heuristic — e.g. a copy-pasted error from an unrelated background
	// check that happens to name the same instance — arrives before the
	// placeholder's own fetch (cmd) resolves.
	m, _ = rootApplyMsg(m, messages.Flash{
		Text:    fmt.Sprintf("unrelated: some other background check failed for %s", costsReview5BogusInstanceID),
		IsError: true,
	})

	if plain := stripANSI(rootViewContent(m)); strings.Contains(plain, "TOTAL") {
		t.Errorf("a messages.Flash whose TEXT merely MENTIONS the pending instance ID (%s) popped the stranded placeholder anyway — only an exact typed messages.ByIDFetchFailed{TargetType,ID} match may pop it, never a string-sniff fallback on an ordinary Flash\n%s", costsReview5BogusInstanceID, plain)
	}

	// The placeholder's own fetch must resolve as messages.ByIDFetchFailed
	// DIRECTLY — the ONE mechanism, not a messages.Flash a separate case
	// then sniffs for the ID.
	msg := cmd()
	failed, ok := msg.(messages.ByIDFetchFailed)
	if !ok {
		t.Fatalf("the by-ID fetch's own not-found result = %#v (%T), want messages.ByIDFetchFailed directly — a messages.Flash here (even one a separate case later sniffs for the ID) means two mechanisms exist, not one", msg, msg)
	}
	if failed.ID != costsReview5BogusInstanceID {
		t.Errorf("messages.ByIDFetchFailed.ID = %q, want the pending target %q", failed.ID, costsReview5BogusInstanceID)
	}
	m, _ = rootApplyMsg(m, failed)
	if plain := stripANSI(rootViewContent(m)); !strings.Contains(plain, "TOTAL") {
		t.Errorf("after the by-ID fetch's OWN messages.ByIDFetchFailed also arrived, still not back on the costs grid (\"TOTAL\")")
	}
}

// ===========================================================================
// S3b — the by-ID fetch failure's Reason text (the user-facing note) must
// still reach the user once the typed outcome pops the placeholder — the
// typed match is not just a silent pop, the failure reason is not lost.
// ===========================================================================

func TestCostsReview5_S3b_TypedByIDFetchFailed_PopsAndSurfacesReason(t *testing.T) {
	m, cmd := costsReview5DrillToStrandedByIDPlaceholder(t, "testprofile-s3b")

	msg := cmd()
	failed, ok := msg.(messages.ByIDFetchFailed)
	if !ok {
		t.Fatalf("precondition: by-ID fetch for a bogus instance ID = %#v (%T), want messages.ByIDFetchFailed", msg, msg)
	}
	if failed.Reason == "" {
		t.Fatal("precondition: messages.ByIDFetchFailed.Reason is empty — no user-facing note to lose")
	}

	m, _ = rootApplyMsg(m, failed)

	if plain := stripANSI(rootViewContent(m)); !strings.Contains(plain, "TOTAL") {
		t.Errorf("after messages.ByIDFetchFailed for the pending placeholder's own target, still not back on the costs grid (\"TOTAL\")\n%s", plain)
	}
}

// ===========================================================================
// S3c — messages.ByIDFetchFailed for a DIFFERENT target than the one
// currently pending must NOT pop the placeholder — the exact guarantee a
// text/flag-only heuristic can never make, since it never looks at which
// fetch actually failed.
// ===========================================================================

func TestCostsReview5_S3c_TypedByIDFetchFailed_MismatchedTarget_DoesNotPop(t *testing.T) {
	m, _ := costsReview5DrillToStrandedByIDPlaceholder(t, "testprofile-s3c")

	m, _ = rootApplyMsg(m, messages.ByIDFetchFailed{
		TargetType: "ec2",
		ID:         "i-someCompletelyDifferentInstance",
		Reason:     "not found",
	})

	if plain := stripANSI(rootViewContent(m)); strings.Contains(plain, "TOTAL") {
		t.Errorf("messages.ByIDFetchFailed for a DIFFERENT (mismatched) target popped the placeholder anyway — back on the costs grid (\"TOTAL\") after a failure that belongs to some other by-ID fetch entirely\n%s", plain)
	}
}
