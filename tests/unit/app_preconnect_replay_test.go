// app_preconnect_replay_test.go — RED regression pins for the pre-connect
// replay (docs/design/cache-requirements.md C10): a navigation issued BEFORE the
// AWS connect completes renders the cached list correctly (C1), but its
// fetch task fails with "AWS clients not initialized" and is never replayed
// once the connect lands — the list is stuck on cached rows plus a
// permanent LastFetchError forever. C10 requires the last navigation to
// replay automatically once connected.
//
// Root causes pinned here (implementation lands in parallel):
//  1. session.New() must seed PendingRefresh: true so a fresh session (no
//     prior profile/region switch) still has a pending post-connect
//     refresh armed at STARTUP, not only after HandleProfileSelected /
//     HandleRegionSelected set it on switch.
//  2. BootstrapLive must pass the REAL StackDepth (len(c.stack)) and
//     HasActiveRL (c.topListState() != nil) into ClientsReadyEvent instead
//     of the hardcoded StackDepth: 1 / omitted HasActiveRL — otherwise
//     maybeRefreshIntents (core/runtime/handlers.go) can never see an
//     active list and never emits RefreshActiveListIntent.
//  3. RefreshActiveListIntent is a documented no-op in the headless
//     controller (core/app/intents.go's ApplyIntents default-case
//     comment) — some new controller-side mechanism (a helper the coder
//     will name activeListRefreshTasks, extracted from
//     handleActionRefresh's list branch, core/app/actions_list.go
//     lines 184-199) must turn that intent into a real
//     KindFetchResources task for the active list's type, at BOTH:
//     - BootstrapLive's return (the STARTUP connect seam), and
//     - Controller.Handle's messages.ClientsReady path (the web
//     profile-switch reconnect seam) — reached when DrainSync/
//     DrainSyncPartition executes a TaskKindConnect task and feeds the
//     resulting messages.ClientsReady through Controller.Handle
//     (core/app/drainsync.go, c.Handle(ev) at the end of the loop
//     body). NOTE: as committed at HEAD, core/runtime/orchestrator.go's
//     Core.HandleEvent switch has NO case for messages.ClientsReady at
//     all (it is explicitly documented as a TUI-shim-only event,
//     handled outside HandleEvent) — so today a ClientsReady fed
//     through Controller.Handle hits the default nil,nil branch and is
//     silently dropped for headless/web callers. Wiring this seam is
//     therefore part of the pre-connect replay fix, not a pre-existing green path;
//     test 1 below drives this exact path and pins the TARGET (fixed)
//     behavior.
//
// All tests are hermetic: A9S_CONFIG_FOLDER redirected to t.TempDir(), no
// AWS credentials, no network. Cache seeding uses the real
// cache.LoadDirForTest/Put/SaveType per-type-file surface (mirrors
// TestPerTypeSave_TouchingOneType_LeavesSiblingFilesByteExact in
// app_web_live_cold_boot_test.go). Fake profile/region names only
// ("pilot-prof"/"us-east-1"); realistic resource IDs.
package unit_test

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/cache"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
)

// seedS3TypeFile writes a single-type disk cache (mirrors the
// TestPerTypeSave_TouchingOneType_LeavesSiblingFilesByteExact fixture
// pattern) so a controller built against the same profile/region pair can
// warm-seed ProbeResources from disk via EnsureCacheStore/CacheStoreToEvent,
// exactly as core/web/construct.go's newSession does for the live,
// no-pre-supplied-clients path.
func seedS3TypeFile(t *testing.T, profile, region string) {
	t.Helper()
	store := cache.LoadDirForTest(profile, region)
	if store == nil {
		t.Fatal("cache.LoadDirForTest on an empty directory returned nil — must return an empty (non-nil) Store")
	}
	store.Put("s3", cache.TypeFile{
		HasResources: true,
		Count:        2,
		Rows: []cache.Row{
			{ID: "preconnect-bucket-1", Name: "preconnect-bucket-1", Fields: map[string]string{"region": region}},
			{ID: "preconnect-bucket-2", Name: "preconnect-bucket-2", Fields: map[string]string{"region": region}},
		},
	})
	if err := store.SaveType("s3"); err != nil {
		t.Fatalf("SaveType(s3) fixture write: %v", err)
	}
}

// newHermeticLiveController builds a live (non-demo) controller against a
// temp-dir-redirected cache, exactly like newLiveWebStyleController in
// app_web_live_cold_boot_test.go, but does not seed any clients — the
// scenario under test is explicitly "navigation issued BEFORE the AWS
// connect completes".
func newHermeticLiveController(t *testing.T, profile, region string) (*runtime.Core, *app.Controller) {
	t.Helper()
	core := runtime.Bootstrap(profile, region, resource.AllResourceTypes())
	ctrl := newBlessedController(t, core)
	t.Cleanup(ctrl.Close)
	ctrl.SetUIMode("web")
	return core, ctrl
}

// TestSessionNew_PendingRefreshTrue pins root cause #1 directly: a freshly
// constructed Session (no profile/region switch has happened yet) must
// already carry PendingRefresh: true, so the very first STARTUP connect can
// fire the replay just as a post-switch reconnect does today via
// HandleProfileSelected/HandleRegionSelected (which explicitly set
// s.PendingRefresh = true on switch).
func TestSessionNew_PendingRefreshTrue(t *testing.T) {
	s := session.New()
	if !s.PendingRefresh {
		t.Error("session.New().PendingRefresh = false, want true — C10: a fresh session must arm the post-connect replay at startup, not only after a profile/region switch")
	}
}

// TestPreConnectNavigate_ReplaysFetchOnClientsReady pins the pre-connect replay end-to-end
// at the seam the web lane actually uses.
//
// Setup: a live controller (no clients) whose disk cache already has s3
// rows (seedS3TypeFile) is warmed via EnsureCacheStore/CacheStoreToEvent —
// the same mechanism core/web/construct.go's newSession runs for the
// live no-pre-supplied-clients path — so the very first snapshot carries
// cached counts (C1 precondition).
//
// The test then applies the pre-connect navigation
// (app.Action{Kind: app.ActionCommand, Arg: "s3"}), asserting the cached
// rows render immediately (C1 half of the contract, already green today per
// TestWebBoot_AvailabilityCacheLoaded_DoesNotSeedProbeResourcesRows's
// sibling pattern — this assertion here is the regression guard that stops
// a future change from breaking C1 while fixing C10).
//
// It then drives the connect-completion seam exactly the way DrainSync does
// for a TaskKindConnect result: feeding a messages.ClientsReady event
// through Controller.Handle (this is what a profile-switch reconnect's
// TaskKindConnect task result flows through, per
// core/app/drainsync.go's `_, followUp := c.Handle(ev)`). The RETURNED
// follow-up tasks must include a KindFetchResources task scoped to "s3" —
// the navigated type — proving the last pre-connect navigation is replayed
// automatically once connected, instead of being permanently dropped.
func TestPreConnectNavigate_ReplaysFetchOnClientsReady(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	const profile, region = "pilot-prof", "us-east-1"
	seedS3TypeFile(t, profile, region)

	core, ctrl := newHermeticLiveController(t, profile, region)

	if store := core.EnsureCacheStore(); store != nil {
		ev := runtime.CacheStoreToEvent(store)
		ctrl.Handle(ev)
	} else {
		t.Fatal("core.EnsureCacheStore() returned nil — cache seeding fixture setup failed")
	}

	_, _ = ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: "s3"})

	snap := ctrl.Snapshot()
	if snap.Body.Kind != app.BodyKindList {
		t.Fatalf("Body.Kind = %q, want %q — pre-connect navigation must still open the s3 list screen", snap.Body.Kind, app.BodyKindList)
	}
	lb := snap.Body.List
	if lb == nil {
		t.Fatal("Body.List is nil after pre-connect s3 navigation")
	}
	if lb.Loading {
		t.Error("Loading = true, want false — C1: the cached s3 list must render immediately even before the AWS connect completes")
	}
	if len(lb.Rows) != 2 {
		t.Fatalf("len(Rows) = %d, want 2 — cached rows must be visible pre-connect", len(lb.Rows))
	}

	_, followUp := ctrl.Handle(messages.ClientsReady{
		Clients: nil,
		Region:  region,
		Gen:     core.ConnectGen(),
		Err:     nil,
	})

	hasReplayFetch := false
	for _, task := range followUp {
		if task.Key.Kind == runtime.KindFetchResources && task.Key.Scope == "s3" {
			hasReplayFetch = true
			break
		}
	}
	if !hasReplayFetch {
		t.Errorf("Controller.Handle(messages.ClientsReady) follow-up tasks = %+v, want a KindFetchResources task scoped to \"s3\" — C10: the last pre-connect navigation must replay automatically once the AWS connect lands, instead of leaving the list on cached rows with no fetch ever retried", followUp)
	}
}

// TestMenuOnlyStartup_NoRefreshTask pins the non-regression half of the pre-connect replay:
// a session that never navigated away from the main menu before connecting
// must NOT get a spurious replay fetch — there is no "last navigation" to
// replay. It also pins that PendingRefresh is consumed (one-shot): a SECOND
// ClientsReady landing after the first must not re-fire either, mirroring
// maybeRefreshIntents' existing "c.session.PendingRefresh = false" clear-on-
// consume behavior for the profile-switch path.
func TestMenuOnlyStartup_NoRefreshTask(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	const profile, region = "pilot-prof", "us-east-1"

	core, ctrl := newHermeticLiveController(t, profile, region)
	// No navigation at all — the stack stays on the menu.

	snap := ctrl.Snapshot()
	if snap.Body.Kind != app.BodyKindMenu {
		t.Fatalf("test setup: Body.Kind = %q, want %q (menu-only startup, no pre-connect navigation)", snap.Body.Kind, app.BodyKindMenu)
	}

	_, followUp := ctrl.Handle(messages.ClientsReady{
		Clients: nil,
		Region:  region,
		Gen:     core.ConnectGen(),
		Err:     nil,
	})
	for _, task := range followUp {
		if task.Key.Kind == runtime.KindFetchResources {
			t.Errorf("Controller.Handle(messages.ClientsReady) on a menu-only startup returned a KindFetchResources task (scope=%q) — no list was ever navigated to, so there is nothing to replay", task.Key.Scope)
		}
	}

	// PendingRefresh must be consumed after the first ClientsReady — a
	// second one (e.g. a stray duplicate delivery) must not re-fire either.
	_, followUp2 := ctrl.Handle(messages.ClientsReady{
		Clients: nil,
		Region:  region,
		Gen:     core.ConnectGen(),
		Err:     nil,
	})
	for _, task := range followUp2 {
		if task.Key.Kind == runtime.KindFetchResources {
			t.Errorf("second Controller.Handle(messages.ClientsReady) returned a KindFetchResources task (scope=%q) — PendingRefresh must be consumed (cleared) after the first ClientsReady, not re-armed indefinitely", task.Key.Scope)
		}
	}
}

// TestPreConnectNavigate_ReplayDrain_ClearsLastFetchError is the optional
// C4 error-marker companion: after the replay fetch task returned by
// ClientsReady is actually drained (fed through Controller.Handle as a
// messages.ResourcesLoaded result, the same lane DrainSync uses), the list's
// LastFetchError marker must be cleared and fresh rows must be showing —
// proving the replay is not just a task being RETURNED but one whose result
// actually reaches the screen.
//
// This seeds LastFetchError directly via a prior messages.APIError landing
// on the s3 list (mirrors C4's "keeps the content, swaps the marker for
// an error marker" contract already pinned elsewhere) to model the exact
// failure state the pre-connect replay describes: "AWS clients not initialized" left a
// permanent error marker on the pre-connect fetch attempt.
func TestPreConnectNavigate_ReplayDrain_ClearsLastFetchError(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	const profile, region = "pilot-prof", "us-east-1"
	seedS3TypeFile(t, profile, region)

	core, ctrl := newHermeticLiveController(t, profile, region)
	if store := core.EnsureCacheStore(); store != nil {
		ctrl.Handle(runtime.CacheStoreToEvent(store))
	} else {
		t.Fatal("core.EnsureCacheStore() returned nil — cache seeding fixture setup failed")
	}

	_, _ = ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: "s3"})

	// Model the pre-connect fetch's "AWS clients not initialized" failure
	// landing on the list before the connect completes.
	ctrl.Handle(messages.APIError{
		Err:          errPreConnectClientsNotInitialized{},
		ResourceType: "s3",
		Gen:          core.AvailabilityGen(),
	})
	preReplay := ctrl.Snapshot().Body.List
	if preReplay == nil || preReplay.LastFetchError == "" {
		t.Skip("test setup could not reproduce a non-empty LastFetchError via messages.APIError — skipping the drain half of the pre-connect replay (C4) as disproportionate to reproduce hermetically without the coder's fixed wiring")
	}

	_, followUp := ctrl.Handle(messages.ClientsReady{
		Clients: nil,
		Region:  region,
		Gen:     core.ConnectGen(),
		Err:     nil,
	})

	var replayTask *runtime.TaskRequest
	for i := range followUp {
		if followUp[i].Key.Kind == runtime.KindFetchResources && followUp[i].Key.Scope == "s3" {
			replayTask = &followUp[i]
			break
		}
	}
	if replayTask == nil {
		t.Fatal("no replay KindFetchResources task returned — cannot exercise the drain half of the pre-connect replay (C4) (test 1 already pins the missing-replay defect directly)")
	}

	freshRows := []resource.Resource{
		{ID: "preconnect-bucket-1", Name: "preconnect-bucket-1", Type: "s3", Fields: map[string]string{"region": region}},
		{ID: "preconnect-bucket-2", Name: "preconnect-bucket-2", Type: "s3", Fields: map[string]string{"region": region}},
	}
	ctrl.Handle(messages.ResourcesLoaded{
		ResourceType: "s3",
		Resources:    freshRows,
		Gen:          core.AvailabilityGen(), Provenance: messages.FetchProvenanceCanonicalList,
	})

	postReplay := ctrl.Snapshot().Body.List
	if postReplay == nil {
		t.Fatal("Body.List is nil after draining the replay fetch")
	}
	if postReplay.LastFetchError != "" {
		t.Errorf("LastFetchError = %q, want \"\" — the pre-connect replay (C4): the replayed fetch's ResourcesLoaded result must clear the permanent error marker left by the pre-connect failure", postReplay.LastFetchError)
	}
	if len(postReplay.Rows) != 2 {
		t.Errorf("len(Rows) = %d, want 2 — the replayed fetch's fresh rows must land on screen", len(postReplay.Rows))
	}
}

// errPreConnectClientsNotInitialized is a minimal error type standing in
// for the real "AWS clients not initialized" error the pre-connect replay names, kept local
// to this file so the test has no dependency on the production error
// value's exact type or message.
type errPreConnectClientsNotInitialized struct{}

func (errPreConnectClientsNotInitialized) Error() string {
	return "AWS clients not initialized"
}
