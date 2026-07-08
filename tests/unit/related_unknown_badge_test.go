// related_unknown_badge_test.go — pins the four-state contract that "(?)" is
// FORBIDDEN: a RESOLVED unknown related count (State: RelatedUnknown, not
// loading, no error, no FetchFilter) renders with NO count badge (blank) and is
// ACTIONABLE, never as a "(?)" badge.
//
// Contract under test (resource.FormatRelatedCount, internal/resource/related.go):
//   - State: RelatedDeferred (server-side FetchFilter pivot) → ""  (actionable
//     drill-in link; the filtered re-fetch resolves the real count on entry)
//   - State: RelatedUnknown (no filter)                      → ""  (blank,
//     actionable — a real registered checker whose backing cache entry has not
//     warmed yet; Enter drills into the target type's plain top-level list.
//     "(?)" is never produced.)
//   - State: RelatedResolved, count == 0                     → "(0)"
//   - State: RelatedResolved, count == N (N > 0)              → "(N)"
package unit_test

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/app"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
)

// ---------------------------------------------------------------------------
// Pin 1: FormatRelatedCount direct table — the single source of truth.
// ---------------------------------------------------------------------------

func TestFormatRelatedCount_Table(t *testing.T) {
	cases := []struct {
		name   string
		state  domain.RelatedRowState
		count  int
		approx bool
		want   string
	}{
		{"ResolvedUnknown_NoFilter", domain.RelatedUnknown, 0, false, ""},
		{"ResolvedUnknown_WithFilter_NoBadge", domain.RelatedDeferred, 0, false, ""},
		{"ExactZero", domain.RelatedResolved, 0, false, "(0)"},
		{"Positive", domain.RelatedResolved, 7, false, "(7)"},
		{"LargeCount", domain.RelatedResolved, 1000, false, "(1000)"},
		// Approximate lower bounds from a truncated target scan render "N+".
		{"ApproxZero", domain.RelatedResolved, 0, true, "(0+)"},
		{"ApproxPositive", domain.RelatedResolved, 3, true, "(3+)"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := resource.FormatRelatedCount(tc.state, tc.count, tc.approx)
			if got != tc.want {
				t.Errorf("FormatRelatedCount(state=%v, %d, approx=%v) = %q, want %q",
					tc.state, tc.count, tc.approx, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Pin 2: panel-level render — the "(?)" badge reaches the live TUI and the
// row stays non-actionable (dim), extending tui_related_dim_parity_test.go's
// conventions (newRelatedDimParityDetail / relatedDimParityBody /
// extractRelatedLine) without duplicating its state-sweep matrix.
// ---------------------------------------------------------------------------

// ngResourceForCacheMissBadge returns a node-group resource, matching the
// TestNGColdCacheGuard_EBS_NoCacheEntry_NoLiveFetch fixture shape: a real
// registered ng→ebs pivot whose State: RelatedUnknown comes from a cold
// ec2/ebs cache, not from a structurally-uncomputable checker.
func ngResourceForCacheMissBadge() resource.Resource {
	return resource.Resource{
		ID:   "prod-workers",
		Name: "prod-workers",
		Fields: map[string]string{
			"nodegroup_name": "prod-workers",
			"cluster_name":   "prod-cluster",
			"status":         "ACTIVE",
		},
	}
}

// TestRenderRelatedPanel_TransientUnknownNoFilter_ShowsNoBadge
// verifies the badge TEXT for a TRANSIENT resolved-unknown row (a real
// registered checker, e.g. ng→ebs, whose required cache entry has not warmed
// yet). Under the four-state contract "(?)" is FORBIDDEN: a resolved-unknown
// no-filter row shows NO count badge (blank) and is ACTIONABLE — a real drill
// target (Enter opens the target type's plain top-level list per
// qa_related_transient_unknown_drill_test.go) — so it renders BRIGHT
// (styles.RowNormal) like any other actionable row.
func TestRenderRelatedPanel_TransientUnknownNoFilter_ShowsNoBadge(t *testing.T) {
	ensureNoColor(t)
	m := newRelatedDimParityDetail("ng", ngResourceForCacheMissBadge())

	block := app.RelatedBlock{
		Name:         "EBS Volumes",
		State:        domain.RelatedUnknown,
		Count:        0,
		Approximate:  false,
		FetchFilter:  nil,
		TargetType:   "ebs",
		Actionable:   resource.IsRelatedActionable(domain.RelatedUnknown, 0, false),
		CountDisplay: resource.FormatRelatedCount(domain.RelatedUnknown, 0, false),
	}
	if !block.Actionable {
		t.Fatal("test setup: transient-unknown-no-filter row must be Actionable (four-state contract)")
	}
	if block.CountDisplay != "" {
		t.Fatalf("test setup: block.CountDisplay = %q, want \"\" (no badge — \"(?)\" is forbidden)", block.CountDisplay)
	}

	body := relatedDimParityBody([]app.RelatedBlock{block})
	rendered := m.RenderDetail(body)
	plain := stripAnsi(rendered)

	if strings.Contains(plain, "(?)") {
		t.Errorf("rendered related panel must NOT contain \"(?)\" (forbidden); got:\n%s", plain)
	}

	line := extractRelatedLine(t, rendered, "EBS Volumes")
	wantText := "  EBS Volumes"
	wantStyled := styles.RowNormal.Render(wantText)
	if line != wantStyled {
		t.Errorf("transient-unknown row not rendered bright (actionable) with no badge.\n  got:  %q\n  want: %q", line, wantStyled)
	}
}

// TestRenderRelatedPanel_ResolvedUnknownWithFilter_NoQuestionMarkBadge is the
// counterpart control: a State: RelatedDeferred FetchFilter pivot remains a
// plain, badge-less, ACTIONABLE row (the filtered fetch resolves the real
// count on entry), so "(?)" must never appear for this state.
func TestRenderRelatedPanel_ResolvedUnknownWithFilter_NoQuestionMarkBadge(t *testing.T) {
	ensureNoColor(t)
	for _, tc := range relatedDimParityTypes() {
		tc := tc
		t.Run(tc.shortName, func(t *testing.T) {
			m := newRelatedDimParityDetail(tc.shortName, tc.res)

			filter := map[string]string{"instance-id": "i-0abc123def456789a"}
			block := app.RelatedBlock{
				Name:         "CloudTrail Events",
				State:        domain.RelatedDeferred,
				Count:        0,
				Approximate:  false,
				FetchFilter:  filter,
				TargetType:   "ct-events",
				Actionable:   resource.IsRelatedActionable(domain.RelatedDeferred, 0, false),
				CountDisplay: resource.FormatRelatedCount(domain.RelatedDeferred, 0, false),
			}
			if !block.Actionable {
				t.Fatal("test setup: resolved-unknown-with-filter row must be Actionable")
			}
			if block.CountDisplay != "" {
				t.Fatalf("test setup: block.CountDisplay = %q, want \"\" (no badge for filtered pivots)", block.CountDisplay)
			}

			body := relatedDimParityBody([]app.RelatedBlock{block})
			rendered := m.RenderDetail(body)
			plain := stripAnsi(rendered)

			if strings.Contains(plain, "(?)") {
				t.Errorf("[%s] FetchFilter pivot (State: RelatedDeferred) must NOT render \"(?)\" badge; got:\n%s", tc.shortName, plain)
			}

			line := extractRelatedLine(t, rendered, "CloudTrail Events")
			wantText := "  CloudTrail Events"
			wantStyled := styles.RowNormal.Render(wantText)
			if line != wantStyled {
				t.Errorf("[%s] FetchFilter pivot (State: RelatedDeferred) must render bright with no badge.\n  got:  %q\n  want: %q", tc.shortName, line, wantStyled)
			}
		})
	}
}
