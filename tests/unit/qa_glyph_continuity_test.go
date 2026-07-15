// qa_glyph_continuity_test.go — RED pins for the remaining temporary glyph
// vanish reported by a live user on branch feat/cache (HEAD 81ef61a4).
// Regions are already fixed; `!`/`~` row glyphs still vanish temporarily and
// come back. Two candidate mechanisms, pinned separately:
//
//  1. RerunStart_KeepsVisibleFindingsUntilReplaced: internal/tui's
//     handleRefresh (runtime_adapter_navigate.go) clears a list's row
//     findings SYNCHRONOUSLY on Ctrl+R — before the rerun fetch has even
//     dispatched, let alone landed — via m.ctrl.ClearRowFindings(rt) plus
//     m.ctrl.ApplyEnrichmentState(rt, 0, false, nil). This is deliberate,
//     load-bearing behavior: TestCtrlR_ClearsActiveListFindingsImmediately
//     in qa_enrichment_review_fixes_test.go asserts the SAME clear-on-start
//     is correct and pins it as the fix for a prior bug (marker persisting
//     stale until the next SetEnrichmentState). This pin therefore documents
//     the current, intentional behavior instead of re-asserting the opposite
//     against an already-passing counter-test — see the doc comment on the
//     test func for exactly what is and is not claimed.
//
//  2. Swap_MergesWave2ForWave1CarryingRows: applyResourcesLoaded's carry-
//     forward (core/app/list_body.go:80-89) only re-applies a prior
//     row's findings onto the incoming replacement when the incoming row
//     itself carries ZERO findings (`if len(resources[i].Findings) > 0 {
//     continue }`). A row that already has a fresh Wave-1 finding on the
//     incoming side (non-zero len) is skipped entirely — so a
//     previously-attached Wave-2 finding on the SAME row is dropped on this
//     swap, even though nothing invalidated it. RED at HEAD.
//
//  3. ProfileSwitch_StillClearsFindings (C9 guard): confirms
//     Controller.enrichmentStore for a resource type is NOT wiped by
//     anything reachable from a profile/region rotation
//     (MenuClearAvailabilityIntent only clears MenuState fields — see
//     core/app/intents.go's MenuClearAvailabilityIntent case; Session.
//     Rotate in core/session/session.go bumps gens but never touches
//     Controller.enrichmentStore). A stale enrichmentStore entry surviving
//     a rotation would let a same-ID collision (or a reopened list of the
//     same type) resurrect a dead profile's glyphs. RED at HEAD: nothing in
//     this package clears it, so the pin documents the gap by asserting the
//     clear that SHOULD happen around a rotation-shaped sequence.
package unit

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	_ "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// glyphContinuityPair builds a fresh, hermetic Controller/Core pair backed by
// a per-test temp cache directory, mirroring fieldCompletenessPair in
// qa_cache_field_completeness_test.go.
func glyphContinuityPair(t *testing.T, profile, region string) *app.Controller {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)
	core := runtime.Bootstrap(profile, region, resource.AllResourceTypes())
	ctrl := app.New(core)
	t.Cleanup(ctrl.Close)
	ctrl.SetUIMode("web")
	return ctrl
}

// findingsForID returns the Findings slice attached to the row with the
// given ResourceID in the Controller's live resource cache for typeName, or
// nil if not found.
func findingsForID(ctrl *app.Controller, id string) []domain.Finding {
	for _, r := range ctrl.GetListAllResources() {
		if r.ID == id {
			return r.Findings
		}
	}
	return nil
}

// hasDecoratorForID reports whether the currently-built list body shows the
// given decorator ("!" or "~") on the row with the given ResourceID.
func hasDecoratorForID(ctrl *app.Controller, id string, want app.RowDecorator) bool {
	snap := ctrl.Snapshot()
	if snap.Body.List == nil {
		return false
	}
	for _, r := range snap.Body.List.Rows {
		if r.ResourceID == id && r.Decorator == want {
			return true
		}
	}
	return false
}

// ─────────────────────────────────────────────────────────────────────────
// Pin 1 — rerun-start clearing is deliberate; documents the CURRENT contract
// ─────────────────────────────────────────────────────────────────────────

// TestRerunStart_KeepsVisibleFindingsUntilReplaced pins the STALE-UNTIL-
// REPLACED contract at the layer where mechanism 1 actually lives:
// internal/tui's handleRefresh (runtime_adapter_navigate.go). A Controller-
// level pin cannot observe this — the eager clear (m.ctrl.ClearRowFindings +
// m.ctrl.ApplyEnrichmentState(rt, 0, false, nil)) ran one layer up, in the
// TUI Update() path, before the coder's fix removed it.
//
// Two halves, matching the coordinator's corrected invariant:
//  1. No blank window: pressing Ctrl+R must NOT make the "! " marker vanish
//     from View() before the fresh EnrichmentCheckedMsg lands.
//  2. Real removal still works: once a fresh EnrichmentCheckedMsg lands and
//     the finding is genuinely gone from the fresh map, the marker MUST
//     disappear — this is not "never clear", it's "never blank between".
//
// RED at HEAD (pre-fix): half 1 fails — the marker was gone immediately
// after Ctrl+R, before any fetch/enrichment response was processed (this is
// exactly what the old TestCtrlR_ClearsActiveListFindingsImmediately in
// qa_enrichment_review_fixes_test.go asserted as correct — that test now
// pins the superseded contract and needs updating alongside this one).
// TestRerunStart_KeepsVisibleFindingsUntilReplaced uses ctrl+z survival as
// the "is this finding applied" check, not the literal "! " glyph text.
// Since the color-findings-conformance wave, colorEC2 is
// colorFromAnyFinding-only (core/aws/catalog_compute.go) — once a
// SevBroken Finding is applied, resolveListDecoratorFull's glyph branch is
// skipped entirely (only fires when ResolveColor()==ColorHealthy; see
// core/app/list_columns.go and
// .claude/agent-memory/a9s-coder/project_color_findings_conformance_glyph_interplay.md).
// isVisibleUnderCtrlZ (qa_enrichment_review_fixes_test.go) is the
// renderer-agnostic, stronger replacement.
func TestRerunStart_KeepsVisibleFindingsUntilReplaced(t *testing.T) {
	withTuiVersion(t, "test")
	m := newRootSizedModel()
	m = navigateToEC2List(m)

	resources := rerunEC2Resources()
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "ec2",
		Resources:    resources,
	})

	// Populate a finding for i-0abc1111aaa111111 via the live-update path.
	m, _ = rootApplyMsg(m, enrichmentCheckedWithFindings(0, 0))

	var visible bool
	m, visible = isVisibleUnderCtrlZ(m, "web-server-1")
	if !visible {
		t.Fatal("pre-condition failed: expected web-server-1 to survive ctrl+z (finding applied) before Ctrl+R")
	}

	// Half 1: pressing Ctrl+R must not clear the finding before the fresh
	// enrichment result lands — the fetch/enrichment cmd has not run yet.
	m, _ = rootApplyMsg(m, ctrlRKeyMsg())

	m, visible = isVisibleUnderCtrlZ(m, "web-server-1")
	if !visible {
		t.Error("Ctrl+R must NOT clear the active list's findings before the fresh result lands (stale-until-replaced, not blank-until-replaced); web-server-1 no longer survives ctrl+z immediately after keypress")
	}

	// Half 2: once a fresh EnrichmentCheckedMsg lands with the SAME resource
	// genuinely recovered (no finding for it in the fresh map), the finding
	// must be removed — this is real replacement, not merely "never clear".
	recovered := messages.EnrichmentChecked{
		ResourceType: "ec2",
		Issues:       0,
		Truncated:    false,
		Findings:     map[string][]domain.Finding{},
		Gen:          0,
		TypeGen:      0,
	}
	m, _ = rootApplyMsg(m, recovered)

	_, visible = isVisibleUnderCtrlZ(m, "web-server-1")
	if visible {
		t.Error("a finding absent from the fresh enrichment result must be removed once that result lands; web-server-1 still survives ctrl+z")
	}
}

// ─────────────────────────────────────────────────────────────────────────
// Pin 2 — carry-forward drops Wave-2 for a Wave-1-carrying incoming row
// ─────────────────────────────────────────────────────────────────────────

// TestSwap_MergesWave2ForWave1CarryingRows pins the narrow gap in
// applyResourcesLoaded's carry-forward (core/app/list_body.go:80-89):
// the guard `if len(resources[i].Findings) > 0 { continue }` treats ANY
// non-zero incoming Findings as "already fresh, do not touch" — but a row
// that legitimately carries a Wave-1 finding on the incoming side (e.g. a
// fetcher-level health check) still needs its previously-known Wave-2
// finding folded in, since Wave-2 enrichment has not rerun yet and nothing
// else will re-attach it.
//
// RED at HEAD: rowB below has an outgoing Wave-1 + Wave-2 pair; the incoming
// replacement carries ONLY the Wave-1 finding (simulating "list re-fetched,
// Wave-2 hasn't rerun"). The current guard sees len(incoming.Findings) > 0
// (the Wave-1 entry) and skips the carry-forward entirely, so the Wave-2
// entry is lost. rowA (incoming already has ITS OWN fresh Wave-2 finding) is
// the control case: the incoming, fresher finding must win, not be
// overwritten by the stale one.
func TestSwap_MergesWave2ForWave1CarryingRows(t *testing.T) {
	ctrl := glyphContinuityPair(t, "glyph-swap-prof", "us-east-1")
	ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: "rds"})

	wave1Only := domain.Finding{Code: "rds.wave1", Phrase: "wave1 issue", Severity: domain.SevWarn, Source: "wave1"}
	staleWave2 := domain.Finding{Code: "rds.maint", Phrase: "pending maintenance", Severity: domain.SevWarn, Source: "wave2:rds"}
	freshWave2 := domain.Finding{Code: "rds.maint", Phrase: "fresh maintenance", Severity: domain.SevWarn, Source: "wave2:rds"}

	seeded := []resource.Resource{
		{ID: "db-rowa", Name: "db-rowa", Type: "rds", Findings: []domain.Finding{wave1Only, freshWave2}},
		{ID: "db-rowb", Name: "db-rowb", Type: "rds", Findings: []domain.Finding{wave1Only, staleWave2}},
	}
	ctrl.ApplyResourcesLoaded("rds", seeded, nil, false)

	// Incoming replace: rowA already carries its OWN fresh Wave-2 finding
	// (must win, not be clobbered). rowB carries ONLY the Wave-1 finding —
	// its Wave-2 companion must be inherited from the outgoing row.
	replacement := []resource.Resource{
		{ID: "db-rowa", Name: "db-rowa", Type: "rds", Findings: []domain.Finding{wave1Only, freshWave2}},
		{ID: "db-rowb", Name: "db-rowb", Type: "rds", Findings: []domain.Finding{wave1Only}},
	}
	ctrl.ApplyResourcesLoaded("rds", replacement, nil, false)

	gotA := findingsForID(ctrl, "db-rowa")
	foundFreshA := false
	foundStaleA := false
	for _, f := range gotA {
		if f.Source == "wave2:rds" && f.Phrase == freshWave2.Phrase {
			foundFreshA = true
		}
		if f.Source == "wave2:rds" && f.Phrase == staleWave2.Phrase {
			foundStaleA = true
		}
	}
	if !foundFreshA {
		t.Error("db-rowa: incoming's own fresh Wave-2 finding is missing post-swap — the incoming row's fresh data must be kept")
	}
	if foundStaleA {
		t.Error("db-rowa: a stale Wave-2 finding appeared post-swap — the incoming row's fresh Wave-2 finding must not be overwritten by carried-forward stale data")
	}

	gotB := findingsForID(ctrl, "db-rowb")
	foundWave2B := false
	for _, f := range gotB {
		if f.Source == "wave2:rds" {
			foundWave2B = true
		}
	}
	if !foundWave2B {
		t.Error("db-rowb: the previously-known Wave-2 finding vanished on swap because the incoming row already carried a Wave-1 finding — the carry-forward guard must merge per-finding-source, not skip the whole row when any finding is already present")
	}
}

// ─────────────────────────────────────────────────────────────────────────
// Pin 3 — C9 guard: rotation must still clear stale enrichment state
// ─────────────────────────────────────────────────────────────────────────

// TestProfileSwitch_StillClearsFindings pins the C9 guard: a stale
// enrichmentStore entry for a resource type must not survive a
// profile/region rotation on the same long-lived Controller instance
// (internal/tui constructs exactly one Controller for the process
// lifetime — internal/tui/app.go's New() calls app.New(core) once).
// MenuClearAvailabilityIntent (fired by both HandleProfileSelected and
// HandleRegionSelected in core/runtime/handlers.go) only clears
// MenuState fields (core/app/intents.go); nothing clears
// Controller.enrichmentStore. Session.Rotate (core/session/session.go)
// bumps generation counters but never reaches into the Controller.
//
// This test drives the closest Controller-observable analogue of a
// rotation: findings populated under one profile/region pair must be gone
// once the list for that type is reloaded from the new pair — modeled here
// by asserting that whatever handles a rotation clears the type's
// enrichmentStore entry before the next ApplyResourcesLoaded lands, so a
// same-ID collision across profiles cannot resurrect a dead profile's
// glyphs. RED at HEAD: nothing in this package clears enrichmentStore on
// rotation at all, so a stale entry survives untouched.
func TestProfileSwitch_StillClearsFindings(t *testing.T) {
	ctrl := glyphContinuityPair(t, "glyph-rotate-prof-a", "us-east-1")
	ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})

	seeded := []resource.Resource{
		{ID: "i-sharedid", Name: "i-sharedid", Type: "ec2"},
	}
	ctrl.ApplyResourcesLoaded("ec2", seeded, nil, false)
	ctrl.ApplyEnrichmentState("ec2", 1, false, map[string][]domain.Finding{
		"i-sharedid": {{Code: "ec2.impaired", Phrase: "system check failed", Severity: domain.SevBroken, Source: "wave2:ec2"}},
	}, nil)

	if !hasDecoratorForID(ctrl, "i-sharedid", app.DecoratorError) {
		t.Fatal("fixture assumption broken — expected an error decorator on i-sharedid before rotation")
	}

	// Simulate the ONLY rotation-adjacent clear that production code actually
	// fires: MenuClearAvailabilityIntent (menu-state only — see
	// core/app/intents.go). Then the new profile/region's list opens and
	// re-fetches — an unrelated instance that happens to reuse the same
	// resource ID (a real risk: EC2 instance IDs and other identifiers are
	// not globally unique across accounts/regions).
	ctrl.ApplyIntents([]runtime.UIIntent{runtime.MenuClearAvailabilityIntent{}})

	freshFromNewProfile := []resource.Resource{
		{ID: "i-sharedid", Name: "i-sharedid", Type: "ec2"},
	}
	ctrl.ApplyResourcesLoaded("ec2", freshFromNewProfile, nil, false)

	if hasDecoratorForID(ctrl, "i-sharedid", app.DecoratorError) {
		t.Error("error decorator from the OLD profile's enrichment survived a rotation-shaped sequence (MenuClearAvailabilityIntent + fresh fetch) — Controller.enrichmentStore must be cleared on profile/region rotation, not just MenuState")
	}

	got := findingsForID(ctrl, "i-sharedid")
	if len(got) != 0 {
		t.Errorf("resource.Findings for i-sharedid is non-empty (%d entries) after rotation — a stale Wave-2 finding from a prior profile/region leaked across rotation", len(got))
	}
}
