// Cache-first seeding: when a list screen opens and the session already
// holds rows for that type (RowStore — first-page rows retained by
// availability probes), the list renders those rows immediately:
// ListBody.Loading=false, rows visible, and ListBody.Refreshing=true while
// the fresh fetch runs. On ResourcesLoaded the rows swap in place and
// Refreshing=false. When no rows are known: Loading=true, no Refreshing.
//
// MenuBody.Refreshing is true while a background availability sweep runs
// after a cache-seeded startup, and false once the sweep completes.
package unit_test

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/cache"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
)

// A9S_CONFIG_FOLDER is redirected to t.TempDir() so the disk-store fallback
// HandleNavigate consults reads an isolated directory instead of
// ~/.a9s/cache, where a leftover ec2.yaml would leak rows into a "nothing
// seeded" precondition.
func newSeededTestController(t *testing.T) (*runtime.Core, *app.Controller) {
	t.Helper()
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	s := session.New()
	s.Profile = "demo"
	s.Region = "us-east-1"
	core := runtime.New(s, nil)
	c := newBlessedController(t, core)
	t.Cleanup(c.Close)
	return core, c
}

func TestListOpen_SeedsFromProbeResources_EC2(t *testing.T) {
	core, c := newSeededTestController(t)

	seeded := []resource.Resource{
		{ID: "i-0aaaa1111bbbb2222", Name: "web-1", Type: "ec2", Fields: map[string]string{"state": "running"}},
		{ID: "i-0ccccc3333dddd444", Name: "web-2", Type: "ec2", Fields: map[string]string{"state": "stopped"}},
	}
	core.Session().RowStore.Observe("ec2", seeded, nil, session.OriginProbe, false)

	_, _ = c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	snap := c.Snapshot()

	if snap.Body.Kind != app.BodyKindList {
		t.Fatalf("Body.Kind = %q, want %q", snap.Body.Kind, app.BodyKindList)
	}
	lb := snap.Body.List
	if lb == nil {
		t.Fatal("Body.List is nil after opening ec2 list with seeded RowStore rows")
	}
	if lb.Loading {
		t.Error("Loading = true, want false — seeded rows must render immediately, not show a loading spinner")
	}
	if !lb.Refreshing {
		t.Error("Refreshing = false, want true — a background fetch is still expected to confirm/replace seeded rows")
	}
	if len(lb.Rows) != 2 {
		t.Fatalf("len(Rows) = %d, want 2 — seeded RowStore rows must be visible immediately", len(lb.Rows))
	}
	gotIDs := map[string]bool{lb.Rows[0].ResourceID: true, lb.Rows[1].ResourceID: true}
	for _, want := range []string{"i-0aaaa1111bbbb2222", "i-0ccccc3333dddd444"} {
		if !gotIDs[want] {
			t.Errorf("seeded row ID %q missing from rendered rows: %v", want, gotIDs)
		}
	}
}

// s3 resolves Key-based columns and ec2 Path-based ones; seeding works for
// both column-resolution styles.
func TestListOpen_SeedsFromProbeResources_S3(t *testing.T) {
	core, c := newSeededTestController(t)

	seeded := []resource.Resource{
		{ID: "my-bucket-one", Name: "my-bucket-one", Type: "s3", Fields: map[string]string{"region": "us-east-1"}},
	}
	core.Session().RowStore.Observe("s3", seeded, nil, session.OriginProbe, false)

	_, _ = c.Apply(app.Action{Kind: app.ActionCommand, Arg: "s3"})
	snap := c.Snapshot()

	lb := snap.Body.List
	if lb == nil {
		t.Fatal("Body.List is nil after opening s3 list with seeded RowStore rows")
	}
	if lb.Loading {
		t.Error("Loading = true, want false for seeded s3 list")
	}
	if !lb.Refreshing {
		t.Error("Refreshing = false, want true for seeded s3 list")
	}
	if len(lb.Rows) != 1 || lb.Rows[0].ResourceID != "my-bucket-one" {
		t.Fatalf("Rows = %+v, want single row with ResourceID=my-bucket-one", lb.Rows)
	}
}

func TestListOpen_NoProbeResourcesNoDiskStore_KeepsTodaysLoadingBehavior(t *testing.T) {
	_, c := newSeededTestController(t)

	_, _ = c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	snap := c.Snapshot()

	lb := snap.Body.List
	if lb == nil {
		t.Fatal("Body.List is nil after opening ec2 list with no seeded rows")
	}
	if !lb.Loading {
		t.Error("Loading = false, want true — with no RowStore rows and no disk-store data known, today's Loading=true behavior must be preserved")
	}
	if lb.Refreshing {
		t.Error("Refreshing = true, want false — Refreshing must not fire when there was nothing to seed from")
	}
	if len(lb.Rows) != 0 {
		t.Errorf("len(Rows) = %d, want 0 with no seeded data", len(lb.Rows))
	}
}

func TestListOpen_NoProbeResourcesButDiskStoreHasRows_SeedsFromDiskStore(t *testing.T) {
	core, c := newSeededTestController(t)

	store := core.EnsureCacheStore()
	if store == nil {
		t.Fatal("core.EnsureCacheStore() = nil — test fixture requires a live disk store to seed rows into")
	}
	store.Put("ec2", cache.TypeFile{
		HasResources: true,
		Count:        1,
		Exact:        true,
		Rows: []cache.Row{
			{ID: "i-0diskstore0001", Name: "disk-seeded-1", Fields: map[string]string{"state": "running"}},
		},
	})
	if err := store.SaveType("ec2"); err != nil {
		t.Fatalf("seed fixture SaveType(ec2): %v", err)
	}

	_, _ = c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	snap := c.Snapshot()

	lb := snap.Body.List
	if lb == nil {
		t.Fatal("Body.List is nil after opening ec2 list with a disk-store-only seed")
	}
	if lb.Loading {
		t.Error("Loading = true, want false — a populated on-disk per-type cache must seed the list immediately even with no RowStore rows known (the disk-store fallback)")
	}
	if !lb.Refreshing {
		t.Error("Refreshing = false, want true — a fresh fetch must still run to confirm/replace the disk-seeded rows")
	}
	if len(lb.Rows) != 1 || lb.Rows[0].ResourceID != "i-0diskstore0001" {
		t.Fatalf("Rows = %+v, want single row with ResourceID=i-0diskstore0001", lb.Rows)
	}
}

func TestListOpen_SeedsFromResourceCache_PreviousVisit(t *testing.T) {
	core, c := newSeededTestController(t)

	priorVisitRows := []resource.Resource{
		{ID: "i-0prevvisit0001", Name: "cached-1", Type: "ec2", Fields: map[string]string{"state": "running"}},
	}
	core.Session().RowStore.Observe("ec2", priorVisitRows, &resource.PaginationMeta{IsTruncated: false}, session.OriginFetch, false)

	_, _ = c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	snap := c.Snapshot()

	lb := snap.Body.List
	if lb == nil {
		t.Fatal("Body.List is nil after opening ec2 list with a prior-visit ResourceCache entry")
	}
	if lb.Loading {
		t.Error("Loading = true, want false — a previous visit's ResourceCache must seed the list immediately")
	}
	if !lb.Refreshing {
		t.Error("Refreshing = false, want true — a fresh fetch must still run to confirm/replace cached rows")
	}
	if len(lb.Rows) != 1 || lb.Rows[0].ResourceID != "i-0prevvisit0001" {
		t.Fatalf("Rows = %+v, want single row with ResourceID=i-0prevvisit0001", lb.Rows)
	}
}

func TestListOpen_ResourcesLoaded_ClearsRefreshingAndSwapsRows(t *testing.T) {
	core, c := newSeededTestController(t)

	seeded := []resource.Resource{
		{ID: "i-0seeded0001", Name: "stale-seed", Type: "ec2", Fields: map[string]string{"state": "pending"}},
	}
	core.Session().RowStore.Observe("ec2", seeded, nil, session.OriginProbe, false)

	_, _ = c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	if lb := c.Snapshot().Body.List; lb == nil || !lb.Refreshing {
		t.Fatal("precondition failed: expected Refreshing=true immediately after seeded list-open")
	}

	fresh := []resource.Resource{
		{ID: "i-0fresh00001", Name: "fresh-row", Type: "ec2", Fields: map[string]string{"state": "running"}},
		{ID: "i-0fresh00002", Name: "fresh-row-2", Type: "ec2", Fields: map[string]string{"state": "running"}},
	}
	vs, _ := handlePage(c, messages.ResourcesLoaded{
		ResourceType: "ec2",
		Resources:    fresh,
		Pagination:   &resource.PaginationMeta{IsTruncated: false},
		Gen:          0, Provenance: // AcceptZeroGen=true — always passes the staleness guard
		messages.FetchProvenanceCanonicalList,
	})

	lb := vs.Body.List
	if lb == nil {
		t.Fatal("Body.List is nil after ResourcesLoaded")
	}
	if lb.Refreshing {
		t.Error("Refreshing = true, want false — ResourcesLoaded must clear Refreshing once the fresh fetch lands")
	}
	if lb.Loading {
		t.Error("Loading = true, want false after ResourcesLoaded")
	}
	if len(lb.Rows) != 2 {
		t.Fatalf("len(Rows) = %d, want 2 — rows must swap to the fresh fetch result, not stay on the seeded page", len(lb.Rows))
	}
	for _, r := range lb.Rows {
		if r.ResourceID == "i-0seeded0001" {
			t.Errorf("stale seeded row %q still present after ResourcesLoaded swap", r.ResourceID)
		}
	}
}

// The sweep-start progress intent is Refreshing's only source; a bare
// RowStore seed is not a sweep.
func TestMenu_Refreshing_TrueDuringBackgroundSweep_FalseOnComplete(t *testing.T) {
	core, c := newSeededTestController(t)

	// Cold start with cache-seeded counts already applied (mirrors a disk
	// cache load having already populated menu availability before any live
	// probe has run).
	c.ApplyIntents([]runtime.UIIntent{
		runtime.PatchMenuAvailability{ResourceType: "ec2", Count: 3, Truncated: false},
	})

	// The sweep starts: the runtime sets its own queue/counters and emits the
	// {Checked: 0, Total: N} progress intent (handlers_availability.go), which
	// is the whole of "a sweep is in flight". A one-type sweep keeps the
	// completion below to a single probe result.
	sess := core.Session()
	sess.AvailQueue = nil
	sess.AvailChecked = 0
	sess.AvailTotal = 1
	c.ApplyIntents([]runtime.UIIntent{
		runtime.PatchMenuCheckProgress{Checked: 0, Total: 1},
	})

	snapDuring := c.Snapshot()
	if snapDuring.Body.Menu == nil {
		t.Fatal("Snapshot().Body.Menu is nil at root menu screen")
	}
	if !snapDuring.Body.Menu.Refreshing {
		t.Error("MenuBody.Refreshing = false, want true — a background availability sweep is in flight after cache-seeded startup")
	}

	// Gen is the live availability generation: the sweep is acknowledged only
	// for a result of the current generation.
	vs, _ := c.Handle(messages.AvailabilityChecked{
		ResourceType: "ec2",
		HasResources: true,
		Count:        3,
		Truncated:    false,
		Gen:          core.AvailabilityGen(),
	})
	if vs.Body.Menu == nil {
		t.Fatal("Handle(AvailabilityChecked) returned nil Body.Menu")
	}
	if vs.Body.Menu.Refreshing {
		t.Error("MenuBody.Refreshing = true, want false — sweep must clear Refreshing once its results land")
	}
}

func TestMenu_Refreshing_FalseWithNoSweepInFlight(t *testing.T) {
	_, c := newSeededTestController(t)

	snap := c.Snapshot()
	if snap.Body.Menu == nil {
		t.Fatal("Snapshot().Body.Menu is nil")
	}
	if snap.Body.Menu.Refreshing {
		t.Error("MenuBody.Refreshing = true, want false — no availability sweep has ever started on a fresh controller")
	}
}

func TestListOpen_Refreshing_AllResourceTypes_Generic(t *testing.T) {
	for _, td := range resource.AllResourceTypes() {
		shortName := td.ShortName
		t.Run(shortName, func(t *testing.T) {
			core, c := newSeededTestController(t)
			core.Session().RowStore.Observe(shortName, []resource.Resource{
				{ID: "generic-seed-1", Name: "generic-seed-1", Type: shortName, Fields: map[string]string{}},
			}, nil, session.OriginProbe, false)

			_, _ = c.Apply(app.Action{Kind: app.ActionCommand, Arg: shortName})
			snap := c.Snapshot()

			if snap.Body.Kind != app.BodyKindList {
				t.Fatalf("Body.Kind = %q, want %q for type %q", snap.Body.Kind, app.BodyKindList, shortName)
			}
			lb := snap.Body.List
			if lb == nil {
				t.Fatalf("Body.List is nil for type %q", shortName)
			}
			if lb.Loading {
				t.Errorf("type %q: Loading = true, want false with seeded RowStore rows", shortName)
			}
			if !lb.Refreshing {
				t.Errorf("type %q: Refreshing = false, want true with seeded RowStore rows", shortName)
			}
			if len(lb.Rows) != 1 {
				t.Errorf("type %q: len(Rows) = %d, want 1", shortName, len(lb.Rows))
			}
		})
	}
}

var _ domain.Gen = domain.Gen(0)
