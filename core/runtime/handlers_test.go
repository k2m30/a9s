// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// handlers_test.go — unit tests for the flash, API-error, connect and
// profile/region handlers.
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

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/session"
)

// newCore returns a Core with a fresh session and nil catalog.
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

func TestHandleFlash_NotError(t *testing.T) {
	c := newCore()
	intents, tasks := c.HandleFlash(FlashEvent{Text: "hello", IsError: false, NewGen: 3})

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

// The error flash is the record, and the history entry is made where the
// flash is applied: a second, history-carrying intent beside the flash would
// let the two hosts disagree about whether a failure was logged.
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

// An error flash IS the history entry, made where it is applied.
func TestHandleAPIError_AlwaysEmitsThreeIntents(t *testing.T) {
	c := newCore()
	intents, _ := c.HandleAPIError(APIErrorEvent{Err: errors.New("any"), NewGen: 1})

	if _, ok := findFlashIntent(intents); !ok {
		t.Error("missing FlashIntent")
	}
	if !findClearActiveListLoading(intents) {
		t.Error("missing ClearActiveListLoadingIntent")
	}
}

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

func TestHandleClientsReady_StaleGen(t *testing.T) {
	c := newCore()
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
	// A failed connect says what failed and why, in the same sentence its
	// error-history entry gets, so the flash and the log entry share one
	// shape.
	if fi.Text != "connect: no route to host" {
		t.Errorf("FlashIntent.Text = %q, want %q", fi.Text, "connect: no route to host")
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

// The TUI adapter's `hasFlashWork` gate relies on this: with
// PendingRefresh=false the success path emits no FlashIntent and no
// FlashTickPayload, so the adapter must not advance m.flash.gen or
// invalidate an in-flight ClearFlashMsg for the current flash.
func TestHandleClientsReady_Success_NoPendingRefresh_NoFlashWork(t *testing.T) {
	c := newCore()
	s := c.session
	s.ConnectGen = 1
	s.PendingRefresh = false

	intents, tasks := c.HandleClientsReady(ClientsReadyEvent{
		Gen: 1, NewGen: 2,
		Clients:     &awsclient.ServiceClients{},
		HasActiveRL: true, // even with an active list, no flash work when PendingRefresh=false
	})

	if _, ok := findFlashIntent(intents); ok {
		t.Error("success-no-pending-refresh must not emit FlashIntent — bumping flash.gen on this path would silently invalidate any in-flight ClearFlashMsg for the current flash")
	}
	for _, ts := range tasks {
		if _, ok := ts.Payload.(FlashTickPayload); ok {
			t.Error("success-no-pending-refresh must not emit FlashTickPayload — bumping flash.gen on this path would silently invalidate any in-flight ClearFlashMsg for the current flash")
		}
	}
}

func TestHandleClientsReady_StaleGen_NoFlashWork(t *testing.T) {
	c := newCore()
	c.session.ConnectGen = 5

	intents, tasks := c.HandleClientsReady(ClientsReadyEvent{
		Gen: 3, NewGen: 6, // ev.Gen != session.ConnectGen
	})

	if _, ok := findFlashIntent(intents); ok {
		t.Error("stale gen must not emit FlashIntent")
	}
	for _, ts := range tasks {
		if _, ok := ts.Payload.(FlashTickPayload); ok {
			t.Error("stale gen must not emit FlashTickPayload")
		}
	}
}

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

// Command set + StackDepth==1 → the live path does NOT emit
// TaskKindEmitNavigate directly. Emitting it here would race handleAvailabilityCacheLoaded's
// session.ProbeResources seed (tea.Batch runs task cmds concurrently),
// landing on a bare "Loading..." list with no title count. Instead
// HandleClientsReady arms the deferred navigation — session.CommandArmed
// is set, session.PendingCommand carries the resource short name — and
// Command itself is cleared immediately (consumed exactly once, regardless
// of eligibility). handleAvailabilityCacheLoaded is the one that actually
// dispatches TaskKindEmitNavigate, once its ProbeResources seed has landed.
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

	if hasTaskKind(tasks, TaskKindEmitNavigate) {
		t.Error("unexpected TaskKindEmitNavigate directly from HandleClientsReady on the live path — the one-shot -c navigation must be armed and deferred to handleAvailabilityCacheLoaded, not fired here (the navigation-race half of D11)")
	}
	if !s.CommandArmed {
		t.Error("session.CommandArmed = false, want true — Command set + StackDepth==1 must arm the deferred navigation")
	}
	if s.PendingCommand != "ec2" {
		t.Errorf("session.PendingCommand = %q, want %q", s.PendingCommand, "ec2")
	}
	if s.Command != "" {
		t.Errorf("session.Command should be cleared after use, got %q", s.Command)
	}
}

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

	if !s.HasPrevState {
		t.Error("HasPrevState should be true after first switch")
	}
	if s.PrevProfile != "original-profile" {
		t.Errorf("PrevProfile = %q, want %q", s.PrevProfile, "original-profile")
	}
	if s.PrevRegion != "us-east-1" {
		t.Errorf("PrevRegion = %q, want %q", s.PrevRegion, "us-east-1")
	}

	if s.ConnectGen != initialConnectGen+1 {
		t.Errorf("ConnectGen = %d, want %d", s.ConnectGen, initialConnectGen+1)
	}

	if s.Profile != "new-profile" {
		t.Errorf("Profile = %q, want %q", s.Profile, "new-profile")
	}
	if s.Region != "" {
		t.Errorf("Region = %q, want empty after profile switch", s.Region)
	}

	if !s.PendingRefresh {
		t.Error("PendingRefresh should be true after profile switch")
	}

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

func TestHandleProfileSelected_SecondSwitch_PreservesRollbackTarget(t *testing.T) {
	c := newCore()
	s := c.session
	s.Profile = "profile-A"
	s.Region = "us-east-1"

	c.HandleProfileSelected(ProfileSelectedEvent{Profile: "profile-B", NewGen: 1}) //nolint:ineffassign,staticcheck // return values intentionally ignored

	if s.PrevProfile != "profile-A" {
		t.Fatalf("after first switch PrevProfile = %q, want %q", s.PrevProfile, "profile-A")
	}

	c.HandleProfileSelected(ProfileSelectedEvent{Profile: "profile-C", NewGen: 2}) //nolint:ineffassign,staticcheck // return values intentionally ignored

	if s.PrevProfile != "profile-A" {
		t.Errorf("after second switch PrevProfile = %q, want %q (rapid A→B→C must keep A)", s.PrevProfile, "profile-A")
	}
}

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

	if !s.HasPrevState {
		t.Error("HasPrevState should be true after first region switch")
	}
	if s.PrevProfile != "my-profile" {
		t.Errorf("PrevProfile = %q, want %q", s.PrevProfile, "my-profile")
	}
	if s.PrevRegion != "eu-west-1" {
		t.Errorf("PrevRegion = %q, want %q", s.PrevRegion, "eu-west-1")
	}

	if s.ConnectGen != initialConnectGen+1 {
		t.Errorf("ConnectGen = %d, want %d", s.ConnectGen, initialConnectGen+1)
	}

	if s.Region != "ap-southeast-1" {
		t.Errorf("Region = %q, want %q", s.Region, "ap-southeast-1")
	}

	if !s.PendingRefresh {
		t.Error("PendingRefresh should be true after region switch")
	}

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
	// Rotate does not clear Profile.
	if cp.Profile != "prod" {
		t.Errorf("ConnectPayload.Profile = %q, want %q", cp.Profile, "prod")
	}
}

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

func TestHandleFlash_FlashTickKind(t *testing.T) {
	c := newCore()
	_, tasks := c.HandleFlash(FlashEvent{Text: "x", IsError: false, NewGen: 1})
	if !hasTaskKind(tasks, TaskKindFlashTick) {
		t.Error("expected TaskKindFlashTick task")
	}
}

func TestHandleAPIError_FlashTickKind(t *testing.T) {
	c := newCore()
	_, tasks := c.HandleAPIError(APIErrorEvent{Err: errors.New("e"), NewGen: 1})
	if !hasTaskKind(tasks, TaskKindFlashTick) {
		t.Error("expected TaskKindFlashTick task")
	}
}

func TestHandleClearFlash_ZeroGen_BothZero_NotStale(t *testing.T) {
	c := newCore()
	intents, _ := c.HandleClearFlash(ClearFlashEvent{Gen: 0, CurrentGen: 0, IsError: false})
	if !findClearFlash(intents) {
		t.Error("Gen==CurrentGen==0 should NOT be stale; expected ClearFlash intent")
	}
}

func TestHandleProfileSelected_ConnectGen_UsedInPayload(t *testing.T) {
	c := newCore()
	s := c.session
	s.Profile = "p"
	s.Region = "r"
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

func TestHandleClientsReady_Failure_NilClients_NoBootstrapTasks(t *testing.T) {
	c := newCore()
	s := c.session
	s.ConnectGen = 1

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

// Session.Rotate() installs fresh per-session stores (PolicyStore /
// IdentityStore / RuleSetStore); the failure path must rewire the retained
// transport with them, or Pattern-C related checkers (Glue tags, EBS Backup)
// and IAM lazy-add read the discarded pre-rotate stores until the next
// successful reconnect.
func TestHandleClientsReady_Failure_RewiresPostRotateStores(t *testing.T) {
	c := newCore()
	s := c.session

	preRotateIAM := session.NewPolicyStore()
	preRotateID := session.NewIdentityStore()
	preRotateRS := session.NewRuleSetStore()
	sc := &awsclient.ServiceClients{}
	sc.SetIAMPolicies(preRotateIAM)
	sc.SetIdentityStore(preRotateID)
	sc.SetRuleSets(preRotateRS)
	s.Clients = sc

	// session.New() already installed fresh "post-rotate" stores on
	// s.IAMPolicies / s.IdentityStore / s.RuleSets — capture them for
	// comparison. They MUST be distinct from the pre-rotate stores wired
	// onto sc above.
	postRotateIAM := s.IAMPolicies
	postRotateID := s.IdentityStore
	postRotateRS := s.RuleSets
	if postRotateIAM == preRotateIAM || postRotateID == preRotateID || postRotateRS == preRotateRS {
		t.Fatal("test setup error: pre- and post-rotate stores must be distinct references")
	}

	s.ConnectGen = 1
	c.HandleClientsReady(ClientsReadyEvent{ //nolint:ineffassign,staticcheck // crash-verification
		Gen: 1, NewGen: 2,
		Err: errors.New("connect failed"),
	})

	if sc.IAMPolicies() != postRotateIAM {
		t.Error("failure path must rewire retained Clients with post-rotate IAMPolicies (P3 invariant)")
	}
	if sc.IdentityStore() != postRotateID {
		t.Error("failure path must rewire retained Clients with post-rotate IdentityStore (P3 invariant)")
	}
	if sc.RuleSets() != postRotateRS {
		t.Error("failure path must rewire retained Clients with post-rotate RuleSets (P3 invariant)")
	}
}

// The TUI adapter gates its flash.gen bump on this (nil, nil): bumping on a
// stale dispatch would invalidate the ClearFlashMsg already in flight for
// the current flash, leaving it stuck on screen. The adapter side is the
// `len(intents) > 0 || len(tasks) > 0` guard in app_session.go.
func TestHandleClientsReady_StaleGen_ReturnsEmpty(t *testing.T) {
	c := newCore()
	c.session.ConnectGen = 5

	intents, tasks := c.HandleClientsReady(ClientsReadyEvent{
		Gen: 3, NewGen: 99, // stale Gen
		Err: errors.New("ignored — stale"),
	})

	if len(intents) != 0 {
		t.Errorf("stale Gen must return empty intents, got %d", len(intents))
	}
	if len(tasks) != 0 {
		t.Errorf("stale Gen must return empty tasks, got %d", len(tasks))
	}
}
