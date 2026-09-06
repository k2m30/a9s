package unit

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/app"
	demofixtures "github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
)

// TestCostsClockSeam_BothLanesOpenOnThePinnedMonth pins the clock and opens
// the Cost Explorer down each lane's own navigation path — the headless
// Controller's :costs command and the TUI Model's Navigate message.
//
// Both lanes seed the costs window from app.Now(). The window is the one
// piece of the screen a wall-clock read decides, so a lane that called
// time.Now() directly would open on the machine's month while the other
// opened on the pinned one, and the same suite would render differently
// tomorrow. The date is deliberately years in the past: a lane that bypasses
// the seam cannot accidentally agree with it.
//
// The expected window comes from the demo fixtures' own At constructor, the
// seam on the fixture side, so the two seams are pinned to one instant.
func TestCostsClockSeam_BothLanesOpenOnThePinnedMonth(t *testing.T) {
	pinned := time.Date(2021, time.March, 15, 12, 0, 0, 0, time.UTC)
	restore := app.SetNowForTest(func() time.Time { return pinned })
	defer restore()

	// The fixtures' window is its own length; the years it spans are what
	// both sides must agree on.
	wantMonths := demofixtures.CostsMonthsAt(pinned)
	wantYears := make(map[string]bool, len(wantMonths))
	for _, m := range wantMonths {
		// "YYYY-MM-01" -> the two-digit year the column label carries.
		wantYears[m[2:4]] = true
	}

	// --- headless lane: the :costs command ---
	c := newTestController(t)

	vs, _ := c.Apply(app.Action{Kind: app.ActionCommand, Arg: "costs"})
	if vs.Body.Costs == nil {
		t.Fatal("the :costs command did not open the Cost Explorer screen")
	}
	cols := vs.Body.Costs.Columns
	if len(cols) == 0 {
		t.Fatal("the Cost Explorer opened with no columns; there is no window to check")
	}
	for i, col := range cols {
		year := col.Label[len(col.Label)-2:]
		if !wantYears[year] {
			t.Errorf("headless column %d = %q, whose year is outside the pinned window %v — the lane read the machine clock, not app.Now()",
				i, col.Label, wantMonths)
		}
		// Only the month the pinned instant falls in is still open.
		wantOpen := i == len(cols)-1
		if col.Open != wantOpen {
			t.Errorf("headless column %d (%q) Open = %v, want %v", i, col.Label, col.Open, wantOpen)
		}
	}
	newest := cols[len(cols)-1].Label

	// --- TUI lane: the Navigate message ---
	tui.Version = "test"
	m := tui.New("costs-clock-seam", "us-east-1", tui.WithNoCache(true))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetCosts})
	rendered := stripANSI(rootViewContent(m))

	// The TUI viewport shows a suffix of the window, so the newest column is
	// the one both lanes must always have on screen; "*" is the renderer's
	// open-period marker.
	if !strings.Contains(rendered, newest+"*") {
		t.Errorf("the TUI lane does not show %q as the open column; both lanes must open on the pinned month:\n%s", newest, rendered)
	}
	// A column carrying any other year is a second clock read somewhere on
	// this lane.
	for _, field := range strings.Fields(rendered) {
		if len(field) < 3 || !strings.Contains(field, "'") {
			continue
		}
		year := strings.TrimSuffix(field[strings.Index(field, "'")+1:], "*")
		if len(year) == 2 && !wantYears[year] {
			t.Errorf("the TUI lane renders column %q, outside the pinned window %v", field, wantMonths)
		}
	}
}
