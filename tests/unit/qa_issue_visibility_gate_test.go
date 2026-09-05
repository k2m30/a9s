// qa_issue_visibility_gate_test.go — the standing OWNER RULE gate: if a
// resource row carries a PROBLEM signal — a row color with IsIssue()==true,
// or an issue-severity Finding (SevWarn/SevBroken; a glyph decorator is just
// the render-time echo of that same Finding, see isVisibilityViolation) — the
// problem must be visible either (a) in the list view's Status column cell,
// or (b) at the top of the detail view (the Attention block).
//
// SCOPE: Dim-ONLY rows (color == ColorDim, findings all SevDim, e.g. a
// ct-events "routine event", a Lambda Inactive function, a deleted SNS
// subscription) are OUT of scope by design — SevDim is a neutral/routine
// state, not a problem, per domain.Severity.IsIssue() and the same "dim =
// neutral" convention used elsewhere (related-panel rows, menu entries). See
// isVisibilityViolation for the exact predicate and rationale.
//
// Registry+demo-driven, one subtest per (type, resource) violation, mirroring
// the ratchet style of qa_demo_state_coverage_test.go / qa_demo_pivot_coverage_test.go:
//
//  1. Drain each registered type's demo fixtures via its real Wave-1 Fetcher
//     (drainDemoFixtures, shared with the state-coverage/pivot-coverage
//     tests), building one shared resource.ResourceCache exactly as those
//     tests do so cross-type Wave-2 enrichers see sibling data.
//  2. Run the type's registered Wave-2 IssueEnricher (awsclient.Wave2EnricherFor)
//     against the fixtures and fold any per-resource Finding into a COPY of
//     that resource's Findings slice — mirroring what production's
//     applyWave2ToRow (internal/tui/app_enrich_fold.go) does before render,
//     without reimplementing that fold's full mechanics: the Color funcs and
//     Attention builder only ever read Resource.Findings, so appending the
//     Wave-2 finding there reproduces the same input those consumers see.
//  3. For every (type, resource) that isVisibilityViolation flags (in-scope
//     PROBLEM signal, not a Dim-only state), verify:
//     (a) the REAL list Status cell for that row (driven through
//     Controller.ApplyResourcesLoaded + Snapshot().Body.List, indexed by
//     ListBody.StatusCol) is non-empty, OR
//     (b) the REAL detail Attention block for that row (driven through
//     Controller.EnsureDetailState + Snapshot().Body.Detail, filtered by
//     FieldRow.Path=="Attention") is non-empty.
//
// RATCHET semantics (identical contract to knownStateCoverageGaps /
// knownDisconnectedPivots):
//   - A violation NOT in knownVisibilityGaps is a NEW regression — always
//     fails, unconditionally.
//   - An allowlisted violation that NOW shows the problem on both surfaces
//     fails with a "remove from allowlist" message.
//   - An allowlisted violation still showing on neither surface is skipped
//     (logged), pre-existing debt.
package unit_test

import (
	"context"
	"fmt"
	"sort"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/session"
	unit "github.com/k2m30/a9s/v3/tests/unit"
)

// knownVisibilityGaps pins the exact inventory of (type, resource-key)
// violations found by this gate: a row with an in-scope PROBLEM signal (see
// isVisibilityViolation — IsIssue()==true color or an issue-severity
// Finding; Dim-only/neutral rows never reach this predicate at all) yet
// showing the problem on NEITHER the list Status cell NOR the detail
// Attention block. Same burn-down semantics as knownStateCoverageGaps in
// qa_demo_state_coverage_test.go:
//   - present + still invisible on both surfaces today -> skip (logged),
//     expected pre-existing debt.
//   - present + now visible on either surface           -> FAIL ("remove
//     from allowlist").
//   - a violation NOT present here                      -> FAIL
//     unconditionally, a new regression the allowlist was never told about.
//
// Key shape: "<shortName>:<resourceID>" so per-type resource IDs never
// collide across types.
//
// TERMINAL STATE (2026-07-06, after the wave1-findings conversion —
// asg/ct-events/lambda/logs/policy/r53/rtb/secrets/sg/sns-sub/ssm — and the
// gate's own scope refinement to PROBLEM-only signals): EMPTY. Every
// (type, resource) pair that carries an issue-severity color or Finding now
// shows the problem on the list Status cell or the detail Attention block.
// The previous 11-entry inventory here (9 ct-events "routine event" rows,
// lambda's Inactive-runtime fixture, sns-sub's Deleted fixture) was never a
// visibility BUG — every one of those rows is Dim-only (SevDim Finding, Dim
// color), which is a neutral/routine state, not a problem, per
// domain.Severity.IsIssue()/domain.Color.IsIssue() (same precedent as
// pivot-coverage's structurally-uncomputable-pivot exclusions). They were
// removed once the predicate stopped flagging Dim-only rows as violations,
// not because a fix made them visible. If this map ever needs a new entry
// again, it means a genuine issue-severity row is invisible on both
// surfaces — a real regression, not a Dim/neutral non-issue.
var knownVisibilityGaps = map[string]bool{}

// drainVisibilityFixtures drains a type's demo rows through its own Wave-1
// Fetcher, via the same helper every other bench in this directory uses.
func drainVisibilityFixtures(t *testing.T, td resource.ResourceTypeDef, clients *awsclient.ServiceClients) ([]resource.Resource, bool) {
	t.Helper()
	return unit.DrainFixtures(t, td, clients)
}

// buildVisibilityTypeCache drains every registered type's demo fixtures and
// returns both the per-type resource lists and one shared resource.ResourceCache
// built from all of them, so cross-type Wave-2 enrichers see sibling data
// exactly as production does. Mirrors buildDemoStateTypeCache.
func buildVisibilityTypeCache(t *testing.T) (map[string][]resource.Resource, resource.ResourceCache) {
	t.Helper()
	clients := demo.NewServiceClients()
	byType := make(map[string][]resource.Resource)
	cache := make(resource.ResourceCache)

	for _, td := range resource.AllResourceTypes() {
		fixtures, ok := drainVisibilityFixtures(t, td, clients)
		if !ok {
			continue
		}
		byType[td.ShortName] = fixtures
		if len(fixtures) > 0 {
			cache[td.ShortName] = resource.ResourceCacheEntry{Resources: fixtures, IsTruncated: false}
		}
	}
	return byType, cache
}

// mergeWave2Findings runs the type's registered Wave-2 IssueEnricher (if any)
// against fixtures and returns a COPY of fixtures folded the way the running
// app folds them — through runtime.ApplyWave2ToRow, the same call the
// enrichment handler makes.
//
// Folding by hand here is what let every wave-2 supporting row escape the
// rendered-surface gates: a hand-rolled merge that appends Findings and
// ignores IssueEnricherResult.AttentionDetails builds a row the app never
// produces, so a gate reading it scans a detail block missing exactly the
// rows this batch added. Any divergence between test fold and app fold is a
// blind spot by construction, so there is only one fold.
func mergeWave2Findings(
	t *testing.T,
	td resource.ResourceTypeDef,
	fixtures []resource.Resource,
	cache resource.ResourceCache,
	clients *awsclient.ServiceClients,
) []resource.Resource {
	t.Helper()
	merged := make([]resource.Resource, len(fixtures))
	copy(merged, fixtures)

	enricher, ok := awsclient.Wave2EnricherFor(td.ShortName)
	if !ok || enricher.Fn == nil {
		return merged
	}
	result, err := enricher.Fn(context.Background(), clients, fixtures, cache)
	if err != nil {
		t.Fatalf("%s: Wave-2 enricher returned error: %v", td.ShortName, err)
	}
	for i := range merged {
		runtime.ApplyWave2ToRow(&merged[i], td, result.Findings, result.AttentionDetails)
	}
	return merged
}

// newVisibilityListController builds a Controller pre-navigated to the list
// screen for shortName, isolated per test via a temp config dir.
func newVisibilityListController(t *testing.T, shortName string) *app.Controller {
	t.Helper()
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	s := session.New()
	s.Profile = "test-profile"
	s.Region = "us-east-1"
	core := runtime.New(s, nil)
	c := app.New(core)
	t.Cleanup(c.Close)
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: shortName})
	return c
}

// listStatusCellFor drives the REAL list body build (Controller.ApplyResourcesLoaded
// + Snapshot().Body.List) for the given type and fixture set, and returns the
// Status-column cell value for the row matching resourceID. Returns ("", false)
// if the row or the status column cannot be located.
func listStatusCellFor(t *testing.T, td resource.ResourceTypeDef, fixtures []resource.Resource, resourceID string) (string, bool) {
	t.Helper()
	c := newVisibilityListController(t, td.ShortName)
	c.ApplyResourcesLoaded(td.ShortName, fixtures, nil, false)
	body := c.Snapshot().Body.List
	if body == nil {
		return "", false
	}
	if body.StatusCol < 0 {
		return "", false
	}
	for _, row := range body.Rows {
		if row.ResourceID != resourceID {
			continue
		}
		if body.StatusCol >= len(row.Cells) {
			return "", false
		}
		return row.Cells[body.StatusCol], true
	}
	return "", false
}

// newVisibilityDetailController builds a Controller pushed straight to the
// detail screen, isolated per test via a temp config dir.
func newVisibilityDetailController(t *testing.T) *app.Controller {
	t.Helper()
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	s := session.New()
	s.Profile = "test-profile"
	s.Region = "us-east-1"
	core := runtime.New(s, nil)
	c := app.New(core)
	t.Cleanup(c.Close)
	c.ApplyIntents([]runtime.UIIntent{
		runtime.PushScreen{ID: runtime.ScreenDetail},
	})
	return c
}

// detailHasAttentionFor drives the REAL detail body build (Controller.EnsureDetailState
// + Snapshot().Body.Detail) for a single resource and reports whether the
// Attention block (FieldRow.Path=="Attention") is non-empty. EnsureDetailState
// seeds DetailState.Findings from res.Findings directly (core/app/detail_state.go),
// so a resource carrying Wave-1/merged-Wave-2 Findings surfaces them here
// without any extra enrichment call.
func detailHasAttentionFor(t *testing.T, res resource.Resource, shortName string) bool {
	t.Helper()
	c := newVisibilityDetailController(t)
	c.EnsureDetailState(res, shortName)
	body := c.Snapshot().Body.Detail
	if body == nil {
		return false
	}
	for _, f := range body.Fields {
		if f.Path == "Attention" {
			return true
		}
	}
	return false
}

// isVisibilityViolation reports whether res is a candidate for the OWNER
// RULE: the rule triggers on PROBLEM signals — a row color with
// IsIssue()==true (domain.Color.IsIssue: Warning/Broken), OR at least one
// issue-severity Finding (domain.Severity.IsIssue: SevWarn/SevBroken) —
// Wave-1 seeded or Wave-2 merged. A glyph decorator (resolveListDecoratorFull
// in core/app/list_columns.go) only ever fires from that same
// issue-severity Finding check (the "healthy color + hidden issue-severity
// Finding" case), so it is already covered by the Finding leg above and is
// not a separate condition to test here.
//
// Dim-ONLY rows (color == ColorDim, and every Finding present is SevDim) are
// explicitly OUT of scope: SevDim is a neutral/routine state by the app's own
// severity design (domain.Severity.IsIssue excludes it, matching the same
// "dim = neutral" convention used by related-panel rows and menu entries),
// not a problem the OWNER RULE is meant to police. A ct-events "routine
// event" tier, a Lambda Inactive function, or a deleted SNS subscription are
// all Dim-only under this design — they are states to observe, not issues to
// surface via Status cell / Attention, so they never become gate violations
// regardless of whether a Finding is attached.
func isVisibilityViolation(td resource.ResourceTypeDef, res resource.Resource) bool {
	if hasIssueSeverityFinding(res.Findings) {
		return true
	}
	if td.Color == nil {
		return false
	}
	return td.ResolveColor(res).IsIssue()
}

// hasIssueSeverityFinding reports whether findings contains at least one
// Finding whose Severity.IsIssue() is true (SevWarn/SevBroken) — SevDim
// findings never count, mirroring domain.Severity.IsIssue itself.
func hasIssueSeverityFinding(findings []domain.Finding) bool {
	for _, f := range findings {
		if f.Severity.IsIssue() {
			return true
		}
	}
	return false
}

// TestIssueVisibilityGate_EveryColoredOrFlaggedRowIsVisibleSomewhere is the
// standing OWNER RULE gate. For every registered type with a Wave-1 Fetcher,
// every fixture resource that isVisibilityViolation flags as an in-scope
// PROBLEM signal (issue-severity color or Finding — Wave-1 seeded or Wave-2
// merged; Dim-only/neutral rows are out of scope, see isVisibilityViolation)
// must show the problem on the list Status cell or the detail Attention
// block — UNLESS the (type, resourceID) pair is pinned in knownVisibilityGaps
// as pre-existing debt, in which case it is skipped (logged) instead of
// failed.
func TestIssueVisibilityGate_EveryColoredOrFlaggedRowIsVisibleSomewhere(t *testing.T) {
	clients := demo.NewServiceClients()
	byType, cache := buildVisibilityTypeCache(t)

	types := resource.AllResourceTypes()
	sort.Slice(types, func(i, j int) bool { return types[i].ShortName < types[j].ShortName })

	var stillGapped []string
	var newlyRegressed []string
	var readyForBurnDown []string

	for _, td := range types {
		fixtures := byType[td.ShortName]
		if len(fixtures) == 0 {
			continue
		}

		merged := mergeWave2Findings(t, td, fixtures, cache, clients)

		for _, res := range merged {
			if !isVisibilityViolation(td, res) {
				continue
			}

			key := td.ShortName + ":" + res.ID
			testName := fmt.Sprintf("%s/%s", td.ShortName, res.ID)

			t.Run(testName, func(t *testing.T) {
				statusCell, _ := listStatusCellFor(t, td, merged, res.ID)
				listVisible := statusCell != ""

				detailVisible := detailHasAttentionFor(t, res, td.ShortName)

				visible := listVisible || detailVisible
				allowlisted := knownVisibilityGaps[key]

				color := "healthy"
				if td.Color != nil {
					color = bucketName(td.ResolveColor(res))
				}

				switch {
				case visible && allowlisted:
					readyForBurnDown = append(readyForBurnDown, key)
					t.Errorf(
						"BURN-DOWN: %s (color=%s, findings=%d) is now visible (list=%q detail-attention=%v) "+
							"but is still pinned in knownVisibilityGaps — remove %q from the allowlist in this PR",
						key, color, len(res.Findings), statusCell, detailVisible, key,
					)
				case visible:
					// Visible on at least one surface and not allowlisted — expected steady state.
				case allowlisted:
					stillGapped = append(stillGapped, key)
					t.Skipf(
						"KNOWN GAP (allowlisted): %s (color=%s, findings=%d): neither list Status cell "+
							"(%q) nor detail Attention block shows the problem — pre-existing debt, see "+
							"knownVisibilityGaps", key, color, len(res.Findings), statusCell,
					)
				default:
					newlyRegressed = append(newlyRegressed, key)
					t.Errorf(
						"NEW VIOLATION (not allowlisted): %s: resource %q (type=%s, color=%s, findings=%d) "+
							"is colored non-healthy or carries findings but shows the problem on NEITHER "+
							"the list Status cell (got %q) NOR the detail Attention block. Either fix the "+
							"Status-cell/Attention wiring or, if this is pre-existing debt, add %q to "+
							"knownVisibilityGaps",
						key, res.ID, td.ShortName, color, len(res.Findings), statusCell, key,
					)
				}
			})
		}
	}

	if len(newlyRegressed) > 0 {
		t.Logf("NEW VIOLATION INVENTORY (%d): %v", len(newlyRegressed), newlyRegressed)
	}
	if len(readyForBurnDown) > 0 {
		t.Logf("READY-FOR-BURN-DOWN INVENTORY (%d): %v", len(readyForBurnDown), readyForBurnDown)
	}
	if len(stillGapped) > 0 {
		t.Logf("STILL-GAPPED (allowlisted, skipped) INVENTORY (%d): %v", len(stillGapped), stillGapped)
	}
}
