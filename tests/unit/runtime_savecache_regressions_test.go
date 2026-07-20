// runtime_savecache_regressions_test.go — regression pins for the
// Codex+CodeRabbit fix wave on core/runtime (branch feat/cache).
//
// Covers, in order:
//
//  1. Depth-loop zero-progress guard (executor.go KindFetchResources): a
//     fetcher whose follow-up page never advances (0 new resources,
//     IsTruncated:true, NextToken repeats) must not spin forever — the loop
//     must terminate and return what it already has.
//  2. Save-path canonicalization: SaveResourceListCache/saveProbeResourcesToTypeFiles
//     must persist under the CANONICAL short name, not whatever alias the
//     caller happened to use, so CachedListDepth is queryable by either name
//     and the on-disk file is named after the canonical type.
//  3. Exact-shrink consistency: SaveAvailabilityCache's counts-only path
//     (writeAvailability in probes.go) must never leave a persisted TypeFile
//     with len(Rows) != Count when a smaller EXACT observation supersedes a
//     larger stale Rows carry-forward.
//  4. Snapshot Fields isolation: snapshotProbeResourcesForSave's
//     *SaveCachePayload must be immune to later in-place mutation of the
//     ORIGINAL ProbeResources rows' Fields maps.
//  5. rowsFromCacheRows store isolation: seeding session.ProbeResources from
//     a disk-loaded TypeFile's Rows must not alias the Store's own Rows
//     slice/maps — later in-memory mutation of the seeded rows must not
//     write through to the Store.
//  6. tf.Issues double-write ordering: within one TaskKindSaveCache
//     execution, an exact issueCounts observation from
//     availabilityFromResourceCache must survive saveProbeResourcesToTypeFiles's
//     own row-derived recomputation when the swept rows carry no findings.
//
// All tests are hermetic: A9S_CONFIG_FOLDER redirected to t.TempDir(), no AWS
// credentials, no network. Fake profile/region/resource IDs only.
package unit_test

import (
	"context"
	"os"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/k2m30/a9s/v3/core/cache"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// ────────────────────────────────────────────────────────────────────────────
// shared helpers (file-local per this package's existing convention of not
// sharing test helpers across files — mirrors runtime_executor_depth_refetch_test.go)
// ────────────────────────────────────────────────────────────────────────────

const (
	saveRegProfile = "savecache-prof"
	saveRegRegion  = "us-east-1"
)

// newSaveCacheRegressionCore builds a demo-clients Core against a temp-dir
// cache, mirroring newDepthExecutorCore in runtime_executor_depth_refetch_test.go.
func newSaveCacheRegressionCore(t *testing.T, noCache bool) *runtime.Core {
	t.Helper()
	c := runtime.Bootstrap(saveRegProfile, saveRegRegion, catalog.All())
	c.SetNoCache(noCache)
	c.SetIsDemo(true)
	fakeClients := demo.NewServiceClients()
	c.SetPreSuppliedClients(fakeClients)
	c.HandleClientsReady(runtime.ClientsReadyEvent{ //nolint:errcheck // intentional — we only need side-effect
		Clients:    nil,
		Gen:        c.ConnectGen(),
		StackDepth: 1,
	})
	return c
}

func saveRegID(prefix string, i int) string {
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

// ────────────────────────────────────────────────────────────────────────────
// Test 1 — depth-loop zero-progress guard
// ────────────────────────────────────────────────────────────────────────────

// TestExecuteTask_FetchResources_ZeroProgressFollowUp_Terminates pins the
// depth-loop's must-terminate guard: a cached depth of 55 combined with a
// follow-up page that returns ZERO new resources but keeps reporting
// IsTruncated:true and the SAME NextToken must not spin forever. The
// executor must detect the lack of progress and return page 1's 50 rows
// rather than looping until context cancellation or a stack overflow.
func TestExecuteTask_FetchResources_ZeroProgressFollowUp_Terminates(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	const shortName = "zeroprogress"

	callCount := 0
	resource.SetPaginatedForTest(shortName, func(_ context.Context, _ any, token string) (resource.FetchResult, error) {
		callCount++
		switch token {
		case "":
			rows := make([]resource.Resource, 50)
			for i := range rows {
				rows[i] = resource.Resource{ID: saveRegID("zp", i), Name: saveRegID("zp", i), Type: shortName}
			}
			return resource.FetchResult{
				Resources: rows,
				Pagination: &resource.PaginationMeta{
					IsTruncated: true,
					NextToken:   "stuck",
					TotalHint:   -1,
					PageSize:    50,
				},
			}, nil
		case "stuck":
			// Zero new resources, but still claims truncation with the SAME
			// token — the classic no-progress pagination bug.
			return resource.FetchResult{
				Resources: nil,
				Pagination: &resource.PaginationMeta{
					IsTruncated: true,
					NextToken:   "stuck",
					TotalHint:   -1,
					PageSize:    0,
				},
			}, nil
		default:
			return resource.FetchResult{}, nil
		}
	})
	t.Cleanup(func() { resource.CleanupPaginatedForTest(shortName) })

	// Seed a cached depth of 55 so the depth loop's bound
	// (len(Resources) < CachedListDepth) stays true forever if the loop
	// never detects zero progress.
	store := cache.LoadDirForTest(saveRegProfile, saveRegRegion)
	rows := make([]cache.Row, 55)
	for i := range rows {
		rows[i] = cache.Row{ID: saveRegID("zp", i), Name: saveRegID("zp", i)}
	}
	store.Put(shortName, cache.TypeFile{HasResources: true, Count: 55, Exact: true, Rows: rows})
	if err := store.SaveType(shortName); err != nil {
		t.Fatalf("seed SaveType(%s): %v", shortName, err)
	}

	c := newSaveCacheRegressionCore(t, false)

	done := make(chan struct{})
	var ev messages.Event
	var err error
	go func() {
		ev, err = c.ExecuteTask(context.Background(), runtime.TaskRequest{
			Key: runtime.TaskKey{Kind: runtime.KindFetchResources, Scope: shortName},
		})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("ExecuteTask did not return within the timeout — depth loop appears to spin forever on zero-progress pagination")
	}

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := ev.(messages.ResourcesLoaded)
	if !ok {
		t.Fatalf("expected messages.ResourcesLoaded, got %T", ev)
	}
	if len(got.Resources) != 50 {
		t.Errorf("len(Resources) = %d, want 50 (page 1's rows returned once zero-progress is detected, not spun past)", len(got.Resources))
	}
	if callCount > 3 {
		t.Errorf("fetcher called %d times, want <= 3 — depth loop must terminate promptly on zero-progress pagination instead of retrying the stuck token repeatedly", callCount)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Test 2 — save-path canonicalization
// ────────────────────────────────────────────────────────────────────────────

// TestSaveProbeResourcesToTypeFiles_AliasCanonicalizes_OnDiskFileIsCanonical
// pins the save-path half of alias canonicalization: a sweep that retained
// rows under the alias "rds" (as ProbeResources happens to be keyed, e.g.
// from an older/aliased caller) must persist under the CANONICAL short name
// "dbi" — CachedListDepth must report the same depth whether queried by
// "rds" or "dbi", and the on-disk file must be named dbi.yaml, never
// rds.yaml. "rds" is a registered alias of "dbi" (see
// core/aws/catalog_databases.go ShortName:"dbi", Aliases includes "rds").
func TestSaveProbeResourcesToTypeFiles_AliasCanonicalizes_OnDiskFileIsCanonical(t *testing.T) {
	if resource.FindResourceType("rds") == nil {
		t.Skip("no registered type has alias \"rds\" in this build; alias case not testable")
	}
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())

	c := newSaveCacheRegressionCore(t, false)

	rows := make([]resource.Resource, 12)
	for i := range rows {
		rows[i] = resource.Resource{ID: saveRegID("db", i), Name: saveRegID("db", i), Type: "rds"}
	}
	payload := &runtime.SaveCachePayload{
		Resources: map[string][]resource.Resource{"rds": rows},
		Truncated: map[string]bool{"rds": false},
	}
	ev, err := c.ExecuteTask(context.Background(), runtime.TaskRequest{
		Key:     runtime.TaskKey{Kind: runtime.TaskKindSaveCache},
		Payload: payload,
	})
	if err != nil {
		t.Fatalf("ExecuteTask(TaskKindSaveCache): %v", err)
	}
	if flash, ok := ev.(messages.Flash); ok && flash.IsError {
		t.Fatalf("save-cache returned an error flash: %s", flash.Text)
	}

	if got := c.CachedListDepth("rds"); got != 12 {
		t.Errorf(`CachedListDepth("rds") = %d, want 12 after saving under alias "rds"`, got)
	}
	if got := c.CachedListDepth("dbi"); got != 12 {
		t.Errorf(`CachedListDepth("dbi") = %d, want 12 — the canonical name must report the same depth as the alias used to save`, got)
	}

	dir := cache.DirForTest(saveRegProfile, saveRegRegion)
	if _, statErr := os.Stat(dir + "/dbi.yaml"); statErr != nil {
		t.Errorf("on-disk file dbi.yaml does not exist (%v) — save must canonicalize the alias to the registered ShortName before persisting", statErr)
	}
	if _, statErr := os.Stat(dir + "/rds.yaml"); statErr == nil {
		t.Error("on-disk file rds.yaml exists — save must persist under the canonical name \"dbi\", never the raw alias \"rds\"")
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Test 3 — exact-shrink consistency (SaveAvailabilityCache counts-only path)
// ────────────────────────────────────────────────────────────────────────────

// TestSaveAvailabilityCache_ExactShrink_CountAdvancesRowsUntouched pins the
// C6a reconciler contract for the counts-only availability-save path
// (reconcileTypeFile rule 2, called from SaveAvailabilityCache): a TypeFile
// that already carries {Count:50, Exact:true, Rows: 50 rows} followed by a
// NEW exact observation of count 48 must advance Count to 48 while leaving
// Rows COMPLETELY UNTOUCHED — the stale 50 rows are kept in full, producing a
// reconstructable Count/Rows pair (Count is the authoritative total, Rows is
// the last-known page). A counts-only write must never shrink, truncate, or
// drop Rows merely to make len(Rows)==Count — see
// docs/design/cache-requirements.md C6a.
func TestSaveAvailabilityCache_ExactShrink_CountAdvancesRowsUntouched(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	const shortName = "exactshrink"

	store := cache.LoadDirForTest(saveRegProfile, saveRegRegion)
	rows50 := make([]cache.Row, 50)
	for i := range rows50 {
		rows50[i] = cache.Row{ID: saveRegID("es", i), Name: saveRegID("es", i)}
	}
	store.Put(shortName, cache.TypeFile{HasResources: true, Count: 50, Exact: true, Rows: rows50})
	if err := store.SaveType(shortName); err != nil {
		t.Fatalf("seed SaveType(%s): %v", shortName, err)
	}

	c := newSaveCacheRegressionCore(t, false)
	err := c.SaveAvailabilityCache(
		map[string]int{shortName: 48},
		map[string]bool{shortName: false}, // untruncated: a genuine EXACT observation
		nil, nil, nil,
	)
	if err != nil {
		t.Fatalf("SaveAvailabilityCache: %v", err)
	}

	reloaded := cache.LoadDirForTest(saveRegProfile, saveRegRegion)
	tf, ok := reloaded.Type(shortName)
	if !ok {
		t.Fatal("TypeFile missing after SaveAvailabilityCache exact-shrink observation")
	}
	if tf.Count != 48 {
		t.Errorf("TypeFile.Count = %d, want 48 (the new exact observation)", tf.Count)
	}
	if len(tf.Rows) != 50 {
		t.Errorf("TypeFile.Rows has %d entries, want 50 UNTOUCHED — a counts-only write must never touch existing Rows, even when the new exact Count is smaller (C6a: Count/Rows form a reconstructable pair, not a forced-equal pair)", len(tf.Rows))
	}
	for i, want := range rows50 {
		if i >= len(tf.Rows) {
			break
		}
		if tf.Rows[i].ID != want.ID {
			t.Errorf("tf.Rows[%d].ID = %q, want %q — stale row identity must survive a counts-only write verbatim", i, tf.Rows[i].ID, want.ID)
		}
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Test 4 — snapshot Fields isolation
// ────────────────────────────────────────────────────────────────────────────

// TestSnapshotProbeResourcesForSave_FieldsIsolatedFromLaterMutation pins
// Fields isolation on the dispatch-time payload freeze: seeding
// session.ProbeResources with rows carrying Fields, taking the
// TaskKindSaveCache snapshot via the sweep-completion seam
// (handleAvailabilityChecked's queue-drained branch), THEN mutating the
// ORIGINAL rows' Fields in place, must not affect what the eventual save
// persists — the persisted Fields must reflect the PRE-mutation values.
func TestSnapshotProbeResourcesForSave_FieldsIsolatedFromLaterMutation(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	const shortName = "s3" // real catalog type; queue-drain relies on session.New() defaults

	c := newSaveCacheRegressionCore(t, false)

	originalRows := []resource.Resource{
		{
			ID:   "bucket-fields-iso-1",
			Name: "bucket-fields-iso-1",
			Type: shortName,
			Fields: map[string]string{
				"region": "us-east-1",
			},
		},
	}

	// Drive the real sweep-completion seam: a single AvailabilityChecked with
	// the queue already drained (session.New() leaves AvailQueue nil,
	// AvailChecked 0, AvailTotal 0) fires the "all checks done" branch that
	// captures the dispatch-time SaveCachePayload snapshot.
	_, tasks := c.HandleEvent(messages.AvailabilityChecked{
		ResourceType: shortName,
		HasResources: true,
		Count:        len(originalRows),
		Resources:    originalRows,
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
		t.Fatal("no TaskKindSaveCache task returned — test assumption broken, cannot exercise the snapshot seam")
	}

	// Mutate the ORIGINAL rows' Fields AFTER the snapshot was captured
	// (snapshotProbeResourcesForSave ran synchronously inside HandleEvent
	// above, before this point) but BEFORE the save task executes.
	originalRows[0].Fields["region"] = "MUTATED-AFTER-SNAPSHOT"
	originalRows[0].Fields["injected"] = "should-not-appear"

	ev, err := c.ExecuteTask(context.Background(), *saveTask)
	if err != nil {
		t.Fatalf("ExecuteTask(TaskKindSaveCache): %v", err)
	}
	if flash, ok := ev.(messages.Flash); ok && flash.IsError {
		t.Fatalf("save-cache returned an error flash: %s", flash.Text)
	}

	store := cache.LoadDirForTest(saveRegProfile, saveRegRegion)
	tf, ok := store.Type(shortName)
	if !ok {
		t.Fatal("TypeFile missing after save-cache execution")
	}
	if len(tf.Rows) != 1 {
		t.Fatalf("TypeFile.Rows has %d entries, want 1", len(tf.Rows))
	}
	if tf.Rows[0].Fields["region"] != "us-east-1" {
		t.Errorf(`persisted Rows[0].Fields["region"] = %q, want "us-east-1" — the dispatch-time snapshot must be isolated from later in-place mutation of the original ProbeResources rows' Fields map`, tf.Rows[0].Fields["region"])
	}
	if _, injected := tf.Rows[0].Fields["injected"]; injected {
		t.Error(`persisted Rows[0].Fields carries the "injected" key added AFTER the snapshot was taken — Fields map is aliased, not deep-copied, by snapshotProbeResourcesForSave`)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Test 5 — rowsFromCacheRows store isolation
// ────────────────────────────────────────────────────────────────────────────

// TestAvailabilityCacheLoaded_SeededRowMutation_DoesNotWriteThroughToStore
// pins store isolation for the cold-boot row-seeding path
// (handleAvailabilityCacheLoaded -> rowsFromCacheRows): disk-seeded rows
// (with Fields and Findings) become session.ProbeResources entries; applying
// Wave-2 enrichment findings/FieldUpdates onto those seeded rows (the normal
// in-session enrichment flow) must NOT mutate the underlying *cache.Store's
// TypeFile.Rows — the Store snapshot returned by (*Store).Type/Types is
// documented as copy-safe, but rowsFromCacheRows must not alias the
// Store-owned Fields/Findings slices when building resource.Resource rows.
func TestAvailabilityCacheLoaded_SeededRowMutation_DoesNotWriteThroughToStore(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	const shortName = "s3"

	seedFindings := []domain.Finding{{Code: "s3-public-read", Phrase: "publicly readable", Severity: domain.SevBroken, Source: "wave2:s3"}}
	store := cache.LoadDirForTest(saveRegProfile, saveRegRegion)
	store.Put(shortName, cache.TypeFile{
		HasResources: true,
		Count:        1,
		Exact:        true,
		Rows: []cache.Row{
			{
				ID:       "bucket-store-iso-1",
				Name:     "bucket-store-iso-1",
				Fields:   map[string]string{"region": "us-east-1"},
				Findings: seedFindings,
			},
		},
	})
	if err := store.SaveType(shortName); err != nil {
		t.Fatalf("seed SaveType(%s): %v", shortName, err)
	}

	c := newSaveCacheRegressionCore(t, false)
	loadedStore := c.EnsureCacheStore()
	if loadedStore == nil {
		t.Fatal("EnsureCacheStore returned nil")
	}
	ev := runtime.CacheStoreToEvent(loadedStore)
	ev.Entries[shortName] = 1 // AvailabilityCacheLoaded seeding requires a positive Entries count

	c.HandleEvent(ev)

	seededRows, ok := c.ProbeResources(shortName)
	if !ok || len(seededRows) == 0 {
		t.Fatal("session.ProbeResources was not seeded for s3 after AvailabilityCacheLoaded — test assumption broken")
	}

	// Mutate the seeded rows' Fields/Findings in place — mirrors the
	// in-session Wave-2 enrichment fold (applyEnrichment / FieldUpdates
	// merge), which writes directly onto ProbeResources[type][i] fields.
	for i := range seededRows {
		if seededRows[i].Fields == nil {
			seededRows[i].Fields = make(map[string]string)
		}
		seededRows[i].Fields["region"] = "MUTATED-IN-SESSION"
		seededRows[i].Findings = append(seededRows[i].Findings, domain.Finding{
			Code: "injected-after-seed", Severity: domain.SevWarn, Source: "wave2:s3",
		})
	}

	// Re-read the Store's TypeFile directly — it must be unchanged.
	tf, ok := loadedStore.Type(shortName)
	if !ok {
		t.Fatal("Store.Type(s3) missing after re-read")
	}
	if len(tf.Rows) != 1 {
		t.Fatalf("Store TypeFile.Rows has %d entries, want unchanged 1", len(tf.Rows))
	}
	if tf.Rows[0].Fields["region"] != "us-east-1" {
		t.Errorf(`Store TypeFile.Rows[0].Fields["region"] = %q, want unchanged "us-east-1" — mutating the seeded session row must not write through to the Store`, tf.Rows[0].Fields["region"])
	}
	if len(tf.Rows[0].Findings) != 1 {
		t.Errorf("Store TypeFile.Rows[0].Findings has %d entries, want unchanged 1 — mutating the seeded session row's Findings slice must not write through to the Store (aliased backing array)", len(tf.Rows[0].Findings))
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Test 6 — tf.Issues double-write ordering within one TaskKindSaveCache
// ────────────────────────────────────────────────────────────────────────────

// TestExecuteTask_SaveCache_ExactIssueCount_SurvivesRowDerivedRecomputation
// pins the tf.Issues double-write ordering bug: within one TaskKindSaveCache
// execution, availabilityFromResourceCache computes an exact issueCounts
// observation from c.session.ResourceCache (issueKnown=true), which
// SaveAvailabilityCache persists correctly. But the SAME execution also
// calls saveProbeResourcesToTypeFiles, which independently recomputes
// td.ExcludeFromIssueBadge-gated issues from the swept ROWS via
// unifiedIssueCount — when those rows carry NO findings and no Wave-1 issue
// color, that second write must not clobber the first exact observation
// down to 0. The final persisted tf.Issues must equal the exact 5 from
// availabilityFromResourceCache, not 0.
func TestExecuteTask_SaveCache_ExactIssueCount_SurvivesRowDerivedRecomputation(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	// Uses "ec2", not "s3": colorS3 unconditionally returns ColorHealthy
	// (ignores Findings entirely), so an s3 fixture cannot produce a
	// Wave-1-issue-colored row for availabilityFromResourceCache to count.
	// colorEC2 honors a Findings[i].Source=="wave1" entry directly.
	const shortName = "ec2"

	c := newSaveCacheRegressionCore(t, false)

	// Seed session.ResourceCache with 5 issue-colored rows (Wave-1) so
	// availabilityFromResourceCache's issueCounts["ec2"] == 5, issueKnown["ec2"] == true.
	issueRows := make([]resource.Resource, 5)
	for i := range issueRows {
		issueRows[i] = resource.Resource{
			ID:   saveRegID("issue", i),
			Name: saveRegID("issue", i),
			Type: shortName,
			Findings: []domain.Finding{
				{Code: "ec2-impaired", Severity: domain.SevBroken, Source: "wave1"},
			},
		}
	}
	c.SetResourceCache(shortName, &domain.ListViewCacheEntry{Resources: issueRows})

	// The SaveCachePayload's swept rows carry NO findings at all (mirrors a
	// sweep landing before Wave-2 enrichment confirms anything) — the
	// row-derived recomputation inside saveProbeResourcesToTypeFiles must not
	// downgrade the exact 5 computed above from the resource cache.
	sweptRows := make([]resource.Resource, 5)
	for i := range sweptRows {
		sweptRows[i] = resource.Resource{ID: saveRegID("issue", i), Name: saveRegID("issue", i), Type: shortName}
	}
	payload := &runtime.SaveCachePayload{
		Resources: map[string][]resource.Resource{shortName: sweptRows},
		Truncated: map[string]bool{shortName: false},
	}

	ev, err := c.ExecuteTask(context.Background(), runtime.TaskRequest{
		Key:     runtime.TaskKey{Kind: runtime.TaskKindSaveCache},
		Payload: payload,
	})
	if err != nil {
		t.Fatalf("ExecuteTask(TaskKindSaveCache): %v", err)
	}
	if flash, ok := ev.(messages.Flash); ok && flash.IsError {
		t.Fatalf("save-cache returned an error flash: %s", flash.Text)
	}

	store := cache.LoadDirForTest(saveRegProfile, saveRegRegion)
	tf, ok := store.Type(shortName)
	if !ok {
		t.Fatal("TypeFile missing after TaskKindSaveCache execution")
	}
	if !tf.IssuesKnown {
		t.Fatal("TypeFile.IssuesKnown = false, want true")
	}
	if tf.Issues != 5 {
		t.Errorf("TypeFile.Issues = %d, want 5 — the exact issue count from availabilityFromResourceCache must survive saveProbeResourcesToTypeFiles's row-derived recomputation when the swept rows carry no findings", tf.Issues)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Test 7 — counts-only save must skip an unchanged type file (no rewrite)
// ────────────────────────────────────────────────────────────────────────────

// statIno stats path and returns its inode (Unix). SaveType always writes
// via a temp file + rename (cache.go SaveType), so a physical rewrite always
// allocates a fresh inode — this is a deterministic, sleep-free way to detect
// "was this file rewritten" independent of on-disk byte content. A byte
// comparison alone cannot do this: TypeFile.SavedAt is stamped fresh on
// every Store.Put, so even a save that changes nothing else still produces
// different bytes.
func statIno(t *testing.T, path string) uint64 {
	t.Helper()
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("os.Stat(%s): %v", path, err)
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		t.Fatalf("os.Stat(%s): Sys() is not *syscall.Stat_t on this platform", path)
	}
	return st.Ino
}

// TestSaveAvailabilityCache_UnchangedEntries_DoesNotRewriteTypeFiles pins the
// upcoming fix: a second SaveAvailabilityCache call carrying IDENTICAL
// entries/truncated data for a type must not physically rewrite that type's
// on-disk file. Today (core/runtime/probes.go SaveAvailabilityCache,
// ~L308-351) every type present in the entries map is unconditionally
// Put+SaveType'd on every call, so each type file's inode changes even when
// nothing about that type's availability state changed since the prior save.
func TestSaveAvailabilityCache_UnchangedEntries_DoesNotRewriteTypeFiles(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	c := newSaveCacheRegressionCore(t, false)

	entries := map[string]int{"ec2": 10, "s3": 20}
	trunc := map[string]bool{"ec2": false, "s3": false}
	if err := c.SaveAvailabilityCache(entries, trunc, nil, nil, nil); err != nil {
		t.Fatalf("SaveAvailabilityCache (initial save): %v", err)
	}

	dir := cache.DirForTest(saveRegProfile, saveRegRegion)
	ec2Path := dir + "/ec2.yaml"
	s3Path := dir + "/s3.yaml"
	ec2InoBefore := statIno(t, ec2Path)
	s3InoBefore := statIno(t, s3Path)

	if err := c.SaveAvailabilityCache(entries, trunc, nil, nil, nil); err != nil {
		t.Fatalf("SaveAvailabilityCache (identical re-save): %v", err)
	}

	ec2InoAfter := statIno(t, ec2Path)
	s3InoAfter := statIno(t, s3Path)

	if ec2InoAfter != ec2InoBefore {
		t.Errorf("ec2.yaml inode changed (%d -> %d) after re-saving IDENTICAL availability data — SaveAvailabilityCache must skip an unchanged type file rather than rewrite it", ec2InoBefore, ec2InoAfter)
	}
	if s3InoAfter != s3InoBefore {
		t.Errorf("s3.yaml inode changed (%d -> %d) after re-saving IDENTICAL availability data — SaveAvailabilityCache must skip an unchanged type file rather than rewrite it", s3InoBefore, s3InoAfter)
	}
}

// TestSaveAvailabilityCache_OneTypeChanged_OnlyThatTypeFileIsRewritten is the
// positive counterpart: when ONE type's Count actually changes between two
// SaveAvailabilityCache calls, that type's file MUST still be rewritten —
// the untouched sibling type's file must keep its original inode. Guards
// against an over-eager "never rewrite" fix.
func TestSaveAvailabilityCache_OneTypeChanged_OnlyThatTypeFileIsRewritten(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	c := newSaveCacheRegressionCore(t, false)

	trunc := map[string]bool{"ec2": false, "s3": false}
	if err := c.SaveAvailabilityCache(map[string]int{"ec2": 10, "s3": 20}, trunc, nil, nil, nil); err != nil {
		t.Fatalf("SaveAvailabilityCache (initial save): %v", err)
	}

	dir := cache.DirForTest(saveRegProfile, saveRegRegion)
	ec2Path := dir + "/ec2.yaml"
	s3Path := dir + "/s3.yaml"
	ec2InoBefore := statIno(t, ec2Path)
	s3InoBefore := statIno(t, s3Path)

	// Only ec2's count changes; s3 is resubmitted with its identical value.
	if err := c.SaveAvailabilityCache(map[string]int{"ec2": 11, "s3": 20}, trunc, nil, nil, nil); err != nil {
		t.Fatalf("SaveAvailabilityCache (ec2 changed): %v", err)
	}

	ec2InoAfter := statIno(t, ec2Path)
	s3InoAfter := statIno(t, s3Path)

	if ec2InoAfter == ec2InoBefore {
		t.Errorf("ec2.yaml inode unchanged (%d) after its Count actually changed from 10 to 11 — a genuinely changed type file MUST still be rewritten", ec2InoBefore)
	}
	if s3InoAfter != s3InoBefore {
		t.Errorf("s3.yaml inode changed (%d -> %d) even though s3's availability data was resubmitted unchanged — only the type whose data actually changed should be rewritten", s3InoBefore, s3InoAfter)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Test 7 — concurrency safety net for the pairMu critical-section-narrowing
// fix (WithCacheStore currently holds Session.pairMu for SaveAvailabilityCache's
// entire per-type deepCopyRows+yaml.Marshal+MkdirAll+temp-write+rename
// sequence; the fix will snapshot/copy/marshal under the lock and do the
// file write+rename outside it — see Session.WithCacheStore's doc comment).
// ────────────────────────────────────────────────────────────────────────────

// TestSaveAvailabilityCache_ConcurrentWithPairMuReads_NoRaceNoDeadlock is a
// SAFETY NET, not a red test: it is written to pass identically BEFORE and
// AFTER the critical-section-narrowing fix, because a single non-reentrant
// sync.Mutex cannot deadlock on its own and a coarse lock trivially satisfies
// "no -race report" too (nothing runs outside it to race against). What it
// DOES catch is a regression the narrowing refactor could plausibly
// introduce: if the fix's snapshot/copy step going into the marshal-outside-
// the-lock stage is insufficiently deep (e.g. still aliases a store row
// slice/map instead of copying it), a concurrent pairMu-guarded reader
// observing that same store data races against the now-unlocked marshal
// goroutine under `go test -race`. Run in isolation to make the -race
// requirement explicit:
//
//	go test -race -run TestSaveAvailabilityCache_ConcurrentWithPairMuReads_NoRaceNoDeadlock ./tests/unit/
func TestSaveAvailabilityCache_ConcurrentWithPairMuReads_NoRaceNoDeadlock(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())

	const numTypes = 30
	const rowsPerType = 50
	shortNames := make([]string, numTypes)
	for i := range shortNames {
		shortNames[i] = saveRegID("concty", i)
	}

	// Seed every type with a realistically large row set on disk first, so
	// the save below performs a genuine per-type read-modify-write (not a
	// bootstrap from an empty store) — matching the "large account" shape
	// the review finding describes.
	seedStore := cache.LoadDirForTest(saveRegProfile, saveRegRegion)
	for _, name := range shortNames {
		rows := make([]cache.Row, rowsPerType)
		for i := range rows {
			rows[i] = cache.Row{ID: saveRegID(name+"-row", i), Name: saveRegID(name+"-row", i)}
		}
		seedStore.Put(name, cache.TypeFile{HasResources: true, Count: rowsPerType, Exact: true, Rows: rows})
		if err := seedStore.SaveType(name); err != nil {
			t.Fatalf("seed SaveType(%s): %v", name, err)
		}
	}

	c := newSaveCacheRegressionCore(t, false)

	// entries carries a genuinely CHANGED count for every type, so the save
	// below cannot take the skip-unchanged fast path (pinned separately by
	// TestSaveAvailabilityCache_UnchangedEntries_DoesNotRewriteTypeFiles) —
	// every type file actually gets marshaled and rewritten.
	entries := make(map[string]int, numTypes)
	trunc := make(map[string]bool, numTypes)
	for i, name := range shortNames {
		entries[name] = rowsPerType + 1 + i
		trunc[name] = false
	}

	var saveErr error
	saveDone := make(chan struct{})
	go func() {
		saveErr = c.SaveAvailabilityCache(entries, trunc, nil, nil, nil)
		close(saveDone)
	}()

	// Concurrent readers hammer the same pairMu-guarded read surface the
	// save's WithCacheStore contends with (Core.FindingFirstSeenForType ->
	// Session.ReadCacheStore -> pairMu), for as long as the save is in
	// flight, then a little past it.
	const readerGoroutines = 8
	var wg sync.WaitGroup
	wg.Add(readerGoroutines)
	stopReaders := make(chan struct{})
	for g := 0; g < readerGoroutines; g++ {
		g := g
		go func() {
			defer wg.Done()
			i := 0
			for {
				select {
				case <-stopReaders:
					return
				default:
				}
				_ = c.FindingFirstSeenForType(shortNames[(g+i)%numTypes])
				i++
			}
		}()
	}

	select {
	case <-saveDone:
	case <-time.After(10 * time.Second):
		t.Fatal("SaveAvailabilityCache did not return within the timeout while readers were hammering pairMu — possible deadlock")
	}
	close(stopReaders)
	wg.Wait()

	if saveErr != nil {
		t.Fatalf("SaveAvailabilityCache (concurrent with pairMu readers): %v", saveErr)
	}

	reloaded := cache.LoadDirForTest(saveRegProfile, saveRegRegion)
	for i, name := range shortNames {
		tf, ok := reloaded.Type(name)
		if !ok {
			t.Errorf("TypeFile %q missing on disk after the concurrent save", name)
			continue
		}
		want := rowsPerType + 1 + i
		if tf.Count != want {
			t.Errorf("TypeFile(%q).Count = %d, want %d — a save run concurrently with pairMu readers must still persist every type's new count correctly", name, tf.Count, want)
		}
	}
}
