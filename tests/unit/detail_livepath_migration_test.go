// detail_livepath_migration_test.go — coverage-preservation migration ahead
// of deleting internal/tui/views/detail.go's dead DetailModel.Update key/
// message handling (the live path is app_stack.go's handleDetailKeyMsg +
// controller.Apply). Re-pins three behaviors that were previously exercised
// ONLY through that dead path, on the live app.Controller path instead, so
// the dead-path tests can be deleted later without losing coverage:
//
//  1. EC2-008 — the field-cursor skip loop in applyDetailActions
//     (core/app/detail_cursor.go ActionMoveDown) never lands FieldCursor
//     on a section-header or spacer row while advancing through interior
//     rows. Previously pinned only via DetailModel.Update
//     (detail_cursor_stable_test.go uses a header-free resource;
//     detail_controller_scroll_follow_test.go also uses a header-free
//     resource, so neither exercises the skip branch).
//
//     NOTE: a full sweep including ActionMoveUp back up to the very first
//     field surfaced a separate, previously-unknown boundary bug — when the
//     topmost field item (index 0) is itself an IsSection row, the skip
//     loop's `ds.FieldCursor > 0` guard never re-checks index 0 once the
//     cursor reaches it, so the cursor can end up parked on the header. That
//     is outside this migration's scope (only ActionMoveDown was asked for)
//     and is NOT covered by this file — flagged separately to the architect
//     rather than papered over here.
//
//  2. #280 — mergeDetailRelatedRow (core/app/handle.go), reached live via
//     Controller.ApplyDetailRelatedResultForResource, matches a
//     RelatedCheckResult to a related row by DefDisplayName so that
//     resource types with multiple related-defs sharing one TargetType
//     (ct-events has 4 self-pivot rows all targeting "ct-events") each
//     resolve independently instead of colliding. Previously pinned only via
//     views.DetailModel.Update in related_cache_replay_selfpivot_test.go — a
//     message path RelatedCheckResult never reaches in the live app (the TUI
//     adapter routes it to Controller, not DetailModel.Update; see
//     internal/tui/runtime_adapter_resources.go handleRelatedCheckResult).
//
//  3. ct-events pivot dispatch content — selecting an actionable
//     RelatedDeferred+FetchFilter pivot row (e.g. a ct-events self-pivot by
//     Username/AccessKeyId/EventName/EventId; Count is not authoritative for
//     RelatedDeferred, only State+FetchFilter drive it) must dispatch a
//     runtime.KindFetchFiltered task carrying the row's FetchFilter, while a
//     resolved-zero row must dispatch nothing. Previously pinned only via
//     DetailModel.Update + tea.Cmd execution in
//     ct_events_rightcol_dispatch_test.go's D3/D2 assertions. The live
//     equivalent is Controller.Apply(ActionRelatedSelect) ->
//     handleActionRelatedSelect -> dispatchRelatedNavigate
//     (core/app/actions_nav.go, core/app/navigate.go).
//
// Each test seeds controller state directly (newDetailController,
// Controller.ApplyDetailRelated) rather than replaying real AWS fixtures, so
// these tests are independent of the ct-events checkers/demo fixtures and of
// the dead-path test files they replace — they must keep passing after those
// files are deleted. The blank import of core/aws below is required for
// that independence: it registers ct-events' RelatedDefs and its
// FilteredPaginatedFetcher in the catalog, which resource.GetRelated and
// resource.GetFilteredPaginatedFetcher read from.
package unit_test

import (
	"testing"

	_ "github.com/k2m30/a9s/v3/core/aws"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
)

// ---------------------------------------------------------------------------
// 1. EC2-008 — field-cursor skip over section headers/spacers.
// ---------------------------------------------------------------------------

// TestDetailController_MoveDown_SkipsSectionHeadersAndSpacers pins that
// repeatedly applying ActionMoveDown on a resource whose field-item list has
// section headers and a trailing spacer between fields never leaves
// FieldCursor pointing at an IsSection or IsSpacer row.
//
// detailParityEC2Resource() alone projects through the generic (unregistered)
// flat-field fallback — no RawStruct, so the ec2-specific fieldpath projector
// yields no sections — so this seeds an Attention block via
// Controller.ApplyDetailFinding (mirroring TestDetailRenderParity_EC2Attention),
// which buildAttentionSectionDetail (core/app/detail_body.go) always
// prepends as exactly one IsSection header ("Attention (N)") followed by the
// finding rows and a trailing IsSpacer — the same field-item shape the skip
// loop in applyDetailActions must skip over regardless of whether the section
// came from the Attention block or a type-specific projector.
func TestDetailController_MoveDown_SkipsSectionHeadersAndSpacers(t *testing.T) {
	c := newDetailController(t, detailParityEC2Resource(), "ec2")
	c.ApplyDetailFinding(detailParityBrokenFinding(), detailParityAttentionDetail())

	body := c.Snapshot().Body.Detail
	if body == nil {
		t.Fatal("Body.Detail is nil after EnsureDetailState")
	}
	sawSection, sawSpacer := false, false
	for _, f := range body.Fields {
		if f.IsSection {
			sawSection = true
		}
		if f.IsSpacer {
			sawSpacer = true
		}
	}
	if !sawSection || !sawSpacer {
		t.Fatalf("test setup problem: expected both an IsSection and an IsSpacer field item after ApplyDetailFinding, got sawSection=%v sawSpacer=%v — the skip loop has nothing to skip",
			sawSection, sawSpacer)
	}

	for i := 0; i < len(body.Fields); i++ {
		c.Apply(app.Action{Kind: app.ActionMoveDown})

		vs := c.Snapshot()
		if vs.Body.Detail == nil {
			t.Fatalf("iteration %d: Body.Detail is nil after ActionMoveDown", i)
		}
		b := vs.Body.Detail
		if b.FieldCursor < 0 || b.FieldCursor >= len(b.Fields) {
			t.Fatalf("iteration %d: FieldCursor=%d out of range [0,%d)", i, b.FieldCursor, len(b.Fields))
		}
		row := b.Fields[b.FieldCursor]
		if row.IsSection || row.IsSpacer {
			t.Fatalf("iteration %d: FieldCursor=%d landed on IsSection=%v IsSpacer=%v (Key=%q) — ActionMoveDown skip loop failed",
				i, b.FieldCursor, row.IsSection, row.IsSpacer, row.Key)
		}
	}
}

// ---------------------------------------------------------------------------
// 2. #280 — DefDisplayName-keyed related-row merge.
// ---------------------------------------------------------------------------

// TestDetailController_ApplyDetailRelatedResultForResource_CtEventsSelfPivots_ResolveByDefDisplayName
// pins that when every ct-events self-pivot row (TargetType == "ct-events")
// receives a result keyed by its own distinct DefDisplayName via
// Controller.ApplyDetailRelatedResultForResource, each row resolves out of
// Loading independently and picks up its own Count — i.e. mergeDetailRelatedRow
// binds by DisplayName rather than colliding on the shared TargetType.
func TestDetailController_ApplyDetailRelatedResultForResource_CtEventsSelfPivots_ResolveByDefDisplayName(t *testing.T) {
	defs := resource.GetRelated("ct-events")
	var selfPivots []resource.RelatedDef
	for _, def := range defs {
		if def.TargetType == "ct-events" {
			selfPivots = append(selfPivots, def)
		}
	}
	if len(selfPivots) < 2 {
		t.Fatalf("ct-events registry must have >=2 self-pivot RelatedDefs to exercise DefDisplayName disambiguation, got %d", len(selfPivots))
	}

	res := resource.Resource{ID: "evt-livepath-merge-0001", Name: "evt-livepath-merge-0001"}
	c := newDetailController(t, res, "ct-events")

	rows := make([]app.DetailRelatedRow, len(selfPivots))
	for i, def := range selfPivots {
		rows[i] = app.DetailRelatedRow{
			TargetType:  "ct-events",
			DisplayName: def.DisplayName,
			State:       domain.RelatedLoading,
			Loading:     true,
		}
	}
	c.ApplyDetailRelated(rows)

	for i, def := range selfPivots {
		c.ApplyDetailRelatedResultForResource(
			"ct-events", res.ID, def.DisplayName, "ct-events",
			domain.RelatedResolved, i+1, false, "", false,
			[]string{"evt-related-a", "evt-related-b"}, nil,
		)
	}

	body := c.Snapshot().Body.Detail
	if body == nil {
		t.Fatal("Body.Detail is nil after seeding ct-events self-pivot rows")
	}
	byName := make(map[string]app.RelatedBlock, len(body.Related))
	for _, rb := range body.Related {
		byName[rb.Name] = rb
	}
	for i, def := range selfPivots {
		rb, ok := byName[def.DisplayName]
		if !ok {
			t.Errorf("self-pivot %q not present in Body.Detail.Related after merge", def.DisplayName)
			continue
		}
		if rb.Loading {
			t.Errorf("self-pivot %q is still Loading after ApplyDetailRelatedResultForResource — DefDisplayName merge failed to bind", def.DisplayName)
		}
		if rb.State != domain.RelatedResolved {
			t.Errorf("self-pivot %q State=%v, want RelatedResolved", def.DisplayName, rb.State)
		}
		if rb.Count != i+1 {
			t.Errorf("self-pivot %q Count=%d, want %d — merge bound to the wrong row (matched by ambiguous TargetType instead of DisplayName?)",
				def.DisplayName, rb.Count, i+1)
		}
	}
}

// ---------------------------------------------------------------------------
// 3. ct-events pivot dispatch content.
// ---------------------------------------------------------------------------

// TestDetailController_ActionRelatedSelect_DeferredPivotWithFetchFilter_DispatchesFetchFilteredTask
// pins that selecting an actionable RelatedDeferred pivot row with a
// non-empty FetchFilter (the ct-events pivot shape — e.g. "same Username")
// dispatches a runtime.KindFetchFiltered task scoped to the row's TargetType,
// carrying the row's FetchFilter verbatim via runtime.FetchFilteredPayload.
// This is the live-path replacement for ct_events_rightcol_dispatch_test.go's
// D3 assertion (the actionability guard bug where len(resourceIDs)>0 was
// checked instead of len(fetchFilter)>0).
//
// TargetType is "ct-events" because it is the only resource type with a
// registered FilteredPaginatedFetcher (resolve_related_navigate_test.go's
// TestResolveRelatedNavigate_FetchFilterHonoredForCtEvents), which
// ResolveRelatedNavigate requires to route FetchFilter into
// NavigationKindFilteredList. The source detail's own ResourceType is
// deliberately "ec2", not "ct-events": a same-type ("self") pivot with
// Count==0 is suppressed from the visible related list entirely by
// isSelfPivotZeroDetailRow (core/app/detail_cursor.go) regardless of
// State, and Count is not meaningful for RelatedDeferred rows (task58 forbids
// the old Count==-1 sentinel), so a cross-type pivot is the only way to keep
// this row visible and selectable at Arg "0" without reintroducing a banned
// sentinel literal.
func TestDetailController_ActionRelatedSelect_DeferredPivotWithFetchFilter_DispatchesFetchFilteredTask(t *testing.T) {
	res := resource.Resource{ID: "i-livepath-dispatch-0001", Name: "i-livepath-dispatch-0001"}
	c := newDetailController(t, res, "ec2")

	seededFilter := map[string]string{"Username": "alice"}
	c.ApplyDetailRelated([]app.DetailRelatedRow{
		{
			TargetType:  "ct-events",
			DisplayName: "Same Username",
			State:       domain.RelatedDeferred,
			FetchFilter: seededFilter,
		},
	})

	_, tasks := c.Apply(app.Action{Kind: app.ActionRelatedSelect, Arg: "0"})

	var fetchFilteredTask *runtime.TaskRequest
	for i := range tasks {
		if tasks[i].Key.Kind == runtime.KindFetchFiltered {
			fetchFilteredTask = &tasks[i]
			break
		}
	}
	if fetchFilteredTask == nil {
		t.Fatalf("ActionRelatedSelect on a RelatedDeferred+FetchFilter pivot dispatched no KindFetchFiltered task — tasks: %+v", tasks)
	}
	if fetchFilteredTask.Key.Scope != "ct-events" {
		t.Errorf("KindFetchFiltered task Scope=%q, want %q", fetchFilteredTask.Key.Scope, "ct-events")
	}
	payload, ok := fetchFilteredTask.Payload.(runtime.FetchFilteredPayload)
	if !ok {
		t.Fatalf("KindFetchFiltered task Payload=%T, want runtime.FetchFilteredPayload", fetchFilteredTask.Payload)
	}
	if len(payload.Filter) != len(seededFilter) {
		t.Errorf("FetchFilteredPayload.Filter=%v, want %v", payload.Filter, seededFilter)
	}
	for k, v := range seededFilter {
		if payload.Filter[k] != v {
			t.Errorf("FetchFilteredPayload.Filter[%q]=%q, want %q", k, payload.Filter[k], v)
		}
	}
}

// TestDetailController_ActionRelatedSelect_ResolvedZeroRow_DispatchesNoTask
// pairs with the pivot-dispatch pin above: a resolved row with Count==0 is
// never actionable (resource.IsRelatedActionable), so ActionRelatedSelect on
// it must dispatch nothing. Live-path replacement for
// ct_events_rightcol_dispatch_test.go's D2 assertion.
func TestDetailController_ActionRelatedSelect_ResolvedZeroRow_DispatchesNoTask(t *testing.T) {
	res := resource.Resource{ID: "evt-livepath-dispatch-0002", Name: "evt-livepath-dispatch-0002"}
	c := newDetailController(t, res, "ct-events")

	c.ApplyDetailRelated([]app.DetailRelatedRow{
		{
			TargetType:  "sg",
			DisplayName: "Security Groups",
			State:       domain.RelatedResolved,
			Count:       0,
		},
	})

	_, tasks := c.Apply(app.Action{Kind: app.ActionRelatedSelect, Arg: "0"})
	if len(tasks) != 0 {
		t.Errorf("ActionRelatedSelect on a resolved-zero row dispatched %d tasks, want 0 — a non-actionable row must not navigate. tasks: %+v",
			len(tasks), tasks)
	}
}
