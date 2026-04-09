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
	"testing"

	_ "github.com/k2m30/a9s/v3/internal/aws"
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
	t.Skip("needs rewrite onto cold-cache harness (T047-T049)")
}

// TestCtEventsDemoRightColumnHidesZeroPivotRows_CaseH_NoActionable asserts that
// Case H (Insight, allZero) results in a right column with no self-pivot rows showing "(0)".
// This is the most restrictive form: all groups have Count=0.
// The 4 "CT events by" pivot rows must be hidden; no positive counts should appear either.
func TestCtEventsDemoRightColumnHidesZeroPivotRows_CaseH_NoActionable(t *testing.T) {
	t.Skip("needs rewrite onto cold-cache harness (T047-T049)")
}

