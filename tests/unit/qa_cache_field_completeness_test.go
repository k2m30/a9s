// qa_cache_field_completeness_test.go — RED pins for three live-verified
// defects surfaced by a real user's cache file + session on branch
// feat/cache (see the architect dispatch for the field report):
//
//  1. PersistedRows_CarryEveryRenderableColumn: a persisted s3 Row lacked the
//     "region" field even though the list renders a Region column — the cells
//     stayed empty until a later fetch replaced the seeded rows.
//     MaterializeListFields (core/app/list_columns.go) only fires when
//     r.RawStruct != nil (its very first line: "if r.RawStruct == nil {
//     return r }"). A fresh live fetch always carries a real RawStruct
//     (Scenario A below, which already passes at HEAD for every type). A
//     gap-carrying on-disk row (RawStruct == nil, so the missing field is not
//     locally reconstructable) is not expected to self-heal with NO fetch at
//     all — C1 (every screen entry re-verifies immediately) instead promises
//     the gap heals within ONE verify cycle: the seeded render tolerates the
//     empty cell (stale-marked, no crash), the verify-refetch lands fresh
//     rows carrying a real RawStruct, and the save the coder's materialize
//     seam performs on that result must persist the now-complete Fields.
//     Scenario B pins that one-cycle healing contract.
//
//  2. PoisonedExact_HealsOnContradiction: the user's on-disk file carried
//     count:50/exact:true from an old build. Once a genuine fetch reaches the
//     SAME 50 rows but is STILL truncated (a real next-page token exists),
//     C5's one-way ratchet ("exactness only ever advances", probes.go
//     SaveResourceListCache) has no path back down — Exact never re-derives
//     from a later contradicting observation, so the list keeps rendering a
//     bare "50" (no "+", no "m" load-more hint) forever even though the
//     fetcher is telling it there is more.
//
//  3. SilentSwap_NeverDropsKnownFindings: applyResourcesLoaded
//     (core/app/list_body.go) re-applies Wave-2 findings onto a freshly
//     swapped-in page ONLY from c.enrichmentStore (the session-scoped Wave-2
//     map) — see its "known := c.listEnrichmentFindings(typeName)" tail. It
//     never consults the OUTGOING rows' own persisted r.Findings. A cold-boot
//     reseed populates ls.Rows with findings straight from cache.Row.Findings
//     (no enrichment probe has run yet this session), so the enrichment store
//     is empty — the very first silent swap (case default: ls.Rows =
//     resources, list_body.go line ~76) throws every persisted finding away
//     with no re-derivation opportunity, and the row's glyph vanishes until
//     the NEXT independent enrichment sweep completes.
package unit

import (
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	_ "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/cache"
	"github.com/k2m30/a9s/v3/core/config"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
)

// ─────────────────────────────────────────────────────────────────────────
// Shared plumbing
// ─────────────────────────────────────────────────────────────────────────

// fieldCompletenessPair builds a fresh, hermetic Controller/Core pair backed
// by a per-test temp cache directory. Callers must still open a top-level
// list screen (openTopLevelList) via the same ActionCommand entry point
// production code uses (core/app/actions_view.go handleActionCommand) so
// maybeSaveResourceListCache's C6 scope gate (screen.ID ==
// ScreenResourceList, EscPops=false, ParentContext=nil) is satisfied and a
// subsequent ApplyResourcesLoaded call actually reaches disk. Mirrors
// alltypesSweepPair in qa_alltypes_cache_sweep_test.go.
func fieldCompletenessPair(t *testing.T, profile, region string) *app.Controller {
	t.Helper()
	core := runtime.Bootstrap(profile, region, resource.AllResourceTypes())
	ctrl := newBlessedController(t, core)
	t.Cleanup(ctrl.Close)
	ctrl.SetUIMode("web")
	return ctrl
}

// openTopLevelList opens shortName's real top-level list on ctrl.
func openTopLevelList(ctrl *app.Controller, shortName string) {
	ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: shortName})
}

// rawStructNode is one level of the shared nested-field tree used to build a
// single RawStruct value that satisfies every target column's Path
// simultaneously. Two columns whose Paths share a leading segment (e.g.
// elb's "State.Code" and "State.Reason" both start with "State") MUST
// resolve to the SAME nested struct field rather than two independently
// built top-level fields of the same name — reflect.StructOf panics on a
// duplicate field name, and this collision is common: many resource types
// have multiple columns hanging off one nested AWS SDK sub-struct.
type rawStructNode struct {
	children map[string]*rawStructNode
	leaf     string // leaf string value; set only when this node has no children
	hasLeaf  bool
}

func newRawStructNode() *rawStructNode {
	return &rawStructNode{children: map[string]*rawStructNode{}}
}

// insert adds dotPath -> leafValue into the tree, creating intermediate
// nodes as needed.
func (n *rawStructNode) insert(dotPath, leafValue string) {
	segments := strings.Split(dotPath, ".")
	cur := n
	for _, seg := range segments[:len(segments)-1] {
		child, ok := cur.children[seg]
		if !ok {
			child = newRawStructNode()
			cur.children[seg] = child
		}
		cur = child
	}
	last := segments[len(segments)-1]
	child, ok := cur.children[last]
	if !ok {
		child = newRawStructNode()
		cur.children[last] = child
	}
	child.leaf = leafValue
	child.hasLeaf = true
}

// build converts this node into a reflect.Value. A node with children (even
// if it also has a leaf — leaf is only ever set on true leaves by insert)
// becomes a nested struct; a childless leaf node becomes a plain string
// field value.
func (n *rawStructNode) buildValue() reflect.Value {
	if len(n.children) == 0 {
		return reflect.ValueOf(n.leaf)
	}
	names := make([]string, 0, len(n.children))
	for name := range n.children {
		names = append(names, name)
	}
	sort.Strings(names)
	fields := make([]reflect.StructField, len(names))
	values := make([]reflect.Value, len(names))
	for i, name := range names {
		v := n.children[name].buildValue()
		fields[i] = reflect.StructField{Name: name, Type: v.Type()}
		values[i] = v
	}
	structType := reflect.StructOf(fields)
	out := reflect.New(structType).Elem()
	for i := range fields {
		out.Field(i).Set(values[i])
	}
	return out
}

// pathBackedKeylessColumns returns every column in cols that is Path-backed
// and Key-less (col.Key == "" && col.Path != "") — exactly the set
// MaterializeListFields targets — excluding the two column classes that are
// genuinely derive-at-render rather than persist-verbatim: the identity
// column ("@id", never Path-backed in practice) and the status/lifecycle
// column (whose cell is overridden at render time from Findings, per
// listExtractCellValue's isStatusCol branch and buildListBody's S4
// override — persisting a materialized value for it would be actively
// wrong, not merely superfluous).
func pathBackedKeylessColumns(cols []app.ColumnDef, lifecycleKey string) []app.ColumnDef {
	if lifecycleKey == "" {
		lifecycleKey = "state"
	}
	var out []app.ColumnDef
	for _, c := range cols {
		if c.Key != "" || c.Path == "" {
			continue
		}
		if c.Key == "status" || c.Key == lifecycleKey {
			continue
		}
		out = append(out, c)
	}
	return out
}

// ─────────────────────────────────────────────────────────────────────────
// Pin 1 — PersistedRows_CarryEveryRenderableColumn (ALL-TYPES)
// ─────────────────────────────────────────────────────────────────────────

// TestPersistedRows_CarryEveryRenderableColumn drives, for every registered
// resource type, a real list-open + ResourcesLoaded save with a resource
// whose RawStruct can materialize each Path-backed/Key-less column the
// type's resolved default column set declares — via TWO distinct scenarios,
// mirroring the two ways a live user's cache file actually acquires rows —
// and asserts the persisted cache.Row.Fields carries a value under EVERY
// such column's resolved key in BOTH cases.
//
// Scenario A (fresh live fetch): a resource carrying a real RawStruct
// satisfying every target column's Path is run through
// ApplyResourcesLoaded and must persist every column via
// MaterializeListFields. This passes at HEAD for every type — pinned here
// as a completeness guard, not the regression itself.
//
// Scenario B (cold-boot cache reseed, the live defect's actual mechanism):
// an old-format on-disk Row that is MISSING a target column's Fields key
// (e.g. an s3 Row saved by a build that predates that column, or any
// partial-write gap) is loaded back via rowsFromCacheRows
// (core/runtime/handlers_availability.go) into a resource.Resource with
// RawStruct == nil (disk never carries RawStruct, C6). RED at HEAD: once
// reseeded this way, MaterializeListFields's very first line ("if
// r.RawStruct == nil { return r }") makes the missing column PERMANENTLY
// unrecoverable by any subsequent save of that same seeded state — a
// re-list-open + re-save round trip (exactly what a user re-opening a9s and
// letting the list re-persist would do without a genuine live re-fetch
// replacing the row) still writes the SAME gap back to disk. This is the
// live bug's actual shape: "seeded rows show empty cells until the swap" —
// only a genuine live fetch (fresh RawStruct) heals it, never a cache
// round-trip.
func TestPersistedRows_CarryEveryRenderableColumn(t *testing.T) {
	types := resource.AllResourceTypes()
	tested := 0

	for _, td := range types {
		t.Run(td.ShortName, func(t *testing.T) {
			// (*app.Controller).ResolveColumnsForType, driven with a nil
			// viewConfig below, mirrors the EXACT resolution
			// materializeListFieldsForType itself uses
			// (resolveListColumnsForBuild): when td.Columns is already as
			// large as the built-in default view's column count, the
			// superset check does not fire and td.Columns wins, with Path
			// merged in only by title match — often leaving a column BOTH
			// Key- and Path-populated (Key wins at extraction time, so
			// MaterializeListFields's Key-less guard correctly skips it).
			// Reading raw config.GetViewDef(nil, ...).List directly
			// (bypassing this resolution) tests the WRONG column set for
			// any type whose td.Columns is not strictly smaller than the
			// default view's column count (e.g. lambda: 6 Columns vs 6
			// default List entries).
			t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
			colsCtrl := fieldCompletenessPair(t, "fieldcomplete-defaultcols-"+td.ShortName, "us-east-1")
			cols := colsCtrl.ResolveColumnsForType(td.ShortName)
			targets := pathBackedKeylessColumns(cols, td.LifecycleKey)
			if len(targets) == 0 {
				t.Skip("no Path-backed, Key-less column in the default view — nothing for MaterializeListFields to persist for this type")
			}
			tested++

			t.Run("scenario_A_fresh_live_fetch_materializes", func(t *testing.T) {
				tmp := t.TempDir()
				t.Setenv("A9S_CONFIG_FOLDER", tmp)
				profile, region := "fieldcomplete-a-"+td.ShortName, "us-east-1"

				ctrl := fieldCompletenessPair(t, profile, region)
				openTopLevelList(ctrl, td.ShortName)

				// Build ONE resource whose RawStruct satisfies every target
				// column's Path simultaneously, merging shared path segments
				// into one nested struct tree (buildMultiColumnRawStruct).
				resVal, wantByKey := buildMultiColumnRawStruct(targets)

				resources := []resource.Resource{
					{ID: "fc-" + td.ShortName + "-1", Name: "fc-" + td.ShortName + "-1", Type: td.ShortName, RawStruct: resVal},
				}
				ctrl.ApplyResourcesLoaded(td.ShortName, resources, nil, false)

				ctrl.WaitForCacheWrites()
				store := cache.LoadDirForTest(profile, region)
				tf, ok := store.Type(td.ShortName)
				if !ok {
					t.Fatalf("%s: cache.LoadDirForTest().Type(%q) missing after ApplyResourcesLoaded — list-open save never persisted", td.ShortName, td.ShortName)
				}
				if len(tf.Rows) != 1 {
					t.Fatalf("%s: persisted Rows = %d, want 1", td.ShortName, len(tf.Rows))
				}
				row := tf.Rows[0]
				for key, want := range wantByKey {
					got, present := row.Fields[key]
					if !present {
						t.Errorf("%s: persisted Row.Fields missing key %q (column Path-backed, Key-less) after a FRESH live fetch with a satisfying RawStruct — neither materializer wrote it (app.MaterializeListFields on the render lane, runtime.saveFieldKey on the save lane) or its output was dropped before persistence", td.ShortName, key)
						continue
					}
					if got != want {
						t.Errorf("%s: persisted Row.Fields[%q] = %q, want %q", td.ShortName, key, got, want)
					}
				}
			})

			t.Run("scenario_B_gap_heals_after_one_verify_cycle", func(t *testing.T) {
				tmp := t.TempDir()
				t.Setenv("A9S_CONFIG_FOLDER", tmp)
				profile, region := "fieldcomplete-b-"+td.ShortName, "us-east-1"

				// Step 1: seed the disk file with a gap-row — carries every
				// OTHER target column's key but is missing one
				// (gapKey/targets[0]). RawStruct is never persisted (C6), so
				// this gap is not locally reconstructable — the only way it
				// can close is a genuine live re-fetch (steps 3-4).
				gapKey := config.TitleFieldKey(targets[0].Title)
				fields := map[string]string{"unrelated-preexisting-field": "kept"}
				seedStore := cache.LoadDirForTest(profile, region)
				seedStore.Put(td.ShortName, cache.TypeFile{
					HasResources: true,
					Count:        1,
					Exact:        true,
					Rows: []cache.Row{
						{ID: "fc-gap-" + td.ShortName + "-1", Name: "fc-gap-" + td.ShortName + "-1", Fields: fields},
					},
				})
				if err := seedStore.SaveType(td.ShortName); err != nil {
					t.Fatalf("%s: seeding gapped Row: %v", td.ShortName, err)
				}

				// Step 2: cold-boot the controller and open the list — the
				// seeded render. The gap cell must render empty (the
				// resolved key absent from Fields) WITHOUT crashing; C1
				// still marks this stale (Refreshing=true) pending the
				// verify-refetch below.
				_, ctrl := alltypesSweepPair(t, profile, region)
				func() {
					defer func() {
						if r := recover(); r != nil {
							t.Fatalf("%s: opening the list from a gap-seeded disk row panicked: %v", td.ShortName, r)
						}
					}()
					ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: td.ShortName})
				}()

				snap := ctrl.Snapshot()
				if snap.Body.List == nil || len(snap.Body.List.Rows) != 1 {
					t.Fatalf("%s: fixture assumption broken — expected the gapped Row to seed exactly 1 row on open", td.ShortName)
				}
				seededAll := ctrl.GetListAllResources()
				if len(seededAll) != 1 {
					t.Fatalf("%s: expected exactly 1 seeded resource, got %d", td.ShortName, len(seededAll))
				}
				if got := seededAll[0].Fields[gapKey]; got != "" {
					t.Errorf("%s: seeded (pre-verify) resource.Fields[%q] = %q, want empty — the gap must render empty on the seeded frame, not a fabricated value", td.ShortName, gapKey, got)
				}

				// Step 3: deliver the verify-fetch result — fresh resources
				// WITH a real RawStruct satisfying every target column's
				// Path, exactly what a live fetcher call produces.
				resVal, wantByKey := buildMultiColumnRawStruct(targets)
				verified := []resource.Resource{
					{ID: "fc-gap-" + td.ShortName + "-1", Name: "fc-gap-" + td.ShortName + "-1", Type: td.ShortName, RawStruct: resVal},
				}
				ctrl.ApplyResourcesLoaded(td.ShortName, verified, nil, false)

				// Step 4: drive the save (the coder's save-seam
				// materialization guarantee applies to this fetch-result
				// save the same way it does for Scenario A).
				ctrl.WaitForCacheWrites()
				store := cache.LoadDirForTest(profile, region)
				tf, ok := store.Type(td.ShortName)
				if !ok {
					t.Fatalf("%s: cache.LoadDirForTest().Type(%q) missing after the verify-fetch save", td.ShortName, td.ShortName)
				}
				if len(tf.Rows) != 1 {
					t.Fatalf("%s: persisted Rows = %d after the verify-fetch save, want 1", td.ShortName, len(tf.Rows))
				}

				// Step 5: re-read the file — the previously-missing field
				// key must now be present, and every other target column
				// must also have materialized (the same completeness bar
				// Scenario A holds a fresh fetch to).
				row := tf.Rows[0]
				for key, want := range wantByKey {
					got, present := row.Fields[key]
					if !present {
						t.Errorf("%s: persisted Row.Fields missing key %q after ONE verify cycle following a gap-seeded cold boot — the gap must heal within one verify cycle (C1), not persist forever", td.ShortName, key)
						continue
					}
					if got != want {
						t.Errorf("%s: persisted Row.Fields[%q] = %q, want %q", td.ShortName, key, got, want)
					}
				}
				if _, present := row.Fields[gapKey]; !present {
					t.Errorf("%s: persisted Row.Fields still missing the originally-gapped key %q after the verify cycle — the seed-time gap must not survive a genuine live re-fetch + save", td.ShortName, gapKey)
				}
			})
		})
	}

	if tested == 0 {
		t.Fatal("no registered resource type has a Path-backed, Key-less default column — the generic drive found nothing to verify; investigate config.GetViewDef/DefaultViewDef wiring")
	}
}

// buildMultiColumnRawStruct builds a single RawStruct value satisfying every
// column in cols's own Path simultaneously, merging any columns that share
// leading path segments (e.g. elb's "State.Code"/"State.Reason") into the
// SAME nested struct field rather than colliding on a duplicate top-level
// field name. fieldpath.ExtractValue requires reflect.Kind == Struct at
// every segment and matches fields by JSON tag first, then case-insensitive
// Go field name — bare exported fields named after each path segment
// satisfy the fallback generically, with no per-type knowledge of real AWS
// SDK shapes required.
//
// Two DISTINCT columns may legitimately share the exact same Path (e.g.
// sns's "Topic Name"/"Topic ARN" both read TopicArn, or sns-sub's
// "Confirmed"/"Subscription ARN" both read SubscriptionArn — a display-label
// alias, not a bug) — such columns necessarily extract the SAME leaf value
// from the one shared struct node, so the leaf value is assigned per unique
// Path, not per column index, and every column sharing that Path expects
// that same value under its own resolved Fields key.
//
// Returns the constructed value and a map of expected Fields-key (lowercased
// column Title, matching MaterializeListFields' resolved key) -> expected
// leaf string value.
func buildMultiColumnRawStruct(cols []app.ColumnDef) (any, map[string]string) {
	root := newRawStructNode()
	want := make(map[string]string, len(cols))
	leafByPath := make(map[string]string, len(cols))
	nextIdx := 0
	for _, c := range cols {
		leafValue, ok := leafByPath[c.Path]
		if !ok {
			leafValue = "fc-val-" + strconv.Itoa(nextIdx)
			nextIdx++
			leafByPath[c.Path] = leafValue
			root.insert(c.Path, leafValue)
		}
		// config.TitleFieldKey, not a second copy of the spelling rule:
		// the spec's row 5 leaves exactly one Fields key per column title,
		// and the save lane writes it under the spelling the extraction
		// cascade reads first. Asserting the spaced spelling here pinned the
		// collision that made a replayed cell depend on map order; do not
		// restore it.
		key := config.TitleFieldKey(c.Title)
		want[key] = leafValue
	}
	return root.buildValue().Interface(), want
}

// ─────────────────────────────────────────────────────────────────────────
// Pin 2 — PoisonedExact_HealsOnContradiction
// ─────────────────────────────────────────────────────────────────────────

// TestPoisonedExact_HealsOnContradiction seeds a disk file with a poisoned
// count:50/exact:true/rows:50 pair (the shape an old build could have left
// behind), then drives a genuine fetch result that reaches the SAME 50 rows
// while STILL truncated (a real pagination token is present). The
// reconciled file must heal: Exact must drop to false (truncated), and the
// resulting list title must show "50+" with load-more (m) enabled.
//
// RED at HEAD: probes.go's SaveResourceListCache applies "exactness only
// ever advances" unconditionally — `if !exact && existing.Exact { incoming.
// Exact = true; incoming.Count = existing.Count }` — with no contradiction
// check against the incoming pagination signal, so a stale exact:true poison
// can never be un-stuck by a later, truthful truncated observation.
func TestPoisonedExact_HealsOnContradiction(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)
	const profile, region = "poisoned-exact-prof", "us-east-1"

	// Seed the poisoned on-disk pair directly.
	rows := make([]cache.Row, 50)
	for i := range rows {
		rows[i] = cache.Row{ID: "obj-" + strconv.Itoa(i), Name: "obj-" + strconv.Itoa(i)}
	}
	store := cache.LoadDirForTest(profile, region)
	store.Put("s3", cache.TypeFile{HasResources: true, Count: 50, Exact: true, Rows: rows})
	if err := store.SaveType("s3"); err != nil {
		t.Fatalf("seeding poisoned exact pair: %v", err)
	}

	core := runtime.Bootstrap(profile, region, resource.AllResourceTypes())
	ctrl := newBlessedController(t, core)
	t.Cleanup(ctrl.Close)
	ctrl.SetUIMode("web")
	openTopLevelList(ctrl, "s3")

	// A genuine fetch reaches the SAME 50 rows but reports there is still
	// more (a real next-page token) — the contradiction the poisoned exact
	// flag must yield to.
	freshRows := make([]resource.Resource, 55)
	for i := range freshRows {
		freshRows[i] = resource.Resource{ID: "obj-" + strconv.Itoa(i), Type: "s3"}
	}
	ctrl.ApplyResourcesLoaded("s3", freshRows, &resource.PaginationMeta{IsTruncated: true, NextToken: "tok-heal"}, false)

	ctrl.WaitForCacheWrites()
	reloaded := cache.LoadDirForTest(profile, region)
	tf, ok := reloaded.Type("s3")
	if !ok {
		t.Fatal("cache.LoadDirForTest().Type(\"s3\") missing after the healing fetch")
	}
	if tf.Exact {
		t.Error("persisted s3 TypeFile.Exact = true after a contradicting truncated fetch with 55 known rows, want false — poisoned exact must heal, not stick forever")
	}

	snap := ctrl.Snapshot()
	lb := snap.Body.List
	if lb == nil {
		t.Fatal("Body.List is nil after the healing fetch")
	}
	if !lb.Truncated {
		t.Error("ListBody.Truncated = false, want true — the healed list must re-enable the load-more (m) hint")
	}
	title := ctrl.ListFrameTitle()
	if !strings.Contains(title, "55+") {
		t.Errorf("list frame title = %q, want it to contain \"55+\" (the true total with the truncation marker, not a bare \"55\" or the stale \"50\")", title)
	}
	if strings.Contains(title, "50") {
		t.Errorf("list frame title = %q, must not still show the poisoned stale total \"50\"", title)
	}
}

// ─────────────────────────────────────────────────────────────────────────
// Pin 3 — SilentSwap_NeverDropsKnownFindings
// ─────────────────────────────────────────────────────────────────────────

// TestSilentSwap_NeverDropsKnownFindings seeds a top-level list with rows
// that already carry a persisted WAVE-2 finding (glyph) — the user-visible
// case that actually motivated this pin: an enrichment-derived issue like
// "PITR off" or "public access block disabled" — with the session's Wave-2
// enrichment store left EMPTY (a genuinely fresh session, no enrichment
// probe has completed yet this session). A replace then lands with the SAME
// row IDs but carrying NO findings at all (the live fetch shape: a fresh
// Wave-1 fetch result, before the next enrichment sweep re-checks this
// type). The post-swap rows must still carry the previously-known WAVE-2
// finding (inherited, since enrichment genuinely has not re-run yet), not
// silently drop it.
//
// Contract (fixed, mirrors qa_cache_lifecycle_test.go's Scenario 3 finding
// and core/app/list_body.go's applyResourcesLoaded carry-forward): a
// fresh fetch result IS the authoritative statement about WAVE-1 state for a
// row — a row that comes back with zero findings this time means any
// WAVE-1-sourced issue is RESOLVED, and carrying that old Wave-1 finding
// forward would make a fixed issue immortal. Only the "wave2:"-prefixed
// portion of a prior finding — the enrichment pass, which runs separately
// from the fetch and has genuinely not re-checked this row yet — may
// outlive a silent swap, and only until the next enrichment sweep completes.
func TestSilentSwap_NeverDropsKnownFindings(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)
	const profile, region = "silentswap-findings-prof", "us-east-1"

	core := runtime.Bootstrap(profile, region, resource.AllResourceTypes())
	ctrl := newBlessedController(t, core)
	t.Cleanup(ctrl.Close)
	ctrl.SetUIMode("web")
	openTopLevelList(ctrl, "s3")

	seededFinding := domain.Finding{
		Code:     "s3-public-read",
		Phrase:   "public read",
		Severity: domain.SevBroken,
		Source:   "wave2:s3",
	}
	seeded := []resource.Resource{
		{ID: "bucket-known-1", Name: "bucket-known-1", Type: "s3", Findings: []domain.Finding{seededFinding}},
		{ID: "bucket-known-2", Name: "bucket-known-2", Type: "s3"},
	}
	ctrl.ApplyResourcesLoaded("s3", seeded, nil, false)

	// Since the color-findings-conformance wave, colorS3 is
	// colorFromAnyFinding-only (core/aws/catalog_databases.go) — a
	// SevBroken Finding resolves the row's whole-row color to "broken"
	// directly (the glyph branch that used to fire when
	// ResolveColor()==ColorHealthy was deleted as unreachable).
	// The stronger, correct check is ListRow.Color=="broken", not the glyph
	// Decorator.
	preSwap := ctrl.Snapshot()
	preRows := preSwap.Body.List.Rows
	foundBrokenBeforeSwap := false
	for _, r := range preRows {
		if r.ResourceID == "bucket-known-1" && r.Color == "broken" {
			foundBrokenBeforeSwap = true
		}
	}
	if !foundBrokenBeforeSwap {
		t.Fatal("fixture assumption broken — seeded bucket-known-1 does not render as a broken row before the swap")
	}

	// Silent swap: same IDs, no findings attached, enrichment store empty
	// (never populated in this test — a genuinely fresh session).
	replacement := []resource.Resource{
		{ID: "bucket-known-1", Name: "bucket-known-1", Type: "s3"},
		{ID: "bucket-known-2", Name: "bucket-known-2", Type: "s3"},
	}
	ctrl.ApplyResourcesLoaded("s3", replacement, nil, false)

	postSwap := ctrl.Snapshot()
	lb := postSwap.Body.List
	if lb == nil {
		t.Fatal("Body.List is nil after the silent swap")
	}
	postBroken := false
	for _, r := range lb.Rows {
		if r.ResourceID == "bucket-known-1" && r.Color == "broken" {
			postBroken = true
		}
	}
	if !postBroken {
		t.Error("bucket-known-1's broken row color vanished after the silent swap even though the enrichment store never changed — the row's previously-known WAVE-2 finding must be inherited, not silently dropped")
	}

	all := ctrl.GetListAllResources()
	var gotFindings []domain.Finding
	for _, r := range all {
		if r.ID == "bucket-known-1" {
			gotFindings = r.Findings
		}
	}
	if len(gotFindings) == 0 {
		t.Error("post-swap resource.Findings for bucket-known-1 is empty, want the inherited seeded WAVE-2 finding to still be present")
	} else if gotFindings[0].Code != seededFinding.Code {
		t.Errorf("post-swap resource.Findings[0].Code = %q, want %q (the inherited seeded WAVE-2 finding)", gotFindings[0].Code, seededFinding.Code)
	}
}

// TestSilentSwap_Wave1FindingNotCarriedOnResolve is the counterpart to
// TestSilentSwap_NeverDropsKnownFindings: a row seeded with a WAVE-1-sourced
// finding (Source: "wave1", the fetcher's own per-fetch observation, e.g. an
// EC2 instance's "stopped" state or an S3 bucket flagged public-read by the
// fetcher itself) must NOT have that finding carried forward onto a silent
// swap's fresh, zero-findings replacement row — a fresh fetch result with no
// Wave-1 finding for that ID IS the authoritative "this is resolved now"
// signal (mirrors qa_cache_lifecycle_test.go's Scenario 3: one resource's
// issue resolves between boots). Only "wave2:"-prefixed findings survive a
// silent swap; this pins the Wave-1 half of that same carry-forward rule.
func TestSilentSwap_Wave1FindingNotCarriedOnResolve(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)
	const profile, region = "silentswap-wave1-resolve-prof", "us-east-1"

	core := runtime.Bootstrap(profile, region, resource.AllResourceTypes())
	ctrl := newBlessedController(t, core)
	t.Cleanup(ctrl.Close)
	ctrl.SetUIMode("web")
	openTopLevelList(ctrl, "s3")

	wave1Finding := domain.Finding{
		Code:     "s3-public-read",
		Phrase:   "public read",
		Severity: domain.SevBroken,
		Source:   "wave1",
	}
	seeded := []resource.Resource{
		{ID: "bucket-resolve-1", Name: "bucket-resolve-1", Type: "s3", Findings: []domain.Finding{wave1Finding}},
		{ID: "bucket-resolve-2", Name: "bucket-resolve-2", Type: "s3"},
	}
	ctrl.ApplyResourcesLoaded("s3", seeded, nil, false)

	// Since the color-findings-conformance wave, colorS3 is
	// colorFromAnyFinding-only (core/aws/catalog_databases.go) — a
	// SevBroken Finding resolves the row's whole-row color to "broken"
	// directly (the glyph branch that used to fire when
	// ResolveColor()==ColorHealthy was deleted as unreachable).
	// The stronger, correct check is ListRow.Color=="broken", not the glyph
	// Decorator.
	preSwap := ctrl.Snapshot()
	foundBrokenBeforeSwap := false
	for _, r := range preSwap.Body.List.Rows {
		if r.ResourceID == "bucket-resolve-1" && r.Color == "broken" {
			foundBrokenBeforeSwap = true
		}
	}
	if !foundBrokenBeforeSwap {
		t.Fatal("fixture assumption broken — seeded bucket-resolve-1 does not render as a broken row before the swap")
	}

	// Silent swap: same IDs, no findings attached — the fresh fetch's
	// authoritative statement that the Wave-1 issue is now resolved.
	replacement := []resource.Resource{
		{ID: "bucket-resolve-1", Name: "bucket-resolve-1", Type: "s3"},
		{ID: "bucket-resolve-2", Name: "bucket-resolve-2", Type: "s3"},
	}
	ctrl.ApplyResourcesLoaded("s3", replacement, nil, false)

	postSwap := ctrl.Snapshot()
	lb := postSwap.Body.List
	if lb == nil {
		t.Fatal("Body.List is nil after the silent swap")
	}
	for _, r := range lb.Rows {
		if r.ResourceID == "bucket-resolve-1" && r.Color == "broken" {
			t.Error("bucket-resolve-1 still renders as a broken row after the silent swap — a Wave-1 finding absent from a fresh fetch result means RESOLVED and must not be carried forward")
		}
	}

	all := ctrl.GetListAllResources()
	var gotFindings []domain.Finding
	for _, r := range all {
		if r.ID == "bucket-resolve-1" {
			gotFindings = r.Findings
		}
	}
	if len(gotFindings) != 0 {
		t.Errorf("post-swap resource.Findings for bucket-resolve-1 = %+v, want empty — the resolved Wave-1 finding must not survive the swap", gotFindings)
	}
}
