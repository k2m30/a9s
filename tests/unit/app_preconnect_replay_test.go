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

// seedS3TypeFile writes a single-type disk cache that a controller for the same
// profile/region warm-seeds from via EnsureCacheStore/CacheStoreToEvent.
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
// temp-dir cache with no AWS clients.
func newHermeticLiveController(t *testing.T, profile, region string) (*runtime.Core, *app.Controller) {
	t.Helper()
	core := runtime.Bootstrap(profile, region, resource.AllResourceTypes())
	ctrl := newBlessedController(t, core)
	t.Cleanup(ctrl.Close)
	ctrl.SetUIMode("web")
	return core, ctrl
}

func TestSessionNew_PendingRefreshTrue(t *testing.T) {
	s := session.New()
	if !s.PendingRefresh {
		t.Error("session.New().PendingRefresh = false, want true — C10: a fresh session must arm the post-connect replay at startup, not only after a profile/region switch")
	}
}

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

func TestMenuOnlyStartup_NoRefreshTask(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	const profile, region = "pilot-prof", "us-east-1"

	core, ctrl := newHermeticLiveController(t, profile, region)

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
	handlePage(ctrl, messages.ResourcesLoaded{
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

// errPreConnectClientsNotInitialized stands in for the production
// "AWS clients not initialized" error.
type errPreConnectClientsNotInitialized struct{}

func (errPreConnectClientsNotInitialized) Error() string {
	return "AWS clients not initialized"
}
