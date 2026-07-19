// runtime_reconciletypefile_test.go — pins for reconcileTypeFile (internal/
// runtime/probes.go), the single chokepoint every type-file write goes
// through as of task #17 wave 1 (C6a, docs/design/cache-requirements.md):
//
//  1. TestSaveAvailabilityCache_CountsOnly_NeverDropsRows — the counts-only
//     rows-drop shape (D16):
//     a counts-only exact write must never nuke existing Rows to zero, even
//     when its Count differs from len(Rows).
//  2. TestSaveResourceListCache_SubsetRowsWrite_KeepsFullerRows — a
//     rows-carrying write whose IDs are a subset of a deeper existing list
//     must keep the fuller existing Rows.
//  3. TestSaveResourceListCache_DeeperRowsWrite_Wins — a rows-carrying write
//     with strictly more rows than existing always wins.
//  4. TestSaveResourceListCache_NonSubsetSameDepth_RefreshWins — same-depth
//     but non-subset (genuinely different) rows win by recency.
//
// All tests are hermetic: A9S_CONFIG_FOLDER redirected to t.TempDir(), no AWS
// credentials, no network. Fake profile/region/resource IDs only.
package unit_test

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/cache"
	"github.com/k2m30/a9s/v3/core/resource"
)

// newSeededTestController and its underlying "demo"/"us-east-1" profile/
// region pair are defined in app_cache_first_seeding_test.go (same package,
// precedented helper) — reused here for the counts-only rows-drop end-to-end pin so the
// Controller-level ApplyResourcesLoaded save seam (which resolves its own
// cache dir from core.Session().Profile/Region) is exercised exactly as the
// real task-result lane would.
const (
	def20Profile = "demo"
	def20Region  = "us-east-1"
)

func reconcileRowID(prefix string, i int) string {
	const digits = "0123456789"
	n := i + 1
	s := ""
	for n > 0 {
		s = string(digits[n%10]) + s
		n /= 10
	}
	for len(s) < 3 {
		s = "0" + s
	}
	return prefix + "-" + s
}

func reconcileRows(prefix string, n int) []cache.Row {
	rows := make([]cache.Row, n)
	for i := range rows {
		rows[i] = cache.Row{ID: reconcileRowID(prefix, i), Name: reconcileRowID(prefix, i)}
	}
	return rows
}

// TestSaveAvailabilityCache_CountsOnly_NeverDropsRows pins the counts-only rows-drop shape (D16)
// directly against SaveAvailabilityCache (the counts-only write lane): an
// existing TypeFile with 50 rows, followed by a counts-only exact
// observation of count 55 (rowsProvided=false in reconcileTypeFile terms),
// must keep the 50 existing rows verbatim and advance Count to 55 — NOT nuke
// Rows to 0. RED at HEAD 9244f1b4: the counts-only path there drops Rows to
// nil/0 whenever Count disagrees with len(Rows).
func TestSaveAvailabilityCache_CountsOnly_NeverDropsRows(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	const shortName = "countsonly"

	store := cache.LoadDir(saveRegProfile, saveRegRegion)
	store.Put(shortName, cache.TypeFile{
		HasResources: true, Count: 50, Exact: true, Rows: reconcileRows("co", 50),
	})
	if err := store.SaveType(shortName); err != nil {
		t.Fatalf("seed SaveType(%s): %v", shortName, err)
	}

	c := newSaveCacheRegressionCore(t, false)
	err := c.SaveAvailabilityCache(
		map[string]int{shortName: 55},
		map[string]bool{shortName: false}, // untruncated: genuine EXACT observation
		nil, nil, nil,
	)
	if err != nil {
		t.Fatalf("SaveAvailabilityCache: %v", err)
	}

	reloaded := cache.LoadDir(saveRegProfile, saveRegRegion)
	tf, ok := reloaded.Type(shortName)
	if !ok {
		t.Fatal("TypeFile missing after counts-only SaveAvailabilityCache write")
	}
	if tf.Count != 55 {
		t.Errorf("TypeFile.Count = %d, want 55", tf.Count)
	}
	if len(tf.Rows) != 50 {
		t.Fatalf("TypeFile.Rows has %d entries, want 50 — a counts-only write must NEVER drop existing Rows", len(tf.Rows))
	}
	for i, want := range reconcileRows("co", 50) {
		if tf.Rows[i].ID != want.ID {
			t.Errorf("tf.Rows[%d].ID = %q, want %q — existing row identity must survive a counts-only write", i, tf.Rows[i].ID, want.ID)
		}
	}
}

// TestSaveResourceListCache_SubsetRowsWrite_KeepsFullerRows pins reconciler
// rule 1: a rows-carrying write whose IDs are a subset of the existing,
// deeper list (a shallower page of the same list — e.g. a truncated
// first-page refetch over an already-fuller stored list) must keep the
// existing fuller Rows in full; Count/Exact still advance per C5.
func TestSaveResourceListCache_SubsetRowsWrite_KeepsFullerRows(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	const shortName = "subsetwrite"

	store := cache.LoadDir(saveRegProfile, saveRegRegion)
	existingRows := reconcileRows("sr", 55)
	store.Put(shortName, cache.TypeFile{
		HasResources: true, Count: 55, Exact: true, Rows: existingRows,
	})
	if err := store.SaveType(shortName); err != nil {
		t.Fatalf("seed SaveType(%s): %v", shortName, err)
	}

	c := newSaveCacheRegressionCore(t, false)
	// First 50 IDs of the existing 55-row list — a genuine subset.
	subsetRows := make([]cache.Row, 50)
	copy(subsetRows, existingRows[:50])
	err := c.SaveResourceListCache(shortName, subsetRows, 50, false /* truncated page */, 0, false, true)
	if err != nil {
		t.Fatalf("SaveResourceListCache: %v", err)
	}

	reloaded := cache.LoadDir(saveRegProfile, saveRegRegion)
	tf, ok := reloaded.Type(shortName)
	if !ok {
		t.Fatal("TypeFile missing after subset-rows SaveResourceListCache write")
	}
	if len(tf.Rows) != 55 {
		t.Errorf("TypeFile.Rows has %d entries, want 55 — a shallower subset write must never regress the deeper existing rows", len(tf.Rows))
	}
	for i, want := range existingRows {
		if i >= len(tf.Rows) {
			break
		}
		if tf.Rows[i].ID != want.ID {
			t.Errorf("tf.Rows[%d].ID = %q, want %q — existing fuller row identity must survive a subset write", i, tf.Rows[i].ID, want.ID)
		}
	}
	// C5: exactness only ever advances — the already-exact stored Count (55)
	// must not regress even though this write's own page was truncated.
	if tf.Count != 55 {
		t.Errorf("TypeFile.Count = %d, want 55 (C5: an already-exact stored total is not regressed by a truncated subset write)", tf.Count)
	}
	if !tf.Exact {
		t.Error("TypeFile.Exact = false, want true (C5: exactness only ever advances)")
	}
}

// TestSaveResourceListCache_DeeperRowsWrite_Wins pins reconciler rule 3: a
// rows-carrying write with strictly MORE rows than existing always wins —
// deeper knowledge always replaces a shallower stored list.
func TestSaveResourceListCache_DeeperRowsWrite_Wins(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	const shortName = "deeperwrite"

	store := cache.LoadDir(saveRegProfile, saveRegRegion)
	store.Put(shortName, cache.TypeFile{
		HasResources: true, Count: 50, Exact: true, Rows: reconcileRows("dw", 50),
	})
	if err := store.SaveType(shortName); err != nil {
		t.Fatalf("seed SaveType(%s): %v", shortName, err)
	}

	c := newSaveCacheRegressionCore(t, false)
	deeperRows := reconcileRows("dwfull", 55)
	err := c.SaveResourceListCache(shortName, deeperRows, 55, true, 0, false, false)
	if err != nil {
		t.Fatalf("SaveResourceListCache: %v", err)
	}

	reloaded := cache.LoadDir(saveRegProfile, saveRegRegion)
	tf, ok := reloaded.Type(shortName)
	if !ok {
		t.Fatal("TypeFile missing after deeper-rows SaveResourceListCache write")
	}
	if len(tf.Rows) != 55 {
		t.Fatalf("TypeFile.Rows has %d entries, want 55 — a deeper incoming write must win over a shallower existing list", len(tf.Rows))
	}
	for i, want := range deeperRows {
		if tf.Rows[i].ID != want.ID {
			t.Errorf("tf.Rows[%d].ID = %q, want %q — the deeper incoming rows must be persisted verbatim", i, tf.Rows[i].ID, want.ID)
		}
	}
	if tf.Count != 55 {
		t.Errorf("TypeFile.Count = %d, want 55", tf.Count)
	}
}

// TestSaveResourceListCache_NonSubsetSameDepth_RefreshWins pins reconciler
// rule 4: a rows-carrying write at the SAME depth as existing but with
// DIFFERENT content (not a subset — a genuine refresh, e.g. the underlying
// AWS list changed membership between sweeps) must win by recency, replacing
// the stored rows outright.
func TestSaveResourceListCache_NonSubsetSameDepth_RefreshWins(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	const shortName = "refreshwrite"

	store := cache.LoadDir(saveRegProfile, saveRegRegion)
	store.Put(shortName, cache.TypeFile{
		HasResources: true, Count: 50, Exact: true, Rows: reconcileRows("old", 50),
	})
	if err := store.SaveType(shortName); err != nil {
		t.Fatalf("seed SaveType(%s): %v", shortName, err)
	}

	c := newSaveCacheRegressionCore(t, false)
	// Same depth (50), but entirely different IDs — a genuine membership
	// change, not a shallower page of the same list.
	freshRows := reconcileRows("new", 50)
	err := c.SaveResourceListCache(shortName, freshRows, 50, true, 0, false, false)
	if err != nil {
		t.Fatalf("SaveResourceListCache: %v", err)
	}

	reloaded := cache.LoadDir(saveRegProfile, saveRegRegion)
	tf, ok := reloaded.Type(shortName)
	if !ok {
		t.Fatal("TypeFile missing after non-subset-same-depth SaveResourceListCache write")
	}
	if len(tf.Rows) != 50 {
		t.Fatalf("TypeFile.Rows has %d entries, want 50", len(tf.Rows))
	}
	for i, want := range freshRows {
		if tf.Rows[i].ID != want.ID {
			t.Errorf("tf.Rows[%d].ID = %q, want %q — a same-depth non-subset write (genuine refresh) must win by recency, replacing the stale rows", i, tf.Rows[i].ID, want.ID)
		}
	}
	for _, stale := range reconcileRows("old", 50) {
		for _, got := range tf.Rows {
			if got.ID == stale.ID {
				t.Errorf("stale row ID %q from the old membership survived a genuine refresh write — refresh must replace outright, not merge", stale.ID)
			}
		}
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Counts-only rows-drop (D16) end-to-end pin — the live evidence chain across three real save
// call sites sharing one on-disk TypeFile, in order:
//
//  1. A list screen opens and pages to 55 exact rows (ApplyResourcesLoaded
//     page 1 + append page 2) — its own maybeSaveResourceListCache save
//     lands 55 rows on disk.
//  2. A background sweep lane (mirrors saveProbeResourcesToTypeFiles's
//     shape — a rows-carrying SaveResourceListCache call) saves 50 probe
//     rows, a genuine SUBSET of the 55 on disk (rule 1) — the file must
//     keep 55.
//  3. A counts-only menu-sync (SaveAvailabilityCache) observes an exact
//     count of 55 — the file must STILL have 55 rows AND count 55.
//
// This exact 3-stage sequence (with the counts-only stage's count already
// matching len(existing.Rows)) passes at HEAD 9244f1b4 too — the pre-
// reconciler count-comparison guards happen to produce the same outcome when
// the observed counts agree. It is pinned anyway as an end-to-end regression
// guard for the reconciler chokepoint (reconcileTypeFile) across all three
// real call sites in sequence, not as a standalone RED-at-HEAD repro; the
// standalone counts-only MISMATCH shape that reproduces the rows-drop bug at
// HEAD (Count disagreeing with len(existing.Rows)) is pinned separately by
// TestSaveAvailabilityCache_CountsOnly_NeverDropsRows and
// TestSaveAvailabilityCache_ExactShrink_CountAdvancesRowsUntouched, both RED
// at HEAD 9244f1b4 (Rows nuked to 0).
// ────────────────────────────────────────────────────────────────────────────

func TestDEF20_ListPageSweepMenuSync_RowsSurviveAllThreeSaveLanes(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	const shortName = "ec2"

	core, ctrl := newSeededTestController(t)
	_, _ = ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: shortName})

	page1 := make([]resource.Resource, 50)
	for i := range page1 {
		page1[i] = resource.Resource{ID: reconcileRowID("d20", i), Name: reconcileRowID("d20", i), Type: shortName}
	}
	ctrl.ApplyResourcesLoaded(shortName, page1, &resource.PaginationMeta{IsTruncated: true, NextToken: "next"}, false)

	page2 := make([]resource.Resource, 5)
	for i := range page2 {
		page2[i] = resource.Resource{ID: reconcileRowID("d20", 50+i), Name: reconcileRowID("d20", 50+i), Type: shortName}
	}
	ctrl.ApplyResourcesLoaded(shortName, page2, &resource.PaginationMeta{IsTruncated: false}, true)

	store := cache.LoadDir(def20Profile, def20Region)
	tf, ok := store.Type(shortName)
	if !ok {
		t.Fatal("stage 1: TypeFile missing after paged list-open save")
	}
	if len(tf.Rows) != 55 {
		t.Fatalf("stage 1: TypeFile.Rows has %d entries, want 55 after page1(50)+page2(5) append save", len(tf.Rows))
	}
	if tf.Count != 55 {
		t.Fatalf("stage 1: TypeFile.Count = %d, want 55", tf.Count)
	}

	// Stage 2 — sweep lane: a rows-carrying SaveResourceListCache call with a
	// genuine 50-row SUBSET of the 55 IDs already on disk (mirrors
	// saveProbeResourcesToTypeFiles observing fewer retained probe rows than
	// the list screen's own fuller page-appended save).
	sweepRows := make([]cache.Row, 50)
	for i := range sweepRows {
		sweepRows[i] = cache.Row{ID: reconcileRowID("d20", i), Name: reconcileRowID("d20", i)}
	}
	if err := core.SaveResourceListCache(shortName, sweepRows, 50, false, 0, false, true); err != nil {
		t.Fatalf("stage 2: SaveResourceListCache: %v", err)
	}

	store = cache.LoadDir(def20Profile, def20Region)
	tf, ok = store.Type(shortName)
	if !ok {
		t.Fatal("stage 2: TypeFile missing after sweep-lane subset save")
	}
	if len(tf.Rows) != 55 {
		t.Fatalf("stage 2: TypeFile.Rows has %d entries, want 55 — the sweep lane's shallower subset save must not regress the list screen's fuller 55-row save", len(tf.Rows))
	}

	// Stage 3 — counts-only menu-sync: SaveAvailabilityCache observes an
	// exact count of 55 (agreeing with what's already on disk, but via the
	// counts-only lane) — the file must still have 55 rows AND count 55.
	if err := core.SaveAvailabilityCache(
		map[string]int{shortName: 55},
		map[string]bool{shortName: false},
		nil, nil, nil,
	); err != nil {
		t.Fatalf("stage 3: SaveAvailabilityCache: %v", err)
	}

	store = cache.LoadDir(def20Profile, def20Region)
	tf, ok = store.Type(shortName)
	if !ok {
		t.Fatal("stage 3: TypeFile missing after counts-only menu-sync save")
	}
	if len(tf.Rows) != 55 {
		t.Errorf("stage 3: TypeFile.Rows has %d entries, want 55 STILL — a counts-only write must never touch existing Rows", len(tf.Rows))
	}
	if tf.Count != 55 {
		t.Errorf("stage 3: TypeFile.Count = %d, want 55", tf.Count)
	}
}
