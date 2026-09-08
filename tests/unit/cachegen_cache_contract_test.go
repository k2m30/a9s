// cachegen_cache_contract_test.go — pins for the cache review of 2026-09-08
// (task cachegen). One test (or small group) per reviewed claim: the cache
// may neither attribute one profile/region pair's answer to another, nor
// lose a live answer to a late disk seed, a failed probe, or an
// out-of-order commit.
//
// Every pin drives the real controller/runtime event path or the real cache
// package; nothing here reaches AWS and every identifier is synthetic.
package unit_test

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/cache"
	"github.com/k2m30/a9s/v3/core/config"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
)

// newCachegenController builds a controller bound to profile/region against
// the caller's own cache root. The caller must have redirected
// A9S_CONFIG_FOLDER to a t.TempDir() first — registered before the
// t.Cleanup(Close) below, so the controller's writer goroutine is stopped
// before the directory it writes into is removed.
func newCachegenController(t *testing.T, profile, region string) (*app.Controller, *runtime.Core, *session.Session) {
	t.Helper()
	if os.Getenv("A9S_CONFIG_FOLDER") == "" {
		t.Fatal("newCachegenController: set A9S_CONFIG_FOLDER to a t.TempDir() before building a controller")
	}
	s := session.New()
	s.SetProfileRegion(profile, region)
	core := runtime.New(s, resource.AllResourceTypes())
	ctrl := app.New(core)
	t.Cleanup(ctrl.Close)
	return ctrl, core, s
}

func cachegenRows(n int, prefix string) []resource.Resource {
	out := make([]resource.Resource, n)
	for i := range out {
		out[i] = resource.Resource{
			ID:     prefix + "-" + string(rune('a'+i%26)) + string(rune('0'+i/26)),
			Name:   prefix,
			Type:   "s3",
			Fields: map[string]string{"region": "us-east-1"},
		}
	}
	return out
}

// ───────────────────────────────────────────────────────────────────────────
// Row 1 — every async cache result and save payload carries its dispatch
// pair, and one apply point rejects a mismatch.
// ───────────────────────────────────────────────────────────────────────────

// TestCacheLoadResult_FromOtherPair_NeverPaintsCurrentMenu: an
// AvailabilityCacheLoaded produced while profile A was current must not
// reach profile B's menu when the operator switches before it is delivered.
func TestCacheLoadResult_FromOtherPair_NeverPaintsCurrentMenu(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())

	seed := cache.LoadDirForTest("cachegen-pair-a", "us-east-1")
	seed.Put("s3", cache.TypeFile{HasResources: true, Count: 9, Exact: true})
	if err := seed.SaveType("s3"); err != nil {
		t.Fatalf("seeding pair A: %v", err)
	}

	ctrl, core, s := newCachegenController(t, "cachegen-pair-a", "us-east-1")
	ev := runtime.CacheStoreToEvent(core.LoadAvailabilityCache())

	// The operator switches to pair B before the queued load is delivered.
	s.SetProfileRegion("cachegen-pair-b", "eu-west-1")
	s.Rotate()
	ctrl.ApplyIntents([]runtime.UIIntent{runtime.MenuClearAvailabilityIntent{}})

	ctrl.Handle(ev)

	if got, ok := ctrl.GetMenuAvailability()["s3"]; ok {
		t.Errorf("pair B's menu shows s3=%d from pair A's cache load — C9: a cache load result must carry its dispatch pair and be rejected when that pair is no longer current", got)
	}
}

// TestSaveCachePayload_FromOtherPair_NeverWritesIntoNewPairDirectory: a
// save frozen under pair A and executed after a switch to pair B must not
// write A's rows into B's cache directory.
func TestSaveCachePayload_FromOtherPair_NeverWritesIntoNewPairDirectory(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())

	_, core, s := newCachegenController(t, "cachegen-save-a", "us-east-1")
	snap := core.CaptureDispatch()
	payload := &runtime.SaveCachePayload{
		Resources: map[string][]resource.Resource{"s3": cachegenRows(3, "bucket-a")},
		Truncated: map[string]bool{"s3": false},
	}

	s.SetProfileRegion("cachegen-save-b", "eu-west-1")
	s.Rotate()

	if _, err := core.ExecuteTaskAt(t.Context(), runtime.TaskRequest{
		Key:     runtime.TaskKey{Kind: runtime.TaskKindSaveCache},
		Payload: payload,
	}, snap); err != nil {
		t.Fatalf("ExecuteTaskAt(save-cache): %v", err)
	}

	if tf, ok := cache.LoadDirForTest("cachegen-save-b", "eu-west-1").Type("s3"); ok {
		t.Errorf("pair B's cache directory holds pair A's s3 rows (count=%d) — C9: a save payload must carry its dispatch pair and be rejected when that pair is no longer current", tf.Count)
	}
}

// ───────────────────────────────────────────────────────────────────────────
// Row 2 — the pair-directory encoding is injective.
// ───────────────────────────────────────────────────────────────────────────

// TestPairDirectory_DistinctProfiles_NeverShareOneDirectory: two profiles
// that differ only in a character the old encoding folded to "_" must not
// display or overwrite each other's cache.
func TestPairDirectory_DistinctProfiles_NeverShareOneDirectory(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())

	slash := cache.DirForTest("team/a", "us-east-1")
	under := cache.DirForTest("team_a", "us-east-1")
	if slash == under {
		t.Fatalf("cache.DirForTest(%q) == cache.DirForTest(%q) == %q — distinct profiles must resolve to distinct directories", "team/a", "team_a", slash)
	}

	seed := cache.LoadDirForTest("team/a", "us-east-1")
	seed.Put("s3", cache.TypeFile{HasResources: true, Count: 7, Exact: true})
	if err := seed.SaveType("s3"); err != nil {
		t.Fatalf("SaveType for %q: %v", "team/a", err)
	}
	if tf, ok := cache.LoadDirForTest("team_a", "us-east-1").Type("s3"); ok {
		t.Errorf("profile %q reads profile %q's cache (count=%d) — the pair-directory encoding must be injective", "team_a", "team/a", tf.Count)
	}
}

// ───────────────────────────────────────────────────────────────────────────
// Row 3 — a Wave-2 result replaces only what it answered.
// ───────────────────────────────────────────────────────────────────────────

func cachegenWave2Finding(code string) domain.Finding {
	return domain.Finding{
		Code:     domain.FindingCode(code),
		Phrase:   "encryption not confirmed",
		Severity: domain.SevBroken,
		Source:   "wave2:s3",
	}
}

// TestEnrichmentFailure_KeepsCachedWave2Findings: a Wave-2 probe that
// answered for nobody (access denied, timeout) must not strip the findings
// already on the rows, and must not persist the apparently clean rows.
func TestEnrichmentFailure_KeepsCachedWave2Findings(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())

	_, core, _ := newCachegenController(t, "cachegen-enrichfail", "us-east-1")
	rows := cachegenRows(1, "bucket-fail")
	rows[0].Findings = []domain.Finding{cachegenWave2Finding("s3-enc-unknown")}
	core.ObserveRows("s3", rows, &resource.PaginationMeta{IsTruncated: false}, session.OriginProbe, false)

	core.HandleEvent(messages.EnrichmentChecked{
		ResourceType: "s3",
		Err:          errors.New("AccessDeniedException: not authorized"),
	})

	got, _ := core.ProbeResources("s3")
	if len(got) != 1 {
		t.Fatalf("s3 rows after a failed enrichment = %d, want 1", len(got))
	}
	if len(got[0].Findings) != 1 {
		t.Errorf("a row-less Wave-2 failure stripped the cached findings (%d left, want 1) — a failed probe answered for nobody and replaces nothing", len(got[0].Findings))
	}
}

// TestEnrichmentPartial_KeepsFindingsForUninspectedIDs: a Wave-2 result
// that lists an ID as uninspected must leave that ID's cached findings
// alone while replacing the ones it did answer for.
func TestEnrichmentPartial_KeepsFindingsForUninspectedIDs(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())

	_, core, _ := newCachegenController(t, "cachegen-enrichpartial", "us-east-1")
	rows := cachegenRows(2, "bucket-partial")
	answered, uninspected := rows[0].ID, rows[1].ID
	rows[0].Findings = []domain.Finding{cachegenWave2Finding("s3-enc-unknown")}
	rows[1].Findings = []domain.Finding{cachegenWave2Finding("s3-enc-unknown")}
	core.ObserveRows("s3", rows, &resource.PaginationMeta{IsTruncated: false}, session.OriginProbe, false)

	core.HandleEvent(messages.EnrichmentChecked{
		ResourceType: "s3",
		Findings:     map[string][]domain.Finding{},
		TruncatedIDs: map[string]bool{uninspected: true},
	})

	got, _ := core.ProbeResources("s3")
	byID := make(map[string]resource.Resource, len(got))
	for _, r := range got {
		byID[r.ID] = r
	}
	if n := len(byID[answered].Findings); n != 0 {
		t.Errorf("answered row %s kept %d Wave-2 findings, want 0 — the result IS the answer for the IDs it inspected", answered, n)
	}
	if n := len(byID[uninspected].Findings); n != 1 {
		t.Errorf("uninspected row %s has %d Wave-2 findings, want 1 — an ID the enricher could not inspect keeps its cached state", uninspected, n)
	}
}

// ───────────────────────────────────────────────────────────────────────────
// Row 4 — a live observation, zero rows included, outranks any disk seed.
// ───────────────────────────────────────────────────────────────────────────

// TestRowStore_DiskSeed_NeverResurrectsRowsOverLiveEmpty: a live exact
// observation of an empty population must not be refilled by a late seed.
func TestRowStore_DiskSeed_NeverResurrectsRowsOverLiveEmpty(t *testing.T) {
	rs := session.NewRowStore()
	rs.Observe("s3", nil, &resource.PaginationMeta{IsTruncated: false}, session.OriginProbe, false)
	rs.Observe("s3", cachegenRows(2, "ghost"), &resource.PaginationMeta{IsTruncated: true}, session.OriginDisk, false)

	if got := rs.Snapshot("s3"); len(got.Rows) != 0 {
		t.Errorf("a disk seed resurrected %d rows over a live empty observation — a live answer outranks any seed, zero rows included", len(got.Rows))
	}
}

// TestMenuAvailability_CacheSeed_NeverRegressesVerifiedOrigin: a late
// cache-origin count must not overwrite the live count already verified
// this session, nor flip the entry back to cache origin.
func TestMenuAvailability_CacheSeed_NeverRegressesVerifiedOrigin(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	ctrl, _, _ := newCachegenController(t, "cachegen-lateseed", "us-east-1")

	ctrl.ApplyIntents([]runtime.UIIntent{
		runtime.PatchMenuAvailability{ResourceType: "s3", Count: 3, Origin: runtime.OriginVerified},
	})
	ctrl.ApplyIntents([]runtime.UIIntent{
		runtime.PatchMenuAvailability{ResourceType: "s3", Count: 9, Origin: runtime.OriginCache},
	})

	if got := ctrl.GetMenuAvailability()["s3"]; got != 3 {
		t.Errorf("menu s3 count = %d after a late cache seed, want 3 — a verified live count is never regressed by a seed", got)
	}
}

// ───────────────────────────────────────────────────────────────────────────
// Row 5 — commits land in preparation order.
// ───────────────────────────────────────────────────────────────────────────

// TestCommitSave_OlderPlanAfterNewer_NeverOverwrites: a plan prepared
// first but committed last must not overwrite the newer state.
func TestCommitSave_OlderPlanAfterNewer_NeverOverwrites(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())

	store := cache.LoadDirForTest("cachegen-order", "us-east-1")
	store.Put("s3", cache.TypeFile{HasResources: true, Count: 10, Exact: true})
	older, err := store.PrepareSave("s3")
	if err != nil {
		t.Fatalf("PrepareSave (older): %v", err)
	}
	store.Put("s3", cache.TypeFile{HasResources: true, Count: 55, Exact: true})
	newer, err := store.PrepareSave("s3")
	if err != nil {
		t.Fatalf("PrepareSave (newer): %v", err)
	}
	if err := store.CommitSave(newer); err != nil {
		t.Fatalf("CommitSave (newer): %v", err)
	}
	if err := store.CommitSave(older); err != nil {
		t.Fatalf("CommitSave (older): %v", err)
	}

	tf, ok := cache.LoadDirForTest("cachegen-order", "us-east-1").Type("s3")
	if !ok {
		t.Fatal("no s3 type file on disk after two commits")
	}
	if tf.Count != 55 {
		t.Errorf("on-disk s3 count = %d, want 55 — a commit older than the last committed plan must be skipped, never land last", tf.Count)
	}
}

// ───────────────────────────────────────────────────────────────────────────
// Row 6 — a per-type disk save never runs under the controller mutex.
// ───────────────────────────────────────────────────────────────────────────

// TestListSave_DoesNotBlockControllerReadersOnDiskIO: while a list result's
// per-type save is running, a concurrent controller reader (the web
// snapshot, the next key) must stay responsive — the save's inputs are
// frozen under the controller mutex, the write itself runs after it.
func TestListSave_DoesNotBlockControllerReadersOnDiskIO(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	ctrl, core, _ := newCachegenController(t, "cachegen-lock", "us-east-1")

	// Stand in for a slow filesystem: the save's own column resolution, which
	// runs inside the write, blocks until the test releases it.
	saving := make(chan struct{})
	release := make(chan struct{})
	var saveOnce sync.Once
	core.SetSaveColumns(func(string) []config.ListColumn {
		saveOnce.Do(func() { close(saving) })
		<-release
		return nil
	})

	_, _ = ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: "s3"})

	handled := make(chan struct{})
	go func() {
		defer close(handled)
		ctrl.Handle(messages.ResourcesLoaded{
			ResourceType: "s3",
			Resources:    cachegenRows(2, "bucket-lock"),
			Pagination:   &resource.PaginationMeta{IsTruncated: false},
			Provenance:   messages.FetchProvenanceCanonicalList,
		})
	}()

	select {
	case <-saving:
	case <-time.After(5 * time.Second):
		close(release)
		<-handled
		t.Fatal("the list result never reached its per-type save")
	}

	snapshotDone := make(chan struct{})
	go func() {
		defer close(snapshotDone)
		ctrl.Snapshot()
	}()
	select {
	case <-snapshotDone:
	case <-time.After(2 * time.Second):
		close(release)
		<-handled
		t.Fatal("Controller.Snapshot blocked behind a per-type cache save — C4: the disk write must be queued after the controller mutex is released, as the costs cache already does")
	}
	close(release)
	<-handled

	if _, ok := cache.LoadDirForTest("cachegen-lock", "us-east-1").Type("s3"); !ok {
		t.Error("the queued list save never reached disk — moving the write off the lock must not lose it")
	}
}

// ───────────────────────────────────────────────────────────────────────────
// Row 7 — an authoritative issue observation assigns; only a
// non-authoritative one is monotonic.
// ───────────────────────────────────────────────────────────────────────────

// TestMenuIssueBadge_AuthoritativeZero_LowersHealedCount: five cached
// issues healed to zero and re-verified by Wave-2 must clear the badge.
func TestMenuIssueBadge_AuthoritativeZero_LowersHealedCount(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	ctrl, _, _ := newCachegenController(t, "cachegen-heal", "us-east-1")

	ctrl.ApplyIntents([]runtime.UIIntent{runtime.PatchResourceList{
		ResourceType: "s3",
		Issues:       &runtime.IssueBadgePatch{Count: 5},
		Enrichment:   &runtime.ListEnrichmentPatch{},
	}})
	if got := ctrl.GetMenuIssueCounts()["s3"]; got != 5 {
		t.Fatalf("menu s3 issue badge = %d after the first Wave-2 result, want 5", got)
	}

	ctrl.ApplyIntents([]runtime.UIIntent{runtime.PatchResourceList{
		ResourceType: "s3",
		Issues:       &runtime.IssueBadgePatch{Count: 0},
		Enrichment:   &runtime.ListEnrichmentPatch{},
	}})
	if got := ctrl.GetMenuIssueCounts()["s3"]; got != 0 {
		t.Errorf("menu s3 issue badge = %d after every issue healed and was re-verified, want 0 — an authoritative observation assigns the count, it does not only raise it", got)
	}
}

// ───────────────────────────────────────────────────────────────────────────
// Row 8 — one issue counter for the live badge and the saved one.
// ───────────────────────────────────────────────────────────────────────────

// TestSaveProjection_Wave2Warning_IsNotPersistedAsAnIssue: a warning-only
// enrichment shows no live issue badge, so the next restart must not load
// one either.
func TestSaveProjection_Wave2Warning_IsNotPersistedAsAnIssue(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())

	_, core, _ := newCachegenController(t, "cachegen-warnsave", "us-east-1")
	rows := cachegenRows(1, "bucket-warn")
	rows[0].Findings = []domain.Finding{{
		Code:     "s3-lifecycle-missing",
		Phrase:   "no lifecycle rule",
		Severity: domain.SevWarn,
		Source:   "wave2:s3",
	}}
	core.ObserveRows("s3", rows, &resource.PaginationMeta{IsTruncated: false}, session.OriginProbe, false)

	if _, err := core.ExecuteTask(t.Context(), runtime.TaskRequest{
		Key: runtime.TaskKey{Kind: runtime.TaskKindSaveCache},
	}); err != nil {
		t.Fatalf("ExecuteTask(save-cache): %v", err)
	}

	tf, ok := cache.LoadDirForTest("cachegen-warnsave", "us-east-1").Type("s3")
	if !ok {
		t.Fatal("no s3 type file on disk after the save")
	}
	if tf.Issues != 0 {
		t.Errorf("persisted s3 issues = %d for a warning-only enrichment, want 0 — the saved count must use the same Wave-1-color plus Wave-2-broken split the live badge uses", tf.Issues)
	}
}

// TestSaveProjection_Wave2Broken_IsPersistedAsAnIssue is the other
// partition: a "!"-severity Wave-2 finding must still reach the file.
func TestSaveProjection_Wave2Broken_IsPersistedAsAnIssue(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())

	_, core, _ := newCachegenController(t, "cachegen-brokensave", "us-east-1")
	rows := cachegenRows(1, "bucket-broken")
	rows[0].Findings = []domain.Finding{cachegenWave2Finding("s3-public-read")}
	core.ObserveRows("s3", rows, &resource.PaginationMeta{IsTruncated: false}, session.OriginProbe, false)

	if _, err := core.ExecuteTask(t.Context(), runtime.TaskRequest{
		Key: runtime.TaskKey{Kind: runtime.TaskKindSaveCache},
	}); err != nil {
		t.Fatalf("ExecuteTask(save-cache): %v", err)
	}

	tf, ok := cache.LoadDirForTest("cachegen-brokensave", "us-east-1").Type("s3")
	if !ok {
		t.Fatal("no s3 type file on disk after the save")
	}
	if tf.Issues != 1 {
		t.Errorf("persisted s3 issues = %d for one broken-severity Wave-2 finding, want 1", tf.Issues)
	}
}

// ───────────────────────────────────────────────────────────────────────────
// Row 9 — one accessor for a type's known population.
// ───────────────────────────────────────────────────────────────────────────

// TestCachedListDepth_UsesKnownCountNotRowDepth: a file that knows 55 and
// stores 50 rows must be re-verified to 55, not to 50.
func TestCachedListDepth_UsesKnownCountNotRowDepth(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())

	seed := cache.LoadDirForTest("cachegen-depth", "us-east-1")
	rows := make([]cache.Row, 50)
	for i := range rows {
		rows[i] = cache.Row{ID: cachegenRows(50, "bucket-depth")[i].ID}
	}
	seed.Put("s3", cache.TypeFile{HasResources: true, Count: 55, Exact: true, Rows: rows})
	if err := seed.SaveType("s3"); err != nil {
		t.Fatalf("seeding: %v", err)
	}

	_, core, _ := newCachegenController(t, "cachegen-depth", "us-east-1")
	if got := core.CachedListDepth("s3"); got != 55 {
		t.Errorf("CachedListDepth(s3) = %d for a count:55/rows:50 file, want 55 — the type's population is its count, not its row depth", got)
	}
}

// TestSaveProjection_ReportsKnownTotalNotRowDepth: the save projection
// must persist the authoritative total, not the number of rows in hand.
func TestSaveProjection_ReportsKnownTotalNotRowDepth(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())

	_, core, _ := newCachegenController(t, "cachegen-total", "us-east-1")
	core.ObserveRows("s3", cachegenRows(50, "bucket-total"), &resource.PaginationMeta{IsTruncated: true}, session.OriginProbe, false)
	core.ObserveCountRows("s3", 55)

	if _, err := core.ExecuteTask(t.Context(), runtime.TaskRequest{
		Key: runtime.TaskKey{Kind: runtime.TaskKindSaveCache},
	}); err != nil {
		t.Fatalf("ExecuteTask(save-cache): %v", err)
	}

	tf, ok := cache.LoadDirForTest("cachegen-total", "us-east-1").Type("s3")
	if !ok {
		t.Fatal("no s3 type file on disk after the save")
	}
	if tf.Count != 55 {
		t.Errorf("persisted s3 count = %d with 50 rows in hand and a known total of 55, want 55 — a counts-only total outranks row depth", tf.Count)
	}
}

// ───────────────────────────────────────────────────────────────────────────
// Row 10 — a pair re-entry sweeps, and only a current-generation result
// acknowledges one.
// ───────────────────────────────────────────────────────────────────────────

// TestPairReEntry_SweepsAgain: revisiting a pair already swept this
// session must still verify it on sight (C1), not report disk values as
// finished.
func TestPairReEntry_SweepsAgain(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())

	ctrl, _, s := newCachegenController(t, "cachegen-reentry", "us-east-1")
	s.MarkPairSwept()
	s.Rotate() // A → B
	s.SetProfileRegion("cachegen-reentry-b", "eu-west-1")
	s.Rotate() // B → A
	s.SetProfileRegion("cachegen-reentry", "us-east-1")

	ctrl.Handle(messages.AvailabilityCacheLoaded{Entries: map[string]int{"s3": 4}})

	body := ctrl.Snapshot().Body.Menu
	if body == nil {
		t.Fatal("menu body is nil")
	}
	if !strings.HasPrefix(body.Progress, "[verifying 0/") {
		t.Errorf("menu progress on re-entering an already-swept pair = %q, want a sweep starting at 0 — C1: everything cached is re-verified on sight, on every pair entry", body.Progress)
	}
}

// TestMenuRefreshing_StaleProbeResult_DoesNotAcknowledgeTheSweep: a probe
// result from a superseded generation must not make the menu claim the
// sweep reached that type.
func TestMenuRefreshing_StaleProbeResult_DoesNotAcknowledgeTheSweep(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())

	ctrl, core, _ := newCachegenController(t, "cachegen-staleack", "us-east-1")
	core.ObserveRows("s3", cachegenRows(1, "bucket-ack"), &resource.PaginationMeta{IsTruncated: false}, session.OriginProbe, false)

	ctrl.Handle(messages.AvailabilityChecked{ResourceType: "s3", Count: 1, Gen: 4242})

	body := ctrl.Snapshot().Body.Menu
	if body == nil {
		t.Fatal("menu body is nil")
	}
	if !body.Refreshing {
		t.Error("a stale-generation probe result acknowledged the current sweep — only a current-generation result may clear a type's refreshing mark")
	}
}

// TestMenuClearAvailability_ResetsSweepAcknowledgements: a manual full
// refresh must leave the menu refreshing until the new sweep answers.
func TestMenuClearAvailability_ResetsSweepAcknowledgements(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())

	ctrl, core, _ := newCachegenController(t, "cachegen-clearack", "us-east-1")
	core.ObserveRows("s3", cachegenRows(1, "bucket-clear"), &resource.PaginationMeta{IsTruncated: false}, session.OriginProbe, false)
	ctrl.Handle(messages.AvailabilityChecked{ResourceType: "s3", Count: 1, Gen: core.AvailabilityGen()})

	if body := ctrl.Snapshot().Body.Menu; body != nil && body.Refreshing {
		t.Fatal("menu still refreshing after the only probed type answered — precondition for this pin failed")
	}

	ctrl.ApplyIntents([]runtime.UIIntent{runtime.MenuClearAvailabilityIntent{}})
	body := ctrl.Snapshot().Body.Menu
	if body == nil {
		t.Fatal("menu body is nil")
	}
	if !body.Refreshing {
		t.Error("the menu reported a finished sweep immediately after a manual refresh cleared it — the acknowledgement map must be cleared at the same point the menu's availability is")
	}
}

// ───────────────────────────────────────────────────────────────────────────
// Row 11 — an alias-named file is canonicalized once, at load.
// ───────────────────────────────────────────────────────────────────────────

// TestAliasNamedTypeFile_SuppliesRowsUnderCanonicalName: an older
// rds.yaml must supply the dbi type's rows, not only its count.
func TestAliasNamedTypeFile_SuppliesRowsUnderCanonicalName(t *testing.T) {
	root := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", root)

	dir := cache.DirForTest("cachegen-alias", "us-east-1")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("MkdirAll(%q): %v", dir, err)
	}
	body := "version: 2\nhas_resources: true\ncount: 3\nexact: true\nrows:\n  - id: db-alias-1\n    name: alias-db\nsaved_at: 2026-01-01T00:00:00Z\n"
	if err := os.WriteFile(filepath.Join(dir, "rds.yaml"), []byte(body), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	tf, ok := cache.LoadDirForTest("cachegen-alias", "us-east-1").Type("dbi")
	if !ok {
		t.Fatal(`Store.Type("dbi") missing for an alias-named rds.yaml — an alias file must be canonicalized at load`)
	}
	if len(tf.Rows) != 1 {
		t.Errorf(`Store.Type("dbi").Rows = %d, want 1 — the canonicalized entry must carry the file's rows, not only its count`, len(tf.Rows))
	}
}

// ───────────────────────────────────────────────────────────────────────────
// Row 12 — an exact zero is knowledge, not an absent observation.
// ───────────────────────────────────────────────────────────────────────────

// TestExactEmptyTypeFile_IsReportedAsAnObservation: a valid
// exact/count:0/issues-unknown file must reach the menu as a real zero,
// not be dropped as "nothing was ever recorded".
func TestExactEmptyTypeFile_IsReportedAsAnObservation(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())

	seed := cache.LoadDirForTest("cachegen-exactzero", "us-east-1")
	seed.Put("s3", cache.TypeFile{HasResources: false, Count: 0, Exact: true})
	if err := seed.SaveType("s3"); err != nil {
		t.Fatalf("seeding: %v", err)
	}

	ev := runtime.CacheStoreToEvent(cache.LoadDirForTest("cachegen-exactzero", "us-east-1"))
	if _, ok := ev.Entries["s3"]; !ok {
		t.Error("an exact, empty s3 type file produced no menu entry — an exact zero is an observation, not an unknown")
	}
}

// TestCacheOriginZero_StaysOpenable: a cached zero has not been verified
// this session, so the menu must still let the operator open it.
func TestCacheOriginZero_StaysOpenable(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	ctrl, _, _ := newCachegenController(t, "cachegen-zeronav", "us-east-1")

	ctrl.ApplyIntents([]runtime.UIIntent{
		runtime.PatchMenuAvailability{ResourceType: "s3", Count: 0, Origin: runtime.OriginCache},
	})
	_, _ = ctrl.Apply(app.Action{Kind: app.ActionSetFilter, Arg: "s3"})

	sel, ok := ctrl.MenuSelected()
	if sel.ShortName != "s3" {
		t.Skipf("menu cursor resolved to %q, not s3 — filter shape changed", sel.ShortName)
	}
	if !ok {
		t.Error("a cache-origin zero blocked navigation — only a verified zero is confirmed empty")
	}
}

// TestExactEmptyCache_SeedsAnObservedEmptyList: a cached exact zero must
// seed the row store as observed-empty, so the list renders empty rather
// than staying an unknown placeholder.
func TestExactEmptyCache_SeedsAnObservedEmptyList(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())

	_, core, _ := newCachegenController(t, "cachegen-emptyseed", "us-east-1")
	core.HandleEvent(messages.AvailabilityCacheLoaded{
		Entries:   map[string]int{"s3": 0},
		Truncated: map[string]bool{},
	})

	seeded := false
	for _, name := range core.ProbeOriginTypeNames() {
		if name == "s3" {
			seeded = true
		}
	}
	if !seeded {
		t.Error("a cached exact zero seeded no observation for s3 — zero is a count like any other and goes through the same seeding path")
	}
}

// ───────────────────────────────────────────────────────────────────────────
// Round 2 — the runtime decides once and the intent carries the decision;
// the controller applies what the intent says and never re-decides.
// ───────────────────────────────────────────────────────────────────────────

// cachegenOpenS3List opens the top-level s3 list with rows and returns the
// controller's rendered list body.
func cachegenListBody(t *testing.T, ctrl *app.Controller) *app.ListBody {
	t.Helper()
	body := ctrl.Snapshot().Body.List
	if body == nil {
		t.Fatal("Body.List is nil — the s3 list is not the top screen")
	}
	return body
}

// cachegenLoadList delivers a canonical top-level s3 list result through the
// production task-result lane.
func cachegenLoadList(ctrl *app.Controller, rows []resource.Resource) {
	ctrl.Handle(messages.ResourcesLoaded{
		ResourceType: "s3",
		Resources:    rows,
		Pagination:   &resource.PaginationMeta{IsTruncated: false},
		Provenance:   messages.FetchProvenanceCanonicalList,
	})
}

func cachegenRowFor(t *testing.T, body *app.ListBody, id string) app.ListRow {
	t.Helper()
	for _, r := range body.Rows {
		if r.ResourceID == id {
			return r
		}
	}
	t.Fatalf("list body has no row for %q", id)
	return app.ListRow{}
}

// TestEnrichmentFailure_KeepsRenderedFindingOnTheList is row 3 on the surface
// the operator actually looks at: a Wave-2 probe that timed out without
// inspecting a row must leave that row's finding standing on screen, exactly
// as the file keeps it.
func TestEnrichmentFailure_KeepsRenderedFindingOnTheList(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	ctrl, _, _ := newCachegenController(t, "cachegen-renderfold", "us-east-1")

	rows := cachegenRows(1, "bucket-render")
	id := rows[0].ID
	rows[0].Findings = []domain.Finding{{
		Code:     "s3-public-read",
		Phrase:   "publicly accessible",
		Severity: domain.SevBroken,
		Source:   "wave2:s3",
	}}

	_, _ = ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: "s3"})
	cachegenLoadList(ctrl, rows)

	before := cachegenRowFor(t, cachegenListBody(t, ctrl), id)
	if before.Color != "broken" {
		t.Fatalf("precondition: rendered row color = %q, want %q", before.Color, "broken")
	}

	// The probe answers for nobody it was asked about: an error, and every
	// row listed as uninspected.
	ctrl.Handle(messages.EnrichmentChecked{
		ResourceType: "s3",
		Err:          errors.New("operation timed out"),
		TruncatedIDs: map[string]bool{id: true},
	})

	after := cachegenRowFor(t, cachegenListBody(t, ctrl), id)
	if after.Color != "broken" {
		t.Errorf("rendered row color = %q after a probe that inspected nothing, want %q — an uninspected row keeps the finding it is showing, the same rule the file follows", after.Color, "broken")
	}
}

// TestLateSeed_NeverRaisesTheIssueBadgeOverAVerifiedType is row 4 on the
// badge: the count beside it already refuses a seed for a type verified this
// session, and the badge must refuse it under the same rule.
func TestLateSeed_NeverRaisesTheIssueBadgeOverAVerifiedType(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	ctrl, core, _ := newCachegenController(t, "cachegen-latebadge", "us-east-1")

	// A live probe verifies s3 as empty and issue-free.
	ctrl.Handle(messages.AvailabilityChecked{
		ResourceType: "s3",
		Count:        0,
		Issues:       0,
		Gen:          core.AvailabilityGen(),
	})
	if got := ctrl.GetMenuIssueCounts()["s3"]; got != 0 {
		t.Fatalf("precondition: menu s3 issue badge = %d after a live probe, want 0", got)
	}

	// The startup disk load lands late, carrying the last session's badge.
	ctrl.Handle(messages.AvailabilityCacheLoaded{
		Entries:     map[string]int{"s3": 3},
		IssueCounts: map[string]int{"s3": 3},
		IssueKnown:  map[string]bool{"s3": true},
	})

	if got := ctrl.GetMenuIssueCounts()["s3"]; got != 0 {
		t.Errorf("menu s3 issue badge = %d after a late disk seed over a verified type, want 0 — a seed never outranks a value verified this session, badge or count", got)
	}
}

// TestListRefresh_BadgeAndFileAgree is row 7 across both surfaces: one
// authority decision per observation feeds the menu badge and the saved
// count, so a restart cannot change the badge with nothing changed in the
// account.
func TestListRefresh_BadgeAndFileAgree(t *testing.T) {
	// seedFiveCachedIssues leaves last session's five issues on disk and
	// loads them, so the badge reads 5 from the cache exactly as a warm
	// start does.
	seedFiveCachedIssues := func(t *testing.T, profile string) (*app.Controller, []resource.Resource) {
		t.Helper()
		rows := cachegenRows(5, "bucket-heal")
		crows := make([]cache.Row, len(rows))
		for i := range rows {
			rows[i].Findings = []domain.Finding{cachegenWave2Finding("s3-public-read")}
			crows[i] = cache.Row{ID: rows[i].ID, Name: rows[i].Name, Findings: rows[i].Findings}
		}
		seed := cache.LoadDirForTest(profile, "us-east-1")
		seed.Put("s3", cache.TypeFile{HasResources: true, Count: 5, Exact: true, Issues: 5, IssuesKnown: true, Rows: crows})
		if err := seed.SaveType("s3"); err != nil {
			t.Fatalf("seeding: %v", err)
		}
		ctrl, core, _ := newCachegenController(t, profile, "us-east-1")
		ctrl.Handle(runtime.CacheStoreToEvent(core.LoadAvailabilityCache()))
		if got := ctrl.GetMenuIssueCounts()["s3"]; got != 5 {
			t.Fatalf("precondition: menu s3 issue badge = %d after loading last session's cache, want 5", got)
		}
		_, _ = ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: "s3"})
		return ctrl, rows
	}

	badgeAndFile := func(t *testing.T, ctrl *app.Controller, profile string) (int, int) {
		t.Helper()
		tf, ok := cache.LoadDirForTest(profile, "us-east-1").Type("s3")
		if !ok {
			t.Fatal("no s3 type file on disk after the refresh")
		}
		return ctrl.GetMenuIssueCounts()["s3"], tf.Issues
	}

	t.Run("a verified refresh lowers both", func(t *testing.T) {
		t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
		const profile = "cachegen-heal-verified"
		ctrl, rows := seedFiveCachedIssues(t, profile)

		// Wave-2 answers for the type: every issue is gone.
		ctrl.Handle(messages.EnrichmentChecked{ResourceType: "s3"})
		healed := make([]resource.Resource, len(rows))
		for i, r := range rows {
			r.Findings = nil
			healed[i] = r
		}
		cachegenLoadList(ctrl, healed)

		badge, file := badgeAndFile(t, ctrl, profile)
		if badge != 0 || file != 0 {
			t.Errorf("menu s3 issue badge = %d and persisted issues = %d after a verified refresh of healed rows, want 0 and 0", badge, file)
		}
	})

	t.Run("an unverified refresh moves neither", func(t *testing.T) {
		t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
		const profile = "cachegen-heal-unverified"
		ctrl, rows := seedFiveCachedIssues(t, profile)

		// No Wave-2 result this session. Four of the five buckets are gone
		// and the survivor comes back on its own, so the rows carry nothing
		// to say about the four issues Wave-2 found last session.
		survivor := rows[:1]
		survivor[0].Findings = nil
		cachegenLoadList(ctrl, survivor)

		badge, file := badgeAndFile(t, ctrl, profile)
		if badge != file {
			t.Errorf("menu s3 issue badge = %d but the file it just wrote holds %d — one observation, two answers: a restart would change the badge with nothing having answered for it", badge, file)
		}
		if badge != 5 {
			t.Errorf("menu s3 issue badge = %d after an unverified refresh, want 5 — an observation that cannot prove an issue is gone lowers neither surface", badge)
		}
	})
}

// ───────────────────────────────────────────────────────────────────────────
// Round 3 — one writer of the menu origin, and one carry rule for a row the
// last Wave-2 result did not answer for.
// ───────────────────────────────────────────────────────────────────────────

// TestMenuOrigin_HasOneWriter is row 13: the menu's origin map records where
// a type's count came from, and the two lanes that observe a type must not
// each write it their own way. Exactly one assignment site may exist, and it
// is the intent that carries the origin.
func TestMenuOrigin_HasOneWriter(t *testing.T) {
	fset := token.NewFileSet()
	var sites []string
	for _, dir := range []string{"core/app", "core/runtime"} {
		entries, err := os.ReadDir(filepath.Join("..", "..", dir))
		if err != nil {
			t.Fatalf("ReadDir(%s): %v", dir, err)
		}
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			path := filepath.Join("..", "..", dir, name)
			file, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				t.Fatalf("parse %s: %v", path, err)
			}
			ast.Inspect(file, func(n ast.Node) bool {
				assign, ok := n.(*ast.AssignStmt)
				if !ok {
					return true
				}
				for _, lhs := range assign.Lhs {
					idx, ok := lhs.(*ast.IndexExpr)
					if !ok {
						continue
					}
					if sel, ok := idx.X.(*ast.SelectorExpr); ok && sel.Sel.Name == "Origin" {
						sites = append(sites, fmt.Sprintf("%s:%d", filepath.Join(dir, name), fset.Position(assign.Pos()).Line))
					}
				}
				return true
			})
		}
	}
	if len(sites) != 1 || !strings.HasPrefix(sites[0], "core/app/intents.go:") {
		t.Errorf("menu origin is written at %v, want exactly one site in core/app/intents.go — a type's origin is one fact, and a lane that writes it directly bypasses the rule every other observation goes through", sites)
	}
}

// TestListFetch_MarksVerifiedAndClearsCause is row 13's behavioural half: the
// list lane observes its type through the same intents the probe lane emits,
// so opening and fetching a type still marks it verified and still retires a
// previous probe's failure mark.
func TestListFetch_MarksVerifiedAndClearsCause(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	ctrl, _, _ := newCachegenController(t, "cachegen-listorigin", "us-east-1")

	ctrl.ApplyIntents([]runtime.UIIntent{
		runtime.PatchMenuProbeCause{ResourceType: "s3", Cause: "access-denied"},
	})

	_, _ = ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: "s3"})
	cachegenLoadList(ctrl, cachegenRows(2, "bucket-origin"))

	// Back to the menu, where the origin and the cause are rendered.
	_, _ = ctrl.Apply(app.Action{Kind: app.ActionBack})
	menu := ctrl.Snapshot().Body.Menu
	if menu == nil {
		t.Fatal("menu body is nil")
	}
	var entry *app.MenuEntry
	for i := range menu.Entries {
		if menu.Entries[i].ShortName == "s3" {
			entry = &menu.Entries[i]
		}
	}
	if entry == nil {
		t.Fatal("menu has no s3 entry")
	}
	if entry.Origin != runtime.OriginVerified {
		t.Errorf("menu s3 origin = %q after the operator opened and fetched the list, want %q", entry.Origin, runtime.OriginVerified)
	}
	if entry.Cause != "" {
		t.Errorf("menu s3 cause = %q after a successful live fetch, want empty — a fetched type is not a refused one", entry.Cause)
	}
}

// TestFreshFetch_KeepsUninspectedRowsFinding is row 14: a row the last Wave-2
// result could not inspect keeps its cached finding across a fresh list
// fetch, on screen and in the file alike. Otherwise the list renders a row
// clean that the probe lane and the file both know is not.
func TestFreshFetch_KeepsUninspectedRowsFinding(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	const profile = "cachegen-carryuninspected"
	ctrl, _, _ := newCachegenController(t, profile, "us-east-1")

	rows := cachegenRows(2, "bucket-carry")
	uninspected, answered := rows[0].ID, rows[1].ID
	for i := range rows {
		rows[i].Findings = []domain.Finding{{
			Code:     "s3-public-read",
			Phrase:   "publicly accessible",
			Severity: domain.SevBroken,
			Source:   "wave2:s3",
		}}
	}
	_, _ = ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: "s3"})
	cachegenLoadList(ctrl, rows)

	// The enrichment pass answers for one row and times out on the other.
	ctrl.Handle(messages.EnrichmentChecked{
		ResourceType: "s3",
		Findings:     map[string][]domain.Finding{answered: {cachegenWave2Finding("s3-public-read")}},
		TruncatedIDs: map[string]bool{uninspected: true},
		Err:          errors.New("operation timed out"),
	})

	// A fresh fetch: AWS returns both buckets, carrying no Wave-2 data of its
	// own (Wave-2 is a separate pass).
	fresh := cachegenRows(2, "bucket-carry")
	cachegenLoadList(ctrl, fresh)

	got := cachegenRowFor(t, cachegenListBody(t, ctrl), uninspected)
	if got.Color != "broken" {
		t.Errorf("rendered row color for the uninspected bucket = %q after a fresh fetch, want %q — nothing has re-checked that row, so its finding stands", got.Color, "broken")
	}
	tf, ok := cache.LoadDirForTest(profile, "us-east-1").Type("s3")
	if !ok {
		t.Fatal("no s3 type file on disk after the fetch")
	}
	for _, r := range tf.Rows {
		if r.ID == uninspected && len(r.Findings) == 0 {
			t.Errorf("persisted row %s lost its finding — the file and the screen must keep the same row's finding", uninspected)
		}
	}
}
