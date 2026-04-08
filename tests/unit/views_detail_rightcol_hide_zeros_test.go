package unit_test

// views_detail_rightcol_hide_zeros_test.go — Bug B test:
// The ct-events right column must not show self-pivot rows (CT events by AccessKeyId /
// Username / EventName / SharedEventId) when their count is zero.  Self-pivot rows are
// filters, not counts — showing "(0)" there is semantically meaningless.
//
// NOTE: typed groups from other resource types (e.g. "EC2 Instances (0)") are intentionally
// allowed to show "(0)" per the project-wide design; this test only enforces the narrower
// constraint on the 4 ct-events self-pivot rows.
//
// Strategy: construct a DetailModel at width 180 (auto-shows right column),
// inject demo checker results via ApplyRelatedResults, call View(), and assert
// that no line matching "CT events by … (0)" appears in the output.

import (
	"strings"
	"testing"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
)

// TestCtEventsDemoRightColumnHidesZeroPivotRows asserts that, once the demo
// checker results are applied to a DetailModel, the rendered right column does not
// contain any of the 4 self-pivot rows (CT events by AccessKeyId / Username /
// EventName / SharedEventId) with a zero count.  Self-pivot rows are filters, not
// counts; showing "(0)" there is semantically meaningless and must be suppressed.
//
// Typed groups from other resource types (e.g. "EC2 Instances (0)") are NOT checked
// here — those are intentionally allowed per the project-wide design.
//
// The test iterates all ct-events demo fixtures to cover all coverage states.
func TestCtEventsDemoRightColumnHidesZeroPivotRows(t *testing.T) {
	ensureNoColor(t)

	// Load all fixtures.
	fixtures, ok := demo.GetResources("ct-events")
	if !ok || len(fixtures) == 0 {
		t.Fatal("demo.GetResources(\"ct-events\") returned no fixtures")
	}

	cfg := configForType("ct-events")
	cache := buildDemoResourceCache(t)

	for _, res := range fixtures {
		res := res
		t.Run(res.ID, func(t *testing.T) {
			// Build the detail model with the right column auto-shown (width 180 >= 60).
			m := newDetailModel(res, "ct-events", cfg)
			m.SetSize(180, 40)

			// Apply checker results.
			results := ctEventsRealCheckerResults(res, cache)
			m.ApplyRelatedResults(results)

			// Render and inspect.
			view := m.View()
			lines := strings.Split(stripAnsi(view), "\n")

			for _, line := range lines {
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(trimmed, "CT events by") && strings.HasSuffix(trimmed, "(0)") {
					t.Errorf("Bug B FAIL: right column contains a zero-count PIVOT row for event=%s — "+
						"line=%q — self-pivot rows with Count=0 are meaningless (pivots are filters, not counts) and must be hidden",
						res.ID, trimmed)
				}
			}
		})
	}
}

// TestCtEventsDemoRightColumnHidesZeroPivotRows_CaseH_NoActionable asserts that
// Case H (Insight, allZero) results in a right column with no self-pivot rows showing "(0)".
// This is the most restrictive form: all groups have Count=0.
// The 4 "CT events by" pivot rows must be hidden; no positive counts should appear either.
func TestCtEventsDemoRightColumnHidesZeroPivotRows_CaseH_NoActionable(t *testing.T) {
	ensureNoColor(t)

	res := loadCTEventsFixtureByID(t, "e-b8c9d0e1")
	cache := buildDemoResourceCache(t)

	cfg := configForType("ct-events")
	m := newDetailModel(res, "ct-events", cfg)
	m.SetSize(180, 40)

	results := ctEventsRealCheckerResults(res, cache)
	m.ApplyRelatedResults(results)

	view := stripAnsi(m.View())
	lines := strings.Split(view, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Self-pivot rows with (0) must be hidden.
		if strings.HasPrefix(trimmed, "CT events by") && strings.HasSuffix(trimmed, "(0)") {
			t.Errorf("Bug B FAIL: right column contains a zero-count PIVOT row for event=%s — "+
				"line=%q — self-pivot rows with Count=0 are meaningless (pivots are filters, not counts) and must be hidden",
				res.ID, trimmed)
		}
		// Sanity: no row should contain a positive count either (allZero fixture).
		if containsPositiveCount(trimmed) {
			t.Errorf("Bug B unexpected: Case H right column contains positive-count row — "+
				"line=%q — Insight fixture should have no related resources", trimmed)
		}
	}
}

// containsPositiveCount reports whether a line ends with "(N)" where N > 0.
func containsPositiveCount(line string) bool {
	// Look for pattern " (N)" where N is one or more digits > 0.
	start := strings.LastIndex(line, "(")
	if start < 0 {
		return false
	}
	end := strings.LastIndex(line, ")")
	if end <= start {
		return false
	}
	inner := line[start+1 : end]
	for _, ch := range inner {
		if ch < '1' || ch > '9' {
			return false
		}
	}
	return len(inner) > 0
}
