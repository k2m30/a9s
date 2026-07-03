// runtime_savecache_regressions_test.go — regression pins for the
// Codex+CodeRabbit fix wave on internal/runtime (branch feat/cache).
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
	"testing"
	"time"

	"github.com/k2m30/a9s/v3/internal/cache"
	"github.com/k2m30/a9s/v3/internal/catalog"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
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
	store := cache.LoadDir(saveRegProfile, saveRegRegion)
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
// internal/aws/catalog_databases.go ShortName:"dbi", Aliases includes "rds").
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

	dir := cache.Dir(saveRegProfile, saveRegRegion)
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

	store := cache.LoadDir(saveRegProfile, saveRegRegion)
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

	reloaded := cache.LoadDir(saveRegProfile, saveRegRegion)
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
// Fields isolation on the DEF-7 dispatch-time snapshot: seeding
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

	store := cache.LoadDir(saveRegProfile, saveRegRegion)
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
	store := cache.LoadDir(saveRegProfile, saveRegRegion)
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

	store := cache.LoadDir(saveRegProfile, saveRegRegion)
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
