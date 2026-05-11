// handlers_test.go — unit tests for the 6 handlers ported from
// internal/tui in Phase-05 PR-05a-h3 (AS-324).
//
// Package runtime (not runtime_test) so we can access unexported fields
// such as c.session directly, and read session-owned fields like
// ConnectGen, HasPrevState, PrevProfile, PrevRegion, PendingRefresh,
// etc., to assert state mutations made by the handlers.
package runtime

import (
	"errors"
	"testing"
	"time"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/session"
)

// ---- helpers ---------------------------------------------------------------

// newCore returns a Core with a fresh session and nil catalog (the 6 ported
// handlers do not consult the catalog).
func newCore() *Core {
	return New(session.New(), nil)
}

// findFlashIntent returns the first FlashIntent in xs, or (FlashIntent{}, false).
func findFlashIntent(xs []UIIntent) (FlashIntent, bool) {
	for _, x := range xs {
		if fi, ok := x.(FlashIntent); ok {
			return fi, true
		}
	}
	return FlashIntent{}, false
}

// findClearFlash returns true when xs contains a ClearFlash intent.
func findClearFlash(xs []UIIntent) bool {
	for _, x := range xs {
		if _, ok := x.(ClearFlash); ok {
			return true
		}
	}
	return false
}

// findSetErrorHint returns (intent, found).
func findSetErrorHint(xs []UIIntent) (SetErrorHintIntent, bool) {
	for _, x := range xs {
		if h, ok := x.(SetErrorHintIntent); ok {
			return h, true
		}
	}
	return SetErrorHintIntent{}, false
}

// findAppendErrorHistory returns true when xs contains AppendErrorHistoryIntent.
func findAppendErrorHistory(xs []UIIntent) bool {
	for _, x := range xs {
		if _, ok := x.(AppendErrorHistoryIntent); ok {
			return true
		}
	}
	return false
}

// findClearActiveListLoading returns true when xs contains ClearActiveListLoadingIntent.
func findClearActiveListLoading(xs []UIIntent) bool {
	for _, x := range xs {
		if _, ok := x.(ClearActiveListLoadingIntent); ok {
			return true
		}
	}
	return false
}

// findFlashTick returns the first FlashTickPayload in tasks, or (FlashTickPayload{}, false).
func findFlashTick(tasks []TaskRequest) (FlashTickPayload, bool) {
	for _, t := range tasks {
		if p, ok := t.Payload.(FlashTickPayload); ok {
			return p, true
		}
	}
	return FlashTickPayload{}, false
}

// hasTaskKind returns true when tasks contains at least one request with Kind k.
func hasTaskKind(tasks []TaskRequest, k TaskKind) bool {
	for _, t := range tasks {
		if t.Key.Kind == k {
			return true
		}
	}
	return false
}

// findMenuClearAvailability returns true when xs contains MenuClearAvailabilityIntent.
func findMenuClearAvailability(xs []UIIntent) bool {
	for _, x := range xs {
		if _, ok := x.(MenuClearAvailabilityIntent); ok {
			return true
		}
	}
	return false
}

// findPopSelector returns true when xs contains PopSelectorIntent.
func findPopSelector(xs []UIIntent) bool {
	for _, x := range xs {
		if _, ok := x.(PopSelectorIntent); ok {
			return true
		}
	}
	return false
}

// findRefreshActiveList returns true when xs contains RefreshActiveListIntent.
func findRefreshActiveList(xs []UIIntent) bool {
	for _, x := range xs {
		if _, ok := x.(RefreshActiveListIntent); ok {
			return true
		}
	}
	return false
}

// findConnectPayload returns the ConnectPayload from the first TaskKindConnect task.
func findConnectPayload(tasks []TaskRequest) (ConnectPayload, bool) {
	for _, t := range tasks {
		if t.Key.Kind == TaskKindConnect {
			if p, ok := t.Payload.(ConnectPayload); ok {
				return p, true
			}
		}
	}
	return ConnectPayload{}, false
}

// findEmitNavigatePayload returns the EmitNavigatePayload from the first
// TaskKindEmitNavigate task.
func findEmitNavigatePayload(tasks []TaskRequest) (EmitNavigatePayload, bool) {
	for _, t := range tasks {
		if t.Key.Kind == TaskKindEmitNavigate {
			if p, ok := t.Payload.(EmitNavigatePayload); ok {
				return p, true
			}
		}
	}
	return EmitNavigatePayload{}, false
}

// ---- HandleFlash tests -----------------------------------------------------

// TestHandleFlash_NotError: IsError=false → single FlashIntent, no
// AppendErrorHistoryIntent, one FlashTick with the right gen and 2 s duration.
func TestHandleFlash_NotError(t *testing.T) {
	c := newCore()
	intents, tasks := c.HandleFlash(FlashEvent{Text: "hello", IsError: false, NewGen: 3})

	// exactly one FlashIntent
	fi, ok := findFlashIntent(intents)
	if !ok {
		t.Fatal("expected FlashIntent, got none")
	}
	if fi.Text != "hello" {
		t.Errorf("FlashIntent.Text = %q, want %q", fi.Text, "hello")
	}
	if fi.IsError {
		t.Error("FlashIntent.IsError = true, want false")
	}

	// no AppendErrorHistoryIntent
	if findAppendErrorHistory(intents) {
		t.Error("unexpected AppendErrorHistoryIntent for non-error flash")
	}

	// FlashTick with correct gen and 2 s
	tick, ok := findFlashTick(tasks)
	if !ok {
		t.Fatal("expected FlashTickPayload task, got none")
	}
	if tick.Gen != 3 {
		t.Errorf("FlashTickPayload.Gen = %d, want 3", tick.Gen)
	}
	if tick.Duration != 2*time.Second {
		t.Errorf("FlashTickPayload.Duration = %v, want 2s", tick.Duration)
	}
}

// TestHandleFlash_IsError: IsError=true → FlashIntent + AppendErrorHistoryIntent
// + FlashTick with 2 s.
func TestHandleFlash_IsError(t *testing.T) {
	c := newCore()
	intents, tasks := c.HandleFlash(FlashEvent{Text: "bad thing", IsError: true, NewGen: 7})

	fi, ok := findFlashIntent(intents)
	if !ok {
		t.Fatal("expected FlashIntent")
	}
	if fi.Text != "bad thing" {
		t.Errorf("FlashIntent.Text = %q, want %q", fi.Text, "bad thing")
	}
	if !fi.IsError {
		t.Error("FlashIntent.IsError = false, want true")
	}

	if !findAppendErrorHistory(intents) {
		t.Error("expected AppendErrorHistoryIntent for error flash, got none")
	}

	tick, ok := findFlashTick(tasks)
	if !ok {
		t.Fatal("expected FlashTickPayload task")
	}
	if tick.Gen != 7 {
		t.Errorf("FlashTickPayload.Gen = %d, want 7", tick.Gen)
	}
	if tick.Duration != 2*time.Second {
		t.Errorf("FlashTickPayload.Duration = %v, want 2s", tick.Duration)
	}
}

// ---- HandleClearFlash tests ------------------------------------------------

// TestHandleClearFlash_StaleGen: Gen != CurrentGen → nil, nil.
func TestHandleClearFlash_StaleGen(t *testing.T) {
	c := newCore()
	intents, tasks := c.HandleClearFlash(ClearFlashEvent{Gen: 1, CurrentGen: 2, IsError: false})
	if intents != nil {
		t.Errorf("expected nil intents for stale gen, got %v", intents)
	}
	if tasks != nil {
		t.Errorf("expected nil tasks for stale gen, got %v", tasks)
	}
}

// TestHandleClearFlash_CurrentGen_NotError: matching gen, non-error flash →
// ClearFlash intent only, no SetErrorHintIntent.
func TestHandleClearFlash_CurrentGen_NotError(t *testing.T) {
	c := newCore()
	intents, tasks := c.HandleClearFlash(ClearFlashEvent{Gen: 5, CurrentGen: 5, IsError: false})

	if !findClearFlash(intents) {
		t.Error("expected ClearFlash intent")
	}
	if _, ok := findSetErrorHint(intents); ok {
		t.Error("unexpected SetErrorHintIntent for non-error flash clear")
	}
	if tasks != nil {
		t.Errorf("expected nil tasks, got %v", tasks)
	}
}

// TestHandleClearFlash_CurrentGen_IsError: matching gen, error flash →
// ClearFlash + SetErrorHintIntent{Show:true}.
func TestHandleClearFlash_CurrentGen_IsError(t *testing.T) {
	c := newCore()
	intents, tasks := c.HandleClearFlash(ClearFlashEvent{Gen: 5, CurrentGen: 5, IsError: true})

	if !findClearFlash(intents) {
		t.Error("expected ClearFlash intent")
	}
	hint, ok := findSetErrorHint(intents)
	if !ok {
		t.Error("expected SetErrorHintIntent, got none")
	}
	if !hint.Show {
		t.Error("SetErrorHintIntent.Show = false, want true")
	}
	if tasks != nil {
		t.Errorf("expected nil tasks, got %v", tasks)
	}
}

// ---- HandleAPIError tests --------------------------------------------------

// TestHandleAPIError_UnknownError: plain errors.New error → classifier returns
// "Unknown", handler uses err.Error() as flash text.
func TestHandleAPIError_UnknownError(t *testing.T) {
	c := newCore()
	err := errors.New("boom")
	intents, tasks := c.HandleAPIError(APIErrorEvent{Err: err, NewGen: 4})

	fi, ok := findFlashIntent(intents)
	if !ok {
		t.Fatal("expected FlashIntent")
	}
	if fi.Text != "boom" {
		t.Errorf("FlashIntent.Text = %q, want %q", fi.Text, "boom")
	}
	if !fi.IsError {
		t.Error("FlashIntent.IsError = false, want true")
	}

	if !findAppendErrorHistory(intents) {
		t.Error("expected AppendErrorHistoryIntent")
	}
	if !findClearActiveListLoading(intents) {
		t.Error("expected ClearActiveListLoadingIntent")
	}

	tick, ok := findFlashTick(tasks)
	if !ok {
		t.Fatal("expected FlashTickPayload task")
	}
	if tick.Duration != 5*time.Second {
		t.Errorf("FlashTickPayload.Duration = %v, want 5s", tick.Duration)
	}
	if tick.Gen != 4 {
		t.Errorf("FlashTickPayload.Gen = %d, want 4", tick.Gen)
	}
}

// TestHandleAPIError_AlwaysEmitsThreeIntents: regardless of error type, the
// three mandatory intents must always be present.
func TestHandleAPIError_AlwaysEmitsThreeIntents(t *testing.T) {
	c := newCore()
	intents, _ := c.HandleAPIError(APIErrorEvent{Err: errors.New("any"), NewGen: 1})

	if _, ok := findFlashIntent(intents); !ok {
		t.Error("missing FlashIntent")
	}
	if !findAppendErrorHistory(intents) {
		t.Error("missing AppendErrorHistoryIntent")
	}
	if !findClearActiveListLoading(intents) {
		t.Error("missing ClearActiveListLoadingIntent")
	}
}

// TestHandleAPIError_AlwaysEmitsFlashTick: the 5 s tick is always scheduled.
func TestHandleAPIError_AlwaysEmitsFlashTick(t *testing.T) {
	c := newCore()
	_, tasks := c.HandleAPIError(APIErrorEvent{Err: errors.New("any"), NewGen: 9})

	tick, ok := findFlashTick(tasks)
	if !ok {
		t.Fatal("expected FlashTickPayload task")
	}
	if tick.Duration != 5*time.Second {
		t.Errorf("FlashTickPayload.Duration = %v, want 5s", tick.Duration)
	}
}

// ---- HandleClientsReady tests ----------------------------------------------

// TestHandleClientsReady_StaleGen: ev.Gen != session.ConnectGen → nil, nil.
func TestHandleClientsReady_StaleGen(t *testing.T) {
	c := newCore()
	// session.New() seeds ConnectGen=0; send Gen=99 which is != 0
	intents, tasks := c.HandleClientsReady(ClientsReadyEvent{
		Gen: 99, NewGen: 1,
	})
	if intents != nil {
		t.Errorf("expected nil intents for stale gen, got %v", intents)
	}
	if tasks != nil {
		t.Errorf("expected nil tasks for stale gen, got %v", tasks)
	}
}

// TestHandleClientsReady_Failure_RollsBackPrevState: failure with HasPrevState=true
// restores PrevProfile/PrevRegion and clears the latch.
func TestHandleClientsReady_Failure_RollsBackPrevState(t *testing.T) {
	c := newCore()
	s := c.session
	s.ConnectGen = 5
	s.HasPrevState = true
	s.PrevProfile = "old-profile"
	s.PrevRegion = "old-region"
	s.Profile = "new-profile"
	s.Region = "new-region"

	c.HandleClientsReady(ClientsReadyEvent{ //nolint:ineffassign,staticcheck // crash-verification; return values intentionally ignored
		Gen: 5, NewGen: 6,
		Err: errors.New("connect failed"),
	})
	//nolint:ineffassign,staticcheck // return values used above; re-reading session state here
	_ = s // used to read session fields below

	if s.Profile != "old-profile" {
		t.Errorf("Profile after rollback = %q, want %q", s.Profile, "old-profile")
	}
	if s.Region != "old-region" {
		t.Errorf("Region after rollback = %q, want %q", s.Region, "old-region")
	}
	if s.HasPrevState {
		t.Error("HasPrevState should be cleared after rollback")
	}
	if s.PrevProfile != "" {
		t.Errorf("PrevProfile should be cleared, got %q", s.PrevProfile)
	}
	if s.PrevRegion != "" {
		t.Errorf("PrevRegion should be cleared, got %q", s.PrevRegion)
	}
	if s.PendingRefresh {
		t.Error("PendingRefresh should be cleared after failure")
	}
}

// TestHandleClientsReady_Failure_EmitsErrorIntents: failure path emits
// FlashIntent(IsError=true) + AppendErrorHistoryIntent + FlashTick(5s).
func TestHandleClientsReady_Failure_EmitsErrorIntents(t *testing.T) {
	c := newCore()
	s := c.session
	s.ConnectGen = 2

	intents, tasks := c.HandleClientsReady(ClientsReadyEvent{
		Gen: 2, NewGen: 3,
		Err: errors.New("no route to host"),
	})

	fi, ok := findFlashIntent(intents)
	if !ok {
		t.Fatal("expected FlashIntent")
	}
	if !fi.IsError {
		t.Error("FlashIntent.IsError = false, want true")
	}
	if fi.Text != "no route to host" {
		t.Errorf("FlashIntent.Text = %q, want %q", fi.Text, "no route to host")
	}
	if !findAppendErrorHistory(intents) {
		t.Error("expected AppendErrorHistoryIntent")
	}

	tick, ok := findFlashTick(tasks)
	if !ok {
		t.Fatal("expected FlashTickPayload task")
	}
	if tick.Duration != 5*time.Second {
		t.Errorf("FlashTickPayload.Duration = %v, want 5s", tick.Duration)
	}
	if tick.Gen != 3 {
		t.Errorf("FlashTickPayload.Gen = %d, want 3", tick.Gen)
	}
}

// TestHandleClientsReady_Failure_WithExistingClients_FiresBootstrapTasks:
// when session.Clients != nil, failure path also fires FetchIdentity +
// LoadAvailCache.
func TestHandleClientsReady_Failure_WithExistingClients_FiresBootstrapTasks(t *testing.T) {
	c := newCore()
	s := c.session
	s.ConnectGen = 2
	s.Clients = &awsclient.ServiceClients{}

	_, tasks := c.HandleClientsReady(ClientsReadyEvent{
		Gen: 2, NewGen: 3,
		Err: errors.New("oops"),
	})

	if !hasTaskKind(tasks, TaskKindFetchIdentity) {
		t.Error("expected TaskKindFetchIdentity when session has existing clients")
	}
	if !hasTaskKind(tasks, TaskKindLoadAvailCache) {
		t.Error("expected TaskKindLoadAvailCache when session has existing clients and NoCache=false")
	}
}

// TestHandleClientsReady_Failure_WithExistingClients_NoCache: NoCache=true →
// DemoPrefetchCounts instead of LoadAvailCache.
func TestHandleClientsReady_Failure_WithExistingClients_NoCache(t *testing.T) {
	c := newCore()
	s := c.session
	s.ConnectGen = 2
	s.Clients = &awsclient.ServiceClients{}
	s.NoCache = true

	_, tasks := c.HandleClientsReady(ClientsReadyEvent{
		Gen: 2, NewGen: 3,
		Err: errors.New("oops"),
	})

	if !hasTaskKind(tasks, TaskKindFetchIdentity) {
		t.Error("expected TaskKindFetchIdentity")
	}
	if !hasTaskKind(tasks, TaskKindDemoPrefetchCounts) {
		t.Error("expected TaskKindDemoPrefetchCounts when NoCache=true")
	}
	if hasTaskKind(tasks, TaskKindLoadAvailCache) {
		t.Error("unexpected TaskKindLoadAvailCache when NoCache=true")
	}
}

// TestHandleClientsReady_Success_PreSuppliedClients: ev.Clients==nil with
// PreSuppliedClients set → installs PreSuppliedClients into session.Clients.
func TestHandleClientsReady_Success_PreSuppliedClients(t *testing.T) {
	c := newCore()
	s := c.session
	s.ConnectGen = 1
	pre := &awsclient.ServiceClients{}
	s.PreSuppliedClients = pre

	c.HandleClientsReady(ClientsReadyEvent{ //nolint:ineffassign,staticcheck // crash-verification
		Gen: 1, NewGen: 2,
		Clients: nil, // triggers PreSuppliedClients path
	})

	if s.Clients != pre {
		t.Error("expected session.Clients to be set to PreSuppliedClients")
	}
}

// TestHandleClientsReady_Success_InstallsClients: ev.Clients as *ServiceClients
// → installs into session.Clients.
func TestHandleClientsReady_Success_InstallsClients(t *testing.T) {
	c := newCore()
	s := c.session
	s.ConnectGen = 1
	fresh := &awsclient.ServiceClients{}

	c.HandleClientsReady(ClientsReadyEvent{ //nolint:ineffassign,staticcheck // crash-verification
		Gen: 1, NewGen: 2,
		Clients: fresh,
	})

	if s.Clients != fresh {
		t.Error("expected session.Clients to be set to ev.Clients")
	}
	if s.HasPrevState {
		t.Error("HasPrevState should be cleared on success")
	}
}

// TestHandleClientsReady_Success_PendingRefreshWithActiveRL: PendingRefresh=true
// AND HasActiveRL=true → RefreshActiveListIntent + "Connected. Refreshing..."
// flash, PendingRefresh cleared.
func TestHandleClientsReady_Success_PendingRefreshWithActiveRL(t *testing.T) {
	c := newCore()
	s := c.session
	s.ConnectGen = 1
	s.PendingRefresh = true

	intents, _ := c.HandleClientsReady(ClientsReadyEvent{
		Gen: 1, NewGen: 2,
		Clients:     &awsclient.ServiceClients{},
		HasActiveRL: true,
	})

	if !findRefreshActiveList(intents) {
		t.Error("expected RefreshActiveListIntent when PendingRefresh=true and HasActiveRL=true")
	}
	fi, ok := findFlashIntent(intents)
	if !ok {
		t.Error("expected FlashIntent for PendingRefresh path")
	}
	if fi.Text != "Connected. Refreshing..." {
		t.Errorf("FlashIntent.Text = %q, want %q", fi.Text, "Connected. Refreshing...")
	}
	if s.PendingRefresh {
		t.Error("PendingRefresh should be cleared after refresh")
	}
}

// TestHandleClientsReady_Success_PendingRefresh_NoActiveRL: PendingRefresh=true
// but HasActiveRL=false → no RefreshActiveListIntent, PendingRefresh cleared.
func TestHandleClientsReady_Success_PendingRefresh_NoActiveRL(t *testing.T) {
	c := newCore()
	s := c.session
	s.ConnectGen = 1
	s.PendingRefresh = true

	intents, _ := c.HandleClientsReady(ClientsReadyEvent{
		Gen: 1, NewGen: 2,
		Clients:     &awsclient.ServiceClients{},
		HasActiveRL: false,
	})

	if findRefreshActiveList(intents) {
		t.Error("unexpected RefreshActiveListIntent when HasActiveRL=false")
	}
	if s.PendingRefresh {
		t.Error("PendingRefresh should still be cleared even when HasActiveRL=false")
	}
}

// TestHandleClientsReady_Success_NoCache: NoCache=true → DemoPrefetchCounts
// task instead of FetchIdentity + LoadAvailCache.
func TestHandleClientsReady_Success_NoCache(t *testing.T) {
	c := newCore()
	s := c.session
	s.ConnectGen = 1
	s.NoCache = true

	_, tasks := c.HandleClientsReady(ClientsReadyEvent{
		Gen: 1, NewGen: 2,
		Clients: &awsclient.ServiceClients{},
	})

	if !hasTaskKind(tasks, TaskKindDemoPrefetchCounts) {
		t.Error("expected TaskKindDemoPrefetchCounts when NoCache=true")
	}
	if hasTaskKind(tasks, TaskKindFetchIdentity) {
		t.Error("unexpected TaskKindFetchIdentity when NoCache=true")
	}
	if hasTaskKind(tasks, TaskKindLoadAvailCache) {
		t.Error("unexpected TaskKindLoadAvailCache when NoCache=true")
	}
}

// TestHandleClientsReady_Success_LivePath: normal live AWS path → FetchIdentity
// + LoadAvailCache tasks.
func TestHandleClientsReady_Success_LivePath(t *testing.T) {
	c := newCore()
	s := c.session
	s.ConnectGen = 1

	_, tasks := c.HandleClientsReady(ClientsReadyEvent{
		Gen: 1, NewGen: 2,
		Clients: &awsclient.ServiceClients{},
	})

	if !hasTaskKind(tasks, TaskKindFetchIdentity) {
		t.Error("expected TaskKindFetchIdentity on live path")
	}
	if !hasTaskKind(tasks, TaskKindLoadAvailCache) {
		t.Error("expected TaskKindLoadAvailCache on live path")
	}
}

// TestHandleClientsReady_Success_Command_StackDepth1: Command set + StackDepth==1
// → EmitNavigate task + Command cleared.
func TestHandleClientsReady_Success_Command_StackDepth1(t *testing.T) {
	c := newCore()
	s := c.session
	s.ConnectGen = 1
	s.Command = "ec2"

	_, tasks := c.HandleClientsReady(ClientsReadyEvent{
		Gen: 1, NewGen: 2,
		Clients:    &awsclient.ServiceClients{},
		StackDepth: 1,
	})

	nav, ok := findEmitNavigatePayload(tasks)
	if !ok {
		t.Fatal("expected TaskKindEmitNavigate task when Command set and StackDepth==1")
	}
	if nav.Target != NavigateTargetResourceList {
		t.Errorf("EmitNavigatePayload.Target = %v, want NavigateTargetResourceList", nav.Target)
	}
	if nav.ResourceType != "ec2" {
		t.Errorf("EmitNavigatePayload.ResourceType = %q, want %q", nav.ResourceType, "ec2")
	}
	if s.Command != "" {
		t.Errorf("session.Command should be cleared after use, got %q", s.Command)
	}
}

// TestHandleClientsReady_Success_Command_StackDepth2: Command set but
// StackDepth > 1 → NO EmitNavigate, Command still cleared.
func TestHandleClientsReady_Success_Command_StackDepth2(t *testing.T) {
	c := newCore()
	s := c.session
	s.ConnectGen = 1
	s.Command = "rds"

	_, tasks := c.HandleClientsReady(ClientsReadyEvent{
		Gen: 1, NewGen: 2,
		Clients:    &awsclient.ServiceClients{},
		StackDepth: 2,
	})

	if hasTaskKind(tasks, TaskKindEmitNavigate) {
		t.Error("unexpected TaskKindEmitNavigate when StackDepth > 1")
	}
	if s.Command != "" {
		t.Errorf("session.Command should still be cleared when StackDepth>1, got %q", s.Command)
	}
}

// ---- HandleProfileSelected tests -------------------------------------------

// TestHandleProfileSelected_FirstSwitch: no prior latch → captures current
// Profile/Region as rollback target, bumps ConnectGen, sets new Profile, clears
// Region, sets PendingRefresh, emits correct intents and tasks.
func TestHandleProfileSelected_FirstSwitch(t *testing.T) {
	c := newCore()
	s := c.session
	s.Profile = "original-profile"
	s.Region = "us-east-1"
	initialConnectGen := s.ConnectGen

	intents, tasks := c.HandleProfileSelected(ProfileSelectedEvent{
		Profile: "new-profile",
		NewGen:  5,
	})

	// rollback latch captured before Rotate
	if !s.HasPrevState {
		t.Error("HasPrevState should be true after first switch")
	}
	if s.PrevProfile != "original-profile" {
		t.Errorf("PrevProfile = %q, want %q", s.PrevProfile, "original-profile")
	}
	if s.PrevRegion != "us-east-1" {
		t.Errorf("PrevRegion = %q, want %q", s.PrevRegion, "us-east-1")
	}

	// ConnectGen bumped by Rotate
	if s.ConnectGen != initialConnectGen+1 {
		t.Errorf("ConnectGen = %d, want %d", s.ConnectGen, initialConnectGen+1)
	}

	// new profile set, region cleared
	if s.Profile != "new-profile" {
		t.Errorf("Profile = %q, want %q", s.Profile, "new-profile")
	}
	if s.Region != "" {
		t.Errorf("Region = %q, want empty after profile switch", s.Region)
	}

	// PendingRefresh set
	if !s.PendingRefresh {
		t.Error("PendingRefresh should be true after profile switch")
	}

	// intents: MenuClearAvailability, PopSelector, Flash
	if !findMenuClearAvailability(intents) {
		t.Error("expected MenuClearAvailabilityIntent")
	}
	if !findPopSelector(intents) {
		t.Error("expected PopSelectorIntent")
	}
	fi, ok := findFlashIntent(intents)
	if !ok {
		t.Fatal("expected FlashIntent")
	}
	if fi.Text != "Switching to new-profile..." {
		t.Errorf("FlashIntent.Text = %q, want %q", fi.Text, "Switching to new-profile...")
	}

	// tasks: Connect + FlashTick
	cp, ok := findConnectPayload(tasks)
	if !ok {
		t.Fatal("expected TaskKindConnect")
	}
	if cp.Profile != "new-profile" {
		t.Errorf("ConnectPayload.Profile = %q, want %q", cp.Profile, "new-profile")
	}
	if cp.Region != "" {
		t.Errorf("ConnectPayload.Region = %q, want empty", cp.Region)
	}
	if cp.Gen != s.ConnectGen {
		t.Errorf("ConnectPayload.Gen = %d, want %d", cp.Gen, s.ConnectGen)
	}
	tick, ok := findFlashTick(tasks)
	if !ok {
		t.Fatal("expected FlashTickPayload task")
	}
	if tick.Gen != 5 {
		t.Errorf("FlashTickPayload.Gen = %d, want 5", tick.Gen)
	}
	if tick.Duration != 2*time.Second {
		t.Errorf("FlashTickPayload.Duration = %v, want 2s", tick.Duration)
	}
}

// TestHandleProfileSelected_SecondSwitch_PreservesRollbackTarget: rapid A→B→C
// case — second switch must keep A (not B) as the rollback target.
func TestHandleProfileSelected_SecondSwitch_PreservesRollbackTarget(t *testing.T) {
	c := newCore()
	s := c.session
	s.Profile = "profile-A"
	s.Region = "us-east-1"

	// First switch A→B
	c.HandleProfileSelected(ProfileSelectedEvent{Profile: "profile-B", NewGen: 1}) //nolint:ineffassign,staticcheck // return values intentionally ignored

	// At this point PrevProfile should be "profile-A"
	if s.PrevProfile != "profile-A" {
		t.Fatalf("after first switch PrevProfile = %q, want %q", s.PrevProfile, "profile-A")
	}

	// Second switch B→C; must keep rollback target as A
	c.HandleProfileSelected(ProfileSelectedEvent{Profile: "profile-C", NewGen: 2}) //nolint:ineffassign,staticcheck // return values intentionally ignored

	if s.PrevProfile != "profile-A" {
		t.Errorf("after second switch PrevProfile = %q, want %q (rapid A→B→C must keep A)", s.PrevProfile, "profile-A")
	}
}

// ---- HandleRegionSelected tests --------------------------------------------

// TestHandleRegionSelected_FirstSwitch: mirrors HandleProfileSelected — captures
// rollback latch, bumps ConnectGen, sets region, preserves profile in ConnectPayload.
func TestHandleRegionSelected_FirstSwitch(t *testing.T) {
	c := newCore()
	s := c.session
	s.Profile = "my-profile"
	s.Region = "eu-west-1"
	initialConnectGen := s.ConnectGen

	intents, tasks := c.HandleRegionSelected(RegionSelectedEvent{
		Region: "ap-southeast-1",
		NewGen: 8,
	})

	// rollback latch
	if !s.HasPrevState {
		t.Error("HasPrevState should be true after first region switch")
	}
	if s.PrevProfile != "my-profile" {
		t.Errorf("PrevProfile = %q, want %q", s.PrevProfile, "my-profile")
	}
	if s.PrevRegion != "eu-west-1" {
		t.Errorf("PrevRegion = %q, want %q", s.PrevRegion, "eu-west-1")
	}

	// ConnectGen bumped
	if s.ConnectGen != initialConnectGen+1 {
		t.Errorf("ConnectGen = %d, want %d", s.ConnectGen, initialConnectGen+1)
	}

	// new region set
	if s.Region != "ap-southeast-1" {
		t.Errorf("Region = %q, want %q", s.Region, "ap-southeast-1")
	}

	// PendingRefresh set
	if !s.PendingRefresh {
		t.Error("PendingRefresh should be true after region switch")
	}

	// intents
	if !findMenuClearAvailability(intents) {
		t.Error("expected MenuClearAvailabilityIntent")
	}
	if !findPopSelector(intents) {
		t.Error("expected PopSelectorIntent")
	}
	fi, ok := findFlashIntent(intents)
	if !ok {
		t.Fatal("expected FlashIntent")
	}
	if fi.Text != "Switching to ap-southeast-1..." {
		t.Errorf("FlashIntent.Text = %q, want %q", fi.Text, "Switching to ap-southeast-1...")
	}

	// ConnectPayload preserves existing Profile, sets new Region
	cp, ok := findConnectPayload(tasks)
	if !ok {
		t.Fatal("expected TaskKindConnect")
	}
	if cp.Profile != "my-profile" {
		t.Errorf("ConnectPayload.Profile = %q, want %q", cp.Profile, "my-profile")
	}
	if cp.Region != "ap-southeast-1" {
		t.Errorf("ConnectPayload.Region = %q, want %q", cp.Region, "ap-southeast-1")
	}
	if cp.Gen != s.ConnectGen {
		t.Errorf("ConnectPayload.Gen = %d, want %d", cp.Gen, s.ConnectGen)
	}
	tick, ok := findFlashTick(tasks)
	if !ok {
		t.Fatal("expected FlashTickPayload task")
	}
	if tick.Gen != 8 {
		t.Errorf("FlashTickPayload.Gen = %d, want 8", tick.Gen)
	}
	if tick.Duration != 2*time.Second {
		t.Errorf("FlashTickPayload.Duration = %v, want 2s", tick.Duration)
	}
}

// TestHandleRegionSelected_SecondSwitch_PreservesRollbackTarget: rapid R1→R2→R3
// case keeps R1 as rollback target.
func TestHandleRegionSelected_SecondSwitch_PreservesRollbackTarget(t *testing.T) {
	c := newCore()
	s := c.session
	s.Profile = "p"
	s.Region = "us-east-1"

	c.HandleRegionSelected(RegionSelectedEvent{Region: "eu-west-1", NewGen: 1}) //nolint:ineffassign,staticcheck // return values intentionally ignored

	if s.PrevRegion != "us-east-1" {
		t.Fatalf("after first switch PrevRegion = %q, want %q", s.PrevRegion, "us-east-1")
	}

	c.HandleRegionSelected(RegionSelectedEvent{Region: "ap-southeast-1", NewGen: 2}) //nolint:ineffassign,staticcheck // return values intentionally ignored

	if s.PrevRegion != "us-east-1" {
		t.Errorf("after second switch PrevRegion = %q, want %q (rapid switch must keep original)", s.PrevRegion, "us-east-1")
	}
}

// TestHandleRegionSelected_ConnectPayload_ProfilePreserved: the ConnectPayload
// must carry the session's current Profile (not empty string) when the region
// changes.
func TestHandleRegionSelected_ConnectPayload_ProfilePreserved(t *testing.T) {
	c := newCore()
	s := c.session
	s.Profile = "prod"
	s.Region = "us-west-2"

	_, tasks := c.HandleRegionSelected(RegionSelectedEvent{Region: "us-east-2", NewGen: 1})

	cp, ok := findConnectPayload(tasks)
	if !ok {
		t.Fatal("expected TaskKindConnect")
	}
	// Profile must come from s.Profile (captured before Rotate clears it —
	// the handler assigns s.Region = ev.Region but never touches s.Profile).
	// After Rotate s.Profile is still "prod" (Rotate does not clear Profile).
	if cp.Profile != "prod" {
		t.Errorf("ConnectPayload.Profile = %q, want %q", cp.Profile, "prod")
	}
}

// TestHandleClientsReady_Success_ClearsHasPrevState: success path always clears
// HasPrevState, PrevProfile, PrevRegion.
func TestHandleClientsReady_Success_ClearsHasPrevState(t *testing.T) {
	c := newCore()
	s := c.session
	s.ConnectGen = 1
	s.HasPrevState = true
	s.PrevProfile = "was-this"
	s.PrevRegion = "was-there"

	c.HandleClientsReady(ClientsReadyEvent{ //nolint:ineffassign,staticcheck // crash-verification
		Gen: 1, NewGen: 2,
		Clients: &awsclient.ServiceClients{},
	})

	if s.HasPrevState {
		t.Error("HasPrevState should be cleared on success")
	}
	if s.PrevProfile != "" {
		t.Errorf("PrevProfile should be cleared, got %q", s.PrevProfile)
	}
	if s.PrevRegion != "" {
		t.Errorf("PrevRegion should be cleared, got %q", s.PrevRegion)
	}
}

// TestHandleFlash_FlashTickKind: FlashTick task always uses TaskKindFlashTick.
func TestHandleFlash_FlashTickKind(t *testing.T) {
	c := newCore()
	_, tasks := c.HandleFlash(FlashEvent{Text: "x", IsError: false, NewGen: 1})
	if !hasTaskKind(tasks, TaskKindFlashTick) {
		t.Error("expected TaskKindFlashTick task")
	}
}

// TestHandleAPIError_FlashTickKind: HandleAPIError always uses TaskKindFlashTick.
func TestHandleAPIError_FlashTickKind(t *testing.T) {
	c := newCore()
	_, tasks := c.HandleAPIError(APIErrorEvent{Err: errors.New("e"), NewGen: 1})
	if !hasTaskKind(tasks, TaskKindFlashTick) {
		t.Error("expected TaskKindFlashTick task")
	}
}

// TestHandleClearFlash_ZeroGen_IsStale: Gen=0, CurrentGen=0 are EQUAL so not
// stale — should emit ClearFlash (verifies the stale guard is Gen != CurrentGen,
// not Gen < CurrentGen).
func TestHandleClearFlash_ZeroGen_BothZero_NotStale(t *testing.T) {
	c := newCore()
	intents, _ := c.HandleClearFlash(ClearFlashEvent{Gen: 0, CurrentGen: 0, IsError: false})
	if !findClearFlash(intents) {
		t.Error("Gen==CurrentGen==0 should NOT be stale; expected ClearFlash intent")
	}
}

// TestHandleProfileSelected_ConnectGen_UsedInPayload: ConnectPayload.Gen must
// equal the post-Rotate ConnectGen (the gen captured after Rotate, not before).
func TestHandleProfileSelected_ConnectGen_UsedInPayload(t *testing.T) {
	c := newCore()
	s := c.session
	s.Profile = "p"
	s.Region = "r"
	// Force a specific starting ConnectGen
	s.ConnectGen = 10

	_, tasks := c.HandleProfileSelected(ProfileSelectedEvent{Profile: "q", NewGen: 1})

	cp, ok := findConnectPayload(tasks)
	if !ok {
		t.Fatal("expected TaskKindConnect")
	}
	// Rotate bumps ConnectGen from 10 to 11; ConnectPayload must carry 11.
	if cp.Gen != 11 {
		t.Errorf("ConnectPayload.Gen = %d, want 11 (post-Rotate ConnectGen)", cp.Gen)
	}
}

// TestHandleClientsReady_Failure_NilClients_NoBootstrapTasks: failure with
// session.Clients == nil → no FetchIdentity / LoadAvailCache tasks.
func TestHandleClientsReady_Failure_NilClients_NoBootstrapTasks(t *testing.T) {
	c := newCore()
	s := c.session
	s.ConnectGen = 1
	// s.Clients is nil (default from session.New())

	_, tasks := c.HandleClientsReady(ClientsReadyEvent{
		Gen: 1, NewGen: 2,
		Err: errors.New("first connect failed"),
	})

	if hasTaskKind(tasks, TaskKindFetchIdentity) {
		t.Error("unexpected TaskKindFetchIdentity when session.Clients == nil")
	}
	if hasTaskKind(tasks, TaskKindLoadAvailCache) {
		t.Error("unexpected TaskKindLoadAvailCache when session.Clients == nil")
	}
	if hasTaskKind(tasks, TaskKindDemoPrefetchCounts) {
		t.Error("unexpected TaskKindDemoPrefetchCounts when session.Clients == nil")
	}
}

// Sentinel to ensure the time import is used (AppendErrorHistoryIntent carries time.Time).
var _ = time.Now
