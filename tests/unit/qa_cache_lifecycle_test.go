// Full cache lifecycle tests:
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
// Each scenario drives a full session over one shared temp cache dir
// (A9S_CONFIG_FOLDER) through the production seams, with no sleeps. The real
// "s3" short name is used so menu/list rendering, column resolution and cache
// save all go through the production registry; only the fetch is swapped.
//
// Root-menu counts are read via Controller.GetMenuAvailability and
// GetMenuIssueCounts, not Snapshot().Body.Menu: the snapshot populates
// Body.Menu only when the menu is top of stack, which is never true once a
// list screen is open.
//
// Wave-2 glyph state is driven through Controller.ApplyEnrichmentState, not
// messages.EnrichmentChecked: that event mutates runtime.Core's session
// ResourceCache/ProbeResources, a distinct cache from the c.enrichmentStore
// that buildListBody's glyphs read.
//
// A fetcher reports a resolved Wave-1 issue by omitting its finding from a
// fresh fetch (a bucket that stops being publicly readable stops carrying the
// s3-public-read finding), so the silent-swap finding carry
// (core/app/list_body.go) must not re-attach a Wave-1 finding to an incoming
// row that carries none.
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

const lifecycleShortName = "s3"

// newLifecycleController builds a fresh Controller/Core pair against the
// CURRENT contents of A9S_CONFIG_FOLDER (must already be set by the caller —
// every scenario below reuses the SAME temp dir across successive "boots" to
// simulate an app restart).
func newLifecycleController(t *testing.T, profile, region string) (*runtime.Core, *app.Controller) {
	t.Helper()
	core := runtime.Bootstrap(profile, region, resource.AllResourceTypes())
	ctrl := newBlessedController(t, core)
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

// setNoopS3Fetcher installs a paginated fetcher for "s3" that the tests never
// invoke (every fetch result is delivered via deliverVerifyFetch): it keeps
// GetPaginatedFetcher("s3") non-nil, as for any registered type, and
// guarantees no live AWS call if task dispatch runs the fetcher. Callers that
// tear an intermediate registration down before a later boot re-registers it
// use this directly with resource.CleanupPaginatedForTest;
// registerNoopS3Fetcher wraps it with t.Cleanup.
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
	vs, _ := handlePage(ctrl, messages.ResourcesLoaded{
		ResourceType: lifecycleShortName,
		Resources:    resources,
		Pagination:   &resource.PaginationMeta{IsTruncated: truncated},
		Gen:          0, Provenance: messages.FetchProvenanceCanonicalList,
	})
	ctrl.WaitForCacheWrites()
	return vs
}

// deliverEnrichment applies wave-2 findings via Controller.ApplyEnrichmentState
// — the store buildListBody's glyph rendering reads (c.enrichmentStore, see
// core/app/list_filter.go's listEnrichmentFindings). messages.EnrichmentChecked
// mutates runtime.Core's session ResourceCache/ProbeResources instead.
func deliverEnrichment(ctrl *app.Controller, issues int, findings map[string][]domain.Finding) {
	ctrl.ApplyEnrichmentState(lifecycleShortName, issues, false, findings, nil)
}

// readTypeFile re-reads the on-disk TypeFile for lifecycleShortName under
// (profile, region), failing the test if it is missing.
// The per-type save does not run on the goroutine that triggered it: a
// 6000-row type file's marshal would sit in the latency of the fetch that
// produced it, so the cache writer owns it. A test that reads the file it
// just caused therefore waits for it rather than assuming it is already
// there. The deadline is what turns "never written" into a failure instead
// of a hang.
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

	vsBoot := bootSeedFromDisk(ctrl, profile, region)
	if vsBoot.Body.Menu == nil {
		t.Fatal("Body.Menu is nil after the boot seed load")
	}
	if avail, known := findMenuEntry(vsBoot.Body.Menu, lifecycleShortName); known && avail.AvailKnown {
		t.Errorf("s3 menu entry AvailKnown=true Availability=%d before any probe has ever run — want unknown (no fabricated zero), an empty cache dir carries no observation", avail.Availability)
	}

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
	// colorS3 is colorFromAnyFinding-only (core/aws/catalog_databases.go), so
	// a SevBroken Finding resolves the row\'s whole-row color to "broken".
	// ListRow.Color=="broken" is the check.
	if row1.Color != "broken" {
		t.Errorf("bucket-s1-1 Color = %q, want %q (broken row for its finding)", row1.Color, "broken")
	}

	deliverEnrichment(ctrl, 1, map[string][]domain.Finding{"bucket-s1-1": {finding}})

	// The keys below are the underscored spelling on purpose: there is exactly
	// one Fields key per column title, written under the spelling the
	// extraction cascade reads first; a second, spaced spelling would make a
	// replayed cell depend on Go's map order.
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
	// "name" rather than "bucket_name": the resolved Bucket Name column
	// carries the catalog\'s Key, so that is the
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

	registerNoopS3Fetcher(t)

	_, ctrl2 := newLifecycleController(t, profile, region)

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

	deliverEnrichment(ctrl2, 1, map[string][]domain.Finding{"bucket-s2-1": {finding}})
	availAfter := ctrl2.GetMenuAvailability()
	if got := availAfter[lifecycleShortName]; got != 2 {
		t.Errorf("GetMenuAvailability()[%q] after re-verify = %d, want 2 unchanged", lifecycleShortName, got)
	}
	issuesAfter := ctrl2.GetMenuIssueCounts()
	if got := issuesAfter[lifecycleShortName]; got != 1 {
		t.Errorf("GetMenuIssueCounts()[%q] after re-verify = %d, want 1 unchanged", lifecycleShortName, got)
	}

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

	// oldWorld's bucket-s3-resolved carries brokenFinding directly (the Wave-1
	// fetcher shape), so deliverVerifyFetch alone persists it.
	setNoopS3Fetcher()
	func() {
		_, ctrl1 := newLifecycleController(t, profile, region)
		bootSeedFromDisk(ctrl1, profile, region)
		openS3List(ctrl1)
		deliverVerifyFetch(ctrl1, oldWorld, false)
	}()
	resource.CleanupPaginatedForTest(lifecycleShortName)

	newWorld := []resource.Resource{
		newS3Resource("bucket-s3-resolved", region, "active"),                 // issue resolved (finding gone)
		newS3Resource("bucket-s3-stable", region, "active", newBrokenFinding), // newly broken
		newS3Resource("bucket-s3-added", region, "active"),                    // added
		// bucket-s3-removed is gone.
	}

	registerNoopS3Fetcher(t)

	_, ctrl3 := newLifecycleController(t, profile, region)
	bootSeedFromDisk(ctrl3, profile, region)

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

	availAfter := ctrl3.GetMenuAvailability()
	if got := availAfter[lifecycleShortName]; got != 3 {
		t.Errorf("GetMenuAvailability()[%q] = %d, want 3 (new world size)", lifecycleShortName, got)
	}
	issuesAfter := ctrl3.GetMenuIssueCounts()
	if got := issuesAfter[lifecycleShortName]; got != 1 {
		t.Errorf("GetMenuIssueCounts()[%q] = %d, want 1 (one broken resource in the new world)", lifecycleShortName, got)
	}

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
	// "name" rather than "bucket_name": the resolved Bucket Name column
	// carries the catalog\'s Key, so that is the
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

// TestCacheLifecycle_Scenario4_CachePresent_ConfigChanged: a view
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

	_, ctrl2 := newLifecycleController(t, profile, region)
	ctrl2.SetViewConfig(vc)

	registerNoopS3Fetcher(t)

	bootSeedFromDisk(ctrl2, profile, region)

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

	// The seeded row has no "bucket_owner" key; the first live re-fetch carries
	// OwnerName in a fresh RawStruct, so the new column fills in and persists.
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
	// A list row carries no marker, so "no findings" is read off the row's
	// colour, which the sibling assertions already cover.

	afterTF := readTypeFile(t, profile, region)
	if len(afterTF.Rows) != 1 {
		t.Fatalf("persisted TypeFile.Rows = %d after the verify+save, want 1", len(afterTF.Rows))
	}
	got := afterTF.Rows[0]
	if v, present := got.Fields["bucket_owner"]; !present || v != "team-platform" {
		t.Errorf(`persisted bucket-s4-1.Fields["bucket_owner"] = %q (present=%v), want "team-platform" — the new config column must materialize and persist once a genuine fetch supplies its source field`, v, present)
	}
	// "name", matching scenarios 1 and 3 above: this scenario's YAML declares
	// a Key-LESS column titled "Bucket Name", and a loaded view file comes
	// back with the catalog's Key — the view owns the column set and its
	// order, the catalog owns what each cell reads, on every path — so the
	// resolved column carries the catalog's Key and that is the key
	// materialization writes. "bucket_name" would mean a column resolved one
	// way for this scenario and another for the two above.
	if v, present := got.Fields["name"]; !present || v == "" {
		t.Errorf(`persisted bucket-s4-1.Fields["name"] = %q (present=%v), want non-empty — surviving columns must still materialize under the new config`, v, present)
	}
}

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
