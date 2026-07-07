// related_unknown_badge_test.go — pins the user-visible fix where a RESOLVED
// unknown related count (State: RelatedUnknown, not loading, no error, no
// FetchFilter) renders as an explicit "(?)" badge instead of an empty/no-badge
// row.
//
// Contract under test (resource.FormatRelatedCount, internal/resource/related.go):
//   - State: RelatedDeferred (server-side FetchFilter pivot) → ""    (actionable
//     drill-in link; the filtered re-fetch resolves the real count once
//     entered, so no misleading "(?)" badge is shown on what is really a
//     navigable pivot)
//   - State: RelatedUnknown                                 → "(?)" (resolved
//     unknown — a computable pivot whose required cache entry is cold, e.g.
//     ng→ebs before the ec2 cache has warmed; see
//     TestNGColdCacheGuard_EBS_NoCacheEntry_NoLiveFetch in
//     aws_ng_cold_cache_guard_test.go for the checker-level contract this
//     badge represents. Must read as "we tried and can't tell you yet",
//     distinct from the loading state, which never reaches this function)
//   - State: RelatedResolved, count == 0                     → "(0)"
//   - State: RelatedResolved, count == N (N > 0)              → "(N)"
//
// A prior revision of this file fixtured the no-filter case as a
// budget-excluded/structurally-uncomputable pivot (e.g. kms→s3). That framing
// no longer applies: budget-excluded pivots (the knownDisconnectedPivots set)
// are being removed from the registry entirely by a parallel change, so their
// rows never reach the panel at all. The remaining resolved-unknown-no-filter
// case is exclusively the TRANSIENT one — a real registered checker whose
// backing cache entry has not warmed yet — so this file fixtures that
// scenario.
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
		name  string
		state domain.RelatedRowState
		count int
		want  string
	}{
		{"ResolvedUnknown_NoFilter", domain.RelatedUnknown, 0, "(?)"},
		{"ResolvedUnknown_WithFilter_NoBadge", domain.RelatedDeferred, 0, ""},
		{"ExactZero_NoFilter", domain.RelatedResolved, 0, "(0)"},
		{"ExactZero_WithFilter", domain.RelatedResolved, 0, "(0)"},
		{"Positive_NoFilter", domain.RelatedResolved, 7, "(7)"},
		{"Positive_WithFilter", domain.RelatedResolved, 7, "(7)"},
		{"LargeCount", domain.RelatedResolved, 1000, "(1000)"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := resource.FormatRelatedCount(tc.state, tc.count)
			if got != tc.want {
				t.Errorf("FormatRelatedCount(state=%v, %d) = %q, want %q",
					tc.state, tc.count, got, tc.want)
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

// TestRenderRelatedPanel_TransientUnknownNoFilter_ShowsQuestionMarkBadge
// verifies the badge TEXT for a TRANSIENT resolved-unknown row (a real
// registered checker, e.g. ng→ebs, whose required cache entry has not warmed
// yet) — as opposed to a budget-excluded/structurally-uncomputable pivot,
// which is removed from the registry entirely rather than rendered with a
// badge. The "(?)" badge text itself is untouched by owner decision #38
// (resource.FormatRelatedCount's count<0/no-filter branch is unchanged); only
// the row's actionability/styling flipped — a transient "(?)" row is now
// actionable (a real drill target: Enter opens the target type's plain
// top-level list per qa_related_transient_unknown_drill_test.go), so it
// renders BRIGHT (styles.RowNormal) like any other actionable row, not dim.
func TestRenderRelatedPanel_TransientUnknownNoFilter_ShowsQuestionMarkBadge(t *testing.T) {
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
		CountDisplay: resource.FormatRelatedCount(domain.RelatedUnknown, 0),
	}
	if !block.Actionable {
		t.Fatal("test setup: transient-unknown-no-filter row must be Actionable (owner decision #38)")
	}
	if block.CountDisplay != "(?)" {
		t.Fatalf("test setup: block.CountDisplay = %q, want \"(?)\"", block.CountDisplay)
	}

	body := relatedDimParityBody([]app.RelatedBlock{block})
	rendered := m.RenderDetail(body)
	plain := stripAnsi(rendered)

	if !strings.Contains(plain, "(?)") {
		t.Errorf("rendered related panel missing \"(?)\" badge for transient-unknown row; got:\n%s", plain)
	}

	line := extractRelatedLine(t, rendered, "EBS Volumes")
	wantText := "  EBS Volumes (?)"
	wantStyled := styles.RowNormal.Render(wantText)
	if line != wantStyled {
		t.Errorf("transient-unknown row not rendered bright (actionable) with \"(?)\" badge.\n  got:  %q\n  want: %q", line, wantStyled)
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
				CountDisplay: resource.FormatRelatedCount(domain.RelatedDeferred, 0),
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
