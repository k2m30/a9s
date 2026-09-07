// rowstore_stage4_pins_test.go — behavior pins for Stage 4 of the row-store
// unification plan (rowstore-unification-plan.md, Stage 4: "ListState.Rows
// becomes a store-derived VIEW; Controller.resourceCache dies; ONE save lane
// through reconcileTypeFile"). Written against HEAD 7ac3b5ca (Stage 3
// landed: session.ResourceCache/LazyResourceCache are gone, RowStore is the
// sole per-type row store; Stage 4 has NOT landed — Controller still owns
// its own resourceCache map (core/app/controller.go:35-37) and
// applyResourcesLoaded still writes both ls.Rows AND c.resourceCache
// (core/app/list_body.go)).
//
// Each pin states its own honest RED/GREEN status at HEAD in its doc
// comment. Several are GREEN today (regression guards for the deletion);
// pin 4 is a source-scan that is RED today by construction (it fails while
// resourceCache still exists, and is designed to flip GREEN once Stage 4
// deletes it).
package unit

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	_ "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/cache"
	"github.com/k2m30/a9s/v3/core/config"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// stage4PinType is the real catalog short name used across every pin in this
// file. "s3" is chosen (matching qa_cache_lifecycle_test.go's own choice)
// because its default view (.a9s/views/s3.yaml) mixes a pure-Key column
// (Status) with pure-Path columns (Bucket Name/Region/Creation Date) — the
// exact "Key-only + Path columns" shape pin 2 requires, and it has a real
// registered ResourceTypeDef so resolveSaveColumns/resolveListColumnsForBuild
// both resolve non-trivially.
const stage4PinType = "s3"

// newStage4PinController builds a fresh Controller/Core pair over an
// isolated temp cache dir, mirroring newStage2PinTestController /
// newRowStorePinsTestController (each stage-pin file duplicates this small
// helper rather than depending on another file's helper lifetime).
func newStage4PinController(t *testing.T) *app.Controller {
	t.Helper()
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	core := runtime.Bootstrap("stage4-pin-profile", "us-east-1", resource.AllResourceTypes())
	ctrl := app.New(core)
	t.Cleanup(ctrl.Close)
	ctrl.SetUIMode("web")
	return ctrl
}

// stage4PinReadTypeFile re-reads the on-disk TypeFile for shortName under
// (profile, region), failing the test if missing. Local variant of
// qa_cache_lifecycle_test.go's readTypeFile / rowstore_stage2_pins_test.go's
// stage2PinReadTypeFile, generalized to an arbitrary shortName since this
// file always uses "s3".
func stage4PinReadTypeFile(t *testing.T, profile, region, shortName string) cache.TypeFile {
	t.Helper()
	store := cache.LoadDirForTest(profile, region)
	tf, ok := store.Type(shortName)
	if !ok {
		t.Fatalf("cache.LoadDirForTest(%q, %q).Type(%q) missing — expected a persisted TypeFile", profile, region, shortName)
	}
	return tf
}

// =============================================================================
// Pin 1 — stacked isolation: a top-level s3 list and a stacked
// related-filtered s3 list (same resource type, two independent
// ScreenResourceList entries) never share rows/cursor/title. Ctrl+R and
// load-more act on the TOP screen only; the screen underneath is untouched.
// StackInSync-equivalent invariant (both screens are ScreenResourceList,
// distinguishable by content) holds after every step.
// =============================================================================

// stage4PinRawFixture satisfies s3's real default column Paths (Bucket Name
// -> Name, Region -> BucketRegion, Creation Date -> CreationDate) via
// fieldpath.ExtractScalar's case-insensitive field-name fallback — mirrors
// qa_cache_lifecycle_test.go's s3RawFixture.
type stage4PinRawFixture struct {
	Name         string
	BucketRegion string
	CreationDate string
}

func stage4PinS3Resource(id string) resource.Resource {
	return resource.Resource{
		ID:   id,
		Name: id,
		Type: stage4PinType,
		RawStruct: stage4PinRawFixture{
			Name:         id,
			BucketRegion: "us-east-1",
			CreationDate: "2024-01-01T00:00:00Z",
		},
	}
}

// pushStackedRelatedFilteredS3List pushes a SECOND ScreenResourceList for
// "s3" on top of ctrl's current top screen, via the real production
// related-navigation seam: a detail view (EnsureDetailState) with one
// actionable DetailRelatedRow targeting "s3" with 2+ ResourceIDs (multi-ID,
// cache-miss => NavigationKindFilteredList per ResolveRelatedNavigate,
// handlers_related.go), selected via ActionRelatedSelect (the same path
// app_related_cursor_skip_test.go's newRelatedSkipController drives). This
// is the real Controller.dispatchRelatedNavigate -> applyRelatedNavResult
// path Stage 4 touches (core/app/navigate.go), NOT the separate legacy
// internal/tui ResourceListModel stacking mechanism.
func pushStackedRelatedFilteredS3List(t *testing.T, ctrl *app.Controller, relatedIDs []string) {
	t.Helper()
	ctrl.ApplyIntents([]runtime.UIIntent{
		runtime.PushScreen{ID: runtime.ScreenDetail},
	})
	ctrl.EnsureDetailState(resource.Resource{ID: "stage4-detail-src", Name: "stage4-detail-src", Type: "ec2"}, "ec2")
	ctrl.ApplyDetailRelated([]app.DetailRelatedRow{
		{TargetType: stage4PinType, DisplayName: stage4PinType, Count: len(relatedIDs), ResourceIDs: relatedIDs},
	})
	_, tasks := ctrl.Apply(app.Action{Kind: app.ActionRelatedSelect, Arg: "0"})
	if len(tasks) != 0 {
		// A cache-miss multi-ID filtered list dispatches relatedFetchTasks; this
		// harness never executes them (no fetch is needed — the stacked list's
		// own rows are seeded directly below via ApplyResourcesLoaded, exactly
		// like every other Stage 2/3 pin's list-lane seeding), so any returned
		// tasks are simply left dangling. Not an error.
		_ = tasks
	}
}

// TestStage4Pin_StackedIsolation_TopScreenActionsNeverTouchUnderlyingList
// drives: open top-level s3 list (3 rows, truncated) -> push a stacked
// related-filtered s3 list (2 different rows, exact) -> Ctrl+R refresh +
// load-more append on the TOP (stacked) screen only -> pop back to the
// underlying top-level list and assert its Rows/SelectedRow/title are
// completely unchanged by what happened on the screen above it. Then the
// reverse: re-push a stacked list, mutate the UNDERLYING list instead (not
// possible without popping first, since only the top screen is
// addressable) — so the isolation direction actually exercised end-to-end is
// "stacked screen's own actions never leak downward or upward", which is
// the only direction Ctrl+R/load-more (both topListState()-scoped) can ever
// violate.
//
// HONEST STATUS AT HEAD (7ac3b5ca): GREEN. core/app/list_body.go's
// applyResourcesLoaded already writes exclusively to the SCREEN's own
// ls.Rows (the "Bug 1 fix" comment: "Writing to ls.Rows ensures that two
// stacked list screens of the same resource type never share a row
// slice"); topListState()/handleActionRefresh/handleActionLoadMore already
// scope to c.stack[len(c.stack)-1] only. This is the exact per-screen
// isolation contract Stage 4's "ListState.Rows becomes a store-derived
// VIEW... keeps per-screen protocol" must NOT regress when the write moves
// from a direct field assignment to store.Observe + a per-screen view
// re-derivation — a naive Stage-4 implementation that assigned ls.Rows from
// a bare RowStore.Snapshot(canon).Rows (shared across every screen of the
// same type) would break this test immediately.
func TestStage4Pin_StackedIsolation_TopScreenActionsNeverTouchUnderlyingList(t *testing.T) {
	ctrl := newStage4PinController(t)

	// --- Top-level s3 list: 3 rows, truncated (so Ctrl+R/load-more are both
	// meaningful actions on it later). ---
	ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: stage4PinType})
	topLevelSeed := []resource.Resource{
		stage4PinS3Resource("bucket-top-1"),
		stage4PinS3Resource("bucket-top-2"),
		stage4PinS3Resource("bucket-top-3"),
	}
	ctrl.ApplyResourcesLoaded(stage4PinType, topLevelSeed, &resource.PaginationMeta{IsTruncated: true, NextToken: "top-tok-1"}, false)

	preStack := ctrl.Snapshot()
	if preStack.Body.List == nil || len(preStack.Body.List.Rows) != 3 {
		t.Fatalf("precondition: top-level s3 list = %+v, want 3 rows", preStack.Body.List)
	}
	topLevelTitleBefore := preStack.FrameTitle

	// --- Stack a related-filtered s3 list on top: 2 DIFFERENT rows, exact
	// (no pagination), distinguishable from the top-level list's rows and
	// truncation state. ---
	pushStackedRelatedFilteredS3List(t, ctrl, []string{"bucket-related-1", "bucket-related-2"})

	stackedSeed := []resource.Resource{
		stage4PinS3Resource("bucket-related-1"),
		stage4PinS3Resource("bucket-related-2"),
	}
	ctrl.ApplyResourcesLoaded(stage4PinType, stackedSeed, nil, false)

	stackedSnap := ctrl.Snapshot()
	if stackedSnap.Body.List == nil || len(stackedSnap.Body.List.Rows) != 2 {
		t.Fatalf("precondition: stacked related-filtered s3 list = %+v, want 2 rows", stackedSnap.Body.List)
	}
	if stackedSnap.Body.List.Truncated {
		t.Fatalf("precondition: stacked list must be exact (Truncated=false) to distinguish it from the top-level list's truncated state, got Truncated=true")
	}

	// --- Ctrl+R (refresh) on the TOP (stacked) screen only. ---
	ctrl.Apply(app.Action{Kind: app.ActionRefresh})
	// DeleteResourceCache + Loading=true is the observable immediate effect
	// when the stacked screen still has rows (C8: rows stay visible under a
	// refreshing marker) — re-seed via the same ApplyResourcesLoaded seam to
	// simulate the refresh's own verify-fetch landing, still scoped to the
	// stacked (top) screen only. The stacked screen carries a non-nil
	// RelatedIDSet (list_filter.go's RelatedIDSet prefilter: "only IDs in the
	// set pass"), so every new ID this test seeds onto it must also be added
	// via PatchListRelatedIDSet or it would render filtered-out even though
	// ls.Rows itself grew — this mirrors production's own
	// ReapplyCheckerAgainst pattern (list_filter.go) for extending a related
	// list's visible set across subsequent pages.
	ctrl.PatchListRelatedIDSet([]string{"bucket-related-1", "bucket-related-2", "bucket-related-3"})
	ctrl.ApplyResourcesLoaded(stage4PinType, []resource.Resource{
		stage4PinS3Resource("bucket-related-1"),
		stage4PinS3Resource("bucket-related-2"),
		stage4PinS3Resource("bucket-related-3"),
	}, nil, false)

	// --- Load-more append on the TOP (stacked) screen only. ---
	// handleActionLoadMore requires ls.HasPagination; seed a truncated state
	// first via another ApplyResourcesLoaded carrying pagination, then append.
	ctrl.ApplyResourcesLoaded(stage4PinType, []resource.Resource{
		stage4PinS3Resource("bucket-related-1"),
		stage4PinS3Resource("bucket-related-2"),
		stage4PinS3Resource("bucket-related-3"),
	}, &resource.PaginationMeta{IsTruncated: true, NextToken: "stacked-tok-1"}, false)
	ctrl.PatchListRelatedIDSet([]string{"bucket-related-1", "bucket-related-2", "bucket-related-3", "bucket-related-4"})
	ctrl.ApplyResourcesLoaded(stage4PinType, []resource.Resource{
		stage4PinS3Resource("bucket-related-4"),
	}, &resource.PaginationMeta{IsTruncated: false}, true)

	afterMutationSnap := ctrl.Snapshot()
	if afterMutationSnap.Body.List == nil || len(afterMutationSnap.Body.List.Rows) != 4 {
		t.Fatalf("stacked list after refresh+load-more = %+v, want 4 rows (bucket-related-1..4)", afterMutationSnap.Body.List)
	}

	// --- Pop back to the underlying top-level list: it must be byte-for-byte
	// unchanged by everything that happened on the screen above it. ---
	ctrl.Apply(app.Action{Kind: app.ActionBack}) // pop stacked list -> detail
	ctrl.Apply(app.Action{Kind: app.ActionBack}) // pop detail -> top-level list

	postStack := ctrl.Snapshot()
	if postStack.Body.List == nil {
		t.Fatal("Body.List is nil after popping back to the top-level s3 list")
	}
	if len(postStack.Body.List.Rows) != 3 {
		t.Fatalf("top-level s3 list Rows after stacked-screen mutation = %d, want 3 (unchanged) — stacked isolation violated", len(postStack.Body.List.Rows))
	}
	gotIDs := make(map[string]bool, len(postStack.Body.List.Rows))
	for _, r := range postStack.Body.List.Rows {
		gotIDs[r.ResourceID] = true
	}
	for _, wantID := range []string{"bucket-top-1", "bucket-top-2", "bucket-top-3"} {
		if !gotIDs[wantID] {
			t.Errorf("top-level s3 list missing original row %q after stacked-screen mutation, got IDs %v", wantID, gotIDs)
		}
	}
	if !postStack.Body.List.Truncated {
		t.Error("top-level s3 list Truncated=false after stacked-screen mutation, want true (unchanged pagination state) — stacked isolation violated")
	}
	if postStack.FrameTitle != topLevelTitleBefore {
		t.Errorf("top-level s3 list FrameTitle = %q after stacked-screen mutation, want unchanged %q — stacked isolation violated", postStack.FrameTitle, topLevelTitleBefore)
	}
}

// =============================================================================
// Pin 2 — single-save-lane byte-parity (D16 strongest form): identical
// controller state saved via the list-close (list-lane) save and via the
// sweep-completion (sweep-lane) save must persist IDENTICAL rows, for a type
// whose columns mix Key-only and Path-only entries AND whose session has a
// user-reordered/renamed column set (SetViewConfig override).
// =============================================================================

// stage4PinReorderedS3ViewConfig builds a ViewsConfig for "s3" with the same
// four columns as the built-in default but in a DIFFERENT order (Region
// first, Bucket Name second) and a renamed title, plus the built-in
// Key-only Status column — a real "user-reordered columns" session
// override, constructible directly as Go values since config.ListColumn
// carries no yaml:"-" restriction on direct field assignment (only
// UnmarshalYAML is custom).
func stage4PinReorderedS3ViewConfig() *config.ViewsConfig {
	return &config.ViewsConfig{
		Views: map[string]config.ViewDef{
			stage4PinType: {
				List: []config.ListColumn{
					{Title: "Region (moved)", Path: "BucketRegion", Width: 14},
					{Title: "Bucket Name", Path: "Name", Width: 36},
					{Title: "Creation Date", Path: "CreationDate", Width: 22},
					{Title: "Status", Key: "status", Width: 32},
				},
			},
		},
	}
}

// TestStage4Pin_D16_ListLaneAndSweepLaneSaveByteIdenticalRows_UserReorderedColumns
// drives the SAME controller state through both save lanes for a type
// (stage4PinType="s3") whose session has a SetViewConfig override with
// user-reordered columns, then asserts the two independently-triggered
// on-disk TypeFile.Rows are field-for-field identical (same set of
// Fields keys/values per row — "byte-identical" modulo TypeFile.SavedAt,
// which is a wall-clock timestamp neither lane controls and cannot be
// pinned to bytes).
//
//   - List-lane save: Controller.maybeSaveResourceListCache, fired
//     synchronously on every Handle(messages.ResourcesLoaded) delivery
//     (core/app/handle.go:212) via materializeAllListFieldsForSave,
//     which resolves columns through resolveListColumnsForBuild(c.viewConfig,
//     ...) — SEES the session's SetViewConfig override.
//   - Sweep-lane save: the EnrichmentChecked "all done" TaskKindSaveCache
//     dispatch, executed via app.DrainSync (Core.ExecuteTaskAt ->
//     saveProbeResourcesToTypeFiles -> materializeListFieldsForSave, which
//     resolves columns through resolveSaveColumns(shortName) — this function
//     takes NO *config.ViewsConfig parameter at all and unconditionally calls
//     config.GetViewDef(nil, shortName), i.e. built-in defaults only.
//
// HONEST STATUS AT HEAD (7ac3b5ca): RED — this is the live D16-class
// divergence the dispatch asked this pin to either confirm-green or catch.
// Verified by direct code reading (core/runtime/probes.go's
// resolveSaveColumns vs core/app/list_columns.go's
// resolveListColumnsForBuild): resolveSaveColumns has no viewConfig
// parameter and always resolves against config.GetViewDef(nil, shortName),
// so a session-level SetViewConfig column reorder/rename is applied by the
// list-lane save but silently ignored by the sweep-lane save. With this
// test's reordered ViewDef (Region first, "Region (moved)" title -> Fields
// key stays "region" since Key is unset and the Title's lowercased form is
// used as the fallback key — see materializeAllPathFields/
// materializeResourceFields's `key := col.Key; if key == "" { key =
// strings.ToLower(col.Title) }`), the two lanes persist DIFFERENT Fields
// key sets for the same underlying row: the list lane's Fields carry a
// "region (moved)" key (lowercased user title) while the sweep lane's
// Fields carry the built-in "region" key (from BucketRegion's built-in
// title-derived key, since s3.yaml's Region column has no explicit Key
// either) — a genuine cache-key mismatch. Stage 4's "ONE save lane... a
// single materializer with an injected column resolver" is exactly the fix:
// once both lanes resolve columns via the SAME injected resolver (which
// must include c.viewConfig), this test flips GREEN.
func TestStage4Pin_D16_ListLaneAndSweepLaneSaveByteIdenticalRows_UserReorderedColumns(t *testing.T) {
	ctrl := newStage4PinController(t)
	ctrl.SetViewConfig(stage4PinReorderedS3ViewConfig())

	ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: stage4PinType})

	rows := []resource.Resource{stage4PinS3Resource("bucket-d16-1")}

	// --- List-lane save: ResourcesLoaded through the real menu-sync seam. ---
	_, _ = ctrl.Handle(messages.ResourcesLoaded{
		ResourceType: stage4PinType,
		Resources:    rows,
		Pagination:   &resource.PaginationMeta{IsTruncated: false},
		Gen:          0, Provenance: messages.FetchProvenanceCanonicalList,
	})

	listLaneTF := stage4PinReadTypeFile(t, "stage4-pin-profile", "us-east-1", stage4PinType)
	if len(listLaneTF.Rows) != 1 {
		t.Fatalf("list-lane save persisted %d rows, want 1", len(listLaneTF.Rows))
	}
	listLaneFields := listLaneTF.Rows[0].Fields

	// --- Sweep-lane save: AvailabilityChecked -> EnrichmentChecked "all done"
	// -> DrainSync executes the returned TaskKindSaveCache task for real,
	// running saveProbeResourcesToTypeFiles (the SEPARATE sweep-lane
	// materializer) against the identical row set. ---
	_, availTasks := ctrl.Handle(messages.AvailabilityChecked{
		ResourceType: stage4PinType,
		HasResources: true,
		Count:        1,
		Gen:          1, // session.New() seeds AvailabilityGen at 1.
		Resources:    rows,
	})
	app.DrainSync(ctrl, availTasks)

	_, enrichTasks := ctrl.Handle(messages.EnrichmentChecked{
		ResourceType: stage4PinType,
		Gen:          1, // session.New() seeds EnrichmentGen at 1.
		TypeGen:      0,
	})
	app.DrainSync(ctrl, enrichTasks)

	sweepLaneTF := stage4PinReadTypeFile(t, "stage4-pin-profile", "us-east-1", stage4PinType)
	if len(sweepLaneTF.Rows) != 1 {
		t.Fatalf("sweep-lane save persisted %d rows, want 1", len(sweepLaneTF.Rows))
	}
	sweepLaneFields := sweepLaneTF.Rows[0].Fields

	if len(listLaneFields) != len(sweepLaneFields) {
		t.Fatalf("field-count mismatch between save lanes for the SAME reordered-column session: list-lane Fields=%v (%d keys), sweep-lane Fields=%v (%d keys) — D16: both save lanes must persist byte-identical rows for the same store state, but the sweep lane's resolveSaveColumns ignores the session's SetViewConfig override that the list lane's resolveListColumnsForBuild honors", listLaneFields, len(listLaneFields), sweepLaneFields, len(sweepLaneFields))
	}
	for k, v := range listLaneFields {
		sv, ok := sweepLaneFields[k]
		if !ok {
			t.Errorf("sweep-lane Fields missing key %q (list-lane value %q) — the two save lanes disagree on the resolved column key set for a user-reordered/renamed column", k, v)
			continue
		}
		if sv != v {
			t.Errorf("Fields[%q]: list-lane=%q sweep-lane=%q — save lanes disagree on value for the same key", k, v, sv)
		}
	}
}

// =============================================================================
// Pin 3 — findings/wave2 carry survives the view-switch: a silent swap on a
// seeded top-level s3 list still carries its known Wave-2 finding through
// applyResourcesLoaded, exercised through the SAME production seam the
// Stage-4 rewrite (store.Observe replacing the direct ls.Rows/resourceCache
// mirror write) must preserve.
// =============================================================================

// TestStage4Pin_FindingsCarrySurvivesSilentSwap_ThroughNewLane extends (does
// not duplicate the assertions of) qa_cache_field_completeness_test.go's
// TestSilentSwap_NeverDropsKnownFindings: same production seams
// (ApplyResourcesLoaded, GetListAllResources, ListRow.Color),
// same "seeded WAVE-2 finding survives a same-ID zero-findings replace"
// contract, but ALSO asserts the finding survives a SUBSEQUENT stacked
// screen's own independent silent swap of the SAME type — i.e. the carry
// mechanism must be per-screen (Stage 4's "keeps per-screen protocol"
// requirement), not leak or double-apply across two ListState instances of
// the same resource type.
//
// HONEST STATUS AT HEAD (7ac3b5ca): GREEN for the base silent-swap carry
// (already pinned at HEAD by TestSilentSwap_NeverDropsKnownFindings — this
// extension re-verifies the same mechanism rather than assuming it).
// Genuinely NEW assertion (not previously pinned anywhere): the stacked
// second screen's OWN silent swap must independently carry ITS OWN prior
// findings (sourced from its own ls.Rows, per outgoingRowFindingsByID's
// "prefers ls.Rows... falls back to the type-keyed resourceCache mirror"
// doc comment) without being contaminated by the top-level screen's
// findings or vice versa — a regression this test would catch if Stage 4's
// re-architecture accidentally sourced "prior findings" from a single
// shared store view instead of each screen's own ls.Rows.
func TestStage4Pin_FindingsCarrySurvivesSilentSwap_ThroughNewLane(t *testing.T) {
	ctrl := newStage4PinController(t)
	openTopLevelList(ctrl, stage4PinType)

	topFinding := domain.Finding{
		Code:     "s3-public-read",
		Phrase:   "public read",
		Severity: domain.SevBroken,
		Source:   "wave2:s3",
	}
	seeded := []resource.Resource{
		{ID: "bucket-carry-top", Name: "bucket-carry-top", Type: stage4PinType, Findings: []domain.Finding{topFinding}},
	}
	ctrl.ApplyResourcesLoaded(stage4PinType, seeded, nil, false)

	// Since the color-findings-conformance wave, colorS3 is
	// colorFromAnyFinding-only (core/aws/catalog_databases.go) — a
	// SevBroken Finding resolves the row's whole-row color to "broken"
	// directly (the glyph branch that used to fire when
	// ResolveColor()==ColorHealthy was deleted as unreachable).
	// ListRow.Color=="broken" is the stronger, correct check throughout this test.
	preSwap := ctrl.Snapshot()
	foundBefore := false
	for _, r := range preSwap.Body.List.Rows {
		if r.ResourceID == "bucket-carry-top" && r.Color == "broken" {
			foundBefore = true
		}
	}
	if !foundBefore {
		t.Fatal("fixture assumption broken — seeded bucket-carry-top does not render as a broken row before the swap")
	}

	// Silent swap on the TOP-LEVEL screen: same ID, no findings, enrichment
	// store empty (fresh session) — must inherit the prior finding.
	ctrl.ApplyResourcesLoaded(stage4PinType, []resource.Resource{
		{ID: "bucket-carry-top", Name: "bucket-carry-top", Type: stage4PinType},
	}, nil, false)

	postSwap := ctrl.Snapshot()
	postFound := false
	for _, r := range postSwap.Body.List.Rows {
		if r.ResourceID == "bucket-carry-top" && r.Color == "broken" {
			postFound = true
		}
	}
	if !postFound {
		t.Error("bucket-carry-top's broken row color vanished after the silent swap — the row's previously-known WAVE-2 finding must be inherited, not silently dropped")
	}

	// Push a SECOND, independent s3 screen (stacked related-filtered list)
	// seeded with its OWN finding on a DIFFERENT ID, then silently swap IT —
	// the two screens' carry-forward must not cross-contaminate.
	pushStackedRelatedFilteredS3List(t, ctrl, []string{"bucket-carry-stacked"})
	stackedFinding := domain.Finding{
		Code:     "s3-versioning-disabled",
		Phrase:   "versioning disabled",
		Severity: domain.SevBroken,
		Source:   "wave2:s3",
	}
	ctrl.ApplyResourcesLoaded(stage4PinType, []resource.Resource{
		{ID: "bucket-carry-stacked", Name: "bucket-carry-stacked", Type: stage4PinType, Findings: []domain.Finding{stackedFinding}},
	}, nil, false)

	ctrl.ApplyResourcesLoaded(stage4PinType, []resource.Resource{
		{ID: "bucket-carry-stacked", Name: "bucket-carry-stacked", Type: stage4PinType},
	}, nil, false)

	stackedPostSwap := ctrl.Snapshot()
	stackedFound := false
	stackedRowCount := len(stackedPostSwap.Body.List.Rows)
	for _, r := range stackedPostSwap.Body.List.Rows {
		if r.ResourceID == "bucket-carry-stacked" && r.Color == "broken" {
			stackedFound = true
		}
		if r.ResourceID == "bucket-carry-top" {
			t.Errorf("stacked screen's row set unexpectedly contains bucket-carry-top (cross-screen contamination): rows=%v", stackedPostSwap.Body.List.Rows)
		}
	}
	if stackedRowCount != 1 {
		t.Fatalf("stacked screen after its own silent swap has %d rows, want 1 (bucket-carry-stacked only)", stackedRowCount)
	}
	if !stackedFound {
		t.Error("bucket-carry-stacked's broken row color vanished after ITS OWN silent swap on the stacked screen — per-screen findings carry must work independently on every screen, not just the top-level one")
	}

	// Pop back to the top-level screen: its own carried finding (from the
	// EARLIER swap, before the stacked screen was even pushed) must still be
	// intact — proves the carry state truly lives per-screen, not in a
	// shared/overwritten mirror.
	ctrl.Apply(app.Action{Kind: app.ActionBack})
	ctrl.Apply(app.Action{Kind: app.ActionBack})
	finalTopSnap := ctrl.Snapshot()
	finalTopFound := false
	for _, r := range finalTopSnap.Body.List.Rows {
		if r.ResourceID == "bucket-carry-top" && r.Color == "broken" {
			finalTopFound = true
		}
	}
	if !finalTopFound {
		t.Error("top-level screen's carried finding (from before the stacked screen was pushed) vanished after popping back — per-screen findings carry must survive an unrelated stacked screen's own lifecycle")
	}
}

// =============================================================================
// Pin 4 — Controller.resourceCache absence: a source-scan asserting no
// `resourceCache map[string]` field exists under core/app. Written to
// FAIL today (the field still exists), listing current readers, so it
// flips to pass once the coder deletes it (mirrors
// rowstore_stage2_pins_test.go's caseInsensitiveGrepSyncProbeResourcesForTypeCallers
// pattern applied to a field declaration instead of a function name).
// =============================================================================

// scanForResourceCacheFieldDeclaration walks core/app's production Go
// source (*.go, excluding *_test.go) for the literal field declaration
// pattern `resourceCache map[string]` — the exact shape of
// Controller.resourceCache's declaration at core/app/controller.go:37
// today. Comment-only lines are skipped (a future doc comment referencing
// the deleted field by name, e.g. explaining what replaced it, must not
// keep this pin permanently red). Returns every non-comment match found as
// "path:line: text", or an error if the tree could not be walked.
func scanForResourceCacheFieldDeclaration(t *testing.T) (string, error) {
	t.Helper()
	const needle = "resourceCache map[string]"
	root, err := filepath.Abs("../../core/app")
	if err != nil {
		return "", err
	}
	var hits []string
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		for i, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "//") {
				continue
			}
			if strings.Contains(line, needle) {
				hits = append(hits, fmt.Sprintf("%s:%d: %s", path, i+1, strings.TrimSpace(line)))
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return strings.Join(hits, "\n"), nil
}

// scanForResourceCacheReaders walks core/app's production Go source for
// any remaining CODE reference to `c.resourceCache` or `.resourceCache[` —
// the field-access shape used throughout list_body.go, footer.go, text.go,
// and controller.go today (per the row-store unification plan's own
// verified inventory: controller.go:35-37 the field, controller.go:197,
// footer.go:97, list_body.go:42/129-142/713-714/770/900, text.go:219 the
// readers). Comment-only lines are skipped for the same reason as the
// declaration scan above.
func scanForResourceCacheReaders(t *testing.T) (string, error) {
	t.Helper()
	const needle = "resourceCache"
	root, err := filepath.Abs("../../core/app")
	if err != nil {
		return "", err
	}
	var hits []string
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		for i, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "//") {
				continue
			}
			if strings.Contains(line, needle) {
				hits = append(hits, fmt.Sprintf("%s:%d: %s", path, i+1, strings.TrimSpace(line)))
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return strings.Join(hits, "\n"), nil
}

// TestStage4Pin_ControllerResourceCacheField_NoLongerExists asserts that no
// production file under core/app declares a `resourceCache
// map[string]` field, and (as a companion sub-test) that no production file
// under core/app references `resourceCache` at all — the full deletion
// the plan's Stage 4 mandates ("Controller.resourceCache dies with all its
// readers").
//
// HONEST STATUS AT HEAD (7ac3b5ca): RED, by construction and confirmed by
// direct source reading — core/app/controller.go:35-37 declares
// `resourceCache map[string][]resource.Resource` today, with readers/writers
// at controller.go:197, footer.go:97, list_body.go (writer at
// applyResourcesLoaded:126-142, readers/mutators at
// ApplyListFieldUpdates:713-714, ClearRowFindings:770-775,
// applyRowFindings:819-824, GetListAllResources's listScreenResources
// helper), list_state.go:104-107, and text.go:219. This test lists every
// current hit in its failure message so the coder has a literal checklist;
// it must flip GREEN the moment Stage 4 deletes the field and every one of
// these call sites.
func TestStage4Pin_ControllerResourceCacheField_NoLongerExists(t *testing.T) {
	t.Run("no_resourceCache_field_declaration_remains", func(t *testing.T) {
		out, err := scanForResourceCacheFieldDeclaration(t)
		if err != nil {
			t.Fatalf("scan for resourceCache field declaration failed: %v", err)
		}
		if out != "" {
			t.Errorf("Controller.resourceCache field declaration still present under core/app (Stage 4 must delete it):\n%s", out)
		}
	})

	t.Run("no_resourceCache_reader_or_writer_remains", func(t *testing.T) {
		out, err := scanForResourceCacheReaders(t)
		if err != nil {
			t.Fatalf("scan for resourceCache readers failed: %v", err)
		}
		if out != "" {
			t.Errorf("production reference(s) to resourceCache still present under core/app after Stage 4 (want the field and every reader/writer deleted per the plan — ListState.Rows / RowStore must be the only remaining row source):\n%s", out)
		}
	})
}

// =============================================================================
// Pin 5 — ApplyListFieldUpdates single-application: field updates during an
// open list apply exactly once (no dual-apply through both ls.Rows and the
// dead resourceCache mirror), and a subsequent silent swap does not
// double-append findings.
// =============================================================================

// TestStage4Pin_FieldUpdatesApplyExactlyOnce_NoDualApplyThroughDeadMirror
// drives ApplyListFieldUpdates on an open top-level s3 list and asserts the
// updated field's value equals exactly the update (not concatenated,
// duplicated, or applied twice with a different final value from a
// double-apply race) — then performs a subsequent silent swap and asserts
// findings are not double-appended (exactly one copy of the carried
// finding survives, not two).
//
// HONEST STATUS AT HEAD (7ac3b5ca): GREEN for the field-value half.
// applyListFieldUpdates (core/app/list_body.go) applies
// map[string]string updates via maps.Copy onto EACH row's Fields map
// independently on ls.Rows and (separately) on c.resourceCache[typeName] —
// two DISTINCT Resource value slices (ls.Rows and c.resourceCache hold
// independently-materialized copies per applyResourcesLoaded, not shared
// backing arrays), so applying the SAME update map to both does not
// "double" a scalar string value (maps.Copy(dst, src) is idempotent for a
// given src) — the field ends up correct on ls.Rows regardless of whether
// the dead mirror is also updated. This test is a regression guard for
// Stage 4: the risk it catches is not today's behavior but a careless
// Stage-4 rewrite that accidentally ran the update loop TWICE over the SAME
// ls.Rows slice (once via a leftover legacy path, once via the new
// store-view derivation), which WOULD show up as an incorrect final value
// if the update function were non-idempotent (it is not, today — Fields are
// scalar strings — but the assertion below checks the update landed exactly
// as given, catching a future non-idempotent-update regression too). The
// findings-double-append half is GREEN already: outgoingRowFindingsByID's
// per-swap capture + fold only ever runs once per applyResourcesLoaded call.
func TestStage4Pin_FieldUpdatesApplyExactlyOnce_NoDualApplyThroughDeadMirror(t *testing.T) {
	ctrl := newStage4PinController(t)
	openTopLevelList(ctrl, stage4PinType)

	ctrl.ApplyResourcesLoaded(stage4PinType, []resource.Resource{
		{ID: "bucket-fieldupdate-1", Name: "bucket-fieldupdate-1", Type: stage4PinType, Fields: map[string]string{"status": "ok"}},
	}, nil, false)

	ctrl.ApplyListFieldUpdates(stage4PinType, map[string]map[string]string{
		"bucket-fieldupdate-1": {"instance_status": "42.00"},
	})

	snap := ctrl.Snapshot()
	if snap.Body.List == nil || len(snap.Body.List.Rows) != 1 {
		t.Fatalf("precondition: list = %+v, want 1 row", snap.Body.List)
	}
	all := ctrl.GetListAllResources()
	var got resource.Resource
	for _, r := range all {
		if r.ID == "bucket-fieldupdate-1" {
			got = r
		}
	}
	if got.Fields["instance_status"] != "42.00" {
		t.Fatalf("Fields[instance_status] = %q after ApplyListFieldUpdates, want exactly %q (no dual-apply corruption)", got.Fields["instance_status"], "42.00")
	}

	// A finding seeded before the update, then a subsequent silent swap: the
	// carried finding must appear exactly once, not twice.
	dupFinding := domain.Finding{
		Code:     "s3-public-read",
		Phrase:   "public read",
		Severity: domain.SevBroken,
		Source:   "wave2:s3",
	}
	ctrl.ApplyResourcesLoaded(stage4PinType, []resource.Resource{
		{ID: "bucket-fieldupdate-1", Name: "bucket-fieldupdate-1", Type: stage4PinType, Findings: []domain.Finding{dupFinding}},
	}, nil, false)
	// Silent swap: same ID, zero findings, enrichment store empty.
	ctrl.ApplyResourcesLoaded(stage4PinType, []resource.Resource{
		{ID: "bucket-fieldupdate-1", Name: "bucket-fieldupdate-1", Type: stage4PinType},
	}, nil, false)

	postAll := ctrl.GetListAllResources()
	var postGot resource.Resource
	for _, r := range postAll {
		if r.ID == "bucket-fieldupdate-1" {
			postGot = r
		}
	}
	matchCount := 0
	for _, f := range postGot.Findings {
		if f.Code == dupFinding.Code {
			matchCount++
		}
	}
	if matchCount != 1 {
		t.Errorf("bucket-fieldupdate-1 carries %d copies of finding %q after a silent swap, want exactly 1 (no double-apply through ls.Rows + the dead resourceCache mirror)", matchCount, dupFinding.Code)
	}
}
