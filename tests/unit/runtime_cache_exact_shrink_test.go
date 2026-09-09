// runtime_cache_exact_shrink_test.go — reconcileTypeFile
// (core/runtime/probes.go) shrinks stored Rows when the incoming
// rows-carrying observation is EXACT and a strict subset of the stored rows
// (an exact 22-row save over a 23-row stored file drops the deleted
// resource's row, so a deleted resource's row does not outlive its deletion
// and the on-disk Count and len(Rows) agree); and a type whose live
// population genuinely went to zero this session persists its emptiness
// (rowStoreResourcesAndTruncated, core/runtime/handlers_availability.go).
//
//  1. TestReconcileTypeFile_ExactSubset_ShrinksStoredRows — an EXACT
//     rows-carrying observation whose row set is a strict subset of the
//     stored rows REPLACES the stored rows wholesale; Count and Rows agree
//     afterward.
//  2. TestReconcileTypeFile_ExactSubset_Wave2CarrySurvivesShrink — the C6b
//     Wave-2 carry (carryWave2ForRows) still applies to the SURVIVING rows
//     of an exact shrink; only the dropped row's data disappears.
//  3. TestReconcileTypeFile_ExactSubset_FirstSeenSurvivesShrink — the
//     per-finding FirstSeen stamp on a surviving row is untouched by an
//     exact shrink that drops a sibling row.
//  4. TestSaveAvailabilityCache_CountsOnlyExactShrink_NeverAppliesRowsCarryingShrinkRule —
//     the exact-shrink rule applies ONLY to the rows-carrying lane
//     (reconcileTypeFile rule 1), never to the counts-only lane (rule 2),
//     which never touches Rows regardless of exactness.
//  5. TestObservedEmptyExact_SweepZeroPopulationPersistsEmptiness — a type
//     observed live this session with ZERO rows and exact pagination
//     (population genuinely went to zero) must persist that emptiness on
//     disk (stored rows emptied, count 0), driven end-to-end through the
//     real sweep-completion save seam (HandleEvent -> ExecuteTask), the
//     same seam TestSnapshotProbeResourcesForSave_FieldsIsolatedFromLaterMutation
//     in runtime_savecache_regressions_test.go exercises.
//
// The rule-1 non-exact (truncated) subset case is pinned by
// runtime_reconciletypefile_test.go
// (TestSaveResourceListCache_SubsetRowsWrite_KeepsFullerRows, exact=false).
//
// A stored exact-zero pair ({Count:0, Exact:true, Rows:[]}, a normal,
// reachable steady state under item 5) must self-heal:
//
//  6. TestReconcileTypeFile_ExactZeroSelfHeal_TruncatedNonZeroObservationHeals —
//     rule 0 (core/runtime/probes.go) must not require existing.Count > 0,
//     or a stored exact-zero pair can never self-heal even when a later
//     TRUNCATED rows-carrying observation proves the population came back,
//     and the pair {Count:0, Exact:true, Rows:N} would stick forever.
//     Rows-carrying lane, via SaveResourceListCache.
//  7. TestSaveAvailabilityCache_ExactZeroSelfHeal_CountsOnlyTruncatedObservationDropsExact —
//     the same rule-0 case from the counts-only lane
//     (SaveAvailabilityCache): Exact drops, Count advances, Rows stays
//     untouched (rule 2).
//
// All tests are hermetic: A9S_CONFIG_FOLDER redirected to t.TempDir(), no AWS
// credentials, no network. Fake profile/region/resource IDs only. Reuses
// newSaveCacheRegressionCore, saveRegProfile, saveRegRegion, and
// reconcileRows/reconcileRowID (same package, defined in
// runtime_savecache_regressions_test.go and runtime_reconciletypefile_test.go
// respectively).
package unit_test

import (
	"context"
	"testing"
	"time"

	"github.com/k2m30/a9s/v3/core/cache"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// TestReconcileTypeFile_ExactSubset_ShrinksStoredRows pins the core
// contract: an EXACT rows-carrying observation whose IDs are a strict
// subset of the stored rows (a genuine deletion between sweeps, not a
// shallower truncated page) must REPLACE the stored rows wholesale, not
// keep the deleted resource's row forever; reconcileTypeFile's rule 1
// dispatch (len(incoming.Rows) < len(existing.Rows) &&
// rowIDsAreSubset(...)) must check incoming.Exact.
func TestReconcileTypeFile_ExactSubset_ShrinksStoredRows(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	const shortName = "exactsubsetshrink"

	store := cache.LoadDirForTest(saveRegProfile, saveRegRegion)
	existingRows := reconcileRows("es", 23)
	store.Put(shortName, cache.TypeFile{
		HasResources: true, Count: 23, Exact: true, Rows: existingRows,
	})
	if err := store.SaveType(shortName); err != nil {
		t.Fatalf("seed SaveType(%s): %v", shortName, err)
	}

	c := newSaveCacheRegressionCore(t, false)
	// First 22 of the existing 23 IDs — a genuine subset, and this
	// observation is itself EXACT: resource es-023 was really deleted.
	survivingRows := make([]cache.Row, 22)
	copy(survivingRows, existingRows[:22])
	if err := c.SaveResourceListCache(c.Pair(), shortName, survivingRows, 22, true /* exact */, 0, false, false); err != nil {
		t.Fatalf("SaveResourceListCache: %v", err)
	}

	reloaded := cache.LoadDirForTest(saveRegProfile, saveRegRegion)
	tf, ok := reloaded.Type(shortName)
	if !ok {
		t.Fatal("TypeFile missing after exact-subset SaveResourceListCache write")
	}
	if len(tf.Rows) != 22 {
		t.Fatalf("TypeFile.Rows has %d entries, want 22 — an EXACT observation must authoritatively shrink stored Rows and drop the deleted resource, not keep it forever", len(tf.Rows))
	}
	if tf.Count != 22 {
		t.Errorf("TypeFile.Count = %d, want 22 — Count and Rows must agree after an exact shrink", tf.Count)
	}
	for _, dropped := range existingRows[22:] {
		for _, got := range tf.Rows {
			if got.ID == dropped.ID {
				t.Errorf("deleted row ID %q survived an exact-subset shrink write", dropped.ID)
			}
		}
	}
}

// TestReconcileTypeFile_ExactSubset_Wave2CarrySurvivesShrink pins that C6b
// Wave-2 carry (carryWave2ForRows) still runs for the surviving rows of an
// exact shrink — a naive "tf.Rows = incoming.Rows wholesale" would silently
// drop a surviving row's Wave-2 Findings/Fields, not just the deleted
// row's. Mirrors TestReconcileTypeFile_Wave2Carry_RefreshDropsWave1KeepsWave2
// (runtime_wave2_carry_test.go) but with a strict-subset (deletion) shape
// instead of a same-depth refresh.
func TestReconcileTypeFile_ExactSubset_Wave2CarrySurvivesShrink(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	const shortName = "exactsubsetwave2"

	store := cache.LoadDirForTest(saveRegProfile, saveRegRegion)
	existingRows := []cache.Row{
		{
			ID:   "s3-bucket-survivor",
			Name: "s3-bucket-survivor",
			Fields: map[string]string{
				"status": "public access block incomplete",
			},
			Findings: []domain.Finding{
				{
					Code:     domain.FindingCode("s3-public-access"),
					Phrase:   "public access block incomplete",
					Severity: domain.SevBroken,
					Source:   "wave2:s3",
				},
			},
		},
		{
			ID:   "s3-bucket-deleted",
			Name: "s3-bucket-deleted",
			Fields: map[string]string{
				"status": "bucket policy denies read",
			},
			Findings: []domain.Finding{
				{
					Code:     domain.FindingCode("s3-fetcher-issue"),
					Phrase:   "bucket policy denies read",
					Severity: domain.SevWarn,
					Source:   "wave2:s3",
				},
			},
		},
	}
	store.Put(shortName, cache.TypeFile{
		HasResources: true, Count: 2, Exact: true, Rows: existingRows,
	})
	if err := store.SaveType(shortName); err != nil {
		t.Fatalf("seed SaveType(%s): %v", shortName, err)
	}

	c := newSaveCacheRegressionCore(t, false)
	// Exact subset: s3-bucket-deleted is genuinely gone. The survivor
	// arrives as a bare Wave-1 row (the shape a fresh sweep-completion save
	// produces) with NO Findings/Fields of its own.
	freshRows := []cache.Row{
		{ID: "s3-bucket-survivor", Name: "s3-bucket-survivor"},
	}
	if err := c.SaveResourceListCache(c.Pair(), shortName, freshRows, 1, true, 0, false, false); err != nil {
		t.Fatalf("SaveResourceListCache: %v", err)
	}

	reloaded := cache.LoadDirForTest(saveRegProfile, saveRegRegion)
	tf, ok := reloaded.Type(shortName)
	if !ok {
		t.Fatal("TypeFile missing after exact-subset Wave-2-carry write")
	}
	if len(tf.Rows) != 1 {
		t.Fatalf("TypeFile.Rows has %d entries, want 1 — the deleted row must not survive an exact shrink", len(tf.Rows))
	}
	row := tf.Rows[0]
	if row.ID != "s3-bucket-survivor" {
		t.Fatalf("surviving row ID = %q, want %q", row.ID, "s3-bucket-survivor")
	}
	if len(row.Findings) != 1 {
		t.Fatalf("survivor Findings = %+v, want 1 carried wave2 finding (C6b) — an exact shrink must still carry forward the surviving row's own Wave-2 data, not just delete the missing row's", row.Findings)
	}
	if got := row.Findings[0].Source; got != "wave2:s3" {
		t.Errorf("survivor Findings[0].Source = %q, want %q", got, "wave2:s3")
	}
	if got := row.Findings[0].Phrase; got != "public access block incomplete" {
		t.Errorf("survivor Findings[0].Phrase = %q, want %q", got, "public access block incomplete")
	}
	if got := row.Fields["status"]; got != "public access block incomplete" {
		t.Errorf(`survivor Fields["status"] = %q, want %q (C6b: enricher Fields carry with the finding, even within an exact shrink)`, got, "public access block incomplete")
	}
}

// TestReconcileTypeFile_ExactSubset_FirstSeenSurvivesShrink pins issue
// #463's FindingFirstSeen interplay with the new exact-shrink rule: a
// surviving row's FirstSeen stamp must be unchanged by the shrink, and the
// dropped row's disappearance must not corrupt it. Mirrors
// TestCacheFirstSeen_PersistsAcrossSaves (cache_first_seen_test.go): two
// real successive saves against the same Core give stampFindingFirstSeen a
// genuine prior generation to diff against.
func TestReconcileTypeFile_ExactSubset_FirstSeenSurvivesShrink(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	const shortName = "exactsubsetfirstseen"

	c := newSaveCacheRegressionCore(t, false)

	finding := domain.Finding{
		Code:     "s3-public-read",
		Phrase:   "publicly readable",
		Severity: domain.SevBroken,
		Source:   "wave2:s3",
	}
	survivorRow := cache.Row{ID: "bucket-survivor", Name: "bucket-survivor", Findings: []domain.Finding{finding}}
	deletedRow := cache.Row{ID: "bucket-deleted", Name: "bucket-deleted", Findings: []domain.Finding{finding}}

	// Save 1: both rows present, exact — stamps FirstSeen for both.
	if err := c.SaveResourceListCache(c.Pair(), shortName, []cache.Row{survivorRow, deletedRow}, 2, true, 0, false, false); err != nil {
		t.Fatalf("save 1: SaveResourceListCache: %v", err)
	}
	store1 := cache.LoadDirForTest(saveRegProfile, saveRegRegion)
	tf1, ok := store1.Type(shortName)
	if !ok || len(tf1.Rows) != 2 {
		t.Fatalf("save 1: store.Type(%s) = (%+v, %v), want exactly 2 rows", shortName, tf1, ok)
	}
	var survivorFirstSeen time.Time
	var haveSurvivorStamp bool
	for _, r := range tf1.Rows {
		if r.ID != "bucket-survivor" {
			continue
		}
		t0, has := r.FindingFirstSeen[finding.Code]
		if !has || t0.IsZero() {
			t.Fatalf("save 1: FindingFirstSeen[%q] missing or zero for survivor", finding.Code)
		}
		survivorFirstSeen = t0
		haveSurvivorStamp = true
	}
	if !haveSurvivorStamp {
		t.Fatal("save 1: survivor row missing from TypeFile.Rows")
	}

	time.Sleep(5 * time.Millisecond)

	// Save 2: the EXACT-shrink observation — bucket-deleted is gone.
	if err := c.SaveResourceListCache(c.Pair(), shortName, []cache.Row{survivorRow}, 1, true, 0, false, false); err != nil {
		t.Fatalf("save 2: SaveResourceListCache: %v", err)
	}

	store2 := cache.LoadDirForTest(saveRegProfile, saveRegRegion)
	tf2, ok := store2.Type(shortName)
	if !ok || len(tf2.Rows) != 1 {
		t.Fatalf("save 2: store.Type(%s) = (%+v, %v), want exactly 1 row — the exact shrink must drop bucket-deleted", shortName, tf2, ok)
	}
	if tf2.Rows[0].ID != "bucket-survivor" {
		t.Fatalf("save 2: surviving row ID = %q, want %q", tf2.Rows[0].ID, "bucket-survivor")
	}
	t2, has := tf2.Rows[0].FindingFirstSeen[finding.Code]
	if !has {
		t.Fatal("save 2: FindingFirstSeen entry dropped for the survivor's still-present finding")
	}
	if !t2.Equal(survivorFirstSeen) {
		t.Errorf("save 2: FindingFirstSeen[%q] = %v, want unchanged from save 1's %v — the survivor's FirstSeen must not be corrupted by the deleted sibling row's disappearance", finding.Code, t2, survivorFirstSeen)
	}
}

// TestSaveAvailabilityCache_CountsOnlyExactShrink_NeverAppliesRowsCarryingShrinkRule
// is a regression safety net, not a new-contract pin: the new exact-shrink
// rule (item 1) must apply ONLY within reconcileTypeFile's rows-carrying
// lane (RowsProvided=true), never to the counts-only lane (RowsProvided=
// false, rule 2), which must keep ignoring Rows entirely regardless of
// whether the new Count is smaller and "exact". Expected GREEN both before
// and after the item-1 fix — this pins the boundary of the fix, mirroring
// TestSaveAvailabilityCache_ExactShrink_CountAdvancesRowsUntouched
// (runtime_savecache_regressions_test.go) but framed explicitly against the
// new rule so an implementation that over-generalizes exact-shrink into
// rule 2 is caught here.
func TestSaveAvailabilityCache_CountsOnlyExactShrink_NeverAppliesRowsCarryingShrinkRule(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	const shortName = "countsonlyexactshrink"

	store := cache.LoadDirForTest(saveRegProfile, saveRegRegion)
	rows3 := reconcileRows("co", 3)
	store.Put(shortName, cache.TypeFile{HasResources: true, Count: 3, Exact: true, Rows: rows3})
	if err := store.SaveType(shortName); err != nil {
		t.Fatalf("seed SaveType(%s): %v", shortName, err)
	}

	c := newSaveCacheRegressionCore(t, false)
	// A counts-only exact observation reporting a SMALLER count (1) — the
	// exact shape that would trigger the new rows-carrying exact-shrink rule
	// if this were a rows-carrying write. It must not: rule 2 never inspects
	// Rows at all, regardless of exactness.
	if err := c.SaveAvailabilityCache(c.Pair(),
		map[string]int{shortName: 1},
		map[string]bool{shortName: false}, // untruncated: genuine EXACT observation
		nil, nil, nil,
	); err != nil {
		t.Fatalf("SaveAvailabilityCache: %v", err)
	}

	reloaded := cache.LoadDirForTest(saveRegProfile, saveRegRegion)
	tf, ok := reloaded.Type(shortName)
	if !ok {
		t.Fatal("TypeFile missing after counts-only exact-shrink observation")
	}
	if tf.Count != 1 {
		t.Errorf("TypeFile.Count = %d, want 1", tf.Count)
	}
	if len(tf.Rows) != 3 {
		t.Errorf("TypeFile.Rows has %d entries, want 3 UNTOUCHED — the new exact-authoritative-shrink rule must apply ONLY to rows-carrying writes, never to the counts-only lane", len(tf.Rows))
	}
}

// TestObservedEmptyExact_SweepZeroPopulationPersistsEmptiness pins the
// second #457 defect: rowStoreResourcesAndTruncated
// (core/runtime/handlers_availability.go) skips any RowStore entry with
// len(Rows)==0, so a type whose live population genuinely went to zero this
// session never reaches the disk save at all — the stale on-disk rows
// survive forever. Driven end-to-end through the real sweep-completion save
// seam: a single AvailabilityChecked with the queue already drained
// (session.New() leaves AvailQueue nil, AvailChecked 0, AvailTotal 0) fires
// the "all checks done" branch, exactly as
// TestSnapshotProbeResourcesForSave_FieldsIsolatedFromLaterMutation
// (runtime_savecache_regressions_test.go) drives it. Err is nil and
// Resources is empty: a genuine live observation that the type's
// population is now zero, with exact (untruncated) pagination — not an
// unobserved/failed-probe type, which must NOT wipe stored rows (that
// safety net is already covered by every rule-2 counts-only pin above: a
// type never mentioned in a SaveAvailabilityCache/SaveResourceListCache
// call is never touched, by construction of those functions only writing
// entries they're actually given).
func TestObservedEmptyExact_SweepZeroPopulationPersistsEmptiness(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	const shortName = "s3"

	store := cache.LoadDirForTest(saveRegProfile, saveRegRegion)
	staleRows := reconcileRows("gone", 3)
	store.Put(shortName, cache.TypeFile{HasResources: true, Count: 3, Exact: true, Rows: staleRows})
	if err := store.SaveType(shortName); err != nil {
		t.Fatalf("seed SaveType(%s): %v", shortName, err)
	}

	c := newSaveCacheRegressionCore(t, false)

	_, tasks := c.HandleEvent(messages.AvailabilityChecked{
		ResourceType: shortName,
		HasResources: false,
		Count:        0,
		Truncated:    false,
		Resources:    nil,
		Gen:          c.AvailabilityGen(),
	})

	var saveTask *runtime.TaskRequest
	for i := range tasks {
		if tasks[i].Key.Kind == runtime.TaskKindSaveCache {
			saveTask = &tasks[i]
			break
		}
	}
	if saveTask == nil {
		t.Fatal("no TaskKindSaveCache task returned — test assumption broken, cannot exercise the sweep-completion save seam")
	}

	ev, err := c.ExecuteTask(context.Background(), *saveTask)
	if err != nil {
		t.Fatalf("ExecuteTask(TaskKindSaveCache): %v", err)
	}
	if flash, ok := ev.(messages.Flash); ok && flash.IsError {
		t.Fatalf("save-cache returned an error flash: %s", flash.Text)
	}

	reloaded := cache.LoadDirForTest(saveRegProfile, saveRegRegion)
	tf, ok := reloaded.Type(shortName)
	if !ok {
		t.Fatal("TypeFile missing after zero-population sweep-completion save")
	}
	if len(tf.Rows) != 0 {
		t.Errorf("TypeFile.Rows has %d entries, want 0 — a live exact zero-population observation this session must empty stored Rows (the type's resources are genuinely gone), not keep the stale rows forever", len(tf.Rows))
	}
	if tf.Count != 0 {
		t.Errorf("TypeFile.Count = %d, want 0", tf.Count)
	}
}

// TestReconcileTypeFile_ExactZeroSelfHeal_TruncatedNonZeroObservationHeals
// pins rule-0 self-heal: a stored exact-zero pair — a normal, reachable
// steady state now that
// TestObservedEmptyExact_SweepZeroPopulationPersistsEmptiness above pins
// observed-empty-exact persistence — heals on a later TRUNCATED
// rows-carrying save whose raw count is nonzero (the type's population came
// back, e.g. a 50-row first page), which is proof the stored {Count:0,
// Exact:true} is stale. With rule 0 (core/runtime/probes.go) blocked by an
// existing.Count > 0 requirement, the caller-side C5 stickiness in
// saveResourceListCache would re-force Exact=true and Count=0 onto the
// incoming observation while the 50 rows win under rules 3/4 (their own
// len is not < existing's 0), producing a permanently poisoned pair
// {Count:0, Exact:true, Rows:50} that no future truncated observation could
// repair.
func TestReconcileTypeFile_ExactZeroSelfHeal_TruncatedNonZeroObservationHeals(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	const shortName = "exactzeroselfheal"

	store := cache.LoadDirForTest(saveRegProfile, saveRegRegion)
	store.Put(shortName, cache.TypeFile{HasResources: false, Count: 0, Exact: true, Rows: []cache.Row{}})
	if err := store.SaveType(shortName); err != nil {
		t.Fatalf("seed SaveType(%s): %v", shortName, err)
	}

	c := newSaveCacheRegressionCore(t, false)
	// The population came back: a truncated (first-page) rows-carrying
	// observation of 50 rows — proof the stored exact-zero total is stale.
	revivedRows := reconcileRows("revived", 50)
	if err := c.SaveResourceListCache(c.Pair(), shortName, revivedRows, 50, false /* truncated */, 0, false, false); err != nil {
		t.Fatalf("SaveResourceListCache: %v", err)
	}

	reloaded := cache.LoadDirForTest(saveRegProfile, saveRegRegion)
	tf, ok := reloaded.Type(shortName)
	if !ok {
		t.Fatal("TypeFile missing after truncated self-heal save")
	}
	if tf.Exact {
		t.Errorf("TypeFile.Exact = true, want false — rule 0 must self-heal a stored exact-zero pair once a truncated observation proves a nonzero population, not let the stale exact-zero claim stick forever")
	}
	if tf.Count != 50 {
		t.Errorf("TypeFile.Count = %d, want 50 (the raw observed count) — a self-healed pair must report the truncated observation's own count, not remain stuck at the stale exact-zero total", tf.Count)
	}
	if len(tf.Rows) != 50 {
		t.Fatalf("TypeFile.Rows has %d entries, want 50", len(tf.Rows))
	}
	for i, want := range revivedRows {
		if tf.Rows[i].ID != want.ID {
			t.Errorf("tf.Rows[%d].ID = %q, want %q", i, tf.Rows[i].ID, want.ID)
		}
	}
	if tf.Exact && tf.Count == 0 && len(tf.Rows) > 0 {
		t.Error("TypeFile is the poisoned pair shape {Exact:true, Count:0} with non-empty Rows — precisely the bug rule 0's guard must prevent")
	}
}

// TestSaveAvailabilityCache_ExactZeroSelfHeal_CountsOnlyTruncatedObservationDropsExact
// pins the same rule-0 gap from the counts-only lane (SaveAvailabilityCache):
// a truncated counts-only observation with a nonzero count must drop the
// stored exact-zero pair's Exact flag. Count already advances correctly
// today in this lane (SaveAvailabilityCache's own stickiness sub-block only
// preserves existing.Count when existing.Count > count, which is never true
// against a stored Count of 0) — Exact is the only field this lane's rule-0
// gap leaves wrong. Rows stays untouched either way: rule 2 never inspects
// Rows regardless of rule 0's outcome.
func TestSaveAvailabilityCache_ExactZeroSelfHeal_CountsOnlyTruncatedObservationDropsExact(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	const shortName = "exactzeroselfhealcounts"

	store := cache.LoadDirForTest(saveRegProfile, saveRegRegion)
	store.Put(shortName, cache.TypeFile{HasResources: false, Count: 0, Exact: true, Rows: []cache.Row{}})
	if err := store.SaveType(shortName); err != nil {
		t.Fatalf("seed SaveType(%s): %v", shortName, err)
	}

	c := newSaveCacheRegressionCore(t, false)
	if err := c.SaveAvailabilityCache(c.Pair(),
		map[string]int{shortName: 50},
		map[string]bool{shortName: true}, // truncated: a first-page-only probe
		nil, nil, nil,
	); err != nil {
		t.Fatalf("SaveAvailabilityCache: %v", err)
	}

	reloaded := cache.LoadDirForTest(saveRegProfile, saveRegRegion)
	tf, ok := reloaded.Type(shortName)
	if !ok {
		t.Fatal("TypeFile missing after counts-only self-heal observation")
	}
	if tf.Exact {
		t.Errorf("TypeFile.Exact = true, want false — a truncated counts-only observation with a nonzero count must self-heal a stored exact-zero pair, same rule 0 gap as the rows-carrying lane")
	}
	if tf.Count != 50 {
		t.Errorf("TypeFile.Count = %d, want 50", tf.Count)
	}
	if len(tf.Rows) != 0 {
		t.Errorf("TypeFile.Rows has %d entries, want 0 UNTOUCHED — the counts-only lane never inspects Rows regardless of rule 0's outcome", len(tf.Rows))
	}
}
