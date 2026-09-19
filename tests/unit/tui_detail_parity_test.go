// The TUI and headless lanes agree on detail
// PatchDetail intents (Controller.applyIntents, core/app/intents.go) and on
// the related-cache replay on detail open, which short-circuits the
// KindRelatedCheck fan-out on a cache hit.
//
// Harness: a sized tui.Model driven via rootApplyMsg is the TUI lane; a
// directly-constructed *app.Controller + *runtime.Core pair is the headless
// lane. Both lanes are driven through the entry-point shape their production
// code uses (messages.Navigate for the TUI lane; app.Action{Kind:
// ActionSelect} for the headless lane).
package unit

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/app"
	_ "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
	"github.com/k2m30/a9s/v3/internal/tui"
)

// newDetailParityHeadlessController builds a Controller+Core pair configured
// exactly like newIntentParityHeadlessController (tui_intent_parity_test.go)
// and newTestController (app_controller_test.go): a fresh in-memory session,
// no disk-cache interaction, hermetic.
//
// Redirects A9S_CONFIG_FOLDER to a fresh t.TempDir() and registers
// t.Cleanup(c.Close) — in that order. This file's Handle(EnrichmentChecked)
// calls reach Controller.applyEnrichmentState, which calls
// persistMenuAvailabilityCache and so can queue an async availability-cache
// save; without this ordering the writer goroutine can still be running
// when t.TempDir()'s RemoveAll fires (see
// app_availsave_tempdir_cleanup_race_test.go for the traced race).
func newDetailParityHeadlessController(t *testing.T) (*app.Controller, *runtime.Core) {
	t.Helper()
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	s := session.New()
	s.Profile = "detail-parity-prof"
	s.Region = "us-east-1"
	core := runtime.New(s, nil)
	c := newBlessedController(t, core)
	t.Cleanup(c.Close)
	return c, core
}

// newDetailParityTUIModel builds a sized, demo-independent tui.Model exactly
// like newIntentParityTUIModel, wide enough (120 cols) that the related
// right column auto-shows on a pushed detail screen.
func newDetailParityTUIModel(t *testing.T) tui.Model {
	t.Helper()
	m := newBlessedModel(t, "detail-parity-prof", "us-east-1", tui.WithNoCache(true))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	return m
}

// detailParityEnrichmentCheckedEvent builds the real production message
// (messages.EnrichmentChecked) that HandleEnrichmentChecked
// (core/runtime/handlers_availability.go) folds into a runtime.PatchDetail
// intent (among others). Gen/TypeGen are left at zero deliberately —
// EnrichmentChecked.AcceptZeroGen() is true and handleEnrichmentChecked's
// per-type guard is `msg.TypeGen != 0 && msg.TypeGen != current`, so a zero
// TypeGen always bypasses the staleness check regardless of the session's
// EnrichmentTypeGen state (see docs/architecture.md "Test Architecture"
// gen-seed pitfall — this event uses the zero-bypass instead of matching a
// seeded generation).
//
// Driving the real message (rather than hand-building a runtime.PatchDetail
// intent) is required for the TUI-lane half of the parity assertion: a bare
// runtime.PatchDetail value is not itself a tea.Msg the TUI's Update()
// switch recognises, so sending it directly via rootApplyMsg would never
// reach applyIntents at all and vacuously pass. EnrichmentChecked is the one
// production seam (internal/tui/app.go's messages.EnrichmentChecked case)
// that both lanes actually receive.
func detailParityEnrichmentCheckedEvent() messages.EnrichmentChecked {
	return messages.EnrichmentChecked{
		ResourceType: "ec2",
		Findings: map[string][]domain.Finding{
			"i-detailparity01": {{
				Code:     "ec2.status-impaired",
				Phrase:   "instance status check failed",
				Severity: domain.SevBroken,
				Source:   "wave2:ec2-status",
			}},
		},
		AttentionDetails: map[string]map[domain.FindingCode]domain.AttentionDetail{
			"i-detailparity01": {
				"ec2.status-impaired": {
					Rows: []domain.DetailRow{
						{Label: "Check", Value: "SystemStatusCheck"},
					},
				},
			},
		},
	}
}

// detailParityClearEnrichmentCheckedEvent mirrors what HandleEnrichmentChecked
// receives when a subsequent enrichment pass finds no issues for the type at
// all (all resources recovered): nil Findings, which folds into a PatchDetail
// intent with a nil EnrichmentFindings map.
func detailParityClearEnrichmentCheckedEvent() messages.EnrichmentChecked {
	return messages.EnrichmentChecked{ResourceType: "ec2"}
}

// pushDetailResource is the shared resource fixture opened on both lanes.
func pushDetailResource() resource.Resource {
	return resource.Resource{
		ID:   "i-detailparity01",
		Name: "detail-parity-instance",
		Type: "ec2",
		Fields: map[string]string{
			"instance_id": "i-detailparity01",
			"state":       "running",
		},
	}
}

// attentionFieldRows returns the FieldRows from body.Fields with
// Path=="Attention" — mirrors core/app/controller_regression_test.go's
// helper of the same name (unexported there; a local copy is required since
// tests/unit is a different package).
func attentionFieldRows(body *app.DetailBody) []app.FieldRow {
	var out []app.FieldRow
	for _, f := range body.Fields {
		if f.Path == "Attention" {
			out = append(out, f)
		}
	}
	return out
}

// TestDetailParity_PatchDetail_TUIEqualsHeadless drives the SAME
// EnrichmentChecked event through the TUI lane (rootApplyMsg -> Model.Update
// -> m.coreUpdate -> m.applyIntents) and the headless lane (ctrl.Handle ->
// Controller.applyIntents), and asserts both converge on an identical
// DetailBody Attention section with the finding applied exactly once.
func TestDetailParity_PatchDetail_TUIEqualsHeadless(t *testing.T) {
	res := pushDetailResource()

	tm := newDetailParityTUIModel(t)
	tm, _ = rootApplyMsg(tm, messages.Navigate{
		Target:       messages.TargetDetail,
		ResourceType: "ec2",
		Resource:     &res,
	})
	tm, _ = rootApplyMsg(tm, detailParityEnrichmentCheckedEvent())
	tuiPlain := stripANSI(rootViewContent(tm))

	ctrl, _ := newDetailParityHeadlessController(t)
	ctrl.ApplyIntents([]runtime.UIIntent{
		runtime.PushScreen{
			ID:      runtime.ScreenDetail,
			Context: runtime.ScreenContext{ResourceType: "ec2", ResourceID: res.ID},
		},
	})
	ctrl.EnsureDetailState(res, "ec2")
	ctrl.Handle(detailParityEnrichmentCheckedEvent())

	snap := ctrl.Snapshot()
	if snap.Body.Detail == nil {
		t.Fatal("headless lane: Snapshot().Body.Detail is nil after EnrichmentChecked")
	}

	attn := attentionFieldRows(snap.Body.Detail)
	if len(attn) == 0 {
		t.Fatal("headless lane: Attention section absent from Fields after Handle(EnrichmentChecked) — PatchDetail is a no-op on the controller (intents.go default case)")
	}

	// buildAttentionSectionDetail emits, for a single issue-severity finding
	// with no Detail text and exactly 1 AttentionDetail row: 1 section-header
	// row ("Attention (N)"), 1 phrase row, 1 AttentionDetail row, and (unless
	// the entry is "bare") 1 trailing spacer — 4 Path=="Attention" rows for ONE
	// applied finding. The header's Key is "Attention (%d)" with
	// %d == len(entries), so it is asserted alongside the total row count.
	if len(attn) != 4 {
		t.Fatalf("headless lane: len(attentionFieldRows) = %d, want exactly 4 (header + phrase + AttentionDetail row + spacer) — a different count means the finding applied zero, partial, or duplicate times: %+v", len(attn), attn)
	}
	if attn[0].Key != "Attention (1)" {
		t.Errorf("headless lane: attention header Key = %q, want %q — a count other than 1 means the finding was applied more than once", attn[0].Key, "Attention (1)")
	}
	phraseFound, detailRowFound := false, false
	for _, row := range attn {
		if strings.Contains(row.Value, "status check failed") {
			phraseFound = true
		}
		if strings.Contains(row.Key, "SystemStatusCheck") || strings.Contains(row.Value, "SystemStatusCheck") {
			detailRowFound = true
		}
	}
	if !phraseFound {
		t.Errorf("headless lane: attention rows %+v do not contain the finding phrase %q", attn, "instance status check failed")
	}
	if !detailRowFound {
		t.Errorf("headless lane: attention rows %+v do not contain the AttentionDetail row %q", attn, "SystemStatusCheck")
	}

	if !strings.Contains(tuiPlain, "status check failed") {
		t.Errorf("TUI lane: rendered detail should contain the finding phrase %q after PatchDetail, got:\n%s", "status check failed", tuiPlain)
	}
	if !strings.Contains(tuiPlain, "SystemStatusCheck") {
		t.Errorf("TUI lane: rendered detail should contain the AttentionDetail row %q after PatchDetail, got:\n%s", "SystemStatusCheck", tuiPlain)
	}
}

// TestDetailParity_PatchDetail_ClearOnEmptyFindings_TUIEqualsHeadless: a
// SECOND PatchDetail with a nil/empty EnrichmentFindings map (all resources
// of the type recovered) strips the Attention section on BOTH lanes.
func TestDetailParity_PatchDetail_ClearOnEmptyFindings_TUIEqualsHeadless(t *testing.T) {
	res := pushDetailResource()

	tm := newDetailParityTUIModel(t)
	tm, _ = rootApplyMsg(tm, messages.Navigate{
		Target:       messages.TargetDetail,
		ResourceType: "ec2",
		Resource:     &res,
	})
	tm, _ = rootApplyMsg(tm, detailParityEnrichmentCheckedEvent())
	tm, _ = rootApplyMsg(tm, detailParityClearEnrichmentCheckedEvent())
	tuiPlain := stripANSI(rootViewContent(tm))

	ctrl, _ := newDetailParityHeadlessController(t)
	ctrl.ApplyIntents([]runtime.UIIntent{
		runtime.PushScreen{
			ID:      runtime.ScreenDetail,
			Context: runtime.ScreenContext{ResourceType: "ec2", ResourceID: res.ID},
		},
	})
	ctrl.EnsureDetailState(res, "ec2")
	ctrl.Handle(detailParityEnrichmentCheckedEvent())
	ctrl.Handle(detailParityClearEnrichmentCheckedEvent())

	snap := ctrl.Snapshot()
	if snap.Body.Detail == nil {
		t.Fatal("headless lane: Snapshot().Body.Detail is nil after clearing EnrichmentChecked")
	}
	if attn := attentionFieldRows(snap.Body.Detail); len(attn) != 0 {
		t.Errorf("headless lane: want 0 attention rows after a nil-EnrichmentFindings PatchDetail, got %d: %+v", len(attn), attn)
	}
	if strings.Contains(tuiPlain, "status check failed") {
		t.Errorf("TUI lane: rendered detail still contains the cleared finding phrase %q:\n%s", "status check failed", tuiPlain)
	}
}

// seedRelatedCache populates core's RelatedCache with 2 results for
// (resourceType, resourceID), mirroring what a completed RelatedCheckBatch
// would have written via PatchRelatedCache on a prior detail open.
func seedRelatedCache(core *runtime.Core, resourceType, resourceID string) {
	key := runtime.RelatedCacheKey(resourceType, resourceID)
	core.RelatedCacheSet(key, []runtime.RelatedCacheResult{
		{
			DefDisplayName: "Security Groups",
			Result:         resource.KnownRelated("sg", []string{"sg-replay1", "sg-replay2"}, false),
		},
		{
			DefDisplayName: "IAM Roles",
			Result:         resource.KnownRelated("role", []string{"role-replay1"}, false),
		},
	})
}

// relatedReplayDefs are the two related defs matching seedRelatedCache's two
// cached results — registered via replaceEC2Related so GetRelated("ec2")
// returns exactly these two, deterministically, regardless of production
// registration drift.
func relatedReplayDefs() []resource.RelatedDef {
	return []resource.RelatedDef{
		{TargetType: "sg", DisplayName: "Security Groups", Checker: noopChecker},
		{TargetType: "role", DisplayName: "IAM Roles", Checker: noopChecker},
	}
}

// findRelatedCheckResultCmd walks cmd (including nested tea.BatchMsg, one
// level of batch-of-batch as findNavigateMsg does) and reports whether a
// messages.RelatedCheckResult — the fan-out's per-def leaf message — is
// present anywhere in the produced commands. Only tea.BatchMsg is unwrapped
// (never e.g. tea.Tick), mirroring findNavigateMsg/extractMsg's walk depth in
// this package.
func findRelatedCheckResultCmd(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	msg := cmd()
	if _, ok := msg.(messages.RelatedCheckResult); ok {
		return true
	}
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		return false
	}
	for _, subCmd := range batch {
		if subCmd == nil {
			continue
		}
		subMsg := subCmd()
		if _, ok := subMsg.(messages.RelatedCheckResult); ok {
			return true
		}
		if subBatch, ok := subMsg.(tea.BatchMsg); ok {
			for _, innerCmd := range subBatch {
				if innerCmd == nil {
					continue
				}
				if _, ok := innerCmd().(messages.RelatedCheckResult); ok {
					return true
				}
			}
		}
	}
	return false
}

// TestDetailParity_RelatedReplay_NoRefanout_TUILane seeds the related cache
// for a resource (2 results), then opens its detail through the TUI
// navigation lane (messages.Navigate, TargetDetail — the real
// NavigateKindPushDetail path in runtime_adapter_navigate.go). The detail's
// related rows must render from cache (both DisplayNames + their counts
// visible, no "?" loading glyph), and the returned tea.Cmd batch must NOT
// contain a messages.RelatedCheckResult — the cache-hit replay must
// short-circuit the fan-out entirely rather than dispatching the checker
// AND replaying stale cache data.
func TestDetailParity_RelatedReplay_NoRefanout_TUILane(t *testing.T) {
	replaceEC2Related(t, relatedReplayDefs())

	res := pushDetailResource()
	tm := newDetailParityTUIModel(t)
	seedRelatedCache(tm.Core(), "ec2", res.ID)

	tm, navCmd := rootApplyMsg(tm, messages.Navigate{
		Target:       messages.TargetDetail,
		ResourceType: "ec2",
		Resource:     &res,
	})

	if findRelatedCheckResultCmd(navCmd) {
		t.Error("TUI lane: opening a detail with a related-cache hit dispatched a RelatedCheckResult fan-out — the cache-hit replay must short-circuit the fan-out, not run it alongside the replay")
	}

	content := stripANSI(rootViewContent(tm))
	if !strings.Contains(content, "Security Groups") || !strings.Contains(content, "(2)") {
		t.Errorf("TUI lane: rendered detail should show %q resolved to count 2 from the related cache, got:\n%s", "Security Groups", content)
	}
	if !strings.Contains(content, "IAM Roles") || !strings.Contains(content, "(1)") {
		t.Errorf("TUI lane: rendered detail should show %q resolved to count 1 from the related cache, got:\n%s", "IAM Roles", content)
	}
}

// TestDetailParity_RelatedReplay_NoRefanout_HeadlessLane mirrors the TUI-lane
// test through the headless controller's detail-open path (app.Action{Kind:
// ActionSelect} against a list-selected row — openSelectedListDetail).
// Seeding the related cache before the first open exercises the immediate
// cache-hit branch on the very first open.
func TestDetailParity_RelatedReplay_NoRefanout_HeadlessLane(t *testing.T) {
	replaceEC2Related(t, relatedReplayDefs())

	ctrl, core := newDetailParityHeadlessController(t)
	res := pushDetailResource()
	seedRelatedCache(core, "ec2", res.ID)

	ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	ctrl.ApplyResourcesLoaded("ec2", []resource.Resource{res}, nil, false)

	_, tasks := ctrl.Apply(app.Action{Kind: app.ActionSelect})

	for _, task := range tasks {
		if task.Key.Kind == runtime.KindRelatedCheck {
			t.Fatalf("headless lane: opening a detail with a related-cache hit returned a KindRelatedCheck task (%+v) — the cache-hit replay must short-circuit the fan-out", task)
		}
	}

	snap := ctrl.Snapshot()
	if snap.Body.Detail == nil {
		t.Fatal("headless lane: Snapshot().Body.Detail is nil after opening the detail")
	}
	related := snap.Body.Detail.Related
	if len(related) != 2 {
		t.Fatalf("headless lane: len(Snapshot().Body.Detail.Related) = %d, want 2 (replayed from cache)", len(related))
	}
	var sgRow, roleRow *app.RelatedBlock
	for i := range related {
		switch related[i].Name {
		case "Security Groups":
			sgRow = &related[i]
		case "IAM Roles":
			roleRow = &related[i]
		}
	}
	if sgRow == nil {
		t.Fatalf("headless lane: no %q row in Related %+v", "Security Groups", related)
	}
	if sgRow.Loading {
		t.Error("headless lane: Security Groups row is still Loading=true after a cache-hit replay")
	}
	if sgRow.Count != 2 {
		t.Errorf("headless lane: Security Groups row Count = %d, want 2", sgRow.Count)
	}
	if roleRow == nil {
		t.Fatalf("headless lane: no %q row in Related %+v", "IAM Roles", related)
	}
	if roleRow.Loading {
		t.Error("headless lane: IAM Roles row is still Loading=true after a cache-hit replay")
	}
	if roleRow.Count != 1 {
		t.Errorf("headless lane: IAM Roles row Count = %d, want 1", roleRow.Count)
	}
}

// TestDetailParity_RelatedReplay_CacheMiss_StillDispatchesFanout: when the
// related cache has no entry for the resource, both lanes dispatch the
// fan-out; the replay short-circuit applies only on an actual cache hit.
func TestDetailParity_RelatedReplay_CacheMiss_StillDispatchesFanout(t *testing.T) {
	replaceEC2Related(t, relatedReplayDefs())

	res := pushDetailResource()
	tm := newDetailParityTUIModel(t)
	_, navCmd := rootApplyMsg(tm, messages.Navigate{
		Target:       messages.TargetDetail,
		ResourceType: "ec2",
		Resource:     &res,
	})
	if !findRelatedCheckResultCmd(navCmd) {
		t.Error("TUI lane: opening a detail with NO related-cache entry did not dispatch a RelatedCheckResult fan-out — a cache miss must still fan out")
	}

	ctrl, _ := newDetailParityHeadlessController(t)
	ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	ctrl.ApplyResourcesLoaded("ec2", []resource.Resource{res}, nil, false)
	_, tasks := ctrl.Apply(app.Action{Kind: app.ActionSelect})
	found := false
	for _, task := range tasks {
		if task.Key.Kind == runtime.KindRelatedCheck {
			found = true
			break
		}
	}
	if !found {
		t.Error("headless lane: opening a detail with NO related-cache entry returned no KindRelatedCheck task — a cache miss must still fan out")
	}
}
