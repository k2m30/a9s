// command_navigation_after_seed_test.go — RED regression tests for the
// navigation-race half of D11: `-p <profile> -c <type>` races the one-shot -c
// navigation against the disk-cache ProbeResources seed. Today the
// EmitNavigate task fires directly from handleClientsReadySuccess, before
// handleAvailabilityCacheLoaded has had a chance to seed
// session.ProbeResources from the on-disk per-type cache — so when the
// adapter's navigation cmd wins the race, HandleNavigate sees an empty
// ProbeResources map and pushes a bare Loading list with no title count.
//
// Target behavior (coder, in parallel): the one-shot -c navigation is armed
// at ClientsReady (StackDepth==1 gate) but the actual EmitNavigate task is
// emitted from handleAvailabilityCacheLoaded, AFTER ProbeResources seeding,
// so a HandleNavigate driven by that task always sees a seeded cache entry.
// The demo / NoCache early-return path (handleAvailabilityPrefetched is
// synchronous — there is no seed race to lose) keeps firing EmitNavigate
// directly from handleClientsReadySuccess.
//
// Tests are driven entirely through Core's public API (HandleClientsReady,
// HandleEvent, HandleNavigate, Session()) — no reach into unexported
// runtime/session fields.
package unit

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/cache"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/runtime"
)

// hasEmitNavigateTask reports whether tasks contains at least one
// TaskKindEmitNavigate request, and returns how many it found (to check
// "exactly once").
func hasEmitNavigateTask(tasks []runtime.TaskRequest) (found bool, count int) {
	for _, tk := range tasks {
		if tk.Key.Kind == runtime.TaskKindEmitNavigate {
			count++
		}
	}
	return count > 0, count
}

// emitNavigatePayloadOf returns the EmitNavigatePayload carried by the
// first TaskKindEmitNavigate task in tasks, or (zero, false).
func emitNavigatePayloadOf(tasks []runtime.TaskRequest) (runtime.EmitNavigatePayload, bool) {
	for _, tk := range tasks {
		if tk.Key.Kind == runtime.TaskKindEmitNavigate {
			if p, ok := tk.Payload.(runtime.EmitNavigatePayload); ok {
				return p, true
			}
		}
	}
	return runtime.EmitNavigatePayload{}, false
}

// seedDiskCacheWithS3Rows writes a real on-disk per-type cache file for
// profile/region carrying 2 s3 rows, via the same cache.Store.Put +
// SaveType path production code uses (TestPerTypeSave_* /
// TestAllLoadedPages_* in app_web_live_cold_boot_test.go establish this as
// the standard fixture-writing pattern for the round-2 cache surface).
// Returns the freshly-reloaded *cache.Store for the pair, mirroring what
// EnsureCacheStore/CacheStoreToEvent would see on a real cold start.
func seedDiskCacheWithS3Rows(t *testing.T, profile, region string) *cache.Store {
	t.Helper()
	store := cache.LoadDirForTest(profile, region)
	store.Put("s3", cache.TypeFile{
		HasResources: true,
		Count:        2,
		Rows: []cache.Row{
			{ID: "bucket-cmdnav-1", Name: "cmdnav-bucket-1", Fields: map[string]string{"region": region}},
			{ID: "bucket-cmdnav-2", Name: "cmdnav-bucket-2", Fields: map[string]string{"region": region}},
		},
	})
	if err := store.SaveType("s3"); err != nil {
		t.Fatalf("seed fixture SaveType(s3): %v", err)
	}
	return cache.LoadDirForTest(profile, region)
}

// newLiveCoreForCommandNav builds a live (non-demo, NoCache=false) *runtime.Core
// for profile/region with Command pre-armed, mirroring what tui.New +
// SetCommand does when the process starts with `-p <profile> -c <type>`.
func newLiveCoreForCommandNav(profile, region, command string) *runtime.Core {
	c := runtime.Bootstrap(profile, region, catalog.All())
	c.SetCommand(command)
	return c
}

// TestCommandNavigation_AfterSeed_Deterministic pins the fixed ordering:
// HandleClientsReady (StackDepth==1, Command="s3") must NOT emit
// TaskKindEmitNavigate yet — only once the follow-up AvailabilityCacheLoaded
// event has been processed (and ProbeResources seeded from it) does
// EmitNavigate appear, exactly once.
//
// RED today: handleClientsReadySuccess emits EmitNavigate directly, so the
// first assertion (no EmitNavigate yet after HandleClientsReady) fails.
func TestCommandNavigation_AfterSeed_Deterministic(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	profile, region := "cmdnav-profile", "us-east-1"
	store := seedDiskCacheWithS3Rows(t, profile, region)

	c := newLiveCoreForCommandNav(profile, region, "s3")

	_, clientsReadyTasks := c.HandleClientsReady(runtime.ClientsReadyEvent{
		Clients:    &runtime.ServiceClients{},
		Gen:        c.ConnectGen(),
		StackDepth: 1,
	})
	if hasKind, _ := hasEmitNavigateTask(clientsReadyTasks); hasKind {
		t.Fatal("HandleClientsReady returned TaskKindEmitNavigate immediately — the one-shot -c navigation must be armed, not fired, at ClientsReady time (it must wait for the post-seed AvailabilityCacheLoaded event)")
	}

	event := runtime.CacheStoreToEvent(store)
	_, afterSeedTasks := c.HandleEvent(event)

	found, count := hasEmitNavigateTask(afterSeedTasks)
	if !found {
		t.Fatal("HandleEvent(AvailabilityCacheLoaded) did not emit TaskKindEmitNavigate — the armed -c command navigation must fire once ProbeResources has been seeded from disk")
	}
	if count != 1 {
		t.Errorf("TaskKindEmitNavigate emitted %d times, want exactly 1", count)
	}

	payload, ok := emitNavigatePayloadOf(afterSeedTasks)
	if !ok {
		t.Fatal("TaskKindEmitNavigate task carried no EmitNavigatePayload")
	}
	if payload.Target != runtime.NavigateTargetResourceList {
		t.Errorf("EmitNavigatePayload.Target = %v, want NavigateTargetResourceList", payload.Target)
	}
	if payload.ResourceType != "s3" {
		t.Errorf("EmitNavigatePayload.ResourceType = %q, want %q", payload.ResourceType, "s3")
	}

	rows, seeded := c.ProbeResources("s3")
	if !seeded || len(rows) == 0 {
		t.Fatal("session.ProbeResources[\"s3\"] is empty right after AvailabilityCacheLoaded — the seed this navigation depends on never landed")
	}

	navResult, _ := c.HandleNavigate(runtime.NavigateEvent{
		Target:       runtime.NavigateTargetResourceList,
		ResourceType: payload.ResourceType,
	})
	// HandleNavigate's cache-miss branch (session.ResourceCache has no entry
	// yet — a fresh fetch is still dispatched to confirm/replace the probe
	// data) is the EXPECTED path here, not a promotion to
	// NavigateKindPushResourceListCached: per the handler's own doc comment,
	// ProbeResources/ProbeTruncated are a distinct, lower-confidence knowledge
	// source from session.ResourceCache, so the seed rides
	// NavigateKindPushResourceList's CachedEntry fallback instead. What
	// D11 actually requires is that THAT fallback is already
	// populated with real rows — not empty — by the time this call runs.
	if navResult.Kind != runtime.NavigateKindPushResourceList {
		t.Errorf("HandleNavigate(s3) Kind = %v, want NavigateKindPushResourceList (cache-miss branch with a ProbeResources-seeded CachedEntry fallback)", navResult.Kind)
	}
	if navResult.CachedEntry == nil || len(navResult.CachedEntry.Resources) == 0 {
		t.Error("HandleNavigate(s3) CachedEntry is nil/empty — this is the exact D11 symptom: a bare Loading list with no rows because the seed had not landed when navigation fired")
	}
}

// TestCommandNavigation_NoCacheDir_StillFires verifies the armed -c
// navigation still fires when the profile/region pair has NO on-disk cache
// directory at all (a cold machine, first-ever run for this pair) — the
// AvailabilityCacheLoaded event still carries Expired:true / empty Entries
// in that case, and the armed command navigation must not be silently lost.
func TestCommandNavigation_NoCacheDir_StillFires(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	profile, region := "cmdnav-cold-profile", "eu-west-1"

	c := newLiveCoreForCommandNav(profile, region, "ec2")

	_, clientsReadyTasks := c.HandleClientsReady(runtime.ClientsReadyEvent{
		Clients:    &runtime.ServiceClients{},
		Gen:        c.ConnectGen(),
		StackDepth: 1,
	})
	if hasKind, _ := hasEmitNavigateTask(clientsReadyTasks); hasKind {
		t.Fatal("HandleClientsReady returned TaskKindEmitNavigate immediately on a cold-cache machine — navigation must still be deferred to the post-seed event")
	}

	// No cache.LoadDirForTest/Put/SaveType ever ran for this pair — mirrors a
	// genuinely cold machine. EnsureCacheStore's LoadDir call returns a
	// non-nil-but-empty Store (per cache.LoadDirForTest's "never fails" contract),
	// so build the event exactly as production does for that case.
	store := c.CacheStore()
	event := runtime.CacheStoreToEvent(store)
	if len(event.Entries) != 0 {
		t.Fatalf("fixture assumption broken: expected an empty cache to produce an empty Entries map, got %v", event.Entries)
	}

	_, afterSeedTasks := c.HandleEvent(event)
	found, count := hasEmitNavigateTask(afterSeedTasks)
	if !found {
		t.Fatal("HandleEvent(AvailabilityCacheLoaded) with an empty/cold cache did not emit TaskKindEmitNavigate — the -c flag must never be silently lost on a cold machine (the D11 goal)")
	}
	if count != 1 {
		t.Errorf("TaskKindEmitNavigate emitted %d times on a cold-cache machine, want exactly 1", count)
	}

	payload, ok := emitNavigatePayloadOf(afterSeedTasks)
	if !ok {
		t.Fatal("TaskKindEmitNavigate task carried no EmitNavigatePayload")
	}
	if payload.ResourceType != "ec2" {
		t.Errorf("EmitNavigatePayload.ResourceType = %q, want %q", payload.ResourceType, "ec2")
	}
}

// TestCommandNavigation_DemoLane_StillFires verifies the demo / --no-cache
// lane is unaffected by the seed-ordering fix: handleAvailabilityPrefetched
// is a SYNCHRONOUS prefetch (no disk read, no seed race to lose), so the
// -c navigation for that lane continues to fire directly from
// handleClientsReadySuccess, exactly like the pre-existing
// TestHandleClientsReady_Success_Command_StackDepth1 pin in
// core/runtime/handlers_test.go (mirrored here through Core's public
// API since tests/unit cannot reach unexported runtime internals).
func TestCommandNavigation_DemoLane_StillFires(t *testing.T) {
	c := runtime.Bootstrap(demo.DemoProfile, demo.DemoRegion, catalog.All())
	c.SetNoCache(true)
	c.SetIsDemo(true)
	c.SetCommand("s3")
	fakeClients := demo.NewServiceClients()
	c.SetPreSuppliedClients(fakeClients)

	_, tasks := c.HandleClientsReady(runtime.ClientsReadyEvent{
		Clients:    nil, // nil Clients + PreSuppliedClients set -> demo fallback path
		Gen:        c.ConnectGen(),
		StackDepth: 1,
	})

	found, count := hasEmitNavigateTask(tasks)
	if !found {
		t.Fatal("demo/--no-cache HandleClientsReady did not emit TaskKindEmitNavigate directly — the synchronous prefetch lane must keep firing -c navigation immediately (no seed race to defer for)")
	}
	if count != 1 {
		t.Errorf("TaskKindEmitNavigate emitted %d times on the demo lane, want exactly 1", count)
	}

	payload, ok := emitNavigatePayloadOf(tasks)
	if !ok {
		t.Fatal("TaskKindEmitNavigate task carried no EmitNavigatePayload")
	}
	if payload.ResourceType != "s3" {
		t.Errorf("EmitNavigatePayload.ResourceType = %q, want %q", payload.ResourceType, "s3")
	}
	if c.Command() != "" {
		t.Errorf("session.Command should be cleared after the demo-lane navigation fires, got %q", c.Command())
	}
}

// TestCommandNavigation_Rotate_PreservesArmedCommand verifies that
// session.Rotate() (invoked by HandleProfileSelected/HandleRegionSelected
// on every profile/region switch) does NOT clear CommandArmed/PendingCommand
// — mirroring the pre-existing rationale for why Command itself survives
// Rotate (an initial connect that armed the -c navigation but then failed
// and rolled back must not lose the flag on the retry/switch). Accessed via
// Core.Session() since these are plain exported fields on *session.Session,
// not a case that needs a narrow accessor.
func TestCommandNavigation_Rotate_PreservesArmedCommand(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	profile, region := "cmdnav-rotate-profile", "us-east-1"

	c := newLiveCoreForCommandNav(profile, region, "ec2")
	_, _ = c.HandleClientsReady(runtime.ClientsReadyEvent{
		Clients:    &runtime.ServiceClients{},
		Gen:        c.ConnectGen(),
		StackDepth: 1,
	})

	s := c.Session()
	if !s.CommandArmed || s.PendingCommand != "ec2" {
		t.Fatalf("fixture assumption broken: after live-path HandleClientsReady, want CommandArmed=true PendingCommand=%q, got CommandArmed=%v PendingCommand=%q", "ec2", s.CommandArmed, s.PendingCommand)
	}

	_, _ = c.HandleProfileSelected(runtime.ProfileSelectedEvent{Profile: "other-profile"})

	if !s.CommandArmed {
		t.Error("session.CommandArmed was cleared by Rotate() (via HandleProfileSelected) — it must survive a profile/region switch the same way session.Command does, so an in-flight -c navigation is not silently lost on a mid-connect switch")
	}
	if s.PendingCommand != "ec2" {
		t.Errorf("session.PendingCommand = %q after Rotate(), want %q to survive unchanged", s.PendingCommand, "ec2")
	}
}
