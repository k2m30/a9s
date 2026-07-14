package unit_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/k2m30/a9s/v3/internal/costs"
)

func filterPinning(dim costs.Dimension, values ...string) costs.Filter {
	return costs.Filter{Equals: map[costs.Dimension][]string{dim: values}}
}

// TestResourceDrillAllowed covers ResourceDrillAllowed's NEW contract: it no
// longer inspects period dates itself — the caller passes l.Window through
// ClampResourceDrillWindow first (see TestClampResourceDrillWindow_* below),
// so a refusal here means only "nothing remained after clamping" (an empty
// Window) or "the SERVICE filter isn't pinned to exactly one value", never
// "some period in Window was too old" (that's ClampResourceDrillWindow's
// job, and it CLAMPS rather than refuses).
func TestResourceDrillAllowed(t *testing.T) {
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	withinWindow := []costs.Period{{Start: "2026-07-05", End: "2026-07-06"}}
	ec2Filter := filterPinning(costs.Dimension("SERVICE"), "Amazon Elastic Compute Cloud - Compute")
	zeroServicesFilter := costs.Filter{}
	twoServicesFilter := filterPinning(costs.Dimension("SERVICE"), "Amazon Elastic Compute Cloud - Compute", "Amazon Simple Storage Service")

	tests := []struct {
		name       string
		l          costs.DrillLevel
		wantAllow  bool
		wantReason bool
	}{
		{
			name:       "window is empty (nothing left after ClampResourceDrillWindow) is refused",
			l:          costs.DrillLevel{Filter: ec2Filter, Window: nil},
			wantAllow:  false,
			wantReason: true,
		},
		{
			name:       "filter pins zero services",
			l:          costs.DrillLevel{Filter: zeroServicesFilter, Window: withinWindow},
			wantAllow:  false,
			wantReason: true,
		},
		{
			name:       "filter pins two services",
			l:          costs.DrillLevel{Filter: twoServicesFilter, Window: withinWindow},
			wantAllow:  false,
			wantReason: true,
		},
		{
			name:       "non-empty window and exactly one SERVICE pinned",
			l:          costs.DrillLevel{Filter: ec2Filter, Window: withinWindow},
			wantAllow:  true,
			wantReason: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allow, reason := costs.ResourceDrillAllowed(tt.l, now)
			if allow != tt.wantAllow {
				t.Errorf("ResourceDrillAllowed() allow = %v, want %v (reason=%q)", allow, tt.wantAllow, reason)
			}
			if tt.wantReason && reason == "" {
				t.Error("ResourceDrillAllowed() reason = \"\", want a non-empty explanation")
			}
			if !tt.wantReason && reason != "" {
				t.Errorf("ResourceDrillAllowed() reason = %q, want \"\" when allowed", reason)
			}
		})
	}
}

// TestClampResourceDrillWindow_DropsTooOld_KeepsRestExactly pins the CLAMP
// contract explicitly chosen over refuse: periods starting before now's
// 14-day retention cutoff are dropped, every retained period is kept byte
// -for-byte (Start AND End untouched — only the too-early portion of the
// window is clamped away, never a retained period's own End).
func TestClampResourceDrillWindow_DropsTooOld_KeepsRestExactly(t *testing.T) {
	now := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC) // cutoff: 2026-07-01
	window := []costs.Period{
		{Start: "2026-06-29", End: "2026-06-30"}, // before cutoff -- dropped
		{Start: "2026-06-30", End: "2026-07-01"}, // before cutoff -- dropped
		{Start: "2026-07-01", End: "2026-07-02"}, // exactly at cutoff -- kept
		{Start: "2026-07-05", End: "2026-07-06"}, // kept
	}

	got := costs.ClampResourceDrillWindow(window, now)

	want := []costs.Period{
		{Start: "2026-07-01", End: "2026-07-02"},
		{Start: "2026-07-05", End: "2026-07-06"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ClampResourceDrillWindow() = %+v, want %+v", got, want)
	}
}

// TestClampResourceDrillWindow_EverythingTooOld_ReturnsEmpty is the "nothing
// remains" case ResourceDrillAllowed's refusal now depends on.
func TestClampResourceDrillWindow_EverythingTooOld_ReturnsEmpty(t *testing.T) {
	now := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	window := []costs.Period{
		{Start: "2026-05-01", End: "2026-05-02"},
		{Start: "2026-05-10", End: "2026-05-11"},
	}

	got := costs.ClampResourceDrillWindow(window, now)

	if len(got) != 0 {
		t.Errorf("ClampResourceDrillWindow() = %+v, want empty (every period predates the 14-day retention cutoff)", got)
	}
}
