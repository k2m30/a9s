// app_cache_first_disk_rows_test.go — RED tests for the CACHE-FIRST LIST UX
// epic, Contract B (on-disk row cache + generic field materialization).
//
// Contract B: the on-disk cache (internal/cache) additionally persists, per
// type, the last first-page rows (ID, Name, Fields map — NO RawStruct) and
// the last enrichment findings map. On a cold start with a valid cache file,
// opening a list before probes complete seeds rows+findings from disk with
// Refreshing=true. Fields must be render-sufficient: a generic materialization
// step runs on every fetch result (controller/runtime layer, before
// caching/rendering) that, for each view-config column with a Path and empty
// Key, extracts the scalar via fieldpath and writes it into Fields under the
// column key — so cached rows render identically without RawStruct.
//
// AMBIGUITY RESOLUTIONS (stated, not deferred):
//   - New disk-cache fields: pinned as cache.Entry.Rows ([]cache.CachedRow
//     with ID/Name/Fields) and cache.Entry.Findings (map[string]domain.Finding)
//     — the natural extension of the existing per-type cache.Entry struct
//     (internal/cache/cache.go), which already holds count/truncated/issues
//     per type. If the coder prefers different field/type names the RENAME
//     is fine; the pinned behavior (round-trips through yaml.Marshal/
//     Unmarshal via cache.Save/cache.Load, and seeds a list on cold start) is
//     what must hold.
//   - "Generic materialization step" is pinned as a new exported function
//     app.MaterializeListFields(r resource.Resource, columns []app.ColumnDef)
//     resource.Resource — placed in internal/app (same package as
//     extractListCells/resolveListColumnsForBuild in list_columns.go, which
//     already do per-column Path/Key resolution) since Contract B explicitly
//     scopes this to "controller/runtime layer, before caching/rendering",
//     and internal/app.applyResourcesLoaded is that seam today. If the coder
//     places it in internal/runtime instead, only the call site in this test
//     needs updating — the round-trip behavior pinned by
//     TestMaterializeListFields_* is what matters.
//   - Pinned with "ec2" because its "State" column is Path-based with an
//     empty Key (Path: "State.Name", Key: "") in
//     internal/config/defaults_compute.go — exactly the column shape the
//     contract calls out.
package unit_test

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/app"
	"github.com/k2m30/a9s/v3/internal/cache"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime"
	"github.com/k2m30/a9s/v3/internal/session"
)

// -----------------------------------------------------------------------
// Generic field materialization (the render-sufficiency half of Contract B)
// -----------------------------------------------------------------------

// ec2InstanceStateFake mirrors the shape fieldpath.ExtractScalar needs for
// the EC2 "State" list column (Path: "State.Name"). Deliberately NOT the
// real AWS SDK ec2.Instance type — fieldpath matches by field name via
// reflection, so a minimal shape exercises the same code path without an
// SDK dependency in this test file.
type ec2InstanceStateFake struct {
	InstanceId string
	State      struct {
		Name string
	}
}

// TestMaterializeListFields_EC2State_PathBasedColumn_EmptyKey pins the core
// of Contract B's render-sufficiency requirement: for a column with a Path
// and an empty Key (ec2's "State" column, Path="State.Name" Key=""), the
// materialization step must extract the scalar via fieldpath and write it
// into Fields under the column's key — using the column's resolved cell
// value as the Fields key, mirroring how extractListCells's key-based first
// lookup (r.Fields[col.Key]) would find it on a cache replay with no
// RawStruct.
//
// Ambiguity resolution: "the column key" for a Key="" column is the
// lowercased Title ("state"), since that is what a RawStruct-less second
// visit needs r.Fields to contain for listExtractCellValue's Fields-map
// lookup to succeed (the title-match loop in list_columns.go already
// tries lowercased title against Fields keys as a fallback cascade step —
// materialization should populate the SAME key so the primary Key-based
// lookup succeeds directly, without relying on the fallback cascade).
func TestMaterializeListFields_EC2State_PathBasedColumn_EmptyKey(t *testing.T) {
	raw := ec2InstanceStateFake{InstanceId: "i-0matlzed0001"}
	raw.State.Name = "running"

	r := resource.Resource{
		ID:        "i-0matlzed0001",
		Name:      "materialize-me",
		Type:      "ec2",
		RawStruct: raw,
		Fields:    map[string]string{},
	}
	columns := []app.ColumnDef{
		{Key: "", Title: "State", Path: "State.Name", Width: 12},
	}

	materialized := app.MaterializeListFields(r, columns)

	if materialized.Fields["state"] != "running" {
		t.Errorf(`materialized.Fields["state"] = %q, want "running" — Path-based column with empty Key must be materialized into Fields`, materialized.Fields["state"])
	}
}

// TestMaterializeListFields_CachedRowRendersIdenticallyToLive is the
// end-to-end pin from Contract B: "cached-row rendering must produce the
// same cells as live rendering". It seeds a live row (with RawStruct) as
// one list screen's rows and its materialized-then-stripped-of-RawStruct
// counterpart (simulating what survives a disk-cache round trip per the
// "NO RawStruct" rule) as a second list screen's rows, renders both through
// the same public Controller.ApplyResourcesLoaded → Snapshot().Body.List.Rows
// path (list_body.go's buildListBody / extractListCells), and asserts the
// rendered cells are byte-identical.
func TestMaterializeListFields_CachedRowRendersIdenticallyToLive(t *testing.T) {
	raw := ec2InstanceStateFake{InstanceId: "i-0parity000001"}
	raw.State.Name = "stopped"

	live := resource.Resource{
		ID:        "i-0parity000001",
		Name:      "parity-row",
		Type:      "ec2",
		RawStruct: raw,
		Fields:    map[string]string{},
	}
	// Use the SAME column set list_body.go's buildListBody resolves for "ec2"
	// (the real 9-column catalog from internal/config/defaults_compute.go),
	// not a 2-column subset — otherwise Path-based columns outside the subset
	// (e.g. Instance ID) are never materialized and the parity check is
	// vacuous (both sides render "" for that cell instead of pinning a real
	// value match).
	columns := app.ResolveListColumns("ec2")

	// Render the live row (RawStruct present, Fields not yet materialized).
	_, liveCtrl := newSeededTestController()
	liveCtrl.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	liveCtrl.ApplyResourcesLoaded("ec2", []resource.Resource{live}, nil, false)
	liveRows := liveCtrl.Snapshot().Body.List.Rows
	if len(liveRows) != 1 {
		t.Fatalf("live: len(Rows) = %d, want 1", len(liveRows))
	}

	// Simulate the disk-cache round trip: materialize fields while RawStruct
	// is still present (this MUST happen before caching per the contract),
	// then drop RawStruct exactly as "NO RawStruct" on disk requires.
	cached := app.MaterializeListFields(live, columns)
	cached.RawStruct = nil

	_, cachedCtrl := newSeededTestController()
	cachedCtrl.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	cachedCtrl.ApplyResourcesLoaded("ec2", []resource.Resource{cached}, nil, false)
	cachedRows := cachedCtrl.Snapshot().Body.List.Rows
	if len(cachedRows) != 1 {
		t.Fatalf("cached: len(Rows) = %d, want 1", len(cachedRows))
	}

	liveCells := liveRows[0].Cells
	cachedCells := cachedRows[0].Cells
	if len(liveCells) != len(cachedCells) {
		t.Fatalf("cell count mismatch: live=%d cached=%d", len(liveCells), len(cachedCells))
	}
	for i := range liveCells {
		if liveCells[i] != cachedCells[i] {
			t.Errorf("cell[%d]: live=%q cached=%q — cached-row rendering must match live rendering byte-for-byte", i, liveCells[i], cachedCells[i])
		}
	}
}

// TestMaterializeListFields_KeyBasedColumn_Untouched verifies the
// materialization step only acts on Path-based columns with an EMPTY Key —
// a column that already has a Key must not be overwritten (it already
// resolves via the Fields-map lookup, so touching it risks clobbering a
// value fieldpath cannot see, e.g. Wave-2 enrichment field overrides).
func TestMaterializeListFields_KeyBasedColumn_Untouched(t *testing.T) {
	r := resource.Resource{
		ID:     "bucket-untouched",
		Name:   "bucket-untouched",
		Type:   "s3",
		Fields: map[string]string{"region": "eu-west-1"},
	}
	columns := []app.ColumnDef{
		{Key: "region", Title: "Region", Path: "", Width: 16},
	}

	materialized := app.MaterializeListFields(r, columns)

	if materialized.Fields["region"] != "eu-west-1" {
		t.Errorf(`Fields["region"] = %q, want unchanged "eu-west-1" — Key-based columns must not be touched by materialization`, materialized.Fields["region"])
	}
}

// -----------------------------------------------------------------------
// Disk persistence of rows + findings (the on-disk half of Contract B)
// -----------------------------------------------------------------------

// TestCacheEntry_RowsRoundTripThroughSaveLoad pins that cache.Entry gains a
// Rows field (ID/Name/Fields, NO RawStruct) that survives a full
// cache.Save → cache.Load round trip via YAML.
func TestCacheEntry_RowsRoundTripThroughSaveLoad(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)

	f := &cache.File{
		Profile: "demo",
		Region:  "us-east-1",
		Resources: map[string]cache.Entry{
			"ec2": {
				HasResources: true,
				Count:        1,
				Rows: []cache.CachedRow{
					{ID: "i-0disk0000001", Name: "disk-row", Fields: map[string]string{"state": "running"}},
				},
			},
		},
	}
	if err := cache.Save(f); err != nil {
		t.Fatalf("cache.Save: %v", err)
	}

	loaded, err := cache.Load("demo", "us-east-1")
	if err != nil {
		t.Fatalf("cache.Load: %v", err)
	}
	if loaded == nil {
		t.Fatal("cache.Load returned nil after Save")
	}
	entry, ok := loaded.Resources["ec2"]
	if !ok {
		t.Fatal(`loaded.Resources["ec2"] missing`)
	}
	if len(entry.Rows) != 1 {
		t.Fatalf("len(entry.Rows) = %d, want 1", len(entry.Rows))
	}
	got := entry.Rows[0]
	if got.ID != "i-0disk0000001" || got.Name != "disk-row" || got.Fields["state"] != "running" {
		t.Errorf("entry.Rows[0] = %+v, want ID=i-0disk0000001 Name=disk-row Fields[state]=running", got)
	}
}

// TestCacheEntry_FindingsRoundTripThroughSaveLoad pins the sibling
// requirement: the last enrichment findings map persists per type alongside
// Rows.
func TestCacheEntry_FindingsRoundTripThroughSaveLoad(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)

	f := &cache.File{
		Profile: "demo",
		Region:  "us-east-1",
		Resources: map[string]cache.Entry{
			"ec2": {
				HasResources: true,
				Count:        1,
				Findings: map[string]domain.Finding{
					"i-0disk0000001": {
						Code:     "ec2-stopped-with-eip",
						Phrase:   "stopped, has EIP",
						Severity: domain.SevWarn,
						Source:   "wave2:ec2",
					},
				},
			},
		},
	}
	if err := cache.Save(f); err != nil {
		t.Fatalf("cache.Save: %v", err)
	}

	loaded, err := cache.Load("demo", "us-east-1")
	if err != nil {
		t.Fatalf("cache.Load: %v", err)
	}
	entry := loaded.Resources["ec2"]
	finding, ok := entry.Findings["i-0disk0000001"]
	if !ok {
		t.Fatal(`entry.Findings["i-0disk0000001"] missing after round trip`)
	}
	if finding.Phrase != "stopped, has EIP" || finding.Severity != domain.SevWarn {
		t.Errorf("finding = %+v, want Phrase=%q Severity=SevWarn", finding, "stopped, has EIP")
	}
}

// -----------------------------------------------------------------------
// Cold-start seeding from disk cache (list opens before probes complete)
// -----------------------------------------------------------------------

// TestListOpen_ColdStart_SeedsFromDiskCache_WithRefreshing pins the
// controller-level outcome of Contract B: on a cold start (empty session,
// no ProbeResources) with a valid disk cache file present, opening a list
// before probes complete must seed rows+findings from disk with
// Refreshing=true — mirroring Contract A's in-session seeding outcome, but
// sourced from disk instead of session state.
//
// Ambiguity resolution: "seeds from disk" is modeled at the controller
// level as the disk cache.Entry.Rows having already been loaded into
// session.ProbeResources by the startup cache-load path (the same seam
// LoadAvailabilityCache/SaveAvailabilityCache in internal/runtime/probes.go
// already uses for counts) — this test drives that outcome directly via
// core.Session().ProbeResources rather than asserting on the startup
// loader's internals, since the loader itself is not in this task's scope
// (probes.go is read-only reference material per the dispatch).
func TestListOpen_ColdStart_SeedsFromDiskCache_WithRefreshing(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)

	diskFile := &cache.File{
		Profile: "demo",
		Region:  "us-east-1",
		Resources: map[string]cache.Entry{
			"ec2": {
				HasResources: true,
				Count:        1,
				Rows: []cache.CachedRow{
					{ID: "i-0coldstart001", Name: "cold-start-row", Fields: map[string]string{"state": "running"}},
				},
			},
		},
	}
	if err := cache.Save(diskFile); err != nil {
		t.Fatalf("cache.Save: %v", err)
	}

	loaded, err := cache.Load("demo", "us-east-1")
	if err != nil {
		t.Fatalf("cache.Load: %v", err)
	}
	if loaded == nil {
		t.Fatal("cache.Load returned nil")
	}

	s := session.New()
	s.Profile = "demo"
	s.Region = "us-east-1"
	core := runtime.New(s, nil)
	c := app.New(core)

	// Simulate the startup cache-load path seeding session state from the
	// loaded disk entry's Rows before any probe has completed.
	rows := make([]resource.Resource, len(loaded.Resources["ec2"].Rows))
	for i, cr := range loaded.Resources["ec2"].Rows {
		rows[i] = resource.Resource{ID: cr.ID, Name: cr.Name, Type: "ec2", Fields: cr.Fields}
	}
	core.Session().ProbeResources = map[string][]resource.Resource{"ec2": rows}

	_, _ = c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	snap := c.Snapshot()

	lb := snap.Body.List
	if lb == nil {
		t.Fatal("Body.List is nil after cold-start disk-cache-seeded list open")
	}
	if lb.Loading {
		t.Error("Loading = true, want false — disk-cache-seeded cold start must render rows immediately")
	}
	if !lb.Refreshing {
		t.Error("Refreshing = false, want true — a live fetch must still run to confirm/replace disk-seeded rows")
	}
	if len(lb.Rows) != 1 || lb.Rows[0].ResourceID != "i-0coldstart001" {
		t.Fatalf("Rows = %+v, want single row ResourceID=i-0coldstart001", lb.Rows)
	}
}
