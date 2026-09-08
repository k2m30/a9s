// qa_cache_lifecycle_test.go — full cache lifecycle tests requested verbatim
// by the owner (branch feat/cache):
//
//  1. первая загрузка (кэша нет) — first load, no cache.
//  2. загрузка с кэшем (данные и ресурсы не менялись) — cache-present load,
//     world unchanged.
//  3. загрузка с кэшем (ресурсы + issues изменились) — cache-present load,
//     world CHANGED (add/remove/resolve/new-issue).
//  4. загрузка с кэшем при изменении конфигурации (колонки и их порядок) —
//     cache-present load across a VIEW-CONFIG change (columns reordered,
//     removed, added).
//
// Each scenario drives a FULL SESSION over one shared temp cache dir
// (A9S_CONFIG_FOLDER) using the real production seams:
//
//   - runtime.Bootstrap + app.New + SetUIMode("web")  (newLifecycleController,
//     mirrors newLiveWebStyleController in app_web_live_cold_boot_test.go).
//   - resource.SetPaginatedForTest to control the fake AWS world
//     deterministically (mirrors registerDepthFetcher in
//     runtime_executor_depth_refetch_test.go). The real "s3" short name is
//     used (not a synthetic type) so menu/list rendering, column resolution,
//     and cache save all go through the exact production registry path — the
//     paginated fetcher registered here is only swapped in for the fetch
//     itself; the type definition, columns, and cache file naming are s3's
//     real ones.
//   - The boot "seed load" step is driven by delivering the real
//     messages.AvailabilityCacheLoaded event (built from the on-disk store via
//     runtime.CacheStoreToEvent, exactly what ExecuteTask(TaskKindLoadAvailCache)
//     produces) through ctrl.Handle — the precedented seam used by
//     TestWebBoot_AvailabilityCacheLoaded_AppliesCountsAndIssuesToMenu in
//     app_web_live_cold_boot_test.go.
//   - The verify-fetch / availability-probe result step is
//     ctrl.Handle(messages.ResourcesLoaded{...}) (menu-syncing seam —
//     handleResourcesLoadedEvent + syncExactTotalToMenu, NOT the
//     ApplyResourcesLoaded test-only seam, which bypasses menu sync).
//   - Root-menu-state assertions (availability/issue counts) read via
//     Controller.GetMenuAvailability/GetMenuIssueCounts, NOT
//     Snapshot().Body.Menu — snapshot() only populates Body.Menu when the
//     TOP-OF-STACK screen is the menu itself (bodyKindForScreen), which is
//     never true once a list screen is open; the accessors read
//     rootMenuState() regardless of the current screen and are what a
//     real renderer uses to paint the (backgrounded) main menu's badges.
//   - The wave-2 glyph-render step is Controller.ApplyEnrichmentState, NOT
//     ctrl.Handle(messages.EnrichmentChecked{...}): that event only mutates
//     runtime.Core's session-level ResourceCache/ProbeResources
//     (core/runtime/helpers.go's applyEnrichment) — a distinct cache in
//     a distinct package. ApplyEnrichmentState is the seam
//     buildListBody's glyph rendering actually reads
//     (c.enrichmentStore/listEnrichmentFindings), and is what the TUI's own
//     ResourceListModel calls after running its own enrichment probe drive
//     (internal/tui/views/resourcelist.go) — the correct headless/web
//     equivalent.
//   - Persistence is asserted by re-reading cache.LoadDirForTest(profile, region)
//     after each Handle call — saves happen synchronously inside the
//     production handlers, no task execution required.
//
// No sleeps: every step is a synchronous Handle/Apply call, deterministic.
//
// REAL FINDING (Scenario 3, not weakened per the dispatch's instruction):
// TestCacheLifecycle_Scenario3_CachePresent_WorldChanged's "issue resolved"
// case is RED at HEAD. applyResourcesLoaded's silent-swap finding-carry-
// forward (core/app/list_body.go, the `case len(resources[i].Findings)
// == 0: resources[i].Findings = f` branch — the silent-swap findings
// carry) treats ANY
// incoming resource with zero Findings as "not yet re-checked" and
// unconditionally re-attaches its FULL prior finding set — including
// Wave-1 findings whose absence on a fresh fetch is exactly how a fetcher
// expresses "this is no longer true" (e.g. a bucket that is no longer
// publicly readable simply stops carrying the s3-public-read finding). The
// carry-forward has no way to distinguish "this fetch didn't check that
// aspect" from "this fetch confirms it's fixed", so a genuinely resolved
// Wave-1 issue's glyph, menu issue count, and persisted cache.Row.Findings
// all incorrectly survive the swap. See the test's assertions and error
// messages for the precise pin.
package unit

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/k2m30/a9s/v3/core/app"
	_ "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/cache"
	"github.com/k2m30/a9s/v3/core/config"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// ─────────────────────────────────────────────────────────────────────────
// Shared plumbing
// ─────────────────────────────────────────────────────────────────────────

const lifecycleShortName = "s3"

// newLifecycleController builds a fresh Controller/Core pair against the
// CURRENT contents of A9S_CONFIG_FOLDER (must already be set by the caller —
// every scenario below reuses the SAME temp dir across successive "boots" to
// simulate an app restart). Mirrors newLiveWebStyleController
// (app_web_live_cold_boot_test.go).
func newLifecycleController(t *testing.T, profile, region string) (*runtime.Core, *app.Controller) {
	t.Helper()
	core := runtime.Bootstrap(profile, region, resource.AllResourceTypes())
	ctrl := app.New(core)
	t.Cleanup(ctrl.Close)
	ctrl.SetUIMode("web")
	return core, ctrl
}

// bootSeedFromDisk delivers the real messages.AvailabilityCacheLoaded event —
// built from whatever is currently on disk for (profile, region) via
// runtime.CacheStoreToEvent, exactly what ExecuteTask(TaskKindLoadAvailCache)
// produces — through ctrl.Handle. This is the "seed load" step of a real
// session boot: it seeds MenuState.Availability/IssueCounts from disk AND
// seeds session.ProbeResources with the on-disk rows (handleAvailabilityCacheLoaded),
// which is what makes a subsequent list-open render instantly from cache.
func bootSeedFromDisk(ctrl *app.Controller, profile, region string) app.ViewState {
	store := cache.LoadDirForTest(profile, region)
	ev := runtime.CacheStoreToEvent(store)
	vs, _ := ctrl.Handle(ev)
	return vs
}

// s3RawFixture is a minimal AWS-SDK-shaped struct satisfying s3's real
// default column Paths (Name, BucketRegion, CreationDate) via
// fieldpath.ExtractScalar's case-insensitive field-name fallback.
type s3RawFixture struct {
	Name         string
	BucketRegion string
	CreationDate string
}

// setNoopS3Fetcher installs a paginated fetcher for "s3" that is never
// actually invoked by these tests (every fetch result is delivered directly
// via deliverVerifyFetch/ctrl.Handle, mirroring the ApplyResourcesLoaded-seam
// precedent in qa_cache_field_completeness_test.go) — it exists only so
// GetPaginatedFetcher("s3") is non-nil, matching what a real registered type
// always has, and to guarantee no live AWS call is ever attempted if
// production code's task dispatch runs the real fetcher. Callers that need
// an intermediate (pre-final) registration torn down manually before a later
// boot re-registers it use this directly with resource.CleanupPaginatedForTest;
// registerNoopS3Fetcher wraps it with t.Cleanup for the final registration in
// a test.
func setNoopS3Fetcher() {
	resource.SetPaginatedForTest(lifecycleShortName, func(_ context.Context, _ any, _ string) (resource.FetchResult, error) {
		return resource.FetchResult{}, nil
	})
}

// registerNoopS3Fetcher calls setNoopS3Fetcher and arranges cleanup via
// t.Cleanup — used for a test's FINAL fetcher registration.
func registerNoopS3Fetcher(t *testing.T) {
	t.Helper()
	setNoopS3Fetcher()
	t.Cleanup(func() { resource.CleanupPaginatedForTest(lifecycleShortName) })
}

// newS3Resource builds one realistic s3 resource.Resource with a RawStruct
// satisfying the real default s3 columns (Name/BucketRegion/CreationDate) plus
// a Fields["status"] and optional findings.
func newS3Resource(id, region, status string, findings ...domain.Finding) resource.Resource {
	return resource.Resource{
		ID:   id,
		Name: id,
		Type: lifecycleShortName,
		Fields: map[string]string{
			"status": status,
		},
		RawStruct: s3RawFixture{
			Name:         id,
			BucketRegion: region,
			CreationDate: "2024-01-01T00:00:00Z",
		},
		Findings: findings,
	}
}

// openS3List drives the real top-level list-open action.
func openS3List(ctrl *app.Controller) (app.ViewState, []runtime.TaskRequest) {
	return ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: lifecycleShortName})
}

// deliverVerifyFetch delivers a ResourcesLoaded event through the REAL
// menu-syncing seam (Controller.Handle -> handleResourcesLoadedEvent ->
// syncExactTotalToMenu), not the ApplyResourcesLoaded test-only seam (which
// bypasses menu sync). Gen:0 always passes the staleness guard
// (AcceptZeroGen=true).
// The delivery's own save runs on the cache writer's goroutine, and these
// scenarios read what one boot persisted from the NEXT boot's controller. So
// the helper waits for it: from a lifecycle test's point of view "the fetch
// landed" and "the file it produced exists" are one step.
func deliverVerifyFetch(ctrl *app.Controller, resources []resource.Resource, truncated bool) app.ViewState {
	vs, _ := ctrl.Handle(messages.ResourcesLoaded{
		ResourceType: lifecycleShortName,
		Resources:    resources,
		Pagination:   &resource.PaginationMeta{IsTruncated: truncated},
		Gen:          0, Provenance: messages.FetchProvenanceCanonicalList,
	})
	ctrl.WaitForCacheWrites()
	return vs
}

// deliverEnrichment applies wave-2 findings via Controller.ApplyEnrichmentState
// — the seam buildListBody's glyph rendering actually reads from
// (c.enrichmentStore, see core/app/list_filter.go's
// listEnrichmentFindings). This is NOT the same store
// messages.EnrichmentChecked populates: that event only mutates
// runtime.Core's session-level ResourceCache/ProbeResources
// (core/runtime/helpers.go's applyEnrichment) — a distinct cache in a
// distinct package from app.Controller's own resourceCache/enrichmentStore.
// In production, ApplyEnrichmentState is called by the TUI's own
// ResourceListModel (internal/tui/views/resourcelist.go) after it runs its
// own enrichment probe drive; a headless/web Controller session (this test's
// shape, and qa_cache_field_completeness_test.go's SilentSwap test) has no
// other wiring that copies EnrichmentChecked's findings into
// Controller.enrichmentStore, so this seam is the correct one to drive
// glyph-visible wave-2 state on a bare app.Controller.
func deliverEnrichment(ctrl *app.Controller, issues int, findings map[string][]domain.Finding) {
	ctrl.ApplyEnrichmentState(lifecycleShortName, issues, false, findings, nil)
}

// readTypeFile re-reads the on-disk TypeFile for lifecycleShortName under
// (profile, region), failing the test if it is missing.
// The per-type save no longer runs on the goroutine that triggered it: a
// 6000-row type file's marshal used to sit in the latency of the fetch that
// produced it, so the cache writer owns it now. A test that reads the file it
// just caused therefore waits for it rather than assuming it is already
// there. The deadline is what turns "never written" into a failure instead of
// a hang.
func readTypeFile(t *testing.T, profile, region string) cache.TypeFile {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if tf, ok := cache.LoadDirForTest(profile, region).Type(lifecycleShortName); ok {
			return tf
		}
		if time.Now().After(deadline) {
			t.Fatalf("cache.LoadDirForTest(%q, %q).Type(%q) missing after 5s — expected a persisted TypeFile", profile, region, lifecycleShortName)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// lifecycleFindRow returns the cache.Row with the given ID, or (Row{}, false).
func lifecycleFindRow(rows []cache.Row, id string) (cache.Row, bool) {
	for _, r := range rows {
		if r.ID == id {
			return r, true
		}
	}
	return cache.Row{}, false
}

// findListRow returns the ListRow with the given ResourceID, or (ListRow{}, false).
func findListRow(rows []app.ListRow, id string) (app.ListRow, bool) {
	for _, r := range rows {
		if r.ResourceID == id {
			return r, true
		}
	}
	return app.ListRow{}, false
}

// ─────────────────────────────────────────────────────────────────────────
// Scenario 1 — first load, no cache
// ─────────────────────────────────────────────────────────────────────────

// TestCacheLifecycle_Scenario1_FirstLoad_NoCache pins the cold-start
// behavior: an empty cache directory means the menu shows NO counts before
// any probe result lands (placeholder semantics, never a fabricated zero),
// the list-open shows the genuine Loading state (nothing to seed), and once
// the fetch + enrichment results land the rows carry real fields/glyphs and
// persist EVERY renderable column's field, findings, and correct
// count/exact/issues to disk.
func TestCacheLifecycle_Scenario1_FirstLoad_NoCache(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)
	const profile, region = "lifecycle-s1-prof", "us-east-1"

	registerNoopS3Fetcher(t)

	_, ctrl := newLifecycleController(t, profile, region)

	// --- Step 1: boot seed load over an EMPTY directory ---
	vsBoot := bootSeedFromDisk(ctrl, profile, region)
	if vsBoot.Body.Menu == nil {
		t.Fatal("Body.Menu is nil after the boot seed load")
	}
	if avail, known := findMenuEntry(vsBoot.Body.Menu, lifecycleShortName); known && avail.AvailKnown {
		t.Errorf("s3 menu entry AvailKnown=true Availability=%d before any probe has ever run — want unknown (no fabricated zero), an empty cache dir carries no observation", avail.Availability)
	}

	// --- Step 2: open the list — nothing known, must show genuine Loading ---
	vsOpen, tasks := openS3List(ctrl)
	lb := vsOpen.Body.List
	if lb == nil {
		t.Fatal("Body.List is nil after opening s3 with no cache and no probe result")
	}
	if !lb.Loading {
		t.Error("ListBody.Loading = false, want true — nothing was seeded (no ProbeResources, no disk rows), so the genuine Loading state must show")
	}
	if lb.Refreshing {
		t.Error("ListBody.Refreshing = true, want false — Refreshing only fires when something was seeded to refresh")
	}
	if len(lb.Rows) != 0 {
		t.Errorf("ListBody.Rows = %d, want 0 — nothing to seed on a genuinely cold start", len(lb.Rows))
	}
	hasFetch := false
	for _, task := range tasks {
		if task.Key.Kind == runtime.KindFetchResources {
			hasFetch = true
		}
	}
	if !hasFetch {
		t.Fatal("no KindFetchResources task dispatched on list-open — test assumption broken")
	}

	// --- Step 3: deliver the fetch result (fresh RawStruct-bearing resources) ---
	finding := domain.Finding{Code: "s3-public-read", Phrase: "public read", Severity: domain.SevBroken, Source: "wave1"}
	fresh := []resource.Resource{
		newS3Resource("bucket-s1-1", region, "active", finding),
		newS3Resource("bucket-s1-2", region, "active"),
	}
	vsLoaded := deliverVerifyFetch(ctrl, fresh, false)

	lbLoaded := vsLoaded.Body.List
	if lbLoaded == nil {
		t.Fatal("Body.List is nil after the fetch result")
	}
	if lbLoaded.Loading {
		t.Error("ListBody.Loading = true after the fetch landed, want false")
	}
	if len(lbLoaded.Rows) != 2 {
		t.Fatalf("ListBody.Rows = %d after the fetch landed, want 2", len(lbLoaded.Rows))
	}
	row1, ok := findListRow(lbLoaded.Rows, "bucket-s1-1")
	if !ok {
		t.Fatal("ListBody.Rows missing bucket-s1-1 after the fetch landed")
	}
	// Since the color-findings-conformance wave, colorS3 is
	// colorFromAnyFinding-only (core/aws/catalog_databases.go) — a
	// SevBroken Finding resolves the row's whole-row color to "broken"
	// directly (the glyph branch that used to fire when
	// ResolveColor()==ColorHealthy was deleted as unreachable).
	// ListRow.Color=="broken" is the stronger, correct check.
	if row1.Color != "broken" {
		t.Errorf("bucket-s1-1 Color = %q, want %q (broken row for its finding)", row1.Color, "broken")
	}

	// --- Step 4: apply wave-2 enrichment state (glyph-render seam;
	// finding already reached disk via Step 3's fetch result, which already
	// carried it on the resource — this call additionally guards the
	// render-time glyph path). ---
	deliverEnrichment(ctrl, 1, map[string][]domain.Finding{"bucket-s1-1": {finding}})

	// --- Step 5: persisted file must carry every renderable column's field,
	// findings, and correct count/exact/issues ---
	//
	// The keys below are the underscored spelling on purpose. The spec's row 5
	// leaves exactly one Fields key per column title, written under the
	// spelling the extraction cascade reads first; the spaced spelling these
	// assertions used to name was the second, colliding key that made a
	// replayed cell depend on Go's map order. Do not restore it.
	tf := readTypeFile(t, profile, region)
	if !tf.HasResources {
		t.Error("persisted TypeFile.HasResources = false, want true")
	}
	if tf.Count != 2 {
		t.Errorf("persisted TypeFile.Count = %d, want 2", tf.Count)
	}
	if !tf.Exact {
		t.Error("persisted TypeFile.Exact = false, want true — the fetch was untruncated")
	}
	if len(tf.Rows) != 2 {
		t.Fatalf("persisted TypeFile.Rows = %d, want 2", len(tf.Rows))
	}
	r1, ok := lifecycleFindRow(tf.Rows, "bucket-s1-1")
	if !ok {
		t.Fatal("persisted Rows missing bucket-s1-1")
	}
	// "name" rather than "bucket_name" since misc4 round 2 item (b): the
	// resolved Bucket Name column carries the catalog's Key, so that is the
	// key its cell is read from and the key materialization writes. The title
	// key nothing reads is not what a cache replay needs.
	for _, key := range []string{"name", "region", "creation_date"} {
		if v, present := r1.Fields[key]; !present || v == "" {
			t.Errorf("persisted bucket-s1-1.Fields[%q] = %q (present=%v), want a non-empty materialized value", key, v, present)
		}
	}
	if len(r1.Findings) != 1 || r1.Findings[0].Code != "s3-public-read" {
		t.Errorf("persisted bucket-s1-1.Findings = %+v, want 1 finding with Code=s3-public-read", r1.Findings)
	}
	r2, ok := lifecycleFindRow(tf.Rows, "bucket-s1-2")
	if !ok {
		t.Fatal("persisted Rows missing bucket-s1-2")
	}
	if len(r2.Findings) != 0 {
		t.Errorf("persisted bucket-s1-2.Findings = %+v, want empty (healthy row)", r2.Findings)
	}
}

// ─────────────────────────────────────────────────────────────────────────
// Scenario 2 — cache present, world unchanged
// ─────────────────────────────────────────────────────────────────────────

// TestCacheLifecycle_Scenario2_CachePresent_WorldUnchanged boots a SECOND
// controller over scenario 1's on-disk state (world unchanged) and asserts:
// menu counts+issue badges are present IMMEDIATELY from disk before any
// probe result lands; list-open seeds the SAME rows instantly (no Loading),
// same glyphs, Refreshing=true; after the verify fetch lands rows are
// IDENTICAL (stable IDs), glyphs never absent, counts unchanged; the
// re-persisted file is semantically identical.
func TestCacheLifecycle_Scenario2_CachePresent_WorldUnchanged(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)
	const profile, region = "lifecycle-s2-prof", "us-east-1"

	finding := domain.Finding{Code: "s3-public-read", Phrase: "public read", Severity: domain.SevBroken, Source: "wave1"}
	world := []resource.Resource{
		newS3Resource("bucket-s2-1", region, "active", finding),
		newS3Resource("bucket-s2-2", region, "active"),
	}

	// --- First boot: seed the disk state (mirrors scenario 1 end-state) ---
	setNoopS3Fetcher()
	func() {
		_, ctrl1 := newLifecycleController(t, profile, region)
		bootSeedFromDisk(ctrl1, profile, region)
		openS3List(ctrl1)
		deliverVerifyFetch(ctrl1, world, false)
		deliverEnrichment(ctrl1, 1, map[string][]domain.Finding{"bucket-s2-1": {finding}})
	}()
	resource.CleanupPaginatedForTest(lifecycleShortName)

	beforeTF := readTypeFile(t, profile, region)

	// --- Second boot: SAME world, fetcher re-registered fresh ---
	registerNoopS3Fetcher(t)

	_, ctrl2 := newLifecycleController(t, profile, region)

	// Step 1: boot seed load from disk — menu counts+badges present BEFORE
	// any probe result.
	vsBoot := bootSeedFromDisk(ctrl2, profile, region)
	if vsBoot.Body.Menu == nil {
		t.Fatal("Body.Menu is nil after the second boot's seed load")
	}
	entry, known := findMenuEntry(vsBoot.Body.Menu, lifecycleShortName)
	if !known || !entry.AvailKnown {
		t.Fatal("s3 menu entry AvailKnown = false immediately after the boot seed load — cache-present counts must show instantly from disk")
	}
	if entry.Availability != 2 {
		t.Errorf("s3 menu entry Availability = %d, want 2 (from disk) before any probe result", entry.Availability)
	}
	if entry.IssueBadge.Count != 1 {
		t.Errorf("s3 menu entry IssueBadge.Count = %d, want 1 (from disk) before any probe result", entry.IssueBadge.Count)
	}

	// Step 2: open the list — seeds the SAME rows instantly, no Loading.
	vsOpen, _ := openS3List(ctrl2)
	lb := vsOpen.Body.List
	if lb == nil {
		t.Fatal("Body.List is nil after opening s3 on the second boot")
	}
	if lb.Loading {
		t.Error("ListBody.Loading = true, want false — cache-seeded rows must render immediately")
	}
	if !lb.Refreshing {
		t.Error("ListBody.Refreshing = false, want true — a verify fetch must still be pending")
	}
	if len(lb.Rows) != 2 {
		t.Fatalf("ListBody.Rows = %d immediately after seeding, want 2", len(lb.Rows))
	}
	seededRow1, ok := findListRow(lb.Rows, "bucket-s2-1")
	if !ok {
		t.Fatal("seeded ListBody.Rows missing bucket-s2-1")
	}
	// colorS3 is colorFromAnyFinding-only (see the Scenario 1 comment above) —
	// ListRow.Color=="broken" is the correct check; no glyph is produced.
	if seededRow1.Color != "broken" {
		t.Errorf("seeded bucket-s2-1 Color = %q, want %q — the persisted finding's row color must show on the seeded (pre-verify) frame", seededRow1.Color, "broken")
	}

	// Step 3: deliver the verify fetch with the SAME world.
	vsLoaded := deliverVerifyFetch(ctrl2, world, false)
	lbLoaded := vsLoaded.Body.List
	if lbLoaded == nil {
		t.Fatal("Body.List is nil after the verify fetch")
	}
	if lbLoaded.Refreshing {
		t.Error("ListBody.Refreshing = true after the verify fetch landed, want false")
	}
	if len(lbLoaded.Rows) != 2 {
		t.Fatalf("ListBody.Rows = %d after the verify fetch, want 2 (identical world)", len(lbLoaded.Rows))
	}
	row1, ok := findListRow(lbLoaded.Rows, "bucket-s2-1")
	if !ok {
		t.Fatal("post-verify ListBody.Rows missing bucket-s2-1")
	}
	if row1.Color != "broken" {
		t.Errorf("post-verify bucket-s2-1 Color = %q, want %q — the broken row color must never go absent across the swap", row1.Color, "broken")
	}
	if _, ok := findListRow(lbLoaded.Rows, "bucket-s2-2"); !ok {
		t.Error("post-verify ListBody.Rows missing bucket-s2-2")
	}

	// Step 4: re-deliver enrichment (unchanged) and re-check menu counts via
	// the root-menu-state accessors (GetMenuAvailability/GetMenuIssueCounts),
	// NOT Snapshot().Body.Menu — the top-of-stack screen is still the s3
	// list here, so Body.Menu is nil per snapshot()'s
	// "populated only when top.State.Menu != nil" contract; the accessors
	// read rootMenuState() regardless of the current screen.
	deliverEnrichment(ctrl2, 1, map[string][]domain.Finding{"bucket-s2-1": {finding}})
	availAfter := ctrl2.GetMenuAvailability()
	if got := availAfter[lifecycleShortName]; got != 2 {
		t.Errorf("GetMenuAvailability()[%q] after re-verify = %d, want 2 unchanged", lifecycleShortName, got)
	}
	issuesAfter := ctrl2.GetMenuIssueCounts()
	if got := issuesAfter[lifecycleShortName]; got != 1 {
		t.Errorf("GetMenuIssueCounts()[%q] after re-verify = %d, want 1 unchanged", lifecycleShortName, got)
	}

	// Step 5: the re-persisted file is semantically identical to before.
	afterTF := readTypeFile(t, profile, region)
	if afterTF.Count != beforeTF.Count || afterTF.Exact != beforeTF.Exact || afterTF.Issues != beforeTF.Issues {
		t.Errorf("re-persisted TypeFile header changed: before={Count:%d Exact:%v Issues:%d} after={Count:%d Exact:%v Issues:%d}",
			beforeTF.Count, beforeTF.Exact, beforeTF.Issues, afterTF.Count, afterTF.Exact, afterTF.Issues)
	}
	if len(afterTF.Rows) != len(beforeTF.Rows) {
		t.Fatalf("re-persisted TypeFile.Rows = %d, want %d (same as before)", len(afterTF.Rows), len(beforeTF.Rows))
	}
	for _, want := range beforeTF.Rows {
		got, ok := lifecycleFindRow(afterTF.Rows, want.ID)
		if !ok {
			t.Errorf("re-persisted Rows missing %q which was present before", want.ID)
			continue
		}
		if len(got.Findings) != len(want.Findings) {
			t.Errorf("re-persisted Rows[%q].Findings = %+v, want %+v (unchanged)", want.ID, got.Findings, want.Findings)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────
// Scenario 3 — cache present, world CHANGED
// ─────────────────────────────────────────────────────────────────────────

// TestCacheLifecycle_Scenario3_CachePresent_WorldChanged mutates the fake
// world before the third boot (one resource removed, one added, one
// resolved, one newly-broken) and asserts: the seeded frame first shows the
// OLD state (stale-until-verified); after the verify fetch lands the new
// truth takes over everywhere (list rows, glyphs, menu count/badge, disk).
func TestCacheLifecycle_Scenario3_CachePresent_WorldChanged(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)
	const profile, region = "lifecycle-s3-prof", "us-east-1"

	brokenFinding := domain.Finding{Code: "s3-public-read", Phrase: "public read", Severity: domain.SevBroken, Source: "wave1"}
	newBrokenFinding := domain.Finding{Code: "s3-encryption-disabled", Phrase: "encryption disabled", Severity: domain.SevBroken, Source: "wave1"}

	oldWorld := []resource.Resource{
		newS3Resource("bucket-s3-removed", region, "active"),
		newS3Resource("bucket-s3-resolved", region, "active", brokenFinding), // will heal
		newS3Resource("bucket-s3-stable", region, "active"),
	}

	// --- First boot: seed the OLD world onto disk. oldWorld's
	// bucket-s3-resolved already carries brokenFinding directly (the real
	// Wave-1 fetcher shape), so deliverVerifyFetch alone is sufficient to
	// persist it — no separate enrichment call needed. ---
	setNoopS3Fetcher()
	func() {
		_, ctrl1 := newLifecycleController(t, profile, region)
		bootSeedFromDisk(ctrl1, profile, region)
		openS3List(ctrl1)
		deliverVerifyFetch(ctrl1, oldWorld, false)
	}()
	resource.CleanupPaginatedForTest(lifecycleShortName)

	// --- Mutate the world: remove one, add one, resolve one, break one ---
	newWorld := []resource.Resource{
		newS3Resource("bucket-s3-resolved", region, "active"),                 // issue resolved (finding gone)
		newS3Resource("bucket-s3-stable", region, "active", newBrokenFinding), // newly broken
		newS3Resource("bucket-s3-added", region, "active"),                    // added
		// bucket-s3-removed is gone.
	}

	// --- Third boot: mutated world ---
	registerNoopS3Fetcher(t)

	_, ctrl3 := newLifecycleController(t, profile, region)
	bootSeedFromDisk(ctrl3, profile, region)

	// Step 1: seeded frame shows the OLD state (stale-until-verified),
	// INCLUDING the resource that is about to be removed.
	vsOpen, _ := openS3List(ctrl3)
	lb := vsOpen.Body.List
	if lb == nil {
		t.Fatal("Body.List is nil after opening s3 on the third boot")
	}
	if lb.Loading {
		t.Error("ListBody.Loading = true on the seeded (pre-verify) frame, want false")
	}
	if !lb.Refreshing {
		t.Error("ListBody.Refreshing = false on the seeded (pre-verify) frame, want true")
	}
	if _, ok := findListRow(lb.Rows, "bucket-s3-removed"); !ok {
		t.Error("seeded (pre-verify) ListBody.Rows missing bucket-s3-removed — the stale OLD state must still show the not-yet-removed row")
	}
	resolvedSeeded, ok := findListRow(lb.Rows, "bucket-s3-resolved")
	if !ok {
		t.Fatal("seeded ListBody.Rows missing bucket-s3-resolved")
	}
	// colorS3 is colorFromAnyFinding-only (see the Scenario 1 comment above) —
	// ListRow.Color=="broken" is the correct check; no glyph is produced.
	if resolvedSeeded.Color != "broken" {
		t.Errorf("seeded (pre-verify) bucket-s3-resolved Color = %q, want %q — the OLD (still-broken) row color must show before verification", resolvedSeeded.Color, "broken")
	}
	if _, addedSeeded := findListRow(lb.Rows, "bucket-s3-added"); addedSeeded {
		t.Error("seeded (pre-verify) ListBody.Rows contains bucket-s3-added — the NEW resource must not appear before the verify fetch lands")
	}

	// Step 2: deliver the verify fetch with the NEW world. newWorld's
	// resources already carry their own accurate Findings (the real Wave-1
	// fetcher shape: bucket-s3-resolved healthy, bucket-s3-stable broken) —
	// no separate enrichment call is needed for the disk/glyph truth to
	// update; applyResourcesLoaded's silent-swap inheritance
	// (outgoingRowFindingsByID) only backfills findings for an incoming row
	// that itself carries none, so an incoming healthy (Findings=nil)
	// bucket-s3-resolved is not overridden by its own stale inherited
	// finding — the swap replaces, it does not merge, once the type resolves
	// via the SAME event/save cycle a genuine fetcher completion produces.
	vsLoaded := deliverVerifyFetch(ctrl3, newWorld, false)
	lbLoaded := vsLoaded.Body.List
	if lbLoaded == nil {
		t.Fatal("Body.List is nil after the verify fetch")
	}
	if len(lbLoaded.Rows) != 3 {
		t.Fatalf("ListBody.Rows = %d after the verify fetch, want 3 (new world size)", len(lbLoaded.Rows))
	}
	if _, removedStillThere := findListRow(lbLoaded.Rows, "bucket-s3-removed"); removedStillThere {
		t.Error("post-verify ListBody.Rows still contains bucket-s3-removed — removed resource must be gone")
	}
	if _, addedPresent := findListRow(lbLoaded.Rows, "bucket-s3-added"); !addedPresent {
		t.Error("post-verify ListBody.Rows missing bucket-s3-added — added resource must now be present")
	}

	resolvedAfter, ok := findListRow(lbLoaded.Rows, "bucket-s3-resolved")
	if !ok {
		t.Fatal("post-verify ListBody.Rows missing bucket-s3-resolved")
	}
	if resolvedAfter.Color == "broken" {
		t.Error("post-verify bucket-s3-resolved Color is still \"broken\", want it gone — the finding was resolved")
	}
	stableAfter, ok := findListRow(lbLoaded.Rows, "bucket-s3-stable")
	if !ok {
		t.Fatal("post-verify ListBody.Rows missing bucket-s3-stable")
	}
	if stableAfter.Color != "broken" {
		t.Errorf("post-verify bucket-s3-stable Color = %q, want %q — the newly-broken finding's row color must show", stableAfter.Color, "broken")
	}

	// Step 4: menu count/issue badge reflect the new truth. Read via the
	// root-menu-state accessors (GetMenuAvailability/GetMenuIssueCounts), not
	// Snapshot().Body.Menu — the top-of-stack screen is still the s3 list.
	availAfter := ctrl3.GetMenuAvailability()
	if got := availAfter[lifecycleShortName]; got != 3 {
		t.Errorf("GetMenuAvailability()[%q] = %d, want 3 (new world size)", lifecycleShortName, got)
	}
	issuesAfter := ctrl3.GetMenuIssueCounts()
	if got := issuesAfter[lifecycleShortName]; got != 1 {
		t.Errorf("GetMenuIssueCounts()[%q] = %d, want 1 (one broken resource in the new world)", lifecycleShortName, got)
	}

	// Step 5: persisted file holds the NEW world.
	tf := readTypeFile(t, profile, region)
	if tf.Count != 3 {
		t.Errorf("persisted TypeFile.Count = %d, want 3", tf.Count)
	}
	if len(tf.Rows) != 3 {
		t.Fatalf("persisted TypeFile.Rows = %d, want 3", len(tf.Rows))
	}
	if _, removedPersisted := lifecycleFindRow(tf.Rows, "bucket-s3-removed"); removedPersisted {
		t.Error("persisted Rows still contains bucket-s3-removed")
	}
	addedRow, ok := lifecycleFindRow(tf.Rows, "bucket-s3-added")
	if !ok {
		t.Fatal("persisted Rows missing bucket-s3-added")
	}
	// "name" rather than "bucket_name" since misc4 round 2 item (b): the
	// resolved Bucket Name column carries the catalog's Key, so that is the
	// key its cell is read from and the key materialization writes. The title
	// key nothing reads is not what a cache replay needs.
	for _, key := range []string{"name", "region", "creation_date"} {
		if v, present := addedRow.Fields[key]; !present || v == "" {
			t.Errorf("persisted bucket-s3-added.Fields[%q] = %q (present=%v), want a non-empty materialized value", key, v, present)
		}
	}
	resolvedRow, ok := lifecycleFindRow(tf.Rows, "bucket-s3-resolved")
	if !ok {
		t.Fatal("persisted Rows missing bucket-s3-resolved")
	}
	if len(resolvedRow.Findings) != 0 {
		t.Errorf("persisted bucket-s3-resolved.Findings = %+v, want empty (resolved)", resolvedRow.Findings)
	}
	stableRow, ok := lifecycleFindRow(tf.Rows, "bucket-s3-stable")
	if !ok {
		t.Fatal("persisted Rows missing bucket-s3-stable")
	}
	if len(stableRow.Findings) != 1 || stableRow.Findings[0].Code != newBrokenFinding.Code {
		t.Errorf("persisted bucket-s3-stable.Findings = %+v, want 1 finding with Code=%q", stableRow.Findings, newBrokenFinding.Code)
	}
}

// ─────────────────────────────────────────────────────────────────────────
// Scenario 4 — cache present, view CONFIG changed (columns + order)
// ─────────────────────────────────────────────────────────────────────────

// TestCacheLifecycle_Scenario4_CachePresent_ConfigChanged pins C6: a view
// config change (columns reordered, one removed, one added) between boots
// must NEVER require cache invalidation — the cached rows still seed and
// render, just projected under the new column set. The config is loaded
// through the real production path: a YAML file under
// A9S_CONFIG_FOLDER/views/s3.yaml, parsed via config.Load(), applied via
// Controller.SetViewConfig — exactly how a live session picks up
// ~/.a9s/views/*.yaml overrides.
func TestCacheLifecycle_Scenario4_CachePresent_ConfigChanged(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)
	const profile, region = "lifecycle-s4-prof", "us-east-1"

	world := []resource.Resource{
		newS3Resource("bucket-s4-1", region, "active"),
	}

	// --- First boot: OLD (built-in default) column config ---
	setNoopS3Fetcher()
	func() {
		_, ctrl1 := newLifecycleController(t, profile, region)
		bootSeedFromDisk(ctrl1, profile, region)
		openS3List(ctrl1)
		deliverVerifyFetch(ctrl1, world, false)
	}()
	resource.CleanupPaginatedForTest(lifecycleShortName)

	beforeTF := readTypeFile(t, profile, region)
	if _, present := beforeTF.Rows[0].Fields["bucket_owner"]; present {
		t.Fatal("fixture assumption broken — 'bucket_owner' must not be a field the OLD config ever materialized")
	}

	// --- Write a NEW view config: reorder (Status before Bucket Name),
	// remove Region, add a new path-backed column (Bucket Owner). ---
	viewsDir := filepath.Join(tmp, "views")
	if err := os.MkdirAll(viewsDir, 0o755); err != nil {
		t.Fatalf("creating views dir: %v", err)
	}
	newConfigYAML := `list:
  Status:
    key: status
    width: 32
  Bucket Name:
    path: Name
    width: 36
  Bucket Owner:
    path: OwnerName
    width: 24
detail:
  - Name
`
	if err := os.WriteFile(filepath.Join(viewsDir, "s3.yaml"), []byte(newConfigYAML), 0o644); err != nil {
		t.Fatalf("writing new s3.yaml view config: %v", err)
	}
	vc, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() after writing the new s3.yaml: %v", err)
	}
	if vc == nil {
		t.Fatal("config.Load() returned nil config after writing views/s3.yaml — production config-load path did not pick up the file")
	}

	// --- Second boot: apply the new config via the real SetViewConfig seam ---
	_, ctrl2 := newLifecycleController(t, profile, region)
	ctrl2.SetViewConfig(vc)

	registerNoopS3Fetcher(t)

	bootSeedFromDisk(ctrl2, profile, region)

	// Step 1: seeded (cached) rows still render under the NEW config, no
	// crash, no Loading regression.
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("opening the list under a changed view config panicked: %v", r)
			}
		}()
		openS3List(ctrl2)
	}()

	snapSeeded := ctrl2.Snapshot()
	lb := snapSeeded.Body.List
	if lb == nil {
		t.Fatal("Body.List is nil after opening s3 under the new config")
	}
	if lb.Loading {
		t.Error("ListBody.Loading = true under the new config, want false — a config change must never force a loading regression on cached rows")
	}
	if len(lb.Rows) != 1 {
		t.Fatalf("ListBody.Rows = %d under the new config, want 1 (cached row still seeds)", len(lb.Rows))
	}

	// Column order: Status, Bucket Name, Bucket Owner (no Region, no
	// Creation Date).
	wantTitles := []string{"Status", "Bucket Name", "Bucket Owner"}
	if len(lb.Columns) != len(wantTitles) {
		t.Fatalf("ListBody.Columns = %d, want %d: %+v", len(lb.Columns), len(wantTitles), lb.Columns)
	}
	for i, want := range wantTitles {
		if lb.Columns[i].Title != want {
			t.Errorf("ListBody.Columns[%d].Title = %q, want %q (new column order)", i, lb.Columns[i].Title, want)
		}
	}
	for _, removed := range []string{"Region", "Creation Date"} {
		for _, col := range lb.Columns {
			if col.Title == removed {
				t.Errorf("ListBody.Columns still contains removed column %q", removed)
			}
		}
	}
	if len(lb.Rows[0].Cells) != len(wantTitles) {
		t.Errorf("seeded row Cells = %d, want %d (one cell per new-config column)", len(lb.Rows[0].Cells), len(wantTitles))
	}

	// Step 2: verify fetch lands with fresh RawStruct carrying the new
	// column's source field (OwnerName) — the new column must fill in and
	// persist. The old-config gap (no "bucket_owner" key on the seeded row)
	// heals on this first live re-fetch, exactly like C1's one-cycle-heal
	// contract for any other Path-backed/Key-less gap.
	freshWithOwner := []resource.Resource{
		{
			ID:   "bucket-s4-1",
			Name: "bucket-s4-1",
			Type: lifecycleShortName,
			Fields: map[string]string{
				"status": "active",
			},
			RawStruct: struct {
				Name         string
				BucketRegion string
				CreationDate string
				OwnerName    string
			}{
				Name:         "bucket-s4-1",
				BucketRegion: region,
				CreationDate: "2024-01-01T00:00:00Z",
				OwnerName:    "team-platform",
			},
		},
	}
	vsLoaded := deliverVerifyFetch(ctrl2, freshWithOwner, false)
	lbLoaded := vsLoaded.Body.List
	if lbLoaded == nil {
		t.Fatal("Body.List is nil after the verify fetch under the new config")
	}
	if len(lbLoaded.Rows) != 1 {
		t.Fatalf("ListBody.Rows = %d after the verify fetch, want 1", len(lbLoaded.Rows))
	}
	// The row-decorator pin that stood here is gone with the plumbing (tui5
	// row 5): a list row carries no marker, so "no findings" is read off the
	// row's colour, which the sibling assertions already cover. Do not restore
	// a Decorator assertion — there is no such field.

	// Step 3: persisted file now carries the new column's field too.
	afterTF := readTypeFile(t, profile, region)
	if len(afterTF.Rows) != 1 {
		t.Fatalf("persisted TypeFile.Rows = %d after the verify+save, want 1", len(afterTF.Rows))
	}
	got := afterTF.Rows[0]
	if v, present := got.Fields["bucket_owner"]; !present || v != "team-platform" {
		t.Errorf(`persisted bucket-s4-1.Fields["bucket_owner"] = %q (present=%v), want "team-platform" — the new config column must materialize and persist once a genuine fetch supplies its source field`, v, present)
	}
	// "name", matching scenarios 1 and 3 above. INVERTED for aws6 row 16 (one
	// cascade arm): this scenario's YAML declares a Key-LESS column titled
	// "Bucket Name", and the save projection used to key such a column by its
	// title, giving "bucket_name". A loaded view file no longer comes back
	// without the catalog's Key — the view owns the column set and its order,
	// the catalog owns what each cell reads, on every path — so the resolved
	// column carries the catalog's Key and that is the key materialization
	// writes. The title key nothing reads is not what a cache replay needs.
	// Do not restore "bucket_name": it would mean a column resolved one way
	// for this scenario and another for the two above.
	if v, present := got.Fields["name"]; !present || v == "" {
		t.Errorf(`persisted bucket-s4-1.Fields["name"] = %q (present=%v), want non-empty — surviving columns must still materialize under the new config`, v, present)
	}
}

// ─────────────────────────────────────────────────────────────────────────
// helpers requiring app package types (kept local, no cross-file coupling)
// ─────────────────────────────────────────────────────────────────────────

// findMenuEntry returns the MenuEntry for shortName from mb, or (zero, false).
func findMenuEntry(mb *app.MenuBody, shortName string) (app.MenuEntry, bool) {
	if mb == nil {
		return app.MenuEntry{}, false
	}
	for _, e := range mb.Entries {
		if e.ShortName == shortName {
			return e, true
		}
	}
	return app.MenuEntry{}, false
}
