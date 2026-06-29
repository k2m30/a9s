// app_controller_pr_b_test.go — contract tests for internal/app.Controller (PR-B).
//
// Covers the two lanes added / wired in PR-B:
//
// RESULT LANE (Handle) — honest no-op contract for events not yet dispatched
// through HandleEvent (PR-C/PR-B0 must relocate TUI-shim pre-processing first):
//
//	messages.ResourcesLoaded    — Handle returns Snapshot() unchanged, nil tasks.
//	messages.RelatedCheckResult — Handle returns Snapshot() unchanged, nil tasks.
//	messages.EnrichDetailResult — Handle returns Snapshot() unchanged, nil tasks.
//	messages.ValueRevealed      — Handle returns Snapshot() unchanged, nil tasks.
//	messages.ClientsReady       — Handle returns Snapshot() unchanged, nil tasks.
//	messages.ClearFlash         — Handle returns Snapshot() unchanged, nil tasks.
//	messages.APIError           — Handle returns Snapshot() unchanged, nil tasks.
//
// COMMAND LANE (Apply) — 6 actions wired for real in PR-B:
//
//	ActionSelectProfile  — pops selector (PopSelectorIntent); returns TaskKindConnect.
//	ActionSelectRegion   — pops selector (PopSelectorIntent); returns TaskKindConnect.
//	ActionSelectTheme    — returns tasks containing TaskKindReadThemeFile.
//	ActionOpenHelp       — pushes help screen; Snapshot reflects BodyKindHelp.
//	ActionBack           — single pop (NOT PopAll); from 1-deep → empty/Unknown.
//	ActionOpenIdentity   — pushes identity screen AND returns TaskKindFetchIdentity.
//	ActionCommand        — dispatches "help","theme","region","root",<shortname>,unknown.
//
// PR-C-BLOCKED LANE — no-ops that must not panic and must return Snapshot:
//
//	ActionOpenDetail, ActionSelect, ActionOpenYAML, ActionOpenJSON,
//	ActionReveal, ActionChildView, ActionToggleRelated, ActionLoadMore.
//
// All fake data uses clearly synthetic values — no real AWS account IDs, ARNs,
// or profile names from real AWS accounts.
package unit_test

import (
	"errors"
	"testing"

	"github.com/k2m30/a9s/v3/internal/app"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/session"
)

// =============================================================================
// RESULT LANE — Handle (honest no-op contract)
//
// The events below are NOT dispatched by Core.HandleEvent in PR-B.
// They require TUI-shim pre-processing (adapter state + renderer types) that
// must be relocated to a shared layer before Core can handle them.
// See PR-B plan note: "Result-lane events deferred to post-PR-C."
//
// Contract: Handle(ev) returns a ViewState equal to Snapshot() and nil/empty
// tasks, without panicking.
// =============================================================================

// TestController_Handle_PRB_ResourcesLoaded_IsNoOpPassThrough verifies that
// Handle fed a messages.ResourcesLoaded returns Snapshot() unchanged with no
// tasks and does not panic.
//
// Deferred to post-PR-C: ResourcesLoaded dispatch is blocked on relocating
// TUI-shim pre-processing (see plan PR-B note).
func TestController_Handle_PRB_ResourcesLoaded_IsNoOpPassThrough(t *testing.T) {
	c := newTestController()

	ev := messages.ResourcesLoaded{
		ResourceType: "ec2",
		Resources: []resource.Resource{
			{ID: "i-0fakeec2111", Type: "ec2", Fields: map[string]string{"state": "running"}},
			{ID: "i-0fakeec2222", Type: "ec2", Fields: map[string]string{"state": "stopped"}},
		},
		Gen: 0, // AcceptZeroGen=true — always passes the staleness guard
	}

	snapBefore := c.Snapshot()

	var vs app.ViewState
	var tasks []runtime.TaskRequest
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Handle(ResourcesLoaded) panicked: %v", r)
			}
		}()
		vs, tasks = c.Handle(ev)
	}()

	snap := c.Snapshot()
	assertViewStateEqualsSnapshot(t, "Handle(ResourcesLoaded)", vs, snap)
	if vs.Body.Kind != snapBefore.Body.Kind {
		t.Errorf("Handle(ResourcesLoaded) changed Body.Kind: before=%q after=%q — expected no-op", snapBefore.Body.Kind, vs.Body.Kind)
	}
	if len(tasks) != 0 {
		t.Errorf("Handle(ResourcesLoaded) returned %d tasks, want 0 (no-op until PR-C)", len(tasks))
	}
}

// TestController_Handle_PRB_RelatedCheckResult_IsNoOpPassThrough verifies that
// Handle fed a messages.RelatedCheckResult returns Snapshot() unchanged with no
// tasks and does not panic.
//
// Deferred to post-PR-C: RelatedCheckResult dispatch is blocked on relocating
// TUI-shim pre-processing (see plan PR-B note).
func TestController_Handle_PRB_RelatedCheckResult_IsNoOpPassThrough(t *testing.T) {
	c := newTestController()

	ev := messages.RelatedCheckResult{
		ResourceType:     "ec2",
		SourceResourceID: "i-0fakeec2source",
		DefDisplayName:   "security-groups",
		Result:           resource.RelatedCheckResult{},
		Generation:       0, // AcceptZeroGen=true
	}

	snapBefore := c.Snapshot()

	var vs app.ViewState
	var tasks []runtime.TaskRequest
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Handle(RelatedCheckResult) panicked: %v", r)
			}
		}()
		vs, tasks = c.Handle(ev)
	}()

	snap := c.Snapshot()
	assertViewStateEqualsSnapshot(t, "Handle(RelatedCheckResult)", vs, snap)
	if vs.Body.Kind != snapBefore.Body.Kind {
		t.Errorf("Handle(RelatedCheckResult) changed Body.Kind: before=%q after=%q — expected no-op", snapBefore.Body.Kind, vs.Body.Kind)
	}
	if len(tasks) != 0 {
		t.Errorf("Handle(RelatedCheckResult) returned %d tasks, want 0 (no-op until PR-C)", len(tasks))
	}
}

// TestController_Handle_PRB_EnrichDetailResult_IsNoOpPassThrough verifies that
// Handle fed a messages.EnrichDetailResult returns Snapshot() unchanged with no
// tasks and does not panic.
//
// Deferred to post-PR-C: EnrichDetailResult dispatch is blocked on relocating
// TUI-shim pre-processing (see plan PR-B note).
func TestController_Handle_PRB_EnrichDetailResult_IsNoOpPassThrough(t *testing.T) {
	c := newTestController()

	ev := messages.EnrichDetailResult{
		ResourceType: "rds",
		ResourceID:   "db-fakeinstance-01",
		EnrichedRes:  resource.Resource{ID: "db-fakeinstance-01", Type: "rds"},
		Generation:   0, // AcceptZeroGen=true
	}

	snapBefore := c.Snapshot()

	var vs app.ViewState
	var tasks []runtime.TaskRequest
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Handle(EnrichDetailResult) panicked: %v", r)
			}
		}()
		vs, tasks = c.Handle(ev)
	}()

	snap := c.Snapshot()
	assertViewStateEqualsSnapshot(t, "Handle(EnrichDetailResult)", vs, snap)
	if vs.Body.Kind != snapBefore.Body.Kind {
		t.Errorf("Handle(EnrichDetailResult) changed Body.Kind: before=%q after=%q — expected no-op", snapBefore.Body.Kind, vs.Body.Kind)
	}
	if len(tasks) != 0 {
		t.Errorf("Handle(EnrichDetailResult) returned %d tasks, want 0 (no-op until PR-C)", len(tasks))
	}
}

// TestController_Handle_PRB_ValueRevealed_Success_IsNoOpPassThrough verifies
// that Handle fed a successful messages.ValueRevealed returns Snapshot() unchanged
// with no tasks and does not panic.
//
// Deferred to post-PR-C: ValueRevealed dispatch is blocked on relocating
// TUI-shim pre-processing (see plan PR-B note).
func TestController_Handle_PRB_ValueRevealed_Success_IsNoOpPassThrough(t *testing.T) {
	c := newTestController()

	ev := messages.ValueRevealed{
		ResourceType: "secrets",
		ResourceID:   "fake/secret/name",
		Value:        "s3cr3t-v4lu3-fake",
		Gen:          0, // AcceptZeroGen=true
	}

	snapBefore := c.Snapshot()

	var vs app.ViewState
	var tasks []runtime.TaskRequest
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Handle(ValueRevealed success) panicked: %v", r)
			}
		}()
		vs, tasks = c.Handle(ev)
	}()

	snap := c.Snapshot()
	assertViewStateEqualsSnapshot(t, "Handle(ValueRevealed success)", vs, snap)
	if vs.Body.Kind != snapBefore.Body.Kind {
		t.Errorf("Handle(ValueRevealed success) changed Body.Kind: before=%q after=%q — expected no-op", snapBefore.Body.Kind, vs.Body.Kind)
	}
	if len(tasks) != 0 {
		t.Errorf("Handle(ValueRevealed success) returned %d tasks, want 0 (no-op until PR-C)", len(tasks))
	}
}

// TestController_Handle_PRB_ValueRevealed_Error_IsNoOpPassThrough verifies that
// Handle fed a messages.ValueRevealed with Err set returns Snapshot() unchanged
// with no tasks and does not panic.
//
// Deferred to post-PR-C: ValueRevealed dispatch is blocked on relocating
// TUI-shim pre-processing (see plan PR-B note).
func TestController_Handle_PRB_ValueRevealed_Error_IsNoOpPassThrough(t *testing.T) {
	c := newTestController()

	ev := messages.ValueRevealed{
		ResourceType: "ssm",
		ResourceID:   "/fake/param/path",
		Value:        "",
		Err:          errors.New("reveal fetch failed: access denied"),
		Gen:          0, // AcceptZeroGen=true
	}

	snapBefore := c.Snapshot()

	var vs app.ViewState
	var tasks []runtime.TaskRequest
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Handle(ValueRevealed error) panicked: %v", r)
			}
		}()
		vs, tasks = c.Handle(ev)
	}()

	snap := c.Snapshot()
	assertViewStateEqualsSnapshot(t, "Handle(ValueRevealed error)", vs, snap)
	if vs.Body.Kind != snapBefore.Body.Kind {
		t.Errorf("Handle(ValueRevealed error) changed Body.Kind: before=%q after=%q — expected no-op", snapBefore.Body.Kind, vs.Body.Kind)
	}
	if len(tasks) != 0 {
		t.Errorf("Handle(ValueRevealed error) returned %d tasks, want 0 (no-op until PR-C)", len(tasks))
	}
}

// TestController_Handle_PRB_ClientsReady_Success_IsNoOpPassThrough verifies that
// Handle fed a messages.ClientsReady returns Snapshot() unchanged with no tasks
// and does not panic.
//
// Deferred to post-PR-C: ClientsReady dispatch is blocked on relocating
// TUI-shim pre-processing (see plan PR-B note).
func TestController_Handle_PRB_ClientsReady_Success_IsNoOpPassThrough(t *testing.T) {
	c := newTestController()

	ev := messages.ClientsReady{
		Clients: nil, // demo/no-AWS path — PreSuppliedClients is nil too; safe no-op
		Err:     nil,
		Region:  "us-east-1",
		Gen:     0, // AcceptZeroGen=true
	}

	snapBefore := c.Snapshot()

	var vs app.ViewState
	var tasks []runtime.TaskRequest
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Handle(ClientsReady success) panicked: %v", r)
			}
		}()
		vs, tasks = c.Handle(ev)
	}()

	snap := c.Snapshot()
	assertViewStateEqualsSnapshot(t, "Handle(ClientsReady success)", vs, snap)
	if vs.Body.Kind != snapBefore.Body.Kind {
		t.Errorf("Handle(ClientsReady success) changed Body.Kind: before=%q after=%q — expected no-op", snapBefore.Body.Kind, vs.Body.Kind)
	}
	if len(tasks) != 0 {
		t.Errorf("Handle(ClientsReady success) returned %d tasks, want 0 (no-op until PR-C)", len(tasks))
	}
}

// TestController_Handle_PRB_ClientsReady_Error_IsNoOpPassThrough verifies that
// Handle fed a messages.ClientsReady with Err set returns Snapshot() unchanged
// with no tasks and does not panic.
//
// Deferred to post-PR-C: ClientsReady dispatch is blocked on relocating
// TUI-shim pre-processing (see plan PR-B note).
func TestController_Handle_PRB_ClientsReady_Error_IsNoOpPassThrough(t *testing.T) {
	c := newTestController()

	ev := messages.ClientsReady{
		Clients: nil,
		Err:     errors.New("NoCredentialProviders: no valid providers in chain"),
		Region:  "",
		Gen:     0,
	}

	snapBefore := c.Snapshot()

	var vs app.ViewState
	var tasks []runtime.TaskRequest
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Handle(ClientsReady error) panicked: %v", r)
			}
		}()
		vs, tasks = c.Handle(ev)
	}()

	snap := c.Snapshot()
	assertViewStateEqualsSnapshot(t, "Handle(ClientsReady error)", vs, snap)
	if vs.Body.Kind != snapBefore.Body.Kind {
		t.Errorf("Handle(ClientsReady error) changed Body.Kind: before=%q after=%q — expected no-op", snapBefore.Body.Kind, vs.Body.Kind)
	}
	if len(tasks) != 0 {
		t.Errorf("Handle(ClientsReady error) returned %d tasks, want 0 (no-op until PR-C)", len(tasks))
	}
}

// TestController_Handle_PRB_ClearFlash_IsNoOpPassThrough verifies that
// Handle fed a messages.ClearFlash returns Snapshot() unchanged with no tasks
// and does not panic.
//
// Deferred to post-PR-C: ClearFlash dispatch is blocked on relocating
// TUI-shim pre-processing (see plan PR-B note).
func TestController_Handle_PRB_ClearFlash_IsNoOpPassThrough(t *testing.T) {
	c := newTestController()

	// ClearFlash is NOT a GenStamped event; the staleness guard skips it.
	ev := messages.ClearFlash{Gen: 0}

	snapBefore := c.Snapshot()

	var vs app.ViewState
	var tasks []runtime.TaskRequest
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Handle(ClearFlash) panicked: %v", r)
			}
		}()
		vs, tasks = c.Handle(ev)
	}()

	snap := c.Snapshot()
	assertViewStateEqualsSnapshot(t, "Handle(ClearFlash)", vs, snap)
	if vs.Body.Kind != snapBefore.Body.Kind {
		t.Errorf("Handle(ClearFlash) changed Body.Kind: before=%q after=%q — expected no-op", snapBefore.Body.Kind, vs.Body.Kind)
	}
	if len(tasks) != 0 {
		t.Errorf("Handle(ClearFlash) returned %d tasks, want 0 (no-op until PR-C)", len(tasks))
	}
}

// TestController_Handle_PRB_APIError_IsNoOpPassThrough verifies that
// Handle fed a messages.APIError returns Snapshot() unchanged with no tasks
// and does not panic.
//
// Deferred to post-PR-C: APIError dispatch is blocked on relocating
// TUI-shim pre-processing (see plan PR-B note).
func TestController_Handle_PRB_APIError_IsNoOpPassThrough(t *testing.T) {
	c := newTestController()

	ev := messages.APIError{
		ResourceType: "lambda",
		Err:          errors.New("AccessDeniedException: fake API error for test"),
		Gen:          0, // AcceptZeroGen=true
	}

	snapBefore := c.Snapshot()

	var vs app.ViewState
	var tasks []runtime.TaskRequest
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Handle(APIError) panicked: %v", r)
			}
		}()
		vs, tasks = c.Handle(ev)
	}()

	snap := c.Snapshot()
	assertViewStateEqualsSnapshot(t, "Handle(APIError)", vs, snap)
	if vs.Body.Kind != snapBefore.Body.Kind {
		t.Errorf("Handle(APIError) changed Body.Kind: before=%q after=%q — expected no-op", snapBefore.Body.Kind, vs.Body.Kind)
	}
	if len(tasks) != 0 {
		t.Errorf("Handle(APIError) returned %d tasks, want 0 (no-op until PR-C)", len(tasks))
	}
}

// TestController_Handle_PRB_StaleResourcesLoaded_DroppedNoPanic verifies that
// a ResourcesLoaded stamped with a non-zero Gen that does not match the session
// AvailabilityGen is silently discarded (staleness guard) without panicking.
// The returned ViewState must still equal Snapshot().
func TestController_Handle_PRB_StaleResourcesLoaded_DroppedNoPanic(t *testing.T) {
	c := newTestController()

	// Gen=999 will not match the session's AvailabilityGen (which starts at 1 per
	// session.New); AcceptZeroGen=true so zero passes, but 999 does not.
	ev := messages.ResourcesLoaded{
		ResourceType: "ec2",
		Resources:    []resource.Resource{{ID: "i-0stale", Type: "ec2"}},
		Gen:          999,
	}

	var vs app.ViewState
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Handle(stale ResourcesLoaded) panicked: %v", r)
			}
		}()
		vs, _ = c.Handle(ev)
	}()

	snap := c.Snapshot()
	assertViewStateEqualsSnapshot(t, "Handle(stale ResourcesLoaded)", vs, snap)
}

// =============================================================================
// COMMAND LANE — Apply (PR-B wired actions)
// =============================================================================

// TestController_Apply_PRB_SelectProfile_ReturnsConnectTask verifies that
// ActionSelectProfile returns a non-empty []TaskRequest containing a connect
// task. PR-B wires HandleProfileSelected which schedules TaskKindConnect.
func TestController_Apply_PRB_SelectProfile_ReturnsConnectTask(t *testing.T) {
	c := newTestController()

	vs, tasks := c.Apply(app.Action{Kind: app.ActionSelectProfile, Arg: "staging-fake"})

	snap := c.Snapshot()
	assertViewStateEqualsSnapshot(t, "Apply(SelectProfile)", vs, snap)

	if len(tasks) == 0 {
		t.Fatal("Apply(SelectProfile) returned no tasks — HandleProfileSelected must schedule a TaskKindConnect task")
	}
	hasConnect := false
	for _, task := range tasks {
		if task.Key.Kind == runtime.TaskKindConnect {
			hasConnect = true
			break
		}
	}
	if !hasConnect {
		t.Errorf("Apply(SelectProfile) tasks contain no TaskKindConnect; got kinds: %v", taskKindStrings(tasks))
	}
}

// TestController_Apply_PRB_SelectProfile_PopsSelectorIntent verifies that
// after ActionSelectProfile the profile-selector screen is gone from the top
// of the stack (PopSelectorIntent was applied by HandleProfileSelected).
// Precondition: push a ScreenProfileSelector so the pop has something to remove.
func TestController_Apply_PRB_SelectProfile_PopsSelectorIntent(t *testing.T) {
	c := newTestController()

	// Push a profile selector onto the stack so PopSelectorIntent can pop it.
	c.ApplyIntents([]runtime.UIIntent{
		runtime.PushScreen{
			ID:      runtime.ScreenProfileSelector,
			Context: runtime.ScreenContext{},
		},
	})
	if c.Snapshot().Body.Kind != app.BodyKindSelector {
		t.Fatalf("precondition: expected BodyKindSelector after push, got %q", c.Snapshot().Body.Kind)
	}

	vs, tasks := c.Apply(app.Action{Kind: app.ActionSelectProfile, Arg: "staging-fake"})

	snap := c.Snapshot()
	assertViewStateEqualsSnapshot(t, "Apply(SelectProfile) with selector on stack", vs, snap)

	// PopSelectorIntent must have dismissed the selector screen.
	if snap.Body.Kind == app.BodyKindSelector {
		t.Errorf("Apply(SelectProfile): selector screen still on top after select — PopSelectorIntent was not applied; Body.Kind=%q", snap.Body.Kind)
	}

	// The connect task must still be returned even when a selector was popped.
	if len(tasks) == 0 {
		t.Fatal("Apply(SelectProfile) with selector on stack returned no tasks — expected TaskKindConnect")
	}
	hasConnect := false
	for _, task := range tasks {
		if task.Key.Kind == runtime.TaskKindConnect {
			hasConnect = true
			break
		}
	}
	if !hasConnect {
		t.Errorf("Apply(SelectProfile) tasks contain no TaskKindConnect after selector pop; got kinds: %v", taskKindStrings(tasks))
	}
}

// TestController_Apply_PRB_SelectRegion_ReturnsConnectTask verifies that
// ActionSelectRegion returns a non-empty []TaskRequest containing a connect
// task. PR-B wires HandleRegionSelected which schedules TaskKindConnect.
func TestController_Apply_PRB_SelectRegion_ReturnsConnectTask(t *testing.T) {
	c := newTestController()

	vs, tasks := c.Apply(app.Action{Kind: app.ActionSelectRegion, Arg: "us-west-2"})

	snap := c.Snapshot()
	assertViewStateEqualsSnapshot(t, "Apply(SelectRegion)", vs, snap)

	if len(tasks) == 0 {
		t.Fatal("Apply(SelectRegion) returned no tasks — HandleRegionSelected must schedule a TaskKindConnect task")
	}
	hasConnect := false
	for _, task := range tasks {
		if task.Key.Kind == runtime.TaskKindConnect {
			hasConnect = true
			break
		}
	}
	if !hasConnect {
		t.Errorf("Apply(SelectRegion) tasks contain no TaskKindConnect; got kinds: %v", taskKindStrings(tasks))
	}
}

// TestController_Apply_PRB_SelectRegion_PopsSelectorIntent verifies that
// after ActionSelectRegion the region-selector screen is gone from the top of
// the stack (PopSelectorIntent applied by HandleRegionSelected).
func TestController_Apply_PRB_SelectRegion_PopsSelectorIntent(t *testing.T) {
	c := newTestController()

	// Push a region selector so PopSelectorIntent has something to remove.
	c.ApplyIntents([]runtime.UIIntent{
		runtime.PushScreen{
			ID:      runtime.ScreenRegion,
			Context: runtime.ScreenContext{},
		},
	})
	if c.Snapshot().Body.Kind != app.BodyKindSelector {
		t.Fatalf("precondition: expected BodyKindSelector after push, got %q", c.Snapshot().Body.Kind)
	}

	vs, tasks := c.Apply(app.Action{Kind: app.ActionSelectRegion, Arg: "us-west-2"})

	snap := c.Snapshot()
	assertViewStateEqualsSnapshot(t, "Apply(SelectRegion) with selector on stack", vs, snap)

	// PopSelectorIntent must have dismissed the selector screen.
	if snap.Body.Kind == app.BodyKindSelector {
		t.Errorf("Apply(SelectRegion): selector screen still on top after select — PopSelectorIntent was not applied; Body.Kind=%q", snap.Body.Kind)
	}

	// The connect task must still be returned even when a selector was popped.
	if len(tasks) == 0 {
		t.Fatal("Apply(SelectRegion) with selector on stack returned no tasks — expected TaskKindConnect")
	}
	hasConnect := false
	for _, task := range tasks {
		if task.Key.Kind == runtime.TaskKindConnect {
			hasConnect = true
			break
		}
	}
	if !hasConnect {
		t.Errorf("Apply(SelectRegion) tasks contain no TaskKindConnect after selector pop; got kinds: %v", taskKindStrings(tasks))
	}
}

// TestController_Apply_PRB_SelectTheme_ReturnsReadThemeTask verifies that
// ActionSelectTheme returns a non-empty []TaskRequest containing a
// TaskKindReadThemeFile task. Uses "default" as the theme name because it is
// always present in the embedded theme catalog regardless of disk state.
func TestController_Apply_PRB_SelectTheme_ReturnsReadThemeTask(t *testing.T) {
	c := newTestController()

	vs, tasks := c.Apply(app.Action{Kind: app.ActionSelectTheme, Arg: "default"})

	snap := c.Snapshot()
	assertViewStateEqualsSnapshot(t, "Apply(SelectTheme)", vs, snap)

	if len(tasks) == 0 {
		t.Fatal("Apply(SelectTheme) returned no tasks — HandleThemeSelected must schedule a TaskKindReadThemeFile task")
	}
	hasReadTheme := false
	for _, task := range tasks {
		if task.Key.Kind == runtime.TaskKindReadThemeFile {
			hasReadTheme = true
			break
		}
	}
	if !hasReadTheme {
		t.Errorf("Apply(SelectTheme) tasks contain no TaskKindReadThemeFile; got kinds: %v", taskKindStrings(tasks))
	}
}

// TestController_Apply_PRB_OpenHelp_PushesHelpScreen verifies that
// ActionOpenHelp pushes a help screen onto the stack, making the Snapshot
// reflect BodyKindHelp. PR-B wires ActionOpenHelp → HandleNavigate(TargetHelp)
// → NavigateKindPushHelp → PushScreen{ScreenHelp}.
func TestController_Apply_PRB_OpenHelp_PushesHelpScreen(t *testing.T) {
	c := newTestController()

	vs, tasks := c.Apply(app.Action{Kind: app.ActionOpenHelp})

	snap := c.Snapshot()
	assertViewStateEqualsSnapshot(t, "Apply(OpenHelp)", vs, snap)

	if snap.Body.Kind != app.BodyKindHelp {
		t.Errorf("Apply(OpenHelp) Body.Kind = %q, want %q — ActionOpenHelp must push ScreenHelp via NavigateKindPushHelp", snap.Body.Kind, app.BodyKindHelp)
	}
	_ = tasks // help screen push produces no background tasks
}

// TestController_Apply_PRB_Back_RootIsNoOp verifies that ActionBack when only
// the root menu screen remains is a no-op: no panic, stack stays at depth 1,
// and Snapshot reports BodyKindMenu (the root is never popped).
func TestController_Apply_PRB_Back_RootIsNoOp(t *testing.T) {
	c := newTestController()

	// Fresh controller starts at depth-1 (root menu). Back must not pop it.
	var vs app.ViewState
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Apply(Back) on root-only stack panicked: %v", r)
			}
		}()
		vs, _ = c.Apply(app.Action{Kind: app.ActionBack})
	}()

	snap := c.Snapshot()
	assertViewStateEqualsSnapshot(t, "Apply(Back) on root-only stack", vs, snap)
	if snap.Body.Kind != app.BodyKindMenu {
		t.Errorf("Apply(Back) on root-only stack: expected BodyKindMenu (root preserved), got %q", snap.Body.Kind)
	}
}

// TestController_Apply_PRB_Back_TwoDeepStack_SinglePop verifies that
// ActionBack on a 2-deep stack pops exactly ONE screen — the stack depth
// drops by one and Snapshot reflects the REMAINING lower screen, not Unknown.
// ActionBack is a single pop (PopScreen), not a full collapse (that is the
// "root" Command). This replaces the old PopAll assertion.
func TestController_Apply_PRB_Back_TwoDeepStack_SinglePop(t *testing.T) {
	c := newTestController()

	// Push two screens: lower=ScreenChildList, upper=ScreenProfileSelector.
	c.ApplyIntents([]runtime.UIIntent{
		runtime.PushScreen{ID: runtime.ScreenChildList, Context: runtime.ScreenContext{ResourceType: "ec2"}},
	})
	lowerKind := c.Snapshot().Body.Kind // BodyKindList — the screen we expect to remain
	c.ApplyIntents([]runtime.UIIntent{
		runtime.PushScreen{ID: runtime.ScreenProfileSelector, Context: runtime.ScreenContext{}},
	})
	if c.Snapshot().Body.Kind == app.BodyKindUnknown {
		t.Fatalf("precondition: expected non-empty 2-deep stack, got BodyKindUnknown")
	}
	upperKind := c.Snapshot().Body.Kind // BodyKindSelector — the screen that must be popped

	vs, tasks := c.Apply(app.Action{Kind: app.ActionBack})

	snap := c.Snapshot()
	assertViewStateEqualsSnapshot(t, "Apply(Back) 2-deep stack", vs, snap)

	// The upper screen (selector) must be gone.
	if snap.Body.Kind == upperKind {
		t.Errorf("Apply(Back) 2-deep: still showing upper screen Body.Kind=%q — Back must pop exactly one screen", snap.Body.Kind)
	}
	// The lower screen (child-list) must now be on top.
	if snap.Body.Kind != lowerKind {
		t.Errorf("Apply(Back) 2-deep: Body.Kind=%q, want %q (lower screen) — Back must not over-pop", snap.Body.Kind, lowerKind)
	}
	// Stack must not be empty — one screen remains.
	if snap.Body.Kind == app.BodyKindUnknown {
		t.Errorf("Apply(Back) 2-deep: stack empty after single Back — expected lower screen to remain")
	}
	// Back returns nil tasks.
	if len(tasks) != 0 {
		t.Errorf("Apply(Back) returned %d tasks, want 0", len(tasks))
	}
}

// TestController_Apply_PRB_Back_OneDeepStack_ReturnsToMenu verifies that
// ActionBack on a 2-deep stack (menu root + help overlay) returns to the
// menu root and Snapshot reports BodyKindMenu without panicking.
//
// PR-C: New(core) starts with ScreenMenu as root, so a fresh stack is
// [ScreenMenu]. Pushing ScreenHelp yields [ScreenMenu, ScreenHelp].
// ActionBack pops ScreenHelp, leaving [ScreenMenu] → BodyKindMenu.
func TestController_Apply_PRB_Back_OneDeepStack_ReturnsToMenu(t *testing.T) {
	c := newTestController()

	c.ApplyIntents([]runtime.UIIntent{
		runtime.PushScreen{ID: runtime.ScreenHelp, Context: runtime.ScreenContext{}},
	})
	if c.Snapshot().Body.Kind != app.BodyKindHelp {
		t.Fatalf("precondition: expected BodyKindHelp after push, got %q", c.Snapshot().Body.Kind)
	}

	var vs app.ViewState
	var tasks []runtime.TaskRequest
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Apply(Back) on 2-deep stack panicked: %v", r)
			}
		}()
		vs, tasks = c.Apply(app.Action{Kind: app.ActionBack})
	}()

	snap := c.Snapshot()
	assertViewStateEqualsSnapshot(t, "Apply(Back) 2-deep stack", vs, snap)

	if snap.Body.Kind != app.BodyKindMenu {
		t.Errorf("Apply(Back) 2-deep: Body.Kind=%q, want %q — back from help returns to menu root", snap.Body.Kind, app.BodyKindMenu)
	}
	if len(tasks) != 0 {
		t.Errorf("Apply(Back) returned %d tasks, want 0", len(tasks))
	}
}

// TestController_Apply_PRB_OpenIdentity_PushesIdentityScreenAndReturnsFetchTask
// verifies that ActionOpenIdentity pushes ScreenIdentity (Snapshot reflects
// BodyKindIdentity), returns a TaskRequest with Kind == TaskKindFetchIdentity,
// AND sets the IdentityFetching latch on Core (P2 review finding #1).
// PR-B pushes ScreenIdentity directly (no NavigateTargetIdentity in the runtime)
// and enqueues the fetch task so the adapter can call STS GetCallerIdentity.
func TestController_Apply_PRB_OpenIdentity_PushesIdentityScreenAndReturnsFetchTask(t *testing.T) {
	core, c := newTestControllerWithCore()

	vs, tasks := c.Apply(app.Action{Kind: app.ActionOpenIdentity})

	snap := c.Snapshot()
	assertViewStateEqualsSnapshot(t, "Apply(OpenIdentity)", vs, snap)

	if snap.Body.Kind != app.BodyKindIdentity {
		t.Errorf("Apply(OpenIdentity) Body.Kind = %q, want %q — ActionOpenIdentity must push ScreenIdentity", snap.Body.Kind, app.BodyKindIdentity)
	}

	// A FetchIdentity task must be returned so the adapter triggers STS.
	if len(tasks) == 0 {
		t.Fatal("Apply(OpenIdentity) returned no tasks — expected TaskKindFetchIdentity")
	}
	hasFetchIdentity := false
	for _, task := range tasks {
		if task.Key.Kind == runtime.TaskKindFetchIdentity {
			hasFetchIdentity = true
			break
		}
	}
	if !hasFetchIdentity {
		t.Errorf("Apply(OpenIdentity) tasks contain no TaskKindFetchIdentity; got kinds: %v", taskKindStrings(tasks))
	}

	// P2 review finding #1: Apply(OpenIdentity) must set the IdentityFetching
	// latch so the header can show a loading spinner before the fetch completes.
	if !core.IdentityFetching() {
		t.Errorf("Apply(OpenIdentity) did not set Core.IdentityFetching = true — latch must be set before returning the fetch task")
	}
}

// =============================================================================
// COMMAND LANE — ActionCommand token dispatch (PR-B)
// =============================================================================

// TestController_Apply_PRB_Command_Help_PushesHelpScreen verifies that
// ActionCommand{Arg:"help"} pushes the help screen, making Snapshot report
// BodyKindHelp. Mirrors the "help" colon-command token in the TUI.
func TestController_Apply_PRB_Command_Help_PushesHelpScreen(t *testing.T) {
	c := newTestController()

	vs, _ := c.Apply(app.Action{Kind: app.ActionCommand, Arg: "help"})

	snap := c.Snapshot()
	assertViewStateEqualsSnapshot(t, "Apply(Command:help)", vs, snap)

	if snap.Body.Kind != app.BodyKindHelp {
		t.Errorf("Apply(Command:help) Body.Kind = %q, want %q", snap.Body.Kind, app.BodyKindHelp)
	}
}

// TestController_Apply_PRB_Command_Theme_PushesThemeSelector verifies that
// ActionCommand{Arg:"theme"} pushes the theme selector, making Snapshot report
// BodyKindSelector. The theme selector uses ScreenTheme → bodyKindForScreen
// maps it to BodyKindSelector.
func TestController_Apply_PRB_Command_Theme_PushesThemeSelector(t *testing.T) {
	c := newTestController()

	vs, _ := c.Apply(app.Action{Kind: app.ActionCommand, Arg: "theme"})

	snap := c.Snapshot()
	assertViewStateEqualsSnapshot(t, "Apply(Command:theme)", vs, snap)

	if snap.Body.Kind != app.BodyKindSelector {
		t.Errorf("Apply(Command:theme) Body.Kind = %q, want %q (ScreenTheme → BodyKindSelector)", snap.Body.Kind, app.BodyKindSelector)
	}
}

// TestController_Apply_PRB_Command_Region_PushesRegionSelector verifies that
// ActionCommand{Arg:"region"} pushes the region selector, making Snapshot
// report BodyKindSelector. The region selector uses ScreenRegion → bodyKindForScreen
// maps it to BodyKindSelector.
func TestController_Apply_PRB_Command_Region_PushesRegionSelector(t *testing.T) {
	c := newTestController()

	vs, _ := c.Apply(app.Action{Kind: app.ActionCommand, Arg: "region"})

	snap := c.Snapshot()
	assertViewStateEqualsSnapshot(t, "Apply(Command:region)", vs, snap)

	if snap.Body.Kind != app.BodyKindSelector {
		t.Errorf("Apply(Command:region) Body.Kind = %q, want %q (ScreenRegion → BodyKindSelector)", snap.Body.Kind, app.BodyKindSelector)
	}
}

// TestController_Apply_PRB_Command_Root_CollapsesStack verifies that
// ActionCommand{Arg:"root"} collapses the stack via NavigateKindPopAll,
// leaving exactly the root menu screen. Snapshot reports BodyKindMenu at
// depth 1 — the root is never popped.
func TestController_Apply_PRB_Command_Root_CollapsesStack(t *testing.T) {
	c := newTestController()

	// Push two screens on top of the root menu → depth 3.
	c.ApplyIntents([]runtime.UIIntent{
		runtime.PushScreen{ID: runtime.ScreenChildList, Context: runtime.ScreenContext{ResourceType: "ec2"}},
		runtime.PushScreen{ID: runtime.ScreenHelp, Context: runtime.ScreenContext{}},
	})
	if c.Snapshot().Body.Kind == app.BodyKindMenu {
		t.Fatalf("precondition: expected non-menu top of stack before :root command, got BodyKindMenu")
	}

	vs, _ := c.Apply(app.Action{Kind: app.ActionCommand, Arg: "root"})

	snap := c.Snapshot()
	assertViewStateEqualsSnapshot(t, "Apply(Command:root)", vs, snap)

	// NavigateKindPopAll leaves exactly the root menu (depth 1), never empties the stack.
	if snap.Body.Kind != app.BodyKindMenu {
		t.Errorf("Apply(Command:root) Body.Kind = %q, want %q — :root must collapse to the root menu", snap.Body.Kind, app.BodyKindMenu)
	}
}

// TestController_Apply_PRB_Command_ResourceShortName_PushesListScreen verifies
// that ActionCommand{Arg:"ec2"} pushes a resource-list screen (BodyKindList).
// applyNavResult handles NavigateKindPushResourceList / Cached, so the stack
// reflects BodyKindList immediately after Apply returns.
//
// Row population requires DrainSync with demo clients; with nil clients the
// fetch task yields an APIError and rows stay empty — which is correct and
// separately asserted in headless integration tests that use demo clients.
func TestController_Apply_PRB_Command_ResourceShortName_PushesListScreen(t *testing.T) {
	c := newTestController()

	vs, tasks := c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})

	snap := c.Snapshot()
	assertViewStateEqualsSnapshot(t, "Apply(Command:ec2)", vs, snap)

	// applyNavResult must push ScreenResourceList for NavigateKindPushResourceList.
	if snap.Body.Kind != app.BodyKindList {
		t.Errorf("Apply(Command:ec2) Body.Kind = %q, want %q — applyNavResult must push ScreenResourceList for NavigateKindPushResourceList", snap.Body.Kind, app.BodyKindList)
	}

	// The fetch task must be returned so the adapter loads the list.
	if len(tasks) == 0 {
		t.Errorf("Apply(Command:ec2) returned no tasks — expected a resource-fetch task for cache-miss path")
	}
	// Body.List must be non-nil — the list state is initialized even before rows arrive.
	if snap.Body.List == nil {
		t.Error("Snapshot().Body.List is nil after pushing BodyKindList — list state must be initialized")
	}
}

// TestController_Apply_PRB_NavigateResourceList_PushesListScreen verifies that
// driving a resource-type navigate directly (same code path as Command:ec2 but
// via ActionCommand with a different resource short name) pushes ScreenResourceList
// (→ BodyKindList) onto the stack. Guards against applyNavResult regressions on
// any resource short name that resolves through HandleNavigate.
func TestController_Apply_PRB_NavigateResourceList_PushesListScreen(t *testing.T) {
	resourceShortNames := []string{"ec2", "rds", "lambda", "s3", "ecs"}

	for _, shortName := range resourceShortNames {
		shortName := shortName
		t.Run(shortName, func(t *testing.T) {
			c := newTestController()

			vs, _ := c.Apply(app.Action{Kind: app.ActionCommand, Arg: shortName})

			snap := c.Snapshot()
			assertViewStateEqualsSnapshot(t, "Apply(Command:"+shortName+")", vs, snap)

			if snap.Body.Kind != app.BodyKindList {
				t.Errorf("Apply(Command:%s) Body.Kind = %q, want %q — ScreenResourceList must be pushed for NavigateKindPushResourceList/Cached", shortName, snap.Body.Kind, app.BodyKindList)
			}
		})
	}
}

// TestController_Apply_PRB_Command_UnknownToken_IsNoPanic verifies that an
// unrecognised command token ("zzz-not-a-resource") is silently dropped:
// no panic, no stack change, Snapshot unchanged.
func TestController_Apply_PRB_Command_UnknownToken_IsNoPanic(t *testing.T) {
	c := newTestController()

	snapBefore := c.Snapshot()

	var vs app.ViewState
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Apply(Command:zzz-not-a-resource) panicked: %v", r)
			}
		}()
		vs, _ = c.Apply(app.Action{Kind: app.ActionCommand, Arg: "zzz-not-a-resource"})
	}()

	snap := c.Snapshot()
	assertViewStateEqualsSnapshot(t, "Apply(Command:zzz-not-a-resource)", vs, snap)

	if snap.Body.Kind != snapBefore.Body.Kind {
		t.Errorf("Apply(Command:zzz-not-a-resource) changed Body.Kind from %q to %q — unknown token must be a no-op", snapBefore.Body.Kind, snap.Body.Kind)
	}
}

// =============================================================================
// PR-C-BLOCKED LANE — no-ops that must not panic and must return Snapshot
// =============================================================================

// TestController_Apply_PRB_PRCBlockedActions_NoPanicReturnSnapshot verifies
// that every action intentionally left as a no-op in PR-B does not panic and
// returns a ViewState equal to Snapshot(). Real behavior for these verbs lands
// in PR-C.
func TestController_Apply_PRB_PRCBlockedActions_NoPanicReturnSnapshot(t *testing.T) {
	// These actions are shape-only guards — no-panic + Snapshot equality.
	// ActionSelect is wired for ScreenMenu (removed from this list).
	// Behavioral assertions for these verbs live in app_controller_regression_test.go.
	prcBlockedActions := []app.Action{
		{Kind: app.ActionOpenDetail},
		{Kind: app.ActionOpenYAML},
		{Kind: app.ActionOpenJSON},
		{Kind: app.ActionReveal, Arg: "i-0fakereveal"},
		{Kind: app.ActionChildView, Arg: "e"},
		{Kind: app.ActionToggleRelated},
		{Kind: app.ActionLoadMore},
	}

	for i := range prcBlockedActions {
		a := prcBlockedActions[i]
		t.Run(string(a.Kind), func(t *testing.T) {
			c := newTestController()
			var vs app.ViewState
			var tasks []runtime.TaskRequest
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("Apply(%q) panicked: %v", a.Kind, r)
					}
				}()
				vs, tasks = c.Apply(a)
			}()

			snap := c.Snapshot()
			assertViewStateEqualsSnapshot(t, "Apply("+string(a.Kind)+")", vs, snap)

			// PR-C-blocked: tasks must be nil in PR-B (no real dispatch yet).
			if len(tasks) != 0 {
				t.Errorf("Apply(%q) returned %d tasks — unexpected for PR-B no-op; if coder wired this in PR-B, update this test", a.Kind, len(tasks))
			}
		})
	}
}

// TestController_Handle_PRB_ResultLane_ConsistencyAfterMultipleEvents verifies
// that the controller remains consistent (Snapshot equals returned ViewState)
// after a sequence of Handle calls with different event types. Guards against
// state corruption between events.
func TestController_Handle_PRB_ResultLane_ConsistencyAfterMultipleEvents(t *testing.T) {
	c := newTestController()

	events := []runtime.Event{
		messages.ResourcesLoaded{ResourceType: "lambda", Gen: 0},
		messages.APIError{ResourceType: "ecs", Err: errors.New("fake error"), Gen: 0},
		messages.ClearFlash{Gen: 0},
		messages.EnrichDetailResult{ResourceType: "rds", ResourceID: "db-fake-01", Generation: 0},
		messages.ValueRevealed{ResourceType: "secrets", ResourceID: "fake/secret", Value: "fake-val", Gen: 0},
		messages.RelatedCheckResult{ResourceType: "ec2", SourceResourceID: "i-0fake", Generation: 0},
		messages.ClientsReady{Region: "us-east-1", Gen: 0},
	}

	for i, ev := range events {
		vs, _ := c.Handle(ev)
		snap := c.Snapshot()
		if vs.Body.Kind != snap.Body.Kind {
			t.Errorf("event[%d] %T: returned ViewState.Body.Kind=%q != Snapshot Body.Kind=%q",
				i, ev, vs.Body.Kind, snap.Body.Kind)
		}
		if vs.Header.Profile != snap.Header.Profile {
			t.Errorf("event[%d] %T: returned ViewState.Header.Profile=%q != Snapshot Profile=%q",
				i, ev, vs.Header.Profile, snap.Header.Profile)
		}
	}
}

// newTestControllerWithCore builds a Controller backed by a fresh runtime.Core
// and returns both so tests can assert on Core state (e.g. IdentityFetching).
// Mirrors newTestController but exposes the Core for latch/getter assertions.
func newTestControllerWithCore() (*runtime.Core, *app.Controller) {
	s := session.New()
	s.Profile = "demo"
	s.Region = "us-east-1"
	core := runtime.New(s, nil)
	return core, app.New(core)
}

// taskKindStrings returns the TaskKind strings from a slice of TaskRequests,
// for use in failure messages.
func taskKindStrings(tasks []runtime.TaskRequest) []string {
	kinds := make([]string, len(tasks))
	for i, task := range tasks {
		kinds[i] = string(task.Key.Kind)
	}
	return kinds
}
