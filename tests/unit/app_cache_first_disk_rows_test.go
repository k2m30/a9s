// The on-disk cache (core/cache) persists, per type, the last first-page rows
// (ID, Name, Fields map — no RawStruct) and the last enrichment findings per
// row (cache.Row.Findings). On a cold start with a valid cache file, opening a
// list before probes complete seeds rows+findings from disk with
// Refreshing=true. app.MaterializeListFields runs on every fetch result
// (core/app.applyResourcesLoaded) and, for each view-config column with a Path
// and an empty Key, writes the fieldpath scalar into Fields under the column
// key, so cached rows render identically without RawStruct.
//
// ec2's "State" column is Path-based with an empty Key (Path: "State.Name",
// Key: "") in core/config/defaults_compute.go.
package unit_test

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/cache"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/session"
)

// ec2InstanceStateFake is the shape fieldpath.ExtractScalar needs for the EC2
// "State" list column (Path: "State.Name"). fieldpath matches by field name
// via reflection, so the SDK type is not needed.
type ec2InstanceStateFake struct {
	InstanceId string
	State      struct {
		Name string
	}
}

// For a Key="" column the Fields key is the lowercased Title ("state"): the
// key a RawStruct-less cache replay looks up in r.Fields.
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
	_, liveCtrl := newSeededTestController(t)

	// The full catalog column set for "ec2": a subset would leave Path-based
	// columns outside it unmaterialized, and both sides would render "" for them.
	columns := liveCtrl.ResolveColumnsForType("ec2")

	liveCtrl.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	liveCtrl.ApplyResourcesLoaded("ec2", []resource.Resource{live}, nil, false)
	liveRows := liveCtrl.Snapshot().Body.List.Rows
	if len(liveRows) != 1 {
		t.Fatalf("live: len(Rows) = %d, want 1", len(liveRows))
	}

	// A disk-cache round trip: fields are materialized while RawStruct is
	// present, then RawStruct is dropped, as on disk.
	cached := app.MaterializeListFields(live, columns)
	cached.RawStruct = nil

	_, cachedCtrl := newSeededTestController(t)
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

// A column that already has a Key resolves via the Fields map; materialization
// leaves it alone so it cannot clobber a value fieldpath cannot see, e.g. a
// Wave-2 enrichment override.
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

func TestCacheTypeFile_RowsRoundTripThroughSaveLoad(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)

	store := cache.LoadDirForTest("demo", "us-east-1")
	store.Put("ec2", cache.TypeFile{
		HasResources: true,
		Count:        1,
		Rows: []cache.Row{
			{ID: "i-0disk0000001", Name: "disk-row", Fields: map[string]string{"state": "running"}},
		},
	})
	if err := store.SaveType("ec2"); err != nil {
		t.Fatalf("SaveType: %v", err)
	}

	reloaded := cache.LoadDirForTest("demo", "us-east-1")
	if reloaded == nil {
		t.Fatal("LoadDir returned nil after SaveType")
	}
	tf, ok := reloaded.Type("ec2")
	if !ok {
		t.Fatal(`reloaded.Type("ec2") missing`)
	}
	if len(tf.Rows) != 1 {
		t.Fatalf("len(tf.Rows) = %d, want 1", len(tf.Rows))
	}
	got := tf.Rows[0]
	if got.ID != "i-0disk0000001" || got.Name != "disk-row" || got.Fields["state"] != "running" {
		t.Errorf("tf.Rows[0] = %+v, want ID=i-0disk0000001 Name=disk-row Fields[state]=running", got)
	}
}

func TestCacheTypeFile_RowFindingsRoundTripThroughSaveLoad(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)

	store := cache.LoadDirForTest("demo", "us-east-1")
	store.Put("ec2", cache.TypeFile{
		HasResources: true,
		Count:        1,
		Rows: []cache.Row{
			{
				ID:   "i-0disk0000001",
				Name: "disk-row",
				Findings: []domain.Finding{
					{Code: "ec2-stopped-with-eip", Phrase: "stopped, has EIP", Severity: domain.SevWarn, Source: "wave2:ec2"},
				},
			},
		},
	})
	if err := store.SaveType("ec2"); err != nil {
		t.Fatalf("SaveType: %v", err)
	}

	reloaded := cache.LoadDirForTest("demo", "us-east-1")
	tf, ok := reloaded.Type("ec2")
	if !ok {
		t.Fatal(`reloaded.Type("ec2") missing`)
	}
	if len(tf.Rows) != 1 {
		t.Fatalf("len(tf.Rows) = %d, want 1", len(tf.Rows))
	}
	findings := tf.Rows[0].Findings
	if len(findings) != 1 {
		t.Fatalf("tf.Rows[0].Findings has %d entries, want 1", len(findings))
	}
	if findings[0].Phrase != "stopped, has EIP" || findings[0].Severity != domain.SevWarn {
		t.Errorf("finding = %+v, want Phrase=%q Severity=SevWarn", findings[0], "stopped, has EIP")
	}
}

func TestListOpen_ColdStart_SeedsFromDiskCache_WithRefreshing(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)

	store := cache.LoadDirForTest("demo", "us-east-1")
	store.Put("ec2", cache.TypeFile{
		HasResources: true,
		Count:        1,
		Rows: []cache.Row{
			{ID: "i-0coldstart001", Name: "cold-start-row", Fields: map[string]string{"state": "running"}},
		},
	})
	if err := store.SaveType("ec2"); err != nil {
		t.Fatalf("SaveType: %v", err)
	}

	reloaded := cache.LoadDirForTest("demo", "us-east-1")
	if reloaded == nil {
		t.Fatal("LoadDir returned nil")
	}
	diskTF, ok := reloaded.Type("ec2")
	if !ok {
		t.Fatal(`reloaded.Type("ec2") missing`)
	}

	s := session.New()
	s.Profile = "demo"
	s.Region = "us-east-1"
	core := runtime.New(s, nil)
	c := newBlessedController(t, core)
	t.Cleanup(c.Close)

	// Simulate the startup cache-load path seeding session state from the
	// loaded disk TypeFile's Rows before any probe has completed.
	rows := make([]resource.Resource, len(diskTF.Rows))
	for i, cr := range diskTF.Rows {
		rows[i] = resource.Resource{ID: cr.ID, Name: cr.Name, Type: "ec2", Fields: cr.Fields}
	}
	core.Session().RowStore.Observe("ec2", rows, &resource.PaginationMeta{IsTruncated: false}, session.OriginDisk, false)

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
