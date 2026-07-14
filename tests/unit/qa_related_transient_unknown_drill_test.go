// qa_related_transient_unknown_drill_test.go — pins FEATURE #38 (owner
// decision 2026-07-06): a related-panel row showing the transient "(?)"
// badge (resolved-unknown, cold-cache State: RelatedUnknown with NO
// FetchFilter — see resource.FormatRelatedCount / related_unknown_badge_test.go for the badge
// contract, and TestNGColdCacheGuard_EBS_NoCacheEntry_NoLiveFetch in
// aws_ng_cold_cache_guard_test.go for the real checker that produces this
// exact shape) must be ACTIONABLE end-to-end:
//
//  1. Enter on a "(?)" row opens the TARGET TYPE's plain top-level list —
//     the same navigation any menu entry would produce — not a filtered/
//     related-contextual list and not a no-op.
//  2. On returning to the detail (Esc), the pivot's count RECOMPUTES: once
//     the target type's cache is warm from the visit, the "(?)" becomes the
//     real number without a manual refresh (Ctrl+R).
//
// Fixture pair: "ng" (node group) -> "ebs" (EBS Volumes), the SAME pair
// related_unknown_badge_test.go and TestNGColdCacheGuard_EBS_NoCacheEntry_NoLiveFetch
// use. checkNGEBS (internal/aws/ng_related.go) returns
// resource.UnknownRelated("ebs") (State: RelatedUnknown, Count: 0) — no
// FetchFilter, no RelatedIDs — whenever the "ec2" RowStore entry is cold, because it joins
// against the EC2 cache by tag rather than issuing a live AWS call
// (docs/resources/ng.md §2 ebs bullet). This is constructible directly from
// the harness: build an ng resource.Resource by hand (mirroring
// ngResourceForCacheMissBadge in related_unknown_badge_test.go) and never
// load "ec2" resources before opening its detail — the checker result is
// then fed via a real messages.RelatedCheckResult, exactly as production's
// fan-out would deliver it.
//
// Registry scoping: "ng" registers NINE related defs in production
// (catalog_containers.go). The real keyboard-cursor machinery
// (detailSkipUnselectableRelated / stepToSelectable, internal/app/detail_cursor.go
// + actions_nav.go) only stops the cursor on a row that is ALREADY
// actionable — it can never land on a purely non-actionable row when no
// actionable row exists anywhere in the panel (stepToSelectable returns the
// cursor unchanged when no non-skippable index is found in either
// direction). Since a transient "(?)" row is, by definition, the exact state
// this feature makes actionable for the FIRST time, there is no way to reach
// it via Up/Down against the full 9-def panel pre-fix. This file scopes "ng"
// down to its real, single ebs def via resource.SetRelatedForTest (restored
// via t.Cleanup(CleanupRelatedForTest)) so the lone row starts focused at
// cursor 0 — the checker, DisplayName, and TargetType are all captured live
// from the production registry before scoping, so this is not a synthetic
// stand-in def.
//
// Harness follows related_circular_reentry_test.go: build a demo root model,
// drive it via rootApplyMsg/drainCmds, assert on the ANSI-stripped rendered
// view (tui.Model exposes no controller accessor from tests/unit, so every
// assertion here reads real, user-visible frame/footer content rather than
// internal state). Package unit (not unit_test) is required to reach those
// harness helpers.
//
// Root-cause seams this file pins (for the coder):
//
//   - Pin 1 (RED today): the live keyboard Enter path on a focused related
//     row is internal/tui/app_stack.go's handleDetailKeyMsg, case
//     `!rs.rightCol.IsFiltering() && key.Matches(msg, m.keys.Enter)`
//     (app_stack.go:315-346). It reads m.ctrl.SelectedRelatedRow() (the
//     controller-owned ds.RelatedCursor/ds.RelatedRows — NOT the renderer's
//     own RightColumnModel.rows) and gates on
//     resource.IsRelatedActionable(row.State, row.Count, row.Truncated).
//     For State: RelatedUnknown with no FetchFilter, IsRelatedActionable
//     now returns true (internal/resource/related.go:290-306) — the owner's
//     2026-07-06 decision made this transient (no-filter, resolved-unknown)
//     row actionable so Enter fires the RelatedNavigate dispatch.
//
//   - Pin 2 (RED today): even if Pin 1's gate is opened, ResolveRelatedNavigate
//     (internal/runtime/handlers_related.go) resolves a RelatedNavigate with
//     empty TargetID/RelatedIDs/FetchFilter to NavigationKindResourceList
//     (its documented "otherwise" fallback, case 7). The TUI adapter's
//     NavigationKindResourceList branch (internal/tui/runtime_adapter_related.go:392-406)
//     calls m.newRelatedList, which ALWAYS sets a title suffix
//     (runtime.RelatedTitleSuffix(src)) and SetEscPops(true) — a
//     related/contextual list, not "the same navigation any menu entry would
//     produce" (messages.Navigate{Target: TargetResourceList}, which carries
//     no title suffix and does not force EscPops). This test pins the
//     user-visible distinguishing facts rendered straight into the frame:
//     the frame TITLE (app_view.go's frameTitle -> ctrl.ListFrameTitle(),
//     rendered by layout.RenderFrameWithHints) must not carry the
//     RelatedTitleSuffix, and the footer must not show the "esc Back" hint
//     (internal/app/footer.go: buildListFooterHints only appends that hint
//     when ls.EscPops is true) that every related/contextual list forces.
//
//   - Pin 3 (RED today): app_input.go's Escape handler pops a related-drill
//     list with zero re-dispatch of any related-check machinery — the
//     `rs.kind == rsKindList && m.ctrl.GetListEscPops()` branch
//     (app_input.go:120-123) calls popRS() (app_stack.go's
//     popRS/popRSWithCtrlPop), a bare stack pop with no RelatedCheckStarted
//     dispatch anywhere in that path (contrast with the Detail-view Ctrl+R
//     handler in runtime_adapter_navigate.go:599-644, which explicitly
//     re-dispatches RelatedCheckStarted — Esc-return has no analogous hook).
//     The revealed detail's rendererState.rightCol is the SAME
//     RightColumnModel instance from before the drill (rendererState fields
//     live underneath the popped list on m.stack, untouched by popRS), so
//     its cached State: RelatedUnknown row for "EBS Volumes" is never
//     re-evaluated — the badge stays stale "(?)" forever without a manual
//     Ctrl+R. This test
//     pins that after Esc-return, once the target cache has warmed, the
//     ng->ebs row's badge must show the real resolved count, not the stale
//     "(?)".
package unit

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/demo/fakes"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
)

// transientUnknownNGResource returns a node-group resource.Resource with no
// registered fetcher of its own — matching ngResourceForCacheMissBadge in
// related_unknown_badge_test.go exactly, so this file's fixture is provably
// the same shape TestNGColdCacheGuard_EBS_NoCacheEntry_NoLiveFetch pins at
// the checker level.
func transientUnknownNGResource() resource.Resource {
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

// scopeNGToEBSOnly captures the REAL, live ng->ebs RelatedDef ("EBS Volumes",
// checkNGEBS) from the production registry, then re-registers "ng" with only
// that single def for the duration of the calling test — restored via
// t.Cleanup(resource.CleanupRelatedForTest). This makes the lone row the
// deterministic cursor-0 landing spot (see file header: the real
// keyboard-cursor skip machinery cannot land on a non-actionable row when no
// actionable row exists anywhere in a multi-row panel), while keeping every
// other fact about the def (TargetType, DisplayName, Checker) identical to
// production.
func scopeNGToEBSOnly(t *testing.T) resource.RelatedDef {
	t.Helper()
	var ebsDef resource.RelatedDef
	found := false
	for _, def := range resource.GetRelated("ng") {
		if def.TargetType == "ebs" {
			ebsDef = def
			found = true
			break
		}
	}
	if !found {
		t.Fatal("test setup: no ng->ebs RelatedDef registered in production (expected \"EBS Volumes\", checkNGEBS)")
	}
	resource.SetRelatedForTest("ng", []resource.RelatedDef{ebsDef})
	t.Cleanup(func() { resource.CleanupRelatedForTest("ng") })
	return ebsDef
}

// transientUnknownSetup builds a demo root model, opens the ng resource's
// DETAIL view via a real Navigate message, and feeds a single
// messages.RelatedCheckResult reproducing checkNGEBS's real cold-cache
// output (State: RelatedUnknown, no FetchFilter, no RelatedIDs) for the ng->ebs row —
// exactly what production's fan-out delivers when the "ec2" RowStore entry
// is cold. Deliberately does NOT load "ec2" resources, so the transient
// "(?)" state is genuine, not simulated past the checker boundary.
func transientUnknownSetup(t *testing.T) (tui.Model, resource.Resource, resource.RelatedDef) {
	t.Helper()

	def := scopeNGToEBSOnly(t)
	ngRes := transientUnknownNGResource()

	m := tui.New("demo", "us-east-1",
		tui.WithClients(demo.NewServiceClients()),
		tui.WithIsDemo(true),
		tui.WithNoCache(true),
		tui.WithProfile(demo.DemoProfile),
		tui.WithRegion(demo.DemoRegion))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 160, Height: 40})
	// tui.WithClients only seeds the option struct read at construction; the
	// runtime Core's own ServiceClients (what runtime/fetchers.go checks
	// before any live fetch, including the plain-list fetch Pin 2 drives)
	// is populated exclusively by a real messages.ClientsReady dispatch
	// (internal/tui/app_session.go's handleClientsReady ->
	// core.HandleClientsReady). Without this, TestTransientUnknownDrill_
	// EnterListShowsUnfilteredEBS's Enter-driven ebs fetch fails with "AWS
	// clients not initialized" and the pushed list renders "ebs(0) No
	// resources found" instead of the real demo EBS fixture rows — mirrors
	// demoClientsReadyMsg() in demo_app_test.go / TestDemoMode_Init_NoAWSConnection.
	m, _ = rootApplyMsg(m, messages.ClientsReady{Clients: demo.NewServiceClients()})

	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		ResourceType: "ng",
		Resource:     &ngRes,
	})

	m, _ = rootApplyMsg(m, messages.RelatedCheckResult{
		ResourceType:     "ng",
		SourceResourceID: ngRes.ID,
		DefDisplayName:   def.DisplayName,
		Result:           resource.UnknownRelated("ebs"),
	})

	view := stripANSI(rootViewContent(m))
	if !strings.Contains(view, "detail -- "+ngRes.ID) {
		t.Fatalf("setup did not land on ng detail (\"detail -- %s\"); got:\n%s", ngRes.ID, view)
	}
	// Four-state contract: a transient resolved-unknown row shows NO count badge
	// (blank), never "(?)". It must be present and badge-less before the drill.
	if !strings.Contains(view, def.DisplayName) || strings.Contains(view, def.DisplayName+" (") {
		t.Fatalf("test setup: ng detail's related panel must show %q as a blank (no-count) transient row before driving Enter; got view:\n%s",
			def.DisplayName, view)
	}

	return m, ngRes, def
}

// focusRelatedRow focuses the detail's right column via Tab
// (keys.Default().Tab, "tab" — internal/tui/app_stack.go's handleDetailKeyMsg
// case key.Matches(msg, m.keys.Tab) toggles rs.rightCol's focus, and
// ActionToggleFocus initializes the controller's RelatedCursor to 0). Since
// scopeNGToEBSOnly leaves exactly one row registered for "ng", the freshly
// focused right column's cursor lands on the ebs row without any extra
// Up/Down navigation — sidestepping the real cursor-skip machinery, which
// (see file header) cannot land on a non-actionable row when other rows
// exist.
func focusRelatedRow(m tui.Model) tui.Model {
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyTab})
	return m
}

// ---------------------------------------------------------------------------
// Pin 1 + Pin 2: Enter on a transient "(?)" row opens the target type's
// PLAIN top-level list — not a no-op, not a filtered/related-contextual list.
// ---------------------------------------------------------------------------

// TestTransientUnknownDrill_EnterOpensPlainTopLevelList verifies the FULL
// contract of owner decision #38 bullet 1: Enter on the ng->ebs "(?)" row
// must push a plain, unfiltered "ebs" list — the same navigation any menu
// entry would produce (messages.Navigate{Target: TargetResourceList}) — not
// a related/contextual list and not a silent no-op.
//
// Distinguishing signals, read straight off the rendered frame (real,
// user-visible facts, not implementation internals):
//   - The view must actually change (pre-#38: resource.IsRelatedActionable
//     for a resolved-unknown row with no FetchFilter was false, so
//     handleDetailKeyMsg's Enter case returned early and the ng detail
//     stayed on screen; #38 flipped RelatedUnknown to actionable).
//   - The pushed list's rendered frame TITLE must NOT carry the
//     RelatedTitleSuffix (" -- prod-workers (prod-workers)") that
//     runtime.RelatedTitleSuffix unconditionally appends in the
//     related-list path (newRelatedList, internal/tui/related_helpers.go) —
//     a plain menu-driven list never carries this suffix.
//   - The rendered footer must NOT show the "esc Back" hint: a
//     related/contextual list always calls SetEscPops(true)
//     (related_helpers.go), which is the ONLY thing that puts "esc Back" in
//     the footer (internal/app/footer.go buildListFooterHints); a
//     menu-driven TargetResourceList list leaves EscPops at its false
//     default and never shows that hint.
func TestTransientUnknownDrill_EnterResolvesInPlaceStaysOnDetail(t *testing.T) {
	m, ngRes, def := transientUnknownSetup(t)
	m = focusRelatedRow(m)

	m, cmd := rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	m, _ = drainCmds(t, m, cmd, 6)

	view := stripANSI(rootViewContent(m))
	// Fix #3: a scoreless row (no ResourceIDs, no FetchFilter) RESOLVES IN
	// PLACE — the ng detail stays rendered on top; had a list been pushed the
	// view would render that list instead of "detail -- <id>".
	if !strings.Contains(view, "detail -- "+ngRes.ID) {
		t.Fatalf("BUG: Enter on the scoreless row (%s) must RESOLVE IN PLACE (stay on the ng detail), not navigate to a list; view:\n%s",
			def.DisplayName, view)
	}
}

// TestTransientUnknownDrill_EnterRecomputesInPlace verifies that once the
// "ec2" cache the ng->ebs pivot joins against is warm, Enter on the scoreless
// row re-dispatches the checks and the row firms up to a numeric badge — in
// place, still on the ng detail, with no target list ever pushed.
func TestTransientUnknownDrill_EnterRecomputesInPlace(t *testing.T) {
	m, ngRes, def := transientUnknownSetup(t)
	m = focusRelatedRow(m)

	ec2Client := fakes.NewEC2()
	ec2Res, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchEC2InstancesPage(t.Context(), ec2Client, token)
	})
	if err != nil || len(ec2Res) == 0 {
		t.Fatalf("demo ec2 fixtures missing (err=%v, len=%d)", err, len(ec2Res))
	}
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{ResourceType: "ec2", Resources: ec2Res})

	m, cmd := rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	m, _ = drainCmds(t, m, cmd, 8)

	view := stripANSI(rootViewContent(m))
	if !strings.Contains(view, "detail -- "+ngRes.ID) {
		t.Fatalf("scoreless-row Enter must stay on the ng detail (resolve in place); view:\n%s", view)
	}
	if !strings.Contains(view, def.DisplayName+" (") {
		t.Fatalf("BUG: scoreless-row Enter must RECOMPUTE %q to a numeric badge in place; row still blank. View:\n%s",
			def.DisplayName, view)
	}
}

// (Owner decision #38's "return via Esc recomputes" pin is retired: fix #3
// makes a scoreless row resolve in place on the drill itself, so there is no
// list to Esc back from — see TestTransientUnknownDrill_EnterRecomputesInPlace.)
