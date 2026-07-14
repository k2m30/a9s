package costs

import (
	"fmt"
	"time"
)

// DrillLevel is one stack frame. The stack root is the pivot view itself.
type DrillLevel struct {
	RowDim      Dimension
	Filter      Filter
	Granularity Granularity
	Window      []Period
	Cursor      struct{ Row, Col int }
	ScrollX     int
	ScrollY     int

	// Fallback carries this frame's granularity-fallback gate: the inputs
	// needed to re-plan ONCE at the parent drill step's own coarser
	// granularity/period when this frame's own fetch genuinely delivers
	// zero records for a parent cell that was itself non-zero (e.g. a
	// charge billed only monthly, so its drilled weekly window is
	// naturally empty). Zero value means "not eligible" — a frame reached
	// any way other than a fresh PushDrill, or one whose selected parent
	// cell was itself zero, never triggers the fallback.
	Fallback FallbackGate
}

// FallbackGate is one frame's granularity-fallback eligibility and
// one-shot state, set by screen.Select's PushDrill outcome at drill time
// and consumed by the fetch-delivery path that applies it.
type FallbackGate struct {
	// Eligible is true only when the parent drill step's own selected cell
	// (the cell the user pressed Enter on) was non-zero — the fallback
	// must never fire for an ordinarily-empty cell.
	Eligible bool
	// Fired is set the moment the fallback re-plan actually runs, so it
	// can never chain a second, ever-coarser re-plan for the same frame.
	Fired bool
	// ParentGranularity/SelectedPeriod are the parent drill step's own
	// granularity and the exact period the user selected — the fallback's
	// re-plan narrows this frame's own Window to exactly SelectedPeriod at
	// ParentGranularity, never a wider trailing window.
	ParentGranularity Granularity
	SelectedPeriod    Period
}

// resourceDrillWindowDays is the CE hard limit for GetCostAndUsageWithResources:
// the queried range cannot start more than this many days before now.
const resourceDrillWindowDays = 14

// resourceDrillAllowedService is the only SERVICE value CE's
// GetCostAndUsageWithResources supports — a hard CE API requirement, not an
// a9s choice. A single-service pin on any other service is refused in-app
// (ResourceDrillAllowed) rather than ever sent to CE.
const resourceDrillAllowedService = "Amazon Elastic Compute Cloud - Compute"

// ClampResourceDrillWindow drops every period in window that starts before
// now's 14-day resource-level retention cutoff (now truncated to its own
// day-start before subtracting, so the cutoff is a calendar-day boundary
// regardless of now's time-of-day), keeping every retained period exactly
// as BuildWindow produced it — only the too-early portion is clamped away,
// never a retained period's End. The caller must pass the RESOURCE_ID
// drill's actual finer-granularity window (BuildWindow's output), not
// merely the coarser cell the user selected: BuildWindow can expand a
// selected period (e.g. a month-clipped week Start) into a wider native
// window (the full Mon-Sun ISO week) that starts earlier than the selected
// period itself, so validating the selected period alone can pass while the
// window actually queried does not.
func ClampResourceDrillWindow(window []Period, now time.Time) []Period {
	cutoff := utcDate(now).AddDate(0, 0, -resourceDrillWindowDays)
	out := make([]Period, 0, len(window))
	for _, p := range window {
		start, err := ParseDate(p.Start)
		if err != nil || start.Before(cutoff) {
			continue
		}
		out = append(out, p)
	}
	return out
}

// ResourceDrillAllowed gates RESOURCE_ID: l.Window must be non-empty — the
// caller passes it through ClampResourceDrillWindow first, so a refusal
// here means nothing remained in the 14-day retention window after
// clamping, not merely that some periods were too old — the accumulated
// filter must pin exactly one SERVICE, since CE rejects
// GetCostAndUsageWithResources calls that don't isolate a single service —
// and that one SERVICE must be resourceDrillAllowedService, the only value
// CE's resource-level API actually supports.
func ResourceDrillAllowed(l DrillLevel, now time.Time) (bool, string) {
	if len(l.Window) == 0 {
		return false, fmt.Sprintf(
			"resource-level cost data is only retained for the last %d days — nothing in range to drill into",
			resourceDrillWindowDays,
		)
	}
	services := l.Filter.Equals[DimensionService]
	if len(services) != 1 {
		return false, "pin exactly one SERVICE to drill to individual resources"
	}
	if services[0] != resourceDrillAllowedService {
		return false, fmt.Sprintf("resource-level cost data is only available for %q", resourceDrillAllowedService)
	}
	return true, ""
}
