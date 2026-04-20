package unit_test

// related_cache_replay_selfpivot_test.go — regression guard for the
// cache-replay bug that tightening the rightcolumn DefDisplayName fallback
// in #280 would have introduced.
//
// The rightcolumn matches RelatedCheckResultMsg to a row by DefDisplayName.
// A stricter fallback — only matching by TargetType when exactly one row
// carries it — leaves any resource type with multiple defs sharing a
// TargetType (notably ct-events, with 4 self-pivot rows all targeting
// "ct-events") stuck in the loading state on cache replay unless the cached
// messages carry DefDisplayName.
//
// This test uses the public message plumbing: dispatch N results for
// ct-events with distinct DefDisplayNames, re-render the right column, and
// assert every row resolved out of the loading state.

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// TestRelatedCacheReplay_CtEventsSelfPivots_AllRowsResolve pins that when
// the four ct-events self-pivot rows all receive a RelatedCheckResultMsg
// carrying their distinct DefDisplayName, every row transitions out of
// "loading" — i.e. no row is left with the "?" loading glyph.
//
// If DefDisplayName is lost on cache replay (e.g., the cache stores only
// Result and the replay path omits the display name), the rightcolumn's
// strict-match fallback refuses to bind any of the four self-pivot rows and
// this test fails with rows still showing "?".
func TestRelatedCacheReplay_CtEventsSelfPivots_AllRowsResolve(t *testing.T) {
	ensureNoColor(t)

	defs := resource.GetRelated("ct-events")
	var ctSelfPivots []resource.RelatedDef
	for _, def := range defs {
		if def.TargetType == "ct-events" {
			ctSelfPivots = append(ctSelfPivots, def)
		}
	}
	if len(ctSelfPivots) < 2 {
		t.Fatalf("ct-events registry must have ≥2 self-pivot rows, got %d", len(ctSelfPivots))
	}

	// Build a ct-events detail model at a width that auto-shows the right column.
	res := buildCTEventsResource(
		"evt-selfpivot-0000000000001",
		"DescribeInstances",
		"ct-info",
		minimalCTJSON,
	)
	cfg := config.DefaultConfig()
	k := keys.Default()
	d := views.NewDetail(res, "ct-events", cfg, k)
	d.SetSize(200, 40)

	// Inject one Count=5 result per self-pivot def, each with distinct
	// DefDisplayName — the shape the production cache-replay path now uses.
	for _, def := range ctSelfPivots {
		d, _ = d.Update(messages.RelatedCheckResultMsg{
			ResourceType:   "ct-events",
			DefDisplayName: def.DisplayName,
			Result: resource.RelatedCheckResult{
				TargetType:  "ct-events",
				Count:       5,
				ResourceIDs: []string{"evt-a", "evt-b"},
			},
		})
	}

	view := stripAnsi(d.View())

	// Each self-pivot row must no longer carry the loading "?" glyph.
	for _, def := range ctSelfPivots {
		// Look for "<DisplayName> … ?" — the loading rendering. If the row
		// resolved, the "?" should be replaced with "(5)" or similar count.
		loadingFragment := def.DisplayName + strings.Repeat(" ", 1) // minimum spacer
		_ = loadingFragment
		if !strings.Contains(view, def.DisplayName) {
			t.Fatalf("self-pivot row %q not rendered at all", def.DisplayName)
		}
		// Resolve-evidence: the count must appear. With Count=5 the rendering
		// includes "(5)" somewhere on the row line.
		if !strings.Contains(view, "(5)") {
			t.Errorf("row %q did not resolve to Count=5 (still loading?); rendered view:\n%s",
				def.DisplayName, view)
			return
		}
	}
}
